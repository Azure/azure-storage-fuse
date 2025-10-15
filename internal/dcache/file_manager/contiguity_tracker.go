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
	"fmt"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

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

const (
	//
	// At the max we will need these many bits in the bitmap to track uploaded chunks.
	// See fileIOManager.maxUnackedWindow and NewStagedChunk(), how this is honoured not allowing new
	// chunk writes to be issued if the unacked window exceeds this.
	// We want this to be high for getting good performance under load where some PutChunkDC calls may
	// take longer than others, but not too high as that would need too much memory for the bitmap.
	// Note that numStagingChunks limits how many chunks can be in-flight for upload at any time, but
	// if chunks complete out of order, the unacked window can be much higher than numStagingChunks.
	//
	// Note: If the write performance is very variable under load, see if increasing this helps to make
	//       it more stable.
	//
	maxUnackedChunks = 4096

	//
	// Commit partial size no later than this interval (even if it has not changed).
	// This acts as a liveness indicator for DeleteDcacheFile() which uses this to determine if
	// a file with a non-Ready state is stale and can be deleted, or is it still being written to.
	//
	commitInterval = 5 * time.Second

	//
	// If metadata chunk is not updated for this much time, it's considered that the writer has gone away
	// without closing the file, and such files can be deleted by DeleteDcacheFile().
	// This can be smaller than 1min but deleting an in-progress file can have nasty consequences, so make
	// it less likely.
	//
	CommitLivenessPeriod = 60 * time.Second
)

type ContiguityTracker struct {
	mu             sync.Mutex
	file           *DcacheFile  // DCache file being tracked.
	lastContiguous int64        // All chunks [0, lastContiguous) are uploaded.
	maxUploadedIdx int64        // Max chunk index successfully uploaded so far.
	unackedWindow  atomic.Int64 // maxUploadedIdx - lastContiguous + 1
	uwChangedAt    atomic.Int64 // Last time unackedWindow changed, in nanoseconds since epoch. To detect stuck window.
	lastCommitted  time.Time    // Last time we updated the metadata chunk (not necessarily with a changed size).
	maxIssuedIdx   int64        // Last chunk index for which upload was issued (not necessarily completed yet).
	bitmap         []uint64
}

// NewContiguityTracker creates a new tracker with the given block size.
func NewContiguityTracker(file *DcacheFile) *ContiguityTracker {
	log.Debug("contiguity_tracker: NewContiguityTracker for file: %s, fileID: %s, commitInterval: %v",
		file.FileMetadata.Filename, file.FileMetadata.FileID, commitInterval)

	ct := &ContiguityTracker{
		maxUploadedIdx: -1,
		maxIssuedIdx:   -1,
		file:           file,
	}

	mdChunk := &dcache.MetadataChunk{
		Size:          0,
		LastUpdatedAt: time.Now(),
	}

	// Write initial metadata chunk with size=0.
	ct.writeMetadataChunk(mdChunk)

	return ct
}

func allocAlignedBuffer(size int) []byte {
	const alignment = common.FS_BLOCK_SIZE
	raw := make([]byte, size+alignment)
	addr := uintptr(unsafe.Pointer(&raw[0]))
	offset := int((alignment - (addr % alignment)) % alignment)
	return raw[offset : offset+size]
}

func (t *ContiguityTracker) writeMetadataChunk(mdChunk *dcache.MetadataChunk) {
	jsonData, err := json.Marshal(mdChunk)
	if err != nil {
		log.Err("contiguity_tracker::writeMetadataChunk: Failed to marshal %+v: %v",
			*mdChunk, err)
		return
	}

	//
	// Use aligned buffer to keep server PutChunk assertions happy.
	// Slight inefficiency here is ok since metadata chunk is small and the update is infrequent.
	//
	alignedJsonData := allocAlignedBuffer(len(jsonData))
	copy(alignedJsonData, jsonData)

	// We write just one chunk for the metadata, so it must fit in one chunk.
	common.Assert(len(alignedJsonData) <= int(t.file.FileMetadata.FileLayout.ChunkSize),
		len(alignedJsonData), t.file.FileMetadata.FileLayout.ChunkSize,
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)
	// Infact it must be much less, smaller than MDChunkSize.
	common.Assert(len(alignedJsonData) < dcache.MDChunkSize,
		len(alignedJsonData), dcache.MDChunkSize,
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
		Data:           alignedJsonData,
		ChunkSizeInMiB: t.file.FileMetadata.FileLayout.ChunkSize / common.MbToBytes,
		IsLastChunk:    true,
	}

	// Call WriteMV method for writing the chunk.
	_, err = rm.WriteMV(writeMVReq)
	if err != nil {
		log.Err("contiguity_tracker::writeMetadataChunk: Failed to upload metadata chunk %+v for %+v: %v",
			*mdChunk, *t.file.FileMetadata, err)
	} else {
		log.Debug("contiguity_tracker::writeMetadataChunk: Uploaded metadata chunk %+v for %+v",
			*mdChunk, *t.file.FileMetadata)
	}
}

// Call this just before WriteMV() is called to upload a chunk.
// This is called before the actual upload is done, while OnSuccessfulUpload is called after WriteMV()
// successfully completes.
func (t *ContiguityTracker) OnUploadStart(chunkIdx int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// We don't support overwrites, so we shouldn't be uploading the same chunk again.
	common.Assert(chunkIdx >= t.lastContiguous,
		chunkIdx, t.lastContiguous, t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	//
	// Usually chunkIdx should be maxIssuedIdx+1, but it can be slightly off (by one or two) as
	// we support limited out of order writes for supporting large application write IOs for perf.
	//
	if common.IsDebugBuild() {
		if chunkIdx != t.maxIssuedIdx+1 {
			// Unexpected, but allowed.
			log.Debug("contiguity_tracker::OnUploadStart: Sparse upload, chunkIdx: %d, maxIssuedIdx: %d, file: %+v",
				chunkIdx, t.maxIssuedIdx, *t.file.FileMetadata)
		} else {
			// Expected.
			log.Debug("contiguity_tracker::OnUploadStart: chunkIdx: %d, maxIssuedIdx: %d, file: %+v",
				chunkIdx, t.maxIssuedIdx, *t.file.FileMetadata)
		}
	}

	t.maxIssuedIdx = max(t.maxIssuedIdx, chunkIdx)
}

// OnSuccessfulUpload marks chunkIdx as uploaded.
func (t *ContiguityTracker) OnSuccessfulUpload(chunkIdx int64) {
	t.mu.Lock()

	// We don't support overwrites, so we shouldn't be uploading the same chunk again.
	common.Assert(chunkIdx >= t.lastContiguous,
		chunkIdx, t.lastContiguous, t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)
	// maxUploadedIdx must always be >= lastContiguous-1.
	common.Assert(t.maxUploadedIdx >= (t.lastContiguous-1),
		t.maxUploadedIdx, t.lastContiguous,
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID)

	log.Debug("contiguity_tracker::OnSuccessfulUpload file: %s, fileID: %s, chunkIdx: %d, maxUploadedIdx: %d, lastContiguous: %d, unackedWindow: %d",
		t.file.FileMetadata.Filename, t.file.FileMetadata.FileID, chunkIdx,
		t.maxUploadedIdx, t.lastContiguous, t.GetUnackedWindow())

	if chunkIdx > t.maxUploadedIdx {
		t.maxUploadedIdx = chunkIdx
	}

	// Bit corresponding to chunkIdx.
	bitOffset := chunkIdx - t.lastContiguous

	//
	// NewStagedChunk() must not allow writes that are too far ahead of lastContiguous.
	// See comment in NewStagedChunk() why we need to relax this check.
	//
	common.Assert(bitOffset <= (maxUnackedChunks+256),
		bitOffset, maxUnackedChunks, chunkIdx, t.lastContiguous, t.file.FileMetadata.FileLayout.ChunkSize,
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
	// If commitInterval or more time has elapsed since last update, we will update the metadata chunk even with
	// less than 1GiB of new contiguous data or even no new contiguous data.
	//
	fullWords := int64(0)
	newChunks := int64(0)
	for _, word := range t.bitmap {
		if word == ^uint64(0) {
			fullWords++
			continue
		} else {
			// And any contiguous chunks from the start of the word.
			newChunks = int64(bits.TrailingZeros64(^uint64(word)))
			common.Assert(newChunks < 64, newChunks, word, fullWords, t.bitmap, *t.file.FileMetadata)
		}

		break
	}

	// One or more full words can be now removed from the bitmap?
	if fullWords > 0 {
		t.lastContiguous += (fullWords * 64)
		t.bitmap = t.bitmap[fullWords:]

		log.Debug("contiguity_tracker::OnSuccessfulUpload file: %s, fileID: %s, fullWords: %d, lastContiguous: %d, newChunks: %d, len(bitmap): %d",
			t.file.FileMetadata.Filename, t.file.FileMetadata.FileID, fullWords, t.lastContiguous,
			newChunks, len(t.bitmap))
	}

	// Update unacked window.
	newUW := t.maxUploadedIdx - (t.lastContiguous + newChunks) + 1
	if newUW != t.unackedWindow.Load() {
		// Unacked window changed.
		t.uwChangedAt.Store(time.Now().UnixNano())
		t.unackedWindow.Store(newUW)
	}

	common.Assert(t.unackedWindow.Load() >= 0,
		t.unackedWindow.Load(), t.maxUploadedIdx, t.lastContiguous, newChunks, *t.file.FileMetadata)

	if fullWords == 0 && newChunks == 0 {
		t.mu.Unlock()
		return
	}

	//
	// Now update the metadata chunk with the new size.
	// Any failure to update the metadata chunk is not fatal, it'll just mean that readers won't be able
	// to read as much data as they could have. The next successful upload that advances lastContiguous
	// will update the metadata chunk again. If those fail too, then the readers will not be able to read
	// the partial file and only when the file is closed will the final size be updated and readers can
	// read the full file.
	//
	mdChunk := &dcache.MetadataChunk{
		Size:          t.file.FileMetadata.FileLayout.ChunkSize * (t.lastContiguous + newChunks),
		LastUpdatedAt: time.Now(),
	}

	// Partial word update is done no sooner than commitInterval, unless we have advanced by 64+ chunks.
	if (time.Since(t.lastCommitted) < commitInterval) && (fullWords == 0) {
		t.mu.Unlock()
		return
	}

	t.lastCommitted = mdChunk.LastUpdatedAt
	t.mu.Unlock()

	t.writeMetadataChunk(mdChunk)
}

// Read the metadata chunk for the given file, to get the highest uploaded byte for the file.
func GetHighestUploadedByte(fileMetadata *dcache.FileMetadata) (int64, time.Time) {
	readMVReq := &rm.ReadMvRequest{
		FileID:         fileMetadata.FileID,
		MvName:         fileMetadata.FileLayout.MVList[0],
		ChunkIndex:     dcache.MDChunkIdx,
		OffsetInChunk:  0,
		Length:         dcache.MDChunkSize,
		ChunkSizeInMiB: fileMetadata.FileLayout.ChunkSize / common.MbToBytes,
	}

	readMVresp, err := rm.ReadMV(readMVReq)
	if err != nil {
		// Most likely error is that the metadata chunk does not exist yet.
		log.Err("contiguity_tracker::GetHighestUploadedByte: Failed to read metadata chunk, %+v: %v",
			*fileMetadata, err)
		//
		// Since we create the metadata chunk with size=0 when the file is created, this should not happen.
		// We return time as 0 to allow the file to be deleted if it ever happens.
		// See comment in NewDcacheFile(), how this can happen.
		//
		//common.Assert(false, *fileMetadata, err)
		return 0, time.Time{}
	}

	var mdChunk dcache.MetadataChunk
	err = json.Unmarshal(readMVresp.Data, &mdChunk)
	if err != nil {
		log.Err("contiguity_tracker::GetHighestUploadedByte: Failed to unmarshal metadata chunk, %+v, %v: %v",
			*fileMetadata, readMVresp.Data, err)
		//
		// Unable to read metadata chunk, return size as 0.
		// We return time as now to prevent the file from being deleted, as we are not sure whether the file
		// is being currently written to or not.
		// Again, this should not happen unless there is some bug.
		//
		common.Assert(false, *fileMetadata, err)
		return 0, time.Now()
	}

	if !readMVresp.IsBufExternal {
		dcache.PutBuffer(readMVresp.Data)
	}

	log.Debug("contiguity_tracker::GetHighestUploadedByte: Read metadata chunk %+v for %+v",
		mdChunk, *fileMetadata)

	common.Assert(mdChunk.Size >= 0, mdChunk.Size, *fileMetadata)
	common.Assert(!mdChunk.LastUpdatedAt.IsZero(), mdChunk.LastUpdatedAt, *fileMetadata)

	return mdChunk.Size, mdChunk.LastUpdatedAt
}

// Unacked window is the difference in chunk index between the max successfully uploaded chunk index and the
// last contiguous chunk index. It's an estimate of the "lag" of some slow/unresponsive RV vs regular/fast
// RVs. We don't want this to grow without bound, so we use this to apply back-pressure on writers.
func (t *ContiguityTracker) GetUnackedWindow() int64 {
	uw := t.unackedWindow.Load()
	common.Assert(uw >= 0, uw, t.maxUploadedIdx, t.lastContiguous, *t.file.FileMetadata)

	//
	// Sanity check for stuck window.
	// If the window has not changed for 5 minutes, i.e., none of our PutChunkDC requests have got a successful
	// response, something is wrong. Restart blobfuse in this case to recover.
	//
	// Note: maxUploadedIdx and maxIssuedIdx are non-atomic and hence accessing them outside lock is racy,
	//       but they are just being logged.
	//
	if uw != 0 && t.uwChangedAt.Load() != 0 &&
		time.Since(time.Unix(0, t.uwChangedAt.Load())) > 30*time.Second {
		// If not changed for 30s, log a slow warning, and if not changed for 5min, panic and restart
		err := fmt.Errorf("Unacked window stuck for %s, uw: %d, maxUploadedIdx: %d, lastContiguous: %d, maxIssuedIdx: %d, file: %+v",
			time.Since(time.Unix(0, t.uwChangedAt.Load())),
			uw, t.maxUploadedIdx, t.lastContiguous, t.maxIssuedIdx, *t.file.FileMetadata)

		log.Warn("[SLOW] %v", err)

		if time.Since(time.Unix(0, t.uwChangedAt.Load())) > 300*time.Second {
			log.GetLoggerObj().Panicf("[%s][BUG] %v", common.GetDebugHostname(), err)
		}
	}

	if uw > maxUnackedChunks/4 {
		//
		// Slow warning log if unacked window is high.
		// This indicates that some RVs are slow/unresponsive or there are packet drops in he network, and
		// hence some PutChunkDC requests are taking much longer than others.
		//
		log.Warn("[SLOW] Unacked window high, uw: %d, maxUploadedIdx: %d, lastContiguous: %d, maxIssuedIdx: %d, file: %s, fileID: %s",
			uw, t.maxUploadedIdx, t.lastContiguous, t.maxIssuedIdx, t.file.FileMetadata.Filename,
			t.file.FileMetadata.FileID)
	}

	return uw
}

func (t *ContiguityTracker) IsChunkUploaded(chunkIdx int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if chunkIdx < t.lastContiguous {
		return true
	}

	bitOffset := chunkIdx - t.lastContiguous
	idx := bitOffset / 64
	bit := bitOffset % 64

	if int64(len(t.bitmap)) <= idx {
		return false
	}

	return (t.bitmap[idx] & (1 << bit)) != 0
}
