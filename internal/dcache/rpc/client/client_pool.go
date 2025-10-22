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
	// Static slice of nodeClientPool, one per node (maximum staticMaxNodes).
	// We allocate this statically to avoid resizing later, for simplifying access.
	// An active nodeClientPool is indicated by ncPool.isActive being true.
	//
	clients []*nodeClientPool

	// clientsCnt is the number of active node client pools in the clients slice.
	// Only active node client pools (which are currently being used) are counted.
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
	// TODO: Change the key to nodeIDInt for faster access.
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

	maxPerNode     uint32 // Maximum number of open RPC clients per node
	maxNodes       uint32 // Maximum number of nodes for which RPC clients are open
	staticMaxNodes uint32 // Static maximum number of nodes
	timeout        uint32 // Duration in seconds after which a RPC client is closed
}

// newClientPool creates a new client pool with the specified parameters
// maxPerNode: Maximum number of RPC clients opened per node
// maxNodes: Maximum number of nodes for which RPC clients are allowed at any time
// staticMaxNodes: Static maximum number of nodes, used to allocate the clients slice
// timeout: Duration in seconds after which a RPC client is closed
//
// TODO: Implement timeout support.
func newClientPool(maxPerNode, maxNodes, staticMaxNodes, timeout uint32) *clientPool {
	log.Info("clientPool::newClientPool: Creating RPC client pool with maxPerNode: %d, maxNodes: %d, staticMaxNodes: %d, timeout: %d",
		maxPerNode, maxNodes, staticMaxNodes, timeout)

	common.Assert(staticMaxNodes >= maxNodes, staticMaxNodes, maxNodes)

	cp := &clientPool{
		maxPerNode:          maxPerNode,
		maxNodes:            maxNodes,
		staticMaxNodes:      staticMaxNodes,
		timeout:             timeout,
		negativeNodesTicker: time.NewTicker(5 * time.Second),
		negativeNodesDone:   make(chan bool),
	}

	//
	// Create static slice of nodeClientPool, one per node for staticMaxNodes.
	// We allocate this statically to avoid resizing later, for simplifying access, but only those
	// entries which are active (ncPool.isActive is true) can be used.
	//
	cp.clients = make([]*nodeClientPool, staticMaxNodes)

	for i := 0; i < int(staticMaxNodes); i++ {
		cp.clients[i] = &nodeClientPool{nodeIDInt: i}
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

// Acquire shared/read lock on the nodeClientPool for the given node.
// getRPCClient() and releaseRPCClient() acquire read lock on the nodeClientPool, while callers that
// mutate the nodeClientPool (like closeLRUNodeClientPool(), deactivateNodeClientPool(), etc.) take
// write lock.
func (cp *clientPool) acquireNodeReadLock(nodeIDInt int) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	// clientPool.RWMutex must be read locked before acquiring node lock.
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)
	ncPool.nodeLock.RLock()

	if common.IsDebugBuild() {
		ncPool.nodeLockDbgCntr.Add(1)
		common.Assert(ncPool.nodeLockDbgCntr.Load() > 0, ncPool.nodeLockDbgCntr.Load(), ncPool.nodeID, nodeIDInt)
	}
}

// Release the read lock on the node client pool.
func (cp *clientPool) releaseNodeReadLock(nodeIDInt int) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	common.Assert(cp.isNodeReadLocked(nodeIDInt), nodeIDInt)
	// clientPool.RWMutex must be read locked while we have the node lock.
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)

	if common.IsDebugBuild() {
		common.Assert(ncPool.nodeLockDbgCntr.Load() > 0, ncPool.nodeLockDbgCntr.Load(), ncPool.nodeID, nodeIDInt)
		ncPool.nodeLockDbgCntr.Add(-1)
	}

	ncPool.nodeLock.RUnlock()
}

func (cp *clientPool) acquireNodeWriteLock(nodeIDInt int) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	// clientPool.RWMutex must be read locked before acquiring node lock.
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)
	ncPool.nodeLock.Lock()

	if common.IsDebugBuild() {
		common.Assert(ncPool.nodeLockDbgCntr.Load() == 0, ncPool.nodeLockDbgCntr.Load(), ncPool.nodeID, nodeIDInt)
		ncPool.nodeLockDbgCntr.Store(-12345) // Special value to signify write lock.
	}
}

// Release the write lock on the rwMutex.
func (cp *clientPool) releaseNodeWriteLock(nodeIDInt int) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	common.Assert(cp.isNodeWriteLocked(nodeIDInt), nodeIDInt)
	// clientPool.RWMutex must be read locked while we have the node lock.
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)

	if common.IsDebugBuild() {
		common.Assert(ncPool.nodeLockDbgCntr.Load() == -12345, ncPool.nodeLockDbgCntr.Load(), ncPool.nodeID, nodeIDInt)
		ncPool.nodeLockDbgCntr.Store(0)
	}

	ncPool.nodeLock.Unlock()
}

// Check if read/shared lock is held for the given node.
// [DEBUG ONLY]
func (cp *clientPool) isNodeReadLocked(nodeIDInt int) bool {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)
	return ncPool.nodeLockDbgCntr.Load() > 0
}

// Check if write/exclusive lock is held for the given node.
// [DEBUG ONLY]
func (cp *clientPool) isNodeWriteLocked(nodeIDInt int) bool {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)
	return ncPool.nodeLockDbgCntr.Load() == -12345
}

// Check if any lock (read or write) is held for the given node.
// [DEBUG ONLY]
func (cp *clientPool) isNodeLocked(nodeIDInt int) bool {
	return cp.isNodeReadLocked(nodeIDInt) || cp.isNodeWriteLocked(nodeIDInt)
}

// Caller must check the returned nodeClientPool to see if it's active.
// Only active nodeClientPool can be used to get/release RPC clients.
func (cp *clientPool) getNodeClientPoolForNodeIdInt(nodeIDInt int, nodeID string) *nodeClientPool {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	// MUST be called with the node lock held (read or write) for the given node.
	common.Assert(cp.isNodeLocked(nodeIDInt), nodeIDInt, nodeID)

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, nodeID)

	if ncPool.isActive.Load() {
		// If active, nodeID must be set.
		common.Assert(ncPool.nodeID == nodeID, nodeIDInt, ncPool.nodeID)
		// If active, clientChan must be allocated.
		common.Assert(ncPool.clientChan != nil, nodeIDInt, ncPool.nodeID)
		// clients and clientsCnt must agree.
		common.Assert(cp.clientsCnt.Load() > 0, cp.clientsCnt.Load(), nodeID)
	} else {
		// If not active, nodeID must be empty and clientChan must be nil.
		common.Assert(ncPool.nodeID == "", nodeIDInt, nodeID)
		common.Assert(ncPool.clientChan == nil, nodeIDInt, nodeID)
	}

	return ncPool
}

// Activate the nodeClientPool for the given node.
func (cp *clientPool) activateNodeClientPool(nodeIDInt int, nodeID string) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	// MUST be called with exclusive node lock held for the given nodeID.
	common.Assert(cp.isNodeWriteLocked(nodeIDInt), nodeIDInt)

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)

	// Must not already be active.
	common.Assert(!ncPool.isActive.Load(), nodeIDInt, ncPool.nodeID)

	ncPool.isActive.Store(true)
	cp.clientsCnt.Add(1)

	// An active nodeClientPool must have nodeID set correctly.
	ncPool.nodeID = nodeID

	common.Assert(cp.clientsCnt.Load() <= int64(cp.maxNodes), cp.clientsCnt.Load(), cp.maxNodes)
}

// Deactivate nodeClientPool for the given node.
func (cp *clientPool) deactivateNodeClientPool(nodeIDInt int, nodeID string) {
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeIDInt, cp.staticMaxNodes)
	// MUST be called with exclusive node lock held for the given nodeID, or with the rwMutex write lock held.
	// Latter is true when called from closeAllNodeClientPools().
	common.Assert(cp.isNodeWriteLocked(nodeIDInt) || cp.isRWMutexWriteLocked(), nodeID, nodeIDInt)

	ncPool := cp.clients[nodeIDInt]
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID)

	// Must be active.
	common.Assert(ncPool.isActive.Load(), nodeIDInt, ncPool.nodeID)
	common.Assert(ncPool.nodeID == nodeID, ncPool.nodeID, nodeID, nodeIDInt)

	// We must never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
		ncPool.numActive.Load(), len(ncPool.clientChan))

	ncPool.nodeID = ""
	ncPool.clientChan = nil
	ncPool.isActive.Store(false)
	ncPool.deleting.Store(false)

	common.Assert(cp.clientsCnt.Load() > 0, cp.clientsCnt.Load(), nodeID, nodeIDInt)
	cp.clientsCnt.Add(-1)
}

// Initialize nodeClientPool for the given node.
// Once nodeClientPool is initialized, clients can be allocated from it.
// Any other thread wanting to get a RPC client for the node will wait for this function to return.
//
// NOTE: Caller MUST hold exclusive lock for the node and read lock on the rwMutex
//       before calling this function.

func (cp *clientPool) newNodeClientPool(nodeID string, nodeIDInt int) (*nodeClientPool, error) {
	common.Assert(cp.isNodeWriteLocked(nodeIDInt), nodeIDInt, nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.clients[nodeIDInt]

	// Not-yet-active nodeClientPool.
	common.Assert(ncPool.nodeIDInt == nodeIDInt, ncPool.nodeIDInt, nodeIDInt, ncPool.nodeID, nodeID)
	common.Assert(ncPool.nodeID == "", ncPool.nodeID, nodeID, nodeIDInt)
	common.Assert(!ncPool.isActive.Load(), nodeIDInt, ncPool.nodeID, nodeID)
	common.Assert(ncPool.clientChan == nil, nodeIDInt, ncPool.nodeID, nodeID)

	// Check in the negative nodes map if we should attempt creating RPC clients for this node ID.
	err := cp.checkNegativeNode(nodeID)
	if err != nil {
		log.Err("clientPool::newNodeClientPool: not creating RPC clients for negative node %s (%d): %v",
			nodeID, nodeIDInt, err)
		// Caller should be able to identify this as a negative node error.
		common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
		return nil, err
	}

	if cp.clientsCnt.Load() >= int64(cp.maxNodes) {
		// TODO: remove this and rely on the closeInactiveRPCClients to close inactive clients
		// newNodeClientPool should be small and fast,
		// refer https://github.com/Azure/azure-storage-fuse/pull/1684#discussion_r2047993390
		log.Debug("clientPool::newNodeClientPool: Maximum number of nodes reached, evicting LRU node client pool")
		err := cp.closeLRUNodeClientPool()
		if err != nil {
			log.Err("clientPool::newNodeClientPool: Failed to close LRU node client pool: %v",
				err)
			return nil, err
		}
	}

	//
	// Note that createRPCClients() can fail to create any client if the remote blobfuse process
	// is not running or the node is down.
	//
	err = ncPool.createRPCClients(nodeID, cp.maxPerNode)
	if err != nil {
		log.Err("clientPool::newNodeClientPool: createRPCClients(%s (%d)) failed: %v", nodeID, nodeIDInt, err)
		return nil, err
	}

	// Successfully created all required RPC clients for the node, add it to the clients map.
	cp.activateNodeClientPool(nodeIDInt, nodeID)

	// Must always create cp.maxPerNode clients to any node.
	common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode), len(ncPool.clientChan), cp.maxPerNode)

	// Brand new nodeClientPool must not be marked deleting.
	common.Assert(!ncPool.deleting.Load(), nodeID)

	// Must be active for sure.
	common.Assert(ncPool.isActive.Load(), nodeID)

	return ncPool, nil
}

// getRPCClient retrieves an RPC client that can be used for calling RPC functions to the given target node.
// If the client pool for nodeID is not available (not created yet or was cleaned up due to pressure),
// a new pool is created, replenished with cp.maxPerNode clients and a client returned from that.
//
// Note: This creates "extra client" if there aren't any free client in the pool, so if this fails it indicates
//       a serious issue and retrying usually won't help. Callers should treat it as such.
//       Caller can check for NegativeNodeError to see if the client couldn't be created because the node is
//       probably down.
//
// NOTE: Caller MUST NOT hold the clientPool or node level lock.

func (cp *clientPool) getRPCClient(nodeID string) (*rpcClient, error) {
	// Get integer ID for the node ID.
	nodeIDInt := cm.UUIDToUniqueInt(nodeID)
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeID, nodeIDInt, cp.staticMaxNodes)

	const slowRPCThreshold = 1 * time.Second
	var retryCnt int64
	var ncPool *nodeClientPool
	//
	// TODO: Leaving timing measurement for some time in case we need it on larger clusters.
	//       Remove later.
	//
	var t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12 time.Duration

	startTime := time.Now()

	defer func() {
		//
		// Let us know if RPC client allocation is slow.
		// We will decide if it's a problem or not based on how often we see this in the logs and what
		// operations are being performed.
		//
		// This is only seen to happen when we are creating a new nodeClientPool for the node and creating
		// all the static connections, under heavy load this can take time.
		//
		if time.Since(startTime) > slowRPCThreshold {
			//log.Warn("[SLOW] clientPool::getRPCClient: Slow getRPCClient(nodeID: %s (%d), retryCnt: %d) took %s",
			//	nodeID, nodeIDInt, retryCnt, time.Since(startTime))
			log.Warn("[SLOW] clientPool::getRPCClient: Slow getRPCClient(nodeID: %s (%d), retryCnt: %d) took %s [t1: %s, t2: %s, t3: %s, t4: %s, t5: %s, t6: %s, t7: %s, t8: %s, t9: %s, t10: %s, t11: %s, t12: %s]",
				nodeID, nodeIDInt, retryCnt, time.Since(startTime),
				t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12)
		}
	}()

	log.Debug("clientPool::getRPCClient: getRPCClient(nodeID: %s (%d))", nodeID, nodeIDInt)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	t1 = time.Since(startTime)
	//
	// Get the nodeClientPool for this node.
	// We get read lock for the given node. This ensures that the nodeClientPool is not mutated while we
	// are looking for a client from the pool, while avoiding contention with other threads wanting to
	// get/release clients for the same node.
	// We need to release the node lock before waiting on the clientChan.
	//
	// Q: Why is it safe to release the node lock and still use ncPool?
	// A: If ncPool has one or more active/free clients, it is guaranteed that deactivateNodeClientPool()
	//    won't delete the nodeClientPool, as it only deletes a nodeClientPool when there are no active
	//    clients and no clients in the channel.
	//    If ncPool has no active clients and no clients in the channel, then it can be deleted by
	//    deactivateNodeClientPool() after we release the node lock, but we can still safely access ncPool
	//    and it'll have no active and free clients and hence the getRPCClient() call will fail.
	//
	for {
		cp.acquireNodeReadLock(nodeIDInt)
		t2 = time.Since(startTime)
		ncPool = cp.getNodeClientPoolForNodeIdInt(nodeIDInt, nodeID)
		t3 = time.Since(startTime)
		cp.releaseNodeReadLock(nodeIDInt)

		// Valid nodeClientPool present (common case).
		if ncPool.isActive.Load() {
			break
		}

		cp.acquireNodeWriteLock(nodeIDInt)
		t4 = time.Since(startTime)
		// Check once more after taking the write lock.
		ncPool = cp.getNodeClientPoolForNodeIdInt(nodeIDInt, nodeID)
		t5 = time.Since(startTime)
		if ncPool.isActive.Load() {
			// Some other thread created the nodeClientPool while we were waiting for the write lock.
			cp.releaseNodeWriteLock(nodeIDInt)
			continue
		}

		var err error
		ncPool, err = cp.newNodeClientPool(nodeID, nodeIDInt)
		t6 = time.Since(startTime)
		cp.releaseNodeWriteLock(nodeIDInt)

		if err != nil {
			return nil, fmt.Errorf("clientPool::getRPCClient: newNodeClientPool(%s) failed: %v",
				nodeID, err)
		}
	}

	log.Debug("clientPool::getRPCClient: Retrieving RPC client for node %s (%d) [free: %d, active: %d]",
		nodeID, nodeIDInt, len(ncPool.clientChan), ncPool.numActive.Load())

	for {
		//
		// For doing isActive, isDeleting checks, we need to take read lock on the node as these are updated
		// while holding the node write lock.
		//
		cp.acquireNodeReadLock(nodeIDInt)

		if !ncPool.isActive.Load() {
			// Publish as NegativeNodeError as we cannot create a client because the node is probably down.
			err := fmt.Errorf("client pool not active for node %s (%d), no clients available, waited for %s: %w",
				nodeID, nodeIDInt, time.Since(startTime), NegativeNodeError)
			log.Err("clientPool::getRPCClient: %v", err)
			cp.releaseNodeReadLock(nodeIDInt)
			return nil, err
		}

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
			err := fmt.Errorf("client pool deleted for node %s (%d) (%s ago), no clients available, waited for %s: %w",
				nodeID, nodeIDInt, time.Since(ncPool.deletingAt), time.Since(startTime), NegativeNodeError)
			log.Err("clientPool::getRPCClient: %v", err)
			//
			// Once we mark a nodeClientPool as deleting, we don't allocate any new clients from it and we have
			// a timeout of 20 secs set for each client, so we should never have nodeClientPool still hanging
			// around after 30 secs.
			//
			common.Assert(time.Since(ncPool.deletingAt) < 30*time.Second,
				nodeID, ncPool.deletingAt, time.Since(ncPool.deletingAt))
			cp.releaseNodeReadLock(nodeIDInt)
			return nil, err
		}

		//
		// If node is marked negative, no point in waiting for a client to become available.
		// See above for explanation on negative nodes.
		//
		t7 = time.Since(startTime)
		if err := cp.checkNegativeNode(nodeID); err != nil {
			err = fmt.Errorf("failing getRPCClient for negative node %s (%d): %w", nodeID, nodeIDInt, err)
			log.Err("clientPool::getRPCClient: %v", err)
			// Caller should be able to identify this as a negative node error.
			common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
			cp.releaseNodeReadLock(nodeIDInt)
			return nil, err
		}
		t8 = time.Since(startTime)

		cp.releaseNodeReadLock(nodeIDInt)

		//
		// If the client pool is empty, we create a new "extra" client.
		// Note that we may create an extra client even if the nodeClientPool is deactivated.
		// This is not desirable but not a big deal as extra clients are not tracked in the pool.
		//
		// Note: We have a tiny race here, the following len(ncPool.clientChan)==0 check may fail but by the
		//       time we dequeue from the channel, some other thread may have dequeued the last client.
		//       In that case the select will fallthrough to the default case and we will quickly loop again.
		//
		if len(ncPool.clientChan) == 0 {
			var client *rpcClient

			client, err := newRPCClient(ncPool.nodeID, ncPool.nodeIDInt, rpc.GetNodeAddressFromID(ncPool.nodeID))
			t9 = time.Since(startTime)
			if err == nil {
				ncPool.numExtraClients.Add(1)
				ncPool.numExtraClientsCum.Add(1)

				log.Debug("clientPool::getRPCClient: Created extra RPC client for node %s (%d) (cur: %d, cum: %d, retryCnt: %d)",
					ncPool.nodeID, ncPool.nodeIDInt, ncPool.numExtraClients.Load(),
					ncPool.numExtraClientsCum.Load(), retryCnt)

				//
				// We should not need too many extra clients, so let us log to know if we are creating.
				// Tag it as [SLOW] for easy searching along with other slow logs.
				//
				if ncPool.numExtraClients.Load() > 64 {
					log.Warn("[SLOW] clientPool::getRPCClient: Created extra RPC client for node %s (%d) (cur: %d, cum: %d, retryCnt: %d)",
						ncPool.nodeID, ncPool.nodeIDInt, ncPool.numExtraClients.Load(),
						ncPool.numExtraClientsCum.Load(), retryCnt)
				}

				client.isExtra = true
				client.ncPool = ncPool
				return client, nil
			}

			//
			// Client creation will mostly fail when server is down or node is unreachable, just fail the
			// request and let the caller handle it as it sees fit.
			//
			common.Assert(rpc.IsConnectionRefused(err) ||
				rpc.IsConnectionReset(err) ||
				rpc.IsTimedOut(err) ||
				rpc.IsNoRouteToHost(err) ||
				errors.Is(err, NegativeNodeError), err)

			err = fmt.Errorf("clientPool::getRPCClient: Failed to create extra RPC client for node %s (%d) [%w]",
				ncPool.nodeID, ncPool.nodeIDInt, err)
			log.Err("%v", err)

			return nil, err
		}

		select {
		case client := <-ncPool.clientChan:
			t10 = time.Since(startTime)
			//
			// Only active nodeClientPool can have clients in the channel and once some thread acquires
			// a client from the channel, ncPool cannot be deactivated till the client is returned to the pool.
			//
			common.Assert(ncPool.isActive.Load(), ncPool.nodeID, ncPool.nodeIDInt)
			common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
			common.Assert(client.nodeIDInt == nodeIDInt, client.nodeIDInt, nodeIDInt, nodeID, client.nodeID)
			// Extra clients are not queued in the pool.
			common.Assert(!client.isExtra, nodeID)
			common.Assert(client.ncPool == ncPool, client, client.ncPool, ncPool, nodeID)

			ncPool.lastUsed.Store(time.Now().Unix())
			ncPool.numActive.Add(1)

			log.Debug("clientPool::getRPCClient: Successfully retrieved RPC client (%p) for node %s (%d) [free: %d, active: %d], took %s",
				client, nodeID, nodeIDInt, len(ncPool.clientChan), ncPool.numActive.Load(),
				time.Since(startTime))

			// numActive must never exceed maxPerNode.
			common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
				ncPool.numActive.Load(), cp.maxPerNode, client.nodeID)

			client.allocatedAt = time.Now()
			t11 = time.Since(startTime)
			return client, nil
		default:
			t12 = time.Since(startTime)
			log.Warn("clientPool::getRPCClient: No free RPC client for node %s (%d) (free: %d, active: %d, waiting: %s, retryCnt: %d)",
				nodeID, nodeIDInt, len(ncPool.clientChan), ncPool.numActive.Load(), time.Since(startTime), retryCnt)
			retryCnt++
			// Continue the for loop, various exit checks will be done there.
		}
	}
}

// Gets an RPC client that can be used for calling RPC functions to the given target node.
// Like getRPCClient() but in case there's no client currently available in the pool, it doesn't wait but
// instead returns error rightaway.
//
// NOTE: Caller MUST hold the lock for the nodeID and read lock on the rwMutex
//       before calling this function.

func (cp *clientPool) getRPCClientNoWait(nodeID string) (*rpcClient, error) {
	nodeIDInt := cm.UUIDToInt(nodeID)
	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeID, nodeIDInt, cp.staticMaxNodes)

	log.Debug("clientPool::getRPCClientNoWait: Retrieving RPC client for node %s (%d)", nodeID, nodeIDInt)

	common.Assert(cp.isNodeWriteLocked(nodeIDInt), nodeIDInt, nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolForNodeIdInt(nodeIDInt, nodeID)
	if !ncPool.isActive.Load() {
		return nil, fmt.Errorf("clientPool::getRPCClientNoWait: no active client pool for node %s (%d): %w",
			nodeID, nodeIDInt, NoFreeRPCClient)
	}

	select {
	case client := <-ncPool.clientChan:
		common.Assert(client.nodeIDInt == nodeIDInt, client.nodeIDInt, nodeIDInt, nodeID, client.nodeID)
		common.Assert(client.nodeID == nodeID, client.nodeID, nodeID)
		common.Assert(client.ncPool == ncPool, client, client.ncPool, ncPool, nodeID)
		// Extra clients are not queued in the pool.
		common.Assert(!client.isExtra, nodeID)

		ncPool.lastUsed.Store(time.Now().Unix())
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
	log.Debug("clientPool::releaseRPCClient: releaseRPCClient(client: %p, nodeID: %s (%d))",
		client, client.nodeID, client.nodeIDInt)

	// ncPool must be set in all allocated clients.
	common.Assert(client.ncPool != nil, client.nodeID)
	common.Assert(client.nodeIDInt == client.ncPool.nodeIDInt,
		client.nodeIDInt, client.ncPool.nodeIDInt, client.nodeID, client.ncPool.nodeID)

	//
	// Extra clients are not part of the pool, so we close them instead of releasing them back to the pool.
	//
	if client.isExtra {
		//
		// Since extra clients are not part of the pool, the nodeClientPool may have been deleted, but we can
		// still do the following assert as no one else could have decremented numExtraClients corresponding
		// to this client.
		//
		ncPool := client.ncPool
		common.Assert(ncPool.numExtraClients.Load() > 0, ncPool.numExtraClients.Load(), client.nodeID)
		ncPool.numExtraClients.Add(-1)

		log.Debug("clientPool::releaseRPCClient: closing extra client, client: %p, nodeID: %s (%d) (cur: %d, cum: %d)",
			client, client.nodeID, ncPool.nodeIDInt, ncPool.numExtraClients.Load(), ncPool.numExtraClientsCum.Load())

		err := cp.closeRPCClient(client)
		if err != nil {
			// Closing the socket should not fail, so we assert.
			common.Assert(false, err, client.nodeAddress, client.nodeID)
			return err
		}

		return nil
	}

	//
	// This assert may not be valid for extra clients, as pool can be deleted while we are using the extra client,
	// and that clears ncPool.nodeID.
	//
	common.Assert(client.nodeID == client.ncPool.nodeID, client.nodeID, client.ncPool.nodeID)

	//
	// Acquire read lock on the rwMutex. This ensures that operations like getRPCClient(),
	// releaseRPCClient(), deleteAllRPCClients(), etc. by other threads can process
	// concurrently. Whereas closeAllNodeClientPools() which takes write lock will be blocked till
	// all the read locks are released.
	//
	cp.acquireRWMutexReadLock()
	defer cp.releaseRWMutexReadLock()

	//
	// Get read lock for the given node.
	// We don't want the pool to be mutated till we successfully release the client back to the pool
	//
	cp.acquireNodeReadLock(client.nodeIDInt)
	defer func() {
		cp.releaseNodeReadLock(client.nodeIDInt)
	}()

	// We don't delete a nodeClientPool with active connections, so client.ncPool will be valid.
	ncPool := client.ncPool
	common.Assert(ncPool.isActive.Load(), client.nodeID, client.nodeIDInt)

	if common.IsDebugBuild() {
		ncPool1 := cp.getNodeClientPoolForNodeIdInt(client.nodeIDInt, client.nodeID)
		common.Assert(ncPool == ncPool1,
			client, client.nodeID, ncPool.nodeID, ncPool1.nodeID, ncPool.nodeIDInt, ncPool1.nodeIDInt)
	}

	//
	// It's possible that we got a connection error on some earlier client/connection to this node and hence
	// marked the nodeClientPool as deleting. This RPC response may have come before the target process restarted
	// and hence this got a success response, but this got processed after a connection with error. We should
	// continue with deleting the nodeClientPool.
	//
	if ncPool.deleting.Load() {
		log.Debug("clientPool::releaseRPCClient: Successful RPC response being processed after nodeClientPool is marked deleting, continuing with deleteAllRPCClients, client: %p, nodeID: %s (%d)", client, client.nodeID, client.nodeIDInt)

		cp.releaseNodeReadLock(client.nodeIDInt)
		cp.releaseRWMutexReadLock()

		cp.deleteAllRPCClients(client, false /* confirmedBadNode */, false /* isClientClosed */)

		cp.acquireRWMutexReadLock()
		cp.acquireNodeReadLock(client.nodeIDInt)
		return nil
	}

	log.Debug("clientPool::releaseRPCClient: %p after %s, node: %s, free: %d, active: %d, maxPerNode: %d",
		client, time.Since(client.allocatedAt), client.nodeID, len(ncPool.clientChan), ncPool.numActive.Load(),
		cp.maxPerNode)

	// We must release only to a non-full pool.
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		client.nodeID, len(ncPool.clientChan), cp.maxPerNode)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID)

	// Must be releasing an active client.
	common.Assert(ncPool.numActive.Load() > 0, client.nodeID)
	ncPool.numActive.Add(-1)

	ncPool.returnClientToPool(client)

	return nil
}

// closeRPCClient closes an RPC client.
// The client MUST have been removed from the pool using a prior getRPCClient() call.
func (cp *clientPool) closeRPCClient(client *rpcClient) error {
	log.Debug("clientPool::closeRPCClient: Closing RPC client (%p) to %s node %s (%d)",
		client, client.nodeAddress, client.nodeID, client.nodeIDInt)

	err := client.close()
	if err != nil {
		err = fmt.Errorf("failed to close RPC client to %s node %s (%d): %v",
			client.nodeAddress, client.nodeID, client.nodeIDInt, err)
		log.Err("nodeClientPool::closeRPCClient: %v", err)
		common.Assert(false, err)
		return err
	}

	log.Info("clientPool::closeRPCClient: Closed RPC client to %s node %s (%d)",
		client.nodeAddress, client.nodeID, client.nodeIDInt)

	return nil
}

// deleteRPCClient deletes an RPC client from the pool.
// It first closes the client and then removes it from the pool.
// This is used when the client is no longer needed, e.g. when the node is down and we want to
// remove all clients to the node.
//
// NOTE: Caller MUST hold the exclusive node lock for the nodeID and read lock on the rwMutex
//       before calling this function.

func (cp *clientPool) deleteRPCClient(client *rpcClient) {
	log.Debug("clientPool::deleteRPCClient: Deleting RPC client (%p) to %s node %s (%d)",
		client, client.nodeAddress, client.nodeID, client.nodeIDInt)

	common.Assert(cp.isNodeWriteLocked(client.nodeIDInt), client.nodeID, client.nodeIDInt)
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

	//
	// Extra clients are not part of the pool, so we don't need to do anything more.
	//
	if client.isExtra {
		return
	}

	ncPool := cp.getNodeClientPoolForNodeIdInt(client.nodeIDInt, client.nodeID)
	// client is allocated from the pool, so pool must exist.
	common.Assert(ncPool.isActive.Load(), client.nodeID, client.nodeIDInt)

	//
	// deleteRPCClient() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID)

	//
	// Must only delete an active client.
	// Also, clients which are deleted are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0, client.nodeID)
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
	log.Debug("clientPool::deleteAllRPCClients: Deleting all RPC clients for %s node %s, client: %p (extra: %v), confirmedBadNode: %v, isClientClosed: %v, adding to negative nodes map",
		client.nodeAddress, client.nodeID, client, client.isExtra, confirmedBadNode, isClientClosed)

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
	// Acquire exclusive lock for the node to prevent other threads from acquiring/releasing clients while we
	// are deleting all clients.
	//
	cp.acquireNodeWriteLock(client.nodeIDInt)
	defer cp.releaseNodeWriteLock(client.nodeIDInt)

	//
	// deleteAllRPCClients() is called only when an RPC call to the node fails with timeout error.
	// Add it to the negative nodes map to help other threads fail fast instead of waiting for timeout.
	//
	if confirmedBadNode {
		cp.addNegativeNode(client.nodeID)
	}

	//
	// Clients not allocated from the pool need to just close the client and return.
	//
	if client.isExtra {
		if !isClientClosed {
			cp.deleteRPCClient(client)
		}
		return
	}

	numConnDeleted := 0
	ncPool := cp.getNodeClientPoolForNodeIdInt(client.nodeIDInt, client.nodeID)

	//
	// Node client pool may not be present for the node in case of PutChunkDC timeout error,
	// where we first reset the client, which closes the client and then creates a new client.
	// If the new client creation fails, we come here to delete all clients. Meanwhile after
	// reset has released the node level lock, some other thread may have closed all the clients
	// for the target node and deleted the node client pool.
	// So, we assert here that the client passed in the argument is closed.
	//
	if !ncPool.isActive.Load() {
		log.Debug("clientPool::deleteAllRPCClients: No client pool found for node %s at %s, nothing to delete",
			client.nodeID, client.nodeAddress)
		common.Assert(isClientClosed, client.nodeID, client.nodeAddress)
		return
	}

	// client is allocated from the pool, so pool must exist.
	common.Assert(ncPool.nodeID == client.nodeID, ncPool.nodeID, client.nodeID)

	//
	// We need to take exclusive mu lock to prevent other threads from releasing a client to the pool
	// after we get numClients below, and before we delete all clients.
	// MAKE SURE NO FUNCTION CALLED FROM THIS POINT TILL THE DEFER RELEASES THE MU LOCK, TRIES TO
	// ACQUIRE MU LOCK.
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
		numConnDeleted++
	}

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
	cp.deleteNodeClientPoolIfInactive(client.nodeID, client.nodeIDInt)
}

// waitForNodeClientPoolToDelete waits till the node client pool for the given node is deleted, which
// means that all existing connections in the pool are closed and deleted. Any new request for a client after
// this would create a new node client pool with new connections. This is useful when we get a connection error
// from a node and we want to make sure that all existing connections to the node are closed, before attempting
// to send new requests to the node.
// A to-be-deleted nodeClientPool waiting for existing connections to drain has "deleting" set to true, so this
// waits till either of the following happens:
// - getNodeClientPoolForNodeIdInt() return nil, which means the nodeClientPool is deleted.
// - nodeClientPool.deleting is false, which means that the nodeClientPool is recreated after being deleted.

func (cp *clientPool) waitForNodeClientPoolToDelete(nodeID string, nodeIDInt int) error {
	log.Debug("clientPool::waitForNodeClientPoolToDelete: node %s", nodeID)

	common.Assert(nodeIDInt > 0 && nodeIDInt < int(cp.staticMaxNodes), nodeID, nodeIDInt, cp.staticMaxNodes)
	common.Assert(common.IsValidUUID(nodeID), nodeID, nodeIDInt)

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

	cp.acquireNodeWriteLock(nodeIDInt)
	defer cp.releaseNodeWriteLock(nodeIDInt)

	startTime := time.Now()

	for {
		ncPool := cp.getNodeClientPoolForNodeIdInt(nodeIDInt, nodeID)
		if !ncPool.isActive.Load() || !ncPool.deleting.Load() {
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
		cp.releaseNodeWriteLock(nodeIDInt)
		time.Sleep(100 * time.Millisecond)
		cp.acquireNodeWriteLock(nodeIDInt)
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

	//
	// Extra clients are not part of the pool, so we close them instead of resetting them.
	//
	if client.isExtra {
		return nil
	}

	log.Info("clientPool::resetRPCClientInternal: Creating new RPC client to %s node %s",
		client.nodeAddress, client.nodeID)

	if needLock {
		cp.acquireRWMutexReadLock()
		defer cp.releaseRWMutexReadLock()

		cp.acquireNodeWriteLock(client.nodeIDInt)
		defer cp.releaseNodeWriteLock(client.nodeIDInt)
	}

	// Assert that the client is locked for the node.
	common.Assert(cp.isNodeWriteLocked(client.nodeIDInt), client.nodeID)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolForNodeIdInt(client.nodeIDInt, client.nodeID)
	// We had allocated the client from the pool, so pool must exist.
	common.Assert(ncPool.isActive.Load(), client.nodeID)

	//
	// resetRPCClientInternal() MUST be called after removing client from the client pool, so
	// the pool must have space for a new client.
	//
	common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode),
		len(ncPool.clientChan), cp.maxPerNode)

	common.Assert(ncPool.numActive.Load() <= int64(cp.maxPerNode),
		ncPool.numActive.Load(), cp.maxPerNode, client.nodeID)

	//
	// Must only reset an active client.
	// Also, clients which are reset are not released, so we drop the numActive here, after
	// closing the connection successfully above.
	//
	common.Assert(ncPool.numActive.Load() > 0, client.nodeID)
	ncPool.numActive.Add(-1)

	newClient, err := newRPCClient(client.nodeID, client.nodeIDInt, rpc.GetNodeAddressFromID(client.nodeID))
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

		cp.deleteNodeClientPoolIfInactive(client.nodeID, client.nodeIDInt)
		return err
	}
	newClient.ncPool = ncPool

	//
	// Reset was successful, so we have at least one good connection to the target node.
	// Clear deleting if we had set it earlier.
	//
	ncPool.deleting.Store(false)

	// Add the new client to the client pool for this node.
	ncPool.returnClientToPool(newClient)

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
	lruNodeIDInt := -1

	//
	// Iterate through the clients map to find the least recently used node client pool
	// and close it if it has no active clients (i.e., it has maxPerNode clients in the pool).
	// This is done to ensure that we don't close a node client pool that has active clients
	// or has less than maxPerNode clients in the pool.
	// We use the lastUsed timestamp to determine the least recently used node client pool.
	//
searchLRUClientPool:
	for _, ncPool := range cp.clients {
		if !ncPool.isActive.Load() {
			continue
		}
		nodeID := ncPool.nodeID

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
			continue
		}

		if lruNcPool == nil || (ncPool.lastUsed.Load() < lruNcPool.lastUsed.Load()) {
			lruNcPool = ncPool
			lruNodeID = nodeID
			lruNodeIDInt = ncPool.nodeIDInt
		}
	}

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
	cp.acquireNodeWriteLock(lruNodeIDInt)

	if lruNcPool.numActive.Load() > 0 {
		log.Debug("clientPool::closeLRUNodeClientPool: Some thread raced with us and got an RPC client for %s, numActive: %d (%d, %d)",
			lruNcPool.nodeID, lruNcPool.numActive.Load(), len(lruNcPool.clientChan), cp.maxPerNode)
		lruNcPool = nil
		lruNodeID = ""

		cp.releaseNodeWriteLock(lruNodeIDInt)
		goto searchLRUClientPool
	}

	defer cp.releaseNodeWriteLock(lruNodeIDInt)

	err := lruNcPool.closeRPCClients()
	if err != nil {
		log.Err("clientPool::closeLRUNodeClientPool: Failed to close LRU node client pool for node %s [%v]",
			lruNodeID, err.Error())
		return err
	}

	// Never delete a nodeClientPool with active connections or non-empty connection pool.
	common.Assert(lruNcPool.numActive.Load() == 0 && len(lruNcPool.clientChan) == 0,
		lruNcPool.numActive.Load(), len(lruNcPool.clientChan))

	cp.deactivateNodeClientPool(lruNodeIDInt, lruNodeID)

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
	for _, ncPool := range cp.clients {
		if !ncPool.isActive.Load() {
			continue
		}

		nodeID := ncPool.nodeID
		//
		// Mark it deleting so that getRPCClient() does not allocate any more clients for this node.
		// Also wakeup any waiters in getRPCClient() so that they can fail fast.
		//
		if !ncPool.deleting.Swap(true) {
			ncPool.deletingAt = time.Now()
		} else {
			common.Assert(!ncPool.deletingAt.IsZero(), ncPool.nodeID)
		}

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
			break
		}

		err = ncPool.closeRPCClients()
		if err != nil {
			err = fmt.Errorf("Failed to close RPC clients for node %s [%v]", nodeID, err)
			log.Err("clientPool::closeAllNodeClientPools: %v", err)
			break
		}

		// Never delete a nodeClientPool with active connections or non-empty connection pool.
		common.Assert(ncPool.numActive.Load() == 0 && len(ncPool.clientChan) == 0,
			ncPool.numActive.Load(), len(ncPool.clientChan))

		cp.deactivateNodeClientPool(ncPool.nodeIDInt, ncPool.nodeID)
	}

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
func (cp *clientPool) deleteNodeClientPoolIfInactive(nodeID string, nodeIDInt int) bool {
	common.Assert(cp.isNodeWriteLocked(nodeIDInt), nodeID, nodeIDInt)
	common.Assert(cp.isRWMutexReadLocked())

	ncPool := cp.getNodeClientPoolForNodeIdInt(nodeIDInt, nodeID)

	// Caller must not call us for a non-existent pool.
	common.Assert(ncPool.isActive.Load(), nodeID, nodeIDInt)

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

	cp.deactivateNodeClientPool(nodeIDInt, nodeID)

	return true
}

// AddNegativeNode adds a node to the negative nodes map when,
//   - The RPC client creation to the node failed due to timeout.
//   - RPC call to the node failed due to timeout.
//
// To keep the caller simple, we don't take any lock here and do not expect caller to take any lock before calling
// this method.
// It increases the negativeNodesCnt counter only when a new entry is added to the map.

func (cp *clientPool) addNegativeNode(nodeID string) bool {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	for {
		now := time.Now()
		val, alreadyPresent := cp.negativeNodes.LoadOrStore(nodeID, now)
		_ = val

		if !alreadyPresent {
			//
			// New entry added.
			//
			// Note: Since we don't take any lock, it's possible that some thread may call removeNegativeNode()
			//       after the LoadOrStore() above and before we do the Add(1) below, which will cause
			//       removeNegativeNode() to not remove the node from negativeNodes map. This is not desireable
			//       but not catastrophic either, as the node will be removed from negativeNodes map in the
			//       next attempt.
			//
			cp.negativeNodesCnt.Add(1)

			log.Debug("clientPool::addNegativeNode: added (%s -> %s) to negativeNodes (total count: %d)",
				nodeID, now, cp.negativeNodesCnt.Load())

			return true
		}

		//
		// CompareAndSwap() can fail if either the key is deleted or updated by another thread after the
		// LoadOrStore() above. If it's updated and not deleted, CompareAndSwap() below will update it
		// with the new timestamp. If it's deleted, we go back and try again to add the key.
		//
		oldTime := val.(time.Time)
		if cp.negativeNodes.CompareAndSwap(nodeID, oldTime, now) {
			log.Debug("clientPool::addNegativeNode: updated (%s -> [%s -> %s]) in negativeNodes (total count: %d)",
				nodeID, oldTime, now, cp.negativeNodesCnt.Load())
			return false
		}

		// This is rare, so if it happens let's know about it.
		log.Warn("clientPool::addNegative CompareAndSwap(%d, %s, %s) failed, retrying",
			nodeID, oldTime, now)
	}
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
//
// To allow multiple threads to check for negative node concurrently, we don't take any lock here and do not expect
// caller to take any lock before calling this method.
//
// Note: A node may be marked negative by another thread anytime after this method is called, usually negative node
//       is a soft/best-effort check and not finding a node negative while it's indeed negative should result in
//       a connection or timeout error while trying to connect to the node.

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

// ----------------------------------------------------------------------------------------------------------------

// nodeClientPool holds a channel of RPC clients for a node
// and the last used timestamp for LRU eviction
type nodeClientPool struct {
	//
	// We use a static slice of node clients, this tells if a particular index has a valid client.
	//
	isActive atomic.Bool

	// Integral node id.
	nodeIDInt int

	//
	// lock for this node
	// getRPCClient() and releaseRPCClient() must hold a shared lock.
	// deleteAllRPCClients(), deactivateNodeClientPool() should hold an exclusive lock.
	//
	// Lock at the node level to ensure that only one thread can create/get/release/delete
	// RPC clients for a node at a time. This also ensures that other threads can
	// create/get/release/delete RPC clients for other nodes at the same time.
	// This MUST be acquired after acquiring read lock on the rwMutex, and the rwMutex read
	// lock MUST be held till the node lock is released.
	nodeLock sync.RWMutex

	// Companion counter to nodeLock for performing various locking related assertions.
	// [DEBUG ONLY]
	nodeLockDbgCntr atomic.Int64

	nodeID string // Node ID of the node this client pool is for

	//
	// Clients must be added to clientChan with mu locked, but can be deleted w/o the mu lock, as channel
	// operations are thread safe and moreover no go routine is interested in client dequeue event.
	// Note that mu lock is only for synchronizing with deleteAllRPCClients() so that clients cannot be added
	// to channel while deleteAllRPCClients() is deleting them. deleteAllRPCClients() must hold the write
	// lock on mu while returnClientToPool() must hold the read lock on mu.
	//
	// Note: mu lock can be safely held inside node lock, but not the other way round.
	//
	clientChan chan *rpcClient // channel to hold the RPC clients to a node
	mu         sync.RWMutex
	lastUsed   atomic.Int64 // used for evicting inactive RPC clients based on LRU (seconds since epoch)
	//
	// These atomic counters are used for debugging and assertions, so the order of updates is important,
	// hence they MUST be accessed with the nodeLock held.
	//
	numActive          atomic.Int64 // number of clients currently created using getRPCClient() call.
	numExtraClients    atomic.Int64 // number of extra clients created beyond the initial pool size.
	numExtraClientsCum atomic.Int64 // cumulative number of extra clients created beyond the initial pool size.
	//numReservedHighPrio int64        // number of high priority clients reserved for this node.
	deleting   atomic.Bool // true when the nodeClientPool is being deleted.
	deletingAt time.Time   // time when deleting was set to true, used for debugging.
}

// Safely return the client to clientChan.
func (ncPool *nodeClientPool) returnClientToPool(client *rpcClient) {
	common.Assert(client != nil, client)
	// nodeClientPool must not go away till we have a client allocated from it.
	common.Assert(ncPool.isActive.Load(), ncPool.nodeID, ncPool.nodeIDInt)

	ncPool.mu.RLock()
	if client != nil {
		common.Assert(len(ncPool.clientChan) < int(cp.maxPerNode), len(ncPool.clientChan), cp.maxPerNode)
		common.Assert(len(ncPool.clientChan) < cap(ncPool.clientChan), len(ncPool.clientChan), cap(ncPool.clientChan))

		// An extra client is not part of the pool, so we don't add it back to the pool.
		common.Assert(!client.isExtra, client, ncPool.nodeID)
		common.Assert(client.ncPool == ncPool, client, client.ncPool, ncPool, ncPool.nodeID)

		ncPool.clientChan <- client
	}
	ncPool.mu.RUnlock()
}

// Creates and populates clients for the given nodeClientPool.
// Must be called with the node write lock held.
func (ncPool *nodeClientPool) createRPCClients(nodeID string, numClients uint32) error {
	common.Assert(ncPool.nodeIDInt > 0, ncPool.nodeIDInt, nodeID)
	common.Assert(cp.isNodeWriteLocked(ncPool.nodeIDInt), nodeID, ncPool.nodeIDInt)

	log.Debug("nodeClientPool::createRPCClients: Creating %d RPC clients for node %s (%d)",
		numClients, nodeID, ncPool.nodeIDInt)

	// Must be called for an uninitialized nodeClientPool.
	common.Assert(ncPool.clientChan == nil)
	common.Assert(!ncPool.isActive.Load(), ncPool.nodeIDInt)
	common.Assert(!ncPool.deleting.Load(), ncPool.nodeIDInt)
	common.Assert(ncPool.nodeID == "", ncPool.nodeID, ncPool.nodeIDInt)
	common.Assert(ncPool.numActive.Load() == 0, ncPool.numActive.Load())

	const slowThreshold = 1 * time.Second
	startTime := time.Now()

	defer func() {
		if time.Since(startTime) > slowThreshold {
			log.Warn("[SLOW] nodeClientPool::createRPCClients: Slow (nodeID: %s (%d), numClients: %d) took %s",
				nodeID, ncPool.nodeIDInt, numClients, time.Since(startTime))
		}
	}()

	ncPool.clientChan = make(chan *rpcClient, numClients)
	ncPool.lastUsed.Store(time.Now().Unix())

	var wg sync.WaitGroup

	createOneClient := func() {
		defer wg.Done()

		client, err1 := newRPCClient(nodeID, ncPool.nodeIDInt, rpc.GetNodeAddressFromID(nodeID))
		if err1 != nil {
			log.Err("nodeClientPool::createRPCClients: Failed to create RPC client for node %s [%v]",
				nodeID, err1)
			//
			// Only valid reason could be connection refused as the blobfuse process is not running on
			// the remote node, a timeout if the node is down, no route to host error in some specific
			// unreachability conditions, or NegativeNodeError if the node is marked negative and newRPCClient()
			// proactively failed the request. There is no point in retrying in that case.
			//
			common.Assert(rpc.IsConnectionRefused(err1) ||
				rpc.IsConnectionReset(err1) ||
				rpc.IsTimedOut(err1) ||
				rpc.IsNoRouteToHost(err1) ||
				errors.Is(err1, NegativeNodeError), err1)
			//
			// Save the last error seen in err, to return if we could not create any client.
			// XXX: Set error atomically.
			//
			err = err1
			return
		}
		// Set nodeClientPool back pointer.
		client.ncPool = ncPool
		ncPool.clientChan <- client
	}

	// Create RPC clients and add them to the channel.
	for i := 0; i < int(numClients); i++ {
		wg.Add(1)
		go createOneClient()
	}

	wg.Wait()

	log.Debug("nodeClientPool::createRPCClients: Created %d RPC clients for node %s in %s",
		len(ncPool.clientChan), nodeID, time.Since(startTime))

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
		ncPool.clientChan = nil
		return fmt.Errorf("could not create any client for node %s: %v", nodeID, err)
	} else if len(ncPool.clientChan) != int(numClients) {
		log.Err("nodeClientPool::createRPCClients: Created %d of %d clients for node %s, cleaning up",
			len(ncPool.clientChan), numClients, nodeID)

		close(ncPool.clientChan)
		for client := range ncPool.clientChan {
			err1 := client.close()
			_ = err1
			// close() should not fail, even if it does there's nothing left to do.
			common.Assert(err1 == nil, err1)
		}
		// All error paths must ensure this.
		common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan))
		ncPool.clientChan = nil
		return fmt.Errorf("could not create all requested clients for node %s: %v", nodeID, err)
	}

	// We just got started, cannot have active clients.
	common.Assert(ncPool.numActive.Load() == 0, ncPool.numActive.Load())
	// We should have created exactly numClients clients.
	common.Assert(len(ncPool.clientChan) == int(numClients), len(ncPool.clientChan), numClients)
	return nil
}

// Close all RPC clients in the channel for the given node client pool.
// Must be called with the node exclusive lock held and no active clients.
func (ncPool *nodeClientPool) closeRPCClients() error {
	log.Debug("nodeClientPool::closeRPCClients: Closing %d RPC clients for node %s (%d)",
		len(ncPool.clientChan), ncPool.nodeID, ncPool.nodeIDInt)

	// MUST be called with exclusive node lock held for the given nodeID, or with the rwMutex write lock held.
	// Latter is true when called from closeAllNodeClientPools().
	common.Assert(cp.isNodeWriteLocked(ncPool.nodeIDInt) || cp.isRWMutexWriteLocked(), ncPool.nodeID, ncPool.nodeIDInt)

	// Must be called for an active nodeClientPool.
	common.Assert(ncPool.isActive.Load(), ncPool.nodeIDInt, ncPool.nodeID)
	common.Assert(common.IsValidUUID(ncPool.nodeID), ncPool.nodeID, ncPool.nodeIDInt)

	// We should not be closing all clients when there are any active clients.
	common.Assert(ncPool.numActive.Load() == 0,
		ncPool.numActive.Load(), len(ncPool.clientChan), cp.maxPerNode, ncPool.nodeID, ncPool.nodeIDInt)

	//
	// We never have a partially allocated client pool and we only clean up a client pool when all
	// previously allocated clients have been released back to the pool
	//
	// Note: This assert can fail if resetRPCClientInternal() fails to create the new connection after
	//       closing the old connection. This can happen when the target node is down or the blobfuse
	//       service is not running on the target node.
	//
	//common.Assert(len(ncPool.clientChan) == int(cp.maxPerNode),
	//	len(ncPool.clientChan), cp.maxPerNode, ncPool.nodeID)

	close(ncPool.clientChan)

	for client := range ncPool.clientChan {
		err := client.close()
		if err != nil {
			log.Err("nodeClientPool::closeRPCClients: Failed to close RPC client for node %s [%v]",
				ncPool.nodeID, err.Error())
			return err
		}
	}

	// All clients must have been closed.
	common.Assert(len(ncPool.clientChan) == 0, len(ncPool.clientChan))

	return nil
}

// ----------------------------------------------------------------------------------------------------------------

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
