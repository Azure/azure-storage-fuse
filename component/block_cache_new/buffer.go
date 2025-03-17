package block_cache_new

import (
	"container/list"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
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
	updateRecentnessOfBlk      chan *block // Channel used to update the recentness of LRU
	localBlksLst               *list.List  // LRU of Local Blocks which are yet to upload for all the open handles. This list can contain only Local Blocks
	LBCache                    map[*block]*list.Element
	onTheWireBlksLst           *list.List // LRU of blocks for which the uploads has scheduled.
	OWBCache                   map[*block]*list.Element
	syncedBlksLst              *list.List // LRU of Synced Blocks which are present in memory for all the open handles. This list can contain both commited and Uncommited blocks.
	SBCache                    map[*block]*list.Element
	wakeUpBufferReclaimation   chan struct{}
	wakeUpAsyncUploadScheduler chan struct{}
	wakeUpAsyncUploadPoller    chan struct{}

	maxBlocks int // Size of the each buffer in the bytes for this pool
}

func newBufferPool(memSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		updateRecentnessOfBlk:      make(chan *block),
		localBlksLst:               list.New(), // Initialize the LRU
		LBCache:                    make(map[*block]*list.Element),
		onTheWireBlksLst:           list.New(), // Initialize the LRU
		OWBCache:                   make(map[*block]*list.Element),
		syncedBlksLst:              list.New(), // Initialize the LRU
		SBCache:                    make(map[*block]*list.Element),
		wakeUpBufferReclaimation:   make(chan struct{}, 1),
		wakeUpAsyncUploadScheduler: make(chan struct{}, 1),
		wakeUpAsyncUploadPoller:    make(chan struct{}, 1),

		maxBlocks: memSize / BlockSize,
	}
	zeroBuffer = bPool.getBuffer(true)
	go bPool.updateLRU()
	go bPool.bufferReclaimation()
	go bPool.asyncUploadScheduler()
	go bPool.asyncUpladPoller()
	return bPool
}

func (bp *BufferPool) addLocalBlockToLRU(blk *block) {
	ele := bp.localBlksLst.PushFront(blk)
	bp.LBCache[blk] = ele
}

func (bp *BufferPool) addSyncedBlockToLRU(blk *block) {
	ele := bp.syncedBlksLst.PushFront(blk)
	bp.SBCache[blk] = ele
}

func (bp *BufferPool) addPendingUploadBlkToLRU(blk *block) {
	ele := bp.onTheWireBlksLst.PushFront(blk)
	bp.OWBCache[blk] = ele
}

func (bp *BufferPool) removeBlockFromLRU(blk *block) {
	cnt := 0
	if ee, ok := bp.SBCache[blk]; ok {
		bp.syncedBlksLst.Remove(ee)
		delete(bp.SBCache, blk)
		log.Info("BlockCache::removeBlockFromLRU : Blk removed from SB cache blk idx : %d, file : %s", blk.idx, blk.file.Name)
		cnt++
	}
	if ee, ok := bp.LBCache[blk]; ok {
		bp.localBlksLst.Remove(ee)
		delete(bp.LBCache, blk)
		log.Info("BlockCache::removeBlockFromLRU : Blk removed from LB cache blk idx : %d, file : %s", blk.idx, blk.file.Name)
		cnt++
	}
	if ee, ok := bp.OWBCache[blk]; ok {
		bp.onTheWireBlksLst.Remove(ee)
		delete(bp.OWBCache, blk)
		log.Info("BlockCache::removeBlockFromLRU : Blk removed from OWB cache blk idx : %d, file : %s", blk.idx, blk.file.Name)
		cnt++
	}
	// Block should only present in 1LRU at any point of time
	if cnt > 1 {
		panic(fmt.Sprintf("BlockCache::removeBlockFromLRU : Blk is present in more than 1 LRU : blk idx : %d, fileName : %s", blk.idx, blk.file.Name))
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
	if ee, ok := bp.SBCache[blk]; ok {
		bp.syncedBlksLst.Remove(ee)
		delete(bp.SBCache, blk)
		bp.addLocalBlockToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromSBLtoLBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
	}
}

func (bp *BufferPool) moveBlkFromLBLtoOWBL(blk *block) {
	if ee, ok := bp.LBCache[blk]; ok {
		bp.localBlksLst.Remove(ee)
		delete(bp.LBCache, blk)
		bp.addPendingUploadBlkToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromLBLtoOWBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
	}
}

func (bp *BufferPool) moveBlkFromOWBLtoLBL(blk *block) {
	if ee, ok := bp.OWBCache[blk]; ok {
		bp.onTheWireBlksLst.Remove(ee)
		delete(bp.OWBCache, blk)
		bp.addLocalBlockToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromOWBLtoLBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
	}
}

func (bp *BufferPool) moveBlkFromOWBLtoSBL(blk *block) {
	if ee, ok := bp.OWBCache[blk]; ok {
		bp.onTheWireBlksLst.Remove(ee)
		delete(bp.OWBCache, blk)
		bp.addSyncedBlockToLRU(blk)
	} else {
		log.Err("BlockCache::moveBlkFromOWBLtoSBL : Block is not present in LRU cache, blk idx : %d, file: %s", blk.idx, blk.file.Name)
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
			// log.Info("BlockCache::updateLRU : updating the recentness of the block idx: %d, filename: %s", blk.idx, blk.file.Name)
			// Update the recentness in the LRU's
			bp.Lock()
			if ee, ok := bp.SBCache[blk]; ok {
				bp.syncedBlksLst.MoveToFront(ee)
			}
			if ee, ok := bp.LBCache[blk]; ok {
				bp.localBlksLst.MoveToFront(ee)
			}
			// if ee, ok := bp.OWBCache[blk]; ok {
			// 	b := ee.Value.(*block)
			// 	panic(fmt.Sprintf("BlockCache::updateLRU : Blk is present in OWB : blk idx : %d, fileName : %s", b.idx, b.file.Name))
			// }
			bp.Unlock()
		}
		prevblk = blk
	}
}

func (bp *BufferPool) getTotalMemUsage() int {
	outstandingBlks := bp.localBlksLst.Len() + bp.syncedBlksLst.Len()
	return ((outstandingBlks * 100) / bp.maxBlocks)
}

// Responsible for giving blocks back to the pool,
// Working:
// 1. Take the buffers from the blks which are idle and the blocks were commited/uncommited.
// 2. If 1 is not enough, then schedule uploads for the local blks so that we can take back memory from them later using step 1.
func (bp *BufferPool) bufferReclaimation() {
	// Take the lock on buffer pool if the request is of type async
	for {
		<-bp.wakeUpBufferReclaimation
		bp.Lock()
		totalUsage := bp.getTotalMemUsage()

		usage := ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
		noOfBlksToFree := max(((usage-60)*bp.maxBlocks)/100, 0)
		log.Info("BlockCache::bufferReclaimation : [START] Total Mem usage: %d, Synced blks Mem Usage: %d, needed %d evictions", totalUsage, usage, noOfBlksToFree)

		currentBlk := bp.syncedBlksLst.Back()
		for currentBlk != nil && noOfBlksToFree > 0 {
			blk := currentBlk.Value.(*block)
			currentBlk = currentBlk.Prev()
			// Check the refcnt for the blk and only release buffer if the refcnt is zero.
			blk.Lock()
			if blk.refCnt == 0 && blk.state != localBlock {
				blk.cancelOngolingAsyncDownload()
				bp.removeBlockFromLRU(blk)
				bp.releaseBuffer(blk)
				log.Info("BlockCache::bufferReclaimation :  Successful reclaim blk idx: %d, file: %s", blk.idx, blk.file.Name)
				noOfBlksToFree--
			} else {
				log.Info("BlockCache::bufferReclaimation :  Unsuccessful reclaim blk idx: %d, file: %s, refcnt: %d", blk.idx, blk.file.Name, blk.refCnt)
			}
			blk.Unlock()
		}

		totalUsage = bp.getTotalMemUsage()
		usage = ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
		log.Info("BlockCache::bufferReclaimation : [END] Total Mem usage: %d, Synced Blks Mem Usage: %d", totalUsage, usage)
		bp.Unlock()
	}
}

func (bp *BufferPool) asyncUploadScheduler() {
	for {
		<-bp.wakeUpAsyncUploadScheduler
		bp.Lock()
		now := time.Now()
		totalUsage := bp.getTotalMemUsage()

		usage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
		noOfAsyncUploads := max(((usage-20)*bp.maxBlocks)/100, 0)
		log.Info("BlockCache::asyncUploadScheduler : [START] Mem usage: %d, Synced blks Mem Usage: %d, needed %d async Uploads", totalUsage, usage, noOfAsyncUploads)
		// Schedule uploads on least recently used blocks
		currentBlk := bp.localBlksLst.Back()
		for currentBlk != nil && noOfAsyncUploads > 0 {
			blk := currentBlk.Value.(*block)
			currentBlk = currentBlk.Prev()
			cow := time.Now()
			uploader(blk, asyncRequest)
			bp.moveBlkFromLBLtoOWBL(blk)
			log.Info("BlockCache::asyncUploadScheduler : [took : %s] Async Upload scheduled for blk idx : %d, file: %s", time.Since(cow).String(), blk.idx, blk.file.Name)
			noOfAsyncUploads--
		}

		totalUsage = bp.getTotalMemUsage()

		usage = ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
		log.Info("BlockCache::asyncUploadScheduler : [END] [took : %s]Mem usage: %d, Synced blks Mem Usage: %d", time.Since(now).String(), totalUsage, usage)
		bp.Unlock()
	}

}

func (bp *BufferPool) asyncUpladPoller() {
	for {
		<-bp.wakeUpAsyncUploadPoller
		bp.Lock()
		now := time.Now()
		totalUsage := bp.getTotalMemUsage()
		// Wait for the async uploads to complete and get the local blks usage to less than 30
		usage := ((bp.onTheWireBlksLst.Len() * 100) / bp.maxBlocks)
		noOfAsyncPolls := max(((usage-20)*bp.maxBlocks)/100, 0)
		log.Info("BlockCache::asyncUploadPoller : [START] Mem usage: %d, Onwire blks Mem Usage: %d, needed %d async Polls", totalUsage, usage, noOfAsyncPolls)
		currentBlk := bp.onTheWireBlksLst.Back()
		for currentBlk != nil && noOfAsyncPolls > 0 {
			blk := currentBlk.Value.(*block)
			currentBlk = currentBlk.Prev()
			cow := time.Now()
			state, err := uploader(blk, syncRequest)
			if err != nil {
				// May be there was an write after scheduling, schedule it again
				bp.moveBlkFromOWBLtoLBL(blk)
				log.Info("BlockCache::asyncUploadPoller : Async Upload failed, err : %s, Rescheduling the blk idx : %d, file : %s", err.Error(), blk.idx, blk.file.Name)
			}
			if state == uncommitedBlock {
				bp.moveBlkFromOWBLtoSBL(blk)
				log.Info("BlockCache::asyncUploadPoller : Async Upload Success, Moved block from the OWBL to SBL blk idx : %d, file: %s", blk.idx, blk.file.Name)
			}
			log.Info("BlockCache::asyncUploadePoller : [took : %s] Async Poll to complete blk idx : %d, file: %s", time.Since(cow).String(), blk.idx, blk.file.Name)
			noOfAsyncPolls--
		}

		totalUsage = bp.getTotalMemUsage()
		usage = ((bp.onTheWireBlksLst.Len() * 100) / bp.maxBlocks)
		log.Info("BlockCache::asyncUploadPoller : [END] [took : %s]Mem usage: %d, Onwire blks Mem Usage: %d", time.Since(now).String(), totalUsage, usage)
		bp.Unlock()
	}
}

func (bp *BufferPool) getBufferForBlock(blk *block) {
	log.Info("BlockCache::getBufferForBlock : blk idx : %d file : %s", blk.idx, blk.file.Name)
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
		if blk.buf != nil {
			return
		}
		// This is like a stopping the world operation where we need to wait for all the dirty blocks to finish the uploads,
		// then commit the file(i.e., doing a putblocklist) to retrieve the buffer back.
		// Secondary caching like disk would come very handy for minimizing this scenario.
		// We use writeback caching enabled by default, hence there may be multiple getBuffer requests for the same block. Hence serialization is important.
		// We use channel to allow only one flush per file, and broadcast the result to all other requests by closing the channel
		log.Info("BlockCache::getBufferForBlock : Doing Stopping the world operation for blk idx : %d, file : %s", blk.idx, blk.file.Name)
		if !blk.requestingBufferFlag {
			blk.requestingBufferFlag = true
			blk.requestingBuffer = make(chan struct{})
			bPool.Unlock()
			blk.Unlock()
			err := syncBuffersForFile(blk.file)
			if err != nil {
				panic(fmt.Sprintf("BlockCache::getBufferForBlock : Stopping the world op failed block idx : %d, file : %s", blk.idx, blk.file.Name))
			}
			log.Info("BlockCache::getBufferForBlock : Stopping the world operation Success for blk idx : %d, file : %s", blk.idx, blk.file.Name)
			blk.Lock()
			bPool.Lock()
			if blk.buf == nil {
				blk.buf = bPool.getBuffer(false)
				bPool.addSyncedBlockToLRU(blk)
			}
			// Broadcast the results to all other requests waiting for getting the buffer for the same block.
			close(blk.requestingBuffer)
			blk.requestingBufferFlag = false
		} else {
			// There is some other request that came before and doing flush operation.
			// Hence wait for its result, without blocking.
			bPool.Unlock()
			blk.Unlock()
			<-blk.requestingBuffer
			blk.Lock()
			bPool.Lock()
			return
		}
	}

	// Check the memory usage of synced blocks
	SBusage := ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
	if SBusage > 60 {
		select {
		case bp.wakeUpBufferReclaimation <- struct{}{}:
		default:
		}
	}

	// Check the memory of the local blocks
	LBusage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
	if LBusage > 20 {
		select {
		case bp.wakeUpAsyncUploadScheduler <- struct{}{}:
		default:
		}
	}

	// Check the memory of the ongoingAsyncupload blocks
	OUBusage := ((bp.onTheWireBlksLst.Len() * 100) / bp.maxBlocks)
	if OUBusage > 20 {
		select {
		case bp.wakeUpAsyncUploadPoller <- struct{}{}:
		default:
		}
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
		//clear the state of the block
		blk.cancelOngolingAsyncDownload()
		blk.downloadDone = make(chan error, 1)
		close(blk.downloadDone)
		blk.cancelOngoingAsyncUpload()
		blk.uploadDone = make(chan error, 1)
		close(blk.uploadDone)
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
	file.changed = true
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
				blk.Lock()
				bPool.getBufferForBlock(blk)
				blk.Unlock()
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
	_, err := downloader(blk, syncRequest)
	bPool.updateRecentnessOfBlk <- blk
	return blk, err
}

func (bp *BufferPool) scheduleAsyncUploadsForFile(file *File) (fileChanged bool) {
	for _, blk := range file.blockList {
		// To make the sync upload faster, first we schedule all the requests as async
		// Then status would be checked using sync requests.
		blkstate, _ := uploader(blk, asyncRequest)
		if blkstate != committedBlock {
			fileChanged = true
		}
	}
	bp.Lock()
	defer bp.Unlock()
	for _, blk := range file.blockList {
		bp.moveBlkFromLBLtoOWBL(blk)
	}
	return
}

func (bp *BufferPool) updateLRUafterUploadSuccess(file *File) {
	bp.Lock()
	defer bp.Unlock()
	for _, blk := range file.blockList {
		bp.moveBlkFromOWBLtoSBL(blk)
	}
}

// Write all the Modified buffers to Azure Storage and return whether file is modified or not.
func syncBuffersForFile(file *File) (err error) {
	log.Trace("BlockCache::syncBuffersForFile : starting to flush the file to Azure storage")
	file.Lock()
	file.flushOngoing = make(chan struct{}) // This prevents the cancellations of the block uploads when there are writes happening in parllel to the flush
	defer func() {
		close(file.flushOngoing)
		file.Unlock()
	}()
	if file.blkListState == blockListInvalid || file.blkListState == blockListNotRetrieved {
		return nil
	}
	if !file.changed {
		return nil
	}

	_ = bPool.scheduleAsyncUploadsForFile(file)
	// Wait for the uploads to finish
	for _, blk := range file.blockList {
		_, err = uploader(blk, syncRequest)
		if err != nil {
			panic(fmt.Sprintf("Upload doesn't happen for the block idx : %d, file : %s\n", blk.idx, blk.file.Name))
		}
	}
	bPool.updateLRUafterUploadSuccess(file)
	if err == nil {
		err = commitBuffersForFile(file)
		if err != nil {
			log.Err("BlockCache::syncBuffersForFile : Commiting buffers failed handle=%d, path=%s, err=%s", file.Name, err.Error())
		} else {
			file.changed = false
			log.Info("BlockCache::syncBuffersForFile : Commit buffers success")
		}

	} else {
		log.Err("BlockCache::syncBuffersForFile : Syncing buffers failed handle=%d, path=%s, err=%s", file.Name, err.Error())
	}
	return
}

func commitBuffersForFile(file *File) error {
	log.Trace("BlockCache::commitBuffersForFile : %s\n", file.Name)
	var blklist []string
	if file.size == 0 {
		// This occurs when user do O_TRUNC on open and then close without doing any writes.
		// Create an empty blob in storage
		// Todo: current implementaion hardcoded the file mode to 0777
		// this may fail to set the ACL's in ADLS if usr dont have permission
		_, err := bc.NextComponent().CreateFile(internal.CreateFileOptions{
			Name: file.Name,
			Mode: os.FileMode(0777),
		})
		if err != nil {
			log.Err("BlockCache::commitBuffersForFile : Failed to create an empty blob %s", file.Name)
			return err
		}
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
func releaseBuffersOfFile(f *File) {
	if len(f.blockList) > 0 {
		bPool.removeBlocksFromLRU(f.blockList)
	}
}
