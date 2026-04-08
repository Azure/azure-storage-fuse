// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dist_cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client"
)

const compName = "dist_cache"

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
}

// dcacheClient abstracts the distributed cache client for testing.
type dcacheClient interface {
	Upload(ctx context.Context, filename string, data io.Reader, size int64, opts ...dcache.UploadOption) error
	DownloadWithSize(ctx context.Context, filename string, fileSize int64, w io.Writer, opts ...dcache.DownloadOption) (*dcache.FileMetadata, error)
	DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
	UploadChunk(ctx context.Context, filename string, offset int64, data []byte, opts ...dcache.UploadOption) error
	Delete(ctx context.Context, filename string, fileSize int64) error
	GetAttr(ctx context.Context, filename string) (*dcache.FileAttr, error)
	PutAttr(ctx context.Context, attrs []dcache.FileAttrEntry) error
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

	ctx := context.Background()

	// Try distributed cache with lock-on-miss enabled
	_, err := dc.client.DownloadWithSize(ctx, options.Name, options.Count, options.File,
		dcache.WithLock(true))
	if err == nil {
		log.Debug("DistCache::CopyToFile : L2 hit %s", options.Name)
		return nil
	}

	if err == dcache.ErrNotFoundGotLock {
		// We own this miss — download from Azure via azstorage
		log.Debug("DistCache::CopyToFile : L2 miss (got lock) %s", options.Name)
		err = dc.NextComponent().CopyToFile(options)
		if err != nil {
			return err
		}
		// Populate the distributed cache for other nodes (best-effort, async)
		go dc.populateCache(options.Name, options.File.Name())
		return nil
	}

	if err == dcache.ErrNotFoundAlreadyLocked {
		// Another node is fetching — retry with backoff
		log.Debug("DistCache::CopyToFile : L2 miss (locked) %s, polling", options.Name)
		meta, pollErr := dc.pollUntilCached(ctx, options.Name, options.Count, options.File)
		if pollErr == nil && meta != nil {
			return nil
		}
		// Timeout — fall through to azstorage
		log.Debug("DistCache::CopyToFile : poll timeout %s, falling through", options.Name)
		return dc.NextComponent().CopyToFile(options)
	}

	// Other errors
	if dc.bypassOnError {
		log.Warn("DistCache::CopyToFile : error, bypassing: %v", err)
		return dc.NextComponent().CopyToFile(options)
	}
	return err
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

func (dc *DistCache) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	if dc.client == nil {
		return dc.NextComponent().ReadInBuffer(options)
	}

	ctx := context.Background()

	n, err := dc.client.DownloadChunk(ctx, options.Path, options.Offset, options.Data)
	if err == nil {
		log.Debug("DistCache::ReadInBuffer : L2 hit %s offset=%d", options.Path, options.Offset)
		return n, nil
	}

	if err == dcache.ErrNotFound || err == dcache.ErrNotFoundGotLock || err == dcache.ErrNotFoundAlreadyLocked {
		// L2 miss — read from azstorage
		n, err = dc.NextComponent().ReadInBuffer(options)
		if err != nil {
			return n, err
		}
		// Copy buffer before launching goroutine — caller may reuse it immediately
		dataCopy := make([]byte, n)
		copy(dataCopy, options.Data[:n])
		go dc.uploadChunkAsync(options.Path, options.Offset, dataCopy)
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
		_ = dc.client.Delete(context.Background(), options.Name, 0)
	}
	return dc.NextComponent().DeleteFile(options)
}

func (dc *DistCache) RenameFile(options internal.RenameFileOptions) error {
	if dc.client != nil {
		_ = dc.client.Delete(context.Background(), options.Src, 0)
	}
	return dc.NextComponent().RenameFile(options)
}

func (dc *DistCache) TruncateFile(options internal.TruncateFileOptions) error {
	if dc.client != nil {
		_ = dc.client.Delete(context.Background(), options.Name, options.OldSize)
	}
	return dc.NextComponent().TruncateFile(options)
}

// --- Internal helpers ---

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

	opts := []dcache.UploadOption{dcache.WithIgnoreLock(true)}
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

	opts := []dcache.UploadOption{dcache.WithIgnoreLock(true)}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.UploadChunk(ctx, name, offset, data, opts...); err != nil {
		log.Warn("DistCache::uploadChunkAsync : upload failed: %v", err)
	}
}

// pollUntilCached waits for another node to finish populating a file in the
// distributed cache. It uses an adaptive timeout scaled to the file size and
// tracks progress by observing how much data DownloadWithSize writes before
// hitting a missing chunk. If no progress is seen for staleTimeout the poll
// gives up early so the caller can fall through to Azure Storage.
func (dc *DistCache) pollUntilCached(ctx context.Context, name string, fileSize int64, f *os.File) (*dcache.FileMetadata, error) {
	const (
		maxPollDuration = 5 * time.Minute
		minPollDuration = 15 * time.Second
		staleTimeout    = 30 * time.Second
		maxBackoff      = 5 * time.Second
		// Assumed Azure→cache population speed for timeout estimation.
		estimatedBytesPerSec = 2 * 1024 * 1024 * 1024 // 2 GB/s
	)

	// Scale timeout with file size: time-to-fill × 2 headroom + base.
	estimatedFillDur := time.Duration(float64(fileSize)/estimatedBytesPerSec*2+15) * time.Second
	if estimatedFillDur < minPollDuration {
		estimatedFillDur = minPollDuration
	}
	if estimatedFillDur > maxPollDuration {
		estimatedFillDur = maxPollDuration
	}

	deadline := time.Now().Add(estimatedFillDur)
	backoff := 200 * time.Millisecond
	var lastFilePos int64
	lastProgressAt := time.Now()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		// Seek to beginning so a partial retry overwrites from the start,
		// preventing file corruption from accumulated partial writes.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("dist_cache: seek failed: %w", err)
		}

		meta, err := dc.client.DownloadWithSize(ctx, name, fileSize, f)
		if err == nil {
			return meta, nil
		}
		if err != dcache.ErrNotFoundAlreadyLocked && err != dcache.ErrNotFound {
			return nil, err
		}

		// Track progress: how many bytes were written before hitting a miss.
		// downloadChunked writes chunks in order, so the file position
		// indicates how far the populating node has gotten.
		currentPos, _ := f.Seek(0, io.SeekCurrent)
		if currentPos > lastFilePos {
			lastFilePos = currentPos
			lastProgressAt = time.Now()
			log.Debug("DistCache::pollUntilCached : progress %d/%d bytes for %s",
				currentPos, fileSize, name)
		} else if time.Since(lastProgressAt) > staleTimeout {
			log.Debug("DistCache::pollUntilCached : no progress for %v, giving up on %s",
				staleTimeout, name)
			break
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return nil, fmt.Errorf("dist_cache: poll timeout for %d byte file %s", fileSize, name)
}


