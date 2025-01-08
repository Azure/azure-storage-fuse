package block_cache_new

import (
	"strings"
	"sync"

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
const (
	localBlock      int = iota //Block is in local memory
	uncommitedBlock            //Block is in the Azure Storage but not reflected yet in the Remote file
	committedBlock             //Block is present inside the remote file
)

type block struct {
	sync.RWMutex
	idx   int     // Block Index
	id    string  // Block Id
	buf   *Buffer // Inmemory buffer if exists.
	state int     // It tells about the state of the block.
	hole  bool    // Hole means this block is a null block. This can be used to do some sneaky optimisations.
}

func createBlock(idx int, id string, block_type int) *block {
	return &block{idx: idx,
		id:    id,
		buf:   nil,
		state: block_type,
		hole:  false,
	}
}

type blockList []*block

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
