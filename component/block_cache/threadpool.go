/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

import "sync"

type ThreadPool struct {
	// Number of workers running in this group
	worker uint32

	// Channel to close all the workers
	close chan int

	// Wait group to wait for all workers to finish
	wg sync.WaitGroup

	// Channel to hold pending requests
	priorityCh chan interface{}
	normalCh   chan interface{}

	// Reader method that will actually read the data
	reader func(interface{})
}

func newThreadPool(count uint32, reader func(interface{})) *ThreadPool {
	return &ThreadPool{
		worker:     count,
		reader:     reader,
		close:      make(chan int),
		priorityCh: make(chan interface{}),
		normalCh:   make(chan interface{}),
	}
}

// Start all the workers
func (t *ThreadPool) Start() {
	for i := uint32(0); i < t.worker; i++ {
		t.wg.Add(1)
		go t.Do()
	}
}

// Stop all the workers
func (t *ThreadPool) Stop() {
	for i := uint32(0); i < t.worker; i++ {
		t.close <- 1
	}

	t.wg.Wait()
	close(t.priorityCh)
	close(t.normalCh)
}

// Schedule the download of a block
func (t *ThreadPool) Schedule(urgent bool, item interface{}) {
	if urgent {
		t.priorityCh <- item
	} else {
		t.normalCh <- item
	}
}

func (t *ThreadPool) Do() {
	defer t.wg.Done()

	for {
		select {
		case item := <-t.priorityCh:
			t.reader(item)
		case item := <-t.normalCh:
			t.reader(item)
		case <-t.close:
			return
		}
	}
}
