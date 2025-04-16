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
	"crypto/tls"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache/service"
	"github.com/apache/thrift/lib/go/thrift"
)

// rpcClient struct holds the Thrift connection to a node
// This is used to make RPC calls to the node
type rpcClient struct {
	nodeID      string                      // Node ID of the node this connection is for, can be used for debug logs
	nodeAddress string                      // Address of the node this connection is for
	ctx         context.Context             // Context for the connection
	transport   thrift.TTransport           // Transport is the Thrift transport layer
	svcClient   *service.ChunkServiceClient // Client is the Thrift client for the ChunkService
}

// newRPCClient creates a new Thrift connection to a node
func newRPCClient(nodeID string, nodeAddress string) (*rpcClient, error) {
	log.Debug("Connection::newRPCClient: Creating new connection to node %s at %s", nodeID, nodeAddress)

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTTransportFactory()

	var transport thrift.TTransport
	cfg := &thrift.TConfiguration{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// if secure {
	// 	transport = thrift.NewTSSLSocketConf(addr, cfg)
	// }

	transport = thrift.NewTSocketConf(nodeAddress, cfg)
	transport, err := transportFactory.GetTransport(transport)
	if err != nil {
		log.Err("Connection::newRPCClient: Failed to create transport [%v]", err.Error())
		return nil, err
	}

	iprot := protocolFactory.GetProtocol(transport)
	oprot := protocolFactory.GetProtocol(transport)
	client := service.NewChunkServiceClient(thrift.NewTStandardClient(iprot, oprot))

	conn := &rpcClient{
		nodeID:      nodeID,
		nodeAddress: nodeAddress,
		ctx:         context.Background(), // TODO: check if context with cancel is needed
		transport:   transport,
		svcClient:   client,
	}

	err = conn.transport.Open()
	if err != nil {
		log.Err("Connection::newRPCClient: Failed to open transport [%v]", err.Error())
		return nil, err
	}

	return conn, nil
}

// close closes the Thrift connection to the node
func (c *rpcClient) close() error {
	err := c.transport.Close()
	if err != nil {
		log.Err("Connection::close: Failed to close transport [%v]", err.Error())
		return err
	}

	return nil
}
