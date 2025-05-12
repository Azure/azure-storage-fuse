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
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
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

// TODO: this will be triggered after the replication manager gets the event of the cluster map update.
// Cluster manager's DegradeMV and FixMV workflow will update the clustermap replacing the offline RVs
// with new online RVs and also marking the MV as degraded. It then publishes the updated clustermap
// which will be picked up by the replication manager.
//
// This method runs at regular intervals in a separate goroutine and periodically resyncs degraded MVs.
func periodicResyncMVs() {
	ticker := time.NewTicker(ResyncInterval * time.Second)

	for range ticker.C {
		log.Debug("ReplicationManager::periodicResyncMVs: Resync of degraded MVs triggered")
		resyncDegradedMVs()
	}
}

// Used for resyncing the degraded MVs in the clustermap.
func resyncDegradedMVs() {
	degradedMVs := cm.GetDegradedMVs()
	if len(degradedMVs) == 0 {
		log.Debug("ReplicationManager::ResyncDegradedMVs: No degraded MVs found")
		return
	}

	log.Debug("ReplicationManager::ResyncDegradedMVs: Degraded MVs found: %+v", degradedMVs)

	// TODO: make this parallel
	for mvName, degradedMV := range degradedMVs {
		common.Assert(degradedMV.State == dcache.StateDegraded,
			fmt.Sprintf("MV %s is not in degraded state: %+v", mvName, degradedMV))

		err := syncMV(mvName, degradedMV)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::ResyncDegradedMVs: Failed to resync MV %s [%v]", mvName, err.Error())
		}
	}
}

// syncMV is used for resyncing the degraded MV to online state.
// It first finds the lowest index online RV for the given MV. If this RV is not hosted
// in this node, it will not take the responsibility of resyncing the MV. So, it returns.
// The node hosting the lowest index online RV will be responsible for resyncing the MV.
func syncMV(mvName string, mvInfo dcache.MirroredVolume) error {
	log.Debug("ReplicationManager::ResyncMV: Resyncing MV %s : %+v", mvName, mvInfo)

	common.Assert(mvInfo.State == dcache.StateDegraded, fmt.Sprintf("MV %s is not in degraded state: %+v",
		mvName, mvInfo))

	lowestIdxRVName := getLowestIndexOnlineRV(mvInfo.RVs)
	if !cm.IsValidRVName(lowestIdxRVName) {
		err := fmt.Errorf("no online RVs found for MV %s : %+v", mvName, mvInfo)
		log.Err("ReplicationManager::ResyncMV: %v", err)
		return err
	}

	log.Debug("ReplicationManager::ResyncMV: Lowest index online RV for MV %s is %s", mvName, lowestIdxRVName)
	if !isRVHostedInThisNode(lowestIdxRVName) {
		log.Debug("ReplicationManager::ResyncMV: Lowest index online RV %s for MV %s is not hosted in this node",
			lowestIdxRVName, mvName)
		return nil
	}

	componentRVs := convertRVMapToList(mvName, mvInfo.RVs)
	log.Debug("ReplicationManager::ResyncMV: Component RVs for the given MV %s are: %v", mvName, rpc.ComponentRVsToString(componentRVs))

	// TODO: check if this is correctly returning the sync size of MV
	// this should be replaced with GetSyncSizeofMV() since the GetDiskUsageOfMV() returns the
	// total size of the MV, which includes the size in regular MV path and the sync MV path.
	// We should only return the size of the regular MV path from source RV.
	syncSize, err := rpc_server.GetDiskUsageOfMV(mvName, lowestIdxRVName)
	if err != nil {
		log.Err("ReplicationManager::ResyncMV: Failed to get disk usage of MV %s for RV %s [%v]", mvName, lowestIdxRVName, err.Error())
		return fmt.Errorf("failed to get disk usage of MV %s for RV %s [%v]", mvName, lowestIdxRVName, err.Error())
	}

	// TODO: make this parallel
	for _, rv := range componentRVs {
		if rv.State == string(dcache.StateOutOfSync) {
			err = syncComponentRV(mvName, lowestIdxRVName, rv.Name, syncSize, componentRVs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to sync component RV %s for MV %s [%v]", rv.Name, mvName, err.Error())
				log.Err("ReplicationManager::ResyncMV: %v", errStr)
				continue
			}
		}
	}

	// TODO:: integration: call cluster manager API to update the syncing RVs to online state
	// and also mark the MV as online if all the RVs are online

	log.Debug("ReplicationManager::ResyncMV: Successfully resynced MV %s", mvName)

	return nil
}

// syncComponentRV is used for syncing the target RV with the lowest index online RV (or source RV).
// It sends the StartSync() RPC call to both source and target nodes. The source node is the one
// hosting the lowest index online RV and the target node is the one hosting the target RV.
// It then updates the state from "outofsync" to "syncing" for the target RV and MV (if all RVs are syncing).
// After this, a sync job is created which is responsible for copying the out of sync chunks from the source RV
// to the target RV, and also sending the EndSync() RPC call to both source and target nodes.
func syncComponentRV(mvName string, lowestIdxRVName string, targetRVName string, syncSize int64, componentRVs []*models.RVNameAndState) error {
	log.Debug("ReplicationManager::syncComponentRV: MV %s, lowest index online RV %s, target RV %s, sync size %d, component RVs %v",
		mvName, lowestIdxRVName, targetRVName, syncSize, rpc.ComponentRVsToString(componentRVs))

	common.Assert(lowestIdxRVName != targetRVName, lowestIdxRVName, targetRVName)
	common.Assert(syncSize > 0, syncSize)

	sourceNodeID := getNodeIDFromRVName(lowestIdxRVName)
	common.Assert(common.IsValidUUID(sourceNodeID))

	targetNodeID := getNodeIDFromRVName(targetRVName)
	common.Assert(common.IsValidUUID(targetNodeID))

	// create StartSyncRequest. Same request will be sent to both source and target nodes.
	startSyncReq := &models.StartSyncRequest{
		MV:           mvName,
		SourceRVName: lowestIdxRVName,
		TargetRVName: targetRVName,
		ComponentRV:  componentRVs,
		SyncSize:     syncSize,
	}

	// Send StartSync() RPC call to the source node which is hosting the lowest index online RV.
	srcResp, err := sendStartSyncRequest(lowestIdxRVName, sourceNodeID, startSyncReq)
	if err != nil {
		errStr := fmt.Sprintf("Failed to start sync for %s/%s [%v] : %v",
			lowestIdxRVName, mvName, err.Error(), rpc.StartSyncRequestToString(startSyncReq))
		log.Err("ReplicationManager::syncComponentRV: %v", errStr)
		return err
	}

	// Send StartSync() RPC call to the target node which is hosting the target RV.
	destResp, err := sendStartSyncRequest(targetRVName, targetNodeID, startSyncReq)
	if err != nil {
		errStr := fmt.Sprintf("Failed to start sync for %s/%s [%v] : %v",
			targetRVName, mvName, err.Error(), rpc.StartSyncRequestToString(startSyncReq))
		log.Err("ReplicationManager::syncComponentRV: %v", errStr)
		return err
	}

	// TODO:: integration: call cluster manager API to update the outofsync RV to syncing state
	// and also mark the MV as syncing if all the RVs in MV are syncing

	syncJob := &syncJob{
		mvName:       mvName,
		srcRVName:    lowestIdxRVName,
		srcSyncID:    srcResp.SyncID,
		destRVName:   targetRVName,
		destSyncID:   destResp.SyncID,
		syncSize:     syncSize,
		componentRVs: componentRVs,
	}

	log.Debug("ReplicationManager::syncComponentRV: Sync job created: %+v", *syncJob)

	// TODO: this can be made asynchronous
	// send the sync job to a channel which will be processed by a worker thread
	err = performSyncJob(syncJob)
	if err != nil {
		errStr := fmt.Sprintf("Failed to perform sync job for %s [%v]", syncJob.toString(), err.Error())
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return err
	}

	return nil
}

// sendStartSyncRequest sends the StartSync() RPC call to the target node.
// rvName is the RV hosted in the target node, to which the StartSync() RPC call is sent.
func sendStartSyncRequest(rvName string, targetNodeID string, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	log.Debug("ReplicationManager::sendStartSyncRequest: Sending StartSync RPC call to %s/%s, target node ID %s : %v",
		rvName, req.MV, targetNodeID, rpc.StartSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	resp, err := rpc_client.StartSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendStartSyncRequest: Failed to start sync for %s/%s [%v] : %v",
			rvName, req.MV, err.Error(), rpc.StartSyncRequestToString(req))
		return nil, err
	}

	common.Assert((resp != nil && common.IsValidUUID(resp.SyncID)),
		rpc.StartSyncRequestToString(req))

	log.Debug("ReplicationManager::sendStartSyncRequest: StartSync RPC response for %s/%s : %+v",
		rvName, req.MV, *resp)

	return resp, nil
}

// This method copies the out of sync chunks from the source RV to the target RV.
// Then it sends the EndSync() RPC call to both source and target nodes.
func performSyncJob(job *syncJob) error {
	log.Debug("ReplicationManager::performSyncJob: Sync job: %s", job.toString())

	common.Assert(job.srcRVName != job.destRVName, job.srcRVName, job.destRVName)
	common.Assert((job.srcSyncID != job.destSyncID &&
		common.IsValidUUID(job.srcSyncID) &&
		common.IsValidUUID(job.destSyncID)),
		job.srcSyncID, job.destSyncID)

	err := copyOutOfSyncChunks(job)
	if err != nil {
		log.Err("ReplicationManager::performSyncJob: Failed to copy out of sync chunks for job %s [%v]", job.toString(), err.Error())
		return fmt.Errorf("failed to copy out of sync chunks for job %s [%v]", job.toString(), err.Error())
	}

	// call EndSync() RPC call to the source node which is hosting the source RV.
	srcNodeID := getNodeIDFromRVName(job.srcRVName)
	common.Assert(common.IsValidUUID(srcNodeID))

	endSyncReq := &models.EndSyncRequest{
		SyncID:       job.srcSyncID,
		MV:           job.mvName,
		SourceRVName: job.srcRVName,
		TargetRVName: job.destRVName,
		ComponentRV:  job.componentRVs,
		SyncSize:     job.syncSize,
	}

	_, err = sendEndSyncRequest(job.srcRVName, srcNodeID, endSyncReq)
	if err != nil {
		errStr := fmt.Sprintf("Failed to end sync for %s/%s [%v] : %v",
			job.srcRVName, job.mvName, err.Error(), rpc.EndSyncRequestToString(endSyncReq))
		log.Err("ReplicationManager::performSyncJob: %v", errStr)
		return err
	}

	// call EndSync() RPC call to the target node which is hosting the target RV.
	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	endSyncReq.SyncID = job.destSyncID
	_, err = sendEndSyncRequest(job.destRVName, destNodeID, endSyncReq)
	if err != nil {
		errStr := fmt.Sprintf("Failed to end sync for %s/%s [%v] : %v",
			job.destRVName, job.mvName, err.Error(), rpc.EndSyncRequestToString(endSyncReq))
		log.Err("ReplicationManager::performSyncJob: %v", errStr)
		return err
	}

	return nil
}

// copyOutOfSyncChunks copies the out of sync chunks from the source RV to the target RV.
// It enumerates the chunks in the source MV path and copies them to the target RV.
// The chunks are copied using the sync PutChunk() RPC call to the target RV.
func copyOutOfSyncChunks(job *syncJob) error {
	log.Debug("ReplicationManager::copyOutOfSyncChunks: Sync job: %s", job.toString())

	sourceMVPath := filepath.Join(getCachePathForRVName(job.srcRVName), job.mvName)
	common.Assert(common.DirectoryExists(sourceMVPath), fmt.Sprintf("source MV path %s does not exist in this node", sourceMVPath))

	destRvID := getRvIDFromRvName(job.destRVName)
	common.Assert(common.IsValidUUID(destRvID))

	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	// enumerate the chunks in the source MV path
	entries, err := os.ReadDir(sourceMVPath)
	if err != nil {
		log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to read directory %s [%v]", sourceMVPath, err.Error())
		return err
	}

	// TODO: make this parallel
	for _, entry := range entries {
		if entry.IsDir() {
			common.Assert(false, fmt.Sprintf("Found directory %s while enumerating chunks in %s", entry.Name(), sourceMVPath))
			log.Warn("ReplicationManager::copyOutOfSyncChunks: Skipping directory %s", entry.Name())
			continue
		}

		// chunks are stored in MV as,
		// <MvName>/<FileID>.<OffsetInMiB>.data and
		// <MvName>/<FileID>.<OffsetInMiB>.hash
		chunkParts := strings.Split(entry.Name(), ".")
		if len(chunkParts) != 3 {
			// TODO: should we return error in this case?
			log.Err("ReplicationManager::copyOutOfSyncChunks: Chunk name %s is not in the expected format", entry.Name())
			common.Assert(false, fmt.Sprintf("chunk name %s is not in the expected format", entry.Name()))
			continue
		}

		// TODO: hash validation will be done later
		// if file type is hash, skip it
		// the hash data will be transferred with the regular chunk file
		if chunkParts[2] == "hash" {
			log.Debug("ReplicationManager::copyOutOfSyncChunks: Skipping hash file %s", entry.Name())
			continue
		}

		fileID := chunkParts[0]
		common.Assert(common.IsValidUUID(fileID))

		// convert string to int64
		offsetInMiB, err := strconv.ParseInt(chunkParts[1], 10, 64)
		if err != nil {
			// TODO: should we return error in this case?
			log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to convert offset %s to int64 [%v]", chunkParts[1], err.Error())
			common.Assert(false, fmt.Sprintf("failed to convert offset %s to int64 [%v]", chunkParts[1], err.Error()))
			continue
		}

		srcChunkPath := filepath.Join(sourceMVPath, entry.Name())
		srcData, err := os.ReadFile(srcChunkPath)
		if err != nil {
			// TODO: should we return error in this case?
			log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to read file %s [%v]", srcChunkPath, err.Error())
			continue
		}

		putChunkReq := &models.PutChunkRequest{
			Chunk: &models.Chunk{
				Address: &models.Address{
					FileID:      fileID,
					RvID:        destRvID,
					MvName:      job.mvName,
					OffsetInMiB: offsetInMiB,
				},
				Data: srcData,
				Hash: "", // TODO: hash validation will be done later
			},
			Length:      int64(len(srcData)),
			IsSync:      true, // this is sync write RPC call
			ComponentRV: job.componentRVs,
		}

		log.Debug("ReplicationManager::copyOutOfSyncChunks: Copying chunk %s to RV %s : %v",
			srcChunkPath, job.destRVName, rpc.PutChunkRequestToString(putChunkReq))

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		putChunkResp, err := rpc_client.PutChunk(ctx, destNodeID, putChunkReq)
		if err != nil {
			// TODO: discuss error handling in this scenario
			log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to put chunk to RV %s [%v] : %v",
				job.destRVName, err.Error(), rpc.PutChunkRequestToString(putChunkReq))
			return err
		}

		common.Assert(putChunkResp != nil, "PutChunk RPC response is nil")

		log.Debug("ReplicationManager::copyOutOfSyncChunks: PutChunk RPC response for chunk %s to RV %s : %v",
			srcChunkPath, job.destRVName, rpc.PutChunkResponseToString(putChunkResp))
	}

	return nil
}

// sendEndSyncRequest sends the EndSync() RPC call to the target node.
// rvName is the RV hosted in the target node, to which the EndSync() RPC call is sent.
func sendEndSyncRequest(rvName string, targetNodeID string, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	log.Debug("ReplicationManager::sendEndSyncRequest: Sending EndSync RPC call to %s/%s, target node ID %s : %v",
		rvName, req.MV, targetNodeID, rpc.EndSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	resp, err := rpc_client.EndSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendEndSyncRequest: Failed to end sync for %s/%s [%v] : %v",
			rvName, req.MV, err.Error(), rpc.EndSyncRequestToString(req))
		return nil, err
	}

	common.Assert(resp != nil, rpc.EndSyncRequestToString(req))

	log.Debug("ReplicationManager::sendEndSyncRequest: EndSync RPC response for %s/%s : %+v",
		rvName, req.MV, *resp)

	return resp, nil
}

func init() {
	go periodicResyncMVs()
}
