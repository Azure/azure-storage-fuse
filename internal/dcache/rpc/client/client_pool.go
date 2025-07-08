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
//
// TODO: Implement timeout support.
func newClientPool(maxPerNode uint32, maxNodes uint32, timeout uint32) *clientPool {
	log.Debug("clientPool::newClientPool: Creating new RPC client pool with maxPerNode: %d, maxNodes: %d, timeout: %d", maxPerNode, maxNodes, timeout)
	return &clientPool{
		clients:    make(map[string]*nodeClientPool),
		maxPerNode: maxPerNode,
		maxNodes:   maxNodes,
		timeout:    timeout,
	}

	// TODO: start a goroutine to periodically close inactive RPC clients
}

// Give a nodeID return the corresponding nodeClientPool.
// Caller MUST hold the clientPool lock.
func (cp *clientPool) getNodeClientPool(nodeID string) (*nodeClientPool, error) {
	var ncPool *nodeClientPool
	ncPool, exists := cp.clients[nodeID]
	if !exists {
		if len(cp.clients) >= int(cp.maxNodes) {
			// TODO: remove this and rely on the closeInactiveRPCClients to close inactive clients
			// getNodeClientPool should be small and fast, refer https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047993390
			log.Debug("clientPool::getNodeClientPool: Maximum number of nodes reached, evicting LRU node client pool")
			err := cp.closeLRUCNodeClientPool()
			if err != nil {
				log.Err("clientPool::getNodeClientPool: Failed to close LRU node client pool: %v",
					err)
				return nil, err
			}
		}

		ncPool = &nodeClientPool{nodeID: nodeID}
		//
		// Note that createRPCClients() can fail to create any client if the remote blobfuse process
		// is not running or the node is down.
		//
		err := ncPool.createRPCClients(cp.maxPerNode)
		if err != nil {
			log.Err("clientPool::getNodeClientPool: createRPCClients(%s) failed: %v", nodeID, err)
			return nil, err
		}

		cp.clients[nodeID] = ncPool
	}

	common.Assert(ncPool.clientChan != nil)
	return ncPool, nil
}

// getRPCClient retrieves an RPC client that can be used for calling RPC functions to the given target node.
// If the client pool for nodeID is not available (not created yet or was cleaned up due to pressure),
// a new pool is created, replenished with cp.maxPerNode clients and a client returned from that.
// If the pool doesn't have any free client, it waits for 60secs for a client to become available and returns as
// soon as an RPC client is released and added to the pool. If no client becomes available for 60secs, it
// indicates some bug and it panics the program.
//
// Caller MUST NOT hold the clientPool lock.
func (cp *clientPool) getRPCClient(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClient: Retrieving RPC client for node %s", nodeID)

	//
	// Get the nodeClientPool for this node.
	// This needs to be performed with the clientPool lock.
	//
	cp.mu.Lock()
	ncPool, err := cp.getNodeClientPool(nodeID)
	cp.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("clientPool::getRPCClient: getNodeClientPool(%s) failed: %v",
			nodeID, err)
	}

	//
	// Get a free client from the pool if available, else wait for a client to be released.
	// In order to catch misbehaving/stuck clients, we cap this wait. This indicates some bug
	// so we crash the program with a trace.
	// Note that accessing clientChan is thread safe, so we don't need the clientPool lock.
	//
	maxWait := time.Duration(60 * time.Second)

	select {
	case client := <-ncPool.clientChan:
		ncPool.lastUsed.Store(time.Now().Unix())
		common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
		ncPool.numActive.Add(1)
		return client, nil
	case <-time.After(maxWait):
		err := fmt.Errorf("no free RPC client for node %s, even after waiting for %s",
			nodeID, maxWait)
		log.GetLoggerObj().Panicf("clientPool::getRPCClient: %v", err)
		return nil, err
	}
}

// Gets an RPC client that can be used for calling RPC functions to the given target node.
// Like getRPCClient() but in case there's no client currently available in the pool, it doesn't wait but
// instead returns error rightaway.
//
// Note: Caller MUST hold the clientPool lock.
func (cp *clientPool) getRPCClientNoWait(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClientNoWait: Retrieving RPC client for node %s", nodeID)

	ncPool, err := cp.getNodeClientPool(nodeID)
	if err != nil {
		return nil, err
	}

	select {
	case client := <-ncPool.clientChan:
		ncPool.lastUsed.Store(time.Now().Unix())
		common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
		ncPool.numActive.Add(1)
		return client, nil
	default:
		err := fmt.Errorf("no free RPC client for node %s", nodeID)
		log.Err("clientPool::getRPCClientNoWait: %v", err)
		return nil, err
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
		// We don't delete a nodeClientPool with active connections, so it cannot go away.
		common.Assert(false)
		return fmt.Errorf("no client pool found for node %s", client.nodeID)
	}

	log.Debug("clientPool::releaseRPCClient: node = %s, current node client pool size = %v, max connections per node = %v ", client.nodeID, len(ncPool.clientChan), cp.maxPerNode)
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode), fmt.Sprintf("node client pool is full, cannot release client: node = %s, current node client pool size = %v, max connections per node = %v ", client.nodeID, len(ncPool.clientChan), cp.maxPerNode))

	ncPool.clientChan <- client

	// Must be releasing an active client.
	common.Assert(ncPool.numActive.Load() > 0)
	ncPool.numActive.Add(-1)
	return nil
}

// closeRPCClient closes an RPC client.
// The client MUST have been removed from the pool using a prior getRPCClient() call.
func (cp *clientPool) closeRPCClient(client *rpcClient) error {
	log.Debug("clientPool::closeRPCClient: Closing RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	err := client.close()
	if err != nil {
		err = fmt.Errorf("failed to close RPC client to %s node %s: %v",
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
	_ = exists
	common.Assert(exists)
	//
	// resetRPCClientInternal() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	//
	// Must only reset an active client.
	// Also, clients which are reset are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0)
	ncPool.numActive.Add(-1)

	newClient, err := newRPCClient(client.nodeID, rpc.GetNodeAddressFromID(client.nodeID))
	if err != nil {
		log.Err("clientPool::resetRPCClientInternal: Failed to create RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		//
		// Connection refused is the only viable error. Assert to know if anything else happens.
		// This will happen when the target node is not running the blob service (but the node
		// itself is up). When the node is no up we should get a connection timeout error here.
		//
		// TODO: Connection timeout has to be tested.
		//
		// In any case when we come here that means we are not able to replenish connections to
		// the target, in the connection pool. When we have no active connections and no more left
		// in the pool, we can delete the nodeClientPool itself.
		//
		common.Assert(rpc.IsConnectionRefused(err) || rpc.IsTimedOut(err))
		cp.deleteNodeClientPoolIfInactive(client.nodeID)
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
	_ = exists

	// client is allocated from the pool, so pool must exist.
	common.Assert(exists)
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// Clients present in the pool If it's less than cp.maxPerNode, rest are currently allocated to other
	// callers. We cannot replenish those. Those will be reset by their respective caller when their RPC
	// requests fail with "connection reset by peer" error.
	//
	numClients := len(ncPool.clientChan)
	common.Assert(numClients < int(cp.maxPerNode), numClients, cp.maxPerNode)

	//
	// Reset this client. This closes this client, creates a new one and adds it to the pool.
	// It can only fail if thrift fails to create a new connection. This can happen when a node
	// (rather the blobfuse service in the node) has gone down.
	//
	err := cp.resetRPCClientInternal(client, false /* needLock */)
	if err != nil {
		err = fmt.Errorf("failed to reset RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		log.Err("clientPool::resetAllRPCClients: %v", err)
		//
		// Connection refused and timeout are the only viable errors.
		// Assert to know if anything else happens.
		//
		common.Assert(rpc.IsConnectionRefused(err) || rpc.IsTimedOut(err), err)
		return err
	}

	numConnReset++

	//
	// Reset all remaining clients in the pool. We try to reset as many as we can, and don't fail
	// on error, as we have reset at least one client.
	//
	for i := 0; i < numClients; i++ {
		client, err = cp.getRPCClientNoWait(client.nodeID)
		//
		// getRPCClientNoWait should not fail, because we have the clientPool for this client,
		// also numClients was the clientChan length before we reset the above client, and we
		// have the clientPool lock.
		//
		common.Assert(err == nil, err)

		err = cp.resetRPCClientInternal(client, false /* needLock */)
		if err != nil {
			//
			// We have reset at least one connection, so we don't fail the resetAllRPCClients()
			// call, log an error and proceed. This way even if we are not able to create new
			// good connections, we drain and close all stale connections.
			//
			log.Err("clientPool::resetAllRPCClients: Failed to reset RPC client to %s node %s: %v",
				client.nodeAddress, client.nodeID, err)
			//
			// Connection refused and timeout are the only viable errors.
			// Assert to know if anything else happens.
			//
			common.Assert(rpc.IsConnectionRefused(err) || rpc.IsTimedOut(err), err)
		} else {
			numConnReset++
		}
	}

	// We must have reset at least the client we are called for (and maybe more).
	common.Assert(numConnReset > 0)

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

		if lruNcPool == nil || (ncPool.lastUsed.Load() < lruNcPool.lastUsed.Load()) {
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

	// Never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(lruNcPool.numActive.Load() == 0 && len(lruNcPool.clientChan) == 0,
		lruNcPool.numActive.Load(), len(lruNcPool.clientChan))

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

		// Never delete a nodeClientPool with active connections or non-empty connection pool.
		common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
			ncPool.numActive.Load(), len(ncPool.clientChan))

		delete(cp.clients, key)
	}

	common.Assert(len(cp.clients) == 0, "client pool is not empty after closing all node client pools")
	return nil
}

// Delete nodeClientPool for the given node, if no active connections and no connections in the pool.
// Caller must hold the clientPool lock.
func (cp *clientPool) deleteNodeClientPoolIfInactive(nodeID string) bool {
	ncPool, exists := cp.clients[nodeID]
	_ = exists

	// Caller must not call us for a non-existent pool.
	common.Assert(exists)

	//
	// Can't delete if we have any active client (issued by getRPCClient()) or any client(s) in the
	// pool. This means only the last user will be able to delete the pool.
	//
	if ncPool.numActive.Load() != 0 || len(ncPool.clientChan) != 0 {
		log.Debug("clientPool::deleteNodeClientPoolIfInactive: Not deleting client pool for %s (%d, %d)",
			ncPool.nodeID, ncPool.numActive.Load(), len(ncPool.clientChan))
		return false
	}

	log.Debug("clientPool::deleteNodeClientPoolIfInactive: Deleting client pool for %s", nodeID)

	// Never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
		ncPool.numActive.Load(), len(ncPool.clientChan))

	delete(cp.clients, nodeID)

	return true
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// nodeClientPool holds a channel of RPC clients for a node
// and the last used timestamp for LRU eviction
type nodeClientPool struct {
	nodeID     string          // Node ID of the node this client pool is for
	clientChan chan *rpcClient // channel to hold the RPC clients to a node
	lastUsed   atomic.Int64    // used for evicting inactive RPC clients based on LRU (seconds since epoch)
	numActive  atomic.Int64    // number of clients currently created using getRPCClient() call.
}

// createRPCClients creates a channel of RPC clients of size numClients for the specified node ID
func (ncPool *nodeClientPool) createRPCClients(numClients uint32) error {
	log.Debug("nodeClientPool::createRPCClients: Creating %d RPC clients for node %s",
		numClients, ncPool.nodeID)

	common.Assert(ncPool.clientChan == nil)
	common.Assert(ncPool.numActive.Load() == 0, ncPool.numActive.Load())
	common.Assert(common.IsValidUUID(ncPool.nodeID))

	ncPool.clientChan = make(chan *rpcClient, numClients)
	ncPool.lastUsed.Store(time.Now().Unix())

	var err error

	// Create RPC clients and add them to the channel.
	for i := 0; i < int(numClients); i++ {
		var client *rpcClient
		client, err = newRPCClient(ncPool.nodeID, rpc.GetNodeAddressFromID(ncPool.nodeID))
		if err != nil {
			log.Err("nodeClientPool::createRPCClients: Failed to create RPC client for node %s [%v]",
				ncPool.nodeID, err)
			//
			// Only valid reason could be connection refused as the blobfuse process is not running
			// on the remote node or a timeout if the node is down.
			// There is no point in retrying in that case.
			//
			common.Assert(rpc.IsConnectionRefused(err) || rpc.IsTimedOut(err), err)
			break
		}
		ncPool.clientChan <- client
	}

	//
	// If we are not able to create all requested connections there's something seriously wrong
	// so clean up and fail. One possibility is that the remote node went down after creating
	// first few connections.
	// What is more likely is that we could not create any connection. This can happen f.e., when
	// clustermap has a component RV for an MV but the RV just went down, if user attempts reading
	// a file that has data on that MV and ReadMV() picks that RV. If there are no existing connections
	// to that node, createRPCClients() will be called which will fail to create any connection.
	//
	if len(ncPool.clientChan) == 0 {
		return fmt.Errorf("could not create any client for node %s: %v", ncPool.nodeID, err)
	} else if len(ncPool.clientChan) != int(numClients) {
		log.Err("nodeClientPool::createRPCClients: Created %d of %d clients for node %s, cleaning up",
			len(ncPool.clientChan), numClients, ncPool.nodeID)

		for client := range ncPool.clientChan {
			err1 := client.close()
			_ = err1
			// close() should not fail, even if it does there's nothing left to do.
			common.Assert(err1 == nil, err1)
		}
		// All error paths must ensure this.
		common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan))
		return fmt.Errorf("could not create all requested clients for node %s: %v", ncPool.nodeID, err)
	}

	// We just got started, cannot have active clients.
	common.Assert(ncPool.numActive.Load() == 0, ncPool.numActive.Load())
	return nil
}

// closeRPCClients closes all RPC clients in the channel for the specified node ID
func (ncPool *nodeClientPool) closeRPCClients() error {
	log.Debug("nodeClientPool::closeRPCClients: Closing %d RPC clients for node %s",
		len(ncPool.clientChan), ncPool.nodeID)

	// We should not be closing all clients when there are active clients.
	common.Assert(ncPool.numActive.Load() == 0,
		ncPool.numActive.Load(), len(ncPool.clientChan), cp.maxPerNode)

	//
	// We never have a partially allocated client pool and we only clean up a client pool when all
	// previously allocated clients have been released back to the pool
	//
	common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode), len(ncPool.clientChan), cp.maxPerNode)

	close(ncPool.clientChan)

	for client := range ncPool.clientChan {
		err := client.close()
		if err != nil {
			log.Err("nodeClientPool::closeRPCClients: Failed to close RPC client for node %s [%v]", ncPool.nodeID, err.Error())
			return err
		}
	}

	// All clients must have been closed.
	common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan))

	return nil
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
}
