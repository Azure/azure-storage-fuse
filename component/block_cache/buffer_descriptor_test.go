package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferDescriptor_String(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(5, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 10,
		block:  blk,
	}
	bd.refCnt.Store(2)
	bd.bytesRead.Store(100)
	bd.bytesWritten.Store(200)
	bd.numEvictionCyclesPassed.Store(1)
	bd.valid.Store(true)
	bd.dirty.Store(false)

	str := bd.String()
	assert.Contains(t, str, "bufIdx: 10")
	assert.Contains(t, str, "blockIdx: 5")
	assert.Contains(t, str, "refCnt: 2")
	assert.Contains(t, str, "bytesRead: 100")
	assert.Contains(t, str, "bytesWritten: 200")
	assert.Contains(t, str, "test.txt")
}

func TestBufferDescriptor_Release(t *testing.T) {
	// Setup mock free list
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Test normal release (refCnt from 2 to 1)
	bd.refCnt.Store(2)
	released := bd.release()
	assert.False(t, released, "Should not be released back to free list yet")
	assert.Equal(t, int32(1), bd.refCnt.Load())

	// Test release to 0
	released = bd.release()
	assert.False(t, released, "Should not be released at 0")
	assert.Equal(t, int32(0), bd.refCnt.Load())

	// Test release to -1 (should trigger free list return)
	released = bd.release()
	assert.True(t, released, "Should be released back to free list at -1")
	assert.Equal(t, int32(-1), bd.refCnt.Load())
}

// SUSPICIOUS FINDING: Release allows refCnt to go to -1 before returning to free list
// This is intentional design but could be confusing - it marks the buffer as "removed from table"
func TestBufferDescriptor_Release_NegativeOne(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Set to 0 and release
	bd.refCnt.Store(0)
	released := bd.release()

	assert.True(t, released, "Should be released at -1")
	assert.Equal(t, int32(-1), bd.refCnt.Load(), "RefCnt should be -1 to mark removal")
}

func TestBufferDescriptor_Release_Panic(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Set to -1 and try to release again - should panic
	bd.refCnt.Store(-1)

	assert.Panics(t, func() {
		bd.release()
	}, "Should panic when refCnt goes below -1")
}

func TestBufferDescriptor_Reset(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Use an allocated buffer from free list instead of getting a new one
	bufDesc, err := freeList.allocateBuffer(blk)
	assert.NoError(t, err)

	// Reuse the buffer descriptor from allocation
	bufDesc.bufIdx = 5
	bufDesc.nxtFreeBuffer = 10
	bufDesc.refCnt.Store(5)
	bufDesc.bytesRead.Store(100)
	bufDesc.bytesWritten.Store(200)
	bufDesc.numEvictionCyclesPassed.Store(3)
	bufDesc.valid.Store(true)
	bufDesc.dirty.Store(true)
	bufDesc.downloadErr = assert.AnError
	bufDesc.uploadErr = assert.AnError

	// Fill buffer with non-zero data
	for i := range bufDesc.buf {
		bufDesc.buf[i] = 0xFF
	}

	// Reset
	bufDesc.reset()

	// Verify all fields are reset
	assert.Nil(t, bufDesc.block)
	assert.Equal(t, -1, bufDesc.nxtFreeBuffer)
	assert.Equal(t, int32(0), bufDesc.refCnt.Load())
	assert.Equal(t, int32(0), bufDesc.bytesRead.Load())
	assert.Equal(t, int32(0), bufDesc.bytesWritten.Load())
	assert.Equal(t, int32(0), bufDesc.numEvictionCyclesPassed.Load())
	assert.False(t, bufDesc.valid.Load())
	assert.False(t, bufDesc.dirty.Load())
	assert.Nil(t, bufDesc.downloadErr)
	assert.Nil(t, bufDesc.uploadErr)

	// Verify buffer is zeroed
	for i := range bufDesc.buf {
		assert.Equal(t, byte(0), bufDesc.buf[i], "Buffer should be zeroed at index %d", i)
	}
}

func TestBufferDescriptor_EnsureBufferValidForRead_Valid(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}
	bd.valid.Store(true)

	err := bd.ensureBufferValidForRead()
	assert.NoError(t, err, "Should return nil for valid buffer")
}

func TestBufferDescriptor_EnsureBufferValidForRead_DownloadError(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx:      0,
		block:       blk,
		downloadErr: assert.AnError,
	}
	bd.valid.Store(false)

	err := bd.ensureBufferValidForRead()
	assert.Error(t, err, "Should return download error")
	assert.Equal(t, assert.AnError, err)
}

func TestBufferDescriptor_EnsureBufferValidForRead_WaitForDownload(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}
	bd.valid.Store(false)

	// Simulate download in progress - lock the content
	bd.contentLock.Lock()

	done := make(chan error)
	go func() {
		// This should wait for the lock
		done <- bd.ensureBufferValidForRead()
	}()

	// Simulate download completing
	bd.valid.Store(true)
	bd.contentLock.Unlock()

	// Should complete without error
	err := <-done
	assert.NoError(t, err)
}

// SUSPICIOUS FINDING: ensureBufferValidForRead panics if buffer is neither valid nor has error
// This indicates an inconsistent state that shouldn't happen
func TestBufferDescriptor_EnsureBufferValidForRead_InconsistentState(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}
	bd.valid.Store(false)
	// No download error set, and not valid - this is inconsistent

	assert.Panics(t, func() {
		bd.ensureBufferValidForRead()
	}, "Should panic on inconsistent state")
}

func TestBufferDescriptor_AtomicFields(t *testing.T) {
	bd := &bufferDescriptor{}

	// Test all atomic fields can be set and read
	bd.refCnt.Store(10)
	assert.Equal(t, int32(10), bd.refCnt.Load())

	bd.bytesRead.Store(20)
	assert.Equal(t, int32(20), bd.bytesRead.Load())

	bd.bytesWritten.Store(30)
	assert.Equal(t, int32(30), bd.bytesWritten.Load())

	bd.numEvictionCyclesPassed.Store(5)
	assert.Equal(t, int32(5), bd.numEvictionCyclesPassed.Load())

	bd.valid.Store(true)
	assert.True(t, bd.valid.Load())

	bd.dirty.Store(true)
	assert.True(t, bd.dirty.Load())
}

func TestBufferDescriptor_Initialization(t *testing.T) {
	bd := &bufferDescriptor{
		bufIdx:        42,
		nxtFreeBuffer: 43,
	}

	assert.Equal(t, 42, bd.bufIdx)
	assert.Equal(t, 43, bd.nxtFreeBuffer)
	assert.Nil(t, bd.block)
	assert.Equal(t, int32(0), bd.refCnt.Load())
	assert.False(t, bd.valid.Load())
	assert.False(t, bd.dirty.Load())
}
