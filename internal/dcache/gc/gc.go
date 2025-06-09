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
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
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
	numChunks := getNumChunksForFile(file)

	for i, mv := range mvs {
		mvState, rvs, _ := getComponentRVsForMV(mv)
		log.Debug("GC::removeAllChunksForFile: mv: %s, state: %s, file: %s", mv, mvState, file.Filename)

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
				// TODO: is it ok to skip the offline and out of sync rvs? what if there state changes to syncing
				// in between. is it necessary to update the clustermap in this case?
			}

			if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
				// mv should not be offline.
				common.Assert(mvState != dcache.StateOffline)

				// Remove all the chunks which were present in this RV..
				rvId := getRvIDFromRvName(rv.Name)
				targetNodeId := getNodeIDFromRVName(rv.Name)

				for chunkIdx := int64(i); chunkIdx < numChunks; chunkIdx += int64(numMvs) {
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
							//
							// This error indicates some transport error, i.e., RPC request couldn't make it to the
							// server and hence didn't solicit a response. It could be some n/w issue, blobfuse
							// process down or node down.
							//
							// TODO: handle this case.
							continue
						}

						common.Assert(rpcErr.GetCode() == models.ErrorCode_ChunkNotFound &&
							rv.State == string(dcache.StateSyncing), file.Filename, rv, rvs, rpcResp)

					}
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
