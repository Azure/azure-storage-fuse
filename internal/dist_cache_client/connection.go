// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
	"google.golang.org/protobuf/proto"
)

// conn wraps a TCP connection with buffered read/write and the wire protocol.
type conn struct {
	nc      net.Conn
	addr    string
	created time.Time
}

// sendRequest sends a protobuf Request followed by optional raw data.
// Wire format: [4-byte big-endian length of protobuf] [protobuf bytes] [raw data bytes]
func (c *conn) sendRequest(req *pb.Request, data []byte) error {
	msg, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Write 4-byte big-endian length prefix (of protobuf only)
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(msg)))
	if _, err := c.nc.Write(hdr[:]); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write protobuf bytes
	if _, err := c.nc.Write(msg); err != nil {
		return fmt.Errorf("write protobuf: %w", err)
	}

	// Write raw data if present (Upload)
	if len(data) > 0 {
		if _, err := c.nc.Write(data); err != nil {
			return fmt.Errorf("write data: %w", err)
		}
	}

	return nil
}

// recvProto reads a length-prefixed protobuf message from the connection.
func (c *conn) recvProto(msg proto.Message) error {
	// Read 4-byte length prefix
	var hdr [4]byte
	if _, err := io.ReadFull(c.nc, hdr[:]); err != nil {
		return fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(hdr[:])
	if length > uint32(defaultMaxMsgSize) {
		return fmt.Errorf("message too large: %d bytes (max %d)", length, defaultMaxMsgSize)
	}

	// Read protobuf bytes
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.nc, buf); err != nil {
		return fmt.Errorf("read protobuf: %w", err)
	}

	if err := proto.Unmarshal(buf, msg); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}

// recvDataToWriter reads exactly n bytes from the connection and writes to w.
// When w is an *os.File and the connection is TCP, Go's io.Copy will
// use splice(2) for zero-copy kernel-to-kernel transfer.
func (c *conn) recvDataToWriter(w io.Writer, n int64) (int64, error) {
	return io.Copy(w, io.LimitReader(c.nc, n))
}

// recvDataToBuffer reads exactly n bytes from the connection into buf.
func (c *conn) recvDataToBuffer(buf []byte) error {
	_, err := io.ReadFull(c.nc, buf)
	return err
}

func (c *conn) close() error {
	return c.nc.Close()
}

func (c *conn) setDeadline(d time.Time) error {
	return c.nc.SetDeadline(d)
}

// connPool manages a pool of TCP connections to a single server.
type connPool struct {
	mu       sync.Mutex
	addr     string
	conns    []*conn
	maxConns int
	dialTO   time.Duration
}

func newConnPool(addr string, maxConns int, dialTimeout time.Duration) *connPool {
	return &connPool{
		addr:     addr,
		conns:    make([]*conn, 0, maxConns),
		maxConns: maxConns,
		dialTO:   dialTimeout,
	}
}

// get retrieves a connection from the pool or dials a new one.
func (p *connPool) get() (*conn, error) {
	p.mu.Lock()
	if len(p.conns) > 0 {
		c := p.conns[len(p.conns)-1]
		p.conns = p.conns[:len(p.conns)-1]
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()

	return p.dial()
}

// put returns a connection to the pool. If the pool is full, the connection is closed.
func (p *connPool) put(c *conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.conns) < p.maxConns {
		p.conns = append(p.conns, c)
	} else {
		c.close()
	}
}

// discard closes a connection without returning it to the pool.
func (p *connPool) discard(c *conn) {
	if c != nil {
		c.close()
	}
}

func (p *connPool) dial() (*conn, error) {
	nc, err := net.DialTimeout("tcp", p.addr, p.dialTO)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConnectionFailed, p.addr, err)
	}

	if tc, ok := nc.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(30 * time.Second)
	}

	return &conn{
		nc:      nc,
		addr:    p.addr,
		created: time.Now(),
	}, nil
}

// closeAll closes all pooled connections.
func (p *connPool) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range p.conns {
		c.close()
	}
	p.conns = p.conns[:0]
}

// connManager manages connection pools for multiple servers.
type connManager struct {
	mu       sync.RWMutex
	pools    map[string]*connPool
	maxConns int
	dialTO   time.Duration
}

func newConnManager(maxConnsPerServer int, dialTimeout time.Duration) *connManager {
	return &connManager{
		pools:    make(map[string]*connPool),
		maxConns: maxConnsPerServer,
		dialTO:   dialTimeout,
	}
}

// getConn retrieves a connection to the specified server address.
func (m *connManager) getConn(addr string) (*conn, error) {
	m.mu.RLock()
	pool, ok := m.pools[addr]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		pool, ok = m.pools[addr]
		if !ok {
			pool = newConnPool(addr, m.maxConns, m.dialTO)
			m.pools[addr] = pool
		}
		m.mu.Unlock()
	}

	return pool.get()
}

// putConn returns a connection to its pool.
func (m *connManager) putConn(c *conn) {
	m.mu.RLock()
	pool, ok := m.pools[c.addr]
	m.mu.RUnlock()

	if ok {
		pool.put(c)
	} else {
		c.close()
	}
}

// discardConn closes a connection without returning it.
func (m *connManager) discardConn(c *conn) {
	if c != nil {
		c.close()
	}
}

// closeAll closes all connection pools.
func (m *connManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pool := range m.pools {
		pool.closeAll()
	}
	m.pools = make(map[string]*connPool)
}
