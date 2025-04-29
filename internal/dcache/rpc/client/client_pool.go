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
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// clientPool manages multiple rpc clients efficiently
type clientPool struct {
	mu         sync.Mutex
	clients    map[string]*nodeClientPool // map of node ID to rpc node client pool
	maxPerNode uint32                     // Maximum number of open RPC clients per node
	maxNodes   uint32                     // Maximum number of nodes for which RPC clients are open
	timeout    uint32                     // Duration in seconds after which a RPC client is closed
}

// newClientPool creates a new client pool with the specified parameters
// maxPerNode: Maximum number of RPC clients opened per node
// maxNodes: Maximum number of nodes for which RPC clients are open
// timeout: Duration in seconds after which a RPC client is closed
func newClientPool(maxPerNode uint32, maxNodes uint32, timeout uint32) *clientPool {
	log.Debug("clientPool::newClientPool: Creating new client pool with maxPerNode: %d, maxNodes: %d, timeout: %d", maxPerNode, maxNodes, timeout)
	return &clientPool{
		clients:    make(map[string]*nodeClientPool),
		maxPerNode: maxPerNode,
		maxNodes:   maxNodes,
		timeout:    timeout,
	}

	// TODO: start a goroutine to periodically close inactive RPC clients
}

// getRPCClient retrieves a RPC client from the pool for the specified node ID
// If no client is available, a new one is created
func (cp *clientPool) getRPCClient(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClient: Retrieving rpc client for node %s", nodeID)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	var ncPool *nodeClientPool
	ncPool, exists := cp.clients[nodeID]
	if !exists {
		if len(cp.clients) >= int(cp.maxNodes) {
			// TODO: remove this and rely on the closeInactiveRPCClients to close inactive clients
			// GetRPCClient should be small and fast, refer https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047993390
			log.Debug("clientPool::getRPCClient: Maximum number of nodes reached, evict LRU node client pool")
			err := cp.closeLRUCNodeClientPool()
			if err != nil {
				log.Err("clientPool::getRPCClient: Failed to close LRU node client pool [%v]", err.Error())
				return nil, err
			}
		}

		ncPool = &nodeClientPool{nodeID: nodeID}
		ncPool.createRPCClients(cp.maxPerNode)
		cp.clients[nodeID] = ncPool
	}

	select {
	case client := <-ncPool.clientChan:
		ncPool.lastUsed = time.Now()
		return client, nil
	default:
		log.Err("clientPool::getRPCClient: No available RPC client in the pool for node %s", nodeID)
		return nil, fmt.Errorf("no available RPC client in the pool for node %s", nodeID)
	}
}

// releaseRPCClient releases a RPC client back to the pool
func (cp *clientPool) releaseRPCClient(client *rpcClient) error {
	log.Debug("clientPool::releaseRPCClient: Releasing RPC client for node %s", client.nodeID)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	ncPool, exists := cp.clients[client.nodeID]
	if !exists {
		log.Err("clientPool::releaseRPCClient: No client pool found for node %s", client.nodeID)
		return fmt.Errorf("no client pool found for node %s", client.nodeID)
	}

	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode), "node client pool is full, cannot release client")
	ncPool.clientChan <- client
	return nil
}

// Close the least recently used node client pool from the client pool
// caller of this method should hold the lock
func (cp *clientPool) closeLRUCNodeClientPool() error {
	// TODO: add assert to check that lock is held, mu.TryLock()
	// TODO: add assert that the length of the client pool is greater than maxNodes
	// Find the least recently used RPC client and close it
	var lruNcPool *nodeClientPool
	lruNodeID := ""
	for nodeID, ncPool := range cp.clients {
		if lruNcPool == nil || ncPool.lastUsed.Before(lruNcPool.lastUsed) {
			lruNcPool = ncPool
			lruNodeID = nodeID
		}
	}

	common.Assert(lruNcPool != nil, "lruNcPool is nil")
	err := lruNcPool.closeRPCClients()
	if err != nil {
		log.Err("clientPool::closeLRUCNodeClientPool: Failed to close LRU node client pool for node %s [%v]", lruNodeID, err.Error())
		return err
	}
	delete(cp.clients, lruNodeID)

	return nil
}

// closeInactiveNodeClientPools closes node client pools that have not been used for a specified timeout
func (cp *clientPool) closeInactiveNodeClientPools() {
	// Cleanup old RPC clients based on the LastUsed timestamp
	// This will run in a separate goroutine and will periodically close the node client pools based on LRU strategy
}

// closeAllNodeClientPools closes all node client pools in the pool
func (cp *clientPool) closeAllNodeClientPools() error {
	// TODO: see if this is needed
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for key, ncPool := range cp.clients {
		err := ncPool.closeRPCClients()
		if err != nil {
			log.Err("clientPool::closeAllNodeClientPools: Failed to close node client pool for node %s [%v]", key, err.Error())
			return err
		}
		delete(cp.clients, key)
	}

	common.Assert(len(cp.clients) == 0, "client pool is not empty after closing all node client pools")
	return nil
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// nodeClientPool holds a channel of RPC clients for a node
// and the last used timestamp for LRU eviction
type nodeClientPool struct {
	nodeID     string          // Node ID of the node this client pool is for
	clientChan chan *rpcClient // channel to hold the RPC clients to a node
	lastUsed   time.Time       // used for evicting inactive RPC clients based on LRU
}

// createRPCClients creates a channel of RPC clients of size numClients for the specified node ID
func (ncPool *nodeClientPool) createRPCClients(numClients uint32) {
	log.Debug("nodeClientPool::createRPCClients: Creating %d RPC clients for node %s", numClients, ncPool.nodeID)

	ncPool.clientChan = make(chan *rpcClient, numClients)
	ncPool.lastUsed = time.Now()

	// Create RPC clients and add them to the channel
	for i := 0; i < int(numClients); i++ {
		// TODO:: integration: getNodeAddressFromID should be replaced with a function to get the node address from the config
		client, err := newRPCClient(ncPool.nodeID, getNodeAddressFromID(ncPool.nodeID))
		if err != nil {
			log.Err("nodeClientPool::createRPCClients: Failed to create RPC client for node %s [%v]", ncPool.nodeID, err.Error())
			continue // skip this client
		}
		ncPool.clientChan <- client
	}
}

// closeRPCClients closes all RPC clients in the channel for the specified node ID
func (ncPool *nodeClientPool) closeRPCClients() error {
	log.Debug("nodeClientPool::closeRPCClients: Closing RPC clients for node %s", ncPool.nodeID)

	// TODO: add assert to check that the length of the channel is maxPerNode, so that all clients are released back
	close(ncPool.clientChan)

	for client := range ncPool.clientChan {
		err := client.close()
		if err != nil {
			log.Err("nodeClientPool::closeRPCClients: Failed to close RPC client for node %s [%v]", ncPool.nodeID, err.Error())
			return err
		}
	}

	common.Assert(len(ncPool.clientChan) == 0, "client channel is not empty after closing all RPC clients")
	return nil
}

// TODO:: integration: call cluster manager to get the node address for the given node ID
// TODO: add assert to check if the node address of the form addr:port - IsValidHostPort(string)
func getNodeAddressFromID(nodeID string) string {
	return "localhost:9090"
}
