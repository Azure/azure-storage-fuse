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

const (
	defaultGRPCPerNodeConns = 16
)

// grpcClientPool manages per-node grpcClientPool.
type grpcClientPool struct {
	rwMutex sync.RWMutex

	// Companion counter to rwMutex for performing various locking related assertions.
	// [DEBUG ONLY]
	rwMutexDbgCntr atomic.Int64

	// Map of nodeID to *grpcNodeClientPool. Use the following helpers to manage the map:
	clients map[string]*grpcNodeClientPool

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

	// Ticker / stop channel for periodic purge of expired negative/iffy entries.
	negativeNodesTicker *time.Ticker
	negativeNodesDone   chan bool

	maxPerNode uint32 // Maximum number of open RPC clients per node
	maxNodes   uint32 // Maximum number of nodes for which RPC clients are open
}

func newGRPCClientPool(maxPerNode, maxNodes uint32) *grpcClientPool {
	log.Info("grpcClientPool::newGRPCClientPool: Creating RPC client pool with maxPerNode: %d, maxNodes: %d",
		maxPerNode, maxNodes)

	cp := &grpcClientPool{
		clients:             make(map[string]*grpcNodeClientPool),
		maxPerNode:          maxPerNode,
		maxNodes:            maxNodes,
		negativeNodesTicker: time.NewTicker(5 * time.Second),
		negativeNodesDone:   make(chan bool),
	}

	go cp.periodicRemoveNegativeNodesAndIffyRVs()
	return cp
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

	// Fast negative-node fail-fast path.
	if err := cp.checkNegativeNode(nodeID); err != nil {
		log.Err("grpcClientPool::getRPCClient: %v", err)
		return nil, err
	}

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

// AddNegativeNode adds a node to the negative nodes map when,
//   - The RPC client creation to the node failed due to timeout.
//   - RPC call to the node failed due to timeout.
//
// To keep the caller simple, we don't take any lock here and do not expect caller to take any lock before calling
// this method.
// It increases the negativeNodesCnt counter only when a new entry is added to the map.
func (cp *grpcClientPool) addNegativeNode(nodeID string) bool {
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

			log.Debug("grpcClientPool::addNegativeNode: added (%s -> %s) to negativeNodes (total count: %d)",
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
			log.Debug("grpcClientPool::addNegativeNode: updated (%s -> [%s -> %s]) in negativeNodes (total count: %d)",
				nodeID, oldTime, now, cp.negativeNodesCnt.Load())
			return false
		}

		// This is rare, so if it happens let's know about it.
		log.Warn("grpcClientPool::addNegative CompareAndSwap(%d, %s, %s) failed, retrying",
			nodeID, oldTime, now)
	}
}

// RemoveNegativeNode removes a node from the negative nodes map when,
//   - periodicRemoveNegativeNodesAndIffyRVs() goroutine which checks if the defaultNegativeTimeout
//     has expired for the node.
//   - successful RPC call to the node indicating that the connection between the client and the
//     node is healthy.
func (cp *grpcClientPool) removeNegativeNode(nodeID string) bool {
	common.Assert(common.IsValidUUID(nodeID), nodeID)

	// Fast path, keep it quick.
	if cp.negativeNodesCnt.Load() == 0 {
		return false
	}

	if val, ok := cp.negativeNodes.LoadAndDelete(nodeID); ok {
		_ = val
		common.Assert(cp.negativeNodesCnt.Load() > 0, cp.negativeNodesCnt.Load(), nodeID)
		cp.negativeNodesCnt.Add(-1)

		log.Debug("grpcClientPool::removeNegativeNode: removed (%s -> %s) from negativeNodes (total count: %d)",
			nodeID, val.(time.Time), cp.negativeNodesCnt.Load())
		return true
	}

	return false
}

// Check if the given node is marked negative.
// This method returns an appropriately wrapped error which can be used by the callers to check
// for negative node error.
//
// To allow multiple threads to check for negative node concurrently, we don't take any lock here
// and do not expect caller to take any lock before calling this method.
//
// Note: A node may be marked negative by another thread anytime after this method is called, usually negative node
//       is a soft/best-effort check and not finding a node negative while it's indeed negative should result in
//       a connection or timeout error while trying to connect to the node.

func (cp *grpcClientPool) checkNegativeNode(nodeID string) error {
	// Fast path, keep it quick.
	if cp.negativeNodesCnt.Load() > 0 {
		if val, ok := cp.negativeNodes.Load(nodeID); ok {
			err := fmt.Errorf("%w: %s (%s ago)", NegativeNodeError, nodeID, time.Since(val.(time.Time)))
			log.Err("grpcClientPool::checkNegativeNode: %v", err)

			// Caller should be able to identify this as a negative node error.
			common.Assert(errors.Is(err, NegativeNodeError), err, nodeID)
			return err
		}
	}

	return nil
}

// Add RV id to the iffyRvIdMap.
// When PutChunkDC() fails with timeout error, we add the next-hop RV and all the RVs in the chain
// to the iffyRvIdMap.
func (cp *grpcClientPool) addIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	now := time.Now()
	val, alreadyPresent := cp.iffyRvIdMap.LoadOrStore(rvID, now)
	_ = val

	if !alreadyPresent {
		// New entry added.
		cp.iffyRvIdMapCnt.Add(1)

		log.Debug("grpcClientPool::addIffyRvById: added (%s -> %s) to iffyRvIdMap (total count: %d)",
			rvID, now, cp.iffyRvIdMapCnt.Load())
		return true
	}

	// Existing entry, update the timestamp without increasing iffyRvIdMapCnt.
	cp.iffyRvIdMap.Store(rvID, now)

	log.Debug("grpcClientPool::addIffyRvById: updated (%s -> [%s -> %s]) in iffyRvIdMap (total count: %d)",
		rvID, val.(time.Time), now, cp.iffyRvIdMapCnt.Load())

	return false
}

// Add the RV name to the iffyRvIdMap.
// This method internally calls the addIffyRvById method.
func (cp *grpcClientPool) addIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)

	if cp.addIffyRvId(cm.RvNameToId(rvName)) {
		log.Debug("grpcClientPool::addIffyRvByName: added %s to iffyRvIdMap", rvName)
		return true
	} else {
		log.Debug("grpcClientPool::addIffyRvByName: updated %s in iffyRvIdMap", rvName)
		return false
	}
}

// RemoveIffyRV removes an RV from the iffyRvIdMap.
// An RV is removed from the map by,
//   - periodicRemoveNegativeNodesAndIffyRVs() goroutine which checks if the defaultNegativeTimeout
//     has expired for the RV.
//   - successful RPC call to the RV indicating that the connection between the client and the
//     RV is healthy.
func (cp *grpcClientPool) removeIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	// Fast path, keep it quick.
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	if val, ok := cp.iffyRvIdMap.LoadAndDelete(rvID); ok {
		_ = val
		common.Assert(cp.iffyRvIdMapCnt.Load() > 0, cp.iffyRvIdMapCnt.Load(), rvID, val.(time.Time))
		cp.iffyRvIdMapCnt.Add(-1)

		log.Debug("grpcClientPool::removeIffyRvById: removed (%s -> %s) from iffyRvIdMap (total count: %d)",
			rvID, val.(time.Time), cp.iffyRvIdMapCnt.Load())
		return true
	}

	return false
}

// Remove the RV name from the iffyRvIdMap.
// This method internally calls the removeIffyRvById method.
func (cp *grpcClientPool) removeIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)

	// Fast path, keep it quick.
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	if cp.removeIffyRvId(cm.RvNameToId(rvName)) {
		log.Debug("grpcClientPool::removeIffyRvByName: removed %s from iffyRvIdMap", rvName)
		return true
	}

	return false
}

// Check if an RV id is marked iffy.
func (cp *grpcClientPool) isIffyRvId(rvID string) bool {
	common.Assert(common.IsValidUUID(rvID), rvID)

	// Fast path, avoid lock
	if cp.iffyRvIdMapCnt.Load() == 0 {
		return false
	}

	_, ok := cp.iffyRvIdMap.Load(rvID)
	return ok
}

// Check if an RV name is marked iffy.
func (cp *grpcClientPool) isIffyRvName(rvName string) bool {
	common.Assert(cm.IsValidRVName(rvName), rvName)
	return cp.isIffyRvId(cm.RvNameToId(rvName))
}

// Goroutine which runs every 5 seconds and removes expired nodes and RVs from the
// negativeNodes and iffyRvIdMap.
func (cp *grpcClientPool) periodicRemoveNegativeNodesAndIffyRVs() {
	log.Info("grpcClientPool::periodicRemoveNegativeNodesAndIffyRVs: Starting")

	for {
		select {
		case <-cp.negativeNodesDone:
			log.Info("grpcClientPool::periodicRemoveNegativeNodesAndIffyRVs: Stopping")
			return
		case <-cp.negativeNodesTicker.C:
			// remove entries from negativeNodes map based on timeout
			cp.negativeNodes.Range(func(key, value any) bool {
				nodeID := key.(string)
				common.Assert(common.IsValidUUID(nodeID), nodeID)

				addedTime := value.(time.Time)

				if time.Since(addedTime) > defaultNegativeTimeout*time.Second {
					log.Debug("grpcClientPool::periodicRemoveNegativeNodesAndIffyRVs: removing negative node %s (%s)",
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

// ----------------------------------------------------------------------------------------------------------------

// Called from fixMV() in cluster_manager to initialize the "excluded nodes" map from the known negative nodes.
func getNegativeNodesGRPC() map[int]struct{} {
	negativeNodes := make(map[int]struct{})

	// Fast path, keep it quick.
	if gp.negativeNodesCnt.Load() > 0 {
		gp.negativeNodes.Range(func(key, value any) bool {
			nodeID := key.(string)
			common.Assert(common.IsValidUUID(nodeID), nodeID)

			log.Debug("grpcClientPool::GetNegativeNodes: Negative node: %s (%s ago) excluded from fix MV",
				nodeID, time.Since(value.(time.Time)))
			negativeNodes[cm.UUIDToUniqueInt(nodeID)] = struct{}{}
			return true
		})
	}

	return negativeNodes
}

// Given the component RVs list, return the RVs which are marked iffy.
func getIffyRVsGRPC(nextHopRV *string, nextRVs *[]string) *[]string {
	common.Assert(nextHopRV != nil)
	common.Assert(nextRVs != nil)

	// Common case, keep it quick.
	if gp.iffyRvIdMapCnt.Load() == 0 {
		return nil
	}

	iffyRVs := make([]string, 0, len(*nextRVs)+1)

	// Check the next-hop RV.
	if gp.isIffyRvName(*nextHopRV) {
		iffyRVs = append(iffyRVs, *nextHopRV)
	}

	// And all other RVs in the chain.
	for _, rv := range *nextRVs {
		common.Assert(rv != *nextHopRV, rv, *nextHopRV)
		if gp.isIffyRvName(rv) {
			iffyRVs = append(iffyRVs, rv)
		}
	}

	return &iffyRVs
}

// Silence unused import errors for release builds.
func init() {
	common.IsValidUUID("00000000-0000-0000-0000-000000000000")
	_ = errors.New("test error")
}
