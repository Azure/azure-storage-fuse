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
	// Channel to signal the periodic GC go routine to stop.
	//
	done chan struct{}
	//
	// If the metadata file for a "deleted file" is not deleted till this timeout then periodic GC will
	// reschedule the delete from the node that enumerates the deleted file.
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
		// File chunks are deleted by RemoveChunk() RPC which enumerates and deletes all chunks of
		// the file from all RVs where one or more file chunks are present. These calls are made
		// serially to each MV and for each component RV for the MV. The time taken will depend on
		// the size of the file as bigger files will have more chunks to be deleted.
		//
		// We set deleteTimeOut to a reasonable value of 5 minutes.
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
			// This is kept to higher value to avoid frequent storage calls for open files which
			// can only be deleted after all open handles are closed.
			//
			time.Sleep(30 * time.Second)
		}
		gc.deletedFileQueue <- gcFile
	}

	//
	// We cannot delete a file until its openCount drops to zero.
	// Fetch fresh openCount to take that decision.
	//
	if gcFile.file.OpenCount > 0 {
		dcFile, err := getDeletedFile(gcFile.file.FileID)
		if err != nil {
			if err == syscall.ENOENT {
				log.Warn("GC::removeAllChunksForFile: Failed to refresh opencount for file: %s [%s]: %v, skipping",
					gcFile.file.Filename, gcFile.file.FileID, err)
				return
			} else {
				// Reschedule the file again in this case.
				log.Err("GC::removeAllChunksForFile: Failed to refresh opencount for file: %s [%s]: %v",
					gcFile.file.Filename, gcFile.file.FileID, err)
				common.Assert(false, *gcFile.file, err)
			}
		} else {
			gcFile.file = dcFile
		}

		//
		// TODO: If file openCount is stuck for a long time, then we can force delete the file chunks.
		//
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
	log.Debug("GC::removeAllChunksForFile: removing file layout for file: %s [%s]",
		gcFile.file.Filename, gcFile.file.FileID)

	err := mm.DeleteFile(gcFile.file.FileID)
	if err != nil {
		// Periodic GC should delete this metadata file.
		log.Err("GC::removeAllChunksForFile: failed to remove file layout for file: %s[%s]: %v",
			gcFile.file.Filename, gcFile.file.FileID, err)
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

// Normally file chunks are deleted when DeleteDcacheFile() queues a file for chunk deletion by a call
// to ScheduleChunkDeletion(). This works fine for the most common cases, but this fails to delete file
// chunks in the following case:
// The node than ran DeleteDcacheFile(), and has the file queued for deletion, stops/crashes/restarts
// before it could delete the chunks.
//
// In such case, a periodic thread calls scheduleDeleteForStaleFiles() to requeue such files for deletion.
//
// Note that a file cannot be deleted till its openCount drops to zero, so if a node crashes after opening
// a file and before closing it, the file openCount will be stuck at non-zero and such files will never
// be deleted.
//
// TODO: Deletion of such files can be given directly to the user where they can delete those files
//	     explicitly thru debugfs? Or, we can have a timeout after which we force delete such files.

func (gc *GcInfo) scheduleDeleteForStaleFiles() {
	log.Debug("GC::scheduleDeleteForStaleFiles: Started")
	//
	// List all the deleted files in the storage.
	//
	deletedFiles, err := mm.ListDeletedFiles()
	if err != nil {
		log.Err("GC::scheduleDeleteForStaleFiles: Failed to list deleted files [%v]", err)
		common.Assert(false, err)
		// We will retry when scheduleDeleteForStaleFiles() is again called.
		return
	}

	// Schedule the delete for all the files that timed out in the cache directory.
	for _, attr := range deletedFiles {
		// Extract the openCount from the attribute.
		openCount, err := getOpenCountForDeletedFile(attr)
		if err != nil {
			log.Info("GC::scheduleDeleteForStaleFiles: Failed to get the opencount for file %s [%s]",
				attr.Path, attr.Name)
			common.Assert(false, *attr, err)
			continue
		}

		//
		// attr.Mtime will be the time when the file was delete (and its metadata file added
		// to mdRoot/Deleted/ folder. We let the "owner node" (that ran DeleteDcacheFile())
		// to delete the file and only after gc.deleteTimeOut period we assume that the node
		// went down and hence the period deleter has the responsibility of deleting the file.
		//
		if time.Since(attr.Mtime) < gc.deleteTimeOut {
			continue
		}

		// Cannot delete files that are open.
		if openCount != 0 {
			log.Info("GC::scheduleDeleteForStaleFiles: Skipping file %s [%s] with openCount %d",
				attr.Path, attr.Name, openCount)
			continue
		}

		log.Info("GC::scheduleDeleteForStaleFiles: Deleting stale fileID %s (deleted %s back)",
			attr.Name, time.Since(attr.Mtime))

		// Get the metadata of the deleted file and schedule the delete.
		dcFile, err := getDeletedFile(attr.Name)
		if err != nil {
			if err == syscall.ENOENT {
				//
				// Some other node delete the file after we enumerated deleted files
				// and before we could fetch its metadata.
				//
				log.Warn("GC::scheduleDeleteForStaleFiles: Failed to get deleted file %s: %v", err)
			} else {
				log.Err("GC::scheduleDeleteForStaleFiles: Failed to get deleted file %s: %v", err)
			}

			continue
		}

		// The node belonging to the lexicographical lowest indexed online RV of the first valid MV in the MV list
		// will delete the stale file.
		if len(dcFile.FileLayout.MVList) == 0 {
			log.Err("GC::scheduleDeleteForStaleFiles: No MVs present for the file: %s", dcFile.Filename)
			common.Assert(false, dcFile.FileLayout.MVList, *dcFile)
			continue
		}

		var scheduleDelete bool

		for _, mv := range dcFile.FileLayout.MVList {
			rvs := cm.GetRVs(mv)
			if len(rvs) == 0 {
				log.Err("GC::scheduleDeleteForStaleFiles: No RVs present for the MV: %v, file: %s",
					mv, dcFile.Filename)
				common.Assert(false, mv, *dcFile)
				continue
			}

			rv := getLowestIndexRv(rvs)

			if cm.IsMyRV(rv) {
				// If the node is part of the lowest indexed RV, then it will delete the file.
				log.Info("GC::scheduleDeleteForStaleFiles: Lowest index RV %s is hosted in my node for MV %s, scheduling file %s for deletion",
					rv, mv, dcFile.Filename)
				scheduleDelete = true
				break
			} else {
				// If the node is not part of the lowest indexed RV, then it will not delete the file.
				log.Info("GC::scheduleDeleteForStaleFiles: Lowest index RV %s is not hosted in my node, for MV %s, skipping file %s",
					rv, mv, dcFile.Filename)
				continue
			}
		}

		if !scheduleDelete {
			log.Debug("GC::scheduleDeleteForStaleFiles: Not scheduling file %s for deletion as it is not hosted in my node",
				dcFile.Filename)
			// If the node is not part of the lowest indexed RV, then it will not delete the file.
			// We can skip scheduling the file for deletion as it will be deleted by the node that hosts the lowest
			// indexed RV.
			continue
		}

		log.Info("GC::scheduleDeleteForStaleFiles: Scheduling file %s for deletion", dcFile.Filename)
		// Schedule the file for GC.
		gcFile := &gcFile{
			file:         dcFile,
			retryCnt:     0,
			removeMVList: dcFile.FileLayout.MVList,
		}

		gc.deletedFileQueue <- gcFile
	}
}

// getLowestIndexRv returns the lexicographically lowest indexed online RV from the given map of RVs.
func getLowestIndexRv(rvs map[string]dcache.StateEnum) string {
	var lowestIndexRV string = "rv999999999" // Initialize with a high value RV.

	for rv, state := range rvs {
		if rv < lowestIndexRV && state == dcache.StateOnline {
			lowestIndexRV = rv
		}
	}

	return lowestIndexRV
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
