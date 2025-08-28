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
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

//go:generate $ASSERT_REMOVER $GOFILE

// This pool is shared across the dcache for allocation of the buffers, Currently this a fixed size singleton buffer
// pool implementation, where caller needs to request the buffer using GetBuffer() and it's responsibility of the caller
// to release the buffer using PutBuffer() after its use. The size of the buffers requested from the buffer will have
// length bufSize.
var bufPool *BufferPool

type BufferPool struct {
	pool       sync.Pool    // sync.Pool to relieve GC
	bufSize    int          // size of buffers in this pool
	maxBuffers int64        // max allocated buffers allowed
	curBuffers atomic.Int64 // buffers currently allocated
}

func InitBufferPool(bufSize uint64) error {
	common.Assert(bufPool == nil)

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
	// thrift and those are not accounted in this, with local RVs being an exception to that. When
	// reading from local RVs, ReadMV() calls GetChunkLocal() which results in buffers being allocated
	// from this pool.
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

	bufPool = &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		bufSize:    int(bufSize),
		maxBuffers: int64(maxBuffers),
	}

	log.Info("Buffer Pool: Initialized with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufPool.bufSize, bufPool.maxBuffers, float64(bufPool.maxBuffers*int64(bufPool.bufSize))/(1024.0*1024.0))

	return nil
}

func GetBuffer() ([]byte, error) {
	if bufPool.curBuffers.Load() > bufPool.maxBuffers {
		// TODO: Add a timeout to wait for the buffers to get free, and only fail after timeout.
		return nil, errors.New("Buffers Exhausted")
	}

	buf := bufPool.pool.Get().([]byte)

	// All buffers allocated must be of size bp.bufSize.
	common.Assert(len(buf) == bufPool.bufSize, len(buf), bufPool.bufSize)

	bufPool.curBuffers.Add(1)
	return buf, nil
}

func PutBuffer(buf []byte) {
	// All buffers allocated from the pool must be of the same size.
	common.Assert(len(buf) <= bufPool.bufSize, len(buf), bufPool.bufSize)
	// Caller must free a buffer that's allocated using getBuffer().
	common.Assert(bufPool.curBuffers.Load() > 0, bufPool.curBuffers.Load())

	// This buffer must have been allocated from the pool and hence must have capacity at least bp.bufSize.
	common.Assert(cap(buf) >= bufPool.bufSize, cap(buf), bufPool.bufSize)

	// Reslice the length of the buffer to its original capacity if it got compacted.
	buf = buf[:bufPool.bufSize]

	bufPool.pool.Put(buf)
	bufPool.curBuffers.Add(-1)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
