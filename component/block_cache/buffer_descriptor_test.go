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
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Test normal release (refCnt from 2 to 1)
	bd.refCnt.Store(2)
	released := bd.release(freeList)
	assert.False(t, released, "Should not be released back to free list yet")
	assert.Equal(t, int32(1), bd.refCnt.Load())

	// Test release to 0 (should trigger free list return)
	released = bd.release(freeList)
	assert.True(t, released, "Should be released back to free list at 0")
	assert.Equal(t, int32(0), bd.refCnt.Load())
}

// Test that release correctly handles the transition to refCnt=0
func TestBufferDescriptor_Release_ToZero(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Set to 1 and release to 0
	bd.refCnt.Store(1)
	released := bd.release(freeList)

	assert.True(t, released, "Should be released at 0")
	assert.Equal(t, int32(0), bd.refCnt.Load(), "RefCnt should be 0 after release")
}

func TestBufferDescriptor_Release_Panic(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
	}

	// Set to 0 and try to release again - should panic
	bd.refCnt.Store(0)

	assert.Panics(t, func() {
		bd.release(freeList)
	}, "Should panic when refCnt goes below 0")
}

func TestBufferDescriptor_Reset(t *testing.T) {
	bc = &BlockCache{
		blockSize: 1024 * 1024,
	}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
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
	assert.NoError(t, bufDesc.downloadErr)
	assert.NoError(t, bufDesc.uploadErr)

	// Verify buffer is zeroed
	for i := range bufDesc.buf {
		assert.Equal(t, byte(0), bufDesc.buf[i], "Buffer should be zeroed at index %d", i)
	}
}

func TestBufferDescriptor_WriteCoverage(t *testing.T) {
	bd := &bufferDescriptor{buf: make([]byte, 4*writeCoverageGranularity)}

	assert.False(t, bd.markWriteCoverage(writeCoverageGranularity, 2*writeCoverageGranularity))
	assert.False(t, bd.markWriteCoverage(0, writeCoverageGranularity))
	assert.False(t, bd.markWriteCoverage(3*writeCoverageGranularity, 4*writeCoverageGranularity))
	assert.False(t, bd.markWriteCoverage(writeCoverageGranularity, 2*writeCoverageGranularity), "rewriting a covered region must not complete a gap")
	assert.True(t, bd.markWriteCoverage(2*writeCoverageGranularity, 3*writeCoverageGranularity))
	assert.Equal(t, []uint64{0b1111}, bd.writeCoverage)
	assert.Equal(t, 4, bd.coveredRegions)

	bd.resetWriteCoverage()
	assert.Equal(t, []uint64{0}, bd.writeCoverage)
	assert.Zero(t, bd.coveredRegions)
	assert.False(t, bd.markWriteCoverage(0, writeCoverageGranularity/2))
	assert.False(t, bd.markWriteCoverage(writeCoverageGranularity/2, writeCoverageGranularity), "partial writes are not merged within a region")
	assert.Zero(t, bd.coveredRegions)
	assert.False(t, bd.markWriteCoverage(-1, 4))
	assert.False(t, bd.markWriteCoverage(0, len(bd.buf)+1))
}

func TestBufferDescriptor_WriteCoveragePartialLastRegion(t *testing.T) {
	bufferSize := 2*writeCoverageGranularity + 123
	bd := &bufferDescriptor{buf: make([]byte, bufferSize)}

	assert.False(t, bd.markWriteCoverage(0, 2*writeCoverageGranularity))
	assert.True(t, bd.markWriteCoverage(2*writeCoverageGranularity, bufferSize))
	assert.Equal(t, []uint64{0b111}, bd.writeCoverage)
}

func TestBufferContentLease(t *testing.T) {
	bd := &bufferDescriptor{}
	other := &bufferDescriptor{}

	lease := bd.lockContent()
	assert.True(t, lease.belongsTo(bd))
	assert.False(t, lease.belongsTo(other))
	assert.False(t, bd.contentLock.TryLock(), "lease must own the exclusive content lock")

	lease.release()
	assert.False(t, lease.belongsTo(bd))
	assert.True(t, bd.contentLock.TryLock(), "releasing the lease must unlock content")
	bd.contentLock.Unlock()

	assert.Panics(t, lease.release, "a content lease must be released exactly once")
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

	err := bd.ensureBufferValidForRead()
	assert.Error(t, err, "Should return error on inconsistent state")
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
