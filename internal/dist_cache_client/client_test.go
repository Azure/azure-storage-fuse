// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dcache

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// mockServer is an in-process TCP server that speaks the distributed cache wire protocol.
type mockServer struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	store    map[string][]byte // cacheKey -> data
	attrs    map[string]*pb.FileAttribute
	locks    map[string]bool // cacheKey -> locked
	handler  func(req *pb.Request, data []byte) (proto.Message, []byte)
	closed   bool
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &mockServer{
		listener: l,
		addr:     l.Addr().String(),
		store:    make(map[string][]byte),
		attrs:    make(map[string]*pb.FileAttribute),
		locks:    make(map[string]bool),
	}

	go s.serve()
	return s
}

func (s *mockServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockServer) handleConn(nc net.Conn) {
	defer nc.Close()
	for {
		// Read 4-byte length
		var hdr [4]byte
		if _, err := io.ReadFull(nc, hdr[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(hdr[:])
		if length > 10*1024*1024 {
			return
		}

		// Read protobuf
		buf := make([]byte, length)
		if _, err := io.ReadFull(nc, buf); err != nil {
			return
		}

		var req pb.Request
		if err := proto.Unmarshal(buf, &req); err != nil {
			return
		}

		// Read upload data if present
		var uploadData []byte
		if upload := req.GetUploadrequest(); upload != nil && upload.Filesize > 0 {
			uploadData = make([]byte, upload.Filesize)
			if _, err := io.ReadFull(nc, uploadData); err != nil {
				return
			}
		}

		// Handle request
		var respMsg proto.Message
		var respData []byte

		if s.handler != nil {
			respMsg, respData = s.handler(&req, uploadData)
		} else {
			respMsg, respData = s.defaultHandler(&req, uploadData)
		}

		// Send response
		respBytes, err := proto.Marshal(respMsg)
		if err != nil {
			return
		}

		binary.BigEndian.PutUint32(hdr[:], uint32(len(respBytes)))
		nc.Write(hdr[:])
		nc.Write(respBytes)
		if len(respData) > 0 {
			nc.Write(respData)
		}
	}
}

func (s *mockServer) defaultHandler(req *pb.Request, uploadData []byte) (proto.Message, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch p := req.Payload.(type) {
	case *pb.Request_Uploadrequest:
		s.store[p.Uploadrequest.Filename] = append([]byte(nil), uploadData...)
		return &pb.UploadResponse{Result: pb.UploadResponse_SUCCESS}, nil

	case *pb.Request_Downloadrequest:
		data, ok := s.store[p.Downloadrequest.Filename]
		if !ok {
			if p.Downloadrequest.Enablelock {
				if s.locks[p.Downloadrequest.Filename] {
					return &pb.DownloadResponse{
						Result: pb.DownloadResponse_NOT_FOUND_ALREADY_LOCKED,
					}, nil
				}
				s.locks[p.Downloadrequest.Filename] = true
				return &pb.DownloadResponse{
					Result: pb.DownloadResponse_NOT_FOUND_GOT_LOCK,
				}, nil
			}
			return &pb.DownloadResponse{
				Result: pb.DownloadResponse_NOT_FOUND,
			}, nil
		}
		return &pb.DownloadResponse{
			Result:   pb.DownloadResponse_SUCCESS,
			Filesize: uint64(len(data)),
		}, data

	case *pb.Request_Deleterequest:
		if fn := p.Deleterequest.GetFilename(); fn != "" {
			delete(s.store, fn)
			delete(s.locks, fn)
		}
		return &pb.DeleteResponse{Result: pb.DeleteResponse_SUCCESS}, nil

	case *pb.Request_Getattributerequest:
		attr, ok := s.attrs[p.Getattributerequest.Filename]
		if !ok {
			return &pb.GetAttributeResponse{
				Result: pb.GetAttributeResponse_NOT_FOUND,
			}, nil
		}
		return &pb.GetAttributeResponse{
			Result:        pb.GetAttributeResponse_SUCCESS,
			Fileattribute: attr,
		}, nil

	case *pb.Request_Putattributerequest:
		for _, fa := range p.Putattributerequest.Fileattributes {
			s.attrs[fa.Filename] = fa.Fileattribute
		}
		return &pb.PutAttributeResponse{Result: pb.PutAttributeResponse_SUCCESS}, nil

	case *pb.Request_Getcacheserversrequest:
		return &pb.GetCacheServersResponse{
			Result:          pb.GetCacheServersResponse_SUCCESS,
			Serveraddresses: []string{s.addr},
		}, nil

	default:
		return &pb.UploadResponse{Result: pb.UploadResponse_INTERNAL_ERROR}, nil
	}
}

func (s *mockServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.listener.Close()
	}
}

// --- Client integration tests with mock server ---

func newTestClient(t *testing.T, server *mockServer) *Client {
	t.Helper()
	c, err := New(
		WithServerList([]string{server.addr}),
		WithCachePrefix("test/container"),
		WithChunkSize(16*1024*1024),
		WithDialTimeout(2*time.Second),
		WithRequestTimeout(5*time.Second),
		WithMaxConnsPerServer(4),
	)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	data := bytes.Repeat([]byte("hello world! "), 100)

	// Store attributes so Download can determine file size
	attrKey := GenerateAttrCacheKey("test/file.txt")
	srv.mu.Lock()
	srv.attrs[attrKey] = &pb.FileAttribute{Filesize: uint64(len(data))}
	srv.mu.Unlock()

	// Upload
	err := client.Upload(ctx, "test/file.txt", bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	// Download
	var buf bytes.Buffer
	meta, err := client.Download(ctx, "test/file.txt", &buf)
	require.NoError(t, err)
	assert.Equal(t, data, buf.Bytes())
	assert.Equal(t, int64(len(data)), meta.Size)
}

func TestUploadDownloadChunk(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	data := bytes.Repeat([]byte("A"), 4096)

	// Upload a single chunk
	err := client.UploadChunk(ctx, "file.bin", 0, data)
	require.NoError(t, err)

	// Download the chunk
	buf := make([]byte, 4096)
	n, err := client.DownloadChunk(ctx, "file.bin", 0, buf)
	require.NoError(t, err)
	assert.Equal(t, 4096, n)
	assert.Equal(t, data, buf[:n])
}

func TestDownloadNotFound(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	buf := make([]byte, 4096)
	_, err := client.DownloadChunk(ctx, "nonexistent", 0, buf)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDownloadLockProtocol(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	buf := make([]byte, 4096)

	// First download with lock — should get lock
	_, err := client.DownloadChunk(ctx, "locked-file", 0, buf, WithLock(true))
	assert.ErrorIs(t, err, ErrNotFoundGotLock)

	// Second download with lock — should see already locked
	_, err = client.DownloadChunk(ctx, "locked-file", 0, buf, WithLock(true))
	assert.ErrorIs(t, err, ErrNotFoundAlreadyLocked)
}

func TestDeleteFile(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	data := bytes.Repeat([]byte("B"), 1024)

	// Upload then delete
	err := client.UploadChunk(ctx, "file.bin", 0, data)
	require.NoError(t, err)

	err = client.Delete(ctx, "file.bin", 1024)
	require.NoError(t, err)

	// Verify deleted
	buf := make([]byte, 1024)
	_, err = client.DownloadChunk(ctx, "file.bin", 0, buf)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetPutAttr(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()

	// GetAttr — not found initially
	_, err := client.GetAttr(ctx, "file.bin")
	assert.ErrorIs(t, err, ErrNotFound)

	// PutAttr
	err = client.PutAttr(ctx, []FileAttrEntry{{
		Filename: "file.bin",
		Attr: FileAttr{
			IsDir:        false,
			Size:         12345,
			ModifiedTime: 1000,
		},
	}})
	require.NoError(t, err)

	// GetAttr — should succeed
	attr, err := client.GetAttr(ctx, "file.bin")
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), attr.Size)
	assert.Equal(t, uint64(1000), attr.ModifiedTime)
	assert.False(t, attr.IsDir)
}

func TestClientClose(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	client.Close()

	ctx := context.Background()
	err := client.Upload(ctx, "f", bytes.NewReader(nil), 0)
	assert.ErrorIs(t, err, ErrClosed)
}

func TestMultiChunkUploadDownload(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	// Use small chunk size for testing
	c, err := New(
		WithServerList([]string{srv.addr}),
		WithCachePrefix("test"),
		WithChunkSize(1024), // 1KB chunks for testing
		WithDialTimeout(2*time.Second),
		WithRequestTimeout(5*time.Second),
	)
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()

	// Create data larger than chunk size (3.5 chunks)
	data := bytes.Repeat([]byte("X"), 3584)

	// Store attributes so Download works
	attrKey := GenerateAttrCacheKey("multi-chunk.bin")
	srv.mu.Lock()
	srv.attrs[attrKey] = &pb.FileAttribute{Filesize: uint64(len(data))}
	srv.mu.Unlock()

	// Upload
	err = c.Upload(ctx, "multi-chunk.bin", bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	// Download
	var buf bytes.Buffer
	meta, err := c.Download(ctx, "multi-chunk.bin", &buf)
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), meta.Size)
	assert.Equal(t, data, buf.Bytes())
}

func TestServerDiscoveryRPC(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()

	// Use the mock server's GetCacheServers response for discovery
	c, err := New(
		WithDiscoveryURL(srv.addr),
		WithCachePrefix("test"),
	)
	require.NoError(t, err)
	defer c.Close()

	servers := c.Servers()
	assert.Contains(t, servers, srv.addr)
}

func TestConnectionPoolReuse(t *testing.T) {
	srv := newMockServer(t)
	defer srv.close()
	client := newTestClient(t, srv)

	ctx := context.Background()
	data := []byte("connection reuse test")

	// Perform multiple operations that should reuse connections
	for i := 0; i < 10; i++ {
		err := client.UploadChunk(ctx, "reuse-test", 0, data)
		require.NoError(t, err)
	}
}
