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
	"sync/atomic"
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

//go:generate $ASSERT_REMOVER $GOFILE

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

	//
	// Maximum number of syncJobs (running syncComponentRV()) that can be running simultaneously.
	// After this we do not start any more syncJobs till some of the existing ones complete.
	// TODO: We should distinguish between "short" and "long" sync jobs (based on the sync size) and
	//       have separate limits for both.
	//
	maxSimulSyncJobs int64

	// Number of sync jobs currently running.
	numSyncJobs atomic.Int64
}

var rm *replicationMgr

// Create a new replication manager instance and start the periodic resync of degraded MVs.
func Start() error {
	common.Assert(rm == nil, "Replication manager already exists")

	log.Debug("ReplicationManager::Start: Starting replication manager")

	rm = &replicationMgr{
		ticker:           time.NewTicker(ResyncInterval * time.Second),
		done:             make(chan bool),
		tp:               newThreadPool(MAX_WORKER_COUNT),
		maxSimulSyncJobs: MAX_SIMUL_SYNC_JOBS,
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

	//
	// We don't expect the caller to pass invalid requests, so only verify in debug builds.
	//
	if common.IsDebugBuild() {
		if err := req.isValid(); err != nil {
			err = fmt.Errorf("invalid ReadMV request %s [%v]", req.toString(), err)
			log.Err("ReplicationManager::ReadMV: %v", err)
			common.Assert(false, err)
			return nil, err
		}
	}

	var rpcResp *models.GetChunkResponse
	var isBufExternal bool = true
	var err error
	var lastClusterMapEpoch int64

	clusterMapRefreshed := false
	retryCnt := 0

retry:
	//
	// Give up after sufficient clustermap refresh attempts.
	// One refresh is all we need in most cases, but we retry a few times to add extra resilience in case
	// of any unexpected errors. This is important as failing here will result in application request failure
	// which should only be done when we really cannot proceed.
	//
	// TODO: make it more resilient
	//
	if retryCnt > 5 {
		err = fmt.Errorf("no suitable RV found for MV %s even after %d clustermap refresh retries, last epoch %d",
			req.MvName, retryCnt, lastClusterMapEpoch)
		log.Err("ReplicationManager::ReadMV: %v", err)
		return nil, err
	}

	// Get component RVs for MV, from clustermap.
	mvState, componentRVs, lastClusterMapEpoch := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::ReadMV: Component RVs for %s (%s) are %s (retryCnt: %d, clusterMapRefreshed: %v)",
		req.MvName, mvState, rpc.ComponentRVsToString(componentRVs), retryCnt, clusterMapRefreshed)

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
			// An MV once marked offline can never become online, so save the trip to clustermap.
			//
			// TODO: We should support reading from offline MVs in case due to some disaster multiple
			//       cluster nodes were brought down and later brought up, so they have valid data, but
			//       currently we won't allow reading from them as they are offline.
			//       This will also require changes in safeCleanupMyRVs() to not delete the MVs from
			//       such RVs. But, note that we cannot allow writing to such MVs.
			//
			if mvState == dcache.StateOffline {
				err = fmt.Errorf("%s is offline", req.MvName)
				log.Err("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

			//
			// If the current clustermap does not have any suitable RV to read from, we try clustermap
			// refresh just in case we have a stale clustermap. This is very unlikely and it would most
			// likely indicate that we have a “very stale” clustermap where all/most of the component RVs
			// have been replaced, or most of them are down.
			//
			// Even after refreshing clustermap if we cannot get a valid MV replica to read from,
			// alas we need to fail the read.
			//
			if clusterMapRefreshed {
				err = fmt.Errorf("no suitable RV found for MV %s even after clustermap refresh to epoch %d",
					req.MvName, lastClusterMapEpoch)
				log.Err("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

			err = cm.RefreshClusterMap(lastClusterMapEpoch)
			if err != nil {
				log.Warn("ReplicationManager::ReadMV: RefreshClusterMap() failed for %s (retryCnt: %d): %v",
					req.toString(), retryCnt, err)
			} else {
				clusterMapRefreshed = true
			}

			retryCnt++
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

		//
		// If the node to which the GetChunk() RPC call must be made is local,
		// then we directly call the GetChunk() method using the local server's handler.
		// Else we call the GetChunk() RPC via the Thrift RPC client.
		//
		if targetNodeID == rpc.GetMyNodeUUID() {
			if rpcResp, err = rpc_server.GetChunkLocal(ctx, rpcReq); err == nil {
				//
				// This Buffer is allocated from the in house bufferPool.
				//
				isBufExternal = false
			}
		} else {
			rpcResp, err = rpc_client.GetChunk(ctx, targetNodeID, rpcReq)
		}

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

		log.Warn("ReplicationManager::ReadMV: Failed to get chunk from node %s for request %s: %v",
			targetNodeID, rpc.GetChunkRequestToString(rpcReq), err)

		rpcErr := rpc.GetRPCResponseError(err)
		if rpcErr != nil && rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
			//
			// RPC server can return models.ErrorCode_NeedToRefreshClusterMap in two cases:
			// 1. It genuinely wants the client to refresh the clustermap as it knows that
			//    the client has an older clustermap.
			// 2. It hit some transient error while fetching the clustermap itself, so it cannot
			//    be sure whether clustermap refresh at the client will help or not. To be safe
			//    we refresh the clustermap for a limited number of times before failing the read.
			//
			errCM := cm.RefreshClusterMap(lastClusterMapEpoch)
			if errCM != nil {
				// Log and retry, it'll help in case of transient errors at the server.
				log.Warn("ReplicationManager::ReadMV: RefreshClusterMap() failed for %s (retryCnt: %d): %v",
					req.toString(), retryCnt, errCM)
			} else {
				clusterMapRefreshed = true
			}

			retryCnt++
			goto retry
		}

		// Try another replica if available.
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
		Data:          rpcResp.Chunk.Data,
		IsBufExternal: isBufExternal,
	}

	//
	// We don't expect the server to return invalid response, so only verify in debug builds.
	//
	if common.IsDebugBuild() {
		if err := resp.isValid(req); err != nil {
			err = fmt.Errorf("invalid ReadMV response [%v]", err)
			log.Err("ReplicationManager::ReadMV: %v", err)
			common.Assert(false, err)
			return nil, err
		}
	}

	return resp, nil
}

func writeMVInternal(req *WriteMvRequest, putChunkStyle PutChunkStyleEnum) (*WriteMvResponse, error) {
	log.Debug("ReplicationManager::writeMVInternal: Received WriteMV request (%v): %v", putChunkStyle, req.toString())

	var rvsWritten []string
	retryCnt := 0

	//
	// If the putChunkStyle is OriginatorSendsToAll, it means that we are retrying after BrokenChain
	// error in the previous attempt using DaisyChain mode.
	//
	brokenChain := (putChunkStyle == OriginatorSendsToAll)

	if brokenChain {
		log.Warn("ReplicationManager::writeMVInternal: Retrying WriteMV %s with OriginatorSendsToAll after BrokenChain error in previous DaisyChain attempt",
			req.toString())
	}

	// TODO: TODO: hash validation will be done later
	// get hash of the data in the request
	// hash := getMD5Sum(req.Data)

retry:
	if retryCnt > 0 {
		//
		// We shouldn't be retrying for a BrokenChain error, instead we should return and caller will
		// reissue writeMVInternal() with OriginatorSendsToAll style.
		//
		common.Assert(!brokenChain)

		log.Info("ReplicationManager::WriteMV: [%d] Retrying WriteMV %v after clustermap refresh, RVs written in prev attempt: %v",
			retryCnt, req.toString(), rvsWritten)
	}

	//
	// Get component RVs for MV, from clustermap and also the corresponding clustermap epoch.
	// If server returns NeedToRefreshClusterMap, we will ask cm.RefreshClusterMap() to update
	// the clustermap to a value higher than this epoch.
	//
	mvState, componentRVs, lastClusterMapEpoch := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::writeMVInternal: Component RVs for %s (%s) are: %v",
		req.MvName, mvState, rpc.ComponentRVsToString(componentRVs))

	//
	// Response channel to receive response for the PutChunk RPCs sent to each component RV.
	//
	responseChannel := make(chan *responseItem, len(componentRVs))

	//
	// List of RVs to which the chunk was written successfully in this WriteMV attempt, used for logging.
	// Note that every time we refresh clustermap we need to write all the replicas according to the
	// latest component RVs. An MV write should be considered a transaction that is applied to the cluster
	// in a given state. Once a transaction is applied successfully, then we are guaranteed that any change
	// to the MV composition will ensure that any chunk written will be correctly synchronized.
	// Note that rvInfo/mvInfo and the NeedToRefreshClusterMap error returned by the target RV(s), helps
	// check if the transaction can be safely applied.
	//
	rvsWritten = nil

	//
	// Allocate the PutChunkDCRequest for orchestrating the required PutChunk calls as per the PutChunkStyle
	// selected. If PutChunkStyle is OriginatorSendsToAll then we will send the PutChunk request to each
	// component RV in putChunkDCReq.Request and putChunkDCReq.NextRVs, while for PutChunkStyle DaisyChain, we
	// will just send the PutChunkDC request to the first RV (in putChunkDCReq.Request).
	//
	// We set the "MaybeOverwrite" flag to true in PutChunkRequest to let the server know that this
	// could potentially be an overwrite of a chunk that we previously wrote, so that it relaxes its
	// overwrite checks. To be safe we set MaybeOverwrite to true when retryCnt > 0 or we are retrying
	// because of brokenChain error for one of the RVs.
	//
	putChunkDCReq := &models.PutChunkDCRequest{
		Request: &models.PutChunkRequest{
			Chunk: &models.Chunk{
				Address: &models.Address{
					FileID:      req.FileID,
					RvID:        "", // will be set later down
					MvName:      req.MvName,
					OffsetInMiB: req.ChunkIndex * req.ChunkSizeInMiB,
				},
				Data: req.Data,
				Hash: "", // TODO: hash validation will be done later
			},
			Length:         int64(len(req.Data)),
			SyncID:         "", // this is regular client write
			ComponentRV:    componentRVs,
			MaybeOverwrite: retryCnt > 0 || brokenChain,
		},
		NextRVs: make([]string, 0), // will be added later down, if needed
	}

	//
	// Go over all the component RVs and populate the putChunkDCReq.
	// If any of the component RVs is local, then putChunkDCReq.Request refers to that and
	// putChunkDCReq.NextRVs contains all the other RVs. If none of the component RVs is local, then
	// putChunkDCReq.Request refers to the first component RV in the componentRVs list and
	// putChunkDCReq.NextRVs contains all the other RVs.
	// We don't write to offline and outofsync component RVs, so they won't be added to putChunkDCReq.
	//
	for _, rv := range componentRVs {
		//
		// Omit writing to RVs in “offline”, "inband-offline" or “outofsync” state. It’s ok to omit them as the chunks not
		// written to them will be copied to them when the mv is (soon) resynced.
		// Otoh if an RV is in “syncing” state then any new chunk written to it may not be copied by the
		// ongoing resync operation as the source RV may have been already gone past the enumeration stage
		// and hence won’t consider this chunk for resync, and hence those MUST have the chunks mandatorily
		// copied to them.
		//
		if rv.State == string(dcache.StateOffline) ||
			rv.State == string(dcache.StateInbandOffline) ||
			rv.State == string(dcache.StateOutOfSync) {
			log.Debug("ReplicationManager::writeMVInternal: Skipping %s/%s (RV state: %s, MV state: %s)",
				rv.Name, req.MvName, rv.State, mvState)

			// Online MV must have all replicas online.
			common.Assert(mvState != dcache.StateOnline, req.MvName)

			//
			// Skip writing to this RV, as it is in offline or outofsync state.
			// So, send nil response to the response channel to indicate that
			// we are not writing to this RV.
			//
			common.Assert(len(responseChannel) < len(componentRVs),
				len(responseChannel), len(componentRVs))
			responseChannel <- nil
		} else if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
			// Offline MV has all replicas offline.
			common.Assert(mvState != dcache.StateOffline, req.MvName)

			rvID := getRvIDFromRvName(rv.Name)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rv.Name)
			common.Assert(common.IsValidUUID(targetNodeID))

			log.Debug("ReplicationManager::writeMVInternal: Writing to %s/%s (rvID: %s, state: %s) on node %s",
				rv.Name, req.MvName, rvID, rv.State, targetNodeID)

			// Add local component RV to putChunkDCReq.Request.
			if targetNodeID == rpc.GetMyNodeUUID() {
				// Only one component RV can be local.
				common.Assert(putChunkDCReq.Request.Chunk.Address.RvID == "",
					putChunkDCReq.Request.Chunk.Address.String(), rv.Name, rvID)
				putChunkDCReq.Request.Chunk.Address.RvID = rvID
			} else {
				// Non-local component RVs get added to putChunkDCReq.NextRVs.
				putChunkDCReq.NextRVs = append(putChunkDCReq.NextRVs, rv.Name)
			}
		} else {
			common.Assert(false, "Unexpected RV state", rv.State, rv.Name, req.MvName)
		}
	}

	//
	// If none of the RVs was writeable, no PutChunk/PutChunkDC calls to make.
	//
	if len(responseChannel) == len(componentRVs) {
		log.Err("ReplicationManager::writeMVInternal: Could not write to any component RV, req: %s, component RVs: %s",
			req.toString(), rpc.ComponentRVsToString(componentRVs))
		common.Assert(len(rvsWritten) == 0, len(rvsWritten))
		goto processResponses
	}

	//
	// If no component RV is local, then set the putChunkDCReq next hop to the first component RV.
	//
	if putChunkDCReq.Request.Chunk.Address.RvID == "" {
		// There is at least one component RV that we want to write to.
		common.Assert(len(putChunkDCReq.NextRVs) > 0)

		rvName := putChunkDCReq.NextRVs[0]
		rvID := getRvIDFromRvName(rvName)

		putChunkDCReq.Request.Chunk.Address.RvID = rvID
		putChunkDCReq.NextRVs = putChunkDCReq.NextRVs[1:]
	}

	common.Assert(common.IsValidUUID(putChunkDCReq.Request.Chunk.Address.RvID),
		putChunkDCReq.Request.Chunk.Address.String())

	//
	// Use PutChunk to write if PutChunkStyle is OriginatorSendsToAll or we have only the nexthop
	// RV to send the request to.
	//
	if putChunkStyle == OriginatorSendsToAll || len(putChunkDCReq.NextRVs) == 0 {
		// TODO: Add rvName to Address to avoid potentially expensive search for RV name.
		rvName := getRvNameFromRvID(putChunkDCReq.Request.Chunk.Address.RvID)

		targetNodeID := getNodeIDFromRVName(rvName)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::writeMVInternal: Sending PutChunk [%s] request for %s/%s to node %s: %s",
			putChunkStyle, rvName, req.MvName, targetNodeID, rpc.PutChunkRequestToString(putChunkDCReq.Request))

		//
		// Set it to OriginatorSendsToAll as we are sending PutChunk to all component RVs.
		// This will ensure RPC errors are handled correctly.
		//
		putChunkStyle = OriginatorSendsToAll

		//
		// Schedule PutChunk RPC call to the nexthop RV.
		// One of the threadpool threads will pick this request and call PutChunk.
		// Since we have to wait for all the replica writes to complete before we
		// can start processing the individual responses we send the last replica
		// inline and save one threadpool thread.
		//
		isLastComponentRV := len(putChunkDCReq.NextRVs) == 0
		rm.tp.schedule(&workitem{
			targetNodeID: targetNodeID,
			rvName:       rvName,
			reqType:      putChunkRequest,
			rpcReq:       putChunkDCReq.Request,
			respChannel:  responseChannel,
		}, isLastComponentRV /* runInline */)

		//
		// Write to all remaining component RVs.
		//
		for componentRVIdx, rvName := range putChunkDCReq.NextRVs {
			rvID := getRvIDFromRvName(rvName)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rvName)
			common.Assert(common.IsValidUUID(targetNodeID))
			common.Assert(targetNodeID != rpc.GetMyNodeUUID(), targetNodeID, rpc.GetMyNodeUUID())

			putChunkReq := &models.PutChunkRequest{
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
				Length:         int64(len(req.Data)),
				SyncID:         "", // this is regular client write
				ComponentRV:    componentRVs,
				MaybeOverwrite: retryCnt > 0 || brokenChain,
			}

			log.Debug("ReplicationManager::writeMVInternal: Sending PutChunk request for %s/%s to node %s: %s",
				rvName, req.MvName, targetNodeID, rpc.PutChunkRequestToString(putChunkReq))

			isLastComponentRV := componentRVIdx == (len(putChunkDCReq.NextRVs) - 1)
			rm.tp.schedule(&workitem{
				targetNodeID: targetNodeID,
				rvName:       rvName,
				reqType:      putChunkRequest,
				rpcReq:       putChunkReq,
				respChannel:  responseChannel,
			}, isLastComponentRV /* runInline */)
		}
	} else if putChunkStyle == DaisyChain {
		rvName := getRvNameFromRvID(putChunkDCReq.Request.Chunk.Address.RvID)
		targetNodeID := getNodeIDFromRVName(rvName)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::writeMVInternal: Sending PutChunkDC request for nexthop %s/%s to node %s: %s",
			rvName, req.MvName, targetNodeID, rpc.PutChunkDCRequestToString(putChunkDCReq))

		//
		// Check if the next-hop RV and the next RVs in chain are present in the iffy RV map.
		// If yes, we retry the operation using OriginatorSendsToAll.
		//
		iffyRVs := getIffyRVs(rvName, putChunkDCReq.NextRVs)
		if len(iffyRVs) > 0 {
			err := fmt.Errorf("Iffy RVs %v found in the component RVs, retrying with OriginatorSendsToAll",
				iffyRVs)
			log.Err("ReplicationManager::writeMVInternal: %v", err)
			return nil, rpc.NewResponseError(models.ErrorCode_BrokenChain, err.Error())
		}

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		var putChunkDCResp *models.PutChunkDCResponse
		var err error

		//
		// If the node to which the PutChunkDC() RPC call must be made is local,
		// then we directly call the PutChunkDC() method using the local server's handler.
		// Else we call the PutChunkDC() RPC via the Thrift RPC client.
		//
		if targetNodeID == rpc.GetMyNodeUUID() {
			putChunkDCResp, err = rpc_server.PutChunkDCLocal(ctx, putChunkDCReq)
		} else {
			putChunkDCResp, err = rpc_client.PutChunkDC(ctx, targetNodeID, putChunkDCReq)
		}

		if err != nil {
			log.Err("ReplicationManager::writeMVInternal: Failed to send PutChunkDC request for nexthop %s/%s to node %s: %v",
				rvName, req.MvName, targetNodeID, err)
			common.Assert(putChunkDCResp == nil)

			//
			// If an RV is marked iffy, it means that either it is down or some other downstream
			// connection issue is preventing the PutChunkDC call to succeed.
			// So, if an RV is marked iffy, the RPC client will fail the PutChunkDC() call to prevent
			// the timeout error from happening again. In this case, we will retry the WriteMV() operation
			// with OriginatorSendsToAll mode.
			//
			if strings.Contains(err.Error(), "iffy") {
				log.Debug("ReplicationManager::writeMVInternal: RV %s is marked iffy, retrying with OriginatorSendsToAll",
					rvName)
				return nil, rpc.NewResponseError(models.ErrorCode_BrokenChain, err.Error())
			}

			//
			// PutChunkDC() call to the RV failed. This indicates that the request was not forwarded to the
			// next RVs. So, convert this error to ThriftError for this RV and add BrokenChain error for the
			// next RVs, and store it in the putChunkDCResp.
			//
			putChunkDCResp = rpc.HandlePutChunkDCError(rvName, putChunkDCReq.NextRVs, req.MvName, err)
		} else {
			log.Debug("ReplicationManager::writeMVInternal: Received PutChunkDC response from nexthop %s/%s node %s: %s",
				rvName, req.MvName, targetNodeID, rpc.PutChunkDCResponseToString(putChunkDCResp))
			common.Assert(len(putChunkDCResp.Responses) == len(putChunkDCReq.NextRVs)+1,
				len(putChunkDCResp.Responses), len(putChunkDCReq.NextRVs))
		}

		common.Assert(putChunkDCResp != nil)

		//
		// Add the PutChunkDC response to the response channel.
		//
		addPutChunkDCResponseToChannel(putChunkDCResp, responseChannel)
	} else {
		common.Assert(false, "Unexpected PutChunkStyle", putChunkStyle)
	}

processResponses:
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

	//
	// Flag to check if we have a BrokenChain error in the PutChunkDC response.
	// If we have a BrokenChain error for an RV, it means that the PutChunkDC request was not
	// forwarded as the nexthop RV was down/offline. We will get ThriftError for the nexthop RV
	// and BrokenChain error for the subsequent RVs.
	// In case of BrokenChain error, we return error to WriteMV() which retries the operation with
	// OriginatorSendsToAll mode.
	//
	brokenChain = false

	for i := 0; i < len(componentRVs); i++ {
		respItem := <-responseChannel
		if respItem == nil {
			//
			// This means that we skipped writing to this RV, as it was in offline/inband-offline/outofsync state.
			//
			continue
		}

		putChunkResp, ok := respItem.rpcResp.(*models.PutChunkResponse)
		_ = putChunkResp
		_ = ok
		common.Assert(ok)

		if respItem.err == nil {
			common.Assert(putChunkResp != nil)

			log.Debug("ReplicationManager::writeMVInternal: PutChunk successful for %s/%s, RPC response: %s",
				respItem.rvName, req.MvName, rpc.PutChunkResponseToString(putChunkResp))

			//
			// Write to this component RV was successful, add it to the list of RVs successfully written
			// in this attempt.
			//
			rvsWritten = append(rvsWritten, respItem.rvName)
			common.Assert(len(rvsWritten) <= len(componentRVs), len(rvsWritten), len(componentRVs))

			continue
		}

		log.Err("ReplicationManager::writeMVInternal: [%v] PutChunk to %s/%s failed [%v]",
			putChunkStyle, respItem.rvName, req.MvName, respItem.err)

		common.Assert(putChunkResp == nil)

		rpcErr := rpc.GetRPCResponseError(respItem.err)
		if rpcErr == nil || rpcErr.GetCode() == models.ErrorCode_ThriftError {
			//
			// This error indicates some transport error, i.e., RPC request couldn't make it to the
			// server and hence didn't solicit a response. It could be some n/w issue, blobfuse
			// process down or node down.
			//
			// We should now run the inband RV offline detection workflow, basically we
			// call the clustermap's UpdateComponentRVState() API to mark this
			// component RV as inband-offline and force the fix-mv workflow which will eventually
			// trigger the resync-mv workflow.
			//
			log.Err("ReplicationManager::writeMVInternal: PutChunk %s/%s, failed to reach node [%v]",
				respItem.rvName, req.MvName, respItem.err)

			//
			// In DaisyChain mode, we cannot tell for sure which node has bad connection, so do not
			// mark the RV as inband-offline.
			//
			if putChunkStyle != DaisyChain {
				errRV := cm.UpdateComponentRVState(req.MvName, respItem.rvName, dcache.StateInbandOffline)
				if errRV != nil {
					//
					// If we fail to update the component RV as offline, we cannot safely complete
					// the chunk write or else the failed replica may not be resynced causing data
					// consistency issues.
					//
					errStr := fmt.Sprintf("failed to update %s/%s state to inband-offline [%v]",
						respItem.rvName, req.MvName, errRV)
					log.Err("ReplicationManager::writeMVInternal: %s", errStr)
					errWriteMV = errRV
					continue
				}

				//
				// If UpdateComponentRVState() succeeds, marking this component RV as offline,
				// we can safely carry on with the write since we are guaranteed that these
				// chunks which we could not write to this component RV will be later sync'ed
				// from one of the good component RVs.
				//
				log.Warn("ReplicationManager::writeMVInternal: Writing to %s/%s failed, marked RV inband-offline",
					respItem.rvName, req.MvName)
			} else {
				log.Warn("ReplicationManager::WriteMV: Writing to %s/%s failed in DaisyChain mode, not marking RV as inband-offline",
					respItem.rvName, req.MvName)
				//
				// This is actually not a broken chain error, but in order to retry with OriginatorSendsToAll
				// to mark the RV as inband-offline, we set brokenChain to true.
				//
				brokenChain = true
			}

			continue
		}

		//
		// The error is RPC error of type *rpc.ResponseError.
		//
		if rpcErr.GetCode() == models.ErrorCode_BrokenChain {
			// BrokenChain error can only be returned for PutChunkStyle DaisyChain.
			common.Assert(putChunkStyle == DaisyChain && len(putChunkDCReq.NextRVs) > 0,
				putChunkStyle, len(putChunkDCReq.NextRVs))

			// BrokenChain error should not be returned for the nexthop RV to which we send the
			// PutChunkDC request. It should only be returned for the next RVs.
			common.Assert(getRvIDFromRvName(respItem.rvName) != putChunkDCReq.Request.Chunk.Address.RvID,
				respItem.rvName, putChunkDCReq.Request.Chunk.Address.RvID)

			brokenChain = true

			log.Debug("ReplicationManager::writeMVInternal: PutChunkDC call not forwarded to %s/%s [%v]",
				respItem.rvName, req.MvName, respItem.err)
		} else if rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
			//
			// We allow 5 refreshes of the clustermap for resiliency, before we fail the write.
			// This is to allow multiple changes to the MV during the course of a single write.
			// It's unlikely but we need to be resilient.
			//
			if retryCnt > 5 {
				errWriteMV = fmt.Errorf("failed to write to %s/%s after refreshing clustermap [%v]",
					respItem.rvName, req.MvName, respItem.err)
				log.Err("ReplicationManager::writeMVInternal: %v", errWriteMV)
				continue
			}

			if clusterMapRefreshed {
				// Clustermap has already been refreshed once in this try, so skip it.
				continue
			}

			//
			// Retry till the next epoch, ensuring that the clustermap is refreshed from what we
			// have cached right now.
			// Case: StartSync() RPC calls are successful, but before the state of the target RV
			//       is updated to "syncing" in clustermap, this node calls WriteMV() with the
			//       outdated clustermap, which results in the component RVs rejecting the request with
			//       NeedToRefreshClusterMap error. Even after the clustermap is refreshed, it may get
			//       the state of the target RV as "outofsync" as the clustermap update by the source
			//       (or lio) RV may not have completed yet, so the target RV may not be in "syncing"
			//       state.
			//
			errCM := cm.RefreshClusterMap(lastClusterMapEpoch)
			if errCM != nil {
				//
				// RPC server can return models.ErrorCode_NeedToRefreshClusterMap in two cases:
				// 1. It genuinely wants the client to refresh the clustermap as it knows that
				//    the client has an older clustermap.
				// 2. It hit some transient error while fetching the clustermap itself, so it
				//    cannot be sure whether clustermap refresh at the client will help or not.
				//
				// To be safe we refresh the clustermap for a limited number of times before
				// failing the write.
				//
				log.Warn("ReplicationManager::writeMVInternal: RefreshClusterMap() failed for %s (retryCnt: %d): %v",
					req.toString(), retryCnt, errCM)
			}

			//
			// Fake clusterMapRefreshed even when RefreshClusterMap() fails, as we later retry only when
			// clusterMapRefreshed is true.
			//
			clusterMapRefreshed = true
		} else {
			// TODO: check if this is non-retriable error.
			if putChunkStyle == DaisyChain {
				//
				// For an unknown error, retry once with OriginatorSendsToAll for better resiliency.
				//
				log.Warn("ReplicationManager::writeMVInternal: PutChunk to %s/%s failed with non-retriable error [%v], will retry with OriginatorSendsToAll",
					respItem.rvName, req.MvName, respItem.err)
				brokenChain = true
			} else {
				errWriteMV = fmt.Errorf("PutChunk to %s/%s failed with non-retriable error [%v]",
					respItem.rvName, req.MvName, respItem.err)
				log.Err("ReplicationManager::writeMVInternal: %v", errWriteMV)
			}
			continue
		}
	}

	//
	// If any of the PutChunk call fails with these errors, we fail the WriteMV operation.
	//   - If the node is unreachable and updating clustermap state to "inband-offline"
	//     for the component RV failed.
	//   - If the clustermap was refreshed 5 times and it still failed with NeedToRefreshClusterMap error.
	//   - If clustermap refresh via RefreshClusterMap() failed.
	//   - If PutChunk failed with non-retriable error.
	//
	if errWriteMV != nil {
		log.Err("ReplicationManager::writeMVInternal: Failed to write to MV %s, %s [%v]",
			req.MvName, req.toString(), errWriteMV)
		return nil, errWriteMV
	}

	if brokenChain {
		common.Assert(putChunkStyle == DaisyChain && len(putChunkDCReq.NextRVs) > 0,
			putChunkStyle, len(putChunkDCReq.NextRVs))

		//
		// If we got BrokenChain error, it means that we need to retry the entire write MV operation
		// again with OriginatorSendsToAll mode.
		// This can be a case of bad connection between 2 nodes which can cause the PutChunkDC operation
		// to fail. In DaisyChain approach we may not tell with surety which node has connection issue.
		// So, retrying with DaisyChain mode may not help in this case. So, we retry the WriteMV operation
		// with OriginatorSendsToAll mode.
		// This might mean re-writing some of the replicas which were successfully written in this iteration.
		// We return BrokenChain error here and WriteMV then retries with OriginatorSendsToAll mode.
		//
		err := fmt.Errorf("BrokenChain error occurred for %s, %s", req.MvName, req.toString())
		log.Err("ReplicationManager::writeMVInternal: %v", err)
		return nil, rpc.NewResponseError(models.ErrorCode_BrokenChain, err.Error())
	}

	if clusterMapRefreshed {
		// Offline MV has all replicas offline, so we cannot get a NeedToRefreshClusterMap error.
		common.Assert(mvState != dcache.StateOffline, req.MvName)

		//
		// If we refreshed the clustermap, we need to retry the entire write MV with the updated clustermap.
		// This might mean re-writing some of the replicas which were successfully written in this iteration.
		//
		retryCnt++
		goto retry
	}

	// Fail write with a meaningful error.
	if mvState == dcache.StateOffline {
		err := fmt.Errorf("%s is offline", req.MvName)
		log.Err("ReplicationManager::writeMVInternal: %v", err)
		return nil, err
	}

	// For a non-offline MV, at least one replica write should succeed.
	if len(rvsWritten) == 0 {
		err := fmt.Errorf("WriteMV could not write to any replica: %v", req.toString())
		log.Err("ReplicationManager::writeMVInternal: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	return &WriteMvResponse{}, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	common.Assert(req != nil)

	if common.IsDebugBuild() {
		startTime := time.Now()
		defer func() {
			log.Debug("ReplicationManager::WriteMV: WriteMV request took %s: %v",
				time.Since(startTime), req.toString())
		}()
	}

	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: %v", req.toString())

	//
	// We don't expect the caller to pass invalid requests, so only verify in debug builds.
	//
	if common.IsDebugBuild() {
		if err := req.isValid(); err != nil {
			err = fmt.Errorf("invalid WriteMV request %s [%v]", req.toString(), err)
			log.Err("ReplicationManager::WriteMV: %v", err)
			common.Assert(false, err)
			return nil, err
		}
	}

	//
	// We first try to write the MV using the DaisyChain mode.
	// If it fails with BrokenChain error we retry using OriginatorSendsToAll mode.
	// This is because in DaisyChain mode we cannot tell which node in the chain had a bad connection,
	// so we cannot correctly mark the offending RV as inband-offline.
	//
	resp, err := writeMVInternal(req, DaisyChain)
	if err != nil {
		log.Err("ReplicationManager::WriteMV: Failed to write MV %s using DaisyChain, %s [%v]",
			req.MvName, req.toString(), err)

		rpcErr := rpc.GetRPCResponseError(err)
		if rpcErr != nil && rpcErr.GetCode() == models.ErrorCode_BrokenChain {
			log.Warn("ReplicationManager::WriteMV: One or more nodes in the path are down, retrying WriteMV with OriginatorSendsToAll mode: %s",
				req.toString())

			// Retry with OriginatorSendsToAll mode.
			resp, err = writeMVInternal(req, OriginatorSendsToAll)
			if err != nil {
				log.Err("ReplicationManager::WriteMV: Failed to write MV %s using OriginatorSendsToAll, %s [%v]",
					req.MvName, req.toString(), err)
				return nil, err
			}
		}
	}

	return resp, err
}

// File IO manager can use this to delete all chunks belonging to a file from a given MV.
// Note that files chunks could be striped across multiple MVs, so file IO manager needs to call this
// for every MV from the file layout.
func RemoveMV(req *RemoveMvRequest) (*RemoveMvResponse, error) {
	common.Assert(req != nil)

	log.Debug("ReplicationManager::RemoveMV: Received RemoveMV request: %s", req.toString())

	//
	// We don't expect the caller to pass invalid requests, so only verify in debug builds.
	//
	if common.IsDebugBuild() {
		if err := req.isValid(); err != nil {
			err = fmt.Errorf("invalid RemoveMV request FileID: %s, MV: %s [%v]", req.FileID, req.MvName, err)
			log.Err("ReplicationManager::RemoveMV: %v", err)
			common.Assert(false, err)
			return nil, err
		}
	}

	//
	// Deleting file chunks from an MV amounts to deleting chunks for that file from all component RVs.
	// Get the list of component RVs and send a RemoveChunk RPC to each.
	//
	mvState, rvs, _ := getComponentRVsForMV(req.MvName)
	_ = mvState
	retryNeeded := false

	//
	// Response channel to receive response for the RemoveChunk RPCs sent to each component RV.
	//
	responseChannel := make(chan *responseItem, len(rvs))

	isRvEligibleForDeletion := func(rv *models.RVNameAndState) bool {
		//
		// We can only safely delete chunks from component RVs that are online.
		// From offline RVs we cannot delete, as they may not be reachable.
		// outofsync RVs may not yet have any chunks.
		// syncing RVs may have chunks added to them while we are deleting, so we may miss some chunks.
		//
		// RemoveMV() succeeds only when it has deleted chunks from all online RVs and there are no
		// outofsync or syncing RVs. If yes, then we simply return an error asking caller to retry after
		// some time.
		//
		if rv.State == string(dcache.StateOffline) || rv.State == string(dcache.StateInbandOffline) ||
			rv.State == string(dcache.StateSyncing) || rv.State == string(dcache.StateOutOfSync) {
			log.Info("ReplicationManager::RemoveMV: skip deleting fileId %s chunks from %s/%s, rv state: %s",
				req.FileID, rv.Name, req.MvName, rv.State)

			if rv.State != string(dcache.StateOffline) && rv.State != string(dcache.StateInbandOffline) {
				//
				// GC must retry for this MV again, till this RV state changes to the StateOnline.
				//
				retryNeeded = true
			}
			return false
		}
		return true
	}

	// Schedule rpc Requests for RemoveChunk RPC for all RVs in parallel.
	for i, rv := range rvs {

		if !isRvEligibleForDeletion(rv) {
			responseChannel <- nil
			continue
		}

		common.Assert(rv.State == string(dcache.StateOnline), rv.Name, req.MvName, rv.State)
		// At least one RV online, MV should not be offline.
		common.Assert(mvState != dcache.StateOffline)

		// Remove all the chunks for the file which are present in this RV.
		rvId := getRvIDFromRvName(rv.Name)
		targetNodeId := getNodeIDFromRVName(rv.Name)

		removeChunkReq := &models.RemoveChunkRequest{
			Address: &models.Address{
				FileID:      req.FileID,
				RvID:        rvId,
				MvName:      req.MvName,
				OffsetInMiB: -1,
			},
			ComponentRV: rvs,
		}

		isLastComponentRV := (i == (len(rvs) - 1))
		rm.tp.schedule(&workitem{
			targetNodeID: targetNodeId,
			rvName:       rv.Name,
			reqType:      removeChunkRequest,
			rpcReq:       removeChunkReq,
			respChannel:  responseChannel,
		}, isLastComponentRV /* runInline */)
	}

	// Get the responses for all the RPC requests.
	for _, rv := range rvs {
		respItem := <-responseChannel
		if respItem == nil {
			// The request for this RV is not Scheduled.
			continue
		}

		err := respItem.err
		rpcResp, ok := respItem.rpcResp.(*models.RemoveChunkResponse)
		_ = ok
		common.Assert(ok)
		if err == nil {
			//
			// Status success with numChunksdeleted==0 signifies RemoveChunk was able to successfully delete all
			// chunks for the file and there are no more chunks left.
			// Note that even if RemoveChunk is able to delete all chunks we still retry once and only in the
			// next RemoveChunk call which does not find any chunks to be deleted, we consider rv/mv as fully
			// deleted.
			//
			if rpcResp.NumChunksDeleted != 0 {
				log.Info("ReplicationManager::RemoveMV: Delete partial success for %s/%s fileID: %s, NumChunksDeleted: %d",
					rv.Name, req.MvName, req.FileID, rpcResp.NumChunksDeleted)
				retryNeeded = true
			}
		} else {
			// Any error in deletion will cause GC to requeue and retry.
			log.Err("ReplicationManager::RemoveMV: Failed to delete chunks from %s/%s, fileID: %s: %v",
				rv.Name, req.MvName, req.FileID, err)
			retryNeeded = true
		}
	}

	if retryNeeded {
		err := fmt.Errorf("retry needed as some RVs of %s may be synchronizing, fileID: %s, RVs: %v",
			req.MvName, req.FileID, rvs)
		log.Err("ReplicationManager::RemoveMV: %v", err)
		return nil, err
	}

	return &RemoveMvResponse{}, nil
}

func periodicResyncMVs() {
	defer rm.wg.Done()

	for {
		select {
		case <-rm.done:
			log.Info("ReplicationManager::periodicResyncMVs: stopping periodic resync of degraded MVs")
			return
		case <-rm.ticker.C:
			log.Debug("ReplicationManager::periodicResyncMVs: Resync of syncable MVs triggered")
			resyncSyncableMVs()
		}
	}
}

// This is run at regular intervals for checking and resync'ing any syncable MVs as per the clustermap.
// syncable MVs are those degraded MVs for which there's at least one component RV in outofsync state, which
// needs to be sync'ed.
// Note that the clustermap can have 0 or more degraded MVs that need to be synchronized. These degraded MVs
// must already have been fixed (replacement RVs selected for each offline component RV) by the fix-mv workflow
// run by the ClusterManager. Fix-mv would have replaced all offline component RVs with good RVs and marked those
// RV state as "outofsync", so resyncSyncableMVs() should synchronize each of those "outofsync" RVs from a good RV.
// It'll update the state of the RVs to "syncing" and the MV state to "syncing" (if all outofsync RVs are set to
// syncing), in the global clustermap and start a synchronization go routine for each outofsync RV.
func resyncSyncableMVs() {
	var syncableMVs map[string]dcache.MirroredVolume
	clusterMapRefreshed := false

	for {
		syncableMVs = cm.GetSyncableMVs()
		if len(syncableMVs) == 0 {
			log.Debug("ReplicationManager::ResyncSyncableMVs: No syncable MVs found (%d degraded MVs)",
				len(cm.GetDegradedMVs()))
			return
		}

		//
		// If the cached clustermap suggests that there could be one or more syncable MVs, we refresh
		// the clustermap once before we start the sync, to make sure we perform the sync based on the
		// latest clustermap.
		// This extra clustermap refresh will help us avoid attempting any invalid sync.
		// Note that we save this clustermap refresh in the common case of no sync needed.
		//
		if clusterMapRefreshed {
			break
		}

		err := cm.RefreshClusterMap(0)
		if err != nil {
			log.Warn("ReplicationManager::ResyncSyncableMVs: could not refresh clustermap, skipping: %v",
				err)
			return
		}
		clusterMapRefreshed = true
	}

	log.Info("ReplicationManager::ResyncSyncableMVs: %d syncable MV(s) found (%d degraded): %+v",
		len(syncableMVs), len(cm.GetDegradedMVs()), syncableMVs)

	//
	// For each degraded MV, call syncMV() to synchronize all the outofsync RVs for that MV.
	// Each of those RV is synchronized using an independent sync job, which can fail/succeed independent
	// of other sync jobs. Hence we don't have a status for the syncMV(). If it fails, all we can do is
	// retry the resync next time around (if one or more RVs are still outofsync).
	//
	for mvName, mvInfo := range syncableMVs {
		common.Assert(mvInfo.State == dcache.StateDegraded, mvInfo.State)

		//
		// If we have more than maxSimulSyncJobs sync jobs currently running, don't start any more.
		// Any syncable MVs left out in this iteration will be synced next time resyncSyncableMVs is
		// called.
		// We can go a little over maxSimulSyncJobs, but that's ok.
		//
		if rm.numSyncJobs.Load() >= rm.maxSimulSyncJobs {
			log.Info("ReplicationManager::ResyncSyncableMVs: numSyncJobs (%d) >= maxSimulSyncJobs (%d), not syncing more MVs till some sync jobs complete",
				rm.numSyncJobs.Load(), rm.maxSimulSyncJobs)
			break
		}

		syncMV(mvName, mvInfo)
	}
}

// syncMV is used for resyncing the degraded MV to online state. To be precise it will synchronize all component
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

	componentRVs := cm.RVMapToList(mvName, mvInfo.RVs)

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
		// This is to prevent periodic calls to resyncSyncableMVs() from starting replication
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

		// Increment syncJobs count.
		rm.numSyncJobs.Add(1)

		go func() {
			// Decrement the wait group when the syncComponentRV() function completes.
			defer rm.wg.Done()

			// Decrement syncJobs count once the syncjob completes.
			defer rm.numSyncJobs.Add(-1)

			// Remove from the map, once the syncjob completes (success or failure).
			defer rm.runningJobs.Delete(tgtReplica)

			syncComponentRV(mvName, lioRV, rv.Name, syncSize, componentRVs)
			common.Assert(rm.numSyncJobs.Load() > 0, rm.numSyncJobs.Load())
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
	//
	// Wallclock time when this sync job is started.
	// This will be later set in syncJob once we create it, and used for finding the running duration
	// of the sync job.
	//
	startTime := time.Now()

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
	// TODO: (sourav) If StartSync fails it could be because the target RV is offline, in that case
	//       we should mark the state as inband-offline, else we might get stuck in a loop as StartSync
	//       will keep failing with NeedToRefreshClusterMap error.
	//       THIS IS IMPORTANT!
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
	// StartSync causes mvInfo state to be changed to "syncing" but server can purge it after GetMvInfoTimeout()
	// time if the state change is not committed in the clustermap. If we have spent more than that, we have to
	// abort the sync.
	//
	if time.Since(startTime) > rpc_server.GetMvInfoTimeout() {
		errStr := fmt.Sprintf("StartSync for %s/%s (%s, %s) took longer than %s, aborting sync",
			targetRVName, mvName, srcSyncId, dstSyncId, rpc_server.GetMvInfoTimeout())
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		common.Assert(false, errStr)
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

	common.Assert(time.Since(startTime) < rpc_server.GetMvInfoTimeout(),
		time.Since(startTime), rpc_server.GetMvInfoTimeout(),
		lioRV, targetRVName, mvName, srcSyncId, dstSyncId)

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
		startedAt:     startTime,
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
		// Note that we pass 0 for higherThanEpoch as we don't have any specific epoch to refresh
		// to, it's a best effort refresh.
		//
		// TODO: Check for NeedToRefreshClusterMap and only on that error, refresh the clustermap.
		//
		err1 := cm.RefreshClusterMap(0 /* higherThanEpoch */)
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

	// Tag the time when copy started.
	job.copyStartedAt = time.Now()

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
		err = fmt.Errorf("sendEndSyncRequest failed for job %s [%v]", job.toString(), err)
		log.Err("ReplicationManager::runSyncJob: %v", err)
		return err
	}

	// Call EndSync() RPC call to the target node which is hosting the target RV.
	destNodeID := getNodeIDFromRVName(job.destRVName)
	common.Assert(common.IsValidUUID(destNodeID))

	endSyncReq.SyncID = job.destSyncID
	err = sendEndSyncRequest(job.destRVName, destNodeID, endSyncReq)
	if err != nil {
		err = fmt.Errorf("sendEndSyncRequest failed for job %s [%v]", job.toString(), err)
		log.Err("ReplicationManager::runSyncJob: %v", err)

		rpcErr := rpc.GetRPCResponseError(err)
		if rpcErr == nil {
			//
			// This error means that the node is not reachable.
			//
			// We should now run the inband RV offline detection workflow, basically we
			// call the clustermap's UpdateComponentRVState() API to mark this
			// component RV as inband-offline and force the fix-mv workflow which will finally
			// trigger the resync-mv workflow.
			//
			log.Err("ReplicationManager::runSyncJob: Failed to reach node %s for job %s [%v]",
				destNodeID, job.toString(), err)

			errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateInbandOffline)
			if errRV != nil {
				errStr := fmt.Sprintf("Failed to mark %s/%s as inband-offline for job %s [%v]",
					job.destRVName, job.mvName, job.toString(), errRV)
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
				errStr := fmt.Sprintf("Failed to mark %s/%s as outofsync for job %s [%v]",
					job.destRVName, job.mvName, job.toString(), errRV)
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
		errStr := fmt.Sprintf("Failed to mark %s/%s as online for job %s [%v]",
			job.destRVName, job.mvName, job.toString(), err)
		log.Err("ReplicationManager::runSyncJob: %s", errStr)
		return err
	}

	log.Debug("ReplicationManager::runSyncJob: Sync job completed successfully: %s", job.toString())

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
		_ = putChunkResp
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
				// component RV as inband-offline and force the fix-mv workflow which will finally
				// trigger the resync-mv workflow.
				//
				log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to reach node %s [%v]",
					destNodeID, err)

				errRV := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateInbandOffline)
				if errRV != nil {
					errStr := fmt.Sprintf("Failed to mark %s/%s as inband-offline [%v]",
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
	_ = resp
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
	var lastClusterMapEpoch int64

	clusterMapRefreshed := false
	retryCnt := 0

retry:
	// Give up after sufficient clustermap refresh attempts.
	if retryCnt > 5 {
		err = fmt.Errorf("no suitable RV found for MV %s even after %d clustermap refresh retries, last epoch %d",
			mvName, retryCnt, lastClusterMapEpoch)
		log.Err("ReplicationManager::GetMVSize: %v", err)
		return 0, err
	}

	mvState, componentRVs, lastClusterMapEpoch := getComponentRVsForMV(mvName)

	log.Debug("ReplicationManager::GetMVSize: Component RVs for %s (%s) are %s (retryCnt: %d, clusterMapRefreshed: %v)",
		mvName, mvState, rpc.ComponentRVsToString(componentRVs), retryCnt, clusterMapRefreshed)

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
	// excludeRVs is the list of component RVs to omit, used when retrying after prev attempts to query
	// MV size from certain RV(s) failed. Those RVs are added to excludeRVs list.
	//
	var excludeRVs []string

	for {
		readerRV := getReaderRV(componentRVs, excludeRVs)

		if readerRV == nil {
			//
			// An MV once marked offline can never become online, so save the trip to clustermap.
			//
			if mvState == dcache.StateOffline {
				err = fmt.Errorf("%s is offline", mvName)
				log.Err("ReplicationManager::GetMVSize: %v", err)
				return 0, err
			}

			//
			// If the current clustermap does not have any suitable RV to query MV size from, we try clustermap
			// refresh just in case we have a stale clustermap. This is very unlikely and it would most
			// likely indicate that we have a “very stale” clustermap where all/most of the component RVs
			// have been replaced, or most of them are down.
			//
			// Even after refreshing clustermap if we cannot get a valid MV replica to query MV size,
			// alas we need to fail the GetMVSize().
			//
			if clusterMapRefreshed {
				err = fmt.Errorf("no suitable RV found for MV %s even after clustermap refresh to epoch %d",
					mvName, lastClusterMapEpoch)
				log.Err("ReplicationManager::GetMVSize: %v", err)
				return 0, err
			}

			err = cm.RefreshClusterMap(lastClusterMapEpoch)
			if err != nil {
				log.Warn("ReplicationManager::GetMVSize: RefreshClusterMap() failed for GetMVSize(%s) (retryCnt: %d): %v",
					mvName, retryCnt, err)
			} else {
				clusterMapRefreshed = true
			}

			retryCnt++
			goto retry
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

		var resp *models.GetMVSizeResponse
		var err error

		//
		// If the node to which the GetMVSize() RPC call must be made is local,
		// then we directly call the GetMVSize() method using the local server's handler.
		// Else we call the GetMVSize() RPC via the Thrift RPC client.
		//
		if targetNodeID == rpc.GetMyNodeUUID() {
			resp, err = rpc_server.GetMVSizeLocal(ctx, req)
		} else {
			resp, err = rpc_client.GetMVSize(ctx, targetNodeID, req)
		}

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

		log.Warn("ReplicationManager::GetMVSize: Failed to get MV size from node %s for request %v [%v]",
			targetNodeID, rpc.GetMVSizeRequestToString(req), err)

		rpcErr := rpc.GetRPCResponseError(err)
		if rpcErr != nil && rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
			//
			// RPC server can return models.ErrorCode_NeedToRefreshClusterMap in two cases:
			// 1. It genuinely wants the client to refresh the clustermap as it knows that
			//    the client has an older clustermap.
			// 2. It hit some transient error while fetching the clustermap itself, so it cannot
			//    be sure whether clustermap refresh at the client will help or not. To be safe
			//    we refresh the clustermap for a limited number of times before failing the read.
			//
			errCM := cm.RefreshClusterMap(lastClusterMapEpoch)
			if errCM != nil {
				// Log and retry, it'll help in case of transient errors at the server.
				log.Warn("ReplicationManager::GetMVSize: RefreshClusterMap() failed for GetMVSize(%s) (retryCnt: %d): %v",
					mvName, retryCnt, errCM)
			} else {
				clusterMapRefreshed = true
			}

			retryCnt++
			goto retry
		}

		// Try another replica if available.
	}

	return mvSize, nil
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
