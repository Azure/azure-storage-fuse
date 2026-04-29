// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDCacheClient implements dcacheClient for testing.
type mockDCacheClient struct {
	store             map[string][]byte
	downloadPartialFn func(ctx context.Context, filename string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) ([]dcache.ChunkError, error)
	chunkFn           func(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
}

func newMockDCacheClient() *mockDCacheClient {
	return &mockDCacheClient{
		store: make(map[string][]byte),
	}
}

func (m *mockDCacheClient) Upload(_ context.Context, filename string, data io.Reader, size int64, _ ...dcache.UploadOption) error {
	buf := make([]byte, size)
	io.ReadFull(data, buf)
	m.store[filename] = buf
	return nil
}

func (m *mockDCacheClient) DownloadWithSizePartial(ctx context.Context, filename string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) ([]dcache.ChunkError, error) {
	if m.downloadPartialFn != nil {
		return m.downloadPartialFn(ctx, filename, fileSize, w, opts...)
	}
	data, ok := m.store[filename]
	if !ok {
		return []dcache.ChunkError{{Offset: 0, Size: fileSize, Err: dcache.ErrNotFound}}, nil
	}
	w.WriteAt(data, 0)
	return nil, nil
}

func (m *mockDCacheClient) DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error) {
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

func (m *mockDCacheClient) UploadChunk(_ context.Context, filename string, offset int64, data []byte, _ ...dcache.UploadOption) error {
	key := fmt.Sprintf("%s:%d", filename, offset)
	m.store[key] = append([]byte(nil), data...)
	return nil
}

func (m *mockDCacheClient) Delete(_ context.Context, filename string, _ int64) error {
	delete(m.store, filename)
	return nil
}

func (m *mockDCacheClient) DeleteGroup(_ context.Context, groupID []byte) error {
	// Remove all entries whose key starts with the group ID (filename)
	prefix := string(groupID)
	for k := range m.store {
		if k == prefix || len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(m.store, k)
		}
	}
	return nil
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
		client:        mock,
		chunkSize:     16 * 1024 * 1024,
		bypassOnError: true,
		dirtyFiles:    make(map[string]time.Time),
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

func TestCommitData_ForwardOnly(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	err := dc.CommitData(internal.CommitDataOptions{
		Name: "test/file.bin",
		List: []string{"block-0", "block-1"},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, next.commitDataCalled)
}

func TestDeleteFile_Invalidation(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
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
	next := &mockNextComponent{}
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
