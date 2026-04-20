# Block Cache — Design Document

## Table of Contents
1. [Buffers, Buffer Pools, Blocks, and File Mapping](#1-buffers-buffer-pools-blocks-and-file-mapping)
2. [Buffer Management and Contention Minimization](#2-buffer-management-and-contention-minimization)
3. [Concurrency Control for Reads and Writes](#3-concurrency-control-for-reads-and-writes)

---

## 1. Buffers, Buffer Pools, Blocks, and File Mapping

### 1.1 Overview

The block cache divides every file into fixed-size **blocks** (default 16 MB, configurable via `block-size-mb`). Each block that is actively being read or written has an in-memory **buffer** backing it. Buffers come from a bounded **buffer pool**. A **buffer descriptor** sits between the two, tracking metadata (reference count, dirty flag, validity, etc.).

```
┌────────────────────────────────────────────────────────────────────────┐
│                         BlockCache Component                          │
│                                                                       │
│  ┌──────┐       ┌───────────┐       ┌──────────────────┐             │
│  │ File │──has──▶│ blockList │──has──▶│ []*block (0..N) │             │
│  └──────┘       └───────────┘       └────────┬─────────┘             │
│                                              │ 1:1 (when cached)     │
│                                              ▼                       │
│                                    ┌─────────────────────┐           │
│       BufferTableMgr               │ bufferDescriptor    │           │
│    map[*block] ──────────────────▶ │  .refCnt            │           │
│                                    │  .dirty / .valid    │           │
│                                    │  .contentLock       │           │
│                                    │  .buf ──────┐       │           │
│                                    └─────────────┼───────┘           │
│                                                  ▼                   │
│                                          ┌──────────────┐            │
│                                          │ []byte       │            │
│                                          │ (from pool)  │            │
│                                          └──────────────┘            │
│                                                  ▲                   │
│                                    ┌─────────────┘                   │
│                                    │ BufferPool                      │
│                                    │  sync.Pool of fixed-size bufs   │
│                                    └─────────────────────────────────┘
└────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Core Data Structures

| Structure | File | Purpose |
|---|---|---|
| `BufferPool` | `buffer_pool.go` | Manages raw `[]byte` allocation using `sync.Pool`. Bounded by `maxBuffers`. |
| `freeListType` | `freelist.go` | Owns an array of `bufferDescriptor`s. Maintains a singly-linked free list and implements eviction. |
| `bufferDescriptor` | `buffer_descriptor.go` | Per-buffer metadata: reference count, dirty/valid flags, content lock, download/upload errors. |
| `BufferTableMgr` | `buffer_mgr.go` | Hash map `map[*block]*bufferDescriptor` protected by `sync.RWMutex`. The "page table" of the cache. |
| `block` | `block.go` | Represents one fixed-size chunk of a file. Stores block index, Azure block ID, and state (`localBlock` / `uncommitedBlock` / `committedBlock`). |
| `blockList` | `block.go` | Ordered slice of `*block` objects that compose a `File`. |
| `File` | `file.go` | Tracks all open handles, file size, block list, and synchronization state for one blob. |

### 1.3 Mapping Client-Side Offsets to Azure Blob Storage Blocks

Files in POSIX are byte-addressable, while Azure Blob Storage's block blobs are composed of discrete, independently addressable **blocks** (each up to 4 GiB, committed via `PutBlockList`). BlockCache bridges these two models.

#### Offset → Block Index Calculation

```
blockIndex      = offset / blockSize          (block.go:getBlockIndex)
offsetInBlock   = offset % blockSize          (block.go:convertOffsetIntoBlockOffset)
blockSize       = min(configuredBlockSize,
                      fileSize - blockIdx * configuredBlockSize)
                                              (block.go:getBlockSize)
numBlocks       = ceil(fileSize / blockSize)  (block.go:getNoOfBlocksInFile)
```

**Example** (16 MB blocks, 49 MB file):

```
Block 0:  bytes [0,         16 MiB)   — full block
Block 1:  bytes [16 MiB,    32 MiB)   — full block
Block 2:  bytes [32 MiB,    49 MiB)   — partial (17 MiB)

Read at offset 20 MiB:
  blockIdx       = 20 / 16 = 1
  offsetInBlock  = 20 - 16  = 4 MiB
  → read from block[1] starting at byte 4 MiB
```

#### Block ↔ Azure Storage Block Mapping

Each `block` object maps 1:1 to a single Azure Storage block:

- **Block ID**: A base64-encoded, 64-byte identifier (`common.BlockIDLengthBase64`), generated at upload time via `common.GetBlockID()`.
- **Staging**: Data is uploaded to Azure via `StageData()` (Azure `Put Block` API). After staging, the block is in **uncommitted** state — visible only to the uploader.
- **Committing**: `CommitData()` (Azure `Put Block List` API) atomically commits the ordered list of block IDs, making the entire file visible to all clients.

#### Block State Machine

```
                ┌────────────┐
                │ localBlock │  ◄── initial state (data only in memory)
                └─────┬──────┘
                      │ StageData (Put Block)
                      ▼
             ┌────────────────┐
             │ uncommitedBlock│  ◄── uploaded but not committed
             └────────┬───────┘
                      │ CommitData (Put Block List)
                      ▼
             ┌────────────────┐
             │ committedBlock │  ◄── synced with storage
             └────────┬───────┘
                      │ modified again (write)
                      ▼
                ┌────────────┐
                │ localBlock │     (cycle restarts)
                └────────────┘
```

State transitions are atomic (`atomic.LoadInt32`/`StoreInt32`).

#### Block List Validation

When a file is opened for writing, the existing block list is fetched from Azure Storage and **validated** (`block.go:validateBlockList`):
- All blocks except the last must be exactly `blockSize` bytes.
- The last block must be ≤ `blockSize` bytes.
- All block IDs must have the correct base64 length.

Files that fail validation are marked `blockListInvalid` and become **read-only** — this prevents corruption of blobs created by other tools with non-aligned block sizes.

For read-only access, a **synthetic block list** is generated from the file size alone (`block.go:updateBlockListForReadOnlyFile`), avoiding the cost of fetching the real block list from storage.

---

## 2. Buffer Management and Contention Minimization

### 2.1 Layered Architecture

Buffer management is organized into four layers, each with distinct responsibilities:

```
Layer 4 ─ BufferTableMgr  (buffer_mgr.go)
           Maps *block → *bufferDescriptor.
           RWMutex: concurrent lookups (RLock), exclusive for insert/remove.

Layer 3 ─ freeListType    (freelist.go)
           Singly-linked free list of available bufferDescriptors.
           Modified clock-sweep eviction for victim selection.
           Background goroutine for async buffer reset.

Layer 2 ─ bufferDescriptor (buffer_descriptor.go)
           Per-buffer metadata: refCnt (atomic), dirty/valid (atomic),
           contentLock (RWMutex), download/upload error state.

Layer 1 ─ BufferPool       (buffer_pool.go)
           sync.Pool of fixed-size []byte slices.
           Atomic counter enforces maxBuffers cap.
           Shared read-only zero buffer for sparse block handling.
```

### 2.2 Allocation Path

When a read or write needs a block's data, it calls `BufferTableMgr.GetOrCreateBufferDescriptor()`:

```
1. FAST PATH — LookUp (RLock on BufferTableMgr)
   ├─ Buffer found → refCnt++ (atomic) → return
   └─ Buffer not found → continue to slow path

2. SLOW PATH — Creation
   ├─ Lock block.mu (prevents duplicate creation for same block)
   ├─ Lock BufferTableMgr.mu (exclusive)
   ├─ Double-check: buffer may have appeared while waiting
   │   └─ If found → refCnt++ → return (avoids race)
   │
   ├─ Check block state:
   │   └─ uncommitedBlock → return bufDescStatusNeedsFileFlush
   │       (caller must flush file, then retry)
   │
   ├─ Try freeList.allocateBuffer()
   │   ├─ Free buffer available → O(1) unlink from list → use it
   │   └─ Free list empty → need eviction
   │
   ├─ Eviction loop (getVictimBuffer):
   │   ├─ Release BufferTableMgr.mu (avoid holding lock during I/O)
   │   ├─ Clock sweep through bufferDescriptors
   │   ├─ Find buffer with refCnt == 1 (only table ref)
   │   ├─ If dirty → schedule synchronous upload before eviction
   │   ├─ Pin buffer (refCnt++)
   │   ├─ Re-acquire BufferTableMgr.mu
   │   └─ Verify victim still valid (refCnt re-check)
   │
   ├─ Insert new mapping: table[block] = bufDesc, refCnt = 2
   │   (1 for table + 1 for caller)
   │
   ├─ Release BufferTableMgr.mu
   │
   └─ If committedBlock → schedule download
       ├─ Lock contentLock exclusively (blocks reads until download completes)
       └─ Worker downloads data, then unlocks contentLock
```

### 2.3 Release Path

```
bufferDescriptor.release():
  ├─ refCnt-- (atomic)
  ├─ If refCnt == 0:
  │   ├─ Queue to resetBufferDesc channel (async)
  │   └─ Background goroutine:
  │       ├─ Zero-fill buffer (security + consistency)
  │       ├─ Clear all metadata fields
  │       └─ Prepend to free list (LIFO for cache locality)
  ├─ If refCnt > 0: no-op (other users still active)
  └─ If refCnt < 0: panic (double-free bug)
```

### 2.4 How Contention Is Minimized

The design applies several techniques to reduce lock contention:

| Technique | Where Applied | Benefit |
|---|---|---|
| **RWMutex for lookups** | `BufferTableMgr.mu` | Concurrent readers never block each other; only inserts/removes take exclusive lock. |
| **Atomic reference counting** | `bufferDescriptor.refCnt` | Pin/unpin operations are lock-free. |
| **Atomic state fields** | `dirty`, `valid`, `block.state`, `File.size` | Hot-path checks avoid any locking. |
| **Per-block creation lock** | `block.mu` | Only contends when two threads try to create a buffer for the *same* block — not globally. |
| **Lock release during I/O** | Eviction path releases `btm.mu` before uploading dirty victim | Prevents long I/O operations from blocking all buffer allocations. |
| **Async buffer reset** | `freeList.resetBufferDesc` channel + goroutine | Expensive 16 MB zero-fill happens in background, not on the release critical path. |
| **Per-buffer content lock** | `bufferDescriptor.contentLock` (RWMutex) | Multiple readers can access the same buffer concurrently; only downloads/uploads take exclusive lock. |
| **LIFO free list insertion** | `resetBufferDescriptors` prepends to head | Recently used buffers are reallocated first, improving CPU cache locality. |

### 2.5 PostgreSQL Buffer Manager Inspiration

The design draws direct inspiration from PostgreSQL's shared buffer manager (see `src/backend/storage/buffer/` in PostgreSQL source). The table below maps the concepts:

| PostgreSQL Concept | BlockCache Equivalent | Details |
|---|---|---|
| **Shared Buffer Pool** | `BufferPool` (`sync.Pool` + `maxBuffers`) | Fixed-size pool of page/block-sized buffers. PostgreSQL uses shared memory; BlockCache uses Go heap with `sync.Pool` for GC-friendly reuse. |
| **Buffer Descriptors** | `bufferDescriptor` | Per-buffer metadata struct. PostgreSQL stores `BufferDesc` with `tag`, `state`, `buf_id`, `refcount`, `usage_count`. BlockCache mirrors this with `block`, `bufIdx`, `refCnt`, `bytesRead`, `numEvictionCyclesPassed`. |
| **Pin/Unpin (Ref Counting)** | `refCnt.Add(1)` / `release()` | PostgreSQL pins buffers to prevent eviction (`PinBuffer`/`UnpinBuffer` manipulate `refcount`). BlockCache uses identical semantics — buffers with `refCnt > 1` cannot be evicted. |
| **Buffer Hash Table** | `BufferTableMgr.table` (`map[*block]*bufferDescriptor`) | PostgreSQL maps `(RelFileNode, ForkNum, BlockNum)` → `buf_id` via a hash table with partition locks. BlockCache maps `*block` → `*bufferDescriptor` with a single `RWMutex`. |
| **Clock Sweep Eviction** | `freeList.getVictimBuffer()` | PostgreSQL's `StrategyGetBuffer()` implements a clock-sweep algorithm: scan buffers round-robin via `nextVictimBuffer`, skip pinned buffers, decrement `usage_count`, evict when `usage_count == 0`. BlockCache's `nxtVictimBuffer` scans round-robin, skips `refCnt > 1`, checks `bytesRead` and `numEvictionCyclesPassed` as the "second chance" mechanism. |
| **Usage Count / Second Chance** | `numEvictionCyclesPassed` + `bytesRead` | PostgreSQL's `usage_count` (0–5) gives popular pages multiple chances. BlockCache's `numEvictionCyclesPassed` counter serves the same purpose: buffers survive at least `minEvictionCyclesToPass` (1) full scans before being eligible for eviction. Fully-read buffers (`bytesRead >= blockSize`) bypass this and are immediately evictable. |
| **Dirty Buffer Handling** | Upload before eviction in `getVictimBuffer()` | PostgreSQL writes dirty pages to disk (via `FlushBuffer`) before reuse. BlockCache uploads dirty blocks to Azure Storage (`scheduleUpload(sync=true)`) before eviction. Both ensure no data loss. |
| **Free List** | `freeListType.firstFreeBuffer` linked list | PostgreSQL maintains a free list of unused buffers. BlockCache's singly-linked list through `nxtFreeBuffer` fields is the same pattern. |
| **Buffer Content Lock** | `bufferDescriptor.contentLock` (`sync.RWMutex`) | PostgreSQL uses `BufferDesc->content_lock` (lightweight lock) with shared/exclusive modes. BlockCache's `contentLock` provides identical semantics: shared for reads, exclusive for downloads/uploads. |

**Key Difference from PostgreSQL**: PostgreSQL has a dedicated background writer (`bgwriter`) that proactively flushes dirty pages. BlockCache does not have an equivalent — dirty blocks are flushed only during eviction or explicit `flush()` calls. This is appropriate because Azure Storage uploads are much more expensive than local disk writes, so proactive flushing could waste bandwidth.

---

## 3. Concurrency Control for Reads and Writes

### 3.1 Lock Hierarchy

The block cache enforces a strict lock ordering to prevent deadlocks:

```
Level 1 (outermost):  File.mu            — sync.RWMutex
Level 2:              block.mu           — sync.RWMutex
Level 3:              BufferTableMgr.mu  — sync.RWMutex
Level 4 (innermost):  bufferDescriptor.contentLock — sync.RWMutex
```

**Rule**: A goroutine holding a lock at level N must never attempt to acquire a lock at level < N.

### 3.2 Per-Level Locking Details

#### Level 1: File Lock (`File.mu` — `sync.RWMutex`)

| Operation | Lock Mode | Purpose |
|---|---|---|
| `write()` | Exclusive (`Lock`) | Protects block list modifications (creating new blocks, updating `synced` flag). Released before buffer operations. |
| `read()` | Shared (`RLock`) | Accesses `blockList.list[idx]` safely. Released immediately after obtaining the block pointer. |
| `flush()` | Exclusive (`Lock`) | Prevents new writes during flush. Held for the entire flush duration. Caller waits on `pendingWriters.Wait()` under this lock. |
| `truncate()` | Exclusive (`Lock`) | Modifies block list and file size atomically. |

#### Level 2: Block Lock (`block.mu` — `sync.RWMutex`)

Used exclusively during buffer creation in `GetOrCreateBufferDescriptor()` to prevent multiple goroutines from creating buffers for the same block. This is the "double-check locking" pattern:

```go
// Fast path: no lock needed
bufDesc, _ := btm.LookUpBufferDescriptor(blk)  // RLock on btm.mu
if bufDesc != nil { return bufDesc }

// Slow path: lock the block
blk.mu.Lock()
defer blk.mu.Unlock()

btm.mu.Lock()
// Double-check: another goroutine may have created it
bufDesc, exists := btm.table[blk]
if exists { ... return }
// Create new buffer...
```

#### Level 3: Buffer Table Manager Lock (`BufferTableMgr.mu` — `sync.RWMutex`)

| Operation | Lock Mode | Duration |
|---|---|---|
| `LookUpBufferDescriptor()` | Shared (`RLock`) | Very brief — read map + atomic increment |
| `GetOrCreateBufferDescriptor()` (insert) | Exclusive (`Lock`) | Held during allocation, but **released during eviction I/O** |
| `removeBufferDescriptor()` | Exclusive (`Lock`) | Brief — delete from map + release references |
| `getVictimBuffer()` (victim pinning) | Exclusive (`Lock`) | Brief — re-check refCnt and pin victim |

**Critical detail**: During the eviction path, `btm.mu` is released before the (potentially slow) dirty buffer upload, then re-acquired. This prevents eviction I/O from blocking all buffer lookups.

#### Level 4: Buffer Content Lock (`bufferDescriptor.contentLock` — `sync.RWMutex`)

This is the fine-grained lock that coordinates concurrent access to the **actual data bytes** in a buffer:

| Operation | Lock Mode | Duration |
|---|---|---|
| Read (`File.read`) | Shared (`RLock`) | Held only during `copy()` from buffer to user space |
| Write (`File.write`) | Exclusive (`Lock`) | Held during `copy()` from user space to buffer. Released after copy unless async upload is scheduled. |
| Download (`worker.downloadBlock`) | Exclusive (inherited) | Acquired before scheduling download in `GetOrCreateBufferDescriptor`. Released by worker after download completes. |
| Upload (`worker.uploadBlock`) | Exclusive (`Lock`) | Acquired in `scheduleUpload()`. Released by worker after upload completes. |
| Flush wait | Exclusive (`Lock` + immediate `Unlock`) | Acquires and immediately releases to wait for any in-flight upload to complete. |

### 3.3 Reference Counting Semantics

Reference counting is the primary mechanism preventing buffer eviction while the buffer is in use:

```
refCnt Value    Meaning
──────────────  ──────────────────────────────────────────────────
    0           Buffer is free (in free list, not mapped)
    1           Only BufferTableMgr holds a reference (eviction candidate)
    2           Table + 1 active user (normal single-user state)
   >2           Table + multiple active users (cannot evict)
```

**Key invariants**:
- `refCnt` is incremented while holding `btm.mu` (shared or exclusive) to prevent races with eviction.
- `refCnt` is decremented atomically (no lock needed).
- Eviction (`getVictimBuffer`) only selects buffers with `refCnt == 1`, then pins them (`refCnt++`) under `btm.mu` to prevent concurrent eviction.
- Panics if `refCnt` goes negative (indicates double-free bug).

### 3.4 Concurrent Read + Read

Multiple readers can access the same block simultaneously:

```
Reader A                          Reader B
────────                          ────────
btm.LookUp(blk)    ─RLock─▶     btm.LookUp(blk)    ─RLock─▶
  refCnt: 2→3                      refCnt: 3→4

bufDesc.contentLock.RLock()       bufDesc.contentLock.RLock()
  copy(userBuf, bufDesc.buf)        copy(userBuf, bufDesc.buf)
bufDesc.contentLock.RUnlock()     bufDesc.contentLock.RUnlock()

bufDesc.release() → refCnt: 4→3  bufDesc.release() → refCnt: 3→2
```

No mutual exclusion — both readers hold shared locks on `contentLock` concurrently.

### 3.5 Concurrent Write + Write (Different Blocks)

Writes to different blocks in the same file are serialized at the file level (`File.mu`) but operate on different buffers in parallel:

```
Writer A (block 0)                Writer B (block 5)
──────────────────                ──────────────────
f.mu.Lock()                       (waits for Writer A to release f.mu)
  create/get block[0]
  f.pendingWriters.Add(1)
f.mu.Unlock()
                                  f.mu.Lock()
                                    create/get block[5]
                                    f.pendingWriters.Add(1)
                                  f.mu.Unlock()

bufDesc0.contentLock.Lock()       bufDesc5.contentLock.Lock()
  copy(bufDesc0.buf, userData)      copy(bufDesc5.buf, userData)
bufDesc0.contentLock.Unlock()     bufDesc5.contentLock.Unlock()

f.pendingWriters.Done()           f.pendingWriters.Done()
```

The file lock serializes block list access, but actual data copying to different blocks proceeds in parallel.

### 3.6 Concurrent Write + Read (Same Block)

A reader and writer accessing the same block are coordinated through `contentLock`:

```
Writer                             Reader
──────                             ──────
bufDesc.contentLock.Lock()
  copy(bufDesc.buf, writeData)     bufDesc.contentLock.RLock()  ← BLOCKS
  bufDesc.dirty.Store(true)          (waiting for writer to release)
bufDesc.contentLock.Unlock()
                                     copy(userBuf, bufDesc.buf) ← proceeds
                                   bufDesc.contentLock.RUnlock()
```

The reader waits for the writer to finish its copy operation. This ensures the reader always sees a consistent snapshot of the block data.

### 3.7 Download and Read Coordination

When a block is not yet cached, the download path coordinates with readers through the content lock:

```
GetOrCreateBufferDescriptor():
  bufDesc.contentLock.Lock()       ← acquired BEFORE returning to caller
  → schedule download to worker pool

Worker (downloadBlock):            Reader (ensureBufferValidForRead):
──────────────────────             ─────────────────────────────────
ReadInBuffer(storage→buf)          if bufDesc.valid.Load() → return (fast)
bufDesc.valid.Store(true)
bufDesc.contentLock.Unlock()       bufDesc.contentLock.RLock()  ← waits here
                                   bufDesc.contentLock.RUnlock()
                                   check valid & downloadErr → proceed
```

The content lock is acquired **before** scheduling the download, and released **by the worker** after the download completes. Any reader that arrives before download finishes will block on `contentLock.RLock()` until the data is ready.

### 3.8 Flush Coordination with Pending Writers

`flush()` uses a `sync.WaitGroup` to wait for all in-flight writes:

```
Writer                              Flush
──────                              ─────
f.mu.Lock()                         f.mu.Lock()  ← blocks until writer releases
  f.pendingWriters.Add(1)
f.mu.Unlock()
  ... (buffer I/O) ...
  f.pendingWriters.Done()

                                    f.pendingWriters.Wait() ← waits for all writers
                                    ... upload all dirty blocks ...
                                    ... CommitData (PutBlockList) ...
                                    f.synced = true
                                    f.mu.Unlock()
```

**Key**: `pendingWriters.Add(1)` happens under the file lock, ensuring that once `flush()` acquires the lock, no new writers can increment the wait group. `flush()` then safely calls `Wait()` knowing the count will only decrease.

### 3.9 Sticky Error Semantics

`File.err` (`atomic.Value`) captures the first error encountered during file operations:

```go
// On upload failure:
block.file.err.Store(err.Error())

// On subsequent write attempt:
if f.err.Load() != nil {
    return fmt.Errorf("previous write error: %v", f.err.Load())
}
```

Once an error is stored, all subsequent writes and flushes fail immediately. This prevents cascading data corruption — if one block's upload fails, the entire file is marked as having an error rather than allowing partial writes to continue.

### 3.10 Summary of Synchronization Primitives

| Primitive | Type | Location | Purpose |
|---|---|---|---|
| `File.mu` | `sync.RWMutex` | `file.go` | Protects file metadata and block list |
| `File.pendingWriters` | `sync.WaitGroup` | `file.go` | Coordinates flush with in-flight writes |
| `File.err` | `atomic.Value` | `file.go` | Sticky error state (lock-free) |
| `File.size` | `int64` (atomic) | `file.go` | File size updated via CAS loop |
| `block.mu` | `sync.RWMutex` | `block.go` | Prevents duplicate buffer creation per block |
| `block.state` | `int32` (atomic) | `block.go` | Block state transitions (lock-free) |
| `block.numWrites` | `atomic.Int32` | `block.go` | Tracks modifications to committed blocks |
| `BufferTableMgr.mu` | `sync.RWMutex` | `buffer_mgr.go` | Protects block→buffer mapping |
| `bufferDescriptor.refCnt` | `atomic.Int32` | `buffer_descriptor.go` | Reference count (pin/unpin) |
| `bufferDescriptor.contentLock` | `sync.RWMutex` | `buffer_descriptor.go` | Protects buffer data bytes |
| `bufferDescriptor.valid` | `atomic.Bool` | `buffer_descriptor.go` | Download completion flag |
| `bufferDescriptor.dirty` | `atomic.Bool` | `buffer_descriptor.go` | Modification tracking |
| `freeListType.mutex` | `sync.Mutex` | `freelist.go` | Protects free list linked-list |
| `fileMap` | `sync.Map` | `handle.go` | Global file path → File lookup |
| `workerPool.tasks` | `chan *task` | `worker.go` | Work queue (channel provides synchronization) |
