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

package dcache

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

//go:generate $ASSERT_REMOVER $GOFILE

var BufPool *BufferPool

type BufferPool struct {
	pool       sync.Pool    // sync.Pool to relieve GC
	bufSize    int          // size of buffers in this pool
	maxBuffers int64        // max allocated buffers allowed
	curBuffers atomic.Int64 // buffers currently allocated
}

func CreateBufferPool(bufSize uint64) error {
	common.Assert(BufPool == nil)
	//
	// Size of buffers managed by bufferPool.
	// This should be equal to the chunk size we support, since each buffer can hold upto one chunk
	// worth of data.
	//

	//
	// Maximum numbers of 'bufSize' buffers can be allocated from the bufferPool.
	// We should allow sufficiently many buffers to support at least few files being read/written
	// simultaneously.
	// Note that only writeChunk uses buffers from this pool while readChunk uses buffers allocated by
	// thrift and those are not accounted in this.
	//
	// TODO: Find out how/if thrift controls those buffers, or does it result in OOM killing of the
	//       process.
	//
	maxBuffers := uint64(1024)

	//
	// How much percent of the system RAM (available memory to be precise) are we allowed to use?
	//
	// TODO: This can be config value.
	//
	usablePercentSystemRAM := 50

	//
	// Allow higher number of maxBuffers if system can afford.
	//
	ramMB, err := common.GetAvailableMemoryInMB()
	if err != nil {
		return fmt.Errorf("NewFileIOManager: %v", err)
	}

	// usableMemory in bytes capped by usablePercentSystemRAM.
	usableMemory := (ramMB * 1024 * 1024 * uint64(usablePercentSystemRAM)) / 100
	maxBuffers = max(maxBuffers, usableMemory/bufSize)

	BufPool = &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		bufSize:    int(bufSize),
		maxBuffers: int64(maxBuffers),
	}

	return nil
}

func (bp *BufferPool) GetBuffer() ([]byte, error) {
	if bp.curBuffers.Load() > bp.maxBuffers {
		return nil, errors.New("Buffers Exhausted")
	}

	buf := bp.pool.Get().([]byte)

	// All buffers allocated must be of size bp.bufSize.
	common.Assert(len(buf) == bp.bufSize, len(buf), bp.bufSize)

	bp.curBuffers.Add(1)
	return buf, nil
}

func (bp *BufferPool) PutBuffer(buf []byte) {
	// All buffers allocated from the pool must be of the same size.
	common.Assert(len(buf) <= bp.bufSize, len(buf), bp.bufSize)
	// Caller must free a buffer that's allocated using getBuffer().
	common.Assert(bp.curBuffers.Load() > 0, bp.curBuffers.Load())

	// Reslice the length of the buffer to its original capacity if it got compacted.
	buf = buf[:bp.bufSize]

	bp.pool.Put(buf)
	bp.curBuffers.Add(-1)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
