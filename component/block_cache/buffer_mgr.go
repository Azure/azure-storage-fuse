package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

var btm *BufferTableMgr

// Buffer table to translate block to buffer index.
type BufferTableMgr struct {
	table map[*block]*bufferDescriptor
	mu    sync.RWMutex
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
	case bufDescStatusInvalid:
		return "bufDescStatusInvalid"
	default:
		return "Unknown"
	}
}

func GetOrCreateBufferDescriptor(blk *block, bytesInterested int32, download bool, sync bool) (*bufferDescriptor, bufDescStatus, error) {
	stime := time.Now()

	// First look up if the buffer descriptor already exists.
	bufDesc, err := btm.LookUpBufferDescriptor(blk, bytesInterested)
	if bufDesc != nil {
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Found existing bufferIdx: %d, blockIdx: %d, took: %v, refCnt: %d, usageCount: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, time.Since(stime), bufDesc.refCnt.Load(), bufDesc.usageCount.Load(), sync)
		return bufDesc, bufDescStatusExists, nil
	}
	if err != nil {
		return nil, bufDescStatusInvalid, err
	}

	// At this point, we know that buffer descriptor does not exist. Need to create a new buffer descriptor.
	// There is a chance that multiple threads are trying to create buffer descriptor for the same block.
	// Hence take an exclusive lock on the block to ensure only one goroutine creates the buffer descriptor.
	blk.mu.Lock()
	defer blk.mu.Unlock()

	// Acquire the lock on buffer table manager to create a new buffer descriptor.
	btm.mu.Lock()

	// Double check if another goroutine created the buffer descriptor.
	bufDesc, exists := btm.table[blk]
	if exists {
		bufDesc.refCnt.Add(1)
		bufDesc.usageCount.Add(bytesInterested)
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: (Double Check) Found existing bufferIdx: %d, blockIdx: %d, refCnt: %d, usageCount: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load(), sync)

		// Release the lock on buffer table manager.
		btm.mu.Unlock()

		if err := bufDesc.ensureBufferValidForRead(); err == nil {
			return bufDesc, bufDescStatusExists, nil
		} else {
			log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Existing bufferIdx: %d, blockIdx: %d, sync: %v, has download error",
				bufDesc.bufIdx, blk.idx, sync)

			if ok := bufDesc.release(); ok {
				log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Released bufferIdx: %d for blockIdx: %d back to free list after download error: %v",
					bufDesc.bufIdx, blk.idx, err)
			}
			return nil, bufDescStatusInvalid, err
		}
	}

	victim := false
	// Get the Buffer Descriptor from free list.
	bufDesc, err = freeList.allocateBuffer(blk)
	if err != nil {
		// Failed to allocate buffer from free list, as free list is full. Need to evict a buffer.
		log.Err("BufferTableMgr::GetOrCreateBufferDescriptor: Failed to allocate buffer for blockIdx: %d, sync: %v: %v",
			blk.idx, sync, err)
		victim = true
		retries := 1

	retry:
		// While getting the victim buffer, there is no point in holding on to the buffer table manager lock.
		btm.mu.Unlock()

		// No free buffer present in freeList, need to evict a buffer.Request a victim buffer from Buffers in use list.
		bufDesc = freeList.getVictimBuffer()

		// Re-acquire the lock on buffer table manager to update the table.
		btm.mu.Lock()

		victimRefCnt := bufDesc.refCnt.Load()
		if victimRefCnt > 1 {
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
		// Remove the victim buffer's block mapping from buffer table.
		delete(btm.table, bufDesc.block)
		bufDesc.reset()
	}

	// Add the new buffer descriptor to buffer table.
	btm.table[blk] = bufDesc

	// Initialize the buffer descriptor.
	bufDesc.refCnt.Store(1)
	bufDesc.usageCount.Store(bytesInterested)
	bufDesc.block = blk

	// Take the content lock on buffer descriptor before releasing the buffer table manager lock, so that no one else
	// can use it first, other than us.
	bufDesc.contentLock.Lock()
	// Release the lock on buffer table manager.
	btm.mu.Unlock()

	// This is where we should downlod the blockdata into the buffer.
	if download {
		wait := make(chan struct{}, 1)
		wp.queueWork(blk, bufDesc, true, wait, sync)
		if sync {
			// Wait for download to complete.
			<-wait

			if bufDesc.downloadErr != nil && bufDesc.valid.Load() == false {
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

	if !sync {
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Async scheduling download for bufferIdx: %d, blockIdx: %d took %v",
			bufDesc.bufIdx, blk.idx, time.Since(stime))
	}

	if victim {
		return bufDesc, bufDescStatusVictim, nil
	}
	return bufDesc, bufDescStatusAllocated, nil
}

// LookUpBufferDescriptor: looks up the buffer descriptor for the given block. and increments the ref count and
// usage count if found. It is necessary that btm must always locked while incrementing the ref count and usage count of
// the buffer descriptor.
func (btm *BufferTableMgr) LookUpBufferDescriptor(blk *block, bytesInterested int32) (*bufferDescriptor, error) {
	btm.mu.RLock()
	bufDesc, exists := btm.table[blk]
	if exists {
		bufDesc.refCnt.Add(1)
		evict := false
		if bufDesc.usageCount.Add(bytesInterested) == int32(bc.blockSize) {
			// This buffer descriptor has reached its maximum usage count, mark it for eviction in future.
			evict = true
		}
		log.Debug("BufferTableMgr::LookUpBufferDescriptor: Looked up bufferIdx: %d, blockIdx: %d, refCnt: %d, usageCount: %d",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load())

		// Release the read lock on buffer table manager.
		btm.mu.RUnlock()

		if evict {
			btm.removeBufferDescriptor(bufDesc)
			log.Debug("BufferTableMgr::LookUpBufferDescriptor: BufferIdx: %d, blockIdx: %d marked for eviction as usageCount reached max",
				bufDesc.bufIdx, blk.idx)
		}

		if err := bufDesc.ensureBufferValidForRead(); err != nil {
			return nil, err
		}

		return bufDesc, nil
	}

	btm.mu.RUnlock()
	return nil, nil
}

// removeBufferDescriptor: removes the buffer descriptor from buffer table manager, and releases the buffer back to free
// list if no one is using it.
// Returns true if the buffer descriptor was released to the freelist, false otherwise.
func (btm *BufferTableMgr) removeBufferDescriptor(bufDesc *bufferDescriptor) bool {
	// We should only remove the buffer descriptor once.
	if ok := bufDesc.evicted.CompareAndSwap(false, true); !ok {
		return false
	}

	log.Debug("BufferTableMgr::removeBufferDescriptor: Removed blockIdx: %d from buffer table", bufDesc.block.idx)

	btm.mu.Lock()
	delete(btm.table, bufDesc.block)
	btm.mu.Unlock()

	// Reduce the ref count for the buffer descriptor itself. This tells the other users that once refCnt reaches -1,
	// this buffer descriptor can be released back to free list.
	if bufDesc.refCnt.Add(-1) == -1 {
		// Release the buffer back to free list.
		log.Debug("BufferTableMgr::removeBufferDescriptor: Released bufferIdx: %d, blockIdx: %d back to free list",
			bufDesc.bufIdx, bufDesc.block.idx)
		freeList.releaseBuffer(bufDesc)
		return true
	}

	return false
}
