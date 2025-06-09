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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

type GcInfo struct {
	// Deletes this many number of files async at any time. Excess files get blocked.
	numGcWorkers     int
	deletedFileQueue chan *dcache.FileMetadata
	wg               sync.WaitGroup
}

var gc *GcInfo

func NewGC() {
	gc = &GcInfo{
		numGcWorkers:     100, // TODO: This number should be decided.
		deletedFileQueue: make(chan *dcache.FileMetadata, 100),
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

	for deletedDcFile := range gc.deletedFileQueue {
		gc.removeAllChunksForFile(deletedDcFile)
	}
}

func (gc *GcInfo) removeAllChunksForFile(file *dcache.FileMetadata) {
	log.Debug("GC::removeAllChunksForFile: file: %s", file.Filename)
	common.Assert(file.Size > 0, file)

	mvs := file.FileLayout.MVList
	numMvs := int64(len(mvs))
	retryCnt := 0
	numChunks := getNumChunksForFile(file)

	for i, mv := range mvs {
	retry:

		// The following map would be used while resuming the deletion of the chunks for an mv, while retrying for its
		// clustermap update. RvName->offsetInMB of chunk.
		deleteProgressForRvs := make(map[string]int64)

		mvState, rvs, lastClusterMapEpoch := getComponentRVsForMV(mv)
		log.Debug("GC::removeAllChunksForFile: retry cnt: %d, mv: %s, state: %s, file: %s", retryCnt, mv, mvState, file.Filename)

		shiftOnlineRVsToStart(rvs)

		//
		// Initially delete all the chunks from the all the online RV's first then move on to the syncing rv's.
		// If we delete the chunk from all the online rvs then this chunk may get listed in the sync job when syncing
		// the online rv to other outofsync rvs. So It is necessary to also make a removeChunk rpc request to all the
		// other rvs which are having the state syncing.
		//

		for _, rv := range rvs {
			if rv.State == string(dcache.StateOffline) || rv.State == string(dcache.StateOutOfSync) {
				log.Info("GC::removeAllChunksForFile: skip deleting the chunks from rv: %s, rv state: %s, file: %s",
					rv, rv.State, file.Filename)
				continue
			}

			if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
				// mv should not be offline.
				common.Assert(mvState != dcache.StateOffline)

				// Remove all the chunks which were present in this RV..
				rvId := getRvIDFromRvName(rv.Name)
				targetNodeId := getNodeIDFromRVName(rv.Name)
				chunkIdx := int64(i)

				// Resume the progress of deletion of the chunks if it's a retry.
				if resumeChunkIdx, ok := deleteProgressForRvs[rv.Name]; ok {
					common.Assert(resumeChunkIdx >= chunkIdx && retryCnt > 0,
						retryCnt, chunkIdx, resumeChunkIdx, file.Filename, rv, rvs)
					chunkIdx = max(chunkIdx, resumeChunkIdx)
				}

				// TODO: remove all the chunks corresponding to an RV in one RPC call.
				for ; chunkIdx < numChunks; chunkIdx = getNextChunkIdxInMV(chunkIdx, numMvs) {
					log.Debug("GC::removeAllChunksForFile: removing chunkIdx: %d, file: %s", chunkIdx, file.Filename)

					rpcReq := &models.RemoveChunkRequest{
						Address: &models.Address{
							FileID:      file.FileID,
							RvID:        rvId,
							MvName:      mv,
							OffsetInMiB: chunkIdx * file.FileLayout.ChunkSize / common.MbToBytes,
						},
						ComponentRV: rvs,
					}

					ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
					defer cancel()

					rpcResp, err := rpc_client.RemoveChunk(ctx, targetNodeId, rpcReq)
					if err != nil {
						rpcErr := rpc.GetRPCResponseError(err)
						log.Err("GC::removeAllChunksForFile: Failed to delete the chunk idx: %d, file: %s, rv: %s: %v",
							chunkIdx, file.Filename, rv.Name, rpcErr)

						if rpcErr == nil {
							// We should now run the inband RV offline detection workflow, basically we
							// call the clustermap's UpdateComponentRVState() API to mark this
							// component RV as offline and force the fix-mv workflow which will eventually
							// trigger the resync-mv workflow.
							//
							log.Err("GC::removeAllChunksForFile: Delete chunk %s/%s, failed to reach node %s [%v]",
								chunkIdx, file.Filename, targetNodeId, err)

							errRV := cm.UpdateComponentRVState(mv, rv.Name, dcache.StateOffline)
							if errRV != nil {
								//
								// If we fail to update the component RV as offline, we cannot safely complete
								// the chunk write or else the failed replica may not be resynced causing data
								// consistency issues.
								//
								errStr := fmt.Sprintf("failed to update %s/%s state to offline [%v] file: %s",
									rv.Name, mv, errRV, file.Filename)
								log.Err("GC::removeAllChunksForFile: %s", errStr)
								common.Assert(false, errStr)
								// As the deletion is asynchrnous, continue deleting the chunks from the other RVs.
								// There is no need for deletion of the other chunks in this RV, as the RV went offline.
								break
							}

							//
							// If UpdateComponentRVState() succeeds, marking this component RV as offline,
							// we can safely carry on with the write since we are guaranteed that these
							// chunks which we could not write to this component RV will be later sync'ed
							// from one of the good component RVs.
							//
							log.Warn("GC::removeAllChunksForFile: Deletion of chunk: %d to %s/%s on node %s failed, "+
								"marked RV offline, file: %s", chunkIdx, rv.Name, mv, targetNodeId)
							// There is no need for deletion of the other chunks in this RV, as the RV went offline.
							break
						}

						if rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
							log.Info("GC::removeAllChunksForFile: Need to refresh the cluster map, file: %s, rv: %s, err: %v",
								file.Filename, rv.Name, rpcErr)

							//
							// We allow 5 refreshes of the clustermap for resiliency, before we fail the delete of a file.
							// This is to allow multiple changes to the MV during the course of a deleting.
							// It's unlikely but we need to be resilient.
							//
							if retryCnt > 5 {
								log.Err("GC::removeAllChunksForFile: Max retries for updating the clusermap exhausted, "+
									"rv: %v, file: %s: %v", rv, file.Filename, rpcErr)
								common.Assert(false, file, rpcErr)
								return
							}

							//
							// Retry till the next epoch, ensuring that the clustermap is refreshed from what we
							// have cached right now.
							//
							errCM := cm.RefreshClusterMap(lastClusterMapEpoch)
							if errCM != nil {
								log.Err("GC::removeAllChunksForFile: Failed to refresh the cluster map, rv: %s, file: %s: %v",
									rv.Name, file.Filename, errCM)
								common.Assert(false, file, errCM)
								return
							}

							retryCnt++
							goto retry
						}

						common.Assert(rpcErr.GetCode() == models.ErrorCode_ChunkNotFound &&
							rv.State == string(dcache.StateSyncing), file.Filename, rv, rvs, rpcErr, rpcResp)
					}

					// Update the progress of deletion for the RV.
					deleteProgressForRvs[rv.Name] = getNextChunkIdxInMV(chunkIdx, numMvs)
				}
			}
		}
	}

	// After removing all the chunks from all the rvs, we can remove the file layout.
	deletedFile := dcache.GetDeletedFileName(file.Filename, file.FileID)
	log.Debug("GC::removeAllChunksForFile: removing file layout for file: %s", deletedFile)

	err := metadata_manager.DeleteFile(deletedFile)
	if err != nil {
		log.Err("GC::removeAllChunksForFile: failed to remove file layout for file: %s: %v", deletedFile, err)
		common.Assert(false, file, err)
		return
	}

}

func AsyncFileChunkGarbageCollector(file *dcache.FileMetadata) {
	if file.Size != 0 {
		gc.deletedFileQueue <- file
	}
}
