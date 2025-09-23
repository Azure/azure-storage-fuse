/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobFUSEdev@microsoft.com>

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
	"encoding/json"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
)

//go:generate $ASSERT_REMOVER $GOFILE

//
// Contiguity tracker for tracking chunks uploaded to a file.
// The uploads can be pseudo sequential due to the write pattern and also due to parallel uploads,
// both of which are needed for high write performance.
// This tracker helps track the highest contiguous chunk index uploaded so far. Readers can safely
// read up to this chunk index from such partially written files.
//

type ContiguityTracker struct {
	mu             sync.Mutex
	file           *DcacheFile // DCache file being tracked.
	lastContiguous int64       // All chunks [0, lastContiguous) are uploaded.
	bitmap         []uint64
}

// NewContiguityTracker creates a new tracker with the given block size.
func NewContiguityTracker(file *DcacheFile) *ContiguityTracker {
	log.Debug("contiguity_tracker: NewContiguityTracker for file: %s, fileID: %s",
		file.FileMetadata.Filename, file.FileMetadata.FileID)

	return &ContiguityTracker{
		file: file,
	}
}

// OnSuccessfulUpload marks chunkIdx as uploaded.
func (t *ContiguityTracker) OnSuccessfulUpload(chunkIdx int64) {
	// We don't support overwrites, so we shouldn't be uploading the same chunk again.
	common.Assert(chunkIdx >= t.lastContiguous,
		chunkIdx, t.lastContiguous, t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	log.Debug("contiguity_tracker::OnSuccessfulUpload file: %s, fileID: %s, chunkIdx: %d",
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID, chunkIdx)

	t.mu.Lock()

	// Bit corresponding to chunkIdx.
	bitOffset := chunkIdx - t.lastContiguous

	// We support only limited deviation from sequential writes.
	common.Assert(bitOffset*t.file.FileMetadata.FileLayout.ChunkSize < (16*common.GbToBytes),
		bitOffset, t.file.FileMetadata.FileLayout.ChunkSize,
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	// Ensure bitmap is large enough.
	bitmapLen := (bitOffset / 64) + 1
	if int64(len(t.bitmap)) < bitmapLen {
		newBitmap := make([]uint64, max(bitmapLen, 16))
		copy(newBitmap, t.bitmap)
		t.bitmap = newBitmap
	}

	// Mark chunk as uploaded.
	idx := bitOffset / 64
	bit := bitOffset % 64

	// Must not already be set.
	common.Assert((t.bitmap[idx]&(1<<bit)) == 0,
		chunkIdx, t.lastContiguous, t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	t.bitmap[idx] |= 1 << bit

	//
	// Advance lastContiguous if we have uploaded one or more full uint64 bit worth of chunks, i.e. 64 chunks.
	// For 16MiB chunk size, this means 1GiB of contiguous data.
	// Every 1GiB of data upload we will update the metadata chunk with the new size.
	//
	fullWords := int64(0)
	for _, word := range t.bitmap {
		if word == ^uint64(0) {
			fullWords++
			continue
		}
		break
	}

	if fullWords == 0 {
		t.mu.Unlock()
		return
	}

	t.lastContiguous += fullWords * 64
	t.bitmap = t.bitmap[fullWords:]

	//
	// Now update the metadata chunk with the new size.
	// Any failure to update the metadata chunk is not fatal, it'll just mean that readers won't be able
	// to read as much data as they could have. The next successful upload that advances lastContiguous
	// will update the metadata chunk again. If those fail too, then the readers will not be able to read
	// the partial file and only when the file is closed will the final size be updated and readers can
	// read the full file.
	//
	mdChunk := &dcache.MetadataChunk{
		Size:          t.file.FileMetadata.FileLayout.ChunkSize * t.lastContiguous,
		LastUpdatedAt: time.Now(),
	}
	t.mu.Unlock()

	jsonData, err := json.Marshal(mdChunk)
	if err != nil {
		log.Err("contiguity_tracker::OnSuccessfulUpload: Failed to marshal %+v: %v",
			mdChunk, err)
		return
	}

	// We write just one chunk for the metadata, so it must fit in one chunk.
	common.Assert(len(jsonData) <= int(t.file.FileMetadata.FileLayout.ChunkSize),
		len(jsonData), t.file.FileMetadata.FileLayout.ChunkSize,
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)
	common.Assert(len(t.file.FileMetadata.FileLayout.MVList) > 0,
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	//
	// Upload the metadata chunk.
	// It's uploaded to the first MV in the MVList.
	//
	writeMVReq := &rm.WriteMvRequest{
		FileID:         t.file.FileMetadata.FileID,
		MvName:         t.file.FileMetadata.FileLayout.MVList[0],
		ChunkIndex:     dcache.MDChunkIdx,
		Data:           jsonData,
		ChunkSizeInMiB: t.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
		IsLastChunk:    true,
	}

	// Call WriteMV method for writing the chunk.
	_, err = rm.WriteMV(writeMVReq)
	if err != nil {
		log.Err("contiguity_tracker::OnSuccessfulUpload: Failed to upload metadata chunk %+v to %s, file: %s, fileID: %s: %v",
			mdChunk, t.file.FileMetadata.FileLayout.MVList[0],
			t.file.FileMetadata.Filename, t.file.FileMetadata.FileID, err)
	} else {
		log.Debug("contiguity_tracker::OnSuccessfulUpload: Uploaded metadata chunk %+v to %s, file: %s, fileID: %s",
			mdChunk, t.file.FileMetadata.FileLayout.MVList[0],
			t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)
	}
}

// HighestContiguous returns the highest contiguous uploaded chunk index.
func (t *ContiguityTracker) HighestUploaded() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastContiguous
}
