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

// clientPool manages (multiple) rpc clients to multiple nodes efficiently.
type clientPool struct {
	//
	// RWMutex at the client pool level.
	// All operations except closeAllNodeClientPools() acquire read lock on this mutex.
	// This ensures that operations like getRPCClient(), releaseRPCClient(), deleteAllRPCClients(),
	// resetAllRPCClients(), etc. by different threads can process concurrently.
	// Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	rwMutex sync.RWMutex

	// Companion counter to rwMutex for performing various locking related assertions.
	// [DEBUG ONLY]
	rwMutexDbgCntr atomic.Int64

	//
	// Lock at the node level to ensure that only one thread can create/get/release/delete
	// RPC clients for a node at a time. This also ensures that other threads can
	// create/get/release/delete RPC clients for other nodes at the same time.
	//
	// This MUST be acquired after acquiring read lock on the rwMutex, and the rwMutex read
	// lock MUST be held till the node lock is released.
	//
	nodeLock *common.LockMap

	//
	// Map of nodeID to *nodeClientPool. Use the following helpers to manage the map:
	// getNodeClientPoolFromMap() to get the nodeClientPool for a given nodeID.
	// addNodeClientPoolToMap() to add a new nodeClientPool to the map, and
	// deleteNodeClientPoolFromMap() to delete a nodeClientPool from the map.
	//
	clients sync.Map

	// clientsCnt is the number of node client pools in the clients map.
	clientsCnt atomic.Int64

	//
	// Map of node ID to which this node cannot create RPC clients. The value of the map is
	// time.Time when the RPC client creation to the node was attempted that failed indicating
	// reachability issue for the node.
	// This is used to prevent creating new RPC clients to the node by different threads till
	// the negative RPC client creation timeout expires.
	//
	negativeNodes sync.Map

	maxPerNode uint32 // Maximum number of open RPC clients per node
	maxNodes   uint32 // Maximum number of nodes for which RPC clients are open
	timeout    uint32 // Duration in seconds after which a RPC client is closed
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
		nodeLock:   common.NewLockMap(),
		maxPerNode: maxPerNode,
		maxNodes:   maxNodes,
		timeout:    timeout,
	}

	// TODO: start a goroutine to periodically close inactive RPC clients
}

// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
// releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc. by different threads can process
// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
// all the read locks are released.
//
// NOTE: We do take lock at node ID level in the client pool operations to ensure that multiple
//       threads cannot create/release/delete RPC clients for the same node at the same time.

func (cp *clientPool) acquireRWMutexReadLock() {
	cp.rwMutex.RLock()

	if common.IsDebugBuild() {
		cp.rwMutexDbgCntr.Add(1)
		common.Assert(cp.rwMutexDbgCntr.Load() > 0, cp.rwMutexDbgCntr.Load())
	}
}

// Release the read lock on the rwMutex.
func (cp *clientPool) releaseRWMutexReadLock() {
	if common.IsDebugBuild() {
		common.Assert(cp.rwMutexDbgCntr.Load() > 0, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Add(-1)
	}

	cp.rwMutex.RUnlock()
}

// Acquire write lock on the rwMutex. This lock is only acquired by closeAllNodeClientPools().
// This ensures that this will wait for the read locks to be released by other threads and when
// this lock is acquired, no other thread can acquire read lock on the rwMutex.
func (cp *clientPool) acquireRWMutexWriteLock() {
	cp.rwMutex.Lock()

	if common.IsDebugBuild() {
		//
		// We get the write lock only when there are no readers which also means no thread can be
		// holding the node lock for any node, since the rwMutex read lock must be held when node
		// lock is acquired.
		//
		common.Assert(cp.clientsCnt.Load() == 0, cp.clientsCnt.Load())
		common.Assert(cp.rwMutexDbgCntr.Load() == 0, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Store(-12345) // Special value to signify write lock.
	}
}

// Release the write lock on the rwMutex.
func (cp *clientPool) releaseRWMutexWriteLock() {
	if common.IsDebugBuild() {
		common.Assert(cp.rwMutexDbgCntr.Load() == -12345, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Store(0)
	}

	cp.rwMutex.Unlock()
}

// Check if read/shared lock is held on rwMutex.
// [DEBUG ONLY]
func (cp *clientPool) isRWMutexReadLocked() bool {
	return cp.rwMutexDbgCntr.Load() > 0
}

// Check if write/exclusive lock is held on rwMutex.
// [DEBUG ONLY]
func (cp *clientPool) isRWMutexWriteLocked() bool {
	return cp.rwMutexDbgCntr.Load() == -12345
}

// Acquire client pool lock for the given nodeID.
// This is used to ensure that only one thread can create/get/release/delete clients for a node at a time.
// It returns a LockMapItem which is used to release the lock later using releaseNodeLock().
func (cp *clientPool) acquireNodeLock(nodeID string) *common.LockMapItem {
	// RWMutex must be read locked before acquiring node lock.
	common.Assert(cp.isRWMutexReadLocked())

	nodeLock := cp.nodeLock.Get(nodeID)
	nodeLock.Lock()

	return nodeLock
}

// Release the client pool lock for a given node, acquired using acquireNodeLock().
// nodeID is used only to assert that the lock is indeed held for the given node ID.
// Note that nodeLock.Unlock() will panic if lock is not held but still the nodeID based assert is useful.
func (cp *clientPool) releaseNodeLock(nodeLock *common.LockMapItem, nodeID string) {
	common.Assert(cp.isNodeLocked(nodeID), nodeID)
	// RWMutex must be held while we have the node lock.
	common.Assert(cp.isRWMutexReadLocked())

	nodeLock.Unlock()
}

// Check if the client pool lock is locked for the given node ID.
func (cp *clientPool) isNodeLocked(nodeID string) bool {
	return cp.nodeLock.Locked(nodeID)
}

// Get the nodeClientPool for the given nodeID, from the clients map.
func (cp *clientPool) getNodeClientPoolFromMap(nodeID string) *nodeClientPool {
	// MUST be called with the node lock held for the given nodeID.
	common.Assert(cp.isNodeLocked(nodeID), nodeID)

	val, ok := cp.clients.Load(nodeID)

	// Not found.
	if !ok {
		return nil
	}

	// clients and clientsCnt must agree.
	common.Assert(cp.clientsCnt.Load() > 0, cp.clientsCnt.Load(), nodeID)

	// Found, value must be of type *ncPool.
	ncPool, ok := val.(*nodeClientPool)
	if ok {
		common.Assert(ncPool != nil, nodeID)
		common.Assert(ncPool.nodeID == nodeID, ncPool.nodeID, nodeID)
		common.Assert(ncPool.clientChan != nil, nodeID)
		return ncPool
	}

	// Value not of type ncPool.
	common.Assert(false, nodeID)

	return nil
}

// Add nodeClientPool for the given nodeID.
func (cp *clientPool) addNodeClientPoolToMap(nodeID string, ncPool *nodeClientPool) {
	// MUST be called with the node lock held for the given nodeID.
	common.Assert(cp.isNodeLocked(nodeID), nodeID)

	// Assert that the nodeID is not already present in the clients map.
	_, ok := cp.clients.Load(nodeID)
	if ok {
		common.Assert(false, nodeID)
		return
	}

	cp.clients.Store(nodeID, ncPool)
	cp.clientsCnt.Add(1)

	common.Assert(cp.clientsCnt.Load() < int64(cp.maxNodes), cp.clientsCnt.Load(), cp.maxNodes)
}

// Delete nodeClientPool for the given nodeID.
func (cp *clientPool) deleteNodeClientPoolFromMap(nodeID string) {
	// MUST be called with the node lock held for the given nodeID, or with the rwMutex write lock held.
	// Latter is true when called from closeAllNodeClientPools().
	common.Assert(cp.isNodeLocked(nodeID) || cp.isRWMutexWriteLocked(), nodeID)

	// Assert that the nodeID is present in the clients map.
	val, ok := cp.clients.Load(nodeID)
	if !ok {
		common.Assert(false, nodeID)
		return
	}

	ncPool, ok := val.(*nodeClientPool)
	_ = ncPool
	common.Assert(ok, nodeID)

	// Never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
		ncPool.numActive.Load(), len(ncPool.clientChan))

	cp.clients.Delete(nodeID)

	common.Assert(cp.clientsCnt.Load() > 0, cp.clientsCnt.Load(), nodeID)
	cp.clientsCnt.Add(-1)
}

// Given a nodeID return the corresponding nodeClientPool.
// If the nodeClientPool does not exist, it creates a new one and returns it.
// Any other thread wanting to get a RPC client for the node will wait for this function to return.
//
// NOTE: Caller MUST hold the lock for the nodeID and read lock on the rwMutex
//       before calling this function.

func (cp *clientPool) getNodeClientPool(nodeID string) (*nodeClientPool, error) {
	common.Assert(cp.isNodeLocked(nodeID), nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolFromMap(nodeID)
	if ncPool == nil {
		//
		// Check in the negative nodes map if we should attempt creating RPC clients for this node ID.
		//
		t, ok := cp.negativeNodes.Load(nodeID)
		if ok {
			timeElapsed := int64(time.Since(t.(time.Time)).Seconds())
			if timeElapsed < defaultNegativeTimeout {
				err := fmt.Errorf("not creating RPC clients for node %s, negative timeout not expired yet (%d seconds elapsed, %d seconds timeout)",
					nodeID, timeElapsed, defaultNegativeTimeout)
				log.Err("clientPool::getNodeClientPool: %v", err)
				return nil, err
			} else {
				log.Debug("clientPool::getNodeClientPool: Negative timeout expired for node %s, removing from negative nodes map (%d seconds elapsed, %d seconds timeout)",
					nodeID, timeElapsed, defaultNegativeTimeout)
				cp.negativeNodes.Delete(nodeID)
			}
		}

		//
		// Assert that the node ID should not be present in the negativeNodes map as we are
		// creating a new nodeClientPool for it.
		//
		if common.IsDebugBuild() {
			_, ok = cp.negativeNodes.Load(nodeID)
			common.Assert(!ok, nodeID)
		}

		if cp.clientsCnt.Load() >= int64(cp.maxNodes) {
			// TODO: remove this and rely on the closeInactiveRPCClients to close inactive clients
			// getNodeClientPool should be small and fast,
			// refer https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047993390
			log.Debug("clientPool::getNodeClientPool: Maximum number of nodes reached, evicting LRU node client pool")
			err := cp.closeLRUNodeClientPool()
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

			//
			// Add to negativeNodes map to prevent creating new RPC clients to the node ID by other
			// threads till the negative timeout expires.
			// Note that createRPCClients() failure indicates some transport problem or the node/blobfuse is down.
			//
			cp.negativeNodes.Store(nodeID, time.Now())

			return nil, err
		}

		// Successfully created all required RPC clients for the node, add it to the clients map.
		cp.addNodeClientPoolToMap(nodeID, ncPool)
	}

	// Must never return a nodeClientPool with no clients allocated.
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
// NOTE: Caller MUST NOT hold the clientPool or node level lock.
func (cp *clientPool) getRPCClient(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClient: Retrieving RPC client for node %s", nodeID)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	//
	// Get lock for the given node ID. This is done so that multiple threads can create RPC
	// clients for the different node IDs concurrently, whereas only one thread can create RPC
	// clients for a given node ID at a time.
	//
	nodeLock := cp.acquireNodeLock(nodeID)

	//
	// Get the nodeClientPool for this node.
	// We can release the node lock after this, as we are guaranteed that deleteNodeClientPoolFromMap()
	// won't delete an active nodeClientPool.
	//
	ncPool, err := cp.getNodeClientPool(nodeID)
	cp.releaseNodeLock(nodeLock, nodeID)

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
	maxWaitTime := 60 // in seconds

	// Time in seconds we have waited for a client to become available.
	waitTime := 0

	for {
		select {
		case client := <-ncPool.clientChan:
			ncPool.lastUsed.Store(time.Now().Unix())
			common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
			ncPool.numActive.Add(1)
			log.Debug("clientPool::getRPCClient: Successfully retrieved RPC client for node %s (now active: %d)",
				nodeID, ncPool.numActive.Load())
			return client, nil
		case <-time.After(2 * time.Second): // Timeout after 2 second
			waitTime += 2

			//
			// There can be a case when client pool for the node is deleted.
			// For example, the node goes down and RPC fails with BrokenPipe error. In this case,
			// we reset the connections available for the node, which first closes the stale connections
			// and then creates new connections. Since, the node is down, creating new RPC connection
			// to the node fails with connection refused error. So, eventually all the connections in the
			// pool are closed and client pool for the node is deleted. In this case, we do not wait
			// for a client to become available and return error to the caller.
			//
			if ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0 {
				err := fmt.Errorf("client pool deleted for node %s, no clients available after waiting for %d seconds",
					nodeID, waitTime)
				log.Err("clientPool::getRPCClient: %v", err)
				return nil, err
			}

			if waitTime >= maxWaitTime {
				err := fmt.Errorf("no free RPC client for node %s, even after waiting for %d seconds",
					nodeID, waitTime)
				log.Err("clientPool::getRPCClient: %v", err)
				log.GetLoggerObj().Panicf("clientPool::getRPCClient: %v", err)
				return nil, err
			}
		}
	}
}

// Gets an RPC client that can be used for calling RPC functions to the given target node.
// Like getRPCClient() but in case there's no client currently available in the pool, it doesn't wait but
// instead returns error rightaway.
//
// NOTE: Caller MUST hold the lock for the nodeID and read lock on the rwMutex
// before calling this function.
func (cp *clientPool) getRPCClientNoWait(nodeID string) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClientNoWait: Retrieving RPC client for node %s", nodeID)

	common.Assert(cp.isNodeLocked(nodeID), nodeID)
	common.Assert(cp.isRWMutexReadLocked())

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
//
// NOTE: Caller MUST NOT hold the clientPool or node level lock.
func (cp *clientPool) releaseRPCClient(client *rpcClient) error {
	log.Debug("clientPool::releaseRPCClient: Releasing RPC client for node %s", client.nodeID)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	//
	// Get lock for the given node ID. This is done so that multiple threads can release RPC
	// clients for the different node IDs concurrently in their respective client pools.
	// Whereas only one thread can release RPC client for the same node ID at a time.
	//
	nodeLock := cp.acquireNodeLock(client.nodeID)
	defer cp.releaseNodeLock(nodeLock, client.nodeID)

	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)
	if ncPool == nil {
		log.Err("clientPool::releaseRPCClient: No client pool found for node %s", client.nodeID)
		// We don't delete a nodeClientPool with active connections, so it cannot go away.
		common.Assert(false)
		return fmt.Errorf("no client pool found for node %s", client.nodeID)
	}

	log.Debug("clientPool::releaseRPCClient: node = %s, current node client pool size = %d, active clients = %d, max connections per node = %d",
		client.nodeID, len(ncPool.clientChan), ncPool.numActive.Load(), cp.maxPerNode)

	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		fmt.Sprintf("node client pool is full, cannot release client: node = %s, current node client pool size = %v, max connections per node = %v ", client.nodeID, len(ncPool.clientChan), cp.maxPerNode))

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

// deleteRPCClient deletes an RPC client from the pool.
// It first closes the client and then removes it from the pool.
// This is used when the client is no longer needed, e.g. when the node is down and we want to
// remove all clients to the node.
//
// NOTE: Caller MUST hold the lock for the nodeID and read lock on the rwMutex
// before calling this function.
func (cp *clientPool) deleteRPCClient(client *rpcClient) error {
	log.Debug("clientPool::deleteRPCClient: Deleting RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	common.Assert(cp.isNodeLocked(client.nodeID), client.nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	// Close the client first.
	err := cp.closeRPCClient(client)
	if err != nil {
		// We don't expect connection close to fail, let's know if it happens.
		common.Assert(false, err)
		return err
	}

	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)
	common.Assert(ncPool != nil, client.nodeID)

	//
	// deleteRPCClient() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	//
	// Must only delete an active client.
	// Also, clients which are deleted are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0)
	ncPool.numActive.Add(-1)
	return nil
}

// Delete all connections in the client pool corresponding to 'client'. This client would have been allocated
// using a prior call to getRPCClient().
// This is used for draining connections in the connection pool in case there is timeout error while
// making an RPC call to the target node.
// It closes the passed in connection and all existing connections in the pool, and if there are no
// active connections and no connections in the channel, it deletes the node client pool.
func (cp *clientPool) deleteAllRPCClients(client *rpcClient) error {
	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	//
	// deleteAllRPCClients() will be called only when we know for sure that an RPC request made using 'client'
	// failed with a "timeout" error, we delete that client and all others in the pool.
	//
	nodeLock := cp.acquireNodeLock(client.nodeID)
	defer cp.releaseNodeLock(nodeLock, client.nodeID)

	numConnDeleted := 0
	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)

	// client is allocated from the pool, so pool must exist.
	common.Assert(ncPool != nil, client.nodeID)
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// Clients present in the pool. The one that we are deleting is not in the pool so the pool can have
	// max cp.maxPerNode-1 clients. If it's less than cp.maxPerNode-1, rest are currently allocated to other
	// callers. We cannot replenish those. Those will be deleted by their respective caller when their RPC
	// requests fail with "timeout" error.
	//
	numClients := len(ncPool.clientChan)
	common.Assert(numClients < int(cp.maxPerNode), numClients, cp.maxPerNode)

	//
	// Delete this client. This closes this client and removes it from the pool.
	// It can only fail if Thrift fails to close the connection.
	// This should technically not happen, so we assert.
	//
	err := cp.deleteRPCClient(client)
	if err != nil {
		err = fmt.Errorf("failed to delete RPC client to %s node %s: %v",
			client.nodeAddress, client.nodeID, err)
		log.Err("clientPool::deleteAllRPCClients: %v", err)
		common.Assert(false, err)
		return err
	}

	numConnDeleted++

	//
	// Delete all remaining clients in the pool. We try to delete as many as we can, and don't fail
	// on error, as we have deleted at least one client.
	//
	for i := 0; i < numClients; i++ {
		client, err = cp.getRPCClientNoWait(client.nodeID)
		//
		// getRPCClientNoWait should not fail, because we have the clientPool for this client,
		// also numClients was the clientChan length before we deleted the above client, and we
		// have the clientPool lock.
		//
		common.Assert(err == nil, err)

		err = cp.deleteRPCClient(client)
		if err != nil {
			//
			// We have deleted at least one connection, so we don't fail the deleteAllRPCClients()
			// call, log an error and proceed.
			//
			log.Err("clientPool::deleteAllRPCClients: Failed to delete RPC client to %s node %s: %v",
				client.nodeAddress, client.nodeID, err)

			// This should technically not happen, so we assert.
			common.Assert(false, err)
		} else {
			numConnDeleted++
		}
	}

	log.Debug("clientPool::deleteAllRPCClients: Deleted %d RPC clients to %s node %s, now available (%d / %d)",
		numConnDeleted, client.nodeAddress, client.nodeID, len(ncPool.clientChan), cp.maxPerNode)

	// We must have deleted at least the client we are called for (and maybe more).
	common.Assert(numConnDeleted > 0)

	// We don't expect failure closing any client connection, so there shouldn't be any client left in the pool.
	common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan), client.nodeAddress, client.nodeID)

	//
	// After deleting all clients, if there are no active connections and no connections in the channel,
	// we delete the node client pool itself.
	//
	cp.deleteNodeClientPoolIfInactive(client.nodeID)

	return nil
}

// Close client and create a new one to the same target/node as client.
//
// Note: This is an internal function, use resetRPCClient() for resetting one connection and
//	     resetAllRPCClients() for resetting all connections.

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
		cp.acquireRWMutexReadLock()
		defer cp.releaseRWMutexReadLock()

		nodeLock := cp.acquireNodeLock(client.nodeID)
		defer cp.releaseNodeLock(nodeLock, client.nodeID)
	}

	// Assert that the client is locked for the node.
	common.Assert(cp.isNodeLocked(client.nodeID), client.nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)
	common.Assert(ncPool != nil, client.nodeID)

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
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	//
	// resetAllRPCClients() will be called only when we know for sure that an RPC request made using 'client'
	// failed with a "connection reset by peer" error, we reset that client and all others in the pool.
	//
	nodeLock := cp.acquireNodeLock(client.nodeID)
	defer cp.releaseNodeLock(nodeLock, client.nodeID)

	numConnReset := 0
	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)

	// client is allocated from the pool, so pool must exist.
	common.Assert(ncPool != nil, client.nodeID)
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// Clients present in the pool. The one that we are resetting is not in the pool so the pool can have
	// max cp.maxPerNode-1 clients. If it's less than cp.maxPerNode-1, rest are currently allocated to other
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

// Close the least recently used node client pool from the client pool.
//
// NOTE: Caller MUST hold the read lock on the rwMutex before calling this function.
func (cp *clientPool) closeLRUNodeClientPool() error {
	common.Assert(cp.clientsCnt.Load() >= int64(cp.maxNodes),
		cp.clientsCnt.Load(), cp.maxNodes)

	common.Assert(cp.isRWMutexReadLocked())

	// Find the least recently used RPC client and close it
	var lruNcPool *nodeClientPool
	lruNodeID := ""

	//
	// Iterate through the clients map to find the least recently used node client pool
	// and close it if it has no active clients (i.e., it has maxPerNode clients in the pool).
	// This is done to ensure that we don't close a node client pool that has active clients
	// or has less than maxPerNode clients in the pool.
	// We use the lastUsed timestamp to determine the least recently used node client pool.
	//
searchLRUClientPool:
	cp.clients.Range(func(key, val any) bool {
		nodeID := key.(string)
		ncPool := val.(*nodeClientPool)

		common.Assert(ncPool != nil, nodeID)
		common.Assert(ncPool.nodeID == nodeID, ncPool.nodeID, nodeID)

		common.Assert(len(ncPool.clientChan) <= int(cp.maxPerNode),
			len(ncPool.clientChan), cp.maxPerNode)

		//
		// Omit the nodeClientPool if it has any client currently in use.
		//
		if len(ncPool.clientChan) != int(cp.maxPerNode) {
			log.Debug("clientPool::closeLRUNodeClientPool: Skipping %s (%d < %d)",
				ncPool.nodeID, len(ncPool.clientChan), cp.maxPerNode)
			return true // continue iteration
		}

		if lruNcPool == nil || (ncPool.lastUsed.Load() < lruNcPool.lastUsed.Load()) {
			lruNcPool = ncPool
			lruNodeID = nodeID
		}

		return true // continue iteration
	})

	if lruNcPool == nil {
		return fmt.Errorf("clientPool::closeLRUNodeClientPool: No free nodeClientPool")
	}

	//
	// Take lock on the LRU node ID to ensure that other threads cannot create/get RPC clients
	// for this node while we are closing and deleting its node client pool.
	// The above search was done w/o the node client pool lock, so before we get the node client pool lock,
	// it's possible that some other thread has called getRPCClient() for this node and created a new client,
	// we should not close that node client pool, so we go back and search again.
	//
	lruNodeLock := cp.acquireNodeLock(lruNodeID)

	if len(lruNcPool.clientChan) != int(cp.maxPerNode) {
		log.Debug("clientPool::closeLRUNodeClientPool: Some thread raced with us and got an RPC client for %s (%d < %d)",
			lruNcPool.nodeID, len(lruNcPool.clientChan), cp.maxPerNode)
		lruNcPool = nil
		lruNodeID = ""

		cp.releaseNodeLock(lruNodeLock, lruNodeID)
		goto searchLRUClientPool
	}

	defer cp.releaseNodeLock(lruNodeLock, lruNodeID)

	err := lruNcPool.closeRPCClients()
	if err != nil {
		log.Err("clientPool::closeLRUNodeClientPool: Failed to close LRU node client pool for node %s [%v]",
			lruNodeID, err.Error())
		return err
	}

	// Never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(lruNcPool.numActive.Load() == 0 && len(lruNcPool.clientChan) == 0,
		lruNcPool.numActive.Load(), len(lruNcPool.clientChan))

	cp.deleteNodeClientPoolFromMap(lruNodeID)

	return nil
}

// closeInactiveNodeClientPools closes node client pools that have not been used for a specified timeout
func (cp *clientPool) closeInactiveNodeClientPools() {
	// Cleanup old RPC clients based on the LastUsed timestamp
	// This will run in a separate goroutine and will periodically close the node client pools based on LRU strategy
}

// closeAllNodeClientPools closes all node client pools in the pool
func (cp *clientPool) closeAllNodeClientPools() error {
	// TODO: see if we need lock here, as we are closing all node client pools
	log.Debug("clientPool::closeAllNodeClientPools: Closing all node client pools")

	//
	// Acquire write lock on the client pool to ensure that no other thread is accessing
	// the client pool while we are closing all node client pools.
	// This also waits till the read locks are released by the client pool operations
	// like getRPCClient(), releaseRPCClient(), deleteAllRPCClients(), resetAllRPCClients(), etc.
	//
	cp.acquireRWMutexWriteLock()
	defer cp.releaseRWMutexWriteLock()

	var err error
	cp.clients.Range(func(key, val any) bool {
		nodeID := key.(string)
		ncPool := val.(*nodeClientPool)

		err = ncPool.closeRPCClients()
		if err != nil {
			log.Err("clientPool::closeAllNodeClientPools: Failed to close node client pool for node %s [%v]", key, err.Error())
			return false // stop iteration
		}

		// Never delete a nodeClientPool with active connections or non-empty connection pool.
		common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
			ncPool.numActive.Load(), len(ncPool.clientChan))

		cp.deleteNodeClientPoolFromMap(nodeID)

		return true // continue iteration
	})

	common.Assert(cp.clientsCnt.Load() == 0, "client pool is not empty after closing all node client pools")
	return nil
}

// Delete nodeClientPool for the given node, if no active connections and no connections in the pool.
// Caller MUST hold the lock for the nodeID and read lock on the rwMutex
// before calling this function.
//
// Note: Don't call this function outside deleteAllRPCClients() and resetRPCClientInternal().
func (cp *clientPool) deleteNodeClientPoolIfInactive(nodeID string) bool {
	common.Assert(cp.isNodeLocked(nodeID), nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolFromMap(nodeID)

	// Caller must not call us for a non-existent pool.
	common.Assert(ncPool != nil, nodeID)

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

	cp.deleteNodeClientPoolFromMap(nodeID)

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

	// We should have created exactly numClients clients.
	common.Assert(len(ncPool.clientChan) == int(numClients), len(ncPool.clientChan), numClients)
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
	// Note: This assert can fail if resetRPCClientInternal() fails to reset one or more connections,
	//       which can happen when the target node is down or the blobfuse service is not running.
	//
	//common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode),
	//	len(ncPool.clientChan), cp.maxPerNode, ncPool.nodeID)

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
