/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
)

// One workitem to be scheduled
type workItem struct {
	block   *Block // Block to hold data for this item
	failCnt int32  // How many times this item has failed to download
	upload  bool   // Flag marking this is a upload request or not
}

// ThreadPool is a group of workers that can be used to execute a task
type ThreadPool struct {
	// Number of workers running in this group
	worker uint32

	// Wait group to wait for all workers to finish
	wg sync.WaitGroup

	// Channel to hold pending requests
	workItems chan *workItem

	// Reader method that will actually read the data
	callback func(*workItem)
}

// newThreadPool creates a new thread pool
func newThreadPool(count uint32, callback func(*workItem)) *ThreadPool {
	if count == 0 || callback == nil {
		return nil
	}

	return &ThreadPool{
		worker:    count,
		callback:  callback,
		workItems: make(chan *workItem, count*2),
	}
}

// Start all the workers and wait till they start receiving requests
func (t *ThreadPool) Start() {
	for i := uint32(0); i < t.worker; i++ {
		t.wg.Add(1)
		go t.Do()
	}
}

// Stop all the workers threads
func (t *ThreadPool) Stop() {
	close(t.workItems)
	t.wg.Wait()
}

// Schedule the download of a block
func (t *ThreadPool) Schedule(item *workItem) {
	t.workItems <- item
}

// Do is the core task to be executed by each worker thread
func (t *ThreadPool) Do() {
	defer t.wg.Done()

	// This thread will work only on both high and low priority channel
	for item := range t.workItems {
		t.callback(item)
	}
}
