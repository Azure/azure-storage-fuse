/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

var (
	errBuffersExhausted error = fmt.Errorf("no free buffers available")
)

// bufferTableMgr manages the mapping between blocks and their associated buffer descriptors.
// It maintains a table (map) that tracks which buffer is caching which block's data.
// Thread-safety is provided by a read-write mutex allowing concurrent lookups.
type bufferTableMgr struct {
	table map[*block]*bufferDescriptor // Maps blocks to their buffer descriptors
	mu    sync.RWMutex                 // Protects concurrent access to the table
}

func newBufferTableMgr() *bufferTableMgr {
	return &bufferTableMgr{
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

// getOrCreateBufferDescriptor retrieves an existing buffer for a block or allocates a new one.
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
func (btm *bufferTableMgr) getOrCreateBufferDescriptor(freeList *freeListType, workerPool *workerPool, blk *block, sync bool) (*bufferDescriptor, bufDescStatus, error) {
	stime := time.Now()

	log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Requesting buffer for blockIdx: %d, sync: %v, file: %s",
		blk.idx, sync, blk.file.Name)

	// Step 1: Check if buffer already exists for this block (fast path)
	bufDesc, err := btm.lookupBufferDescriptor(blk, freeList)
	if bufDesc != nil {
		// Buffer exists, refCnt already incremented by LookUp
		log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Found existing bufferIdx: %d, blockIdx: %d, took: %v, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
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
		log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: (Double Check) Found existing bufferIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), sync)

		btm.mu.Unlock()

		// Ensure the buffer is valid before returning
		if err := bufDesc.ensureBufferValidForRead(); err == nil {
			return bufDesc, bufDescStatusExists, nil
		} else {
			// Buffer has download error, release our reference and return error
			log.Err("bufferTableMgr::getOrCreateBufferDescriptor: Existing bufferIdx: %d, blockIdx: %d, sync: %v, has error: %v",
				bufDesc.bufIdx, blk.idx, sync, err)

			if ok := bufDesc.release(freeList); ok {
				log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after error: %v",
					bufDesc.bufIdx, blk.idx, err)
			}
			return nil, bufDescStatusInvalid, err
		}
	}

	// Step 5: Check if block is in uncommitted state (requires file flush before reading)
	// You cannot read the uncommitted data from azure storage, so we need to flush the file first.
	if blk.getState() == uncommitedBlock {
		// Release the lock on buffer table manager.
		btm.mu.Unlock()
		log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Cannot create buffer for blockIdx: %d in uncommitedBlock state, file: %s flush needed",
			blk.idx, blk.file.Name)

		return nil, bufDescStatusNeedsFileFlush, nil
	}

	// download is needed only for committed blocks.
	doRead := (blk.getState() == committedBlock)

	victim := false
	// Get the Buffer Descriptor from free list.
	bufDesc, err = freeList.allocateBuffer(blk)
	if err == errFreeListFull {
		// Failed to allocate buffer from free list, as free list is full. Need to evict a buffer.
		log.Info("bufferTableMgr::getOrCreateBufferDescriptor: Failed to allocate buffer for blockIdx: %d, sync: %v: %v",
			blk.idx, sync, err)

		// for readahead blocks, there is no need to get the block by getting victim buffer, just fail with error.
		if !sync {
			// Release the lock on buffer table manager.
			btm.mu.Unlock()

			log.Info("bufferTableMgr::getOrCreateBufferDescriptor: Async request for blockIdx: %d, sync: %v failed to allocate buffer and will not retry with eviction, file: %s",
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
				log.Crit("bufferTableMgr::getOrCreateBufferDescriptor: Failed to get victim buffer for blockIdx: %d, file: %s, sync: %v: %v",
					blk.idx, blk.file.Name, sync, err)
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
					err := fmt.Sprintf("bufferTableMgr::getOrCreateBufferDescriptor: Victim bufferIdx: %d for blockIdx: %d has invalid refCount: %d, something is wrong",
						bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load())
					panic(err)
				}

				// Victim buffer is still in use, cannot evict. Retry getting another victim.
				// Reduce the refCnt on victim buffer that was chosen.
				if ok := bufDesc.release(freeList); ok {
					log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Released victim bufferIdx: %d for blockIdx: %d back to free list after failed eviction attempt, file: %s",
						bufDesc.bufIdx, bufDesc.block.idx, blk.file.Name)
				}

				log.Err("bufferTableMgr::getOrCreateBufferDescriptor: Victim bufferIdx: %d, blockIdx: %d has refCount: %d, dirty: %v for blockIdx: %d, sync: %v, retries: %d retrying eviction",
					bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load(), bufDesc.dirty.Load(), blk.idx, sync, retries)
				retries++
			}
		}

		log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Evicting bufferIdx: %d for blockIdx: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, sync)
	}

	if victim {
		// Eviction successful: remove victim buffer's old block mapping and reset it for reuse
		delete(btm.table, bufDesc.block)
		bufDesc.reset()
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

	// This is where we should download the blockdata into the buffer, check the blocks flag status.
	if doRead {
		blk.scheduleDownload(workerPool, freeList, bufDesc, sync)

		if sync {
			// Check if there was any error during download, and also blocks here until download is complete.
			if err := bufDesc.ensureBufferValidForRead(); err != nil {
				log.Err("bufferTableMgr::getOrCreateBufferDescriptor: Download block failed for file: %s, blockIdx: %d: %v, err: %v",
					blk.file.Name, blk.idx, bufDesc.downloadErr, err)

				if ok := bufDesc.release(freeList); ok {
					log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after download failure: %v",
						bufDesc.bufIdx, blk.idx, err)
				}
				return nil, bufDescStatusInvalid, err
			}
		}
	}

	if !sync {
		log.Debug("bufferTableMgr::getOrCreateBufferDescriptor: Async scheduling download for bufferIdx: %d, blockIdx: %d took %v, file: %s",
			bufDesc.bufIdx, blk.idx, time.Since(stime), blk.file.Name)
	}

	if victim {
		return bufDesc, bufDescStatusVictim, nil
	}
	return bufDesc, bufDescStatusAllocated, nil
}

// lookupBufferDescriptor searches for an existing buffer descriptor for the given block.
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
func (btm *bufferTableMgr) lookupBufferDescriptor(blk *block, fl *freeListType) (*bufferDescriptor, error) {
	btm.mu.RLock()
	bufDesc, exists := btm.table[blk]
	if exists {
		bufDesc.refCnt.Add(1)
		log.Debug("bufferTableMgr::lookupBufferDescriptor: Looked up bufferIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load())

		// Release the read lock on buffer table manager.
		btm.mu.RUnlock()

		if err := bufDesc.ensureBufferValidForRead(); err != nil {
			log.Err("bufferTableMgr::lookupBufferDescriptor: BufferIdx: %d for blockIdx: %d has error: %v during lookup, file: %s",
				bufDesc.bufIdx, blk.idx, bufDesc.downloadErr, blk.file.Name)
			btm.detachBufferDescriptor(bufDesc, fl)
			bufDesc.release(fl)
			return nil, err
		}

		return bufDesc, nil
	}

	btm.mu.RUnlock()
	return nil, nil
}

// detachBufferDescriptor removes the exact block-to-buffer mapping from the table.
// It releases only the reference owned by the table. Callers retain ownership of
// their own reference and must release it independently.
//
// Parameters:
//   - bufDesc: The buffer descriptor to remove
//
// Returns:
//   - true if this descriptor was detached from the table
//   - false if the mapping was absent or now points to another descriptor
//
// Detaching does not require the descriptor to be clean or idle. Existing users
// keep it alive through their references, while new lookups can no longer acquire it.
// This is also the cleanup path for terminal I/O errors.
func (btm *bufferTableMgr) detachBufferDescriptor(bufDesc *bufferDescriptor, freeList *freeListType) bool {
	blk := bufDesc.block
	log.Debug("bufferTableMgr::detachBufferDescriptor: Detach blockIdx: %d, bufferIdx: %d for file: %s from buffer table",
		bufDesc.block.idx, bufDesc.bufIdx, blk.file.Name)

	btm.mu.Lock()
	current, ok := btm.table[blk]
	if !ok || current != bufDesc {
		btm.mu.Unlock()
		return false
	}
	delete(btm.table, blk)
	btm.mu.Unlock()

	bufDesc.release(freeList)
	return true
}
