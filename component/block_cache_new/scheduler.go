package block_cache_new

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Scheduler is responsible for downloading/uploading blobs from the Azure Storage.
// We mainly have 4types of requests.
// Sync Reads : When we get the read call from the user application. The block should be downloaded on priority.
// Async Reads: When blobfuse does readahead calls
// Sync Writes: When User application does a flush call. The block should be uploaded on priority.
// Async Writes:When blobfuse schedules a block to upload.

type syncType bool

var Sync syncType = true
var Async syncType = false

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
		syncStream:  make(chan *task, 500),
		asyncStream: make(chan *task, 500),
	}
	for i := 0; i < size; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	return wp
}
func (wp *workerPool) createTask(ctx context.Context, taskDone chan<- struct{}, upload bool, syncRequest bool, blk *block) {
	t := &task{
		ctx:      ctx,
		taskDone: taskDone,
		upload:   upload,
		blk:      blk,
	}
	if syncRequest {
		wp.syncStream <- t
	} else {
		wp.asyncStream <- t
	}
}

func (wp *workerPool) worker(workerNo int) {
	defer wp.wg.Done()
	var t *task
	for {
		select {
		case t = <-wp.syncStream:
			performTask(t, workerNo, true)
		case t = <-wp.asyncStream:
			performTask(t, workerNo, false)
		case <-wp.close:
			return
		}
	}
}

func performTask(t *task, workerNo int, sync bool) {
	if t.upload {
		doUpload(t, workerNo, sync)
	} else {
		doDownload(t, workerNo, sync)
	}
}

func doDownload(t *task, workerNo int, sync bool) {
	blk := t.blk
	if blk.buf == nil {
		blk.buf = bPool.getBuffer()
		switch blk.state {
		case localBlock:
			// This case occurs when we get read call on sparse local Blocks which are not even put on the wire.
			close(blk.downloadDone)
			return
		case uncommitedBlock:
			// This case occurs when we clear the uncommited block from the cache.
			// generally the block should be committed otherwise old data will be served.
			// Todo: Handle this case.
			// We don't hit here yet as we dont invalidate cache entries for local and uncommited blocks
			//return errors.New("todo : read for uncommited block which was removed from the cache")
		}
		logy.Write([]byte(fmt.Sprintf("BlockCache::doDownload : Download Scheduled for block[sync: %t], path=%s, blk Idx = %d, worker No = %d\n", sync, t.blk.file.Name, t.blk.idx, workerNo)))
		dataRead, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Name:   t.blk.file.Name,
			Offset: int64(blk.idx * BlockSize),
			Data:   blk.buf.data[:getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)],
		})
		if err == nil {
			logy.Write([]byte(fmt.Sprintf("BlockCache::doDownload : Download Completed for block[sync: %t], path=%s, blk Idx = %d, worker No = %d\n", sync, t.blk.file.Name, t.blk.idx, workerNo)))
			blk.buf.dataSize = int64(dataRead)
			blk.state = committedBlock
			close(blk.downloadDone)
		} else {
			blk.buf = nil
			logy.Write([]byte(fmt.Sprintf("BlockCache::doDownload : Download failed for block[sync: %t], path=%s, blk Idx = %d, worker No = %d\n", sync, t.blk.file.Name, t.blk.idx, workerNo)))
			blk.downloadDone <- err
		}
	}
}

func doUpload(t *task, workerNo int, sync bool) {
	blk := t.blk
	blk.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	if blk.buf == nil {
		panic("Something has seriously messed up")
	}
	logy.Write([]byte(fmt.Sprintf("BlockCache::doUpload : Upload Scheduled for block[sync: %t], path=%s, blk Idx = %d, worker No = %d\n", sync, t.blk.file.Name, t.blk.idx, workerNo)))

	err := bc.NextComponent().StageData(
		internal.StageDataOptions{
			Ctx:  t.ctx,
			Name: t.blk.file.Name,
			Id:   blk.id,
			Data: blk.buf.data[:getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)],
		},
	)
	if err == nil && !errors.Is(t.ctx.Err(), context.Canceled) {
		blk.uploadDone <- nil
	} else if err == nil {
		blk.uploadDone <- t.ctx.Err()
	} else {
		blk.uploadDone <- err
	}
	blk.uploadInProgress <- struct{}{}
	close(t.taskDone)
}
