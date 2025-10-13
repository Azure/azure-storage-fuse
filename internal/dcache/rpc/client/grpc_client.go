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
	grpcservice "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

//go:generate $ASSERT_REMOVER $GOFILE

// grpcClient wraps a single gRPC connection & stub.
type grpcClient struct {
	nodeID      string                         // Node ID of the node this client is for, can be used for debug logs
	nodeAddress string                         // Address of the node this client is for
	conn        *grpc.ClientConn               // Underlying gRPC connection
	svcClient   grpcservice.ChunkServiceClient // gRPC client for the chunk service
}

var opts []grpc.DialOption

func newGRPCClient(nodeID string, nodeAddress string) (*grpcClient, error) {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	//
	// If the node is present in the negativeNodes map, it means we have recently experienced timeout
	// when communicating with this node, learn from our recent experience and save a potential timeout.
	//
	err := gp.checkNegativeNode(nodeID)
	if err != nil {
		log.Err("rpcClient::newRPCClient: not creating RPC client to negative node %s (%s): %v",
			nodeID, nodeAddress, err)
		return nil, err
	}

	conn, err := grpc.NewClient(nodeAddress, opts...)
	if err != nil {
		log.Err("grpcClient::newGRPCClient: Failed to create client to node %s at %s: %v",
			nodeID, nodeAddress, err)
		return nil, err
	}

	err = checkConnectionReady(conn)
	if err != nil {
		log.Err("grpcClient::newGRPCClient: Failed to create connection to node %s at %s: %v",
			nodeID, nodeAddress, err)

		//
		// Any error indicates a problem connecting to the node, so add to negative nodes map to quarantine
		// the node for a short period, so that we don't keep trying to connect to it repeatedly.
		//
		gp.addNegativeNode(nodeID)

		return nil, err
	}

	client := grpcservice.NewChunkServiceClient(conn)

	grpcClient := &grpcClient{
		nodeID:      nodeID,
		nodeAddress: nodeAddress,
		conn:        conn,
		svcClient:   client,
	}

	return grpcClient, nil
}

func (c *grpcClient) close() error {
	err := c.conn.Close()
	if err != nil {
		log.Err("grpcClient::close: Failed to close connection for node %s at %s [%v]",
			c.nodeID, c.nodeAddress, err)
		return err
	}

	return nil
}

// Check if the connection is ready within defaultConnectionTimeout.
func checkConnectionReady(conn *grpc.ClientConn) error {
	conn.Connect()

	ctx, cancel := context.WithTimeout(context.Background(), defaultConnectionTimeout)
	defer cancel()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}

		if !conn.WaitForStateChange(ctx, state) {
			return fmt.Errorf("connection not ready within timeout %v, last state: %v",
				defaultConnectionTimeout, state)
		}
	}
}

func init() {
	log.Debug("rpcClient::init: Initializing protocol and transport factories")

	// Use insecure connection for now
	// TODO: add TLS support
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// Update max message size to 64 MB, default being 4 MB
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(64*1024*1024),
		grpc.MaxCallSendMsgSize(64*1024*1024),
	))
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
