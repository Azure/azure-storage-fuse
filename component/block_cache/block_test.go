package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateBlock(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testBlockId123", localBlock, f)

	assert.NotNil(t, blk)
	assert.Equal(t, 0, blk.idx)
	assert.Equal(t, "testBlockId123", blk.id)
	assert.Equal(t, localBlock, blk.state)
	assert.Equal(t, f, blk.file)
	assert.Equal(t, int32(0), blk.numWrites.Load())
}

func TestBlockStates(t *testing.T) {
	// Test the different block states
	assert.Equal(t, blockState(0), localBlock)
	assert.Equal(t, blockState(1), uncommitedBlock)
	assert.Equal(t, blockState(2), committedBlock)
}

func TestNewBlockList(t *testing.T) {
	bl := newBlockList()

	assert.NotNil(t, bl)
	assert.NotNil(t, bl.list)
	assert.Equal(t, 0, len(bl.list))
	assert.Equal(t, blockListNotRetrieved, bl.state)
}

func TestGetBlockIndex(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	// Test various offsets
	assert.Equal(t, 0, getBlockIndex(0))
	assert.Equal(t, 0, getBlockIndex(1024*1024-1))
	assert.Equal(t, 1, getBlockIndex(1024*1024))
	assert.Equal(t, 1, getBlockIndex(2*1024*1024-1))
	assert.Equal(t, 2, getBlockIndex(2*1024*1024))
	assert.Equal(t, 10, getBlockIndex(10*1024*1024))
}

func TestConvertOffsetIntoBlockOffset(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	// Test various offsets
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(0))
	assert.Equal(t, int64(100), convertOffsetIntoBlockOffset(100))
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(1024*1024))
	assert.Equal(t, int64(500), convertOffsetIntoBlockOffset(1024*1024+500))
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(5*1024*1024))
}

func TestGetBlockSize(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	// Test full blocks
	assert.Equal(t, 1024*1024, getBlockSize(10*1024*1024, 0))
	assert.Equal(t, 1024*1024, getBlockSize(10*1024*1024, 5))

	// Test last partial block
	assert.Equal(t, 512*1024, getBlockSize(5*1024*1024+512*1024, 5))

	// Test single block file smaller than block size
	assert.Equal(t, 500, getBlockSize(500, 0))
}

func TestGetNoOfBlocksInFile(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	// Test various file sizes
	assert.Equal(t, 0, getNoOfBlocksInFile(0))
	assert.Equal(t, 1, getNoOfBlocksInFile(1))
	assert.Equal(t, 1, getNoOfBlocksInFile(1024*1024))
	assert.Equal(t, 2, getNoOfBlocksInFile(1024*1024+1))
	assert.Equal(t, 10, getNoOfBlocksInFile(10*1024*1024))
	assert.Equal(t, 11, getNoOfBlocksInFile(10*1024*1024+1))
}

func TestUpdateBlockListForReadOnlyFile(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size = 5 * 1024 * 1024 // 5 MB file

	updateBlockListForReadOnlyFile(f)

	assert.Equal(t, 5, len(f.blockList.list))
	for i := 0; i < 5; i++ {
		assert.NotNil(t, f.blockList.list[i])
		assert.Equal(t, i, f.blockList.list[i].idx)
		assert.Equal(t, "", f.blockList.list[i].id)
		assert.Equal(t, committedBlock, f.blockList.list[i].state)
		assert.Equal(t, f, f.blockList.list[i].file)
	}

	// Test that calling again doesn't recreate the list
	oldList := f.blockList.list
	updateBlockListForReadOnlyFile(f)
	assert.Equal(t, oldList, f.blockList.list, "Should not recreate list if already present")
}

func TestUpdateBlockListForReadOnlyFile_EmptyFile(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("empty.txt")
	f.size = 0

	updateBlockListForReadOnlyFile(f)

	assert.Equal(t, 0, len(f.blockList.list))
}

func TestUpdateBlockListForReadOnlyFile_PartialBlock(t *testing.T) {
	// Setup mock bc
	bc = &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("partial.txt")
	f.size = 2*1024*1024 + 512*1024 // 2.5 MB file

	updateBlockListForReadOnlyFile(f)

	// Should have 3 blocks (0, 1, 2)
	assert.Equal(t, 3, len(f.blockList.list))
}

// SUSPICIOUS FINDING: ErrInvalidBlockList is returned when blocklist doesn't match expected format
// This could happen if files were created with different block sizes or by other tools
func TestErrInvalidBlockList(t *testing.T) {
	assert.NotNil(t, ErrInvalidBlockList)
	assert.Contains(t, ErrInvalidBlockList.Error(), "Invalid Block List")
}

func TestBlockListStates(t *testing.T) {
	// Test the different blocklist states
	assert.Equal(t, blocklistState(0), blockListInvalid)
	assert.Equal(t, blocklistState(1), blockListValid)
	assert.Equal(t, blocklistState(2), blockListNotRetrieved)
}
