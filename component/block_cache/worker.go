package block_cache

import (
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type task struct {
	block              *block
	bufDesc            *bufferDescriptor
	download           bool
	sync               bool
	signalOnCompletion chan<- struct{}
}

type workerPool struct {
	workers int
	wg      sync.WaitGroup
	close   chan struct{}
	tasks   chan *task
}

var wp *workerPool

func NewWorkerPool(workers int) {
	// Create the worker pool.
	wp = &workerPool{
		workers: workers,
		close:   make(chan struct{}),
		tasks:   make(chan *task, workers*2),
	}

	// Start the workers.
	log.Info("BlockCache::startWorkerPool: Starting worker Pool, num workers: %d", wp.workers)

	wp.wg.Add(wp.workers)
	for range wp.workers {
		go wp.worker()
	}
}

func (wp *workerPool) destroyWorkerPool() {
	close(wp.close)
	wp.wg.Wait()
}

func (wp *workerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case task := <-wp.tasks:
			if task.download {
				wp.downloadBlock(task)
			} else {
				wp.uploadBlock(task)
			}
		case <-wp.close:
			return
		}
	}
}

func (wp *workerPool) queueWork(block *block, bufDesc *bufferDescriptor, download bool, signalOnCompletion chan<- struct{}, sync bool) {
	t := &task{
		block:              block,
		bufDesc:            bufDesc,
		download:           download,
		signalOnCompletion: signalOnCompletion,
		sync:               sync,
	}
	wp.tasks <- t
}

func (wp *workerPool) downloadBlock(task *task) {
	var err error
	// time.Sleep(10 * time.Millisecond) // Simulate download time
	// err := fmt.Errorf("simulated download error") // Simulate an error

	_, err = bc.NextComponent().ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   task.block.file.Name,
		Offset: int64(uint64(task.block.idx) * bc.blockSize),
		Data:   task.bufDesc.buf,
		Size:   atomic.LoadInt64(&task.block.file.size),
	})
	// time.Sleep(2 * time.Millisecond) // Simulate download time
	if err != nil {
		log.Err("BlockCache::downloadBlock: ReadInBuffer failed for file %s block idx %d: %v",
			task.block.file.Name, task.block.idx, err)

		task.bufDesc.downloadErr = err

		// Remove it from buffer table manager, so that it accepts no more new readers.
		btm.removeBufferDescriptor(task.bufDesc)
	} else {
		log.Debug("BlockCache::downloadBlock: Successfully downloaded block idx %d into buffer idx %d",
			task.block.idx, task.bufDesc.bufIdx)

		task.bufDesc.valid.Store(true)
	}

	task.bufDesc.contentLock.Unlock()

	if !task.sync {
		if ok := task.bufDesc.release(); ok {
			log.Debug("BlockCache::downloadBlock: Released bufferIdx: %d for blockIdx: %d back to free list after async download",
				task.bufDesc.bufIdx, task.block.idx)
		}
		log.Debug("BlockCache::downloadBlock: Async download completed for buffer idx %d for block idx %d, refCnt: %d",
			task.bufDesc.bufIdx, task.block.idx, task.bufDesc.refCnt.Load())
	}

	close(task.signalOnCompletion)
}

func (wp *workerPool) uploadBlock(task *task) {
}
