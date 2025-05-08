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
	"math/rand"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	gouuid "github.com/google/uuid"
)

func getChunkStartOffsetFromFileOffset(offset int64, fileLayout *dcache.FileLayout) int64 {
	return getChunkIdxFromFileOffset(offset, fileLayout) * fileLayout.ChunkSize
}

func getChunkIdxFromFileOffset(offset int64, fileLayout *dcache.FileLayout) int64 {
	return offset / fileLayout.ChunkSize
}

func getChunkOffsetFromFileOffset(offset int64, fileLayout *dcache.FileLayout) int64 {
	return offset - getChunkStartOffsetFromFileOffset(offset, fileLayout)
}

func getChunkSize(offset int64, file *DcacheFile) int64 {
	return min(file.FileMetadata.Size-
		getChunkStartOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout),
		file.FileMetadata.FileLayout.ChunkSize)
}

func isOffsetChunkStarting(offset int64, fileLayout *dcache.FileLayout) bool {
	return (offset%fileLayout.ChunkSize == 0)
}

func getMVForChunk(chunk *StagedChunk, fileMetadata *dcache.FileMetadata) string {
	numMvs := int64(len(fileMetadata.FileLayout.MVList))
	// Must have full strip worth of MVs.
	common.Assert(numMvs == (fileMetadata.FileLayout.StripeSize / fileMetadata.FileLayout.ChunkSize))
	// For writes file size won't be set yet, for reads we must be reading within the file.
	common.Assert((fileMetadata.Size == -1) ||
		((chunk.Idx * fileMetadata.FileLayout.ChunkSize) < fileMetadata.Size))

	return fileMetadata.FileLayout.MVList[chunk.Idx%numMvs]
}

// Does all file Init Process for creation of the file.
func NewDcacheFile(fileName string) (*DcacheFile, error) {
	fileMetadata := &dcache.FileMetadata{
		Filename: fileName,
		State:    dcache.Writing,
		Size:     -1,
		FileID:   gouuid.New().String(),
	}
	common.Assert(common.IsValidUUID(fileMetadata.FileID))

	chunkSize := clustermap.GetCacheConfig().ChunkSize
	stripeSize := clustermap.GetCacheConfig().StripeSize

	common.Assert(stripeSize%chunkSize == 0, fmt.Sprintf("Stripe Size %d is not divisibe by chunkSize %d", stripeSize, chunkSize))
	numMVs := stripeSize / chunkSize

	fileMetadata.FileLayout = dcache.FileLayout{
		ChunkSize:  int64(chunkSize),
		StripeSize: int64(stripeSize),
		MVList:     make([]string, numMVs),
	}

	// Get active MV's from the clustermap
	activeMVs := clustermap.GetActiveMVNames()
	common.Assert(len(activeMVs) >= int(numMVs))
	// shuffle the slice and pick starting 3.
	rand.Shuffle(len(activeMVs), func(i, j int) {
		activeMVs[i], activeMVs[j] = activeMVs[j], activeMVs[i]
	})
	// Pick starting numMVs from the active MVs
	for i := range numMVs {
		fileMetadata.FileLayout.MVList[i] = activeMVs[i]
	}

	log.Debug("DistributedCache[FM]::NewDcacheFile : Initial metadata for file: %s, %+v",
		fileName, fileMetadata)

	fileMetadataBytes, err := json.Marshal(fileMetadata)
	if err != nil {
		log.Err("DistributedCache[FM]::NewDcacheFile : FileMetadata marshalling fail, file: %s", fileName)
		return nil, err
	}
	err = mm.CreateFileInit(fileName, fileMetadataBytes)
	if err != nil {
		log.Err("DistributedCache::NewDcacheFile : File Creation failed for file :  %s with err : %s", fileName, err.Error())
		return nil, err
	}
	return &DcacheFile{
		FileMetadata: fileMetadata,
	}, nil
}

// Does all init process for opening the file.
func OpenDcacheFile(fileName string) (*DcacheFile, error) {
	fileMetadataBytes, fileSize, err := mm.GetFile(fileName)
	if err != nil {
		//todo : See if we can have error other that ENOENT here.
		return nil, err
	}

	var fileMetadata dcache.FileMetadata
	err = json.Unmarshal(fileMetadataBytes, &fileMetadata)
	if err != nil {
		err = fmt.Errorf("DistributedCache[FM]::OpenDcacheFile : failed to unmarshal filemetadata file: %s, err: %s",
			fileName, err.Error())
		common.Assert(false, err)
		return nil, err
	}

	//
	// Filesize can be following under various file states:
	// - When file is being written, it must be -1.
	// - When file is ready, it must be >= 0.
	// - A file can be deleted from ready or writing state, so in deleting state fileSize can be anything.
	//
	common.Assert((fileMetadata.State == dcache.Writing && fileSize == -1) ||
		(fileMetadata.State == dcache.Ready && fileSize >= 0) ||
		(fileMetadata.State == dcache.Deleting),
		fmt.Sprintf("file: %s, file metadata: %+v, fileSize: %d", fileName, fileMetadata, fileSize))

	// Return ENOENT if the file is not in ready state.
	if fileMetadata.State != dcache.Ready {
		log.Info("DistributedCache[FM]::OpenDcacheFile : File : %s is not in ready state, metadata: %+v",
			fileName, fileMetadata)
		return nil, syscall.ENOENT
	}

	fileMetadata.Size = fileSize

	return &DcacheFile{
		FileMetadata: &fileMetadata,
	}, nil
}

// Creates the chunk and allocates the chunk buf
func NewStagedChunk(idx int64, file *DcacheFile) (*StagedChunk, error) {
	buf, err := fileIOMgr.bp.getBuffer()
	if err != nil {
		return nil, err
	}
	return &StagedChunk{
		Idx:              idx,
		Len:              0,
		Buf:              buf,
		Err:              make(chan error),
		ScheduleDownload: make(chan struct{}),
		ScheduleUpload:   make(chan struct{}),
	}, nil
}
