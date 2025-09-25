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

package rpc_server

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"maps"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

// returns the chunk and hash path for the given fileID and offsetInMB from RV/MV directory as,
// <cache dir>/<mvName>/<fileID>.<offsetInMB>.data and
// <cache dir>/<mvName>/<fileID>.<offsetInMB>.hash
func getChunkAndHashPath(cacheDir string, mvName string, fileID string, offsetInMB int64) (string, string) {
	chunkPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.data", fileID, offsetInMB))
	hashPath := filepath.Join(cacheDir, mvName, fmt.Sprintf("%v.%v.hash", fileID, offsetInMB))
	return chunkPath, hashPath
}

// Sort the component RVs in the MV.
// The RVs are sorted in increasing order of their names.
func sortComponentRVs(rvs []*models.RVNameAndState) {
	sort.Slice(rvs, func(i, j int) bool {
		return rvs[i].Name < rvs[j].Name
	})
}

// Check if the component RVs are the same. The list is sorted before comparison.
// An example of RV array can be like,
//
// [
//
//	{"name":"rv0", "state": "online"},
//	{"name":"rv5", "state": "syncing"},
//	{"name":"rv9", "state": "outofsync"}
//
// ]
//
// checkState boolean flag indicates if the state of the component RVs in the request should be
// matched against the state of the component RVs in the mvInfo data.
func isComponentRVsValid(rvInMV []*models.RVNameAndState, rvInReq []*models.RVNameAndState, checkState bool) error {
	if len(rvInMV) != len(rvInReq) {
		return fmt.Errorf("request component RVs %s is not same as MV component RVs %s",
			rpc.ComponentRVsToString(rvInReq), rpc.ComponentRVsToString(rvInMV))
	}

	sortComponentRVs(rvInReq)

	isValid := true
	for i := 0; i < len(rvInMV); i++ {
		common.Assert(rvInMV[i] != nil)
		common.Assert(rvInReq[i] != nil)

		if rvInMV[i].Name != rvInReq[i].Name || (checkState && rvInMV[i].State != rvInReq[i].State) {
			isValid = false
			break
		}
	}

	if !isValid {
		rvInMvStr := rpc.ComponentRVsToString(rvInMV)
		rvInReqStr := rpc.ComponentRVsToString(rvInReq)
		err := fmt.Errorf("request component RVs %s is not same as MV component RVs %s",
			rvInReqStr, rvInMvStr)
		log.Err("utils::isComponentRVsValid: %v", err)
		return err
	}

	return nil
}

// Check if the RV is present in the component RVs of the MV.
func isRVPresentInMV(rvs []*models.RVNameAndState, rvName string) bool {
	for _, rv := range rvs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			return true
		}
	}

	return false
}

// create the rvID map from RVs present in the current node
func getRvIDMap(rvs map[string]dcache.RawVolume) map[string]*rvInfo {
	rvIDMap := make(map[string]*rvInfo)

	for rvName, val := range rvs {
		rvInfo := &rvInfo{
			rvID:     val.RvId,
			rvName:   rvName,
			cacheDir: val.LocalCachePath,
		}

		rvIDMap[val.RvId] = rvInfo
	}

	return rvIDMap
}

// This returns the maximum MVsPerRV value that we allow.
// We allow more MVs to be placed per RV in fix-mv than new-mv.
func getMVsPerRV() int64 {
	mvsPerRV := int64(cm.MVsPerRVForFixMV.Load())
	common.Assert(mvsPerRV > 0, mvsPerRV)
	common.Assert(mvsPerRV > int64(cm.MVsPerRVForNewMV), mvsPerRV, cm.MVsPerRVForNewMV)
	return mvsPerRV
}

// Check if any of the RV present in the component RVs has inband-offline state.
func containsInbandOfflineState(componentRVs *[]*models.RVNameAndState) bool {
	for _, rv := range *componentRVs {
		common.Assert(rv != nil)
		if rv.State == string(dcache.StateInbandOffline) {
			return true
		}
	}

	return false
}

// Update the inband-offline state to offline for all the component RVs in the request.
func updateInbandOfflineToOffline(componentRVs *[]*models.RVNameAndState) {
	for _, rv := range *componentRVs {
		common.Assert(rv != nil)
		if rv.State == string(dcache.StateInbandOffline) {
			rv.State = string(dcache.StateOffline)
		}
	}
}

// This method is wrapper for the GetChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetChunkLocal(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	//
	// This chunk is being read locally without any RPC, so we can set IsLocalRV to true. This is used for
	// taking the decision in the handler to allocate the chunk from the buffer pool instead of initializing
	// a new buffer.
	//
	req.IsLocalRV = true

	common.Assert(handler != nil)

	return handler.GetChunk(ctx, req)
}

// This method is wrapper for the PutChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkLocal(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.PutChunk(ctx, req)
}

// This method is wrapper for the PutChunkDC() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkDCLocal(ctx context.Context, req *models.PutChunkDCRequest) (*models.PutChunkDCResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Request != nil)
	common.Assert(req.Request.Chunk != nil)
	common.Assert(req.Request.Chunk.Address != nil)
	common.Assert(len(req.NextRVs) > 0)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.Request.SenderNodeID) == 0, req.Request.SenderNodeID)
	req.Request.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	//
	// Even though this call is made locally and doesn't need an RPC client, we still allocate a dummy
	// RPC client. This is needed as we also use RPC clients as a way to rate-limit the number of concurrent
	// PutChunkDC calls to avoid overwhelming the receivers, since receivers might have to daisy chain
	// the call to other nodes and they will need RPC clients to do that. If we make too many concurrent
	// calls, then multiple nodes may might run out of RPC clients in the pool and they might deadlock
	// waiting for each other's PutChunkDC response.
	//
	client, err := rpc_client.GetRPCClientDummy(req.Request.SenderNodeID)
	if err != nil {
		err = fmt.Errorf("rpc_server::PutChunkDCLocal: Failed to get dummy RPC client for node %s %v: %v",
			req.Request.SenderNodeID, rpc.PutChunkDCRequestToString(req), err)
		log.Err("%v", err)
		common.Assert(false, err)
		return nil, err
	}

	resp, err := handler.PutChunkDC(ctx, req)

	// Release RPC client back to the pool.
	err1 := rpc_client.ReleaseRPCClientDummy(client)
	if err1 != nil {
		log.Err("rpc_server::PutChunkDCLocal: Failed to release dummy RPC client for node %s %v: %v",
			req.Request.SenderNodeID, rpc.PutChunkDCRequestToString(req), err1)
		// Assert, but do not fail the PutChunkDC call.
		common.Assert(false, err1)
	}

	return resp, err
}

// This method is wrapper for the GetMVSize() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetMVSizeLocal(ctx context.Context, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.GetMVSize(ctx, req)
}

// Get the time when the RV joined this MV and the last write to this RV/MV replica by a sync PutChunk request.
// This will be used to determine if there are any stuck sync jobs caused due to source RV going offline.
// For more details see the comments in mvInfo.joinTime and mvInfo.lastSyncWriteTime.
func GetMVJoinAndLastSyncWriteTime(rvName string, mvName string) (int64, int64) {
	common.Assert(cm.IsValidRVName(rvName), rvName)
	common.Assert(cm.IsValidMVName(mvName), mvName)
	common.Assert(handler != nil)

	rvInfo := handler.getRVInfoFromRVName(rvName)
	common.Assert(rvInfo != nil, rvName)

	mvInfo := rvInfo.getMVInfo(mvName)
	common.Assert(mvInfo != nil, rvName, mvName)

	// Since the RV has joined the MV, the joinTime must be set.
	common.Assert(mvInfo.joinTime.Load() > 0 && mvInfo.joinTime.Load() <= time.Now().Unix(),
		rvName, mvName, mvInfo.joinTime.Load(), time.Now().Unix())
	// lastSyncWriteTime can be 0 if there has not been any sync write to this RV/MV replica.
	common.Assert(mvInfo.lastSyncWriteTime.Load() >= 0 && mvInfo.lastSyncWriteTime.Load() <= time.Now().Unix(),
		rvName, mvName, mvInfo.lastSyncWriteTime.Load(), time.Now().Unix())
	// If set, lastSyncWriteTime must be >= joinTime.
	common.Assert(mvInfo.lastSyncWriteTime.Load() == 0 || mvInfo.lastSyncWriteTime.Load() >= mvInfo.joinTime.Load(),
		rvName, mvName, mvInfo.lastSyncWriteTime.Load(), mvInfo.joinTime.Load())

	return mvInfo.joinTime.Load(), mvInfo.lastSyncWriteTime.Load()
}

// Maps are passed as reference in Go. So, if we get the local clustermap reference and update it,
// it can lead to inconsistency. So, as temporary workaround, we are deep copying the map here.
//
// TODO: Check at all places where we pass the clustermap as reference and are updating it.
//       Check the best way to avoid deep copying the map.

func deepCopyRVMap(rvs map[string]dcache.StateEnum) map[string]dcache.StateEnum {
	common.Assert(rvs != nil)

	newRVs := make(map[string]dcache.StateEnum)
	maps.Copy(newRVs, rvs)

	return newRVs
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	time.Since(time.Now())
}
