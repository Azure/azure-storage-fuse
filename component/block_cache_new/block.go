package block_cache_new

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// The following is the hardcoded blockId for punching the holes inside the file.
// Utilised When writing the file in the sparse manner.
// If we fix on this then we can do some sneaky optimisations while reading/writing the file.
// It is very less probable that the any UUID generated will match the same.
// It will come very handy when reading blob which was written by blobfuse and has holes.
// also while doing aggressive random write we do flush the file sometimes to make the file consistent. It also helps there.
const zeroBlockId string = "mb7yh/CyR8dYgZnL0kunig=="

// "QXp1cmVMRXZCbG9iZnVzZQo="
const StdBlockIdLength int = 24 // We use base64 encoded strings of length 24 in Blobfuse when updating the files.

// Represents the Block State
type blockState int

const (
	localBlock      blockState = iota //Block is in local memory and is outofsync with Azure Storage.
	uncommitedBlock                   //Block is in the Azure Storage but not reflected yet in the Remote file
	committedBlock                    //Block is present inside the remote file
)

type block struct {
	sync.RWMutex
	idx                         int        // Block Index
	id                          string     // Block Id
	buf                         *Buffer    // Inmemory buffer if exists.
	state                       blockState // It tells about the state of the block.
	hole                        bool       // Hole means this block is a null block. This can be used to do some optimisations.
	refCnt                      int        // reference counter for block, how many handles are currenlty using block
	asyncUploadTimer            *time.Timer
	uploadDone                  chan error    // Channel to know when the uplaod completes.
	downloadDone                chan error    // Channel to know when the download completes.
	cancelOngoingAsyncUpload    func()        // This function cancels the ongoing async upload, maybe triggered by any write that comes after its scheduling.
	cancelOngolingAsyncDownload func()        // This function cancel the ongoing async download.
	requestingBuffer            chan struct{} // Used to serilaize the getBuffer calls
	requestingBufferFlag        bool          // first request of all getBuffer requests for the same block will make it true to say all others requests that it is doing flush operation
	file                        *File         // file object that this block belong to.
}

func createBlock(idx int, id string, state blockState, f *File) *block {
	blk := &block{
		idx:                         idx,
		id:                          id,
		buf:                         nil,
		state:                       state,
		hole:                        false,
		asyncUploadTimer:            time.NewTimer(defaultBlockTimeout),
		uploadDone:                  make(chan error, 1),
		downloadDone:                make(chan error, 1),
		cancelOngoingAsyncUpload:    func() {},
		cancelOngolingAsyncDownload: func() {},
		requestingBuffer:            make(chan struct{}),
		requestingBufferFlag:        false,
		file:                        f,
	}
	close(blk.uploadDone)
	close(blk.downloadDone)
	close(blk.requestingBuffer)
	return blk
}

func (blk *block) resetAsyncUploadTimer() {
	blk.asyncUploadTimer.Reset(defaultBlockTimeout)
}

func (blk *block) incrementRefCnt() int {
	blk.Lock()
	defer blk.Unlock()
	blk.refCnt++
	return blk.refCnt
}

func (blk *block) decrementRefCnt() int {
	blk.Lock()
	defer blk.Unlock()
	blk.refCnt--
	return blk.refCnt
}

type blockList []*block

func validateBlockList(blkList *internal.CommittedBlockList, f *File) (blockList, bool) {
	if blkList == nil || len(*blkList) == 0 {
		return createBlockListForReadOnlyFile(f), false
	}
	listLen := len(*blkList)
	var newblkList blockList
	for idx, blk := range *blkList {
		if (idx < (listLen-1) && blk.Size != bc.blockSize) || (idx == (listLen-1) && blk.Size > bc.blockSize) || (len(blk.Id) != StdBlockIdLength) {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format ")
			return createBlockListForReadOnlyFile(f), false
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, committedBlock, f))
	}
	return newblkList, true
}

func createBlockListForReadOnlyFile(f *File) blockList {
	size := f.size
	var newblkList blockList
	noOfBlocks := (size + int64(BlockSize) - 1) / int64(BlockSize)
	for i := range int(noOfBlocks) {
		newblkList = append(newblkList, createBlock(i, "", committedBlock, f))
	}
	return newblkList
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

// When block gets modified. Modify the LRU's and state.
func updateModifiedBlock(blk *block) bool {
	blk.hole = false
	if blk.state == committedBlock || blk.state == uncommitedBlock {
		blk.state = localBlock
		return true
	}
	return false
}

func changeStateOfBlockToLocal(idx int, blk *block) error {
	_, err := downloader(blk, syncRequest)
	if err != nil {
		log.Err("BlockCache::changeStateOfBlockToLocal : download failed for blk idx=%d, path=%s, size=%d", idx, blk.file.Name, blk.file.size)
		return err
	}
	blk.cancelOngoingAsyncUpload()
	blk.Lock()
	firstWritetoBlk := updateModifiedBlock(blk)
	blk.refCnt--
	if blk.refCnt < 0 {
		panic("BlockCache::changeStateOfBlockToLocal : Ref cnt for the blk is not getting modififed correctly")
	}
	blk.Unlock()
	if firstWritetoBlk {
		bPool.moveBlkFromSBLtoLBL(blk)
	}
	return nil
}

// PrintMemUsage outputs the current, total and OS memory being used. As well as the number
// of garage collection cycles completed.
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b
}
