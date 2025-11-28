package block_cache

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const StdBlockIdLength int = 24 // We use base64 encoded strings of length 24 in Blobfuse when updating the files.

var ErrInvalidBlockList = errors.New("Invalid Block List, not compatible with Block Cache for write operations")

// Represents the Block State
type blockState int32

const (
	localBlock      blockState = iota //Block is in local memory and is outofsync with Azure Storage.
	uncommitedBlock                   //Block is in the Azure Storage but not yet committed.
	committedBlock                    //Block is in the Azure Storage and committed.
)

type block struct {
	mu        sync.RWMutex
	file      *File        // Pointer to the parent file.
	idx       int          // Block Index
	id        string       // Block Id
	state     blockState   // It tells about the state of the block.
	numWrites atomic.Int32 // Number of writes happened to this block.
}

func createBlock(idx int, id string, state blockState, f *File) *block {
	blk := &block{
		idx:   idx,
		id:    id,
		state: state,
		file:  f,
	}

	return blk
}

type blocklistState int

const (
	// Invalid blocklist means the blocklist is not compatible with block cache, (i.e., blocks are not aligned to block
	//	size configured)
	blockListInvalid blocklistState = iota
	blockListValid
	blockListNotRetrieved
)

type blockList struct {
	list  []*block
	state blocklistState
}

func newBlockList() *blockList {
	return &blockList{
		list:  make([]*block, 0),
		state: blockListNotRetrieved,
	}
}

func validateBlockList(blkList *internal.CommittedBlockList, f *File) error {
	if blkList == nil || len(*blkList) == 0 {
		return ErrInvalidBlockList
	}
	listLen := len(*blkList)
	var newblkList []*block = make([]*block, 0, listLen)

	for idx, blk := range *blkList {
		if idx < (listLen-1) && blk.Size != bc.blockSize {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format blk idx : %d is having size %d bytes, while block size set is %d bytes", idx, blk.Size, bc.blockSize)
			return ErrInvalidBlockList
		} else if idx == (listLen-1) && blk.Size > bc.blockSize {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format, Last block(i.e., blk idx : %d) is having greater size(i.e., %d bytes) than block size configured is %d bytes", idx, blk.Size, bc.blockSize)
			return ErrInvalidBlockList
		} else if len(blk.Id) != StdBlockIdLength {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format, block Id length for blk idx : %d is %d bytes is not matching to what blobfuse uses(i.e., %d bytes)", idx, len(blk.Id), StdBlockIdLength)
			return ErrInvalidBlockList
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, committedBlock, f))
	}

	f.blockList.list = newblkList

	return nil
}

func updateBlockListForReadOnlyFile(f *File) {
	if len(f.blockList.list) != 0 {
		// no need to update blocklist again, if already present
		return
	}

	noOfBlocks := (f.size + int64(bc.blockSize) - 1) / int64(bc.blockSize)
	var newblkList []*block = make([]*block, 0, noOfBlocks)

	for i := range int(noOfBlocks) {
		newblkList = append(newblkList, createBlock(i, "", committedBlock, f))
	}

	f.blockList.list = newblkList
}

func getBlockIndex(offset int64) int {
	return int(offset / int64(bc.blockSize))
}

func convertOffsetIntoBlockOffset(offset int64) int64 {
	return offset - int64(getBlockIndex(offset))*int64(bc.blockSize)
}

func getBlockSize(size int64, idx int) int {
	return min(int(bc.blockSize), int(size)-(idx*int(bc.blockSize)))
}

func getNoOfBlocksInFile(size int64) int {
	return int((size + int64(bc.blockSize) - 1) / int64(bc.blockSize))
}

func (blk *block) scheduleUpload(bufDesc *bufferDescriptor, sync bool) {
	// This buffer descriptor has reached its maximum usage count, schedule upload.
	log.Debug("block::scheduleUpload: Scheduling upload for blockIdx: %d, bufferIdx: %d, sync: %v, usageCnt: %d, refCnt: %d",
		blk.idx, bufDesc.bufIdx, sync, bufDesc.bytesWritten.Load(), bufDesc.refCnt.Load())

	wait := make(chan struct{}, 1)
	//
	// Take the exclusive lock on buffer content to prevent further writes while upload is in progress.
	// This will be released after upload is complete.
	if sync {
		bufDesc.contentLock.Lock()
	}
	// Increment refCnt for upload
	bufDesc.refCnt.Add(1)

	// Schedule upload
	wp.queueWork(blk, bufDesc, false /*download*/, wait, sync /*sync*/)

	if sync {
		// Wait for upload to complete.
		<-wait
		if ok := bufDesc.release(); ok {
			log.Debug("BlockCache::scheduleUpload: Released bufferIdx: %d for blockIdx: %d back to free list after sync upload",
				bufDesc.bufIdx, blk.idx)
		}
	}
}

func (blk *block) scheduleDownload(bufDesc *bufferDescriptor, sync bool) {
	wait := make(chan struct{}, 1)
	// Increment refCnt for download
	bufDesc.refCnt.Add(1)

	// Schedule download
	wp.queueWork(blk, bufDesc, true, wait, sync)

	if sync {
		// Wait for download to complete.
		<-wait
		if ok := bufDesc.release(); ok {
			log.Debug("BlockCache::scheduleDownload: Released bufferIdx: %d for blockIdx: %d back to free list after sync download",
				bufDesc.bufIdx, blk.idx)
		}
	}
}
