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

package agents

import (
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

//go:generate $ASSERT_REMOVER $GOFILE

// The following go-routines are used when writing the file to Unqualified Path(i.e., Dcache, Azure) at the same time.

type writeReq struct {
	writer func() error // Function for writing to the file.
	err    chan error   // writer returns the error on this channel.
}

type parallelWriter struct {
	maxWriters        int
	dcacheWriterQueue chan *writeReq
	wg                sync.WaitGroup
}

var pw *parallelWriter

// Spawns 64 go-routines for Dcache for writing.
func StartParallelWriter() {
	pw = &parallelWriter{
		maxWriters:        64,
		dcacheWriterQueue: make(chan *writeReq, 64),
	}

	for range pw.maxWriters {
		go pw.dacheWriter()
	}

	log.Info("parallelWriter:: %d writers started for dcache, Used when writing to Unqualified path",
		pw.maxWriters)
}

func DestroyParallelWriter() {
	close(pw.dcacheWriterQueue)
	pw.wg.Wait()
	log.Info("parallelWriter:: %d writers destroyed for dcache, Used when writing to Unqualified path",
		pw.maxWriters)
}

func (pw *parallelWriter) dacheWriter() {
	pw.wg.Add(1)
	defer pw.wg.Done()

	for az := range pw.dcacheWriterQueue {
		err := az.writer()
		az.err <- err
	}
}

// caller should wait on the returned error for the status of the call.
func (pw *parallelWriter) EnqueuDcacheWrite(dcacheWrite func() error) <-chan error {
	common.Assert(dcacheWrite != nil)

	dcacheWriteWorkItem := &writeReq{
		writer: dcacheWrite,
		err:    make(chan error),
	}
	// Queue the work Item.
	pw.dcacheWriterQueue <- dcacheWriteWorkItem

	return dcacheWriteWorkItem.err
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
