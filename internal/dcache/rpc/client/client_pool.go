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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
)

//go:generate $ASSERT_REMOVER $GOFILE

// clientPool manages (multiple) rpc clients to multiple nodes efficiently.
type clientPool struct {
	//
	// RWMutex at the client pool level.
	// All operations except closeAllNodeClientPools() acquire read lock on this mutex.
	// This ensures that operations like getRPCClient(), releaseRPCClient(), deleteAllRPCClients(),
	// etc. by different threads can process concurrently.
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
	// We maintain information on nodes to which we have problems communicating with recently.
	// The idea is to save this information by the first thread which encounters connection
	// error and then use that to fail fast for other threads trying to communicate with the
	// same node, and save them from waiting for timeout and thus unnecessarily slowing error
	// handling.
	// The key is the node ID and the value is the time.Time when the timeout error was observed,
	// either while creating the RPC client (connection timeout) or while making an RPC call
	// (receive timeout).
	//
	// A node is removed from the negative nodes map if,
	//   - The negative timeout has expired for the node.
	//     This is done by a periodic goroutine which scans the map every few seconds.
	//   - Any RPC call to the node succeeds indicating that the connection between the client and
	//     the node is healthy.
	//     Since we don't make any RPC calls to the negative nodes, this is only for rare cases
	//     when the node was marked negative by one thread but another thread was able to get a
	//     successful RPC call.
	//
	negativeNodes    sync.Map
	negativeNodesCnt atomic.Int64

	//
	// Similar to negativeNodes map, iffyRvIdMap is another data structure that is (only) used by
	// WriteMV() to avoid making a PutChunkDC call to any RV (nexthop or in the chain) which was
	// present in the daisy	chain of RVs for a recent PutChunkDC call which failed with timeout
	// error. iffyRvIdMap stores the RV id as key and value is the time when the error was observed.
	// When we make PutChunkDC() call and one/more connections between the downstream nodes are bad,
	// it will result in timeout error. We cannot know for sure which of the RVs in the chain had
	// problem, so we mark all of them as iffy and make the PutChunk call instead of PutChunkDC.
	// That helps us exactly know which RVs are bad and must be marked inband-offline.
	// This way the RPC client can know if the RV is marked iffy. If yes, it will return error back
	// to the caller (WriteMV) indicating it to retry the operation using OriginatorSendsToAll mode.
	// Future calls can use this information to avoid making calls to the iffy RVs and save the timeouts.
	//
	// An RV is removed from the iffyRvIdMap if,
	//   - The negative timeout has expired for the RV.
	//   - Any RPC call to the RV succeeds indicating that the RV is reachable.
	//
	iffyRvIdMap    sync.Map
	iffyRvIdMapCnt atomic.Int64

	// Ticker for periodicRemoveNegativeNodesAndIffyRVs() goroutine.
	negativeNodesTicker *time.Ticker

	// Channel to stop the periodicRemoveNegativeNodesAndIffyRVs() goroutine.
	negativeNodesDone chan bool

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
	cp := &clientPool{
		nodeLock:            common.NewLockMap(),
		maxPerNode:          maxPerNode,
		maxNodes:            maxNodes,
		timeout:             timeout,
		negativeNodesTicker: time.NewTicker(5 * time.Second),
		negativeNodesDone:   make(chan bool),
	}

	go cp.periodicRemoveNegativeNodesAndIffyRVs()

	// TODO: start a goroutine to periodically close inactive RPC clients

	return cp
}

// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
// releaseRPCClient(), deleteAllRPCClients(), etc. by different threads can process
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
		common.Assert(cp.rwMutexDbgCntr.Load() == 0, cp.rwMutexDbgCntr.Load())
		cp.rwMutexDbgCntr.Store(-12345) // Special value to signify write lock.
	}
}

// Release the write lock on the rwMutex.
func (cp *clientPool) releaseRWMutexWriteLock() {
	if common.IsDebugBuild() {
		//
		// Once closeAllNodeClientPools() is done, we should have no clients left in the pool.
		// Note: Cannot assert this.
		//       See how closeAllNodeClientPools() may call this to wait for active clients of a node
		//       client pool to be released before deleting the node client pool.
		//
		//common.Assert(cp.clientsCnt.Load() == 0, cp.clientsCnt.Load())

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

	common.Assert(cp.clientsCnt.Load() <= int64(cp.maxNodes), cp.clientsCnt.Load(), cp.maxNodes)
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
		err := cp.checkNegativeNode(nodeID)
		if err != nil {
			log.Err("clientPool::getNodeClientPool: not creating RPC clients for negative node %s: %v", nodeID, err)
			// Caller should be able to identify this as a negative node error.
			common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
			return nil, err
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
		err = ncPool.createRPCClients(cp.maxPerNode)
		if err != nil {
			log.Err("clientPool::getNodeClientPool: createRPCClients(%s) failed: %v", nodeID, err)
			return nil, err
		}

		// Successfully created all required RPC clients for the node, add it to the clients map.
		cp.addNodeClientPoolToMap(nodeID, ncPool)

		// Must always create cp.maxPerNode clients to any node.
		common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode), len(ncPool.clientChan), cp.maxPerNode)

		// Brand new nodeClientPool must not be marked deleting.
		common.Assert(!ncPool.deleting.Load(), nodeID)
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
// Note: This waits enough to get a free client, so if this fails it indicates a serious issue and retrying
//	     usually won't help. Callers should treat it as such.
//       Caller can check for NegativeNodeError to see if the client couldn't be created because the node is
//       probably down.
//
// NOTE: Caller MUST NOT hold the clientPool or node level lock.

func (cp *clientPool) getRPCClient(nodeID string, highPrio bool) (*rpcClient, error) {
	log.Debug("clientPool::getRPCClient: getRPCClient(nodeID: %s, highPrio: %v)", nodeID, highPrio)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
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
	// We need to release the node lock before waiting on the clientChan.
	//
	// Q: Why is it safe to release the node lock and still use ncPool?
	// A: If ncPool has one or more active/free clients, it is guaranteed that deleteNodeClientPoolFromMap()
	//    won't delete the nodeClientPool, as it only deletes a nodeClientPool when there are no active
	//    clients and no clients in the channel.
	//    If ncPool has no active clients and no clients in the channel, then it can be deleted by
	//    deleteNodeClientPoolFromMap() after we release the node lock, but we can still safely access ncPool
	//    and it'll have no active and free clients and hence the getRPCClient() call will fail.
	//
	// Though we must use the node lock for accessing the various num* atomics to ensure proper visibility
	// order needed by various assertions.
	//
	ncPool, err := cp.getNodeClientPool(nodeID)

	if err != nil {
		cp.releaseNodeLock(nodeLock, nodeID)
		return nil, fmt.Errorf("clientPool::getRPCClient: getNodeClientPool(%s, %v) failed: %v",
			nodeID, highPrio, err)
	}

	//
	// Track number of threads waiting for a client from the pool.
	// This is only for debugging purposes, to understand the contention on the pool (both regular and high priority).
	//
	if common.IsDebugBuild() {
		ncPool.numWaiting.Add(1)
		defer ncPool.numWaiting.Add(-1)

		if highPrio {
			ncPool.numWaitingHighPrio.Add(1)
			defer ncPool.numWaitingHighPrio.Add(-1)
		}

		log.Debug("clientPool::getRPCClient: Retrieving (highPrio: %v) RPC client for node %s [free: %d, active: %d, hactive: %d, waiting: %d, hwaiting: %d]",
			highPrio, nodeID, len(ncPool.clientChan), ncPool.numActive.Load(), ncPool.numActiveHighPrio.Load(),
			ncPool.numWaiting.Load(), ncPool.numWaitingHighPrio.Load())

	}

	cp.releaseNodeLock(nodeLock, nodeID)

	//
	// Get a free client from the pool if available, else wait for a client to be released.
	// In order to catch misbehaving/stuck clients, we cap this wait. This indicates some bug
	// so we crash the program with a trace.
	// Note that accessing clientChan is thread safe, so we don't need the clientPool lock.
	//
	maxWaitTime := 60 * time.Second // in seconds

	startTime := time.Now()

	for {
		//
		// There can be a case when client pool for the node is deleted (or being deleted).
		// For example, the node goes down and RPC fails with BrokenPipe error. In this case,
		// we reset the connections available for the node, which first closes the stale connections
		// and then creates new connections. Since, the node is down, creating new RPC connection
		// to the node fails with connection refused error. So, eventually all the connections in the
		// pool are closed and client pool for the node is deleted. In this case, we do not wait
		// for a client to become available and return error to the caller.
		//
		// Note: Any code path that doesn't want getRPCClient() to return a new client and fail fast, MUST
		//       set deleting to true.
		//
		if ncPool.deleting.Load() {
			// If deleting is set deletingAt must also be set.
			common.Assert(!ncPool.deletingAt.IsZero(), nodeID)
			// Publish as NegativeNodeError as we cannot create a client because the node is probably down.
			err := fmt.Errorf("client pool deleted for node %s (%s ago), no clients available after waiting for %s: %w",
				nodeID, time.Since(ncPool.deletingAt), time.Since(startTime), NegativeNodeError)
			log.Err("clientPool::getRPCClient: %v", err)
			//
			// Once we mark a nodeClientPool as deleting, we don't allocate any new clients from it and we have
			// a timeout of 20 secs set for each client, so we should never have nodeClientPool still hanging
			// around after 30 secs.
			//
			common.Assert(time.Since(ncPool.deletingAt) < 30*time.Second,
				nodeID, ncPool.deletingAt, time.Since(ncPool.deletingAt))
			return nil, err
		}

		//
		// If node is marked negative, no point in waiting for a client to become available.
		// See above for explanation on negative nodes.
		//
		if err := cp.checkNegativeNode(nodeID); err != nil {
			err = fmt.Errorf("failing getRPCClient for negative node %s: %w", nodeID, err)
			log.Err("clientPool::getRPCClient: %v", err)
			// Caller should be able to identify this as a negative node error.
			common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
			return nil, err
		}

		//
		// Never wait more than maxWaitTime.
		// This indicates some bug and moreover we cannot legitimately proceed if we cannot get a client
		// so we panic.
		//
		if time.Since(startTime) >= maxWaitTime {
			err := fmt.Errorf("no free (highPrio: %v) RPC client for node %s, even after waiting for %s: %w",
				highPrio, nodeID, time.Since(startTime), NoFreeRPCClient)
			log.Err("clientPool::getRPCClient: %v", err)
			log.GetLoggerObj().Panicf("clientPool::getRPCClient: %v", err)
			return nil, err
		}

		select {
		case client := <-ncPool.clientChan:
			ncPool.lastUsed.Store(time.Now().Unix())
			common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
			// Nothing queued in the pool should be high priority, we set highPrio flag after client is dequeued.
			common.Assert(!client.highPrio, nodeID)

			// Take the node lock as we access the various nodeClientPool atomic counters.
			nodeLock = cp.acquireNodeLock(nodeID)

			//
			// If the node is marked negative, it means that the last RPC call to it failed with
			// timeout error. So, to prevent timeout error from happening again, we return an error
			// indicating the node is negative.
			// Though we did this check at the start of the for loop, we need to do it after we get the
			// client from the channel, as this caller may have been waiting for a free client and one of
			// the existing threads would have returned the client to the pool but only after marking the
			// node negative. Others who get a client after that must benefit from the negative node
			// information and avoid unnecessary timeouts.
			//
			if err := cp.checkNegativeNode(nodeID); err != nil {
				//
				// Release the client back to the channel.
				// Even though this node is negative, we need to signal *all* waiters on the channel as
				// we want them to wake up and take note of the fact that the node is negative and fail
				// the getRPCClient() call rightaway instead of waiting more.
				//
				ncPool.returnClientToPoolAndSignalWaiters(client, true /* signalAll */)
				cp.releaseNodeLock(nodeLock, nodeID)

				err = fmt.Errorf("failing getRPCClient for node %s, after getting the client [%w]", nodeID, err)
				log.Err("clientPool::getRPCClient: %v", err)
				// Caller should be able to identify this as a negative node error.
				common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
				return nil, err
			}

			//
			// If this is not a high priority request, make sure we don't dig into the reserved high priority
			// connections.
			//
			if !highPrio {
				if ncPool.numActiveHighPrio.Load()+int64(len(ncPool.clientChan)) < ncPool.numReservedHighPrio {
					//
					// Return back to the pool and wait for a non-high-priority connection.
					// Release the node lock before waiting on the condition variable.
					// Hold the mu lock before releasing the node lock to make sure no other go routine can
					// release a client to the pool (and signal the condition variable) before we wait on the
					// condition variable.
					//
					// Note: Since we wait on the condition variable, any path that can change the state of
					//       RPC clients for this node client pool MUST signal the condition variable after
					//       making the change, else this may end up waiting forever.
					//
					ncPool.mu.Lock()
					cp.releaseNodeLock(nodeLock, nodeID)

					ncPool.clientChan <- client
					ncPool.cond.Wait()
					ncPool.mu.Unlock()
					continue
				}
			} else {
				client.highPrio = true
				ncPool.numActiveHighPrio.Add(1)
			}
			ncPool.numActive.Add(1)

			log.Debug("clientPool::getRPCClient: Successfully retrieved (highPrio: %v) RPC client (%p) for node %s [free: %d, active: %d, hactive: %d, waiting: %d, hwaiting: %d], waited for %s",
				highPrio, client, nodeID, len(ncPool.clientChan), ncPool.numActive.Load(),
				ncPool.numActiveHighPrio.Load(), ncPool.numWaiting.Load(), ncPool.numWaitingHighPrio.Load(),
				time.Since(startTime))

			// numActive includes both high priority and regular active connections.
			common.Assert((ncPool.numActiveHighPrio.Load() <= ncPool.numActive.Load()),
				ncPool.numActiveHighPrio.Load(), ncPool.numActive.Load(), client.nodeID, highPrio)
			common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
				ncPool.numActive.Load(), cp.maxPerNode, client.nodeID, highPrio)
			common.Assert((ncPool.numWaitingHighPrio.Load() <= ncPool.numWaiting.Load()),
				ncPool.numWaitingHighPrio.Load(), ncPool.numWaiting.Load(), client.nodeID, highPrio)

			cp.releaseNodeLock(nodeLock, nodeID)
			client.allocatedAt = time.Now()
			return client, nil
		case <-time.After(2 * time.Second): // Timeout after 2 second
			log.Debug("clientPool::getRPCClient: No free (highPrio: %v) RPC client for node %s (active: %d, hactive:%d, waiting: %d, hwaiting: %d), for %s",
				highPrio, nodeID, ncPool.numActive.Load(), ncPool.numActiveHighPrio.Load(),
				ncPool.numWaiting.Load(), ncPool.numWaitingHighPrio.Load(), time.Since(startTime))
			// Continue the for loop, various exit checks will be done there.
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
		// Nothing queued in the pool should be high priority.
		common.Assert(!client.highPrio, nodeID)
		ncPool.numActive.Add(1)
		return client, nil
	default:
		err := fmt.Errorf("no free RPC client for node %s: %w", nodeID, NoFreeRPCClient)
		log.Err("clientPool::getRPCClientNoWait: %v", err)
		return nil, err
	}
}

// releaseRPCClient releases a RPC client back to the pool
//
// NOTE: Caller MUST NOT hold the clientPool or node level lock.
func (cp *clientPool) releaseRPCClient(client *rpcClient) error {
	log.Debug("clientPool::releaseRPCClient: releaseRPCClient(client: %p, nodeID: %s, highPrio: %v)",
		client, client.nodeID, client.highPrio)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
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

	//
	// It's possible that we got a connection error on some earlier client/connection to this node and hence
	// marked the nodeClientPool as deleting. This RPC response may have come before the target process restarted
	// and hence this got a success response, but this got processed after a connection with error. We should
	// continue with deleting the nodeClientPool.
	//
	if ncPool.deleting.Load() {
		log.Debug("clientPool::releaseRPCClient: Successful RPC response being processed after nodeClientPool is marked deleting, continuing with deleteAllRPCClients, client: %p, nodeID: %s", client, client.nodeID)

		cp.releaseNodeLock(nodeLock, client.nodeID)
		cp.releaseRWMutexReadLock()

		cp.deleteAllRPCClients(client, false /* confirmedBadNode */, false /* isClientClosed */)

		cp.acquireRWMutexReadLock()
		nodeLock = cp.acquireNodeLock(client.nodeID)
		return nil
	}

	log.Debug("clientPool::releaseRPCClient: %p after %s, node: %s, free: %d, active: %d, hactive: %d, waiting: %d, hwaiting: %d, maxPerNode: %d",
		client, time.Since(client.allocatedAt), client.nodeID, len(ncPool.clientChan), ncPool.numActive.Load(),
		ncPool.numActiveHighPrio.Load(), ncPool.numWaiting.Load(), ncPool.numActiveHighPrio.Load(), cp.maxPerNode)

	// We must release only to a non-full pool.
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		client.nodeID, len(ncPool.clientChan), cp.maxPerNode)

	// numActive includes both high priority and regular active connections.
	common.Assert((ncPool.numActiveHighPrio.Load() <= ncPool.numActive.Load()),
		ncPool.numActiveHighPrio.Load(), ncPool.numActive.Load(), client.nodeID, client.highPrio)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID, client.highPrio)

	if client.highPrio {
		common.Assert(ncPool.numActiveHighPrio.Load() > 0, ncPool.numActive.Load(), client.nodeID)
		ncPool.numActiveHighPrio.Add(-1)
	}

	// Must be releasing an active client.
	common.Assert(ncPool.numActive.Load() > 0, ncPool.numActiveHighPrio.Load(), client.nodeID, client.highPrio)
	ncPool.numActive.Add(-1)

	client.highPrio = false
	ncPool.returnClientToPoolAndSignalWaiters(client, false /* signalAll */)

	return nil
}

// closeRPCClient closes an RPC client.
// The client MUST have been removed from the pool using a prior getRPCClient() call.
func (cp *clientPool) closeRPCClient(client *rpcClient) error {
	log.Debug("clientPool::closeRPCClient: Closing RPC client (%p) to %s node %s",
		client, client.nodeAddress, client.nodeID)

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
//       before calling this function.

func (cp *clientPool) deleteRPCClient(client *rpcClient) {
	log.Debug("clientPool::deleteRPCClient: Deleting RPC client (%p) to %s node %s",
		client, client.nodeAddress, client.nodeID)

	common.Assert(cp.isNodeLocked(client.nodeID), client.nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	// Close the client first.
	err := cp.closeRPCClient(client)
	if err != nil {
		log.Err("clientPool::deleteRPCClient: closeRPCClient(%p) to %s node %s failed: %v",
			client, client.nodeAddress, client.nodeID, err)
		// We don't expect connection close to fail, let's know if it happens.
		common.Assert(false, err)
		// TODO: This will cause a socket fd leak.
	}

	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)
	common.Assert(ncPool != nil, client.nodeID)

	//
	// deleteRPCClient() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	// numActive includes both high priority and regular active connections.
	common.Assert((ncPool.numActiveHighPrio.Load() <= ncPool.numActive.Load()),
		ncPool.numActiveHighPrio.Load(), ncPool.numActive.Load(), client.nodeID, client.highPrio)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID, client.highPrio)

	if client.highPrio {
		common.Assert(ncPool.numActiveHighPrio.Load() > 0, ncPool.numActive.Load(), client.nodeID)
		ncPool.numActiveHighPrio.Add(-1)
	}

	//
	// Must only delete an active client.
	// Also, clients which are deleted are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0, ncPool.numActiveHighPrio.Load(), client.nodeID, client.highPrio)
	ncPool.numActive.Add(-1)
}

// Delete all connections in the client pool corresponding to 'client'. This client would have been allocated
// using a prior call to getRPCClient().
// This is used for draining connections in the connection pool in case there is timeout error while
// making an RPC call to the target node.
// It closes the passed in connection and all existing connections in the pool, and if there are no
// active connections and no connections in the channel, it deletes the node client pool.
//
// It also takes a boolean flag to indicate if the client has been closed or not. This is used to avoid
// double closing of the client. In PutChunkDC we can get timeout because of bad connection between the
// downstream nodes, and not necessarily between the client node and target/next-hop node. So, we reset
// the client for the target node. Resetting involves closing the client first and then creating a new one.
// If the target node is bad, then RPC client creation fails. We then call deleteAllRPCClients() to delete
// all clients to the target node, which tries to close the same client again which was already closed by the
// reset workflow. So, to prevent this, we use this flag.
// The value of this flag is true only in case of PutChunkDC timeout error when the target node is confirmed bad.
func (cp *clientPool) deleteAllRPCClients(client *rpcClient, confirmedBadNode bool, isClientClosed bool) {
	log.Debug("clientPool::deleteAllRPCClients: Deleting all RPC clients for %s node %s, client: %p, confirmedBadNode: %v, isClientClosed: %v, adding to negative nodes map",
		client.nodeAddress, client.nodeID, client, confirmedBadNode, isClientClosed)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
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

	//
	// deleteAllRPCClients() is called only when an RPC call to the node fails with timeout error.
	// Add it to the negative nodes map to help other threads fail fast instead of waiting for timeout.
	//
	if confirmedBadNode {
		cp.addNegativeNode(client.nodeID)
	}

	numConnDeleted := 0
	ncPool := cp.getNodeClientPoolFromMap(client.nodeID)

	//
	// Node client pool may not be present for the node in case of PutChunkDC timeout error,
	// where we first reset the client, which closes the client and then creates a new client.
	// If the new client creation fails, we come here to delete all clients. Meanwhile after
	// reset has released the node level lock, some other thread may have closed all the clients
	// for the target node and deleted the node client pool.
	// So, we assert here that the client passed in the argument is closed.
	//
	if ncPool == nil {
		log.Debug("clientPool::deleteAllRPCClients: No client pool found for node %s at %s, nothing to delete",
			client.nodeID, client.nodeAddress)
		common.Assert(isClientClosed, client.nodeID, client.nodeAddress)
		return
	}

	// client is allocated from the pool, so pool must exist.
	common.Assert(ncPool != nil, client.nodeID)
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// Waiters in getRPCClient() need to know and re-evaluate.
	// This is regardless of whether we are able to delete all clients or not.
	//
	defer ncPool.returnClientToPoolAndSignalWaiters(nil /* client */, true /* signalAll */)

	//
	// We need to take the mu lock to prevent other threads from releasing a client to the pool
	// after we get numClients below, and before we delete all clients.
	// MAKE SURE NO FUNCTION CALLED FROM THIS POINT TILL THE DEFER RELEASES THE MU LOCK, TRIES TO
	// ACQUIRE MU LOCK, also note that the returnClientToPoolAndSignalWaiters() above acquires mu
	// lock, so defer mu.Unlock() MUST BE AFTER defer returnClientToPoolAndSignalWaiters(), so that
	// we don't have a deadlock.
	//
	ncPool.mu.Lock()
	defer ncPool.mu.Unlock()

	//
	// Clients present in the pool. The one that we are deleting is not in the pool so the pool can have
	// max cp.maxPerNode-1 clients. If it's less than cp.maxPerNode-1, rest are currently allocated to other
	// callers. We cannot replenish those. Those will be deleted by their respective caller when their RPC
	// requests fail with "timeout" error.
	//
	numClients := len(ncPool.clientChan)
	common.Assert(numClients < int(cp.maxPerNode), numClients, cp.maxPerNode)

	log.Debug("clientPool::deleteAllRPCClients: node %s (%s), client: %p, numActive: %d (%d, %d)",
		client.nodeID, client.nodeAddress, client, ncPool.numActive.Load(), numClients, cp.maxPerNode)

	//
	// In PutChunkDC fails with timeout, we reset the client to the target node, which first closes the
	// client and then creates a new client. If the new client creation fails, we delete all the clients
	// to the target node, which tries to close the same client again which was already closed by the reset
	// workflow. So, to prevent this, we check if the client is already closed.
	//
	if isClientClosed {
		log.Debug("clientPool::deleteAllRPCClients: client (%p) to %s node %s is already closed",
			client, client.nodeAddress, client.nodeID)
	} else {
		//
		// Delete this client. This closes this client and removes it from the pool.
		// It can only fail if Thrift fails to close the connection.
		// This should technically not happen, so we assert.
		//
		cp.deleteRPCClient(client)
	}

	numConnDeleted++

	//
	// Delete all remaining clients in the pool. We try to delete as many as we can, and don't fail
	// on error, as we have deleted at least one client.
	//
	for i := 0; i < numClients; i++ {
		client1, err := cp.getRPCClientNoWait(client.nodeID)
		if err != nil {
			//
			// There can be a race condition between when we get numClients (count of number of free clients
			// in the channel) and in the getRPCClient() method where a thread can acquire a free client from
			// the channel. In getRPCClient(), a thread acquires a free client outside the node level lock, so
			// we can expect that the number of free clients in the channel can be less than numClients count.
			// So, we cannot assert here that we will always get a free client.
			//
			log.Err("clientPool::deleteAllRPCClients: getRPCClientNoWait failed: %v", err)
			common.Assert(errors.Is(err, NoFreeRPCClient), err)
			break
		}

		cp.deleteRPCClient(client1)
		numConnDeleted++
	}

	log.Debug("clientPool::deleteAllRPCClients: Deleted %d RPC clients to %s node %s, now available (%d / %d), active: %d",
		numConnDeleted, client.nodeAddress, client.nodeID, len(ncPool.clientChan),
		cp.maxPerNode, ncPool.numActive.Load())

	// We must have deleted at least the client we are called for (and maybe more).
	common.Assert(numConnDeleted > 0)

	// We don't expect failure closing any client connection, so there shouldn't be any client left in the pool.
	common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan), client.nodeAddress, client.nodeID)

	//
	// Mark it deleting so that getRPCClient() does not allocate any more clients for this node, till all
	// the existing clients are closed and the nodeClientPool is deleted and recreated afresh.
	//
	// Since we have timeout set for every RPC connection, eventually all connections will get closed and
	// corresponding clients deleted.
	//
	if !ncPool.deleting.Swap(true) {
		ncPool.deletingAt = time.Now()
	} else {
		common.Assert(!ncPool.deletingAt.IsZero(), ncPool.nodeID)
	}

	//
	// After deleting all clients, if there are no active connections and no connections in the channel,
	// we delete the node client pool itself.
	//
	cp.deleteNodeClientPoolIfInactive(client.nodeID)
}

// waitForNodeClientPoolToDelete waits till the node client pool for the given node is deleted, which
// means that all existing connections in the pool are closed and deleted. Any new request for a client after
// this would create a new node client pool with new connections. This is useful when we get a connection error
// from a node and we want to make sure that all existing connections to the node are closed, before attempting
// to send new requests to the node.
// A to-be-deleted nodeClientPool waiting for existing connections to drain has "deleting" set to true, so this
// waits till either of the following happens:
// - getNodeClientPoolFromMap() return nil, which means the nodeClientPool is deleted.
// - nodeClientPool.deleting is false, which means that the nodeClientPool is recreated after being deleted.
func (cp *clientPool) waitForNodeClientPoolToDelete(nodeID string) error {
	log.Debug("clientPool::waitForNodeClientPoolToDelete: node %s", nodeID)

	//
	// For a negative node there's not much point in waiting for the node client pool to be deleted.
	// We anyways won't be able to create a new client to the node, so return error rightaway.
	//
	err := cp.checkNegativeNode(nodeID)
	if err != nil {
		err = fmt.Errorf("not waiting for node client pool to delete for negative node %s: %w", nodeID, err)
		log.Debug("clientPool::waitForNodeClientPoolToDelete: %v", err)
		return err
	}

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	nodeLock := cp.acquireNodeLock(nodeID)
	defer cp.releaseNodeLock(nodeLock, nodeID)

	startTime := time.Now()

	for {
		ncPool := cp.getNodeClientPoolFromMap(nodeID)
		if ncPool == nil || !ncPool.deleting.Load() {
			log.Debug("clientPool::waitForNodeClientPoolToDelete: node: %s, now deleted (after %s)!",
				nodeID, time.Since(startTime))
			return nil
		}

		common.Assert(ncPool.nodeID == nodeID, ncPool.nodeID, nodeID)

		//
		// Caller will call waitForNodeClientPoolToDelete() when they get some connection errors when
		// communicating with this node, so typically all connections will fail fast and nodeClientPool
		// will be deleted almost immediately, but we give it some time.
		//
		if time.Since(startTime) > 10*time.Second {
			err := fmt.Errorf("timeout (%s) waiting for node client pool to be deleted for node %s",
				time.Since(startTime), nodeID)
			log.Err("clientPool::waitForNodeClientPoolToDelete: %v", err)
			return err
		}

		//
		// If it's deleting, wait for it to be deleted.
		// Since we expect it to be deleted soon, we wait for a short time.
		//
		cp.releaseNodeLock(nodeLock, nodeID)
		time.Sleep(100 * time.Millisecond)
		cp.acquireNodeLock(nodeID)
	}
}

// Close client and create a new one to the same target/node as client.
//
// Note: This is an internal function, use resetRPCClient() instead.

func (cp *clientPool) resetRPCClientInternal(client *rpcClient, needLock bool) error {
	log.Debug("clientPool::resetRPCClientInternal: client %s for node %s",
		client.nodeAddress, client.nodeID)

	//
	// First close the client and then create a new one and add that to the pool.
	//
	err := cp.closeRPCClient(client)
	if err != nil {
		// Closing the socket should not fail, so we assert.
		common.Assert(false, err, client.nodeAddress, client.nodeID)
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

	// numActive includes both high priority and regular active connections.
	common.Assert((ncPool.numActiveHighPrio.Load() <= ncPool.numActive.Load()),
		ncPool.numActiveHighPrio.Load(), ncPool.numActive.Load(), client.nodeID, client.highPrio)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID, client.highPrio)

	if client.highPrio {
		common.Assert(ncPool.numActiveHighPrio.Load() > 0, ncPool.numActive.Load(), client.nodeID)
		ncPool.numActiveHighPrio.Add(-1)
	}

	//
	// Must only reset an active client.
	// Also, clients which are reset are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0, ncPool.numActiveHighPrio.Load(), client.nodeID, client.highPrio)
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
		// Also seen connection reset, connection timeout and no route to host errors here.
		//
		// In any case when we come here that means we are not able to replenish connections to
		// the target, in the connection pool. When we have no active connections and no more left
		// in the pool, we can delete the nodeClientPool itself.
		//
		common.Assert(rpc.IsConnectionRefused(err) ||
			rpc.IsConnectionReset(err) ||
			rpc.IsTimedOut(err) ||
			rpc.IsNoRouteToHost(err) ||
			errors.Is(err, NegativeNodeError), err)

		cp.deleteNodeClientPoolIfInactive(client.nodeID)
		return err
	}

	//
	// Reset was successful, so we have at least one good connection to the target node.
	// Clear deleting if we had set it earlier.
	//
	ncPool.deleting.Store(false)

	// Add the new client to the client pool for this node and wakeup one waiter in getRPCClient().
	ncPool.returnClientToPoolAndSignalWaiters(newClient, false /* signalAll */)

	return nil
}

// Close client and create a new one to the same target/node as 'client'.
// This is typically used when a node goes down and comes back up, rendering all existing clients
// "broken", they need to be replaced with a brand new connection/client.
func (cp *clientPool) resetRPCClient(client *rpcClient) error {
	return cp.resetRPCClientInternal(client, true /* needLock */)
}

// Close the least recently used node client pool from the client pool.
//
// NOTE: Caller MUST hold the read lock on the rwMutex before calling this function.
func (cp *clientPool) closeLRUNodeClientPool() error {
	common.Assert(cp.clientsCnt.Load() == int64(cp.maxNodes),
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
		if ncPool.numActive.Load() > 0 {
			log.Debug("clientPool::closeLRUNodeClientPool: Skipping %s with active clients, numActive: %d (%d, %d)",
				ncPool.nodeID, ncPool.numActive.Load(), len(ncPool.clientChan), cp.maxPerNode)
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
	// it's possible that some other thread has called getRPCClient() for this node and got a new client,
	// we should not close that node client pool, so we go back and search again.
	//
	lruNodeLock := cp.acquireNodeLock(lruNodeID)

	if lruNcPool.numActive.Load() > 0 {
		log.Debug("clientPool::closeLRUNodeClientPool: Some thread raced with us and got an RPC client for %s, numActive: %d (%d, %d)",
			lruNcPool.nodeID, lruNcPool.numActive.Load(), len(lruNcPool.clientChan), cp.maxPerNode)
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
	log.Debug("clientPool::closeAllNodeClientPools: Closing all %d node client pools", cp.clientsCnt.Load())

	//
	// Acquire write lock on the client pool to ensure that no other thread is accessing
	// the client pool while we are closing all node client pools.
	// This also waits till the read locks are released by the client pool operations
	// like getRPCClient(), releaseRPCClient(), deleteAllRPCClients(), etc.
	//
	cp.acquireRWMutexWriteLock()
	defer cp.releaseRWMutexWriteLock()

	startTime := time.Now()

	// Wait for max 60 seconds for all active clients to be released back to the channel.
	maxWaitTime := 60 * time.Second

	var err error
	cp.clients.Range(func(key, val any) bool {
		nodeID := key.(string)
		ncPool := val.(*nodeClientPool)
		//
		// Mark it deleting so that getRPCClient() does not allocate any more clients for this node.
		// Also wakeup any waiters in getRPCClient() so that they can fail fast.
		//
		if !ncPool.deleting.Swap(true) {
			ncPool.deletingAt = time.Now()
		} else {
			common.Assert(!ncPool.deletingAt.IsZero(), ncPool.nodeID)
		}
		ncPool.returnClientToPoolAndSignalWaiters(nil /* client */, true /* signalAll */)

		//
		// Check if there are any active clients for this node. If yes, release the write lock and wait for a second
		// for the active clients to be released back to the channel. After that acquire the write lock and recheck.
		//
		for ncPool.numActive.Load() > 0 && time.Since(startTime) < maxWaitTime {
			log.Debug("clientPool::closeAllNodeClientPools: sleeping till %d active clients for %s are released, (%d, %d)",
				ncPool.numActive.Load(), nodeID, len(ncPool.clientChan), cp.maxPerNode)

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
			err = fmt.Errorf("Node %s has %d active clients (%d, %d), cannot close even after waiting for %v",
				nodeID, ncPool.numActive.Load(), len(ncPool.clientChan), cp.maxPerNode, maxWaitTime)
			log.Err("clientPool::closeAllNodeClientPools: %v", err)
			return false // stop iteration
		}

		err = ncPool.closeRPCClients()
		if err != nil {
			err = fmt.Errorf("Failed to close RPC clients for node %s [%v]", nodeID, err)
			log.Err("clientPool::closeAllNodeClientPools: %v", err)
			return false // stop iteration
		}

		// Never delete a nodeClientPool with active connections or non-empty connection pool.
		common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
			ncPool.numActive.Load(), len(ncPool.clientChan))

		cp.deleteNodeClientPoolFromMap(nodeID)

		return true // continue iteration
	})

	if err != nil {
		return err
	}

	// client pool is not empty after closing all node client pools?
	common.Assert(cp.clientsCnt.Load() == 0, cp.clientsCnt.Load())
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

// AddNegativeNode adds a node to the negative nodes map when,
//   - The RPC client creation to the node failed due to timeout.
//   - RPC call to the node failed due to timeout.
func (cp *clientPool) addNegativeNode(nodeID string) bool {
	common.Assert(common.IsValidUUID(nodeID), nodeID)
	common.Assert(cp.isNodeLocked(nodeID), nodeID)

	now := time.Now()
	val, alreadyPresent := cp.negativeNodes.LoadOrStore(nodeID, now)
	_ = val

	if !alreadyPresent {
		// New entry added.
		cp.negativeNodesCnt.Add(1)

		log.Debug("clientPool::addNegativeNode: added (%s -> %s) to negativeNodes (total count: %d)",
			nodeID, now, cp.negativeNodesCnt.Load())

		//
		// Signal any waiters in getRPCClient(), they must recheck the negativeNodes map and fail fast.
		// When called from newRPCClient() we may not have the nodeClientPool yet.
		//
		ncPool := cp.getNodeClientPoolFromMap(nodeID)
		if ncPool != nil {
			ncPool.returnClientToPoolAndSignalWaiters(nil /* client */, true /* signalAll */)
		}

		return true
	}

	// Existing entry, update the timestamp without increasing negativeNodesCnt.
	cp.negativeNodes.Store(nodeID, now)

	log.Debug("clientPool::addNegativeNode: updated (%s -> [%s -> %s]) in negativeNodes (total count: %d)",
		nodeID, val.(time.Time), now, cp.negativeNodesCnt.Load())

	return false
}

// RemoveNegativeNode removes a node from the negative nodes map when,
//   - periodicRemoveNegativeNodesAndIffyRVs() goroutine which checks if the defaultNegativeTimeout
//     has expired for the node.
//   - successful RPC call to the node indicating that the connection between the client and the
//     node is healthy.
func (cp *clientPool) removeNegativeNode(nodeID string) bool {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	// Fast path, keep it quick.
	if cp.negativeNodesCnt.Load() == 0 {
		return false
	}

	if val, ok := cp.negativeNodes.LoadAndDelete(nodeID); ok {
		_ = val
		common.Assert(cp.negativeNodesCnt.Load() > 0, cp.negativeNodesCnt.Load(), nodeID, val.(time.Time))
		cp.negativeNodesCnt.Add(-1)

		log.Debug("clientPool::removeNegativeNode: removed (%s -> %s) from negativeNodes (total count: %d)",
			nodeID, val.(time.Time), cp.negativeNodesCnt.Load())
		return true
	}

	return false
}

// Check if the node is marked negative.
func (cp *clientPool) IsNegativeNode(nodeID string) bool {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	// Fast path, avoid lock.
	if cp.negativeNodesCnt.Load() == 0 {
		return false
	}

	_, ok := cp.negativeNodes.Load(nodeID)
	return ok
}

// Check if the given node is marked negative.
// It is same as IsNegativeNode(), but this method returns an appropriately wrapped error which can be used by the
// callers to check for negative node error.
func (cp *clientPool) checkNegativeNode(nodeID string) error {
	// Fast path, keep it quick.
	if cp.negativeNodesCnt.Load() > 0 {
		if val, ok := cp.negativeNodes.Load(nodeID); ok {
			err := fmt.Errorf("%w: %s (%s ago)", NegativeNodeError, nodeID, time.Since(val.(time.Time)))
			log.Err("clientPool::checkNegativeNode: %v", err)
			// Caller should be able to identify this as a negative node error.
			common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
			return err
		}
	}
	return nil
}

// Called from fixMV() in cluster_manager to initialize the "excluded nodes" map from the known negative nodes.
func GetNegativeNodes() map[int]struct{} {
	negativeNodes := make(map[int]struct{})

	// Fast path, keep it quick.
	if cp.negativeNodesCnt.Load() > 0 {
		cp.negativeNodes.Range(func(key, value any) bool {
			nodeID := key.(string)
			common.Assert(common.IsValidUUID(nodeID), nodeID)

			log.Debug("clientPool::GetNegativeNodes: Negative node: %s (%s ago) excluded from fix MV",
				nodeID, time.Since(value.(time.Time)))
			negativeNodes[cm.UUIDToUniqueInt(nodeID)] = struct{}{}
			return true
		})
	}

	return negativeNodes
}

// Add RV id to the iffyRvIdMap.
// When PutChunkDC() fails with timeout error, we add the next-hop RV and all the RVs in the chain
// to the iffyRvIdMap.
func (cp *clientPool) addIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	now := time.Now()
	val, alreadyPresent := cp.iffyRvIdMap.LoadOrStore(rvID, now)
	_ = val

	if !alreadyPresent {
		// New entry added.
		cp.iffyRvIdMapCnt.Add(1)

		log.Debug("clientPool::addIffyRvById: added (%s -> %s) to iffyRvIdMap (total count: %d)",
			rvID, now, cp.iffyRvIdMapCnt.Load())
		return true
	}

	// Existing entry, update the timestamp without increasing iffyRvIdMapCnt.
	cp.iffyRvIdMap.Store(rvID, now)

	log.Debug("clientPool::addIffyRvById: updated (%s -> [%s -> %s]) in iffyRvIdMap (total count: %d)",
		rvID, val.(time.Time), now, cp.iffyRvIdMapCnt.Load())

	return false
}

// Add the RV name to the iffyRvIdMap.
// This method internally calls the addIffyRvById method.
func (cp *clientPool) addIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)

	if cp.addIffyRvId(cm.RvNameToId(rvName)) {
		log.Debug("clientPool::addIffyRvByName: added %s to iffyRvIdMap", rvName)
		return true
	} else {
		log.Debug("clientPool::addIffyRvByName: updated %s in iffyRvIdMap", rvName)
		return false
	}
}

// RemoveIffyRV removes an RV from the iffyRvIdMap.
// An RV is removed from the map by,
//   - periodicRemoveNegativeNodesAndIffyRVs() goroutine which checks if the defaultNegativeTimeout
//     has expired for the RV.
//   - successful RPC call to the RV indicating that the connection between the client and the
//     RV is healthy.
func (cp *clientPool) removeIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	// Fast path, keep it quick.
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	if val, ok := cp.iffyRvIdMap.LoadAndDelete(rvID); ok {
		_ = val
		common.Assert(cp.iffyRvIdMapCnt.Load() > 0, cp.iffyRvIdMapCnt.Load(), rvID, val.(time.Time))
		cp.iffyRvIdMapCnt.Add(-1)

		log.Debug("clientPool::removeIffyRvById: removed (%s -> %s) from iffyRvIdMap (total count: %d)",
			rvID, val.(time.Time), cp.iffyRvIdMapCnt.Load())
		return true
	}

	return false
}

// Remove the RV name from the iffyRvIdMap.
// This method internally calls the removeIffyRvById method.
func (cp *clientPool) removeIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)

	// Fast path, keep it quick.
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	if cp.removeIffyRvId(cm.RvNameToId(rvName)) {
		log.Debug("clientPool::removeIffyRvByName: removed %s from iffyRvIdMap", rvName)
		return true
	}

	return false
}

// Check if an RV id is marked iffy.
func (cp *clientPool) isIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	// Fast path, avoid lock
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	_, ok := cp.iffyRvIdMap.Load(rvID)
	return ok
}

// Check if an RV name is marked iffy.
func (cp *clientPool) isIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)
	return cp.isIffyRvId(cm.RvNameToId(rvName))
}

// Goroutine which runs every 5 seconds and removes expired nodes and RVs from the
// negativeNodes and iffyRvIdMap.
func (cp *clientPool) periodicRemoveNegativeNodesAndIffyRVs() {
	log.Info("clientPool::periodicRemoveNegativeNodesAndIffyRVs: Starting")

	for {
		select {
		case <-cp.negativeNodesDone:
			log.Info("clientPool::periodicRemoveNegativeNodesAndIffyRVs: Stopping")
			return
		case <-cp.negativeNodesTicker.C:
			// remove entries from negativeNodes map based on timeout
			cp.negativeNodes.Range(func(key, value any) bool {
				nodeID := key.(string)
				common.Assert(common.IsValidUUID(nodeID), nodeID)

				addedTime := value.(time.Time)

				if time.Since(addedTime) > defaultNegativeTimeout*time.Second {
					log.Debug("clientPool::periodicRemoveNegativeNodesAndIffyRVs: removing negative node %s (%s)",
						nodeID, time.Since(addedTime))
					cp.removeNegativeNode(nodeID)
				}

				return true
			})

			// remove entries from iffyRvIdMap based on timeout
			cp.iffyRvIdMap.Range(func(key, value any) bool {
				rvID := key.(string)
				common.Assert(common.IsValidUUID(rvID), rvID)

				addedTime := value.(time.Time)

				if time.Since(addedTime) > defaultNegativeTimeout*time.Second {
					log.Debug("clientPool::periodicRemoveNegativeNodesAndIffyRVs: removing iffy RV %s (%s)",
						rvID, time.Since(addedTime))
					cp.removeIffyRvId(rvID)
				}

				return true
			})
		}
	}
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// nodeClientPool holds a channel of RPC clients for a node
// and the last used timestamp for LRU eviction
type nodeClientPool struct {
	nodeID string // Node ID of the node this client pool is for
	//
	// Clients must be added to clientChan with mu locked, and cond must be signal'ed to wake up any go routine
	// that might be waiting for a non-highPrio free client. Clients can be dequeued from clientChan without holding
	// mu as channel operations are thread safe and moreover no go routine is interested in client dequeue event.
	// We have a single channel for both high priority and regular clients, but not all clients can be allocated
	// as regular clients, as some are reserved for high priority clients, so callers needing a regular client might
	// need to wait after dequeuing a client from the channel if all non-reserved clients are in use. They wait
	// on cond variable which MUST be signal'ed when a client is returned to the channel.
	// See comments above nodeClientPool.highPrio.
	//
	// Note: mu lock can be safely held inside nodeLock, but not the other way round.
	// Note: Both cond.Wait() and cond.Signal()/cond.Broadcast() must be called with mu locked.
	//
	clientChan chan *rpcClient // channel to hold the RPC clients to a node
	cond       *sync.Cond
	mu         sync.Mutex
	lastUsed   atomic.Int64 // used for evicting inactive RPC clients based on LRU (seconds since epoch)
	//
	// These atomic counters are used for debugging and assertions, so the order of updates is important,
	// hence they MUST be accessed with the nodeLock held.
	//
	numActive           atomic.Int64 // number of clients currently created using getRPCClient() call.
	numWaiting          atomic.Int64 // number of users waiting for a free client in getRPCClient().
	numWaitingHighPrio  atomic.Int64 // number of high priority callers waiting for a free client in getRPCClient().
	numActiveHighPrio   atomic.Int64 // number of high priority clients currently active.
	numReservedHighPrio int64        // number of high priority clients reserved for this node.
	deleting            atomic.Bool  // true when the nodeClientPool is being deleted.
	deletingAt          time.Time    // time when deleting was set to true, used for debugging.
}

// Return the client to clientChan and signal one/all of the waiters (if any).
// If client is nil, just signal the waiters.
func (ncPool *nodeClientPool) returnClientToPoolAndSignalWaiters(client *rpcClient, signalAll bool) {

	ncPool.mu.Lock()
	// First add to the channel and then signal waiter(s).
	if client != nil {
		common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode), len(ncPool.clientChan), cp.maxPerNode)
		ncPool.clientChan <- client
	}
	if signalAll {
		ncPool.cond.Broadcast()
	} else {
		ncPool.cond.Signal()
	}
	ncPool.mu.Unlock()
}

// createRPCClients creates a channel of RPC clients of size numClients for the specified node ID
func (ncPool *nodeClientPool) createRPCClients(numClients uint32) error {
	common.Assert(cp.isNodeLocked(ncPool.nodeID), ncPool.nodeID)

	//
	// With maxPerNode==64, we get 16 regular and 48 high priority clients.
	// All other requests, other than PutChunkDC use the regular priority clients.
	// 16 connections should be enough for PutChunk/PutChunkDC/GetChunk requests to saturate the network.
	//
	// TODO: Make sure 16 clients per node are enough for extra large clusters for various workflows
	//       like fixMV, resync, and other heavy data movement operations like GetChunk.
	//
	numReservedHighPrio := int64(numClients - (numClients / 4))
	common.Assert(numReservedHighPrio > 0 && numReservedHighPrio < int64(numClients),
		numReservedHighPrio, numClients)

	log.Debug("nodeClientPool::createRPCClients: Creating %d RPC clients (%d high prio) for node %s",
		numClients, numReservedHighPrio, ncPool.nodeID)

	common.Assert(ncPool.clientChan == nil)
	common.Assert(ncPool.numActive.Load() == 0, ncPool.numActive.Load())
	common.Assert(common.IsValidUUID(ncPool.nodeID))

	ncPool.clientChan = make(chan *rpcClient, numClients)
	ncPool.lastUsed.Store(time.Now().Unix())
	ncPool.numReservedHighPrio = numReservedHighPrio
	ncPool.cond = sync.NewCond(&ncPool.mu)

	var err error

	// Create RPC clients and add them to the channel.
	for i := 0; i < int(numClients); i++ {
		var client *rpcClient
		client, err = newRPCClient(ncPool.nodeID, rpc.GetNodeAddressFromID(ncPool.nodeID))
		if err != nil {
			log.Err("nodeClientPool::createRPCClients: Failed to create RPC client for node %s [%v]",
				ncPool.nodeID, err)
			//
			// Only valid reason could be connection refused as the blobfuse process is not running on
			// the remote node, a timeout if the node is down, no route to host error in some specific
			// unreachability conditions, or NegativeNodeError if the node is marked negative and newRPCClient()
			// proactively failed the request. There is no point in retrying in that case.
			//
			common.Assert(rpc.IsConnectionRefused(err) ||
				rpc.IsTimedOut(err) ||
				rpc.IsNoRouteToHost(err) ||
				errors.Is(err, NegativeNodeError), err)

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
	common.Assert(ncPool.numActiveHighPrio.Load() == 0, ncPool.numActiveHighPrio.Load())
	// We must have reserved some high priority clients.
	common.Assert(ncPool.numReservedHighPrio > 0, ncPool.numReservedHighPrio)
	// At least some regular clients must be there.
	common.Assert(ncPool.numReservedHighPrio < int64(len(ncPool.clientChan)),
		ncPool.numReservedHighPrio, len(ncPool.clientChan))

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

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// Given the component RVs list, return the RVs which are marked iffy.
func GetIffyRVs(nextHopRV *string, nextRVs *[]string) *[]string {
	common.Assert(nextHopRV != nil)
	common.Assert(nextRVs != nil)

	// Common case, keep it quick.
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return nil
	}

	iffyRVs := make([]string, 0, len(*nextRVs)+1)

	// Check the next-hop RV.
	if cp.isIffyRvName(*nextHopRV) {
		iffyRVs = append(iffyRVs, *nextHopRV)
	}

	// And all other RVs in the chain.
	for _, rv := range *nextRVs {
		common.Assert(rv != *nextHopRV, rv, *nextHopRV)
		if cp.isIffyRvName(rv) {
			iffyRVs = append(iffyRVs, rv)
		}
	}

	return &iffyRVs
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	cm.IsValidRVName("rv0")
	_ = errors.New("test error")
}
