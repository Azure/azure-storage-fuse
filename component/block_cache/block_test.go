package block_cache

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
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
	blockSize := int64(1024 * 1024) // 1 MB

	// Test various offsets
	assert.Equal(t, 0, getBlockIndex(0, blockSize))
	assert.Equal(t, 0, getBlockIndex(1024*1024-1, blockSize))
	assert.Equal(t, 1, getBlockIndex(1024*1024, blockSize))
	assert.Equal(t, 1, getBlockIndex(2*1024*1024-1, blockSize))
	assert.Equal(t, 2, getBlockIndex(2*1024*1024, blockSize))
	assert.Equal(t, 10, getBlockIndex(10*1024*1024, blockSize))
}

func TestConvertOffsetIntoBlockOffset(t *testing.T) {
	blockSize := int64(1024 * 1024) // 1 MB

	// Test various offsets
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(0, blockSize))
	assert.Equal(t, int64(100), convertOffsetIntoBlockOffset(100, blockSize))
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(1024*1024, blockSize))
	assert.Equal(t, int64(500), convertOffsetIntoBlockOffset(1024*1024+500, blockSize))
	assert.Equal(t, int64(0), convertOffsetIntoBlockOffset(5*1024*1024, blockSize))
}

func TestGetBlockSize(t *testing.T) {
	blockSize := int64(1024 * 1024) // 1 MB

	// Test full blocks
	assert.Equal(t, 1024*1024, getBlockSize(10*1024*1024, 0, blockSize))
	assert.Equal(t, 1024*1024, getBlockSize(10*1024*1024, 5, blockSize))

	// Test last partial block
	assert.Equal(t, 512*1024, getBlockSize(5*1024*1024+512*1024, 5, blockSize))

	// Test single block file smaller than block size
	assert.Equal(t, 500, getBlockSize(500, 0, blockSize))
}

func TestGetNoOfBlocksInFile(t *testing.T) {
	blockSize := int64(1024 * 1024) // 1 MB

	// Test various file sizes
	assert.Equal(t, 0, getNoOfBlocksInFile(0, blockSize))
	assert.Equal(t, 1, getNoOfBlocksInFile(1, blockSize))
	assert.Equal(t, 1, getNoOfBlocksInFile(1024*1024, blockSize))
	assert.Equal(t, 2, getNoOfBlocksInFile(1024*1024+1, blockSize))
	assert.Equal(t, 10, getNoOfBlocksInFile(10*1024*1024, blockSize))
	assert.Equal(t, 11, getNoOfBlocksInFile(10*1024*1024+1, blockSize))
}

func TestUpdateBlockListForReadOnlyFile(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size = 5 * 1024 * 1024 // 5 MB file

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

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
	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))
	assert.Equal(t, oldList, f.blockList.list, "Should not recreate list if already present")
}

func TestUpdateBlockListForReadOnlyFile_EmptyFile(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("empty.txt")
	f.size = 0

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

	assert.Equal(t, 0, len(f.blockList.list))
}

func TestUpdateBlockListForReadOnlyFile_PartialBlock(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("partial.txt")
	f.size = 2*1024*1024 + 512*1024 // 2.5 MB file

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

	// Should have 3 blocks (0, 1, 2)
	assert.Equal(t, 3, len(f.blockList.list))
}

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

func TestValidateBlockList_Valid(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size = 5 * 1024 * 1024 // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 1024*1024 + 1, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 2*1024*1024 + 1, Size: 1024 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)

	validate := func() {
		assert.NoError(t, err, "Block list should be valid")
		assert.NotNil(t, f.blockList)
		assert.Equal(t, 3, len(f.blockList.list))

		// Check that the blocks in the file's block list match the storage block list
		for i, blk := range f.blockList.list {
			assert.Equal(t, i, blk.idx)
			assert.Equal(t, (*storageBlockList)[i].Id, blk.id)
			assert.Equal(t, committedBlock, blk.state)
			assert.Equal(t, f, blk.file)
		}
	}

	validate()

	// Make the last block size smaller to test that validation allows it
	(*storageBlockList)[2].Size = 512 * 1024
	err = validateBlockList(storageBlockList, f, bc.blockSize)
	assert.NoError(t, err, "Block list should still be valid with smaller last block")

	validate()

	// Make the laste block size greater than block size to test that validation catches it
	(*storageBlockList)[2].Size = 2 * 1024 * 1024
	err = validateBlockList(storageBlockList, f, bc.blockSize)
	assert.Error(t, err, "Block list should be invalid with last block size greater than block size")
}

func TestValidateBlockList_Invalid_BlockIDLen(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size = 5 * 1024 * 1024 // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 1024*1024 + 1, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 2*1024*1024 + 1, Size: 1024 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)
	assert.Error(t, err, "Block list should be invalid with incorrect block ID length")
}

func TestValidateBlockList_Invalid_BlockSizes(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size = 5 * 1024 * 1024 // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 1024*1024 + 1, Size: 2 * 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 3*1024*1024 + 1, Size: 10 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)
	assert.Error(t, err, "Block list should be invalid with incorrect block sizes")
}
