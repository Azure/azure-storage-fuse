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
	"crypto/tls"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
	"github.com/apache/thrift/lib/go/thrift"
)

const (
	//
	// defaultConnectionTimeout is the default timeout for establishing a new connection to the node.
	// We don't want to keep this value very low to be resilient to occasional packet drops.
	//
	defaultConnectionTimeout = 10 * time.Second

	//
	// defaultSocketTimeout is the default read/write timeout for the underlying socket.
	// We don't want to keep this value very low to be resilient to occasional packet drops.
	//
	defaultSocketTimeout = 20 * time.Second
)

// rpcClient struct holds the Thrift client to a node
// This is used to make RPC calls to the node
type rpcClient struct {
	nodeID      string                      // Node ID of the node this client is for, can be used for debug logs
	nodeAddress string                      // Address of the node this client is for
	transport   thrift.TTransport           // Transport is the Thrift transport layer
	svcClient   *service.ChunkServiceClient // Client is the Thrift client for the ChunkService
	allocatedAt time.Time                   // Time when this client was allocated from the pool, used for debug logs
	//
	// PutChunkDC puts an interesting demand on the RPC client pool manager.
	// PutChunkDC requires RPC clients for two distinct use cases:
	// 1. RPC clients needed to make the PutChunkDC call to the first nexthop node.
	//    This is called from WriteMV() invoked by file_manager when writing chunks from the writeback cache.
	// 2. RPC clients needed to make the PutChunkDC/PutChunk call to subsequent nodes in the daisy chain
	//    in response to a PutChunkDC call received from a previous node (or the local node).
	//
	// The important distinction to note here is that the RPC clients used by case (1) cannot be freed
	// till (2) gets the required RPC clients and complete their respective requests. There could be writes
	// going on any node which means (1) and (2) will contest for the same pool of RPC clients. If we issue
	// all RPC clients to (1), then there won't be any left for (2) and hence daisy chain requests from other
	// nodes will get stuck. The clients used by (1) may also be stuck due to the unavailability of clients
	// for (2) on other nodes. This can lead to a deadlock situation, where all clients are used up by (1) and
	// they cannot be freed as they need (2) to complete, but (2) doesn't have any clients to proceed.
	// Also note that demand for RPC clients by (2) is proportial to the RPC clients used by (1), as each of
	// those would result in a daisy chain of PutChunkDC/PutChunk calls to subsequent nodes.
	//
	// This brings us to two key observations:
	// 1. We need to prioritize (and reserve) RPC clients needed for (2) over (1). This is because RPC clients
	//    used by (1) will only be released when (2) completes.
	//    See nodeClientPool.numReservedHighPrio.
	// 2. We need to rate-limit the number of RPC clients used by (1) to a fraction of the total pool size.
	//    See GetRPCClientDummy().
	//
	// highPrio indicates if this client is used for high priority requests (case 2 above).
	//
	highPrio bool
}

var protocolFactory thrift.TProtocolFactory
var transportFactory thrift.TTransportFactory
var thriftCfg *thrift.TConfiguration

// newRPCClient creates a new Thrift RPC client for the node
func newRPCClient(nodeID string, nodeAddress string) (*rpcClient, error) {
	// Caller must call with node lock held.
	common.Assert(cp.isNodeLocked(nodeID), nodeID, nodeAddress)

	log.Debug("rpcClient::newRPCClient: Creating new RPC client for node %s at %s", nodeID, nodeAddress)

	//
	// If the node is present in the negativeNodes map, it means we have recently experienced timeout
	// when communicating with this node, learn from our recent experience and save a potential timeout.
	//
	err := cp.checkNegativeNode(nodeID)
	if err != nil {
		log.Err("rpcClient::newRPCClient: not creating RPC client to negative node %s: %v", nodeID, err)
		return nil, err
	}

	var transport thrift.TTransport

	// if secure {
	// 	transport = thrift.NewTSSLSocketConf(nodeAddress, thriftCfg)
	// }

	transport = thrift.NewTSocketConf(nodeAddress, thriftCfg)
	transport, err = transportFactory.GetTransport(transport)
	if err != nil {
		log.Err("rpcClient::newRPCClient: Failed to create transport for node %s at %s [%v]", nodeID, nodeAddress, err.Error())
		return nil, err
	}

	iprot := protocolFactory.GetProtocol(transport)
	oprot := protocolFactory.GetProtocol(transport)
	svcClient := service.NewChunkServiceClient(thrift.NewTStandardClient(iprot, oprot))

	client := &rpcClient{
		nodeID:      nodeID,
		nodeAddress: nodeAddress,
		transport:   transport,
		svcClient:   svcClient,
	}

	err = client.transport.Open()
	if err != nil {
		log.Err("rpcClient::newRPCClient: Failed to create connection to node %s at %s, adding to negative nodes map: %v",
			nodeID, nodeAddress, err)

		//
		// TODO: We can also get other errors here like,
		//   - "no route to host" - the node is on different subnet/vnet or the node is deallocated.
		//   - "cannot assign requested address" - can be caused due to port exhaustion if we are
		//     creating too many connections in a short period of time.
		//
		// common.Assert(rpc.IsTimedOut(err) ||
		// 	rpc.IsConnectionRefused(err) ||
		// 	rpc.IsConnectionReset(err), err)

		//
		// Any error indicates a problem connecting to the node, so add to negative nodes map to quarantine
		// the node for a short period, so that we don't keep trying to connect to it repeatedly.
		//
		cp.addNegativeNode(nodeID)

		return nil, err
	}

	return client, nil
}

// close closes the Thrift RPC client to the node
func (c *rpcClient) close() error {
	err := c.transport.Close()
	if err != nil {
		log.Err("rpcClient::close: Failed to close transport for node %s at %s [%v]", c.nodeID, c.nodeAddress, err.Error())
		return err
	}

	return nil
}

func init() {
	log.Debug("rpcClient::init: Initializing protocol and transport factories")
	protocolFactory = thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory = thrift.NewTTransportFactory()

	thriftCfg = &thrift.TConfiguration{
		ConnectTimeout: defaultConnectionTimeout,
		SocketTimeout:  defaultSocketTimeout,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
