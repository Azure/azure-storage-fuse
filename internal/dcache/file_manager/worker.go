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
	t := &task{
		file:      file,
		chunk:     chunk,
		get_chunk: get_chunk,
	}
	fileIOMgr.wp.tasks <- t
}

func (wp *workerPool) readChunk(task *task) {
	log.Debug("DistributedCache::readChunk: Reading chunk idx: %d, chunk Len: %d, file: %s",
		task.chunk.Idx, task.chunk.Len, task.file.FileMetadata.Filename)

	// For read chunk, buffer must not be pre-allocated, ReadMV() returns the buffer.
	// buffer is pre-allocated only when reading the chunk from the Local RV which would be decided after the ReadMV
	// call from the replication manager.
	common.Assert(task.chunk.IsBufExternal)
	common.Assert(task.chunk.Buf == nil, len(task.chunk.Buf))

	// Read From the Dcache.
	readMVReq := &rm.ReadMvRequest{
		FileID:         task.file.FileMetadata.FileID,
		MvName:         getMVForChunk(task.chunk, task.file.FileMetadata),
		ChunkIndex:     task.chunk.Idx,
		OffsetInChunk:  0,
		Length:         task.chunk.Len,
		ChunkSizeInMiB: task.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
	}

	readMVresp, err := rm.ReadMV(readMVReq)

	if err == nil {
		// ReadMV() must read all that we asked for.
		common.Assert(readMVresp.Data != nil)
		common.Assert(len(readMVresp.Data) == int(task.chunk.Len))

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

		return
	}

	log.Err("DistrubuteCache[FM]::readChunk: Reading chunk from Dcache failed, chnk idx: %d, file: %s: %v",
		task.chunk.Idx, task.file.FileMetadata.Filename, err)

	task.chunk.Err <- err
}

func (wp *workerPool) writeChunk(task *task) {
	log.Debug("DistributedCache::writeChunk: Writing chunk idx: %d, file: %s",
		task.chunk.Idx, task.file.FileMetadata.Filename)

	// Only dirty StagedChunk must be written.
	common.Assert(task.chunk.Dirty.Load())

	writeMVReq := &rm.WriteMvRequest{
		FileID:         task.file.FileMetadata.FileID,
		MvName:         getMVForChunk(task.chunk, task.file.FileMetadata),
		ChunkIndex:     task.chunk.Idx,
		Data:           task.chunk.Buf[:task.chunk.Len],
		ChunkSizeInMiB: task.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
		IsLastChunk:    task.chunk.Len != int64(len(task.chunk.Buf)),
	}

	// Call WriteMV method for writing the chunk.
	_, err := rm.WriteMV(writeMVReq)
	if err == nil {
		// WriteMV completed successfully, staged chunk is now no more dirty.
		common.Assert(task.chunk.Dirty.Load())
		task.chunk.Dirty.Store(false)
		close(task.chunk.Err)
		return
	}

	log.Err("DistrubuteCache[FM]::WriteChunk: Writing chunk to DCache failed, chnk idx: %d, file: %s: %v",
		task.chunk.Idx, task.file.FileMetadata.Filename, err)

	task.chunk.Err <- err
}
