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

//go:generate $ASSERT_REMOVER $GOFILE

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
	deleteTimeout time.Duration
	//
	// If file OpenCount doesn't change for so long we consider it as hung (the node that opened the file
	// crashed) and force delete the file chunks.
	//
	openCountHungTimeout time.Duration
	//
	// Time to trigger the Periodic GC go routine which will reclaim the chunks for the stale files.
	//
	interval time.Duration
	//
	// Map of all stale files which are currently schedule for deletion.
	// A file deletion can take very long (especially with openCount not 0) so we don't want to
	// requeue the same file again.
	//
	staleFilesScheduled sync.Map
}

var gc *GcInfo

type gcFile struct {
	file *dcache.FileMetadata
	//
	// file.OpenCount is the last open count.
	// This is the time when it was seen to be changed.
	// If openCount is stuck for more than gc.openCountHungTimeout we force delete the file chunks.
	//
	lastOpenCountChangedAt time.Time
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
		// serially to each MV and parallelly for each component RV for the MV. The time taken will
		// depend on the size of the file as bigger files will have more chunks to be deleted.
		//
		// We set deleteTimeout to a reasonable value of 5 minutes.
		//
		deleteTimeout: 5 * time.Minute,
		//
		// We are not too eager in deleting openCount hung files, to avoid deleting legitimately
		// open files. We don't know the usecases yet, hence play safe.
		//
		// Note: If the leader node changes, the hung counter will be reset, so for this to work
		//       the same node must be leader for at least openCountHungTimeout period.
		//
		openCountHungTimeout: 1 * time.Hour,
		interval:             10 * time.Minute,
	}

	// Start Periodic GC to reclaim the chunks for the Stale files.
	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		pollTicker := time.NewTicker(gc.interval)

		for {
			select {
			case <-gc.done:
				log.Info("GC:: Stopping Periodic GC go routine")
				return
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
		defer gc.wg.Done()

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
			if dcFile.OpenCount != gcFile.file.OpenCount {
				// For deleted files, opencount can only go down.
				common.Assert(dcFile.OpenCount < gcFile.file.OpenCount,
					dcFile.OpenCount, gcFile.file.OpenCount,
					gcFile.file.Filename, gcFile.file.FileID)
				gcFile.lastOpenCountChangedAt = time.Now()
			}
			gcFile.file = dcFile
		}

		//
		// OpenCount still not 0, can't delete the file, skip till next iteration.
		// If file openCount is stuck for a long time, we force delete the file chunks.
		//
		if gcFile.file.OpenCount > 0 {
			common.Assert(!gcFile.lastOpenCountChangedAt.IsZero(),
				gcFile.file.OpenCount, gcFile.file.Filename, gcFile.file.FileID)

			openCountStuckFor := time.Since(gcFile.lastOpenCountChangedAt)
			if openCountStuckFor < gc.openCountHungTimeout {
				gc.wg.Add(1)
				go rescheduleFile()
				return
			} else {
				log.Warn("GC::removeAllChunksForFile: Opencount for file: %s [%s] stuck at %d for %s, force deleting chunks",
					gcFile.file.Filename, gcFile.file.FileID,
					gcFile.file.OpenCount, openCountStuckFor)
			}
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
		log.Err("GC::removeAllChunksForFile: Failed to refresh the cluster map, file: %s, file ID: %s: %v",
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
				log.Warn("GC::removeAllChunksForFile: Could not delete all chunks from MV: %s, file: %s (%s): %v",
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

		gc.wg.Add(1)
		go rescheduleFile()

		return
	}

deleteMetadataFile:
	//
	// If this file was scheduled by the stale file deletion goroutine, then we need to
	// remove the file from gc.staleFilesScheduled. If not, this will be a no-op.
	//
	defer gc.staleFilesScheduled.Delete(gcFile.file.FileID)

	// After removing all the chunks from all the rvs, we can remove the metadata file.
	log.Debug("GC::removeAllChunksForFile: removing metadata file for %s [%s]",
		gcFile.file.Filename, gcFile.file.FileID)

	err := mm.DeleteFile(gcFile.file.FileID)
	if err != nil {
		// Periodic GC should delete this metadata file.
		log.Err("GC::removeAllChunksForFile: failed to remove metadata file for %s [%s]: %v",
			gcFile.file.Filename, gcFile.file.FileID, err)
		common.Assert(false, gcFile.file, err)
		return
	}
}

func ScheduleChunkDeletion(file *dcache.FileMetadata) {
	gcFile := &gcFile{
		file:                   file,
		lastOpenCountChangedAt: time.Now(),
		retryCnt:               0,
		removeMVList:           file.FileLayout.MVList,
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
// a file and before closing it, the file openCount will be stuck at non-zero and such files can never be
// technically deleted. We define openCountHungTimeout and delete files which are stuck with unchanging
// openCount for longer than that, indicating node that opened the file crashed.

func (gc *GcInfo) scheduleDeleteForStaleFiles() {
	log.Debug("GC::scheduleDeleteForStaleFiles: Started")

	//
	// Only the leader node runs the stale file deleting logic, to avoid too many calls for
	// listing deleted files and then all nodes attempting deletion.
	//
	if !isMyNodeLeaderToDeleteStaleFiles() {
		log.Debug("GC::scheduleDeleteForStaleFiles: Skipping as not leader")
		return
	}

	log.Info("GC::scheduleDeleteForStaleFiles: Leader for deleting stale files, starting deletion")

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

	//
	// Schedule delete for all files that could not be deleted for gc.deleteTimeout.
	// This would indicate some issue, most likely the node that deleted the file
	// and hence was responsible for deleting he file chunks, crashed.
	//
	for _, attr := range deletedFiles {
		//
		// If already scheduled in previous iterations, don't requeue.
		//
		val, ok := gc.staleFilesScheduled.Load(attr.Name)
		if ok {
			log.Info("GC::scheduleDeleteForStaleFiles: File %s (%s) already scheduled, skipping",
				attr.Name, val.(string))
			continue
		}

		// Extract the openCount from the attribute.
		openCount, err := getOpenCountForDeletedFile(attr)
		if err != nil {
			log.Info("GC::scheduleDeleteForStaleFiles: Failed to get opencount for file %s [%s]",
				attr.Path, attr.Name)
			common.Assert(false, *attr, err)
			continue
		}

		//
		// attr.Mtime will be the time when the file was deleted (and its metadata file added
		// to mdRoot/Deleted/ folder. We let the "owner node" (that ran DeleteDcacheFile())
		// delete the file and only after gc.deleteTimeout period we assume that the node
		// went down and hence the periodic deleter has the responsibility of deleting the file.
		//
		if time.Since(attr.Mtime) < gc.deleteTimeout {
			continue
		}

		// Get the metadata of the deleted file and schedule the delete.
		dcFile, err := getDeletedFile(attr.Name)
		if err != nil {
			if err == syscall.ENOENT {
				//
				// Some other node delete the file after we enumerated deleted files
				// and before we could fetch its metadata.
				//
				log.Warn("GC::scheduleDeleteForStaleFiles: getDeletedFile(%s) failed: %v", attr.Name, err)
			} else {
				log.Err("GC::scheduleDeleteForStaleFiles: getDeletedFile(%s) failed: %v", attr.Name, err)
			}

			continue
		}

		// Store it in the map to avoid requeue in the next iteration.
		gc.staleFilesScheduled.Store(dcFile.FileID, dcFile.Filename)

		//
		// Schedule deletion of file regardless of the opencount.
		// removeAllChunksForFile() will honor the openCount correctly, it also handles
		// files whose openCount is stuck.
		//
		log.Info("GC::scheduleDeleteForStaleFiles: Scheduling stale file %s (%s) for deletion (deleted %s back, openCount: %d)",
			dcFile.Filename, dcFile.FileID, time.Since(attr.Mtime), openCount)

		ScheduleChunkDeletion(dcFile)
	}
}

// Are we the leader node to delete the stale files?
// Currently Leader Node that is responsible for clustermap Update is also responsible for deletion of
// the stale files.
// TODO: See if we need to have a different leader for stale file deletion.

func isMyNodeLeaderToDeleteStaleFiles() bool {
	leaderNode := cm.GetClusterMap().LastUpdatedBy
	myNodeID, err := common.GetNodeUUID()
	if err != nil {
		log.Err("GC::isLeaderToDeleteStaleFiles: Failed to Get My NodeId [%v]", err)
		common.Assert(false, err)
		return false
	}

	log.Debug("GC::isLeaderToDeleteStaleFiles: myNodeID: %s, leaderNode: %s", myNodeID, leaderNode)

	return (leaderNode == myNodeID)
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
	fileMetadataBytes, fileSize, _, openCount, _, err := mm.GetFile(fileId, true /* isDeleted */)
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

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	var err error
	errors.Is(err, syscall.ENOENT)
}
