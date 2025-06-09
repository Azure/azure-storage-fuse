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

package distributed_cache

import (
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// The following go-routines are used when writing the file to Unqualified Path(i.e., Dcache, Azure) at the same time.

type writeReq struct {
	writer func() error // Function for writing to the file.
	err    chan error   // writer returns the error on this channel.
}

type parallelWriter struct {
	maxWriters       int
	azureWriterQueue chan *writeReq
	wg               sync.WaitGroup
}

// Spawns 64 go-routines for Dcache for writing.
func newParallelWriter() *parallelWriter {
	pw := &parallelWriter{
		maxWriters:       64,
		azureWriterQueue: make(chan *writeReq, 64),
	}

	for range pw.maxWriters {
		go pw.azureWriter()
	}

	log.Info("parallelWriter:: %d writers started for dcache, Used when writing to Unqualified path")

	return pw
}

func (pw *parallelWriter) destroyParallelWriter() {
	close(pw.azureWriterQueue)
	pw.wg.Wait()
	log.Info("parallelWriter:: %d writers destroyed for dcache, Used when writing to Unqualified path")
}

func (pw *parallelWriter) azureWriter() {
	pw.wg.Add(1)
	defer pw.wg.Done()

	for az := range pw.azureWriterQueue {
		err := az.writer()
		az.err <- err
	}
}

// caller should wait on the returned error for the status of the call.
func (pw *parallelWriter) EnqueuAzureWrite(azureWrite func() error) <-chan error {
	common.Assert(azureWrite != nil)

	azureWriteWorkItem := &writeReq{
		writer: azureWrite,
		err:    make(chan error),
	}
	// Queue the work Item.
	pw.azureWriterQueue <- azureWriteWorkItem

	return azureWriteWorkItem.err
}
