// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dist_cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	dcache "github.com/nearora-msft/dist-cache-client-go"
)

const compName = "dist_cache"

// readUploadTimeout bounds a single async L2 populate goroutine kicked off from the read path.
const readUploadTimeout = 60 * time.Second

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
	Port           int    `config:"port"              yaml:"port,omitempty"`
	TTLSeconds     uint32 `config:"ttl-seconds"       yaml:"ttl-seconds,omitempty"`
	BypassOnError  bool   `config:"bypass-on-error"   yaml:"bypass-on-error,omitempty"`
	CachePrefix    string `config:"cache-prefix"      yaml:"cache-prefix,omitempty"`
	MaxConnsPerSvr int    `config:"max-conns-per-server" yaml:"max-conns-per-server,omitempty"`

	// Chunk size for distributed cache operations. When block_cache is present,
	// this is overridden by block_cache.block-size-mb to keep alignment consistent.
	ChunkSizeMB float64 `config:"chunk-size-mb" yaml:"chunk-size-mb,omitempty"`
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

	// chunkPool recycles chunk-sized byte buffers used on the read hot path to
	// avoid per-operation allocations. Buffers returned from getBuf have
	// capacity == chunkSize; callers slice down to the length they need and
	// must return the buffer via putBuf when ownership ends. Requests larger
	// than chunkSize bypass the pool.
	chunkPool sync.Pool
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

func init() {
	internal.AddComponent(compName, NewDistCacheComponent)

	discoveryFlag := config.AddStringFlag("dist-cache-discovery-url", "",
		"distributed cache discovery endpoint (recommended)")
	config.BindPFlag(compName+".discovery-url", discoveryFlag)

	serverListFlag := config.AddStringFlag("dist-cache-server-list", "",
		"comma-separated list of distributed cache server addresses (fallback)")
	config.BindPFlag(compName+".server-list", serverListFlag)

	// Support DIST_CACHE_SERVER_LIST env var
	config.BindEnv(compName+".server-list", "DIST_CACHE_SERVER_LIST")
}

func (dc *DistCache) Configure(isParent bool) error {
	log.Trace("DistCache::Configure")

	conf := DistCacheOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("DistCache: config error [invalid config attributes]")
		return fmt.Errorf("dist_cache: config error: %w", err)
	}

	// Validate that at least one server discovery method is configured
	if conf.DiscoveryURL == "" && conf.K8sService == "" && conf.ServerList == "" {
		if os.Getenv("DIST_CACHE_SERVER_LIST") == "" {
			return fmt.Errorf("dist_cache: no server discovery configured (set discovery-url, k8s-service, server-list, or DIST_CACHE_SERVER_LIST)")
		}
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
	if conf.ServerList != "" || os.Getenv("DIST_CACHE_SERVER_LIST") != "" {
		configured = append(configured, "server-list")
	}
	if len(configured) > 1 {
		log.Warn("DistCache::Configure : multiple discovery methods configured (%s); precedence is discovery-url > k8s DNS > server-list, lower-precedence entries will only be used as a fallback",
			strings.Join(configured, ", "))
	}

	dc.conf = conf
	dc.bypassOnError = conf.BypassOnError

	// Resolve cache prefix. Explicit dist_cache.cache-prefix wins; otherwise
	// derive from azstorage.account-name/azstorage.container.
	if conf.CachePrefix != "" {
		dc.cachePrefix = conf.CachePrefix
		log.Info("DistCache::Configure : cache-prefix=%s (from explicit config)", dc.cachePrefix)
	} else {
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
			return fmt.Errorf("dist_cache: cache prefix unresolved; set dist_cache.cache-prefix or both azstorage.account-name and azstorage.container")
		}
		dc.cachePrefix = accountName + "/" + container
		log.Info("DistCache::Configure : cache-prefix=%s (derived from azstorage account/container)", dc.cachePrefix)
	}

	// Resolve chunk size: block_cache.block-size-mb > stream.block-size-mb > dist_cache.chunk-size-mb > default
	const defaultBlockSizeMB = 16
	var blockSizeMB float64 = defaultBlockSizeMB
	if config.IsSet("block_cache.block-size-mb") {
		err = config.UnmarshalKey("block_cache.block-size-mb", &blockSizeMB)
		if err != nil {
			log.Warn("DistCache::Configure : Failed to read block-size-mb, using default %d MB", defaultBlockSizeMB)
			blockSizeMB = defaultBlockSizeMB
		}
	} else if config.IsSet("stream.block-size-mb") {
		err = config.UnmarshalKey("stream.block-size-mb", &blockSizeMB)
		if err != nil {
			blockSizeMB = defaultBlockSizeMB
		}
	} else if conf.ChunkSizeMB > 0 {
		blockSizeMB = conf.ChunkSizeMB
	}
	dc.chunkSize = int64(blockSizeMB * 1024 * 1024)

	// Initialize the buffer pool sized to the resolved chunk size. New buffers
	// are allocated at full chunkSize capacity so they can be reused for any
	// request up to that size without re-allocation.
	chunkSize := dc.chunkSize
	dc.chunkPool.New = func() any {
		buf := make([]byte, chunkSize)
		return &buf
	}

	log.Info("DistCache::Configure : chunk-size=%d, bypass-on-error=%v",
		dc.chunkSize, dc.bypassOnError)

	return nil
}

func (dc *DistCache) Start(ctx context.Context) error {
	log.Trace("Starting component : %s", dc.Name())

	// Build client options
	opts := []dcache.Option{
		dcache.WithChunkSize(dc.chunkSize),
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
	if dc.conf.MaxConnsPerSvr > 0 {
		opts = append(opts, dcache.WithMaxConnsPerServer(dc.conf.MaxConnsPerSvr))
	}
	if dc.conf.DiscoveryRefreshSec > 0 {
		opts = append(opts, dcache.WithDiscoveryRefresh(
			time.Duration(dc.conf.DiscoveryRefreshSec)*time.Second))
	}

	client, err := dcache.New(opts...)
	if err != nil {
		if dc.bypassOnError {
			log.Warn("DistCache::Start : Failed to connect to distributed cache, bypassing: %v", err)
			return nil
		}
		return fmt.Errorf("dist_cache: failed to start: %w", err)
	}

	dc.client = client
	log.Info("DistCache::Start : connected to distributed cache cluster")

	return nil
}

func (dc *DistCache) Stop() error {
	log.Trace("Stopping component : %s", dc.Name())
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
	if dc.client == nil {
		return dc.NextComponent().ReadInBuffer(options)
	}

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
			dataCopy := dc.getBuf(n)
			copy(dataCopy, options.Data[:n])
			uploadCtx, cancel := context.WithTimeout(context.Background(), readUploadTimeout)
			go func() {
				defer cancel()
				dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
			}()
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
		dataCopy := dc.getBuf(n)
		copy(dataCopy, options.Data[:n])
		uploadCtx, cancel := context.WithTimeout(context.Background(), readUploadTimeout)
		go func() {
			defer cancel()
			dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
		}()
		return n, nil
	}

	if err == dcache.ErrNotFoundAlreadyLocked {
		// Another node is fetching this chunk — poll until cached
		log.Debug("DistCache::ReadInBuffer : L2 miss (locked) %s offset=%d, polling", name, options.Offset)
		n, pollErr := dc.pollChunkIntoBuffer(ctx, name, etag, options.Offset, options.Data)
		if pollErr == nil {
			return n, nil
		}
		// Poll timed out — fall through to Azure
		log.Debug("DistCache::ReadInBuffer : chunk poll timeout %s offset=%d, falling through", name, options.Offset)
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		dataCopy := dc.getBuf(n)
		copy(dataCopy, options.Data[:n])
		uploadCtx, cancel := context.WithTimeout(context.Background(), readUploadTimeout)
		go func() {
			defer cancel()
			dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
		}()
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

	if dc.bypassOnError {
		log.Warn("DistCache::ReadInBuffer : error, bypassing: %v", err)
		return dc.NextComponent().ReadInBuffer(options)
	}
	return 0, err
}

// --- Internal helpers ---

// pollChunkIntoBuffer waits for a single chunk to become available in the
// distributed cache and copies it into buf. Returns the number of bytes read.
func (dc *DistCache) pollChunkIntoBuffer(ctx context.Context, name, etag string, offset int64, buf []byte) (int, error) {
	const (
		maxPollDuration = 10 * time.Second
		maxBackoff      = 2 * time.Second
	)

	deadline := time.Now().Add(maxPollDuration)
	backoff := 200 * time.Millisecond

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(backoff):
		}

		n, err := dc.client.DownloadChunk(ctx, name, etag, offset, buf)
		if err == nil {
			return n, nil
		}

		if err != dcache.ErrNotFoundAlreadyLocked && err != dcache.ErrNotFound {
			return 0, err
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return 0, fmt.Errorf("dist_cache: chunk poll timeout for %s offset=%d", name, offset)
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

// getBuf returns a byte slice of length n backed by a pooled chunk buffer.
// If n exceeds chunkSize, or the pool has not been initialized (e.g. in
// tests that construct DistCache directly), a fresh allocation is used
// instead and putBuf becomes a no-op for that slice. Callers must not
// grow the returned slice beyond its capacity.
func (dc *DistCache) getBuf(n int) []byte {
	if n <= 0 {
		return nil
	}
	if dc.chunkSize == 0 || int64(n) > dc.chunkSize {
		return make([]byte, n)
	}
	v := dc.chunkPool.Get()
	if v == nil {
		return make([]byte, n)
	}
	bufp := v.(*[]byte)
	return (*bufp)[:n]
}

// putBuf returns a buffer previously obtained from getBuf to the pool.
// Safe to call on nil or on slices whose capacity does not match chunkSize
// (over-sized fallbacks) — those are simply dropped for the GC.
func (dc *DistCache) putBuf(b []byte) {
	if b == nil || dc.chunkSize == 0 {
		return
	}
	if int64(cap(b)) != dc.chunkSize {
		return
	}
	buf := b[:cap(b)]
	dc.chunkPool.Put(&buf)
}

func (dc *DistCache) uploadChunkAsync(ctx context.Context, name, etag string, offset int64, data []byte) {
	defer dc.putBuf(data)

	// Respect cancellation from write path
	select {
	case <-ctx.Done():
		return
	default:
	}

	gid := fileGroupID(name, etag)
	log.Debug("DistCache::uploadChunkAsync : uploading chunk %s offset=%d with group %q", name, offset, string(gid))
	opts := []dcache.UploadOption{
		dcache.WithIgnoreLock(true),
		dcache.WithGroupID(gid),
		dcache.WithMetadata(map[string][]byte{"gid": gid}),
	}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.UploadChunk(ctx, name, etag, offset, data, opts...); err != nil {
		if ctx.Err() != nil {
			log.Debug("DistCache::uploadChunkAsync : cancelled for %s offset=%d", name, offset)
			return
		}
		log.Warn("DistCache::uploadChunkAsync : upload failed: %v", err)
	}
}
