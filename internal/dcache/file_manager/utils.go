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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/gc"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	gouuid "github.com/google/uuid"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	ErrFileNotReady error = errors.New("Dcache file not in ready state")
)

// Returns the actual file size if finalized else the partial size.
func getFileSize(file *dcache.FileMetadata) int64 {
	common.Assert((file.Size >= 0) == (file.State == dcache.Ready), *file)
	common.Assert((file.Size == -1) == (file.State == dcache.Writing), *file)
	common.Assert(file.PartialSize >= 0, *file)

	if file.Size > 0 {
		// PartialSize can never be more than actual file size.
		common.Assert(file.Size >= file.PartialSize, file.Size, file.PartialSize, *file)
		return file.Size
	} else {
		return file.PartialSize
	}
}

// Index of the chunk that contains the given file offset.
func getChunkIdxFromFileOffset(offset, chunkSize int64) int64 {
	return offset / chunkSize
}

// Offset within the chunk corresponding to the given file offset.
func getChunkOffsetFromFileOffset(offset, chunkSize int64) int64 {
	return offset % chunkSize
}

// File offset of the start of the chunk.
func getChunkStartOffset(chunkIdx, chunkSize int64) int64 {
	return chunkIdx * chunkSize
}

// File offset of the last byte of the chunk + 1.
// Note: This is one byte past the end of the chunk.
func getChunkEndOffset(chunkIdx, chunkSize int64) int64 {
	return (chunkIdx + 1) * chunkSize
}

// Returns the size of the chunk containing the given file offset.
// For all chunks except the last chunk, this will be equal to chunkSize.
// Can be called for both finalized and non-finalized files.
func getChunkSize(offset int64, file *DcacheFile) int64 {
	chunkIdx := getChunkIdxFromFileOffset(offset, file.FileMetadata.FileLayout.ChunkSize)
	size := min(getFileSize(file.FileMetadata)-
		getChunkStartOffset(chunkIdx, file.FileMetadata.FileLayout.ChunkSize),
		file.FileMetadata.FileLayout.ChunkSize)

	common.Assert(size >= 0, size)
	return size
}

// Get the highest chunk index for the file.
// Works for both finalized and non-finalized files.
func getMaxChunkIdxForFile(file *dcache.FileMetadata) int64 {
	if file.Size > 0 {
		// PartialSize can never be more than actual file size.
		common.Assert(file.Size >= file.PartialSize, file.Size, file.PartialSize, *file)
		return getChunkIdxFromFileOffset(file.Size-1, file.FileLayout.ChunkSize)
	} else {
		return getChunkIdxFromFileOffset(max(file.PartialSize-1, 0), file.FileLayout.ChunkSize)
	}
}

func isOffsetChunkStarting(offset, chunkSize int64) bool {
	return (offset%chunkSize == 0)
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

	//
	// Get active MV's from the clustermap
	//
	// TODO: Allow degraded MVs to be used for placement too.
	// TODO: See if we can use some heuristics to pick MVs, instead of random.
	//
	activeMVs := cm.GetActiveMVNames()

	//
	// Shuffle the slice and pick starting numMVs.
	//
	// TODO: For very large number of MVs, we can avoid shuffling all and just picking numMVs randomly.
	//
	// rand.Shuffle(len(activeMVs), func(i, j int) {
	// 	activeMVs[i], activeMVs[j] = activeMVs[j], activeMVs[i]
	// })

	//
	// Pick starting numMVs from the active MVs.
	// If not enough MVs are active, repeat from the start of the list.
	// It's ok to pick same MV multiple times.
	//
	j := 0
	for i := range stripeWidth {
		fileMetadata.FileLayout.MVList[i] = activeMVs[int(j)%len(activeMVs)]
		j += 2
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

	//
	// This DcacheFile will be used for writing, hence it doesn't need a read pattern tracker.
	//
	dcacheFile := &DcacheFile{
		FileMetadata: fileMetadata,
		// This Etag is used while finalizing the file.
		finalizeEtag: eTag,
		StagedChunks: make(map[int64]*StagedChunk),
	}

	//
	// Contiguity tracker is used to track contiguous chunks written to the file.
	// Useful for reading partially written files.
	//
	// Note: CreateFileInit() above would create the file metadata and hence the file will start
	//       showing up in listings, so it's possible that some reader may open the file and query
	//       metadata chunk even before we create it below.
	//
	dcacheFile.CT = NewContiguityTracker(dcacheFile)

	// freeChunks semaphore is used to limit StagedChunks map to numStagingChunks.
	dcacheFile.initFreeChunks(fileIOMgr.numStagingChunks)

	return dcacheFile, nil
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
	common.Assert(fileMetadata.PartialSize == 0, fileMetadata.PartialSize, fileMetadata)
	common.Assert(fileMetadata.PartialSizeAt.IsZero(), fileMetadata.PartialSizeAt, fileMetadata)
	common.Assert(fileMetadata.OpenCount == 0, fileMetadata.OpenCount, fileMetadata)

	fileMetadata.State = fileState

	common.Assert(fileMetadata.FileLayout.ChunkSize == int64(cm.GetCacheConfig().ChunkSizeMB*common.MbToBytes),
		fileMetadata.FileLayout.ChunkSize, fileMetadata)
	common.Assert(fileMetadata.FileLayout.StripeWidth == int64(cm.GetCacheConfig().StripeWidth),
		fileMetadata.FileLayout.StripeWidth, fileMetadata)
	common.Assert(int64(len(fileMetadata.FileLayout.MVList)) == fileMetadata.FileLayout.StripeWidth,
		len(fileMetadata.FileLayout.MVList), fileMetadata)

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

	//
	// If file is currently being written, fileSize will be -1, set PartialSize in that case.
	// Note that since the metadata chunk (that holds the partial size) is updated infrequently,
	// we may not have the latest partial size. Since we create the metadata chunk on file create,
	// it must always be present for a file.
	//
	if fileMetadata.Size == -1 {
		fileMetadata.PartialSize, fileMetadata.PartialSizeAt = GetHighestUploadedByte(&fileMetadata)
		common.Assert(fileMetadata.PartialSize >= 0, fileName, fileMetadata.PartialSize, fileMetadata)
		// 5 sec to account for clock skews.
		common.Assert(!fileMetadata.PartialSizeAt.After(time.Now().Add(5*time.Second)),
			fileName, fileMetadata.PartialSizeAt, time.Now(), fileMetadata)
	}

	fileMetadata.OpenCount = openCount
	common.Assert(fileMetadata.OpenCount >= 0, fileName, fileMetadata.OpenCount, fileMetadata)

	log.Debug("DistributedCache[FM]::GetDcacheFile: File %s metadata %+v, prop %+v",
		fileName, fileMetadata, *prop)

	return &fileMetadata, prop, nil
}

// Does all init process for opening the file.
func OpenDcacheFile(fileName string, fromFuse bool) (*DcacheFile, error) {
	fileMetadata, prop, err := GetDcacheFile(fileName)
	if err != nil {
		return nil, err
	}

	common.Assert(prop != nil, fileName)

	//
	// This is to prevent files which are being created, from being opened.
	//
	if fileMetadata.State != dcache.Ready {
		common.Assert(fileMetadata.Size == -1 && fileMetadata.PartialSize >= 0, fileMetadata.Size, *fileMetadata)
		// We don't allow reading non-finalized files from fuse, but we do allow internal readers.
		if fromFuse {
			log.Err("DistributedCache[FM]::OpenDcacheFile: File %s is not in ready state, metadata: %+v",
				fileName, fileMetadata)
			return nil, ErrFileNotReady
		} else {
			log.Debug("DistributedCache[FM]::OpenDcacheFile: File %s being open'ed in non-ready state, metadata: %+v",
				fileName, fileMetadata)
		}
	} else {
		// Finalized files must have size >= 0.
		common.Assert(fileMetadata.Size >= 0, fileMetadata.Size, *fileMetadata)
	}

	//
	// Increment the open count, if safe deletes is enabled.
	// We pass 'prop' to mm.OpenFile() so that it can directly try to update the "opencount" property
	// w/o needing to do a GetPropertiesFromStorage() call. For the most common case this will work,
	// unless some other node/thread opens the file between the GetDcacheFile() above and till mm.OpenFile()
	// increments the opencount.
	//
	// TODO: Shall we support safe deletes for partial files too?
	//
	if fileIOMgr.safeDeletes && (fileMetadata.State == dcache.Ready) {
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

	//
	// This DcacheFile will be used for reading, hence it needs a read pattern tracker.
	//
	dcacheFile := &DcacheFile{
		FileMetadata: fileMetadata,
		attr:         prop,
		RPT:          NewRPTracker(fileName),
		StagedChunks: make(map[int64]*StagedChunk),
	}

	//
	// freeChunks semaphore is used to limit StagedChunks map to numReadAheadChunks plus
	// the window size supported by read pattern tracker. See NewRPTracker().
	// DO NOT make it very low else most IOs will have to wait for chunk reclaim.
	//
	dcacheFile.initFreeChunks(fileIOMgr.numReadAheadChunks + 300)
	dcacheFile.lastReadaheadChunkIdx.Store(-1)

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
	if fileMetadata.State != dcache.Ready {
		log.Info("DistributedCache[FM]::DeleteDcacheFile: File %s is not in ready state, metadata: %+v",
			fileName, fileMetadata)
		//
		// If the file is currently being written to, don't delete it, else if it is stuck in writing (likely
		// the writer node crashed) then allow delete only if it has not been updated for CommitLivenessPeriod.
		//
		if time.Since(fileMetadata.PartialSizeAt) < CommitLivenessPeriod {
			return syscall.EBUSY
		}

		log.Warn("DistributedCache[FM]::DeleteDcacheFile: File %s possibly stuck in writing state (not updated for %v), proceeding with delete",
			fileName, time.Since(fileMetadata.PartialSizeAt))
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
		//
		// RenameFileToDeleting() will fail with EEXIST if some other node/thread has already
		// deleted the file. This mostly happens when multiple deleting threads race and they
		// all get the file metadata before any of them renames it to deleting.
		// For all purposes this is equivalent to ENOENT for all but the first deleter.
		//
		if err == syscall.EEXIST {
			return syscall.ENOENT
		}
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
func NewStagedChunk(idx, offset, length int64, file *DcacheFile, allocateBuf bool) (*StagedChunk, error) {
	var buf []byte
	var err error
	//
	// Maximum time a "reclaimable" chunk is allowed in StagedChunks map. This is to allow it to
	// capture all sequential reads/writes that fall on the chunk.
	// Beyond this it may indicate chunk not receiving application IO (as the pattern may not be
	// sequential enough) and hence not much point in keeping it in the cache.
	// See below for what "reclaimable" means.
	//
	// Note: reclaimTime will typically be much less (~1-2 sec) but we keep it at 5 sec to be conservative,
	//       just in case some sequential reader is slow.
	//
	// Note: For maxWaitTime, we choose a large value (15 minutes) to avoid failing IOs due to temporary
	//       congestion in the cluster or some transient issue. We are in no hurry to fail application IOs.
	//       This is particularly important for writes, as write chunks can only be reclaimed after they
	//       have been flushed and that can take time if the cluster is slow/busy.
	//
	const reclaimTime = 5 * time.Second
	const maxWaitTime = 900 * time.Second

	startTime := time.Now()
	count := 0
	ucount := 0
loop:
	for {
		select {
		case <-file.freeChunks:
			if file.CT == nil {
				// Read file, no need to check unacked window.
				break loop
			}
			//
			// Writable files will have file.CT set, for them we need to ensure that we don't exceed
			// maxUnackedWindow else the contiguity_tracker needs to track too many chunks. Also it
			// indicates some issue with some RV(s) (node or network) so let's not add fuel to the fire.
			// Anyways, it's not much performance benefit in keeping too many chunks outstanding.
			//
			uw := file.CT.GetUnackedWindow()
			if uw > fileIOMgr.maxUnackedWindow {
				file.freeChunks <- struct{}{}

				time.Sleep(10 * time.Millisecond)
				ucount++
				if (ucount % 100) != 0 {
					continue
				}

				// Log every 1 second.

				log.Debug("DistributedCache[FM]::NewStagedChunk: chunkIdx: %d, file: %+v, unacked window %d exceeds max %d, waiting...",
					idx, *file.FileMetadata, uw, fileIOMgr.maxUnackedWindow)

				continue
			}

			// TODO: Log only if wait was more than 1 second.
			if ucount > 0 {
				log.Debug("DistributedCache[FM]::NewStagedChunk: chunkIdx: %d, file: %+v, unacked window %d now ok, after %s",
					idx, *file.FileMetadata, uw, time.Since(startTime))
			}

			break loop
		case <-time.After(10 * time.Millisecond):
			//
			// No free chunks in StagedChunks map, see if we can free some up by removing
			// "reclaimable" chunks. We have the following rules for reclaiming chunks:
			// - Dirty chunks cannot be reclaimed.
			// - Partial chunks (Len != chunkSize) cannot be reclaimed.
			// - Chunks allocated for reads can be reclaimed safely, whether they have been
			//   read fully or partially. Note that read chunks will always have Len==chunkSize.
			//
			count++
			if (count % 100) != 0 {
				continue
			}

			//
			// Every 1 second check if all chunks in StagedChunks map are "aged".
			//
			file.chunkLock.RLock()
			dirty := 0
			partial := 0
			young := 0

			chunks := make([]*StagedChunk, 0)
			for chunkIdx, chunk := range file.StagedChunks {
				_ = chunkIdx

				// Partial chunks cannot be reclaimed.
				if chunk.Len != file.FileMetadata.FileLayout.ChunkSize {
					partial++
					continue
				}

				// Dirty chunks cannot be reclaimed.
				if chunk.Dirty.Load() {
					dirty++
					continue
				}

				allocatedFor := time.Since(chunk.AllocatedAt)
				if allocatedFor < reclaimTime {
					young++
					continue
				}

				chunks = append(chunks, chunk)
			}

			_ = dirty
			_ = partial
			_ = young

			log.Debug("DistributedCache[FM]::NewStagedChunk: chunkIdx: %d, file: %+v, reclaiming %d of %d chunks in StagedChunks map (%d, %d, %d)",
				idx, *file.FileMetadata, len(chunks), len(file.StagedChunks), dirty, partial, young)

			file.chunkLock.RUnlock()

			for _, chunk := range chunks {
				file.removeChunk(chunk.Idx)
			}

			if time.Since(startTime) > maxWaitTime {
				err := fmt.Errorf("DistributedCache[FM]::NewStagedChunk: Could not reclaim any chunk after %v, while waiting for chunkIdx: %d, file: %+v",
					time.Since(startTime), idx, *file.FileMetadata)
				log.Err("%v", err)
				return nil, err
			}
		}
	}

	if allocateBuf {
		buf, err = dcache.GetBuffer()
		if err != nil {
			return nil, err
		}
	}

	//
	// length==0 means entire chunk.
	// If non-zero it means only a part of the chunk is being staged, precisely [offset, offset+length).
	// In that case we need to ensure that the length doesn't exceed the chunk boundary.
	//
	if length != 0 {
		chunkSize := int64(cm.GetCacheConfig().ChunkSizeMB * common.MbToBytes)
		length = min(length, chunkSize-offset)

	}

	chunk := &StagedChunk{
		Idx:           idx,
		Len:           length,
		Offset:        offset,
		Buf:           buf,
		Err:           make(chan error, 1),
		IsBufExternal: !allocateBuf,
		Dirty:         atomic.Bool{},
		UpToDate:      atomic.Bool{},
		XferScheduled: atomic.Bool{},
		SavedInMap:    atomic.Bool{},
		RefCount:      atomic.Int32{},
		IOTracker:     NewChunkIOTracker(),
		AllocatedAt:   time.Now(),
	}

	// Take the refcount for the original creator of the chunk.
	chunk.RefCount.Store(1)

	return chunk, nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
