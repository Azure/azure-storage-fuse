// Copyright (c) Microsoft Corporation.
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

	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client"
	"golang.org/x/sync/errgroup"
)

const compName = "dist_cache"

// maxParallelChunkOps limits the number of concurrent chunk-level recovery
// operations (Azure fetches and cache polls) during CopyToFile.
const maxParallelChunkOps = 8

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
	Port            int    `config:"port"              yaml:"port,omitempty"`
	TTLSeconds      uint32 `config:"ttl-seconds"       yaml:"ttl-seconds,omitempty"`
	MaxFileSizeMB   int    `config:"max-file-size-mb"  yaml:"max-file-size-mb,omitempty"`
	AuthAccountName string `config:"auth-account-name" yaml:"auth-account-name,omitempty"`
	AuthAccountKey  string `config:"auth-account-key"  yaml:"auth-account-key,omitempty"`
	HashType        string `config:"hash-type"         yaml:"hash-type,omitempty"`
	BypassOnError   bool   `config:"bypass-on-error"   yaml:"bypass-on-error,omitempty"`
	CachePrefix     string `config:"cache-prefix"      yaml:"cache-prefix,omitempty"`
	MaxConnsPerSvr  int    `config:"max-conns-per-server" yaml:"max-conns-per-server,omitempty"`

	// Chunk size for distributed cache operations. When block_cache is present,
	// this is overridden by block_cache.block-size-mb to keep alignment consistent.
	// When used with file_cache (no block_cache), this is the primary chunk size config.
	ChunkSizeMB float64 `config:"chunk-size-mb" yaml:"chunk-size-mb,omitempty"`
}

// DistCache is the blobfuse component that sits between the local cache and azstorage,
// providing a shared distributed cache layer across nodes.
type DistCache struct {
	internal.BaseComponent
	conf   DistCacheOptions
	client dcacheClient

	chunkSize     int64
	bypassOnError bool

	// dirtyFiles tracks recently invalidated files to avoid serving stale data.
	// After a delete/truncate/rename, the file is added here. Reads for files
	// in this set bypass dist_cache until the TTL expires, giving the server
	// time to process the async group-based deletion.
	dirtyMu    sync.Mutex
	dirtyFiles map[string]time.Time
}

const dirtyTTL = 10 * time.Second

// dcacheClient abstracts the distributed cache client for testing.
type dcacheClient interface {
	Upload(ctx context.Context, filename string, data io.Reader, size int64, opts ...dcache.UploadOption) error
	DownloadWithSizePartial(ctx context.Context, filename string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) ([]dcache.ChunkError, error)
	DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	UploadChunk(ctx context.Context, filename string, offset int64, data []byte, opts ...dcache.UploadOption) error
	Delete(ctx context.Context, filename string, fileSize int64) error
	DeleteGroup(ctx context.Context, groupID []byte) error
	GetAttr(ctx context.Context, filename string) (*dcache.FileAttr, error)
	PutAttr(ctx context.Context, attrs []dcache.FileAttrEntry) error
	Close() error
}

// Verify interface compliance.
var _ internal.Component = &DistCache{}

func NewDistCacheComponent() internal.Component {
	comp := &DistCache{
		dirtyFiles: make(map[string]time.Time),
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

	dc.conf = conf
	dc.bypassOnError = conf.BypassOnError

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
	if dc.conf.AuthAccountName != "" {
		opts = append(opts, dcache.WithAuth(dc.conf.AuthAccountName, dc.conf.AuthAccountKey))
	}
	if dc.conf.CachePrefix != "" {
		opts = append(opts, dcache.WithCachePrefix(dc.conf.CachePrefix))
	}
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

	ctx := context.Background()

	// Try distributed cache with lock-on-miss enabled, collecting per-chunk misses
	chunkErrors, err := dc.client.DownloadWithSizePartial(ctx, options.Name, options.Count, options.File,
		dcache.WithLock(true))
	if err != nil {
		if dc.bypassOnError {
			log.Warn("DistCache::CopyToFile : error, bypassing: %v", err)
			return dc.NextComponent().CopyToFile(options)
		}
		return err
	}

	if len(chunkErrors) == 0 {
		log.Debug("DistCache::CopyToFile : L2 hit %s", options.Name)
		return nil
	}

	// Handle per-chunk cache misses in parallel
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallelChunkOps)

	for _, ce := range chunkErrors {
		ce := ce
		switch ce.Err {
		case dcache.ErrNotFoundGotLock:
			g.Go(func() error {
				log.Debug("DistCache::CopyToFile : L2 chunk miss (got lock) %s offset=%d", options.Name, ce.Offset)
				return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, true)
			})

		case dcache.ErrNotFoundAlreadyLocked:
			g.Go(func() error {
				log.Debug("DistCache::CopyToFile : L2 chunk miss (locked) %s offset=%d, polling", options.Name, ce.Offset)
				if err := dc.pollUntilChunkCached(gctx, options, ce.Offset, ce.Size); err != nil {
					log.Debug("DistCache::CopyToFile : chunk poll timeout %s offset=%d, falling through", options.Name, ce.Offset)
					return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, false)
				}
				return nil
			})

		default:
			g.Go(func() error {
				log.Debug("DistCache::CopyToFile : L2 chunk miss %s offset=%d", options.Name, ce.Offset)
				return dc.fetchChunkFromRemote(gctx, options, ce.Offset, ce.Size, false)
			})
		}
	}

	return g.Wait()
}

// --- Write path (file_cache) ---

func (dc *DistCache) CopyFromFile(options internal.CopyFromFileOptions) error {
	// Write-through to azstorage first (source of truth)
	err := dc.NextComponent().CopyFromFile(options)
	if err != nil {
		return err
	}

	if dc.client == nil {
		return nil
	}

	// Populate distributed cache (best-effort, async)
	go dc.populateCache(options.Name, options.File.Name())
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

	ctx := context.Background()

	n, err := dc.client.DownloadChunk(ctx, name, options.Offset, options.Data)
	if err == nil && n > 0 {
		log.Debug("DistCache::ReadInBuffer : L2 hit %s offset=%d", name, options.Offset)
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
			go dc.uploadChunkAsync(name, options.Offset, dataCopy)
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
		go dc.uploadChunkAsync(name, options.Offset, dataCopy)
		return n, nil
	}

	if err == dcache.ErrNotFoundAlreadyLocked {
		// Another node is fetching this chunk — poll until cached
		log.Debug("DistCache::ReadInBuffer : L2 miss (locked) %s offset=%d, polling", name, options.Offset)
		n, pollErr := dc.pollChunkIntoBuffer(ctx, name, options.Offset, options.Data)
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
		go dc.uploadChunkAsync(name, options.Offset, dataCopy)
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
		go dc.uploadChunkAsync(name, options.Offset, dataCopy)
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

	// Populate distributed cache (best-effort)
	// Copy buffer before launching goroutine — caller may reuse it immediately
	dataCopy := make([]byte, len(options.Data))
	copy(dataCopy, options.Data)
	go dc.uploadChunkAsync(options.Name, int64(options.Offset), dataCopy)
	return nil
}

func (dc *DistCache) CommitData(options internal.CommitDataOptions) error {
	// Forward to azstorage — commit doesn't need caching
	return dc.NextComponent().CommitData(options)
}

// --- Invalidation ---

func (dc *DistCache) DeleteFile(options internal.DeleteFileOptions) error {
	if dc.client != nil {
		dc.markDirty(options.Name)
		if err := dc.client.DeleteGroup(context.Background(), fileGroupID(options.Name)); err != nil {
			log.Warn("DistCache::DeleteFile : cache invalidation failed for %s: %v", options.Name, err)
		}
	}
	return dc.NextComponent().DeleteFile(options)
}

func (dc *DistCache) RenameFile(options internal.RenameFileOptions) error {
	if dc.client != nil {
		dc.markDirty(options.Src)
		if err := dc.client.DeleteGroup(context.Background(), fileGroupID(options.Src)); err != nil {
			log.Warn("DistCache::RenameFile : cache invalidation failed for %s: %v", options.Src, err)
		}
	}
	return dc.NextComponent().RenameFile(options)
}

func (dc *DistCache) TruncateFile(options internal.TruncateFileOptions) error {
	if dc.client != nil {
		dc.markDirty(options.Name)
		if err := dc.client.DeleteGroup(context.Background(), fileGroupID(options.Name)); err != nil {
			log.Warn("DistCache::TruncateFile : cache invalidation failed for %s: %v", options.Name, err)
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
		go dc.uploadChunkAsync(options.Name, offset, dataCopy)
	}
	return nil
}

// pollUntilChunkCached waits for a single chunk to become available in the
// distributed cache and writes it to the file. Returns nil on success.
func (dc *DistCache) pollUntilChunkCached(ctx context.Context, options internal.CopyToFileOptions, offset, size int64) error {
	buf := make([]byte, size)
	n, err := dc.pollChunkIntoBuffer(ctx, options.Name, offset, buf)
	if err != nil {
		return err
	}
	_, err = options.File.WriteAt(buf[:n], offset)
	return err
}

// pollChunkIntoBuffer waits for a single chunk to become available in the
// distributed cache and copies it into buf. Returns the number of bytes read.
func (dc *DistCache) pollChunkIntoBuffer(ctx context.Context, name string, offset int64, buf []byte) (int, error) {
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

		n, err := dc.client.DownloadChunk(ctx, name, offset, buf)
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
// will bypass dist_cache until dirtyTTL expires.
func (dc *DistCache) markDirty(name string) {
	dc.dirtyMu.Lock()
	dc.dirtyFiles[name] = time.Now()
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

// fileGroupID returns a deterministic group ID for a file, used for group-based
// cache invalidation. All chunks of the same file share this group ID.
func fileGroupID(name string) []byte {
	return []byte(name)
}

func (dc *DistCache) populateCache(name string, filePath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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

	opts := []dcache.UploadOption{
		dcache.WithIgnoreLock(true),
		dcache.WithGroupID(fileGroupID(name)),
	}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.Upload(ctx, name, f, info.Size(), opts...); err != nil {
		log.Warn("DistCache::populateCache : upload failed: %v", err)
	}
}

func (dc *DistCache) uploadChunkAsync(name string, offset int64, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := []dcache.UploadOption{
		dcache.WithIgnoreLock(true),
		dcache.WithGroupID(fileGroupID(name)),
	}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.UploadChunk(ctx, name, offset, data, opts...); err != nil {
		log.Warn("DistCache::uploadChunkAsync : upload failed: %v", err)
	}
}
