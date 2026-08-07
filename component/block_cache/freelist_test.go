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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func isBufDescExistsInFreeList(fl *freeListType, bufIdx int) bool {
	fl.mutex.Lock()
	defer fl.mutex.Unlock()
	for i := fl.firstFreeBuffer; i != -1; i = fl.bufDescriptors[i].nxtFreeBuffer {
		if i == bufIdx {
			return true
		}
	}
	return false
}

func TestCreateFreeList(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}

	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	assert.NotNil(t, freeList)
	assert.Len(t, freeList.zeroBuf, int(bc.blockSize))
	assert.NotNil(t, freeList.bufDescriptors)
	assert.Len(t, freeList.bufDescriptors, 10)
	assert.Equal(t, 0, freeList.firstFreeBuffer)
	assert.Equal(t, 9, freeList.lastFreeBuffer)
	assert.Equal(t, 0, freeList.nxtVictimBuffer)

	// Verify linked list structure
	for i := 0; i < 9; i++ {
		assert.Equal(t, i+1, freeList.bufDescriptors[i].nxtFreeBuffer)
	}
	assert.Equal(t, -1, freeList.bufDescriptors[9].nxtFreeBuffer)
}

func TestCreateFreeList_ZeroMemSize(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}

	// When memSize is 0, free list creation should fail.
	var err error
	freeList, err = createFreeList(bc.blockSize, 0)
	assert.Error(t, err)
	assert.Nil(t, freeList)
}

func TestCreateFreeList_InvalidBufferSize(t *testing.T) {
	fl, err := createFreeList(0, 1024)
	assert.Error(t, err)
	assert.Nil(t, fl)

	fl, err = createFreeList(1, ^uint64(0))
	assert.Error(t, err)
	assert.Nil(t, fl)
}

func TestDestroyFreeList(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}

	setupTestFreeList(t, bc.blockSize, 5*bc.blockSize)

	destroyFreeList()

	// After destroy, freeList should be nil
	assert.Nil(t, freeList)
}

func TestFreeList_AllocateBuffer(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Allocate first buffer
	bufDesc, err := freeList.allocateBuffer(blk)
	assert.NoError(t, err)
	assert.NotNil(t, bufDesc)
	assert.Equal(t, 0, bufDesc.bufIdx)
	assert.Equal(t, blk, bufDesc.block)
	assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
	assert.Equal(t, 1, freeList.firstFreeBuffer)

	// Allocate second buffer
	blk2 := createBlock(1, "testId2", localBlock, f)
	bufDesc2, err := freeList.allocateBuffer(blk2)
	assert.NoError(t, err)
	assert.NotNil(t, bufDesc2)
	assert.Equal(t, 1, bufDesc2.bufIdx)
	assert.Equal(t, 2, freeList.firstFreeBuffer)
}

func TestFreeList_AllocateBuffer_Exhausted(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 3*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")

	// Allocate all buffers
	for i := 0; i < 3; i++ {
		blk := createBlock(i, "testId", localBlock, f)
		_, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
	}

	// Try to allocate one more - should fail
	blk := createBlock(99, "testId", localBlock, f)
	bufDesc, err := freeList.allocateBuffer(blk)
	assert.Error(t, err)
	assert.Equal(t, errFreeListFull, err)
	assert.Nil(t, bufDesc)
	assert.Equal(t, -1, freeList.firstFreeBuffer)
	assert.Equal(t, -1, freeList.lastFreeBuffer)
}

func TestFreeList_ReleaseBuffer(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Allocate a buffer
	bufDesc, err := freeList.allocateBuffer(blk)
	assert.NoError(t, err)

	// Release it back
	freeList.releaseBuffer(bufDesc)

	// Verify the buffer is back in the free list
	isBack := isBufDescExistsInFreeList(freeList, bufDesc.bufIdx)
	assert.True(t, isBack, "Released buffer should be back in the free list")
}

func TestFreeList_AllocateClearsReleasedBuffer(t *testing.T) {
	bc = &BlockCache{blockSize: 1024}
	setupTestFreeList(t, bc.blockSize, bc.blockSize)
	defer destroyFreeList()

	first, err := freeList.allocateBuffer(createBlock(0, "first", localBlock, createFile("first.txt")))
	assert.NoError(t, err)
	for i := range first.buf {
		first.buf[i] = 0xff
	}
	freeList.releaseBuffer(first)

	second, err := freeList.allocateBuffer(createBlock(0, "second", localBlock, createFile("second.txt")))
	assert.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, make([]byte, len(second.buf)), second.buf)
}

func TestFreeList_EvictBuffer(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	bc.btm = newBufferTableMgr()
	defer destroyFreeList()

	// Verify victim pointer is initialized
	assert.Equal(t, 0, freeList.nxtVictimBuffer)

	// Allocate all buffers and pin 9 buffers and leave 1 buffer unpinned to be selected as victim
	for i := range 10 {
		blk := createBlock(int(i), "testId", localBlock, createFile("test.txt"))
		bufDesc, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
		assert.NotNil(t, bufDesc)
		assert.Equal(t, i, bufDesc.bufIdx)
		assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
		assert.Equal(t, int32(0), bufDesc.refCnt.Load(), "Newly allocated buffer should have refCnt 0")

		bufDesc.refCnt.Store(refCountTableOnly) // Pin the buffer for buffer table manager.
		bc.btm.table[blk] = bufDesc
		if i < 9 {
			bufDesc.refCnt.Add(1) // Pin the buffer to indicate it's in use and should not be selected as victim
		}
	}

	// Eviction returns the unpinned buffer detached and reset for reuse.
	victimBufDesc, err := freeList.evictBuffer(bc.workerPool, bc.btm, accessDemand)
	assert.NoError(t, err)
	assert.NotNil(t, victimBufDesc)
	assert.Equal(t, 9, victimBufDesc.bufIdx)
	assert.Equal(t, 0, freeList.nxtVictimBuffer) // Should advance victim pointer
	assert.Zero(t, victimBufDesc.refCnt.Load())
	assert.Nil(t, victimBufDesc.block)
	assert.Len(t, bc.btm.table, 9)
}

func TestFreeList_EvictBuffer_AllInUse(t *testing.T) {
	// In production, FUSE limits threads so buffers eventually release
	// Testing this so that the error is getting thrown in this edge case

	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	bc.btm = newBufferTableMgr()
	defer destroyFreeList()

	// Verify victim pointer is initialized
	assert.Equal(t, 0, freeList.nxtVictimBuffer)

	// Allocate all buffers and pin 9 buffers and leave 1 buffer unpinned to be selected as victim
	for i := range 10 {
		blk := createBlock(int(i), "testId", localBlock, createFile("test.txt"))
		bufDesc, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
		assert.NotNil(t, bufDesc)
		assert.Equal(t, i, bufDesc.bufIdx)
		assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
		assert.Equal(t, int32(0), bufDesc.refCnt.Load(), "Newly allocated buffer should have refCnt 0")

		bufDesc.refCnt.Store(refCountTableAndOneUser) // Pin the buffer for buffer table manager.
		bc.btm.table[blk] = bufDesc
	}

	// Get victim buffer - should return nil, as all the buffers are in use.
	victimBufDesc, err := freeList.evictBuffer(bc.workerPool, bc.btm, accessDemand)
	assert.Nil(t, victimBufDesc)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errNoVictimBufferFound)
}

func TestBufferDescriptor_UsageCount(t *testing.T) {
	bd := &bufferDescriptor{}

	bd.recordAccess(accessPrefetch)
	bd.recordAccess(accessMaintenance)
	assert.Zero(t, bd.usageCount.Load())
	bd.recordAccess(accessDemand)
	bd.recordAccess(accessWrite)
	assert.Equal(t, uint32(2), bd.usageCount.Load())
	for range maxBufferUsageCount * 2 {
		bd.recordAccess(accessDemand)
	}
	assert.Equal(t, uint32(maxBufferUsageCount), bd.usageCount.Load())
	assert.True(t, bd.ageUsage())
	assert.Equal(t, uint32(maxBufferUsageCount-1), bd.usageCount.Load())
	bd.recordAccess(bufferAccessKind(255))
	assert.Equal(t, uint32(maxBufferUsageCount-1), bd.usageCount.Load())
}

func TestFreeList_ClockSweepPrefersColdBuffer(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 3*bc.blockSize)
	bc.btm = newBufferTableMgr()
	defer destroyFreeList()

	f := createFile("clock.txt")
	descriptors := make([]*bufferDescriptor, 3)
	for i := range descriptors {
		blk := createBlock(i, "id", localBlock, f)
		bd, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
		bd.refCnt.Store(refCountTableOnly)
		bd.dirty.Store(false)
		bc.btm.table[blk] = bd
		descriptors[i] = bd
	}
	descriptors[0].usageCount.Store(3)
	descriptors[1].usageCount.Store(0)
	descriptors[2].usageCount.Store(1)

	victim, err := freeList.evictBuffer(bc.workerPool, bc.btm, accessDemand)
	assert.NoError(t, err)
	assert.Same(t, descriptors[1], victim)
	assert.Nil(t, victim.block)
	assert.Equal(t, uint32(2), descriptors[0].usageCount.Load())
}

func TestFreeList_ClockSweepPrefersPrefetchOverDemand(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 2*bc.blockSize)
	bc.btm = newBufferTableMgr()
	defer destroyFreeList()

	f := createFile("prefetch.txt")
	prefetchBlock := createBlock(0, "prefetch", localBlock, f)
	demandBlock := createBlock(1, "demand", localBlock, f)
	prefetched, _, err := bc.btm.getOrCreateBufferDescriptor(freeList, bc.workerPool, prefetchBlock, accessPrefetch)
	assert.NoError(t, err)
	demand, _, err := bc.btm.getOrCreateBufferDescriptor(freeList, bc.workerPool, demandBlock, accessDemand)
	assert.NoError(t, err)
	prefetched.dirty.Store(false)
	demand.dirty.Store(false)
	prefetched.release(freeList)
	demand.release(freeList)

	victim, err := freeList.evictBuffer(bc.workerPool, bc.btm, accessDemand)
	assert.NoError(t, err)
	assert.Same(t, prefetched, victim)
	assert.Nil(t, victim.block)
	assert.Equal(t, uint32(1), demand.usageCount.Load())
}

func TestFreeList_AvailabilityNotificationBeforeWait(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, bc.blockSize)
	defer destroyFreeList()

	changed, err := freeList.watchAvailability()
	assert.NoError(t, err)
	freeList.notifyAvailability()

	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("availability notification was lost before wait")
	}
}

func TestFreeList_DestroyWakesAvailabilityWaiter(t *testing.T) {
	fl, err := createFreeList(1024, 1024)
	assert.NoError(t, err)
	changed, err := fl.watchAvailability()
	assert.NoError(t, err)

	fl.destroy()

	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("availability waiter did not wake during buffer-pool shutdown")
	}
	_, err = fl.watchAvailability()
	assert.ErrorIs(t, err, errFreeListClosed)
	_, err = fl.allocateBuffer(createBlock(0, "closed", localBlock, createFile("closed.txt")))
	assert.ErrorIs(t, err, errFreeListClosed)
}

func TestFreeList_CircularVictimSelection(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 5*bc.blockSize)
	defer destroyFreeList()

	// Verify that nxtVictimBuffer wraps around
	assert.Equal(t, 0, freeList.nxtVictimBuffer)

	// Simulate advancing victim pointer
	numBuffers := len(freeList.bufDescriptors)
	for i := 0; i < numBuffers*2; i++ {
		freeList.mutex.Lock()
		bufDesc := freeList.bufDescriptors[freeList.nxtVictimBuffer]
		freeList.nxtVictimBuffer = (freeList.nxtVictimBuffer + 1) % numBuffers
		freeList.mutex.Unlock()
		assert.NotNil(t, bufDesc)
	}

	// Should wrap around to 0
	assert.Equal(t, 0, freeList.nxtVictimBuffer)
}

func TestErrFreeListFull(t *testing.T) {
	assert.Error(t, errFreeListFull)
	assert.Contains(t, errFreeListFull.Error(), "free list is full")
}

func TestFreeList_DebugListMustBeFull(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 4*bc.blockSize)
	defer destroyFreeList()

	// Free list should be full initially.
	freeList.debugListMustBeFull()
}

func TestFreeList_DebugListMustBeFull_Panics(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 4*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Allocate one buffer so free list is no longer full
	_, err := freeList.allocateBuffer(blk)
	assert.NoError(t, err)

	assert.Panics(t, func() {
		freeList.debugListMustBeFull()
	})
}

func TestCreateFreeList_ZeroBuffers(t *testing.T) {
	// bufSize bigger than memSize should yield 0 buffers and error
	_, err := createFreeList(1024*1024, 512)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "0 buffers")
}
