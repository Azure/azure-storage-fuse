package block_cache_new

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Scheduler is responsible for downloading/uploading blobs from the Azure Storage.
// We mainly have 4types of requests.
// Sync Reads : When we get the read call from the user application. The block should be downloaded on priority.
// Async Reads: When blobfuse does readahead calls
// Sync Writes: When User application does a flush call. The block should be uploaded on priority.
// Async Writes:When blobfuse schedules a block to upload.

type requestType int

const (
	asyncRequest requestType = iota
	syncRequest
)

type task struct {
	ctx      context.Context
	taskDone chan<- struct{} // gets notified when the task completed fully.
	upload   bool            // Represents upload, !upload represents download
	blk      *block
}

// Create Worker Pool of fixed Size.
// Todo: know this fixed Size from the system spec.
type workerPool struct {
	workers                 int
	wg                      sync.WaitGroup
	close                   chan struct{}
	asyncStream, syncStream chan *task
}

func (wp *workerPool) destroyWorkerPool() {
	close(wp.close)
	wp.wg.Wait()
}
func createWorkerPool(size int) *workerPool {
	wp := &workerPool{
		workers:     size,
		close:       make(chan struct{}),
		syncStream:  make(chan *task, 2000),
		asyncStream: make(chan *task, 2000),
	}
	for i := 0; i < size; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	return wp
}
func (wp *workerPool) createTask(ctx context.Context, taskDone chan<- struct{}, upload bool, r requestType, blk *block) {
	t := &task{
		ctx:      ctx,
		taskDone: taskDone,
		upload:   upload,
		blk:      blk,
	}
	if r == syncRequest {
		wp.syncStream <- t
	} else {
		wp.asyncStream <- t
	}
}

func (wp *workerPool) worker(workerNo int) {
	log.Info("BlockCache::worker Starting worker %d", workerNo)
	defer wp.wg.Done()
	var t *task
	for {
		select {
		case t = <-wp.syncStream:
			performTask(t, workerNo, syncRequest)
		case t = <-wp.asyncStream:
			performTask(t, workerNo, asyncRequest)
		case <-wp.close:
			return
		}
	}
}

func performTask(t *task, workerNo int, r requestType) {
	if t.upload {
		doUpload(t, workerNo, r)
	} else {
		doDownload(t, workerNo, r)
	}
}

func doDownload(t *task, workerNo int, r requestType) {
	blk := t.blk
	log.Trace("BlockCache::doDownload : [sync:%d] Download Starting for blk idx: %d, file : %s", r, blk.idx, blk.file.Name)
	if blk.buf == nil {
		panic("BlockCache::doDownload : Something has seriously messed up While Reading")
	}
	if blk.id == zeroBlockId {
		log.Debug("BlockCache::doDownload : Reading a hole that was created by block cache")
		blk.downloadDone <- nil
		close(t.taskDone)
		return
	}
	sizeOfData := getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)

	_, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Name:   t.blk.file.Name,
		Offset: int64(uint64(blk.idx) * bc.blockSize),
		Data:   blk.buf.data[:sizeOfData],
	})

	if err == nil && !errors.Is(t.ctx.Err(), context.Canceled) {
		log.Debug("BlockCache::doDownload : Download Success for blk idx: %d, file : %s", blk.idx, blk.file.Name)
		blk.downloadDone <- nil
	} else if err == nil {
		log.Debug("BlockCache::doDownload : Download Success but context canceled blk idx: %d, file : %s", blk.idx, blk.file.Name)
		blk.downloadDone <- t.ctx.Err()
	} else {
		log.Err("BlockCache::doDownload : Download failed blk idx: %d, file : %s, err : %s", blk.idx, blk.file.Name, err.Error())
		blk.downloadDone <- err
	}
	close(t.taskDone)
}

func doUpload(t *task, workerNo int, r requestType) {
	blk := t.blk
	blk.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	if blk.buf == nil {
		panic("BlockCache::doUpload : messed up")
	}
	log.Trace("BlockCache::doUpload : [sync:%d] Upload Starting for blk idx: %d, file : %s", r, blk.idx, blk.file.Name)
	blkSize := getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)
	if blkSize <= 0 {
		// There has been a truncate call came to shrink the filesize.
		// No need for uploading this block
		log.Err("BlockCache::doUpload : Not uploading the block as blocklist got contracted[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)
		blk.uploadDone <- errors.New("BlockList got contracted")
	} else {
		err := bc.NextComponent().StageData(
			internal.StageDataOptions{
				Ctx:  t.ctx,
				Name: t.blk.file.Name,
				Id:   blk.id,
				Data: blk.buf.data[:blkSize],
			},
		)
		if err == nil && !errors.Is(t.ctx.Err(), context.Canceled) {
			log.Debug("BlockCache::doUpload : Upload Success for blk idx: %d, file : %s", blk.idx, blk.file.Name)
			blk.uploadDone <- nil
		} else if err == nil {
			log.Debug("BlockCache::doUpload : Upload Success but context canceled for blk idx: %d, file : %s", blk.idx, blk.file.Name)
			blk.uploadDone <- t.ctx.Err()
		} else {
			log.Err("BlockCache::doUpload : Upload failed blk idx: %d, file : %s, err : %s", blk.idx, blk.file.Name, err.Error())
			blk.uploadDone <- err
		}
		if r == asyncRequest {
			//todo p1: generally we should push the blks for the async requests that were scheduler from the asyn scheduler.
			// but we also schedule async uploads while flushing the file, at that time we should not push the blk in the below stream
			bPool.uploadCompletedStream <- blk
		}
	}
	close(t.taskDone)
}
