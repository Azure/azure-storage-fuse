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
	"sync"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
)

type fileIOManager struct {
	numReadAheadChunks int // Number of chunks to readahead from the current chunk.
	numStagingChunks   int // Max Number of chunks per file that can be staging area at any time.
	wp                 *workerPool
	bp                 *bufferPool
}

var fileIOMgr fileIOManager

func NewFileIOManager(workers int, numReadAheadChunks int, numStagingChunks int, bufSize int, maxBuffers int) {
	fileIOMgr = fileIOManager{
		numReadAheadChunks: numReadAheadChunks,
		numStagingChunks:   numStagingChunks,
	}
	fileIOMgr.wp = NewWorkerPool(workers)
	fileIOMgr.bp = NewBufferPool(bufSize, maxBuffers)
}

func EndFileIOManager() {
	fileIOMgr.wp.destroyWorkerPool()
}

type DcacheFile struct {
	FileMetadata    *dcache.FileMetadata
	lastWriteOffset int64 // Every new offset that come to write should be greater than lastWriteOffset
	// Offset should be monotonically increasing while writing to the file.
	StagedChunks sync.Map // Chunk Idx -> *chunk
	// The above chunks are the outstanding chunks for the file.
	// Those chunks can be readahead chunks for read /
	// current staging chunks for write
	// Todo: Chunks should be tracked globally rather than per file.
}

// Reads the file from the corresponding chunk(s) to the buf.
func (file *DcacheFile) ReadFile(offset int64, buf []byte) (bytesRead int, err error) {
	log.Debug("DistributedCache::ReadFile : offset : %d, bufSize : %d, file : %s", offset, len(buf), file.FileMetadata.Filename)
	if offset >= file.FileMetadata.Size {
		return 0, io.EOF
	}
	// endOffset is 1 + offset of the last byte to be read.
	endOffset := min(offset+int64(len(buf)), file.FileMetadata.Size)
	bufOffset := 0
	for offset < endOffset {
		//  Currently calling direct readaahead for the chunk assuming sequential workflow
		// todo : Add Sequential pattern detection.
		// todo : Support Partial read of chunk, useful for random read scenarios.
		chunk, err := file.readChunkWithReadAhead(offset)
		if err != nil {
			return 0, err
		}
		chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)
		copied := copy(buf[bufOffset:], chunk.Buf[chunkOffset:chunk.Len])
		offset += int64(copied)
		bufOffset += copied
	}
	common.Assert(offset <= file.FileMetadata.Size, "Read beyond the file size")
	return bufOffset, nil
}

// Writes the file to the corresponding chunk(s) from the buf.
func (file *DcacheFile) WriteFile(offset int64, buf []byte) error {
	log.Debug("DistributedCache[FM]::WriteFile : offset : %d, bufSize : %d, file : %s", offset, len(buf), file.FileMetadata.Filename)

	if offset < file.lastWriteOffset {
		log.Err("DistributedCache[FM]::WriteFile: File handle is not writing in sequential manner, current Offset : %d, Prev Offset %d", offset, file.lastWriteOffset)
		return syscall.ENOTSUP
	}

	// endOffset is 1 + offset of the last byte to be write.
	endOffset := (offset + int64(len(buf)))
	bufOffset := 0
	for offset < endOffset {
		chunk, err := file.CreateOrGetStagedChunk(offset)
		if err != nil {
			errMsg := fmt.Sprintf("DistributedCache[FM]::WriteFile: Failed to get the chunk for write, err : %s chnk idx : %d, file : %s",
				err.Error(), getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout), file.FileMetadata.Filename)
			common.Assert(false, errMsg)
			return err
		}
		chunkOffset := getChunkOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout)

		// Todo: Support Bigger Holes(hole size > chunkSize) inside the file if offset jumps suddenly to a higher offset.
		// Sanity check for resetting the garbage data of the chunk buf.
		if chunkOffset > chunk.Len {
			resetBytes := copy(chunk.Buf[chunk.Len:chunkOffset], make([]byte, chunkOffset-chunk.Len))
			chunk.Len += int64(resetBytes)
		}

		copied := copy(chunk.Buf[chunkOffset:], buf[bufOffset:])
		offset += int64(copied)
		bufOffset += copied
		chunk.Len += int64(copied)

		common.Assert(chunk.Len == getChunkOffsetFromFileOffset(offset-1, &file.FileMetadata.FileLayout)+1,
			fmt.Sprintf("Actual Chunk Len : %d is modified incorrectly, Expected chunkLen : %d",
				chunk.Len, getChunkOffsetFromFileOffset(offset-1, &file.FileMetadata.FileLayout)+1))

		// Schedule the upload when staged chunk is fully written
		if chunk.Len == int64(len(chunk.Buf)) {
			// todo: This not always true. if some writes to this chunk were skipped then there should be a
			// way to stage this block.
			scheduleUpload(chunk, file)
		}
	}
	common.Assert(offset == endOffset, fmt.Sprintf("Write is not successful, expected Endoffset : %d, actual EndOffset : %d", endOffset, offset))
	common.Assert(bufOffset == len(buf), fmt.Sprintf("Amount of Bytes copied : %d is not equal to buf len %d", bufOffset, len(buf)))
	file.lastWriteOffset = offset
	return nil
}

// Sync Buffers for the file with dcache/azure
// This call can come when user application calls fsync()/close()
func (file *DcacheFile) SyncFile() error {
	log.Debug("DistributedCache[FM]::SyncFile : Sync File for %s", file.FileMetadata.Filename)
	var err error
	file.StagedChunks.Range(func(chunkIdx any, Ichunk any) bool {
		chunk := Ichunk.(*StagedChunk)
		// todo: parallelize the uploads for the chunks
		log.Debug("DistributedCache[FM]::SyncFile : chunkIdx : %d, chunkLen : %d, file : %s",
			chunk.Idx, chunk.Len, file.FileMetadata.Filename)
		scheduleUpload(chunk, file)
		err = <-chunk.Err
		if err != nil {
			return false
		}
		return true
	})
	common.Assert(err == nil, fmt.Sprintf("Filemanager::SyncFile failed, file: %s, err: %v",
		file.FileMetadata.Filename, err))
	return err
}

// Close and Finalize the file. writes are failed after this operation
func (file *DcacheFile) CloseFile() error {
	log.Debug("DistributedCache[FM]::CloseFile : Close File for %s", file.FileMetadata.Filename)
	// We stage application writes into StagedChunk and upload only when we have a full chunk.
	// In case of last chunk being partial, we need to upload it now.
	err := file.SyncFile()
	common.Assert(err == nil, fmt.Sprintf("Filemanager::CloseFile failed, file: %s, err: %v ",
		file.FileMetadata.Filename, err))
	if err == nil {
		err := file.finalizeFile()
		common.Assert(err == nil, fmt.Sprintf("Filemanager::CloseFile failed, file: %s, err: %v",
			file.FileMetadata.Filename, err))
		if err != nil {
			log.Err("DistributedCache[FM]::Close : finalize file failed with err: %s, file: %s", err.Error(), file.FileMetadata.Filename)
		}
	}
	return err
}

// Release all allocated buffers for the file
func (file *DcacheFile) ReleaseFile() error {
	log.Debug("DistributedCache[FM]::ReleaseFile :Releasing buffers for File for %s", file.FileMetadata.Filename)
	file.StagedChunks.Range(func(chunkIdx any, Ichunk any) bool {
		chunk := Ichunk.(*StagedChunk)
		// todo: assert for each chunk that err is closed. currently not doing it as readahead chunks
		// error channel might be opened.
		file.releaseChunk(chunk)
		return true
	})
	return nil
}

// This method is called when all the File IO operations are successful
// and user wants to sync the file
func (file *DcacheFile) finalizeFile() error {
	common.Assert(file.FileMetadata.State == dcache.Writing)
	file.FileMetadata.State = dcache.Ready
	file.FileMetadata.Size = file.lastWriteOffset
	common.Assert(file.FileMetadata.Size != 0)
	fileMetadataBytes, err := json.Marshal(file.FileMetadata)
	if err != nil {
		log.Err("DistributedCache[FM]::finalizeFile : FileMetadata marshalling fail, file: %s, %+v",
			file.FileMetadata.Filename, file.FileMetadata)
		return err
	}
	err = mm.CreateFileFinalize(file.FileMetadata.Filename, fileMetadataBytes, file.FileMetadata.Size)
	if err != nil {
		log.Err("DistributedCache[FM]::finalizeFile : File Finalize failed for file: %s, %+v with err: %s",
			file.FileMetadata.Filename, file.FileMetadata, err.Error())
		return err
	}
	log.Debug("DistributedCache[FM]::finalizeFile : Final metadata for file %s, : %+v",
		file.FileMetadata.Filename, file.FileMetadata)
	return nil
}

// Get's the existing chunk from the chunks
// or Create a new one and add it to the the chunks
func (file *DcacheFile) getChunk(chunkIdx int64) (*StagedChunk, bool, error) {
	log.Debug("DistributedCache::getChunk : getChunk for chunkIdx: %d, file: %s", chunkIdx, file.FileMetadata.Filename)
	if chunkIdx < 0 {
		return nil, false, errors.New("ChunkIdx is less than 0")
	}
	if chunk, ok := file.StagedChunks.Load(chunkIdx); ok {
		return chunk.(*StagedChunk), true, nil
	}
	chunk, err := NewStagedChunk(chunkIdx, file)
	if err != nil {
		return nil, false, err
	}
	file.StagedChunks.Store(chunkIdx, chunk)
	return chunk, false, nil
}

func (file *DcacheFile) getChunkForRead(chunkIdx int64) (*StagedChunk, error) {
	log.Debug("DistributedCache::getChunkForRead : getChunk for Read chunkIdx: %d, file: %s", chunkIdx, file.FileMetadata.Filename)
	chunk, loaded, err := file.getChunk(chunkIdx)
	if err == nil && !loaded {
		close(chunk.ScheduleUpload)
		chunk.Len = getChunkSize(chunkIdx*file.FileMetadata.FileLayout.ChunkSize, file)
	}
	return chunk, err
}

func (file *DcacheFile) getChunkForWrite(chunkIdx int64) (*StagedChunk, error) {
	log.Debug("DistributedCache::getChunkForWrite : getChunk for Write chunkIdx: %d, file: %s", chunkIdx, file.FileMetadata.Filename)
	chunk, loaded, err := file.getChunk(chunkIdx)
	if err == nil && !loaded {
		close(chunk.ScheduleDownload)
	}
	return chunk, err
}

// load chunk from the file chunks
func (file *DcacheFile) loadChunk(chunkIdx int64) (*StagedChunk, error) {
	if chunkIdx < 0 {
		return nil, errors.New("ChunkIdx is less than 0")
	}
	if chunk, ok := file.StagedChunks.Load(chunkIdx); ok {
		return chunk.(*StagedChunk), nil
	}
	return nil, errors.New("Chunk is not found inside the file chunks")
}

// remove chunk from the file chunks
func (file *DcacheFile) removeChunk(chunkIdx int64) {
	log.Debug("DisttributedCache::removeChunk : removing chunk from the chunks, chunk idx: %d, file: %s",
		chunkIdx, file.FileMetadata.Filename)
	Ichunk, loaded := file.StagedChunks.LoadAndDelete(chunkIdx)
	if loaded {
		chunk := Ichunk.(*StagedChunk)
		fileIOMgr.bp.putBuffer(chunk.Buf)
	}
}

// release the buffer for chunk
func (file *DcacheFile) releaseChunk(chunk *StagedChunk) {
	log.Debug("DisttributedCache::releaseChunk : releasing buffer for chunk, chunk idx: %d, file: %s",
		chunk.Idx, file.FileMetadata.Filename)
	fileIOMgr.bp.putBuffer(chunk.Buf)
}

// Read Chunk data from the file.
// Sync true : Schedules and waits for the download to complete.
// Sync false: Schedules the read
func (file *DcacheFile) readChunk(offset int64, sync bool) (*StagedChunk, error) {
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::readChunk : sync: %t, chunkIdx : %d, file : %s", sync, chunkIdx, file.FileMetadata.Filename)
	chunk, err := file.getChunkForRead(chunkIdx)
	if err != nil {
		return chunk, err
	}

	scheduleDownload(chunk, file)

	if sync {
		err = <-chunk.Err
		if err != nil {
			chunk.Err <- err
		}
		// Release the previous chunks if any.
		if isOffsetChunkStarting(offset, &file.FileMetadata.FileLayout) {
			// Clean the previous chunk
			file.removeChunk(chunkIdx - 1)
		}
	}
	return chunk, err
}

// Reads the chunk and also schedules the downloads for the readahead chunks
func (file *DcacheFile) readChunkWithReadAhead(offset int64) (*StagedChunk, error) {
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::readAheadChunk : offset : %d, chunkIdx : %d, file : %s", offset, chunkIdx, file.FileMetadata.Filename)

	readAheadEndChunkIdx := min(chunkIdx+int64(fileIOMgr.numReadAheadChunks),
		getChunkIdxFromFileOffset(file.FileMetadata.Size-1, &file.FileMetadata.FileLayout))
	// Schedule downloads for the readahead chunks
	if isOffsetChunkStarting(offset, &file.FileMetadata.FileLayout) {
		for i := chunkIdx + 1; i <= readAheadEndChunkIdx; i++ {
			_, err := file.readChunk(i*file.FileMetadata.FileLayout.ChunkSize, false)
			if err != nil {
				return nil, err
			}
		}
	}
	// Download the actual chunk
	chunk, err := file.readChunk(offset, true)
	return chunk, err
}

// Creates/return the chunk that is ready to be written.
// Also responsible for releasing the chunks, if the chunks are greater than staging area chunks.
func (file *DcacheFile) CreateOrGetStagedChunk(offset int64) (*StagedChunk, error) {
	chunkIdx := getChunkIdxFromFileOffset(offset, &file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::CreateOrGetStagedChunk : chunkIdx : %d, file : %s", chunkIdx, file.FileMetadata.Filename)
	//	fmt.Printf("DistributedCache::CreateOrGetStagedChunk : chunkIdx : %d, file : %s\n", chunkIdx, file.FileMetadata.Filename)
	chunk, err := file.getChunkForWrite(chunkIdx)
	if err != nil {
		return chunk, err
	}

	// release the chunks that are out of staging area, by waiting for their uploads to complete.
	// only done at chunk boundaris to decrease the overhead
	if isOffsetChunkStarting(offset, &file.FileMetadata.FileLayout) {
		// Release the buffer if it falls out of staging area.
		releaseChunkIdx := chunkIdx - int64(fileIOMgr.numStagingChunks)
		releaseChunk, err := file.loadChunk(releaseChunkIdx)
		if err == nil {
			err := <-releaseChunk.Err
			if err != nil {
				// If there is an error while uploading the block.
				// As we are using writeback policy to upload the data.
				// Better to fail early.
				releaseChunk.Err <- err
				return nil, errors.New("DistributedCache::WriteChunk: failed to upload the previous chunk")
			}
			file.removeChunk(releaseChunkIdx)
		}
	}
	return chunk, nil
}

func scheduleDownload(chunk *StagedChunk, file *DcacheFile) {
	select {
	case <-chunk.ScheduleDownload:
	default:
		close(chunk.ScheduleDownload)
		fileIOMgr.wp.queueWork(file, chunk, true)
	}
}

func scheduleUpload(chunk *StagedChunk, file *DcacheFile) {
	select {
	case <-chunk.ScheduleUpload:
	default:
		close(chunk.ScheduleUpload)
		fileIOMgr.wp.queueWork(file, chunk, false)
	}
}
