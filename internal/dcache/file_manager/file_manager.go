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
	// TODO: 256 readahead chunks perform better, let's see if we want to reduce it.
	//
	numReadAheadChunks := 256

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

	// Initialize chunk IO tracker parameters.
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

	// Read pattern tracker for detecting read pattern (sequential/random).
	// Only valid for files opened for reading, nil for files opened for writing.
	RPT *RPTracker

	// Last byte written + 1, also the next write offset we expect in case of sequential writes.
	// We don't support overwrites but we support slightly out-of-order writes to enable parallel
	// processing of writes issued by FUSE as a result of large application writes.
	// We allow writes only within a window of maxWriteOffset +/- 10MiB, where 10MiB comes from
	// two factors:
	// - Fuse uses max 1MiB IO size.
	// - Libfuse has default 10 worker threads.
	// Such writes are tracked using ChunkIOTracker in each StagedChunk.
	maxWriteOffset int64

	// Error encountered during write, if any. It needs to be atomic since multiple threads can be
	// accessing it.
	writeErr atomic.Value

	// Are we enforcing strict sequential writes on this file?
	// This starts as false but if we see small or non block size aligned writes, we set this to true.
	// When this is true we don't allow any out-of-order writes but only strictly sequential writes, i.e.,
	// next write must start immediately after the prev write. This is needed as we cannot track writes
	// which are not multiple of MinTrackableIOSize, or which do not start at MinTrackableIOSize
	// aligned offsets. This is our last ditch effort to allow writes to a file from applications where
	// we cannot control the IO sizes.
	strictSeqWrites bool

	// Last offset read by prev read + 1, also the next read offset we expect in case of sequential reads.
	// Note the difference from maxWriteOffset above.
	nextReadOffset atomic.Int64

	//
	// Chunk index (inclusive) till which we have done readahead.
	// The actual readahead may or may not have been issued, but these will be issued by some thread.
	// readaheadToBeIssued is the number of chunks for which readahead is yet to be issued. Once readahead
	// is issued for a chunk it's added to StagedChunks map and readaheadToBeIssued is decremented. So the
	// sum of readaheadToBeIssued and len(StagedChunks) is the number of readahead chunks that are either
	// in memory or will be in memory soon.
	//
	lastReadaheadChunkIdx atomic.Int64
	readaheadToBeIssued   atomic.Int64

	// Chunk Idx -> *StagedChunk
	// These are the chunks that are currently cached in memory for this file.
	// These chunks can be either readahead chunks or writeback chunks.
	//
	// This is protected by chunkLock.
	//
	// TODO: Should we share chunks among multiple open handles of the same file?
	StagedChunks map[int64]*StagedChunk
	chunkLock    sync.RWMutex

	// How many max chunks can be allocated for use by this file.
	// Depending on whether the file is opened for read or write, the maxChunks will be set differently.
	// For writers it'll be set to numStagingChunks, for readers it'll be set to numReadAheadChunks plus
	// the window size supported by the read pattern tracker. See NewRPTracker().
	maxChunks int64

	// Semaphore to limit number of in-use chunks.
	// Note that in order to support parallel readers/writers, we need multiple partial chunks to be present
	// at any time. As soon as chunk is "fully used" (fully read/written by application), it can be released.
	// We limit the number of chunks in use to avoid using too much memory.
	// NewStagedChunk() reads from this channel before allocating a new chunk.
	// releaseChunk() signals on this channel when a chunk is freed.
	freeChunks chan struct{}

	// Etag returned by createFileInit(), later used by createFileFinalize() to catch unexpected
	// out-of-band changes to the metadata file between init and finalize.
	finalizeEtag string

	// This cached attr is used for optimizing the REST API calls, when safeDeletes is enabled.
	// It is saved when the file is opened, if there are no more file opens done on that file, the same etag
	// can be used to update the file open count on close.
	attr *internal.ObjAttr
}

// Get the write error encountered during file writes, if any.
func (file *DcacheFile) getWriteError() error {
	val := file.writeErr.Load()
	if val == nil {
		return nil
	}
	err, ok := val.(error)
	_ = ok
	common.Assert(ok, val)
	return err
}

func (file *DcacheFile) initFreeChunks(maxChunks int) {
	// Must be called only once.
	common.Assert(file.freeChunks == nil)
	common.Assert(maxChunks >= min(fileIOMgr.numReadAheadChunks, fileIOMgr.numStagingChunks),
		maxChunks, fileIOMgr.numReadAheadChunks, fileIOMgr.numStagingChunks)

	file.freeChunks = make(chan struct{}, maxChunks)
	file.maxChunks = int64(maxChunks)

	// Fill the semaphore with initial tokens.
	for i := 0; i < maxChunks; i++ {
		file.freeChunks <- struct{}{}
	}
}

// Reads the file data from the given offset and length to buf[].
// It translates the requested offsets into chunks, reads those chunks from distributed cache and copies data from
// those chunks into user buffer.
func (file *DcacheFile) ReadFile(offset int64, buf *[]byte) (bytesRead int, err error) {
	log.Debug("DistributedCache::ReadFile: file: %s, nextReadOffset: %d, offset: %d, length: %d, chunkIdx: %d",
		file.FileMetadata.Filename, file.nextReadOffset.Load(), offset, len(*buf),
		getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize))

	// Read must only be allowed on a properly finalized file, for which size must not be -1.
	common.Assert(int64(file.FileMetadata.Size) >= 0)
	// FUSE sends requests not exceeding 1MiB, put this assert to know if that changes in future.
	common.Assert(len(*buf) <= common.MbToBytes, len(*buf))
	// Files opened for reading must have a valid read patterm tracker.
	common.Assert(file.RPT != nil, file.FileMetadata.Filename)

	if offset >= file.FileMetadata.Size {
		log.Warn("DistributedCache::ReadFile: Read beyond eof. file: %s, offset: %d, length: %d, file size: %d",
			file.FileMetadata.Filename, offset, len(*buf), file.FileMetadata.Size)
		return 0, io.EOF
	}

	// endOffset is 1 + offset of the last byte to be read, but not more than file size.
	endOffset := min(offset+int64(len(*buf)), file.FileMetadata.Size)
	// Catch wraparound.
	common.Assert(endOffset >= offset, endOffset, offset)

	bufOffset := 0

	// Update sequential/random pattern tracker with this new IO and get the current access pattern.
	accessPattern := file.RPT.Update(offset, endOffset-offset)

	//
	// This read starts where the prev one ended.
	// These are strictly sequential reads, which we try to optimize for, in the absence of sequential
	// (but parallel) reads.
	//
	isStrictlySequential := (offset == file.nextReadOffset.Swap(endOffset))

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
		// Offset within the chunk corresponding to the file offset.
		chunkOffset := getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
		//
		// Bytes to be read from the current chunk.
		// Minimum of what user asked and what the current chunk can provide.
		//
		common.Assert(int64(len(*buf)-bufOffset) >= endOffset-offset,
			len(*buf), bufOffset, endOffset, offset)
		readSize := min(endOffset-offset, file.FileMetadata.FileLayout.ChunkSize-chunkOffset)

		// Reads not multiple of or not aligned with MinTrackableIOSize cannot be tracked by ChunkIOTracker.
		isSequential := (accessPattern == 1 &&
			(offset%GetMinTrackableIOSize() == 0) &&
			(readSize%GetMinTrackableIOSize() == 0))

		//
		// accessPattern will be +1 for confirmed sequential, -1 for confirmed random, and 0 for unsure.
		// 0 is not very common as we will soon confirm either sequential/random pattern.
		// 0 means that we might be transitioning from sequential to random, or maybe sequential pattern
		// is shifting to another region of the file. In any case it's a good time to flush all cached
		// chunks as they may not be relevant for the new access pattern.
		//
		if accessPattern == 0 {
			file.removeAllChunks(true /* needLock */)
		}

		if isSequential || isStrictlySequential || (accessPattern == 0) {
			// Perform readahead only if the read pattern has proven to be sequential.
			unsure := (accessPattern == 0)
			chunk, err = file.readChunkWithReadAhead(offset, unsure)
			if err != nil {
				// Chunk != nil means we had allocated a new chunk, but failed to read into it.
				if chunk != nil {
					// Release our refcount on the chunk.
					file.releaseChunk(chunk)
				}
				return 0, err
			}

			// For sequential reads we must be reading the entire chunk.
			common.Assert(chunk.Offset == 0 && chunk.Len == getChunkSize(offset, file),
				chunk.Idx, chunk.Offset, chunk.Len, file.FileMetadata.Filename)
			//
			// And the chunk must be saved in StagedChunks map.
			// We cannot assert for the following as the chunk may be removed from the map by the time we
			// reach here. e.g., one case I saw is some other thread called removeAllChunks() above.
			//
			// common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)
		} else {
			//
			// For random read pattern we don't do readahead, and also don't save the chunk in StagedChunks map,
			// but we do lookup StagedChunks, so it's possible that we find the chunk in StagedChunks map.
			// TODO: Remove the following commented assert after some time.
			//
			chunk, err = file.readChunkNoReadAhead(offset, readSize)
			if err != nil {
				// Chunk != nil means we had allocated a new chunk, but failed to read into it.
				if chunk != nil {
					// Release our refcount on the chunk.
					file.releaseChunk(chunk)
				}
				return 0, err
			}
			/*
				common.Assert(chunk.SavedInMap.Load() == false, chunk.Idx, file.FileMetadata.Filename)
				// We only read what we need.
				common.Assert(chunk.Offset == chunkOffset && chunk.Len == readSize,
					chunk.Offset, chunkOffset, chunk.Len, readSize, chunk.Idx, file.FileMetadata.Filename)
			*/
			//
			// If it's a cached chunk, it must have been fully read by some previous sequential read, else
			// it'll have just the requested data.
			//
			common.Assert((chunk.Offset == 0 && chunk.Len == getChunkSize(offset, file)) ||
				(chunk.Offset == chunkOffset && chunk.Len == readSize),
				chunk.Offset, chunkOffset, offset, chunk.Len, readSize, chunk.Idx, file.FileMetadata.Filename)
		}

		// We should only be reading from an up-to-date chunk (that has been successfully read from dcache).
		common.Assert(chunk.UpToDate.Load())
		// Offset+Len cannot be pointing beyond ChunkSize.
		common.Assert((chunk.Offset+chunk.Len) <= file.FileMetadata.FileLayout.ChunkSize,
			chunk.Offset, chunk.Len, chunk.Idx, file.FileMetadata.FileLayout.ChunkSize)

		// This chunk has chunk.Len valid bytes starting @ chunk.Offset, we cannot be reading past those.
		common.Assert((chunkOffset+readSize) <= (chunk.Offset+chunk.Len),
			chunkOffset, readSize, chunk.Offset, chunk.Len, chunk.Idx, file.FileMetadata.Filename)

		// We must be holding a refcount on the chunk.
		common.Assert(chunk.RefCount.Load() > 0, chunk.Idx, chunk.RefCount.Load())

		//
		// Note/TODO: If we use the fuse low level API we can avoid this copy and return the chunk.Buf
		//            directly in the fuse response.
		//
		copied := copy((*buf)[bufOffset:], chunk.Buf[(chunkOffset-chunk.Offset):chunk.Len])

		// Must copy at least one byte.
		common.Assert(copied > 0, chunkOffset,
			bufOffset, chunk.Offset, chunk.Len, len(*buf), chunk.Idx, file.FileMetadata.Filename)

		offset += int64(copied)
		bufOffset += copied

		chunkFullyRead := false

		if isSequential {
			chunkFullyRead = chunk.IOTracker.MarkAccessed(chunkOffset, int64(copied), chunk.Idx)
		} else if isStrictlySequential {
			chunkFullyRead = (offset == getChunkEndOffset(chunk.Idx, file.FileMetadata.FileLayout.ChunkSize))
		}

		if chunkFullyRead {
			log.Debug("DistributedCache::ReadFile: Chunk fully read (sequential: %v), file: %s, chunkIdx: %d, chunkOffset: %d, chunk.Len: %d, chunk.Offset: %d, copied: %d",
				isSequential, file.FileMetadata.Filename, chunk.Idx, chunkOffset, chunk.Len, chunk.Offset, copied)
			//
			// We are done with this chunk, remove from StagedChunks map.
			// The first reader to observe this will remove it. Note that this just removes the chunk from
			// the map so that new readers won't find it, but it doesn't free the chunk memory. It may still
			// be in use by other readers and the last reader to release the chunk will free the chunk memory.
			//
			removed := file.removeChunk(chunk.Idx)
			_ = removed
			//
			// We cannot assert for the following as the chunk may be removed from the map by the time we
			// reach here. e.g., one case I saw is some other thread called removeAllChunks() above.
			//
			//common.Assert(removed == true, chunk.Idx, file.FileMetadata.Filename)
		}

		// Release our refcount on the chunk. If this is the last reference, it will free the chunk memory.
		file.releaseChunk(chunk)
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
	log.Debug("DistributedCache[FM]::WriteFile: file: %s, maxWriteOffset: %d [%v], offset: %d, length: %d, chunkIdx: %d",
		file.FileMetadata.Filename, file.maxWriteOffset, file.strictSeqWrites, offset, len(buf),
		getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize))

	// DCache files are immutable, all writes must be before first close, by which time file size is not known.
	common.Assert(int64(file.FileMetadata.Size) == -1, file.FileMetadata.Size)
	// FUSE sends requests not exceeding 1MiB, put this assert to know if that changes in future.
	common.Assert(len(buf) <= common.MbToBytes, len(buf))
	// Read patterm tracker must not be present for files opened for writing.
	common.Assert(file.RPT == nil, file.FileMetadata.Filename)
	// We should not be called for 0 byte writes.
	common.Assert(len(buf) > 0)
	common.Assert(file.maxWriteOffset >= 0, file.maxWriteOffset, file.FileMetadata.Filename)

	err := file.getWriteError()

	//
	// Once file has a write error, all subsequent writes (if any) must fail.
	//
	if err != nil {
		log.Err("DistributedCache[FM]::WriteFile: %s, previous write failed: %v",
			file.FileMetadata.Filename, err)
		return err
	}

	//
	// We can only track parallel writes which are aligned on MinTrackableIOSize and are multiple of that size.
	// Those give the best write performance as we can write multiple of them in parallel.
	// If not that, we can only allow strictly sequential writes which can be tracked using maxWriteOffset.
	// Once strict sequential writes are enforced on a file handle, we cannot go back to parallel writes.
	//
	if !file.strictSeqWrites &&
		(offset%int64(GetMinTrackableIOSize()) != 0 || len(buf)%int(GetMinTrackableIOSize()) != 0) {
		log.Debug("DistributedCache[FM]::WriteFile: Enforcing strict sequential writes as offset (%d) or length (%d) is not a multiple of MinTrackableIOSize (%d)",
			offset, len(buf), GetMinTrackableIOSize())
		file.strictSeqWrites = true
	}

	//
	// TODO:
	// What if application skips one or more full chunks while writing?
	// We should not allow the write to succeed as that would create a sparse file which we don't support.
	//

	//
	// We allow writes only within the staging area.
	// Anything before that amounts to overwrite, and anything beyond that is not allowed by our writeback
	// cache limits.
	//
	// TODO: Allowable write range should be a factor of fileIOMgr.numStagingChunks and how much is already
	//       in the cache. Currently we are hardcoding +/- 100MiB range.
	//
	allowableWriteOffsetStart := max(0, file.maxWriteOffset-int64(100)*common.MbToBytes)
	allowableWriteOffsetEnd := file.maxWriteOffset + int64(100)*common.MbToBytes

	// With strict sequential writes we don't allow any out-of-order writes.
	if file.strictSeqWrites {
		allowableWriteOffsetStart = file.maxWriteOffset
		allowableWriteOffsetEnd = file.maxWriteOffset
	}

	//
	// We only support append writes.
	// 'cp' utility uses sparse writes to avoid writing zeroes from the source file to target file, we support
	// that too, hence offset can be greater than maxWriteOffset.
	//
	if offset < allowableWriteOffsetStart {
		err := fmt.Errorf("DistributedCache[FM]::WriteFile: Overwrite unsupported, file: %s, offset: %d, allowed: [%d, %d)",
			file.FileMetadata.Filename, offset, allowableWriteOffsetStart, allowableWriteOffsetEnd)
		log.Err("%v", err)
		file.writeErr.Store(err)
		return syscall.ENOTSUP
	} else if offset > allowableWriteOffsetEnd {
		err := fmt.Errorf("DistributedCache[FM]::WriteFile: Random write unsupported, file: %s, offset: %d, allowed: [%d, %d)",
			file.FileMetadata.Filename, offset, allowableWriteOffsetStart, allowableWriteOffsetEnd)
		log.Err("%v", err)
		file.writeErr.Store(err)
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
		// Offset within the chunk to write.
		chunkOffset := getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)

		// How many bytes we can write to the current chunk.
		writeSize := min(file.FileMetadata.FileLayout.ChunkSize-chunkOffset, (endOffset - offset))

		// Get the StagedChunk that will hold the file data at offset.
		chunk, err := file.CreateOrGetStagedChunk(offset)
		if err != nil {
			err = fmt.Errorf("Failed to get chunk for write, file: %s, chunkIdx: %d: %v",
				file.FileMetadata.Filename,
				getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize), err)
			log.Err("DistributedCache[FM]::WriteFile: %v", err)
			common.Assert(false, err)
			file.writeErr.Store(err)
			return err
		}

		//
		// Detect and disallow overwrites.
		// Strict sequential writes cannot overwrite as we always write at maxWriteOffset.
		//
		if !file.strictSeqWrites && chunk.IOTracker.IsAccessed(chunkOffset, int64(writeSize)) {
			err := fmt.Errorf("DistributedCache[FM]::WriteFile: Overwrite unsupported, file: %s, offset: %d, length: %d, chunkOffset: %d, chunkIdx: %d",
				file.FileMetadata.Filename, offset, writeSize, chunkOffset, chunk.Idx)

			log.Err("%v", err)
			file.writeErr.Store(err)
			file.releaseChunk(chunk)
			return syscall.ENOTSUP
		}

		//
		// This case is mostly for handling 'cp' where it jumps the offset to avoid copying zeroes from
		// source to target file. We fill in the gap with zeroes.
		//
		/*
			if chunkOffset > chunk.Len {
				// TODO: Use a static zero buffer instead of allocating one every time.
				resetBytes := copy(chunk.Buf[chunk.Len:chunkOffset], make([]byte, chunkOffset-chunk.Len))
				common.Assert(resetBytes > 0)
				chunk.Len += int64(resetBytes)
			}
		*/

		copied := copy(chunk.Buf[chunkOffset:], buf[bufOffset:])
		// Must copy at least one byte from the chunk.
		common.Assert(copied > 0, chunkOffset, bufOffset, chunk.Len, len(buf))
		// But not cross chunk boundary.
		common.Assert(int64(copied) <= writeSize,
			copied, writeSize, chunkOffset, file.FileMetadata.FileLayout.ChunkSize)

		offset += int64(copied)
		bufOffset += copied
		atomic.AddInt64(&chunk.Len, int64(copied))

		// Valid chunk data cannot be more than ChunkSize. We don't allow overwrites.
		common.Assert(chunk.Len <= file.FileMetadata.FileLayout.ChunkSize,
			chunk.Len, file.FileMetadata.FileLayout.ChunkSize)

		// Cannot be more than the buffer.
		common.Assert(chunk.Len <= int64(len(chunk.Buf)), chunk.Len, int64(len(chunk.Buf)))

		// At least one byte of user data copied to chunk, it's dirty now and must be written to dcache.
		chunk.Dirty.Store(true)

		//
		// With strict sequential writes only one thread must be calling WriteFile() at any time and hence
		// the chunk.Len must be updated by the amount we just copied.
		//
		if common.IsDebugBuild() {
			if file.strictSeqWrites {
				common.Assert(chunk.Len ==
					getChunkOffsetFromFileOffset(offset-1, file.FileMetadata.FileLayout.ChunkSize)+1,
					fmt.Sprintf("Actual Chunk Len: %d is modified incorrectly, expected chunkLen: %d",
						chunk.Len, getChunkOffsetFromFileOffset(offset-1, file.FileMetadata.FileLayout.ChunkSize)+1))
			}
		}

		//
		// Schedule the upload when a staged chunk is fully written.
		// There's no point in waiting any more. Sooner we write completed chunks, faster we will complete
		// writes.
		//
		chunkFullyWritten := false

		if file.strictSeqWrites {
			chunkFullyWritten = (chunk.Len == file.FileMetadata.FileLayout.ChunkSize)
		} else {
			log.Debug("DistributedCache[FM]::WriteFile: Marking blocks accessed, file: %s, offset: %d, length: %d, chunkOffset: %d, chunk.Len: %d, chunkIdx: %d, refcount: %d",
				file.FileMetadata.Filename, offset-int64(copied), copied, chunkOffset,
				chunk.Len, chunk.Idx, chunk.RefCount.Load())

			chunkFullyWritten = chunk.IOTracker.MarkAccessed(chunkOffset, int64(copied), chunk.Idx)

			// If chunkFullyWritten is true, chunk must have been fully written.
			common.Assert(!chunkFullyWritten || (chunk.Len == file.FileMetadata.FileLayout.ChunkSize),
				chunkFullyWritten, chunk.Len, file.FileMetadata.FileLayout.ChunkSize,
				chunk.Idx, file.FileMetadata.Filename)
		}

		if chunkFullyWritten {
			log.Debug("DistributedCache[FM]::WriteFile: Fully written chunk, file: %s, offset: %d, length: %d, chunkIdx: %d, refcount: %d",
				file.FileMetadata.Filename, offset, len(buf), chunk.Idx, chunk.RefCount.Load())

			common.Assert(chunk.Len == file.FileMetadata.FileLayout.ChunkSize,
				chunk.Len, file.FileMetadata.FileLayout.ChunkSize, chunk.Idx, file.FileMetadata.Filename)

			//
			// Since we only support sequential writes, all but the last chunk will be uploaded from here.
			// The last chunk may be partial and will be uploaded by SyncFile().
			//
			scheduleUpload(chunk, file)
		}

		//
		// We are done using the chunk, drop our ref.
		// This should not free the chunk as it's still used by the StagedChunks map which holds a refcount
		// on it. That refcount will be dropped when the upload completes.
		//
		file.releaseChunk(chunk)
	}

	// Must write complete data.
	common.Assert(offset == endOffset, offset, endOffset)
	// and only that much.
	common.Assert(bufOffset == len(buf), bufOffset, len(buf))

	// Next write expected at offset maxWriteOffset.
	common.AtomicMaxInt64(&file.maxWriteOffset, offset)

	return nil
}

// Sync Buffers for the file with dcache/azure.
// This call can come when user application calls fsync()/close().
func (file *DcacheFile) SyncFile() error {
	err := file.getWriteError()
	var ret error
	chunks := make([]*StagedChunk, 0)

	//
	// Go over all the staged chunks and write to cache if not already done.
	// Note that we keep fileIOMgr.numStagingChunks number of chunks per file. As chunks are fully written
	// we upload them to cache, so only the last incomplete chunk would be actually written by the following loop.
	//
	file.chunkLock.RLock()

	//
	// No need to upload any chunk if file write has already failed.
	//
	if err != nil {
		err = fmt.Errorf("DistributedCache[FM]::SyncFile: %s, failed write, %d chunks, file.writeErr: %v",
			file.FileMetadata.Filename, len(file.StagedChunks), err)
		log.Err("%v", err)
		file.chunkLock.RUnlock()
		return err
	}

	log.Debug("DistributedCache[FM]::SyncFile: %s, syncing %d chunks",
		file.FileMetadata.Filename, len(file.StagedChunks))

	for chunkIdx, chunk := range file.StagedChunks {
		_ = chunkIdx
		common.Assert(chunkIdx == chunk.Idx, chunkIdx, chunk.Idx)

		log.Debug("DistributedCache[FM]::SyncFile: file: %s, chunkIdx: %d, chunkLen: %d",
			file.FileMetadata.Filename, chunk.Idx, chunk.Len)

		chunkStartOffset := getChunkStartOffset(chunk.Idx, file.FileMetadata.FileLayout.ChunkSize)
		chunkEndOffset := chunkStartOffset + chunk.Len
		// if maxWriteOffset is at chunk end, this must be the last chunk.
		isLastChunk := file.maxWriteOffset == chunkEndOffset

		//
		// Uploading partial chunks in the middle of the file is recipe for data corruption.
		//
		if !isLastChunk && chunk.Len < file.FileMetadata.FileLayout.ChunkSize {
			err := fmt.Errorf("DistributedCache[FM]::SyncFile: Partial non-last chunk. file: %s, chunkIdx: %d, chunkLen: %d, file size: %d",
				file.FileMetadata.Filename, chunk.Idx, chunk.Len, file.maxWriteOffset)
			log.Err("%v", err)
			return err
		}

		// Schedule chunk upload if not already done.
		if scheduleUpload(chunk, file) {
			//
			// Completed chunks must have been uploaded by WriteFile().
			//
			common.Assert(chunk.Len < file.FileMetadata.FileLayout.ChunkSize,
				chunk.Len, file.FileMetadata.FileLayout.ChunkSize, chunk.Idx, file.FileMetadata.Filename)
		}

		chunks = append(chunks, chunk)
	}
	file.chunkLock.RUnlock()

	// Now wait for all chunks to be uploaded.
	for _, chunk := range chunks {
		err = <-chunk.Err
		if err != nil {
			ret = fmt.Errorf("DistributedCache[FM]::SyncFile: file: %s, chunkIdx: %d, chunkLen: %d, failed: %v",
				file.FileMetadata.Filename, chunk.Idx, chunk.Len, err)
		}
	}

	common.Assert(ret == nil, file.FileMetadata.Filename, ret)

	return ret
}

// Close and Finalize the file. writes are failed after successful file close.
func (file *DcacheFile) CloseFile() error {
	log.Debug("DistributedCache[FM]::CloseFile: %s", file.FileMetadata.Filename)

	//
	// We stage application writes into StagedChunk and upload only when we have a full chunk.
	// In case of last chunk being partial, we need to upload it now.
	// SyncFile() will fail if the write had failed and some of the chunks could not be uploaded.
	//
	err := file.SyncFile()

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
	file.chunkLock.Lock()
	defer file.chunkLock.Unlock()

	log.Debug("DistributedCache[FM]::ReleaseFile: Releasing all %d staged chunks for file %s",
		len(file.StagedChunks), file.FileMetadata.Filename)

	for chunkIdx, chunk := range file.StagedChunks {
		_ = chunkIdx
		common.Assert(chunkIdx == chunk.Idx, chunkIdx, chunk.Idx)
		common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)
		//
		// Dirty chunks must have been uploaded and removed from StagedChunks.
		// Full chunks would be uploaded by WriteFile() and the last incomplete chunk (if any) by SyncFile().
		// Also SyncFile() must have waited for all dirty chunks to be uploaded and removed from StagedChunks.
		//
		common.Assert(chunk.Dirty.Load() == false, chunk.Idx, file.FileMetadata.Filename)

		//
		// TODO: assert for each chunk that err is closed. currently not doing it as readahead chunks
		//       error channel might be opened.
		//

		delete(file.StagedChunks, chunkIdx)

		chunk.SavedInMap.Store(false)

		//
		// Refcount corresponding to StagedChunks must be present.
		// For the case when file is closed after this chunk download was scheduled but before the chunk is
		// downloaded, we will have the download refcount too.
		//
		common.Assert(chunk.RefCount.Load() == 1 || chunk.RefCount.Load() == 2, chunk.Idx, chunk.RefCount.Load())
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
	file.FileMetadata.Size = file.maxWriteOffset
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

// noCache => Do not add newly allocated chunk to file.StagedChunks. This is for random reads where we read
//            only part of the chunk and don't want to cache it. But, we do lookup in file.StagedChunks to
//            see if the chunk is already present.
//
// Else, it returns the requested chunk from the staged chunks, if present, else create a new one and add to the
// staged chunks.
// 'allocateBuf' controls if the StagedChunk returned has its buffer allocated by us. Note that ReadMV()
// returns the buffer where the data is read by the GetChunk() RPC, so we don't want a pre-allocated buffer
// in that case.
// Since we don't support random writes, writes always pass noCache=false.
//
// The middle return value is true if the chunk is an existing chunk, else false if a freshly created chunk
// is created.
//
// The returned chunk has its refcount incremented, the caller must call file.releaseChunk() when done with it.
// Additionally, if the chunk is newly created, it has one extra refcount to protect against freeing before
// the chunk is uploaded/downloaded. The caller must drop that ref when they are done uploading/downloading.

func (file *DcacheFile) getChunk(chunkIdx, chunkOffset, length int64, noCache, allocateBuf bool) (*StagedChunk, bool, error) {
	log.Debug("DistributedCache::getChunk: file: %s, chunkIdx: %d, chunkOffset: %d, length: %d, allocateBuf: %v, noCache: %v",
		file.FileMetadata.Filename, chunkIdx, chunkOffset, length, allocateBuf, noCache)

	common.Assert(chunkIdx >= 0, chunkIdx, chunkOffset, length, noCache, file.FileMetadata.Filename)
	common.Assert(chunkOffset+length <= file.FileMetadata.FileLayout.ChunkSize,
		chunkIdx, chunkOffset, length, file.FileMetadata.FileLayout.ChunkSize, noCache, file.FileMetadata.Filename)

	//
	// If already present in StagedChunks, return that.
	// Note that we lookup in StagedChunks even if noCache==true, this is to avoid multiple chunks being
	// allocated for the same chunkIdx. In case of true random reads the chunk will mostly not be present
	// in StagedChunks, but the lookup overhead is not so important for random reads. For some access
	// patterns we may have it in the cache, so we lookup anyway.
	//
	// Note: After the check below some other thread may add the chunk to StagedChunks, in which case we
	//       will have dup chunks, one of which will be cached. This doesn't cause any correctness issues,
	//       but wastes some memory. This cannot be avoided as we can always have a thread allocate a
	//       uncached chunk and then have another thread add a cached chunk for the same idx to StagedChunks.
	//       The check below is a best effort check.
	//
	file.chunkLock.RLock()
	if chunk, ok := file.StagedChunks[chunkIdx]; ok {
		//
		// Increment chunk refcount before returning to the caller.
		// Once caller is done with the chunk it must call file.releaseChunk().
		//
		chunk.RefCount.Add(1)
		common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)
		common.Assert(chunk.Idx == chunkIdx, chunk.Idx, chunkIdx, file.FileMetadata.Filename)
		// One for the map and one for the caller.
		common.Assert(chunk.RefCount.Load() >= 2, chunk.Idx, chunk.RefCount.Load(), file.FileMetadata.Filename)
		file.chunkLock.RUnlock()
		return chunk, true, nil
	}
	file.chunkLock.RUnlock()

	//
	// NewStagedChunk() can wait if no free chunk is available, so do it before taking the lock.
	// In the rare case that another thread adds the chunk to StagedChunks meanwhile, we will
	// release the newly allocated chunk. That should be rare enough to avoid too many wasted calls.
	//
	chunk, err := NewStagedChunk(chunkIdx, chunkOffset, length, file, allocateBuf)
	if err != nil {
		return nil, false, err
	}

	//
	// Before adding a new chunk to StagedChunks we must ensure that no other thread has added it meanwhile.
	// We don't need the lock for uncached chunks.
	//
	if !noCache {
		file.chunkLock.Lock()
		defer file.chunkLock.Unlock()

		if chunk1, ok := file.StagedChunks[chunkIdx]; ok {
			common.Assert(chunk1.SavedInMap.Load() == true, chunk1.Idx, file.FileMetadata.Filename)
			common.Assert(chunk1.Idx == chunkIdx, chunk1.Idx, chunkIdx, file.FileMetadata.Filename)
			chunk1.RefCount.Add(1)

			// Release the newly allocated chunk.
			log.Debug("DistributedCache::getChunk: Releasing newly allocated chunk as another thread added it meanwhile, file: %s, chunkIdx: %d",
				file.FileMetadata.Filename, chunkIdx)
			common.Assert(chunk.RefCount.Load() == 1, chunk1.Idx, chunk1.RefCount.Load(), file.FileMetadata.Filename)
			file.releaseChunk(chunk)

			// One for the map and one for the caller.
			common.Assert(chunk1.RefCount.Load() >= 2, chunk1.Idx, chunk1.RefCount.Load(), file.FileMetadata.Filename)
			return chunk1, true, nil
		}
	}

	// IsBufExternal is always true in here as the allocation of this buffer is decided by Replication manager
	// when the actual chunk is read, where the buffer is allocated from the bufferPool only when reading the chunk
	// from the local RV, So based on the response of ReadMV request, we will decide the buffer is external or not.
	common.Assert(chunk.IsBufExternal == !allocateBuf, chunk.IsBufExternal)
	common.Assert(chunk.IsBufExternal == (chunk.Buf == nil), chunk.IsBufExternal, len(chunk.Buf))

	// Add it to the StagedChunks, and return.
	if !noCache {
		file.StagedChunks[chunkIdx] = chunk
		chunk.SavedInMap.Store(true)
		// Hold one refcount for the map. This is dropped by removeChunk().
		chunk.RefCount.Add(1)
	}

	//
	// This newly created chunk will need to be downloaded/uploaded, one refcount to protect against freeing
	// before the chunk is uploaded/downloaded.
	// This ref must be dropped by the caller when they are done downloading/uploading the chunk.
	// Only the caller that gets the fresh chunk, will perform the download (upload is handled differently), and
	// must drop this ref once download is complete.
	//
	// Note that NewStagedChunk() already adds the caller's refcount which they must drop once they are done
	// using the chunk.
	//
	chunk.RefCount.Add(1)

	// Some sanity assertions for the newly created chunk.
	common.Assert(!chunk.XferScheduled.Load(), chunk.Idx, chunk.Len, file.FileMetadata.Filename)
	common.Assert(!chunk.Dirty.Load(), chunk.Idx, chunk.Len, file.FileMetadata.Filename)
	common.Assert(!chunk.UpToDate.Load(), chunk.Idx, chunk.Len, file.FileMetadata.Filename)
	common.Assert(chunk.Idx == chunkIdx, chunk.Idx, chunkIdx, file.FileMetadata.Filename)
	// Caller has the right to one refcount (+ one download/upload ref), they must call file.releaseChunk() when done.
	common.Assert(chunk.RefCount.Load() >= 2, chunk.Idx, chunk.RefCount.Load(), file.FileMetadata.Filename)

	return chunk, false, nil
}

// length==0 => Caller wants to read the entire chunk. This signifies sequential read and file.getChunk()
//              can return a chunk from file.StagedChunks if present and if not present and it creates a new
//              chunk, it adds it to file.StagedChunks.
// length>0 =>  Caller wants to read only 'length' bytes from the chunk (at 'chunkOffset'). This signifies
//				random read and such chunks are not returned from or added to file.StagedChunks.
//
// The middle return value is true if the chunk is an existing chunk, else false if a freshly created chunk
// is created.

func (file *DcacheFile) getChunkForRead(chunkIdx, chunkOffset, length int64) (*StagedChunk, bool, error) {
	log.Debug("DistributedCache::getChunkForRead: file: %s, chunkIdx: %d, chunkOffset: %d, length: %d",
		file.FileMetadata.Filename, chunkIdx, chunkOffset, length)

	common.Assert(chunkIdx >= 0, chunkIdx, chunkOffset, length)
	common.Assert(chunkOffset >= 0, chunkIdx, chunkOffset, length)
	common.Assert(length >= 0 && (chunkOffset+length) <= file.FileMetadata.FileLayout.ChunkSize,
		chunkIdx, chunkOffset, length)

	noCache := (length != 0)
	if length == 0 {
		//
		// length==0 means caller wants to read the entire chunk.
		// For last chunk 'length' returned can be less than ChunkSize.
		//
		length = getChunkSize(chunkIdx*file.FileMetadata.FileLayout.ChunkSize, file)
	}

	//
	// For read chunk, we use the buffer returned by the GetChunk() RPC, that saves an extra copy.
	//
	chunk, isExisting, err := file.getChunk(chunkIdx, chunkOffset, length, noCache, false /* allocateBuf */)
	_ = isExisting
	if err == nil {
		// There's no point in having a chunk and not reading anything on to it.
		common.Assert(chunk.Len > 0, chunk.Len, chunk.Idx, chunkOffset, file.FileMetadata.Filename, isExisting)

		log.Debug("DistributedCache::getChunkForRead: Got chunk, file: %s, chunkIdx: %d, isExisting: %v, refcount: %d",
			file.FileMetadata.Filename, chunk.Idx, isExisting, chunk.RefCount.Load())
		return chunk, isExisting, err
	}

	common.Assert(chunk == nil)
	return nil, false, err
}

func (file *DcacheFile) getChunkForWrite(chunkIdx int64) (*StagedChunk, bool, error) {
	if common.IsDebugBuild() {
		file.chunkLock.RLock()
		numWriteChunks := len(file.StagedChunks)
		file.chunkLock.RUnlock()

		log.Debug("DistributedCache::getChunkForWrite: file: %s, chunkIdx: %d, current chunks: %d",
			file.FileMetadata.Filename, chunkIdx, numWriteChunks)
		common.Assert(chunkIdx >= 0)
	}

	chunk, isExisting, err := file.getChunk(chunkIdx, 0 /* chunkOffset */, 0, /* length */
		false /* noCache */, true /* allocateBuf */)

	//
	// For write chunks chunk.Len is the amount of valid data in the chunk. It starts at 0 and updated as user
	// data is copied to the chunk. We cannot assert chunk.Len==0 here as once file.getChunk() released the lock
	// another thread could have written to the chunk and updated chunk.Len.
	//
	if err == nil {
		// We always write full chunks except possibly the last chunk, but even that starts at offset 0.
		common.Assert(chunk.Offset == 0, chunk.Offset, chunk.Idx, file.FileMetadata.Filename, isExisting)

		log.Debug("DistributedCache::getChunkForWrite: Got chunk, file: %s, chunkIdx: %d, isExisting: %v, refcount: %d",
			file.FileMetadata.Filename, chunk.Idx, isExisting, chunk.RefCount.Load())
	}

	return chunk, isExisting, err
}

// Remove chunk from staged chunks.
func (file *DcacheFile) removeChunk(chunkIdx int64) bool {
	log.Debug("DistributedCache::removeChunk: removing staged chunk, file: %s, chunkIdx: %d",
		file.FileMetadata.Filename, chunkIdx)

	file.chunkLock.Lock()
	defer file.chunkLock.Unlock()

	chunk, ok := file.StagedChunks[chunkIdx]
	if !ok {
		log.Err("DistributedCache::removeChunk: chunk not found, file: %s, chunk idx: %d",
			file.FileMetadata.Filename, chunkIdx)
		//
		// One valid case where this can happen:
		// workerPool::writeChunk() calls removeChunk() after the chunk upload is done, but by that time
		// DcacheFile::ReleaseFile() released and removed this chunk from StagedChunks map. Since the chunk
		// is already removed from the map, we don't need to do anything here.
		//
		return false
	}

	//
	// The thread doing the remove must be still holding a refcount on the chunk.
	// In case of reads the extra refcount is also held on the chunk by the caller, so the refcount would
	// be at least 2, but we can only assert for >=1 here
	//
	common.Assert(chunk.RefCount.Load() >= 1, chunk.Idx, chunk.RefCount.Load())
	// Dirty chunks must be uploaded first.
	common.Assert(chunk.Dirty.Load() == false, chunk.Idx, file.FileMetadata.Filename)

	common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)
	chunk.SavedInMap.Store(false)

	//
	// For reads, this will just drop the map refcount, for writes this will also free the chunk memory.
	//
	file.releaseChunk(chunk)

	// Remove from the map, regardless of whether chunk memory is freed or not.
	delete(file.StagedChunks, chunkIdx)

	return true
}

// Remove all chunks from StagedChunks map.
// This drops the map refcount on each chunk and also calls chunk.releaseChunk() which will free the chunk memory
// if no other user is using it.
// DO NOT call it if there are dirty chunks, they must be uploaded before calling this.
func (file *DcacheFile) removeAllChunks(needLock bool) error {
	if needLock {
		file.chunkLock.Lock()
		defer file.chunkLock.Unlock()
	}

	// Avoid the log if there are no chunks.
	if len(file.StagedChunks) == 0 {
		return nil
	}

	log.Debug("DistributedCache[FM]::removeAllChunks: %s, removing %d chunks from StagedChunks map",
		file.FileMetadata.Filename, len(file.StagedChunks))

	for chunkIdx, chunk := range file.StagedChunks {
		_ = chunkIdx
		common.Assert(chunkIdx == chunk.Idx, chunkIdx, chunk.Idx, file.FileMetadata.Filename)

		log.Debug("DistributedCache[FM]::removeAllChunks: file: %s, chunkIdx: %d, chunk.Len: %d, chunk.Offset: %d",
			file.FileMetadata.Filename, chunk.Idx, chunk.Len, chunk.Offset)

		// Silently dropping dirty chunks will result in data loss, so refuse to do that.
		if chunk.Dirty.Load() {
			err := fmt.Errorf("Refusing to remove dirty  chunk, file: %s, chunkIdx: %d, chunk.Len: %d",
				file.FileMetadata.Filename, chunk.Idx, chunk.Len)
			log.Err("DistributedCache[FM]::removeAllChunks: %v", err)
			common.Assert(false, chunk.Idx, file.FileMetadata.Filename, chunk.Len)
			return err
		}

		//
		// The map refcount must be held, there could be other refcounts too if the chunk is being used.
		// We will remove the chunk from the map in any case, but if the chunk is being used, we won't
		// free it, it'll be freed when the last user releases it.
		//
		common.Assert(chunk.RefCount.Load() >= 1, chunk.Idx, chunk.RefCount.Load(), file.FileMetadata.Filename)
		common.Assert(chunk.SavedInMap.Load() == true, chunk.Idx, file.FileMetadata.Filename)

		chunk.SavedInMap.Store(false)

		// If there are no other users of the chunk, this will free the chunk memory.
		file.releaseChunk(chunk)

		// Remove from the map, regardless of whether chunk memory is freed or not.
		delete(file.StagedChunks, chunkIdx)
	}

	// Usually removeAllChunks() is called when we detect random read pattern, so reset the readahead state.
	file.lastReadaheadChunkIdx.Store(0)

	return nil
}

// Release buffer for the staged chunk.
func (file *DcacheFile) releaseChunk(chunk *StagedChunk) bool {
	log.Debug("DistributedCache::releaseChunk: file: %s, chunkIdx: %d, refcount: %d, external: %v",
		file.FileMetadata.Filename, chunk.Idx, chunk.RefCount.Load(), chunk.IsBufExternal)

	// Only a user holding a valid chunk refcount should release the chunk.
	common.Assert(chunk.RefCount.Load() > 0, chunk.Idx, chunk.RefCount.Load())

	if chunk.RefCount.Add(-1) != 0 {
		// Not the last user of the chunk, so cannot free it.
		return false
	}

	log.Debug("DistributedCache::releaseChunk: Freeing chunk, file: %s, chunkIdx: %d, refcount: %d, external: %v",
		file.FileMetadata.Filename, chunk.Idx, chunk.RefCount.Load(), chunk.IsBufExternal)

	//
	// We must not be freeing a chunk which is still in the StagedChunks map, as the map holds a refcount
	// of its own which is dropped only when the chunk is removed from the map.
	// Also dirty chunks must be first uploaded before freeing.
	//
	common.Assert(chunk.SavedInMap.Load() == false, chunk.Idx, file.FileMetadata.Filename)
	common.Assert(chunk.Dirty.Load() == false, chunk.Idx, file.FileMetadata.Filename)

	//
	// If buffer is allocated by NewStagedChunk(), free it to the pool, else it's an external buffer
	// returned by ReadMV(), just drop our reference and let GC free it.
	//
	if chunk.IsBufExternal {
		chunk.Buf = nil
	} else {
		dcache.PutBuffer(chunk.Buf)
	}

	// Let waiters know that a chunk is freed.
	file.freeChunks <- struct{}{}

	return true
}

// Read Chunk data from the file at 'offset' and 'length' bytes and returns the StagedChunk containing the
// requested data. If length is 0, the entire chunk containing file data at 'offset' is read. This is normally
// the case when reading sequentially, while for random reads we read only as much as the user asked for.
// If the chunk is already in file.StagedChunks, it is returned else a new chunk is created, and data is read
// from the file into that chunk.
//
// Sync true: Schedules and waits for the download to complete.
// Sync false: Schedules the read but doesn't wait for download to complete. This is the readahead case.

func (file *DcacheFile) readChunk(offset, length int64, sync bool) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
	chunkOffset := int64(0)
	if length != 0 {
		chunkOffset = getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
	}

	log.Debug("DistributedCache::readChunk: file: %s, offset: %d, chunkIdx: %d, chunkOffset: %d, length: %d, sync: %t",
		file.FileMetadata.Filename, offset, chunkIdx, chunkOffset, length, sync)

	//
	// If this chunk is already staged, return the staged chunk else create a new chunk, add to the staged
	// chunks list and return. For a new chunk, the chunk download is not yet scheduled.
	//
	chunk, isExisting, err := file.getChunkForRead(chunkIdx, chunkOffset, length)
	if err != nil {
		common.Assert(chunk == nil)
		return nil, err
	}

	//
	// For readahead chunks the caller doesn't wait for the download to complete, and doesn't drop the refcount.
	// So drop the refcount here. When reader thread reads from this readahead chunk, it'll get a brand new
	// refcount at that time.
	//
	if !sync {
		file.releaseChunk(chunk)
	}

	//
	// Only the caller that creates a new chunk schedules the download.
	//
	if !isExisting {
		scheduleDownload(chunk, file)
	}

	if sync {
		// Block here till the chunk download is done.
		err = <-chunk.Err

		if err != nil {
			log.Err("DistributedCache::readChunk: Failed, file: %s, chunkIdx: %d, chunkOffset: %d, length: %d, sync: %t",
				file.FileMetadata.Filename, chunkIdx, chunkOffset, length, sync)

			// Requeue the error for whoever reads this chunk next.
			chunk.Err <- err
		}
	}

	//
	// The only case where we fail with an error but still return a valid chunk is when chunk download
	// fails. Caller must release the chunk refcount.
	//
	return chunk, err
}

// Reads the chunk and also schedules downloads for the necessary readahead chunks.
// Returns the StagedChunk containing data at 'offset' in the file.
// Readahead is done only when reading the start of a chunk and when caller is sure of the sequential
// access pattern (unsure==false). If not sure, we store the chunk in cache but don't do readahead.
//
// Note: This reads the entire chunk into an allocated StagedChunk and adds it to the StagedChunks map.
//       This is suitable for sequential read patterns where readahead is beneficial and it's useful
//       to save the chunk in StagedChunks map for subsequent reads.
//       See readChunkNoReadAhead() for a version that does not do readahead and does not save the chunk
//       in StagedChunks map. That version is suitable for random read patterns.

func (file *DcacheFile) readChunkWithReadAhead(offset int64, unsure bool) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)

	log.Debug("DistributedCache::readChunkWithReadAhead: file: %s, offset: %d, chunkIdx: %d, unsure: %v",
		file.FileMetadata.Filename, offset, chunkIdx, unsure)

	//
	// Schedule downloads for the readahead chunks. The chunk at chunkIdx is to be read synchronously,
	// for the remaining we do async/readahead read.
	// We do it only when reading the start of a chunk.
	//
	if !unsure && isOffsetChunkStarting(offset, file.FileMetadata.FileLayout.ChunkSize) {
		//
		// Start readahead after the last chunk readahead by prev read calls.
		// How many more chunks can we readahead?
		// We are allowed to cache upto fileIOMgr.numReadAheadChunks chunks per file and file.StagedChunks
		// are already cached.
		//
		file.chunkLock.Lock()

		// Conservative estimate of chunks in use (already staged + readahead scheduled but not yet issued).
		chunksInUse := file.readaheadToBeIssued.Load() + int64(len(file.StagedChunks))
		// How many more chunks are we allowed to readahead?
		readAheadCount := max(int64(fileIOMgr.numReadAheadChunks-int(chunksInUse)), 0)

		// Start readahead after the last chunk readahead by prev read calls or after this chunk.
		readAheadStartChunkIdx := max(file.lastReadaheadChunkIdx.Load()+1, chunkIdx+1)
		readAheadEndChunkIdx := min(readAheadStartChunkIdx+readAheadCount,
			getChunkIdxFromFileOffset(file.FileMetadata.Size-1, file.FileMetadata.FileLayout.ChunkSize))
		common.Assert(readAheadEndChunkIdx >= chunkIdx, readAheadEndChunkIdx, chunkIdx)

		//
		// Update lastReadaheadChunkIdx inside the lock to avoid duplicate readahead by multiple threads.
		// The actual readahead is done outside the lock. In the unlikely event of readahead reads failing,
		// we won't reattempt readahead of those chunks, but that's ok.
		//
		if readAheadEndChunkIdx > readAheadStartChunkIdx {
			file.lastReadaheadChunkIdx.Store(readAheadEndChunkIdx - 1)
			file.readaheadToBeIssued.Add(readAheadEndChunkIdx - readAheadStartChunkIdx)
		}

		if common.IsDebugBuild() {
			log.Debug("DistributedCache::readChunkWithReadAhead: file: %s, Readahead %d chunks [%d, %d), %d in cache",
				file.FileMetadata.Filename, (readAheadEndChunkIdx - readAheadStartChunkIdx),
				readAheadStartChunkIdx, readAheadEndChunkIdx, len(file.StagedChunks))
		}

		file.chunkLock.Unlock()

		for i := readAheadStartChunkIdx; i < readAheadEndChunkIdx; i++ {
			_, err := file.readChunk(i*file.FileMetadata.FileLayout.ChunkSize, 0, false /* sync */)

			common.Assert(file.readaheadToBeIssued.Load() > 0,
				file.readaheadToBeIssued.Load(), file.FileMetadata.Filename)

			file.readaheadToBeIssued.Add(-1)
			if err != nil {
				// Don't fail the read because readahead failed.
				log.Warn("DistributedCache::readChunkWithReadAhead: file: %s, chunkIdx: %d, readahead failed: %v",
					file.FileMetadata.Filename, i, err)
			}
		}
	}

	// Now the actual chunk, unlike readahead chunks we wait for this one to download.
	chunk, err := file.readChunk(offset, 0 /* length */, true /* sync */)
	return chunk, err
}

// Reads 'length' bytes from file at 'offset' and returns the StagedChunk containing the requested data.
//
// Note: This has following differences from readChunkWithReadAhead():
//       1. It does not necessarily read the entire chunk, only [offset, offset+length) bytes are read.
//       2. It does not save the chunk in StagedChunks map, so it is not available for subsequent reads.
//       3. It does not do readahead of subsequent chunks.
//
// This is suitable for random read patterns where readahead is not beneficial and we don't want to save
// the chunk in StagedChunks map for subsequent reads.

func (file *DcacheFile) readChunkNoReadAhead(offset, length int64) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
	_ = chunkIdx
	// Offset within that chunk.
	chunkOffset := getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
	_ = chunkOffset

	log.Debug("DistributedCache::readChunkNoReadAhead: file: %s, offset: %d, length: %d, chunkIdx: %d, chunkOffset: %d",
		file.FileMetadata.Filename, offset, length, chunkIdx, chunkOffset)

	// length must not be 0 to signify random read.
	common.Assert(length > 0 && (chunkOffset+length) <= file.FileMetadata.FileLayout.ChunkSize,
		length, chunkOffset, file.FileMetadata.FileLayout.ChunkSize)

	// Read the chunk, wait for it to download.
	chunk, err := file.readChunk(offset, length, true /* sync */)
	return chunk, err
}

// Create/return the chunk that is ready to be written.
func (file *DcacheFile) CreateOrGetStagedChunk(offset int64) (*StagedChunk, error) {
	// Given the file layout, get the index of chunk that contains data at 'offset'.
	chunkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)

	log.Debug("DistributedCache::CreateOrGetStagedChunk: file: %s, offset: %d, chunkIdx: %d",
		file.FileMetadata.Filename, offset, chunkIdx)

	chunk, isExisting, err := file.getChunkForWrite(chunkIdx)
	if err != nil {
		return chunk, err
	}

	// Currently we don't use the upload ref, so drop it here.
	if !isExisting {
		file.releaseChunk(chunk)
	}

	common.Assert(chunk.Idx == chunkIdx, chunk.Idx, chunkIdx, offset, file.FileMetadata.Filename)
	// We never write partial chunks, except possibly the last chunk in the file, but even those start at offset 0.
	common.Assert(chunk.Offset == 0, chunk.Offset, chunk.Idx, file.FileMetadata.Filename)

	return chunk, nil
}

func scheduleDownload(chunk *StagedChunk, file *DcacheFile) bool {
	// chunk.Len is the amount of bytes to download, cannot be 0.
	common.Assert(chunk.Len > 0)
	// Must have a valid download refcount.
	common.Assert(chunk.RefCount.Load() >= 1, chunk.Idx, chunk.RefCount.Load())
	// Only the original thread that allocated the chunk starts the download.
	common.Assert(!chunk.XferScheduled.Load(), chunk.Idx, chunk.Len, file.FileMetadata.Filename)
	// Cannot be overwriting a dirty staged chunk.
	common.Assert(!chunk.Dirty.Load())
	// Cannot be reading an already up-to-date chunk.
	common.Assert(!chunk.UpToDate.Load())

	chunk.XferScheduled.Store(true)

	log.Debug("DistributedCache::scheduleDownload: file: %s, chunkIdx: %d, chunk.Len: %d, chunk.Offset: %d, refcount: %d",
		file.FileMetadata.Filename, chunk.Idx, chunk.Len, chunk.Offset, chunk.RefCount.Load())

	fileIOMgr.wp.queueWork(file, chunk, true /* get_chunk */)
	return true
}

func scheduleUpload(chunk *StagedChunk, file *DcacheFile) bool {
	// chunk.Len is the amount of bytes to upload, cannot be 0.
	common.Assert(chunk.Len > 0)
	// We never upload partial chunks, except possibly the last chunk in the file, but even those start at offset 0.
	common.Assert(chunk.Offset == 0)
	// Must have a valid refcount.
	common.Assert(chunk.RefCount.Load() >= 1, chunk.Idx, chunk.RefCount.Load())

	if !chunk.XferScheduled.Swap(true) {
		log.Debug("DistributedCache::scheduleUpload: file: %s, chunkIdx: %d, chunk.Len: %d, chunk.Offset: %d, refcount: %d",
			file.FileMetadata.Filename, chunk.Idx, chunk.Len, chunk.Offset, chunk.RefCount.Load())

		// Only dirty staged chunk should be written to dcache.
		common.Assert(chunk.Dirty.Load())
		// Up-to-date chunk should not be written.
		common.Assert(!chunk.UpToDate.Load())

		fileIOMgr.wp.queueWork(file, chunk, false /* get_chunk */)
		return true
	}

	return false
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
