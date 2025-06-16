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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	rm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/replication_manager"
)

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
	// When deletion failed for one or more MVs, it will be retired from this here.
	//
	retryMvList []string
	retryCnt    int
}

func Start() {
	//
	// GC'ing 100 files chunks at a time should be sufficient.
	// TODO: Experiment on large clusters and large files see if we need to increase/decrease.
	//
	gc = &GcInfo{
		numGcWorkers:     100,
		deletedFileQueue: make(chan *gcFile, 100),
	}

	for range gc.numGcWorkers {
		go gc.worker()
	}

	log.Info("GC::startGC: started %d go routines for GC", gc.numGcWorkers)
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
	if gcFile.retryCnt > 20 {
		//
		// The periodic GC thread will soon schedule this gcFile again for the deletion while listing the deleted files.
		//
		log.Warn("GC::removeAllChunksForFile: file: %s, retryCnt: %d, skipping the deletion of the chunks as max retries used",
			gcFile.file.Filename, gcFile.retryCnt)
	}

	log.Debug("GC::removeAllChunksForFile: file: %s, retryCnt: %d", gcFile.file.Filename, gcFile.retryCnt)
	var errCM error
	newRetryMvList := make([]string, 0)
	gcFile.retryCnt++

	mvs := gcFile.file.FileLayout.MVList
	common.Assert(mvs != nil)

	if len(gcFile.retryMvList) != 0 {
		mvs = gcFile.retryMvList
	}

	if gcFile.file.Size == 0 {
		//
		// There are no chunks assosiated with this file, remove the metadata file directly.
		//
		goto deleteMetadataFile
	}

	//
	// Ensure the clustermap is refreshed from what we have cached right now. If there is an error while doing an RPC
	// call for retrying the clustermap. We fail the call here and GC will retry the RemoveMV call again for those
	// failed MV's.
	//
	errCM = cm.RefreshClusterMap(0)
	if errCM != nil {
		log.Err("ReplicationManager::RemoveMV: Failed to refresh the cluster map, file: %s, file ID: %s: %v",
			gcFile.file.Filename, gcFile.file.FileID, errCM)
		common.Assert(false, gcFile.file.Filename, errCM)
		//
		// Schedule the delete file again for this file in GC.
		//
		newRetryMvList = mvs
	} else {

		for _, mvName := range mvs {
			removeMvRequest := &rm.RemoveMvRequest{
				FileID: gcFile.file.FileID,
				MvName: mvName,
			}

			_, err := rm.RemoveMV(removeMvRequest)
			if err != nil {
				log.Err("GC::removeAllChunksForFile: Failed to delete chunks from MV: %s, file: %s: %v",
					mvName, gcFile.file.Filename, err)
				newRetryMvList = append(newRetryMvList, mvName)
			}
		}
	}

	if len(newRetryMvList) != 0 {
		//
		// Retry this gcFile with new MVs again. Schedule the file Non-blocking to release this worker for the other
		// files which are need to be GC'ed.
		//
		gcFile.retryMvList = newRetryMvList

		go func() {
			gc.deletedFileQueue <- gcFile
		}()

		return
	}

deleteMetadataFile:
	// After removing all the chunks from all the rvs, we can remove the file layout.
	deletedFile := dcache.GetDeletedFileName(gcFile.file.Filename, gcFile.file.FileID)
	log.Debug("GC::removeAllChunksForFile: removing file layout for file: %s", deletedFile)

	err := metadata_manager.DeleteFile(deletedFile)
	if err != nil {
		log.Err("GC::removeAllChunksForFile: failed to remove file layout for file: %s: %v", deletedFile, err)
		common.Assert(false, gcFile.file, err)
		return
	}

}

func AsyncFileChunkGarbageCollector(file *dcache.FileMetadata) {
	gcFile := &gcFile{
		file:        file,
		retryCnt:    1,
		retryMvList: make([]string, 0),
	}
	gc.deletedFileQueue <- gcFile
}
