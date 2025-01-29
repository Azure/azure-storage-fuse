package block_cache_new

import (
	"context"
	"strings"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// The following is the hardcoded blockId for punching the holes inside the file.
// Utilised When writing the file in the sparse manner.
// If we fix on this then we can do some sneaky optimisations while reading/writing the file.
// It is very less probable that the any UUID generated will match the same.
// How do I come up with the value?
// It is the base64 encoding string which came from calling common.NewUUIDWithLength function on
// the string "AzureLuvBlobfuse". The string took is of 16 bytes.
const zeroBlockId string = "mb7yh/CyR8dYgZnL0kunig=="

// "QXp1cmVMRXZCbG9iZnVzZQo="
const StdBlockIdLength int = 24 // We use base64 encoded strings of length 24 in Blobfuse when updating the files.

// Represents the Block State
type blockState int

const (
	localBlock      blockState = iota //Block is in local memory
	uncommitedBlock                   //Block is in the Azure Storage but not reflected yet in the Remote file
	committedBlock                    //Block is present inside the remote file
)

type block struct {
	sync.RWMutex
	idx                      int                // Block Index
	id                       string             // Block Id
	buf                      *Buffer            // Inmemory buffer if exists.
	state                    blockState         // It tells about the state of the block.
	hole                     bool               // Hole means this block is a null block. This can be used to do some optimisations.
	downloadDone             chan error         // Channel to know when the download completes.
	uploadDone               chan error         // Channel to know when the uplaod completes.
	uploadInProgress         chan struct{}      // communication between sync and async uploaders.
	cancelOngoingAsyncUpload context.CancelFunc // This function cancels the ongoing async upload, if any write comes after its scheduling.
	uploadCtx                context.Context
	file                     *File // file object that this block belong to.
}

func createBlock(idx int, id string, state blockState, f *File) *block {
	blk := &block{
		idx:                      idx,
		id:                       id,
		buf:                      nil,
		state:                    state,
		hole:                     false,
		downloadDone:             make(chan error, 1),
		uploadInProgress:         make(chan struct{}, 1),
		uploadDone:               make(chan error, 1),
		cancelOngoingAsyncUpload: func() {},
		file:                     f,
	}
	close(blk.downloadDone)
	close(blk.uploadInProgress)
	close(blk.uploadDone)
	return blk
}

type blockList []*block

func validateBlockList(blkList *internal.CommittedBlockList, f *File) (blockList, bool) {
	if blkList == nil {
		return nil, false
	}
	listLen := len(*blkList)
	var newblkList blockList
	for idx, blk := range *blkList {
		if (idx < (listLen-1) && blk.Size != bc.blockSize) || (idx == (listLen-1) && blk.Size > bc.blockSize) || (len(blk.Id) != StdBlockIdLength) {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format ")
			return nil, false
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, committedBlock, f))
	}
	return newblkList, true
}

func getBlockIndex(offset int64) int {
	return int(offset / int64(BlockSize))
}

func convertOffsetIntoBlockOffset(offset int64) int64 {
	return offset - int64(getBlockIndex(offset))*int64(BlockSize)
}

func getBlockSize(size int64, idx int) int {
	return min(int(BlockSize), int(size)-(idx*BlockSize))
}

// Todo: This following is incomplete
func populateFileInfo(file *File, attr *internal.ObjAttr) {
	file.size = attr.Size
}

func isDstPathTempFile(path string) bool {
	return strings.Contains(path, ".fuse_hidden")
}

// When block gets modified, update its fields.
func updateModifiedBlock(blk *block) {
	blk.state = localBlock
	blk.hole = false
}

func changeStateOfBlockToLocal(idx int, blk *block) error {
	_, err := getBlock(idx, blk, true)
	if err != nil {
		log.Trace("BlockCache::Truncate File : FAILED when retrieving last block idx=%d, path=%s, size=%d", idx, blk.file.Name, blk.file.size)
		return err
	}
	// todo: Lock simplification
	blk.Lock()
	updateModifiedBlock(blk)
	blk.Unlock()
	return nil
}
