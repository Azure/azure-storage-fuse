// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package dcache provides a Go client for the distributed cache protocol.
// Files are automatically split into fixed-size chunks and distributed across
// the cluster via consistent hashing. The client handles connection pooling,
// server discovery, and the lock-on-miss protocol for stampede prevention.
//
// This package has no blobfuse-specific imports and is designed for future
// extraction to the upstream distributed cache repository.
package dcache

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
)

// Client is a distributed cache client with chunked storage, connection pooling,
// and server discovery.
type Client struct {
	cfg     *clientConfig
	connMgr *connManager
	disc    *discovery

	// Buffer pool for chunk-sized allocations (reduces GC pressure)
	bufPool sync.Pool

	closed atomic.Bool
}

// New creates a new distributed cache client.
func New(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	connMgr := newConnManager(cfg.maxConnsPerSvr, cfg.dialTimeout, cfg.socketBufSize)

	disc, err := newDiscovery(cfg, connMgr, cfg.virtualNodes)
	if err != nil {
		connMgr.closeAll()
		return nil, fmt.Errorf("dcache: discovery: %w", err)
	}

	c := &Client{
		cfg:     cfg,
		connMgr: connMgr,
		disc:    disc,
		bufPool: sync.Pool{
			New: func() any {
				buf := make([]byte, cfg.chunkSize)
				return buf
			},
		},
	}

	return c, nil
}

// Upload stores a file in the distributed cache, splitting it into chunks.
// Each chunk is routed to a server via consistent hash of its chunk key.
func (c *Client) Upload(ctx context.Context, filename string, data io.Reader, size int64, opts ...UploadOption) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	ucfg := &uploadConfig{}
	for _, o := range opts {
		o(ucfg)
	}

	return c.uploadChunked(ctx, filename, data, size, ucfg)
}

// Download retrieves a complete file from the distributed cache, reassembling chunks.
// When w is an *os.File and the download is a single chunk, Go uses splice(2) for
// zero-copy kernel-to-kernel transfer.
func (c *Client) Download(ctx context.Context, filename string, w io.Writer, opts ...DownloadOption) (*FileMetadata, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	dcfg := &downloadConfig{}
	for _, o := range opts {
		o(dcfg)
	}

	// We need the file size to plan chunks. Get it from attributes first.
	attr, err := c.GetAttr(ctx, filename)
	if err != nil {
		return nil, err
	}

	return c.downloadChunked(ctx, filename, int64(attr.Size), w, dcfg)
}

// DownloadWithSize retrieves a file when the size is already known (avoids GetAttr call).
func (c *Client) DownloadWithSize(ctx context.Context, filename string, fileSize int64, w io.Writer, opts ...DownloadOption) (*FileMetadata, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	dcfg := &downloadConfig{}
	for _, o := range opts {
		o(dcfg)
	}

	return c.downloadChunked(ctx, filename, fileSize, w, dcfg)
}

// DownloadChunk retrieves a single chunk at the given offset.
// When chunkSize-aligned, this is a 1:1 mapping to one distributed cache entry.
// Ideal for block_cache integration where each ReadInBuffer = one chunk.
func (c *Client) DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...DownloadOption) (int, error) {
	if err := c.checkClosed(); err != nil {
		return 0, err
	}

	dcfg := &downloadConfig{}
	for _, o := range opts {
		o(dcfg)
	}

	cacheKey := GenerateCacheKey(c.cfg.cachePrefix, filename, offset, c.cfg.chunkSize)
	server, err := c.disc.getServer(cacheKey)
	if err != nil {
		return 0, err
	}

	plan := chunkPlan{
		offset:     offset,
		size:       int64(len(buf)),
		cacheKey:   cacheKey,
		serverAddr: server,
	}

	n, _, err := c.downloadSingleChunkToBuffer(ctx, plan, buf, dcfg)
	return n, err
}

// UploadChunk stores a single chunk at the given offset.
// Ideal for block_cache StageData integration.
func (c *Client) UploadChunk(ctx context.Context, filename string, offset int64, data []byte, opts ...UploadOption) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	ucfg := &uploadConfig{}
	for _, o := range opts {
		o(ucfg)
	}

	cacheKey := GenerateCacheKey(c.cfg.cachePrefix, filename, offset, c.cfg.chunkSize)
	server, err := c.disc.getServer(cacheKey)
	if err != nil {
		return err
	}

	plan := chunkPlan{
		offset:     offset,
		size:       int64(len(data)),
		cacheKey:   cacheKey,
		serverAddr: server,
	}

	return c.uploadSingleChunk(ctx, plan, data, ucfg)
}

// Delete removes all chunks of a file from the distributed cache.
func (c *Client) Delete(ctx context.Context, filename string, fileSize int64) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	plans, err := c.planChunks(filename, fileSize)
	if err != nil {
		return err
	}

	// Delete each chunk
	for _, plan := range plans {
		if err := c.deleteKey(ctx, plan.cacheKey, plan.serverAddr); err != nil {
			// Best-effort: continue deleting other chunks
			continue
		}
	}

	return nil
}

// DeleteGroup removes all files with the given group ID.
func (c *Client) DeleteGroup(ctx context.Context, groupID []byte) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	// Group delete must be sent to all servers
	servers := c.disc.getServers()
	for _, server := range servers {
		cn, err := c.connMgr.getConn(server)
		if err != nil {
			continue // best-effort
		}

		if err := cn.setDeadline(c.deadline(ctx)); err != nil {
			c.connMgr.discardConn(cn)
			continue
		}

		req := &pb.Request{
			Payload: &pb.Request_Deleterequest{
				Deleterequest: &pb.DeleteRequest{
					Type: &pb.DeleteRequest_Groupid{Groupid: groupID},
				},
			},
		}

		if err := cn.sendRequest(req, nil); err != nil {
			c.connMgr.discardConn(cn)
			continue
		}

		var resp pb.DeleteResponse
		if err := cn.recvProto(&resp); err != nil {
			c.connMgr.discardConn(cn)
			continue
		}

		c.connMgr.putConn(cn)
	}

	return nil
}

// GetAttr retrieves file attributes from the distributed cache.
func (c *Client) GetAttr(ctx context.Context, filename string) (*FileAttr, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	attrKey := GenerateAttrCacheKey(filename)
	server, err := c.disc.getServer(attrKey)
	if err != nil {
		return nil, err
	}

	cn, err := c.connMgr.getConn(server)
	if err != nil {
		return nil, err
	}

	if err := cn.setDeadline(c.deadline(ctx)); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	req := &pb.Request{
		Payload: &pb.Request_Getattributerequest{
			Getattributerequest: &pb.GetAttributeRequest{
				Filename: attrKey,
			},
		},
	}

	if err := cn.sendRequest(req, nil); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	var resp pb.GetAttributeResponse
	if err := cn.recvProto(&resp); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	c.connMgr.putConn(cn)

	switch resp.Result {
	case pb.GetAttributeResponse_SUCCESS:
		if resp.Fileattribute == nil {
			return nil, ErrServerError
		}
		return &FileAttr{
			IsDir:        resp.Fileattribute.Isdir,
			Size:         resp.Fileattribute.Filesize,
			AccessedTime: resp.Fileattribute.Accessedtime,
			ModifiedTime: resp.Fileattribute.Modifiedtime,
		}, nil
	case pb.GetAttributeResponse_NOT_FOUND_GOT_LOCK:
		return nil, ErrNotFoundGotLock
	case pb.GetAttributeResponse_NOT_FOUND:
		return nil, ErrNotFound
	case pb.GetAttributeResponse_AUTH_FAILED:
		return nil, ErrAuthFailed
	default:
		return nil, ErrServerError
	}
}

// PutAttr stores file attributes in the distributed cache.
func (c *Client) PutAttr(ctx context.Context, attrs []FileAttrEntry) error {
	if err := c.checkClosed(); err != nil {
		return err
	}

	// Group attributes by target server
	serverAttrs := make(map[string][]*pb.FileAttributes)
	for _, entry := range attrs {
		attrKey := GenerateAttrCacheKey(entry.Filename)
		server, err := c.disc.getServer(attrKey)
		if err != nil {
			return err
		}
		serverAttrs[server] = append(serverAttrs[server], &pb.FileAttributes{
			Filename: attrKey,
			Fileattribute: &pb.FileAttribute{
				Isdir:        entry.Attr.IsDir,
				Filesize:     entry.Attr.Size,
				Accessedtime: entry.Attr.AccessedTime,
				Modifiedtime: entry.Attr.ModifiedTime,
			},
		})
	}

	// Send to each server
	for server, faList := range serverAttrs {
		cn, err := c.connMgr.getConn(server)
		if err != nil {
			return err
		}

		if err := cn.setDeadline(c.deadline(ctx)); err != nil {
			c.connMgr.discardConn(cn)
			return err
		}

		req := &pb.Request{
			Payload: &pb.Request_Putattributerequest{
				Putattributerequest: &pb.PutAttributeRequest{
					Fileattributes: faList,
				},
			},
		}

		if err := cn.sendRequest(req, nil); err != nil {
			c.connMgr.discardConn(cn)
			return err
		}

		var resp pb.PutAttributeResponse
		if err := cn.recvProto(&resp); err != nil {
			c.connMgr.discardConn(cn)
			return err
		}

		c.connMgr.putConn(cn)

		switch resp.Result {
		case pb.PutAttributeResponse_SUCCESS:
			// ok
		case pb.PutAttributeResponse_AUTH_FAILED:
			return ErrAuthFailed
		default:
			return ErrServerError
		}
	}

	return nil
}

// Close shuts down the client and releases all connections.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	c.disc.close()
	c.connMgr.closeAll()
	return nil
}

// Servers returns the current list of distributed cache servers.
func (c *Client) Servers() []string {
	return c.disc.getServers()
}

// --- Internal helpers ---

func (c *Client) deleteKey(ctx context.Context, cacheKey, server string) error {
	cn, err := c.connMgr.getConn(server)
	if err != nil {
		return err
	}

	if err := cn.setDeadline(c.deadline(ctx)); err != nil {
		c.connMgr.discardConn(cn)
		return err
	}

	req := &pb.Request{
		Payload: &pb.Request_Deleterequest{
			Deleterequest: &pb.DeleteRequest{
				Type: &pb.DeleteRequest_Filename{Filename: cacheKey},
			},
		},
	}

	if err := cn.sendRequest(req, nil); err != nil {
		c.connMgr.discardConn(cn)
		return err
	}

	var resp pb.DeleteResponse
	if err := cn.recvProto(&resp); err != nil {
		c.connMgr.discardConn(cn)
		return err
	}

	c.connMgr.putConn(cn)

	switch resp.Result {
	case pb.DeleteResponse_SUCCESS:
		return nil
	case pb.DeleteResponse_AUTH_FAILED:
		return ErrAuthFailed
	default:
		return ErrServerError
	}
}

func (c *Client) checkClosed() error {
	if c.closed.Load() {
		return ErrClosed
	}
	return nil
}

func (c *Client) deadline(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Now().Add(c.cfg.requestTimeout)
}

func (c *Client) getBuffer() []byte {
	return c.bufPool.Get().([]byte)
}

func (c *Client) putBuffer(buf []byte) {
	if int64(cap(buf)) == c.cfg.chunkSize {
		c.bufPool.Put(buf[:c.cfg.chunkSize])
	}
}
