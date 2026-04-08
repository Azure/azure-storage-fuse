// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"fmt"
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
	store      map[string][]byte
	downloadFn func(ctx context.Context, filename string, fileSize int64, w *os.File, opts ...dcache.DownloadOption) (*dcache.FileMetadata, error)
	chunkFn    func(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
}

func newMockDCacheClient() *mockDCacheClient {
	return &mockDCacheClient{
		store: make(map[string][]byte),
	}
}

func (m *mockDCacheClient) Upload(_ context.Context, filename string, data *os.File, size int64, _ ...dcache.UploadOption) error {
	buf := make([]byte, size)
	data.ReadAt(buf, 0)
	m.store[filename] = buf
	return nil
}

func (m *mockDCacheClient) DownloadWithSize(ctx context.Context, filename string, fileSize int64, w *os.File, opts ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, filename, fileSize, w, opts...)
	}
	data, ok := m.store[filename]
	if !ok {
		return nil, dcache.ErrNotFound
	}
	n, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	return &dcache.FileMetadata{Size: int64(n)}, nil
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
	mock.downloadFn = func(_ context.Context, _ string, _ int64, w *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		w.Write(testData)
		return &dcache.FileMetadata{Size: int64(len(testData))}, nil
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
	next := &mockNextComponent{copyToFileData: []byte("data from azure")}
	dc := newTestDistCache(mock, next)

	// Simulate L2 miss with lock acquired
	mock.downloadFn = func(_ context.Context, _ string, _ int64, _ *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		return nil, dcache.ErrNotFoundGotLock
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
	assert.Equal(t, 1, next.copyToFileCalled, "should call azstorage on L2 miss")
}

func TestCopyToFile_BypassOnError(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{copyToFileData: []byte("fallback data")}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = true

	// Simulate connection error
	mock.downloadFn = func(_ context.Context, _ string, _ int64, _ *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
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
	dc := &DistCache{bypassOnError: true}
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

func TestPriority(t *testing.T) {
	dc := &DistCache{}
	assert.Equal(t, internal.EComponentPriority.LevelMid(), dc.Priority())
}

func TestPollUntilCached_SucceedsOnRetry(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{copyToFileData: []byte("azure data")}
	dc := newTestDistCache(mock, next)

	// First call (from CopyToFile with lock): ErrNotFoundAlreadyLocked.
	// Second call (from pollUntilCached retry): data is available.
	callCount := 0
	testData := []byte("cached after retry")
	mock.downloadFn = func(_ context.Context, _ string, _ int64, w *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		callCount++
		if callCount == 1 {
			return nil, dcache.ErrNotFoundAlreadyLocked
		}
		w.Write(testData)
		return &dcache.FileMetadata{Size: int64(len(testData))}, nil
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

func TestPollUntilCached_SeeksBeforeRetry(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Simulate progressive caching: write partial data then fail, then succeed.
	callCount := 0
	fullData := []byte("complete file contents here")
	mock.downloadFn = func(_ context.Context, _ string, _ int64, w *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		callCount++
		switch {
		case callCount == 1:
			return nil, dcache.ErrNotFoundAlreadyLocked // initial check
		case callCount == 2:
			// Partial write (simulating some cached chunks)
			w.Write(fullData[:10])
			return nil, dcache.ErrNotFoundAlreadyLocked
		default:
			// Full data available
			w.Write(fullData)
			return &dcache.FileMetadata{Size: int64(len(fullData))}, nil
		}
	}

	f, err := os.CreateTemp("", "dcache-seek-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	ctx := context.Background()
	meta, err := dc.pollUntilCached(ctx, "test/seek.txt", int64(len(fullData)), f)

	assert.NoError(t, err)
	require.NotNil(t, meta)

	// Verify the file contains the correct data (not corruption from partial writes)
	f.Seek(0, 0)
	got := make([]byte, len(fullData))
	n, _ := f.Read(got)
	assert.Equal(t, len(fullData), n)
	assert.Equal(t, fullData, got)
}

func TestPollUntilCached_StaleGivesUpEarly(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	// Always return ErrNotFoundAlreadyLocked with no progress (0 bytes written).
	mock.downloadFn = func(_ context.Context, _ string, _ int64, _ *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		return nil, dcache.ErrNotFoundAlreadyLocked
	}

	f, err := os.CreateTemp("", "dcache-stale-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	// Use a large file size so the size-adaptive timeout would be long,
	// but stale detection should trigger much sooner.
	ctx := context.Background()
	start := time.Now()
	_, err = dc.pollUntilCached(ctx, "test/stale.txt", 10*1024*1024*1024, f)

	elapsed := time.Since(start)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "poll timeout")
	// Stale detection (30s) + some backoff overhead. Should not wait the
	// full size-proportional timeout (~10s at 2GB/s × 2 headroom = 25s, but
	// stale kicks in at 30s). Allow generous margin for CI.
	assert.Less(t, elapsed, 90*time.Second, "should give up after stale timeout, not wait full duration")
}

func TestPollUntilCached_ContextCancellation(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	mock.downloadFn = func(_ context.Context, _ string, _ int64, _ *os.File, _ ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
		return nil, dcache.ErrNotFoundAlreadyLocked
	}

	f, err := os.CreateTemp("", "dcache-ctx-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = dc.pollUntilCached(ctx, "test/cancel.txt", 1024, f)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
