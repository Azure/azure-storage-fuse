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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
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
	//
	// Channel to signal that periodic GC go routine to stop.
	//
	done chan struct{}
	//
	// If the metadata file for the "file that was deleted" is not deleted in this timeout then periodic GC will
	// reschedule the delete from the node where the timeout has triggered.
	//
	deleteTimeOut time.Duration
	//
	// Time to trigger the Periodic GC go routine which will reclaim the chunks for the stale files.
	//
	interval time.Duration
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
		done:             make(chan struct{}),
		//
		// The deleteTimeOut is kept to 5 minutes, The time taken to delete a file that was scheduled in worst case is:
		// (((stripeSize / chunkSize) * numReplicas) RPC calls + timeTaken to delete Metadata file + clustermap Update) * (Queue Size / numWorkers) for GC
		// (((16M/4M) * 3) 100ms for each RPC call + 300ms + 300ms) * (1000 / 100) = 18000ms in worst case maybe.
		// The time taken to delete the last file that was scheduled into the GC queue in worst case would be ~18s
		// Hence 5 minutes can be taken as reasonable time to say that the node which is deleting this file has went down.
		//
		deleteTimeOut: 5 * time.Minute,
		interval:      10 * time.Minute,
	}

	// Start Periodic GC to reclaim the chunks for the Stale files.
	go func() {
		pollTicker := time.NewTicker(gc.interval)

		for {
			select {
			case <-gc.done:
				log.Info("GC::Stopping Periodic GC go routine")
				break
			case <-pollTicker.C:
				log.Debug("GC:: Periodic GC triggered")
				gc.scheduleDeleteForStaleFiles()
			}
		}
	}()

	for range gc.numGcWorkers {
		go gc.worker()
	}

	log.Info("GC::startGC: started %d go routines for GC for deleted files chunks", gc.numGcWorkers)
}

func End() {
	close(gc.deletedFileQueue)
	gc.done <- struct{}{}
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

	rescheduleFile := func() {
		if gcFile.file.OpenCount == 0 {
			time.Sleep(5 * time.Second)
		} else {
			//
			// This is kept to higher value to avoid frequent storage calls as there is file IO going on this handle.
			//
			time.Sleep(30 * time.Second)
		}
		gc.deletedFileQueue <- gcFile
	}

	// Refresh the OpenCount for this file.
	if gcFile.file.OpenCount > 0 {
		dcFile, err := getDeletedFile(gcFile.file.FileID)
		if err != nil {
			if err == syscall.ENOENT {
				log.Warn("GC::removeAllChunksForFile: Failed to Refresh the opencount for file: %s[%s]: %v, skipping",
					gcFile.file.Filename, gcFile.file.FileID, err)
				return
			} else {
				// Reschedule the file again in this case.
				log.Err("GC::removeAllChunksForFile: Failed to Refresh the opencount for file: %s[%s]: %v",
					gcFile.file.Filename, gcFile.file.FileID, err)
				common.Assert(false, *gcFile.file, err)
			}
		} else {
			gcFile.file = dcFile
		}

		if gcFile.file.OpenCount > 0 {
			go rescheduleFile()
			return
		}
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

		go rescheduleFile()

		return
	}

deleteMetadataFile:
	// After removing all the chunks from all the rvs, we can remove the file layout.
	log.Debug("GC::removeAllChunksForFile: removing file layout for file: %s [%s]", gcFile.file.Filename, gcFile.file.FileID)

	err := mm.DeleteFile(gcFile.file.FileID)
	if err != nil {
		// This will cause the file to hang around for ever. Such files would be GC'ed in the periodic scan.
		log.Err("GC::removeAllChunksForFile: failed to remove file layout for file: %s[%s]: %v", gcFile.file.Filename, gcFile.file.FileID, err)
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

// A file can be in stale state if,
//  1. The node who is responsible for deletion has crashed before the removal of the chunks.
//  2. OpenCount of the file will not comeback to zero, if the node responsible for opening the file has crashed before
//     closing the file.
//
// TODO: for the case2, deletion of such files can be given directly to the user where they can delete those files
// explicitly thru debugfs?
func (gc *GcInfo) scheduleDeleteForStaleFiles() {
	log.Debug("GC::scheduleDeleteForStaleFiles: Started")
	//
	// List all the deleted files in the storage.
	//
	deletedFiles, err := mm.ListDeletedFiles()
	if err != nil {
		log.Err("GC::scheduleDeleteForStaleFiles: Failed to List the Deleted Files [%v]", err)
		common.Assert(false, err)
	}

	// Schedule the delete for all the files that were timedout in the cache directory.
	for _, attr := range deletedFiles {
		// Extract the openCount from the attribute.
		openCount, err := getOpenCountForDeletedFile(attr)
		if err != nil {
			log.Info("GC::scheduleDeleteForStaleFiles: Failed to get the opencount for file %s[%s]", attr.Path, attr.Name)
			common.Assert(false, *attr, err)
			continue
		}

		// Assuming the Node responsible for deletion of this file went down when the metadata file is not deleted
		// before deleteTimeOut.
		if time.Since(attr.Mtime) >= gc.deleteTimeOut && openCount == 0 {
			log.Info("GC::scheduleDeleteForStaleFiles: Deleting stale fileID %s", attr.Name)

			// Get the metadata of the deleted file and schedule the delete.
			dcFile, err := getDeletedFile(attr.Name)
			if err != nil {
				if err == syscall.ENOENT {
					log.Warn("GC::scheduleDeleteForStaleFiles: Failed to get the deleted file %s: %v", err)
				} else {
					log.Err("GC::scheduleDeleteForStaleFiles: Failed to get the deleted file %s: %v", err)
				}
				continue
			}

			// Schedule the file for GC.
			gcFile := &gcFile{
				file:         dcFile,
				retryCnt:     0,
				removeMVList: dcFile.FileLayout.MVList,
			}
			gc.deletedFileQueue <- gcFile
		}
	}
}

func getOpenCountForDeletedFile(attr *internal.ObjAttr) (int, error) {
	openCountStr, ok := attr.Metadata["opencount"]
	if !ok {
		err := fmt.Errorf("GC::getOpenCountForDeletedFile: File opencount not found in metadata for path %s", attr.Path)
		log.Err("%v", err)
		common.Assert(false, err)
		return -1, err
	}

	openCount, err := strconv.Atoi(*openCountStr)
	if err != nil {
		err := fmt.Errorf("GC::getOpenCountForDeletedFile: Failed to parse open count for path %s with value %s: %v",
			attr.Path, *openCountStr, err)
		log.Err("%v", err)
		common.Assert(false, err)
		return -1, err
	}

	if openCount < 0 {
		err := fmt.Errorf("GC::getOpenCountForDeletedFile: open count -ve for path %s with value %d: %v",
			attr.Path, openCount, err)
		log.Err("%v", err)
		common.Assert(false, err)
		return -1, err
	}

	return openCount, nil
}

// Get the file metadata of the deleted file based by their fileID.
func getDeletedFile(fileId string) (*dcache.FileMetadata, error) {
	fileMetadataBytes, fileSize, _, openCount, _, err := mm.GetFile(fileId, true)
	if err != nil {
		log.Err("GC::getDeletedFile: Failed to get metadata file content for file %s: %v", fileId, err)
		common.Assert(errors.Is(err, syscall.ENOENT), err)
		return nil, err
	}

	var fileMetadata dcache.FileMetadata
	err = json.Unmarshal(fileMetadataBytes, &fileMetadata)
	if err != nil {
		err = fmt.Errorf("File metadata unmarshal failed for fileId %s: %v", fileId, err)
		common.Assert(false, err)
		return nil, err
	}

	// Following fields must be ignored by unmarshal.
	common.Assert(len(fileMetadata.State) == 0, fileMetadata.State, fileMetadata)
	common.Assert(fileMetadata.Size == 0, fileMetadata.Size, fileMetadata)
	common.Assert(fileMetadata.OpenCount == 0, fileMetadata.OpenCount, fileMetadata)

	common.Assert(fileSize >= 0, fileId, fileMetadata, fileSize)
	common.Assert(openCount >= 0, fileId, fileMetadata.OpenCount, fileMetadata)

	fileMetadata.Size = fileSize
	fileMetadata.OpenCount = openCount

	return &fileMetadata, nil
}
