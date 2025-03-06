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
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
	updateRecentnessOfBlk chan *block // Channel used to update the recentness of LRU
	localBlksLst          *list.List  // LRU of Local Blocks which are yet to upload for all the open handles. This list can contain only Local Blocks
	syncedBlksLst         *list.List  // LRU of Synced Blocks which are present in memory for all the open handles. This list can contain both commited and Uncommited blocks.
	lruCache              map[*block]*list.Element
	maxBlocks             int // Size of the each buffer in the bytes for this pool
}

func newBufferPool(memSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		updateRecentnessOfBlk: make(chan *block),
		localBlksLst:          list.New(), // Intialize the LRU
		syncedBlksLst:         list.New(), // Intialize the LRU
		lruCache:              make(map[*block]*list.Element),
		maxBlocks:             memSize / BlockSize,
	}
	zeroBuffer = bPool.getBuffer(true)
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

func (bp *BufferPool) removeBlockFromLRU(blk *block) {
	if ee, ok := bp.lruCache[blk]; ok {
		bp.localBlksLst.Remove(ee)
		bp.syncedBlksLst.Remove(ee)
		delete(bp.lruCache, blk)
	}
}

// This is called when all the references to open file were closed.
func (bp *BufferPool) removeBlocksFromLRU(blkList blockList) {
	bp.Lock()
	defer bp.Unlock()
	for _, blk := range blkList {
		bp.removeBlockFromLRU(blk)
		bp.releaseBuffer(blk)
	}
}

func (bp *BufferPool) moveBlkFromSBLtoLBL(blk *block) {
	bp.Lock()
	defer bp.Unlock()
	if ee, ok := bp.lruCache[blk]; ok {
		bp.syncedBlksLst.Remove(ee)
		bp.addLocalBlockToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromSBLtoLBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
	}
}

func (bp *BufferPool) moveBlkFromLBLtoSBL(blk *block) {
	if ee, ok := bp.lruCache[blk]; ok {
		bp.localBlksLst.Remove(ee)
		bp.addSyncedBlockToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromLBLtoSBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
	}
}

// small optimisation to reduce the lock usage in the sequentail workflows.
// When you are sequentially reading/writing the file, no need to update the recentness for every (4/128)K read/write, do it when the block change
// But this don't work when the user works on multiple files at the same time.
func (bp *BufferPool) updateLRU() {
	var prevblk *block
	for {
		blk := <-bp.updateRecentnessOfBlk
		if prevblk != blk {
			log.Info("BlockCache::updateLRU : updating the recentness of the block idx: %d, filename: %s", blk.idx, blk.file.Name)
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

// Responsible for giving blocks back to the pool,
// Working:
// 1. Take the buffers from the blks which are idle and the blocks were commited/uncommited.
// 2. If 1 is not enough, then schedule uploads for the local blks so that we can take back memory from them later using step 1.
func (bp *BufferPool) bufferReclaimation(r requestType) {
	// Take the lock on buffer pool if the request is of type async
	if r == asyncRequest {
		bp.Lock()
		defer bp.Unlock()
	}

	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	totalUsage := ((outstandingBlks * 100) / bp.maxBlocks)

	usage := ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
	noOfBlksToFree := max(((usage-70)*bp.maxBlocks)/100, 0)
	log.Info("BlockCache::bufferReclaimation : [START] [sync: %d] Total Mem usage: %d, Synced blks Mem Usage: %d, needed %d evictions", r, totalUsage, usage, noOfBlksToFree)

	currentBlk := bp.syncedBlksLst.Back()
	for currentBlk != nil && noOfBlksToFree > 0 {
		nxtblk := currentBlk.Prev()
		blk := currentBlk.Value.(*block)
		// Check the refcnt for the blk and only release buffer if the refcnt is zero.
		blk.Lock()
		if blk.refCnt == 0 {
			bp.removeBlockFromLRU(blk)
			bp.releaseBuffer(blk)
			log.Info("BlockCache::bufferReclaimation :  Successful reclaim blk idx: %d, file: %s", blk.idx, blk.file.Name)
			noOfBlksToFree--
		} else {
			log.Info("BlockCache::bufferReclaimation :  Unsuccessful reclaim blk idx: %d, file: %s, refcnt: %d", blk.idx, blk.file.Name, blk.refCnt)
		}
		blk.Unlock()
		currentBlk = nxtblk
	}

	outstandingBlks = bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	totalUsage = ((outstandingBlks * 100) / bp.maxBlocks)
	usage = ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
	log.Info("BlockCache::bufferReclaimation : [END] [sync: %d] Total Mem usage: %d, Synced Blks Mem Usage: %d, Unsuccessful evictions: %d", r, totalUsage, usage, noOfBlksToFree)
}

func (bp *BufferPool) asyncUploader(r requestType) {
	now := time.Now()
	if r == asyncRequest {
		bp.Lock()
		defer bp.Unlock()
	}
	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	totalUsage := ((outstandingBlks * 100) / bp.maxBlocks)

	usage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
	noOfAsyncUploads := max(((usage-20)*bp.maxBlocks)/100, 0)
	log.Info("BlockCache::asyncUploader : [START] [sync: %d]Mem usage: %d, Synced blks Mem Usage: %d, needed %d async Uploads", r, totalUsage, usage, noOfAsyncUploads)
	// Schedule uploads on least recently used blocks
	currentBlk := bp.localBlksLst.Back()
	for currentBlk != nil && noOfAsyncUploads > 0 {
		blk := currentBlk.Value.(*block)
		cow := time.Now()
		uploader(blk, asyncRequest)
		log.Info("BlockCache::asyncUploader : [took : %s] Upload scheduled for blk idx : %d, file: %s", time.Since(cow).String(), blk.idx, blk.file.Name)
		noOfAsyncUploads--
		currentBlk = currentBlk.Prev()
	}

	if r == syncRequest {
		// Wait for the async uploads to complete and get the local blks usage to less than 30
		noOfAsyncUploads = max(((usage-20)*bp.maxBlocks)/100, 0)
		currentBlk := bp.localBlksLst.Back()
		for currentBlk != nil && noOfAsyncUploads > 0 {
			blk := currentBlk.Value.(*block)
			cow := time.Now()
			state, _ := uploader(blk, syncRequest)
			if state == uncommitedBlock {
				bp.moveBlkFromLBLtoSBL(blk)
				log.Info("BlockCache::asyncUploader : Moved block from the LBL to SBL blk idx : %d, file: %s", blk.idx, blk.file.Name)
			}
			log.Info("BlockCache::asyncUploader : [took : %s] Waiting for Async Upload to complete blk idx : %d, file: %s", time.Since(cow).String(), blk.idx, blk.file.Name)
			noOfAsyncUploads--
			currentBlk = currentBlk.Prev()
		}
	}

	outstandingBlks = bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	totalUsage = ((outstandingBlks * 100) / bp.maxBlocks)

	usage = ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
	log.Info("BlockCache::asyncUploader : [END] [sync: %d][took : %s]Mem usage: %d, Synced blks Mem Usage: %d, Unsuccessful async upload schedules :%d", r, time.Since(now).String(), totalUsage, usage, noOfAsyncUploads)

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
		panic("BlockCache::getBufferForBlock : Todo: Read of evicted Uncommited block")
	}

	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	usage := ((outstandingBlks * 100) / bp.maxBlocks)
	log.Debug("BlockCache::getBufferForBlock : Total Mem Usage %d", usage)

	// Check the memory usage of synced blocks
	SBusage := ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
	log.Debug("BlockCache::getBufferForBlock : Synced Blocks Mem Usage %d", SBusage)
	if SBusage > 80 && SBusage < 95 {
		// reclaim the memory asynchronously
		go bPool.bufferReclaimation(asyncRequest)
	} else if SBusage >= 95 {
		// reclaim the memory synchronously.
		go bPool.bufferReclaimation(syncRequest)
	}

	// Check the memory of the local blocks
	LBusage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
	log.Debug("BlockCache::getBufferForBlock : local Blocks Mem Usage %d", LBusage)
	// Always keep the local blocks to less than 50%
	// Schedule the remaining blocks for async uploads.
	if LBusage > 30 && LBusage < 40 {
		go bPool.asyncUploader(asyncRequest)
	} else if LBusage > 50 {
		// doom is near, wait until it gets under 50.
		// Writing to the memory is superfast, while uploading the blk takes an eternity.
		// Better wait until the async uploads complete rather than getting into out of memory state.
		go bPool.asyncUploader(syncRequest)
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

func (bp *BufferPool) releaseBuffer(blk *block) {
	if blk.buf != nil {
		bp.pool.Put(blk.buf)
		blk.buf = nil
	}
}

// ************************************************************************
// The following are the functions describes on the buffer
// **************************************************************

func getBlockWithReadAhead(idx int, start int, file *File) (*block, error) {
	for i := 0; i <= 4; i++ {
		if i+start < (int(atomic.LoadInt64(&file.size))+BlockSize-1)/BlockSize {
			getBlockForRead(i+start, file, asyncRequest)
		}
	}
	blk, err := getBlockForRead(idx, file, syncRequest)
	return blk, err
}

// Returns the buffer containing block.
// Incements the refcnt on block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, file *File, r requestType) (blk *block, err error) {

	file.Lock()
	if idx >= len(file.blockList) {
		file.Unlock()
		log.Err("BlockCache::getBlockForRead : Cannot read block as offset is out of the file's blocklist")
		return nil, errors.New("block is out of the blocklist scope")
	}
	blk = file.blockList[idx]
	file.Unlock()
	if r == syncRequest {
		blk.incrementRefCnt()
	}
	_, err = downloader(blk, r)
	if r == syncRequest {
		bPool.updateRecentnessOfBlk <- blk
	}
	return blk, err
}

// This call will return buffer for writing for the block and also increments the refcnt of blk
// It is the responsibility of the caller to decrement the refcnt after the completion of the op.
// This call should always return some buffer if len(blocklist) <= 50000
func getBlockForWrite(idx int, file *File) (*block, error) {
	if idx >= MAX_BLOCKS {
		return nil, errors.New("write not supported, space completed") // can we return ENOSPC error here?
	}

	file.Lock()
	lenOfBlkLst := len(file.blockList)
	if idx >= lenOfBlkLst {
		// Update the state of the last block to local as null data may get's appended to it.
		if lenOfBlkLst > 0 {
			err := changeStateOfBlockToLocal(lenOfBlkLst-1, file.blockList[lenOfBlkLst-1])
			if err != nil {
				log.Err("BlockCache::getBlockForWrite : failed to convert the last block to local, file path=%s, size = %d, err = %s", file.Name, file.size, err.Error())
				file.Unlock()
				return nil, err
			}
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
		// Increment the refcnt on newly created block.
		blk.refCnt++
		file.Unlock()
		return blk, nil
	}
	blk := file.blockList[idx]
	file.Unlock()
	blk.incrementRefCnt()
	_, err := downloader(blk, syncRequest)
	bPool.updateRecentnessOfBlk <- blk
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
	log.Trace("BlockCache::commitBuffersForFile : %s\n", file.Name)
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
	bPool.removeBlocksFromLRU(f.blockList)
}
