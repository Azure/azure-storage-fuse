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

package block_cache

import (
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// ThreadPool is a group of workers that can be used to execute a task
type ThreadPool struct {
	// Number of workers running in this group
	worker uint32

	// Channel to close all the workers
	close chan int

	// Wait group to wait for all workers to finish
	wg sync.WaitGroup

	// Channel to hold pending requests
	priorityCh chan *workItem
	normalCh   chan *workItem

	// Reader method that will actually read the data
	reader func(*workItem)

	// Writer method that will actually write the data
	writer func(*workItem)
}

// One workitem to be scheduled
type workItem struct {
	handle   *handlemap.Handle // Handle to which this item belongs
	block    *Block            // Block to hold data for this item
	prefetch bool              // Flag marking this is a prefetch request or not
	failCnt  int32             // How many times this item has failed to download
	upload   bool              // Flag marking this is a upload request or not
	blockId  string            // BlockId of the block
	ETag     string            // Etag of the file before scheduling.
}

// Reason for storing Etag in workitem struct
// here getting the value of ETag inside upload/download methods
// from the handle is somewhat tricker.
// firstly we need to aquire a lock to read it from the handle.
// In these methods the handle may be locked/maynotbe locked by
// other go routine hence acquring would cause a deadlock.
// It is already locked if the call came from the readInBuffer.
// It is may be locked if the call come from the prefetch.

// newThreadPool creates a new thread pool
func newThreadPool(count uint32, reader func(*workItem), writer func(*workItem)) *ThreadPool {
	if count == 0 || reader == nil {
		return nil
	}

	return &ThreadPool{
		worker:     count,
		reader:     reader,
		writer:     writer,
		close:      make(chan int, count),
		priorityCh: make(chan *workItem, count*2),
		normalCh:   make(chan *workItem, count*5000),
	}
}

// Start all the workers and wait till they start receiving requests
func (t *ThreadPool) Start() {
	// 10% threads will listne only on high priority channel
	highPriority := (t.worker * 10) / 100

	for i := uint32(0); i < t.worker; i++ {
		t.wg.Add(1)
		go t.Do(i < highPriority)
	}
}

// Stop all the workers threads
func (t *ThreadPool) Stop() {
	for i := uint32(0); i < t.worker; i++ {
		t.close <- 1
	}

	t.wg.Wait()

	close(t.close)
	close(t.priorityCh)
	close(t.normalCh)
}

// Schedule the download of a block
func (t *ThreadPool) Schedule(urgent bool, item *workItem) {
	// urgent specifies the priority of this task.
	// true means high priority and false means low priority
	if urgent {
		t.priorityCh <- item
	} else {
		t.normalCh <- item
	}
}

// Do is the core task to be executed by each worker thread
func (t *ThreadPool) Do(priority bool) {
	defer t.wg.Done()

	if priority {
		// This thread will work only on high priority channel
		for {
			select {
			case item := <-t.priorityCh:
				if item.upload {
					t.writer(item)
				} else {
					t.reader(item)
				}
			case <-t.close:
				return
			}
		}
	} else {
		// This thread will work only on both high and low priority channel
		for {
			select {
			case item := <-t.priorityCh:
				if item.upload {
					t.writer(item)
				} else {
					t.reader(item)
				}
			case item := <-t.normalCh:
				if item.upload {
					t.writer(item)
				} else {
					t.reader(item)
				}
			case <-t.close:
				return
			}
		}
	}
}
