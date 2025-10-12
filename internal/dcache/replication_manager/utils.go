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
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

const (
	// TODO: For prod we should increase it for resilience, but not too much as to affect
	// our responsiveness.
	RPCClientTimeout = 2 // in seconds

	// This is a practically infeasible chunk index, for sanity checks.
	ChunkIndexUpperBound = 1e9

	// Time interval in seconds for resyncing degraded MVs.
	ResyncInterval = 10

	// Time in microseconds to add to the sync start time to account for clock skew
	NTPClockSkewMargin = 5 * 1e6

	//
	// Max concurrent PutChunkDC calls across all nodes and to a specific node.
	// Both of these (specially the global limit) need to be carefully tuned based on experiments
	// on different size clusters.
	//
	// Note: This is not an ideal situation, ideally we would not like to limit ourselves artifically,
	//       and would like to get limited only by resources like n/w and disk throughput. We are
	//       having to do this because Thrift does not support asynchronous calls (where multiple RPC
	//       calls can be in-flight on the same connection and the responses are matched using a unique
	//       RPC Id that each request/response carries), so if we do not limit the number of concurrent calls,
	//       it puts a lot of pressure on the RPC connection pool and under load it takes multiple seconds
	//       to get a connection from the pool, which results in timeouts and failures and hurts performance.
	//
	//       If the outstanding PutChunkDC calls hang/get delayed for any reason, then we would not be
	//       optimally utilizing the available n/w and disk throughput. Hopefully this will not happen often.
	//
	// TODO: Consider gRPC which supports async calls.
	//
	// Note: These values need to be set in close coordination with defaultMaxPerNode.
	//       These must be set as low as possible, just enough to saturate the n/w and disk throughput from
	//       a single node.
	//
	PutChunkDCIODepthTotal   = 32
	PutChunkDCIODepthPerNode = 8

	//
	// Number of workers in the thread pool for sending the RPC requests.
	// Note that one worker is used for one replica IO (read or write), so we need these to be accordingly
	// higher than the fileIOManager workers setting.
	//
	MAX_WORKER_COUNT = 512

	// Maximum number of sync jobs (running syncComponentRV()) that can be running at any time.
	// This should be smaller than cm.MAX_SIMUL_RV_STATE_UPDATES.
	MAX_SIMUL_SYNC_JOBS = 100
)

type PutChunkStyleEnum int

const (
	//
	// Originator calls PutChunk for each component RV in parallel.
	//
	OriginatorSendsToAll PutChunkStyleEnum = iota

	//
	// Originator writes to the local component RV (if any) and calls PutChunkDC (with the list of remaining
	// component RV) to the next component RV, which will then write locally and send PutChunkDC to the next
	// component RV (with a smaller list of component RVs) and so on.
	// This has the advantage that the overall file write throughput is not limited by the egress throughput
	// of the originator node.
	//
	DaisyChain
)

var (
	//
	// These stats are used to find slowness in one of the following legs:
	// - Time taken to schedule the WriteMV call (i.e. time spent waiting to acquire the PutChunkDC semaphores).
	// - Time taken to complete the WriteMV call (i.e. time taken by the RV to write the chunk locally and
	//   send PutChunkDC calls to other RVs and get response from all).
	//
	// Also see getRPCClient() for stats related to acquiring the RPC client connection.
	//
	// If the throughput is low, then we can look at these stats to find out which leg is slow.
	//
	aggrWriteMVCalls      atomic.Int64 // aggregate number of WriteMV calls.
	aggrWriteMVTime       atomic.Int64 // aggregate time for completing all WriteMV calls (in nanoseconds).
	aggrWriteMVWait       atomic.Int64 // aggregate time spent waiting to schedule WriteMV calls (in nanoseconds).
	aggrPutChunkDCSemWait atomic.Int64 // aggregate time spent waiting to acquire PutChunkDC semaphore (in nanoseconds).
	aggrPutChunkDCSemHold atomic.Int64 // aggregate time spent holding PutChunkDC semaphore (in nanoseconds).
	aggrPutChunkDCCalls   atomic.Int64 // aggregate number of PutChunkDC calls.
	putChunkDCSemWaiting  atomic.Int64 // number of PutChunkDC calls currently waiting to acquire semaphore.
)

// Semaphores to limit the number of concurrent PutChunkDC calls across all nodes and to a specific node.
var putChunkDCTotalSem = make(chan struct{}, PutChunkDCIODepthTotal)
var putChunkDCPerNodeSemMap = make(map[string]*chan struct{})
var putChunkDCPerNodeSemMapLock sync.Mutex

func (s PutChunkStyleEnum) String() string {
	switch s {
	case OriginatorSendsToAll:
		return "OriginatorSendsToAll"
	case DaisyChain:
		return "DaisyChain"
	default:
		common.Assert(false, s)
		return "Unknown"
	}
}

// Acquire the semaphores for sending PutChunkDC to the given target node.
func getPutChunkDCSem(targetNodeID string, chunkIdx int64) *chan struct{} {
	// Anything above this threshold is considered a large/unusual wait and is logged as a warning.
	const largeWaitThreshold = 2 * time.Second

	putChunkDCSemWaiting.Add(1)
	startTime := time.Now()

	// Grab per-node semaphore.
	putChunkDCPerNodeSemMapLock.Lock()
	putChunkDCSemNode, ok := putChunkDCPerNodeSemMap[targetNodeID]
	if !ok {
		sem := make(chan struct{}, PutChunkDCIODepthPerNode)
		putChunkDCSemNode = &sem
		putChunkDCPerNodeSemMap[targetNodeID] = putChunkDCSemNode
	}
	putChunkDCPerNodeSemMapLock.Unlock()

	(*putChunkDCSemNode) <- struct{}{}

	//
	// Grab global semaphore.
	// We do it after grabbing the per-node semaphore as this is a more sacred resource, since
	// PutChunkDC calls to *any* node will be limited by this. We do not want to grab this and then
	// wait for the per-node semaphore.
	//
	putChunkDCTotalSem <- struct{}{}

	common.Assert(putChunkDCSemWaiting.Load() > 0, putChunkDCSemWaiting.Load())
	putChunkDCSemWaiting.Add(-1)

	//
	// Update aggregate wait time for acquiring the semaphore. To reflect the current situation more accurately,
	// we reset the aggregate stats after every 200 calls, roughly ~3GB of data written (with 16MB chunks).
	// If the avg wait time is very high, then we should consider increasing the semaphore depth, or
	// find out why the PutChunkDC calls are taking too long.
	//
	if aggrPutChunkDCCalls.Add(1) == 200 {
		// Since it's not protected by a lock, we don't set it to zero, but to 1, to avoid division by zero.
		aggrPutChunkDCCalls.Store(1)
		aggrPutChunkDCSemWait.Store(time.Since(startTime).Nanoseconds())
		aggrPutChunkDCSemHold.Store(1)
	} else {
		aggrPutChunkDCSemWait.Add(time.Since(startTime).Nanoseconds())
	}

	log.Debug("getPutChunkDCSem: Acquired semaphore for node: %s, chunkIdx: %d, took %s, now available: {global: %d/%d, node: %d/%d}, waiting: %d, avg wait: %s",
		targetNodeID, chunkIdx, time.Since(startTime),
		PutChunkDCIODepthTotal-len(putChunkDCTotalSem), PutChunkDCIODepthTotal,
		PutChunkDCIODepthPerNode-len(*putChunkDCSemNode), PutChunkDCIODepthPerNode,
		putChunkDCSemWaiting.Load(),
		time.Duration(aggrPutChunkDCSemWait.Load()/aggrPutChunkDCCalls.Load()))

	if time.Since(startTime) > largeWaitThreshold {
		log.Warn("[SLOW] getPutChunkDCSem: Acquired semaphore for node: %s, chunkIdx: %d, took %s (> %s), now available: {global: %d/%d, node: %d/%d}, waiting: %d, avg wait: %s",
			targetNodeID, chunkIdx, time.Since(startTime), largeWaitThreshold,
			PutChunkDCIODepthTotal-len(putChunkDCTotalSem), PutChunkDCIODepthTotal,
			PutChunkDCIODepthPerNode-len(*putChunkDCSemNode), PutChunkDCIODepthPerNode,
			putChunkDCSemWaiting.Load(),
			time.Duration(aggrPutChunkDCSemWait.Load()/aggrPutChunkDCCalls.Load()))
	}

	return putChunkDCSemNode
}

// Release the semaphore acquired by getPutChunkDCSem() for the given target node.
func releasePutChunkDCSem(putChunkDCSemNode *chan struct{}, targetNodeID string, chunkIdx int64, dur time.Duration) {
	const largeHoldThreshold = 2 * time.Second

	// We must be releasing a semaphore that we have acquired.
	common.Assert(len(*putChunkDCSemNode) > 0, len(*putChunkDCSemNode))
	common.Assert(len(putChunkDCTotalSem) > 0, len(putChunkDCTotalSem))

	// Duration is the time the semaphore was held, which is the time taken for the PutChunkDC call to complete.
	aggrPutChunkDCSemHold.Add(dur.Nanoseconds())

	<-putChunkDCTotalSem
	<-*putChunkDCSemNode

	log.Debug("releasePutChunkDCSem: Released semaphore for node: %s, chunkIdx: %d, now available: {global: %d/%d, node: %d/%d}, held for: %s, avg hold: %s",
		targetNodeID, chunkIdx,
		PutChunkDCIODepthTotal-len(putChunkDCTotalSem), PutChunkDCIODepthTotal,
		PutChunkDCIODepthPerNode-len(*putChunkDCSemNode), PutChunkDCIODepthPerNode,
		dur, time.Duration(aggrPutChunkDCSemHold.Load()/aggrPutChunkDCCalls.Load()))

	if dur > largeHoldThreshold {
		log.Warn("[SLOW] releasePutChunkDCSem: Released semaphore for node: %s, chunkIdx: %d, now available: {global: %d/%d, node: %d/%d}, held for: %s (> %s), avg hold: %s",
			targetNodeID, chunkIdx,
			PutChunkDCIODepthTotal-len(putChunkDCTotalSem), PutChunkDCIODepthTotal,
			PutChunkDCIODepthPerNode-len(*putChunkDCSemNode), PutChunkDCIODepthPerNode,
			dur, largeHoldThreshold, time.Duration(aggrPutChunkDCSemHold.Load()/aggrPutChunkDCCalls.Load()))
	}
}

// Return the most suitable online RV from the list of component RVs to which we should send the RPC call.
// The RV is selected based on the following criteria:
//  1. Local online RV is preferred, if available.
//  2. Else, select a random online RV from the list of component RVs.
//
// This method also takes an excludeRVs list, which is used to skip the RVs that should not be selected.
func getReaderRV(componentRVs []*models.RVNameAndState, excludeRVs []string) *models.RVNameAndState {
	log.Debug("utils::getReaderRV: Component RVs are: %v, excludeRVs: %v",
		rpc.ComponentRVsToString(componentRVs), excludeRVs)

	// componentRVs must have exactly NumReplicas RVs.
	common.Assert(len(componentRVs) == int(getNumReplicas()), len(componentRVs), getNumReplicas())
	// excludeRVs can have at max all the RVs in componentRVs.
	common.Assert(len(excludeRVs) <= len(componentRVs), len(excludeRVs), len(componentRVs))

	var readerRV *models.RVNameAndState

	myNodeID := rpc.GetMyNodeUUID()
	for _, rv := range componentRVs {
		if rv.State != string(dcache.StateOnline) || slices.Contains(excludeRVs, rv.Name) {
			// Not an online RV or present in the exclude list, skip.
			log.Debug("utils::getReaderRV: skipping RV %s with state %s", rv.Name, rv.State)
			continue
		}

		nodeIDForRV := getNodeIDFromRVName(rv.Name)
		common.Assert(common.IsValidUUID(nodeIDForRV))
		if nodeIDForRV == myNodeID {
			//
			// Prefer local RV.
			// TODO: We can further optimize this by checking the local RV's load and avoid skewed load.
			//
			return rv
		}

		//
		// getComponentRVsForMV() already returns a shuffled list of RVs, so we can pick the last one
		// and we will get a random RV.
		//
		readerRV = rv
	}

	if readerRV == nil {
		log.Debug("utils::getReaderRV: no suitable RVs found for component RVs %v",
			rpc.ComponentRVsToString(componentRVs))
		return nil
	}

	return readerRV
}

// TODO: hash validation will be done later
// TODO: should byte array be used for storing hash instead of string?
// check is md5sum can be used for hash or crc should be used
// func getMD5Sum(data []byte) string {
// 	hash := md5.Sum(data)
// 	return hex.EncodeToString(hash[:])
// }

// Return list of component RVs (name and state) for the given MV, and its state, and also the clustermap Epoch.
// The epoch should be used by the caller to correctly refresh the clustermap on receiving a NeedToRefreshClusterMap
// error.
func getComponentRVsForMV(mvName string, randomize bool) (dcache.StateEnum, []*models.RVNameAndState, int64) {
	mvState, rvMap, epoch := cm.GetRVsEx(mvName)
	return mvState, cm.RVMapToList(mvName, rvMap, randomize), epoch
}

// return the number of replicas
func getNumReplicas() uint32 {
	return cm.GetCacheConfig().NumReplicas
}

// return the RV ID for the given RV name
func getRvIDFromRvName(rvName string) string {
	return cm.RvNameToId(rvName)
}

// return the RV name for the given RV ID
func getRvNameFromRvID(rvId string) string {
	return cm.RvIdToName(rvId)
}

// return the node ID for the given rvName
func getNodeIDFromRVName(rvName string) string {
	return cm.RVNameToNodeId(rvName)
}

// return the local cache path for the given RV name
// Note: this RV should be hosted by the this node
func getCachePathForRVName(rvName string) string {
	myRVs := cm.GetMyRVs()

	common.Assert(myRVs != nil)
	common.Assert(len(myRVs) > 0)

	rv, ok := myRVs[rvName]
	_ = ok

	common.Assert(ok, fmt.Sprintf("%s not hosted by this node, %+v", rvName, myRVs))
	common.Assert(rv.LocalCachePath != "", rvName)
	common.Assert(common.DirectoryExists(rv.LocalCachePath), rv.LocalCachePath)

	return rv.LocalCachePath
}

// Update the state of the RV in the given component RVs list.
func updateLocalComponentRVState(rvs []*models.RVNameAndState, rvName string,
	oldState dcache.StateEnum, newState dcache.StateEnum) {

	common.Assert(len(rvs) == int(getNumReplicas()), len(rvs), getNumReplicas())
	common.Assert(cm.IsValidRVName(rvName), rvName)
	common.Assert(oldState != newState &&
		cm.IsValidComponentRVState(oldState) &&
		cm.IsValidComponentRVState(newState), rvName, oldState, newState)

	for _, rv := range rvs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			common.Assert(rv.State == string(oldState), rvName, rv.State, oldState)
			log.Debug("utils::updateLocalComponentRVState: %s (%s -> %s) %s",
				rvName, rv.State, newState, rpc.ComponentRVsToString(rvs))

			rv.State = string(newState)
			return
		}
	}

	// RV is not present in the list.
	common.Assert(false, rpc.ComponentRVsToString(rvs), rvName)
}

// Add the PutChunkDCResponse to the response channel.
func addPutChunkDCResponseToChannel(response *models.PutChunkDCResponse, responseChannel chan *responseItem) {
	common.Assert(response != nil)
	common.Assert(response.Responses != nil)
	// There shouldn't be any PutChunkDCResponse with no responses.
	common.Assert(len(response.Responses) > 0)

	for rvName, resp := range response.Responses {
		common.Assert(cm.IsValidRVName(rvName), rvName)
		common.Assert(resp != nil)
		common.Assert(len(responseChannel) < int(getNumReplicas()),
			len(responseChannel), getNumReplicas())

		var err error

		if resp.Error != nil {
			// One and only one of Response and Error will be nil/non-nil.
			common.Assert(resp.Response == nil)
			err = resp.Error
		} else {
			common.Assert(resp.Response != nil)
		}

		responseChannel <- &responseItem{
			rvName:  rvName,
			rpcResp: resp.Response,
			err:     err,
		}
	}

	common.Assert(len(responseChannel) == int(getNumReplicas()),
		len(responseChannel), getNumReplicas())
}

func init() {
	common.Assert(MAX_SIMUL_SYNC_JOBS < cm.MAX_SIMUL_RV_STATE_UPDATES,
		MAX_SIMUL_SYNC_JOBS, cm.MAX_SIMUL_RV_STATE_UPDATES)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	log.Info("")
	fmt.Printf("")
}
