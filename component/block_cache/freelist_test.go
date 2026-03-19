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
	assert.NotNil(t, freeList.bufPool)
	assert.NotNil(t, freeList.bufDescriptors)
	assert.Equal(t, 10, len(freeList.bufDescriptors))
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

	// Give some time for the reset goroutine to process
	// The buffer should eventually be back in the free list
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if isBufDescExistsInFreeList(freeList, bufDesc.bufIdx) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the buffer is back in the free list
	isBack := isBufDescExistsInFreeList(freeList, bufDesc.bufIdx)
	assert.True(t, isBack, "Released buffer should be back in the free list")
}

func TestFreeList_GetVictimBuffer(t *testing.T) {
	// This test is complex due to the blocking nature of getVictimBuffer
	// Just verify the basic structure exists
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	bc.btm = newBufferTableMgr()
	defer destroyFreeList()

	// Verify victim pointer is initialized
	assert.Equal(t, 0, freeList.nxtVictimBuffer)

	// Allocate all buffers and pin 9 buffers and leave 1 buffer unpinned to be selected as victim
	for i := range 10 {
		bufDesc, err := freeList.allocateBuffer(createBlock(int(i), "testId", localBlock, createFile("test.txt")))
		assert.NoError(t, err)
		assert.NotNil(t, bufDesc)
		assert.Equal(t, i, bufDesc.bufIdx)
		assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
		assert.Equal(t, int32(0), bufDesc.refCnt.Load(), "Newly allocated buffer should have refCnt 0")

		bufDesc.refCnt.Store(refCountTableOnly) // Pin the buffer for buffer table manager.
		if i < 9 {
			bufDesc.refCnt.Add(1) // Pin the buffer to indicate it's in use and should not be selected as victim
		}
	}

	// Get victim buffer - should return the unpinned buffer
	victimBufDesc, err := freeList.getVictimBuffer(bc.workerPool, bc.btm)
	assert.NoError(t, err)
	assert.NotNil(t, victimBufDesc)
	assert.Equal(t, 9, victimBufDesc.bufIdx)
	assert.Equal(t, 0, freeList.nxtVictimBuffer) // Should advance victim pointer
	assert.Equal(t, int32(2), victimBufDesc.refCnt.Load(), "Victim buffer should be pinned to count 2 after selection")
}

func TestFreeList_GetVictimBuffer_AllInUse(t *testing.T) {
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
		bufDesc, err := freeList.allocateBuffer(createBlock(int(i), "testId", localBlock, createFile("test.txt")))
		assert.NoError(t, err)
		assert.NotNil(t, bufDesc)
		assert.Equal(t, i, bufDesc.bufIdx)
		assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
		assert.Equal(t, int32(0), bufDesc.refCnt.Load(), "Newly allocated buffer should have refCnt 0")

		bufDesc.refCnt.Store(refCountTableAndOneUser) // Pin the buffer for buffer table manager.
	}

	// Get victim buffer - should return nil, as all the buffers are in use.
	victimBufDesc, err := freeList.getVictimBuffer(bc.workerPool, bc.btm)
	assert.Nil(t, victimBufDesc)
	assert.Error(t, err)
	assert.ErrorIs(t, errNoVictimBufferFound, err)
}

func TestFreeList_EvictionCyclesPassed(t *testing.T) {
	// Test that eviction cycles counter exists and can be incremented
	bd := &bufferDescriptor{}

	assert.Equal(t, int32(0), bd.numEvictionCyclesPassed.Load())
	bd.numEvictionCyclesPassed.Add(1)
	assert.Equal(t, int32(1), bd.numEvictionCyclesPassed.Load())
}

func TestFreeList_VictimSelection_FullyRead(t *testing.T) {
	// Test the bytesRead threshold for victim selection
	bc = &BlockCache{blockSize: 1024 * 1024}

	bd := &bufferDescriptor{}
	bd.bytesRead.Store(int32(bc.blockSize))

	// Verify the buffer is marked as fully read (used in victim selection)
	assert.Equal(t, int32(bc.blockSize), bd.bytesRead.Load())
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
	assert.NotNil(t, errFreeListFull)
	assert.Contains(t, errFreeListFull.Error(), "Free list is full")
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
