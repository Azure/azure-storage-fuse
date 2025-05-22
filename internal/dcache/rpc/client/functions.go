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
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

var (
	cp       *clientPool
	myNodeId string
)

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
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.HelloRequestToString(req)
	log.Debug("rpc_client::Hello: Sending Hello request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::Hello: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.Hello(ctx, req)
		if err != nil {
			log.Err("rpc_client::Hello: Hello failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				//
				// Note: In case of multiple contexts contesting, we may have those contexts
				//	 reset "good" connections too. See if we need to worry about that.
				//
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::Hello: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry Hello once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::Hello: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the Hello call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::Hello: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func GetChunk(ctx context.Context, targetNodeID string, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	common.Assert(req != nil && req.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.GetChunkRequestToString(req)
	log.Debug("rpc_client::GetChunk: Sending GetChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::GetChunk: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.GetChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::GetChunk: GetChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::GetChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					return nil, err
				}

				// Retry GetChunk once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::GetChunk: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the GetChunk call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::GetChunk: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func PutChunk(ctx context.Context, targetNodeID string, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	common.Assert(req != nil && req.Chunk != nil && req.Chunk.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.PutChunkRequestToString(req)
	log.Debug("rpc_client::PutChunk: Sending PutChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::PutChunk: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.PutChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::PutChunk: PutChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::PutChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry PutChunk once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::PutChunk: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the PutChunk call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::PutChunk: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func RemoveChunk(ctx context.Context, targetNodeID string, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	common.Assert(req != nil && req.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.RemoveChunkRequestToString(req)
	log.Debug("rpc_client::RemoveChunk: Sending RemoveChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::RemoveChunk: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.RemoveChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::RemoveChunk: RemoveChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::RemoveChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry RemoveChunk once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::RemoveChunk: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the RemoveChunk call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::RemoveChunk: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func JoinMV(ctx context.Context, targetNodeID string, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.JoinMVRequestToString(req)
	log.Debug("rpc_client::JoinMV: Sending JoinMV request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::JoinMV: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.JoinMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::JoinMV: JoinMV failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::JoinMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry JoinMV once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::JoinMV: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the JoinMV call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::JoinMV: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func UpdateMV(ctx context.Context, targetNodeID string, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.UpdateMVRequestToString(req)
	log.Debug("rpc_client::UpdateMV: Sending UpdateMV request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::UpdateMV: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.UpdateMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::UpdateMV: UpdateMV failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::UpdateMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry UpdateMV once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::UpdateMV: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the UpdateMV call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::UpdateMV: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func LeaveMV(ctx context.Context, targetNodeID string, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.LeaveMVRequestToString(req)
	log.Debug("rpc_client::LeaveMV: Sending LeaveMV request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::LeaveMV: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.LeaveMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::LeaveMV: LeaveMV failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::LeaveMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry LeaveMV once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::LeaveMV: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the LeaveMV call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::LeaveMV: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func StartSync(ctx context.Context, targetNodeID string, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.StartSyncRequestToString(req)
	log.Debug("rpc_client::StartSync: Sending StartSync request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::StartSync: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.StartSync(ctx, req)
		if err != nil {
			log.Err("rpc_client::StartSync: StartSync failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::StartSync: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry StartSync once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::StartSync: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the StartSync call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::StartSync: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
}

func EndSync(ctx context.Context, targetNodeID string, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.EndSyncRequestToString(req)
	log.Debug("rpc_client::EndSync: Sending EndSync request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::EndSync: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.EndSync(ctx, req)
		if err != nil {
			log.Err("rpc_client::EndSync: EndSync failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsConnectionClosed(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::EndSync: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					// resetAllRPCClients() may fail but unlikely, so assert.
					common.Assert(false, err1)
					return nil, err
				}

				// Retry EndSync once more with fresh connection.
				continue
			}

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::EndSync: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the EndSync call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::EndSync: Could not find a valid RPC client for node %s %v",
		targetNodeID, reqStr)
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
	// Must be called only once.
	common.Assert(len(myNodeId) == 0)

	myNodeId, err := common.GetNodeUUID()
	if err != nil {
		// Cannot proceed w/o our node id.
		log.GetLoggerObj().Panicf("rpc_client::init: PANIC: failed to get my node id [%v]", err)
	}

	cp = newClientPool(defaultMaxPerNode, defaultMaxNodes, defaultTimeout)
	common.Assert(cp != nil)

	log.Info("rpc_client::init: myNodeId: %s, maxNodes: %d, maxPerNode: %d, timeout: %d",
		myNodeId, defaultMaxNodes, defaultMaxPerNode, defaultTimeout)
}
