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

type requestType int

const (
	syncRequest requestType = iota
	asyncRequest
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
		syncStream:  make(chan *task, 500),
		asyncStream: make(chan *task, 500),
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
	if blk.buf == nil {
		panic("Something has seriously messed up While Reading")
	}
	sizeOfData := getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)
	logy.Write([]byte(fmt.Sprintf("BlockCache::doDownload : Download Scheduled for block[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
	_, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Name:   t.blk.file.Name,
		Offset: int64(blk.idx * BlockSize),
		Data:   blk.buf.data[:sizeOfData],
	})
	logy.Write([]byte(fmt.Sprintf("BlockCache::doDownload : Download Complete for block[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
	if err == nil && !errors.Is(t.ctx.Err(), context.Canceled) {
		blk.downloadDone <- nil
	} else if err == nil {
		blk.downloadDone <- t.ctx.Err()
	} else {
		blk.downloadDone <- err
	}
	close(t.taskDone)
}

func doUpload(t *task, workerNo int, r requestType) {
	blk := t.blk
	blk.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	if blk.buf == nil {
		logy.Write([]byte(fmt.Sprintf("BlockCache::doUpload : this is the work of async stuff[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
		panic("messed up")
	}
	logy.Write([]byte(fmt.Sprintf("BlockCache::doUpload : Upload Scheduled for block[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
	blkSize := getBlockSize(atomic.LoadInt64(&t.blk.file.size), blk.idx)
	if blkSize <= 0 {
		// There has been a truncate call came to shrink the filesize.
		// No need for uploading this block
		logy.Write([]byte(fmt.Sprintf("BlockCache::doUpload : Not uploading the block as blocklist got contracted[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
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
		logy.Write([]byte(fmt.Sprintf("BlockCache::doUpload : Upload Complete for block[sync: %d], path=%s, blk Idx = %d, worker No = %d\n", r, t.blk.file.Name, t.blk.idx, workerNo)))
		if err == nil && !errors.Is(t.ctx.Err(), context.Canceled) {
			blk.uploadDone <- nil
		} else if err == nil {
			blk.uploadDone <- t.ctx.Err()
		} else {
			blk.uploadDone <- err
		}
	}
	close(t.taskDone)
}
