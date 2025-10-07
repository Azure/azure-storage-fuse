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
	"sync/atomic"
	"time"

	grpcmodels "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/models"
	grpcservice "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// grpcRPCClient wraps a single gRPC connection & stub.
type grpcRPCClient struct {
	conn   *grpc.ClientConn
	client grpcservice.ChunkServiceClient
	// simple stats
	lastUsed atomic.Int64 // unix nano
}

func newGrpcRPCClient(address string) (*grpcRPCClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	// Manual readiness wait (max 5s)
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(context.Background(), st) { // context never cancels; use deadline check
			// WaitForStateChange returns false if ctx expired; we only pass background so continue
		}
		if time.Now().After(deadline) && st != connectivity.Ready {
			_ = conn.Close()
			return nil, errors.New("grpc: connection not ready within timeout")
		}
	}

	c := grpcservice.NewChunkServiceClient(conn)
	cli := &grpcRPCClient{conn: conn, client: c}
	cli.touch()
	return cli, nil
}

func (c *grpcRPCClient) touch() { c.lastUsed.Store(time.Now().UnixNano()) }
func (c *grpcRPCClient) Close() { _ = c.conn.Close() }

// Example wrapper for Hello.
func (c *grpcRPCClient) Hello(ctx context.Context, req *grpcmodels.HelloRequest) (*grpcmodels.HelloResponse, error) {
	c.touch()
	return c.client.Hello(ctx, req)
}
