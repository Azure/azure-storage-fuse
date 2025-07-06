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

package gc

import (
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
)

//go:generate $ASSERT_REMOVER $GOFILE

type GcInfo struct {
	//
	// Deletes these many number of files at any time, excess files need to wait for one or more
	// workers to get freed.
	//
	numGcWorkers     int
	deletedFileQueue chan *gcFile
	wg               sync.WaitGroup
}

var gc *GcInfo

type gcFile struct {
	file *dcache.FileMetadata
	//
	// List of MVs to remove. It starts as file.FileLayout.MVList, but as MVs get deleted and one or more
	// MVs fail, this will contain only those MVs which still need to be deleted in later retries.
	//
	removeMVList []string
	retryCnt     int
}

func Start() {
	//
	// GC'ing 100 files chunks at a time should be sufficient.
	// TODO: Experiment on large clusters and large files see if we need to increase/decrease.
	//
	gc = &GcInfo{
		numGcWorkers: 100,
		// Allow more requests to be queued than workers.
		deletedFileQueue: make(chan *gcFile, 1000),
	}

	for range gc.numGcWorkers {
		go gc.worker()
	}

	log.Info("GC::startGC: started %d go routines for GC for deleted files chunks", gc.numGcWorkers)
}

func End() {
	close(gc.deletedFileQueue)
	gc.wg.Wait()
}

func (gc *GcInfo) worker() {
	gc.wg.Add(1)
	defer gc.wg.Done()

	// Get the next deleted file from the queue and remove its chunks.
	for gcFile := range gc.deletedFileQueue {
		gc.removeAllChunksForFile(gcFile)
	}
}

func (gc *GcInfo) removeAllChunksForFile(gcFile *gcFile) {
	// We should be called only when we have at least one MV to delete.
	common.Assert(len(gcFile.removeMVList) > 0, gcFile.file)

	log.Debug("GC::removeAllChunksForFile: file: %s (%s), retryCnt: %d, MVs to delete: %v",
		gcFile.file.Filename, gcFile.file.FileID, gcFile.retryCnt, gcFile.removeMVList)

	//
	// Log a warning log if we are not able to delete the file chunks after too many retries.
	// In the common case we should be able to delete all the chunks in one/few call(s).
	//
	if gcFile.retryCnt > 100 {
		log.Warn("GC::removeAllChunksForFile: file: %s (%s), retryCnt: %d, too many failures! %v of %v still need to be deleted",
			gcFile.file.Filename, gcFile.file.FileID, gcFile.retryCnt,
			gcFile.removeMVList, gcFile.file.FileLayout.MVList)
		// TODO: This may fail when resync takes a long time due to too many chunks.
		common.Assert(false, gcFile.file, gcFile)
	}

	var errCM error
	retryMVList := make([]string, 0)

	if gcFile.file.Size == 0 {
		//
		// There are no chunks associated with this file, remove the metadata file directly.
		//
		log.Debug("GC::removeAllChunksForFile: file: %s (%s), 0 byte file, no chunks to remove",
			gcFile.file.Filename, gcFile.file.FileID)
		common.Assert(gcFile.retryCnt == 0, gcFile.file, gcFile)
		goto deleteMetadataFile
	}

	gcFile.retryCnt++

	//
	// Refresh the clustermap to make sure we have the latest list of component RVs for each of the file MVs.
	// Note that we need to delete chunks from all the component RVs. In case of any error in this function,
	// we simply requeue the file for deletion and it'll be attempted again by one of the GC workers.
	//
	errCM = cm.RefreshClusterMap(0)
	if errCM != nil {
		log.Err("ReplicationManager::RemoveMV: Failed to refresh the cluster map, file: %s, file ID: %s: %v",
			gcFile.file.Filename, gcFile.file.FileID, errCM)
		common.Assert(false, gcFile.file.Filename, errCM)
		//
		// Schedule the delete file again for this file in GC.
		//
		retryMVList = gcFile.removeMVList
	} else {
		//
		// Call RemoveMV() for each MV of the file.
		// It'll cause file chunks to be deleted from all component RVs by sending a RemoveChunk request to
		// each of them.
		// RemoveMV() returns success when all chunks of the file are successfully deleted from all component RVs.
		//
		for _, mvName := range gcFile.removeMVList {
			removeMvRequest := &rm.RemoveMvRequest{
				FileID: gcFile.file.FileID,
				MvName: mvName,
			}

			_, err := rm.RemoveMV(removeMvRequest)
			if err != nil {
				log.Err("GC::removeAllChunksForFile: Failed to delete one or more chunks from MV: %s, file: %s (%s): %v",
					mvName, gcFile.file.Filename, gcFile.file.FileID, err)
				// Queue the failed MV for deletion when GC retries file deletion.
				retryMVList = append(retryMVList, mvName)
			} else {
				log.Debug("GC::removeAllChunksForFile: deleted all chunks from MV: %s, file: %s (%s)",
					mvName, gcFile.file.Filename, gcFile.file.FileID)
			}
		}
	}

	if len(retryMVList) != 0 {
		//
		// Not all MVs fully deleted, schedule file deletion with the remaining MVs.
		// We queue it to the end of the deletedFileQueue to let other file deletes proceed.
		// We queue it for retrying after a wait hoping for the error condition to improve.
		//
		// TODO: Make sure this doesn't cause too many go routines to be created.
		//
		gcFile.removeMVList = retryMVList

		go func() {
			time.Sleep(5 * time.Second)
			gc.deletedFileQueue <- gcFile
		}()

		return
	}

deleteMetadataFile:
	// After removing all the chunks from all the rvs, we can remove the file layout.
	deletedFile := dcache.GetDeletedFileName(gcFile.file.Filename, gcFile.file.FileID)
	log.Debug("GC::removeAllChunksForFile: removing file layout for file: %s", deletedFile)

	err := mm.DeleteFile(deletedFile)
	if err != nil {
		// This will cause the file to hang around for ever.
		log.Err("GC::removeAllChunksForFile: failed to remove file layout for file: %s: %v", deletedFile, err)
		common.Assert(false, gcFile.file, err)
		return
	}
}

func ScheduleChunkDeletion(file *dcache.FileMetadata) {
	gcFile := &gcFile{
		file:         file,
		retryCnt:     0,
		removeMVList: file.FileLayout.MVList,
	}
	gc.deletedFileQueue <- gcFile
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
