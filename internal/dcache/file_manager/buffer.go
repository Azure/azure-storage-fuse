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
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

//go:generate $ASSERT_REMOVER $GOFILE

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

	// All buffers allocated must be of size bp.bufSize.
	common.Assert(len(buf) == bp.bufSize, len(buf), bp.bufSize)

	bp.curBuffers.Add(1)
	return buf, nil
}

func (bp *bufferPool) putBuffer(buf []byte) {
	// All buffers allocated from the pool must be of the same size.
	common.Assert(len(buf) == bp.bufSize, len(buf), bp.bufSize)
	// Caller must free a buffer that's allocated using getBuffer().
	common.Assert(bp.curBuffers.Load() > 0, bp.curBuffers.Load())

	bp.pool.Put(buf)
	bp.curBuffers.Add(-1)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
