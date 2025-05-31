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
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

type replicationMgr struct {
	ticker *time.Ticker // ticker for periodic resync of degraded MVs

	// Channel to signal when the replication manager is done.
	// This is used to stop the thread doing the periodic resync of degraded MVs.
	done chan bool

	// Wait group to wait for the goroutines spawned, before stopping the replication manager.
	wg sync.WaitGroup

	// Set of currently running sync jobs, indexed by target replica ("rvX/mvY") and the value
	// stored is the source replica in "rvX/mvY" format.
	// Note that there can only be a single sync job for a given target replica.
	runningJobs sync.Map

	// Thread pool for sending RPC requests.
	tp *threadpool
}

var rm *replicationMgr

// Create a new replication manager instance and start the periodic resync of degraded MVs.
func Start() error {
	common.Assert(rm == nil, "Replication manager already exists")

	log.Debug("ReplicationManager::Start: Starting replication manager")

	rm = &replicationMgr{
		ticker: time.NewTicker(ResyncInterval * time.Second),
		done:   make(chan bool),
		tp:     newThreadPool(MAX_WORKER_COUNT),
	}

	rm.wg.Add(1)

	// run the periodic resync of degraded MVs in a separate goroutine
	go periodicResyncMVs()

	// Start the thread pool for sending RPC requests.
	rm.tp.start()

	return nil
}

// Stop the replication manager instance.
// This will stop the periodic resync of degraded MVs.
func Stop() {
	common.Assert(rm != nil, "Replication manager does not exist")

	log.Debug("ReplicationManager::Stop: Stopping replication manager")

	rm.ticker.Stop()
	rm.done <- true
	rm.wg.Wait()

	rm.tp.stop()
}

func ReadMV(req *ReadMvRequest) (*ReadMvResponse, error) {
	common.Assert(req != nil)

	log.Debug("ReplicationManager::ReadMV: Received ReadMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		err = fmt.Errorf("invalid ReadMV request %s [%v]", req.toString(), err)
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
				err = fmt.Errorf("no suitable RV found for MV %s", req.MvName)
				log.Err("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

			//
			// This is very unlikely and it would most likely indicate that we have a “very stale”
			// clustermap where all/most of the component RVs have been replaced.
			//

			err = cm.RefreshClusterMap()
			if err != nil {
				err = fmt.Errorf("RefreshClusterMap() failed, failing read %s",
					req.toString())
				log.Warn("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

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

			// Must read all the requested data.
			common.Assert(len(rpcResp.Chunk.Data) == int(req.Length), len(rpcResp.Chunk.Data), req.Length)

			break
		}

		//
		// TODO: We should handle errors that indicate retrying from a different RV would help.
		// 	 RVs are the final source of truth wrt MV membership (and anything else),
		// 	 so if the target RV feels that the sender seems to have out-of-date clustermap,
		// 	 it can help him by failing the request with an appropriate error and then
		// 	 caller should fetch the latest clustermap and then try again.
		//       Note that the current code will also work as it'll refresh the clustermap after
		//	 read attempts from all current component RVs fail, but we can be more efficient.
		//

		log.Err("ReplicationManager::ReadMV: Failed to get chunk from node %s for request %v [%v]",
			targetNodeID, rpc.GetChunkRequestToString(rpcReq), err)
	}

	log.Debug("ReplicationManager::ReadMV: GetChunk RPC response: %v", rpc.GetChunkResponseToString(rpcResp))

	// TODO: hash validation will be done later
	// TODO: should we validate the hash of the chunk here?
	// hash := getMD5Sum(rpcResp.Chunk.Data)
	// if hash != rpcResp.Chunk.Hash {
	//      log.Err("ReplicationManager::ReadMV: Hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr)
	//      common.Assert(false, fmt.Sprintf("hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr))
	//      return nil, fmt.Errorf("hash mismatch for the chunk read from node %s for request %v", targetNodeID, rpcReqStr)
	// }

	resp := &ReadMvResponse{
		Data: rpcResp.Chunk.Data,
	}

	if err := resp.isValid(req); err != nil {
		err = fmt.Errorf("invalid ReadMV response [%v]", err)
		log.Err("ReplicationManager::ReadMV: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	return resp, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	common.Assert(req != nil)

	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		err = fmt.Errorf("invalid WriteMV request %s [%v]", req.toString(), err)
		log.Err("ReplicationManager::WriteMV: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	var rvsWritten []string
	retryCnt := 0

	// TODO: TODO: hash validation will be done later
	// get hash of the data in the request
	// hash := getMD5Sum(req.Data)

retry:
	if retryCnt > 0 {
		log.Info("ReplicationManager::WriteMV: [%d] Retrying WriteMV %v after clustermap refresh, RVs written in prev attempt: %v",
			retryCnt, req.toString(), rvsWritten)
	}

	// Get component RVs for MV, from clustermap.
	componentRVs := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::WriteMV: Component RVs for %s are: %v",
		req.MvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Response channel to receive response for the PutChunk RPCs sent to each component RV.
	//
	responseChannel := make(chan *responseItem, len(componentRVs))

	//
	// List of RVs to which the chunk was written successfully in this WriteMV attempt, used for logging.
	// Note that everytime we refresh clustermap we need to write all the replicas according to the
	// latest component RVs. An MV write should be considered a transaction that is applied to the cluster
	// in a given state. Once a transaction is applied successfully, then we are guaranteed that any change
	// to the MV composition will ensure that any chunk written will be correctly synchronized.
	// Note that rvInfo/mvInfo and the NeedToRefreshClusterMap error returned by the target RV(s), helps
	// check if the transaction can be safely applied.
	//
	rvsWritten = nil

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
			log.Debug("ReplicationManager::WriteMV: Skipping %s/%s (state %s)",
				rv.Name, req.MvName, rv.State)

			//
			// Skip writing to this RV, as it is in offline or outofsync state.
			// So, send nil response to the response channel to indicate that
			// we are not writing to this RV.
			//
			common.Assert(len(responseChannel) < len(componentRVs),
				len(responseChannel), len(componentRVs))
			responseChannel <- nil
		} else if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
			rvID := getRvIDFromRvName(rv.Name)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rv.Name)
			common.Assert(common.IsValidUUID(targetNodeID))

			log.Debug("ReplicationManager::WriteMV: Writing to %s/%s (rvID: %s, state: %s) on node %s",
				rv.Name, req.MvName, rvID, rv.State, targetNodeID)

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
				SyncID:      "", // this is regular client write
				ComponentRV: componentRVs,
			}

			log.Debug("ReplicationManager::WriteMV: Sending PutChunk request for %s/%s to node %s: %s",
				rv.Name, req.MvName, targetNodeID, rpc.PutChunkRequestToString(rpcReq))

			//
			// Schedule PutChunk RPC call to the target node.
			// One of the threadpool threads will pick this request and call PutChunk.
			//
			rm.tp.schedule(&workitem{
				targetNodeID: targetNodeID,
				rvName:       rv.Name,
				putChunkReq:  rpcReq,
				respChannel:  responseChannel,
			})
		} else {
			common.Assert(false, "Unexpected RV state", rv.State, rv.Name, req.MvName)
		}
	}

	//
	// Non-retriable error that we should fail the WriteMV() with.
	// It could be non-retriable error returned by any of the replica PutChunks.
	// Note that WriteMV is considered successful only if all the replica writes are successful.
	//
	var errWriteMV error

	//
	// We have scheduled all replica PutChunks, they will complete as they are sent out and served by the
	// target RV. Wait for all the PutChunk RPC calls to complete.
	//
	common.Assert(len(responseChannel) <= len(componentRVs), len(responseChannel), len(componentRVs))

	//
	// Flag to track if any of the RVs failed with NeedToRefreshClusterMap.
	// We refresh the clustermap once per iteration (labelled "retry") even if multiple replica
	// PutChunks failed with NeedToRefreshClusterMap.
	//
	clusterMapRefreshed := false

	for i := 0; i < len(componentRVs); i++ {
		respItem := <-responseChannel
		if respItem == nil {
			//
			// This means that we skipped writing to this RV, as it was in offline/outofsync state.
			//
			continue
		}

		if respItem.err == nil {
			common.Assert(respItem.putChunkResp != nil)

			log.Debug("ReplicationManager::WriteMV: PutChunk successful for %s/%s, RPC response: %v",
				respItem.rvName, req.MvName, rpc.PutChunkResponseToString(respItem.putChunkResp))

			//
			// Write to this component RV was successful, add it to the list of RVs successfully written
			// in this attempt.
			//
			rvsWritten = append(rvsWritten, respItem.rvName)
			common.Assert(len(rvsWritten) <= len(componentRVs), len(rvsWritten), len(componentRVs))

			continue
		}

		log.Err("ReplicationManager::WriteMV: PutChunk to %s/%s, node %s failed [%v]",
			respItem.rvName, req.MvName, respItem.targetNodeID, respItem.err)

		rpcErr := rpc.GetRPCResponseError(respItem.err)
		if rpcErr == nil {
			//
			// This error indicates some transport error, i.e., RPC request couldn't make it to the
			// server and hence didn't solicit a response. It could be some n/w issue, blobfuse
			// process down or node down.
			//
			// We should now run the inband RV offline detection workflow, basically we
			// call the clustermap's UpdateComponentRVState() API to mark this
			// component RV as offline and force the fix-mv workflow which will eventually
			// trigger the resync-mv workflow.
			//
			log.Err("ReplicationManager::WriteMV: PutChunk %s/%s, failed to reach node %s [%v]",
				respItem.rvName, req.MvName, respItem.targetNodeID, respItem.err)

			errRV := cm.UpdateComponentRVState(req.MvName, respItem.rvName, dcache.StateOffline)
			if errRV != nil {
				//
				// If we fail to update the component RV as offline, we cannot safely complete
				// the chunk write or else the failed replica may not be resynced causing data
				// consistency issues.
				//
				errStr := fmt.Sprintf("failed to update %s/%s state to offline [%v]",
					respItem.rvName, req.MvName, errRV)
				log.Err("ReplicationManager::WriteMV: %s", errStr)
				errWriteMV = errRV
				continue
			}

			//
			// If UpdateComponentRVState() succeeds, marking this component RV as offline,
			// we can safely carry on with the write since we are guaranteed that these
			// chunks which we could not write to this component RV will be later sync'ed
			// from one of the good component RVs.
			//
			log.Warn("ReplicationManager::WriteMV: Writing to %s/%s on node %s failed, marked RV offline",
				respItem.rvName, req.MvName, respItem.targetNodeID)
			continue
		}

		// The error is RPC error of type *rpc.ResponseError.
		if rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
			// TODO: Should we allow more than one clustermap refresh?
			if retryCnt > 0 {
				errWriteMV = fmt.Errorf("failed to write to %s/%s after refreshing clustermap [%v]",
					respItem.rvName, req.MvName, respItem.err)
				log.Err("ReplicationManager::WriteMV: %v", errWriteMV)
				continue
			}

			if clusterMapRefreshed {
				// Clustermap has already been refreshed once in this try, so skip it.
				continue
			}

			// TODO: retry till the next epoch, till the clustermap is refreshed.
			// Case: StartSync() RPC calls are successful, but before the state of the target RV
			// is updated to "syncing" in clustermap, some other node calls WriteMV() with the outdated
			// clustermap, which results in the component RVs rejecting the request with
			// NeedToRefreshClusterMap error. Even after the clustermap is refreshed, it may get the same
			// state of the target RV as "outofsync" as the clustermap update by the source (or lio) RV
			// may not have completed yet, so the target RV may not be in "syncing" state.
			errCM := cm.RefreshClusterMap()
			if errCM != nil {
				errWriteMV = fmt.Errorf("RefreshClusterMap() failed, failing write %s [%v]",
					req.toString(), errCM)
				log.Warn("ReplicationManager::WriteMV: %v", errWriteMV)
				continue
			}

			clusterMapRefreshed = true
		} else {
			// TODO: check if this is non-retriable error.
			log.Err("ReplicationManager::WriteMV: PutChunk to %s/%s node %s, failed with non-retriable error [%v]",
				respItem.rvName, req.MvName, respItem.targetNodeID, respItem.err)
			errWriteMV = respItem.err
			continue
		}
	}

	//
	// If any of the PutChunk call fails with these errors, we fail the WriteMV operation.
	//   - If the node is unreachable and updating clustermap state to "offline"
	//     for the component RV failed.
	//   - If the clustermap was refreshed and retry failed with NeedToRefreshClusterMap error.
	//   - If clustermap refresh via RefreshClusterMap() failed.
	//   - If PutChunk failed with non-retriable error.
	//
	if errWriteMV != nil {
		log.Err("ReplicationManager::WriteMV: Failed to write to MV %s, %s [%v]",
			req.MvName, req.toString(), errWriteMV)
		return nil, errWriteMV
	}

	if clusterMapRefreshed {
		//
		// If we refreshed the clustermap, we need to retry the entire write MV with the updated clustermap.
		// This might mean re-writing some of the replicas which were successfully written in this iteration.
		//
		retryCnt++
		goto retry
	}

	// For a non-offline MV, at least one replica write should succeed.
	if len(rvsWritten) == 0 {
		err := fmt.Errorf("WriteMV could not write to any replica: %v", req.toString())
		log.Err("ReplicationManager::WriteMV: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	return &WriteMvResponse{}, nil
}

func periodicResyncMVs() {
	defer rm.wg.Done()

	for {
		select {
		case <-rm.done:
			log.Info("ReplicationManager::periodicResyncMVs: stopping periodic resync of degraded MVs")
			return
		case <-rm.ticker.C:
			log.Debug("ReplicationManager::periodicResyncMVs: Resync of degraded MVs triggered")
			resyncDegradedMVs()
		}
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
	log.Debug("ReplicationManager::syncMV: Resyncing MV %s %+v", mvName, mvInfo)

	common.Assert(mvInfo.State == dcache.StateDegraded, mvName, mvInfo.State)

	lioRV := cm.LowestIndexOnlineRV(mvInfo)
	// For a degraded MV, we must have a lowest index online RV.
	common.Assert(cm.IsValidRVName(lioRV))

	log.Debug("ReplicationManager::syncMV: Lowest index online RV for MV %s is %s", mvName, lioRV)

	//
	// Only the node hosting the lowest index online RV performs the resync.
	//
	// TODO: See if this puts pressure on the single source replica.
	//
	if !cm.IsMyRV(lioRV) {
		log.Debug("ReplicationManager::syncMV: Lowest index online RV %s for MV %s, not hosted by us",
			lioRV, mvName)
		return
	}

	componentRVs := convertRVMapToList(mvName, mvInfo.RVs)

	log.Debug("ReplicationManager::syncMV: Component RVs for MV %s are %v",
		mvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Fetch the current disk usage of this MV. We convey this via StartSync, it can be used to check
	// %age progress. Note that JoinMV carries the reservedSpace parameter which is the more critical one
	// to decide if an RV can host a new MV replica or not.
	//
	syncSize, err := GetMVSize(mvName)
	if err != nil {
		err = fmt.Errorf("failed to get disk usage of %s/%s [%v]", lioRV, mvName, err)
		log.Err("ReplicationManager::syncMV: %v", err)
		common.Assert(false, err)
		return
	}

	for _, rv := range componentRVs {
		// Only outofsync RVs need to be resynced.
		if rv.State != string(dcache.StateOutOfSync) {
			continue
		}

		srcReplica := fmt.Sprintf("%s/%s", lioRV, mvName)
		tgtReplica := fmt.Sprintf("%s/%s", rv.Name, mvName)

		//
		// Don't run more than one sync job for the same target replica.
		// This is to prevent periodic calls to resyncDegradedMVs() from starting replication
		// for a target replica, that's already running.
		//
		val, ok := rm.runningJobs.Load(tgtReplica)
		if ok {
			log.Info("ReplicationManager::syncMV: Not starting sync job (%s/%s -> %s/%s), %s -> %s already running",
				lioRV, mvName, rv.Name, mvName, val.(string), tgtReplica)
			continue
		}

		log.Info("ReplicationManager::syncMV: Starting sync job (%s/%s -> %s/%s) for syncing %d bytes",
			lioRV, mvName, rv.Name, mvName, syncSize)

		// Store it in the map to avoid multiple sync jobs for the same target.
		rm.runningJobs.Store(tgtReplica, srcReplica)

		// Increment the wait group for the goroutine that will run the syncComponentRV() function.
		rm.wg.Add(1)

		go func() {
			// Decrement the wait group when the syncComponentRV() function completes.
			defer rm.wg.Done()

			// Remove from the map, once the syncjob completes (success or failure).
			defer rm.runningJobs.Delete(tgtReplica)

			syncComponentRV(mvName, lioRV, rv.Name, syncSize, componentRVs)
		}()
	}
}

// syncComponentRV is used for syncing the target RV from the lowest index online RV (or source RV).
// It sends the StartSync() RPC call to both source and target nodes. The source node is the one
// hosting the lowest index online RV and the target node is the one hosting the target RV.
// It then updates the state from "outofsync" to "syncing" for the target RV and MV (if all RVs are syncing).
// After this, a sync job is created which is responsible for copying the out of sync chunks from the source RV
// to the target RV, and also sending the EndSync() RPC call to both source and target nodes.
func syncComponentRV(mvName string, lioRV string, targetRVName string, syncSize int64,
	componentRVs []*models.RVNameAndState) {
	log.Debug("ReplicationManager::syncComponentRV: %s/%s -> %s/%s, sync size %d bytes, component RVs %v",
		lioRV, mvName, targetRVName, mvName, syncSize, rpc.ComponentRVsToString(componentRVs))

	common.Assert(lioRV != targetRVName, lioRV, targetRVName)
	common.Assert(syncSize >= 0, syncSize)

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
	// TODO: Send StartSync() to all the component RVs, since it changes the RV state from outofsync
	//       to syncing, every component RV needs to know the change, not just the source and target.
	//       This will matter when an MV starts syncing during client write.
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

	//
	// Update the destination RV from outofsync to syncing state. The cluster manager will take care of
	// updating the MV state to syncing if all component RVs have either online or syncing state.
	//
	err = cm.UpdateComponentRVState(mvName, targetRVName, dcache.StateSyncing)
	if err != nil {
		errStr := fmt.Sprintf("Failed to update component RV %s/%s state to syncing [%v]",
			targetRVName, mvName, err)
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return
	}

	//
	// Now that the target RV state is updated to syncing from outofsync, the WriteMV() workflow will
	// consider the target RV as valid candidate for client PutChunk() calls.
	// This means that all the chunks written in this MV before now, will need to be synced or copied
	// to the target RV by the sync PutChunk() RPC calls.
	// The chunks written to the MV after this point will be written to the target RV as well,
	// since the target RV is now in syncing state.
	//
	syncStartTime := time.Now().UnixMicro() + NTPClockSkewMargin

	//
	// Update the state of target RV from outofsync to syncing in local component RVs list.
	// The updated component RVs list will be later used in the PutChunk(sync) RPC calls to the target RV.
	//
	updateLocalComponentRVState(componentRVs, targetRVName, dcache.StateOutOfSync, dcache.StateSyncing)

	syncJob := &syncJob{
		mvName:        mvName,
		srcRVName:     lioRV,
		srcSyncID:     srcSyncId,
		destRVName:    targetRVName,
		destSyncID:    dstSyncId,
		syncSize:      syncSize,
		componentRVs:  componentRVs,
		syncStartTime: syncStartTime,
	}

	log.Debug("ReplicationManager::syncComponentRV: Sync job created: %s", syncJob.toString())

	//
	// Copy all chunks from source to target replica followed by EndSync to both.
	//
	err = runSyncJob(syncJob)
	if err != nil {
		errStr := fmt.Sprintf("Failed to run sync job %s [%v]", syncJob.toString(), err)
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return
	}
}

// sendStartSyncRequest sends the StartSync() RPC call to the target node.
// rvName is the RV hosted in the target node, to which the StartSync() RPC call is sent.
// Note that we send StartSync to every component RV of an MV.
func sendStartSyncRequest(rvName string, targetNodeID string, req *models.StartSyncRequest) (string, error) {
	log.Debug("ReplicationManager::sendStartSyncRequest: Sending StartSync RPC call to %s/%s, node %s %v",
		rvName, req.MV, targetNodeID, rpc.StartSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	//
	// Caller passes the same StartSyncRequest for both source and target RVs.
	// Clear it here to keep the assert in StartSync() happy.
	//
	req.SenderNodeID = ""

	resp, err := rpc_client.StartSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendStartSyncRequest: StartSync failed for %s/%s %v: %v",
			rvName, req.MV, rpc.StartSyncRequestToString(req), err)

		//
		// Right now we treat all StartSync failures as being caused by stale clustermap.
		// Refresh the clustermap and fail the job. This target replica will be picked up
		// in the next periodic call to syncMV().
		// TODO: Check for NeedToRefreshClusterMap and only on that error, refresh the clustermap.
		//
		err1 := cm.RefreshClusterMap()
		if err1 != nil {
			log.Err("ReplicationManager::sendStartSyncRequest: RefreshClusterMap failed: %v", err1)
		}

		return "", err
	}

	common.Assert((resp != nil && common.IsValidUUID(resp.SyncID)),
		rpc.StartSyncRequestToString(req))

	log.Debug("ReplicationManager::sendStartSyncRequest: StartSync RPC response for %s/%s: %+v",
		rvName, req.MV, *resp)

	return resp.SyncID, nil
}

// This method runs one sync job that synchronizes one MV replica.
// It copies all chunks from the source replica to the target replica.
// Then it sends the EndSync() RPC call to both source and target nodes.
func runSyncJob(job *syncJob) error {
	log.Debug("ReplicationManager::runSyncJob: Sync job: %s", job.toString())

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
		log.Err("ReplicationManager::runSyncJob: %v", err)
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

	//
	// TODO: Send EndSync to all the component RVs, since it changes the RV state from syncing
	//       to online, every component RV needs to know the change, not just the source and target.
	//       This will matter when an MV starts syncing during client write.
	//
	err = sendEndSyncRequest(job.srcRVName, srcNodeID, endSyncReq)
	if err != nil {
		// TODO: We need to check the error extensively as we do for the dest RV below.
		log.Err("ReplicationManager::runSyncJob: %v", err)
		return err
	}

	// Call EndSync() RPC call to the target node which is hosting the target RV.
	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	endSyncReq.SyncID = job.destSyncID
	err = sendEndSyncRequest(job.destRVName, destNodeID, endSyncReq)
	if err != nil {
		log.Err("ReplicationManager::runSyncJob: %v", err)

		rpcErr := rpc.GetRPCResponseError(err)
		if rpcErr == nil {
			//
			// This error means that the node is not reachable.
			//
			// We should now run the inband RV offline detection workflow, basically we
			// call the clustermap's UpdateComponentRVState() API to mark this
			// component RV as offline and force the fix-mv workflow which will finally
			// trigger the resync-mv workflow.
			//
			log.Err("ReplicationManager::runSyncJob: Failed to reach node %s [%v]",
				destNodeID, err)

			errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateOffline)
			if errRV != nil {
				errStr := fmt.Sprintf("Failed to mark %s/%s as offline [%v]",
					job.destRVName, job.mvName, errRV)
				log.Err("ReplicationManager::runSyncJob: %s", errStr)
			}
		} else {
			//
			// Update the destination RV from syncing to outofsync state. The cluster manager will
			// take care of updating the MV state to degraded.
			// The periodic resyncMVs() will take care of resyncing this outofsync RV in next iteration.
			//
			errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateOutOfSync)
			if errRV != nil {
				errStr := fmt.Sprintf("Failed to mark %s/%s as outofsync [%v]",
					job.destRVName, job.mvName, errRV)
				log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
			}
		}

		return err
	}

	//
	// Now that we have successfully copied all chunks from source to target replica, update the
	// destination RV from syncing to online state. The cluster manager will take care of
	// updating the MV state to online if all component RVs have online state.
	//
	err = cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateOnline)
	if err != nil {
		errStr := fmt.Sprintf("Failed to mark %s/%s as online [%v]",
			job.destRVName, job.mvName, err)
		log.Err("ReplicationManager::runSyncJob: %s", errStr)
		return err
	}

	// Log this only if this was the last sync job for the MV
	//log.Debug("ReplicationManager::ResyncMV: Successfully resynced MV %s", mvName)

	return nil
}

// copyOutOfSyncChunks copies the out of sync chunks from the source to target MV replica.
// The out of sync chunks are determined on the basis of the sync start time.
// The chunks that are created before this time are considered out of sync and
// need to be copied to the target RV by the sync PutChunk() RPC call.
// Whereas the chunks created after this time are written to both source and target RVs by the
// client PutChunk() RPC calls, and hence ignored here.
func copyOutOfSyncChunks(job *syncJob) error {
	log.Debug("ReplicationManager::copyOutOfSyncChunks: Sync job: %s", job.toString())

	sourceMVPath := filepath.Join(getCachePathForRVName(job.srcRVName), job.mvName)
	common.Assert(common.DirectoryExists(sourceMVPath), sourceMVPath)

	destRvID := getRvIDFromRvName(job.destRVName)
	common.Assert(common.IsValidUUID(destRvID))

	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	//
	// Enumerate the chunks in the source MV path
	// TODO: os.ReadDir() will returns all enumerated chunks. For really large number of chunk, consider
	//       using getdents() kind of streaming API.
	//
	entries, err := os.ReadDir(sourceMVPath)
	if err != nil {
		log.Err("ReplicationManager::copyOutOfSyncChunks: os.ReadDir(%s) failed: [%v]",
			sourceMVPath, err)
		return err
	}

	// TODO: make this parallel
	for _, entry := range entries {
		if entry.IsDir() {
			log.Warn("ReplicationManager::copyOutOfSyncChunks: Skipping directory %s/%s",
				sourceMVPath, entry.Name())
			// We don't expect dirs in our MV replicas.
			common.Assert(false, entry.Name(), sourceMVPath)
			continue
		}

		info, err := entry.Info()
		common.Assert(err == nil, err)

		if info.ModTime().UnixMicro() > job.syncStartTime {
			// This chunk is created after the sync start time, so it will be written to both source and target
			// RVs by the client PutChunk() RPC calls, so we can skip it here.
			log.Debug("ReplicationManager::copyOutOfSyncChunks: Skipping chunk %s/%s, "+
				"Mtime (%d) > syncStartTime (%d) [%d usecs after sync start]",
				sourceMVPath, entry.Name(), info.ModTime().UnixMicro(), job.syncStartTime,
				info.ModTime().UnixMicro()-job.syncStartTime)
			continue
		}

		log.Debug("ReplicationManager::copyOutOfSyncChunks: Copying chunk %s/%s, Mtime (%d) <= syncStartTime (%d)",
			sourceMVPath, entry.Name(), info.ModTime().UnixMicro(), job.syncStartTime)
		log.Debug("ReplicationManager::copyOutOfSyncChunks: Copying chunk %s/%s, "+
			"Mtime (%d) <= syncStartTime (%d) [%d usecs before sync start]",
			sourceMVPath, entry.Name(), info.ModTime().UnixMicro(), job.syncStartTime,
			job.syncStartTime-info.ModTime().UnixMicro())

		//
		// chunks are stored in MV as,
		// <MvName>/<FileID>.<OffsetInMiB>.data and
		// <MvName>/<FileID>.<OffsetInMiB>.hash
		//
		chunkParts := strings.Split(entry.Name(), ".")
		if len(chunkParts) != 3 {
			// TODO: should we return error in this case?
			errStr := fmt.Sprintf("Invalid chunk name %s", entry.Name())
			log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
			common.Assert(false, errStr)
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
			errStr := fmt.Sprintf("Invalid offset for chunk %s [%v]", entry.Name(), err)
			log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
			common.Assert(false, errStr)
			continue
		}

		srcChunkPath := filepath.Join(sourceMVPath, entry.Name())
		srcData, err := os.ReadFile(srcChunkPath)
		if err != nil {
			// TODO: should we return error in this case?
			errStr := fmt.Sprintf("os.ReadFile(%s) failed [%v]", srcChunkPath, err.Error())
			log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
			common.Assert(false, errStr)
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
			Length: int64(len(srcData)),
			// this is sync write RPC call, so the sync ID should be that of the target RV.
			SyncID:      job.destSyncID,
			ComponentRV: job.componentRVs,
		}

		log.Debug("ReplicationManager::copyOutOfSyncChunks: Copying chunk %s to %s/%s: %v",
			srcChunkPath, job.destRVName, job.mvName, rpc.PutChunkRequestToString(putChunkReq))

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		putChunkResp, err := rpc_client.PutChunk(ctx, destNodeID, putChunkReq)
		if err != nil {
			log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to put chunk to %s/%s [%v]: %v",
				job.destRVName, job.mvName, err, rpc.PutChunkRequestToString(putChunkReq))

			rpcErr := rpc.GetRPCResponseError(err)
			if rpcErr == nil {
				//
				// This error means that the node is not reachable.
				//
				// We should now run the inband RV offline detection workflow, basically we
				// call the clustermap's UpdateComponentRVState() API to mark this
				// component RV as offline and force the fix-mv workflow which will finally
				// trigger the resync-mv workflow.
				//
				log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to reach node %s [%v]",
					destNodeID, err)

				errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName,
					dcache.StateOffline)
				if errRV != nil {
					errStr := fmt.Sprintf("Failed to mark %s/%s as offline [%v]",
						job.destRVName, job.mvName, errRV)
					log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
					common.Assert(false, errStr)
				}
			} else {
				//
				// Update the destination RV from syncing to outofsync state. The cluster manager
				// will take care of updating the MV state to degraded.
				// The periodic resyncMVs() will take care of resyncing this outofsync RV in next
				// iteration.
				//
				errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName,
					dcache.StateOutOfSync)
				if errRV != nil {
					errStr := fmt.Sprintf("Failed to mark %s/%s as outofsync [%v]",
						job.destRVName, job.mvName, errRV)
					log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
					common.Assert(false, errStr)
				}
			}

			return err
		}

		common.Assert(putChunkResp != nil)

		log.Debug("ReplicationManager::copyOutOfSyncChunks: Successfully copied chunk %s to %s/%s: %v",
			srcChunkPath, job.destRVName, job.mvName, rpc.PutChunkResponseToString(putChunkResp))
	}

	return nil
}

// sendEndSyncRequest sends the EndSync() RPC call to the target node.
// rvName is the RV hosted in the target node, to which the EndSync() RPC call is sent.
// Note that we send EndSync to every component RV of an MV.
func sendEndSyncRequest(rvName string, targetNodeID string, req *models.EndSyncRequest) error {
	log.Debug("ReplicationManager::sendEndSyncRequest: Sending EndSync RPC call to %s/%s, node %s %v",
		rvName, req.MV, targetNodeID, rpc.EndSyncRequestToString(req))

	common.Assert(common.IsValidUUID(targetNodeID))

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	//
	// Caller passes the same StartSyncRequest for both source and target RVs.
	// Clear it here to keep the assert in StartSync() happy.
	//
	req.SenderNodeID = ""

	resp, err := rpc_client.EndSync(ctx, targetNodeID, req)
	if err != nil {
		log.Err("ReplicationManager::sendEndSyncRequest: EndSync failed for %s/%s %v: %v",
			rvName, req.MV, rpc.EndSyncRequestToString(req), err)
		return err
	}

	common.Assert(resp != nil, rpc.EndSyncRequestToString(req))

	log.Debug("ReplicationManager::sendEndSyncRequest: EndSync RPC response for %s/%s %+v",
		rvName, req.MV, *resp)

	return nil
}

func GetMVSize(mvName string) (int64, error) {
	common.Assert(cm.IsValidMVName(mvName), mvName)

	log.Debug("ReplicationManager::GetMVSize: MV = %s", mvName)

	var mvSize int64
	var err error

	componentRVs := getComponentRVsForMV(mvName)

	log.Debug("ReplicationManager::GetMVSize: Component RVs for %s are: %v",
		mvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Get the most suitable RV from the list of component RVs,
	// from which we should get the size of the MV. Selecting most
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
			err = fmt.Errorf("no suitable RV found for MV %s", mvName)
			log.Err("ReplicationManager::GetMVSize: %v", err)
			return 0, err
		}

		common.Assert(!slices.Contains(excludeRVs, readerRV.Name), readerRV.Name, excludeRVs)

		targetNodeID := getNodeIDFromRVName(readerRV.Name)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::GetMVSize: Selected %s for %s, hosted by node %s",
			readerRV.Name, mvName, targetNodeID)

		req := &models.GetMVSizeRequest{
			MV:     mvName,
			RVName: readerRV.Name,
		}

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		resp, err := rpc_client.GetMVSize(ctx, targetNodeID, req)

		// Exclude this RV from further iterations (if any).
		excludeRVs = append(excludeRVs, readerRV.Name)

		if err == nil {
			// Success.
			common.Assert(resp != nil, rpc.GetMVSizeRequestToString(req))
			mvSize = resp.MvSize
			log.Debug("ReplicationManager::GetMVSize: GetMVSize successful for %s, RPC response: MV size = %d",
				rpc.GetMVSizeRequestToString(req), resp.MvSize)
			break
		}

		log.Err("ReplicationManager::GetMVSize: Failed to get MV size from node %s for request %v [%v]",
			targetNodeID, rpc.GetMVSizeRequestToString(req), err)
	}

	return mvSize, nil
}
