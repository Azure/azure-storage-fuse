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

func NewGC() {
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

func EndGC() {
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
	log.Debug("GC::removeAllChunksForFile: file: %s, retryCnt: %d", gcFile.file.Filename, gcFile.retryCnt)
	gcFile.retryCnt++
	common.Assert(gcFile.file.Size > 0, gcFile.file)

	mvs := gcFile.file.FileLayout.MVList

	if gcFile.retryMvList != nil {
		mvs = gcFile.retryMvList
	}

	common.Assert(mvs != nil)
	newRetryMvList := make([]string, 0)

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

	if len(newRetryMvList) != 0 {
		//
		// Retry this gcFile with new MVs again. Schedule the file Non-blocking to release this worker for the other
		// files which are need to be GC'ed.
		//
		go func() {
			gc.deletedFileQueue <- gcFile
		}()

		return
	}

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
	if file.Size != 0 {
		gcFile := &gcFile{
			file:     file,
			retryCnt: 1,
		}
		gc.deletedFileQueue <- gcFile
	}
}
