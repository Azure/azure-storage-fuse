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

	if req.Length > req.ChunkSizeInMB*common.MbToBytes {
		log.Err("ReplicationManager::ReadMV: Read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB)
		common.Assert(false, fmt.Sprintf("read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB))
		return nil, fmt.Errorf("read length %v exceeds chunk size %vMB", req.Length, req.ChunkSizeInMB)
	}

	// calculate the offset in MB which is multiple of chunk size
	// chunks are stored in RV as,
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.data and
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.hash
	offsetInMB := (req.Offset / (req.ChunkSizeInMB * common.MbToBytes)) * req.ChunkSizeInMB

	// relative offset within the chunk
	relativeOffset := req.Offset - (offsetInMB * common.MbToBytes)

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

	common.Assert(rpcResp != nil, fmt.Sprintf("GetChunk RPC response is nil for request %+v", *rpcReq))
	common.Assert(rpcResp.Chunk != nil, fmt.Sprintf("chunk in GetChunk RPC response is nil for request %+v", *rpcReq))
	common.Assert(rpcResp.Chunk.Address != nil, fmt.Sprintf("address of chunk in GetChunk RPC response is nil for request %+v", *rpcReq))

	log.Debug("ReplicationManager::ReadMV: GetChunk RPC response: chunk lmt %v, time taken %v, component RVs %v, chunk address %+v",
		rpcResp.ChunkWriteTime, rpcResp.TimeTaken, rpcResp.PeerRV, *rpcResp.Chunk.Address)

	// TODO:  should we validate the hash of the chunk here?

	resp := &ReadMvResponse{
		Data: rpcResp.Chunk.Data,
		Hash: rpcResp.Chunk.Hash,
	}

	return resp, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	if req == nil {
		log.Err("ReplicationManager::WriteMV: Received nil WriteMV request")
		common.Assert(false, "received nil WriteMV request")
		return nil, fmt.Errorf("received nil WriteMV request")
	}
	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: file id %v, rv id %v, mv name %v, offset %v, chunk size in MB %v",
		req.FileID, req.RvID, req.MvName, req.Offset, req.ChunkSizeInMB)

	if req.FileID == "" || req.RvID == "" || req.MvName == "" || req.Offset < 0 || len(req.Data) == 0 || req.Hash == "" || req.ChunkSizeInMB <= 0 {
		log.Err("ReplicationManager::WriteMV: Invalid WriteMV request parameters")
		common.Assert(false, fmt.Sprintf("invalid WriteMV request parameters"))
		return nil, fmt.Errorf("invalid WriteMV request parameters")
	}

	if len(req.Data) != int(req.ChunkSizeInMB*common.MbToBytes) {
		log.Err("ReplicationManager::WriteMV: Write data length %v is not equal to chunk size %vMB", len(req.Data), req.ChunkSizeInMB)
		common.Assert(false, fmt.Sprintf("write data length %v is not equal to chunk size %vMB", len(req.Data), req.ChunkSizeInMB))
		return nil, fmt.Errorf("write data length %v is not equal to chunk size %vMB", len(req.Data), req.ChunkSizeInMB)
	}

	// offset given in request should be multiple of chunk size
	if req.Offset%(req.ChunkSizeInMB*common.MbToBytes) != 0 {
		log.Err("ReplicationManager::WriteMV: Write offset %v is not multiple of chunk size %vMB", req.Offset, req.ChunkSizeInMB)
		common.Assert(false, fmt.Sprintf("write offset %v is not multiple of chunk size %vMB", req.Offset, req.ChunkSizeInMB))
		return nil, fmt.Errorf("write offset %v is not multiple of chunk size %vMB", req.Offset, req.ChunkSizeInMB)
	}

	// calculate the offset in MB which is multiple of chunk size
	// chunks are stored in RV as,
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.data and
	// <cacheDir>/<mvName>/<fileID>.<offsetInMB>.hash
	offsetInMB := (req.Offset / (req.ChunkSizeInMB * common.MbToBytes)) * req.ChunkSizeInMB

	componentRVs := getComponentRVsForMV(req.MvName)
	if len(componentRVs) == 0 {
		log.Err("ReplicationManager::WriteMV: No component RVs found for the given MV %s", req.MvName)
		common.Assert(false, "no component RVs found for the given MV", req.MvName)
		return nil, fmt.Errorf("no component RVs found for the given MV %s", req.MvName)
	}

	// check if the mv is online. This is done by checking if all the component RVs are online.
	// If any of the component RVs are offline, then fail this request.
	// The caller of WriteMV should make sure that the MV is online at the time calling this function.
	// This does not ensure that the MV will be online at the time of writing.
	// If any of the PutChunk RPC calls fail, then the degradeMV method should be called.
	rvsValid := checkComponentRVsOnline(componentRVs)
	if !rvsValid {
		log.Err("ReplicationManager::WriteMV: Not all component RVs are online for the given MV %s", req.MvName)
		common.Assert(false, "not all component RVs are online for the given MV", req.MvName)
		return nil, fmt.Errorf("not all component RVs are online for the given MV %s", req.MvName)
	}

	log.Debug("ReplicationManager::WriteMV: Component RVs for the given MV %s are: %v", req.MvName, componentRVs)

	// write to all component RVs
	// TODO: put chunk to each component RV can be done in parallel
	for _, rv := range componentRVs {
		rvID := getRvIDFromRvName(rv)
		common.Assert(len(rvID) > 0, fmt.Sprintf("RV ID is empty for RV %s", rv))

		targetNodeID := getNodeIDForRVName(rv)
		common.Assert(len(targetNodeID) > 0, fmt.Sprintf("target node ID is empty for RV %s", rv))

		log.Debug("ReplicationManager::WriteMV: Writing to RV %s having RV id %s and is hosted in node id %s", rv, rvID, targetNodeID)

		rpcReq := &models.PutChunkRequest{
			Chunk: &models.Chunk{
				Address: &models.Address{
					FileID:     req.FileID,
					FsID:       rvID,
					MvID:       req.MvName,
					OffsetInMB: offsetInMB,
				},
				Data: req.Data,
				Hash: req.Hash,
			},
			Length: req.ChunkSizeInMB * common.MbToBytes,
			IsSync: false, // this is regular client write
			// ComponentRV: componentRVs,
		}

		ctx, cancel := context.WithTimeout(context.Background(), RPCClientTimeout*time.Second)
		defer cancel()

		rpcResp, err := rpc_client.PutChunk(ctx, targetNodeID, rpcReq)
		if err != nil {
			log.Err("ReplicationManager::WriteMV: Failed to put chunk to node %s [%v]", targetNodeID, err)
			common.Assert(false, fmt.Sprintf("failed to put chunk to node %s", targetNodeID))
			// TODO: run degradeMV method to degrade the MV
			return nil, err
		}

		common.Assert(rpcResp != nil, "PutChunk RPC response is nil")
		log.Debug("ReplicationManager::WriteMV: PutChunk RPC response: %+v", *rpcResp)
	}

	return &WriteMvResponse{}, nil
}
