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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
)

//go:generate $ASSERT_REMOVER $GOFILE

type fileIOManager struct {
	// If SafeDeletes is enabled we always increment and decrement the file descriptor count while opening
	// and closing the file. This ensures the delete/unlink of a file works as per the POSIX semantics
	// (i.e., delete of the file would be deferred until the last open file descriptor is closed).
	// It defaults to false unless specified in the config.
	// Multiple readers opening the same file will contend to atomically update the open count, so this will
	// increase the file open/close latency, and must be used only if POSIX semantics is desired.
	safeDeletes bool

	// Number of chunks to readahead after the current chunk.
	// This controls our readahead size.
	numReadAheadChunks int

	// Max Number of chunks per file that can be in staging area at any time.
	// This is our writeback buffer. If it's too small, application writes will need to wait while
	// we write the current staged chunks to dcache. Note that though we schedule write of staged
	// chunks as soon as the chunk is fully written (with application data) but the actual write
	// may take time, so having more staged chunks allows application writes to proceed while staged
	// chunks are being written.
	numStagingChunks int
	wp               *workerPool
}

var fileIOMgr fileIOManager

func NewFileIOManager() error {
	//
	// A worker runs either readChunk() or writeChunk(), so this is the number of chunks we can be
	// reading/writing in parallel. fileIOManager is one for the entire blobfuse process so these
	// chunks can be spread across multiple files and served from multiple nodes/RVs.
	// Note that for writeChunk one worker is used up regardless of the NumReplicas setting. Each
	// replica write uses just one ReplicationManager worker but only one fileIOManager worker per
	// MV write (not replica write).
	// Since all these reads/writes may be served from different nodes/RVs, the limiting factor would
	// be the n/w b/w of this node. We need enough parallel readChunk/writeChunk for maxing out the
	// n/w b/w of the node. Keeping small files in mind, and given that go routines are not very
	// expensive, we keep 1000 workers.
	//
	workers := 1000

	//
	// How many chunks will we readahead per file.
	// To achieve high sequential read throughput, this number should be kept reasonably high.
	// With 4MiB chunk size, 64 readahead chunks will use up 256MiB of memory per file.
	//
	numReadAheadChunks := 64

	//
	// How many writeback chunks per file.
	// These many chunks we will store per file before we put back pressure on the writer application.
	// Obviously we start upload of chunks as soon as we have a full chunk, so only those chunks will
	// eat up the writeback space which are not completely written to the target node.
	// Hopefully we won't be writing too many large files simultaneously, so we can keep this number
	// high enough to give 1GiB writeback space per file.
	//
	numStagingChunks := 256

	common.Assert(workers > 0)
	common.Assert(numReadAheadChunks > 0)
	common.Assert(numStagingChunks > 0)

	// NewFileIOManager() must be called only once, during startup.
	common.Assert(fileIOMgr.wp == nil)

	fileIOMgr = fileIOManager{
		safeDeletes:        cm.GetCacheConfig().SafeDeletes,
		numReadAheadChunks: numReadAheadChunks,
		numStagingChunks:   numStagingChunks,
	}

	fileIOMgr.wp = NewWorkerPool(workers)

	common.Assert(fileIOMgr.wp != nil)

	InitChunkIOTracker()

	return nil
}

func EndFileIOManager() {
	common.Assert(fileIOMgr.wp != nil)

	if fileIOMgr.wp != nil {
		fileIOMgr.wp.destroyWorkerPool()
	}
}

type DcacheFile struct {
	FileMetadata *dcache.FileMetadata

	//
	// Read pattern tracker for detecting read pattern (sequential/random).
	// Only valid for files opened for reading, nil for files opened for writing.
	//
	RPT *RPTracker

	// Next write offset we expect in case of sequential writes.
	// Every new write offset should be >= nextWriteOffset, else it's the case of overwriting existing
	// data and we don't support that.
	nextWriteOffset     int64
	committedTillOffset int64

	// Chunk index (inclusive) till which we have done readahead.
	lastReadaheadChunkIdx atomic.Int64

	//
	// Chunk Idx -> *StagedChunk
	// These are the chunks that are currently cached in memory for this file.
	// These chunks can be either readahead chunks or write back chunks.
	//
	// This is protected by chunkLock.
	//
	// TODO: Should we share chunks among multiple open handles of the same file?
	//
	StagedChunks map[int64]*StagedChunk
	chunkLock    sync.RWMutex

	// Etag returned by createFileInit(), later used by createFileFinalize() to catch unexpected
	// out-of-band changes to the metadata file between init and finalize.
	finalizeEtag string

	// This cached attr is used for optimizing the REST API calls, when safeDeletes is enabled.
	// It is saved when the file is opened, if there are no more file opens done on that file, the same etag
	// can be used to update the file open count on close.
	attr *internal.ObjAttr
}

// Reads the file data from the given offset and length to buf[].
// It translates the requested offsets into chunks, reads those chunks from distributed cache and copies data from
// chunk into user buffer.
func (file *DcacheFile) ReadFile(offset int64, buf *[]byte) (bytesRead int, err error) {
	log.Debug("DistributedCache::ReadFile: file: %s, offset: %d, length: %d",
		file.FileMetadata.Filename, offset, len(*buf))

	// Read must only be allowed on a properly finalized file, for which size must not be -1.
	common.Assert(int64(file.FileMetadata.Size) >= 0)
	// Read patterm tracker must be valid for files opened for reading.
	common.Assert(file.RPT != nil, file.FileMetadata.Filename)

	if offset >= file.FileMetadata.Size {
		log.Warn("DistributedCache::ReadFile: Read beyond eof. file: %s, offset: %d, file size: %d, length: %d",
			file.FileMetadata.Filename, offset, file.FileMetadata.Size, len(*buf))
		return 0, io.EOF
	}

	// endOffset is 1 + offset of the last byte to be read.
	endOffset := min(offset+int64(len(*buf)), file.FileMetadata.Size)
	// Catch wraparound.
	common.Assert(endOffset >= offset, endOffset, offset)
	bufOffset := 0

	// Update sequential/random pattern tracker with this new IO.
	file.RPT.Update(offset, endOffset-offset)

	//
	// Read all the requested data bytes from the distributed cache.
	// Note that distributed cache stores data in units of chunks.
	//
	for offset < endOffset {
		//
		// Read chunk containing data at 'offset', also schedule readahead if the read pattern qualifies
		// as sequential.
		//
		var chunk *StagedChunk
		var err error
		// Reads less than MinTrackableIOSize are always treated as random reads.
		doSequentialRead := file.RPT.IsSequential() && (endOffset-offset >= GetMinTrackableIOSize())

		if doSequentialRead {
			chunk, err = file.readChunkWithReadAhead(offset)
			common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)
		} else {
			if endOffset-offset < GetMinTrackableIOSize() {
				log.Warn("DistributedCache::ReadFile: Small read (%d < %d) file: %s, offset: %d, forcing random read",
					endOffset-offset, GetMinTrackableIOSize(), file.FileMetadata.Filename, offset)
			}

			// For random read pattern we don't do readahead, and also don't save the chunk in StagedChunks map.
			chunk, err = file.readChunkNoReadAhead(offset, endOffset-offset)
			common.Assert(chunk.SavedInMap.Load() == false, chunk.Idx, file.FileMetadata.Filename)
		}

		if err != nil {
			return 0, err
		}

		// We should only be reading from an up-to-date chunk (that has been successfully read from dcache).
		common.Assert(chunk.UpToDate.Load())

		chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)

		// This chunk has chunk.Len valid bytes, we cannot be reading past those.
		common.Assert(chunkOffset < chunk.Len, chunkOffset, chunk.Len)

		//
		// Note/TODO: If we use the fuse low level API we can avoid this copy and return the chunk.Buf
		//            directly in the fuse response.
		//
		copied := copy((*buf)[bufOffset:], chunk.Buf[chunkOffset:chunk.Len])
		// Must copy at least one byte.
		common.Assert(copied > 0, chunkOffset, bufOffset, chunk.Len, len(*buf))

		offset += int64(copied)
		bufOffset += copied

		if doSequentialRead {
			if chunk.IOTracker.MarkAccessed(chunkOffset, int64(copied)) {
				file.removeChunk(chunk.Idx)
			}
		} else {
			file.releaseChunk(chunk)
		}
	}

	// Must exactly read what's determined above (what user asks, capped by file size).
	common.Assert(offset == endOffset, offset, endOffset)
	// Must not read beyond eof.
	common.Assert(offset <= file.FileMetadata.Size, offset, file.FileMetadata.Size)
	// Must never read more than the length asked.
	common.Assert(bufOffset <= len(*buf), bufOffset, len(*buf))

	return bufOffset, nil
}

// Writes user data into file at given offset and length.
// It translates the requested offsets into chunks, and writes to those chunks in the distributed cache.
func (file *DcacheFile) WriteFile(offset int64, buf []byte) error {
	log.Debug("DistributedCache[FM]::WriteFile: file: %s, offset: %d, length: %d",
		file.FileMetadata.Filename, offset, len(buf))

	// DCache files are immutable, all writes must be before first close, by which time file size is not known.
	common.Assert(int64(file.FileMetadata.Size) == -1, file.FileMetadata.Size)
	// Read patterm tracker must not be present for files opened for writing.
	common.Assert(file.RPT == nil, file.FileMetadata.Filename)
	// We should not be called for 0 byte writes.
	common.Assert(len(buf) > 0)

	//
	// We allow writes only within the staging area.
	// Anything before that amounts to overwrite, and anything beyond that is not allowed by our writeback
	// cache limits.
	//
	allowableWriteOffsetStart := file.committedTillOffset + 1
	allowableWriteOffsetEnd := (allowableWriteOffsetStart +
		int64(fileIOMgr.numStagingChunks)*file.FileMetadata.FileLayout.ChunkSize)

	//
	// We only support append writes.
	// 'cp' utility uses sparse writes to avoid writing zeroes from the source file to target file, we support
	// that too, hence offset can be greater than nextWriteOffset.
	//
	if offset < allowableWriteOffsetStart {
		log.Err("DistributedCache[FM]::WriteFile: Overwrite unsupported, file: %s, offset: %d, committedTillOffset: %d",
			file.FileMetadata.Filename, offset, file.committedTillOffset)
		return syscall.ENOTSUP
	} else if offset > allowableWriteOffsetEnd {
		log.Err("DistributedCache[FM]::WriteFile: Random write unsupported, file: %s, offset: %d, committedTillOffset: %d, numStagingChunks: %d",
			file.FileMetadata.Filename, offset, file.committedTillOffset, fileIOMgr.numStagingChunks)
		return syscall.ENOTSUP
	} else if len(buf) < int(GetMinTrackableIOSize()) {
		//
		// Writes less than MinTrackableIOSize cannot be tracked and hence not supported.
		// TODO: If they are strictly sequential writes we can possibly support them.
		//
		log.Err("DistributedCache[FM]::WriteFile: Small write (%d < %d), file: %s, offset: %d, unsupported",
			len(buf), GetMinTrackableIOSize(), file.FileMetadata.Filename, offset)
		return syscall.ENOTSUP
	}

	// endOffset is 1 + offset of the last byte to be write.
	endOffset := (offset + int64(len(buf)))
	// Catch wraparound.
	common.Assert(endOffset > offset, endOffset, offset)
	bufOffset := 0

	//
	// Write all the requested data bytes to the distributed cache.
	// Note that distributed cache stores data in units of chunks.
	//
	for offset < endOffset {
		// Get the StagedChunk that will hold the file data at offset.
		chunk, err := file.CreateOrGetStagedChunk(offset)
		if err != nil {
			err = fmt.Errorf("Failed to get chunk for write, file: %s, chnk idx: %d: %v",
				file.FileMetadata.Filename,
				getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout), err)
			log.Err("DistributedCache[FM]::WriteFile: %v", err)
			common.Assert(false, err)
			return err
		}

		// Offset within the chunk to write.
		chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)
		// chunkOffset cannot overshoot ChunkSize.
		common.Assert(chunkOffset < file.FileMetadata.FileLayout.ChunkSize,
			chunkOffset, file.FileMetadata.FileLayout.ChunkSize)
		// We don't support overwriting chunk data.
		common.Assert(chunkOffset >= chunk.Len, chunkOffset, chunk.Len)

		//
		// This case is mostly for handling 'cp' where it jumps the offset to avoid copying zeroes from
		// source to target file. We fill in the gap with zeroes.
		//
		if chunkOffset > chunk.Len {
			// TODO: Use a static zero buffer instead of allocating one every time.
			resetBytes := copy(chunk.Buf[chunk.Len:chunkOffset], make([]byte, chunkOffset-chunk.Len))
			common.Assert(resetBytes > 0)
			chunk.Len += int64(resetBytes)
		}

		// We must copy right after where the prev copy ended.
		common.Assert(chunk.Len == chunkOffset, chunk.Len, chunkOffset)

		copied := copy(chunk.Buf[chunkOffset:], buf[bufOffset:])
		// Must copy at least one byte from the chunk.
		common.Assert(copied > 0, chunkOffset, bufOffset, chunk.Len, len(buf))
		offset += int64(copied)
		bufOffset += copied
		chunk.Len += int64(copied)

		// Valid chunk data cannot be more than ChunkSize.
		common.Assert(chunk.Len <= file.FileMetadata.FileLayout.ChunkSize,
			chunk.Len, file.FileMetadata.FileLayout.ChunkSize)

		// Cannot be more than the buffer.
		common.Assert(chunk.Len <= int64(len(chunk.Buf)), chunk.Len, int64(len(chunk.Buf)))

		// At least one byte of user data copied to chunk, it's dirty now and must be written to dcache.
		chunk.Dirty.Store(true)

		common.Assert(chunk.Len == getChunkOffsetFromFileOffset(offset-1, &file.FileMetadata.FileLayout)+1,
			fmt.Sprintf("Actual Chunk Len: %d is modified incorrectly, expected chunkLen: %d",
				chunk.Len, getChunkOffsetFromFileOffset(offset-1, &file.FileMetadata.FileLayout)+1))

		//
		// Schedule the upload when a staged chunk is fully written.
		// There's no point in waiting any more. Sooner we write completed chunks, faster we will complete
		// writes.
		//
		if chunk.IOTracker.MarkAccessed(chunkOffset, int64(copied)) {
			// TODO: This not always true. if some writes to this chunk were skipped then there should be a
			// way to stage this block.
			scheduleUpload(chunk, file)
		}
	}

	// Must write complete data.
	common.Assert(offset == endOffset, offset, endOffset)
	// and only that much.
	common.Assert(bufOffset == len(buf), bufOffset, len(buf))

	// Next write expected at offset nextWriteOffset.
	file.nextWriteOffset = offset

	return nil
}

// Sync Buffers for the file with dcache/azure.
// This call can come when user application calls fsync()/close().
func (file *DcacheFile) SyncFile() error {
	log.Debug("DistributedCache[FM]::SyncFile: %s", file.FileMetadata.Filename)

	var err error
	//
	// Go over all the staged chunks and write to cache if not already done.
	// Note that we keep fileIOMgr.numStagingChunks number of chunks per file. As chunks are fully written
	// we upload them to cache, so only the last incomplete chunk would be actually written by the following loop.
	//
	for chunkIdx, chunk := range file.StagedChunks {
		_ = chunkIdx
		common.Assert(chunkIdx == chunk.Idx, chunkIdx, chunk.Idx)
		// TODO: parallelize the uploads for the chunks.
		log.Debug("DistributedCache[FM]::SyncFile: file: %s, chunkIdx: %d, chunkLen: %d",
			file.FileMetadata.Filename, chunk.Idx, chunk.Len)

		// Synchronously write the chunk to dcache.
		scheduleUpload(chunk, file)
		err = <-chunk.Err
		if err != nil {
			log.Err("DistributedCache[FM]::SyncFile: file: %s, chunkIdx: %d, chunkLen: %d, failed: %v",
				file.FileMetadata.Filename, chunk.Idx, chunk.Len, err)
			break
		}
	}

	common.Assert(err == nil, file.FileMetadata.Filename, err)

	return err
}

// Close and Finalize the file. writes are failed after successful file close.
func (file *DcacheFile) CloseFile() error {
	log.Debug("DistributedCache[FM]::CloseFile: %s", file.FileMetadata.Filename)

	//
	// We stage application writes into StagedChunk and upload only when we have a full chunk.
	// In case of last chunk being partial, we need to upload it now.
	//
	err := file.SyncFile()
	common.Assert(err == nil, file.FileMetadata.Filename, err)

	//
	// On successful close we finalize the file.
	// This will update the file metadata with final file size.
	//
	if err == nil {
		err := file.finalizeFile()
		if err != nil {
			log.Err("DistributedCache[FM]::Close: finalize file failed for %s: %v",
				file.FileMetadata.Filename, err)
			common.Assert(false, file.FileMetadata.Filename, err)
		}
	}

	return err
}

// Release all allocated buffers for the file.
func (file *DcacheFile) ReleaseFile(isReadOnlyHandle bool) error {
	log.Debug("DistributedCache[FM]::ReleaseFile: Releasing all staged chunk for file %s",
		file.FileMetadata.Filename)

	for chunkIdx, chunk := range file.StagedChunks {
		_ = chunkIdx
		common.Assert(chunkIdx == chunk.Idx, chunkIdx, chunk.Idx)
		// TODO: assert for each chunk that err is closed. currently not doing it as readahead chunks
		// error channel might be opened.
		file.releaseChunk(chunk)
	}

	//
	// Decrement the file open count if safeDeletes is enabled and handle corresponds to a file opened for
	// reading.
	//
	if fileIOMgr.safeDeletes && isReadOnlyHandle {
		// attr must have been saved when file was opened for read.
		common.Assert(file.attr != nil, file.FileMetadata)

		openCount, err := mm.CloseFile(file.FileMetadata.Filename, file.attr)
		_ = openCount
		if err != nil {
			err = fmt.Errorf("Failed to decrement open count for file %s: %v",
				file.FileMetadata.Filename, err)
			log.Err("DistributedCache[FM]::ReleaseFile: %v", err)
			common.Assert(false, err)
			//
			// TODO: Should we fail or silently succeed?
			//       Failing may not be an option as we may have released critical data structures
			//       corresponding to the file, but if we silently succeed this file data chunks
			//       can never be released and will be leaked.
			//       One way of handling could be to force remove chunks for the file if file opencount
			//       stays non-zero for a long time.
			//
		} else {
			log.Debug("DistributedCache[FM]::ReleaseFile: Decremented open count, now: %d, file: %s",
				openCount, file.FileMetadata.Filename)
		}
	}

	return nil
}

// This method is called when all the File IO operations are successful and the file is closed.
// Since files are immutable, no further writes will be allowed.
func (file *DcacheFile) finalizeFile() error {
	// State must be "writing", since we finalize a file only once.
	common.Assert(file.FileMetadata.State == dcache.Writing)
	file.FileMetadata.State = dcache.Ready

	// Till we finalize a file we don't know the size.
	common.Assert(file.FileMetadata.Size == -1, file.FileMetadata.Filename, file.FileMetadata.Size)
	file.FileMetadata.Size = file.nextWriteOffset
	common.Assert(file.FileMetadata.Size >= 0)

	fileMetadataBytes, err := json.Marshal(file.FileMetadata)
	if err != nil {
		log.Err("DistributedCache[FM]::finalizeFile: FileMetadata marshalling failed for %s %+v: %v",
			file.FileMetadata.Filename, file.FileMetadata, err)
		return err
	}

	err = mm.CreateFileFinalize(file.FileMetadata.Filename, fileMetadataBytes, file.FileMetadata.Size,
		file.finalizeEtag)
	if err != nil {
		//
		// Finalize file  should not fail unless the metadata file is deleted/modified out-of-band.
		// That's an unexpected error.
		//
		err = fmt.Errorf("mm.CreateFileFinalize failed %+v: %v", file.FileMetadata, err)
		common.Assert(false, err)
		return err
	}

	log.Debug("DistributedCache[FM]::finalizeFile: Final metadata for %s %+v",
		file.FileMetadata.Filename, file.FileMetadata)

	return nil
}

// length>0 => Do not look in file.StagedChunks for the chunk and do not add newly allocated chunk to
//             file.StagedChunks. This is for random reads where we read only part of the chunk and don't
//             want to cache it.
//
// Else, it returns the requested chunk from the staged chunks, if present, else create a new one and add to the
// staged chunks.
// 'allocateBuf' controls if the StagedChunk returned has its buffer allocated by us. Note that ReadMV()
// returns the buffer where the data is read by the GetChunk() RPC, so we don't want a pre-allocated buffer
// in that case.

func (file *DcacheFile) getChunk(chunkIdx, chunkOffset, length int64, allocateBuf bool) (*StagedChunk, bool, error) {
	log.Debug("DistributedCache::getChunk: file: %s, chunkIdx: %d, chunkOffset: %d, length: %d",
		file.FileMetadata.Filename, chunkIdx, chunkOffset, length)

	if chunkIdx < 0 {
		common.Assert(false, chunkIdx)
		return nil, false, errors.New("ChunkIdx is less than 0")
	}

	//
	// If already present in StagedChunks, return that.
	// We lookup cache only if length==0, which signifies sequential read.
	// Since we don't support random writes, writes always pass length as 0..
	//
	if length == 0 {
		file.chunkLock.RLock()
		if chunk, ok := file.StagedChunks[chunkIdx]; ok {
			//
			// Increment chunk refcount before returning to the caller.
			// Once caller is done with the chunk it must call file.releaseChunk()
			//
			chunk.RefCount.Add(1)
			file.chunkLock.RUnlock()
			return chunk, true, nil
		}

		file.chunkLock.RUnlock()
	}

	//
	// Check once more under lock.
	// We don't need the lock for uncached chunks.
	//
	if length == 0 {
		file.chunkLock.Lock()
		defer file.chunkLock.Unlock()

		if chunk, ok := file.StagedChunks[chunkIdx]; ok {
			chunk.RefCount.Add(1)
			return chunk, true, nil
		}
	}

	// Else, allocate a new staged chunk.
	chunk, err := NewStagedChunk(chunkIdx, chunkOffset, length, file, allocateBuf)
	if err != nil {
		return nil, false, err
	}

	// IsBufExternal is always true in here as the allocation of this buffer is decided by Replication manager
	// when the actual chunk is read, where the buffer is allocated from the bufferPool only when reading the chunk
	// from the local RV, So based on the response of ReadMV request, we will decide the buffer is external or not.
	common.Assert(chunk.IsBufExternal == !allocateBuf, chunk.IsBufExternal)
	common.Assert(chunk.IsBufExternal == (chunk.Buf == nil), chunk.IsBufExternal, len(chunk.Buf))

	// Add it to the StagedChunks, and return.
	if length == 0 {
		file.StagedChunks[chunkIdx] = chunk
		chunk.SavedInMap.Store(true)
	}

	chunk.RefCount.Add(1)
	return chunk, false, nil
}

// length==0 => Caller wants to read the entire chunk. This signifies sequential read and file.getChunk()
//              can return a chunk from file.StagedChunks if present and if not present and it creates a new
//              chunk, it adds it to file.StagedChunks.
// length>0 =>  Caller wants to read only 'length' bytes from the chunk (at 'chunkOffset'). This signifies
//				random read and such chunks are not returned from or added to file.StagedChunks.

func (file *DcacheFile) getChunkForRead(chunkIdx, chunkOffset, length int64) (*StagedChunk, error) {
	log.Debug("DistributedCache::getChunkForRead: file: %s, chunkIdx: %d, chunkOffset: %d, length: %d",
		file.FileMetadata.Filename, chunkIdx, chunkOffset, length)

	common.Assert(chunkIdx >= 0, chunkIdx, chunkOffset, length)
	common.Assert(chunkOffset >= 0, chunkIdx, chunkOffset, length)
	common.Assert(length >= 0 && length <= file.FileMetadata.FileLayout.ChunkSize, chunkIdx, chunkOffset, length)
	common.Assert(chunkOffset+length <= file.FileMetadata.FileLayout.ChunkSize, chunkIdx, chunkOffset, length)

	//
	// For read chunk, we use the buffer returned by the GetChunk() RPC, that saves an extra copy.
	//
	chunk, loaded, err := file.getChunk(chunkIdx, chunkOffset, length, false /* allocateBuf */)
	if err == nil {
		if !loaded {
			// Brand new staged chunk, could not have been scheduled for read already.
			common.Assert(!chunk.XferScheduled.Load())
			// For read chunks chunk.Len is the amount of data that must be read into this chunk.
			chunk.Len = getChunkSize(chunkIdx*file.FileMetadata.FileLayout.ChunkSize, file)
		}
		// There's no point in having a chunk and not reading anything on to it.
		common.Assert(chunk.Len > 0, chunk.Len)
	}
	// TODO: Assert that number of staged chunks is less than fileIOManager.numReadAheadChunks.

	return chunk, err
}

func (file *DcacheFile) getChunkForWrite(chunkIdx int64) (*StagedChunk, error) {
	log.Debug("DistributedCache::getChunkForWrite: file: %s, chunkIdx: %d", file.FileMetadata.Filename, chunkIdx)

	common.Assert(chunkIdx >= 0)

	chunk, loaded, err := file.getChunk(chunkIdx, 0, 0, true /* allocateBuf */)
	// TODO: Assert that number of staged chunks is less than fileIOManager.numStagingChunks.
	//
	// For write chunks chunk.Len is the amount of valid data in the chunk. It starts at 0 and updated as user
	// data is copied to the chunk.
	//
	if err == nil && !loaded {
		// Brand new chunk must start with chunk.Len == 0.
		common.Assert(chunk.Len == 0, chunk.Len)
		// Brand new staged chunk, could not have been scheduled for write already.
		common.Assert(!chunk.XferScheduled.Load())
	}

	return chunk, err
}

// Load chunk from staged chunks.
func (file *DcacheFile) loadChunk(chunkIdx int64) (*StagedChunk, error) {
	common.Assert(chunkIdx >= 0)

	if chunk, ok := file.StagedChunks[chunkIdx]; ok {
		return chunk, nil
	}

	return nil, fmt.Errorf("Chunkidx %s not found in staged chunks for file %s",
		chunkIdx, file.FileMetadata.Filename)
}

// Remove chunk from staged chunks.
func (file *DcacheFile) removeChunk(chunkIdx int64) {
	log.Debug("DistributedCache::removeChunk: removing staged chunk, file: %s, chunk idx: %d",
		file.FileMetadata.Filename, chunkIdx)

	file.chunkLock.Lock()
	defer file.chunkLock.Unlock()

	chunk, ok := file.StagedChunks[chunkIdx]
	if !ok {
		log.Err("DistributedCache::removeChunk: chunk not found, file: %s, chunk idx: %d",
			file.FileMetadata.Filename, chunkIdx)
		common.Assert(false, file.FileMetadata.Filename, chunkIdx)
		return
	}

	//
	// Drop the chunk refcount and if it becomes 0, free the chunk buffer and remove from StagedChunks map.
	//
	if file.releaseChunk(chunk) {
		delete(file.StagedChunks, chunkIdx)
	}
}

// Release buffer for the staged chunk.
func (file *DcacheFile) releaseChunk(chunk *StagedChunk) bool {
	log.Debug("DistributedCache::releaseChunk: releasing buffer for staged chunk, file: %s, chunk idx: %d, refcount: %d, external: %v",
		file.FileMetadata.Filename, chunk.Idx, chunk.RefCount.Load(), chunk.IsBufExternal)

	// Only the last user will attempt to free the chunk.
	common.Assert(chunk.RefCount.Load() > 0, chunk.Idx, chunk.RefCount.Load())
	if chunk.RefCount.Add(-1) != 0 {
		return false
	}

	//
	// If buffer is allocated by NewStagedChunk(), free it to the pool, else it's an external buffer
	// returned by ReadMV(), just drop our reference and let GC free it.
	//
	if chunk.IsBufExternal {
		chunk.Buf = nil
	} else {
		dcache.PutBuffer(chunk.Buf)
	}

	return true
}

// Read Chunk data from the file at 'offset' and 'length' bytes and returns the StagedChunk containing the
// requested data. If length is 0, the entire chunk containing file data at 'offset' is read. This is normally
// the case when reading sequentially, while for random reads we read only as much as the user asked for.
// If the chunk is already in file.StagedChunks, it is returned else a new chunk is created, and data is read
// from the file into that chunk.
//
// Sync true: Schedules and waits for the download to complete.
// Sync false: Schedules the read. This is the readahead path.

func (file *DcacheFile) readChunk(offset, length int64, sync bool) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)
	chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)

	log.Debug("DistributedCache::readChunk: file: %s, chunkIdx: %d, chunkOffset: %d, length: %d, sync: %t",
		file.FileMetadata.Filename, chunkIdx, chunkOffset, length, sync)

	//
	// If this chunk is already staged, return the staged chunk else create a new chunk, add to the staged
	// chunks list and return. The chunk download is not yet scheduled.
	//
	chunk, err := file.getChunkForRead(chunkIdx, chunkOffset, length)
	if err != nil {
		return chunk, err
	}

	// This will be a no-op if this chunk is already read from dcache.
	scheduleDownload(chunk, file)

	if sync {
		err = <-chunk.Err

		if err != nil {
			log.Err("DistributedCache::readChunk: Failed, file: %s, chunkIdx: %d, chunkOffset: %d, length: %d, sync: %t",
				file.FileMetadata.Filename, chunkIdx, chunkOffset, length, sync)

			// Requeue the error for whoever reads thus chunk next.
			chunk.Err <- err
		}
	}

	return chunk, err
}

// Reads the chunk and also schedules the downloads for the readahead chunks.
// Returns the StagedChunk containing data at 'offset' in the file.
//
// Note: This reads the entire chunk into an allocated StagedChunk and adds it to the StagedChunks map.
//       This is suitable for sequential read patterns where readahead is beneficial and it's useful
//       to save the chunk in StagedChunks map for subsequent reads.
//       See readChunkNoReadAhead() for a version that does not do readahead and does not save the chunk
//       in StagedChunks map. That version is suitable for random read patterns.

func (file *DcacheFile) readChunkWithReadAhead(offset int64) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)

	log.Debug("DistributedCache::readChunkWithReadAhead: file: %s, offset: %d, chunkIdx: %d",
		file.FileMetadata.Filename, offset, chunkIdx)

	//
	// Schedule downloads for the readahead chunks. The chunk at chunkIdx is to be read synchronously,
	// for the remaining we do async/readahead read.
	// We do it only when reading the start of a chunk.
	//
	if isOffsetChunkStarting(offset, &file.FileMetadata.FileLayout) {
		//
		// Start readahead after the last chunk readahead by prev read calls.
		// How many more chunks can we readahead?
		// We are allowed to cache upto fileIOMgr.numReadAheadChunks chunks per file and file.StagedChunks
		// are already cached.
		// We update lastReadaheadChunkIdx inside exclusive chunk lock to avoid duplicate readahead
		// by multiple threads reading sequentially.
		// If the readahead fails we won't read ahead those chunks again, but that's ok.
		//
		file.chunkLock.Lock()
		readAheadCount := int64(fileIOMgr.numReadAheadChunks - len(file.StagedChunks))
		common.Assert(readAheadCount >= 0, readAheadCount, len(file.StagedChunks))

		readAheadEndChunkIdx := min(chunkIdx+readAheadCount,
			getChunkIdxFromFileOffset(file.FileMetadata.Size-1, &file.FileMetadata.FileLayout))
		common.Assert(readAheadEndChunkIdx >= chunkIdx, readAheadEndChunkIdx, chunkIdx)

		readAheadStartChunkIdx := max(file.lastReadaheadChunkIdx.Load()+1, chunkIdx+1)
		if readAheadEndChunkIdx > readAheadStartChunkIdx {
			file.lastReadaheadChunkIdx.Store(readAheadEndChunkIdx)
		}
		file.chunkLock.Unlock()

		for i := readAheadStartChunkIdx; i <= readAheadEndChunkIdx; i++ {
			_, err := file.readChunk(i*file.FileMetadata.FileLayout.ChunkSize, 0, false /* sync */)
			if err != nil {
				return nil, err
			}
		}
	}

	// Now the actual chunk, unlike readahead chunks we wait for this one to download.
	chunk, err := file.readChunk(offset, 0, true /* sync */)
	return chunk, err
}

// Reads 'length' bytes from file at 'offset' and returns the StagedChunk containing the requested data.
//
// Note: This has following differences from readChunkWithReadAhead():
//       1. It does not read the entire chunk, only [offset, offset+length) bytes are read.
//       2. It does not save the chunk in StagedChunks map, so it is not available for subsequent reads.
//       3. It does not do readahead of subsequent chunks.
//
// This is suitable for random read patterns where readahead is not beneficial and we don't want to
// save the chunk in StagedChunks map for subsequent reads.

func (file *DcacheFile) readChunkNoReadAhead(offset, length int64) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)
	_ = chunkIdx
	chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)
	_ = chunkOffset

	log.Debug("DistributedCache::readChunkNoReadAhead: file: %s, offset: %d, length: %d, chunkIdx: %d, chunkOffset: %d",
		file.FileMetadata.Filename, offset, length, chunkIdx, chunkOffset)

	// length must not be 0 to signify random read.
	common.Assert(length > 0 && length <= file.FileMetadata.FileLayout.ChunkSize, length)

	// Now the actual chunk, unlike readahead chunks we wait for this one to download.
	chunk, err := file.readChunk(offset, length, true /* sync */)
	return chunk, err
}

// Creates/return the chunk that is ready to be written.
// Also responsible for releasing the chunks, if the chunks are greater than staging area chunks.
func (file *DcacheFile) CreateOrGetStagedChunk(offset int64) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)

	log.Debug("DistributedCache::CreateOrGetStagedChunk: file: %s, offset: %d, chunkIdx: %d",
		file.FileMetadata.Filename, offset, chunkIdx)

	chunk, err := file.getChunkForWrite(chunkIdx)
	if err != nil {
		return chunk, err
	}

	return chunk, nil
}

func scheduleDownload(chunk *StagedChunk, file *DcacheFile) {
	// chunk.Len is the amount of bytes to download, cannot be 0.
	common.Assert(chunk.Len > 0)

	if !chunk.XferScheduled.Swap(true) {
		// Cannot be overwriting a dirty staged chunk.
		common.Assert(!chunk.Dirty.Load())
		// Cannot be reading an already up-to-date chunk.
		common.Assert(!chunk.UpToDate.Load())

		fileIOMgr.wp.queueWork(file, chunk, true /* get_chunk */)
	}
}

func scheduleUpload(chunk *StagedChunk, file *DcacheFile) {
	// chunk.Len is the amount of bytes to upload, cannot be 0.
	common.Assert(chunk.Len > 0)

	if !chunk.XferScheduled.Swap(true) {
		// Only dirty staged chunk should be written to dcache.
		common.Assert(chunk.Dirty.Load())
		// Up-to-date chunk should not be written.
		common.Assert(!chunk.UpToDate.Load())

		// Offset of the last committed byte.
		file.committedTillOffset = chunk.Offset + chunk.Len - 1

		fileIOMgr.wp.queueWork(file, chunk, false /* get_chunk */)
	}
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
