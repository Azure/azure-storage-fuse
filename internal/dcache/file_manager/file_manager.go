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
	"io"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager/models"
)

func NewFile(fileName string) *File {
	fileMetadata := &models.FileMetadata{
		Filename: fileName,
	}
	// todo : assign uuid for fileID
	// todo : get chunkSize and Stripe Size from the config and assign.
	// todo : Choose appropriate MV's from the online mv's returned by the clustermap
	fileMetadata.FileLayout = &models.FileLayout{
		ChunkSize:  4 * 1024 * 1024,
		StripeSize: 16 * 1024 * 1024,
		MVList:     []string{"mv0", "mv1", "mv2"},
	}
	return &File{
		FileMetadata: fileMetadata,
	}
}

type File struct {
	FileMetadata *models.FileMetadata
	Chunks       sync.Map // Chunk Idx -> *chunk
}

// Get's the existing chunk from the chunks
// or Create a new one and add it to the the chunks
func (f *File) getChunk(chnkIdx int64) (*models.Chunk, error) {
	if chnkIdx < 0 {
		return nil, errors.New("ChnkIdx is less than 0")
	}
	if chnk, ok := f.Chunks.Load(chnkIdx); ok {
		return chnk.(*models.Chunk), nil
	}
	chnk, err := NewChunk(chnkIdx)
	if err != nil {
		return chnk, err
	}
	f.Chunks.Store(chnkIdx, chnk)
	return chnk, nil
}

// load chunk from the file chunks
func (f *File) loadChunk(chnkIdx int64) (*models.Chunk, error) {
	if chnkIdx < 0 {
		return nil, errors.New("ChnkIdx is less than 0")
	}
	if chnk, ok := f.Chunks.Load(chnkIdx); ok {
		return chnk.(*models.Chunk), nil
	}
	return nil, errors.New("Chunk is not found inside the file chunks")
}

func (f *File) removeChunk(chunkIdx int64) {
	Ichnk, loaded := f.Chunks.LoadAndDelete(chunkIdx)
	if loaded {
		chnk := Ichnk.(*models.Chunk)
		fileIOMgr.bp.putBuffer(chnk.Buf)
	}
}

func NewChunk(idx int64) (*models.Chunk, error) {
	buf, err := fileIOMgr.bp.getBuffer()
	if err != nil {
		return nil, err
	}
	return &models.Chunk{
		Idx:              idx,
		Buf:              buf,
		Err:              make(chan error),
		ScheduleDownload: make(chan struct{}),
		ScheduleUpload:   make(chan struct{}),
	}, nil
}

type fileIOManager struct {
	numReadAheadChunks int
	wp                 *workerPool
	bp                 *bufferPool
}

var fileIOMgr fileIOManager

func NewFileIOManager(workers int, numReadAheadChunks int, bufSize int, maxBuffers int) {
	fileIOMgr = fileIOManager{
		numReadAheadChunks: numReadAheadChunks,
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

func ReadChunkSync(offset int64, file *File) (*models.Chunk, error) {
	if offset >= file.FileMetadata.Size {
		return nil, io.EOF
	}
	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	chnk, err := file.getChunk(chnkIdx)
	if err != nil {
		return chnk, err
	}

	scheduleDownload(chnk, file)
	err = <-chnk.Err
	if err != nil {
		chnk.Err <- err
	}
	return chnk, err
}

func ReadChunkAsync(offset int64, file *File) (*models.Chunk, error) {
	if offset >= file.FileMetadata.Size {
		return nil, io.EOF
	}

	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	lastChnkIdx := min(chnkIdx+int64(fileIOMgr.numReadAheadChunks),
		getChunkIdxFromFileOffset(file.FileMetadata.Size-1, file.FileMetadata.FileLayout))
	for i := chnkIdx; i <= lastChnkIdx; i++ {
		chnk, err := file.getChunk(chnkIdx)
		if err != nil {
			return chnk, err
		}
		scheduleDownload(chnk, file)
	}
	curChnk, err := file.getChunk(chnkIdx)
	if err != nil {
		return nil, err
	}
	err = <-curChnk.Err
	if err != nil {
		curChnk.Err <- err
	}
	file.removeChunk(chnkIdx - 1)
	return curChnk, err
}

// Note : only support sequential writes.
// Todo: Support Holes inside the file if offset jumps.
// Todo: Support Read-modify-write scenario
func WriteChunk(offset int64, buf []byte, file *File) (*models.Chunk, error) {
	chnkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout)
	chnk, err := file.getChunk(chnkIdx)
	if err != nil {
		return chnk, err
	}

	prevChnk, err := file.loadChunk(chnkIdx - 1)
	if err == nil {
		// Schedule Upload for the previous chunk
		scheduleUpload(prevChnk, file)
	}
	releaseChnk, err := file.loadChunk(chnkIdx - 3)
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
	return chnk, nil
}

func scheduleDownload(chnk *models.Chunk, file *File) {
	select {
	case <-chnk.ScheduleDownload:
	default:
		close(chnk.ScheduleDownload)
		t := &task{
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
			chunk:      chnk,
			fileLayout: *file.FileMetadata.FileLayout,
			get_chunk:  false,
		}
		fileIOMgr.wp.tasks <- t
	}
}
