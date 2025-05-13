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

func periodicResyncMVs() {
	// TODO: Stop this ticker from Stop() method of RM.
	ticker := time.NewTicker(ResyncInterval * time.Second)

	for range ticker.C {
		log.Debug("ReplicationManager::periodicResyncMVs: Resync of degraded MVs triggered")
		resyncDegradedMVs()
	}
}

// This is run at regular intervals for checking and resync'ing any degraded MVs as per the clustermap.
// Note that the clustermap can have 0 or more degraded MVs that need to be synchronized. These degraded MVs
// must already have been fixed (replacement RVs selected for each offline component RV) by the fix-mv workflow
// run by the ClusterManager. Fix-mv would have replaced all offline component RVs with good RVs and marked those
// RV state as "outofsync", so resyncDegradedMVs() should synchronize each of those "outofsync" RVs from a good RV.
// It'll update the state of the RVs to "syncing" and the MV state to "syncing" (if all outofsync RVs are set to
// syncing), in the global clustermap and start a synchronization go routine for each outofsync RV.
func resyncDegradedMVs() {
	degradedMVs := cm.GetDegradedMVs()
	if len(degradedMVs) == 0 {
		log.Debug("ReplicationManager::ResyncDegradedMVs: No degraded MVs found")
		return
	}

	log.Info("ReplicationManager::ResyncDegradedMVs: %d degraded MV(s) found: %+v",
		len(degradedMVs), degradedMVs)

	//
	// For each degraded MV, call syncMV() to synchronize all the outofsync RVs for that MV.
	// Each of those RV is synchronized using an independent sync job, which can fail/succeed independent
	// of other sync jobs. Hence we don't have a status for the syncMV(). If it fails, all we can do is
	// retry the resync next time around (if one or more RVs are still outofsync).
	//
	for mvName, mvInfo := range degradedMVs {
		common.Assert(mvInfo.State == dcache.StateDegraded, mvInfo.State)

		syncMV(mvName, mvInfo)
	}
}

// syncMV is used for resyncing the degraded MV to online state. To be precice it will synchronize all component
// RVs which are outofsync. It first finds the lowest index online RV (LIORV) for the given MV. If the LIORV is
// not hosted by this node, it will not take the responsibility of resyncing the MV and bails out. If it does
// host the LIORV then it takes the responsibility of syncing this MV. For that it starts a sync job for each
// component RV that's outofsync. Each of these jobs run independent of each other and they can fail or succeed
// independent of each other.
// Each sync job does the following:
//   - Send StartSync to the source and target RVs.
//   - Update MV in the global clustermap, marking the RV state as "syncing" (from "outofsync") and MV state as
//     "syncing" if there's no more "outofsync" RVs, else leaves the MV state as "degraded".
//   - Perform the chunk transfer from source to target RV.
//   - Send EndSync to the source and target RVs.
//   - Update MV in the global clustermap, marking the RV state as "online" (from "syncing") and MV state as
//     "online" if this was the last/only sync, else leaves the MV state unchanged.
func syncMV(mvName string, mvInfo dcache.MirroredVolume) {
	log.Debug("ReplicationManager::ResyncMV: Resyncing MV %s %+v", mvName, mvInfo)

	common.Assert(mvInfo.State == dcache.StateDegraded, mvName, mvInfo.State)

	lioRV := cm.LowestIndexOnlineRV(mvInfo)
	// For a degraded MV, we must have a lowest index online RV.
	common.Assert(cm.IsValidRVName(lioRV))

	log.Debug("ReplicationManager::ResyncMV: Lowest index online RV for MV %s is %s", mvName, lioRV)

	//
	// Only the node hosting the lowest index online RV performs the resync.
	//
	// TODO: See if this puts pressure on the single source replica.
	//
	if !cm.IsMyRV(lioRV) {
		log.Debug("ReplicationManager::ResyncMV: Lowest index online RV %s for MV %s, not hosted by us",
			lioRV, mvName)
		return
	}

	componentRVs := convertRVMapToList(mvName, mvInfo.RVs)

	log.Debug("ReplicationManager::ResyncMV: Component RVs for MV %s are %v",
		mvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Fetch the current disk usage of this MV. We convey this via StartSync, it can be used to check
	// %age progress. Note that JoinMV carries the reservedSpace parameter which is the more critical one
	// to decide if an RV can host a new MV replica or not.
	//
	// TODO: Make sure GetDiskUsageOfMV() correctly returns the to-be-synced data, i.e., data in the regular
	//       MV folder.
	//
	syncSize, err := rpc_server.GetDiskUsageOfMV(mvName, lioRV)
	if err != nil {
		err = fmt.Errorf("Failed to get disk usage of %s/%s [%v]", lioRV, mvName, err)
		log.Err("ReplicationManager::ResyncMV: %v", err)
		common.Assert(false, err)
		return
	}

	for _, rv := range componentRVs {
		// Only outofsync RVs need to be resynced.
		if rv.State != string(dcache.StateOutOfSync) {
			continue
		}

		log.Info("ReplicationManager::ResyncMV: Starting sync job (%s/%s -> %s/%s) for syncing %d bytes",
			lioRV, mvName, rv.Name, mvName, syncSize)

		go syncComponentRV(mvName, lioRV, rv.Name, syncSize, componentRVs)
	}
}

// syncComponentRV is used for syncing the target RV with the lowest index online RV (or source RV).
// It sends the StartSync() RPC call to both source and target nodes. The source node is the one
// hosting the lowest index online RV and the target node is the one hosting the target RV.
// It then updates the state from "outofsync" to "syncing" for the target RV and MV (if all RVs are syncing).
// After this, a sync job is created which is responsible for copying the out of sync chunks from the source RV
// to the target RV, and also sending the EndSync() RPC call to both source and target nodes.
func syncComponentRV(mvName string, lioRV string, targetRVName string, syncSize int64, componentRVs []*models.RVNameAndState) {
	log.Debug("ReplicationManager::syncComponentRV: MV %s, LIORV %s, target RV %s, sync size %d, component RVs %v",
		mvName, lioRV, targetRVName, syncSize, rpc.ComponentRVsToString(componentRVs))

	common.Assert(lioRV != targetRVName, lioRV, targetRVName)
	common.Assert(syncSize > 0, syncSize)

	sourceNodeID := getNodeIDFromRVName(lioRV)
	common.Assert(common.IsValidUUID(sourceNodeID))

	targetNodeID := getNodeIDFromRVName(targetRVName)
	common.Assert(common.IsValidUUID(targetNodeID))

	// Create StartSyncRequest. Same request will be sent to both source and target nodes.
	startSyncReq := &models.StartSyncRequest{
		MV:           mvName,
		SourceRVName: lioRV,
		TargetRVName: targetRVName,
		ComponentRV:  componentRVs,
		SyncSize:     syncSize,
	}

	//
	// Send StartSync() RPC call to the source and target RVs.
	//
	// TODO: If we encounter some failure before we send EndSync, we need to undo this StartSync?
	//
	srcSyncId, err := sendStartSyncRequest(lioRV, sourceNodeID, startSyncReq)
	if err != nil {
		log.Err("ReplicationManager::syncComponentRV: %v", err)
		return
	}

	dstSyncId, err := sendStartSyncRequest(targetRVName, targetNodeID, startSyncReq)
	if err != nil {
		log.Err("ReplicationManager::syncComponentRV: %v", err)
		return
	}

	// TODO:: integration: call cluster manager API to update the outofsync RV to syncing state
	// and also mark the MV as syncing if all the RVs in MV are syncing

	syncJob := &syncJob{
		mvName:       mvName,
		srcRVName:    lioRV,
		srcSyncID:    srcSyncId,
		destRVName:   targetRVName,
		destSyncID:   dstSyncId,
		syncSize:     syncSize,
		componentRVs: componentRVs,
	}

	log.Debug("ReplicationManager::syncComponentRV: Sync job created: %s", syncJob.toString())

	//
	// Copy all chunks from source to target replica followed by EndSync to both.
	//
	err = performSyncJob(syncJob)
	if err != nil {
		errStr := fmt.Sprintf("Failed to perform sync job for %s [%v]", syncJob.toString(), err.Error())
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return
	}

	return
}

// sendStartSyncRequest sends the StartSync() RPC call to the target node.
// rvName is the RV hosted in the target node, to which the StartSync() RPC call is sent.
func sendStartSyncRequest(rvName string, targetNodeID string, req *models.StartSyncRequest) (string, error) {
	log.Debug("ReplicationManager::sendStartSyncRequest: Sending StartSync RPC call to %s/%s, node %s %v",
		rvName, req.MV, targetNodeID, rpc.StartSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	resp, err := rpc_client.StartSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendStartSyncRequest: Failed to start sync for %s/%s %v: %v",
			rvName, req.MV, rpc.StartSyncRequestToString(req), err)
		return "", err
	}

	common.Assert((resp != nil && common.IsValidUUID(resp.SyncID)),
		rpc.StartSyncRequestToString(req))

	log.Debug("ReplicationManager::sendStartSyncRequest: StartSync RPC response for %s/%s: %+v",
		rvName, req.MV, *resp)

	return resp.SyncID, nil
}

// This method copies all chunks from the source replica to the target replica.
// Then it sends the EndSync() RPC call to both source and target nodes.
func performSyncJob(job *syncJob) error {
	log.Debug("ReplicationManager::performSyncJob: Sync job: %s", job.toString())

	common.Assert((job.srcRVName != job.destRVName) &&
		cm.IsValidRVName(job.srcRVName) &&
		cm.IsValidRVName(job.destRVName), job.srcRVName, job.destRVName)
	common.Assert((job.srcSyncID != job.destSyncID &&
		common.IsValidUUID(job.srcSyncID) &&
		common.IsValidUUID(job.destSyncID)),
		job.srcSyncID, job.destSyncID)

	err := copyOutOfSyncChunks(job)
	if err != nil {
		err = fmt.Errorf("failed to copy out of sync chunks for job %s [%v]", job.toString(), err)
		log.Err("ReplicationManager::performSyncJob: %v", err)
		return err
	}

	// Call EndSync() RPC call to the source node which is hosting the source RV.
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

	err = sendEndSyncRequest(job.srcRVName, srcNodeID, endSyncReq)
	if err != nil {
		log.Err("ReplicationManager::performSyncJob: %v", err)
		return err
	}

	// Call EndSync() RPC call to the target node which is hosting the target RV.
	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	endSyncReq.SyncID = job.destSyncID
	err = sendEndSyncRequest(job.destRVName, destNodeID, endSyncReq)
	if err != nil {
		log.Err("ReplicationManager::performSyncJob: %v", err)
		return err
	}

	// TODO:: integration: call cluster manager API to update the syncing RVs to online state
	// and also mark the MV as online if all the RVs are online

	// Log this only if this was the last sync job for the MV
	//log.Debug("ReplicationManager::ResyncMV: Successfully resynced MV %s", mvName)

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
func sendEndSyncRequest(rvName string, targetNodeID string, req *models.EndSyncRequest) error {
	log.Debug("ReplicationManager::sendEndSyncRequest: Sending EndSync RPC call to %s/%s, node %s %v",
		rvName, req.MV, targetNodeID, rpc.EndSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	resp, err := rpc_client.EndSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendEndSyncRequest: Failed to end sync for %s/%s %v: %v",
			rvName, req.MV, rpc.EndSyncRequestToString(req), err)
		return err
	}

	common.Assert(resp != nil, rpc.EndSyncRequestToString(req))

	log.Debug("ReplicationManager::sendEndSyncRequest: EndSync RPC response for %s/%s %+v",
		rvName, req.MV, *resp)

	return nil
}

func init() {
	go periodicResyncMVs()
}
