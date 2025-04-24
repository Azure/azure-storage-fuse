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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager/models"
)

type task struct {
	chunk      *models.Chunk
	fileLayout models.FileLayout
	get_chunk  bool
}

type workerPool struct {
	workers int
	wg      sync.WaitGroup
	close   chan struct{}
	tasks   chan *task
}

func NewWorkerPool(workers int) *workerPool {
	return &workerPool{
		workers: workers,
		close:   make(chan struct{}),
		tasks:   make(chan *task, workers*2),
	}
}

func (wp *workerPool) startWorkerPool() {
	log.Info("DistributedCache[FM]::startWorkerPool : Starting worker Pool, num workers : %d", wp.workers)
	wp.wg.Add(wp.workers)
	for range wp.workers {
		go wp.worker()
	}
}

func (wp *workerPool) endWorkerPool() {
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

func (wp *workerPool) readChunk(task *task) {
	var err error
	//Call MvRead method for reading the chunk.
	// err = rm.MVRead()
	if err == nil {
		close(task.chunk.Err)
		return
	}
	task.chunk.Err <- err
}

func (wp *workerPool) writeChunk(task *task) {
	var err error
	//Call MvWrite method for reading the chunk.
	// err = rm.MVWrite()
	if err == nil {
		close(task.chunk.Err)
		return
	}
	task.chunk.Err <- err
}
