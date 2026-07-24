// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal"
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
	if m.uploadChunkFn != nil {
		return m.uploadChunkFn(ctx, filename, offset, data)
	}
	m.uploadChunkCalled++
	key := fmt.Sprintf("%s:%d", filename, offset)
	m.store[key] = append([]byte(nil), data...)
	return nil
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
}

func (m *mockNextComponent) GetAttr(_ internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if m.getAttrETag == "" {
		return &internal.ObjAttr{}, nil
	}
	return &internal.ObjAttr{ETag: m.getAttrETag}, nil
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
	dc.bufs = make(chan *[]byte, testBuffers)
	for i := 0; i < testBuffers; i++ {
		b := make([]byte, dc.chunkSize)
		dc.bufs <- &b
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
