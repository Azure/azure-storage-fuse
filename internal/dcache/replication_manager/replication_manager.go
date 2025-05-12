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

package replication_manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

func ReadMV(req *ReadMvRequest) (*ReadMvResponse, error) {
	common.Assert(req != nil)

	log.Debug("ReplicationManager::ReadMV: Received ReadMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		err = fmt.Errorf("Invalid ReadMV request parameters [%v]", err)
		log.Err("ReplicationManager::ReadMV: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	var rpcResp *models.GetChunkResponse
	var err error

	clusterMapRefreshed := false

retry:
	// Get component RVs for MV, from clustermap.
	componentRVs := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::ReadMV: Component RVs for %s are: %v",
		req.MvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Get the most suitable RV from the list of component RVs,
	// from which we should read the chunk. Selecting most
	// suitable RV is mostly a heuristical process which might
	// pick the most suitable RV based on one or more of the
	// following criteria:
	// - Local RV must be preferred.
	// - Prefer a node that has recently responded successfully to any of our RPCs.
	// - Pick a random one.
	//
	// excludeRVs is the list of component RVs to omit, used when retrying after prev attempts to read from
	// certain RV(s) failed. Those RVs are added to excludeRVs list.
	//
	var excludeRVs []string
	for {
		readerRV := getReaderRV(componentRVs, excludeRVs)
		if readerRV == nil {
			//
			// Even after refreshing clustermap if we cannot get a valid MV replica to read from,
			// alas we need to fail the read.
			//
			if clusterMapRefreshed {
				err = fmt.Errorf("No suitable RV found for MV %s", req.MvName)
				log.Err("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

			// This is very unlikely and it would most likely indicate that we have a “very stale”
			// clustermap where all/most of the component RVs have been replaced.

			// TODO: will be done later
			// err = cm.RefreshClusterMapSync()
			clusterMapRefreshed = true
			goto retry
		}

		common.Assert(!slices.Contains(excludeRVs, readerRV.Name), readerRV.Name, excludeRVs)

		selectedRvID := getRvIDFromRvName(readerRV.Name)
		common.Assert(common.IsValidUUID(selectedRvID))

		targetNodeID := getNodeIDFromRVName(readerRV.Name)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::ReadMV: Selected %s for %s RV id %s hosted by node %s",
			readerRV.Name, req.MvName, selectedRvID, targetNodeID)

		// TODO: optimization, should we send buffer also in the GetChunk request?
		rpcReq := &models.GetChunkRequest{
			Address: &models.Address{
				FileID:      req.FileID,
				RvID:        selectedRvID,
				MvName:      req.MvName,
				OffsetInMiB: req.ChunkIndex * req.ChunkSizeInMiB,
			},
			OffsetInChunk: req.OffsetInChunk,
			Length:        req.Length,
			ComponentRV:   componentRVs,
		}

		// TODO: how to handle timeouts in case when node is unreachable
		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		rpcResp, err = rpc_client.GetChunk(ctx, targetNodeID, rpcReq)

		// Exclude this RV from further iterations (if any).
		excludeRVs = append(excludeRVs, readerRV.Name)

		if err == nil {
			// Success.
			common.Assert((rpcResp != nil &&
				rpcResp.Chunk != nil &&
				rpcResp.Chunk.Address != nil),
				rpc.GetChunkRequestToString(rpcReq))
			// TODO: Validate other rpcResp fields.
			break
		}

		// TODO: we should handle errors that indicate retrying from a different RV would help.
		// RVs are the final source of truth wrt MV membership (and anything else),
		// so if the target RV feels that the sender seems to have out-of-date clustermap,
		// it can help him by failing the request with an appropriate error and then
		// caller should fetch the latest clustermap and then try again.

		log.Err("ReplicationManager::ReadMV: Failed to get chunk from node %s for request %v [%v]",
			targetNodeID, rpc.GetChunkRequestToString(rpcReq), err)
	}

	log.Debug("ReplicationManager::ReadMV: GetChunk RPC response: %v", rpc.GetChunkResponseToString(rpcResp))

	n := copy(req.Data, rpcResp.Chunk.Data)
	// req.Data must be large enough to copy entire rpcResp.Chunk.Data.
	common.Assert(n == len(rpcResp.Chunk.Data), n, len(rpcResp.Chunk.Data))

	// TODO: in GetChunk RPC request add data buffer to the request
	// TODO: in GetChunk RPC response return bytes read

	// TODO: hash validation will be done later
	// TODO: should we validate the hash of the chunk here?
	// hash := getMD5Sum(rpcResp.Chunk.Data)
	// if hash != rpcResp.Chunk.Hash {
	//      log.Err("ReplicationManager::ReadMV: Hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr)
	//      common.Assert(false, fmt.Sprintf("hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr))
	//      return nil, fmt.Errorf("hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr)
	// }

	resp := &ReadMvResponse{
		// TODO: update this field after bytes read in response.
		BytesRead: int64(len(rpcResp.Chunk.Data)),
	}

	return resp, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	common.Assert(req != nil)

	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		err = fmt.Errorf("Invalid WriteMV request parameters [%v]", err)
		log.Err("ReplicationManager::WriteMV: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	clusterMapRefreshed := 0

	// TODO: TODO: hash validation will be done later
	// get hash of the data in the request
	// hash := getMD5Sum(req.Data)

retry:
	// Get component RVs for MV, from clustermap.
	componentRVs := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::WriteMV: Component RVs for %s are: %v",
		req.MvName, rpc.ComponentRVsToString(componentRVs))

	// TODO: put chunk to each component RV should be done in parallel
	for _, rv := range componentRVs {
		//
		// Omit writing to RVs in “offline” or “outofsync” state. It’s ok to omit them as the chunks not
		// written to them will be copied to them when the mv is (soon) resynced.
		// Otoh if an RV is in “syncing” state then any new chunk written to it may not be copied by the
		// ongoing resync operation as the source RV may have been already gone past the enumeration stage
		// and hence won’t consider this chunk for resync, and hence those MUST have the chunks mandatorily
		// copied to them.
		//

		if rv.State == string(dcache.StateOffline) || rv.State == string(dcache.StateOutOfSync) {
			log.Debug("ReplicationManager::WriteMV: Skipping RV %s (state %s) for %s",
				rv.Name, rv.State, req.MvName)
			continue
		} else if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
			rvID := getRvIDFromRvName(rv.Name)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rv.Name)
			common.Assert(common.IsValidUUID(targetNodeID))

			log.Debug("ReplicationManager::WriteMV: %s writing to %s RV id %s hosted by node %s",
				req.MvName, rv.Name, rvID, targetNodeID)

			rpcReq := &models.PutChunkRequest{
				Chunk: &models.Chunk{
					Address: &models.Address{
						FileID:      req.FileID,
						RvID:        rvID,
						MvName:      req.MvName,
						OffsetInMiB: req.ChunkIndex * req.ChunkSizeInMiB,
					},
					Data: req.Data,
					Hash: "", // TODO: hash validation will be done later
				},
				Length:      int64(len(req.Data)),
				IsSync:      false, // this is regular client write
				ComponentRV: componentRVs,
			}

			// TODO: how to handle timeouts in case when node is unreachable
			ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
			defer cancel()

			rpcResp, err := rpc_client.PutChunk(ctx, targetNodeID, rpcReq)
			if err != nil {
				log.Err("ReplicationManager::WriteMV: PutChunk to node %s failed [%v]",
					targetNodeID, err)
				rpcErr := rpc.GetRPCResponseError(err)
				if rpcErr == nil {
					//
					// This error means that the node is not reachable.
					// TODO:
					// We should now run the inband RV offline detection workflow, basically we
					// call the clustermap's updateComponentRVState() API to mark this
					// component RV as offline and force the fix-mv workflow which will finally
					// trigger the resync-mv workflow.
					//
					log.Err("ReplicationManager::WriteMV: Failed to reach node %s [%v]",
						targetNodeID, err)
					return nil, err
				}

				// The error is RPC error of type *rpc.ResponseError.
				if rpcErr.Code() == rpc.NeedToRefreshClusterMap {
					// TODO: Should we allow more than one clustermap refresh?
					if clusterMapRefreshed > 0 {
						log.Err("ReplicationManager::WriteMV: Failed after refreshing clustermap")
						return nil, err
					}

					// TODO: will be done later
					// err = cm.RefreshClusterMapSync()
					clusterMapRefreshed++
					goto retry
				} else {
					// TODO: check if this is non-retriable error.
					log.Err("ReplicationManager::WriteMV: Got non-retriable error for put chunk to node %s [%v]",
						targetNodeID, err)
					return nil, err
				}
			}

			common.Assert(rpcResp != nil, "PutChunk RPC response is nil")
			log.Debug("ReplicationManager::WriteMV: PutChunk successful RPC response: %v", rpc.PutChunkResponseToString(rpcResp))
		} else {
			common.Assert(false, "Unexpected RV state", rv.State, rv.Name)
		}
	}

	return &WriteMvResponse{}, nil
}

// TODO: this will be triggered after the replication manager gets the event of the cluster map update
// ResyncDegradedMVs will be triggered if there is any change in the clustermap.
// Cluster manager's DegradeMV and FixMV workflow will update the clustermap replacing the offline RVs
// with new online RVs and also marking the MV as degraded. It then publishes the updated clustermap
// which will be picked up by the replication manager.
func ResyncDegradedMVs() error {
	degradedMVs := clustermap.GetDegradedMVs()
	if len(degradedMVs) == 0 {
		log.Debug("ReplicationManager::ResyncDegradedMVs: No degraded MVs found")
		return nil
	}

	log.Debug("ReplicationManager::ResyncDegradedMVs: Degraded MVs found: %+v", degradedMVs)

	// TODO: make this parallel
	for mvName, degradedMV := range degradedMVs {
		common.Assert(degradedMV.State == dcache.StateDegraded, fmt.Sprintf("MV %s is not in degraded state", mvName))
		err := ResyncMV(mvName, degradedMV)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::ResyncDegradedMVs: Failed to resync MV %s [%v]", mvName, err.Error())
		}
	}

	return nil
}

// ResyncMV is used for resyncing the degraded MV to online state.
// It first finds the lowest index online RV for the given MV and if this RV is not hosted
// in the current node, it returns and skips the resync operation.
// The node hosting the lowest index online RV will be responsible for resyncing the MV.
// It then calls StartSync() RPC call to all the component RVs. After this, it copies the chunks
// from the lowest index online RV to the out of sync RVs.
// Finally, it calls EndSync() RPC call to all the component RVs.
func ResyncMV(mvName string, mvInfo dcache.MirroredVolume) error {
	log.Debug("ReplicationManager::ResyncMV: Resyncing MV %s : %+v", mvName, mvInfo)

	lowestIdxRVName := getLowestIndexOnlineRV(mvInfo.RVs)
	if lowestIdxRVName == "" {
		log.Err("ReplicationManager::ResyncMV: No online RVs found for MV %s", mvName)
		return fmt.Errorf("no online RVs found for MV %s", mvName)
	}

	log.Debug("ReplicationManager::ResyncMV: Lowest index online RV for MV %s is %s", mvName, lowestIdxRVName)
	if !isRVHostedInCurrentNode(lowestIdxRVName) {
		log.Debug("ReplicationManager::ResyncMV: Lowest index online RV %s is not hosted in current node", lowestIdxRVName)
		return nil
	}

	componentRVs := convertRVMapToList(mvName, mvInfo.RVs)
	log.Debug("ReplicationManager::ResyncMV: Component RVs for the given MV %s are: %v", mvName, rpc.ComponentRVsToString(componentRVs))

	// TODO: check if this is correctly returning the sync size of MV
	syncSize, err := rpc_server.GetDiskUsageOfMV(mvName, lowestIdxRVName)
	if err != nil {
		log.Err("ReplicationManager::ResyncMV: Failed to get disk usage of MV %s for RV %s [%v]", mvName, lowestIdxRVName, err.Error())
		return fmt.Errorf("failed to get disk usage of MV %s for RV %s [%v]", mvName, lowestIdxRVName, err.Error())
	}

	// call StartSync() RPC call to all the component RVs
	outOfSyncRVs, syncIDMap, err := startSyncForRVs(mvName, lowestIdxRVName, syncSize, componentRVs)
	if err != nil {
		log.Err("ReplicationManager::ResyncMV: Failed to start sync for MV %s [%v]", mvName, err.Error())
		return fmt.Errorf("failed to start sync for MV %s [%v]", mvName, err.Error())
	}

	log.Debug("ReplicationManager::ResyncMV: Out of sync RVs for MV %s are: %v", mvName, outOfSyncRVs)

	// TODO:: integration: call cluster manager API to update the outofsync RVs to syncing state
	// and also mark the MV as syncing

	// copy chunks for out of sync RVs
	// TODO: make this parallel
	for _, rv := range outOfSyncRVs {
		err = copyOutOfSyncChunks(mvName, lowestIdxRVName, rv, componentRVs)
		if err != nil {
			log.Err("ReplicationManager::ResyncMV: Failed to copy out of sync chunks for MV %s, RV %s [%v]", mvName, rv, err.Error())
			return fmt.Errorf("failed to copy out of sync chunks for MV %s, RV %s [%v]", mvName, rv, err.Error())
		}
	}

	// call EndSync() RPC call to all the component RVs
	err = endSyncForRVs(mvName, lowestIdxRVName, syncSize, syncIDMap, componentRVs)
	if err != nil {
		log.Err("ReplicationManager::ResyncMV: Failed to end sync for MV %s [%v]", mvName, err.Error())
		return fmt.Errorf("failed to end sync for MV %s [%v]", mvName, err.Error())
	}

	// TODO:: integration: call cluster manager API to update the syncing RVs to online state
	// and also mark the MV as online if all the RVs are online

	log.Debug("ReplicationManager::ResyncMV: Successfully resynced MV %s", mvName)

	return nil
}

// startSyncForRVs will call StartSync() RPC call to all the component RVs.
// It also returns the out of sync RVs and the sync IDs for each RV
func startSyncForRVs(mvName string, lowestIdxRVName string, syncSize int64, componentRVs []*models.RVNameAndState) ([]string, map[string]string, error) {
	log.Debug("ReplicationManager::startSyncForRVs: Starting sync for MV %s, lowest index RV %s, sync size %d, component RVs %v",
		mvName, lowestIdxRVName, syncSize, rpc.ComponentRVsToString(componentRVs))

	outOfSyncRVs := make([]string, 0)
	syncIDMap := make(map[string]string)

	// TODO: make this parallel
	for _, rv := range componentRVs {
		targetNodeID := getNodeIDFromRVName(rv.Name)
		common.Assert(common.IsValidUUID(targetNodeID))

		// send StartSync() RPC call
		startSyncReq := &models.StartSyncRequest{
			MV:           mvName,
			SourceRVName: lowestIdxRVName,
			TargetRVName: rv.Name,
			ComponentRV:  componentRVs,
			SyncSize:     syncSize,
		}

		log.Debug("ReplicationManager::startSyncForRVs: Sending StartSync RPC call to MV %s, RV %s, hosted in node %s : %v",
			mvName, rv.Name, targetNodeID, rpc.StartSyncRequestToString(startSyncReq))

		// TODO: how to handle timeouts in case when node is unreachable
		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		startSyncResp, err := rpc_client.StartSync(ctx, targetNodeID, startSyncReq)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::startSyncForRVs: Failed to start sync for MV %s, RV %s [%v] : %v",
				mvName, rv.Name, err.Error(), rpc.StartSyncRequestToString(startSyncReq))
			continue
		}

		common.Assert(startSyncResp != nil, "StartSync RPC response is nil")
		common.Assert(common.IsValidUUID(startSyncResp.SyncID), "Sync ID in StartSync RPC response is empty")
		log.Debug("ReplicationManager::startSyncForRVs: StartSync RPC response for MV %s and RV %s : %+v",
			mvName, rv.Name, *startSyncResp)

		syncIDMap[rv.Name] = startSyncResp.SyncID

		if rv.State == string(dcache.StateOutOfSync) {
			outOfSyncRVs = append(outOfSyncRVs, rv.Name)
		}
	}

	return outOfSyncRVs, syncIDMap, nil
}

// copyOutOfSyncChunks will copy the chunks from the source RV to the destination RV
func copyOutOfSyncChunks(mvName string, srcRVName string, destRVName string, componentRVs []*models.RVNameAndState) error {
	log.Debug("ReplicationManager::copyChunksForRVs: Copying chunks for MV %s, source RV %s, destination RV %s, component RVs %v",
		mvName, srcRVName, destRVName, rpc.ComponentRVsToString(componentRVs))

	common.Assert(srcRVName != destRVName, "source and destination RV names are same")

	sourceMVPath := filepath.Join(getCachePathForRVName(srcRVName), mvName)
	common.Assert(common.DirectoryExists(sourceMVPath), fmt.Sprintf("source MV path %s does not exist in current node", sourceMVPath))

	destRvID := getRvIDFromRvName(destRVName)
	common.Assert(common.IsValidUUID(destRvID))

	// enumerate the chunks in the source MV path
	entries, err := os.ReadDir(sourceMVPath)
	if err != nil {
		log.Err("ReplicationManager::copyChunksForRVs: Failed to read directory %s [%v]", sourceMVPath, err.Error())
		return err
	}

	// TODO: make this parallel
	for _, entry := range entries {
		if entry.IsDir() {
			log.Warn("ReplicationManager::copyChunksForRVs: Skipping directory %s", entry.Name())
			continue
		}

		// chunks are stored in mv as,
		// <MvName>/<FileID>.<OffsetInMiB>.data and
		// <MvName>/<FileID>.<OffsetInMiB>.hash
		chunkParts := strings.Split(entry.Name(), ".")
		if len(chunkParts) != 3 {
			log.Err("ReplicationManager::copyChunksForRVs: Chunk name %s is not in the expected format", entry.Name())
			common.Assert(false, fmt.Sprintf("chunk name %s is not in the expected format", entry.Name()))
			continue
		}

		// TODO: hash validation will be done later
		// if file type is hash, skip it
		// the hash data will be transferred with the regular chunk file
		if chunkParts[2] == "hash" {
			log.Debug("ReplicationManager::copyChunksForRVs: Skipping hash file %s", entry.Name())
			continue
		}

		fileID := chunkParts[0]
		common.Assert(common.IsValidUUID(fileID))

		// convert string to int64
		offsetInMiB, err := strconv.ParseInt(chunkParts[1], 10, 64)
		if err != nil {
			log.Err("ReplicationManager::copyChunksForRVs: Failed to convert offset %s to int64 [%v]", chunkParts[1], err.Error())
			common.Assert(false, fmt.Sprintf("failed to convert offset %s to int64 [%v]", chunkParts[1], err.Error()))
		}

		srcChunkPath := filepath.Join(sourceMVPath, entry.Name())
		srcData, err := os.ReadFile(srcChunkPath)
		if err != nil {
			log.Err("ReplicationManager::copyChunksForRVs: Failed to read file %s [%v]", srcChunkPath, err.Error())
			return err
		}

		putChunkReq := &models.PutChunkRequest{
			Chunk: &models.Chunk{
				Address: &models.Address{
					FileID:      fileID,
					RvID:        destRvID,
					MvName:      mvName,
					OffsetInMiB: offsetInMiB,
				},
				Data: srcData,
				Hash: "", // TODO: hash validation will be done later
			},
			Length:      int64(len(srcData)),
			IsSync:      true, // this is sync write
			ComponentRV: componentRVs,
		}

		log.Debug("ReplicationManager::copyChunksForRVs: Copying chunk %s to RV %s : %v",
			srcChunkPath, destRVName, rpc.PutChunkRequestToString(putChunkReq))

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		putChunkResp, err := rpc_client.PutChunk(ctx, destRVName, putChunkReq)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::copyChunksForRVs: Failed to put chunk to RV %s [%v] : %v",
				destRVName, err.Error(), rpc.PutChunkRequestToString(putChunkReq))
			return err
		}

		common.Assert(putChunkResp != nil, "PutChunk RPC response is nil")
		log.Debug("ReplicationManager::copyChunksForRVs: PutChunk RPC response for chunk %s to RV %s : %v",
			srcChunkPath, destRVName, rpc.PutChunkResponseToString(putChunkResp))
	}

	return nil
}

// endSyncForRVs will call EndSync() RPC call to all the component RVs
func endSyncForRVs(mvName string, lowestIdxRVName string, syncSize int64, syncIDMap map[string]string, componentRVs []*models.RVNameAndState) error {
	log.Debug("ReplicationManager::endSyncForRVs: End sync for MV %s, lowest index RV %s, sync size %d, sync id map %v, component RVs %v",
		mvName, lowestIdxRVName, syncSize, syncIDMap, rpc.ComponentRVsToString(componentRVs))

	// TODO: make this parallel
	for _, rv := range componentRVs {
		targetNodeID := getNodeIDFromRVName(rv.Name)
		common.Assert(common.IsValidUUID(targetNodeID))

		syncID, ok := syncIDMap[rv.Name]
		if !ok {
			log.Err("ReplicationManager::endSyncForRVs: Sync ID not found for RV %s", rv.Name)
			common.Assert(false, fmt.Sprintf("sync ID not found for RV %s", rv.Name))
			continue
		}

		common.Assert(common.IsValidUUID(syncID), fmt.Sprintf("sync ID %s is not valid", syncID))

		// send EndSync() RPC call
		endSyncReq := &models.EndSyncRequest{
			SyncID:       syncID,
			MV:           mvName,
			SourceRVName: lowestIdxRVName,
			TargetRVName: rv.Name,
			ComponentRV:  componentRVs,
			SyncSize:     syncSize,
		}

		log.Debug("ReplicationManager::endSyncForRVs: Sending EndSync RPC call to MV %s, RV %s, hosted in node %s : %v",
			mvName, rv.Name, targetNodeID, rpc.EndSyncRequestToString(endSyncReq))

		// TODO: how to handle timeouts in case when node is unreachable
		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		endSyncResp, err := rpc_client.EndSync(ctx, targetNodeID, endSyncReq)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::endSyncForRVs: Failed to end sync for MV %s, RV %s [%v] : %v",
				mvName, rv.Name, err.Error(), rpc.EndSyncRequestToString(endSyncReq))
			continue
		}

		common.Assert(endSyncReq != nil, "EndSync RPC response is nil")
		log.Debug("ReplicationManager::endSyncForRVs: EndSync RPC response for MV %s and RV %s : %+v",
			mvName, rv.Name, *endSyncResp)
	}

	return nil
}
