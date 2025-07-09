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
	"os"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	cp       *clientPool
	myNodeId string
)

const (
	//
	// defaultMaxPerNode is the default maximum number of RPC clients created per target node
	// Every RPC client creates a TCP connection to the target node RPC server port, so these
	// many TCP connections can be active at any time to one node.
	// Once RPC client remains used for the duration of sending the RPC request and till the
	// response is returned, so these many active RPC requests can be outstanding to one target
	// node at any time.
	// The heaviest and longest use of an RPC connection would be IO calls like GetChunk and
	// PutChunk. Multiple of these calls can be outstanding against a target node for:
	// - Reading multiple chunks of same/different files hosted by one or more RV(s) on that node.
	//   This could be due to readahead or writeback cache or resync writes.
	// - Other RPC requests to that node. These won't be too many and are not so time critical.
	//
	// Since these many simultaneous chunk IOs (read/write) can be running on the target node,
	// this value actually depends on number of RVs exported by a node. Since we don't know it
	// at init time, we assume a fair value of say 4 and for each of those RVs, keeping anything
	// more than 8 chunk sized IOs won't be useful. So we set the default to 32.
	//
	defaultMaxPerNode = 32

	//
	// defaultMaxNodes is the default maximum number of nodes for which RPC clients are created
	// and stored. If we are sending simultaneous RPC requests to more than these many nodes, we
	// will need to evict RPC clients for the node that we sent an RPC longest time back.
	// We want this number to be more than the possible number of nodes to which we will be sending
	// RPCs under most conditions, else we will spend too much time creating connections.
	// How many nodes will we typically interact with?
	// Since GetChunk and PutChunk are the most common RPCs, we need to see how many nodes will
	// host RVs of files that we will be reading and writing. Depending on the number of files
	// being simultaneously read/written, this number will vary but we can pick a number that's
	// a decent fraction of the max nodes expected.
	//
	defaultMaxNodes = 1000

	//
	// defaultTimeout is the default duration in seconds after which an idle RPC client is closed.
	//
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
			if rpc.IsBrokenPipe(err) {
				//
				// Note: In case of multiple contexts contesting, we may have those contexts
				//	 reset "good" connections too. See if we need to worry about that.
				//
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::Hello: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry Hello once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::GetChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry GetChunk once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::PutChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry PutChunk once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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

// This is the daisy chain PutChunk function.
// Unlike PutChunk() that writes a chunk to one component RV, PutChunkDC() writes the chunk to one RV (called
// the nexthop RV) and additionally provides a list of other component RVs to which the chunk must be written.
// The node receiving the PutChunkDCRequest (which hosts the nexthop RV) writes the chunk to its local RV, and
// relays the request to the new nexthop along with the remaining RV list. This goes on in a daisy chain fashion
// till all the component RVs are covered.
func PutChunkDC(ctx context.Context, targetNodeID string, req *models.PutChunkDCRequest) (*models.PutChunkDCResponse, error) {
	common.Assert(req != nil &&
		req.Request != nil &&
		req.Request.Chunk != nil &&
		req.Request.Chunk.Address != nil)

	// Caller must call PutChunkDC() only if it wants the request to be daisy chained to at least one more RV.
	common.Assert(len(req.NextRVs) > 0)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.Request.SenderNodeID) == 0, req.Request.SenderNodeID)
	req.Request.SenderNodeID = myNodeId

	reqStr := rpc.PutChunkDCRequestToString(req)
	log.Debug("rpc_client::PutChunkDC: Sending PutChunkDC request to nexthop node %s and %d daisy chain RV(s): %v",
		targetNodeID, len(req.NextRVs), reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::PutChunkDC: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.PutChunkDC(ctx, req)
		if err != nil {
			log.Err("rpc_client::PutChunkDC: PutChunkDC failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::PutChunkDC: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry PutChunkDC once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::PutChunkDC: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the PutChunkDC call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::PutChunkDC: Could not find a valid RPC client for node %s %v",
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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::RemoveChunk: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry RemoveChunk once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			//
			// This code is special only for JoinMV and specifically for the new-mv case.
			// Note that ClusterManager.start() has a tiny window where it publishes its RVs into the
			// clustermap but it has not started the RPC server yet.
			// If some other node starts a new-mv workflow in the meantime, its attempt to create RPC
			// client connections will fail with connection refused.
			// Retry after a small wait.
			//
			log.Info("rpc_client::JoinMV: Retrying after 5 secs in case the RPC server is just starting on the target")
			time.Sleep(5 * time.Second)
			continue
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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::JoinMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry JoinMV once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::UpdateMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry UpdateMV once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::LeaveMV: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry LeaveMV once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::StartSync: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry StartSync once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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
			if rpc.IsBrokenPipe(err) {
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::EndSync: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry EndSync once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

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

// TODO:: integration : use this API in the fix-mv workflow to get the size of the MV
// while making JoinMV calls to new online RVs
func GetMVSize(ctx context.Context, targetNodeID string, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	reqStr := rpc.GetMVSizeRequestToString(req)
	log.Debug("rpc_client::GetMVSize: Sending GetMVSize request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			log.Err("rpc_client::GetMVSize: Failed to get RPC client for node %s %v: %v",
				targetNodeID, reqStr, err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.GetMVSize(ctx, req)
		if err != nil {
			log.Err("rpc_client::GetMVSize: GetMVSize failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// If the failure is due to a stale connection to a node that has restarted, reset the connections
			// and retry once more.
			//
			if rpc.IsBrokenPipe(err) {
				//
				// Note: In case of multiple contexts contesting, we may have those contexts
				//	 reset "good" connections too. See if we need to worry about that.
				//
				err1 := cp.resetAllRPCClients(client)
				if err1 != nil {
					log.Err("rpc_client::GetMVSize: resetAllRPCClients failed for node %s: %v",
						targetNodeID, err1)
					//
					// Connection refused and timeout are the only viable errors.
					// Assert to know if anything else happens.
					//
					common.Assert(rpc.IsConnectionRefused(err1) || rpc.IsTimedOut(err1), err1)
					return nil, err
				}

				// Retry GetMVSize once more with fresh connection.
				continue
			}

			//
			// Only other possible errors:
			// - Actual RPC error returned by the server.
			// - Connection closed by the server (maybe it restarted before it could respond).
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - Time out (either node is down or cannot be reached over the n/w).
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			// Fall through to release the RPC client.
			resp = nil
		}

		// Release RPC client back to the pool.
		err1 := cp.releaseRPCClient(client)
		if err1 != nil {
			log.Err("rpc_client::GetMVSize: Failed to release RPC client for node %s %v: %v",
				targetNodeID, reqStr, err1)
			// Assert, but not fail the GetMVSize call.
			common.Assert(false, err1)
		}

		return resp, err
	}

	//
	// We come here when we could not succeed even after resetting stale connections and retrying.
	// This is unexpected, but can happen if the target node goes offline or restarts more than once in
	// quick succession.
	//
	return nil, fmt.Errorf("rpc_client::GetMVSize: Could not find a valid RPC client for node %s %v",
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

func Start() {
	// Must be called only once.
	common.Assert(len(myNodeId) == 0)

	// Init function is called before mount.go where this directory is created
	// so we need to create it here.
	if err := os.MkdirAll(common.DefaultWorkDir, 0777); err != nil && !os.IsExist(err) {
		log.GetLoggerObj().Panicf("rpc_client::init: PANIC: failed to create default work directory at %s : %v", common.DefaultWorkDir, err)
	}

	var err error
	myNodeId, err = common.GetNodeUUID()
	if err != nil {
		// Cannot proceed w/o our node id.
		log.GetLoggerObj().Panicf("rpc_client::init: PANIC: failed to get my node id [%v]", err)
	}

	common.Assert(common.IsValidUUID(myNodeId), myNodeId)

	cp = newClientPool(defaultMaxPerNode, defaultMaxNodes, defaultTimeout)
	common.Assert(cp != nil)

	log.Info("rpc_client::init: myNodeId: %s, maxNodes: %d, maxPerNode: %d, timeout: %d",
		myNodeId, defaultMaxNodes, defaultMaxPerNode, defaultTimeout)
}
