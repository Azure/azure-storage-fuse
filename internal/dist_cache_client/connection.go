// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
	"google.golang.org/protobuf/proto"
)

// protoPool reuses buffers for protobuf receive (response messages are small).
var protoPool = sync.Pool{
	New: func() any { return make([]byte, 0, 4096) },
}

// conn wraps a TCP connection with buffered I/O and the wire protocol.
type conn struct {
	nc      net.Conn
	br      *bufio.Reader // buffered reader reduces syscalls for small proto responses
	addr    string
	created time.Time
}

// sendRequest sends a protobuf Request followed by optional raw data.
// Header and protobuf are batched into a single write to reduce syscalls.
func (c *conn) sendRequest(req *pb.Request, data []byte) error {
	msg, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Batch header + protobuf into a single write
	batch := make([]byte, 4+len(msg))
	binary.BigEndian.PutUint32(batch, uint32(len(msg)))
	copy(batch[4:], msg)

	if _, err := c.nc.Write(batch); err != nil {
		return fmt.Errorf("write request: %w", err)
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
// Uses pooled buffers to reduce allocations.
func (c *conn) recvProto(msg proto.Message) error {
	// Read 4-byte length prefix via buffered reader
	var hdr [4]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(hdr[:])
	if length > uint32(defaultMaxMsgSize) {
		return fmt.Errorf("message too large: %d bytes (max %d)", length, defaultMaxMsgSize)
	}

	// Get pooled buffer, grow if needed
	buf := protoPool.Get().([]byte)
	if cap(buf) < int(length) {
		buf = make([]byte, length)
	} else {
		buf = buf[:length]
	}

	if _, err := io.ReadFull(c.br, buf); err != nil {
		protoPool.Put(buf[:0])
		return fmt.Errorf("read protobuf: %w", err)
	}

	err := proto.Unmarshal(buf, msg)
	protoPool.Put(buf[:0])
	if err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}

// recvDataToWriter reads exactly n bytes from the connection and writes to w.
// When w is an *os.File and the connection is TCP, Go's io.Copy will
// use splice(2) for zero-copy kernel-to-kernel transfer.
// Reads bypass the bufio.Reader to avoid double-buffering large data.
func (c *conn) recvDataToWriter(w io.Writer, n int64) (int64, error) {
	// Drain any bytes already in the bufio.Reader's buffer first
	buffered := c.br.Buffered()
	if buffered > 0 {
		take := int64(buffered)
		if take > n {
			take = n
		}
		written, err := io.CopyN(w, c.br, take)
		if err != nil {
			return written, err
		}
		n -= written
		if n == 0 {
			return written, nil
		}
		// Remaining bytes read directly from socket (enables splice)
		n2, err := io.Copy(w, io.LimitReader(c.nc, n))
		return written + n2, err
	}
	// Nothing buffered: read directly from socket (enables splice)
	return io.Copy(w, io.LimitReader(c.nc, n))
}

// recvDataToBuffer reads exactly n bytes from the connection into buf.
// Uses the buffered reader for any buffered data, then reads directly.
func (c *conn) recvDataToBuffer(buf []byte) error {
	_, err := io.ReadFull(c.br, buf)
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
	mu        sync.Mutex
	addr      string
	conns     []*conn
	maxConns  int
	dialTO    time.Duration
	sockBufSz int // SO_RCVBUF/SO_SNDBUF size (0 = system default)
}

func newConnPool(addr string, maxConns int, dialTimeout time.Duration, sockBufSize int) *connPool {
	return &connPool{
		addr:      addr,
		conns:     make([]*conn, 0, maxConns),
		maxConns:  maxConns,
		dialTO:    dialTimeout,
		sockBufSz: sockBufSize,
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
		if p.sockBufSz > 0 {
			tc.SetReadBuffer(p.sockBufSz)
			tc.SetWriteBuffer(p.sockBufSz)
		}
	}

	return &conn{
		nc:      nc,
		br:      bufio.NewReaderSize(nc, 64*1024), // 64KB read buffer
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
	mu        sync.RWMutex
	pools     map[string]*connPool
	maxConns  int
	dialTO    time.Duration
	sockBufSz int
}

func newConnManager(maxConnsPerServer int, dialTimeout time.Duration, sockBufSize int) *connManager {
	return &connManager{
		pools:     make(map[string]*connPool),
		maxConns:  maxConnsPerServer,
		dialTO:    dialTimeout,
		sockBufSz: sockBufSize,
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
			pool = newConnPool(addr, m.maxConns, m.dialTO, m.sockBufSz)
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
