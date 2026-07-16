/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package block_cache

import (
	"errors"
	"fmt"
	"sync"

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
	block              *block              // Block to download/upload
	bufDesc            *bufferDescriptor   // Buffer for block data
	download           bool                // true=download, false=upload
	sync               bool                // true=synchronous, false=asynchronous
	signalOnCompletion chan struct{}       // Closed when operation completes
	path               string              // Immutable storage path for this operation
	fileSize           int64               // File size snapshot for downloads
	blockID            string              // Block ID snapshot for uploads
	uploadSize         int                 // Validated number of bytes to upload
	fileGeneration     uint64              // File contents generation captured at queue time
	contentLease       *bufferContentLease // Exclusive descriptor content ownership
	err                error               // Task result, published before completion is signaled
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
	tasks   chan *task     // Buffered channel of pending tasks
	bc      *BlockCache    // Reference to parent BlockCache for I/O operations
	stop    sync.Once      // Ensures the task channel is closed exactly once
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
		tasks:   make(chan *task, workers*2),
		bc:      bc,
	}

	// Start the workers.
	log.Info("BlockCache::startWorkerPool: Starting worker Pool, num workers: %d", wp.workers)

	wp.wg.Add(wp.workers)
	for i := 0; i < wp.workers; i++ {
		go wp.worker()
	}

	return wp
}

// destroyWorkerPool shuts down the worker pool and waits for all workers to exit.
//
// This method:
//  1. Closes the task channel so no more work can be accepted
//  2. Drains all accepted tasks and waits for workers to exit
//
// Called during BlockCache.Stop() to clean up resources.
func (wp *workerPool) destroy() {
	wp.stop.Do(func() { close(wp.tasks) })
	wp.wg.Wait()
}

// worker is the main worker loop that processes tasks until pool is destroyed.
//
// This function runs as a goroutine, one per worker in the pool. It:
//
//  1. Waits for tasks on the tasks channel
//  2. Executes downloads or uploads as appropriate
//  3. Exits after the task channel is closed and drained
//
// Workers run continuously, waiting for work. This approach minimizes
// goroutine creation overhead compared to spawning a goroutine per task.
func (wp *workerPool) worker() {
	defer wp.wg.Done()
	for task := range wp.tasks {
		if task.download {
			wp.downloadBlock(task, wp.bc)
		} else {
			wp.uploadBlock(task, wp.bc)
		}
		wp.completeTask(task)
	}
}

// completeTask releases all ownership transferred to a worker task before
// signaling completion. Task implementations never unlock or release directly.
func (wp *workerPool) completeTask(task *task) {
	task.contentLease.release()
	if !task.sync {
		task.bufDesc.release(wp.bc.freeList)
	}
	task.block.file.generations.finish(task.fileGeneration)
	close(task.signalOnCompletion)
}

func (task *task) isCurrent() bool {
	return task.fileGeneration == task.block.file.generations.currentID()
}

func (task *task) mode() string {
	if task.sync {
		return "sync"
	}
	return "async"
}

// queueTask submits a fully initialized task to the worker pool.
//
// Blocking behavior:
//   - If task channel is full, this method blocks until space is available
//   - This provides backpressure when workers can't keep up with requests
func (wp *workerPool) queueTask(task *task) {
	wp.tasks <- task
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
	// time.Sleep(10 * time.Millisecond) // Simulate download time
	// err := fmt.Errorf("simulated download error") // Simulate an error

	block := task.block
	bufDesc := task.bufDesc
	log.Debug("BlockCache::downloadBlock: Starting %s download for file: %s, blockIdx: %d, bufferIdx: %d, offset: %d, size: %d",
		task.mode(), task.path, block.idx, bufDesc.bufIdx, int64(uint64(block.idx)*bc.blockSize), task.fileSize)

	var err error
	if !task.isCurrent() {
		task.err = errStaleTask
	} else {
		_, err = bc.NextComponent().ReadInBuffer(&internal.ReadInBufferOptions{
			Path:   task.path,
			Offset: int64(uint64(block.idx) * bc.blockSize),
			Data:   bufDesc.buf,
			Size:   task.fileSize,
		})
	}
	if task.err == nil && !task.isCurrent() {
		task.err = errStaleTask
	} else if task.err == nil && err != nil {
		task.err = err
		log.Err("BlockCache::downloadBlock: ReadInBuffer failed for file %s block idx %d: %v",
			block.file.Name, block.idx, err)

		bufDesc.downloadErr = err
		bc.btm.detachBufferDescriptor(bufDesc, bc.freeList)
	} else if task.err == nil {
		log.Debug("BlockCache::downloadBlock: Successfully downloaded blockIdx: %d into bufferIdx: %d, file: %s",
			block.idx, bufDesc.bufIdx, block.file.Name)
		bufDesc.valid.Store(true)
	}
	if errors.Is(task.err, errStaleTask) {
		bufDesc.downloadErr = task.err
		bc.btm.detachBufferDescriptor(bufDesc, bc.freeList)
	}

}

// uploadBlock uploads a block from a buffer to Azure Storage.
//
// This method:
//  1. Generates a new block ID for the upload
//  2. Calls the storage layer to stage the block data
//  3. On success: marks buffer as clean, updates block state to uncommitedBlock
//  4. On failure: stores error in bufDesc and file error state
//  5. Resets write count on the block
//  6. Releases content lock to allow new writes
//  7. Signals completion via task.signalOnCompletion channel
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
// Upload holds a task reference during the operation. The caller waits for
// completion and releases that reference.
func (wp *workerPool) uploadBlock(task *task, bc *BlockCache) {
	block := task.block
	bufDesc := task.bufDesc
	log.Debug("BlockCache::uploadBlock: Starting %s upload for file: %s, blockIdx: %d, bufferIdx: %d, size: %d, blockId: %s",
		task.mode(), task.path, block.idx, bufDesc.bufIdx, task.uploadSize, task.blockID)

	var err error
	if !task.isCurrent() {
		task.err = errStaleTask
	} else if task.uploadSize <= 0 || task.uploadSize > len(bufDesc.buf) {
		err = fmt.Errorf("invalid upload size %d for block %d of %s", task.uploadSize, block.idx, task.path)
	} else {
		err = bc.NextComponent().StageData(internal.StageDataOptions{
			Name: task.path,
			Data: bufDesc.buf[:task.uploadSize],
			Id:   task.blockID,
		})
	}

	if !task.isCurrent() {
		task.err = errStaleTask
	} else if err != nil {
		task.err = err
		log.Err("BlockCache::getBlockIDList : Failed to write block for %v, ID: %v, file: %s [%v]",
			task.path, task.blockID, task.path, err)
		bufDesc.uploadErr = err
		block.file.err.Store(&err)
	} else {
		log.Debug("BlockCache::uploadBlock: Successfully uploaded blockIdx: %d from bufferIdx: %d, file: %s, sync: %t",
			block.idx, bufDesc.bufIdx, task.path, task.sync)
		block.id = task.blockID
		bufDesc.dirty.Store(false)
		bufDesc.bytesWritten.Store(0)
		bufDesc.resetWriteCoverage()
		// Change the state of the block to uncommitted, to reflect that it is uploaded but not yet committed.
		block.setState(uncommitedBlock)
		block.numWrites.Store(0)
	}

}
