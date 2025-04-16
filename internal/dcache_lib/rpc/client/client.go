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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache/models"
)

var cp *connectionPool

const (
	// TODO: discuss with the team about these values
	// defaultMaxPerNode is the default maximum number of open connections per node
	defaultMaxPerNode = 5
	// defaultMaxNodes is the default maximum number of nodes for which connections are open
	defaultMaxNodes = 100
	// defaultTimeout is the default duration in seconds after which a connection is closed
	defaultTimeout = 60
)

func Hello(ctx context.Context, targetNodeID string, req *models.HelloRequest) (*models.HelloResponse, error) {
	log.Debug("rpc_client::Hello: Sending Hello request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::Hello: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::Hello: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.Hello(ctx, req)
	if err != nil {
		log.Err("rpc_client::Hello: Failed to send Hello request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func GetChunk(ctx context.Context, targetNodeID string, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	log.Debug("rpc_client::GetChunk: Sending GetChunk request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::GetChunk: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::GetChunk: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.GetChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::GetChunk: Failed to send GetChunk request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func PutChunk(ctx context.Context, targetNodeID string, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	log.Debug("rpc_client::PutChunk: Sending PutChunk request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::PutChunk: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::PutChunk: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.PutChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::PutChunk: Failed to send PutChunk request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func RemoveChunk(ctx context.Context, targetNodeID string, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	log.Debug("rpc_client::RemoveChunk: Sending RemoveChunk request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::RemoveChunk: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::RemoveChunk: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.RemoveChunk(ctx, req)
	if err != nil {
		log.Err("rpc_client::RemoveChunk: Failed to send RemoveChunk request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func JoinMV(ctx context.Context, targetNodeID string, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	log.Debug("rpc_client::JoinMV: Sending JoinMV request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::JoinMV: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::JoinMV: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.JoinMV(ctx, req)
	if err != nil {
		log.Err("rpc_client::JoinMV: Failed to send JoinMV request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func LeaveMV(ctx context.Context, targetNodeID string, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	log.Debug("rpc_client::LeaveMV: Sending LeaveMV request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::LeaveMV: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::LeaveMV: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.LeaveMV(ctx, req)
	if err != nil {
		log.Err("rpc_client::LeaveMV: Failed to send LeaveMV request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func StartSync(ctx context.Context, targetNodeID string, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	log.Debug("rpc_client::StartSync: Sending StartSync request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::StartSync: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::StartSync: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.StartSync(ctx, req)
	if err != nil {
		log.Err("rpc_client::StartSync: Failed to send StartSync request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func EndSync(ctx context.Context, targetNodeID string, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	log.Debug("rpc_client::EndSync: Sending EndSync request to node %s", targetNodeID)

	// get rpc client from the connection pool
	conn, err := cp.getRPCClient(targetNodeID)
	if err != nil {
		log.Err("rpc_client::EndSync: Failed to get connection for node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}
	defer func() {
		// release rpc client back to the pool
		err = cp.releaseRPCClient(targetNodeID, conn)
		if err != nil {
			log.Err("rpc_client::EndSync: Failed to release connection for node %s [%v]", targetNodeID, err.Error())
		}
	}()

	// call the rpc method
	resp, err := conn.svcClient.EndSync(ctx, req)
	if err != nil {
		log.Err("rpc_client::EndSync: Failed to send EndSync request to node %s [%v]", targetNodeID, err.Error())
		return nil, err
	}

	return resp, nil
}

func init() {
	log.Debug("rpc_client package initialized, create connection pool")
	cp = newConnectionPool(defaultMaxPerNode, defaultMaxNodes, defaultTimeout)
}
