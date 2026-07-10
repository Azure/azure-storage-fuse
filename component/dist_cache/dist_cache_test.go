// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcache "github.com/nearora-msft/dist-cache-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDCacheClient implements dcacheClient for testing.
type mockDCacheClient struct {
	store             map[string][]byte
	downloadPartialFn func(ctx context.Context, filename string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) ([]dcache.ChunkError, error)
	groups            map[string]map[string]bool // groupID -> set of store keys
	chunkGroupIDs     map[string]string          // storeKey -> groupID (for GetChunkGroupID)
	chunkFn           func(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	uploadFn          func(ctx context.Context, filename string, data io.Reader, size int64, opts ...dcache.UploadOption) error
	uploadChunkFn     func(ctx context.Context, filename string, offset int64, data []byte) error
	uploadChunkCalled int
	deleteGroupCalled int
	lastDeletedGroup  string
}

func newMockDCacheClient() *mockDCacheClient {
	return &mockDCacheClient{
		store:         make(map[string][]byte),
		groups:        make(map[string]map[string]bool),
		chunkGroupIDs: make(map[string]string),
	}
}

func (m *mockDCacheClient) Upload(ctx context.Context, filename, etag string, data io.Reader, size int64, opts ...dcache.UploadOption) error {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, filename, data, size, opts...)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(data, buf); err != nil {
		return err
	}
	m.store[filename] = buf
	return nil
}

func (m *mockDCacheClient) DownloadWithSizePartial(ctx context.Context, filename, etag string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) (<-chan dcache.ChunkError, func() error, error) {
	var chunkErrors []dcache.ChunkError
	var fatalErr error
	if m.downloadPartialFn != nil {
		chunkErrors, fatalErr = m.downloadPartialFn(ctx, filename, fileSize, w, opts...)
	} else {
		data, ok := m.store[filename]
		if !ok {
			chunkErrors = []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrNotFound}}
		} else {
			w.WriteAt(data, 0)
		}
	}

	// Convert slice + error to channel-based API
	ch := make(chan dcache.ChunkError, len(chunkErrors))
	for _, ce := range chunkErrors {
		ch <- ce
	}
	close(ch)

	return ch, func() error { return fatalErr }, nil
}

func (m *mockDCacheClient) DownloadChunk(ctx context.Context, filename, etag string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error) {
	if m.chunkFn != nil {
		return m.chunkFn(ctx, filename, offset, buf, opts...)
	}
	key := fmt.Sprintf("%s:%d", filename, offset)
	data, ok := m.store[key]
	if !ok {
		return 0, dcache.ErrNotFound
	}
	n := copy(buf, data)
	return n, nil
}

func (m *mockDCacheClient) UploadChunk(ctx context.Context, filename, etag string, offset int64, data []byte, _ ...dcache.UploadOption) error {
	if m.uploadChunkFn != nil {
		return m.uploadChunkFn(ctx, filename, offset, data)
	}
	m.uploadChunkCalled++
	key := fmt.Sprintf("%s:%d", filename, offset)
	m.store[key] = append([]byte(nil), data...)
	return nil
}

func (m *mockDCacheClient) Delete(_ context.Context, filename string, _ int64) error {
	delete(m.store, filename)
	return nil
}

func (m *mockDCacheClient) DeleteGroup(_ context.Context, groupID []byte) error {
	m.deleteGroupCalled++
	m.lastDeletedGroup = string(groupID)
	// The versioned group ID has format "filename\x00vN". Extract the filename
	// prefix and remove all store entries that belong to that file.
	gid := string(groupID)
	fileName := gid
	if idx := strings.IndexByte(gid, '\x00'); idx >= 0 {
		fileName = gid[:idx]
	}
	// Track which keys belong to which group. Remove entries registered under
	// this exact group ID.
	if keys, ok := m.groups[gid]; ok {
		for k := range keys {
			delete(m.store, k)
		}
		delete(m.groups, gid)
	} else {
		// Fallback: remove entries whose key starts with the filename
		for k := range m.store {
			if k == fileName || (len(k) > len(fileName) && k[:len(fileName)] == fileName && (k[len(fileName)] == ':' || k[len(fileName)] == '\x00')) {
				delete(m.store, k)
			}
		}
	}
	return nil
}

func (m *mockDCacheClient) GetChunkGroupID(_ context.Context, filename, etag string) ([]byte, error) {
	// Check if a group ID was recorded for chunk 0 of this file
	key := fmt.Sprintf("%s:0", filename)
	if gid, ok := m.chunkGroupIDs[key]; ok {
		return []byte(gid), nil
	}
	return nil, dcache.ErrNotFound
}

func (m *mockDCacheClient) GetAttr(_ context.Context, _ string) (*dcache.FileAttr, error) {
	return nil, dcache.ErrNotFound
}

func (m *mockDCacheClient) PutAttr(_ context.Context, _ []dcache.FileAttrEntry) error {
	return nil
}

func (m *mockDCacheClient) Close() error {
	return nil
}

// mockNextComponent records calls to NextComponent methods.
type mockNextComponent struct {
	internal.BaseComponent
	copyToFileCalled   int
	copyFromFileCalled int
	readInBufferCalled int
	stageDataCalled    int
	commitDataCalled   int
	deleteFileCalled   int
	renameFileCalled   int
	truncateFileCalled int

	copyToFileData   []byte // data written on CopyToFile
	readInBufferData []byte // data returned by ReadInBuffer
	readInBufferFn   func(options *internal.ReadInBufferOptions) (int, error)
	getAttrETag      string // ETag returned by GetAttr (empty = new file)
}

func (m *mockNextComponent) GetAttr(_ internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if m.getAttrETag == "" {
		return &internal.ObjAttr{}, nil
	}
	return &internal.ObjAttr{ETag: m.getAttrETag}, nil
}

func (m *mockNextComponent) CopyToFile(options internal.CopyToFileOptions) error {
	m.copyToFileCalled++
	if m.copyToFileData != nil {
		options.File.Write(m.copyToFileData)
	}
	return nil
}

func (m *mockNextComponent) CopyFromFile(_ internal.CopyFromFileOptions) error {
	m.copyFromFileCalled++
	return nil
}

func (m *mockNextComponent) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	m.readInBufferCalled++
	if m.readInBufferFn != nil {
		return m.readInBufferFn(options)
	}
	if m.readInBufferData != nil {
		n := copy(options.Data, m.readInBufferData)
		return n, nil
	}
	return 0, nil
}

func (m *mockNextComponent) StageData(_ internal.StageDataOptions) error {
	m.stageDataCalled++
	return nil
}

func (m *mockNextComponent) CommitData(_ internal.CommitDataOptions) error {
	m.commitDataCalled++
	return nil
}

func (m *mockNextComponent) DeleteFile(_ internal.DeleteFileOptions) error {
	m.deleteFileCalled++
	return nil
}

func (m *mockNextComponent) RenameFile(_ internal.RenameFileOptions) error {
	m.renameFileCalled++
	return nil
}

func (m *mockNextComponent) TruncateFile(_ internal.TruncateFileOptions) error {
	m.truncateFileCalled++
	return nil
}

func newTestDistCache(mock *mockDCacheClient, next *mockNextComponent) *DistCache {
	dc := &DistCache{
		client:            mock,
		chunkSize:         16 * 1024 * 1024,
		bypassOnError:     true,
		dirtyFiles:        make(map[string]time.Time),
		pendingWrites:     make(map[string]*pendingFile),
		flushCancel:       make(map[string]context.CancelFunc),
		readUploadCancels: make(map[string]*readUploadEntry),
		stopCleanup:       make(chan struct{}),
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)
	return dc
}

// --- Tests ---

func TestCopyToFile_L2Hit(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Pre-populate mock cache
	testData := []byte("cached data from distributed cache")
	mock.downloadPartialFn = func(_ context.Context, _ string, _ int64, w io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		w.WriteAt(testData, 0)
		return nil, nil
	}

	f, err := os.CreateTemp("", "dcache-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: int64(len(testData)),
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, next.copyToFileCalled, "should NOT call azstorage on L2 hit")
}

func TestCopyToFile_L2MissGotLock(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("data from azure")}
	dc := newTestDistCache(mock, next)

	// Simulate L2 miss with lock acquired
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrNotFoundGotLock}}, nil
	}

	f, err := os.CreateTemp("", "dcache-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: 15,
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.readInBufferCalled, "should call ReadInBuffer for failed chunk")
	assert.Equal(t, 0, next.copyToFileCalled, "should NOT call CopyToFile for the entire file")
}

func TestCopyToFile_BypassOnError(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{copyToFileData: []byte("fallback data")}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = true

	// Simulate connection error
	mock.downloadPartialFn = func(_ context.Context, _ string, _ int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return nil, dcache.ErrConnectionFailed
	}

	f, err := os.CreateTemp("", "dcache-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: 13,
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyToFileCalled, "should bypass to azstorage on error")
}

func TestCopyToFile_NilClient(t *testing.T) {
	next := &mockNextComponent{copyToFileData: []byte("data")}
	dc := &DistCache{bypassOnError: true, dirtyFiles: make(map[string]time.Time)}
	dc.SetName(compName)
	dc.SetNextComponent(next)

	f, err := os.CreateTemp("", "dcache-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: 4,
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyToFileCalled, "should pass through when client is nil")
}

func TestCopyFromFile_WriteThrough(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	f, err := os.CreateTemp("", "dcache-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("data to upload")
	f.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/file.txt",
		File: f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyFromFileCalled, "should write-through to azstorage")
}

func TestReadInBuffer_L2Hit(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	chunkData := []byte("block data from cache")
	mock.chunkFn = func(_ context.Context, _ string, _ int64, buf []byte, _ ...dcache.DownloadOption) (int, error) {
		n := copy(buf, chunkData)
		return n, nil
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(chunkData), n)
	assert.Equal(t, chunkData, buf[:n])
	assert.Equal(t, 0, next.readInBufferCalled, "should NOT call azstorage on L2 hit")
}

func TestReadInBuffer_L2ZeroByteHit_FallsThrough(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("data from azure storage")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// Simulate a cache entry with 0 bytes (corrupt/empty)
	mock.chunkFn = func(_ context.Context, _ string, _ int64, buf []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, nil
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should fall through to azstorage on zero-byte L2 hit")
}

func TestReadInBuffer_L2Miss(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("data from azure storage")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should call azstorage on L2 miss")
}

func TestReadInBuffer_GotLock_DownloadsFromAzure(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure block data")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// DownloadChunk returns ErrNotFoundGotLock
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 4096,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should call ReadInBuffer on Azure for the chunk")
}

func TestReadInBuffer_AlreadyLocked_PollSucceeds(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	cachedData := []byte("block arrived after poll")

	// First DownloadChunk: ErrNotFoundAlreadyLocked (triggers ReadInBuffer → poll)
	// Poll calls DownloadChunk again: first locked, then succeeds
	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, buf []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount <= 2 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return copy(buf, cachedData), nil
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(cachedData), n)
	assert.Equal(t, cachedData, buf[:n])
	assert.Equal(t, 0, next.readInBufferCalled, "should serve from cache after poll, not Azure")
}

func TestReadInBuffer_AlreadyLocked_PollTimeout_FallsThrough(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure fallback data")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// DownloadChunk always returns locked
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundAlreadyLocked
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should fall through to Azure after poll timeout")
}

func TestStageData_WriteThrough(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	err := dc.StageData(internal.StageDataOptions{
		Name:   "test/file.bin",
		Offset: 0,
		Data:   []byte("block data"),
		Id:     "block-0",
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.stageDataCalled, "should forward to azstorage")
}

func TestStageData_BuffersPendingChunks(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Stage two chunks for the same file
	err := dc.StageData(internal.StageDataOptions{
		Name:   "test/file.bin",
		Offset: 0,
		Data:   []byte("chunk-0-data"),
		Id:     "block-0",
	})
	assert.NoError(t, err)

	err = dc.StageData(internal.StageDataOptions{
		Name:   "test/file.bin",
		Offset: 1024,
		Data:   []byte("chunk-1-data"),
		Id:     "block-1",
	})
	assert.NoError(t, err)

	// Verify chunks are buffered in pendingWrites
	dc.pendingMu.Lock()
	pf := dc.pendingWrites["test/file.bin"]
	dc.pendingMu.Unlock()

	require.NotNil(t, pf, "should have a pendingFile entry")
	assert.Equal(t, 2, len(pf.chunks), "should buffer both chunks")
	assert.Equal(t, int64(0), pf.chunks[0].offset)
	assert.Equal(t, []byte("chunk-0-data"), pf.chunks[0].data)
	assert.Equal(t, int64(1024), pf.chunks[1].offset)
	assert.Equal(t, []byte("chunk-1-data"), pf.chunks[1].data)
	assert.Equal(t, int64(24), pf.totalSize, "totalSize should track cumulative data")

	// Verify no L2 upload happened yet
	assert.Equal(t, 0, mock.uploadChunkCalled, "should not upload to L2 during stage")
}

func TestStageData_SizeCapEvictsPending(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)
	dc.conf.MaxFileSizeMB = 1 // 1MB cap

	// Pre-populate L2 with old chunks (simulating previously cached data)
	mock.store["test/big.bin:0"] = make([]byte, 512*1024)
	mock.store["test/big.bin:524288"] = make([]byte, 512*1024)

	// Stage a chunk that's under the cap
	chunk := make([]byte, 512*1024) // 512KB
	err := dc.StageData(internal.StageDataOptions{
		Name: "test/big.bin", Offset: 0, Data: chunk, Id: "b0",
	})
	assert.NoError(t, err)

	dc.pendingMu.Lock()
	assert.NotNil(t, dc.pendingWrites["test/big.bin"], "should buffer chunk under cap")
	dc.pendingMu.Unlock()

	// Stage another chunk that pushes it over the 1MB cap
	chunk2 := make([]byte, 512*1024+1) // 512KB + 1 byte, total exceeds 1MB
	err = dc.StageData(internal.StageDataOptions{
		Name: "test/big.bin", Offset: 512 * 1024, Data: chunk2, Id: "b1",
	})
	assert.NoError(t, err)

	// Pending should be evicted (over cap)
	dc.pendingMu.Lock()
	_, exists := dc.pendingWrites["test/big.bin"]
	dc.pendingMu.Unlock()
	assert.False(t, exists, "should evict pending when size cap exceeded")

	// L2 should NOT be invalidated — the committed state hasn't changed,
	// so existing L2 data is still valid. CommitData handles invalidation.
	assert.Equal(t, 0, mock.deleteGroupCalled, "should not invalidate L2 on size cap (committed state unchanged)")
	_, exists = mock.store["test/big.bin:0"]
	assert.True(t, exists, "L2 chunk should remain (still valid)")
	_, exists = mock.store["test/big.bin:524288"]
	assert.True(t, exists, "L2 chunk should remain (still valid)")
}

func TestEvictStalePending(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Manually insert a stale pending entry
	dc.pendingMu.Lock()
	dc.pendingWrites["stale/file.bin"] = &pendingFile{
		chunks:       []pendingChunk{{offset: 0, data: []byte("old")}},
		totalSize:    3,
		lastActivity: time.Now().Add(-10 * time.Minute), // well past TTL
	}
	dc.pendingWrites["fresh/file.bin"] = &pendingFile{
		chunks:       []pendingChunk{{offset: 0, data: []byte("new")}},
		totalSize:    3,
		lastActivity: time.Now(), // fresh
	}
	dc.pendingMu.Unlock()

	// Run eviction
	dc.evictStalePending()

	dc.pendingMu.Lock()
	_, staleExists := dc.pendingWrites["stale/file.bin"]
	_, freshExists := dc.pendingWrites["fresh/file.bin"]
	dc.pendingMu.Unlock()

	assert.False(t, staleExists, "stale entry should be evicted")
	assert.True(t, freshExists, "fresh entry should remain")
}

func TestCommitData_ForwardOnly(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{getAttrETag: "existingetag"}
	dc := newTestDistCache(mock, next)

	err := dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"block-0", "block-1"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.commitDataCalled)
	// CommitData should always invalidate old L2 entries
	assert.Equal(t, 1, mock.deleteGroupCalled)
	assert.Equal(t, "test/file.bin\x00vexistingetag", mock.lastDeletedGroup)
}

func TestCommitData_FlushesPendingToL2(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{getAttrETag: "oldetag"}
	dc := newTestDistCache(mock, next)

	// Pre-populate L2 with old chunks (simulating a previously cached file)
	mock.store["test/file.bin:0"] = []byte("old-chunk-0")
	mock.store["test/file.bin:4096"] = []byte("old-chunk-1")
	mock.store["test/file.bin:8192"] = []byte("old-chunk-2")

	// Stage chunks first
	_ = dc.StageData(internal.StageDataOptions{
		Name: "test/file.bin", Offset: 0, Data: []byte("chunk-0"), Id: "b0",
	})
	_ = dc.StageData(internal.StageDataOptions{
		Name: "test/file.bin", Offset: 4096, Data: []byte("chunk-1"), Id: "b1",
	})

	// Verify chunks are pending
	dc.pendingMu.Lock()
	pf := dc.pendingWrites["test/file.bin"]
	require.NotNil(t, pf)
	assert.Equal(t, 2, len(pf.chunks))
	dc.pendingMu.Unlock()

	// Commit
	err := dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"b0", "b1"},
	})
	assert.NoError(t, err)

	// pendingWrites should be drained immediately
	dc.pendingMu.Lock()
	_, exists := dc.pendingWrites["test/file.bin"]
	dc.pendingMu.Unlock()
	assert.False(t, exists, "pending chunks should be drained after commit")

	// DeleteGroup should have been called to invalidate old L2 data
	assert.Equal(t, 1, mock.deleteGroupCalled, "should invalidate old L2 before flushing new data")
	assert.Equal(t, "test/file.bin\x00voldetag", mock.lastDeletedGroup)

	// Old chunk beyond new file extent should be gone
	_, exists = mock.store["test/file.bin:8192"]
	assert.False(t, exists, "old L2 chunk beyond new extent should be deleted")

	// Wait briefly for the async flush goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// Verify new chunks were uploaded to L2 (after DeleteGroup cleared old ones)
	assert.Equal(t, 2, mock.uploadChunkCalled, "should flush both chunks to L2")
	assert.Equal(t, []byte("chunk-0"), mock.store["test/file.bin:0"])
	assert.Equal(t, []byte("chunk-1"), mock.store["test/file.bin:4096"])
}

func TestCommitData_CancelsPreviousFlush(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Gate that blocks UploadChunk until we release it or the context is cancelled
	gate := make(chan struct{})
	var cancelledChunks int32
	var uploadedChunks int32

	mock.uploadChunkFn = func(ctx context.Context, filename string, offset int64, data []byte) error {
		select {
		case <-ctx.Done():
			atomic.AddInt32(&cancelledChunks, 1)
			return ctx.Err()
		case <-gate:
			atomic.AddInt32(&uploadedChunks, 1)
			key := fmt.Sprintf("%s:%d", filename, offset)
			mock.store[key] = append([]byte(nil), data...)
			return nil
		}
	}

	// Stage chunks for commit #1 (large file)
	for i := 0; i < 4; i++ {
		_ = dc.StageData(internal.StageDataOptions{
			Name: "test/file.bin", Offset: uint64(i * 4096), Data: []byte(fmt.Sprintf("old-chunk-%d", i)), Id: fmt.Sprintf("b%d", i),
		})
	}

	// Commit #1: starts an async flush that will block on the gate
	err := dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"b0", "b1", "b2", "b3"},
	})
	assert.NoError(t, err)

	// Give the flush goroutine time to start and block
	time.Sleep(50 * time.Millisecond)

	// Stage chunks for commit #2 (smaller file rewrite via O_TRUNC)
	_ = dc.StageData(internal.StageDataOptions{
		Name: "test/file.bin", Offset: 0, Data: []byte("new-chunk-0"), Id: "c0",
	})

	// Commit #2: should cancel flush #1 before doing DeleteGroup + its own flush
	err = dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"c0"},
	})
	assert.NoError(t, err)

	// Now unblock the gate so flush #2 can proceed
	close(gate)

	// Wait for flush #2 to complete
	time.Sleep(100 * time.Millisecond)

	// Flush #1's chunks should have been cancelled (not uploaded)
	assert.True(t, atomic.LoadInt32(&cancelledChunks) > 0, "flush #1 should have been cancelled")

	// Flush #2's new chunk should have been uploaded
	assert.Equal(t, []byte("new-chunk-0"), mock.store["test/file.bin:0"])

	// Old chunks from flush #1 should NOT be in L2
	_, exists := mock.store["test/file.bin:4096"]
	assert.False(t, exists, "old chunk from cancelled flush should not be in L2")
	_, exists = mock.store["test/file.bin:8192"]
	assert.False(t, exists, "old chunk from cancelled flush should not be in L2")
}

func TestDeleteFile_Invalidation(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{getAttrETag: "deleteetag"}
	dc := newTestDistCache(mock, next)

	mock.store["test/file.txt"] = []byte("cached data")

	err := dc.DeleteFile(internal.DeleteFileOptions{Name: "test/file.txt"})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.deleteFileCalled)
	_, exists := mock.store["test/file.txt"]
	assert.False(t, exists, "should invalidate cache entry")
}

func TestRenameFile_Invalidation(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{getAttrETag: "renameetag"}
	dc := newTestDistCache(mock, next)

	mock.store["old-name.txt"] = []byte("data")

	err := dc.RenameFile(internal.RenameFileOptions{Src: "old-name.txt", Dst: "new-name.txt"})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.renameFileCalled)
	_, exists := mock.store["old-name.txt"]
	assert.False(t, exists, "should invalidate old cache entry")
}

func TestTruncateFile_Invalidation(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	err := dc.TruncateFile(internal.TruncateFileOptions{
		Name:    "test/file.txt",
		OldSize: 1024,
		NewSize: 512,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.truncateFileCalled)
}

func TestReadInBuffer_BypassesDirtyFile(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("fresh-data")}
	dc := newTestDistCache(mock, next)

	// Populate cache with stale data
	mock.store["test/file.txt:0"] = []byte("stale-data")

	// Truncate marks the file as dirty
	err := dc.TruncateFile(internal.TruncateFileOptions{
		Name:    "test/file.txt",
		OldSize: 1024,
		NewSize: 512,
	})
	require.NoError(t, err)

	// Read should bypass dist_cache and go to azstorage
	buf := make([]byte, 64)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.txt",
		Offset: 0,
		Data:   buf,
	})
	assert.NoError(t, err)
	assert.Equal(t, len("fresh-data"), n)
	assert.Equal(t, "fresh-data", string(buf[:n]))
	assert.Equal(t, 1, next.readInBufferCalled, "should bypass dist_cache for dirty file")
}

func TestCopyToFile_BypassesDirtyFile(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{copyToFileData: []byte("fresh-data")}
	dc := newTestDistCache(mock, next)

	// Populate cache with stale data
	mock.store["test/file.txt"] = []byte("stale-data")

	// Delete marks the file as dirty
	err := dc.DeleteFile(internal.DeleteFileOptions{Name: "test/file.txt"})
	require.NoError(t, err)

	f, err := os.CreateTemp("", "dcache-dirty-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	// Read should bypass dist_cache
	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		File:  f,
		Count: 10,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyToFileCalled, "should bypass dist_cache for dirty file")
}

func TestPriority(t *testing.T) {
	dc := &DistCache{dirtyFiles: make(map[string]time.Time)}
	assert.Equal(t, internal.EComponentPriority.LevelMid(), dc.Priority())
}

func TestPollUntilCached_SucceedsOnRetry(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("azure data")}
	dc := newTestDistCache(mock, next)

	testData := []byte("cached after retry")

	// DownloadWithSizePartial: all chunks miss (locked by another node).
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrNotFoundAlreadyLocked}}, nil
	}

	// First DownloadChunk call (from pollUntilChunkCached): still locked.
	// Second call: data is available.
	chunkCallCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, buf []byte, _ ...dcache.DownloadOption) (int, error) {
		chunkCallCount++
		if chunkCallCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		n := copy(buf, testData)
		return n, nil
	}

	f, err := os.CreateTemp("", "dcache-poll-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/poll-retry.txt",
		Count: int64(len(testData)),
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, next.copyToFileCalled, "should serve from cache after poll retry")
}

func TestCopyToFile_ChunkLevelGotLock_MultipleChunks(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	chunkA := []byte("chunk-A-data-here!")
	chunkB := []byte("chunk-B-data-here!")

	// Return correct data for each offset via ReadInBuffer
	next.readInBufferFn = func(options *internal.ReadInBufferOptions) (int, error) {
		if options.Offset == 0 {
			return copy(options.Data, chunkA), nil
		}
		return copy(options.Data, chunkB), nil
	}

	// Chunk 0 is cached, chunk 1 misses with GotLock
	mock.downloadPartialFn = func(_ context.Context, _ string, _ int64, w io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		w.WriteAt(chunkA, 0) // chunk 0 succeeds
		return []dcache.ChunkError{{Offset: int64(len(chunkA)), Size: int64(len(chunkB)), Err: dcache.ErrNotFoundGotLock}}, nil
	}

	f, err := os.CreateTemp("", "dcache-chunk-gotlock-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/multi-chunk.txt",
		Count: int64(len(chunkA) + len(chunkB)),
		File:  f,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, next.readInBufferCalled, "should only call ReadInBuffer for the missing chunk")

	// Verify file contents: chunk A from cache, chunk B from Azure
	f.Seek(0, 0)
	got := make([]byte, len(chunkA)+len(chunkB))
	n, _ := f.Read(got)
	assert.Equal(t, len(chunkA)+len(chunkB), n)
	assert.Equal(t, append(chunkA, chunkB...), got)
}

func TestCopyToFile_ChunkPollTimeout_FallsThrough(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("azure-fallback")}
	dc := newTestDistCache(mock, next)

	// DownloadWithSizePartial: chunk locked by another node
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrNotFoundAlreadyLocked}}, nil
	}

	// DownloadChunk always returns locked (simulates poll timeout)
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundAlreadyLocked
	}

	f, err := os.CreateTemp("", "dcache-chunk-timeout-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/chunk-timeout.txt",
		Count: int64(len("azure-fallback")),
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.readInBufferCalled, "should fall through to ReadInBuffer after poll timeout")
	assert.Equal(t, 0, next.copyToFileCalled, "should NOT call CopyToFile for the entire file")
}

func TestCopyFromFile_InvalidatesL2(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	next.getAttrETag = "oldetag123"
	dc := newTestDistCache(mock, next)

	// Pre-populate L2 with old data (simulating stale cached file)
	mock.store["test/file.txt"] = []byte("old cached content")
	mock.store["test/file.txt:0"] = []byte("old chunk 0")
	mock.store["test/file.txt:16777216"] = []byte("old chunk 1")

	f, err := os.CreateTemp("", "dcache-invalidate-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("new file content")
	f.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/file.txt",
		File: f,
	})

	expectedGroup := "test/file.txt\x00voldetag123"
	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyFromFileCalled, "should write-through to azstorage")
	assert.Equal(t, 1, mock.deleteGroupCalled, "should invalidate old L2 entry")
	assert.Equal(t, expectedGroup, mock.lastDeletedGroup, "should delete the correct group")

	// Verify old chunks were removed
	_, exists := mock.store["test/file.txt"]
	assert.False(t, exists, "old whole-file entry should be deleted")
	_, exists = mock.store["test/file.txt:0"]
	assert.False(t, exists, "old chunk 0 should be deleted")
	_, exists = mock.store["test/file.txt:16777216"]
	assert.False(t, exists, "old chunk 1 should be deleted")
}

func TestCopyFromFile_MarksDirty(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	f, err := os.CreateTemp("", "dcache-dirty-write-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("content")
	f.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/dirty-write.txt",
		File: f,
	})

	assert.NoError(t, err)
	assert.True(t, dc.isDirty("test/dirty-write.txt"), "file should be marked dirty after CopyFromFile")
}

func TestCopyFromFile_DirtyPreventsStaleRead(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{copyToFileData: []byte("fresh from azure")}
	dc := newTestDistCache(mock, next)

	// Block the async populateCache so clearDirty doesn't fire before our read
	uploadStarted := make(chan struct{})
	uploadRelease := make(chan struct{})
	mock.uploadFn = func(_ context.Context, _ string, _ io.Reader, _ int64, _ ...dcache.UploadOption) error {
		close(uploadStarted)
		<-uploadRelease
		return nil
	}

	// Pre-populate L2 with stale data (for the download path).
	// CopyToFile uses DownloadWithSizePartial, so intercept that.
	staleData := []byte("stale cached version")
	mock.downloadPartialFn = func(_ context.Context, _ string, _ int64, w io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		w.WriteAt(staleData, 0)
		return nil, nil
	}

	// Simulate a write via CopyFromFile
	wf, err := os.CreateTemp("", "dcache-write-*")
	require.NoError(t, err)
	defer os.Remove(wf.Name())
	wf.WriteString("new content")
	wf.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/read-after-write.txt",
		File: wf,
	})
	require.NoError(t, err)

	// Wait for the upload goroutine to start (ensures populateCache is in-flight)
	<-uploadStarted

	// Now a read on the same node should bypass L2 (dirty) and go to azstorage
	rf, err := os.CreateTemp("", "dcache-read-*")
	require.NoError(t, err)
	defer os.Remove(rf.Name())
	defer rf.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/read-after-write.txt",
		Count: 16,
		File:  rf,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyToFileCalled, "should bypass L2 and go to azstorage for dirty file")

	// Release the upload goroutine and verify dirty is cleared after populate
	close(uploadRelease)
	time.Sleep(50 * time.Millisecond)
	assert.False(t, dc.isDirty("test/read-after-write.txt"), "dirty flag should be cleared after successful L2 populate")
}

func TestCopyFromFile_NilClientPassesThrough(t *testing.T) {
	next := &mockNextComponent{}
	dc := &DistCache{bypassOnError: true, dirtyFiles: make(map[string]time.Time)}
	dc.SetName(compName)
	dc.SetNextComponent(next)

	f, err := os.CreateTemp("", "dcache-nil-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.WriteString("data")
	f.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/nil-client.txt",
		File: f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.copyFromFileCalled)
	assert.False(t, dc.isDirty("test/nil-client.txt"), "should not mark dirty when client is nil")
}

func TestCommitData_DeletesOldETagGroup(t *testing.T) {
	// Simulate: a file exists with ETag-A in Azure. A new commit produces ETag-B.
	// CommitData should GetAttr to find ETag-A, commit, then DeleteGroup(ETag-A).
	mock := newMockDCacheClient()
	next := &mockNextComponent{getAttrETag: "0x8DC3A2B1C4E5F6A7"}
	dc := newTestDistCache(mock, next)

	// Simulate pre-existing chunks cached under old ETag
	oldGroupID := "test/file.bin\x00v0x8DC3A2B1C4E5F6A7"
	mock.store["test/file.bin:0"] = []byte("old-chunk-0")
	mock.store["test/file.bin:4096"] = []byte("old-chunk-1")
	mock.groups[oldGroupID] = map[string]bool{
		"test/file.bin:0":    true,
		"test/file.bin:4096": true,
	}

	// Stage and commit new data (simulating block_cache providing new ETag)
	newETag := "0x8DC4B3C2D5F607B8"
	err := dc.StageData(internal.StageDataOptions{
		Name: "test/file.bin", Offset: 0, Data: []byte("new-data"), Id: "b0",
	})
	require.NoError(t, err)

	err = dc.CommitData(internal.CommitDataOptions{
		Name:    "test/file.bin",
		List:    []string{"b0"},
		NewETag: &newETag,
	})
	require.NoError(t, err)

	// Should have deleted the old ETag group
	assert.Equal(t, 1, mock.deleteGroupCalled)
	assert.Equal(t, oldGroupID, mock.lastDeletedGroup, "should delete old ETag group")

	// Old chunks should be gone
	_, exists := mock.store["test/file.bin:0"]
	assert.False(t, exists, "old chunk 0 should be deleted via old ETag group")
	_, exists = mock.store["test/file.bin:4096"]
	assert.False(t, exists, "old chunk 1 should be deleted via old ETag group")
}

func TestReadUploadCancelled_OnCommitData(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Gate that blocks UploadChunk until context is cancelled or released
	uploadStarted := make(chan struct{})
	var uploadCancelled int32
	var startedOnce int32

	mock.uploadChunkFn = func(ctx context.Context, filename string, offset int64, data []byte) error {
		// Only track the first call (from the read-path uploadChunkAsync)
		if atomic.CompareAndSwapInt32(&startedOnce, 0, 1) {
			close(uploadStarted)
			<-ctx.Done()
			atomic.AddInt32(&uploadCancelled, 1)
			return ctx.Err()
		}
		// Subsequent calls (from flush) just succeed
		key := fmt.Sprintf("%s:%d", filename, offset)
		mock.store[key] = append([]byte(nil), data...)
		return nil
	}

	// Simulate a read-path cache miss that triggers uploadChunkAsync
	azData := []byte("data from azure")
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}
	next.readInBufferData = azData

	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})
	require.NoError(t, err)

	// Wait for the uploadChunkAsync goroutine to start and block
	<-uploadStarted

	// Now commit new data — this should cancel the in-flight read upload
	_ = dc.StageData(internal.StageDataOptions{
		Name: "test/file.bin", Offset: 0, Data: []byte("new-data"), Id: "b0",
	})
	err = dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"b0"},
	})
	assert.NoError(t, err)

	// Wait briefly for the cancellation to propagate
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&uploadCancelled),
		"read-path uploadChunkAsync should be cancelled when CommitData arrives")
}

func TestReadUploadCancelled_OnCopyFromFile(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Gate that blocks UploadChunk until context is cancelled
	uploadStarted := make(chan struct{})
	var uploadCancelled int32

	mock.uploadChunkFn = func(ctx context.Context, filename string, offset int64, data []byte) error {
		close(uploadStarted)
		<-ctx.Done()
		atomic.AddInt32(&uploadCancelled, 1)
		return ctx.Err()
	}

	// Simulate a read-path cache miss that triggers uploadChunkAsync
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFound
	}
	next.readInBufferData = []byte("old file content")

	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})
	require.NoError(t, err)

	// Wait for the uploadChunkAsync goroutine to start and block
	<-uploadStarted

	// Now write via CopyFromFile — should cancel the in-flight read upload
	// Block populateCache so it doesn't interfere
	mock.uploadFn = func(ctx context.Context, _ string, _ io.Reader, _ int64, _ ...dcache.UploadOption) error {
		<-ctx.Done()
		return ctx.Err()
	}

	wf, err := os.CreateTemp("", "dcache-cancel-*")
	require.NoError(t, err)
	defer os.Remove(wf.Name())
	wf.WriteString("new content")
	wf.Seek(0, 0)

	err = dc.CopyFromFile(internal.CopyFromFileOptions{
		Name: "test/file.bin",
		File: wf,
	})
	assert.NoError(t, err)

	// Wait briefly for the cancellation to propagate
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&uploadCancelled),
		"read-path uploadChunkAsync should be cancelled when CopyFromFile arrives")
}

func TestReadUploadCancelled_OnDeleteFile(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	uploadStarted := make(chan struct{})
	var uploadCancelled int32

	mock.uploadChunkFn = func(ctx context.Context, _ string, _ int64, _ []byte) error {
		close(uploadStarted)
		<-ctx.Done()
		atomic.AddInt32(&uploadCancelled, 1)
		return ctx.Err()
	}

	// Trigger a read-path upload
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}
	next.readInBufferData = []byte("file data")

	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})
	require.NoError(t, err)

	<-uploadStarted

	// Delete the file — should cancel read uploads
	err = dc.DeleteFile(internal.DeleteFileOptions{Name: "test/file.bin"})
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&uploadCancelled),
		"read-path uploadChunkAsync should be cancelled when DeleteFile arrives")
}

func TestReadUploadNotCancelled_ForDifferentFile(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	uploadStarted := make(chan struct{})
	uploadDone := make(chan struct{})
	var uploadCompleted int32
	var startedOnce int32

	mock.uploadChunkFn = func(ctx context.Context, filename string, offset int64, data []byte) error {
		// Only block on the first call (from read-path uploadChunkAsync for file-a)
		if atomic.CompareAndSwapInt32(&startedOnce, 0, 1) {
			close(uploadStarted)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-uploadDone:
				key := fmt.Sprintf("%s:%d", filename, offset)
				mock.store[key] = append([]byte(nil), data...)
				atomic.AddInt32(&uploadCompleted, 1)
				return nil
			}
		}
		// Subsequent calls (from flush for file-b) just succeed
		key := fmt.Sprintf("%s:%d", filename, offset)
		mock.store[key] = append([]byte(nil), data...)
		return nil
	}

	// Trigger read-path upload for file A
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}
	next.readInBufferData = []byte("file-a-data")

	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file-a.bin",
		Offset: 0,
		Data:   buf,
	})
	require.NoError(t, err)

	<-uploadStarted

	// Write to a DIFFERENT file — should NOT cancel file-a's read upload
	_ = dc.StageData(internal.StageDataOptions{
		Name: "test/file-b.bin", Offset: 0, Data: []byte("data-b"), Id: "b0",
	})
	_ = dc.CommitData(internal.CommitDataOptions{
		Name: "test/file-b.bin",
		List: []string{"b0"},
	})

	// Unblock the upload for file A
	close(uploadDone)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&uploadCompleted),
		"read-path upload for file-a should NOT be cancelled by write to file-b")
	assert.Equal(t, []byte("file-a-data"), mock.store["test/file-a.bin:0"])
}

// --- Tests for recoverable network error handling ---

func TestCopyToFile_RecoverableNetErr_FetchesFromStorage(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("data from azure after net error")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// Simulate a recoverable network error (connection failed) on one chunk
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrConnectionFailed}}, nil
	}

	f, err := os.CreateTemp("", "dcache-neterr-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: int64(len(azData)),
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.readInBufferCalled, "should fetch from Azure on recoverable network error")
	assert.Equal(t, 0, next.copyToFileCalled, "should NOT fall back to full CopyToFile")
}

func TestCopyToFile_RecoverableNetErr_MultipleChunks(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	chunkSize := int64(16 * 1024 * 1024)

	// Return data based on offset in ReadInBuffer
	next.readInBufferFn = func(options *internal.ReadInBufferOptions) (int, error) {
		data := fmt.Sprintf("chunk-at-%d", options.Offset)
		n := copy(options.Data, data)
		return n, nil
	}

	// Simulate multiple chunks: one hit, two recoverable network errors
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, w io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		// First chunk is a hit
		w.WriteAt([]byte("cached-chunk-0"), 0)
		// Second and third chunks have network errors
		return []dcache.ChunkError{
			{Offset: chunkSize, Size: chunkSize, Err: dcache.ErrConnectionFailed},
			{Offset: 2 * chunkSize, Size: chunkSize, Err: io.EOF},
		}, nil
	}

	f, err := os.CreateTemp("", "dcache-neterr-multi-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/largefile.bin",
		Count: 3 * chunkSize,
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, next.readInBufferCalled, "should fetch both errored chunks from Azure")
	assert.Equal(t, 0, next.copyToFileCalled, "should NOT fall back to full CopyToFile")
}

func TestCopyToFile_RecoverableNetErr_DoesNotPopulateCache(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("data from azure")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// Simulate a recoverable network error — should fetch from storage
	// but NOT re-populate cache (populateCache=false for this path)
	mock.downloadPartialFn = func(_ context.Context, _ string, fileSize int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrConnectionFailed}}, nil
	}

	f, err := os.CreateTemp("", "dcache-neterr-nopop-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/file.txt",
		Count: int64(len(azData)),
		File:  f,
	})

	assert.NoError(t, err)
	// Wait briefly for any async uploads
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, mock.uploadChunkCalled, "should NOT populate cache on recoverable net error (no lock held)")
}

func TestCopyToFile_RecoverableNetErr_MixedWithMisses(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	chunkSize := int64(16 * 1024 * 1024)

	next.readInBufferFn = func(options *internal.ReadInBufferOptions) (int, error) {
		data := fmt.Sprintf("data-at-%d", options.Offset)
		n := copy(options.Data, data)
		return n, nil
	}

	// Mix of: cache miss with lock, network error, and plain miss
	mock.downloadPartialFn = func(_ context.Context, _ string, _ int64, _ io.WriterAt, _ ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
		return []dcache.ChunkError{
			{Offset: 0, Size: chunkSize, Err: dcache.ErrNotFoundGotLock},
			{Offset: chunkSize, Size: chunkSize, Err: dcache.ErrConnectionFailed},
			{Offset: 2 * chunkSize, Size: chunkSize, Err: dcache.ErrNotFound},
		}, nil
	}

	f, err := os.CreateTemp("", "dcache-neterr-mixed-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = dc.CopyToFile(internal.CopyToFileOptions{
		Name:  "test/largefile.bin",
		Count: 3 * chunkSize,
		File:  f,
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, next.readInBufferCalled, "all three chunks should be fetched from Azure")

	// Wait for async uploads from the GotLock path
	time.Sleep(50 * time.Millisecond)
	// Only the GotLock chunk should trigger a cache populate
	assert.Equal(t, 1, mock.uploadChunkCalled, "only GotLock chunk should populate cache")
}

func TestReadInBuffer_RecoverableNetErr_BypassesToStorage(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure data after net error")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// DownloadChunk returns a recoverable network error
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrConnectionFailed
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should bypass to Azure on recoverable network error")
}

// --- Configure() tests: cache prefix auto-derivation from azstorage config ---

// loadConfig resets viper state and loads the given YAML into config for a test.
func loadConfig(t *testing.T, yaml string) {
	t.Helper()
	config.ResetConfig()
	err := config.ReadConfigFromReader(strings.NewReader(yaml))
	require.NoError(t, err)
}

func TestConfigure_DerivesCachePrefixFromAzStorage(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
dist_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err)
	assert.Equal(t, "myacct/mycontainer", dc.cachePrefix)
}

func TestConfigure_FailsWhenAccountNameMissing(t *testing.T) {
	loadConfig(t, `
azstorage:
  container: mycontainer
dist_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "azstorage.account-name")
	assert.Contains(t, err.Error(), "azstorage.container")
}

func TestConfigure_FailsWhenContainerMissing(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
dist_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "azstorage.container")
}

func TestConfigure_FailsWhenBothMissing(t *testing.T) {
	loadConfig(t, `
dist_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache prefix")
}

func TestConfigure_FailsWhenAccountNameEmptyString(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: ""
  container: mycontainer
dist_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache prefix")
}

func TestConfigure_CachePrefixIsolatesTenants(t *testing.T) {
	// Two configs with the same filePath in different containers must produce
	// distinct cache prefixes, preventing key collisions on a shared cluster.
	loadConfig(t, `
azstorage:
  account-name: tenantA
  container: shared
dist_cache:
  server-list: "localhost:9065"
`)
	dcA := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dcA.Configure(true))

	loadConfig(t, `
azstorage:
  account-name: tenantB
  container: shared
dist_cache:
  server-list: "localhost:9065"
`)
	dcB := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dcB.Configure(true))

	assert.NotEqual(t, dcA.cachePrefix, dcB.cachePrefix,
		"different accounts must yield different prefixes")
	assert.Equal(t, "tenantA/shared", dcA.cachePrefix)
	assert.Equal(t, "tenantB/shared", dcB.cachePrefix)
}

func TestConfigure_ExplicitCachePrefixOverridesAzStorage(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
dist_cache:
  server-list: "localhost:9065"
  cache-prefix: "custom/override"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err)
	assert.Equal(t, "custom/override", dc.cachePrefix,
		"explicit cache-prefix must take precedence over azstorage-derived default")
}

func TestConfigure_ExplicitCachePrefixWithoutAzStorage(t *testing.T) {
	// An explicit cache-prefix should be accepted even when azstorage.account-name
	// and azstorage.container are not configured (e.g. loopback / non-Azure tests).
	loadConfig(t, `
dist_cache:
  server-list: "localhost:9065"
  cache-prefix: "loopback/tests"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err)
	assert.Equal(t, "loopback/tests", dc.cachePrefix)
}

func TestConfigure_EmptyExplicitCachePrefixFallsBackToAzStorage(t *testing.T) {
	// An empty-string cache-prefix must not shadow the azstorage-derived default.
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
dist_cache:
  server-list: "localhost:9065"
  cache-prefix: ""
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err)
	assert.Equal(t, "myacct/mycontainer", dc.cachePrefix)
}
