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
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	gp *grpcClientPool
)

func helloGRPC(ctx context.Context, targetNodeID string, req *models.HelloRequest) (*models.HelloResponse, error) {
	common.Assert(req != nil)

	// Caller must not set SenderNodeID, catch misbehaving callers.
	common.Assert(len(req.SenderNodeID) == 0, req.SenderNodeID)
	req.SenderNodeID = myNodeId

	// Caller cannot send a clustermap epoch greater than what we have.
	common.Assert(req.ClustermapEpoch <= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	// TODO: write toString() methods for GRPC models
	// reqStr := rpc.HelloRequestToString(req)

	reqStr := req.String()
	log.Debug("rpc_client::helloGRPC: Sending Hello request to node %s: %v", targetNodeID, reqStr)

	//
	// We retry once after resetting bad connections.
	//
	for range 2 {
		client, err := gp.getRPCClient(targetNodeID)
		if err != nil {
			err = fmt.Errorf("rpc_client::helloGRPC: Failed to get RPC client for node %s %v: %v [%w]",
				targetNodeID, reqStr, err, NoFreeRPCClient)
			log.Err("%v", err)
			return nil, err
		}

		// Call the rpc method.
		resp, err := client.svcClient.Hello(ctx, req)
		if err != nil {
			log.Err("rpc_client::helloGRPC: Hello failed to node %s %v: %v",
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
				continue
			} else if rpc.IsTimedOut(err) {
				//
				// RPC call to the node fails with timeout error. So, add it to the negative nodes map to
				// help other threads fail fast instead of waiting for timeout.
				//
				gp.addNegativeNode(targetNodeID)
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
			gp.removeNegativeNode(targetNodeID)
		}

		// Release RPC client back to the pool.
		gp.releaseRPCClient(client)

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

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	cm.IsValidRVName("rv0")
}
