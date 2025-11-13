package block_cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

var errFreeListFull = errors.New("All buffers are in use, Free list is full!")

var freeList *freeListType

type freeListType struct {
	bufPool         *BufferPool
	firstFreeBuffer int
	lastFreeBuffer  int
	nxtVictimBuffer int
	bufDescriptors  []*bufferDescriptor
	mutex           sync.Mutex
}

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
			return fmt.Errorf("NewFileIOManager: %v", err)
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

	log.Info("Buffer Pool: Free list created with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufSize, maxBuffers, float64(uint64(maxBuffers)*bufSize)/(1024.0*1024.0))

	return nil
}

func destroyFreeList() {
	if freeList == nil {
		return
	}

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

func (fl *freeListType) releaseBuffer(bufDesc *bufferDescriptor) {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()

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
}

func (fl *freeListType) debugListMustBeFull() {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()

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

}

func (fl *freeListType) getVictimBuffer() *bufferDescriptor {
	log.Debug("getVictimBuffer: Starting to look for victim buffer")

	numBuffers := len(fl.bufDescriptors)
	numTries := 0

	// This loop should always find a victim buffer, as at any time the assumption is there can only be 10 FUSE threads
	// working on 10 different bufferes in the worst case.
	for {

		log.Debug("getVictimBuffer: Trying to find victim buffer, try number: %d", numTries+1)
		fl.mutex.Lock()

		bufDesc := fl.bufDescriptors[fl.nxtVictimBuffer]
		fl.nxtVictimBuffer = (fl.nxtVictimBuffer + 1) % numBuffers

		fl.mutex.Unlock()

		numTries++

		if bufDesc.refCnt.Load() == 0 {
			if bufDesc.usageCount.Load() == int32(bc.blockSize) || bufDesc.numEvictionCyclesPassed.Load() > 0 {
				// Found a victim buffer. pin the buffer by increasing refCnt.
				log.Debug("getVictimBuffer: Selected victim bufferIdx: %d, blockIdx: %d after %d tries",
					bufDesc.bufIdx, bufDesc.block.idx, numTries)

				bufDesc.refCnt.Add(1)
				return bufDesc
			} else {
				// Give one more chance to this buffer to be used.
				bufDesc.numEvictionCyclesPassed.Add(1)
			}
		}

		log.Debug("getVictimBuffer: bufferIdx: %d for blockIdx: %d is in use, refCnt: %d, usageCount: %d",
			bufDesc.bufIdx, bufDesc.block.idx, bufDesc.refCnt, bufDesc.usageCount)
	}

	return nil
}
