# Distributed Cache for BlobFuse2

## Overview

BlobFuse2 supports an optional distributed cache layer (`dist_cache`) that provides a shared L2 cache between the local cache (L1) and Azure Blob Storage (L3). When multiple nodes mount the same Azure container, the distributed cache allows them to share cached data — when one node reads a file from Azure, the data is stored in the distributed cache so that other nodes can read it without making separate Azure requests.

### Benefits

- **Reduced Azure egress**: Only one node downloads each file from Azure; other nodes read from the distributed cache
- **Faster cold reads**: Data from the distributed cache (in-cluster network) is 1.2–4.5× faster than Azure Blob Storage
- **Cross-node sharing**: Nodes benefit from data cached by other nodes in the cluster
- **Graceful degradation**: If the distributed cache is unavailable, BlobFuse falls back to Azure Blob Storage transparently

### Architecture

```
Application
    │
    ▼
┌─────────────────┐
│ FUSE Kernel      │
└────────┬────────┘
         ▼
┌─────────────────┐
│ block_cache (L1) │  Local memory/disk cache — microsecond reads
└────────┬────────┘
         ▼ (on L1 miss)
┌─────────────────┐
│ dist_cache (L2)  │  Distributed cache cluster — sub-millisecond reads
└────────┬────────┘
         ▼ (on L2 miss)
┌─────────────────┐
│ azstorage (L3)   │  Azure Blob Storage — tens of milliseconds
└─────────────────┘
```

## Prerequisites

- A running distributed cache server cluster (3+ nodes recommended)
- Network connectivity between BlobFuse nodes and cache servers
- BlobFuse2 built with the `dist_cache` component (included in standard builds)

## Configuration

### Quick Start

Add `dist_cache` to your pipeline and provide server addresses:

```yaml
components:
  - libfuse
  - block_cache
  - dist_cache
  - attr_cache
  - azstorage

block_cache:
  block-size-mb: 32
  mem-size-mb: 4096
  prefetch: 32
  parallelism: 128

dist_cache:
  server-list: "cacheserver-0:9065,cacheserver-1:9065,cacheserver-2:9065"
  bypass-on-error: true

attr_cache:
  timeout-sec: 120

azstorage:
  type: block
  account-name: mystorageaccount
  account-key: <ACCOUNT_KEY>
  mode: key
  container: mycontainer
```

### Server Discovery Methods

At least one discovery method must be configured. The priority order is:

#### 1. Discovery Endpoint (Recommended)

Connect to a known endpoint that returns the full server list. Handles cluster scaling automatically.

```yaml
dist_cache:
  discovery-url: "cacheserver-discovery.tachyon-cache-system.svc.cluster.local:9000"
  bypass-on-error: true
```

#### 2. Kubernetes DNS Discovery

Resolve servers via a headless StatefulSet service. No discovery endpoint needed.

```yaml
dist_cache:
  k8s-service: cacheserver
  k8s-namespace: tachyon-cache-system
  port: 9065
  bypass-on-error: true
```

#### 3. Static Server List

Comma-separated list of `host:port` addresses. Best for bare-metal or non-K8s environments.

```yaml
dist_cache:
  server-list: "10.0.1.10:9065,10.0.1.11:9065,10.0.1.12:9065"
  bypass-on-error: true
```

#### 4. Environment Variable

Set `DIST_CACHE_SERVER_LIST` to inject server addresses without changing config files.

```bash
export DIST_CACHE_SERVER_LIST="10.0.1.10:9065,10.0.1.11:9065,10.0.1.12:9065"
blobfuse2 mount /mnt/blobfuse --config-file=config.yaml
```

### CLI Flags

Server discovery can also be set via command-line flags:

```bash
blobfuse2 mount /mnt/blobfuse --config-file=config.yaml \
  --dist-cache-server-list="host1:9065,host2:9065,host3:9065"
```

Or:

```bash
blobfuse2 mount /mnt/blobfuse --config-file=config.yaml \
  --dist-cache-discovery-url="discovery.my-namespace.svc.cluster.local:9000"
```

### Configuration Reference

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `discovery-url` | string | | Discovery endpoint for auto-detecting servers |
| `discovery-refresh-sec` | int | 60 | How often to refresh server list from discovery |
| `k8s-service` | string | | Kubernetes headless service name |
| `k8s-namespace` | string | | Kubernetes namespace for DNS discovery |
| `server-list` | string | | Comma-separated `host:port` list |
| `port` | int | 9065 | Cache server port (used with k8s-service) |
| `bypass-on-error` | bool | false | Fall through to Azure on cache errors |
| `ttl-seconds` | int | 0 | TTL for cached data (0 = no expiry) |
| `chunk-size-mb` | float | 16 | Chunk size when block_cache is not present |
| `cache-prefix` | string | | Prefix for cache keys |
| `max-conns-per-server` | int | 8 | Max TCP connections per cache server |
| `request-timeout-sec` | int | 30 | Per-request timeout |
| `auth-account-name` | string | | Cache auth account (if auth enabled) |
| `auth-account-key` | string | | Cache auth key (if auth enabled) |

### Recommended block_cache Tuning

For best performance with the distributed cache:

```yaml
block_cache:
  block-size-mb: 32          # Match cache chunk size
  mem-size-mb: 4096          # 4 GB memory for local cache
  prefetch: 32               # Aggressive prefetch for sequential reads
  parallelism: 128           # High parallelism for concurrent downloads
  prefetch-on-open: true     # Start prefetching on file open
```

## Pipeline Compatibility

| Pipeline | Supported | Notes |
|----------|-----------|-------|
| `block_cache` + `dist_cache` | ✅ | **Recommended** — best for large files and ML workloads |
| `file_cache` + `dist_cache` | ✅ | Good for general workloads with whole-file caching |
| `dist_cache` only (no local cache) | ❌ | Not supported — dist_cache requires block_cache or file_cache |
| `xload` + `dist_cache` | ❌ | Not supported — mutually exclusive |

## How It Works

### Read Path

1. **L1 hit** (local cache): Data is served from block_cache memory/disk — no network I/O
2. **L1 miss, L2 hit** (distributed cache): block_cache requests data from dist_cache → served from cache cluster over network
3. **L1 miss, L2 miss** (Azure fetch): dist_cache forwards to azstorage → data downloaded from Azure, then **asynchronously uploaded to distributed cache** for other nodes
4. **Warm reads**: After the first read, the Linux kernel page cache provides ~4–6 GB/s reads without calling into FUSE at all

### Write Path

Writes are **write-through**: data is written to Azure Blob Storage first (source of truth), then asynchronously uploaded to the distributed cache for cross-node sharing.

### Cache Invalidation

Delete, rename, and truncate operations invalidate the corresponding distributed cache entries before forwarding to Azure Blob Storage.

### Stampede Prevention

When multiple nodes request the same uncached file simultaneously, the distributed cache's lock protocol ensures only one node downloads from Azure. Other nodes wait briefly and then read from the distributed cache once it's populated.

## Performance

Benchmarked on 3× Standard_E192ids_v6 (192 vCPU, 1.8 TiB RAM, 200 Gbps NIC) with a 3-node distributed cache cluster:

### Cold Reads (First Access)

| File Size | Blob Only | With dist_cache | Speedup |
|-----------|----------|----------------|---------|
| 128 MB | 294 MB/s | 1,280 MB/s | 4.4× |
| 256 MB | 396 MB/s | 1,910 MB/s | 4.8× |
| 512 MB | 651 MB/s | 1,605 MB/s | 2.5× |
| 1 GB | 895 MB/s | 1,689 MB/s | 1.9× |
| 5 GB | 1,459 MB/s | 2,480 MB/s | 1.7× |
| 10 GB | 1,950 MB/s | 2,265 MB/s | 1.2× |

### Cross-Node Reads (Node B reading data cached by Node A)

| File Size | Blob Only | With dist_cache | Speedup |
|-----------|----------|----------------|---------|
| 128 MB | 305 MB/s | 1,376 MB/s | 4.5× |
| 1 GB | 1,039 MB/s | 2,048 MB/s | 2.0× |
| 10 GB | 1,499 MB/s | 1,997 MB/s | 1.3× |

### Warm Reads (Kernel Page Cache)

After initial access, warm reads reach ~4–6 GB/s for both configurations — the Linux kernel page cache dominates and serves reads at memory speed without calling into FUSE.

## Troubleshooting

### dist_cache: no server discovery configured

**Cause**: No server discovery method is set in config.

**Fix**: Set one of `discovery-url`, `k8s-service` + `k8s-namespace`, `server-list`, or the `DIST_CACHE_SERVER_LIST` environment variable.

### dist_cache: failed to start

**Cause**: Cannot connect to any cache server.

**Fix**: 
- Verify cache servers are running and accessible from BlobFuse nodes
- Check network connectivity: `nc -zv <cache-server-ip> 9065`
- If using K8s DNS discovery, ensure the headless service exists and pods are ready
- Set `bypass-on-error: true` to allow BlobFuse to start even if cache is unreachable

### Slow performance with dist_cache

**Possible causes**:
- Network MTU mismatch — use MTU 9000 (jumbo frames) for best performance
- Insufficient block_cache memory — increase `mem-size-mb` (recommend 4096+)
- Low parallelism — increase `parallelism` in block_cache config (recommend 128)
- Small prefetch — increase `prefetch` in block_cache config (recommend 32)

### dist_cache + xload is not supported

**Cause**: dist_cache and xload are mutually exclusive.

**Fix**: Use `block_cache` + `dist_cache` instead of `xload` + `dist_cache`.

## Sample Configurations

- [`sampleDistCacheConfig.yaml`](../sampleDistCacheConfig.yaml) — block_cache + dist_cache with static server list
- [`sampleBlockCacheConfig.yaml`](../sampleBlockCacheConfig.yaml) — block_cache only (no distributed cache)

For the full configuration reference, see [`setup/baseConfig.yaml`](../setup/baseConfig.yaml).
