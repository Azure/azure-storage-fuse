package block_cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// errFreeListFull indicates that all buffers are currently in use.
// When this error is returned, buffer eviction is required to proceed.
var errFreeListFull = errors.New("All buffers are in use, Free list is full!")

// freeList is the global free list instance, initialized during Start().
var freeList *freeListType

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
//  4. Released: returned to free list (via async goroutine)
//  5. Evicted: reused for different block
//  6. Destroyed during Stop(): all buffers deallocated
type freeListType struct {
	bufPool         *BufferPool            // Buffer pool for actual memory allocation
	firstFreeBuffer int                    // Index of first buffer in free list (-1 if empty)
	lastFreeBuffer  int                    // Index of last buffer in free list (-1 if empty)
	nxtVictimBuffer int                    // Next index to consider for eviction (round-robin)
	bufDescriptors  []*bufferDescriptor    // Array of all buffer descriptors
	resetBufferDesc chan *bufferDescriptor // Channel for async buffer reset
	wg              sync.WaitGroup         // Tracks reset goroutine
	mutex           sync.Mutex             // Protects free list state
}

// createFreeList initializes the free list and buffer pool.
//
// This function is called during BlockCache.Start() to set up buffer management.
// It performs:
//
//  1. Calculates number of buffers based on config or system RAM
//  2. Allocates buffer descriptors for all buffers
//  3. Initializes free list linking all buffers
//  4. Starts async reset goroutine
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
//
// Buffer Reset Goroutine:
//
// Buffer reset (zero-fill and metadata clear) is expensive and done
// asynchronously to avoid blocking allocation. Released buffers are
// queued for reset in a background goroutine.
func createFreeList(bufSize uint64, memSize uint64) error {
	//
	// Size of buffers managed by bufferPool.
	// This should be equal to the block size configured by the user.
	maxBuffers := int(memSize / bufSize)

	if maxBuffers == 0 {
		//
		// How much percennt of the system RAM (available memory to be precise) are we allowed to use?
		//
		// TODO: This can be config value.
		//
		usablePercentSystemRAM := 50

		//
		// Allow higher number of maxBuffers if system can afford.
		//
		ramMB, err := common.GetAvailableMemoryInMB()
		if err != nil {
			return fmt.Errorf("createFreeList: %v", err)
		}

		// usableMemory in bytes capped by usablePercentSystemRAM.
		usableMemory := (ramMB * 1024 * 1024 * uint64(usablePercentSystemRAM)) / 100
		maxBuffers = max(maxBuffers, int(usableMemory/bufSize))
	}

	freeList = &freeListType{
		firstFreeBuffer: 0,
		lastFreeBuffer:  maxBuffers - 1,
		nxtVictimBuffer: 0,
		bufDescriptors:  make([]*bufferDescriptor, maxBuffers),
		resetBufferDesc: make(chan *bufferDescriptor, maxBuffers/2),
	}

	freeList.bufPool = initBufferPool(bufSize, uint64(maxBuffers))

	for i := 0; i < maxBuffers; i++ {
		buf, err := freeList.bufPool.GetBuffer()
		if err != nil {
			log.Err("BufferPool::newFreeList: Failed to get buffer from pool: %v", err)
			// Release already allocated buffers.
			for j := 0; j < i; j++ {
				freeList.bufPool.PutBuffer(freeList.bufDescriptors[j].buf)
			}
			return err
		}
		freeList.bufDescriptors[i] = &bufferDescriptor{
			bufIdx:        i,
			nxtFreeBuffer: i + 1,
			buf:           buf,
		}
	}

	// Last buffer's next free buffer should be -1.
	freeList.bufDescriptors[maxBuffers-1].nxtFreeBuffer = -1

	// This is long running goroutine to reset the buffer descriptors released back to free list.
	freeList.wg.Add(1)
	go freeList.resetBufferDescriptors()

	log.Info("Buffer Pool: Free list created with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufSize, maxBuffers, float64(uint64(maxBuffers)*bufSize)/(1024.0*1024.0))

	return nil
}

// destroyFreeList cleans up the free list and releases all resources.
//
// This function is called during BlockCache.Stop(). It:
//  1. Closes the reset channel to signal the reset goroutine to exit
//  2. Waits for the reset goroutine to finish
//  3. Returns all buffers to the buffer pool
//  4. Clears all data structures
//
// After destroy completes, the free list cannot be used without recreating it.
func destroyFreeList() {
	if freeList == nil {
		return
	}

	close(freeList.resetBufferDesc)
	freeList.wg.Wait()

	freeList.mutex.Lock()
	defer freeList.mutex.Unlock()

	for i := 0; i < len(freeList.bufDescriptors); i++ {
		freeList.bufPool.PutBuffer(freeList.bufDescriptors[i].buf)
		freeList.bufDescriptors[i].buf = nil
	}

	freeList.bufDescriptors = nil
	freeList.bufPool = nil
	freeList = nil

	log.Info("Buffer Pool: Free list destroyed")
}

// resetBufferDescriptors is a background goroutine that resets released buffers.
//
// This function runs continuously, consuming buffer descriptors from the
// resetBufferDesc channel and resetting them (clearing data and metadata).
//
// Why async reset:
//
// Resetting a buffer involves:
//   - Zeroing the entire buffer (e.g., 16 MB memcpy)
//   - Clearing metadata fields
//
// This is expensive (~1-2 ms per buffer). Doing it synchronously would
// block the release operation and slow down the critical path. By doing
// it asynchronously, we can:
//   - Release buffers immediately
//   - Perform reset in background
//   - Batch multiple resets
//
// The goroutine exits when the resetBufferDesc channel is closed during
// destroyFreeList().
func (fl *freeListType) resetBufferDescriptors() {
	defer fl.wg.Done()
	for {
		bufDesc, ok := <-fl.resetBufferDesc
		if !ok {
			// Channel closed, exit the goroutine.
			return
		}

		fl.mutex.Lock()

		log.Debug("releaseBuffer: Released bufferIdx: %d for blockIdx: %d", bufDesc.bufIdx, bufDesc.block.idx)

		// Reset the buffer descriptor.
		bufDesc.reset()

		if fl.lastFreeBuffer == -1 {
			// Free list is empty.
			fl.firstFreeBuffer = bufDesc.bufIdx
			fl.lastFreeBuffer = bufDesc.bufIdx
		} else {
			// Append to the end of free list.
			fl.bufDescriptors[fl.lastFreeBuffer].nxtFreeBuffer = bufDesc.bufIdx
			fl.lastFreeBuffer = bufDesc.bufIdx
		}
		fl.mutex.Unlock()
	}
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
	defer fl.mutex.Unlock()

	if fl.firstFreeBuffer == -1 {
		// No free buffer, need to evict a buffer.
		return nil, errFreeListFull
	}

	// Allocate from free list.
	bufDesc := fl.bufDescriptors[fl.firstFreeBuffer]
	fl.firstFreeBuffer = bufDesc.nxtFreeBuffer
	if fl.firstFreeBuffer == -1 {
		fl.lastFreeBuffer = -1
	}

	bufDesc.nxtFreeBuffer = -1
	bufDesc.block = blk

	log.Debug("allocateBuffer: Allocated bufferIdx: %d for blockIdx: %d", bufDesc.bufIdx, blk.idx)

	return bufDesc, nil
}

// releaseBuffer queues a buffer for reset and return to the free list.
//
// This method is called when a buffer's refCnt reaches 0 (no more users).
// The buffer is queued for async reset via the resetBufferDesc channel.
//
// Parameters:
//   - bufDesc: Buffer descriptor to release
//
// The actual reset and free list insertion happens in the background
// goroutine (resetBufferDescriptors). This avoids blocking the release
// operation on expensive buffer clearing.
//
// Why async:
//
// Buffer reset involves zeroing potentially large buffers (e.g., 16 MB).
// Doing this synchronously would add latency to every buffer release,
// which happens on the critical path of file operations.
func (fl *freeListType) releaseBuffer(bufDesc *bufferDescriptor) {
	fl.resetBufferDesc <- bufDesc
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

	log.Debug("debugListMustBeFull: Checking if free list is full")

	count := 0
	next := fl.firstFreeBuffer
	for next != -1 {
		count++
		next = fl.bufDescriptors[next].nxtFreeBuffer
	}

	if count != len(fl.bufDescriptors) {
		err := fmt.Sprintf("freeList::debugListMustBeFull: Free list is not full, count: %d, expected: %d",
			count, len(fl.bufDescriptors))
		log.Err(err)
		panic(err)
	}

	log.Debug("debugListMustBeFull:  free list is indeed full!")

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
func (fl *freeListType) getVictimBuffer() *bufferDescriptor {
	log.Debug("getVictimBuffer: Starting to look for victim buffer")

	numBuffers := len(fl.bufDescriptors)
	numTries := 0

	// This loop should always find a victim buffer, as at any time the assumption is there can only be 10 FUSE threads
	// working on 10 different buffers in the worst case.
	for {
		log.Debug("getVictimBuffer: Trying to find victim buffer, try number: %d", numTries+1)

		fl.mutex.Lock()
		bufDesc := fl.bufDescriptors[fl.nxtVictimBuffer]
		fl.nxtVictimBuffer = (fl.nxtVictimBuffer + 1) % numBuffers
		fl.mutex.Unlock()

		numTries++

		// See if the buffer descriptor is present in only buffer table manager(refCnt=1) and has no users for it.
		if bufDesc.refCnt.Load() == 1 {
			if bufDesc.bytesRead.Load() == int32(bc.blockSize) || bufDesc.numEvictionCyclesPassed.Load() > 0 {
				// Found a victim buffer. pin the buffer by increasing refCnt.
				log.Debug("getVictimBuffer: Selected victim bufferIdx: %d, blockIdx: %d after %d tries",
					bufDesc.bufIdx, bufDesc.block.idx, numTries)

				bufDesc.refCnt.Add(1)

				// If the block is dirty, we should need to upload it before reusing it.
				if bufDesc.dirty.Load() {
					log.Debug("getVictimBuffer: Victim bufferIdx: %d for blockIdx: %d is dirty, scheduling upload before reuse",
						bufDesc.bufIdx, bufDesc.block.idx)
					bufDesc.block.scheduleUpload(bufDesc, true /* sync */)
				}

				return bufDesc
			} else {
				// Give one more chance to this buffer to be used.
				bufDesc.numEvictionCyclesPassed.Add(1)
			}
		}

		log.Debug("getVictimBuffer: bufferIdx: %d is in use, refCnt: %d, bytesRead: %d, bytesWritten: %d",
			bufDesc.bufIdx, bufDesc.refCnt, bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load())
	}

	return nil
}
