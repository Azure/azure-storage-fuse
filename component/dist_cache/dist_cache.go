// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dist_cache

import (
	"context"
	"fmt"
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

	// Chunk size is NOT configured here — it is read from block_cache.block-size-mb
	// (default 16 MiB). This ensures a single source of truth for block/chunk alignment.
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
	Upload(ctx context.Context, filename string, data *os.File, size int64, opts ...dcache.UploadOption) error
	DownloadWithSize(ctx context.Context, filename string, fileSize int64, w *os.File, opts ...dcache.DownloadOption) (*dcache.FileMetadata, error)
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

	// Read chunk size from block_cache.block-size-mb (single source of truth)
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

	dc.client = &dcacheClientAdapter{client: client}
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
		go dc.populateCache(options.Name, options.File)
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
	go dc.populateCache(options.Name, options.File)
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
		// Populate distributed cache (best-effort)
		go dc.uploadChunkAsync(options.Path, options.Offset, options.Data[:n])
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
	go dc.uploadChunkAsync(options.Name, int64(options.Offset), options.Data)
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

func (dc *DistCache) populateCache(name string, f *os.File) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get file size
	info, err := f.Stat()
	if err != nil {
		log.Warn("DistCache::populateCache : stat failed: %v", err)
		return
	}

	// Seek to beginning
	if _, err := f.Seek(0, 0); err != nil {
		log.Warn("DistCache::populateCache : seek failed: %v", err)
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

	// Make a copy since the caller may reuse the buffer
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	opts := []dcache.UploadOption{dcache.WithIgnoreLock(true)}
	if dc.conf.TTLSeconds > 0 {
		opts = append(opts, dcache.WithTTL(dc.conf.TTLSeconds))
	}

	if err := dc.client.UploadChunk(ctx, name, offset, dataCopy, opts...); err != nil {
		log.Warn("DistCache::uploadChunkAsync : upload failed: %v", err)
	}
}

func (dc *DistCache) pollUntilCached(ctx context.Context, name string, fileSize int64, f *os.File) (*dcache.FileMetadata, error) {
	const maxRetries = 10
	backoff := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		meta, err := dc.client.DownloadWithSize(ctx, name, fileSize, f)
		if err == nil {
			return meta, nil
		}
		if err != dcache.ErrNotFoundAlreadyLocked && err != dcache.ErrNotFound {
			return nil, err
		}

		// Exponential backoff, max 2 seconds
		backoff *= 2
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
		}
	}

	return nil, fmt.Errorf("dist_cache: poll timeout after %d retries", maxRetries)
}

// dcacheClientAdapter adapts the real dcache.Client to the dcacheClient interface,
// bridging the io.Reader/io.Writer types to *os.File for splice(2) support.
type dcacheClientAdapter struct {
	client *dcache.Client
}

func (a *dcacheClientAdapter) Upload(ctx context.Context, filename string, data *os.File, size int64, opts ...dcache.UploadOption) error {
	return a.client.Upload(ctx, filename, data, size, opts...)
}

func (a *dcacheClientAdapter) DownloadWithSize(ctx context.Context, filename string, fileSize int64, w *os.File, opts ...dcache.DownloadOption) (*dcache.FileMetadata, error) {
	return a.client.DownloadWithSize(ctx, filename, fileSize, w, opts...)
}

func (a *dcacheClientAdapter) DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error) {
	return a.client.DownloadChunk(ctx, filename, offset, buf, opts...)
}

func (a *dcacheClientAdapter) UploadChunk(ctx context.Context, filename string, offset int64, data []byte, opts ...dcache.UploadOption) error {
	return a.client.UploadChunk(ctx, filename, offset, data, opts...)
}

func (a *dcacheClientAdapter) Delete(ctx context.Context, filename string, fileSize int64) error {
	return a.client.Delete(ctx, filename, fileSize)
}

func (a *dcacheClientAdapter) GetAttr(ctx context.Context, filename string) (*dcache.FileAttr, error) {
	return a.client.GetAttr(ctx, filename)
}

func (a *dcacheClientAdapter) PutAttr(ctx context.Context, attrs []dcache.FileAttrEntry) error {
	return a.client.PutAttr(ctx, attrs)
}

func (a *dcacheClientAdapter) Close() error {
	return a.client.Close()
}
