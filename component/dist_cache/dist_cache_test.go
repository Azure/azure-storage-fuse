// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

//go:build unittest

package dist_cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// --- Tests for async-upload buffer pool: memory-budget & drop paths ---

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
		client:        mock,
		chunkSize:     16 * 1024 * 1024,
		bypassOnError: true,
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)
	// 1-buffer pool so we can prove recycling.
	dc.bufs = make(chan *[]byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- &b

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
// resolveMemBudget applies a fair-share / demand-ceiling / floor pipeline on
// top of a reference budget (bcRef). bcRef is block_cache.mem-size-mb when
// set, else block_cache.MemShareFraction × free RAM. The tests below fix
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
	// fair   = 10240 × 10%       = 1024 MiB
	// demand = 4 × 2 × 16 MiB    = 128 MiB
	// budget = min(fair, demand) = 128 MiB (floor 64 MiB does not raise)
	assert.Equal(t, int64(128)*_1MiB, got)
}

// TestResolveMemBudget_MemSize_FairShareBinds verifies that when the demand
// ceiling is larger than the fair share, the fair share wins (and the floor
// does not raise it).
func TestResolveMemBudget_MemSize_FairShareBinds(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 100
  parallelism: 100
`)

	dc := &DistCache{chunkSize: 1 * _1MiB}
	got := dc.resolveMemBudget()

	// bcRef  = 100 MiB
	// fair   = 100 × 10%         = 10 MiB
	// demand = 4 × 100 × 1 MiB   = 400 MiB
	// budget = min(fair, demand) = 10 MiB (floor 4 MiB does not raise)
	assert.Equal(t, int64(10)*_1MiB, got)
}

// TestResolveMemBudget_MemSize_FloorRaisesTinyBudget verifies that a fair
// share too small to be useful is raised to the min-buffers floor, and that
// the re-cap step keeps the floor from exceeding the demand ceiling.
func TestResolveMemBudget_MemSize_FloorRaisesTinyBudget(t *testing.T) {
	loadConfig(t, `
block_cache:
  mem-size-mb: 100
  parallelism: 2
`)

	dc := &DistCache{chunkSize: 16 * _1MiB}
	got := dc.resolveMemBudget()

	// bcRef  = 100 MiB
	// fair   = 100 × 10%          = 10 MiB
	// demand = 4 × 2 × 16 MiB     = 128 MiB
	// budget = min(fair, demand)  = 10 MiB
	// floor  = 4 × 16 MiB         = 64 MiB → budget raised to 64 MiB
	// re-cap = min(64, 128)       = 64 MiB
	assert.Equal(t, int64(64)*_1MiB, got)
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
	dc.bufs = make(chan *[]byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- &b

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
		client:        mock,
		chunkSize:     16 * 1024 * 1024,
		bypassOnError: true,
	}
	dc.SetName(compName)
	dc.SetNextComponent(next)
	// 1-buffer pool forces the second populate to reuse the first's buffer.
	dc.bufs = make(chan *[]byte, 1)
	b := make([]byte, dc.chunkSize)
	dc.bufs <- &b

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
