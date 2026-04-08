# Design: Distributed Caching for Blobfuse2

| Field | Value |
|---|---|
| **Authors** | |
| **Status** | Implemented |
| **Target** | Blobfuse2 |
| **Last Updated** | 2026-04-08 |

## 1. Summary

This document proposes adding an optional distributed caching layer to Blobfuse2, initially
backed by the Tachyon distributed cache platform (internal project name — may change). The
distributed cache acts as a shared L2 cache between Blobfuse2's existing local disk
cache (L1) and Azure Blob Storage (L3), reducing Azure egress costs and read latency for
multi-node workloads.

> **Note**: "Tachyon" is the current internal name for the distributed cache server. All
> blobfuse code uses generic naming (`dist_cache`, `dcache`) so that renaming the server
> requires no blobfuse code changes.

## 2. Motivation

### Current State

Blobfuse2 provides two single-node cache components:

- **file_cache**: Downloads entire files to local disk. Fast reads after first access, but
  each node independently fetches from Azure.
- **block_cache**: Block-level cache with prefetch. Better for large files and streaming, but
  also node-local.

In multi-node deployments (HPC clusters, Kubernetes pods, training jobs), every node independently
downloads the same data from Azure Blob Storage. There is no sharing of cached data between nodes.

### Problem

- **Redundant egress**: N nodes downloading the same dataset = N× Azure egress cost and bandwidth
- **Cold start latency**: Each node pays the full Azure round-trip on first access
- **No cluster awareness**: Nodes cannot benefit from data another node has already fetched

### Prior Art: feature/dcache Branch

A previous attempt (`feature/dcache`, ~48,000 lines across 112 files) built a custom distributed
storage layer from scratch, including:
- Thrift-based RPC (with acknowledged need to migrate to gRPC)
- Custom cluster management (heartbeat, placement, mirrored/raw volumes)
- Custom replication manager, garbage collector, and metadata manager
- Custom file manager with chunk-based storage

This effort was incomplete: truncate and rename were unsupported, write semantics were limited to
write-through only, and the codebase was large enough to be a maintenance burden. The approach
of building a distributed storage system inside blobfuse is not the right level of abstraction.

### Proposed Solution

Integrate the distributed cache platform as an optional pipeline
component. The server already provides server discovery, consistent hashing, replication, TTL-based
eviction, Kubernetes operators, and Prometheus monitoring. The integration requires ~3,000 lines of Go code (implemented) versus ~48,000 for the custom approach.

## 3. Architecture

### 3.1 Three-Tier Cache Hierarchy

The key insight is that the distributed cache should **complement** blobfuse's existing local cache
rather than replace it. Blobfuse2's chain-of-responsibility pipeline naturally supports this — a
local cache's `NextComponent()` calls flow through the distributed cache before reaching Azure
Storage.

```
┌─────────────────────────────────────────────────────────────────────┐
│  FUSE Kernel Interface                                              │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                    ┌───────────▼──────────┐
                    │   libfuse            │  Priority: Producer (1000)
                    │   FUSE ops adapter   │
                    └───────────┬──────────┘
                                │
                    ┌───────────▼──────────┐
                    │   file_cache (L1)    │  Priority: LevelMid (500)
                    │   Local NVMe/SSD     │  Latency: ~μs
                    │   Per-node           │
                    └───────────┬──────────┘
                                │ NextComponent() on cache miss
                    ┌───────────▼──────────┐
                    │   dist_cache (L2)  │  Priority: LevelMid (500)
                    │   Distributed cluster │  Latency: ~sub-ms to low-ms
                    │   Shared across nodes │
                    └───────────┬──────────┘
                                │ NextComponent() on cache miss
                    ┌───────────▼──────────┐
                    │   attr_cache          │  Priority: LevelTwo (300)
                    │   Attribute caching   │
                    └───────────┬──────────┘
                                │
                    ┌───────────▼──────────┐
                    │   azstorage (L3)     │  Priority: Consumer (100)
                    │   Azure Blob Storage  │  Latency: ~10s of ms
                    └──────────────────────┘
```

### 3.2 Why Stacking Works

Blobfuse2's pipeline priority check uses strict greater-than (`>`), not greater-than-or-equal.
Two components at the same priority level (LevelMid = 500) can coexist in the pipeline.
`ValidatePipeline()` only blocks known-incompatible pairs (file_cache+block_cache,
file_cache+xload, block_cache+xload) — it does not block file_cache+dist_cache.

The local cache component (file_cache or block_cache) requires **zero modifications**. On a
cache miss, it calls `NextComponent().CopyToFile()` which reaches dist_cache instead of
azstorage directly. The dist_cache component transparently serves from the distributed cache
on hit, or forwards to azstorage on miss and populates the distributed cache for future requests.

### 3.3 Data Flow

#### Read Path (L1 miss, L2 hit)

```
1. App reads file via FUSE
2. libfuse → file_cache.ReadInBuffer()
3. file_cache: local file not present → calls NextComponent().CopyToFile()
4. dist_cache.CopyToFile():
   a. Send Download request to distributed cache server (selected by consistent hash of filename)
   b. Server responds with file data → write to local temp file → return success
5. file_cache: reads from local file, caches for future requests
```

#### Read Path (L1 miss, L2 miss, L3 fetch — with stampede prevention)

The distributed cache server supports a **lock-on-miss protocol**: when a Download request includes
`enableLock=true`, a miss returns `NOT_FOUND_GOT_LOCK` to the first requester and
`NOT_FOUND_ALREADY_LOCKED` to subsequent requesters for the same file. This prevents cache
stampedes — only one node in the cluster downloads from Azure for any given file.

```
1–3. Same as above
4. dist_cache.CopyToFile():
   a. Send Download request to the distributed cache with enableLock=true
   b. Response: NOT_FOUND_GOT_LOCK (this node owns the miss)
   c. Forward CopyToFile to NextComponent() (azstorage)
   d. azstorage downloads from Azure Blob → writes to local temp file
   e. dist_cache uploads file data to the distributed cache (populates cache for cluster)
   f. Return success
5. file_cache: reads from local file, caches for future requests
```

#### Read Path (L1 miss, L2 miss, another node is fetching)

```
1–3. Same as above
4. dist_cache.CopyToFile():
   a. Send Download request to the distributed cache with enableLock=true
   b. Response: NOT_FOUND_ALREADY_LOCKED (another node is populating)
   c. Poll/retry Download until SUCCESS or timeout
   d. On SUCCESS → write cached data to file → return success
   e. On timeout → fall through to NextComponent() (azstorage) as fallback
5. file_cache: reads from local file, caches for future requests
```

This approach uses the server's lock protocol for stampede prevention while keeping the
pipeline clean — blobfuse's azstorage component handles all Azure Blob downloads using
its existing authentication. dist_cache does not need any blob storage configuration.

#### Write Path

```
1. App writes file via FUSE
2. libfuse → file_cache.WriteFile() → writes to local file
3. App closes/flushes → file_cache.FlushFile()
4. file_cache calls NextComponent().CopyFromFile()
5. dist_cache.CopyFromFile():
   a. Forward CopyFromFile to NextComponent() (azstorage) — write-through first
   b. On success, upload to the distributed cache (populates distributed cache for other nodes)
   c. Return result from azstorage
```

#### Delete / Rename / Truncate

```
1. dist_cache: invalidate entry in the distributed cache (Delete request)
2. Forward operation to NextComponent() (azstorage)
```

### 3.4 Stampede Prevention via Lock Protocol

The server's `Download(enableLock=true)` provides cluster-wide coordination on cache misses.
This is the same pattern used by the StreamingClient in other integrations, but
implemented purely in Go using the lock responses from the protobuf protocol:

| Response | Meaning | dist_cache action |
|---|---|---|
| `SUCCESS` | Cache hit | Serve data to caller |
| `NOT_FOUND_GOT_LOCK` | Cache miss, you own it | Download from azstorage, upload to the distributed cache |
| `NOT_FOUND_ALREADY_LOCKED` | Miss, another node is fetching | Retry with backoff until SUCCESS or timeout |
| `NOT_FOUND` | Miss, no lock available | Download from azstorage (don't populate cache) |

**Why this matters**: In an N-node cluster where all nodes open the same file simultaneously,
without locking all N nodes independently download from Azure (N× egress cost). With the lock
protocol, exactly one node downloads from Azure and the other N-1 nodes read from the distributed cache once
the first node populates the cache.

**Why not use the C++ StreamingClient directly**: StreamingClient includes its own Azure
Blob client, which would duplicate blobfuse's existing azstorage component and require Azure
credentials to be configured twice. By implementing just the lock protocol in Go, we reuse
blobfuse's azstorage for all Azure operations — no credential duplication, no C++ dependency.

### 3.5 Component Interface

dist_cache implements `internal.Component` by embedding `internal.BaseComponent` and
overriding only the methods that file_cache (or block_cache) calls on its NextComponent:

| file_cache calls | dist_cache behavior |
|---|---|
| `CopyToFile(name, offset, count, file)` | Download from the distributed cache; on miss forward to azstorage and populate |
| `CopyFromFile(name, file)` | Forward to azstorage, then upload to the distributed cache |
| `GetAttr(name)` | Check distributed cache attributes; forward to azstorage for freshness |
| `DeleteFile(name)` | Invalidate in the distributed cache + forward to azstorage |
| `RenameFile(src, dst)` | Invalidate old key + forward to azstorage |
| `TruncateFile(name, size)` | Invalidate in the distributed cache + forward to azstorage |
| `CreateFile(name)` | Forward to azstorage (create not cached) |
| `ReadDir / StreamDir` | Forward to azstorage (directories not cached) |
| `DeleteDir / RenameDir` | Forward to azstorage |
| `Chmod / Chown / SyncFile` | Forward to azstorage |
| `IsDirEmpty` | Forward to azstorage |

All non-overridden methods inherit the BaseComponent pass-through behavior.

### 3.6 Graceful Degradation

If the distributed cache cluster is unreachable or returns errors:

1. **On read miss**: Forward directly to azstorage (skip cache population)
2. **On read hit failure**: Forward to azstorage as fallback
3. **On write**: Forward to azstorage regardless (writes always go through)
4. **On invalidation failure**: Log warning, continue (stale data will expire via TTL)

The local file_cache continues to work independently. Loss of the distributed cache cluster degrades
performance to the current baseline (each node fetches from Azure independently) but does not
cause failures.

### 3.7 Deployment Topologies

#### Topology A: File Cache + Distributed Cache (Recommended for general workloads)

```yaml
components:
  - libfuse
  - file_cache        # L1 local
  - dist_cache      # L2 distributed
  - attr_cache
  - azstorage          # L3 Azure
```

Best for: Multi-node clusters, Kubernetes, HPC. Combines local speed with distributed sharing.
Whole-file caching suits workloads that read files end-to-end (e.g., config files, scripts,
small-to-medium data files).

#### ~~Topology B: Distributed-Only (No Local Cache)~~ — NOT SUPPORTED

> **Note**: dist_cache cannot currently operate without block_cache or file_cache. FUSE sends
> small reads (1 MB via `max_read`) but dist_cache works with 32 MB chunks — this buffer size
> mismatch causes failures. Making dist_cache standalone would require reimplementing block
> management. Use Topology A or B instead.

#### Topology B: Block Cache + Distributed Cache (Recommended for large-file / ML workloads)

```yaml
components:
  - libfuse
  - block_cache        # L1 block-level local
  - dist_cache      # L2 distributed
  - azstorage
```

Best for: Large file workloads (ML training datasets, checkpoints) where block-level access
patterns dominate. block_cache calls `NextComponent().ReadInBuffer()` for block misses and
`NextComponent().StageData()`/`CommitData()` for uploads, all of which dist_cache intercepts.

**Topologies A and B are both recommended** — choose based on workload. Both use the same
chunk keys in the distributed cache (derived from `block_cache.block-size-mb`), so nodes running Topology A
and nodes running Topology B can share the same distributed cache cluster with no data duplication.

### 3.8 file_cache vs block_cache: Integration Requirements

Blobfuse2 has two local cache components that use fundamentally different data access patterns.
dist_cache must support both to be a universal distributed cache layer.

#### file_cache (Whole-File)

file_cache downloads entire files to local disk on first open and uploads them back on flush.
It calls these methods on NextComponent:

| Method | When | Data Unit |
|---|---|---|
| `CopyToFile(name, offset, count, file)` | Cache miss — download file | Whole file |
| `CopyFromFile(name, file)` | Flush — upload dirty file | Whole file |
| `GetAttr(name)` | Freshness check on open | Metadata only |
| `DeleteFile / RenameFile / TruncateFile` | Mutation | N/A |
| `ReadDir / StreamDir / DeleteDir / RenameDir` | Directory ops | N/A |
| `Chmod / Chown / SyncFile` | Metadata | N/A |

This maps directly to the distributed cache's Upload/Download API — a natural fit.

#### block_cache (Block-Level, configurable block size)

block_cache downloads individual blocks (default 16 MB, recommended 32 MB for production) on demand with prefetch, and uploads
via staged block commits. It calls these methods on NextComponent:

| Method | When | Data Unit |
|---|---|---|
| `ReadInBuffer(handle, offset, data, etag)` | Block miss — download 16 MB | Single block |
| `StageData(name, data, offset, id)` | Upload one dirty block | Single block |
| `CommitData(name, blockList, blockSize)` | Finalize staged blocks | Block list |
| `GetAttr(name)` | Open — get file size/metadata | Metadata only |
| `GetCommittedBlockList(name)` | Open — get existing block layout | Block list |
| `CreateFile(name)` | New file creation | N/A |
| `DeleteFile / RenameFile / TruncateFile` | Mutation | N/A |
| `DeleteDir / RenameDir` | Directory ops | N/A |

This maps to the distributed cache's chunk-level storage — each block is one distributed cache chunk at the same offset.

Note: `stream` mode is an alias for block_cache with different config keys — the same code runs.
`xload` is a separate read-only preload component and is mutually exclusive with both caches.

#### Unified Chunk Keys — No Data Duplication

A critical design property: **both cache modes produce identical distributed cache chunk keys** for the
same data. There is no data duplication or block translation in the cache.

Distributed cache chunk key format: `SHA256(cachePrefix/filePath:chunkOffset:chunkSize)`

block_cache always uses aligned offsets (`block.offset = index * blockSize`, line 1224 of
`block_cache.go`), and the Go client uses the same aligned offsets for chunk keys.

```
File: "container/data/model.bin" (50 MB, chunkSize = blockSize = 16 MiB)

                    CopyToFile                ReadInBuffer
                    (file_cache)              (block_cache)
Chunk 0 (offset 0):    SHA256(...:0:16MiB)    SHA256(...:0:16MiB)         ← same key
Chunk 1 (offset 16M):  SHA256(...:16M:16MiB)  SHA256(...:16M:16MiB)      ← same key
Chunk 2 (offset 32M):  SHA256(...:32M:16MiB)  SHA256(...:32M:16MiB)      ← same key
Chunk 3 (offset 48M):  SHA256(...:48M:16MiB)  SHA256(...:48M:16MiB)      ← same key (2MB data)
```

This enables **cross-pipeline cache sharing**:
- Node A (file_cache pipeline) caches a file → all chunks populated in the distributed cache
- Node B (block_cache pipeline) reads block 2 → L2 hit on the same chunk
- Node C (block_cache pipeline) caches only block 0 → Node D (file_cache pipeline) downloads
  all chunks, block 0 is already an L2 hit

**Chunk size derivation**: dist_cache resolves chunk size using this priority chain:
`block_cache.block-size-mb` > `stream.block-size-mb` > `dist_cache.chunk-size-mb` > default (16 MiB).
When block_cache is in the pipeline, its block-size-mb takes precedence automatically.
The `dist_cache.chunk-size-mb` setting serves as a fallback when used with file_cache
(where no block_cache is present). This eliminates the possibility of mismatched sizes
within a pipeline — there is exactly one effective knob to tune.

#### Implementation Phasing

Because the Go client implements chunking at the protocol level (matching the server's
StreamingClient), both cache modes are supported from Phase 1 — no separate block_cache
integration phase is needed:

- **file_cache calls `CopyToFile`** → Go client downloads all chunks, reassembles in order
- **file_cache calls `CopyFromFile`** → Go client splits into 16 MiB chunks, uploads each
- **block_cache calls `ReadInBuffer(offset, 16MB)`** → Go client downloads one chunk (1:1)
- **block_cache calls `StageData(offset, data)`** → Go client uploads one chunk

With chunk size aligned to block size, each block_cache operation maps to exactly one
distributed cache chunk. No block-key-scheme invention or data translation needed.

Directory operations, metadata ops, and invalidation ops pass through to NextComponent
via BaseComponent regardless of which local cache is in the pipeline.

### 4.1 Go Client Package — Blobfuse Repo (Initially)

The Go client for the distributed cache protocol will live **in the blobfuse repository** initially, at
`internal/dist_cache_client/`. The upstream distributed cache repository is not yet open source,
making it impractical to import as an external Go module. The package should be designed for
future extraction once the upstream repo is open-sourced.

| Phase | Location | Rationale |
|---|---|---|
| **Now** | `internal/dist_cache_client/` in blobfuse | Upstream repo not yet OSS; avoids import complications |
| **Later** | `sdk/go/dcache/` in upstream repo | Enables reuse across projects; proto co-evolution; server CI |

**Design for portability**: The package should have **no imports from blobfuse** (`internal/`,
`common/`, `component/`). It should be a self-contained client that communicates via the distributed cache
wire protocol only. This makes the future move a file copy + module path rename with no code
changes.

#### Blobfuse Repo Layout

```
azure-storage-fuse/
├── internal/
│   └── dist_cache_client/
│       ├── client.go                 # High-level client (Upload, Download, Delete, GetAttr)
│       ├── client_test.go
│       ├── connection.go             # Connection pooling, TCP framing
│       ├── connection_test.go
│       ├── discovery.go              # Server discovery + consistent hashing
│       ├── discovery_test.go
│       ├── chunking.go               # Client-side chunking (matching StreamingClient)
│       ├── options.go                # Functional options for client config
│       ├── errors.go                 # Typed errors
│       └── proto/
│           ├── cache.proto           # Copied from the distributed cache common/proto/
│           ├── cache.pb.go           # Generated
│           └── generate.go           # //go:generate protoc ...
```

The existing protobuf types in the upstream repo's `test/pkg/scenario/cachepb/` and protocol code in
`cache_evict.go` serve as the foundation. The proto file is copied (not symlinked) since the
upstream repo may not be available on all build machines.

#### Go Client API (Proposed)

The client implements chunked storage internally — callers work with whole files or byte ranges,
and the client handles chunk splitting, per-chunk server selection, and reassembly.

```go
package dcache

// Client is a distributed cache client with chunked storage, connection pooling,
// and server discovery. Files are automatically split into fixed-size chunks
// and distributed across the cluster via consistent hashing.
type Client struct { ... }

// Option configures the client.
type Option func(*clientConfig)

// New creates a new distributed cache client.
func New(opts ...Option) (*Client, error)

func WithServerList(servers []string) Option
func WithDiscoveryURL(url string) Option
func WithPort(port int) Option
func WithAuth(accountName, accountKey string) Option
func WithHashType(hashType string) Option           // "consistent" or "modulo"
func WithChunkSize(size int64) Option                // default 16 MiB; 32 MiB recommended for production
func WithCachePrefix(prefix string) Option           // e.g., "accountName/containerName"
func WithMaxConnsPerServer(n int) Option
func WithDialTimeout(d time.Duration) Option
func WithRequestTimeout(d time.Duration) Option

// Upload stores a file in the distributed cache, splitting it into chunks.
// Each chunk is routed to a server via consistent hash of its chunk key.
func (c *Client) Upload(ctx context.Context, filename string, data io.Reader, size int64, opts ...UploadOption) error

// Download retrieves a complete file from the distributed cache, reassembling chunks.
// Returns ErrNotFound if any chunk is missing.
func (c *Client) Download(ctx context.Context, filename string, w io.Writer, opts ...DownloadOption) (*FileMetadata, error)

// DownloadChunk retrieves a single chunk at the given offset.
// When chunkSize-aligned, this is a 1:1 mapping to one distributed cache entry.
// Ideal for block_cache integration where each ReadInBuffer = one chunk.
func (c *Client) DownloadChunk(ctx context.Context, filename string, offset int64, buf []byte, opts ...DownloadOption) (int, error)

// UploadChunk stores a single chunk at the given offset.
// Ideal for block_cache StageData integration.
func (c *Client) UploadChunk(ctx context.Context, filename string, offset int64, data []byte, opts ...UploadOption) error

// Delete removes all chunks of a file from the distributed cache.
func (c *Client) Delete(ctx context.Context, filename string, fileSize int64) error

// DeleteGroup removes all files with the given group ID.
func (c *Client) DeleteGroup(ctx context.Context, groupID []byte) error

// GetAttr retrieves file attributes from the distributed cache.
func (c *Client) GetAttr(ctx context.Context, filename string) (*FileAttribute, error)

// PutAttr stores file attributes in the distributed cache.
func (c *Client) PutAttr(ctx context.Context, attrs []FileAttributes) error

// Close shuts down the client and releases all connections.
func (c *Client) Close() error

// UploadOption configures an upload operation.
type UploadOption func(*uploadConfig)

func WithTTL(seconds uint32) UploadOption
func WithGroupID(groupID []byte) UploadOption
func WithMetadata(meta map[string][]byte) UploadOption
func WithIgnoreLock(ignore bool) UploadOption

// FileMetadata is returned alongside downloads.
type FileMetadata struct {
    Size     int64
    Metadata map[string][]byte
}

// FileAttribute represents file attributes stored in the distributed cache.
type FileAttribute struct {
    IsDir        bool
    Size         uint64
    AccessedTime uint64
    ModifiedTime uint64
}

// Sentinel errors
var (
    ErrNotFound              = errors.New("dcache: file not found")
    ErrNotFoundGotLock       = errors.New("dcache: not found, lock acquired")
    ErrNotFoundAlreadyLocked = errors.New("dcache: not found, locked by another client")
    ErrAuthFailed            = errors.New("dcache: authentication failed")
    ErrServerError           = errors.New("dcache: internal server error")
)

// DownloadOption configures a download operation.
type DownloadOption func(*downloadConfig)

func WithLock(enable bool) DownloadOption   // enable lock-on-miss protocol
```

### 4.2 Wire Protocol

The distributed cache uses a custom framing protocol over TCP:

```
┌──────────────┬─────────────────────────┬──────────────────────┐
│ 4-byte       │ Protobuf Request        │ Raw data bytes       │
│ big-endian   │ message                 │ (Upload only)        │
│ length       │                         │                      │
└──────────────┴─────────────────────────┴──────────────────────┘
```

- **Request**: 4-byte length prefix (of the protobuf message only), then serialized `Request`
  protobuf, then raw file bytes for uploads.
- **Response**: 4-byte length prefix, then serialized response protobuf, then raw file bytes
  for downloads.
- **Connection**: Plain TCP. No TLS (within cluster). No HTTP. No gRPC framing.

The protocol is stateless per-request — each request is independent. Connections can be pooled
and reused.

### 4.3 Chunked Storage and Server Selection

The distributed cache stores files as **fixed-size chunks** distributed across the cluster. The chunking is
performed client-side (matching the C++ StreamingClient behavior):

```
File: "container/path/to/blob" (100 MB)
  → Chunk 0: key=SHA256("account/container/path:0:16777216")        → Server A
  → Chunk 1: key=SHA256("account/container/path:16777216:16777216") → Server C
  → Chunk 2: key=SHA256("account/container/path:33554432:16777216") → Server B
  → ...
```

**Chunk key format**: `SHA256(cachePrefix/filePath:chunkOffset[:chunkSize])`
- Each chunk hashes to a potentially different server via consistent hashing
- This distributes load across the cluster — a 1 GB file spreads across many servers
- The non-default chunk size is appended to the key to avoid collisions

**Default chunk size**: The server defaults to 4 MiB. For blobfuse integration, we recommend
**32 MiB** to align with block_cache's recommended production block size (`block-size-mb: 32`).
The default in blobfuse is 16 MiB (matching block_cache's built-in default), but production
deployments should use 32 MiB for optimal throughput. This alignment means:

```
block_cache ReadInBuffer(offset=N×32MB, size=32MB) = exactly one distributed cache chunk
```

This eliminates the need for separate block-level integration logic — the Go client's chunking
naturally serves both file_cache (multi-chunk reassembly) and block_cache (single-chunk fetch).

**Server selection**: SHA256-based consistent hashing with 750 virtual nodes per server,
matching the C++ client's `ConsistentHasher` implementation.

### 4.4 Unified Chunking Serves Both Cache Modes

Because the Go client implements chunking (like StreamingClient), a single implementation
handles both blobfuse cache modes:

| blobfuse cache | Operation | Go client behavior |
|---|---|---|
| **file_cache** `CopyToFile` | Download whole file | Download all chunks, reassemble in order |
| **file_cache** `CopyFromFile` | Upload whole file | Split into 16 MB chunks, upload each |
| **block_cache** `ReadInBuffer` | Read 16 MB block at offset N | Download chunk N (1:1 mapping) |
| **block_cache** `StageData` | Write 16 MB block at offset N | Upload as chunk N |

With 16 MB chunk alignment, block_cache integration is **trivial** — no block-key-scheme
invention needed. The same chunked client API serves both modes from Phase 1.

### 4.5 Performance Design

The C++ client uses standard `send()`/`recv()` with `poll()` — no client-side zero-copy
(`sendfile`/`splice`/`mmap`). The only `sendfile()` is server-side for downloads. This means
a well-optimized Go client can match or exceed C++ client throughput.

#### Go Performance Advantages

| Technique | Go | C++ client |
|---|---|---|
| **Download: socket→file** | `io.Copy(file, conn)` uses `splice(2)` — zero-copy kernel-to-kernel | `recv()` → userspace buffer → `write()` (two copies) |
| **Chunk parallelism** | Goroutine pool (µs overhead per task) | `std::async` (thread-per-chunk, higher overhead) |
| **Connection pooling** | `sync.Pool` or channel-based pool | `queue<unique_ptr<IConnection>>` (equivalent) |
| **SHA256 hashing** | Hardware-accelerated (SHA-NI) automatically | Same |

#### Required Optimizations

1. **`splice(2)` for downloads**: When writing to a file, use `io.Copy(file, tcpConn)`
   rather than reading into a `[]byte` buffer then writing. Go's standard library detects
   `*net.TCPConn` → `*os.File` and uses `splice(2)` for zero-copy. This avoids the
   userspace buffer entirely — data flows kernel-to-kernel. The C++ client does NOT do this.

   ```go
   // GOOD — triggers splice(2) zero-copy
   n, err := io.Copy(localFile, tcpConn)

   // BAD — two copies through userspace
   buf := make([]byte, 16*1024*1024)
   n, err := tcpConn.Read(buf)
   localFile.Write(buf[:n])
   ```

   **Caveat**: splice only works when copying directly between the TCP connection and the
   file. If we need to read the protobuf header first, we must read the header bytes
   separately, then splice the remaining data bytes. Use `io.LimitReader` to bound the
   splice to the exact data size from the protobuf response.

2. **Buffer pooling**: Use `sync.Pool` for chunk-sized buffers (16 MiB) to avoid GC
   pressure on upload paths and non-file download paths (e.g., `ReadInBuffer` where data
   goes to a caller-provided `[]byte`, not a file).

   ```go
   var chunkPool = sync.Pool{
       New: func() any { return make([]byte, 16*1024*1024) },
   }
   ```

3. **Parallel chunk transfers**: Bounded goroutine pool for multi-chunk downloads
   (file_cache `CopyToFile`). Single-chunk operations (block_cache `ReadInBuffer`) don't
   need parallelism. Use `errgroup.Group` with `SetLimit()`:

   ```go
   g, ctx := errgroup.WithContext(ctx)
   g.SetLimit(maxParallelChunks)  // e.g., 8
   for i := range chunks {
       g.Go(func() error { return tc.downloadChunk(ctx, chunks[i]) })
   }
   ```

4. **TCP tuning**: Match the C++ client settings:
   - `TCP_NODELAY` — set via `tcpConn.SetNoDelay(true)` (Go default is already true)
   - Socket buffer sizes — `SetReadBuffer()`/`SetWriteBuffer()` if needed
   - Connection keep-alive — `SetKeepAlive(true)`, `SetKeepAlivePeriod(30s)`

5. **Minimise allocations on hot path**: Pre-allocate protobuf response structs and reuse
   them per-connection. Use `proto.UnmarshalOptions{AllowPartial: true}` to avoid
   validation overhead.

6. **Pipelining** (future): For multi-chunk file downloads, send all Download requests
   before reading any responses. The server processes requests sequentially per connection,
   but pipelining eliminates one RTT per chunk. The C++ client does NOT do this.

#### Performance Validation — Actual Results

Benchmarked on 3× Standard_E192ids_v6 (192 vCPU, 1.8 TiB RAM, 200 Gbps NIC) in AKS
with MTU 9000 and TCP tuning (BBR, 64 MB socket buffers). Pod-to-pod: 64 Gbps measured.

##### Go Client Direct (no FUSE overhead)

| File Size | Throughput | Configuration |
|-----------|-----------|---------------|
| 128 MB | 4,512 MB/s | P8 / C32 / S0 |
| 256 MB | 5,030 MB/s | P8 / C32 / S0 |
| 512 MB | 5,267 MB/s | P8 / C32 / S0 |
| 1 GB | 5,927 MB/s | P8 / C32 / S0 |

##### Blobfuse End-to-End (block_cache + dist_cache, 32 MB chunks)

Cold reads = first access (from Azure Blob or dist_cache). Warm reads = kernel page cache.

| File Size | Baseline Cold | dcache Cold | Cold Speedup | Baseline Warm | dcache Warm |
|-----------|--------------|-------------|-------------|---------------|-------------|
| 128 MB | 294 MB/s | 1,280 MB/s | **4.4×** | 4,129 MB/s | 4,129 MB/s |
| 256 MB | 396 MB/s | 1,910 MB/s | **4.8×** | 4,413 MB/s | 4,266 MB/s |
| 512 MB | 651 MB/s | 1,605 MB/s | **2.5×** | 4,530 MB/s | 4,571 MB/s |
| 1 GB | 895 MB/s | 1,689 MB/s | **1.9×** | 4,551 MB/s | 4,675 MB/s |
| 5 GB | 1,459 MB/s | 2,480 MB/s | **1.7×** | 6,044 MB/s | 5,988 MB/s |
| 10 GB | 1,950 MB/s | 2,265 MB/s | **1.2×** | 4,818 MB/s | 4,904 MB/s |

Cross-node reads (Node B reading data cached by Node A):

| File Size | Baseline Cold | dcache Cold | Cold Speedup |
|-----------|--------------|-------------|-------------|
| 128 MB | 305 MB/s | 1,376 MB/s | **4.5×** |
| 256 MB | 405 MB/s | 1,651 MB/s | **4.1×** |
| 512 MB | 642 MB/s | 1,712 MB/s | **2.7×** |
| 1 GB | 1,039 MB/s | 2,048 MB/s | **2.0×** |
| 5 GB | 1,672 MB/s | 2,249 MB/s | **1.3×** |
| 10 GB | 1,499 MB/s | 1,997 MB/s | **1.3×** |

**Key findings**:
- dist_cache cold reads are **1.2–4.5× faster** than blob baseline
- Smaller files benefit most (blob latency dominates at small sizes)
- Warm reads converge to ~4–6 GB/s regardless of config (kernel page cache dominates)
- Cross-node sharing eliminates redundant Azure egress entirely

## 5. Blobfuse Component Design

### 5.1 Registration

```go
package dist_cache

const compName = "dist_cache"

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

    // Also support DIST_CACHE_SERVER_LIST env var
    config.BindEnv(compName+".server-list", "DIST_CACHE_SERVER_LIST")

    // ... additional flag bindings ...
}
```

### 5.2 Configuration & Server Discovery

The distributed cache server supports two auto-discovery mechanisms that eliminate the need
for static server lists in most deployments:

#### Discovery Priority (first match wins)

1. **Discovery endpoint** (`discovery-url`): Connect to a single known endpoint, call the
   `GetCacheServers` RPC, receive the full server list. The client refreshes the list
   periodically in the background (`discovery-refresh-sec`, default 60s). This is the
   recommended approach for production — it handles cluster scaling and failover automatically.

2. **Kubernetes DNS** (`k8s-service` + `k8s-namespace`): Resolve a headless StatefulSet
   service. The cache controller creates pods named `cacheserver-{0,1,...}.cacheserver.<ns>.svc.cluster.local`.
   The Go client enumerates ordinals 0..N-1 using DNS A record lookups. No discovery endpoint
   needed — just the service name and namespace.

3. **Static server list** (`server-list`): Comma-separated list of `host:port` addresses.
   Fallback for bare-metal or non-K8s environments where discovery is not available.

4. **Environment variable** (`DIST_CACHE_SERVER_LIST`): Same as static list, set via env.
   Useful for container injection without config file changes.

If none of the above are configured, `Configure()` returns an error.

```go
type DistCacheOptions struct {
    // Discovery (preferred — auto-detects servers)
    DiscoveryURL       string `config:"discovery-url"        yaml:"discovery-url,omitempty"`
    DiscoveryRefreshSec int    `config:"discovery-refresh-sec" yaml:"discovery-refresh-sec,omitempty"`

    // Kubernetes DNS discovery
    K8sService   string `config:"k8s-service"   yaml:"k8s-service,omitempty"`
    K8sNamespace string `config:"k8s-namespace" yaml:"k8s-namespace,omitempty"`

    // Static fallback
    ServerList string `config:"server-list" yaml:"server-list,omitempty"`

    // Common options
    Port           int    `config:"port"            yaml:"port,omitempty"`
    TTLSeconds     uint32 `config:"ttl-seconds"     yaml:"ttl-seconds,omitempty"`
    MaxFileSizeMB  int    `config:"max-file-size-mb" yaml:"max-file-size-mb,omitempty"`
    AuthAccountName string `config:"auth-account-name" yaml:"auth-account-name,omitempty"`
    AuthAccountKey  string `config:"auth-account-key"  yaml:"auth-account-key,omitempty"`
    HashType        string `config:"hash-type"       yaml:"hash-type,omitempty"`
    BypassOnError   bool   `config:"bypass-on-error" yaml:"bypass-on-error,omitempty"`
    // Chunk size for distributed cache operations. When block_cache is present,
    // this is overridden by block_cache.block-size-mb to keep alignment consistent.
    // When used with file_cache (no block_cache), this is the primary chunk size config.
    ChunkSizeMB     float64 `config:"chunk-size-mb" yaml:"chunk-size-mb,omitempty"`
    // Cache prefix for generating chunk keys (e.g., "accountName/containerName")
    CachePrefix     string `config:"cache-prefix"  yaml:"cache-prefix,omitempty"`
    // Maximum connections per cache server for connection pooling
    MaxConnsPerSvr  int    `config:"max-conns-per-server" yaml:"max-conns-per-server,omitempty"`
}
```

YAML examples:

```yaml
# Kubernetes deployment (most common) — discovery endpoint from the cache controller
dist_cache:
  discovery-url: "cacheserver-discovery.my-namespace.svc.cluster.local:9000"
  bypass-on-error: true

# Kubernetes deployment — DNS-based (no discovery service needed)
dist_cache:
  k8s-service: cacheserver
  k8s-namespace: my-namespace
  port: 9000
  bypass-on-error: true

# Bare-metal / VM deployment — static list
dist_cache:
  server-list: "dcache-0:9000,dcache-1:9000,dcache-2:9000"
  bypass-on-error: true
```

### 5.3 Core Operations

#### CopyToFile (Read Path — Most Critical)

```go
func (tc *DistCache) CopyToFile(options internal.CopyToFileOptions) error {
    // Try distributed cache with lock-on-miss enabled
    meta, err := tc.client.Download(ctx, options.Name, options.File, dcache.WithLock(true))
    if err == nil {
        // L2 hit — data written to file
        return nil
    }

    if errors.Is(err, dcache.ErrNotFoundGotLock) {
        // We own this miss — download from Azure via azstorage
        err = tc.NextComponent().CopyToFile(options)
        if err != nil {
            return err
        }
        // Populate the distributed cache for other nodes (best-effort)
        go tc.uploadToCache(options.Name, options.File)
        return nil
    }

    if errors.Is(err, dcache.ErrNotFoundAlreadyLocked) {
        // Another node is fetching — poll until available or timeout
        meta, err = tc.pollUntilCached(ctx, options.Name, options.File)
        if err == nil {
            return nil
        }
        // Timeout — fall through to azstorage
        return tc.NextComponent().CopyToFile(options)
    }

    if tc.bypassOnError {
        log.Warn("dist_cache: error, bypassing: %v", err)
        return tc.NextComponent().CopyToFile(options)
    }
    return err
}
```

#### CopyFromFile (Write Path)

```go
func (tc *DistCache) CopyFromFile(options internal.CopyFromFileOptions) error {
    // Write-through to azstorage first (source of truth)
    err := tc.NextComponent().CopyFromFile(options)
    if err != nil {
        return err
    }

    // Populate distributed cache (best-effort, async)
    go tc.uploadToCache(options.Name, options.File)

    return nil
}
```

#### DeleteFile / RenameFile (Invalidation)

```go
func (tc *DistCache) DeleteFile(options internal.DeleteFileOptions) error {
    // Invalidate in the distributed cache (best-effort)
    _ = tc.client.Delete(ctx, options.Name)

    // Forward to azstorage
    return tc.NextComponent().DeleteFile(options)
}

func (tc *DistCache) RenameFile(options internal.RenameFileOptions) error {
    // Invalidate old name in the distributed cache
    _ = tc.client.Delete(ctx, options.Src)

    // Forward to azstorage
    return tc.NextComponent().RenameFile(options)
}
```

## 6. Cache Coherency

### 6.1 Freshness Model

Distributed cache entries include metadata and a TTL:

| Mechanism | Purpose |
|---|---|
| **TTL** | Automatic expiry. Configurable per-upload, default from config. |
| **ETag/LastModified metadata** | Stored in distributed cache metadata map on upload. Compared with azstorage GetAttr on access. |
| **Explicit invalidation** | Delete/Rename/Truncate immediately invalidate Distributed cache entries. |
| **Group invalidation** | `DeleteGroup` can invalidate all files for a container at once. |

### 6.2 Consistency Guarantees

- **Read-after-write** (same node): Guaranteed — file_cache (L1) has the file locally.
- **Read-after-write** (different node): Guaranteed if the write populates distributed cache synchronously.
  With async population, there is a small window where another node may get a miss.
- **Read-after-delete**: Guaranteed — delete invalidates distributed cache before forwarding to azstorage.
- **External writes** (writes to Azure outside blobfuse): Visible after TTL expires or on
  attr_cache miss triggering a fresh GetAttr from azstorage.

### 6.3 Trade-offs

The three-tier model prioritizes **availability and performance** over strict consistency,
matching blobfuse2's existing eventual-consistency model. Blobfuse2 already accepts that
external writes to Azure may not be visible until cache timeout expires — the distributed cache adds another
caching tier with the same semantics.

## 7. Alternatives Considered

### 7.1 CGo Binding to libCacheClientSharedLib.so

The server provides a C-compatible API via `libCacheClientSharedLib.so`. A CGo wrapper would give
direct access to the production C++ client including its connection pooling, consistent hashing,
and retry logic.

**Rejected because:**
- Adds C/C++ toolchain dependency to blobfuse2 build
- Complicates cross-compilation (blobfuse2 supports x86_64 and ARM64)
- Runtime dependency on shared library installation
- CGo disables some Go runtime optimizations and complicates debugging
- The protocol is simple enough to implement in pure Go

### 7.2 Replacing Local Cache

Instead of stacking, dist_cache could replace file_cache entirely.

**Rejected because:**
- Local cache provides microsecond reads — the distributed cache adds network latency
- Local cache works offline — the distributed cache requires cluster availability
- Stacking gives the best of both: local speed + distributed sharing
- Zero changes needed to file_cache

### 7.3 Continue feature/dcache Development

Continue building the custom distributed cache in the dcache branch.

**Rejected because:**
- ~48,000 lines of custom distributed systems code to maintain
- Incomplete (no truncate, limited rename, write-through only)
- Reimplements what the distributed cache already provides (cluster management, replication, eviction)
- Thrift RPC with acknowledged need to migrate to gRPC
- Maintenance burden falls entirely on the blobfuse team

### 7.4 gRPC Wrapper Around Distributed Cache

Add a gRPC service in front of the distributed cache server and use standard gRPC from Go.

**Rejected because:**
- Adds an extra network hop and deployment component
- The native protocol is simpler than gRPC (no HTTP/2, no streaming frames)
- The 4-byte-length-prefix + protobuf format is trivial to implement in Go

## 8. Implementation Plan

### Phase 1: Go Client Package ✅ Complete

Created `internal/dist_cache_client/` — a self-contained Go client with no blobfuse imports:
- Copy protobuf types from the distributed cache's `test/pkg/scenario/cachepb/`
- Implement full client API (see §8.1 for full protocol surface)
- Client-side chunking matching StreamingClient behavior
- Connection pooling with configurable limits
- Server discovery via GetCacheServers RPC
- Consistent hashing (SHA256, 750 virtual nodes, matching C++ client)
- Lock protocol (Download with enableLock for stampede prevention)
- Functional options pattern for configuration
- Unit tests with mock TCP server
- Compatibility validation (see §8.2)
- Design for portability: no `internal/`, `common/`, or `component/` imports so the
  package can be moved to the upstream repo once it is open-sourced

#### 8.1 Protocol Surface Coverage

The Go client must implement the following protocol operations (from `cache.proto`).
Operations marked ★ are required for initial integration; others can be deferred.

| Operation | Proto Messages | Purpose | Priority |
|---|---|---|---|
| **Upload** ★ | `UploadRequest/Response` | Store chunk data | Required |
| **Download** ★ | `DownloadRequest/Response` | Retrieve chunk data (with lock protocol) | Required |
| **Delete** ★ | `DeleteRequest/Response` | Invalidate cache entries | Required |
| **GetAttribute** ★ | `GetAttributeRequest/Response` | File metadata lookup | Required |
| **PutAttribute** ★ | `PutAttributeRequest/Response` | Store file metadata | Required |
| **GetCacheServers** ★ | `GetCacheServersRequest/Response` | Server discovery | Required |
| **LockFile** | `LockFileRequest/Response` | Explicit file locking | Deferred |
| **IBInit** | `IBInitRequest/Response` | InfiniBand RDMA setup | Deferred |
| **GetCacheLastAccessTimeDist** | `GetCache...Request/Response` | Access time analytics | Deferred |
| **Locator RPCs** | `LocatorRegister/Lookup/Refresh/Unregister` | Distributed locator cache | Deferred |

Additionally, the Go client must implement:
- **Wire framing**: 4-byte big-endian length prefix + protobuf + raw data bytes
- **Auth**: `AuthParams` construction (account name + HMAC signature)
- **Checksum**: XXH32 or CRC32 on upload/download (validate data integrity)
- **Chunking**: Client-side file splitting matching StreamingClient chunk key format
- **Consistent hashing**: SHA256-based ring with 750 virtual nodes per server

#### 8.2 Compatibility Validation Strategy

Ensuring the Go client is a faithful implementation of the C++ client protocol and behavior
is critical. The following layers of validation are used:

##### Layer 1: Protocol Correctness (unit tests, no server needed)

**Wire format tests**: Construct requests in Go, verify the serialized bytes match
the expected 4-byte-length-prefix + protobuf format. Use known-good captures from the
C++ client as golden test vectors.

```go
func TestUploadRequestWireFormat(t *testing.T) {
    // Build the same request the C++ client would build
    req := buildUploadRequest("test/file.bin", 0, 1024, data)
    wire := serializeToWire(req)

    // Verify against golden bytes captured from C++ client
    assert.Equal(t, goldenUploadRequestBytes, wire)
}
```

**Protobuf field mapping tests**: For each request type, verify that all fields
specified in `cache.proto` are populated correctly — especially tricky fields like
`filesize` (which holds chunk data size, not full file size) and `metadata` maps.

##### Layer 2: Deterministic Parity (unit tests with golden vectors)

**Chunk key generation**: Port test vectors from `GetCacheChunkPropertiesTest.cpp`
(lines 44–213) to Go. These verify:
- SHA256 key determinism (same input → same key)
- Default vs custom chunk size suffix behavior
- Different offsets produce different keys
- Account/container/path formatting matches C++ exactly

```go
func TestChunkKeyMatchesCpp(t *testing.T) {
    // Vectors extracted from GetCacheChunkPropertiesTest.cpp
    tests := []struct {
        account, container, path string
        offset, chunkSize        int64
        expectedKey              string // SHA256 hex from C++ test
    }{
        {"acct", "ctr", "data/model.bin", 0, 16777216, "a1b2c3..."},
        {"acct", "ctr", "data/model.bin", 16777216, 16777216, "d4e5f6..."},
        // ... all vectors from C++ test
    }
    for _, tt := range tests {
        key := GenerateCacheKey(tt.account, tt.container, tt.path, tt.offset, tt.chunkSize)
        assert.Equal(t, tt.expectedKey, key)
    }
}
```

**Consistent hash ring**: Extract deterministic mapping vectors from
`ConsistentHashingTests.cpp` (lines 73–384):
- Given server list `[A, B, C]` and key `K`, verify Go selects the same server as C++
- Scale-up: add server D, verify only expected keys remap
- Distribution: verify standard deviation of key-to-server mapping is within bounds

```go
func TestConsistentHashMatchesCpp(t *testing.T) {
    ring := NewConsistentHashRing([]string{"server-0", "server-1", "server-2"}, 750)

    // Vectors from C++ ConsistentHashingTests
    assert.Equal(t, "server-1", ring.GetServer("known-key-1"))
    assert.Equal(t, "server-0", ring.GetServer("known-key-2"))
    // ... extracted from C++ test assertions
}
```

**Checksum computation**: Verify XXH32/CRC32 outputs match C++ for known inputs.

##### Layer 3: Cross-Client Integration (requires real server)

**Interop tests**: Run against a real distributed cache server cluster. Upload data with
the C++ client, download with the Go client (and vice versa). This validates end-to-end
protocol compatibility including:
- Wire format correctness under real server parsing
- Auth handshake
- Chunk key compatibility (Go writes, C++ reads the same chunk)
- Metadata round-trip (attributes set by one client, read by the other)

```go
func TestCrossClientInterop(t *testing.T) {
    if os.Getenv("DIST_CACHE_SERVERS") == "" {
        t.Skip("requires live distributed cache cluster")
    }

    // 1. C++ client uploads a file (via CLI tool or test binary)
    exec.Command("cache_tool", "upload", "--file=testdata/model.bin", "--key=interop/test").Run()

    // 2. Go client downloads same file
    goClient := dcache.New(dcache.WithServers(servers))
    data, err := goClient.Download(ctx, "interop/test")
    require.NoError(t, err)

    // 3. Verify data matches
    assert.Equal(t, originalData, data)

    // 4. Go client uploads, C++ client downloads
    goClient.Upload(ctx, "interop/test2", newData)
    result := exec.Command("cache_tool", "download", "--key=interop/test2").Output()
    assert.Equal(t, newData, result)
}
```

This can be integrated into the distributed cache's existing E2E harness
(`test/pkg/common/common.go`) which already sets up real server clusters in K8s.

##### Layer 4: Behavioral Parity (integration tests)

Cover the behavior matrix validated by the C++ tests (`CacheClientTests.cpp` lines 175–1082):

| Behavior | C++ Test | Go Test |
|---|---|---|
| Upload + download round-trip | ✅ | Must match |
| Duplicate upload handling | ✅ | Must match |
| Large file rejection (over max size) | ✅ | Must match |
| TTL expiry (upload, wait, download fails) | ✅ | Must match |
| Delete then download (not found) | ✅ | Must match |
| Lock protocol: got-lock, already-locked | ✅ | Must match |
| Lock expiry and retry | ✅ | Must match |
| Metadata round-trip (arbitrary key-value) | ✅ | Must match |
| Checksum validation (corrupt data detected) | ✅ | Must match |
| Connection reuse across requests | ✅ | Must match |
| Parallel uploads/downloads | ✅ | Must match |
| Server failover (one server down) | ✅ | Must match |
| Discovery refresh (server list changes) | ✅ | Must match |
| Auth failure (wrong credentials) | ✅ | Must match |

##### Layer 5: Performance Parity (see §4.5)

`dcache-bench` standalone tool: benchmark Go client vs C++ client against the same server,
comparing throughput (MB/s), latency (p50/p99), and CPU usage. Target: within 5% of C++
throughput. This can also be added to the distributed cache's perf test suite
(`test/perf/tachyon_client_suite_test.go`).

### Phase 2: Blobfuse Component ✅ Complete

Created `component/dist_cache/`:
- Component registration at LevelMid priority
- Config parsing and CLI flag bindings
- **file_cache ops**: CopyToFile (multi-chunk download with lock protocol), CopyFromFile (multi-chunk upload)
- **block_cache ops**: ReadInBuffer (single-chunk download), StageData (single-chunk upload), CommitData (forward)
- GetAttr: serve from the distributed cache attribute cache
- Invalidation: Delete/Rename/Truncate
- Graceful degradation on distributed cache errors
- Unit tests with mocked distributed cache client

### Phase 3: Pipeline Integration ✅ Complete

- Updated `cmd/mount.go` to support dist_cache in pipeline
- Updated `common.ValidatePipeline()` to allow file_cache+dist_cache and block_cache+dist_cache
  (rejects dist_cache+xload)
- CLI flag bindings: `--dist-cache-discovery-url`, `--dist-cache-server-list`
- Environment variable: `DIST_CACHE_SERVER_LIST`
- Added blank import in `cmd/imports.go` for component registration

### Phase 4: Testing & Documentation ✅ Testing Complete, Documentation In Progress

#### 4a. Testing Strategy

Tests follow blobfuse's existing patterns and testing pyramid:

**Layer 1: Go client unit tests** (`internal/dist_cache_client/*_test.go`)
- Embedded mock TCP server that speaks the wire protocol (4-byte length prefix + protobuf)
- Coverage: connect/reconnect, upload, download, chunking (split/reassemble), consistent
  hashing (ring distribution), lock protocol (got-lock, already-locked, retry), error paths,
  connection pooling, server failover
- No external dependencies — runs in CI with `--tags=unittest`

**Layer 2: Component unit tests** (`component/dist_cache/*_test.go`)
- Use `internal/mock_component.go` (GoMock) as the downstream `NextComponent()`
- Define a `Client` interface in dist_cache and mock it for the upstream distributed cache
- Coverage: CopyToFile (L2 hit, L2 miss with L3 fetch, lock-acquired flow, lock-contention
  retry), CopyFromFile (write-through), ReadInBuffer (block-level L2 hit/miss),
  StageData/CommitData, invalidation (Delete/Rename/Truncate), graceful degradation
  (bypass-on-error when cluster is down), config validation
- Pattern: matches `file_cache_test.go` (GoMock + `SetNextComponent`)

**Layer 3: Pipeline integration tests** (`component/dist_cache/*_test.go`)
- Real pipeline with `loopbackfs` as the storage backend:
  - `file_cache → dist_cache → loopbackfs`
  - `block_cache → dist_cache → loopbackfs`
- dist_cache talks to an embedded test TCP server (in-process, real wire protocol)
- Coverage: end-to-end data correctness, cache population on first read, cache hit on
  second read, write-through, invalidation, mixed file_cache and block_cache operations
  produce identical cache keys (cross-pipeline sharing)
- Pattern: matches `block_cache_test.go` (real `loopbackfs` backend)

**Layer 4: E2E tests** (nightly pipeline, requires Azure credentials + distributed cache cluster)
- Mount blobfuse with dist_cache enabled against real Azure Storage
- Run existing `test/e2e_tests/` suites (file, dir, data validation) against the mount
- Additional multi-node scenario: 2 blobfuse mounts sharing a distributed cache cluster,
  verify that Node B gets an L2 hit for data written/read by Node A
- Extend Azure Pipeline templates: `mount.yml` with dist_cache config, dedicated test stage
- Topologies tested: file_cache+dist_cache and block_cache+dist_cache

#### 4b. Performance Testing

Extend blobfuse's existing fio-based benchmark infrastructure (`perf_testing/scripts/fio_bench.sh`):

**Latency benchmarks** (compare against baseline without dist_cache):

| Scenario | Metric | Method |
|---|---|---|
| L1 hit (local cache) | Read latency, IOPS | fio random read, file already in local cache |
| L2 hit (distributed cache) | Read latency, IOPS | fio random read, local cache cold, dist cache warm |
| L3 miss (Azure fetch) | Read latency | fio sequential read, both caches cold |
| L2 overhead on L1 hit | Latency delta | Compare file_cache-only vs file_cache+dist_cache (L1 hit path should add ~0 overhead) |
| Write-through | Write latency | fio sequential write with dist_cache enabled vs disabled |

**Throughput benchmarks**:

| Scenario | Metric | Method |
|---|---|---|
| Single-node sequential read | MB/s | fio seq read, 1GB file, L2 warm |
| Multi-node shared read | Aggregate MB/s | 2+ nodes reading same files, L2 warm, measure total throughput |
| Cache population | MB/s | Sequential read of cold data — measures L3→L2 population speed |
| Block cache 16MB reads | MB/s, IOPS | fio with block_cache+dist_cache, random 16MB block reads |

**Scalability benchmarks** (manual/nightly):
- Vary cluster size (1, 3, 5 distributed cache nodes) — measure throughput scaling
- Vary file count (1K, 10K, 100K files) — measure cache lookup latency
- Vary concurrent readers (1, 4, 16, 64 clients) — measure contention and lock protocol efficiency

**Regression tracking**: Add dist_cache scenarios to the weekly perf pipeline
(`blobfuse2-perf.yaml`) with results published alongside existing file_cache and block_cache
benchmarks.

#### 4c. Documentation

- User guide in `docs/` with configuration examples for both topologies
- Update README with distributed cache option
- Add sample config files: `sampleDistCacheFileCacheConfig.yaml`, `sampleDistCacheBlockCacheConfig.yaml`
- Troubleshooting: common errors (cluster unreachable, chunk size mismatch, etc.)

### Phase 5: Advanced Features

- Async cache population with background goroutines
- Cache warming on mount
- Metrics integration with blobfuse health monitor (bfusemon)
- Kubernetes-aware server discovery via Distributed Cache CRD

## 9. Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Distributed cache protocol breaking changes | Client fails to communicate | Semantic versioning of Go client; proto compatibility tests in distributed cache CI |
| Distributed cache cluster outage | L2 cache unavailable | `bypass-on-error: true` degrades to L1+L3; local cache unaffected |
| Network partition | Subset of servers unreachable | Consistent hashing redistributes; affected files fall through to azstorage |
| Cache poisoning (stale data served) | Incorrect reads | ETag/LastModified in metadata; TTL expiry; explicit invalidation on writes |
| Large file memory pressure | OOM on blobfuse node | Stream via io.Writer/io.Reader; never buffer full file in memory |
| Go client performance vs C++ client | Higher latency per RPC | Protocol is I/O-bound not CPU-bound; Go TCP performance is adequate |
| Two-cache pipeline unforeseen interactions | Subtle bugs | Extensive integration testing; dist_cache is a simple pass-through on miss |

## 10. Open Questions

1. ~~**Cache key namespacing**~~: **Resolved** — Cache keys use `cachePrefix/filePath:offset:chunkSize`
   format. The `cache-prefix` config (typically `accountName/containerName`) provides namespacing
   to avoid collisions across storage accounts and containers.

2. **Distributed cache auth**: In deployments where auth is enabled, how should blobfuse obtain
   the auth credentials? Currently supported via `auth-account-name` and `auth-account-key` in
   config. Environment variable and managed identity support are future work.

3. **Write-back vs write-through**: The current design is write-through (azstorage first, then
   populate distributed cache). Write-back (distributed cache first, async flush to azstorage)
   could improve write-heavy workloads but trades durability for performance. Deferred to Phase 5.

4. ~~**Chunk size alignment**~~: **Resolved** — dist_cache resolves chunk size via priority chain:
   `block_cache.block-size-mb` > `stream.block-size-mb` > `dist_cache.chunk-size-mb` > default (16 MiB).
   Production recommendation is 32 MiB.

5. ~~**Go client module path**~~: **Resolved** — client lives in `internal/dist_cache_client/` within
   the blobfuse repo initially. Will move to the distributed cache repo (e.g., `sdk/go/dcache/`) once
   the upstream repo is open-sourced. Package has no blobfuse imports to keep it portable.

6. ~~**dist_cache standalone (no local cache)**~~: **Resolved** — Not supported. FUSE sends 1 MB reads
   but dist_cache works with 32 MB chunks — buffer size mismatch prevents standalone operation.
   dist_cache requires block_cache or file_cache in the pipeline.
