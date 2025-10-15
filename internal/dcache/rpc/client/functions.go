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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

//
// This file has functions that clients can call to send RPCs to target nodes.
// The functions here get an RPC client from the client pool, call the necessary RPC client method from the
// thrift generated code, handle errors, and release the client back to the pool.
//
// Caller can check if the error returned is NoFreeRPCClient error, which means the call failed as we couldn't
// get a valid client to make the call. This indicates a serious issue like target node down or unreachable due
// to network issue, where retrying the operation (soon) is not likely to succeed. The correct action is to ensure
// that the node/RV is marked offline in the clustermap and we don't attempt to access it till it comes back online.
//

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
	// A better way to look at it is, how many parallel TCP connections do we need to saturate the
	// n/w bandwidth between two nodes (regardless of the RVs hosted by a node). Again, 8 should be
	// sufficient for that.
	//
	// Note: 64 is seen to perform better, since we only have 16 regular clients which are used for
	//       all operations other than PutChunkDC from forwardPutChunk().
	// Note: If you modify this, also modify PutChunkDCIODepthTotal and PutChunkDCIODepthPerNode.
	//
	defaultMaxPerNode = 96

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
	// We pre-initialize nodeClientPool for these many nodes.
	// Must be more than the max nodes in the biggest cluster size we want to support.
	//
	// TODO: This is a temporary fix till we move to gRPC.
	//
	staticMaxNodes = 10000

	//
	// defaultTimeout is the default duration in seconds after which an idle RPC client is closed.
	//
	defaultTimeout = 60

	//
	// defaultNegativeTimeout is the default duration in seconds after which the node ID is deleted
	// from the negative clients map and its RPC client creation is attempted again.
	// We want to keep it large enough so that clients trying to connect in close proximity make use of
	// this information and not attempt to connect to an unreachable node, potentially delaying the
	// caller, but small enough to promptly attempt connection to a node that might have now come up.
	//
	// Ideally if a node is down and we figured out by RPC failing, our higher level workflows should not
	// attempt to access the node/RV till it comes back up.
	// Keep it more than the heartbeat timeout to avoid premature retries.
	//
	defaultNegativeTimeout = 15
)

var (
	NegativeNodeError = errors.New("node is marked negative")
	IffyRVError       = errors.New("RV is marked iffy")
	NoFreeRPCClient   = errors.New("no free RPC client")
)

// TODO: add asserts for function arguments and return values
// refer this for details, https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047924726

// TODO: templatize the code for all the RPC calls
func Hello(ctx context.Context, targetNodeID string, req *models.HelloRequest) (*models.HelloResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	// Caller cannot send a clustermap epoch greater than what we have.
	common.Assert(req.ClustermapEpoch <= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.HelloRequestToString(req)
	log.Debug("rpc_client::Hello: Sending Hello request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		//
		// Get RPC client from the client pool.
		// For all other RPCs other than PutChunkDC called from forwardPutChunk(), we use the regular
		// priority client quota as we want to keep clients available for forwardPutChunk() calls if
		// needed to prevent delays in PutChunkDC completions that can potentially cause timeouts.
		//
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::Hello: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.Hello(ctx, req)
		if err != nil {
			log.Err("rpc_client::Hello: Hello failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node is marked negative, clear it now.
			//
			// TODO: Remove RVs hosted by the target node from iffyRvIdMap.
			//
			cp.removeNegativeNode(targetNodeID)
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

	// Caller cannot send a clustermap epoch greater than what we have.
	common.Assert(req.ClustermapEpoch <= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.GetChunkRequestToString(req)
	log.Debug("rpc_client::GetChunk: Sending GetChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::GetChunk: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.GetChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::GetChunk: GetChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvId(req.Address.RvID)
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

func PutChunk(ctx context.Context, targetNodeID string, req *models.PutChunkRequest, fromFwder bool) (*models.PutChunkResponse, error) {
	common.Assert(req != nil && req.Chunk != nil && req.Chunk.Address != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	// All PutChunk requests must carry a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	//
	// Caller cannot send a clustermap epoch greater than what we have.
	// When called from forwardPutChunk() we cannot assert this as req.Request.ClustermapEpoch is the one sent
	// by the originator and could be greater than what we have if our clustermap is stale.
	//
	common.Assert(fromFwder || (req.ClustermapEpoch <= cm.GetEpoch()),
		req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.PutChunkRequestToString(req)
	log.Debug("rpc_client::PutChunk: Sending PutChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::PutChunk: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.PutChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::PutChunk: PutChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvId(req.Chunk.Address.RvID)
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
func PutChunkDC(ctx context.Context, targetNodeID string, req *models.PutChunkDCRequest, fromFwder bool) (*models.PutChunkDCResponse, error) {
	common.Assert(req != nil &&
		req.Request != nil &&
		req.Request.Chunk != nil &&
		req.Request.Chunk.Address != nil)

	// Caller must call PutChunkDC() only if it wants the request to be daisy chained to at least one more RV.
	common.Assert(len(req.NextRVs) > 0)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.Request.SenderNodeID) == 0, req.Request.SenderNodeID)
	req.Request.SenderNodeID = myNodeId

	// All PutChunkDC requests must carry a valid clustermap epoch.
	common.Assert(req.Request.ClustermapEpoch > 0, req.Request.ClustermapEpoch)
	//
	// Caller cannot send a clustermap epoch greater than what we have.
	// When called from forwardPutChunk() we cannot assert this as req.Request.ClustermapEpoch is the one sent
	// by the originator and could be greater than what we have if our clustermap is stale.
	//
	common.Assert(fromFwder || (req.Request.ClustermapEpoch <= cm.GetEpoch()),
		req.Request.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.PutChunkDCRequestToString(req)
	log.Debug("rpc_client::PutChunkDC: Sending PutChunkDC (fromFwder: %v) request to nexthop node %s and %d daisy chain RV(s): %v",
		fromFwder, targetNodeID, len(req.NextRVs), reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		//
		// Get RPC client from the client pool.
		// If the target node is marked negative, getRPCClient() will return NegativeNodeError.
		// This indicates the caller (WriteMV) to retry the operation using OriginatorSendsToAll,
		// which will do PutChunk to each RV instead of PutChunkDC to just he nexthop RV.
		// PutChunk to this negative node will most likely still fail to get an RPC client, but other
		// nodes may succeed. The negative node will be marked inband-offline and will be replaced
		// by fix-mv.
		//
		// If this call is made from forwardPutChunk() we need to dig into the higher priority quota,
		// as blocking a forwardPutChunk() will block the entire daisy chain operation, which will keep
		// many RPC clients busy, across various node. This can lead to a deadlock if
		// forwardPutChunk()->PutChunkDC()->getRPCClient() blocks waiting for a free RPC client and all
		// the clients are busy waiting for forwardPutChunk() calls to complete.
		//
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::PutChunkDC: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		//
		// If the next-hop RV is marked iffy, it means that the last PutChunkDC call to it failed
		// with timeout error. So, prevent timeout error from happening again, we return an error
		// indicating the RV is iffy. The caller (WriteMV) will then retry the operation using
		// OriginatorSendsToAll mode.
		//
		if cp.isIffyRvId(req.Request.Chunk.Address.RvID) {
			//
			// Release RPC client back to the pool.
			//
			err1 := cp.releaseRPCClient(client)
			if err1 != nil {
				log.Err("rpc_client::PutChunkDC: Failed to release RPC client for node %s %s: %v",
					targetNodeID, reqStr, err1)
				common.Assert(false, err1)
			}

			err1 = fmt.Errorf("Failing PutChunkDC to RV id %s, MV %s on node %s [%w]: %s",
				req.Request.Chunk.Address.RvID, req.Request.Chunk.Address.MvName,
				targetNodeID, IffyRVError, reqStr)

			log.Err("rpc_client::PutChunkDC: %v", err1)
			return nil, err1
		}

		// Call the rpc method.
		resp, err := client.svcClient.PutChunkDC(ctx, req)
		if err != nil {
			log.Err("rpc_client::PutChunkDC: PutChunkDC failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				//
				// If we get timeout error in PutChunkDC(), it means that one/more of the downstream
				// nodes/connections are down/bad. We cannot say for sure which node is down or which
				// connection is bad.
				// So, we mark all the RVs (next-hop as well as next RVs in chain) as iffy RVs.
				// This helps other threads wanting to call PutChunkDC to one of these RVs, avoid the
				// timeout and quickly fallback to OriginatorSendsToAll mode.
				//
				cp.addIffyRvId(req.Request.Chunk.Address.RvID)

				// Add the next RVs to the iffy RVs map.
				for _, nextRV := range req.NextRVs {
					cp.addIffyRvName(nextRV)
				}

				//
				// In PutChunkDC we can get timeout because of bad connection between the downstream nodes,
				// and not necessarily between the client node and target node. In this case, we retry WriteMV using
				// the OriginatorSendsToAll strategy. So, to be safe we don't delete the connections if we get
				// timeout error in PutChunkDC. In the PutChunk using OriginatorSendsToAll strategy fails using
				// timeout, we then delete the connections.
				//
				// We reset the RPC client here because if the connection between client and target node is good,
				// the response from target node will eventually return after the timeout error. In this case, we
				// cannot reuse the same client which was timed out as it will result in ambiguous behavior as the
				// next caller will fetch that error from the previous call.
				// So, we reset this client for the target node.
				//
				// If the reset fails to establish a new connection, we press the panic button and delete all
				// connections to the target node. This will force creation of a new connection when needed.
				//
				err1 := cp.resetRPCClient(client)
				if err1 != nil {
					log.Err("rpc_client::PutChunkDC: resetRPCClient failed for node %s: %v",
						targetNodeID, err1)

					//
					// The client has already been closed in resetRPCClient().
					// So, we pass true for isClientClosed flag.
					//
					cp.deleteAllRPCClients(client, true /* confirmedBadNode */, true /* isClientClosed */)
				}

				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvId(req.Request.Chunk.Address.RvID)

			// Remove the next RVs from the iffy RVs map.
			for _, nextRV := range req.NextRVs {
				cp.removeIffyRvName(nextRV)
			}
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

	// All RemoveChunk requests must carry a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	// Caller cannot send a clustermap epoch greater than what we have.
	common.Assert(req.ClustermapEpoch <= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.RemoveChunkRequestToString(req)
	log.Debug("rpc_client::RemoveChunk: Sending RemoveChunk request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::RemoveChunk: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.RemoveChunk(ctx, req)
		if err != nil {
			log.Err("rpc_client::RemoveChunk: RemoveChunk failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvId(req.Address.RvID)
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

func JoinMV(ctx context.Context, targetNodeID string, req *models.JoinMVRequest, newMV bool) (*models.JoinMVResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	// All JoinMV requests must carry a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	//
	// See joinMV() comments in cluster_manager.go for details on the following asserts.
	//
	common.Assert(req.ClustermapEpoch%2 == 1, req.ClustermapEpoch, cm.GetEpoch())
	common.Assert(req.ClustermapEpoch == cm.GetEpoch() || req.ClustermapEpoch == cm.GetEpoch()+1,
		req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.JoinMVRequestToString(req)
	log.Debug("rpc_client::JoinMV: Sending JoinMV request (newMV: %v) to node %s: %v", newMV, targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::JoinMV: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)

			//
			// TODO: The following code is not right, as retrying after a wait will not help if the
			//       connection creation fails, as we would have added the node to negative list and
			//       getRPCClient() will fail fast, but we need to handle the issue described below.
			//       Without this what will happen is that this new node will not be inducted into
			//       the cluster till the next clustermap epoch, which is not very bad too.
			//
			/*
				//
				// This code is special only for JoinMV and specifically for the new-mv case.
				// Note that ClusterManager.start() has a tiny window where it publishes its RVs into the
				// clustermap but it has not started the RPC server yet.
				// If some other node starts a new-mv workflow in the meantime, its attempt to create RPC
				// client connections will fail with connection refused.
				// Retry after a small wait.
				//
				if newMV {
					log.Info("rpc_client::JoinMV: Retrying after 2 secs in case the RPC server is just starting on the target")
					time.Sleep(2 * time.Second)
					continue
				}
			*/
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.JoinMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::JoinMV: JoinMV (newMV: %v) failed to node %s %v: %v",
				newMV, targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvName(req.RVName)
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

	// All UpdateMV requests must carry a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	//
	// See joinMV() comments in cluster_manager.go for details on the following asserts.
	//
	common.Assert(req.ClustermapEpoch%2 == 1, req.ClustermapEpoch, cm.GetEpoch())
	common.Assert(req.ClustermapEpoch == cm.GetEpoch() || req.ClustermapEpoch == cm.GetEpoch()+1,
		req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.UpdateMVRequestToString(req)
	log.Debug("rpc_client::UpdateMV: Sending UpdateMV request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::UpdateMV: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.UpdateMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::UpdateMV: UpdateMV failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvName(req.RVName)
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

	// Caller cannot send a clustermap epoch greater than what we have.
	common.Assert(req.ClustermapEpoch <= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	reqStr := rpc.LeaveMVRequestToString(req)
	log.Debug("rpc_client::LeaveMV: Sending LeaveMV request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for i := 0; i < 2; i++ {
		// Get RPC client from the client pool.
		client, err := cp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::LeaveMV: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.LeaveMV(ctx, req)
		if err != nil {
			log.Err("rpc_client::LeaveMV: LeaveMV failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvName(req.RVName)
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

// TODO:: integration : use this API in the fix-mv workflow to get the size of the MV
// while making JoinMV calls to new online RVs
func GetMVSize(ctx context.Context, targetNodeID string, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	common.Assert(req != nil)

	//
	// All GetMVSize requests must carry a valid clustermap epoch.
	// Other then this we cannot assert anything about the clustermap epoch since GetMVSize() can be called
	// from joinMV() and also syncMV(). For joinMV() the epoch must be odd and == cm.GetEpoch()+1, as it's
	// called from updateMVList() which locks the clustermap. We cannot say the same for syncMV().
	//
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
			err = fmt.Errorf("rpc_client::GetMVSize: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.GetMVSize(ctx, req)
		if err != nil {
			log.Err("rpc_client::GetMVSize: GetMVSize failed to node %s %v: %v",
				targetNodeID, reqStr, err)

			//
			// Only possible errors:
			// - Actual RPC error returned by the server.
			// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
			// - Connection closed by the server (maybe it restarted before it could respond).
			//   In this case we could send the request before the blobfuse2 process stopped but it
			//   stopped before it could respond.
			// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
			//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
			// - TimedOut means the node is down or cannot be reached over the n/w.
			//
			// All other errors other than RPC error indicate some problem with the target node or the
			// n/w, so we delete all existing connections to the node, prohibit new connections for a
			// short period and then create new connections when needed.
			//
			// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
			//
			common.Assert(rpc.IsRPCError(err) ||
				rpc.IsBrokenPipe(err) ||
				rpc.IsConnectionClosed(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err), err)

			if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
				//
				// Common reason for first time error could be that we have old connections and since
				// then blobfuse2 process or the node has restarted causing those connections to fail
				// with broken pipe or connection closed/reset errors, so first time around we don't
				// mark the node as negative, but if retrying also fails with similar error, it means the
				// blobfuse2 process is still down so we mark it as negative.
				//
				cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
				if i == 1 {
					return nil, err
				}
				err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
				if err1 != nil {
					return nil, err
				}
				// Continue with newly created client.
				continue
			} else if rpc.IsTimedOut(err) {
				cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
				return nil, err
			}

			// Fall through to release the RPC client.
			resp = nil
		} else {
			//
			// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
			// clear it now.
			//
			cp.removeNegativeNode(targetNodeID)
			cp.removeIffyRvName(req.RVName)
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

// GetLogs fetches log tarball from target node and writes it to <outDir>/<tarName>.
// Returns full path to the written file.
func GetLogs(ctx context.Context, targetNodeID, outDir string, chunkSize int64) (*string, error) {
	common.Assert(chunkSize > 0 && chunkSize <= rpc.MaxLogChunkSize, chunkSize)

	var outPath string
	var fh *os.File
	defer func() {
		if fh != nil {
			fh.Close()
		}
	}()

	for chunkIndex := int64(0); ; chunkIndex++ {
		req := &models.GetLogsRequest{
			SenderNodeID: myNodeId,
			ChunkIndex:   chunkIndex,
			ChunkSize:    chunkSize,
			Reset:        chunkIndex == 0, // Create new log tarball for first request
		}

		reqStr := req.String()
		log.Debug("rpc_client::GetLogs: Sending GetLogs request to node %s: %s", targetNodeID, reqStr)

		var resp *models.GetLogsResponse
		var err error

		//
		// We retry once after resetting bad connections.
		//
		for i := 0; i < 2; i++ {
			// Get RPC client from the client pool.
			var client *rpcClient
			client, err = cp.getRPCClient(targetNodeID)
			if err != nil {
				err = fmt.Errorf("rpc_client::GetLogs: Failed to get RPC client for node %s %v: %v [%w]",
					targetNodeID, reqStr, err, NoFreeRPCClient)
				log.Err("%v", err)
				break
			}

			// Call the rpc method.
			resp, err = client.svcClient.GetLogs(ctx, req)
			if err != nil {
				log.Err("rpc_client::GetLogs: GetLogs failed to node %s %v: %v",
					targetNodeID, reqStr, err)

				//
				// Only possible errors:
				// - Actual RPC error returned by the server.
				// - Broken pipe means we attempted to write the RPC request after the blobfuse2 process stopped.
				// - Connection closed by the server (maybe it restarted before it could respond).
				//   In this case we could send the request before the blobfuse2 process stopped but it
				//   stopped before it could respond.
				// - Connection reset by the server (same as above, but peer send a TCP RST instead of FIN).
				//   Only read()/recv() can fail with this, write()/send() will fail with broken pipe.
				// - TimedOut means the node is down or cannot be reached over the n/w.
				//
				// All other errors other than RPC error indicate some problem with the target node or the
				// n/w, so we delete all existing connections to the node, prohibit new connections for a
				// short period and then create new connections when needed.
				//
				// TODO: See if we need to optimize any of these cases, i.e., don't delete all connections.
				//
				common.Assert(rpc.IsRPCError(err) ||
					rpc.IsBrokenPipe(err) ||
					rpc.IsConnectionClosed(err) ||
					rpc.IsConnectionReset(err) ||
					rpc.IsTimedOut(err), err)

				if rpc.IsBrokenPipe(err) || rpc.IsConnectionClosed(err) || rpc.IsConnectionReset(err) {
					//
					// Common reason for first time error could be that we have old connections and since
					// then blobfuse2 process or the node has restarted causing those connections to fail
					// with broken pipe or connection closed/reset errors, so first time around we don't
					// mark the node as negative, but if retrying also fails with similar error, it means the
					// blobfuse2 process is still down so we mark it as negative.
					//
					cp.deleteAllRPCClients(client, i == 1 /* confirmedBadNode */, false /* isClientClosed */)
					if i == 1 {
						break
					}
					err1 := cp.waitForNodeClientPoolToDelete(client.nodeID, client.nodeIDInt)
					if err1 != nil {
						break
					}
					// Continue with newly created client.
					continue
				} else if rpc.IsTimedOut(err) {
					cp.deleteAllRPCClients(client, true /* confirmedBadNode */, false /* isClientClosed */)
					break
				}

				// Fall through to release the RPC client.
				resp = nil
			} else {
				//
				// The RPC call to the target node succeeded. If the node or the RV is marked negative or iffy,
				// clear it now.
				//
				cp.removeNegativeNode(targetNodeID)
			}

			// Release RPC client back to the pool.
			err1 := cp.releaseRPCClient(client)
			if err1 != nil {
				log.Err("rpc_client::GetLogs: Failed to release RPC client for node %s %v: %v",
					targetNodeID, reqStr, err1)
				// Assert, but not fail the GetLogs call.
				common.Assert(false, err1)
			}
		}

		//
		// If the GetLogs RPC failed, the log tar file was written partially.
		// So, we delete this file.
		//
		if err != nil {
			common.Assert(resp == nil)

			// We create the output file when chunkIndex is 0.
			if chunkIndex == 0 {
				common.Assert(fh == nil, reqStr)
				common.Assert(len(outPath) == 0, reqStr)
			} else {
				common.Assert(fh != nil, reqStr)
				common.Assert(len(outPath) > 0, reqStr)

				// Delete the output file.
				err1 := os.Remove(outPath)
				_ = err1
				common.Assert(err1 == nil, err1, outPath, reqStr)
			}

			return nil, err
		}

		common.Assert(resp != nil, reqStr)
		common.Assert(len(resp.Data) > 0, reqStr, resp.ChunkIndex, resp.IsLast, resp.TarName, resp.TotalSize)

		//
		// For local node, the first GetLogs RPC call for chunkIndex=0 will create the log tarball
		// in /tmp/<tarName> path. So, we don't make RPC calls again for next chunks as we can directly
		// copy the local file from /tmp/<tarName> to <outDir>/<tarName>.
		//
		if targetNodeID == rpc.GetMyNodeUUID() {
			common.Assert(chunkIndex == 0, reqStr)

			srcPath := filepath.Join(os.TempDir(), resp.TarName)
			outPath = filepath.Join(outDir, resp.TarName)

			log.Debug("rpc_client::GetLogs: Copying local log tar file from %s -> %s", srcPath, outPath)
			err = copyFile(srcPath, outPath)
			if err != nil {
				log.Err("rpc_client::GetLogs: Failed to copy log tar file in local node %v [%v]", err)
				return nil, err
			}

			break
		}

		if fh == nil {
			// first chunk
			common.Assert(chunkIndex == 0, reqStr)

			// Output directory is already created by the caller.
			outPath = filepath.Join(outDir, resp.TarName)
			fh, err = os.Create(outPath)
			common.Assert(err == nil, err)

			log.Info("rpc_client::GetLogs: Started receiving logs from node %s -> %s (totalSize=%d)",
				targetNodeID, outPath, resp.TotalSize)
		}

		n, err := fh.Write(resp.Data)
		_ = n
		common.Assert(err == nil, err)
		common.Assert(n == len(resp.Data), n, len(resp.Data))

		log.Debug("rpc_client::GetLogs: Wrote %d bytes in %s, chunkIndex=%d, totalSize=%d, isLast=%v for node %s",
			len(resp.Data), resp.TarName, resp.ChunkIndex, resp.TotalSize, resp.IsLast, targetNodeID)

		if resp.IsLast {
			break
		}
	}

	log.Info("rpc_client::GetLogs: Completed log download from node %s -> %s", targetNodeID, outPath)
	return &outPath, nil
}

// cleanup closes all the RPC node client pools
func Cleanup() error {
	log.Info("rpc_client::Cleanup: Closing all node client pools")

	cp.negativeNodesTicker.Stop()
	cp.negativeNodesDone <- true

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

	cp = newClientPool(defaultMaxPerNode, defaultMaxNodes, staticMaxNodes, defaultTimeout)
	common.Assert(cp != nil)

	log.Info("rpc_client::init: myNodeId: %s, maxNodes: %d, staticMaxNodes: %d, maxPerNode: %d, timeout: %d",
		myNodeId, defaultMaxNodes, staticMaxNodes, defaultMaxPerNode, defaultTimeout)
}

// Silence unused import errors for release builds.
func init() {
	_ = errors.New("test error")
	cm.IsValidRVName("rv0")
}
