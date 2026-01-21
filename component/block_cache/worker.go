package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// task represents a unit of work for the worker pool.
//
// Tasks encapsulate either a block download or upload operation along with
// all necessary context and synchronization primitives.
//
// Fields:
//   - block: The block being downloaded or uploaded
//   - bufDesc: The buffer descriptor holding (or to hold) the block data
//   - download: true for download operations, false for upload operations
//   - sync: If true, caller waits for completion; if false, runs async
//   - signalOnCompletion: Channel to signal when operation completes
//
// Lifecycle:
//  1. Task created by scheduleUpload or scheduleDownload
//  2. Task queued to worker pool
//  3. Worker picks up task and executes it
//  4. Operation completes (success or failure)
//  5. signalOnCompletion channel is closed to notify waiter
type task struct {
	block              *block            // Block to download/upload
	bufDesc            *bufferDescriptor // Buffer for block data
	download           bool              // true=download, false=upload
	sync               bool              // true=synchronous, false=asynchronous
	signalOnCompletion chan<- struct{}   // Closed when operation completes
}

// workerPool manages a pool of goroutines for async I/O operations.
//
// Overview:
//
// The worker pool provides asynchronous execution of block download and upload
// operations, decoupling I/O from the main FUSE request threads.
//
// Key Characteristics:
//
//   - Fixed number of workers (configured via parallelism parameter)
//   - Buffered task channel allows queueing pending operations
//   - Workers run continuously until pool is destroyed
//   - Each worker handles both downloads and uploads
//
// Concurrency Benefits:
//
//   - Allows multiple concurrent uploads/downloads
//   - Prevents blocking FUSE threads on slow network operations
//   - Enables read-ahead and write-behind optimizations
//   - Limits resource usage via worker count
//
// Thread Safety:
//
// The worker pool is thread-safe. Multiple goroutines can queue tasks
// concurrently without synchronization.
type workerPool struct {
	workers int            // Number of worker goroutines
	wg      sync.WaitGroup // Tracks active workers for clean shutdown
	close   chan struct{}  // Closed to signal workers to exit
	tasks   chan *task     // Buffered channel of pending tasks
}

// wp is the global worker pool instance, initialized during Start().
var wp *workerPool

// NewWorkerPool creates and starts a worker pool with the specified number of workers.
//
// Parameters:
//   - workers: Number of worker goroutines to create
//
// This function is called during BlockCache.Start() to initialize the worker pool.
// Workers start immediately and wait for tasks to arrive on the tasks channel.
//
// The task channel is buffered (workers*2) to allow some queueing of pending
// operations without blocking the submitter.
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

// destroyWorkerPool shuts down the worker pool and waits for all workers to exit.
//
// This method:
//  1. Closes the close channel to signal workers to exit
//  2. Waits for all workers to finish their current tasks and exit
//
// Called during BlockCache.Stop() to clean up resources.
//
// Note: Pending tasks in the channel are abandoned. Callers should ensure
// all important tasks complete before destroying the pool.
func (wp *workerPool) destroyWorkerPool() {
	close(wp.close)
	wp.wg.Wait()
}

// worker is the main worker loop that processes tasks until pool is destroyed.
//
// This function runs as a goroutine, one per worker in the pool. It:
//
//  1. Waits for tasks on the tasks channel
//  2. Executes downloads or uploads as appropriate
//  3. Exits when the close channel is closed
//
// Workers run continuously, waiting for work. This approach minimizes
// goroutine creation overhead compared to spawning a goroutine per task.
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

// queueWork submits a task to the worker pool.
//
// Parameters:
//   - block: Block to operate on
//   - bufDesc: Buffer descriptor for the block
//   - download: true for download, false for upload
//   - signalOnCompletion: Channel to close when operation completes
//   - sync: true if caller will wait, false if fire-and-forget
//
// This method creates a task and queues it to the worker pool. A worker will
// pick up the task and execute it asynchronously.
//
// Blocking behavior:
//   - If task channel is full, this method blocks until space is available
//   - This provides backpressure when workers can't keep up with requests
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

// downloadBlock downloads a block from Azure Storage into a buffer.
//
// This method:
//  1. Calls the storage layer to read block data
//  2. On success: marks buffer as valid
//  3. On failure: stores error and removes buffer from table
//  4. Releases content lock to unblock readers
//  5. For async downloads: releases buffer reference
//  6. Signals completion via task.signalOnCompletion channel
//
// Parameters:
//   - task: Task describing the download operation
//
// Error Handling:
//
// Download errors are stored in bufDesc.downloadErr. The buffer is removed
// from the buffer table to prevent subsequent operations from using invalid data.
// Readers will see the error when they call ensureBufferValidForRead().
//
// Reference Counting:
//
//   - Download holds a reference during the operation
//   - Sync downloads: caller releases reference after waiting
//   - Async downloads: worker releases reference after completion
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
	if err != nil {
		log.Err("BlockCache::downloadBlock: ReadInBuffer failed for file %s block idx %d: %v",
			task.block.file.Name, task.block.idx, err)

		task.bufDesc.downloadErr = err

		// Remove it from buffer table manager, so that it accepts no more new readers.
		btm.removeBufferDescriptor(task.bufDesc, false /*strict*/)
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

// uploadBlock uploads a block from a buffer to Azure Storage.
//
// This method:
//  1. Generates a new block ID for the upload
//  2. Calls the storage layer to stage the block data
//  3. On success: marks buffer as clean, updates block state to uncommitedBlock
//  4. On failure: stores error in bufDesc and file error state
//  5. Resets write count on the block
//  6. For async uploads: removes buffer from table (cleanup)
//  7. Releases content lock to allow new writes
//  8. Signals completion via task.signalOnCompletion channel
//
// Parameters:
//   - task: Task describing the upload operation
//
// Block State Transitions:
//
//	localBlock -> uncommitedBlock (after successful upload)
//
// The block is in "uncommitted" state after StageData. It becomes "committed"
// later when the file is flushed and CommitData (PutBlockList) is called.
//
// Error Handling:
//
// Upload errors are stored in both bufDesc.uploadErr and file.err. This ensures
// both the buffer-level and file-level operations fail fast after an error.
//
// Reference Counting:
//
//   - Upload holds a reference during the operation
//   - Sync uploads: caller releases reference after waiting
//   - Async uploads: buffer is removed from table (releases table reference)
//
// Async Upload Cleanup:
//
// For async uploads, the worker removes the buffer from the table after upload
// completes. This frees cache space for other blocks. The caller doesn't wait,
// so we can't rely on the caller to release the buffer.
func (wp *workerPool) uploadBlock(task *task) {
	task.block.id = common.GetBlockID(common.BlockIDLength)

	err := bc.NextComponent().StageData(internal.StageDataOptions{
		Name: task.block.file.Name,
		Data: task.bufDesc.buf[:getBlockSize(atomic.LoadInt64(&task.block.file.size), task.block.idx)],
		Id:   task.block.id,
	})

	if err != nil {
		log.Err("BlockCache::getBlockIDList : Failed to write block for %v, ID: %v [%v]",
			task.block.file.Name, task.block.id, err)
		task.bufDesc.uploadErr = err
		task.block.file.err.Store(err.Error())
	} else {
		log.Debug("BlockCache::uploadBlock: Successfully uploaded block idx %d from buffer idx %d",
			task.block.idx, task.bufDesc.bufIdx)
		task.bufDesc.dirty.Store(false)
	}

	// Change the state of the block to uncommited, to reflect that it is uploaded but not yet committed.
	atomic.StoreInt32((*int32)(&task.block.state), int32(uncommitedBlock))
	// Reset the numWrites.
	task.block.numWrites.Store(0)

	if !task.sync {
		if ok := task.bufDesc.release(); ok {
			// This should not be released as we did not removed it from buffer table manager yet.
			err := fmt.Sprintf("BlockCache::uploadBlock: Released bufferIdx: %d for blockIdx: %d back to free list after async upload",
				task.bufDesc.bufIdx, task.block.idx)
			panic(err)
		}
		log.Debug("BlockCache::uploadBlock: Async upload completed for buffer idx %d for block idx %d, refCnt: %d",
			task.bufDesc.bufIdx, task.block.idx, task.bufDesc.refCnt.Load())

		ok1, ok2 := btm.removeBufferDescriptor(task.bufDesc, true /*strict*/)
		log.Debug("BlockCache::uploadBlock: Removed bufferIdx: %d for blockIdx: %d from buffer table manager after async upload, isRemovedFromBufMgr: %v, isReleasedToFreeList: %v",
			task.bufDesc.bufIdx, task.block.idx, ok1, ok2)
	}

	task.bufDesc.contentLock.Unlock()

	close(task.signalOnCompletion)
}
