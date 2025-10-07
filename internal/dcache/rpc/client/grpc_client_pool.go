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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
)

const grpcPerNodeConns = 16

// grpcNodePool holds fixed connections for one node.
type grpcNodePool struct {
	conns []*grpcRPCClient
	rrIdx atomic.Uint32
}

// grpcClientPool manages per-node grpcNodePool.
type grpcClientPool struct {
	mu   sync.RWMutex
	pool map[string]*grpcNodePool // key: nodeID
}

func newGrpcClientPool() *grpcClientPool {
	return &grpcClientPool{pool: make(map[string]*grpcNodePool)}
}

// getClient returns a gRPC client for given nodeID in round-robin manner; creates pool if missing.
func (gp *grpcClientPool) getClient(nodeID string) (*grpcRPCClient, error) {
	address := rpc.GetNodeAddressFromID(nodeID)
	gp.mu.RLock()
	np := gp.pool[nodeID]
	gp.mu.RUnlock()
	if np == nil {
		// create
		gp.mu.Lock()
		// double-check
		np = gp.pool[nodeID]
		if np == nil {
			conns := make([]*grpcRPCClient, 0, grpcPerNodeConns)
			for i := 0; i < grpcPerNodeConns; i++ {
				c, err := newGrpcRPCClient(address)
				if err != nil {
					// close any created
					for _, pc := range conns {
						pc.Close()
					}
					gp.mu.Unlock()
					return nil, fmt.Errorf("newGrpcClientPool: dial %s failed: %w", address, err)
				}
				conns = append(conns, c)
			}
			np = &grpcNodePool{conns: conns}
			gp.pool[nodeID] = np
			log.Debug("grpcClientPool: created %d conns for node %s", len(conns), nodeID)
		}
		gp.mu.Unlock()
	}
	// Pick round-robin
	idx := int(np.rrIdx.Add(1)-1) % len(np.conns)
	return np.conns[idx], nil
}

// closeAll closes all connections (best effort).
func (gp *grpcClientPool) closeAll() {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	for id, np := range gp.pool {
		for _, c := range np.conns {
			c.Close()
		}
		delete(gp.pool, id)
	}
}
