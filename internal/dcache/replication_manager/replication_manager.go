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
	"errors"
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
	gouuid "github.com/google/uuid"
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
	// TODO: make it more resilient. We should never fail client IO.
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
			// TODO: See if refreshing clustermap really gets us some benefit.
			//
			if clusterMapRefreshed {
				err = fmt.Errorf("no suitable RV found for MV %s even after clustermap refresh to epoch %d",
					req.MvName, lastClusterMapEpoch)
				log.Err("ReplicationManager::ReadMV: %v", err)
				return nil, err
			}

			err = cm.RefreshClusterMap(-lastClusterMapEpoch)
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
			OffsetInChunk:   req.OffsetInChunk,
			Length:          req.Length,
			ComponentRV:     componentRVs,
			ClustermapEpoch: lastClusterMapEpoch,
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
			// TODO: Pass resp.ClustermapEpoch from server to client for targeted refresh.
			//
			errCM := cm.RefreshClusterMap(-lastClusterMapEpoch)
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

	var err error

	//
	// If PutChunk fails with NeedToRefreshClusterMap more than once, it most likely is due to clustermap
	// being stuck in "updating" state (odd epoch number) as the node responsible for updating the clustermap
	// is either stuck or down. Note that PutChunk fails with NeedToRefreshClusterMap when an offline MV
	// replica is replaced with an outofsync RV and the clustermap epoch is odd i.e., it's in transition.
	// Since a new leader may take upto 6 minutes, we need to set the write temout sufficiently high.
	//
	writeStartTime := time.Now()
	writeTimeout := 900 * time.Second

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
	err = nil
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
	// Note: getComponentRVsForMV() returns a randomized list of component RVs. This helps to distribute
	//       load in case of daisy chain writes as daisy chain writes utilize ingress and egress n/w b/w
	//       for all but the last RV in the chain and for the last RV only ingress n/w b/w is used.
	//
	mvState, componentRVs, lastClusterMapEpoch := getComponentRVsForMV(req.MvName)

	log.Debug("ReplicationManager::writeMVInternal: %s (%s), componentRVs: %v, chunkIdx: %d, cepoch: %d",
		req.MvName, mvState, rpc.ComponentRVsToString(componentRVs), req.ChunkIndex, lastClusterMapEpoch)

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
			Length:          int64(len(req.Data)),
			SyncID:          "", // this is regular client write
			ComponentRV:     componentRVs,
			MaybeOverwrite:  retryCnt > 0 || brokenChain,
			ClustermapEpoch: lastClusterMapEpoch,
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
		// Omit writing to RVs in “offline” or "inband-offline" state. It’s ok to omit them as the chunks
		// not written to them will be copied to them when the mv is (soon) fixed+resynced.
		// RVs in "outofsync" state are good RVs and we must write chunks to them. Whatever chunks were
		// not written to these RVs from the time the (bad) RV(s) went offline, till we got a replacement
		// RV will be copied by the resync process.
		// RVs in "syncing" and "online" must obviously be written to.
		//
		if rv.State == string(dcache.StateOffline) ||
			rv.State == string(dcache.StateInbandOffline) {
			log.Debug("ReplicationManager::writeMVInternal: Skipping %s/%s (RV state: %s, MV state: %s), chunkIdx: %d, cepoch: %d",
				rv.Name, req.MvName, rv.State, mvState, req.ChunkIndex, lastClusterMapEpoch)

			// Online MV must have all replicas online.
			common.Assert(mvState != dcache.StateOnline, req.MvName, rv.Name, rv.State)

			//
			// Skip writing to this RV, as it is in offline or outofsync state.
			// So, send nil response to the response channel to indicate that
			// we are not writing to this RV.
			//
			common.Assert(len(responseChannel) < len(componentRVs),
				len(responseChannel), len(componentRVs))
			responseChannel <- nil
		} else if rv.State == string(dcache.StateOnline) ||
			rv.State == string(dcache.StateSyncing) ||
			rv.State == string(dcache.StateOutOfSync) {
			// Offline MV has all replicas offline.
			common.Assert(mvState != dcache.StateOffline, req.MvName)

			rvID := getRvIDFromRvName(rv.Name)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rv.Name)
			common.Assert(common.IsValidUUID(targetNodeID))

			log.Debug("ReplicationManager::writeMVInternal: Writing to %s/%s (rvID: %s, state: %s) on node %s, chunkIdx: %d, cepoch: %d",
				rv.Name, req.MvName, rvID, rv.State, targetNodeID, req.ChunkIndex, lastClusterMapEpoch)

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
		log.Err("ReplicationManager::writeMVInternal: Could not write to any component RV, req: %s, component RVs: %s, chunkIdx: %d, cepoch: %d",
			req.toString(), rpc.ComponentRVsToString(componentRVs), req.ChunkIndex, lastClusterMapEpoch)
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

		log.Debug("ReplicationManager::writeMVInternal: Sending PutChunk [%s] request for %s/%s to node %s: %s, chunkIdx: %d, cepoch: %d",
			putChunkStyle, rvName, req.MvName, targetNodeID, rpc.PutChunkRequestToString(putChunkDCReq.Request),
			req.ChunkIndex, lastClusterMapEpoch)

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
				Length:          int64(len(req.Data)),
				SyncID:          "", // this is regular client write
				ComponentRV:     componentRVs,
				MaybeOverwrite:  retryCnt > 0 || brokenChain,
				ClustermapEpoch: lastClusterMapEpoch,
			}

			log.Debug("ReplicationManager::writeMVInternal: Sending PutChunk request for %s/%s to node %s: %s, chunkIdx: %d, cepoch: %d",
				rvName, req.MvName, targetNodeID, rpc.PutChunkRequestToString(putChunkReq),
				req.ChunkIndex, lastClusterMapEpoch)

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
		// TODO: This is O(number of RVs), make this O(1).
		rvName := getRvNameFromRvID(putChunkDCReq.Request.Chunk.Address.RvID)
		targetNodeID := getNodeIDFromRVName(rvName)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::writeMVInternal: Sending PutChunkDC request for nexthop %s/%s to node %s: %s, chunkIdx: %d, cepoch: %d",
			rvName, req.MvName, targetNodeID, rpc.PutChunkDCRequestToString(putChunkDCReq),
			req.ChunkIndex, lastClusterMapEpoch)

		//
		// Check if next-hop RV and any RV in chain are present in the iffy RV map.
		// If yes, we retry the operation using OriginatorSendsToAll and save a potential PutChunkDC timeout.
		//
		// This check for skipping the DaisyChain write is done in the rpc_client.PutChunkDC() call
		// also, where we just check if the next-hop RV is present in the iffy RV map.
		// Whereas here, we are also checking the next RVs in the chain if they are present in the iffy RV map.
		// If one of the next RVs in chain is present in the iffy RV map, whereas the next-hop
		// RV is not, then the check present in the PutChunkDC() will allow the RPC call to go through which will
		// eventually timeout. So, adding the check for all the RVs here prevents an additional timeout
		// error from occurring.
		//
		iffyRVs := rpc_client.GetIffyRVs(&rvName, &putChunkDCReq.NextRVs)
		if iffyRVs != nil && len(*iffyRVs) > 0 {
			err := fmt.Errorf("%d iffy RVs %v found for MV %s (next-hop RV: %s), retrying with OriginatorSendsToAll",
				len(*iffyRVs), *iffyRVs, req.MvName, rvName)
			log.Err("ReplicationManager::writeMVInternal: %v", err)
			return nil, rpc.NewResponseError(models.ErrorCode_BrokenChain, err.Error())
		}

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		var putChunkDCResp *models.PutChunkDCResponse

		//
		// If the node to which the PutChunkDC() RPC call must be made is local,
		// then we directly call the PutChunkDC() method using the local server's handler.
		// Else we call the PutChunkDC() RPC via the Thrift RPC client.
		//
		if targetNodeID == rpc.GetMyNodeUUID() {
			putChunkDCResp, err = rpc_server.PutChunkDCLocal(ctx, putChunkDCReq)
		} else {
			putChunkDCResp, err = rpc_client.PutChunkDC(ctx, targetNodeID, putChunkDCReq, false /* fromFwder */)
		}

		if err != nil {
			log.Err("ReplicationManager::writeMVInternal: Failed to send PutChunkDC request for nexthop %s/%s to node %s, chunkIdx: %d, cepoch: %d: %v",
				rvName, req.MvName, targetNodeID, req.ChunkIndex, lastClusterMapEpoch, err)
			common.Assert(putChunkDCResp == nil)

			//
			// If the node containing the RV is marked negative or if the RV is marked iffy,
			// it means that either it is down or some other downstream connection issue is preventing
			// the PutChunkDC call to succeed.
			// So, if the node/RV is marked negative/iffy, the RPC client will fail the PutChunkDC() call
			// to prevent the timeout error from happening again. In this case, we will retry the WriteMV()
			// operation with OriginatorSendsToAll mode.
			//
			// We check for NegativeNodeError or IffyRVError here, though we have checked it before making
			// the PutChunkDC call. This is because while a thread is waiting for getting an RPC client,
			// some other thread may have marked the node as negative or the next-hop RV as iffy. So, we
			// directly return error after getting the client and before making the PutChunkDC() call.
			// This prevents making additional PutChunkDC() calls which will eventually timeout and also
			// indicate the caller (WriteMV) to retry the operation using OriginatorSendsToAll.
			//
			if errors.Is(err, rpc_client.NegativeNodeError) || errors.Is(err, rpc_client.IffyRVError) {
				log.Warn("ReplicationManager::writeMVInternal: %s/%s is marked negative/iffy [%v], retrying with OriginatorSendsToAll, chunkIdx: %d, cepoch: %d",
					rvName, req.MvName, err, req.ChunkIndex, lastClusterMapEpoch)
				return nil, rpc.NewResponseError(models.ErrorCode_BrokenChain, err.Error())
			}

			//
			// PutChunkDC() call to the RV failed. This indicates that the request was not forwarded to the
			// next RVs. So, convert this error to ThriftError for this RV and add BrokenChain error for the
			// next RVs, and store it in the putChunkDCResp.
			//
			putChunkDCResp = rpc.HandlePutChunkDCError(rvName, putChunkDCReq.NextRVs, req.MvName, err)
		} else {
			log.Debug("ReplicationManager::writeMVInternal: Received PutChunkDC response from nexthop %s/%s node %s, chunkIdx: %d, cepoch: %d: %s",
				rvName, req.MvName, targetNodeID, req.ChunkIndex, lastClusterMapEpoch,
				rpc.PutChunkDCResponseToString(putChunkDCResp))
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

			log.Debug("ReplicationManager::writeMVInternal: PutChunk successful for %s/%s, chunkIdx: %d, cepoch: %d, RPC response: %s",
				respItem.rvName, req.MvName, req.ChunkIndex, lastClusterMapEpoch, rpc.PutChunkResponseToString(putChunkResp))

			//
			// Write to this component RV was successful, add it to the list of RVs successfully written
			// in this attempt.
			//
			rvsWritten = append(rvsWritten, respItem.rvName)
			common.Assert(len(rvsWritten) <= len(componentRVs), len(rvsWritten), len(componentRVs))

			continue
		}

		log.Err("ReplicationManager::writeMVInternal: [%v] PutChunk to %s/%s failed, chunkIdx: %d, cepoch: %d [%v]",
			putChunkStyle, respItem.rvName, req.MvName, req.ChunkIndex, lastClusterMapEpoch, respItem.err)

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
			// mark the RV as inband-offline. Instead we retry the WriteMV with OriginatorSendsToAll mode
			// which will mark the RV as inband-offline if the PutChunk to that RV fails again.
			//
			if putChunkStyle != DaisyChain {
				errRV, errChan := cm.UpdateComponentRVState(req.MvName, respItem.rvName, dcache.StateInbandOffline, true /* isBlocking */)
				common.Assert(errChan == nil)

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
			if time.Since(writeStartTime) > writeTimeout {
				errWriteMV = fmt.Errorf("failed to write to %s/%s even after refreshing clustermap %d times, for %s [%v]",
					respItem.rvName, req.MvName, retryCnt, time.Since(writeStartTime), respItem.err)
				log.Err("ReplicationManager::writeMVInternal: %v", errWriteMV)
				continue
			}

			if clusterMapRefreshed {
				//
				// Clustermap has already been refreshed once in this try, so skip it.
				// We wait a little before retrying to allow the clustermap update to
				// complete. Beyond 5 seconds the chances that the clustermap epoch is
				// stuck due to a node failure increases, so we wait longer.
				//
				if time.Since(writeStartTime) < 5*time.Second {
					time.Sleep(100 * time.Millisecond)
				} else {
					time.Sleep(5 * time.Second)
				}

				//
				// We will be asked to refresh more than once only if the clustermap is being updated.
				// In this state, refreshFromClustermap() cannot safely override mvInfo from the clustermap,
				// so it keeps asking the client to retry.
				//
				common.Assert(retryCnt < 2 || lastClusterMapEpoch%2 == 1,
					respItem.rvName, req.MvName, retryCnt, lastClusterMapEpoch)
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
			// TODO: Pass resp.ClustermapEpoch from server to client for targeted refresh.
			//
			errCM := cm.RefreshClusterMap(-lastClusterMapEpoch)
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
				log.Warn("ReplicationManager::writeMVInternal: PutChunkDC to %s/%s failed with non-retriable error [%v], will retry with OriginatorSendsToAll",
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
		err = fmt.Errorf("ReplicationManager::writeMVInternal: Failed to write to MV %s, %s, chunkIdx: %d, cepoch: %d [%v]",
			req.MvName, req.toString(), req.ChunkIndex, lastClusterMapEpoch, errWriteMV)
		log.Err("%v", err)
		return nil, err
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
		err = fmt.Errorf("%s is offline", req.MvName)
		log.Err("ReplicationManager::writeMVInternal: %v", err)
		return nil, err
	}

	// For a non-offline MV, at least one replica write should succeed.
	if len(rvsWritten) == 0 {
		err = fmt.Errorf("WriteMV could not write to any replica: %v", req.toString())
		log.Err("ReplicationManager::writeMVInternal: %v", err)
		common.Assert(false, err)
		return nil, err
	}

	common.Assert(err == nil, err)
	return &WriteMvResponse{}, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	common.Assert(req != nil)

	var err error
	var resp *WriteMvResponse

	if common.IsDebugBuild() {
		startTime := time.Now()
		defer func() {
			if err != nil {
				log.Err("[TIMING] ReplicationManager::WriteMV: WriteMV failed after %s: %v: %v",
					time.Since(startTime), req.toString(), err)
			} else {
				log.Debug("[TIMING] ReplicationManager::WriteMV: WriteMV request took %s: %v",
					time.Since(startTime), req.toString())
			}
		}()
	}

	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: %v", req.toString())

	//
	// We don't expect the caller to pass invalid requests, so only verify in debug builds.
	//
	if common.IsDebugBuild() {
		if err = req.isValid(); err != nil {
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
	resp, err = writeMVInternal(req, DaisyChain)
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
	mvState, rvs, lastClusterMapEpoch := getComponentRVsForMV(req.MvName)
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
			ComponentRV:     rvs,
			ClustermapEpoch: lastClusterMapEpoch,
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
			abortStuckSyncJobs()
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
	var lastClusterMapEpoch int64
	clusterMapRefreshed := false

	for {
		// syncable MVs are degraded MVs which have at least one component RV in outofsync state.
		syncableMVs, lastClusterMapEpoch = cm.GetSyncableMVs()
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

		// mvInfo corresponds to lastClusterMapEpoch.
		syncMV(mvName, mvInfo, lastClusterMapEpoch)
	}
}

// If the node running a sync job dies, the target RV will be stuck in syncing state.
// We need to mark it offline again to restart the sync process for it.
// This function goes over all RVs hosted by this node, which are in syncing state and are not making
// progress, based on the mvInfo's lastSyncWriteTime. If the lastSyncWriteTime is older than a threshold
// it'll mark the RV as inband-offline, which will trigger the fix-mv workflow to select a new RV.
func abortStuckSyncJobs() {
	myRVs := cm.GetMyRVs()
	common.Assert(len(myRVs) > 0, myRVs)

	//
	// Store the responses for all RV/MV state update for which we attempted to abort the sync job
	// by marking the RV as inband-offline.
	//
	var updateResponses []*updateRVStateResponse

	for rvName, rvInfo := range myRVs {
		_ = rvInfo
		common.Assert(cm.IsValidRVName(rvName), rvName)
		// We are up, so our RVs must be online in the clustermap.
		common.Assert(rvInfo.State == dcache.StateOnline, rvName, rvInfo.State)

		//
		// Active MVs are those MVs for which this RV is a component RV and is not in offline/inband-offline state.
		// We still need to check if any of these active MVs have this RV in syncing state, and only those are
		// valid candidates for aborting the sync job, but only if the sync job is stuck.
		//
		myMVs := cm.GetActiveMVsForRV(rvName)
		for mvName, _ := range myMVs {
			errChan := checkAndAbortSyncJob(rvName, mvName)

			//
			// If errChan is not nil, it means that we attempted to abort the sync job for this RV/MV
			// by updating the RV state to inband-offline. We make non-blocking call to UpdateComponentRVState()
			// and get an error channel on which we can wait for the result later.
			//
			if errChan != nil {
				updateResponses = append(updateResponses, &updateRVStateResponse{
					mvName:  mvName,
					rvName:  rvName,
					errChan: errChan,
				})
			}
		}
	}

	//
	// Wait for all the UpdateComponentRVState() calls to complete and log the result.
	//
	for _, updateResp := range updateResponses {
		err := <-updateResp.errChan
		if err != nil {
			log.Err("ReplicationManager::abortStuckSyncJobs: Failed to mark %s/%s inband-offline: %v",
				updateResp.rvName, updateResp.mvName, err)
		} else {
			log.Info("ReplicationManager::abortStuckSyncJobs: Marked %s/%s inband-offline",
				updateResp.rvName, updateResp.mvName)
		}
	}
}

func checkAndAbortSyncJob(rvName, mvName string) <-chan error {
	common.Assert(cm.IsValidRVName(rvName), rvName)
	common.Assert(cm.IsValidMVName(mvName), mvName)

	_, componentRVs, epoch := cm.GetRVsEx(mvName)
	_ = epoch
	common.Assert(len(componentRVs) == int(getNumReplicas()),
		mvName, componentRVs, getNumReplicas())

	rvState, ok := componentRVs[rvName]
	_ = ok

	//
	// This unlikely but can happen if the local clustermap changes after the call to cm.GetActiveMVsForRV()
	// by the caller and before this call to GetRVsEx() and rvName is no longer a componentRV for the MV.
	// In this case we simply return.
	//
	if !ok {
		// Assert, since it's uncommon.
		common.Assert(false, rvName, mvName, componentRVs, epoch)
		return nil
	}

	common.Assert(cm.IsValidComponentRVState(rvState), rvName, mvName, rvState)

	// This component RV is not the target of a sync job.
	if rvState != dcache.StateSyncing {
		return nil
	}

	log.Debug("ReplicationManager::checkAndAbortSyncJob: Checking if sync job for %s/%s needs to be aborted, state: %s",
		rvName, mvName, rvState)

	// Get the joinMV and last sync write time for this RV/MV.
	joinMVTime, lastSyncWriteTime := rpc_server.GetMVJoinAndLastSyncWriteTime(rvName, mvName)

	// If RV state is syncing, joinMVTime must be > 0.
	common.Assert(joinMVTime > 0, rvName, mvName, joinMVTime)

	//
	// Must not be stuck in JoinMV state for more than AbortSyncAfterJoinMVThresholdSecs, use a factor of
	// 2 for some leeway. We must have aborted it.
	//
	common.Assert(lastSyncWriteTime > 0 || time.Now().Unix()-joinMVTime < 2*AbortSyncAfterJoinMVThresholdSecs,
		rvName, mvName, joinMVTime)
	// lastSyncWriteTime can be 0 if the sync job hasn't yet written any chunks to the target RV.
	common.Assert(lastSyncWriteTime >= 0, rvName, mvName, lastSyncWriteTime)
	// If lastSyncWriteTime is non-zero, it must not be less than joinMVTime.
	common.Assert(lastSyncWriteTime == 0 || lastSyncWriteTime >= joinMVTime,
		rvName, mvName, joinMVTime, lastSyncWriteTime)
	//
	// Must not be stuck in syncing state for more than AbortOngoingSyncThresholdSecs, use a factor of
	// 2 for some leeway. We must have aborted it.
	//
	common.Assert(lastSyncWriteTime == 0 || time.Now().Unix()-lastSyncWriteTime < 2*AbortOngoingSyncThresholdSecs,
		rvName, mvName, joinMVTime, lastSyncWriteTime)

	//
	// If lastSyncWriteTime is 0, it means that the sync job hasn't yet started writing any chunks to
	// the target RV. This can be the case where the source RV marked the target RV as syncing in the clustermap,
	// but before it could send the PutChunk(sync) RPC calls, it went down.
	// In this case, we check if the time after the target RV joined the MV becomes greater than
	// AbortSyncAfterJoinMVThresholdSecs, we mark the target RV as inband-offline, which will trigger the
	// fix-mv workflow to select a new RV.
	//
	if lastSyncWriteTime == 0 && time.Now().Unix()-joinMVTime > AbortSyncAfterJoinMVThresholdSecs {
		log.Warn("ReplicationManager::checkAndAbortSyncJob: %s/%s stuck in syncing state since %d secs after JoinMV, marking it inband-offline",
			rvName, mvName, time.Now().Unix()-joinMVTime)

		err, errChan := cm.UpdateComponentRVState(mvName, rvName, dcache.StateInbandOffline, false /* isBlocking */)
		common.Assert(err == nil, rvName, mvName, err)
		common.Assert(errChan != nil, rvName, mvName)

		return errChan
	}

	//
	// If the last sync write time is older than AbortOngoingSyncThresholdSecs, we mark this RV as inband-offline.
	// This will trigger the fix-mv workflow to select a new RV and mark it outofsync, which will
	// be synced next time around by resyncSyncableMVs().
	//
	if lastSyncWriteTime != 0 && time.Now().Unix()-lastSyncWriteTime > AbortOngoingSyncThresholdSecs {
		log.Warn("ReplicationManager::checkAndAbortSyncJob: %s/%s stuck in syncing state since %d secs, marking it inband-offline",
			rvName, mvName, time.Now().Unix()-lastSyncWriteTime)

		err, errChan := cm.UpdateComponentRVState(mvName, rvName, dcache.StateInbandOffline, false /* isBlocking */)
		common.Assert(err == nil, rvName, mvName, err)
		common.Assert(errChan != nil, rvName, mvName)

		return errChan
	}

	return nil
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
// Note: mvInfo corresponds to lastClusterMapEpoch.

func syncMV(mvName string, mvInfo dcache.MirroredVolume, lastClusterMapEpoch int64) {
	log.Debug("ReplicationManager::syncMV: Resyncing MV %s %+v, lastClusterMapEpoch: %d",
		mvName, mvInfo, lastClusterMapEpoch)

	common.Assert(mvInfo.State == dcache.StateDegraded, mvName, mvInfo.State)

	lioRV := cm.LowestIndexOnlineRV(mvInfo)
	// For a degraded MV, we must have at least one online component RV.
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

	// componentRVs is derived from mvInfo.RVs which corresponds to lastClusterMapEpoch.
	componentRVs := cm.RVMapToList(mvName, mvInfo.RVs)

	log.Debug("ReplicationManager::syncMV: Component RVs for MV %s are %v",
		mvName, rpc.ComponentRVsToString(componentRVs))

	//
	// Fetch the current disk usage of this MV. We convey this via StartSync, it can be used to check
	// %age progress. Note that JoinMV carries the reservedSpace parameter which is the more critical one
	// to decide if an RV can host a new MV replica or not.
	//
	syncSize, err := GetMVSize(mvName, componentRVs, lastClusterMapEpoch)
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

			syncComponentRV(mvName, lioRV, rv.Name, syncSize, componentRVs, lastClusterMapEpoch)
			common.Assert(rm.numSyncJobs.Load() > 0, rm.numSyncJobs.Load())
		}()
	}
}

// syncComponentRV is used for syncing the target RV from the lowest index online RV (or source RV).
// It sets the target RV state to "syncing" in the global clustermap and then starts a sync job that
// copies all chunks that were written to the MV before this point, from the source RV to the target RV.
// When the first PutChunk(sync) call reaches the server, the server will note that the target RV is not
// in syncing state, so it'll refresh its mvInfo from the clustermap. Since we have set the target RV state
// as syncing, the server will now accept the PutChunk(sync) calls.
func syncComponentRV(mvName string, lioRV string, targetRVName string, syncSize int64,
	componentRVs []*models.RVNameAndState, lastClusterMapEpoch int64) {
	//
	// Wallclock time when this sync job is started.
	// This will be later set in syncJob once we create it, and used for finding the running duration
	// of the sync job.
	//
	startTime := time.Now()

	sourceNodeID := getNodeIDFromRVName(lioRV)
	common.Assert(common.IsValidUUID(sourceNodeID))
	_ = sourceNodeID

	targetNodeID := getNodeIDFromRVName(targetRVName)
	common.Assert(common.IsValidUUID(targetNodeID))
	_ = targetNodeID

	log.Debug("ReplicationManager::syncComponentRV: %s/%s -> %s/%s [%s -> %s], sync size %d bytes, component RVs %v, cepoch: %d",
		lioRV, mvName, targetRVName, mvName, sourceNodeID, targetNodeID, syncSize,
		rpc.ComponentRVsToString(componentRVs), lastClusterMapEpoch)

	common.Assert(lioRV != targetRVName, lioRV, targetRVName)
	common.Assert(syncSize >= 0, syncSize)

	//
	// Update the destination RV from outofsync to syncing state. The cluster manager will take care of
	// updating the MV state to syncing if all component RVs have either online or syncing state.
	//
	err, errChan := cm.UpdateComponentRVState(mvName, targetRVName, dcache.StateSyncing, true /* isBlocking */)
	common.Assert(errChan == nil)

	if err != nil {
		errStr := fmt.Sprintf("Failed to update component RV %s/%s state to syncing [%v]",
			targetRVName, mvName, err)
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return
	}

	// UpdateComponentRVState() must result in a clustermap update.
	common.Assert(cm.GetEpoch() > lastClusterMapEpoch, cm.GetEpoch(), lastClusterMapEpoch)

	//
	// Update the state of target RV from outofsync to syncing in local component RVs list, to match the
	// global clustermap state.
	// This updated component RVs list will be later used in the PutChunk(sync) RPC calls to the target RV,
	// hence the state must match the global clustermap state, else server will reject the PutChunk(sync).
	//
	updateLocalComponentRVState(componentRVs, targetRVName, dcache.StateOutOfSync, dcache.StateSyncing)

	//
	// WriteMV() would be writing client writes to the target RV after it was joined to the MV (as outofsync).
	// Now that the sync job is starting, we will be syncing all chunks written to the MV before this point
	// (with a clock skew margin as well), so we might end up copying (much) more chunks than needed, but it's
	// ok to be careful.
	//
	// TODO: See if the chunks copied is very high for actively written MVs. If yes, we may want to reduce
	//       syncStartTime to when the RV was marked outofsync and not when sync job started.
	//
	syncStartTime := time.Now().UnixMicro() + NTPClockSkewMargin

	syncJob := &syncJob{
		mvName:          mvName,
		srcRVName:       lioRV,
		destRVName:      targetRVName,
		syncSize:        syncSize,
		componentRVs:    componentRVs,
		syncStartTime:   syncStartTime,
		startedAt:       startTime,
		clustermapEpoch: cm.GetEpoch(), // componentRVs corresponds to this epoch.
		syncID:          gouuid.New().String(),
	}

	log.Debug("ReplicationManager::syncComponentRV: Sync job created: %s", syncJob.toString())

	//
	// Copy all chunks from source to target replica.
	//
	err = runSyncJob(syncJob)
	if err != nil {
		errStr := fmt.Sprintf("Failed to run sync job %s [%v]", syncJob.toString(), err)
		log.Err("ReplicationManager::syncComponentRV: %s", errStr)
		return
	}
}

// This method runs one sync job that synchronizes one MV replica.
// It copies all chunks from the source replica to the target replica.
// If all chunks are copied successfully, it updates the target RV state to online, else if any chunk
// fails it marks the target RV as inband-offline, so that the fix-mv workflow can select a new RV for it
// and the resync can be reattempted.
func runSyncJob(job *syncJob) error {
	log.Debug("ReplicationManager::runSyncJob: Sync job: %s, cepoch: %d", job.toString(), job.clustermapEpoch)

	common.Assert(job.clustermapEpoch > 0, job.clustermapEpoch)
	common.Assert((job.srcRVName != job.destRVName) &&
		cm.IsValidRVName(job.srcRVName) &&
		cm.IsValidRVName(job.destRVName), job.srcRVName, job.destRVName)

	// Tag the time when copy started.
	job.copyStartedAt = time.Now()

	err := copyOutOfSyncChunks(job)
	if err != nil {
		err = fmt.Errorf("failed to copy out of sync chunks for job %s [%v]", job.toString(), err)
		log.Err("ReplicationManager::runSyncJob: %v", err)

		//
		// Sync failed, mark the target RV as inband-offline, to reattempt sync with a fresh target RV.
		// If this fails, abortStuckSyncJobs() will redo this.
		//
		errRV, errChan := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateInbandOffline, true /* isBlocking */)
		common.Assert(errChan == nil)

		if errRV != nil {
			errStr := fmt.Sprintf("Failed to mark %s/%s as inband-offline for job %s [%v]",
				job.destRVName, job.mvName, job.toString(), errRV)
			log.Err("ReplicationManager::runSyncJob: %s", errStr)
		}
		return err
	}

	//
	// Now that we have successfully copied all chunks from source to target replica, update the
	// destination RV from syncing to online state. The cluster manager will take care of
	// updating the MV state to online if all component RVs have online state.
	// If this fails, abortStuckSyncJobs() will mark this inband-offline which will restart the
	// entire fix-mv+resync-mv workflows.
	//
	err, errChan := cm.UpdateComponentRVState(job.mvName, job.destRVName, dcache.StateOnline, true /* isBlocking */)
	common.Assert(errChan == nil)

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
	// TODO: os.ReadDir() will return all enumerated chunks. For really large number of chunk, consider
	//       using getdents() kind of streaming API.
	//
	entries, err := os.ReadDir(sourceMVPath)
	if err != nil {
		log.Err("ReplicationManager::copyOutOfSyncChunks: os.ReadDir(%s) failed: [%v]",
			sourceMVPath, err)
		return err
	}

	chunksCopied := 0
	bytesCopied := int64(0)

	// TODO: make this parallel
	for _, entry := range entries {
		if entry.IsDir() {
			log.Warn("ReplicationManager::copyOutOfSyncChunks: Skipping directory %s/%s",
				sourceMVPath, entry.Name())
			// We don't expect dirs in our MV replicas.
			common.Assert(false, entry.Name(), sourceMVPath)
			continue
		}

		//
		// chunks are stored in MV as,
		// <MvName>/<FileID>.<OffsetInMiB>.data and
		// <MvName>/<FileID>.<OffsetInMiB>.hash
		// <MvName>/<FileID>.<OffsetInMiB>.data.tmp (temporary file created during safeWrite())
		//
		chunkParts := strings.Split(entry.Name(), ".")
		if len(chunkParts) != 3 {
			// This is most likely the temp chunk file created by safeWrite().
			if len(chunkParts) == 4 && chunkParts[3] == "tmp" {
				log.Debug("ReplicationManager::copyOutOfSyncChunks: Skipping temp chunk file %s/%s",
					sourceMVPath, entry.Name())
			} else {
				// TODO: should we return error in this case?
				errStr := fmt.Sprintf("Invalid chunk name %s/%s", sourceMVPath, entry.Name())
				log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
				common.Assert(false, errStr)
			}
			continue
		}

		// TODO: hash validation will be done later
		// if file type is hash, skip it
		// the hash data will be transferred with the regular chunk file
		if chunkParts[2] == "hash" {
			log.Debug("ReplicationManager::copyOutOfSyncChunks: Skipping hash file %s", entry.Name())
			continue
		}

		//
		// Info() does a stat() syscall to fetch the file info, so we do it after we have performed
		// name based exclusion.
		//
		// Note: This can fail for chunks which are being removed (corresponding to a deleted file),
		//       so if ReadDir() above finds a chunk and it's removed by the time we come here, the
		//       assert below will fail. Let's leave it for some time and later we can remove it.
		//
		info, err := entry.Info()
		if err != nil {
			log.Err("ReplicationManager::copyOutOfSyncChunks: entry.Info() failed for %s/%s: %v",
				sourceMVPath, entry.Name(), err)
			common.Assert(false, err, sourceMVPath, entry.Name())
			continue
		}

		// We don't expect any chunk to have mod time before 2025-01-01.
		common.Assert(info.ModTime().Unix() > 1735689600, info.ModTime().Unix(), info.ModTime().String())
		common.Assert(job.syncStartTime > 1735689600000000, job.syncStartTime)

		if info.ModTime().UnixMicro() > job.syncStartTime {
			// This chunk is created after the sync start time, so it will be written to both source and target
			// RVs by the client PutChunk() RPC calls, so we can skip it here.
			log.Debug("ReplicationManager::copyOutOfSyncChunks: Skipping chunk %s/%s, "+
				"Mtime (%d) > syncStartTime (%d) [%d usecs after sync start]",
				sourceMVPath, entry.Name(), info.ModTime().UnixMicro(), job.syncStartTime,
				info.ModTime().UnixMicro()-job.syncStartTime)
			continue
		}

		log.Debug("ReplicationManager::copyOutOfSyncChunks: Copying chunk %s/%s, Mtime (%d) <= syncStartTime (%d) [%d usecs before sync start]",
			sourceMVPath, entry.Name(), info.ModTime().UnixMicro(), job.syncStartTime,
			job.syncStartTime-info.ModTime().UnixMicro())

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
			// SyncID is used for logging and debugging, to easily match client and server side logs.
			SyncID:          job.syncID,
			SourceRVName:    job.srcRVName,
			ComponentRV:     job.componentRVs,
			ClustermapEpoch: job.clustermapEpoch,
		}

		retryCnt := 0
		for {
			log.Debug("ReplicationManager::copyOutOfSyncChunks: [%d] Copying chunk %s (%s/%s -> %s/%s): %v",
				retryCnt, srcChunkPath, job.srcRVName, job.mvName, job.destRVName, job.mvName,
				rpc.PutChunkRequestToString(putChunkReq))

			ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
			defer cancel()

			putChunkResp, err := rpc_client.PutChunk(ctx, destNodeID, putChunkReq, false /* fromFwder */)
			_ = putChunkResp

			// Common case.
			if err == nil {
				common.Assert(putChunkResp != nil)
				log.Debug("ReplicationManager::copyOutOfSyncChunks: Copied chunk %s (%s/%s -> %s/%s): %v",
					srcChunkPath, job.srcRVName, job.mvName, job.destRVName, job.mvName,
					rpc.PutChunkResponseToString(putChunkResp))
				break
			}

			log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to copy chunk %s (%s/%s -> %s/%s) %v: %v",
				srcChunkPath, job.srcRVName, job.mvName, job.destRVName, job.mvName,
				rpc.PutChunkRequestToString(putChunkReq), err)

			rpcErr := rpc.GetRPCResponseError(err)
			if rpcErr == nil || rpcErr.GetCode() == models.ErrorCode_ThriftError {
				//
				// This error means that the node is not reachable.
				// Mark the destination RV as inband-offline, so that the fix-mv workflow can select a new RV
				// and the resync can be reattempted.
				//
				log.Err("ReplicationManager::copyOutOfSyncChunks: Failed to reach node %s [%v]",
					destNodeID, err)

				// Fall through and return error, caller will mark job.destRVName as inband-offline.
			} else if rpcErr.GetCode() == models.ErrorCode_NeedToRefreshClusterMap {
				//
				// NeedToRefreshClusterMap is the only error on which we retry the PutChunk, but only if
				// the new clustermap still has the same source and destination RVs, in online and syncing
				// state respectively. Note that the sync job is responsible for sync'ing one MV replica,
				// so all we care about is that the source and destination RVs have not changed.
				//
				errCM := cm.RefreshClusterMap(-putChunkReq.ClustermapEpoch)
				if errCM == nil {
					mvState, rvs, epoch := cm.GetRVsEx(job.mvName)
					srcRVState, srcRVok := rvs[job.srcRVName]
					dstRVState, dstRVok := rvs[job.destRVName]

					if srcRVok && dstRVok && srcRVState == dcache.StateOnline &&
						dstRVState == dcache.StateSyncing {
						job.componentRVs = cm.RVMapToList(job.mvName, rvs)
						job.clustermapEpoch = epoch

						putChunkReq.ComponentRV = job.componentRVs
						putChunkReq.ClustermapEpoch = job.clustermapEpoch

						retryCnt++

						if retryCnt < 5 {
							log.Debug("ReplicationManager::copyOutOfSyncChunks: Retrying copy of chunk %s (%s/%s -> %s/%s), retryCnt: %d, mvState: %s, epoch: %d",
								srcChunkPath, job.srcRVName, job.mvName, job.destRVName, job.mvName,
								retryCnt, mvState, epoch)
							continue
						}

						log.Err("ReplicationManager::copyOutOfSyncChunks: Exceeded %d retries while copying chunk %s (%s/%s -> %s/%s), epoch: %d",
							retryCnt, srcChunkPath, job.srcRVName, job.mvName, job.destRVName, job.mvName, epoch)
					} else {
						// Clustermap changed in a way that makes it unsafe to continue the sync job.
						errStr := fmt.Sprintf("Aborting sync, Clustermap changed, srcRVok: %v, srcRVState: %s, dstRVok: %v, dstRVState: %s, mvState: %s, epoch: %d",
							srcRVok, srcRVState, dstRVok, dstRVState, mvState, epoch)
						log.Err("ReplicationManager::copyOutOfSyncChunks: %s", errStr)
					}
				} else {
					log.Err("ReplicationManager::copyOutOfSyncChunks: RefreshClusterMap() failed for %s (retryCnt: %d): %v",
						rpc.PutChunkRequestToString(putChunkReq), retryCnt, errCM)
				}
			} else {
				//
				// Non-retriable error in syncing.
				// Fall through and return error, caller will mark job.destRVName as inband-offline.
				//
			}

			return err
		}

		chunksCopied++
		bytesCopied += int64(len(srcData))
	}

	log.Debug("ReplicationManager::copyOutOfSyncChunks: Copied %d chunks totalling %d bytes, Sync job: %s",
		chunksCopied, bytesCopied, job.toString())
	return nil
}

// GetMVSize() is called from fixMV workflow, by the cluster manager or from syncMV() by replication manager.
// The cluster manager has the final MV composition (which is different from the one in the clustermap as it
// would have replaced offline RVs with new outofsync RVs and it may have also made some component RVs offline).
// So we take the new MV composition from the caller and save wasted calls to offline RVs.
// clustermapEpoch is the epoch at which the componentRVs were fetched by the caller.
func GetMVSize(mvName string, componentRVs []*models.RVNameAndState, clustermapEpoch int64) (int64, error) {
	common.Assert(cm.IsValidMVName(mvName), mvName, clustermapEpoch)
	common.Assert(len(componentRVs) == int(getNumReplicas()),
		mvName, componentRVs, getNumReplicas(), clustermapEpoch)
	// Since GetMVSize() can be called from syncMV() as well, we can't assert anything else.
	common.Assert(clustermapEpoch > 0, clustermapEpoch, mvName)

	var mvSize int64
	var err error

	log.Debug("ReplicationManager::GetMVSize: Component RVs for %s are %+v, at epoch %d",
		mvName, componentRVs, clustermapEpoch)

	//
	// Get the most suitable RV from the provided list of component RVs, from which we should query the size of
	// the MV. Selecting most suitable RV is mostly a heuristical process which might pick the most suitable RV
	// based on one or more of the following criteria:
	// - Local RV would be preferred.
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
			MV:              mvName,
			RVName:          readerRV.Name,
			ClustermapEpoch: clustermapEpoch,
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

		if err == nil {
			// Success.
			common.Assert(resp != nil, rpc.GetMVSizeRequestToString(req))
			mvSize = resp.MvSize
			log.Debug("ReplicationManager::GetMVSize: GetMVSize successful for %s/%s, MV size: %d",
				req.RVName, req.MV, mvSize)
			break
		}

		log.Warn("ReplicationManager::GetMVSize: Failed to get MV size from node %s for %s/%s [%v]",
			targetNodeID, req.RVName, req.MV, err)

		//
		// Try another replica if available.
		// Exclude already tried RVs from further iterations (if any).
		//
		excludeRVs = append(excludeRVs, readerRV.Name)
	}

	return mvSize, nil
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
