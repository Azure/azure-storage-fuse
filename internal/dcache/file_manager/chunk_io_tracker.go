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
	"math"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
)

//
// ChunkIOTracker tracks which parts of a chunk have been read or written.
// Caller can use this to determine if the entire chunk has been read or written. If entire chunk is read by
// the application then we would want to release the chunk from cache. Similarly, if entire chunk is written
// then we would want to flush the chunk to the backing store.
//

const (
	//
	// IOs smaller than this cannot be tracked by the chunk IO tracker.
	// We use 1 bit to track MinTrackableIOSize bytes of chunk memory.
	// This has the following implications:
	// 1. We cannot allow writes smaller than this, as we cannot track them and hence cannot be sure
	//    if/when the chunk is fully written.
	// 2. Reads smaller than this cannot be served from cache, as we cannot track them and hence cannot
	//    be sure if/when the chunk is fully read and hence safe to evict.
	//
	// TODO: Make this configurable. Also default to a smaller value like 4KB or 8KB.
	//
	MinTrackableIOSize = 64 * 1024 // 64KB
)

var (
	//Configured chunk size in bytes.
	chunkSize int64
	// How many MinTrackableIOSize blocks are there in chunkSize. We need 1 bit each to track each block.
	numBlocks int
	// How many uint64 integers are needed to track numBlocks bits.
	numUint64 int
)

type ChunkIOTracker struct {
	bitmap []uint64
	// How many bits are set aka how many unique blocks have been accessed (read or written).
	count int
}

func NewChunkIOTracker() *ChunkIOTracker {
	//
	// TODO: See how we can avoid this dynamic memory allocation.
	//       For most common chunk size and MinTrackableIOSize we can use a fixed size array.
	//
	return &ChunkIOTracker{
		bitmap: make([]uint64, numUint64),
	}
}

func GetMinTrackableIOSize() int64 {
	return MinTrackableIOSize
}

// MarkAccessed marks the given range [offsetInChunk, offsetInChunk+length) as accessed (read or written).
// If the length spans the chunk boundary, it returns the number of bytes that are in the remaining chunks.
func (bt *ChunkIOTracker) MarkAccessed(offsetInChunk, length int64) bool {
	// We should not be called for IO sizes smaller than MinTrackableIOSize.
	common.Assert(length >= MinTrackableIOSize && length <= chunkSize, length, chunkSize)
	common.Assert(offsetInChunk >= 0 && offsetInChunk < chunkSize, offsetInChunk, chunkSize)
	common.Assert(offsetInChunk+length <= chunkSize, offsetInChunk, length, chunkSize)

	//
	// Set all the bits corresponding to the given range.
	//
	for {
		block := int(offsetInChunk / MinTrackableIOSize)
		common.Assert(block >= 0 && block < numBlocks, offsetInChunk, block, chunkSize, MinTrackableIOSize)

		word := block / 64
		bit := uint(block % 64)

		common.Assert(word >= 0 && word < len(bt.bitmap), word, len(bt.bitmap))

		if common.AtomicTestAndSetBitUint64(&bt.bitmap[word], bit) {
			bt.count++
			common.Assert(bt.count <= numBlocks, bt.count, numBlocks)
		}

		offsetInChunk += MinTrackableIOSize
		length -= MinTrackableIOSize

		if length <= 0 {
			break
		}
	}

	return bt.count == numBlocks
}

func (bt *ChunkIOTracker) FullyAccessed() bool {
	return bt.count == numBlocks
}

func InitChunkIOTracker() {
	chunkSize = int64(cm.GetCacheConfig().ChunkSizeMB * common.MbToBytes)
	numBlocks = int(math.Ceil(float64(chunkSize) / MinTrackableIOSize))
	numUint64 = int(math.Ceil(float64(numBlocks) / 64))

	log.Info("ChunkIOTracker::init: chunkSize=%d, MinTrackableIOSize=%d, numBlocks=%d, bitmapSize=%d bytes",
		chunkSize, MinTrackableIOSize, numBlocks, numUint64*8)

	// Chunk size is a multiple of 1MiB and MinTrackableIOSize should be chosen to divide it evenly.
	common.Assert(chunkSize%MinTrackableIOSize == 0, chunkSize, MinTrackableIOSize)
	common.Assert(numBlocks > 0, numBlocks)
	common.Assert(numUint64 > 0, numUint64)
}
