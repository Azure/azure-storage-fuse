package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	errFlushNeeded = "Flush needed before reading block data"
)

var btm *BufferTableMgr

// BufferTableMgr manages the mapping between blocks and their associated buffer descriptors.
// It maintains a table (map) that tracks which buffer is caching which block's data.
// Thread-safety is provided by a read-write mutex allowing concurrent lookups.
type BufferTableMgr struct {
	table map[*block]*bufferDescriptor // Maps blocks to their buffer descriptors
	mu    sync.RWMutex                 // Protects concurrent access to the table
}

func NewBufferTableMgr() {
	btm = &BufferTableMgr{
		table: make(map[*block]*bufferDescriptor),
	}
}

type bufDescStatus int

const (
	bufDescStatusExists bufDescStatus = iota
	bufDescStatusAllocated
	bufDescStatusVictim
	bufDescStatusNeedsFileFlush
	bufDescStatusInvalid
)

// Map bufDescStatus values to their string representations
func (b bufDescStatus) String() string {
	switch b {
	case bufDescStatusExists:
		return "bufDescStatusExists"
	case bufDescStatusAllocated:
		return "bufDescStatusAllocated"
	case bufDescStatusVictim:
		return "bufDescStatusVictim"
	case bufDescStatusNeedsFileFlush:
		return "bufDescStatusNeedsFileFlush"
	case bufDescStatusInvalid:
		return "bufDescStatusInvalid"
	default:
		return "Unknown"
	}
}

// GetOrCreateBufferDescriptor retrieves an existing buffer for a block or allocates a new one.
// This is the main entry point for accessing block data through the buffer cache.
//
// Parameters:
//   - blk: The block for which we need a buffer
//   - doesRead: If true, buffer data will be downloaded from storage (for read operations)
//   - sync: If true, operations (download/upload) complete before returning; if false, they run asynchronously
//
// Returns:
//   - bufferDescriptor: The buffer holding (or will hold) the block's data
//   - bufDescStatus: Status indicating if buffer existed, was allocated, was a victim, etc.
//   - error: Any error encountered during buffer acquisition or download
//
// Reference counting flow:
//  1. If buffer exists: refCnt is incremented by LookUp (user acquires reference)
//  2. If buffer doesn't exist: new buffer allocated with refCnt=1 (table holds initial reference)
//  3. Caller must call release() when done to decrement refCnt
//
// Thread-safety: Uses block-level locking to prevent concurrent creation for the same block
func GetOrCreateBufferDescriptor(blk *block, doesRead bool, sync bool) (*bufferDescriptor, bufDescStatus, error) {
	stime := time.Now()

	log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Requesting buffer for blockIdx: %d, doesRead: %v, sync: %v, file: %s",
		blk.idx, doesRead, sync, blk.file.Name)

	// Step 1: Check if buffer already exists for this block (fast path)
	bufDesc, err := btm.LookUpBufferDescriptor(blk)
	if bufDesc != nil {
		// Buffer exists, refCnt already incremented by LookUp
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Found existing bufferIdx: %d, blockIdx: %d, took: %v, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, time.Since(stime), bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), sync)
		return bufDesc, bufDescStatusExists, nil
	}
	if err != nil {
		return nil, bufDescStatusInvalid, err
	}

	// Step 2: Buffer doesn't exist, need to create one (slow path)
	// Lock the block to prevent multiple goroutines from creating buffers for the same block
	blk.mu.Lock()
	defer blk.mu.Unlock()

	// Step 3: Acquire buffer table lock for modifications
	btm.mu.Lock()

	// Step 4: Double-check pattern - another goroutine may have created the buffer while we waited for the lock
	bufDesc, exists := btm.table[blk]
	if exists {
		// Another goroutine created the buffer, increment refCnt and use it
		bufDesc.refCnt.Add(1)
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: (Double Check) Found existing bufferIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), sync)

		btm.mu.Unlock()

		// Ensure the buffer is valid before returning
		if err := bufDesc.ensureBufferValidForRead(); err == nil {
			return bufDesc, bufDescStatusExists, nil
		} else {
			// Buffer has download error, release our reference and return error
			log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Existing bufferIdx: %d, blockIdx: %d, sync: %v, has download error",
				bufDesc.bufIdx, blk.idx, sync)

			if ok := bufDesc.release(); ok {
				log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after download error: %v",
					bufDesc.bufIdx, blk.idx, err)
			}
			return nil, bufDescStatusInvalid, err
		}
	}

	// Step 5: Check if block is in uncommitted state (requires file flush before reading)
	if blk.state == uncommitedBlock {
		// Release the lock on buffer table manager.
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Cannot create buffer for blockIdx: %d in uncommitedBlock state, file: %s flush needed",
			blk.idx, blk.file.Name)

		return nil, bufDescStatusNeedsFileFlush, nil
	}

	victim := false
	// Get the Buffer Descriptor from free list.
	bufDesc, err = freeList.allocateBuffer(blk)
	if err == errFreeListFull {
		// TODO: Do we really need to get the victim buffer for readaheads as well?
		// Failed to allocate buffer from free list, as free list is full. Need to evict a buffer.
		log.Info("BufferTableMgr::GetOrCreateBufferDescriptor: Failed to allocate buffer for blockIdx: %d, sync: %v: %v",
			blk.idx, sync, err)
		victim = true
		retries := 1

	retry:
		// While getting the victim buffer, there is no point in holding on to the buffer table manager lock.
		btm.mu.Unlock()

		// No free buffer present in freeList, need to evict a buffer. Request a victim buffer from Buffers in use list.
		bufDesc = freeList.getVictimBuffer()

		// Re-acquire the lock on buffer table manager to update the table.
		btm.mu.Lock()

		victimRefCnt := bufDesc.refCnt.Load()
		if victimRefCnt > 1 || victimRefCnt == 0 {
			// There is a slight chance that between the time we selected the victim buffer and now, someone else
			// acquired a reference to this buffer. But this should be very rare, if eviction is working correctly.

			log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Victim bufferIdx: %d, blockIdx: %d has refCount: %d for blockIdx: %d, sync: %v, retries: %d, retrying eviction",
				bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load(), blk.idx, sync, retries)
			retries++
			goto retry
		} else if victimRefCnt < 1 {
			err := fmt.Sprintf("BufferTableMgr::GetOrCreateBufferDescriptor: Victim bufferIdx: %d for blockIdx: %d has invalid refCount: %d, something is wrong",
				bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load())
			panic(err)
		}

		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Evicting bufferIdx: %d for blockIdx: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, sync)
	}

	if victim {
		// Eviction successful: remove victim buffer's old block mapping and reset it for reuse
		delete(btm.table, bufDesc.block)
		bufDesc.reset()
	}

	// Step 6: Add the new buffer descriptor to the table and initialize it
	btm.table[blk] = bufDesc

	// Initialize buffer with refCnt=1 (table holds the initial reference)
	// Callers who use this buffer will have their reference already counted (returned from this function)
	bufDesc.refCnt.Store(1)
	bufDesc.block = blk

	// Step 7: Prepare buffer for download if needed
	// Lock buffer content before releasing table lock to prevent others from accessing incomplete buffer
	if doesRead {
		bufDesc.contentLock.Lock()
		// This lock will be released after download completes in the worker goroutine
	} else {
		// For write operations, buffer doesn't need download - mark as valid and dirty immediately
		bufDesc.valid.Store(true)
		bufDesc.dirty.Store(true)
	}

	// Release the lock on buffer table manager.
	btm.mu.Unlock()

	// This is where we should downlod the blockdata into the buffer, check the blocks flag status.
	if doesRead {
		if blk.state == localBlock {
			// This block is already present locally, this is a hole created.
			bufDesc.valid.Store(true)
			bufDesc.dirty.Store(true)
			bufDesc.contentLock.Unlock()
			log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: BlockIdx: %d is a localBlock (hole), no download needed, file: %s",
				blk.idx, blk.file.Name)
		} else {

			blk.scheduleDownload(bufDesc, sync)

			if sync {
				// Check if there was any error during download.
				if err := bufDesc.ensureBufferValidForRead(); err != nil {
					log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Download block failed for file: %s, blockIdx: %d: %v",
						blk.file.Name, blk.idx, bufDesc.downloadErr)

					if ok := bufDesc.release(); ok {
						log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after download failure: %v",
							bufDesc.bufIdx, blk.idx)
					}
					return nil, bufDescStatusInvalid, err
				}
			}
		}
	}

	if !sync {
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Async scheduling download for bufferIdx: %d, blockIdx: %d took %v, file: %s",
			bufDesc.bufIdx, blk.idx, time.Since(stime))
	}

	if victim {
		return bufDesc, bufDescStatusVictim, nil
	}
	return bufDesc, bufDescStatusAllocated, nil
}

// LookUpBufferDescriptor searches for an existing buffer descriptor for the given block.
// If found, it increments the reference count to prevent the buffer from being evicted while in use.
//
// Parameters:
//   - blk: The block to look up
//
// Returns:
//   - bufferDescriptor: The buffer if found, nil if not found
//   - error: Any error during validation (e.g., download error on the buffer)
//
// Reference counting:
//   - If buffer exists, refCnt is incremented atomically (user acquires reference)
//   - Caller MUST call release() when done to decrement refCnt
//   - Increment happens while holding bufferTableMgr lock to ensure thread-safety
//
// Thread-safety: Uses read lock for lookup, allowing concurrent lookups by multiple threads
func (btm *BufferTableMgr) LookUpBufferDescriptor(blk *block) (*bufferDescriptor, error) {
	btm.mu.RLock()
	bufDesc, exists := btm.table[blk]
	if exists {
		bufDesc.refCnt.Add(1)
		log.Debug("BufferTableMgr::LookUpBufferDescriptor: Looked up bufferIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load())

		// Release the read lock on buffer table manager.
		btm.mu.RUnlock()

		if err := bufDesc.ensureBufferValidForRead(); err != nil {
			return nil, err
		}

		return bufDesc, nil
	}

	btm.mu.RUnlock()
	return nil, nil
}

// removeBufferDescriptor removes a buffer descriptor from the buffer table and releases it if no longer in use.
//
// Parameters:
//   - bufDesc: The buffer descriptor to remove
//   - strict: If true, removal fails if there are active user references (refCnt > 1)
//     If false, removal proceeds even if users still hold references
//
// Returns:
//   - isRemovedFromBufMgr: true if buffer was removed from the table
//   - isReleasedToFreeList: true if buffer was returned to free list (refCnt reached 0)
//
// Reference counting semantics:
//   - Buffer must not be dirty (flush required first)
//   - In strict mode: fails if refCnt > 1 (users other than table hold references)
//   - Removes buffer from table, then decrements refCnt (releases table's reference)
//   - If refCnt reaches 0 after decrement, buffer is returned to free list
//   - If refCnt > 0 after decrement, other users still hold references (buffer not freed yet)
//
// Use cases:
//   - strict=true: Used when we want to ensure no active users (e.g., file closure)
//   - strict=false: Used for eviction (acceptable to remove even if users exist)
func (btm *BufferTableMgr) removeBufferDescriptor(bufDesc *bufferDescriptor, strict bool) (isRemovedFromBufMgr bool, isReleasedToFreeList bool) {
	log.Debug("BufferTableMgr::removeBufferDescriptor: Remove blockIdx: %d, bufferIdx: %d for file: %s from buffer table",
		bufDesc.block.idx, bufDesc.bufIdx, bufDesc.block.file.Name)

	btm.mu.Lock()

	// Check 1: Cannot remove dirty buffers (data not yet uploaded to storage)
	if bufDesc.dirty.Load() {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove dirty bufferIdx: %d for blockIdx: %d, flush needed before reading block data",
			bufDesc.bufIdx, bufDesc.block.idx)
		return false, false
	}

	// Check 2: In strict mode, ensure no user references exist (only table reference allowed)
	// refCnt > 1 means: 1 (table) + N (active users), so removal would be unsafe
	if strict && bufDesc.refCnt.Load() > 1 {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove bufferIdx: %d for blockIdx: %d, refCnt: %d > 1",
			bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load())
		return false, false
	}

	// Check 3: Verify buffer is still in the table (may have been removed by another goroutine)
	if _, ok := btm.table[bufDesc.block]; !ok {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: BufferIdx: %d not found in buffer table, already removed",
			bufDesc.bufIdx)
		return true, false
	}

	// Step 1: Remove buffer from the table (no longer mapped to this block)
	delete(btm.table, bufDesc.block)
	btm.mu.Unlock()

	// Step 2: Decrement refCnt to release table's reference
	// If refCnt becomes 0, no one is using the buffer anymore and it can be freed
	// If refCnt > 0, other users still hold references (they will release later)
	if bufDesc.refCnt.Add(-1) == 0 {
		// Buffer completely released - return to free list for reuse
		log.Debug("BufferTableMgr::removeBufferDescriptor: Released bufferIdx: %d, blockIdx: %d back to free list",
			bufDesc.bufIdx, bufDesc.block.idx)
		freeList.releaseBuffer(bufDesc)
		return true, true
	}

	// Buffer removed from table but still has active user references
	// Users will eventually release() and the last one will return buffer to free list
	return true, false
}
