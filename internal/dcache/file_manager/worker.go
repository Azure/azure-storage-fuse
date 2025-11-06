/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package filemanager

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
)

//go:generate $ASSERT_REMOVER $GOFILE

type task struct {
	file      *DcacheFile
	chunk     *StagedChunk
	get_chunk bool
}

type workerPool struct {
	workers int
	wg      sync.WaitGroup
	close   chan struct{}
	tasks   chan *task
	busyCnt atomic.Int64
}

// uploadTracker keeps track of the uploads scheduled, in-progress and last completed time.
// Used for debugging slow throughput issues which could be due to:
// - Application not writing fast enough (lastScheduledAt is old).
// - Uploads not completing fast enough (lastCompletedAt is old).
//
// TODO: This can be removed once we are satisfied that upload performance is good.

type uploadTracker struct {
	slowGapThresh     time.Duration // Gap more than this between schedule/uploads is considered slow.
	scheduledUploads  atomic.Int64  // Number of uploads scheduled but not yet started.
	uploadsInProgress atomic.Int64  // Number of uploads in progress.
	cumScheduled      atomic.Int64  // Cumulative number of uploads scheduled.
	cumCompleted      atomic.Int64  // Cumulative number of uploads completed.
	slowScheduled     atomic.Int64  // Number of uploads which were scheduled slow (after more gap than usual).
	slowCompleted     atomic.Int64  // Number of uploads which completed slow (after more gap than usual).
	firstScheduledAt  atomic.Int64  // Timestamp (in unix nano seconds) when the first upload to this file was scheduled.
	lastScheduledAt   atomic.Int64  // Timestamp (in unix nano seconds) when last upload was scheduled.
	lastCompletedAt   atomic.Int64  // Timestamp (in unix nano seconds) when last upload completed.
}

func NewWorkerPool(workers int) *workerPool {
	common.Assert(workers > 0)

	// Create the worker pool.
	wp := &workerPool{
		workers: workers,
		close:   make(chan struct{}),
		tasks:   make(chan *task, workers*2),
	}

	// Start the workers.
	log.Info("DistributedCache[FM]::startWorkerPool: Starting worker Pool, num workers: %d", wp.workers)

	wp.wg.Add(wp.workers)
	for range wp.workers {
		go wp.worker()
	}
	return wp
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
			busyCnt := wp.busyCnt.Add(1)
			common.Assert(busyCnt <= int64(wp.workers), busyCnt, wp.workers)
			if busyCnt == int64(wp.workers) {
				//
				// If this log shows up often, it means we need to increase the number of workers.
				// See workers in NewFileIOManager().
				//
				log.Warn("[SLOW] DistributedCache[FM]::worker: All %d workers are busy", wp.workers)
			}

			if task.get_chunk {
				wp.readChunk(task)
			} else {
				wp.writeChunk(task)
			}
			busyCnt = wp.busyCnt.Add(-1)
			common.Assert(busyCnt >= 0, busyCnt)
			if busyCnt == 0 {
				//
				// If this log shows up often, it means application is not writing or reading fast enough.
				//
				log.Warn("[SLOW] DistributedCache[FM]::worker: All %d workers are idle now", wp.workers)
			}
		case <-wp.close:
			return
		}
	}
}

func (wp *workerPool) queueWork(file *DcacheFile, chunk *StagedChunk, get_chunk bool) {
	// We must be called only after scheduling the chunk for transfer (upload/download)
	common.Assert(chunk.XferScheduled.Load() == true, chunk.Idx, get_chunk, file.FileMetadata.Filename)

	t := &task{
		file:      file,
		chunk:     chunk,
		get_chunk: get_chunk,
	}
	fileIOMgr.wp.tasks <- t
}

func (wp *workerPool) readChunk(task *task) {
	log.Debug("DistributedCache::readChunk: [busyCnt: %d] Reading chunkIdx: %d, chunk Offset: %d, chunk Len: %d, refcount: %d, file: %s",
		wp.busyCnt.Load(), task.chunk.Idx, task.chunk.Offset, task.chunk.Len,
		task.chunk.RefCount.Load(), task.file.FileMetadata.Filename)

	// For read chunk, buffer must not be pre-allocated, ReadMV() returns the buffer.
	// buffer is pre-allocated only when reading the chunk from the Local RV which would be decided after the ReadMV
	// call from the replication manager.
	common.Assert(task.chunk.IsBufExternal)
	common.Assert(task.chunk.Buf == nil, len(task.chunk.Buf))
	common.Assert(task.file.FileMetadata.FileLayout.ChunkSize%common.MbToBytes == 0,
		task.file.FileMetadata.FileLayout.ChunkSize)

	// Read From the Dcache.
	readMVReq := &rm.ReadMvRequest{
		FileID:         task.file.FileMetadata.FileID,
		MvName:         getMVForChunk(task.chunk, task.file.FileMetadata),
		ChunkIndex:     task.chunk.Idx,
		OffsetInChunk:  task.chunk.Offset,
		Length:         task.chunk.Len,
		ChunkSizeInMiB: task.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
	}

	common.Assert(readMVReq.ChunkSizeInMiB > 0)

	readMVresp, err := rm.ReadMV(readMVReq)

	if err == nil {
		log.Debug("DistrubuteCache[FM]::readChunk: ReadMV completed, chunkIdx: %d, offset: %d, length: %d, file: %s, err: %v",
			task.chunk.Idx, task.chunk.Offset, task.chunk.Len, task.file.FileMetadata.Filename, err)

		// ReadMV() must read all that we asked for.
		common.Assert(readMVresp.Data != nil)
		common.Assert(len(readMVresp.Data) == int(task.chunk.Len))
		// We must come here only for chunks scheduled for transfer (download).
		common.Assert(task.chunk.XferScheduled.Load() == true, task.chunk.Idx, task.file.FileMetadata.Filename)

		//
		// ReadMV completed successfully, staged chunk is now up-to-date.
		// We should copy data to user buffer only from up-to-date staged chunks.
		//
		common.Assert(!task.chunk.UpToDate.Load())

		task.chunk.UpToDate.Store(true)
		task.chunk.Buf = readMVresp.Data
		//
		// While reading from the Local RV, The RPC handler allocate the buffer from the BufferPool which must be
		// released by the file_manager after its use.
		//
		task.chunk.IsBufExternal = readMVresp.IsBufExternal

		// Close the Err channel to indicate "no error".
		close(task.chunk.Err)

		//
		// Drop the download reference after IsBufExternal is correctly set.
		// This is important as this may be the last reference causing releaseChunk() to free the chunk,
		// and we must duly free the chunk buffer if IsBufExternal is false, else it would leak.
		//
		task.file.releaseChunk(task.chunk)

		return
	}

	//
	// If the chunk is not found in DCache (ENOENT), and we have the backing Azure handle, try reading it
	// from Azure.
	//
	if errors.Is(err, syscall.ENOENT) && task.file.AzureHandle != nil {
		// Try reading directly from Azure for unqualified opens.
		fileOffset := (task.chunk.Idx * task.file.FileMetadata.FileLayout.ChunkSize) + task.chunk.Offset
		bytesRead, err1 := task.file.readChunkFromAzure(fileOffset, task.chunk)

		common.Assert((err1 != nil) || (len(task.chunk.Buf) == int(bytesRead)),
			len(task.chunk.Buf), bytesRead)

		if err1 == nil && bytesRead != task.chunk.Len {
			err1 = fmt.Errorf("DistributedCache::ReadFile: Azure read size mismatch, file: %s, expected: %d, got: %d",
				task.file.FileMetadata.Filename, task.chunk.Len, bytesRead)
			common.Assert(false, err1)
		}

		if err1 == nil {
			task.chunk.UpToDate.Store(true)
			// Close the Err channel to indicate "no error".
			close(task.chunk.Err)

			task.file.releaseChunk(task.chunk)
			return
		}
	}

	// Drop the download reference.
	task.file.releaseChunk(task.chunk)

	log.Err("DistrubuteCache[FM]::readChunk: Reading chunk from Dcache failed, chunkIdx: %d, offset: %d, length: %d, file: %s: %v",
		readMVReq.ChunkIndex, readMVReq.OffsetInChunk, readMVReq.Length, task.file.FileMetadata.Filename, err)

	task.chunk.Err <- err
}

func (wp *workerPool) writeChunk(task *task) {
	log.Debug("DistributedCache::writeChunk: [busyCnt: %d] Writing chunk chunkIdx: %d, file: %s",
		wp.busyCnt.Load(), task.chunk.Idx, task.file.FileMetadata.Filename)

	// Only dirty StagedChunk must be written.
	common.Assert(task.chunk.Dirty.Load())
	// We always write full chunks (except for the last chunk, but that also starts at offset 0).
	common.Assert(task.chunk.Offset == 0, task.chunk.Idx, task.file.FileMetadata.Filename, task.chunk.Offset)
	common.Assert(task.chunk.Len > 0 && task.chunk.Len <= int64(len(task.chunk.Buf)),
		task.chunk.Idx, task.file.FileMetadata.Filename, task.chunk.Len, len(task.chunk.Buf))

	// writeChunk() is called only after scheduling the chunk for upload.
	common.Assert(task.file.ut.scheduledUploads.Load() > 0)
	task.file.ut.scheduledUploads.Add(-1)
	task.file.ut.uploadsInProgress.Add(1)

	writeMVReq := &rm.WriteMvRequest{
		FileID:         task.file.FileMetadata.FileID,
		MvName:         getMVForChunk(task.chunk, task.file.FileMetadata),
		ChunkIndex:     task.chunk.Idx,
		Data:           task.chunk.Buf[:task.chunk.Len],
		ChunkSizeInMiB: task.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
		IsLastChunk:    task.chunk.Len != int64(len(task.chunk.Buf)),
	}

	// Notify contiguity tracker before we issue upload for this chunk.
	task.file.CT.OnUploadStart(task.chunk.Idx)

	// Call WriteMV method for writing the chunk.
	_, err := rm.WriteMV(writeMVReq)

	common.Assert(task.file.ut.uploadsInProgress.Load() > 0)
	task.file.ut.uploadsInProgress.Add(-1)
	task.file.ut.cumCompleted.Add(1)

	if task.file.ut.lastCompletedAt.Load() != 0 {
		compGap := time.Since(time.Unix(0, task.file.ut.lastCompletedAt.Load()))
		if compGap > task.file.ut.slowGapThresh {
			task.file.ut.slowCompleted.Add(1)
			if compGap > task.file.ut.slowGapThresh*2 {
				log.Warn("[SLOW] DistributedCache::writeChunk: task.file: %s, chunkIdx: %d, compGap: %s, slowCompleted: %d (of %d in total %s)",
					task.file.FileMetadata.Filename, task.chunk.Idx, compGap,
					task.file.ut.slowCompleted.Load(), task.file.ut.cumCompleted.Load(),
					time.Since(time.Unix(0, task.file.ut.firstScheduledAt.Load())))
			}
		}
	}
	task.file.ut.lastCompletedAt.Store(time.Now().UnixNano())

	if err == nil {
		// We must come here only for chunks scheduled for transfer (upload).
		common.Assert(task.chunk.XferScheduled.Load() == true, task.chunk.Idx, task.file.FileMetadata.Filename)

		// WriteMV completed successfully, staged chunk is now no more dirty.
		common.Assert(task.chunk.Dirty.Load())
		task.chunk.Dirty.Store(false)

		// Convey success to anyone waiting for the upload to complete.
		close(task.chunk.Err)

		// The chunk is uploaded to DCache, we can release it now.
		log.Debug("DistributedCache::writeChunk: completed for file: %s, chunkIdx: %d, chunk.Len: %d, refcount: %d",
			task.file.FileMetadata.Filename, task.chunk.Idx, task.chunk.Len, task.chunk.RefCount.Load())

		// Notify contiguity tracker of this chunk's successful upload.
		task.file.CT.OnSuccessfulUpload(task.chunk.Idx)

		task.file.removeChunk(task.chunk.Idx)
		return
	}

	err = fmt.Errorf("DistrubuteCache[FM]::WriteChunk: Writing chunk to DCache failed, chunkIdx: %d, file: %s: %v",
		task.chunk.Idx, task.file.FileMetadata.Filename, err)
	log.Err("%v", err)

	//
	// Even though the upload failed, and thus the chunk is still dirty, we mark it not-dirty as removeChunk()
	// asserts for that.
	//
	common.Assert(task.chunk.Dirty.Load())
	task.chunk.Dirty.Store(false)

	// This will drop the reference held for upload.
	task.file.removeChunk(task.chunk.Idx)

	//
	// Set the write error on the file so that subsequent writes fail fast and also we want to convey the failure
	// to the user on file close.
	//
	task.file.setWriteError(err)

	task.chunk.Err <- err
}
