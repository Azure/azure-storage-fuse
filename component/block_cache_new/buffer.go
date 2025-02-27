package block_cache_new

import (
	"container/list"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// TODO: Implement GC after 80% of memory given for blobfuse
var zeroBuffer *Buffer
var defaultBlockTimeout = 1000 * time.Millisecond

type Buffer struct {
	data     []byte // Data hoding in the buffer
	valid    bool   // is date present in the buffer valid?
	checksum []byte // Checksum for the data
}

type BufferPool struct {
	pool sync.Pool // Pool used for buffer management.
	sync.Mutex
	asyncUploadQueue      chan *block
	updateRecentnessOfBlk chan *block // Channel used to update the recentness of LRU
	localBlksLst          *list.List  // LRU of Local Blocks which are yet to upload for all the open handles. This list can contain only Local Blocks
	syncedBlksLst         *list.List  // LRU of Synced Blocks which are present in memory for all the open handles. This list can contain both commited and Uncommited blocks.
	lruCache              map[*block]*list.Element
	maxBlocks             int // Size of the each buffer in the bytes for this pool
}

func newBufferPool(bufSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		// Sizes of following channel need to be decided.
		asyncUploadQueue:      make(chan *block, 5000),
		updateRecentnessOfBlk: make(chan *block),
		localBlksLst:          list.New(), // Intialize the LRU
		syncedBlksLst:         list.New(), // Intialize the LRU
		lruCache:              make(map[*block]*list.Element),
		maxBlocks:             bufSize / BlockSize,
	}
	zeroBuffer = bPool.getBuffer(true)
	go bPool.asyncUploadScheduler()
	go bPool.updateLRU()
	return bPool
}

func (bp *BufferPool) addLocalBlockToLRU(blk *block) {
	ele := bp.localBlksLst.PushFront(blk)
	bp.lruCache[blk] = ele
}

func (bp *BufferPool) addSyncedBlockToLRU(blk *block) {
	ele := bp.syncedBlksLst.PushFront(blk)
	bp.lruCache[blk] = ele
}

func (bp *BufferPool) removeBlksFromLRU(blkList blockList) {

}

// small optimisation to reduce the lock usage in the sequentail workflows.
// When you are sequentially reading/writing the file, no need to update the recentness for every (4/128)K read/write, do it when the block change
// But this don't work when the user works on multiple files at a time.
func (bp *BufferPool) updateLRU() {
	var prevblk *block
	for {
		blk := <-bp.updateRecentnessOfBlk
		if prevblk != blk {
			// Update the recentness in the LRU's
			bp.Lock()
			if ee, ok := bp.lruCache[blk]; ok {
				bp.localBlksLst.MoveToFront(ee)
				bp.syncedBlksLst.MoveToFront(ee)
			}
			bp.Unlock()
		}
		prevblk = blk
	}
}

// It will Schedule a task for uploading the block to Azure Storage asynchronously
// when timer since last operation on the block was over.
// lazy scheduling of the blocks to the azure storage if the file is opened for so much time.
func (bp *BufferPool) asyncUploadScheduler() {
	for blk := range bp.asyncUploadQueue {
		<-blk.asyncUploadTimer.C
		uploader(blk, asyncRequest)
	}
}

// Responsible for giving blocks back to the pool,
// This can only take the buffers from the commited/uncommited blocks
// and not from the local block
// This will kick-in when the memory configured for the process is getting exhausted:
// So we will take back the memory blks from the already open handles which are not using them.
// Tries to keep the memory under 75%
// What if all the blocks are in localblocks queue, How can you retake from them?
// Maybe schedule uploads on them and then free here.
func (bp *BufferPool) bufferReclaimation(r requestType, usage int) {
	logy.Write([]byte(fmt.Sprintf("BlockCache::bufferReclaimation [START] cur usage: %d\n", usage)))
	// Take the lock on buffer pool if the request is of type async
	if r == asyncRequest {
		bp.Lock()
		defer bp.Unlock()
	}
	// noOfBlksToFree := ((usage - 75) * bPool.maxBlocks) / 100
	// currentBlk := bp.syncedBlks.Front()
	// for currentBlk != nil && noOfBlksToFree > 0 {
	// blk := currentBlk.Value.(*block)
	// blk.Lock()
	// //TODO: Release this blk only when no open handle is working on it
	// blk.releaseBuffer()
	// blk.Unlock()
	// bp.syncedBlks.Remove(currentBlk)
	// currentBlk = bp.syncedBlks.Front()
	// noOfBlksToFree--
	// }
	// Print the memory Utilization after reclaiming the blocks to pool.
	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	usage = (outstandingBlks / bp.maxBlocks) * 100
	logy.Write([]byte(fmt.Sprintf("BlockCache::bufferReclaimation [END] cur usage: %d\n", usage)))
}

func (bp *BufferPool) getBufferForBlock(blk *block) {
	bp.Lock()
	defer bp.Unlock()
	switch blk.state {
	case localBlock:
		blk.buf = bPool.getBuffer(true)
		bPool.addLocalBlockToLRU(blk)
	case committedBlock:
		blk.buf = bPool.getBuffer(false)
		bPool.addSyncedBlockToLRU(blk)
	case uncommitedBlock:
		//Todo: Block is evicted from the cache, to retrieve it, first we should do putBlockList
		panic("Todo: Read of evicted Uncommited block")
	}

	// Check the memory usage
	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	usage := (outstandingBlks / bp.maxBlocks) * 100

	if usage > 80 {
		// reclaim the memory asynchronously
		go bPool.bufferReclaimation(asyncRequest, usage)
	} else if usage > 95 {
		// reclaim the memory synchronously.
		bPool.bufferReclaimation(syncRequest, usage)
	}
}

// Returns the buffer.
// parameters: valid (Represents the data present in buffer is valid/not)
func (bp *BufferPool) getBuffer(valid bool) *Buffer {
	b := bp.pool.Get().(*Buffer)
	if b.data == nil {
		b.data = make([]byte, BlockSize)
	} else {
		copy(b.data, zeroBuffer.data)
	}
	b.valid = valid
	b.checksum = nil
	return b
}

func (bp *BufferPool) putBuffer(blk *block) {
	if blk.buf != nil {
		bp.pool.Put(blk.buf)
		blk.buf = nil
	}
}

// ************************************************************************
// The following are the functions describes on the buffer
// **************************************************************

func getBlockWithReadAhead(idx int, start int, file *File) (*block, error) {
	for i := 1; i <= 3; i++ {
		if i+start < (int(atomic.LoadInt64(&file.size))+BlockSize-1)/BlockSize {
			getBlockForRead(i+start, file, asyncRequest)
		}
	}
	blk, err := getBlockForRead(idx, file, syncRequest)
	return blk, err
}

// Returns the buffer containing block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, file *File, r requestType) (blk *block, err error) {

	file.Lock()
	if idx >= len(file.blockList) {
		file.Unlock()
		return nil, errors.New("block is out of the blocklist scope")
	}
	blk = file.blockList[idx]
	file.Unlock()
	_, err = downloader(blk, r)
	return blk, err
}

// This call will return buffer for writing for the block
// This call should always return some buffer if len(blocklist) <= 50000
func getBlockForWrite(idx int, file *File) (*block, error) {
	if idx >= MAX_BLOCKS {
		return nil, errors.New("write not supported space completed") // can we return ENOSPC error here?
	}

	file.Lock()
	lenOfBlkLst := len(file.blockList)
	if idx >= lenOfBlkLst {
		// Update the state of the last block to local as null data may get's appended to it.
		if lenOfBlkLst > 0 {
			changeStateOfBlockToLocal(lenOfBlkLst-1, file.blockList[lenOfBlkLst-1])
		}

		// Create at least 1 block. i.e, create blocks in the range (len(blocklist), idx]
		for i := lenOfBlkLst; i <= idx; i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, localBlock, file)
			file.blockList = append(file.blockList, blk)
			if i == idx {
				//Allocate a buffer for last block.
				//No need to lock the block as these are newly created blocks
				bPool.getBufferForBlock(blk)
			} else {
				blk.hole = true
			}
		}
		blk := file.blockList[idx]
		bPool.asyncUploadQueue <- blk
		file.Unlock()
		return blk, nil
	}
	blk := file.blockList[idx]
	file.Unlock()
	blkState, err := downloader(blk, syncRequest)
	if blkState == committedBlock || blkState == uncommitedBlock {
		bPool.asyncUploadQueue <- blk
	}
	return blk, err
}

// Write all the Modified buffers to Azure Storage and return whether file is modified or not.
func syncBuffersForFile(file *File) (bool, error) {
	var err error = nil
	var fileChanged bool = false

	file.Lock()
	for _, blk := range file.blockList {
		// To make the sync upload faster, first we schedule all the requests as async
		// Then status would be checked using sync requests.
		blkstate, _ := uploader(blk, asyncRequest)
		if blkstate != committedBlock {
			fileChanged = true
		}
	}
	for _, blk := range file.blockList {
		_, err := uploader(blk, syncRequest)
		if err != nil {
			panic(fmt.Sprintf("Upload doesn't happen for the block idx : %d, file : %s\n", blk.idx, blk.file.Name))
		}
	}
	file.Unlock()
	return fileChanged, err
}

func syncZeroBuffer(name string) error {
	return bc.NextComponent().StageData(
		internal.StageDataOptions{
			Ctx:  context.Background(),
			Name: name,
			Id:   zeroBlockId,
			Data: zeroBuffer.data,
		},
	)

}

// stages empty block for the hole
func punchHole(f *File) error {
	if f.holePunched {
		return nil
	}
	err := syncZeroBuffer(f.Name)
	if err == nil {
		f.holePunched = true
	}

	return err
}

func commitBuffersForFile(file *File) error {
	logy.Write([]byte(fmt.Sprintf("BlockCache::commitFile : %s\n", file.Name)))
	var blklist []string
	file.Lock()
	defer file.Unlock()
	if file.blkListState == blockListInvalid || file.blkListState == blockListNotRetrieved {
		return nil
	}

	for _, blk := range file.blockList {
		if blk.hole {
			blklist = append(blklist, zeroBlockId)
		} else {
			blklist = append(blklist, blk.id)
		}
	}
	err := bc.NextComponent().CommitData(internal.CommitDataOptions{Name: file.Name, List: blklist, BlockSize: uint64(BlockSize)})
	if err == nil {
		for _, blk := range file.blockList {
			blk.Lock()
			blk.state = committedBlock
			blk.Unlock()
		}
	}
	return err
}

// Release all the buffers to the file if this handle is the last one opened on the file.
func releaseBuffers(f *File) {
	//Lock was already acquired on file
	for _, blk := range f.blockList {
		blk.Lock()
		bPool.putBuffer(blk)
		blk.Unlock()
	}
}
