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
	"sync/atomic"
	"time"
)

type StagedChunk struct {
	Idx int64  // chunk index
	Buf []byte // buf size == chunkSize
	//
	// offset in chunk that we are caching. Usually we cache entire chunk, so this will be 0, but for random
	// reads we only cache whatever is requested, so offset may be non-zero.
	//
	Offset int64
	Len    int64      // valid bytes in Buf (starting from Offset)
	Err    chan error // Download/upload status, available after download/upload completes, nil means success.
	//
	// For ReadMV(), buffer is returned by GetChunk() RPC, so we don't allocate it in
	// NewStagedChunk() while for WriteMV() we need to provide data to be sent using PutChunk().
	// If allocated using getBuffer() it must be freed using putBuffer(), IsBufExternal helps
	// track that.
	//
	IsBufExternal bool
	Dirty         atomic.Bool // Chunk has application data that must be written to the dcache.
	UpToDate      atomic.Bool // Chunk has been read from the cache and data matches dcache data.
	XferScheduled atomic.Bool // Is read/write from/to dcache already scheduled for this staged chunk?
	SavedInMap    atomic.Bool // This staged chunk is saved in DcacheFile.StagedChunks map.
	//
	// Reference count, number of active users of this staged chunk.
	// getChunk() increments the count, and caller must call releaseChunk().
	// Last user to drop their reference will free the chunk memory.
	//
	RefCount  atomic.Int32
	IOTracker *ChunkIOTracker // IOTracker for this staged chunk.

	//
	// When was this chunk allocated for read/write?
	// Currently only used by writers to see if unwritten chunks are not completing for long time.
	//
	AllocatedAt time.Time
}
