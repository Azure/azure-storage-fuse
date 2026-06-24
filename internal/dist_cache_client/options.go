// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import "time"

const (
	defaultPort             = 9000
	defaultChunkSize        = 32 * 1024 * 1024 // 32 MiB, matching Tachyon production default
	defaultMaxConnsPerSvr   = 64
	defaultDialTimeout      = 5 * time.Second
	defaultRequestTimeout   = 30 * time.Second
	defaultDiscoveryRefresh = 60 * time.Second
	defaultMaxParallelOps   = 8
	defaultVirtualNodes     = 750
	defaultMaxMsgSize       = 10 * 1024 * 1024 // 10 MB protobuf message limit
	defaultSocketBufSize    = 0                // 0 = kernel auto-tune (best when host tcp_rmem is tuned)
)

// clientConfig holds all client configuration.
type clientConfig struct {
	servers          []string
	discoveryURL     string
	k8sService       string
	k8sNamespace     string
	port             int
	authAccountName  string
	authAccountKey   string
	hashType         string
	chunkSize        int64
	cachePrefix      string
	maxConnsPerSvr   int
	dialTimeout      time.Duration
	requestTimeout   time.Duration
	discoveryRefresh time.Duration
	maxParallelOps   int
	virtualNodes     int
	socketBufSize    int
}

func defaultConfig() *clientConfig {
	return &clientConfig{
		port:             defaultPort,
		hashType:         "consistent",
		chunkSize:        defaultChunkSize,
		maxConnsPerSvr:   defaultMaxConnsPerSvr,
		dialTimeout:      defaultDialTimeout,
		requestTimeout:   defaultRequestTimeout,
		discoveryRefresh: defaultDiscoveryRefresh,
		maxParallelOps:   defaultMaxParallelOps,
		virtualNodes:     defaultVirtualNodes,
		socketBufSize:    defaultSocketBufSize,
	}
}

// Option configures the distributed cache client.
type Option func(*clientConfig)

// WithServerList sets the initial static list of server addresses (host:port).
func WithServerList(servers []string) Option {
	return func(c *clientConfig) { c.servers = servers }
}

// WithDiscoveryURL sets the discovery endpoint for dynamic server list refresh.
func WithDiscoveryURL(url string) Option {
	return func(c *clientConfig) { c.discoveryURL = url }
}

// WithK8sDiscovery sets Kubernetes headless service discovery parameters.
func WithK8sDiscovery(service, namespace string) Option {
	return func(c *clientConfig) {
		c.k8sService = service
		c.k8sNamespace = namespace
	}
}

// WithPort sets the server port (default 9000).
func WithPort(port int) Option {
	return func(c *clientConfig) { c.port = port }
}

// WithAuth configures authentication credentials.
func WithAuth(accountName, accountKey string) Option {
	return func(c *clientConfig) {
		c.authAccountName = accountName
		c.authAccountKey = accountKey
	}
}

// WithChunkSize sets the chunk size in bytes (default 32 MiB).
func WithChunkSize(size int64) Option {
	return func(c *clientConfig) { c.chunkSize = size }
}

// WithCachePrefix sets the cache key prefix (e.g., "account/container").
func WithCachePrefix(prefix string) Option {
	return func(c *clientConfig) { c.cachePrefix = prefix }
}

// WithMaxConnsPerServer sets the connection pool size per server.
func WithMaxConnsPerServer(n int) Option {
	return func(c *clientConfig) { c.maxConnsPerSvr = n }
}

// WithDialTimeout sets the TCP dial timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.dialTimeout = d }
}

// WithRequestTimeout sets the per-request timeout.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.requestTimeout = d }
}

// WithDiscoveryRefresh sets the server discovery refresh interval.
func WithDiscoveryRefresh(d time.Duration) Option {
	return func(c *clientConfig) { c.discoveryRefresh = d }
}

// WithMaxParallelOps sets the maximum parallel chunk operations for multi-chunk transfers.
func WithMaxParallelOps(n int) Option {
	return func(c *clientConfig) { c.maxParallelOps = n }
}

// WithVirtualNodes sets the number of virtual nodes per server in the hash ring.
func WithVirtualNodes(n int) Option {
	return func(c *clientConfig) { c.virtualNodes = n }
}

// WithSocketBufferSize sets the TCP socket buffer size (SO_RCVBUF/SO_SNDBUF).
// 0 uses the system default. Larger values improve throughput on high-bandwidth links.
func WithSocketBufferSize(bytes int) Option {
	return func(c *clientConfig) { c.socketBufSize = bytes }
}

// uploadConfig holds per-upload options.
type uploadConfig struct {
	ttlSeconds uint32
	groupID    []byte
	metadata   map[string][]byte
	ignoreLock bool
}

// UploadOption configures an upload operation.
type UploadOption func(*uploadConfig)

// WithTTL sets the time-to-live in seconds for the uploaded data.
func WithTTL(seconds uint32) UploadOption {
	return func(c *uploadConfig) { c.ttlSeconds = seconds }
}

// WithGroupID sets the group ID for batch deletion.
func WithGroupID(groupID []byte) UploadOption {
	return func(c *uploadConfig) { c.groupID = groupID }
}

// WithMetadata sets arbitrary key-value metadata for the upload.
func WithMetadata(meta map[string][]byte) UploadOption {
	return func(c *uploadConfig) { c.metadata = meta }
}

// WithIgnoreLock overrides any existing lock on upload.
func WithIgnoreLock(ignore bool) UploadOption {
	return func(c *uploadConfig) { c.ignoreLock = ignore }
}

// downloadConfig holds per-download options.
type downloadConfig struct {
	enableLock bool
}

// DownloadOption configures a download operation.
type DownloadOption func(*downloadConfig)

// WithLock enables the lock-on-miss protocol for stampede prevention.
func WithLock(enable bool) DownloadOption {
	return func(c *downloadConfig) { c.enableLock = enable }
}

// FileMetadata is returned alongside successful downloads.
type FileMetadata struct {
	Size     int64
	Metadata map[string][]byte
}

// FileAttr represents file attributes stored in the distributed cache.
type FileAttr struct {
	IsDir        bool
	Size         uint64
	AccessedTime uint64
	ModifiedTime uint64
}

// FileAttrEntry pairs a filename with its attributes.
type FileAttrEntry struct {
	Filename string
	Attr     FileAttr
}
