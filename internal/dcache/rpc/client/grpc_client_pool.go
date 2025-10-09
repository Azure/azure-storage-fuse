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
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
)

//go:generate $ASSERT_REMOVER $GOFILE

const (
	defaultGRPCPerNodeConns = 16
)

var (
	gp *grpcClientPool
)

// grpcClientPool manages per-node grpcClientPool.
type grpcClientPool struct {
	rwMutex sync.RWMutex

	// Companion counter to rwMutex for performing various locking related assertions.
	// [DEBUG ONLY]
	rwMutexDbgCntr atomic.Int64

	// Map of nodeID to *grpcNodeClientPool. Use the following helpers to manage the map:
	clients map[string]*grpcNodeClientPool

	maxPerNode uint32 // Maximum number of open RPC clients per node
	maxNodes   uint32 // Maximum number of nodes for which RPC clients are open
}

func newGRPCClientPool(maxPerNode, maxNodes uint32) *grpcClientPool {
	log.Info("grpcClientPool::newGRPCClientPool: Creating RPC client pool with maxPerNode: %d, maxNodes: %d",
		maxPerNode, maxNodes)

	return &grpcClientPool{
		clients:    make(map[string]*grpcNodeClientPool),
		maxPerNode: maxPerNode,
		maxNodes:   maxNodes,
	}
}

func (cp *grpcClientPool) acquireRWMutexReadLock() {
	cp.rwMutex.RLock()

	if common.IsDebugBuild() {
		cp.rwMutexDbgCntr.Add(1)
		common.Assert(cp.rwMutexDbgCntr.Load() > 0, cp.rwMutexDbgCntr.Load())
	}
}

// Release the read lock on the rwMutex.
func (cp *grpcClientPool) releaseRWMutexReadLock() {
	if common.IsDebugBuild() {
		common.Assert(cp.rwMutexDbgCntr.Load() > 0, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Add(-1)
	}

	cp.rwMutex.RUnlock()
}

func (cp *grpcClientPool) acquireRWMutexWriteLock() {
	cp.rwMutex.Lock()

	if common.IsDebugBuild() {
		common.Assert(cp.rwMutexDbgCntr.Load() == 0, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Store(-12345) // Special value to signify write lock.
	}
}

// Release the write lock on the rwMutex.
func (cp *grpcClientPool) releaseRWMutexWriteLock() {
	if common.IsDebugBuild() {
		common.Assert(cp.rwMutexDbgCntr.Load() == -12345, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Store(0)
	}

	cp.rwMutex.Unlock()
}

// Check if read/shared lock is held on rwMutex.
// [DEBUG ONLY]
func (cp *grpcClientPool) isRWMutexReadLocked() bool {
	return cp.rwMutexDbgCntr.Load() > 0
}

// Check if write/exclusive lock is held on rwMutex.
// [DEBUG ONLY]
func (cp *grpcClientPool) isRWMutexWriteLocked() bool {
	return cp.rwMutexDbgCntr.Load() == -12345
}

// getRPCClient returns a gRPC client for given nodeID in round-robin manner; creates pool if missing.
func (cp *grpcClientPool) getRPCClient(nodeID string) (*grpcClient, error) {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	startTime := time.Now()
	_ = startTime

	log.Debug("grpcClientPool::getRPCClient: getRPCClient(nodeID: %s)", nodeID)

	cp.acquireRWMutexReadLock()
	ncPool, exists := cp.clients[nodeID]
	cp.releaseRWMutexReadLock()

	if !exists {
		cp.acquireRWMutexWriteLock()

		// double-check
		ncPool, exists = cp.clients[nodeID]
		if !exists {
			common.Assert(ncPool == nil)

			ncPool = &grpcNodeClientPool{nodeID: nodeID}
			err := ncPool.createRPCClients(cp.maxPerNode)
			if err != nil {
				cp.releaseRWMutexWriteLock()
				log.Err("grpcClientPool::getRPCClient: Failed to create RPC clients for node %s [%v]",
					nodeID, err)
				return nil, err
			}

			cp.clients[nodeID] = ncPool
		}

		cp.releaseRWMutexWriteLock()
	}

	common.Assert(ncPool != nil, nodeID)
	common.Assert(len(ncPool.conns) == int(cp.maxPerNode), len(ncPool.conns), cp.maxPerNode)

	// Pick round-robin
	idx := int(ncPool.idx.Add(1)-1) % len(ncPool.conns)

	numActive := ncPool.numActive.Add(1)
	_ = numActive
	log.Debug("grpcClientPool::getRPCClient: Successfully retrieved RPC client for node %s [active: %d], took %s",
		nodeID, numActive, time.Since(startTime))

	return ncPool.conns[idx], nil
}

func (cp *grpcClientPool) releaseRPCClient(client *grpcClient) {
	log.Debug("grpcClientPool::releaseRPCClient: releaseRPCClient(nodeID: %s)", client.nodeID)

	cp.acquireRWMutexReadLock()
	ncPool, exists := cp.clients[client.nodeID]
	_ = exists
	cp.releaseRWMutexReadLock()

	common.Assert(exists, client.nodeID)
	common.Assert(ncPool != nil, client.nodeID)
	common.Assert(len(ncPool.conns) == int(cp.maxPerNode), len(ncPool.conns), cp.maxPerNode)

	numActive := ncPool.numActive.Add(-1)
	_ = numActive
	common.Assert(ncPool.numActive.Load() >= 0, ncPool.numActive.Load(), client.nodeID)

	log.Debug("grpcClientPool::releaseRPCClient: Successfully released RPC client for node %s [active: %d]",
		client.nodeID, numActive)
}

// closeAllNodeClientPools closes all node client pools.
func (cp *grpcClientPool) closeAllNodeClientPools() error {
	log.Debug("grpcClientPool::closeAllNodeClientPools: Closing all %d node client pools", len(cp.clients))

	//
	// Acquire write lock on the client pool to ensure that no other thread is accessing
	// the client pool while we are closing all node client pools.
	// This also waits till the read locks are released by the client pool operations
	// like getRPCClient() and releaseRPCClient().
	//
	cp.acquireRWMutexWriteLock()
	defer cp.releaseRWMutexWriteLock()

	startTime := time.Now()

	// Wait for max 60 seconds for all active clients to be released back to the channel.
	maxWaitTime := 60 * time.Second

	var err error
	for nodeID, ncPool := range cp.clients {
		common.Assert(common.IsValidUUID(nodeID), nodeID)

		//
		// Check if there are any active clients for this node. If yes, release the write lock and wait for a second
		// for the active clients to be released back to the channel. After that acquire the write lock and recheck.
		//
		for ncPool.numActive.Load() > 0 && time.Since(startTime) < maxWaitTime {
			log.Debug("grpcClientPool::closeAllNodeClientPools: sleeping till %d active clients for %s are released",
				ncPool.numActive.Load(), nodeID)

			// release write lock so that the releaseRPCClient() can proceed while we wait
			cp.releaseRWMutexWriteLock()

			time.Sleep(time.Second)

			// acquire write lock again and recheck if the active clients are released for this node
			cp.acquireRWMutexWriteLock()
		}

		//
		// Even after waiting for 60 seconds, if the active clients are not released back, return error to the caller.
		//
		if ncPool.numActive.Load() > 0 {
			err = fmt.Errorf("Node %s has %d active clients, cannot close even after waiting for %v",
				nodeID, ncPool.numActive.Load(), maxWaitTime)
			log.Err("grpcClientPool::closeAllNodeClientPools: %v", err)
			break
		}

		err := ncPool.closeRPCClients()
		if err != nil {
			err = fmt.Errorf("Failed to close RPC clients for node %s [%v]", nodeID, err)
			log.Err("grpcClientPool::closeAllNodeClientPools: %v", err)
			break
		}

		// Never delete a nodeClientPool with active connections or non-empty connections slice.
		common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.conns) == 0,
			ncPool.numActive.Load(), len(ncPool.conns))

		delete(cp.clients, nodeID)
	}

	common.Assert(len(cp.clients) == 0)
	return nil
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// grpcNodeClientPool holds fixed connections for one node.
type grpcNodeClientPool struct {
	nodeID    string
	conns     []*grpcClient
	idx       atomic.Uint32
	numActive atomic.Int64 // number of clients currently created using getRPCClient() call.
}

func (ncPool *grpcNodeClientPool) createRPCClients(numClients uint32) error {
	common.Assert(gp.isRWMutexWriteLocked(), ncPool.nodeID)

	nodeAddress := rpc.GetNodeAddressFromID(ncPool.nodeID)
	log.Debug("grpcNodeClientPool::createRPCClients: Creating %d RPC clients for node %s (%s)",
		numClients, ncPool.nodeID, nodeAddress)

	common.Assert(len(ncPool.conns) == 0)

	ncPool.conns = make([]*grpcClient, 0, numClients)
	var err error

	for range int(numClients) {
		client, err1 := newGRPCClient(ncPool.nodeID, nodeAddress)
		if err1 != nil {
			log.Err("grpcNodeClientPool::createRPCClients: Failed to create gRPC client for node %s (%s) [%v]",
				ncPool.nodeID, nodeAddress, err1)
			err = err1
			break
		}

		ncPool.conns = append(ncPool.conns, client)
	}

	if err != nil {
		log.Err("grpcNodeClientPool::createRPCClients: Created %d of %d clients for node %s (%s), cleaning up",
			len(ncPool.conns), numClients, ncPool.nodeID, nodeAddress)

		for _, client := range ncPool.conns {
			err1 := client.close()
			_ = err1
			common.Assert(err1 == nil, err1)
		}

		return fmt.Errorf("could not create all requested clients for node %s: %v",
			ncPool.nodeID, err)
	}

	common.Assert(len(ncPool.conns) == int(numClients), len(ncPool.conns), numClients)

	log.Debug("grpcClientPool: created %d conns for node %s (%s)",
		len(ncPool.conns), ncPool.nodeID, nodeAddress)
	return nil
}

// closeRPCClients closes (best-effort) all gRPC clients held in this node pool.
// Caller MUST hold the parent grpcClientPool write lock to avoid races with create / map mutation.
func (ncPool *grpcNodeClientPool) closeRPCClients() error {
	common.Assert(gp.isRWMutexWriteLocked())
	common.Assert(ncPool.numActive.Load() == 0, ncPool.nodeID, ncPool.numActive.Load())

	log.Debug("grpcNodeClientPool::closeRPCClients: Closing %d RPC clients for node %s",
		len(ncPool.conns), ncPool.nodeID)

	for _, client := range ncPool.conns {
		if client == nil {
			// Should never happen; protect against nil dereference.
			common.Assert(false, ncPool.nodeID, len(ncPool.conns))
			continue
		}

		err := client.close()
		if err != nil {
			log.Err("grpcNodeClientPool::closeRPCClients: Failed to close client for node %s [%v]",
				ncPool.nodeID, err)
			return err
		}
	}

	ncPool.conns = nil
	ncPool.idx.Store(0)

	log.Debug("grpcNodeClientPool::closeRPCClients: Completed closing clients for node %s", ncPool.nodeID)
	return nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
