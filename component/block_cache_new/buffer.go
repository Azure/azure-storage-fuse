package block_cache_new

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

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
	onTheWireBlksLst           *list.List               // List of blocks for which the uploads has scheduled.
	OWBCache                   map[*block]*list.Element // This is not an LRU but this map is used to cache the references to the elements in the onTheWireBlksLst as it is very frequent we delete from this lst.
	syncedBlksLst              *list.List               // LRU of Synced Blocks which are present in memory for all the open handles. This list can contain both commited and Uncommited blocks.
	SBCache                    map[*block]*list.Element
	wakeUpBufferReclaimation   chan struct{}
	wakeUpAsyncUploadScheduler chan struct{}
	wakeUpAsyncUploadPoller    chan struct{}
	uploadCompletedStream      chan *block

	maxBlocks int // Size of the each buffer in the bytes for this pool
}

func newBufferPool(memSize uint64) *BufferPool {
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
		uploadCompletedStream:      make(chan *block, 2000), //todo p1: this is put to a large value as flush call is also pushing its blocks into this stream

		maxBlocks: int(memSize / bc.blockSize),
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
		noOfBlksToFree := max(((usage-45)*bp.maxBlocks)/100, 0)
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

// We schedule based on the LRU of the writes. This works fine in the normal conditions.
// But when user writes on large number of files at the same time, then we may see so many context cancellations for uploads if the memory set is not enough.
// as the nature of writeback caching from the kernel is asynchronous. Hence memory set in the config should be increased(temporary fix)/ there should be some heuristic
// that should be checked before uploading. (maybe check how many writes happened to the block and only schedule the upload once the writes has happed to the entire block)
func (bp *BufferPool) asyncUploadScheduler() {
	for {
		<-bp.wakeUpAsyncUploadScheduler
		bp.Lock()
		now := time.Now()
		totalUsage := bp.getTotalMemUsage()

		usage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
		noOfAsyncUploads := max(((usage-25)*bp.maxBlocks)/100, 0)
		log.Info("BlockCache::asyncUploadScheduler : [START] Mem usage: %d, Synced blks Mem Usage: %d, needed %d async Uploads", totalUsage, usage, noOfAsyncUploads)
		// Schedule uploads on least recently used blocks
		currentBlk := bp.localBlksLst.Back()
		for currentBlk != nil && noOfAsyncUploads > 0 {
			blk := currentBlk.Value.(*block)
			currentBlk = currentBlk.Prev()
			cow := time.Now()
			r := asyncRequest
			r |= asyncUploadScheduler
			//todo : Schedule only when the ref cnt of the blk is zero, else there is a chance of cancelling the upload
			uploader(blk, r)
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

// Write throughput of the system depends on the following funcion. The lesser time it takes, more throughput we get.
// The time taken depends on the concurrency value set, If it set to very high value, then we may temporarily saturate the link but the latency of each
// upload increases so the following function takes more time decreasing the overall write throughput.
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

		// We want any async blocks to finish the uploads, We dont care which blocks will complete first.
		// Hence Listen on all Uploads simultaneosly.
		for range noOfAsyncPolls {
			select {
			case blk := <-bp.uploadCompletedStream:
				blk.Lock()
				err, ok := <-blk.uploadDone
				if ok {
					close(blk.uploadDone)
				}
				if ok && err == nil && blk.uploadCtx.Err() == nil {
					log.Info("BlockCache::asyncUploadPoller : Upload Success, Moved block from the OWBL to SBL blk idx : %d, file: %s", blk.idx, blk.file.Name)
					blk.state = uncommitedBlock
					bp.moveBlkFromOWBLtoSBL(blk)
				} else {
					// May be the upload is failed/ Context got cancelled as there may be write afterwards
					if err != nil {
						log.Err("BlockCache::asyncUploadPoller : Upload failed err : %s, blk idx : %d, file : %s", err.Error(), blk.idx, blk.file.Name)
					} else {
						// The status of the upload is consumed by the flush operation
						log.Info("BlockCache::asyncUploadPoller : Upload failed without err blk idx : %d, file : %s", blk.idx, blk.file.Name)
					}
					if blk.state == localBlock {
						bp.moveBlkFromOWBLtoLBL(blk)
					} else {
						bp.moveBlkFromOWBLtoSBL(blk)
					}
				}
				blk.Unlock()
			}
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

	// P1 Todo: The following wakeup calls can result in memory to go greater than 100% so keep a hard limit so that system to kill our process.
	// P2 Todo: The following threshold values set can be dynamically adjusted to increase the overall throughput of the system

	// Check the memory usage of synced blocks
	SBusage := ((bp.syncedBlksLst.Len() * 100) / bp.maxBlocks)
	if SBusage > 50 {
		select {
		case bp.wakeUpBufferReclaimation <- struct{}{}:
		default:
		}
	}

	// Check the memory of the local blocks
	LBusage := ((bp.localBlksLst.Len() * 100) / bp.maxBlocks)
	if LBusage > 30 {
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
		b.data = make([]byte, bc.blockSize)
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
		close(blk.forceCancelUpload)
		blk.cancelOngoingAsyncUpload()
		blk.uploadDone = make(chan error, 1)
		close(blk.uploadDone)
		blk.uploadCtx = context.Background()
		bp.pool.Put(blk.buf)
		blk.buf = nil
		blk.forceCancelUpload = make(chan struct{})
	}
}

// ************************************************************************
// The following are the functions describes on the buffer
// **************************************************************

func getBlockWithReadAhead(idx int, start int, file *File) (*block, error) {
	for i := 0; i <= 4; i++ {
		if i+start < int((uint64(atomic.LoadInt64(&file.size))+bc.blockSize-1)/bc.blockSize) {
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
	if r.isRequestSync() {
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

	var fileSize int64
	var dirtyBlock *block // When appending a block to blocklist, there may be a chance of reuploading the last block of prev blocklist by appending the zeros. hence change the state of it.
	var isBlockAppended bool = false
	file.Lock()
	fileSize = file.size
	file.changed = true
	lenOfBlkLst := len(file.blockList)
	if idx >= lenOfBlkLst {
		isBlockAppended = true
		// Update the state of the last block to local as null data may get's appended to it.
		if lenOfBlkLst > 0 && getBlockSize(fileSize, lenOfBlkLst-1) != int(bc.blockSize) {
			dirtyBlock = file.blockList[lenOfBlkLst-1]
		}

		// Create at least 1 block. i.e, create blocks in the range (len(blocklist), idx]
		for i := lenOfBlkLst; i <= idx; i++ {
			//id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, zeroBlockId, localBlock, file)
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
	}
	blk := file.blockList[idx]
	file.Unlock()

	if dirtyBlock != nil {
		// last block of the prev blocklist may need to be uploaded again, if it's entire block data was not present previously.
		err := changeStateOfBlockToLocal(dirtyBlock)
		if err != nil {
			log.Err("BlockCache::getBlockForWrite : failed to convert the last block to local, file path=%s, size = %d, err = %s", file.Name, file.size, err.Error())
			return nil, err
		}
	}
	if !isBlockAppended {
		_, err := downloader(blk, syncRequest)
		if err != nil {
			log.Err("BlockCache::getBlockForWrite : failed to download the data, file path=%s, size = %d, err = %s", file.Name, file.size, err.Error())
			return nil, err
		}
		bPool.updateRecentnessOfBlk <- blk
	}

	return blk, nil
}

func (bp *BufferPool) scheduleAsyncUploadsForFile(file *File) (fileChanged bool) {
	bp.Lock()
	for _, blk := range file.blockList {
		// Move the blocks to ongoing state to prevent the async uploader to reschedule
		bp.moveBlkFromLBLtoOWBL(blk)
	}
	bp.Unlock()
	for _, blk := range file.blockList {
		// To make the sync upload faster, first we schedule all the requests as async
		// Then status would be checked using sync requests.
		blkstate, _ := uploader(blk, asyncRequest)
		if blkstate != committedBlock {
			fileChanged = true
		}
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
			panic(fmt.Sprintf("Upload doesn't happen for the block idx : %d, file : %s, err: %s\n", blk.idx, blk.file.Name, err.Error()))
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
	err := bc.NextComponent().CommitData(internal.CommitDataOptions{Name: file.Name, List: blklist, BlockSize: uint64(bc.blockSize)})
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
