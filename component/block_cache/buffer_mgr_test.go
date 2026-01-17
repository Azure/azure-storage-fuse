package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBufferTableMgr(t *testing.T) {
	NewBufferTableMgr()

	assert.NotNil(t, btm)
	assert.NotNil(t, btm.table)
	assert.Equal(t, 0, len(btm.table))
}

func TestBufDescStatus_String(t *testing.T) {
	assert.Equal(t, "bufDescStatusExists", bufDescStatusExists.String())
	assert.Equal(t, "bufDescStatusAllocated", bufDescStatusAllocated.String())
	assert.Equal(t, "bufDescStatusVictim", bufDescStatusVictim.String())
	assert.Equal(t, "bufDescStatusNeedsFileFlush", bufDescStatusNeedsFileFlush.String())
	assert.Equal(t, "bufDescStatusInvalid", bufDescStatusInvalid.String())
	assert.Equal(t, "Unknown", bufDescStatus(99).String())
}

func TestBufferTableMgr_LookUpBufferDescriptor_NotExists(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Lookup non-existent buffer
	bufDesc, err := btm.LookUpBufferDescriptor(blk)

	assert.NoError(t, err)
	assert.Nil(t, bufDesc)
}

func TestBufferTableMgr_LookUpBufferDescriptor_Exists(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Manually add a buffer to the table
	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(1)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Lookup existing buffer
	foundBd, err := btm.LookUpBufferDescriptor(blk)

	assert.NoError(t, err)
	assert.NotNil(t, foundBd)
	assert.Equal(t, bd, foundBd)
	assert.Equal(t, int32(2), foundBd.refCnt.Load(), "refCnt should be incremented")
}

func TestBufferTableMgr_RemoveBufferDescriptor_NotInTable(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(1)

	// Try to remove buffer that's not in table
	isRemoved, isReleased := btm.removeBufferDescriptor(bd, false)

	assert.True(t, isRemoved, "Should report as removed even if not in table")
	assert.False(t, isReleased, "Should not be released")
}

func TestBufferTableMgr_RemoveBufferDescriptor_Dirty(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(1)
	bd.dirty.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Try to remove dirty buffer
	isRemoved, isReleased := btm.removeBufferDescriptor(bd, false)

	assert.False(t, isRemoved, "Should not remove dirty buffer")
	assert.False(t, isReleased)
}

// SUSPICIOUS FINDING: removeBufferDescriptor with strict=true won't remove if refCnt > 0
// This prevents removing buffers that are still in use, but caller must handle this
func TestBufferTableMgr_RemoveBufferDescriptor_StrictWithRefs(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(2)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Try to remove buffer with strict=true and refCnt > 0
	isRemoved, isReleased := btm.removeBufferDescriptor(bd, true)

	assert.False(t, isRemoved, "Should not remove in strict mode with refs")
	assert.False(t, isReleased)
}

func TestBufferTableMgr_RemoveBufferDescriptor_Success(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(0)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Remove buffer successfully
	isRemoved, isReleased := btm.removeBufferDescriptor(bd, false)

	assert.True(t, isRemoved, "Should be removed from table")
	assert.True(t, isReleased, "Should be released to free list")
	assert.Equal(t, int32(-1), bd.refCnt.Load(), "refCnt should be -1")

	// Verify it's not in table anymore
	btm.mu.RLock()
	_, exists := btm.table[blk]
	btm.mu.RUnlock()
	assert.False(t, exists, "Should not be in table")
}

func TestBufferTableMgr_RemoveBufferDescriptor_WithRemainingRefs(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(2)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Remove buffer with strict=false (allows removal even with refs)
	isRemoved, isReleased := btm.removeBufferDescriptor(bd, false)

	assert.True(t, isRemoved, "Should be removed from table")
	assert.False(t, isReleased, "Should not be released yet (has refs)")
	assert.Equal(t, int32(1), bd.refCnt.Load(), "refCnt should be decremented")
}
