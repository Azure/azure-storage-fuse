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

	"github.com/Azure/azure-storage-fuse/v2/common"
)

type bufferPool struct {
	pool                sync.Pool
	numRequestedBuffers int64
	maxBuffers          int64
	bufSize             int
	emptyBuf            []byte // Used to reset the buffers and also used as a buf to
	//fill the holes inside the file while writing
}

func NewBufferPool(bufSize int, maxBuffers int) *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		bufSize:    bufSize,
		emptyBuf:   make([]byte, bufSize),
		maxBuffers: int64(maxBuffers),
	}
}

func (bp *bufferPool) getBuffer() ([]byte, error) {
	if atomic.LoadInt64(&bp.numRequestedBuffers) > bp.maxBuffers {
		return nil, errors.New("Buffers Exhausted")
	}

	buf := bp.pool.Get().([]byte)
	common.Assert(len(buf) == bp.bufSize, fmt.Sprintf("Allocated Buf Size %d != Required Buf Size %d", len(buf), bp.bufSize))
	bp.numRequestedBuffers++
	return buf, nil
}

func (bp *bufferPool) putBuffer(buf []byte) {
	copy(buf, bp.emptyBuf)
	bp.pool.Put(buf)
	common.Assert(atomic.LoadInt64(&bp.numRequestedBuffers) > 0, fmt.Sprintf("Number of buffers are less than zero:  %d", atomic.LoadInt64(&bp.numRequestedBuffers)))
	bp.numRequestedBuffers--
}
