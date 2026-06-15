# Dist Cache: Versioned Overwrite Design

## Problem Statement

The current dist_cache uses a monotonically-increasing integer version per file (stored only in-process memory) to construct group IDs for cache chunks. This version is lost on process restart, requiring a server round-trip (`GetChunkGroupID`) to recover. More critically, the version has no external persistence — if two nodes write the same file concurrently, or a node restarts mid-operation, cache consistency relies on heuristics and timing.

This design replaces the in-memory integer version with the **Azure blob ETag**, providing:
1. A durable, globally-unique version per blob revision (intrinsic to Azure — no custom metadata needed).
2. Cross-node consistency: any node can read the current ETag from blob properties via `GetAttr`.
3. Clean invalidation: old version chunks are identifiable and deletable by group.
4. Open-close consistency: the ETag is pinned on the file handle at open time — all reads for a handle use the same version.

---

## Design Overview

### Core Concept

The **ETag** returned by Azure after each blob modification serves as the version identifier. The ETag is:
- Embedded into the dist_cache **cache key** (so each version occupies a distinct slot).
- Used to construct the **group ID** (so all chunks of a version can be deleted atomically).
- Already stored on the file handle by block_cache (`handle.SetValue("ETAG", ...)`) at open time and updated after each commit.
- Resolved by dist_cache from the handle on reads (`options.Handle.GetValue("ETAG")`).

No custom blob metadata is required. The ETag is an intrinsic property that Azure guarantees changes atomically with every blob modification.

### Cache Key Format (changed)

```
Current:  SHA256(cachePrefix/filePath:offset:chunkSize)
Proposed: SHA256(cachePrefix/filePath:etag:offset:chunkSize)
```

The ETag is included in the cache key input **before** hashing. This means each version of a chunk is stored at a different cache slot. A new write never overwrites the previous version's cached data — old chunks persist until explicitly deleted via `DeleteGroup`.

Examples (pre-hash input):
```
acct/container/file.txt:0x8DC3A2B1C4E5F6A7:0:16777216        (offset 0, 16MB chunk)
acct/container/file.txt:0x8DC3A2B1C4E5F6A7:16777216:16777216  (offset 16MB)
acct/container/file.txt:0x8DC4B3C2D5F607B8:0:16777216         (new ETag after overwrite)
```

### Chunk Group ID Format

```
Current:  <filename>\x00v<uint64>
Proposed: <filename>\x00v<etag>
```

Examples:
```
container/path/file.txt\x00v0x8DC3A2B1C4E5F6A7   (ETag from one revision)
container/path/file.txt\x00v0x8DC4B3C2D5F607B8   (ETag from next revision)
```

All chunks of a single file version share the same group ID. The `DeleteGroup` RPC deletes all chunks with that group ID across the cluster.

### Why Version Must Be in the Cache Key

Without version in the cache key, all versions of a chunk map to the same slot:
- Upload with ETag-A writes data to `SHA256(file:0:16MB)`.
- Upload with ETag-B **overwrites** that same slot.
- If `DeleteGroup(A)` arrives after B's upload, it deletes B's data (wrong!).
- A reader with a stale ETag could read B's data thinking it's A's (stale read).

With version in the cache key:
- ETag-A → `SHA256(file:A:0:16MB)` (slot X).
- ETag-B → `SHA256(file:B:0:16MB)` (slot Y, different from X).
- `DeleteGroup(A)` only deletes chunks at slot X — B's data at slot Y is untouched.
- A reader uses the ETag pinned on its handle, so it can only read the version it opened with.

---

## Detailed Design

### 1. Write Path — ETag as Version

#### How it works
The ETag is produced by Azure as a response to `CommitBlockList` (block_cache path) or `PutBlob`/`Flush` (file_cache path). Since the dist_cache uploads chunks to the cache cluster **after** the blob is committed (never before), the ETag is always known at chunk-upload time. No pre-generated version identifier is needed.

block_cache already:
1. Stores the ETag on the handle at open time: `handle.SetValue("ETAG", attr.ETag)`
2. Updates it after commit: `handle.SetValue("ETAG", newEtag)` (via `CommitDataOptions.NewETag`)

dist_cache simply reads the new ETag from `CommitDataOptions.NewETag` (or `CopyFromFileOptions.NewETag`) when uploading chunks.

#### Flow (block_cache path — `StageData` → `CommitData`)

```
StageData(name, offset, data)
  │
  ├─ Forward to NextComponent (stage block in Azure)
  ├─ Buffer chunk locally in pendingWrites (unchanged)
  │
CommitData(name, blockList)
  │
  ├─ 1. Resolve old ETag from remote blob (BEFORE commit overwrites it):
  │      oldAttr = GetAttr(name)
  │      oldETag = oldAttr.ETag
  │      (if file is new / GetAttr 404 → oldETag = "", skip DeleteGroup)
  │
  ├─ 2. Forward to NextComponent (PutBlockList in Azure)
  │      └─ Receives newETag on success (via CommitDataOptions.NewETag)
  │
  ├─ 3. DeleteGroup(oldGroupID)          ← invalidate old chunks
  │      oldGroupID = fileGroupID(name, oldETag)
  │
  ├─ 4. Flush pending chunks to dist_cache with new ETag:
  │      groupID = "<name>\x00v<newETag>"
  │      for each chunk:
  │        UploadChunk(name, newETag, offset, data, WithGroupID(groupID))
  │                          ↑ ETag used in cache key
  │
  └─ 5. Clear dirty flag
```

#### Flow (file_cache path — `CopyFromFile`)

```
CopyFromFile(name, file)
  │
  ├─ 1. Resolve old ETag from remote blob (BEFORE commit overwrites it):
  │      oldAttr = GetAttr(name)
  │      oldETag = oldAttr.ETag
  │      (if file is new / GetAttr 404 → oldETag = "", skip DeleteGroup)
  │
  ├─ 2. Forward to NextComponent (uploads blob)
  │      └─ Receives newETag on success (via CopyFromFileOptions.NewETag)
  │
  ├─ 3. DeleteGroup(oldGroupID)
  │      oldGroupID = fileGroupID(name, oldETag)
  │
  ├─ 4. Populate cache asynchronously with new ETag:
  │      Upload(name, newETag, file, size, WithGroupID(gid))
  │                    ↑ ETag used in cache key for all chunks
  │
  └─ 5. Clear dirty flag
```

#### Key Points
- The **same ETag** is used for all chunks of the file in a single write (it's the ETag returned from the commit).
- No custom blob metadata is read or written. The ETag is an intrinsic Azure property always present on the commit response.
- No metadata merge logic is needed — user-defined metadata on blobs is completely untouched.
- `GetAttr` before commit only needs standard properties (ETag), not `RetrieveMetadata: true`.
- `CopyFromFileOptions` needs a new `NewETag *string` field (analogous to `CommitDataOptions.NewETag`).

---

### 2. Read Path — Version from Handle

#### Flow (block_cache path — `ReadInBuffer`)

```
ReadInBuffer(name, offset, buf, handle)
  │
  ├─ If dirty(name): bypass dist_cache → NextComponent
  │
  ├─ Resolve current version from handle:
  │   etag = handle.GetValue("ETAG")     ← pinned at open time by block_cache
  │
  ├─ DownloadChunk(name, etag, offset, buf, WithLock=true)
  │   │                  ↑ ETag used to construct cache key
  │   ├─ HIT: return data
  │   ├─ MISS (got lock):
  │   │   ├─ Download chunk from Azure (NextComponent.ReadInBuffer)
  │   │   ├─ UploadChunk(name, etag, offset, data, WithGroupID(gid))
  │   │   └─ Return data
  │   └─ MISS (already locked):
  │       ├─ Poll until cached (using same versioned key)
  │       └─ Fallback to Azure on timeout
  │
  └─ Return data
```

#### Flow (file_cache path — `CopyToFile`)

```
CopyToFile(name, count, file, handle)
  │
  ├─ If dirty(name): bypass → NextComponent
  │
  ├─ Resolve version from handle:
  │   etag = handle.GetValue("ETAG")
  │
  ├─ DownloadWithSizePartial(name, etag, count, file, WithLock=true)
  │   │                           ↑ ETag used to construct cache keys
  │   ├─ Full HIT: done
  │   └─ Partial MISS: for each missed chunk:
  │       ├─ Download from Azure
  │       ├─ Write to local file
  │       └─ UploadChunk(name, etag, offset, data, WithGroupID(gid))
  │
  └─ Return
```

#### Open-Close Consistency
Because the ETag is pinned on the handle at `OpenFile` time:
- All reads within a single open session use the same ETag → same cache keys → consistent data.
- If another node overwrites the file, this node's open handle still points to the old ETag. Reads may get cache misses (old chunks were deleted by the writer's `DeleteGroup`), but the fallback to Azure uses the ETag in `ReadInBufferOptions.Etag` for conditional reads — Azure returns 412 if the ETag is stale, signaling the application that the data changed.
- A new `OpenFile` fetches the fresh ETag from the current blob state.

#### No Version Cache Needed
Unlike the GUID approach, there's no need for an in-memory `versionCache` map with TTL-based refresh logic. The ETag lives on the handle, which is the authoritative reference for the duration of an open session. This eliminates:
- TTL tuning and staleness windows.
- `GetAttr` calls on the read path (the handle already has the ETag).
- Race conditions between cache refresh and concurrent operations.

---

### 3. DeleteGroup — Invalidate Old Versions Before Upload

#### Current Behavior
`DeleteGroup(groupID)` broadcasts a delete for all chunks tagged with that group ID across all cache servers. This is already called before writes.

#### Proposed Change
Before uploading new chunks, we must delete chunks from the **previous version** of the file. Since the old ETag is discovered via `GetAttr` before commit, the flow is:

```
resolveOldETag(name) → oldETag:
  │
  ├─ 1. GetAttr(name) — standard properties from remote blob:
  │      oldETag = attr.ETag
  │      return oldETag
  │
  └─ 2. If GetAttr returns 404 (new file) → return "" (skip DeleteGroup)

DeleteOldVersions(name, oldETag):
  │
  ├─ if oldETag == "": return (nothing to delete)
  │
  └─ oldGID = fileGroupID(name, oldETag)
     DeleteGroup(oldGID)
```

**Why always from remote?** The local node's handle has the ETag from open time, but another node may have overwritten the file since then. Using the authoritative blob ETag guarantees we delete the correct version's chunks. The extra `GetAttr` round-trip on the write path is acceptable because writes are already expensive (staging blocks + PutBlockList), and correctness is more important than saving one HEAD call.

#### Timing
```
CommitData / CopyFromFile:
  1. Resolve oldETag (GetAttr from remote blob — BEFORE commit)
  2. Commit blob to Azure (Azure assigns new ETag)
  3. DeleteGroup(oldGID)        ← old version chunks deleted
  4. Upload chunks with newETag ← safe from deletion (different cache keys)
```

The ordering ensures:
- The old ETag is captured **before** the commit changes it.
- After step 2, any concurrent reader that opens the file will get the new ETag and use new cache keys.
- Concurrent readers still using the old ETag during the delete window (step 3) will get cache misses and fall through to Azure (which already has the new data committed).
- New chunks (step 4) occupy different cache slots than old chunks, so DeleteGroup(oldGID) cannot affect them.

---

## Struct / Interface Changes

### Modified `DistCache` struct

```go
type DistCache struct {
    // ... existing fields ...

    // Remove:
    // versionMu    sync.Mutex
    // fileVersions map[string]uint64

    // No version cache needed — ETag lives on handles
}
```

### Modified `fileGroupID`

```go
// Before:
func fileGroupID(name string, version uint64) []byte {
    return []byte(fmt.Sprintf("%s\x00v%d", name, version))
}

// After:
// etag is the Azure blob ETag for the file revision.
func fileGroupID(name string, etag string) []byte {
    return []byte(fmt.Sprintf("%s\x00v%s", name, etag))
}
```

### Modified `GenerateCacheKey` (in `internal/dist_cache_client/hashing.go`)

```go
// Before:
func GenerateCacheKey(cachePrefix, filePath string, offset, chunkSize int64) string {
    const defaultServerChunkSize = 4 * 1024 * 1024
    var keyInput string
    if chunkSize == defaultServerChunkSize {
        keyInput = fmt.Sprintf("%s/%s:%d", cachePrefix, filePath, offset)
    } else {
        keyInput = fmt.Sprintf("%s/%s:%d:%d", cachePrefix, filePath, offset, chunkSize)
    }
    h := sha256.Sum256([]byte(keyInput))
    return hex.EncodeToString(h[:])
}

// After:
// etag is included in the key so each blob revision occupies a distinct cache slot.
func GenerateCacheKey(cachePrefix, filePath, etag string, offset, chunkSize int64) string {
    const defaultServerChunkSize = 4 * 1024 * 1024
    var keyInput string
    if chunkSize == defaultServerChunkSize {
        keyInput = fmt.Sprintf("%s/%s:%s:%d", cachePrefix, filePath, etag, offset)
    } else {
        keyInput = fmt.Sprintf("%s/%s:%s:%d:%d", cachePrefix, filePath, etag, offset, chunkSize)
    }
    h := sha256.Sum256([]byte(keyInput))
    return hex.EncodeToString(h[:])
}
```

This change means **all callers** of `UploadChunk` and `DownloadChunk` must supply the ETag. The dist_cache client methods gain an etag parameter (or accept it via options).

### Modified `dcacheClient` Interface

```go
type dcacheClient interface {
    // Upload/Download now require an ETag to construct the versioned cache key.
    Upload(ctx context.Context, filename, etag string, data io.Reader, size int64, opts ...dcache.UploadOption) error
    DownloadWithSizePartial(ctx context.Context, filename, etag string, fileSize int64, w io.WriterAt, opts ...dcache.DownloadOption) ([]dcache.ChunkError, error)
    DownloadChunk(ctx context.Context, filename, etag string, offset int64, buf []byte, opts ...dcache.DownloadOption) (int, error)
    UploadChunk(ctx context.Context, filename, etag string, offset int64, data []byte, opts ...dcache.UploadOption) error

    // These don't need ETag (operate on group IDs or metadata, not cache keys).
    Delete(ctx context.Context, filename string, fileSize int64) error
    DeleteGroup(ctx context.Context, groupID []byte) error
    GetChunkGroupID(ctx context.Context, filename string) ([]byte, error)
    GetAttr(ctx context.Context, filename string) (*dcache.FileAttr, error)
    PutAttr(ctx context.Context, attrs []dcache.FileAttrEntry) error
    Close() error
}
```

### New Field on `CopyFromFileOptions`

```go
// In internal/component_options.go
type CopyFromFileOptions struct {
    Name     string
    File     *os.File
    Metadata map[string]*string
    NewETag  *string  // NEW: populated by azstorage after successful upload
}
```

This mirrors the existing `NewETag *string` field on `CommitDataOptions`, allowing the azstorage component to return the new ETag to dist_cache after a whole-file upload.

---

## Sequence Diagrams

### Overwrite Scenario (Write → Read on Same Node)

```
Node A                          Azure Blob             Dist Cache Cluster
  │                                │                        │
  ├─ CommitData(file.txt)          │                        │
  │   ├─ resolveOldETag:           │                        │
  │   │   GetAttr(file.txt) ──────►│                        │
  │   │   ◄── ETag: ETag-1        │                        │
  │   │                            │                        │
  │   ├─ PutBlockList ────────────►│                        │
  │   │                 ETag-2  ◄──┤                        │
  │   │                            │                        │
  │   ├─ DeleteGroup(file.txt\0vETag-1) ──────────────────►│ (old chunks gone)
  │   │                            │                        │
  │   ├─ UploadChunk(key=SHA(file:ETag-2:0:16M)) ────────►│ (new slot)
  │   │                            │                        │
  │   └─ handle.SetValue("ETAG", ETag-2) ← done by block_cache
  │                                │                        │
  ├─ ReadInBuffer(file.txt, off=0) │                        │
  │   ├─ etag = handle.GetValue("ETAG") → ETag-2           │
  │   ├─ DownloadChunk(key=SHA(file:ETag-2:0:16M)) ──────►│
  │   │   ◄── HIT (chunk at ETag-2 slot)                   │
  │   └─ Return data              │                        │
```

### Overwrite Scenario (Write on Node A, Read on Node B)

```
Node A                     Node B                  Azure Blob        Dist Cache
  │                          │                        │                  │
  ├─ CommitData              │                        │                  │
  │   ├─ resolveOldETag: GetAttr ────────────────────►│ ◄── ETag-1      │
  │   ├─ PutBlockList ──────────────────────────────►│ → ETag-2         │
  │   ├─ DeleteGroup(ETag-1) ─────────────────────────────────────────►│
  │   ├─ UploadChunk(key=SHA(file:ETag-2:0:16M)) ────────────────────►│
  │                          │                        │                  │
  │                          ├─ OpenFile(file.txt)    │                  │
  │                          │   ├─ GetAttr ─────────►│                  │
  │                          │   │   ◄── ETag-2      │                  │
  │                          │   └─ handle.SetValue("ETAG", ETag-2)     │
  │                          │                        │                  │
  │                          ├─ ReadInBuffer          │                  │
  │                          │   ├─ etag = handle.GetValue("ETAG") → ETag-2
  │                          │   ├─ DownloadChunk(key=SHA(file:ETag-2:0:16M)) ►│
  │                          │   │   ◄── HIT                            │
  │                          │   └─ Return data       │                  │
```

### Read-After-Write with Cache Miss (chunk not yet uploaded)

```
Node B                              Azure Blob             Dist Cache
  │                                    │                      │
  ├─ ReadInBuffer(file.txt, off=48MB)  │                      │
  │   ├─ etag = handle.GetValue("ETAG") → ETag-2             │
  │   ├─ DownloadChunk(key=SHA(file:ETag-2:48M:16M)) ────►│
  │   │   ◄── MISS (chunk not uploaded yet by Node A)       │
  │   │                                │                      │
  │   ├─ ReadInBuffer(Azure) ─────────►│                      │
  │   │   ◄── data                     │                      │
  │   │                                │                      │
  │   ├─ UploadChunk(key=SHA(file:ETag-2:48M:16M)) ──────►│
  │   │                                │                      │
  │   └─ Return data                   │                      │
```

---

## Edge Cases

### 1. First Write (No Previous Blob)
- `GetAttr` returns 404 → `oldETag = ""` → skip `DeleteGroup`.
- Commit proceeds, Azure returns a fresh ETag, chunks uploaded with that ETag.

### 2. Externally Written Blob
- External tool writes blob → Azure assigns ETag-X.
- Blobfuse opens file → `GetAttr` returns ETag-X → handle stores ETag-X.
- On read, `handle.GetValue("ETAG")` → ETag-X → cache key uses ETag-X.
- On cache miss, chunks are downloaded from Azure and uploaded to dist_cache with `groupID = "<name>\x00vETag-X"`.
- If the blob is later overwritten externally (ETag-Y), a new `OpenFile` gets ETag-Y, reads use new cache keys.
- Old ETag-X chunks are cleaned up when any blobfuse node writes the file (step 3 of the write path discovers ETag-X via `GetAttr` and calls `DeleteGroup`).
- No special handling needed — the ETag approach works identically for blobfuse-written and externally-written blobs.

### 3. Process Restart
- File handles are closed on process restart (FUSE contract).
- New opens fetch fresh ETags from Azure → handles are correctly initialized.
- No in-memory state to lose (no version cache map).
- First write after restart calls `GetAttr` to discover old ETag for `DeleteGroup` — correct.

### 4. Concurrent Writers (Node A and Node B overwrite same file)
- Both call `GetAttr` to get oldETag (both see ETag-1).
- Both call `PutBlockList`; last writer wins in Azure (gets ETag-3 or ETag-2 depending on ordering — Azure serializes).
- Each writer uploads chunks with **their own new ETag** (different keys — no conflict).
- Each writer calls `DeleteGroup(fileGroupID(name, ETag-1))` — idempotent, safe to call twice.
- Subsequent readers open the file, get the winning ETag, read from the correct slots.
- The losing writer's chunks (uploaded under a non-winning ETag) become orphans — cleaned up on the next write (discovered as the "old ETag" in step 1) or via TTL expiry.

### 5. Delete After Write
- `DeleteFile` calls `GetAttr` → gets current ETag → `DeleteGroup(gid)`.
- The blob is then deleted from Azure.
- Subsequent reads get 404 from Azure (correct behavior).

### 6. Stale Handle (File Overwritten While Open)
- Node A has file open with ETag-1 on the handle.
- Node B overwrites the file → ETag becomes ETag-2, and B calls `DeleteGroup(ETag-1)`.
- Node A reads → `handle.GetValue("ETAG")` → ETag-1 → cache key uses ETag-1.
- Cache miss (chunks were deleted by B's `DeleteGroup`).
- Falls through to Azure with ETag-1 in the `ReadInBufferOptions.Etag` field.
- Azure returns the data (if the blob's current ETag differs but the old data is gone, Azure returns the current version — depending on read semantics) or returns 412 Precondition Failed if conditional reads are used.
- **This is the expected open-close consistency behavior**: the handle sees a consistent snapshot until it detects the blob changed. The FUSE layer / attr_cache invalidation handles re-opens.

### 7. Partial Overwrite (O_RDWR without O_TRUNC)
- When only some blocks of a file are modified, `StageData` is called only for dirty blocks.
- `CommitData` triggers `DeleteGroup(oldETag)`, which removes **all** old-version chunks from the cache (dirty and unchanged alike).
- Only the dirty chunks (from `pendingWrites`) are uploaded under the new ETag.
- Unchanged chunks become cache misses on subsequent reads — they are fetched from Azure and re-uploaded to the dist_cache with the new ETag on demand.
- **This is acceptable**: no stale data is ever served. The trade-off is extra Azure round-trips for unchanged chunks after a partial overwrite, which is a performance cost but not a correctness issue.
- A future optimization could copy unchanged chunks from the old version to the new version before deleting the old group, but this adds complexity and is not required for V1.

---
## Configuration

No new user-facing configuration required. The approach has no tunable parameters (no TTL cache to configure).

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/dist_cache_client/hashing.go` | Add `etag` parameter to `GenerateCacheKey` — key format becomes `SHA256(prefix/file:etag:offset:chunkSize)` |
| `internal/dist_cache_client/client.go` | Add `etag string` parameter to `UploadChunk`, `DownloadChunk`, `Upload`, `DownloadWithSizePartial`; pass etag to `GenerateCacheKey` |
| `component/dist_cache/dist_cache.go` | Replace `fileVersions map[string]uint64` with ETag-from-handle reads; update `fileGroupID` to accept string; modify `CommitData`/`CopyFromFile` to use `GetAttr` for old ETag + `NewETag` for new version; modify `ReadInBuffer`/`CopyToFile` to read ETag from handle |
| `internal/component_options.go` | Add `NewETag *string` to `CopyFromFileOptions` |
| `component/azstorage/block_blob.go` | Populate `CopyFromFileOptions.NewETag` after successful upload |
| `component/azstorage/datalake.go` | Same for datalake path |
