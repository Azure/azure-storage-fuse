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
	"fmt"
	"sync"

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
			if task.get_chunk {
				wp.readChunk(task)
			} else {
				wp.writeChunk(task)
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
	log.Debug("DistributedCache::readChunk: Reading chunkIdx: %d, chunk Offset: %d, chunk Len: %d, refcount: %d, file: %s",
		task.chunk.Idx, task.chunk.Offset, task.chunk.Len, task.chunk.RefCount.Load(), task.file.FileMetadata.Filename)

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

	// Drop the download reference.
	task.file.releaseChunk(task.chunk)

	log.Err("DistrubuteCache[FM]::readChunk: Reading chunk from Dcache failed, chunkIdx: %d, offset: %d, length: %d, file: %s: %v",
		readMVReq.ChunkIndex, readMVReq.OffsetInChunk, readMVReq.Length, task.file.FileMetadata.Filename, err)

	task.chunk.Err <- err
}

func (wp *workerPool) writeChunk(task *task) {
	log.Debug("DistributedCache::writeChunk: Writing chunk chunkIdx: %d, file: %s",
		task.chunk.Idx, task.file.FileMetadata.Filename)

	// Only dirty StagedChunk must be written.
	common.Assert(task.chunk.Dirty.Load())
	// We always write full chunks (except for the last chunk, but that also starts at offset 0).
	common.Assert(task.chunk.Offset == 0, task.chunk.Idx, task.file.FileMetadata.Filename, task.chunk.Offset)
	common.Assert(task.chunk.Len > 0 && task.chunk.Len <= int64(len(task.chunk.Buf)),
		task.chunk.Idx, task.file.FileMetadata.Filename, task.chunk.Len, len(task.chunk.Buf))

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
