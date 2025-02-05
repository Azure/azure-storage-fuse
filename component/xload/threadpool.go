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

package xload

import (
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// ThreadPool is a group of workers that can be used to execute a task
type ThreadPool struct {
	// Number of workers running in this group
	worker uint32

	// Wait group to wait for all workers to finish
	waitGroup sync.WaitGroup

	// Channel to hold pending requests
	workItems chan *WorkItem

	// Reader method that will actually read the data
	callback func(*WorkItem) (int, error)
}

// NewThreadPool creates a new thread pool
func NewThreadPool(count uint32, callback func(*WorkItem) (int, error)) *ThreadPool {
	if count == 0 || callback == nil {
		return nil
	}

	return &ThreadPool{
		worker:    count,
		callback:  callback,
		workItems: make(chan *WorkItem, count*2),
	}
}

// Start all the workers and wait till they start receiving requests
func (threadPool *ThreadPool) Start() {
	for i := uint32(0); i < threadPool.worker; i++ {
		threadPool.waitGroup.Add(1)
		go threadPool.Do()
	}
}

// Stop all the workers threads
func (threadPool *ThreadPool) Stop() {
	close(threadPool.workItems)
	threadPool.waitGroup.Wait()
}

// Schedule the download of a block
func (threadPool *ThreadPool) Schedule(item *WorkItem) {
	threadPool.workItems <- item
}

// Do is the core task to be executed by each worker thread
func (threadPool *ThreadPool) Do() {
	defer threadPool.waitGroup.Done()

	// This thread will work only on both high and low priority channel
	for item := range threadPool.workItems {
		dataLength, err := threadPool.callback(item)
		if err != nil {
			// TODO:: xload : add retry logic
			log.Err("ThreadPool::Do : Error in %s processing workitem %s : %v", item.CompName, item.Path, err)
		}

		// add this error in response channel
		if cap(item.ResponseChannel) > 0 {
			item.Err = err
			item.DataLen = (uint64)(dataLength)
			item.ResponseChannel <- item
		}
	}
}
