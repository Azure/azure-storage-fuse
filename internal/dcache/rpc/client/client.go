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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
	"github.com/apache/thrift/lib/go/thrift"
)

// rpcClient struct holds the Thrift client to a node
// This is used to make RPC calls to the node
type rpcClient struct {
	nodeID      string                      // Node ID of the node this client is for, can be used for debug logs
	nodeAddress string                      // Address of the node this client is for
	transport   thrift.TTransport           // Transport is the Thrift transport layer
	svcClient   *service.ChunkServiceClient // Client is the Thrift client for the ChunkService
}

var protocolFactory thrift.TProtocolFactory
var transportFactory thrift.TTransportFactory
var thriftCfg *thrift.TConfiguration

// newRPCClient creates a new Thrift RPC client for the node
func newRPCClient(nodeID string, nodeAddress string) (*rpcClient, error) {
	log.Debug("rpcClient::newRPCClient: Creating new RPC client for node %s at %s", nodeID, nodeAddress)

	var transport thrift.TTransport

	// if secure {
	// 	transport = thrift.NewTSSLSocketConf(nodeAddress, thriftCfg)
	// }

	transport = thrift.NewTSocketConf(nodeAddress, thriftCfg)
	transport, err := transportFactory.GetTransport(transport)
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
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}
