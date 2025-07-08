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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
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

// return mvs-per-rv from dcache config
func getMVsPerRV() int64 {
	return int64(cm.GetCacheConfig().MvsPerRv)
}

// This method is wrapper for the GetChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetChunkLocal(ctc context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.GetChunk(ctc, req)
}

// This method is wrapper for the PutChunk() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkLocal(ctc context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.PutChunk(ctc, req)
}

// This method is wrapper for the PutChunkDC() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func PutChunkDCLocal(ctc context.Context, req *models.PutChunkDCRequest) (*models.PutChunkDCResponse, error) {
	common.Assert(req != nil)
	common.Assert(req.Request != nil)
	common.Assert(req.Request.Chunk != nil)
	common.Assert(req.Request.Chunk.Address != nil)
	common.Assert(len(req.NextRVs) > 0)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.Request.SenderNodeID) == 0, req.Request.SenderNodeID)
	req.Request.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.PutChunkDC(ctc, req)
}

// This method is wrapper for the GetMVSize() RPC call. It is used when the both the client and server
// belong to the same node, i.e. the RPC is called locally.
func GetMVSizeLocal(ctc context.Context, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = rpc.GetMyNodeUUID()

	common.Assert(handler != nil)

	return handler.GetMVSize(ctc, req)
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
