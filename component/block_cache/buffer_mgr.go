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

func GetOrCreateBufferDescriptor(blk *block, doesRead bool, sync bool) (*bufferDescriptor, bufDescStatus, error) {
	stime := time.Now()

	log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Requesting buffer for blockIdx: %d, doesRead: %v, sync: %v, file: %s",
		blk.idx, doesRead, sync, blk.file.Name)

	// First look up if the buffer descriptor already exists.
	bufDesc, err := btm.LookUpBufferDescriptor(blk)
	if bufDesc != nil {
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: Found existing bufferIdx: %d, blockIdx: %d, took: %v, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, time.Since(stime), bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), sync)
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
		log.Debug("BufferTableMgr::GetOrCreateBufferDescriptor: (Double Check) Found existing bufferIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d, sync: %v",
			bufDesc.bufIdx, blk.idx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), sync)

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

	// Before Creating a new buffer descriptor, check if the file needs to be flushed.
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
		// Remove the victim buffer's block mapping from buffer table.
		delete(btm.table, bufDesc.block)
		bufDesc.reset()
	}

	// Add the new buffer descriptor to buffer table.
	btm.table[blk] = bufDesc

	// Initialize the buffer descriptor.
	bufDesc.refCnt.Store(1)
	bufDesc.block = blk

	// Take the content lock on buffer descriptor before releasing the buffer table manager lock, so that no one else
	// can use it first, other than us, when the buffer is being downloaded, This will get unlocked once download is complete.
	if doesRead {
		bufDesc.contentLock.Lock()
		// The unlock will happen after download is complete in another goroutine worker function.
	} else {
		// This buffer will be used for writing new data, so mark it as valid & dirty.
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

// LookUpBufferDescriptor: looks up the buffer descriptor for the given block. and increments the ref count and
// usage count if found. It is necessary that btm must always locked while incrementing the ref count and usage count of
// the buffer descriptor.
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

// removeBufferDescriptor: removes the buffer descriptor from buffer table manager, and releases the buffer back to free
// list if no one is using it.
// strict: if true, will not remove the buffer descriptor if it has any references.
// Returns true if the buffer descriptor was released to the freelist, false otherwise.
func (btm *BufferTableMgr) removeBufferDescriptor(bufDesc *bufferDescriptor, strict bool) (isRemovedFromBufMgr bool, isReleasedToFreeList bool) {
	log.Debug("BufferTableMgr::removeBufferDescriptor: Remove blockIdx: %d, bufferIdx: %d for file: %s from buffer table",
		bufDesc.block.idx, bufDesc.bufIdx, bufDesc.block.file.Name)

	btm.mu.Lock()
	if bufDesc.dirty.Load() {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove dirty bufferIdx: %d for blockIdx: %d, flush needed before reading block data",
			bufDesc.bufIdx, bufDesc.block.idx)
		return false, false
	}

	if strict && bufDesc.refCnt.Load() > 0 {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: Cannot remove bufferIdx: %d for blockIdx: %d, refCnt: %d > 1",
			bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt.Load())
		return false, false
	}

	// Check evict status of the buffer descriptor, we need to evict only once from the map.
	if _, ok := btm.table[bufDesc.block]; !ok {
		btm.mu.Unlock()
		log.Debug("BufferTableMgr::removeBufferDescriptor: BufferIdx: %d not found in buffer table, already removed",
			bufDesc.bufIdx)
		return true, false
	}

	// Remove the buffer descriptor from buffer table.
	delete(btm.table, bufDesc.block)
	btm.mu.Unlock()

	// Reduce the ref count for the buffer descriptor itself. This tells the other users that once refCnt reaches -1,
	// this buffer descriptor can be released back to free list safely.
	if bufDesc.refCnt.Add(-1) == -1 {
		// Release the buffer back to free list.
		log.Debug("BufferTableMgr::removeBufferDescriptor: Released bufferIdx: %d, blockIdx: %d back to free list",
			bufDesc.bufIdx, bufDesc.block.idx)
		freeList.releaseBuffer(bufDesc)
		return true, true
	}

	return true, false
}
