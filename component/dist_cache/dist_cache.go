// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dist_cache

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	dcache "github.com/nearora-msft/dist-cache-client-go"
)

const compName = "dist_cache"

// errPollCorruptHit signals that pollChunkIntoBuffer observed a zero-byte
// cache entry. The caller should treat it like a timeout: fetch from Azure
// but skip populate (we don't hold the miss-lock).
var errPollCorruptHit = errors.New("dist_cache: poll saw zero-byte cache entry")

// errPollChecksumMismatch signals a checksum failure during polling. The
// caller decides whether to fall through (bypass-on-error) or surface EIO.
var errPollChecksumMismatch = errors.New("dist_cache: poll saw checksum mismatch")

// Async upload memory budget policy. See resolveMemBudget for how these combine.
const (
	// distCacheSharePct is the share of block_cache's reference memory
	// budget reserved for the async upload buffer pool. The reference
	// budget is block_cache.mem-size-mb when set, else
	// block_cache.MemShareFraction × free RAM (what block_cache itself
	// would grab by default). Taking a slice of block_cache's footprint
	// keeps the two components from independently laying claim to the
	// same RAM.
	distCacheSharePct = 10

	// distCacheMinBuffers is the enable threshold for the async upload pool.
	// If the fair-share memory signal is below distCacheMinBuffers × chunkSize,
	// resolveMemBudget returns 0 and async populate stays disabled for the
	// mount. Below this size the pool doesn't pay for itself: schedulePopulate
	// drops on exhaustion, so any small burst of misses or one stuck upload
	// collapses the L2 hit rate. Four buffers is tiny in absolute terms
	// (e.g. 4 × 16 MiB = 64 MiB at the default block size); hosts that can't
	// spare that are better off skipping the pool than forcing the allocation.
	distCacheMinBuffers = 4

	// distCacheDemandMultiplier scales the arrival-side concurrency
	// (block_cache.parallelism) into an expected in-flight count for the
	// async upload pool.
	// A multiplier of 4 tolerates upload latency up to ~4× the azstorage
	// read latency without dropping populates, while keeping the ceiling
	// meaningful on large-memory hosts (where fair-share alone would over-
	// provision — e.g. 128 GiB free RAM ⇒ ~10 GiB pool if uncapped).
	distCacheDemandMultiplier = 4

	_1MiB = 1024 * 1024
)

// Environment variables recognized by dist_cache. Only server-discovery
// keys are exposed; behavioral tuning stays YAML/CLI-only, matching the
// identity-vs-tuning split used by azstorage.
const (
	EnvDistCacheDiscoveryURL = "DIST_CACHE_DISCOVERY_URL"
	EnvDistCacheK8sService   = "DIST_CACHE_K8S_SERVICE"
	EnvDistCacheK8sNamespace = "DIST_CACHE_K8S_NAMESPACE"
	EnvDistCacheServerList   = "DIST_CACHE_SERVER_LIST"
)

// RegisterEnvVariables binds dist_cache discovery keys to env vars.
// Precedence via viper: CLI flag > env > YAML > default.
func RegisterEnvVariables() {
	config.BindEnv(compName+".discovery-url", EnvDistCacheDiscoveryURL)
	config.BindEnv(compName+".k8s-service", EnvDistCacheK8sService)
	config.BindEnv(compName+".k8s-namespace", EnvDistCacheK8sNamespace)
	config.BindEnv(compName+".server-list", EnvDistCacheServerList)
}

// DistCacheOptions holds configuration for the distributed cache component.
type DistCacheOptions struct {
	// Discovery (preferred — auto-detects servers)
	DiscoveryURL        string `config:"discovery-url"        yaml:"discovery-url,omitempty"`
	DiscoveryRefreshSec int    `config:"discovery-refresh-sec" yaml:"discovery-refresh-sec,omitempty"`

	// Kubernetes DNS discovery
	K8sService   string `config:"k8s-service"   yaml:"k8s-service,omitempty"`
	K8sNamespace string `config:"k8s-namespace" yaml:"k8s-namespace,omitempty"`

	// Static fallback
	ServerList string `config:"server-list" yaml:"server-list,omitempty"`

	// Common options
	Port       int    `config:"port"        yaml:"port,omitempty"`        // Default 9065
	TTLSeconds uint32 `config:"ttl-seconds" yaml:"ttl-seconds,omitempty"` // Default 0 (no TTL)

	// Per-chunk CRC verification on the L2 client. Default true.
	VerifyChecksum bool `config:"verify-checksum" yaml:"verify-checksum,omitempty"`

	// Internal knob, no YAML/CLI surface. Wire up later if a caller needs it.
	BypassOnError bool `config:"bypass-on-error" yaml:"-"` //Default true (skip L2 on client error)
}

// DistCache is the blobfuse component that sits between the local cache and azstorage,
// providing a shared distributed cache layer across nodes.
type DistCache struct {
	internal.BaseComponent
	conf   DistCacheOptions
	client dcacheClient

	chunkSize     int64
	cachePrefix   string
	bypassOnError bool

	// Bounded async-upload buffer pool. Preallocated in Configure and never
	// grown. cap(bufs) * chunkSize is the hard ceiling on memory dist_cache
	// holds for in-flight L2 populates. Nil means async populate was disabled
	// (resolveMemBudget returned 0); ReadInBuffer then runs as passthrough.
	bufs chan *[]byte

	// inflight tracks in-flight upload goroutines so Stop can wait for them
	// to finish before closing the dcache client.
	inflight sync.WaitGroup
}

// dcacheClient abstracts the distributed cache client for testing.
type dcacheClient interface {
	DownloadChunk(ctx context.Context, filename, etag string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	UploadChunk(ctx context.Context, filename, etag string, offset int64, data []byte, opts ...dcache.UploadOption) error
	Close() error
}

// Verify interface compliance.
var _ internal.Component = &DistCache{}

func NewDistCacheComponent() internal.Component {
	comp := &DistCache{}
	comp.SetName(compName)
	return comp
}

func (dc *DistCache) Configure(isParent bool) error {
	log.Trace("DistCache::Configure")

	conf := DistCacheOptions{BypassOnError: true, VerifyChecksum: true}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("DistCache: config error [invalid config attributes]")
		return fmt.Errorf("dist_cache: config error: %w", err)
	}

	// At least one discovery method must be set (YAML, CLI flag, or env).
	if conf.DiscoveryURL == "" && conf.K8sService == "" && conf.ServerList == "" {
		return fmt.Errorf("dist_cache: no server discovery configured (set discovery-url, k8s-service, or server-list)")
	}

	// Warn if multiple discovery methods are configured. The dcache client
	// applies them in precedence order: discovery-url > k8s DNS > server-list;
	// lower-precedence entries are effectively ignored.
	var configured []string
	if conf.DiscoveryURL != "" {
		configured = append(configured, "discovery-url")
	}
	if conf.K8sService != "" {
		configured = append(configured, "k8s-service")
	}
	if conf.ServerList != "" {
		configured = append(configured, "server-list")
	}
	if len(configured) > 1 {
		log.Warn("DistCache::Configure : multiple discovery methods configured (%s); precedence is discovery-url > k8s DNS > server-list, lower-precedence entries will only be used as a fallback",
			strings.Join(configured, ", "))
	}

	dc.conf = conf
	dc.bypassOnError = conf.BypassOnError

	// Derive the cache namespace from the storage identity so mounts for
	// different accounts or containers cannot collide.
	var accountName, container string
	if config.IsSet("azstorage.account-name") {
		if err := config.UnmarshalKey("azstorage.account-name", &accountName); err != nil {
			return fmt.Errorf("dist_cache: failed to read azstorage.account-name: %w", err)
		}
	}
	if config.IsSet("azstorage.container") {
		if err := config.UnmarshalKey("azstorage.container", &container); err != nil {
			return fmt.Errorf("dist_cache: failed to read azstorage.container: %w", err)
		}
	}
	if accountName == "" || container == "" {
		return fmt.Errorf("dist_cache: cache prefix unresolved; set both azstorage.account-name and azstorage.container")
	}
	dc.cachePrefix = accountName + "/" + container
	log.Info("DistCache::Configure : cache-prefix=%s (derived from azstorage account/container)", dc.cachePrefix)

	// L1 (block_cache) and L2 (dist_cache) must align on chunk size.
	// block_cache.block-size-mb is the single source — either set directly
	// by the user or fanned out from dist_cache.block-size-mb by
	// normalizeDistCacheConfig at mount time.
	var blockSizeMB float64 = common.DefaultBlockSize
	if config.IsSet("block_cache.block-size-mb") {
		if err = config.UnmarshalKey("block_cache.block-size-mb", &blockSizeMB); err != nil {
			log.Warn("DistCache::Configure : Failed to read block-size-mb, using default %d MB", common.DefaultBlockSize)
			blockSizeMB = common.DefaultBlockSize
		}
	}
	dc.chunkSize = int64(blockSizeMB * _1MiB)

	// Resolve the async-upload memory budget and preallocate the buffer pool.
	// Budget 0 means async populate is disabled; ReadInBuffer still services
	// L2 lookups and falls through to azstorage on miss.
	budget := dc.resolveMemBudget()
	if budget > 0 {
		numBuffers := int(budget / dc.chunkSize)
		dc.bufs = make(chan *[]byte, numBuffers)
		for i := 0; i < numBuffers; i++ {
			b := make([]byte, dc.chunkSize)
			dc.bufs <- &b
		}
		log.Info("DistCache::Configure : async upload pool = %d buffers × %d bytes = %d MiB",
			numBuffers, dc.chunkSize, int64(numBuffers)*dc.chunkSize/_1MiB)
	} else {
		log.Warn("DistCache::Configure : async upload disabled; L2 populate skipped, reads run as passthrough")
	}

	log.Info("DistCache::Configure : chunk-size=%d", dc.chunkSize)
	log.Info("DistCache::Configure : discovery-url=%s", dc.conf.DiscoveryURL)
	log.Info("DistCache::Configure : discovery-refresh-sec=%d", dc.conf.DiscoveryRefreshSec)
	log.Info("DistCache::Configure : k8s-service=%s", dc.conf.K8sService)
	log.Info("DistCache::Configure : k8s-namespace=%s", dc.conf.K8sNamespace)
	log.Info("DistCache::Configure : server-list=%s", dc.conf.ServerList)
	log.Info("DistCache::Configure : port=%d", dc.conf.Port)
	log.Info("DistCache::Configure : ttl-seconds=%d", dc.conf.TTLSeconds)
	log.Info("DistCache::Configure : verify-checksum=%t", dc.conf.VerifyChecksum)
	log.Info("DistCache::Configure : bypass-on-error=%t", dc.bypassOnError)

	return nil
}

func (dc *DistCache) Start(ctx context.Context) error {
	log.Trace("Starting component : %s", dc.Name())

	// Build client options
	opts := []dcache.Option{
		dcache.WithChunkSize(dc.chunkSize),
	}

	if dc.conf.VerifyChecksum {
		// Per-chunk CRC on upload, validated on download; mismatches
		// surface as ErrChecksumMismatch and fall through to azstorage.
		opts = append(opts, dcache.WithChecksumVerification(true))
	}

	if dc.conf.DiscoveryURL != "" {
		opts = append(opts, dcache.WithDiscoveryURL(dc.conf.DiscoveryURL))
	}
	if dc.conf.K8sService != "" && dc.conf.K8sNamespace != "" {
		opts = append(opts, dcache.WithK8sDiscovery(dc.conf.K8sService, dc.conf.K8sNamespace))
	}
	if dc.conf.ServerList != "" {
		servers := strings.Split(dc.conf.ServerList, ",")
		for i := range servers {
			servers[i] = strings.TrimSpace(servers[i])
		}
		opts = append(opts, dcache.WithServerList(servers))
	}
	if dc.conf.Port > 0 {
		opts = append(opts, dcache.WithPort(dc.conf.Port))
	}
	opts = append(opts, dcache.WithCachePrefix(dc.cachePrefix))
	if dc.conf.DiscoveryRefreshSec > 0 {
		opts = append(opts, dcache.WithDiscoveryRefresh(
			time.Duration(dc.conf.DiscoveryRefreshSec)*time.Second))
	}

	client, err := dcache.New(opts...)
	if err != nil {
		log.Err("DistCache::Start : Failed to connect to distributed cache: %v", err)
		return fmt.Errorf("dist_cache: failed to start: %w", err)
	}

	dc.client = client
	log.Info("DistCache::Start : connected to distributed cache cluster")

	return nil
}

func (dc *DistCache) Stop() error {
	log.Trace("Stopping component : %s", dc.Name())
	// Wait for any in-flight populates to finish before closing the client so
	// they don't observe a nil/closed client mid-upload.
	dc.inflight.Wait()
	if dc.client != nil {
		return dc.client.Close()
	}
	return nil
}

func (dc *DistCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// --- Read path ---

// resolveReadPath returns the file path for a ReadInBuffer call. block_cache
// sets Handle but not Path.
func resolveReadPath(options *internal.ReadInBufferOptions) string {
	if options.Handle != nil {
		return options.Handle.Path
	}
	return options.Path
}

func (dc *DistCache) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	name := resolveReadPath(options)

	// Resolve ETag from handle (pinned at open time by block_cache)
	etag := resolveETag(options)
	log.Debug("DistCache::ReadInBuffer : %s offset=%d etag=%q", name, options.Offset, etag)

	ctx := context.Background()

	n, err := dc.client.DownloadChunk(ctx, name, etag, options.Offset, options.Data,
		dcache.WithLock(true))
	if err == nil && n > 0 {
		log.Debug("DistCache::ReadInBuffer : L2 hit %s offset=%d etag=%q", name, options.Offset, etag)
		return n, nil
	}
	if err == nil && n == 0 {
		// Zero-byte hit means corrupt/empty cache entry — treat as miss
		log.Warn("DistCache::ReadInBuffer : L2 zero-byte hit %s offset=%d, falling through to storage", name, options.Offset)
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		if n > 0 {
			dc.schedulePopulate(name, etag, options.Offset, options.Data[:n])
		}
		return n, nil
	}

	if err == dcache.ErrNotFoundGotLock {
		// We own this chunk's miss — download from Azure and populate cache
		log.Debug("DistCache::ReadInBuffer : L2 miss (got lock) %s offset=%d", name, options.Offset)
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		if n > 0 {
			dc.schedulePopulate(name, etag, options.Offset, options.Data[:n])
		}
		return n, nil
	}

	if err == dcache.ErrNotFoundAlreadyLocked {
		// Poll until L2 hit, inherited lock, or timeout.
		log.Debug("DistCache::ReadInBuffer : L2 miss (locked) %s offset=%d, polling", name, options.Offset)
		n, pollErr := dc.pollChunkIntoBuffer(ctx, name, etag, options.Offset, options.Data)
		if pollErr == nil {
			return n, nil // L2 hit during poll
		}
		if pollErr == errPollChecksumMismatch && !dc.bypassOnError {
			log.Err("DistCache::ReadInBuffer : L2 checksum mismatch during poll %s offset=%d, returning EIO", name, options.Offset)
			return 0, syscall.EIO
		}
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		if pollErr == dcache.ErrNotFoundGotLock && n > 0 {
			// Inherited the miss-lock mid-poll; safe to populate.
			dc.schedulePopulate(name, etag, options.Offset, options.Data[:n])
		}
		// Timeout (or other poll error): peer still owns the lock; skip
		// populate to avoid racing its write.
		return n, nil
	}

	if err == dcache.ErrNotFound {
		// L2 miss without lock semantics — read from Azure but do NOT populate.
		// Populating here would cause a thundering herd: every reader that
		// races on a cold chunk would also write. Cache population is reserved
		// for the client that explicitly holds the miss-lock (ErrNotFoundGotLock).
		log.Debug("DistCache::ReadInBuffer : L2 miss %s offset=%d", name, options.Offset)
		return dc.NextComponent().ReadInBuffer(options)
	}

	if errors.Is(err, dcache.ErrChecksumMismatch) {
		// Corrupt L2 chunk. Populate would race the peer's write, so
		// always skip it; bypass-on-error decides whether we mask the
		// corruption by falling through or surface it as EIO.
		if dc.bypassOnError {
			log.Warn("DistCache::ReadInBuffer : L2 checksum mismatch %s offset=%d, falling through to storage: %v", name, options.Offset, err)
			return dc.NextComponent().ReadInBuffer(options)
		}
		log.Err("DistCache::ReadInBuffer : L2 checksum mismatch %s offset=%d, returning EIO: %v", name, options.Offset, err)
		return 0, syscall.EIO
	}

	if dc.bypassOnError {
		log.Warn("DistCache::ReadInBuffer : error, bypassing: %v", err)
		return dc.NextComponent().ReadInBuffer(options)
	}
	return 0, err
}

// --- Internal helpers ---

// pollChunkIntoBuffer waits for a chunk to land in the cache. Polls with
// WithLock(true) so we can inherit the miss-lock if the peer's expires.
// Returns (n, nil) on L2 hit, (0, ErrNotFoundGotLock) if we inherited the
// lock (caller must fetch and populate), (0, timeout err) if the peer still
// holds the lock at deadline (caller should fetch without populating), or
// (0, err) on other client errors.
func (dc *DistCache) pollChunkIntoBuffer(ctx context.Context, name, etag string, offset int64, buf []byte) (int, error) {
	const (
		maxPollDuration = 3 * time.Second
		maxBackoff      = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(ctx, maxPollDuration)
	defer cancel()

	backoff := 200 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("dist_cache: chunk poll timeout for %s offset=%d: %w", name, offset, ctx.Err())
		case <-time.After(backoff):
		}

		n, err := dc.client.DownloadChunk(ctx, name, etag, offset, buf, dcache.WithLock(true))
		switch {
		case err == nil:
			if n == 0 {
				// Corrupt/empty cache entry — treat like a miss but
				// skip populate since we don't hold the lock.
				log.Warn("DistCache::pollChunkIntoBuffer : L2 zero-byte hit %s offset=%d, falling through to storage", name, offset)
				return 0, errPollCorruptHit
			}
			return n, nil // L2 hit
		case err == dcache.ErrNotFoundGotLock:
			// Peer's lock expired; we now own the miss. No server-side
			// release API, so a dropped populate leaks the lock until TTL.
			return 0, dcache.ErrNotFoundGotLock
		case errors.Is(err, dcache.ErrChecksumMismatch):
			// Peer wrote a corrupt chunk; caller decides fall-through vs EIO.
			log.Warn("DistCache::pollChunkIntoBuffer : L2 checksum mismatch %s offset=%d: %v", name, offset, err)
			return 0, errPollChecksumMismatch
		case err == dcache.ErrNotFoundAlreadyLocked, err == dcache.ErrNotFound:
			// Keep waiting.
		default:
			return 0, err
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// fileGroupID returns a versioned group ID for a file. All chunks uploaded in
// the same version share this ID. The etag is the Azure blob ETag for the file
// revision, ensuring that DeleteGroup for an old version cannot affect chunks
// uploaded under a new version.
func fileGroupID(name string, etag string) []byte {
	return []byte(fmt.Sprintf("%s\x00v%s", name, etag))
}

// resolveETag extracts the ETag from a ReadInBufferOptions, preferring the
// handle's stored value (pinned at open time by block_cache).
func resolveETag(options *internal.ReadInBufferOptions) string {
	if options.Etag != nil && *options.Etag != "" {
		return *options.Etag
	}
	if options.Handle != nil {
		if v, ok := options.Handle.GetValue("ETAG"); ok {
			if etag, ok := v.(string); ok && etag != "" {
				return etag
			}
		}
	}
	log.Debug("DistCache::resolveETag : no etag found (Etag field nil/empty, handle missing or no ETAG key)")
	return ""
}

// getBlockCacheWorkers returns the number of concurrent downloads block_cache
// is configured to run (block_cache.parallelism). This is the true ceiling on
// concurrent callers into dist_cache: FUSE threads enqueue work onto
// block_cache's thread pool and then block on a channel — they do not call
// down into dist_cache themselves. Prefetches share the same pool, so they
// don't raise the ceiling either.
//
// Mirrors block_cache's own default of 3 * runtime.NumCPU() when the knob is
// unset. Read via config rather than reaching into block_cache's state so we
// don't depend on component start ordering.
func getBlockCacheWorkers() int {
	if config.IsSet("block_cache.parallelism") {
		var n uint32
		if err := config.UnmarshalKey("block_cache.parallelism", &n); err == nil && n > 0 {
			return int(n)
		}
	}
	return 3 * runtime.NumCPU()
}

// resolveMemBudget returns the total bytes dist_cache will preallocate for its
// async-upload buffer pool. Zero means "async populate disabled" (caller
// should skip buffer allocation and treat the L2 populate as a no-op).
//
// dist_cache does not expose its own mem-size-mb knob. It sizes the pool as a
// fraction of block_cache's reference budget so the two caches stay
// coordinated under one memory budget:
//
//  1. Establish block_cache's reference budget (bcRef):
//     - block_cache.mem-size-mb when set (explicit user sizing), else
//     - block_cache.MemShareFraction × free RAM (what block_cache would
//     itself grab by default).
//  2. Pick min(fair-share, demand-ceiling), where
//     fair-share    = distCacheSharePct% of bcRef,
//     demand-ceiling = distCacheDemandMultiplier × block_cache.parallelism ×
//     chunkSize. block_cache's worker pool bounds the *arrival* rate of
//     new populates (FUSE threads enqueue and block, they don't reach
//     dist_cache), but populates are fire-and-forget goroutines, so
//     in-flight uploads scale with upload/read latency ratio.
//     The multiplier turns arrival concurrency into
//     expected in-flight capacity. dist_cache is enforced to sit below
//     block_cache in the pipeline (see common.ValidatePipeline), so
//     block_cache.parallelism is always available.
//  3. If fair-share < distCacheMinBuffers × chunkSize, return 0 (async
//     populate disabled). See distCacheMinBuffers for the rationale.
//     Otherwise both fair and demand are ≥ floor, so min(fair, demand) is
//     a valid budget with no further clamping.
func (dc *DistCache) resolveMemBudget() int64 {
	var bcRef int64
	if config.IsSet("block_cache.mem-size-mb") {
		var n uint64
		if err := config.UnmarshalKey("block_cache.mem-size-mb", &n); err == nil && n > 0 {
			bcRef = int64(n) * _1MiB
		}
	}
	if bcRef == 0 {
		free, err := common.GetAvailableMemoryBytes()
		if err != nil {
			free = common.DefaultMemFallbackBytes
			log.Warn("DistCache::resolveMemBudget : failed to read available memory [%s], using %d MiB fallback", err.Error(), free/_1MiB)
		}
		bcRef = int64(common.DefaultMemShareFraction * float64(free))
	}
	demand := int64(distCacheDemandMultiplier) * int64(getBlockCacheWorkers()) * dc.chunkSize
	fair := bcRef * distCacheSharePct / 100
	floor := int64(distCacheMinBuffers) * dc.chunkSize

	if fair < floor {
		log.Warn("DistCache::resolveMemBudget : fair-share %d < floor %d (chunk=%d, bcRef=%d); async populate disabled",
			fair, floor, dc.chunkSize, bcRef)
		return 0
	}
	return min(fair, demand)
}

// schedulePopulate spawns an async upload for the given chunk. It never
// blocks: if the buffer pool is exhausted, the populate is dropped and
// logged. This is intentional — read latency must not depend on dcache
// cluster health. Memory is bounded by cap(dc.bufs) since every in-flight
// upload holds exactly one buffer for its lifetime.
//
// Drop caveat: the caller implicitly holds the miss-lock, and dcache has
// no lock-release API. A dropped populate leaks the lock until its
// server-side TTL expires; other readers polling this chunk will either
// inherit it on TTL (ErrNotFoundGotLock) or fall through to Azure on
// their own poll deadline. Impact is bounded by min(lock TTL, poll
// deadline). Mitigation: grow the fair-share cap in resolveMemBudget.
// Drops are logged at Warn.
func (dc *DistCache) schedulePopulate(name, etag string, offset int64, src []byte) {
	if dc.bufs == nil {
		return // async populate disabled at Configure time
	}
	if int64(len(src)) > dc.chunkSize {
		log.Warn("DistCache::schedulePopulate : payload %d > chunk size %d, dropping populate for %s offset=%d",
			len(src), dc.chunkSize, name, offset)
		return
	}

	var buf *[]byte
	select {
	case buf = <-dc.bufs:
	default:
		log.Warn("DistCache::schedulePopulate : buffer pool exhausted, dropping populate for %s offset=%d; miss-lock will linger until TTL", name, offset)
		return
	}

	*buf = (*buf)[:len(src)]
	copy(*buf, src)

	dc.inflight.Add(1)
	go dc.doUpload(name, etag, offset, buf, len(src))
}

// doUpload runs a single populate. It always returns the borrowed buffer to
// the pool, even on error. The per-request timeout is enforced by the dcache
// client (see WithRequestTimeout; default 30s), which applies a socket-level
// deadline covering both send and receive.
func (dc *DistCache) doUpload(name, etag string, offset int64, buf *[]byte, length int) {
	defer dc.inflight.Done()
	defer func() { dc.bufs <- buf }()

	gid := fileGroupID(name, etag)
	log.Debug("DistCache::doUpload : uploading chunk %s offset=%d with group %q", name, offset, string(gid))
	opts := []dcache.UploadOption{
		dcache.WithIgnoreLock(true),
		dcache.WithGroupID(gid),
		dcache.WithMetadata(map[string][]byte{"gid": gid}),
	}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	data := (*buf)[:length]
	if err := dc.client.UploadChunk(context.Background(), name, etag, offset, data, opts...); err != nil {
		log.Warn("DistCache::doUpload : upload failed: %v", err)
	}
}

func init() {
	internal.AddComponent(compName, NewDistCacheComponent)

	discoveryFlag := config.AddStringFlag("dist-cache-discovery-url", "",
		"distributed cache discovery endpoint (recommended)")
	config.BindPFlag(compName+".discovery-url", discoveryFlag)

	serverListFlag := config.AddStringFlag("dist-cache-server-list", "",
		"comma-separated list of distributed cache server addresses (fallback)")
	config.BindPFlag(compName+".server-list", serverListFlag)

	RegisterEnvVariables()
}
