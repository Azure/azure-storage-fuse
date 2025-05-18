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

package rpc_client

import (
	"context"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

var cp *clientPool

const (
	// TODO: discuss with the team about these values
	// defaultMaxPerNode is the default maximum number of RPC clients created per node
	defaultMaxPerNode = 20
	// defaultMaxNodes is the default maximum number of nodes for which RPC clients are created
	defaultMaxNodes = 100
	// defaultTimeout is the default duration in seconds after which a RPC client is closed
	defaultTimeout = 60
)

// TODO: add asserts for function arguments and return values
// refer this for details, https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047924726

// TODO: templatize the code for all the RPC calls

func Hello(ctx context.Context, targetNodeID string, req *models.HelloRequest) (*models.HelloResponse, error) {
	log.Debug("rpc_client::Hello: Sending Hello request to node %s: %+v", targetNodeID, *req)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::Hello: Failed to get RPC client for node %s [%v] : %+v", targetNodeID, err.Error(), *req)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::Hello: Failed to release RPC client for node %s [%v] : %+v", targetNodeID, err.Error(), *req)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.Hello(ctx, req)
	if err != nil {
		log.Err("rpc_client::Hello: Failed to send Hello request to node %s [%v] : %+v", targetNodeID, err.Error(), *req)
		return nil, err
	}

	return resp, nil
}

func GetChunk(ctx context.Context, targetNodeID string, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	reqStr := rpc.GetChunkRequestToString(req)
	log.Debug("rpc_client::GetChunk: Sending GetChunk request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::GetChunk: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::GetChunk: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.GetChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::GetChunk: Failed to send GetChunk request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	// TODO: add assert for error check in all RPC APIs
	// TODO: if success, add assert that the componentRVs returned in response is same as the one sent in request

	return resp, nil
}

func PutChunk(ctx context.Context, targetNodeID string, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	reqStr := rpc.PutChunkRequestToString(req)
	log.Debug("rpc_client::PutChunk: Sending PutChunk request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::PutChunk: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::PutChunk: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.PutChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::PutChunk: Failed to send PutChunk request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

func RemoveChunk(ctx context.Context, targetNodeID string, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	reqStr := rpc.RemoveChunkRequestToString(req)
	log.Debug("rpc_client::RemoveChunk: Sending RemoveChunk request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::RemoveChunk: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::RemoveChunk: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.RemoveChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::RemoveChunk: Failed to send RemoveChunk request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

func JoinMV(ctx context.Context, targetNodeID string, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	reqStr := rpc.JoinMVRequestToString(req)
	log.Debug("rpc_client::JoinMV: Sending JoinMV request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::JoinMV: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::JoinMV: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.JoinMV(ctx, req)
	if err != nil {
		log.Err("rpc_client::JoinMV: Failed to send JoinMV request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

func UpdateMV(ctx context.Context, targetNodeID string, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	reqStr := rpc.UpdateMVRequestToString(req)
	log.Debug("rpc_client::UpdateMV: Sending UpdateMV request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::UpdateMV: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::UpdateMV: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.UpdateMV(ctx, req)
	if err != nil {
		log.Err("rpc_client::UpdateMV: Failed to send UpdateMV request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

func LeaveMV(ctx context.Context, targetNodeID string, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	reqStr := rpc.LeaveMVRequestToString(req)
	log.Debug("rpc_client::LeaveMV: Sending LeaveMV request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::LeaveMV: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::LeaveMV: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.LeaveMV(ctx, req)
	if err != nil {
		log.Err("rpc_client::LeaveMV: Failed to send LeaveMV request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

func StartSync(ctx context.Context, targetNodeID string, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	reqStr := rpc.StartSyncRequestToString(req)
	log.Debug("rpc_client::StartSync: Sending StartSync request to node %s: %v", targetNodeID, reqStr)

	// Get an RPC client from the client pool, for making the StartSync RPC call.
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::StartSync: Failed to get RPC client for node %s [%v] : %v",
			targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	defer func() {
		//
		// If the RPC call failed with an error we assume something wrong with the client and close
		// it, else release it back to the pool.
		//
		// TODO: See if we should close only on TCP error signifying socket is not connected.
		// TODO: Do this for other RPC messages too.
		//
		if err == nil {
			// Release RPC client back to the pool.
			err1 := cp.releaseRPCClient(client)
			// Release client should not fail.
			common.Assert(err1 == nil, err1)
		} else {
			err1 := cp.resetRPCClient(client)
			// Reset client should not fail.
			common.Assert(err1 == nil, err1)
		}
	}()

	// Call the rpc method.
	resp, err := client.svcClient.StartSync(ctx, req)
	if err != nil {
		log.Err("rpc_client::StartSync: Failed to send StartSync request to node %s [%v]: %v",
			targetNodeID, err, reqStr)
		return nil, err
	}

	return resp, nil
}

func EndSync(ctx context.Context, targetNodeID string, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	reqStr := rpc.EndSyncRequestToString(req)
	log.Debug("rpc_client::EndSync: Sending EndSync request to node %s: %v", targetNodeID, reqStr)

	// get RPC client from the client pool
	client, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::EndSync: Failed to get RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}
	defer func() {
		// release RPC client back to the pool
		err = cp.releaseRPCClient(client)
		if err != nil {
			log.Err("rpc_client::EndSync: Failed to release RPC client for node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		}
	}()

	// call the rpc method
	resp, err := client.svcClient.EndSync(ctx, req)
	if err != nil {
		log.Err("rpc_client::EndSync: Failed to send EndSync request to node %s [%v] : %v", targetNodeID, err.Error(), reqStr)
		return nil, err
	}

	return resp, nil
}

// cleanup closes all the RPC node client pools
func Cleanup() error {
	log.Info("rpc_client::Cleanup: Closing all node client pools")
	err := cp.closeAllNodeClientPools()
	if err != nil {
		log.Err("rpc_client::Cleanup: Failed to close all node client pools [%v]", err.Error())
	}

	return err
}

func init() {
	log.Info("rpc_client::init: package initialized, create client pool")
	cp = newClientPool(defaultMaxPerNode, defaultMaxNodes, defaultTimeout)
}
