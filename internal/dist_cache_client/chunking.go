// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"context"
	"fmt"
	"io"
	"sync"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
	"golang.org/x/sync/errgroup"
)

// ChunkError describes a chunk that failed to download with a recoverable miss
// error. The caller can handle each chunk individually (e.g., fetch from origin
// or poll for a locked chunk).
type ChunkError struct {
	Offset int64
	Size   int64
	Err    error
}

// chunkPlan describes a single chunk within a multi-chunk file transfer.
type chunkPlan struct {
	index      int    // chunk ordinal (0-based)
	offset     int64  // byte offset within the file
	size       int64  // chunk size in bytes
	cacheKey   string // SHA256 cache key for this chunk
	serverAddr string // target server for this chunk
}

// planChunks divides a file into aligned chunks and assigns each to a server.
func (c *Client) planChunks(filePath string, fileSize int64) ([]chunkPlan, error) {
	if fileSize <= 0 {
		return nil, nil
	}

	numChunks := (fileSize + c.cfg.chunkSize - 1) / c.cfg.chunkSize
	plans := make([]chunkPlan, 0, numChunks)

	for i := int64(0); i < numChunks; i++ {
		offset := i * c.cfg.chunkSize
		size := c.cfg.chunkSize
		if offset+size > fileSize {
			size = fileSize - offset
		}

		cacheKey := GenerateCacheKey(c.cfg.cachePrefix, filePath, offset, c.cfg.chunkSize)
		server, err := c.disc.getServer(cacheKey)
		if err != nil {
			return nil, err
		}

		plans = append(plans, chunkPlan{
			index:      int(i),
			offset:     offset,
			size:       size,
			cacheKey:   cacheKey,
			serverAddr: server,
		})
	}

	return plans, nil
}

// uploadChunked splits data from a reader into chunks and uploads each to the
// appropriate server in parallel.
func (c *Client) uploadChunked(ctx context.Context, filePath string, r io.Reader, fileSize int64, ucfg *uploadConfig) error {
	plans, err := c.planChunks(filePath, fileSize)
	if err != nil {
		return err
	}

	if len(plans) == 0 {
		return nil
	}

	// For single chunk, skip parallelism overhead
	if len(plans) == 1 {
		data := c.getBuffer()
		defer c.putBuffer(data)

		n, err := io.ReadFull(r, data[:plans[0].size])
		if err != nil && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read chunk 0: %w", err)
		}

		return c.uploadSingleChunk(ctx, plans[0], data[:n], ucfg)
	}

	// Parallel multi-chunk upload
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.cfg.maxParallelOps)

	// We need to read sequentially from r but upload in parallel
	chunkData := make([][]byte, len(plans))
	for i, plan := range plans {
		buf := c.getBuffer()
		n, err := io.ReadFull(r, buf[:plan.size])
		if err != nil && err != io.ErrUnexpectedEOF {
			c.putBuffer(buf)
			// Clean up previously allocated buffers
			for j := 0; j < i; j++ {
				c.putBuffer(chunkData[j][:cap(chunkData[j])])
			}
			return fmt.Errorf("read chunk %d: %w", i, err)
		}
		chunkData[i] = buf[:n]
	}

	for i := range plans {
		plan := plans[i]
		data := chunkData[i]
		g.Go(func() error {
			defer c.putBuffer(data[:cap(data)])
			return c.uploadSingleChunk(ctx, plan, data, ucfg)
		})
	}

	return g.Wait()
}

// uploadSingleChunk uploads one chunk to the designated server.
func (c *Client) uploadSingleChunk(ctx context.Context, plan chunkPlan, data []byte, ucfg *uploadConfig) error {
	cn, err := c.connMgr.getConn(plan.serverAddr)
	if err != nil {
		return err
	}

	if err := cn.setDeadline(c.deadline(ctx)); err != nil {
		c.connMgr.discardConn(cn)
		return err
	}

	req := &pb.Request{
		Payload: &pb.Request_Uploadrequest{
			Uploadrequest: &pb.UploadRequest{
				Filename:      plan.cacheKey,
				Filesize:      uint64(len(data)),
				Ignorelock:    ucfg.ignoreLock,
				Expiryseconds: ucfg.ttlSeconds,
				Metadata:      byteMapToProto(ucfg.metadata),
				Groupid:       ucfg.groupID,
			},
		},
	}

	if err := cn.sendRequest(req, data); err != nil {
		c.connMgr.discardConn(cn)
		return fmt.Errorf("upload chunk %s: %w", plan.cacheKey, err)
	}

	var resp pb.UploadResponse
	if err := cn.recvProto(&resp); err != nil {
		c.connMgr.discardConn(cn)
		return fmt.Errorf("upload response %s: %w", plan.cacheKey, err)
	}

	c.connMgr.putConn(cn)
	return uploadResultToError(resp.Result)
}

// downloadChunked downloads all chunks of a file and writes them in order to w.
// Chunks are downloaded in parallel and streamed to the writer as soon as
// each in-order chunk completes, reducing memory from O(file) to O(parallelism * chunk).
func (c *Client) downloadChunked(ctx context.Context, filePath string, fileSize int64, w io.Writer, dcfg *downloadConfig) (*FileMetadata, error) {
	plans, err := c.planChunks(filePath, fileSize)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		return &FileMetadata{Size: 0}, nil
	}

	// For single chunk, write directly (enables splice)
	if len(plans) == 1 {
		return c.downloadSingleChunkToWriter(ctx, plans[0], w, dcfg)
	}

	// Multi-chunk: download in parallel, stream to writer in order
	type chunkSlot struct {
		data     []byte
		metadata map[string][]byte
		err      error
		done     chan struct{} // closed when chunk download completes
	}

	slots := make([]chunkSlot, len(plans))
	for i := range slots {
		slots[i].done = make(chan struct{})
	}

	// Writer goroutine: writes chunks in order as they complete
	var totalWritten int64
	var firstMeta map[string][]byte
	var writeErr error
	writerDone := make(chan struct{})

	go func() {
		defer close(writerDone)
		for i := range slots {
			<-slots[i].done
			if slots[i].err != nil {
				writeErr = slots[i].err
				return
			}
			if firstMeta == nil && slots[i].metadata != nil {
				firstMeta = slots[i].metadata
			}
			n, err := w.Write(slots[i].data)
			c.putBuffer(slots[i].data[:cap(slots[i].data)])
			slots[i].data = nil
			if err != nil {
				writeErr = fmt.Errorf("write chunk: %w", err)
				return
			}
			totalWritten += int64(n)
		}
	}()

	// Download goroutines with bounded parallelism
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(c.cfg.maxParallelOps)

	for i := range plans {
		plan := plans[i]
		idx := i
		g.Go(func() error {
			buf := c.getBuffer()
			n, meta, err := c.downloadSingleChunkToBuffer(gctx, plan, buf, dcfg)
			if err != nil {
				c.putBuffer(buf)
				slots[idx].err = err
				close(slots[idx].done)
				return err
			}
			slots[idx].data = buf[:n]
			slots[idx].metadata = meta
			close(slots[idx].done)
			return nil
		})
	}

	dlErr := g.Wait()

	// Signal any remaining slots on error so writer doesn't hang
	if dlErr != nil {
		for i := range slots {
			select {
			case <-slots[i].done:
			default:
				slots[i].err = dlErr
				close(slots[i].done)
			}
		}
	}

	<-writerDone

	// Clean up any unreleased buffers
	for i := range slots {
		if slots[i].data != nil {
			c.putBuffer(slots[i].data[:cap(slots[i].data)])
		}
	}

	if dlErr != nil {
		return nil, dlErr
	}
	if writeErr != nil {
		return nil, writeErr
	}

	return &FileMetadata{
		Size:     totalWritten,
		Metadata: firstMeta,
	}, nil
}

// downloadSingleChunkToWriter downloads one chunk directly to a writer.
// Enables splice(2) when w is *os.File.
func (c *Client) downloadSingleChunkToWriter(ctx context.Context, plan chunkPlan, w io.Writer, dcfg *downloadConfig) (*FileMetadata, error) {
	cn, err := c.connMgr.getConn(plan.serverAddr)
	if err != nil {
		return nil, err
	}

	if err := cn.setDeadline(c.deadline(ctx)); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	req := &pb.Request{
		Payload: &pb.Request_Downloadrequest{
			Downloadrequest: &pb.DownloadRequest{
				Filename:   plan.cacheKey,
				Offset:     0,
				Length:     0, // 0 = entire chunk
				Enablelock: dcfg.enableLock,
			},
		},
	}

	if err := cn.sendRequest(req, nil); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	var resp pb.DownloadResponse
	if err := cn.recvProto(&resp); err != nil {
		c.connMgr.discardConn(cn)
		return nil, err
	}

	if err := downloadResultToError(resp.Result); err != nil {
		c.connMgr.putConn(cn)
		return nil, err
	}

	// Read file data using splice-capable path
	n, err := cn.recvDataToWriter(w, int64(resp.Filesize))
	if err != nil {
		c.connMgr.discardConn(cn)
		return nil, fmt.Errorf("read chunk data: %w", err)
	}

	c.connMgr.putConn(cn)
	return &FileMetadata{
		Size:     n,
		Metadata: protoToByteMap(resp.Metadata),
	}, nil
}

// downloadSingleChunkToBuffer downloads one chunk into a buffer.
func (c *Client) downloadSingleChunkToBuffer(ctx context.Context, plan chunkPlan, buf []byte, dcfg *downloadConfig) (int, map[string][]byte, error) {
	cn, err := c.connMgr.getConn(plan.serverAddr)
	if err != nil {
		return 0, nil, err
	}

	if err := cn.setDeadline(c.deadline(ctx)); err != nil {
		c.connMgr.discardConn(cn)
		return 0, nil, err
	}

	req := &pb.Request{
		Payload: &pb.Request_Downloadrequest{
			Downloadrequest: &pb.DownloadRequest{
				Filename:   plan.cacheKey,
				Offset:     0,
				Length:     0,
				Enablelock: dcfg.enableLock,
			},
		},
	}

	if err := cn.sendRequest(req, nil); err != nil {
		c.connMgr.discardConn(cn)
		return 0, nil, err
	}

	var resp pb.DownloadResponse
	if err := cn.recvProto(&resp); err != nil {
		c.connMgr.discardConn(cn)
		return 0, nil, err
	}

	if err := downloadResultToError(resp.Result); err != nil {
		c.connMgr.putConn(cn)
		return 0, nil, err
	}

	dataSize := int(resp.Filesize)
	if dataSize > len(buf) {
		c.connMgr.discardConn(cn)
		return 0, nil, fmt.Errorf("chunk too large: %d > buffer %d", dataSize, len(buf))
	}

	if err := cn.recvDataToBuffer(buf[:dataSize]); err != nil {
		c.connMgr.discardConn(cn)
		return 0, nil, fmt.Errorf("read chunk data: %w", err)
	}

	c.connMgr.putConn(cn)
	return dataSize, protoToByteMap(resp.Metadata), nil
}

// Helper: convert upload response result to error.
func uploadResultToError(r pb.UploadResponse_Result) error {
	switch r {
	case pb.UploadResponse_SUCCESS:
		return nil
	case pb.UploadResponse_FILE_EXISTS:
		return ErrFileExists
	case pb.UploadResponse_AUTH_FAILED:
		return ErrAuthFailed
	case pb.UploadResponse_BAD_REQUEST:
		return ErrBadRequest
	default:
		return ErrServerError
	}
}

// Helper: convert download response result to error.
func downloadResultToError(r pb.DownloadResponse_Result) error {
	switch r {
	case pb.DownloadResponse_SUCCESS:
		return nil
	case pb.DownloadResponse_NOT_FOUND_GOT_LOCK:
		return ErrNotFoundGotLock
	case pb.DownloadResponse_NOT_FOUND_ALREADY_LOCKED:
		return ErrNotFoundAlreadyLocked
	case pb.DownloadResponse_NOT_FOUND:
		return ErrNotFound
	case pb.DownloadResponse_INVALID_OFFSET_LENGTH:
		return ErrInvalidOffset
	case pb.DownloadResponse_FILENAME_LIMIT_EXCEEDED:
		return ErrFilenameTooLong
	case pb.DownloadResponse_AUTH_FAILED:
		return ErrAuthFailed
	default:
		return ErrServerError
	}
}

// Helper: map[string][]byte ↔ protobuf map<string, bytes>
func byteMapToProto(m map[string][]byte) map[string][]byte {
	if m == nil {
		return nil
	}
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func protoToByteMap(m map[string][]byte) map[string][]byte {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// downloadChunkedPartial downloads all chunks of a file but collects per-chunk
// miss errors instead of aborting on the first failure. Successfully downloaded
// chunks are written to w at their correct offsets. Chunks that fail with
// ErrNotFoundGotLock, ErrNotFoundAlreadyLocked, or ErrNotFound are returned as
// ChunkError entries so the caller can handle them individually.
func (c *Client) downloadChunkedPartial(ctx context.Context, filePath string, fileSize int64, w io.WriterAt, dcfg *downloadConfig) ([]ChunkError, error) {
	plans, err := c.planChunks(filePath, fileSize)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		return nil, nil
	}

	var mu sync.Mutex
	var chunkErrors []ChunkError

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(c.cfg.maxParallelOps)

	for i := range plans {
		plan := plans[i]
		g.Go(func() error {
			buf := c.getBuffer()
			n, _, err := c.downloadSingleChunkToBuffer(gctx, plan, buf, dcfg)
			if err != nil {
				c.putBuffer(buf)
				if err == ErrNotFoundGotLock || err == ErrNotFoundAlreadyLocked || err == ErrNotFound {
					mu.Lock()
					chunkErrors = append(chunkErrors, ChunkError{
						Offset: plan.offset,
						Size:   plan.size,
						Err:    err,
					})
					mu.Unlock()
					return nil
				}
				return err
			}
			_, writeErr := w.WriteAt(buf[:n], plan.offset)
			c.putBuffer(buf)
			return writeErr
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return chunkErrors, nil
}
