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
	assert.Equal(t, localBlock, blk.getState())
	assert.Equal(t, f, blk.file)
	assert.Equal(t, int32(0), blk.numWrites.Load())
}

func TestBlockStates(t *testing.T) {
	// Test the different block states
	assert.Equal(t, localBlock, blockState(0))
	assert.Equal(t, uncommitedBlock, blockState(1))
	assert.Equal(t, committedBlock, blockState(2))
}

func TestNewBlockList(t *testing.T) {
	bl := newBlockList()

	assert.NotNil(t, bl)
	assert.NotNil(t, bl.list)
	assert.Empty(t, bl.list)
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
	f.size.Store(5 * 1024 * 1024) // 5 MB file

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

	assert.Len(t, f.blockList.list, 5)
	for i := 0; i < 5; i++ {
		assert.NotNil(t, f.blockList.list[i])
		assert.Equal(t, i, f.blockList.list[i].idx)
		assert.Empty(t, f.blockList.list[i].id)
		assert.Equal(t, committedBlock, f.blockList.list[i].getState())
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
	f.size.Store(0)

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

	assert.Empty(t, f.blockList.list)
}

func TestUpdateBlockListForReadOnlyFile_PartialBlock(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("partial.txt")
	f.size.Store(2*1024*1024 + 512*1024) // 2.5 MB file

	updateBlockListForReadOnlyFile(f, int64(bc.blockSize))

	// Should have 3 blocks (0, 1, 2)
	assert.Len(t, f.blockList.list, 3)
}

// This could happen if files were created with different block sizes or by other tools
func TestErrInvalidBlockList(t *testing.T) {
	assert.Error(t, ErrInvalidBlockList)
	assert.Contains(t, ErrInvalidBlockList.Error(), "invalid block list")
}

func TestBlockListStates(t *testing.T) {
	// Test the different blocklist states
	assert.Equal(t, blockListInvalid, blocklistState(0))
	assert.Equal(t, blockListValid, blocklistState(1))
	assert.Equal(t, blockListNotRetrieved, blocklistState(2))
}

func TestValidateBlockList_Valid(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size.Store(5 * 1024 * 1024) // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 1024 * 1024, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 2 * 1024 * 1024, Size: 1024 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)

	validate := func() {
		assert.NoError(t, err, "Block list should be valid")
		assert.NotNil(t, f.blockList)
		assert.Len(t, f.blockList.list, 3)

		// Check that the blocks in the file's block list match the storage block list
		for i, blk := range f.blockList.list {
			assert.Equal(t, i, blk.idx)
			assert.Equal(t, (*storageBlockList)[i].Id, blk.id)
			assert.Equal(t, committedBlock, blk.getState())
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
	f.size.Store(5 * 1024 * 1024) // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 1024 * 1024, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(9), Offset: 2 * 1024 * 1024, Size: 1024 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)
	assert.Error(t, err, "Block list should be invalid with incorrect block ID length")
}

func TestValidateBlockList_Invalid_BlockSizes(t *testing.T) {
	bc := &BlockCache{
		blockSize: 1024 * 1024, // 1 MB
	}

	f := createFile("readonly.txt")
	f.size.Store(5 * 1024 * 1024) // 5 MB file

	storageBlockList := &internal.CommittedBlockList{
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 0, Size: 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 1024 * 1024, Size: 2 * 1024 * 1024},
		internal.CommittedBlock{Id: common.GetBlockID(common.BlockIDLength), Offset: 3 * 1024 * 1024, Size: 10 * 1024},
	}

	err := validateBlockList(storageBlockList, f, bc.blockSize)
	assert.Error(t, err, "Block list should be invalid with incorrect block sizes")
}

func TestBlockListStateString(t *testing.T) {
	assert.Equal(t, "Invalid", blockListInvalid.String())
	assert.Equal(t, "Valid", blockListValid.String())
	assert.Equal(t, "NotRetrieved", blockListNotRetrieved.String())
	assert.Contains(t, blocklistState(99).String(), "Unknown")
}

func TestValidateBlockIndex(t *testing.T) {
	assert.Error(t, validateBlockIndex(-1))
	assert.Error(t, validateBlockIndex(MAX_BLOCKS+1))
	assert.NoError(t, validateBlockIndex(0))
	assert.NoError(t, validateBlockIndex(MAX_BLOCKS))
}

func TestGetUploadSize(t *testing.T) {
	const blockSize = int64(16 * 1024 * 1024)

	size, err := getUploadSize(3*blockSize, 2, blockSize)
	assert.NoError(t, err)
	assert.Equal(t, int(blockSize), size)

	size, err = getUploadSize(2*blockSize+1024, 2, blockSize)
	assert.NoError(t, err)
	assert.Equal(t, 1024, size)

	_, err = getUploadSize(blockSize, 2, blockSize)
	assert.Error(t, err)
	_, err = getUploadSize(0, 0, blockSize)
	assert.Error(t, err)
	_, err = getUploadSize(blockSize, -1, blockSize)
	assert.Error(t, err)
}

func TestValidateBlockList_EmptyOrNil(t *testing.T) {
	blockSize := uint64(1024 * 1024)
	f := createFile("empty_list.txt")
	var nilList *internal.CommittedBlockList
	assert.Error(t, validateBlockList(nilList, f, blockSize))

	empty := internal.CommittedBlockList{}
	assert.Error(t, validateBlockList(&empty, f, blockSize))
}

func TestBlock_GetSetState(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	assert.Equal(t, localBlock, blk.getState())

	blk.setState(uncommitedBlock)
	assert.Equal(t, uncommitedBlock, blk.getState())

	blk.setState(committedBlock)
	assert.Equal(t, committedBlock, blk.getState())
}

func TestBlock_NumWrites(t *testing.T) {
	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	assert.Equal(t, int32(0), blk.numWrites.Load())
	blk.numWrites.Add(1)
	assert.Equal(t, int32(1), blk.numWrites.Load())
	blk.numWrites.Store(0)
	assert.Equal(t, int32(0), blk.numWrites.Load())
}
