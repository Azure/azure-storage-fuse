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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
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
	highPrio    bool                        // highPrio indicates if this client is for high priority operations
}

var protocolFactory thrift.TProtocolFactory
var transportFactory thrift.TTransportFactory
var thriftCfg *thrift.TConfiguration

// newRPCClient creates a new Thrift RPC client for the node
func newRPCClient(nodeID string, nodeAddress string) (*rpcClient, error) {
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
		log.Err("rpcClient::newRPCClient: Failed to open transport node %s at %s [%v]", nodeID, nodeAddress, err.Error())

		//
		// If the RPC client creation fails due to timeout error, it means some connection problem or
		// the node is down. In this case we should prevent creating RPC clients to the same node by
		// other threads, as each operation will fail with the timeout error.
		// So, add this node ID to negativeNodes map to prevent creating new RPC clients to it by other
		// threads till the negative timeout expires.
		//
		if rpc.IsTimedOut(err) {
			log.Warn("rpcClient::newRPCClient: Adding node %s at %s to negative nodes map", nodeID, nodeAddress)
			cp.addNegativeNode(nodeID)
		}

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
