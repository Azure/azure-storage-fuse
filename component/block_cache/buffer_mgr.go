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
	"errors"
	"fmt"
	"sync"
)

var (
	errBuffersExhausted error = fmt.Errorf("no free buffers available")
)

type bufferAccessKind uint8

const (
	accessDemand bufferAccessKind = iota
	accessPrefetch
	accessWrite
	accessMaintenance
)

func (kind bufferAccessKind) synchronous() bool {
	return kind != accessPrefetch
}

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
//   - access: Access class controlling synchronous I/O and clock-sweep promotion
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
func (btm *bufferTableMgr) getOrCreateBufferDescriptor(freeList *freeListType, workerPool *workerPool, blk *block, access bufferAccessKind) (*bufferDescriptor, bufDescStatus, error) {
	sync := access.synchronous()
	var prefetch *prefetchPermit
	if access == accessPrefetch {
		prefetch = workerPool.tryAcquirePrefetch()
		if prefetch == nil {
			return nil, bufDescStatusInvalid, errBuffersExhausted
		}
		defer func() {
			prefetch.release()
		}()
	}

	// Step 1: Check if buffer already exists for this block (fast path)
	bufDesc, err := btm.lookupBufferDescriptorForAccess(blk, freeList, access)
	if bufDesc != nil {
		// Buffer exists, refCnt already incremented by LookUp
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
		bufDesc.recordAccess(access)

		btm.mu.Unlock()

		// Ensure the buffer is valid before returning
		if err := bufDesc.ensureBufferValidForRead(); err == nil {
			return bufDesc, bufDescStatusExists, nil
		} else {
			// Buffer has download error, release our reference and return error
			bufDesc.release(freeList)
			return nil, bufDescStatusInvalid, fmt.Errorf("block %d download: %w", blk.idx, err)
		}
	}

	// Step 5: Check if block is in uncommitted state (requires file flush before reading)
	// You cannot read the uncommitted data from azure storage, so we need to flush the file first.
	if blk.getState() == uncommitedBlock {
		// Release the lock on buffer table manager.
		btm.mu.Unlock()
		return nil, bufDescStatusNeedsFileFlush, nil
	}

	// download is needed only for committed blocks.
	doRead := (blk.getState() == committedBlock)

	btm.mu.Unlock()
	bufDesc, status, err := btm.acquireBuffer(freeList, workerPool, blk, access)
	if err != nil {
		return nil, bufDescStatusInvalid, err
	}
	if !doRead {
		bufDesc.prepareForLocalWrite()
	}
	btm.mu.Lock()

	// Step 6: Add the new buffer descriptor to the table and initialize it
	// Initialize buffer with refCnt=2
	// 1 for table + 1 for caller
	btm.table[blk] = bufDesc
	bufDesc.refCnt.Store(refCountTableAndOneUser)
	bufDesc.block = blk
	bufDesc.recordAccess(access)

	// Step 7: Prepare buffer for download if needed
	// Lock buffer content before releasing table lock to prevent others from accessing incomplete buffer
	var contentLease *bufferContentLease
	if doRead {
		// Download needed - lock buffer content until download completes
		// The lease is transferred to the download task and released by worker cleanup.
		contentLease = bufDesc.lockContent()
	} else {
		// For write operations, buffer doesn't need download - mark as valid and dirty immediately
		bufDesc.valid.Store(true)
		bufDesc.dirty.Store(true)
	}

	// Release the lock on buffer table manager.
	btm.mu.Unlock()

	// This is where we should download the blockdata into the buffer, check the blocks flag status.
	if doRead {
		if err := blk.scheduleDownload(workerPool, bufDesc, contentLease, sync, prefetch); err != nil {
			btm.detachBufferDescriptor(bufDesc, freeList)
			bufDesc.release(freeList)
			return nil, bufDescStatusInvalid, err
		}
		if prefetch != nil {
			prefetch = nil
		}

		if sync {
			// Check if there was any error during download, and also blocks here until download is complete.
			if err := bufDesc.ensureBufferValidForRead(); err != nil {
				bufDesc.release(freeList)
				return nil, bufDescStatusInvalid, fmt.Errorf("block %d download: %w", blk.idx, err)
			}
		}
	}

	return bufDesc, status, nil
}

func (btm *bufferTableMgr) acquireBuffer(freeList *freeListType, workerPool *workerPool, blk *block, access bufferAccessKind) (*bufferDescriptor, bufDescStatus, error) {
	for {
		bufDesc, status, err := btm.tryAcquireBuffer(freeList, workerPool, blk, access)
		if err == nil || !errors.Is(err, errNoVictimBufferFound) {
			return bufDesc, status, err
		}
		if access == accessPrefetch {
			return nil, bufDescStatusInvalid, errBuffersExhausted
		}

		// Register before retrying so a descriptor transition cannot be lost
		// between the failed scan and the wait.
		changed, err := freeList.watchAvailability()
		if err != nil {
			return nil, bufDescStatusInvalid, err
		}
		bufDesc, status, err = btm.tryAcquireBuffer(freeList, workerPool, blk, access)
		if err == nil || !errors.Is(err, errNoVictimBufferFound) {
			freeList.stopWatchingAvailability()
			return bufDesc, status, err
		}
		<-changed
		freeList.stopWatchingAvailability()
	}
}

func (btm *bufferTableMgr) tryAcquireBuffer(freeList *freeListType, workerPool *workerPool, blk *block, access bufferAccessKind) (*bufferDescriptor, bufDescStatus, error) {
	bufDesc, err := freeList.allocateBuffer(blk)
	if err == nil {
		return bufDesc, bufDescStatusAllocated, nil
	}
	if !errors.Is(err, errFreeListFull) {
		return nil, bufDescStatusInvalid, err
	}

	bufDesc, err = freeList.evictBuffer(workerPool, btm, access)
	if err == nil {
		return bufDesc, bufDescStatusVictim, nil
	}
	return nil, bufDescStatusInvalid, err
}

// isReusableVictimLocked verifies that a selected descriptor did not become
// active, hot, dirty, or detached while eviction waited for I/O. btm.mu must be
// held so the exact mapping remains stable through the subsequent replacement.
func (btm *bufferTableMgr) isReusableVictimLocked(bufDesc *bufferDescriptor) bool {
	current, mapped := btm.table[bufDesc.block]
	return mapped && current == bufDesc &&
		bufDesc.refCnt.Load() == refCountTableAndOneUser &&
		bufDesc.usageCount.Load() == 0 &&
		!bufDesc.dirty.Load()
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
	return btm.lookupBufferDescriptorForAccess(blk, fl, accessMaintenance)
}

func (btm *bufferTableMgr) lookupBufferDescriptorForAccess(blk *block, fl *freeListType, access bufferAccessKind) (*bufferDescriptor, error) {
	btm.mu.RLock()
	bufDesc, exists := btm.table[blk]
	if exists {
		bufDesc.refCnt.Add(1)
		bufDesc.recordAccess(access)

		// Release the read lock on buffer table manager.
		btm.mu.RUnlock()

		if err := bufDesc.ensureBufferValidForRead(); err != nil {
			btm.detachBufferDescriptor(bufDesc, fl)
			bufDesc.release(fl)
			return nil, fmt.Errorf("block %d download: %w", blk.idx, err)
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
