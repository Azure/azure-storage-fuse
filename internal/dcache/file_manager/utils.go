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
	"math/rand"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/gc"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	gouuid "github.com/google/uuid"
)

var (
	ErrFileNotReady error = errors.New("Dcache file not in ready state")
)

//go:generate $ASSERT_REMOVER $GOFILE

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
	// getChunkSize() must be called for a finalized file which will have size >= 0.
	common.Assert(file.FileMetadata.Size >= 0, file.FileMetadata.Size)
	size := min(file.FileMetadata.Size-
		getChunkStartOffsetFromFileOffset(offset, &file.FileMetadata.FileLayout),
		file.FileMetadata.FileLayout.ChunkSize)
	common.Assert(size >= 0, size)
	return size
}

func isOffsetChunkStarting(offset int64, fileLayout *dcache.FileLayout) bool {
	return (offset%fileLayout.ChunkSize == 0)
}

func getNumChunksFromBytes(bytes int64, fileLayout *dcache.FileLayout) int64 {
	return (bytes + fileLayout.ChunkSize - 1) / fileLayout.ChunkSize
}

func getMVForChunk(chunk *StagedChunk, fileMetadata *dcache.FileMetadata) string {
	numMvs := int64(len(fileMetadata.FileLayout.MVList))

	// Must have full strip worth of MVs.
	common.Assert(numMvs == fileMetadata.FileLayout.StripeWidth,
		numMvs, fileMetadata.FileLayout.StripeWidth, fileMetadata.FileLayout.ChunkSize)
	common.Assert(numMvs > 0, numMvs)
	// For writes file size won't be set yet, for reads we must be reading within the file.
	common.Assert((fileMetadata.Size == -1) ||
		((chunk.Idx * fileMetadata.FileLayout.ChunkSize) < fileMetadata.Size))

	return fileMetadata.FileLayout.MVList[chunk.Idx%numMvs]
}

// Does all file Init Process for creation of the file.
func NewDcacheFile(fileName string) (*DcacheFile, error) {
	//
	// Do not allow file creation in a readonly cluster.
	//
	if cm.IsClusterReadonly() {
		err := fmt.Errorf("Cannot create file %s, cluster is readonly!", fileName)
		log.Err("DistributedCache[FM]::NewDcacheFile: %v", err)
		return nil, syscall.EROFS
	}

	fileMetadata := &dcache.FileMetadata{
		Filename: fileName,
		State:    dcache.Writing,
		Size:     -1,
		FileID:   gouuid.New().String(),
	}
	common.Assert(common.IsValidUUID(fileMetadata.FileID))

	chunkSize := cm.GetCacheConfig().ChunkSizeMB * common.MbToBytes
	stripeWidth := cm.GetCacheConfig().StripeWidth

	fileMetadata.FileLayout = dcache.FileLayout{
		ChunkSize:   int64(chunkSize),
		StripeWidth: int64(stripeWidth),
		MVList:      make([]string, stripeWidth),
	}

	// Get active MV's from the clustermap
	activeMVs := cm.GetActiveMVNames()

	//
	// Cannot create file if we don't have enough active MVs.
	//
	if len(activeMVs) < int(stripeWidth) {
		err := fmt.Errorf("Cannot create file %s, active MVs (%d) < stripeWidth (%d)",
			fileName, len(activeMVs), stripeWidth)
		log.Err("DistributedCache[FM]::NewDcacheFile: %v", err)
		return nil, err
	}

	//
	// Shuffle the slice and pick starting numMVs.
	//
	// TODO: For very large number of MVs, we can avoid shuffling all and just picking numMVs randomly.
	//
	rand.Shuffle(len(activeMVs), func(i, j int) {
		activeMVs[i], activeMVs[j] = activeMVs[j], activeMVs[i]
	})

	// Pick starting numMVs from the active MVs.
	for i := range stripeWidth {
		fileMetadata.FileLayout.MVList[i] = activeMVs[i]
	}

	log.Debug("DistributedCache[FM]::NewDcacheFile: Initial metadata for file %s %+v",
		fileName, fileMetadata)

	fileMetadataBytes, err := json.Marshal(fileMetadata)
	if err != nil {
		log.Err("DistributedCache[FM]::NewDcacheFile: FileMetadata marshalling failed for file %s %+v: %v",
			fileName, fileMetadata, err)
		return nil, err
	}

	eTag, err := mm.CreateFileInit(fileName, fileMetadataBytes)
	if err != nil {
		log.Err("DistributedCache::NewDcacheFile: CreateFileInit failed for file %s: %v",
			fileName, err)
		return nil, err
	}

	// This Etag is used while finalizing the file.
	return &DcacheFile{
		FileMetadata:     fileMetadata,
		finalizeEtag:     eTag,
		endReleaseChunks: make(chan struct{}),
	}, nil
}

// Gets the metadata of the file from the metadata store.
func GetDcacheFile(fileName string) (*dcache.FileMetadata, *internal.ObjAttr, error) {
	// Fetch file metadata from metadata store.
	fileMetadataBytes, fileSize, fileState, openCount, prop, err := mm.GetFile(fileName, false /* isDeleted */)
	if err != nil {
		//todo : See if we can have error other that ENOENT here.
		return nil, nil, err
	}

	var fileMetadata dcache.FileMetadata
	err = json.Unmarshal(fileMetadataBytes, &fileMetadata)
	if err != nil {
		err = fmt.Errorf("DistributedCache[FM]::GetDcacheFile: File metadata unmarshal failed for file %s: %v",
			fileName, err)
		common.Assert(false, err)
		return nil, nil, err
	}

	// Following fields must be ignored by unmarshal.
	common.Assert(len(fileMetadata.State) == 0, fileMetadata.State, fileMetadata)
	common.Assert(fileMetadata.Size == 0, fileMetadata.Size, fileMetadata)
	common.Assert(fileMetadata.OpenCount == 0, fileMetadata.OpenCount, fileMetadata)

	fileMetadata.State = fileState

	//
	// Filesize can be following under various file states:
	// - When file is being written, it must be -1.
	// - When file is ready, it must be >= 0.
	//
	common.Assert((fileMetadata.State == dcache.Writing && fileSize == -1) ||
		(fileMetadata.State == dcache.Ready && fileSize >= 0),
		fmt.Sprintf("file: %s, file metadata: %+v, fileSize: %d", fileName, fileMetadata, fileSize))

	fileMetadata.Size = fileSize
	common.Assert(fileMetadata.Size >= -1, fileName, fileMetadata.Size, fileMetadata)

	fileMetadata.OpenCount = openCount
	common.Assert(fileMetadata.OpenCount >= 0, fileName, fileMetadata.OpenCount, fileMetadata)

	return &fileMetadata, prop, nil
}

// Does all init process for opening the file.
func OpenDcacheFile(fileName string) (*DcacheFile, error) {
	fileMetadata, prop, err := GetDcacheFile(fileName)
	if err != nil {
		return nil, err
	}

	common.Assert(prop != nil, fileName)

	//
	// This is to prevent files which are being created, from being opened.
	//
	if fileMetadata.State != dcache.Ready {
		log.Info("DistributedCache[FM]::OpenDcacheFile: File %s is not in ready state, metadata: %+v",
			fileName, fileMetadata)
		return nil, ErrFileNotReady
	}

	// Finalized files must have size >= 0.
	common.Assert(fileMetadata.Size >= 0, fileMetadata.Size)

	//
	// Increment the open count, if safe deletes is enabled.
	// We pass 'prop' to mm.OpenFile() so that it can directly try to update the "opencount" property
	// w/o needing to do a GetPropertiesFromStorage() call. For the most common case this will work,
	// unless some other node/thread opens the file between the GetDcacheFile() above and till mm.OpenFile()
	// increments the opencount.
	//
	if fileIOMgr.safeDeletes {
		newOpenCount, err := mm.OpenFile(fileName, prop)
		_ = newOpenCount
		if err != nil {
			err = fmt.Errorf("failed to increment open count for file %s: %v", fileName, err)
			log.Err("DistributedCache[FM]::OpenDcacheFile: %v", err)
			common.Assert(false, fileName, err)
			return nil, err
		}

		common.Assert(newOpenCount > 0, newOpenCount, fileName)
	}

	dcacheFile := &DcacheFile{
		FileMetadata:     fileMetadata,
		chunksQueue:      make(chan *StagedChunk, 64),
		endReleaseChunks: make(chan struct{}),
		fullQueueSignal:  make(chan struct{}, 1),
		attr:             prop,
	}
	dcacheFile.lastReadaheadChunkIdx.Store(-1)
	dcacheFile.wg.Add(1)
	go dcacheFile.cleanupChunks()

	return dcacheFile, nil
}

func DeleteDcacheFile(fileName string) error {
	log.Debug("DistributedCache[FM]::DeleteDcacheFile : file: %s", fileName)

	fileMetadata, _, err := GetDcacheFile(fileName)
	if err != nil {
		log.Err("DistributedCache[FM]::DeleteDcacheFile : failed to delete file %s: %v", fileName, err)
		// If err is ENOENT, then possibly file was deleted by some other node before us.
		common.Assert(err == syscall.ENOENT, fileName)
		return err
	}

	//
	// Prevent deletion of files which are being created.
	//
	// TODO: We should allow deleting stale files which are left in creating state indefinitely due to
	//       blobfuse crashing between createFileInit() and createFileFinalize().
	//
	if fileMetadata.State != dcache.Ready {
		log.Info("DistributedCache[FM]::DeleteDcacheFile: File %s is not in ready state, metadata: %+v",
			fileName, fileMetadata)
		return syscall.ENOENT
	}

	//
	// Deleting a dcache file amounts to renaming it to a special name mdRoot/Objects/<fileId>.
	// This is useful for tracking file chunks for garbage collection as well as for the POSIX requirement
	// that the file data should be available till the last open fd is closed.
	//
	err = mm.RenameFileToDeleting(fileName, fileMetadata.FileID)
	if err != nil {
		log.Err("DistributedCache[FM]::DeleteDcacheFile: Failed to rename file %s -> %s: %v",
			fileName, fileMetadata.FileID, err)
		return err
	}

	//
	// Pass the file to garbage collector, which will later delete the chunks when safe to do so.
	// If safe-deletes config option is off then the file chunks can be deleted immediately o/w they
	// will be deleted when the file OpenCount falls to 0.
	//
	gc.ScheduleChunkDeletion(fileMetadata)

	return nil
}

// Creates the chunk and allocates the chunk buf
func NewStagedChunk(idx int64, file *DcacheFile, allocateBuf bool) (*StagedChunk, error) {
	var buf []byte
	var err error

	if allocateBuf {
		buf, err = dcache.GetBuffer()
		if err != nil {
			return nil, err
		}
	}

	return &StagedChunk{
		Idx:           idx,
		Len:           0,
		Buf:           buf,
		Err:           make(chan error, 1),
		IsBufExternal: !allocateBuf,
		Dirty:         atomic.Bool{},
		UpToDate:      atomic.Bool{},
		XferScheduled: atomic.Bool{},
	}, nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
