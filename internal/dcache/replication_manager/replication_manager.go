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
	"slices"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

func ReadMV(req *ReadMvRequest) (*ReadMvResponse, error) {
	if req == nil {
		log.Err("ReplicationManager::ReadMV: Received nil ReadMV request")
		common.Assert(false, "received nil ReadMV request")
		return nil, fmt.Errorf("received nil ReadMV request")
	}

	log.Debug("ReplicationManager::ReadMV: Received ReadMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		log.Err("ReplicationManager::ReadMV: Invalid ReadMV request parameters [%v]", err.Error())
		common.Assert(false, fmt.Sprintf("invalid ReadMV request parameters [%v]", err.Error()))
		return nil, err
	}

	var rpcResp *models.GetChunkResponse
	var err error

	clusterMapRefreshed := false

retry:
	componentRVs := getComponentRVsForMV(req.MvName)
	log.Debug("ReplicationManager::ReadMV: Component RVs for the given MV %s are: %v", req.MvName, rpc_server.ComponentRVsToString(componentRVs))

	// Get the most suitable RV from the list of component RVs,
	// from which we should read the chunk. Selecting most
	// suitable RV is mostly a heuristical process which might
	// pick the most suitable RV based on one or more of the
	// following criteria:
	// - Local RV must be preferred.
	// - Prefer a node that has recently responded successfully to any of our RPCs.
	// - Pick a random one.
	var excludeRVs []string
	for {
		readerRV := getReaderRV(componentRVs, excludeRVs)
		if readerRV == nil {
			if clusterMapRefreshed {
				log.Err("ReplicationManager::ReadMV: No suitable RV found for the given MV %s", req.MvName)
				return nil, fmt.Errorf("no suitable RV found for the given MV %s", req.MvName)
			}

			// This is very unlikely and it would most likely indicate that we have a “very stale”
			// clustermap where all/most of the component RVs have been replaced.

			// TODO: will be done later
			// refreshClusterMap()
			clusterMapRefreshed = true
			goto retry
		}

		common.Assert(!slices.Contains(excludeRVs, readerRV.Name), fmt.Sprintf("getReaderRV returned %s which is already present in the excludeRVs list %v", readerRV.Name, excludeRVs))

		selectedRvID := getRvIDFromRvName(readerRV.Name)
		common.Assert(common.IsValidUUID(selectedRvID))

		targetNodeID := getNodeIDFromRVName(readerRV.Name)
		common.Assert(common.IsValidUUID(targetNodeID))

		log.Debug("ReplicationManager::ReadMV: Selected online RV for MV %s is %s having RV id %s and is hosted in node id %s", req.MvName, readerRV.Name, selectedRvID, targetNodeID)

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
		excludeRVs = append(excludeRVs, readerRV.Name)
		if err == nil {
			// success
			common.Assert(rpcResp != nil, fmt.Sprintf("GetChunk RPC response is nil for request %+v", *rpcReq))
			common.Assert(rpcResp.Chunk != nil, fmt.Sprintf("chunk in GetChunk RPC response is nil for request %+v", *rpcReq))
			common.Assert(rpcResp.Chunk.Address != nil, fmt.Sprintf("address of chunk in GetChunk RPC response is nil for request %+v", *rpcReq))
			break
		}

		// TODO: we should handle errors that indicate retrying from a different RV would help.
		// RVs are the final source of truth wrt MV membership (and anything else),
		// so if the target RV feels that the sender seems to have out-of-date clustermap,
		// it can help him by failing the request with an appropriate error and then
		// caller should fetch the latest clustermap and then try again.
		log.Err("ReplicationManager::ReadMV: Failed to get chunk from node %s for request %+v [%v]", targetNodeID, *rpcReq, err.Error())
	}

	log.Debug("ReplicationManager::ReadMV: GetChunk RPC response: chunk lmt %v, time taken %v, component RVs %v, chunk address %+v",
		rpcResp.ChunkWriteTime, rpcResp.TimeTaken, rpcResp.ComponentRV, *rpcResp.Chunk.Address)

	// TODO: this should be deep copy
	n := copy(req.Data, rpcResp.Chunk.Data)
	common.Assert(n == len(rpcResp.Chunk.Data), fmt.Sprintf("data copied %d is not same as data in the chunk length %d", n, len(rpcResp.Chunk.Data)))

	// TODO: in GetChunk RPC request add data buffer to the request
	// TODO: in GetChunk RPC response return bytes read

	// TODO: hash validation will be done later
	// TODO: should we validate the hash of the chunk here?
	// hash := getMD5Sum(rpcResp.Chunk.Data)
	// if hash != rpcResp.Chunk.Hash {
	// 	log.Err("ReplicationManager::ReadMV: Hash mismatch for the chunk read from node %s for request %+v", targetNodeID, *rpcReq)
	// 	common.Assert(false, fmt.Sprintf("hash mismatch for the chunk read from node %s for request %+v", targetNodeID, *rpcReq))
	// 	return nil, fmt.Errorf("hash mismatch for the chunk read from node %s for request %+v", targetNodeID, *rpcReq)
	// }

	resp := &ReadMvResponse{
		// TODO: update this filed after bytes read in response
		BytesRead: int64(len(rpcResp.Chunk.Data)),
	}

	return resp, nil
}

func WriteMV(req *WriteMvRequest) (*WriteMvResponse, error) {
	if req == nil {
		log.Err("ReplicationManager::WriteMV: Received nil WriteMV request")
		common.Assert(false, "received nil WriteMV request")
		return nil, fmt.Errorf("received nil WriteMV request")
	}

	log.Debug("ReplicationManager::WriteMV: Received WriteMV request: %v", req.toString())

	if err := req.isValid(); err != nil {
		log.Err("ReplicationManager::WriteMV: Invalid WriteMV request parameters [%v]", err.Error())
		common.Assert(false, fmt.Sprintf("invalid WriteMV request parameters [%v]", err.Error()))
		return nil, err
	}

	// TODO: TODO: hash validation will be done later
	// get hash of the data in the request
	// hash := getMD5Sum(req.Data)

retry:
	componentRVs := getComponentRVsForMV(req.MvName)
	log.Debug("ReplicationManager::WriteMV: Component RVs for the given MV %s are: %v", req.MvName, rpc_server.ComponentRVsToString(componentRVs))

	// TODO: put chunk to each component RV can be done in parallel
	for _, rv := range componentRVs {
		//  Omit RVs in “offline” or “outofsync” state. It’s ok to omit them as the chunks not written
		//  to them will be copied to them when the mv is (soon) resynced.
		//  Otoh if an RV is in “syncing” state then any new chunk written to it may not be copied by the
		//  ongoing resync operation as the source RV may have been already gone past the enumeration stage
		//  and hence won’t consider this chunk for resync, and hence those MUST have the chunks mandatorily copied to them.

		if rv.State == string(dcache.StateOffline) || rv.State == string(dcache.StateOutOfSync) {
			log.Debug("ReplicationManager::WriteMV: Skipping RV %s having state %s", rv.Name, rv.State)
			continue
		} else if rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing) {
			rvID := getRvIDFromRvName(rv.Name)
			common.Assert(common.IsValidUUID(rvID))

			targetNodeID := getNodeIDFromRVName(rv.Name)
			common.Assert(common.IsValidUUID(targetNodeID))

			log.Debug("ReplicationManager::WriteMV: Writing to RV %s having RV id %s and is hosted in node id %s", rv, rvID, targetNodeID)

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
				log.Err("ReplicationManager::WriteMV: Failed to put chunk to node %s [%v]", targetNodeID, err.Error())
				rpcErr := rpc.GetRPCResponseError(err)
				if rpcErr == nil {
					// this error means that the node is not reachable
					log.Err("ReplicationManager::WriteMV: Failed to reach node %s [%v]", targetNodeID, err.Error())
					return nil, err
				}

				// the error is RPC error of type *rpc.ResponseError
				if rpcErr.Code() == rpc.NeedToRefreshClusterMap {
					// TODO: will be done later
					// refreshClusterMap()
					goto retry
				} else {
					// TODO: check if this is non-retriable error
					log.Err("ReplicationManager::WriteMV: Got non-retriable error for put chunk to node %s [%v]", targetNodeID, err.Error())
					return nil, err
				}
			}

			common.Assert(rpcResp != nil, "PutChunk RPC response is nil")
			log.Debug("ReplicationManager::WriteMV: PutChunk RPC response: %+v", *rpcResp)
		}
	}

	return &WriteMvResponse{}, nil
}
