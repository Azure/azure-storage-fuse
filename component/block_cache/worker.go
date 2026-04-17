package block_cache

import (
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
	bc      *BlockCache    // Reference to parent BlockCache for I/O operations
}

// createWorkerPool creates and starts a worker pool with the specified number of workers.
//
// Parameters:
//   - workers: Number of worker goroutines to create
//
// This function is called during BlockCache.Start() to initialize the worker pool.
// Workers start immediately and wait for tasks to arrive on the tasks channel.
//
// The task channel is buffered (workers*2) to allow some queueing of pending
// operations without blocking the submitter.
func createWorkerPool(workers int, bc *BlockCache) *workerPool {
	// Create the worker pool.
	wp := &workerPool{
		workers: workers,
		close:   make(chan struct{}),
		tasks:   make(chan *task, workers*2),
		bc:      bc,
	}

	// Start the workers.
	log.Info("BlockCache::startWorkerPool: Starting worker Pool, num workers: %d", wp.workers)

	wp.wg.Add(wp.workers)
	for range wp.workers {
		go wp.worker()
	}

	return wp
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
func (wp *workerPool) destroy() {
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
				wp.downloadBlock(task, wp.bc)
			} else {
				wp.uploadBlock(task, wp.bc)
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
func (wp *workerPool) downloadBlock(task *task, bc *BlockCache) {
	var err error
	// time.Sleep(10 * time.Millisecond) // Simulate download time
	// err := fmt.Errorf("simulated download error") // Simulate an error

	block := task.block
	bufDesc := task.bufDesc

	_, err = bc.NextComponent().ReadInBuffer(&internal.ReadInBufferOptions{
		Path:   block.file.Name,
		Offset: int64(uint64(block.idx) * bc.blockSize),
		Data:   bufDesc.buf,
		Size:   atomic.LoadInt64(&block.file.size),
		Layout: block.file.layout,
	})
	if err != nil {
		log.Err("BlockCache::downloadBlock: ReadInBuffer failed for file %s block idx %d: %v",
			block.file.Name, block.idx, err)

		bufDesc.downloadErr = err
	} else {
		log.Debug("BlockCache::downloadBlock: Successfully downloaded blockIdx: %d into bufferIdx: %d, file: %s",
			block.idx, bufDesc.bufIdx, block.file.Name)
		bufDesc.valid.Store(true)
	}

	if !task.sync {
		if ok := bufDesc.release(bc.freeList); ok {
			log.Debug("BlockCache::downloadBlock: Released bufferIdx: %d for blockIdx: %d of file: %s back to free list after async download",
				bufDesc.bufIdx, block.idx, block.file.Name)
		}
		log.Debug("BlockCache::downloadBlock: Async download completed for bufferIdx %d for blockIdx %d, refCnt: %d, file: %s",
			bufDesc.bufIdx, block.idx, bufDesc.refCnt.Load(), block.file.Name)
	}

	bufDesc.contentLock.Unlock()

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
func (wp *workerPool) uploadBlock(task *task, bc *BlockCache) {
	block := task.block
	bufDesc := task.bufDesc

	block.id = common.GetBlockID(common.BlockIDLength)

	err := bc.NextComponent().StageData(internal.StageDataOptions{
		Name: block.file.Name,
		Data: bufDesc.buf[:getBlockSize(atomic.LoadInt64(&block.file.size), block.idx, int64(bc.blockSize))],
		Id:   block.id,
	})

	if err != nil {
		log.Err("BlockCache::getBlockIDList : Failed to write block for %v, ID: %v, file: %s [%v]",
			block.file.Name, block.id, block.file.Name, err)
		bufDesc.uploadErr = err
		block.file.err.Store(err.Error())
	} else {
		log.Debug("BlockCache::uploadBlock: Successfully uploaded blockIdx: %d from bufferIdx: %d, file: %s, sync: %t",
			block.idx, bufDesc.bufIdx, block.file.Name, task.sync)
		bufDesc.dirty.Store(false)
		// Change the state of the block to uncommitted, to reflect that it is uploaded but not yet committed.
		block.setState(uncommitedBlock)
	}

	// Reset the numWrites.
	block.numWrites.Store(0)

	bufDesc.contentLock.Unlock()

	if !task.sync {
		if bc.btm.removeBufferDescriptor(bufDesc, bc.freeList) {
			log.Debug("BlockCache::uploadBlock: Removed bufferIdx: %d for blockIdx: %d of file: %s from buffer table manager after async upload",
				bufDesc.bufIdx, block.idx, block.file.Name)
		} else {
			// release the buffer
			if ok := bufDesc.release(bc.freeList); ok {
				// This should not be released as we did not removed it from buffer table manager yet.
				log.Err("BlockCache::uploadBlock: Released bufferIdx: %d for blockIdx: %d of file: %s back to free list after async upload",
					bufDesc.bufIdx, block.idx, block.file.Name)
			}
		}
	}

	close(task.signalOnCompletion)
}
