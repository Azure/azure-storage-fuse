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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// errFreeListFull indicates that all buffers are currently in use.
// When this error is returned, buffer eviction is required to proceed.
var errFreeListFull = errors.New("all buffers are in use, free list is full")
var errNoVictimBufferFound = errors.New("cannot find victim buffer, all buffers are busy, increase the memory limit configured")

const (
	minEvictionCyclesToPass = 1
	maxRoundsBeforeGivingUp = 5
)

// freeListType manages the pool of available buffers and implements eviction.
//
// Overview:
//
// The free list is the core buffer allocation mechanism for BlockCache. It:
//
//   - Maintains a list of available (free) buffers
//   - Allocates buffers on demand for new blocks
//   - Implements LRU-based eviction when no free buffers exist
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
//  4. If no victim: operation fails (all buffers pinned)
//
// Eviction Policy:
//
// When the free list is empty, we must evict a buffer to make room.
// Eviction uses a modified clock algorithm:
//
//  1. Round-robin scan through all buffers
//  2. Skip buffers with refCnt > 1 (actively in use)
//  3. For refCnt == 1 (only in table), check usage:
//     - bytesRead >= blockSize: fully read, good candidate
//     - numEvictionCyclesPassed > 0: seen before, good candidate
//     - Otherwise: give one more chance, increment cycle counter
//  4. Evict first suitable buffer found
//
// This policy balances:
//   - Keeping recently accessed blocks (LRU aspect)
//   - Avoiding eviction of actively used blocks (refCnt check)
//   - Not evicting blocks that were just allocated (cycle counter)
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
// If memSize is 0, uses 50% of available system RAM (configurable).
// This ensures BlockCache doesn't consume excessive memory while
// still providing good cache hit rates.
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

	log.Info("freeList::createFreeList: Free list created with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufSize, maxBuffers, float64(uint64(maxBuffers)*bufSize)/(1024.0*1024.0))

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

	for i := range len(fl.bufDescriptors) {
		fl.bufDescriptors[i].buf = nil
	}

	fl.bufDescriptors = nil
	fl.zeroBuf = nil

	log.Info("freeList::destroy: Free list destroyed")
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

	log.Debug("freeList::allocateBuffer: Allocated bufferIdx: %d for blockIdx: %d", bufDesc.bufIdx, blk.idx)

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
	fl.mutex.Unlock()

	log.Debug("freeList::releaseBuffer: Added bufferIdx: %d back to free list", bufDesc.bufIdx)
}

// debugListMustBeFull verifies that all buffers are in the free list.
//
// This is a debugging function used during testing to ensure no buffer
// leaks have occurred. It:
//  1. Walks the free list counting buffers
//  2. Verifies count equals total buffer descriptors
//  3. Panics if counts don't match (indicates leak)
//
// This should only be called when all handles are closed and all buffers
// are expected to be free.
//
// Why this is important:
//
// Buffer leaks are serious bugs that can:
//   - Exhaust the buffer pool over time
//   - Cause performance degradation
//   - Lead to operation failures
//
// This function helps catch such leaks during development and testing.
func (fl *freeListType) debugListMustBeFull() {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()

	log.Debug("freeList::debugListMustBeFull: Checking if free list is full")

	count := 0
	next := fl.firstFreeBuffer
	for next != -1 {
		count++
		next = fl.bufDescriptors[next].nxtFreeBuffer
	}

	if count != len(fl.bufDescriptors) {
		err := fmt.Sprintf("freeList::debugListMustBeFull: Free list is not full, count: %d, expected: %d",
			count, len(fl.bufDescriptors))
		log.Err("%s", err)
		panic(err)
	}

	log.Debug("freeList::debugListMustBeFull:  free list is indeed full!")

}

// getVictimBuffer finds and returns a buffer suitable for eviction.
//
// This method implements the buffer eviction policy. It scans through
// buffer descriptors using a round-robin approach to find a buffer
// that can be evicted:
//
// Eviction Criteria:
//  1. Buffer must not be actively in use (refCnt == 1, only in table)
//  2. Buffer must meet one of:
//     a. Fully read (bytesRead >= blockSize)
//     b. Has survived at least one eviction cycle (numEvictionCyclesPassed > 0)
//
// If a buffer doesn't meet criteria but has refCnt == 1, we give it one
// more chance by incrementing numEvictionCyclesPassed. This prevents
// immediate eviction of newly allocated buffers.
//
// Returns a buffer descriptor with refCnt incremented (pinned).
//
// Round-Robin Scanning:
//
// The nxtVictimBuffer index cycles through all buffers, providing
// approximate LRU behavior without maintaining a true LRU list.
// This is much simpler and faster than maintaining timestamps.
//
// Dirty Buffer Handling:
//
// If the selected victim buffer is dirty (modified but not uploaded),
// it's uploaded synchronously before being evicted. This ensures no
// data loss but may block the allocation request.
//
// Why this always succeeds:
//
// At any time, at most N threads can be actively using buffers (where
// N is the number of FUSE threads, typically ~10). With hundreds of
// buffers in the pool, this function will always find eviction candidates
// unless the pool is severely undersized.
//
// Thread Safety:
//
// This method can be called concurrently by multiple allocators.
// The mutex is released during victim search to avoid blocking other
// operations. The victim buffer is pinned (refCnt incremented) before
// returning to prevent eviction by other threads.
func (fl *freeListType) getVictimBuffer(workerPool *workerPool, btm *bufferTableMgr) (*bufferDescriptor, error) {
	log.Debug("freeList::getVictimBuffer: Starting to look for victim buffer")

	maxBuffers := len(fl.bufDescriptors)
	numTries := 0

	// This loop should always find a victim buffer, as at any time the assumption is there can only be 10 FUSE threads
	// working on 10 different buffers in the worst case.
	for {
		log.Debug("freeList::getVictimBuffer: Trying to find victim buffer, try number: %d", numTries+1)

		if numTries >= maxBuffers*maxRoundsBeforeGivingUp {
			// We've scanned through all buffers maxRounds times without finding a victim. This should never happen.
			break
		}

		fl.mutex.Lock()
		bufDesc := fl.bufDescriptors[fl.nxtVictimBuffer]
		fl.nxtVictimBuffer = (fl.nxtVictimBuffer + 1) % maxBuffers
		fl.mutex.Unlock()

		numTries++

		// See if the buffer descriptor is present in only buffer table manager(refCnt=1) and has no users for it.
		if bufDesc.refCnt.Load() == refCountTableOnly {
			if bufDesc.bytesRead.Load() == int32(fl.bufSize) || bufDesc.numEvictionCyclesPassed.Load() >= minEvictionCyclesToPass {
				// Found a victim buffer. pin the buffer by increasing refCnt.
				log.Debug("freeList::getVictimBuffer: Selected victim bufferIdx: %d, blockIdx: %d after %d tries",
					bufDesc.bufIdx, bufDesc.block.idx, numTries)

				pinnedBuffer := false

				btm.mu.Lock()
				// Check for the refCnt again after acquiring the lock to make sure the buffer is still a valid victim before pinning it.
				if bufDesc.refCnt.Load() != refCountTableOnly {
					log.Debug("freeList::getVictimBuffer: Victim bufferIdx: %d is no longer a valid victim after acquiring lock, refCnt: %d, giving it another chance",
						bufDesc.bufIdx, bufDesc.refCnt.Load())
					bufDesc.numEvictionCyclesPassed.Store(0) // Reset eviction cycle counter to give it another chance.
				} else {
					log.Debug("freeList::getVictimBuffer: Victim bufferIdx: %d for blockIdx: %d of file: %s is still a valid victim after acquiring lock, pinning it for eviction",
						bufDesc.bufIdx, bufDesc.block.idx, bufDesc.block.file.Name)
					bufDesc.refCnt.Add(1)
					pinnedBuffer = true
				}
				btm.mu.Unlock()

				if pinnedBuffer {
					// If the block is dirty, we should need to upload it before reusing it.
					if bufDesc.dirty.Load() {
						log.Debug("freeList::getVictimBuffer: Victim bufferIdx: %d for blockIdx: %d is dirty, scheduling upload before reuse",
							bufDesc.bufIdx, bufDesc.block.idx)
						if err := bufDesc.block.scheduleUpload(workerPool, fl, bufDesc); err != nil {
							bufDesc.release(fl)
							return nil, err
						}
					}

					return bufDesc, nil
				}
			} else {
				// Give one more chance to this buffer to be used.
				bufDesc.numEvictionCyclesPassed.Add(1)
			}
		}

		log.Debug("freeList::getVictimBuffer: bufferIdx: %d is in use, refCnt: %d, bytesRead: %d, bytesWritten: %d",
			bufDesc.bufIdx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load())
	}

	log.Err("freeList::getVictimBuffer: Scanned through all buffers %d times without finding a victim. This should never happen. numTries: %d, numBuffers: %d",
		maxRoundsBeforeGivingUp, numTries, maxBuffers)
	// print all the buffer descriptors for debugging.
	log.Err("freeList::getVictimBuffer: Printing all buffer descriptors for debugging:")
	for i := range fl.bufDescriptors {
		bufDesc := fl.bufDescriptors[i]
		blockIdx := -1
		fileName := ""
		if bufDesc.block != nil {
			blockIdx = bufDesc.block.idx
			fileName = bufDesc.block.file.Name
		}
		log.Err("BufferIdx: %d, BlockIdx: %d, RefCnt: %d, BytesRead: %d, BytesWritten: %d, Dirty: %t, EvictionCycles: %d, file: %s",
			bufDesc.bufIdx, blockIdx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(),
			bufDesc.bytesWritten.Load(), bufDesc.dirty.Load(), bufDesc.numEvictionCyclesPassed.Load(), fileName)
	}

	return nil, errNoVictimBufferFound
}
