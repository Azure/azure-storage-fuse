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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type bufferPool struct {
	pool                sync.Pool
	numRequestedBuffers int64
	maxBuffers          int64
	bufSize             int
	emptyBuf            []byte
}

func NewBufferPool(bufSize int, maxBuffers int) *bufferPool {
	var buf []byte
	return &bufferPool{
		pool: sync.Pool{
			New: func() any {
				return buf
			},
		},
		bufSize:    bufSize,
		emptyBuf:   make([]byte, bufSize),
		maxBuffers: int64(maxBuffers),
	}
}

func (bp *bufferPool) getBuffer() ([]byte, error) {
	if atomic.LoadInt64(&bp.numRequestedBuffers) > bp.maxBuffers {
		log.Info("Distributed Cache::getBuffer : Buffers Exhausted")
		return nil, errors.New("Buffers Exhausted")
	}

	buf := bp.pool.Get().([]byte)
	if buf == nil {
		buf = make([]byte, bp.bufSize)
	} else {
		// This can be remove in the future.
		copy(buf, bp.emptyBuf)
	}
	bp.numRequestedBuffers++
	return buf, nil
}

func (bp *bufferPool) putBuffer(buf []byte) {
	bp.pool.Put(buf)
	bp.numRequestedBuffers--
}
