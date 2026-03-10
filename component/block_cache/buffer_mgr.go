package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

var (
	errBuffersExhausted error = fmt.Errorf("No free buffers available")
)

// BufferTableMgr manages the mapping between blocks and their associated buffer descriptors.
// It maintains a table (map) that tracks which buffer is caching which block's data.
// Thread-safety is provided by a read-write mutex allowing concurrent lookups.
type BufferTableMgr struct {
	table map[*block]*bufferDescriptor // Maps blocks to their buffer descriptors
	mu    sync.RWMutex                 // Protects concurrent access to the table
}

func newBufferTableMgr() *BufferTableMgr {
	return &BufferTableMgr{
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
func (btm *BufferTableMgr) GetOrCreateBufferDescriptor(freeList *freeListType, workerPool *workerPool, blk *block, sync bool) (*bufferDescriptor, bufDescStatus, error) {
	stime := time.Now()

	log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Requesting buffer for blockIdx: %d, sync: %v, file: %s",
		blk.idx, sync, blk.file.Name)

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
			log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Existing bufferIdx: %d, blockIdx: %d, sync: %v, has error: %v",
				bufDesc.bufIdx, blk.idx, sync, err)

			if ok := bufDesc.release(freeList); ok {
				log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after error: %v",
					bufDesc.bufIdx, blk.idx, err)
			}
			return nil, bufDescStatusInvalid, err
		}
	}

	// Step 5: Check if block is in uncommitted state (requires file flush before reading)
	// You cannot read the uncommited data from azure storage, so we need to flush the file first.
	if blk.state == uncommitedBlock {
		// Release the lock on buffer table manager.
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Cannot create buffer for blockIdx: %d in uncommitedBlock state, file: %s flush needed",
			blk.idx, blk.file.Name)

		return nil, bufDescStatusNeedsFileFlush, nil
	}

	// download is needed only for committed blocks.
	doRead := (blk.state == committedBlock)

	victim := false
	// Get the Buffer Descriptor from free list.
	bufDesc, err = freeList.allocateBuffer(blk)
	if err == errFreeListFull {
		// Failed to allocate buffer from free list, as free list is full. Need to evict a buffer.
		log.Info("BufferTableMgr::GetOrCreateBufferDescriptor: Failed to allocate buffer for blockIdx: %d, sync: %v: %v",
			blk.idx, sync, err)

		// for readahead blocks, there is no need to get the block by getting victim buffer, just fail with error.
		if !sync {
			// Release the lock on buffer table manager.
			btm.mu.Unlock()

			log.Info("BufferTableMgr::GetOrCreateBufferDescriptor: Async request for blockIdx: %d, sync: %v failed to allocate buffer and will not retry with eviction, file: %s",
				blk.idx, sync, blk.file.Name)
			return nil, bufDescStatusInvalid, errBuffersExhausted
		}

		retries := 1

		// Retry loop to find a victim buffer for eviction.
		for !victim {
			// While getting the victim buffer, there is no point in holding on to the buffer table manager lock.
			btm.mu.Unlock()

			// No free buffer present in freeList, need to evict a buffer. Request a victim buffer from Buffers in use list.
			bufDesc, err = freeList.getVictimBuffer(workerPool, btm)
			if err != nil {
				// This should never happen as we just failed to allocate a buffer from free list.
				log.Crit(fmt.Sprintf("BufferTableMgr::GetOrCreateBufferDescriptor: Failed to get victim buffer for blockIdx: %d, file: %s, sync: %v: %v",
					blk.idx, blk.file.Name, sync, err))
				return nil, bufDescStatusInvalid, err
			}

			// Re-acquire the lock on buffer table manager to update the table.
			btm.mu.Lock()

			victimRefCnt := bufDesc.refCnt.Load()
			if victimRefCnt == refCountTableAndOneUser && !bufDesc.dirty.Load() {
				// Victim buffer is not in use, can evict.
				victim = true
			} else {
				// There is a slight chance that between the time we selected the victim buffer and now, someone else
				// acquired a reference to this buffer. But this should be very rare, if eviction is working correctly/
				// we are just unlucky:(
				if victimRefCnt < 1 {
					// as we took a reference while getting victim, refCnt should never be less than 1 here.
					err := fmt.Sprintf("BufferTableMgr::GetOrCreateBufferDescriptor: Victim bufferIdx: %d for blockIdx: %d has invalid refCount: %d, something is wrong",
						bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load())
					panic(err)
				}

				// Victim buffer is still in use, cannot evict. Retry getting another victim.
				// Reduce the refCnt on victim buffer that was chosen.
				if ok := bufDesc.release(freeList); ok {
					log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released victim bufferIdx: %d for blockIdx: %d back to free list after failed eviction attempt, file: %s",
						bufDesc.bufIdx, bufDesc.block.idx, blk.file.Name)
				}

				log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Victim bufferIdx: %d, blockIdx: %d has refCount: %d, dirty: %v for blockIdx: %d, sync: %v, retries: %d retrying eviction",
					bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load(), bufDesc.dirty.Load(), blk.idx, sync, retries)
				retries++
			}
		}

		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Evicting bufferIdx: %d for blockIdx: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, sync)
	}

	if victim {
		// Eviction successful: remove victim buffer's old block mapping and reset it for reuse
		delete(btm.table, bufDesc.block)
		bufDesc.reset(freeList)
	}

	// Step 6: Add the new buffer descriptor to the table and initialize it
	// Initialize buffer with refCnt=2
	// 1 for table + 1 for caller
	btm.table[blk] = bufDesc
	bufDesc.refCnt.Store(refCountTableAndOneUser)
	bufDesc.block = blk

	// Step 7: Prepare buffer for download if needed
	// Lock buffer content before releasing table lock to prevent others from accessing incomplete buffer
	if doRead {
		// Download needed - lock buffer content until download completes
		// This lock will be released after download completes in the worker goroutine
		bufDesc.contentLock.Lock()
	} else {
		// For write operations, buffer doesn't need download - mark as valid and dirty immediately
		bufDesc.valid.Store(true)
		bufDesc.dirty.Store(true)
	}

	// Release the lock on buffer table manager.
	btm.mu.Unlock()

	// This is where we should downlod the blockdata into the buffer, check the blocks flag status.
	if doRead {
		blk.scheduleDownload(workerPool, freeList, bufDesc, sync)

		if sync {
			// Check if there was any error during download, and also blocks here until download is complete.
			if err := bufDesc.ensureBufferValidForRead(); err != nil {
				log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Download block failed for file: %s, blockIdx: %d: %v, err: %v",
					blk.file.Name, blk.idx, bufDesc.downloadErr, err)

				if ok := bufDesc.release(freeList); ok {
					log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after download failure: %v",
						bufDesc.bufIdx, blk.idx)
				}
				return nil, bufDescStatusInvalid, err
			}
		}
	}

	if !sync {
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Async scheduling download for bufferIdx: %d, blockIdx: %d took %v, file: %s",
			bufDesc.bufIdx, blk.idx, time.Since(stime), blk.file.Name)
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
// If the buffer is removed from the bufferTableMgr, it also drops the reference for the caller, so the caller
// must not use the buffer descriptor when removal is successful and also caller must not call release() on this
// buffer descriptor as well, as the reference will be dropped for the caller as well in this function.
//
// Parameters:
//   - bufDesc: The buffer descriptor to remove
//
// Returns:
//   - isRemovedFromBufMgr: true if buffer was removed from the table,
//     false if it was not removed (e.g., due to being dirty or having too many references)
//
// Reference counting semantics:
//   - Buffer must not be dirty (flush required first)
//   - Buffer must have refCnt == 2 (only table + caller reference) to be safely removed, otherwise removal fails.
//   - refCnt < 2 for the caller means something is sus.
//   - On successful removal from table, refCnt is decremented to release table's reference & also for caller's reference.
//     this is done to prevent other users from acquiring reference to this buffer after it's removed from the table and
//     also our victim selection logic relies on table refCnt to determine if buffer is in use or not.
func (btm *BufferTableMgr) removeBufferDescriptor(bufDesc *bufferDescriptor, freeList *freeListType) (isRemovedFromBufMgr bool) {
	blk := bufDesc.block
	log.Debug("BufferTableMgr::removeBufferDescriptor: Remove blockIdx: %d, bufferIdx: %d for file: %s from buffer table",
		bufDesc.block.idx, bufDesc.bufIdx, blk.file.Name)

	btm.mu.Lock()
	defer btm.mu.Unlock()

	// Check 1: Cannot remove dirty buffers (data not yet uploaded to storage)
	if bufDesc.dirty.Load() {
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove dirty bufferIdx: %d for blockIdx: %d, flush needed before reading block data, file: %s",
			bufDesc.bufIdx, blk.idx, blk.file.Name)
		return false
	}

	// Check 2: In strict mode, ensure no extra user references exist (only table reference allowed + one user reference for the caller)
	// refCnt > 2 means: 1 (table) + (1 + N) (active users), so removal would be unsafe
	curRefCnt := bufDesc.refCnt.Load()
	if curRefCnt > refCountTableAndOneUser {
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove bufferIdx: %d for blockIdx: %d, refCnt: %d >= refCntTableAndOneUser, file: %s",
			bufDesc.bufIdx, blk.idx, curRefCnt, blk.file.Name)
		return false
	}

	if curRefCnt <= refCountTableOnly {
		// This should not happen as the caller should be holding a reference to this buffer if this control came here
		// which means there is a bug in the code where refCnt is being decremented incorrectly somewhere, as the caller
		// should have at least refCnt>1 for its reference.
		panic(fmt.Sprintf("BufferTableMgr::removeBufferDescriptor: BufferIdx: %d[%v] for blockIdx: %d has refCnt: %d which is unexpected, something is wrong, file: %s",
			bufDesc.bufIdx, bufDesc, blk.idx, curRefCnt, blk.file.Name))
	}

	// Check 3: Verify buffer is still in the table (This cannot happen as we have 2 references to this buffer,
	// but adding for extra safety)
	if _, ok := btm.table[bufDesc.block]; !ok {
		panic(fmt.Sprintf("BufferTableMgr::removeBufferDescriptor: BufferIdx: %d[%v] for blockIdx: %d not found in buffer table during removal, something is wrong, file: %s",
			bufDesc.bufIdx, bufDesc, blk.idx, blk.file.Name))
	}

	// Step 1: Remove buffer from the table (no longer mapped to this block)
	delete(btm.table, bufDesc.block)

	// Step 2: Release the buffer descriptor reference held by the table and also for the caller's reference.

	if ok := bufDesc.release(freeList); ok {
		// This should not release the buffer to free list as the caller should also be holding a reference to this buffer,
		// so refCnt should be 1 after this release, This means bug in the code and refCnt is being decremented incorrectly somewhere.
		panic(fmt.Sprintf("BufferTableMgr::removeBufferDescriptor: Failed to release bufferDescriptor: %v for blockIdx: %d back to free list after removal from buffer table, file: %s",
			bufDesc, blk.idx, blk.file.Name))
	}

	if ok := bufDesc.release(freeList); ok {
		log.Debug("BufferTableMgr::removeBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after removal from buffer table, file: %s",
			bufDesc.bufIdx, blk.idx, blk.file.Name)
		return true
	}

	// This should not happen as the caller should be holding a reference to this buffer if this control came here which
	// is not expected.
	panic(fmt.Sprintf("BufferTableMgr::removeBufferDescriptor: Failed to release bufferDescriptor: %v for blockIdx: %d back to free list after removal from buffer table, file: %s",
		bufDesc, blk.idx, blk.file.Name))
}
