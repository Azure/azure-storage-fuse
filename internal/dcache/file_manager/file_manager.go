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
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager/models"
)

const (
	cacheAccessAzure uint16 = iota
	cacheAccessDcache
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
	wp := NewWorkerPool(workers)
	wp.startWorkerPool()
	bp := NewBufferPool(bufSize, maxBuffers)
	fileIOMgr.wp = wp
	fileIOMgr.bp = bp
}

func EndFileIOManager() {
	fileIOMgr.wp.endWorkerPool()
}

type File struct {
	FileMetadata    *models.FileMetadata
	Access          common.BitMap16 // Represents the cache access type like azure, dcache, both.
	lastWriteOffset int64           // Every new offset that come to write should be greater than lastWriteOffset
	// Offset should be monotonically increasing while writing to the file.
	Chunks sync.Map // Chunk Idx -> *chunk
	// The above chunks are the outstanding chunks for the file.
	// Those chunks can be readahead chunks for read /
	// current staging chunks for write
	// Todo: Chunks should be tracked globally rather than per file.
}

// Reads the file from the corresponding chunk(s) to the buf.
func (file *File) ReadFile(offset int64, buf []byte) (bytesRead int, err error) {
	log.Debug("DistributedCache::ReadFile : offset : %d, bufSize : %d, file : %s", offset, len(buf), file.FileMetadata.Filename)
	if offset >= file.FileMetadata.Size {
		return 0, io.EOF
	}
	endOffset := min(offset+int64(len(buf)), file.FileMetadata.Size)
	bufOffset := 0
	for offset < endOffset {
		//  Currently calling direct readaahead for the chunk assuming sequential workflow
		chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
		// todo : Detection of the sequential nature of the handle can be found from storing the offset
		// in the handle.
		// todo : Support Partial read of chunk, useful for random read scenarios.
		chnk, err := file.readChunkWithReadAhead(chnkIdx)
		if err != nil {
			return 0, nil
		}
		chnkOffset := getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout)
		chnkSize := getChunkSize(offset, file)
		copied := copy(buf[bufOffset:], chnk.Buf[chnkOffset:chnkSize])
		offset += int64(copied)
		bufOffset += copied
	}
	common.Assert(offset <= file.FileMetadata.Size, "Read beyond the file size")
	return bufOffset, nil
}

// Writes the file to the corresponing chunk(s) from the buf.
func (file *File) WriteFile(offset int64, buf []byte) error {
	log.Debug("DistributedCache::WriteFile : offset : %d, bufSize : %d, file : %s", offset, len(buf), file.FileMetadata.Filename)
	if offset < file.lastWriteOffset {
		log.Info("DistributedCache[FM]::WriteFile: File handle is not writing in sequential manner, current Offset : %d, Prev Offset %d", offset, file.lastWriteOffset)
		return syscall.ENOTSUP
	}
	endOffset := (offset + int64(len(buf)))
	bufOffset := 0
	for offset < endOffset {
		chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
		chnk, err := file.writeChunk(chnkIdx)
		if err != nil {
			return err
		}
		chnkOffset := getChunkOffsetFromFileOffset(offset, file.FileMetadata.FileLayout)
		copied := copy(chnk.Buf[chnkOffset:], buf[bufOffset:])
		offset += int64(copied)
		bufOffset += copied
	}
	common.Assert(offset == endOffset, fmt.Sprintf("Write is not successful, expected Endoffset : %d, actual EndOffset : %d", endOffset, offset))
	common.Assert(bufOffset == len(buf), fmt.Sprintf("Amount of Bytes copied : %d is not equal to buf len %d", bufOffset, len(buf)))
	file.lastWriteOffset = offset
	return nil
}

// Sync Buffers for the file with dcache/azure
// This call can come when user application calls fsync()/close()
func (file *File) SyncFile() error {
	log.Debug("DistributedCache[FM]::SyncFile : Sync File for %s", file.FileMetadata.Filename)
	var err error
	file.Chunks.Range(func(chnkIdx any, Ichnk any) bool {
		chnk := Ichnk.(*models.Chunk)
		scheduleUpload(chnk, file)
		err = <-chnk.Err
		if err != nil {
			return false
		}
		return true
	})
	if file.CanAccessAzure() {
		// todo: if handle came for writing, do putblocklist to commit data.
	}
	return err
}

// Release all allocated buffers for the file
func (file *File) ReleaseFile() error {
	log.Debug("DistributedCache[FM]::ReleaseFile :Releasing buffers for File for %s", file.FileMetadata.Filename)
	file.Chunks.Range(func(chnkIdx any, Ichnk any) bool {
		chnk := Ichnk.(*models.Chunk)
		file.releaseChunk(chnk)
		return true
	})
	return nil
}

func (file *File) SetAzureAccessFlag() {
	file.Access.Set(cacheAccessAzure)
}

func (file *File) SetDcacheAccessFlag() {
	file.Access.Set(cacheAccessDcache)
}

func (file *File) SetDefaultCache() {
	file.Access.Set(cacheAccessAzure)
	file.Access.Set(cacheAccessDcache)
}

func (file *File) CanAccessDcache() bool {
	return file.Access.IsSet(cacheAccessDcache)
}

func (file *File) CanAccessAzure() bool {
	return file.Access.IsSet(cacheAccessAzure)
}

// Get's the existing chunk from the chunks
// or Create a new one and add it to the the chunks
func (file *File) getChunk(chnkIdx int64) (*models.Chunk, bool, error) {
	log.Debug("DistributedCache::getChunk : getChunk for chnkIdx : %d, file : %s", chnkIdx, file.FileMetadata.Filename)
	if chnkIdx < 0 {
		return nil, false, errors.New("ChnkIdx is less than 0")
	}
	if chnk, ok := file.Chunks.Load(chnkIdx); ok {
		return chnk.(*models.Chunk), true, nil
	}
	chnk, err := NewChunk(chnkIdx, file)
	if err != nil {
		return nil, false, err
	}
	file.Chunks.Store(chnkIdx, chnk)
	return chnk, false, nil
}

func (file *File) getChunkForRead(chnkIdx int64) (*models.Chunk, error) {
	log.Debug("DistributedCache::getChunkForRead : getChunk for Read chnkIdx : %d, file : %s", chnkIdx, file.FileMetadata.Filename)
	chnk, loaded, err := file.getChunk(chnkIdx)
	if err == nil && !loaded {
		close(chnk.ScheduleUpload)
	}
	return chnk, err
}

func (file *File) getChunkForWrite(chnkIdx int64) (*models.Chunk, error) {
	log.Debug("DistributedCache::getChunkForWrite : getChunk for Write chnkIdx : %d, file : %s", chnkIdx, file.FileMetadata.Filename)
	chnk, loaded, err := file.getChunk(chnkIdx)
	if err == nil && !loaded {
		close(chnk.ScheduleDownload)
	}
	return chnk, err
}

// load chunk from the file chunks
func (file *File) loadChunk(chnkIdx int64) (*models.Chunk, error) {
	if chnkIdx < 0 {
		return nil, errors.New("ChnkIdx is less than 0")
	}
	if chnk, ok := file.Chunks.Load(chnkIdx); ok {
		return chnk.(*models.Chunk), nil
	}
	return nil, errors.New("Chunk is not found inside the file chunks")
}

// remove chunk from the file chunks
func (file *File) removeChunk(chunkIdx int64) {
	log.Debug("DisttributedCache::removeChunk : removing chunk from the chunks, chunk idx : %d, file : %s",
		chunkIdx, file.FileMetadata.Filename)
	Ichnk, loaded := file.Chunks.LoadAndDelete(chunkIdx)
	if loaded {
		chnk := Ichnk.(*models.Chunk)
		fileIOMgr.bp.putBuffer(chnk.Buf)
	}
}

// release the buffer for chunk
func (file *File) releaseChunk(chnk *models.Chunk) {
	log.Debug("DisttributedCache::releaseChunk : releasing buffer for chunk, chunk idx : %d, file : %s",
		chnk.Idx, file.FileMetadata.Filename)
	fileIOMgr.bp.putBuffer(chnk.Buf)
}

// Read Chunk data from the file.
// Sync true : Schedules and waits for the download to complete.
// Sync false: Schedules the read
func (file *File) readChunk(offset int64, sync bool) (*models.Chunk, error) {
	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::readChunk : sync: %t, chnkIdx : %d, file : %s", sync, chnkIdx, file.FileMetadata.Filename)
	chnk, err := file.getChunkForRead(chnkIdx)
	if err != nil {
		return chnk, err
	}

	scheduleDownload(chnk, file)

	if sync {
		err = <-chnk.Err
		if err != nil {
			chnk.Err <- err
		}
		// Do the cleanUp if the offset is starting offset of a chunk.
		// Do the cleanup only at chunk starting.
		if isOffsetChunkStarting(offset, file.FileMetadata.FileLayout) {
			// Clean the previous chunk
			file.removeChunk(chnkIdx - 1)
		}
	}
	return chnk, err
}

// Reads the chunk and also schedules the downloads for the readahead chunks
func (file *File) readChunkWithReadAhead(offset int64) (*models.Chunk, error) {
	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::readAheadChunk : offset : %d, chnkIdx : %d, file : %s", offset, chnkIdx, file.FileMetadata.Filename)

	readAheadEndChnkIdx := min(chnkIdx+int64(fileIOMgr.numReadAheadChunks),
		getChunkIdxFromFileOffset(file.FileMetadata.Size-1, file.FileMetadata.FileLayout))
	// Schedule downloads for the readahead chunks
	if isOffsetChunkStarting(offset, file.FileMetadata.FileLayout) {
		for i := chnkIdx + 1; i <= readAheadEndChnkIdx; i++ {
			_, err := file.readChunk(i*file.FileMetadata.FileLayout.ChunkSize, false)
			if err != nil {
				return nil, err
			}
		}
	}
	// Download the actual chunk
	chnk, err := file.readChunk(offset, true)
	return chnk, err
}

// Todo: Support Holes inside the file if offset jumps suddenly to a higher offset.
func (file *File) writeChunk(offset int64) (*models.Chunk, error) {
	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	log.Debug("DistributedCache::writeChunk : chnkIdx : %d, file : %s", chnkIdx, file.FileMetadata.Filename)
	chnk, err := file.getChunkForWrite(chnkIdx)
	if err != nil {
		return chnk, err
	}

	// Do the Uploads and release only at chunk boundaris to decrease the overhead
	if isOffsetChunkStarting(offset, file.FileMetadata.FileLayout) {
		// Schedule Upload for the previous chunk
		prevChnk, err := file.loadChunk(chnkIdx - 1)
		if err == nil {
			scheduleUpload(prevChnk, file)
		}

		// Release the buffer if it falls out of staging area.
		releaseChnk, err := file.loadChunk(chnkIdx - int64(fileIOMgr.numStagingChunks))
		if err == nil {
			err := <-releaseChnk.Err
			if err != nil {
				// If there is an error while uploading the block.
				// As we are using writeback policy to upload the data.
				// Better to fail early.
				releaseChnk.Err <- err
				return nil, errors.New("DistributedCache::WriteChunk: failed to upload the previous chunk")
			}
			file.removeChunk(chnkIdx - 3)
		}
	}
	return chnk, nil
}

func scheduleDownload(chnk *models.Chunk, file *File) {
	select {
	case <-chnk.ScheduleDownload:
	default:
		close(chnk.ScheduleDownload)
		t := &task{
			file:       file,
			chunk:      chnk,
			fileLayout: *file.FileMetadata.FileLayout,
			get_chunk:  true,
		}
		fileIOMgr.wp.tasks <- t
	}
}

func scheduleUpload(chnk *models.Chunk, file *File) {
	select {
	case <-chnk.ScheduleUpload:
	default:
		close(chnk.ScheduleUpload)
		t := &task{
			file:       file,
			chunk:      chnk,
			fileLayout: *file.FileMetadata.FileLayout,
			get_chunk:  false,
		}
		fileIOMgr.wp.tasks <- t
	}
}
