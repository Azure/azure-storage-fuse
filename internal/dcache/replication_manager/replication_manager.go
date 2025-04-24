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
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

func ReadMV(req *ReadMvRequest) (*ReadMvResponse, error) {
	if req == nil {
		log.Err("ReplicationManager::ReadMV: Received nil ReadMV request")
		common.Assert(false, "received nil ReadMV request")
		return nil, fmt.Errorf("received nil ReadMV request")
	}

	log.Debug("ReplicationManager::ReadMV: Received ReadMV request: %+v", *req)

	if req.FileID == "" || req.RvID == "" || req.MvName == "" || req.Offset < 0 || req.Length <= 0 || req.ChunkSizeInMB <= 0 {
		log.Err("ReplicationManager::ReadMV: Invalid ReadMV request parameters: %+v", *req)
		common.Assert(false, fmt.Sprintf("invalid ReadMV request parameters: %+v", *req))
		return nil, fmt.Errorf("invalid ReadMV request parameters: %+v", *req)
	}

	if req.Length > int64(req.ChunkSizeInMB)*1024*1024 {
		log.Err("ReplicationManager::ReadMV: Read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB)
		common.Assert(false, fmt.Sprintf("read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB))
		return nil, fmt.Errorf("read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB)
	}

	// calculate the offset in MB which is multiple of chunk size
	// chunks are stored in RV as,
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.data and
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.hash
	offsetInMB := (req.Offset / req.ChunkSizeInMB * (1024 * 1024)) * req.ChunkSizeInMB

	// relative offset within the chunk
	relativeOffset := req.Offset - (offsetInMB * 1024 * 1024)

	selectedRvName, err := selectOnlineRVForMV(req.MvName)
	if err != nil {
		log.Err("ReplicationManager::ReadMV: Failed to select online RV for MV %s [%v]", req.MvName, err)
		return nil, err
	}

	// TODO:: integration: call cluster manager to get the RV ID for the given RV name
	// this might not be needed if the RV struct is returned containing the RV ID
	selectedRvID := getRvIDFromRvName(selectedRvName)
	common.Assert(len(selectedRvID) > 0, "selected RV ID is empty")

	// TODO:: integration: call cluster manager to get the node ID hosting the given RV
	// this might not be needed if the RV struct is returned containing the node ID
	targetNodeID := getNodeIDForRVName(selectedRvName)
	// TODO: formatting check of node id in assert
	common.Assert(len(targetNodeID) > 0, "target node ID is empty")

	log.Debug("ReplicationManager::ReadMV: Selected online RV for MV %s is %s having RV id %s and is hosted in node id %s", req.MvName, selectedRvName, selectedRvID, targetNodeID)

	rpcReq := &models.GetChunkRequest{
		Address: &models.Address{
			FileID:     req.FileID,
			FsID:       selectedRvID,
			MvID:       req.MvName,
			OffsetInMB: offsetInMB,
		},
		Offset: relativeOffset,
		Length: req.Length,
	}

	ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
	defer cancel()

	rpcResp, err := rpc_client.GetChunk(ctx, targetNodeID, rpcReq)
	if err != nil {
		log.Err("ReplicationManager::ReadMV: Failed to get chunk from node %s for request %+v [%v]", targetNodeID, *rpcReq, err)
		common.Assert(false, fmt.Sprintf("failed to get chunk from node %s for request %+v", targetNodeID, *rpcReq))
		return nil, err
	}

	common.Assert(rpcResp != nil, "GetChunk RPC response is nil for request %+v", *rpcReq)
	common.Assert(rpcResp.Chunk != nil, "chunk in GetChunk RPC response is nil for request %+v", *rpcReq)
	common.Assert(rpcResp.Chunk.Address != nil, "address of chunk in GetChunk RPC response is nil for request %+v", *rpcReq)

	log.Debug("ReplicationManager::ReadMV: GetChunk RPC response: chunk lmt %v, time taken %v, component RVs %v, chunk address %+v", rpcResp.ChunkWriteTime, rpcResp.TimeTaken, rpcResp.PeerRV, *rpcResp.Chunk.Address)

	// TODO:  should we validate the hash of the chunk here?

	resp := &ReadMvResponse{
		Data: rpcResp.Chunk.Data,
		Hash: rpcResp.Chunk.Hash,
	}

	return resp, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	return nil, nil
}

func OfflineMV(req *OfflineMvRequest) (*OfflineMvResponse, error) {
	return nil, nil
}
