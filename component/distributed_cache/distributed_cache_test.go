// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package distributed_cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	dcache "github.com/nearora-msft/dist-cache-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDCacheClient implements dcacheClient for testing.
type mockDCacheClient struct {
	store             map[string][]byte
	chunkFn           func(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	uploadChunkFn     func(ctx context.Context, filename string, offset int64, data []byte) error
	uploadChunkCalled int

	// uploadEtags captures the etag passed to each UploadChunk call in
	// invocation order. Populated regardless of whether uploadChunkFn is set.
	uploadMu    sync.Mutex
	uploadEtags []string
}

func newMockDCacheClient() *mockDCacheClient {
	return &mockDCacheClient{
		store: make(map[string][]byte),
	}
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
	m.uploadMu.Lock()
	m.uploadEtags = append(m.uploadEtags, etag)
	m.uploadMu.Unlock()
	if m.uploadChunkFn != nil {
		return m.uploadChunkFn(ctx, filename, offset, data)
	}
	m.uploadChunkCalled++
	key := fmt.Sprintf("%s:%d", filename, offset)
	m.store[key] = append([]byte(nil), data...)
	return nil
}

// uploadedEtags returns a snapshot of etags recorded so far.
func (m *mockDCacheClient) uploadedEtags() []string {
	m.uploadMu.Lock()
	defer m.uploadMu.Unlock()
	out := make([]string, len(m.uploadEtags))
	copy(out, m.uploadEtags)
	return out
}

func (m *mockDCacheClient) Close() error {
	return nil
}

// mockNextComponent records calls to NextComponent methods.
type mockNextComponent struct {
	internal.BaseComponent
	readInBufferCalled int

	readInBufferData []byte // data returned by ReadInBuffer
	readInBufferFn   func(options *internal.ReadInBufferOptions) (int, error)
	getAttrETag      string // ETag returned by GetAttr (empty = new file)

	// writeReturnedETag, if non-empty, is written into *options.Etag before
	// ReadInBuffer returns. Mirrors azstorage.BlockBlob.ReadInBuffer, which
	// sets *options.Etag to the observed blob ETag on success.
	writeReturnedETag string
}

func (m *mockNextComponent) GetAttr(_ internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if m.getAttrETag == "" {
		return &internal.ObjAttr{}, nil
	}
	return &internal.ObjAttr{ETag: m.getAttrETag}, nil
}

func (m *mockNextComponent) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	m.readInBufferCalled++
	if m.writeReturnedETag != "" && options.Etag != nil {
		*options.Etag = m.writeReturnedETag
	}
	if m.readInBufferFn != nil {
		return m.readInBufferFn(options)
	}
	if m.readInBufferData != nil {
		n := copy(options.Data, m.readInBufferData)
		return n, nil
	}
	return 0, nil
}

func newTestDistCache(mock *mockDCacheClient, next *mockNextComponent) *DistCache {
	dc := &DistCache{
		client:        mock,
		chunkSize:     16 * 1024 * 1024,
		bypassOnError: true,
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)

	// Preallocate a small bounded upload pool so tests exercise the real
	// schedulePopulate / doUpload path. Spawn-on-demand goroutines are
	// tracked by dc.inflight; tests don't call Stop but the goroutines exit
	// on their own after each upload.
	const testBuffers = 4
	dc.bufs = make(chan []byte, testBuffers)
	for i := 0; i < testBuffers; i++ {
		dc.bufs <- make([]byte, dc.chunkSize)
	}
	return dc
}

// --- Tests ---

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

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
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

	// Peer still holds the lock at timeout, so populating from this
	// reader would race the peer's write. Confirm we skipped populate.
	dc.inflight.Wait()
	assert.Equal(t, int32(0), uploads.Load(),
		"must NOT populate on poll timeout: peer holds lock, populate would thundering-herd")
}

// TestReadInBuffer_AlreadyLocked_PollInheritsLock_DownloadsAndPopulates verifies
// the recovery path when the peer's miss-lock expires mid-poll. The polling
// DownloadChunk (with WithLock) returns ErrNotFoundGotLock, transferring
// lock ownership to us; ReadInBuffer must then fetch from Azure AND populate
// the cache since we now own the miss.
func TestReadInBuffer_AlreadyLocked_PollInheritsLock_DownloadsAndPopulates(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure data via inherited lock")
	// writeReturnedETag mirrors azstorage.BlockBlob.ReadInBuffer, which
	// sets *options.Etag on success. Populate is skipped without it.
	next := &mockNextComponent{readInBufferData: azData, writeReturnedETag: "az-etag"}
	dc := newTestDistCache(mock, next)

	// First DownloadChunk (from ReadInBuffer's initial attempt): locked → poll.
	// Second DownloadChunk (first poll tick): peer's lock TTL'd out → we own it.
	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return 0, dcache.ErrNotFoundGotLock
	}

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
	}

	buf := make([]byte, 1024)
	// block_cache always passes a non-nil pointer so azstorage can stamp it.
	etag := ""
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
		Etag:   &etag,
	})

	assert.NoError(t, err)
	assert.Equal(t, len(azData), n)
	assert.Equal(t, azData, buf[:n])
	assert.Equal(t, 1, next.readInBufferCalled, "should fetch from Azure after inheriting the miss-lock")

	// We own the lock now, so populate must run.
	dc.inflight.Wait()
	assert.Equal(t, int32(1), uploads.Load(),
		"must populate after inheriting the miss-lock via poll")
}

// TestReadInBuffer_AlreadyLocked_PollZeroByteHit_FallsThroughWithoutPopulate
// verifies that a corrupt (zero-byte) cache entry observed during polling is
// treated as a miss: read is served from Azure, but populate is skipped
// because we don't hold the miss-lock.
func TestReadInBuffer_AlreadyLocked_PollZeroByteHit_FallsThroughWithoutPopulate(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure data after corrupt L2 hit")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)

	// First DownloadChunk (from ReadInBuffer): locked → poll.
	// Second DownloadChunk (from poll): zero-byte hit → treat as miss.
	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return 0, nil // zero-byte hit
	}

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
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
	assert.Equal(t, 1, next.readInBufferCalled, "should fetch from Azure on zero-byte L2 hit during poll")

	// We don't hold the lock, so populate must NOT run.
	dc.inflight.Wait()
	assert.Equal(t, int32(0), uploads.Load(),
		"must NOT populate on zero-byte poll hit: no lock ownership")
}

// --- Tests for checksum mismatch handling ---

// wrappedChecksumErr mimics how the real client wraps the sentinel via fmt.Errorf.
func wrappedChecksumErr() error {
	return fmt.Errorf("download chunk foo: %w: expected 1 calculated 2", dcache.ErrChecksumMismatch)
}

func TestReadInBuffer_ChecksumMismatch_BypassFallsThrough(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure data after checksum mismatch")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = true

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, wrappedChecksumErr()
	}

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
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
	assert.Equal(t, 1, next.readInBufferCalled, "should fall through to azstorage on checksum mismatch when bypass-on-error=true")

	dc.inflight.Wait()
	assert.Equal(t, int32(0), uploads.Load(), "must NOT populate on checksum mismatch: peer still owns the entry")
}

func TestReadInBuffer_ChecksumMismatch_StrictReturnsEIO(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("should not be read")}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = false

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, wrappedChecksumErr()
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, syscall.EIO)
	assert.Equal(t, 0, next.readInBufferCalled, "must NOT fall through to azstorage when bypass-on-error=false")
}

// Poll observes a checksum mismatch. bypass-on-error=true → fall through to
// azstorage without populating (peer still owns the miss-lock).
func TestReadInBuffer_AlreadyLocked_PollChecksumMismatch_BypassFallsThrough(t *testing.T) {
	mock := newMockDCacheClient()
	azData := []byte("azure data after poll checksum mismatch")
	next := &mockNextComponent{readInBufferData: azData}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = true

	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return 0, wrappedChecksumErr()
	}

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
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
	assert.Equal(t, 1, next.readInBufferCalled, "should fall through to azstorage on poll checksum mismatch when bypass-on-error=true")

	dc.inflight.Wait()
	assert.Equal(t, int32(0), uploads.Load(), "must NOT populate on poll checksum mismatch: no lock ownership")
}

// Poll observes a checksum mismatch, strict mode → EIO (no azstorage fallback).
func TestReadInBuffer_AlreadyLocked_PollChecksumMismatch_StrictReturnsEIO(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{readInBufferData: []byte("should not be read")}
	dc := newTestDistCache(mock, next)
	dc.bypassOnError = false

	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return 0, wrappedChecksumErr()
	}

	buf := make([]byte, 1024)
	n, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
	})

	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, syscall.EIO)
	assert.Equal(t, 0, next.readInBufferCalled, "must NOT fall through to azstorage when bypass-on-error=false")
}

// pollChunkIntoBuffer maps a wrapped ErrChecksumMismatch to errPollChecksumMismatch,
// keeping it distinct from the zero-byte errPollCorruptHit so the caller can enforce policy.
func TestPollChunkIntoBuffer_ChecksumMismatch_ReturnsDistinctSentinel(t *testing.T) {
	mock := newMockDCacheClient()
	dc := newTestDistCache(mock, &mockNextComponent{})

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, wrappedChecksumErr()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	buf := make([]byte, 1024)
	n, err := dc.pollChunkIntoBuffer(ctx, "test/file.bin", "", 0, buf)

	assert.Equal(t, 0, n)
	assert.True(t, errors.Is(err, errPollChecksumMismatch), "poll must return errPollChecksumMismatch, got %v", err)
	assert.False(t, errors.Is(err, errPollCorruptHit), "checksum mismatch must NOT be conflated with zero-byte hit")
}

func TestPriority(t *testing.T) {
	dc := &DistCache{}
	assert.Equal(t, internal.EComponentPriority.LevelMid(), dc.Priority())
}

// --- Tests for recoverable network error handling ---

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
distributed_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err)
	assert.Equal(t, "myacct/mycontainer", dc.cachePrefix)
}

func TestConfigure_VerifyChecksumDefaultAndOverride(t *testing.T) {
	tests := []struct {
		name       string
		setting    string
		wantVerify bool
	}{
		{name: "defaults to true", wantVerify: true},
		{name: "explicit false overrides default", setting: "  verify-checksum: false\n", wantVerify: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
block_cache:
  mem-size-mb: 100
distributed_cache:
  server-list: "localhost:9065"
`+test.setting)

			dc := NewDistCacheComponent().(*DistCache)
			require.NoError(t, dc.Configure(true))
			assert.Equal(t, test.wantVerify, dc.conf.VerifyChecksum)
		})
	}
}

func TestConfigure_FailsWhenAccountNameMissing(t *testing.T) {
	loadConfig(t, `
azstorage:
  container: mycontainer
distributed_cache:
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
distributed_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "azstorage.container")
}

func TestConfigure_FailsWhenBothMissing(t *testing.T) {
	loadConfig(t, `
distributed_cache:
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
distributed_cache:
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
distributed_cache:
  server-list: "localhost:9065"
`)
	dcA := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dcA.Configure(true))

	loadConfig(t, `
azstorage:
  account-name: tenantB
  container: shared
distributed_cache:
  server-list: "localhost:9065"
`)
	dcB := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dcB.Configure(true))

	assert.NotEqual(t, dcA.cachePrefix, dcB.cachePrefix,
		"different accounts must yield different prefixes")
	assert.Equal(t, "tenantA/shared", dcA.cachePrefix)
	assert.Equal(t, "tenantB/shared", dcB.cachePrefix)
}

// TestConfigure_TinyFairShare_DisablesPool verifies the wiring from
// resolveMemBudget → Configure: when the fair-share signal is below
// distCacheMinBuffers × chunkSize, Configure must succeed but leave dc.bufs
// nil so the L2 populate path is inert for the mount.
func TestConfigure_TinyFairShare_DisablesPool(t *testing.T) {
	// bcRef  = 100 MiB → fair = 10 MiB
	// chunk  = 16 MiB → floor = 64 MiB
	// fair < floor → resolveMemBudget returns 0, pool must not be allocated.
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
block_cache:
  mem-size-mb: 100
  block-size-mb: 16
  parallelism: 2
distributed_cache:
  server-list: "localhost:9065"
`)

	dc := NewDistCacheComponent().(*DistCache)
	err := dc.Configure(true)
	require.NoError(t, err, "Configure must not fail just because the pool is disabled")
	assert.Nil(t, dc.bufs, "tiny fair-share must leave the buffer pool unallocated")
}

// --- Tests for async-upload buffer pool: memory-budget & drop paths ---

// TestSchedulePopulate_NilBufs_Drops verifies that when the buffer pool is
// disabled (dc.bufs == nil, i.e. resolveMemBudget returned 0 at Configure),
// schedulePopulate is a no-op: no upload is issued and no goroutine is
// spawned, so the L2 populate path stays fully off.
func TestSchedulePopulate_NilBufs_Drops(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)
	dc.bufs = nil // simulate the disabled-pool Configure outcome

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
	}

	dc.schedulePopulate("f", "e", 0, []byte("data"))
	dc.inflight.Wait()

	assert.Equal(t, int32(0), uploads.Load(),
		"schedulePopulate must not issue an upload when the pool is disabled")
}

// TestSchedulePopulate_PayloadTooLarge_Drops verifies that populates whose
// payload exceeds chunkSize are rejected before consuming a buffer.
func TestSchedulePopulate_PayloadTooLarge_Drops(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next)

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
	}

	poolBefore := len(dc.bufs)
	oversize := make([]byte, dc.chunkSize+1)
	dc.schedulePopulate("f", "e", 0, oversize)
	dc.inflight.Wait()

	assert.Equal(t, int32(0), uploads.Load(), "oversize payload must not trigger an upload")
	assert.Equal(t, poolBefore, len(dc.bufs), "oversize drop must not consume a buffer")
}

// TestSchedulePopulate_PoolExhausted_Drops verifies the core memory-safety
// invariant: once all buffers are held by in-flight uploads, additional
// populates are dropped (non-blocking select) rather than allocating more
// memory or blocking the caller.
func TestSchedulePopulate_PoolExhausted_Drops(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next) // 4-buffer pool

	release := make(chan struct{})
	var started atomic.Int32
	var completed atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		started.Add(1)
		<-release
		completed.Add(1)
		return nil
	}

	// Fill the pool and wait until every upload is actively holding a buffer.
	for i := 0; i < 4; i++ {
		dc.schedulePopulate("f", "e", int64(i), []byte("data"))
	}
	require.Eventually(t, func() bool { return started.Load() == 4 },
		time.Second, 5*time.Millisecond, "expected all 4 initial uploads to start")

	// Schedule many more populates while the pool is empty, from concurrent
	// goroutines to mimic FUSE worker threads racing on dc.bufs. Each call
	// must return immediately without blocking and without incrementing
	// started. Running in parallel (rather than serially) gives `-race` a
	// real chance to catch any unsafe access to the buffer pool.
	const extras = 50
	var extrasWG sync.WaitGroup
	extrasWG.Add(extras)
	for i := 0; i < extras; i++ {
		go func(offset int64) {
			defer extrasWG.Done()
			dc.schedulePopulate("f", "e", offset, []byte("data"))
		}(int64(1000 + i))
	}
	extrasWG.Wait()

	// Give any (incorrectly) scheduled extras time to appear.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(4), started.Load(),
		"pool exhaustion must drop populates, not queue them")

	// Release the four in-flight uploads and drain.
	close(release)
	dc.inflight.Wait()
	assert.Equal(t, int32(4), completed.Load(), "exactly the 4 buffered uploads should complete")
}

// TestSchedulePopulate_BufferReturnedAfterUpload verifies that doUpload
// returns its borrowed buffer to the pool on success, so subsequent
// populates can succeed once earlier uploads drain.
func TestSchedulePopulate_BufferReturnedAfterUpload(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := &DistCache{
		client:    mock,
		chunkSize: 16 * 1024 * 1024,
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)
	// 1-buffer pool so we can prove recycling.
	dc.bufs = make(chan []byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- b

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return nil
	}

	// First populate consumes the only buffer.
	dc.schedulePopulate("f", "e", 0, []byte("data"))
	dc.inflight.Wait()
	require.Equal(t, int32(1), uploads.Load())
	require.Equal(t, 1, len(dc.bufs), "buffer must be returned after upload completes")

	// Second populate re-uses the returned buffer.
	dc.schedulePopulate("f", "e", 1, []byte("more"))
	dc.inflight.Wait()
	assert.Equal(t, int32(2), uploads.Load())
	assert.Equal(t, 1, len(dc.bufs), "buffer must be returned again after second upload")
}

// --- Tests for resolveMemBudget ---
//
// resolveMemBudget picks min(fair-share, demand-ceiling) on top of a
// reference budget (bcRef), but only if the fair-share signal covers at
// least distCacheMinBuffers × chunkSize; below that it returns 0 to
// disable async populate. bcRef is block_cache.mem-size-mb when set, else
// block_cache.MemShareFraction × free RAM. The tests below fix
// block_cache.mem-size-mb and block_cache.parallelism so the arithmetic is
// deterministic; separate tests cover the free-RAM fallback path.

// TestResolveMemBudget_MemSize_DemandCeilingBinds verifies that when
// block_cache.mem-size-mb yields a fair share larger than the demand ceiling,
// the demand ceiling wins.
func TestResolveMemBudget_MemSize_DemandCeilingBinds(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 10240
  parallelism: 2
`)

	dc := &DistCache{chunkSize: 16 * _1MiB}
	got := dc.resolveMemBudget()

	// bcRef  = 10240 MiB
	// fair   = 10240 × 10%       = 1024 MiB (≥ floor 64 MiB, pool enabled)
	// demand = 4 × 2 × 16 MiB    = 128 MiB
	// budget = min(fair, demand) = 128 MiB
	assert.Equal(t, int64(128)*_1MiB, got)
}

// TestResolveMemBudget_MemSize_FairShareBinds verifies that when the demand
// ceiling is larger than the fair share, the fair share wins.
func TestResolveMemBudget_MemSize_FairShareBinds(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 100
  parallelism: 100
`)

	dc := &DistCache{chunkSize: 1 * _1MiB}
	got := dc.resolveMemBudget()

	// bcRef  = 100 MiB
	// fair   = 100 × 10%         = 10 MiB (≥ floor 4 MiB, pool enabled)
	// demand = 4 × 100 × 1 MiB   = 400 MiB
	// budget = min(fair, demand) = 10 MiB
	assert.Equal(t, int64(10)*_1MiB, got)
}

// TestResolveMemBudget_MemSize_FairShareBelowFloor_DisablesPool verifies
// that when the fair-share signal cannot cover distCacheMinBuffers chunks,
// resolveMemBudget returns 0 so Configure will skip pool allocation and
// L2 populate stays disabled for the mount.
func TestResolveMemBudget_MemSize_FairShareBelowFloor_DisablesPool(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 100
  parallelism: 2
`)

	dc := &DistCache{chunkSize: 16 * _1MiB}
	got := dc.resolveMemBudget()

	// bcRef  = 100 MiB
	// fair   = 100 × 10%         = 10 MiB
	// floor  = 4 × 16 MiB        = 64 MiB
	// fair < floor  → return 0 (async populate disabled).
	assert.Equal(t, int64(0), got)
}

// TestResolveMemBudget_MemSizeUnset_UsesFreeRAM verifies that when
// block_cache.mem-size-mb is absent, the reference budget is derived from
// available system memory (which produces a fair share substantially larger
// than the floor on any realistic host).
func TestResolveMemBudget_MemSizeUnset_UsesFreeRAM(t *testing.T) {
	loadConfig(t, `
block_cache:
  parallelism: 1000
`)

	dc := &DistCache{chunkSize: 1 * _1MiB}
	got := dc.resolveMemBudget()

	// demand = 4 × 1000 × 1 MiB = 4000 MiB (deliberately large so demand does
	// not clip the fair share on hosts with a few GiB of free RAM).
	// fair from the free-RAM fallback path is bounded below by
	// DefaultMemFallbackBytes × MemShareFraction × 10% ≈ 251 MiB even if
	// /proc/meminfo cannot be read, which is well above the floor.
	floor := int64(distCacheMinBuffers) * dc.chunkSize
	assert.Greater(t, got, floor,
		"free-RAM fallback should produce a fair share above the floor")
}

// TestResolveMemBudget_MemSizeZero_FallsBackToFreeRAM verifies that
// block_cache.mem-size-mb=0 is treated the same as unset (guarded by the
// n > 0 check inside resolveMemBudget), rather than pinning bcRef to zero
// and collapsing the fair share.
func TestResolveMemBudget_MemSizeZero_FallsBackToFreeRAM(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 0
  parallelism: 1000
`)

	dc := &DistCache{chunkSize: 1 * _1MiB}
	got := dc.resolveMemBudget()

	// Same reasoning as the "unset" case: an explicit zero must not silently
	// disable async populate.
	floor := int64(distCacheMinBuffers) * dc.chunkSize
	assert.Greater(t, got, floor,
		"mem-size-mb=0 must fall back to the free-RAM path, not zero-out bcRef")
}

// --- Test: Stop() drains in-flight uploads before closing the dcache client ---

// blockingUploadClient is a controllable dcacheClient used to prove Stop()
// waits for in-flight populates before invoking client.Close(). UploadChunk
// and Close both run test-supplied hooks so the test can gate upload
// completion and observe the exact ordering of the two events.
type blockingUploadClient struct {
	onUpload func()
	onClose  func()
}

func (c *blockingUploadClient) DownloadChunk(_ context.Context, _, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
	return 0, dcache.ErrNotFound
}

func (c *blockingUploadClient) UploadChunk(_ context.Context, _, _ string, _ int64, _ []byte, _ ...dcache.UploadOption) error {
	if c.onUpload != nil {
		c.onUpload()
	}
	return nil
}

func (c *blockingUploadClient) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}

// TestStop_WaitsForInflightUploads verifies the Stop() contract: it must
// block on dc.inflight until every async populate has returned, then invoke
// client.Close(). If Close ran before an in-flight UploadChunk completed,
// doUpload could observe a torn-down client mid-request.
func TestStop_WaitsForInflightUploads(t *testing.T) {
	release := make(chan struct{})
	uploadStarted := make(chan struct{})
	var startOnce sync.Once

	var (
		mu             sync.Mutex
		uploadFinished time.Time
		closedAt       time.Time
	)

	client := &blockingUploadClient{
		onUpload: func() {
			startOnce.Do(func() { close(uploadStarted) })
			<-release
			mu.Lock()
			uploadFinished = time.Now()
			mu.Unlock()
		},
		onClose: func() {
			mu.Lock()
			closedAt = time.Now()
			mu.Unlock()
		},
	}

	// Wire a minimal DistCache directly to the mock client, skipping
	// Configure/Start so we exercise Stop()'s ordering contract in isolation.
	dc := &DistCache{
		client:    client,
		chunkSize: 1024,
	}
	dc.SetName(compName)
	dc.bufs = make(chan []byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- b

	// Schedule an async populate. schedulePopulate borrows the buffer and
	// spawns doUpload, which enters UploadChunk and blocks on `release`.
	dc.schedulePopulate("file.bin", "etag", 0, []byte("hello"))

	// Wait until UploadChunk is actually running before calling Stop, so the
	// test measures Stop's wait rather than a race where the goroutine hadn't
	// entered UploadChunk yet.
	select {
	case <-uploadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("UploadChunk was not entered within 2s")
	}

	stopDone := make(chan error, 1)
	go func() { stopDone <- dc.Stop() }()

	// Stop must remain blocked while the upload is still in flight.
	select {
	case err := <-stopDone:
		t.Fatalf("Stop() returned before in-flight upload finished: err=%v", err)
	case <-time.After(200 * time.Millisecond):
	}

	// Release the upload; Stop() should now drain and Close the client.
	close(release)

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s after upload released")
	}

	mu.Lock()
	defer mu.Unlock()
	require.False(t, uploadFinished.IsZero(), "UploadChunk never recorded a finish time")
	require.False(t, closedAt.IsZero(), "Close() was never called on the client")
	assert.False(t, closedAt.Before(uploadFinished),
		"Close() ran before UploadChunk finished: closedAt=%v uploadFinished=%v",
		closedAt, uploadFinished)
}

// --- Tests for doUpload failure semantics ---

// TestDoUpload_UploadChunkError_ReturnsBufferToPool verifies the load-bearing
// invariant that doUpload's `defer` returns the borrowed buffer to the pool
// even when UploadChunk fails. Without this, a run of failing populates
// would drain the pool permanently and silently disable async populate for
// the lifetime of the mount.
func TestDoUpload_UploadChunkError_ReturnsBufferToPool(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := newTestDistCache(mock, next) // 4-buffer pool

	var uploads atomic.Int32
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, _ []byte) error {
		uploads.Add(1)
		return fmt.Errorf("simulated dcache upload failure")
	}

	poolBefore := len(dc.bufs)

	// Schedule more populates than the pool can hold in parallel. If failed
	// uploads leaked their buffer, later populates would be dropped by the
	// non-blocking select and uploads.Load() would fall short of the total.
	const total = 20
	for i := 0; i < total; i++ {
		dc.schedulePopulate("f", "e", int64(i), []byte("data"))
		dc.inflight.Wait() // serialize so each upload can recycle its buffer
	}

	assert.Equal(t, int32(total), uploads.Load(),
		"every scheduled populate must reach UploadChunk (buffer must be recycled after failure)")
	assert.Equal(t, poolBefore, len(dc.bufs),
		"pool must be fully restored after failed uploads drain")
}

// TestDoUpload_UploadsExactPayloadBytes verifies that doUpload sends exactly
// len(src) bytes matching the source, not the full chunk-sized buffer. This
// guards two invariants at once:
//   - the `data := (*buf)[:length]` slice in doUpload correctly bounds the
//     upload to the payload length (a regression to `(*buf)` would ship
//     chunkSize bytes of trailing junk for every short populate);
//   - buffer recycling does not leak stale bytes from a previous upload into
//     a later, shorter one.
func TestDoUpload_UploadsExactPayloadBytes(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{}
	dc := &DistCache{
		client:    mock,
		chunkSize: 16 * 1024 * 1024,
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)
	// 1-buffer pool forces the second populate to reuse the first's buffer.
	dc.bufs = make(chan []byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- b

	var (
		mu   sync.Mutex
		seen [][]byte
	)
	mock.uploadChunkFn = func(_ context.Context, _ string, _ int64, data []byte) error {
		// Copy defensively: doUpload's slice aliases the pooled buffer and
		// will be reused after this call returns.
		snapshot := append([]byte(nil), data...)
		mu.Lock()
		seen = append(seen, snapshot)
		mu.Unlock()
		return nil
	}

	long := []byte(strings.Repeat("A", 4096))
	short := []byte("XYZ")

	dc.schedulePopulate("f", "e", 0, long)
	dc.inflight.Wait()
	dc.schedulePopulate("f", "e", 1, short)
	dc.inflight.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 2)
	assert.Equal(t, long, seen[0], "first upload must ship exactly the long payload")
	assert.Equal(t, short, seen[1],
		"second upload must ship exactly the short payload — no leftover 'A' bytes from the recycled buffer")
}

// --- RegisterEnvVariables: env-var → config binding for discovery keys ---
//
// config.ResetConfig() (called by loadConfig) wipes the envTree, so each test
// must re-install the bindings via RegisterEnvVariables() after loading YAML.
// Env values are set with t.Setenv so cleanup is automatic.

// TestRegisterEnvVariables_BindsDiscoveryEndpoint verifies that
// DIST_CACHE_DISCOVERY_ENDPOINT flows into conf.DiscoveryEndpoint when YAML omits it.
func TestRegisterEnvVariables_BindsDiscoveryEndpoint(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
distributed_cache: {}
`)
	RegisterEnvVariables()
	t.Setenv(EnvDistCacheDiscoveryEndpoint, "127.0.0.1:9000")

	dc := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dc.Configure(true))
	assert.Equal(t, "127.0.0.1:9000", dc.conf.DiscoveryEndpoint)
}

// TestRegisterEnvVariables_BindsK8sServiceAndNamespace verifies that both
// K8s discovery env vars land in conf.
func TestRegisterEnvVariables_BindsK8sServiceAndNamespace(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
distributed_cache: {}
`)
	RegisterEnvVariables()
	t.Setenv(EnvDistCacheK8sService, "dcache-svc")
	t.Setenv(EnvDistCacheK8sNamespace, "cache-ns")

	dc := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dc.Configure(true))
	assert.Equal(t, "dcache-svc", dc.conf.K8sService)
	assert.Equal(t, "cache-ns", dc.conf.K8sNamespace)
}

// TestRegisterEnvVariables_BindsServerList verifies the env-only path for
// server-list: no YAML entry, only the env var, Configure must succeed.
func TestRegisterEnvVariables_BindsServerList(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
distributed_cache: {}
`)
	RegisterEnvVariables()
	t.Setenv(EnvDistCacheServerList, "host1:9065,host2:9065")

	dc := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dc.Configure(true))
	assert.Equal(t, "host1:9065,host2:9065", dc.conf.ServerList)
}

// TestRegisterEnvVariables_EnvOverridesYAML verifies viper precedence:
// env value wins over an unchanged YAML value for the same key.
func TestRegisterEnvVariables_EnvOverridesYAML(t *testing.T) {
	loadConfig(t, `
azstorage:
  account-name: myacct
  container: mycontainer
distributed_cache:
  server-list: "yaml-host:9065"
`)
	RegisterEnvVariables()
	t.Setenv(EnvDistCacheServerList, "env-host:9065")

	dc := NewDistCacheComponent().(*DistCache)
	require.NoError(t, dc.Configure(true))
	assert.Equal(t, "env-host:9065", dc.conf.ServerList,
		"env var must override YAML for bound keys")
}

// --- resolveETag: only the storage-returned etag drives L2 population ---
//
// azstorage.BlockBlob.ReadInBuffer writes the observed blob ETag into
// *options.Etag on success. distributed_cache must key the L2 populate on that
// returned value so the chunk lands under the blob's current version. When
// storage does not set it (nil pointer or empty string), resolveETag returns
// "" and schedulePopulate skips the upload rather than falling back to a
// possibly stale ETag or an ambiguous empty group ID.

func TestResolveETag_ReturnsStorageETagWhenNonEmpty(t *testing.T) {
	returned := "etag-from-storage"
	opts := &internal.ReadInBufferOptions{Etag: &returned}
	got := resolveETag(opts)
	assert.Equal(t, "etag-from-storage", got,
		"must return the storage-observed etag verbatim")
}

func TestResolveETag_ReturnsEmptyWhenPointerNil(t *testing.T) {
	opts := &internal.ReadInBufferOptions{Etag: nil}
	got := resolveETag(opts)
	assert.Equal(t, "", got,
		"must return \"\" when storage didn't set options.Etag")
}

func TestResolveETag_ReturnsEmptyWhenPointerEmpty(t *testing.T) {
	empty := ""
	opts := &internal.ReadInBufferOptions{Etag: &empty}
	got := resolveETag(opts)
	assert.Equal(t, "", got,
		"must return \"\" when storage left *options.Etag empty")
}

// TestReadInBuffer_GotLock_PopulateUsesReturnedETag verifies that when
// azstorage returns a newer ETag on the miss-lock download path, the L2
// populate is keyed on the returned ETag, not the pre-lookup one.
func TestReadInBuffer_GotLock_PopulateUsesReturnedETag(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{
		readInBufferData:  []byte("azure chunk"),
		writeReturnedETag: "new-etag",
	}
	dc := newTestDistCache(mock, next)

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}

	lookupETag := "old-etag"
	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
		Etag:   &lookupETag,
	})
	require.NoError(t, err)

	dc.inflight.Wait()
	assert.Equal(t, []string{"new-etag"}, mock.uploadedEtags(),
		"populate must key on the ETag returned by storage, not the lookup ETag")
}

// TestReadInBuffer_ZeroByteHit_DoesNotPopulate verifies that a corrupt L2 hit
// falls through without uploading because the caller does not own a miss-lock.
func TestReadInBuffer_ZeroByteHit_DoesNotPopulate(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{
		readInBufferData:  []byte("azure chunk"),
		writeReturnedETag: "new-etag",
	}
	dc := newTestDistCache(mock, next)

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, nil // zero-byte hit
	}

	lookupETag := "old-etag"
	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
		Etag:   &lookupETag,
	})
	require.NoError(t, err)

	dc.inflight.Wait()
	assert.Empty(t, mock.uploadedEtags(),
		"zero-byte hit must not populate without owning the miss-lock")
}

// TestReadInBuffer_AlreadyLocked_InheritedLock_PopulateUsesReturnedETag verifies
// that inheriting the miss-lock via poll also routes populate through the
// storage-returned ETag.
func TestReadInBuffer_AlreadyLocked_InheritedLock_PopulateUsesReturnedETag(t *testing.T) {
	mock := newMockDCacheClient()
	next := &mockNextComponent{
		readInBufferData:  []byte("azure chunk"),
		writeReturnedETag: "new-etag",
	}
	dc := newTestDistCache(mock, next)

	callCount := 0
	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		callCount++
		if callCount == 1 {
			return 0, dcache.ErrNotFoundAlreadyLocked
		}
		return 0, dcache.ErrNotFoundGotLock
	}

	lookupETag := "old-etag"
	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   "test/file.bin",
		Offset: 0,
		Data:   buf,
		Etag:   &lookupETag,
	})
	require.NoError(t, err)

	dc.inflight.Wait()
	assert.Equal(t, []string{"new-etag"}, mock.uploadedEtags(),
		"inherited-lock populate must key on the storage-returned ETag")
}

// TestReadInBuffer_GotLock_EmptyReturnedETag_SkipsPopulate verifies that when
// storage does not update options.Etag (empty pointer preserved), populate is
// skipped entirely rather than falling back to a possibly stale lookup ETag.
func TestReadInBuffer_GotLock_EmptyReturnedETag_SkipsPopulate(t *testing.T) {
	mock := newMockDCacheClient()
	// writeReturnedETag left empty — mimics a storage layer that returns no
	// ETag header (sanitizeEtag maps a nil ETag to "").
	next := &mockNextComponent{readInBufferData: []byte("azure chunk")}
	dc := newTestDistCache(mock, next)

	mock.chunkFn = func(_ context.Context, _ string, _ int64, _ []byte, _ ...dcache.DownloadOption) (int, error) {
		return 0, dcache.ErrNotFoundGotLock
	}

	h := handlemap.NewHandle("test/file.bin")

	empty := ""
	buf := make([]byte, 1024)
	_, err := dc.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: h,
		Offset: 0,
		Data:   buf,
		Etag:   &empty,
	})
	require.NoError(t, err)

	dc.inflight.Wait()
	assert.Empty(t, mock.uploadedEtags(),
		"populate must be skipped when storage returned no ETag; falling back to the lookup ETag would risk a stale-version write")
}
