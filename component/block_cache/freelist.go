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
	"math"
	"sync"
)

// errFreeListFull indicates that all buffers are currently in use.
// When this error is returned, buffer eviction is required to proceed.
var errFreeListFull = errors.New("all buffers are in use, free list is full")
var errNoVictimBufferFound = errors.New("all buffer descriptors are currently pinned")
var errFreeListClosed = errors.New("buffer pool is shutting down")

const maxSweepPasses = maxBufferUsageCount + 1

// freeListType manages the pool of available buffers and implements eviction.
//
// Overview:
//
// The free list is the core buffer allocation mechanism for BlockCache. It:
//
//   - Maintains a list of available (free) buffers
//   - Allocates buffers on demand for new blocks
//   - Implements bounded clock-sweep eviction when no free buffers exist
//   - Manages buffer descriptor lifecycle
//
// Data Structures:
//
//   - Free list: Singly-linked list of available buffers (via nxtFreeBuffer)
//   - Buffer descriptors: Array of all buffer descriptors (fixed size)
//   - Victim pointer: Index for round-robin eviction candidate selection
//
// Allocation Strategy:
//
//  1. If free list not empty: allocate from free list (O(1))
//  2. If free list empty: find victim buffer to evict (O(n))
//  3. If victim found: reuse victim's buffer (O(1))
//  4. If no victim: foreground allocation waits for a descriptor state change
//
// Eviction Policy:
//
// When the free list is empty, we must evict a buffer to make room.
// Eviction uses a modified clock algorithm:
//
//  1. Round-robin scan through all buffers
//  2. Skip buffers with refCnt > 1 (actively in use)
//  3. For refCnt == 1 (only in table), decrement usageCount when nonzero
//  4. Evict the first table-only buffer already at usageCount zero
//
// This policy balances:
//   - Protecting repeatedly accessed blocks with a bounded usage count
//   - Avoiding eviction of actively used blocks (refCnt check)
//   - Giving prefetched blocks no protection until they receive a demand access
//
// Thread Safety:
//
// The free list mutex protects all free list operations. Buffer allocation
// and release are serialized, but this is acceptable because:
//   - Operations are fast (linked list manipulation)
//   - Contention is low (many buffers, few allocation requests)
//   - Alternative (lock-free) would be much more complex
//
// Buffer Lifecycle:
//
//  1. Created during Start(): all buffers start in free list
//  2. Allocated: removed from free list, given to caller
//  3. In use: held by buffer table manager and/or operations
//  4. Released: metadata reset and returned directly to free list
//  5. Evicted: reused for different block
//  6. Destroyed during Stop(): all buffers deallocated
type freeListType struct {
	bufSize         int64               // Size of each buffer in bytes (should match block size)
	zeroBuf         []byte              // Shared immutable zero block for sparse writes
	firstFreeBuffer int                 // Index of first buffer in free list (-1 if empty)
	lastFreeBuffer  int                 // Index of last buffer in free list (-1 if empty)
	nxtVictimBuffer int                 // Next index to consider for eviction (round-robin)
	bufDescriptors  []*bufferDescriptor // Array of all buffer descriptors
	mutex           sync.Mutex          // Protects free list state
	changed         chan struct{}       // Closed and replaced whenever descriptor availability changes
	closed          bool                // Stops allocation waits during shutdown
}

// createFreeList initializes the free list and buffer pool.
//
// This function is called during BlockCache.Start() to set up buffer management.
// It performs:
//
//  1. Calculates number of buffers based on config or system RAM
//  2. Allocates buffer descriptors for all buffers
//  3. Initializes free list linking all buffers
//
// Parameters:
//   - bufSize: Size of each buffer in bytes (typically equals block size)
//   - memSize: Total memory for buffer pool (0 = auto-calculate from RAM)
//
// Returns an error if buffer pool initialization fails.
//
// Memory Calculation:
//
// Configure resolves memSize before creating the pool. By default it uses 50%
// of available memory and rounds down to a whole number of buffers.
//
// Why maxBuffers can be large:
//
// The number of buffers is calculated as memSize / bufSize.
// With large block sizes (e.g., 16 MB), this may result in
// relatively few buffers (e.g., 1 GB / 16 MB = 64 buffers).
func createFreeList(bufSize uint64, memSize uint64) (*freeListType, error) {
	if bufSize == 0 || bufSize > uint64(math.MaxInt) {
		return nil, fmt.Errorf("invalid buffer size: %d", bufSize)
	}
	//
	// Number of fixed-size buffers managed by the free list.
	// This should be equal to the block size configured by the user.
	bufferCount := memSize / bufSize
	if bufferCount == 0 {
		return nil, fmt.Errorf("Buffer Pool: Memory size %d bytes is too small for buffer size %d bytes, resulting in 0 buffers",
			memSize, bufSize)
	}
	if bufferCount > uint64(math.MaxInt) {
		return nil, fmt.Errorf("buffer count exceeds platform limit: %d", bufferCount)
	}
	maxBuffers := int(bufferCount)

	freeList := &freeListType{
		firstFreeBuffer: 0,
		lastFreeBuffer:  maxBuffers - 1,
		nxtVictimBuffer: 0,
		bufDescriptors:  make([]*bufferDescriptor, maxBuffers),
		zeroBuf:         make([]byte, int(bufSize)),
		changed:         make(chan struct{}),
	}
	for i := range maxBuffers {
		freeList.bufDescriptors[i] = &bufferDescriptor{
			bufIdx:        i,
			nxtFreeBuffer: i + 1,
			buf:           make([]byte, int(bufSize)),
		}
	}

	// Last buffer's next free buffer should be -1.
	freeList.bufDescriptors[maxBuffers-1].nxtFreeBuffer = -1

	freeList.bufSize = int64(bufSize)

	return freeList, nil
}

// destroyFreeList cleans up the free list and releases all resources.
//
// This function is called during BlockCache.Stop(). It:
//  1. Releases all buffers
//  2. Clears all data structures
//
// After destroy completes, the free list cannot be used without recreating it.
func (fl *freeListType) destroy() {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()
	if !fl.closed {
		fl.closed = true
		close(fl.changed)
	}

	for i := range len(fl.bufDescriptors) {
		fl.bufDescriptors[i].buf = nil
	}

	fl.bufDescriptors = nil
	fl.zeroBuf = nil
}

// allocateBuffer allocates a buffer from the free list.
//
// This method attempts to allocate a buffer for the given block:
//  1. Checks if free list has available buffers
//  2. If yes: removes buffer from free list and returns it
//  3. If no: returns errFreeListFull to trigger eviction
//
// Parameters:
//   - blk: Block that will use this buffer
//
// Returns:
//   - *bufferDescriptor: Allocated buffer (with block set)
//   - error: errFreeListFull if no free buffers available
//
// Thread Safety:
//
// This method holds the free list mutex during allocation to ensure
// consistent free list state.
//
// Why link block here:
//
// We set bufDesc.block = blk to establish the association immediately.
// This simplifies error handling and ensures the buffer knows which
// block it belongs to from the start.
func (fl *freeListType) allocateBuffer(blk *block) (*bufferDescriptor, error) {
	fl.mutex.Lock()

	if fl.closed {
		fl.mutex.Unlock()
		return nil, errFreeListClosed
	}

	if fl.firstFreeBuffer == -1 {
		// No free buffer, need to evict a buffer.
		fl.mutex.Unlock()
		return nil, errFreeListFull
	}

	// Allocate from free list.
	bufDesc := fl.bufDescriptors[fl.firstFreeBuffer]
	fl.firstFreeBuffer = bufDesc.nxtFreeBuffer
	if fl.firstFreeBuffer == -1 {
		fl.lastFreeBuffer = -1
	}
	fl.mutex.Unlock()

	// Clearing on allocation keeps released descriptors immediately available
	// without exposing data from their previous block to sparse/local writes.
	clear(bufDesc.buf)
	bufDesc.nxtFreeBuffer = -1
	bufDesc.block = blk

	return bufDesc, nil
}

// releaseBuffer resets descriptor metadata and returns it directly to the free list.
//
// Parameters:
//   - bufDesc: Buffer descriptor to release
func (fl *freeListType) releaseBuffer(bufDesc *bufferDescriptor) {
	bufDesc.resetMetadata()

	fl.mutex.Lock()
	if fl.lastFreeBuffer == -1 {
		fl.firstFreeBuffer = bufDesc.bufIdx
		fl.lastFreeBuffer = bufDesc.bufIdx
	} else {
		bufDesc.nxtFreeBuffer = fl.firstFreeBuffer
		fl.firstFreeBuffer = bufDesc.bufIdx
	}
	fl.notifyAvailabilityLocked()
	fl.mutex.Unlock()
}

func (fl *freeListType) notifyAvailability() {
	fl.mutex.Lock()
	fl.notifyAvailabilityLocked()
	fl.mutex.Unlock()
}

func (fl *freeListType) notifyAvailabilityLocked() {
	if fl.closed {
		return
	}
	close(fl.changed)
	fl.changed = make(chan struct{})
}

func (fl *freeListType) watchAvailability() (<-chan struct{}, error) {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()
	if fl.closed {
		return nil, errFreeListClosed
	}
	return fl.changed, nil
}

// evictBuffer finds, detaches, and resets a buffer suitable for reuse.
//
// This method implements the buffer eviction policy. It scans through
// buffer descriptors using a round-robin approach to find a buffer
// that can be evicted:
//
// Eviction Criteria:
//  1. Buffer must not be actively in use (refCnt == 1, only in table)
//  2. Buffer's usageCount must be zero
//
// Each sweep ages table-only descriptors by one. Demand reads and writes
// increase usageCount up to maxBufferUsageCount; prefetch and maintenance
// accesses do not increase it.
//
// Returns an unmapped, reset descriptor exclusively owned by the allocator.
//
// Round-Robin Scanning:
//
// The nxtVictimBuffer index cycles through all buffers, providing
// recency/frequency approximation without maintaining timestamps or a list.
//
// Dirty Buffer Handling:
//
// If the selected victim buffer is dirty (modified but not uploaded),
// it's uploaded synchronously before being evicted. This ensures no
// data loss but may block the allocation request.
//
// Foreground allocation may make multiple passes and upload dirty victims.
// Prefetch makes one pass, evicts only clean buffers already at usageCount zero,
// and returns errNoVictimBufferFound instead of waiting or performing writeback.
//
// Thread Safety:
//
// This method can be called concurrently by multiple allocators.
// Candidate pinning and final detachment are validated under btm.mu. The
// returned descriptor is no longer visible through either table or free list.
func (fl *freeListType) evictBuffer(workerPool *workerPool, btm *bufferTableMgr, access bufferAccessKind) (*bufferDescriptor, error) {
	maxBuffers := len(fl.bufDescriptors)
	maxTries := maxBuffers * int(maxSweepPasses)
	if access == accessPrefetch {
		maxTries = maxBuffers
	}
	numTries := 0

	for {
		if numTries >= maxTries {
			// A bounded scan fully ages any unpinned descriptor. Reaching this
			// limit means every descriptor remained pinned or changed concurrently.
			break
		}

		fl.mutex.Lock()
		bufDesc := fl.bufDescriptors[fl.nxtVictimBuffer]
		fl.nxtVictimBuffer = (fl.nxtVictimBuffer + 1) % maxBuffers
		fl.mutex.Unlock()

		numTries++

		// See if the buffer descriptor is present in only buffer table manager(refCnt=1) and has no users for it.
		if bufDesc.refCnt.Load() == refCountTableOnly {
			if !bufDesc.ageUsage() {
				// Found a victim buffer. pin the buffer by increasing refCnt.
				pinnedBuffer := false

				btm.mu.Lock()
				// Check for the refCnt again after acquiring the lock to make sure the buffer is still a valid victim before pinning it.
				current, mapped := btm.table[bufDesc.block]
				if bufDesc.refCnt.Load() == refCountTableOnly && bufDesc.usageCount.Load() == 0 && mapped && current == bufDesc &&
					(access != accessPrefetch || !bufDesc.dirty.Load()) {
					bufDesc.refCnt.Add(1)
					pinnedBuffer = true
				}
				btm.mu.Unlock()

				if pinnedBuffer {
					// If the block is dirty, we should need to upload it before reusing it.
					if bufDesc.dirty.Load() {
						if access == accessPrefetch {
							bufDesc.release(fl)
							continue
						}
						if err := bufDesc.block.scheduleUpload(workerPool, bufDesc); err != nil {
							bufDesc.release(fl)
							return nil, err
						}
					}

					btm.mu.Lock()
					if btm.isReusableVictimLocked(bufDesc) {
						delete(btm.table, bufDesc.block)
						bufDesc.reset()
						btm.mu.Unlock()
						return bufDesc, nil
					}
					btm.mu.Unlock()
					bufDesc.release(fl)
				}
			}
		}
	}

	return nil, errNoVictimBufferFound
}
