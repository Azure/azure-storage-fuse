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
	pool       sync.Pool    // sync.Pool to relieve GC
	bufSize    int          // size of buffers in this pool
	maxBuffers int64        // max allocated buffers allowed
	curBuffers atomic.Int64 // buffers currently allocated
}

func NewBufferPool(bufSize int, maxBuffers int) *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		bufSize:    bufSize,
		maxBuffers: int64(maxBuffers),
	}
}

func (bp *bufferPool) getBuffer() ([]byte, error) {
	if bp.curBuffers.Load() > bp.maxBuffers {
		return nil, errors.New("Buffers Exhausted")
	}

	buf := bp.pool.Get().([]byte)
	common.Assert(len(buf) == bp.bufSize, fmt.Sprintf("Allocated Buf Size %d != Required Buf Size %d", len(buf), bp.bufSize))
	bp.curBuffers.Add(1)
	return buf, nil
}

func (bp *bufferPool) putBuffer(buf []byte) {
	if len(buf) == bp.bufSize {
		bp.pool.Put(buf)
		common.Assert(bp.curBuffers.Load() > 0, fmt.Sprintf("Number of buffers are less than zero:  %d", bp.curBuffers.Load()))
		bp.curBuffers.Add(-1)
	}
}

// When the buffer is allocated outside the pool, and we want the pool to track that buffer for us.
// This should only be called if we have intension to return such buffer back to the pool after use usin putBuffer API.
// Buffer pool will track only if it matches its specification, else this and future putBuffer on this buf will be no op.
func (bp *bufferPool) getOutsideBufferIntoPool(buf []byte) error {
	if bp.curBuffers.Load() > bp.maxBuffers {
		return errors.New("Buffers Exhausted from the user configured limit")
	}

	if len(buf) == bp.bufSize {
		bp.curBuffers.Add(1)
	}

	return nil
}
