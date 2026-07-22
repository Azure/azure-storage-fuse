// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dist_cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	dcache "github.com/nearora-msft/dist-cache-client-go"
	"golang.org/x/sync/errgroup"
)

const compName = "dist_cache"

// maxParallelChunkOps limits the number of concurrent chunk-level recovery
// operations (Azure fetches and cache polls) during CopyToFile.
const maxParallelChunkOps = 8

// maxPendingL2Uploads limits the number of concurrent L2 cache uploads when
// flushing pending chunks at commit time.
const maxPendingL2Uploads = 8

// pendingWriteTTL is the maximum time pending chunks are held before being
// evicted. This handles abandoned writes (e.g., process crash before commit,
// lazy-write with long-lived handles).
const pendingWriteTTL = 5 * time.Minute

// pendingCleanupInterval is how often the background goroutine scans for
// expired pending entries.
const pendingCleanupInterval = 30 * time.Second

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
	MaxFileSizeMB  int    `config:"max-file-size-mb"  yaml:"max-file-size-mb,omitempty"`
	BypassOnError  bool   `config:"bypass-on-error"   yaml:"bypass-on-error,omitempty"`
	CachePrefix    string `config:"cache-prefix"      yaml:"cache-prefix,omitempty"`
	MaxConnsPerSvr int    `config:"max-conns-per-server" yaml:"max-conns-per-server,omitempty"`

	// Chunk size for distributed cache operations. When block_cache is present,
	// this is overridden by block_cache.block-size-mb to keep alignment consistent.
	// When used with file_cache (no block_cache), this is the primary chunk size config.
	ChunkSizeMB float64 `config:"chunk-size-mb" yaml:"chunk-size-mb,omitempty"`
}

// pendingChunk holds a staged chunk's data until the file is committed.
type pendingChunk struct {
	offset int64
	data   []byte
}

// pendingFile tracks buffered chunks for a single file along with metadata
// for size-cap and TTL-based eviction.
type pendingFile struct {
	chunks       []pendingChunk
	totalSize    int64     // sum of len(chunk.data) for all chunks
	lastActivity time.Time // updated on each StageData; used for TTL eviction
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

	// dirtyFiles tracks recently invalidated files to avoid serving stale data.
	// After a delete/truncate/rename, the file is added here. Reads for files
	// in this set bypass dist_cache until the TTL expires, giving the server
	// time to process the async group-based deletion.
	dirtyMu    sync.Mutex
	dirtyFiles map[string]time.Time

	// pendingMu protects pendingWrites. Chunks are buffered here during
	// StageData and flushed to L2 only after CommitData succeeds, preventing
	// other nodes from reading partially-written data.
	pendingMu     sync.Mutex
	pendingWrites map[string]*pendingFile

	// flushMu protects flushCancel. When a new commit or invalidation arrives
	// for a file, any in-flight flush goroutine for that file is cancelled to
	// prevent it from uploading stale data after a DeleteGroup.
	flushMu     sync.Mutex
	flushCancel map[string]context.CancelFunc

	// readUploadMu protects readUploadCancels. Read-path uploadChunkAsync
	// goroutines share a per-file context so they can be cancelled when a
	// write/invalidation arrives, preventing stale data from overwriting
	// freshly flushed chunks.
	readUploadMu      sync.Mutex
	readUploadCancels map[string]*readUploadEntry

	// stopCleanup signals the background pending-writes cleanup goroutine to exit.
	stopCleanup chan struct{}
}

const dirtyTTL = 10 * time.Second

// dcacheClient abstracts the distributed cache client for testing.
type dcacheClient interface {
	Upload(ctx context.Context, filename, etag string, data io.Reader, size int64, opts ...dcache.UploadOption) error
	DownloadWithSizePartial(ctx context.Context, filename, etag string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) (<-chan dcache.ChunkError, func() error, error)
	DownloadChunk(ctx context.Context, filename, etag string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	UploadChunk(ctx context.Context, filename, etag string, offset int64, data []byte, opts ...dcache.UploadOption) error
	Delete(ctx context.Context, filename string, fileSize int64) error
	DeleteGroup(ctx context.Context, groupID []byte) error
	GetChunkGroupID(ctx context.Context, filename, etag string) ([]byte, error)
	GetAttr(ctx context.Context, filename string) (*dcache.FileAttr, error)
	PutAttr(ctx context.Context, attrs []dcache.FileAttrEntry) error
	Close() error
}

// Verify interface compliance.
var _ internal.Component = &DistCache{}

func NewDistCacheComponent() internal.Component {
	comp := &DistCache{
		dirtyFiles:        make(map[string]time.Time),
		pendingWrites:     make(map[string]*pendingFile),
		flushCancel:       make(map[string]context.CancelFunc),
		readUploadCancels: make(map[string]*readUploadEntry),
		stopCleanup:       make(chan struct{}),
	}
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

	// Start background goroutine to evict stale pending writes
	go dc.pendingCleanupLoop()

	return nil
}

func (dc *DistCache) Stop() error {
	log.Trace("Stopping component : %s", dc.Name())
	close(dc.stopCleanup)
	if dc.client != nil {
		return dc.client.Close()
	}
	return nil
}

func (dc *DistCache) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// --- Read path (file_cache) ---

func (dc *DistCache) CopyToFile(options internal.CopyToFileOptions) error {
	if dc.client == nil {
		return dc.NextComponent().CopyToFile(options)
	}

	// Skip dist_cache for recently invalidated files to avoid stale data
	if dc.isDirty(options.Name) {
		log.Debug("DistCache::CopyToFile : dirty, bypassing %s", options.Name)
		return dc.NextComponent().CopyToFile(options)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	etag := options.Etag
	log.Debug("DistCache::CopyToFile : %s etag=%q size=%d", options.Name, etag, options.Count)

	// Try distributed cache with lock-on-miss enabled, collecting per-chunk misses
	chunkErrCh, wait, err := dc.client.DownloadWithSizePartial(ctx, options.Name, etag, options.Count, options.File, dcache.WithLock(true))
	if err != nil {
		if dc.bypassOnError {
			log.Warn("DistCache::CopyToFile : error, bypassing: %v", err)
			return dc.NextComponent().CopyToFile(options)
		}
		return err
	}

	// Handle chunk misses in parallel as they arrive from the channel,
	// concurrently with remaining downloads still in flight.
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallelChunkOps)

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for ce := range chunkErrCh {
			ce := ce
			switch {
			case ce.Err == dcache.ErrNotFoundGotLock:
				g.Go(func() error {
					log.Debug("DistCache::CopyToFile : L2 chunk miss (got lock) %s offset=%d", options.Name, ce.Offset)
					return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, true)
				})

			case ce.Err == dcache.ErrNotFoundAlreadyLocked:
				g.Go(func() error {
					log.Debug("DistCache::CopyToFile : L2 chunk miss (locked) %s offset=%d, polling", options.Name, ce.Offset)
					if err := dc.pollUntilChunkCached(gctx, options, ce.Offset, ce.Size); err != nil {
						log.Debug("DistCache::CopyToFile : chunk poll timeout %s offset=%d, falling through", options.Name, ce.Offset)
						return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, false)
					}
					return nil
				})

			case dcache.IsRecoverableNetErr(ce.Err):
				g.Go(func() error {
					log.Warn("DistCache::CopyToFile : L2 chunk network error %s offset=%d err=%v, fetching from storage", options.Name, ce.Offset, ce.Err)
					return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, false)
				})

			default:
				g.Go(func() error {
					log.Debug("DistCache::CopyToFile : L2 chunk miss %s offset=%d", options.Name, ce.Offset)
					return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, false)
				})
			}
		}
	}()

	// Wait for all cache downloads to finish (closes chunkErrCh)
	if fatalErr := wait(); fatalErr != nil {
		cancel() // cancel recovery goroutines
		<-readerDone
		_ = g.Wait() // drain in-flight recovery work before returning
		if dc.bypassOnError {
			log.Warn("DistCache::CopyToFile : fatal download error, bypassing: %v", fatalErr)
			return dc.NextComponent().CopyToFile(options)
		}
		return fatalErr
	}

	// Wait for channel reader to finish queueing all recovery work
	<-readerDone

	// Wait for all miss recovery operations to finish
	if err := g.Wait(); err != nil {
		return err
	}

	log.Debug("DistCache::CopyToFile : completed %s", options.Name)
	return nil
}

// --- Write path (file_cache) ---

func (dc *DistCache) CopyFromFile(options internal.CopyFromFileOptions) error {
	// Resolve old ETag from remote blob BEFORE commit overwrites it
	var oldETag string
	if dc.client != nil {
		oldAttr, err := dc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
		if err == nil && oldAttr != nil {
			oldETag = oldAttr.ETag
		}
		log.Debug("DistCache::CopyFromFile : %s oldETag=%q (err=%v)", options.Name, oldETag, err)
		// If GetAttr returns error (e.g., 404 for new file), oldETag stays empty
	}

	// Provide a pointer for azstorage to write the new ETag into
	var newETagStr string
	if options.NewETag == nil {
		options.NewETag = &newETagStr
	}

	// Write-through to azstorage first (source of truth)
	err := dc.NextComponent().CopyFromFile(options)
	if err != nil {
		return err
	}

	if dc.client == nil {
		return nil
	}

	// Get new ETag from the commit response
	newETag := *options.NewETag
	log.Debug("DistCache::CopyFromFile : %s commit succeeded, oldETag=%q newETag=%q", options.Name, oldETag, newETag)

	// Cancel any in-flight flush/populate from a previous write
	dc.cancelFlush(options.Name)

	// Cancel any in-flight read-path uploads that may overwrite our fresh data
	dc.cancelReadUploads(options.Name)

	// Mark dirty so other nodes bypass stale L2 data during the populate window
	dc.markDirty(options.Name)

	// Delete old version chunks if we had a previous ETag
	if oldETag != "" {
		oldGID := fileGroupID(options.Name, oldETag)
		log.Debug("DistCache::CopyFromFile : deleting old group %q for %s", string(oldGID), options.Name)
		if err := dc.client.DeleteGroup(context.Background(), oldGID); err != nil {
			log.Warn("DistCache::CopyFromFile : L2 invalidation failed for %s: %v", options.Name, err)
		}
	} else {
		log.Debug("DistCache::CopyFromFile : %s no old ETag (new file), skipping DeleteGroup", options.Name)
	}

	// Populate distributed cache (best-effort, async) with new ETag
	log.Debug("DistCache::CopyFromFile : populating L2 for %s with newETag=%q", options.Name, newETag)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	dc.flushMu.Lock()
	dc.flushCancel[options.Name] = cancel
	dc.flushMu.Unlock()
	go func() {
		defer cancel() // release timer resources on the success path
		dc.populateCache(ctx, options.Name, options.File.Name(), newETag)
	}()
	return nil
}

// --- Read path (block_cache) ---

// resolveReadPath returns the file path for a ReadInBuffer call. block_cache
// sets Handle but not Path; file_cache/azstorage may set Path directly.
func resolveReadPath(options *internal.ReadInBufferOptions) string {
	if options.Path != "" {
		return options.Path
	}
	if options.Handle != nil {
		return options.Handle.Path
	}
	return ""
}

func (dc *DistCache) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	if dc.client == nil {
		return dc.NextComponent().ReadInBuffer(options)
	}

	name := resolveReadPath(options)

	// Skip dist_cache for recently invalidated files to avoid stale data
	if dc.isDirty(name) {
		log.Debug("DistCache::ReadInBuffer : dirty, bypassing %s", name)
		return dc.NextComponent().ReadInBuffer(options)
	}

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
			dataCopy := make([]byte, n)
			copy(dataCopy, options.Data[:n])
			uploadCtx := dc.getReadUploadCtx(name)
			go dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
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
		dataCopy := make([]byte, n)
		copy(dataCopy, options.Data[:n])
		uploadCtx := dc.getReadUploadCtx(name)
		go dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
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
		dataCopy := make([]byte, n)
		copy(dataCopy, options.Data[:n])
		uploadCtx := dc.getReadUploadCtx(name)
		go dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
		return n, nil
	}

	if err == dcache.ErrNotFound {
		// L2 miss — read from Azure
		log.Debug("DistCache::ReadInBuffer : L2 miss %s offset=%d", name, options.Offset)
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		dataCopy := make([]byte, n)
		copy(dataCopy, options.Data[:n])
		uploadCtx := dc.getReadUploadCtx(name)
		go dc.uploadChunkAsync(uploadCtx, name, etag, options.Offset, dataCopy)
		return n, nil
	}

	if dc.bypassOnError {
		log.Warn("DistCache::ReadInBuffer : error, bypassing: %v", err)
		return dc.NextComponent().ReadInBuffer(options)
	}
	return 0, err
}

// --- Write path (block_cache) ---

func (dc *DistCache) StageData(options internal.StageDataOptions) error {
	// Write-through to azstorage first
	err := dc.NextComponent().StageData(options)
	if err != nil {
		return err
	}

	if dc.client == nil {
		return nil
	}

	dataLen := int64(len(options.Data))
	maxSize := int64(dc.conf.MaxFileSizeMB) * 1024 * 1024

	dataCopy := make([]byte, dataLen)
	copy(dataCopy, options.Data)

	dc.pendingMu.Lock()
	pf := dc.pendingWrites[options.Name]

	// Size cap: if buffering this chunk would exceed MaxFileSizeMB, drop all
	// pending data for this file. L2 will be warmed via the read path instead.
	// No need to invalidate L2 here — the committed state hasn't changed, so
	// existing L2 data is still valid. CommitData will handle invalidation if
	// and when the write is actually committed.
	pendingSize := dataLen
	if pf != nil {
		pendingSize = pf.totalSize + dataLen
	}
	if maxSize > 0 && pendingSize > maxSize {
		log.Debug("DistCache::StageData : pending size would exceed %dMB for %s, skipping L2 write-warming",
			dc.conf.MaxFileSizeMB, options.Name)
		delete(dc.pendingWrites, options.Name)
		dc.pendingMu.Unlock()
		return nil
	}

	if pf == nil {
		pf = &pendingFile{}
		dc.pendingWrites[options.Name] = pf
	}
	pf.chunks = append(pf.chunks, pendingChunk{
		offset: int64(options.Offset),
		data:   dataCopy,
	})
	pf.totalSize += dataLen
	pf.lastActivity = time.Now()
	dc.pendingMu.Unlock()

	return nil
}

func (dc *DistCache) CommitData(options internal.CommitDataOptions) error {
	// Resolve old ETag from remote blob BEFORE commit overwrites it
	var oldETag string
	if dc.client != nil {
		oldAttr, err := dc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
		if err == nil && oldAttr != nil {
			oldETag = oldAttr.ETag
		}
		log.Debug("DistCache::CommitData : %s oldETag=%q (err=%v)", options.Name, oldETag, err)
		// If GetAttr returns error (e.g., 404 for new file), oldETag stays empty
	}

	// Forward to azstorage first — commit is the source-of-truth operation
	err := dc.NextComponent().CommitData(options)
	if err != nil {
		return err
	}

	if dc.client == nil {
		return nil
	}

	// Get new ETag from the commit response
	var newETag string
	if options.NewETag != nil {
		newETag = *options.NewETag
	}
	log.Debug("DistCache::CommitData : %s commit succeeded, oldETag=%q newETag=%q", options.Name, oldETag, newETag)

	// Cancel any in-flight flush from a previous commit. This prevents a racing
	// goroutine from uploading stale chunks after our DeleteGroup below.
	dc.cancelFlush(options.Name)

	// Cancel any in-flight read-path uploads that may overwrite our fresh data
	dc.cancelReadUploads(options.Name)

	dc.markDirty(options.Name)

	// Delete old version chunks if we had a previous ETag
	if oldETag != "" {
		oldGID := fileGroupID(options.Name, oldETag)
		log.Debug("DistCache::CommitData : deleting old group %q for %s", string(oldGID), options.Name)
		if err := dc.client.DeleteGroup(context.Background(), oldGID); err != nil {
			log.Warn("DistCache::CommitData : L2 invalidation failed for %s: %v", options.Name, err)
		}
	} else {
		log.Debug("DistCache::CommitData : %s no old ETag (new file), skipping DeleteGroup", options.Name)
	}

	// Drain pending chunks and flush to L2 asynchronously now that the
	// file is committed in Azure and safe for other nodes to read.
	dc.pendingMu.Lock()
	pf := dc.pendingWrites[options.Name]
	delete(dc.pendingWrites, options.Name)
	dc.pendingMu.Unlock()

	if pf != nil && len(pf.chunks) > 0 {
		log.Debug("DistCache::CommitData : flushing %d pending chunks to L2 for %s with newETag=%q", len(pf.chunks), options.Name, newETag)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		dc.flushMu.Lock()
		dc.flushCancel[options.Name] = cancel
		dc.flushMu.Unlock()
		go func() {
			defer cancel() // release timer resources on the success path
			dc.flushPendingToL2(ctx, options.Name, pf.chunks, newETag)
		}()
	}
	return nil
}

// --- Invalidation ---

func (dc *DistCache) DeleteFile(options internal.DeleteFileOptions) error {
	if dc.client != nil {
		// Cancel any in-flight flush/populate goroutine so it cannot re-upload
		// chunks under the group ID we are about to delete.
		dc.cancelFlush(options.Name)
		dc.cancelReadUploads(options.Name)
		dc.markDirty(options.Name)
		dc.clearPending(options.Name)
		etag := dc.resolveRemoteETag(options.Name)
		log.Debug("DistCache::DeleteFile : %s resolvedETag=%q", options.Name, etag)
		if etag != "" {
			gid := fileGroupID(options.Name, etag)
			log.Debug("DistCache::DeleteFile : deleting group %q for %s", string(gid), options.Name)
			if err := dc.client.DeleteGroup(context.Background(), gid); err != nil {
				log.Warn("DistCache::DeleteFile : cache invalidation failed for %s: %v", options.Name, err)
			}
		}
	}
	return dc.NextComponent().DeleteFile(options)
}

func (dc *DistCache) RenameFile(options internal.RenameFileOptions) error {
	if dc.client != nil {
		// Cancel any in-flight flush/populate goroutine so it cannot re-upload
		// chunks under the group ID we are about to delete.
		dc.cancelFlush(options.Src)
		dc.cancelReadUploads(options.Src)
		dc.markDirty(options.Src)
		dc.clearPending(options.Src)
		etag := dc.resolveRemoteETag(options.Src)
		log.Debug("DistCache::RenameFile : %s -> %s resolvedETag=%q", options.Src, options.Dst, etag)
		if etag != "" {
			gid := fileGroupID(options.Src, etag)
			log.Debug("DistCache::RenameFile : deleting group %q for %s", string(gid), options.Src)
			if err := dc.client.DeleteGroup(context.Background(), gid); err != nil {
				log.Warn("DistCache::RenameFile : cache invalidation failed for %s: %v", options.Src, err)
			}
		}
	}
	return dc.NextComponent().RenameFile(options)
}

func (dc *DistCache) TruncateFile(options internal.TruncateFileOptions) error {
	if dc.client != nil {
		// Cancel any in-flight flush/populate goroutine so it cannot re-upload
		// chunks under the group ID we are about to delete.
		dc.cancelFlush(options.Name)
		dc.cancelReadUploads(options.Name)
		dc.markDirty(options.Name)
		dc.clearPending(options.Name)
		etag := dc.resolveRemoteETag(options.Name)
		log.Debug("DistCache::TruncateFile : %s resolvedETag=%q", options.Name, etag)
		if etag != "" {
			gid := fileGroupID(options.Name, etag)
			log.Debug("DistCache::TruncateFile : deleting group %q for %s", string(gid), options.Name)
			if err := dc.client.DeleteGroup(context.Background(), gid); err != nil {
				log.Warn("DistCache::TruncateFile : cache invalidation failed for %s: %v", options.Name, err)
			}
		}
	}
	return dc.NextComponent().TruncateFile(options)
}

// --- Internal helpers ---

// fetchChunkFromRemote downloads a single chunk from Azure via the next component
// and writes it to the file at the correct offset. If populateCache is true,
// the chunk is also uploaded to the distributed cache asynchronously.
func (dc *DistCache) fetchChunkFromRemote(ctx context.Context, options internal.CopyToFileOptions, offset, size int64, populateCache bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	buf := make([]byte, size)
	readOpts := &internal.ReadInBufferOptions{
		Path:   options.Name,
		Offset: offset,
		Data:   buf,
		Size:   options.Count,
	}
	n, err := dc.NextComponent().ReadInBuffer(readOpts)
	if err != nil {
		return err
	}
	if _, err := options.File.WriteAt(buf[:n], offset); err != nil {
		return err
	}
	if populateCache {
		dataCopy := make([]byte, n)
		copy(dataCopy, buf[:n])
		uploadCtx := dc.getReadUploadCtx(options.Name)
		go dc.uploadChunkAsync(uploadCtx, options.Name, options.Etag, offset, dataCopy)
	}
	return nil
}

// pollUntilChunkCached waits for a single chunk to become available in the
// distributed cache and writes it to the file. Returns nil on success.
func (dc *DistCache) pollUntilChunkCached(ctx context.Context, options internal.CopyToFileOptions, offset, size int64) error {
	buf := make([]byte, size)
	n, err := dc.pollChunkIntoBuffer(ctx, options.Name, options.Etag, offset, buf)
	if err != nil {
		return err
	}
	_, err = options.File.WriteAt(buf[:n], offset)
	return err
}

// pollChunkIntoBuffer waits for a single chunk to become available in the
// distributed cache and copies it into buf. Returns the number of bytes read.
func (dc *DistCache) pollChunkIntoBuffer(ctx context.Context, name, etag string, offset int64, buf []byte) (int, error) {
	const (
		maxPollDuration = 30 * time.Second
		maxBackoff      = 5 * time.Second
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

// markDirty records that a file was recently invalidated. Reads for this file
// will bypass dist_cache until dirtyTTL expires or clearDirty is called.
func (dc *DistCache) markDirty(name string) {
	dc.dirtyMu.Lock()
	dc.dirtyFiles[name] = time.Now()
	dc.dirtyMu.Unlock()
}

// clearDirty removes the dirty flag for a file, allowing reads to use L2 again.
// Called after L2 has been successfully re-populated with fresh data.
func (dc *DistCache) clearDirty(name string) {
	dc.dirtyMu.Lock()
	delete(dc.dirtyFiles, name)
	dc.dirtyMu.Unlock()
}

// isDirty returns true if the file was recently invalidated and should not be
// read from dist_cache.
func (dc *DistCache) isDirty(name string) bool {
	dc.dirtyMu.Lock()
	t, ok := dc.dirtyFiles[name]
	if ok && time.Since(t) > dirtyTTL {
		delete(dc.dirtyFiles, name)
		ok = false
	}
	dc.dirtyMu.Unlock()
	return ok
}

// fileGroupID returns a versioned group ID for a file. All chunks uploaded in
// the same version share this ID. The etag is the Azure blob ETag for the file
// revision, ensuring that DeleteGroup for an old version cannot affect chunks
// uploaded under a new version.
func fileGroupID(name string, etag string) []byte {
	return []byte(fmt.Sprintf("%s\x00v%s", name, etag))
}

// resolveRemoteETag queries the remote blob for its current ETag.
// Returns empty string if the blob doesn't exist or GetAttr fails.
func (dc *DistCache) resolveRemoteETag(name string) string {
	attr, err := dc.NextComponent().GetAttr(internal.GetAttrOptions{Name: name})
	if err != nil || attr == nil {
		return ""
	}
	return attr.ETag
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

// clearPending discards any buffered chunks for a file (e.g. on delete/truncate).
func (dc *DistCache) clearPending(name string) {
	dc.pendingMu.Lock()
	delete(dc.pendingWrites, name)
	dc.pendingMu.Unlock()
}

// cancelFlush cancels any in-flight flush goroutine for the given file.
// Must be called before DeleteGroup to prevent a racing flush from re-uploading
// stale data after the group has been deleted.
func (dc *DistCache) cancelFlush(name string) {
	dc.flushMu.Lock()
	if cancel, ok := dc.flushCancel[name]; ok {
		log.Debug("DistCache::cancelFlush : cancelling in-flight flush goroutine for %s (new write arrived)", name)
		cancel()
		delete(dc.flushCancel, name)
	}
	dc.flushMu.Unlock()
}

// flushPendingToL2 uploads all buffered chunks for a file to the distributed
// cache. Called asynchronously after CommitData succeeds. The etag parameter
// is the new ETag to use for the group ID and cache keys.
func (dc *DistCache) flushPendingToL2(ctx context.Context, name string, chunks []pendingChunk, etag string) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxPendingL2Uploads)

	for i := range chunks {
		chunk := chunks[i]
		g.Go(func() error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			gid := fileGroupID(name, etag)
			log.Debug("DistCache::flushPendingToL2 : uploading chunk %s offset=%d with group %q", name, chunk.offset, string(gid))
			opts := []dcache.UploadOption{
				dcache.WithIgnoreLock(true),
				dcache.WithGroupID(gid),
				dcache.WithMetadata(map[string][]byte{"gid": gid}),
			}
			if dc.conf.TTLSeconds > 0 {
				opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
			}

			if err := dc.client.UploadChunk(gctx, name, etag, chunk.offset, chunk.data, opts...); err != nil {
				log.Warn("DistCache::flushPendingToL2 : upload failed for %s offset=%d: %v", name, chunk.offset, err)
			}
			return nil // best-effort: don't abort other uploads on failure
		})
	}

	_ = g.Wait()
	log.Debug("DistCache::flushPendingToL2 : flushed %d chunks for %s", len(chunks), name)

	// Only clear dirty if we weren't cancelled (a cancellation means a new
	// commit/invalidation is in progress and will manage the dirty state).
	if ctx.Err() == nil {
		dc.clearDirty(name)
	}

	// Clean up the cancel entry
	dc.flushMu.Lock()
	delete(dc.flushCancel, name)
	dc.flushMu.Unlock()
}

// pendingCleanupLoop periodically evicts pending entries that have exceeded
// pendingWriteTTL. This handles abandoned writes where CommitData is never called.
func (dc *DistCache) pendingCleanupLoop() {
	ticker := time.NewTicker(pendingCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dc.stopCleanup:
			return
		case <-ticker.C:
			dc.evictStalePending()
		}
	}
}

// evictStalePending removes pending entries whose lastActivity exceeds pendingWriteTTL.
func (dc *DistCache) evictStalePending() {
	now := time.Now()
	dc.pendingMu.Lock()
	for name, pf := range dc.pendingWrites {
		if now.Sub(pf.lastActivity) > pendingWriteTTL {
			log.Debug("DistCache::evictStalePending : evicting %d stale chunks for %s (idle %v)",
				len(pf.chunks), name, now.Sub(pf.lastActivity))
			delete(dc.pendingWrites, name)
		}
	}
	dc.pendingMu.Unlock()
}

func (dc *DistCache) populateCache(ctx context.Context, name string, filePath string, etag string) {
	defer func() {
		dc.flushMu.Lock()
		delete(dc.flushCancel, name)
		dc.flushMu.Unlock()
	}()

	// Re-open the file by path (the original handle may be closed by the caller)
	f, err := os.Open(filePath)
	if err != nil {
		log.Warn("DistCache::populateCache : open failed: %v", err)
		return
	}
	defer f.Close()

	// Get file size
	info, err := f.Stat()
	if err != nil {
		log.Warn("DistCache::populateCache : stat failed: %v", err)
		return
	}

	gid := fileGroupID(name, etag)
	log.Debug("DistCache::populateCache : uploading %s (size=%d) with group %q", name, info.Size(), string(gid))
	opts := []dcache.UploadOption{
		dcache.WithIgnoreLock(true),
		dcache.WithGroupID(gid),
		dcache.WithMetadata(map[string][]byte{"gid": gid}),
	}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.Upload(ctx, name, etag, f, info.Size(), opts...); err != nil {
		log.Warn("DistCache::populateCache : upload failed: %v", err)
		return
	}

	// Only clear dirty if we weren't cancelled
	if ctx.Err() == nil {
		dc.clearDirty(name)
	}
}

// readUploadEntry holds a shared context for read-path uploads on a single file.
type readUploadEntry struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// getReadUploadCtx returns a cancellable context for read-path uploads on the
// given file. All uploadChunkAsync goroutines for the same file share this
// context so they can be bulk-cancelled when a write arrives.
func (dc *DistCache) getReadUploadCtx(name string) context.Context {
	dc.readUploadMu.Lock()
	defer dc.readUploadMu.Unlock()

	if entry, ok := dc.readUploadCancels[name]; ok {
		// Reuse existing context if it hasn't been cancelled
		if entry.ctx.Err() == nil {
			return entry.ctx
		}
		// Previous context was cancelled (by a write), create a fresh one
		delete(dc.readUploadCancels, name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	dc.readUploadCancels[name] = &readUploadEntry{ctx: ctx, cancel: cancel}
	return ctx
}

// cancelReadUploads cancels all in-flight read-path uploadChunkAsync goroutines
// for the given file. Called from write/invalidation paths to prevent stale
// read data from overwriting freshly committed chunks.
func (dc *DistCache) cancelReadUploads(name string) {
	dc.readUploadMu.Lock()
	if entry, ok := dc.readUploadCancels[name]; ok {
		log.Debug("DistCache::cancelReadUploads : cancelling previous uploadChunkAsync goroutines for %s (new write/invalidation arrived)", name)
		entry.cancel()
		delete(dc.readUploadCancels, name)
	}
	dc.readUploadMu.Unlock()
}

func (dc *DistCache) uploadChunkAsync(ctx context.Context, name, etag string, offset int64, data []byte) {
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
