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
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
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

// getRPCClient retrieves an RPC client from the pool for the specified node ID.
// If the client pool for nodeID is not is not available (not created yet or was cleaned up due to pressure),
// a new pool is created, replenished with cp.maxPerNode clients and a client returned.
//
// Note: This is an internal functional that expects the caller to hold the clientPool lock.
//
//	You may want to use getRPCClient().
func (cp *clientPool) getRPCClientNoLock(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClient: Retrieving rpc client for node %s", nodeID)

	var ncPool *nodeClientPool
	ncPool, exists := cp.clients[nodeID]
	if !exists {
		if len(cp.clients) >= int(cp.maxNodes) {
			// TODO: remove this and rely on the closeInactiveRPCClients to close inactive clients
			// GetRPCClient should be small and fast, refer https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047993390
			log.Debug("clientPool::getRPCClient: Maximum number of nodes reached, evict LRU node client pool")
			err := cp.closeLRUCNodeClientPool()
			if err != nil {
				log.Err("clientPool::getRPCClient: Failed to close LRU node client pool: %v",
					err)
				return nil, err
			}
		}

		ncPool = &nodeClientPool{nodeID: nodeID}
		ncPool.createRPCClients(cp.maxPerNode)
		cp.clients[nodeID] = ncPool
	}

	// TODO: this should be a blocking call, if a caller does not get the client for a node,
	// it should wait for a client to be released back to the pool
	select {
	case client := <-ncPool.clientChan:
		ncPool.lastUsed = time.Now()
		common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
		return client, nil
	default:
		log.Err("clientPool::getRPCClient: No available RPC client in the pool for node %s", nodeID)
		return nil, fmt.Errorf("no available RPC client in the pool for node %s", nodeID)
	}
}

func (cp *clientPool) getRPCClient(nodeID string) (*rpcClient, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	return cp.getRPCClientNoLock(nodeID)
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

	log.Debug("clientPool::releaseRPCClient: node = %s, current node client pool size = %v, max connections per node = %v ", client.nodeID, len(ncPool.clientChan), cp.maxPerNode)
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode), fmt.Sprintf("node client pool is full, cannot release client: node = %s, current node client pool size = %v, max connections per node = %v ", client.nodeID, len(ncPool.clientChan), cp.maxPerNode))
	ncPool.clientChan <- client
	return nil
}

// closeRPCClient closes an RPC client.
// The client MUST have been removed from the pool using a prior getRPCClient() call.
func (cp *clientPool) closeRPCClient(client *rpcClient) error {
	log.Debug("clientPool::closeRPCClient: Closing RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	err := client.close()
	if err != nil {
		err = fmt.Errorf("Failed to close RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		log.Err("nodeClientPool::closeRPCClient: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("clientPool::closeRPCClient: Closed RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	return nil
}

// Close client and create a new one to the same target/node as client.
//
// Note: This is an internal function, use resetRPCClient() for resetting one connection and
//
//	resetAllRPCClients() for resetting all connections.
func (cp *clientPool) resetRPCClientInternal(client *rpcClient, needLock bool) error {
	log.Debug("clientPool::resetRPCClientInternal: client %s for node %s",
		client.nodeAddress, client.nodeID)

	//
	// First close the client and then create a new one and add that to the pool.
	//
	err := cp.closeRPCClient(client)
	if err != nil {
		return err
	}

	log.Info("clientPool::resetRPCClientInternal: Creating new RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	if needLock {
		cp.mu.Lock()
		defer cp.mu.Unlock()
	}

	ncPool, exists := cp.clients[client.nodeID]
	common.Assert(exists)
	//
	// resetRPCClientInternal() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	newClient, err := newRPCClient(client.nodeID, rpc.GetNodeAddressFromID(client.nodeID))
	if err != nil {
		log.Err("clientPool::resetRPCClientInternal: Failed to create RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		common.Assert(false, err)
		return err
	}

	// Add the new client to the client pool for this node.
	ncPool.clientChan <- newClient

	return nil
}

// Close client and create a new one to the same target/node as 'client'.
// This is typically used when a node goes down and comes back up, rendering all existing clients
// "broken", they need to be replaced with a brand new connection/client.
func (cp *clientPool) resetRPCClient(client *rpcClient) error {
	return cp.resetRPCClientInternal(client, true /* needLock */)
}

// Reset all connections in the client pool corresponding to 'client'. This client would have been allocated
// using a prior call to getRPCClient().
// This is used for draining and fixing all old/bad connections in the connection pool in case of the target
// node restarting. It closes the passed in connection and all existing connections in the pool and replaces
// them with newly created ones.
func (cp *clientPool) resetAllRPCClients(client *rpcClient) error {
	//
	// resetAllRPCClients() will be called only when we know for sure that an RPC request made using 'client'
	// failed with a "connection reset by peer" error, we reset that client and all others in the pool.
	//
	cp.mu.Lock()
	defer cp.mu.Unlock()

	numConnReset := 0
	ncPool, exists := cp.clients[client.nodeID]

	// client is allocated from the pool, so pool must exist.
	common.Assert(exists)
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// Reset this client. This closes this client, creates a new one and adds it to the pool.
	// It can only fail if thrift fails to create a new connection. Though it can happen, but
	// it's unlikely as we are called only when we get a "connection reset by peer" error which
	// means the target node exists (was restarted), so new connection must not fail.
	//
	err := cp.resetRPCClientInternal(client, false /* needLock */)
	if err != nil {
		err = fmt.Errorf("Failed to reset RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		log.Err("clientPool::resetAllRPCClients: %v", err)
		common.Assert(false, err)
		return err
	}

	numConnReset++

	//
	// Clients present in the pool If it's less than cp.maxPerNode, rest are currently allocated to other
	// callers. We cannot replenish those. Those will be reset by their respective caller when their RPC
	// requests fail with "connection reset by peer" error.
	//
	numClients := len(ncPool.clientChan)
	common.Assert(numClients <= int(cp.maxPerNode), numClients, cp.maxPerNode)

	//
	// And all remaining. We try to reset as many as we can.
	//
	for i := 0; i < numClients; i++ {
		client, err = cp.getRPCClientNoLock(client.nodeID)
		// getRPCClientNoLock should not fail, because we have the clientPool for this client.
		common.Assert(err == nil, err)

		err = cp.resetRPCClientInternal(client, false /* needLock */)
		if err != nil {
			//
			// We have reset at least one connection, so we don't fail the resetAllRPCClients()
			// call, log an error and proceed.
			//
			log.Err("clientPool::resetAllRPCClients: Failed to reset RPC client to %s node %s: %v",
				client.nodeAddress, client.nodeID, err)
			common.Assert(false, err)
		} else {
			numConnReset++
		}
	}

	log.Debug("clientPool::resetAllRPCClients: Reset %d RPC clients to %s node %s, now available (%d / %d)",
		numConnReset, client.nodeAddress, client.nodeID, len(ncPool.clientChan), cp.maxPerNode)

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
		common.Assert(len(ncPool.clientChan) <= int(cp.maxPerNode),
			len(ncPool.clientChan), cp.maxPerNode)
		//
		// Omit the nodeClientPool if it has any client currently in use.
		//
		if len(ncPool.clientChan) != int(cp.maxPerNode) {
			log.Debug("clientPool::closeLRUCNodeClientPool: Skipping %s (%d < %d)",
				ncPool.nodeID, len(ncPool.clientChan), cp.maxPerNode)
			continue
		}

		if lruNcPool == nil || ncPool.lastUsed.Before(lruNcPool.lastUsed) {
			lruNcPool = ncPool
			lruNodeID = nodeID
		}
	}

	if lruNcPool == nil {
		return fmt.Errorf("clientPool::closeLRUCNodeClientPool: No free nodeClientPool")
	}

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
		client, err := newRPCClient(ncPool.nodeID, rpc.GetNodeAddressFromID(ncPool.nodeID))
		if err != nil {
			log.Err("nodeClientPool::createRPCClients: Failed to create RPC client for node %s [%v]", ncPool.nodeID, err.Error())
			continue // skip this client
		}
		ncPool.clientChan <- client
	}

	common.Assert(len(ncPool.clientChan) == int(numClients), "client channel is not full after creating RPC clients", len(ncPool.clientChan), numClients)
}

// closeRPCClients closes all RPC clients in the channel for the specified node ID
func (ncPool *nodeClientPool) closeRPCClients() error {
	log.Debug("nodeClientPool::closeRPCClients: Closing RPC clients for node %s", ncPool.nodeID)

	// check that the length of the channel is maxPerNode, so that all clients are released back
	common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode), "client channel is not full before closing RPC clients", len(ncPool.clientChan), cp.maxPerNode)
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
