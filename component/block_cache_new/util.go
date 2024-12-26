package block_cache_new

import (
	"strings"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

const remote_block bool = true
const local_block bool = false
const zero_block_id string = "nBjhkW1MQstCqpeuOmlBOQ=="

type block struct {
	sync.RWMutex
	idx            int      // Block Index
	id             string   // Block Id
	buf            *Buffer  // Inmemory buffer if exists.
	block_type     bool     // It tells about the block came from the remote or  block came from the local
	downloadStatus chan int // This channel gets handy when multiple handles works on same block at a time.
}

func createBlock(idx int, id string, block_type bool) *block {
	return &block{idx: idx,
		id:             id,
		buf:            nil,
		block_type:     block_type,
		downloadStatus: make(chan int),
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
