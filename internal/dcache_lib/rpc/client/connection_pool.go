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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// connectionPool manages multiple connections efficiently
type connectionPool struct {
	mu          sync.Mutex
	connections map[string]*connectionPair // map of node ID to rpc client
	maxPerNode  uint32                     // Maximum number of open connections per node
	maxNodes    uint32                     // Maximum number of nodes for which connections are open
	timeout     uint32                     // Duration in seconds after which a connection is closed
}

// newConnectionPool creates a new connection pool with the specified parameters
// maxPerNode: Maximum number of open connections per node
// maxNodes: Maximum number of nodes for which connections are open
// timeout: Duration in seconds after which a connection is closed
func newConnectionPool(maxPerNode uint32, maxNodes uint32, timeout uint32) *connectionPool {
	log.Debug("ConnectionPool::newConnectionPool: Creating new connection pool with maxPerNode: %d, maxNodes: %d, timeout: %d", maxPerNode, maxNodes, timeout)
	return &connectionPool{
		connections: make(map[string]*connectionPair),
		maxPerNode:  maxPerNode,
		maxNodes:    maxNodes,
		timeout:     timeout,
	}

	// TODO: start a goroutine to periodically close inactive connections
}

// getRPCClient retrieves a rpc client from the pool for the specified node ID
// If no client is available, a new one is created
func (cp *connectionPool) getRPCClient(nodeID string) (*rpcClient, error) {
	log.Debug("connectionPool::getRPCClient: Retrieving rpc client for node %s", nodeID)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	var connPair *connectionPair
	connPair, exists := cp.connections[nodeID]
	if !exists {
		if len(cp.connections) >= int(cp.maxNodes) {
			log.Debug("connectionPool::getRPCClient: Maximum number of nodes reached, evict LRU rpc client")
			err := cp.closeLRUClient()
			if err != nil {
				log.Err("connectionPool::getRPCClient: Failed to close LRU rpc client [%v]", err.Error())
				return nil, err
			}
		}

		connPair = &connectionPair{}
		connPair.createConnections(nodeID, cp.maxPerNode)
		cp.connections[nodeID] = connPair
	}

	select {
	case conn := <-connPair.connChan:
		connPair.lastUsed = time.Now()
		return conn, nil
	default:
		log.Err("connectionPool::getRPCClient: No available rpc client in the pool for node %s", nodeID)
		return nil, fmt.Errorf("no available rpc client in the pool for node %s", nodeID)
	}
}

// releaseRPCClient releases a rpc client back to the pool for the specified node ID
func (cp *connectionPool) releaseRPCClient(nodeID string, conn *rpcClient) error {
	log.Debug("connectionPool::releaseConnection: Releasing connection for node %s", nodeID)

	connPair, exists := cp.connections[nodeID]
	if !exists {
		log.Err("connectionPool::releaseConnection: No connection pair found for node %s", nodeID)
		return fmt.Errorf("no connection pair found for node %s", nodeID)
	}

	connPair.connChan <- conn
	return nil
}

// Close the least recently used rpc client from the connections pool
func (cp *connectionPool) closeLRUClient() error {
	// Find the least recently used connection and close it
	var lruConnPair *connectionPair
	lruNodeID := ""
	for key, conn := range cp.connections {
		if lruConnPair == nil || conn.lastUsed.Before(lruConnPair.lastUsed) {
			lruConnPair = conn
			lruNodeID = key
		}
	}

	if lruConnPair != nil {
		err := lruConnPair.closeConnections()
		if err != nil {
			log.Err("connectionPool::closeLRUClient: Failed to close LRU client for node %s [%v]", lruNodeID, err.Error())
			return err
		}
		delete(cp.connections, lruNodeID)
	}

	return nil
}

// closeInactiveConnections closes connections that have not been used for a specified timeout
func (cp *connectionPool) closeInactiveConnections() {
	// Cleanup old connections based on the LastUsed timestamp
	// This will run in a separate goroutine and will periodically close the connections based on LRU strategy
}

// close closes all connections in the pool
func (cp *connectionPool) close() error {
	// see if this is needed
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for key, connPair := range cp.connections {
		err := connPair.closeConnections()
		if err != nil {
			log.Err("ConnectionPool::close: Failed to close connection for node %s [%v]", key, err.Error())
			return err
		}
		delete(cp.connections, key)
	}

	cp.connections = make(map[string]*connectionPair)
	return nil
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// connectionPair holds a channel of connections to a node
// and the last used timestamp for LRU eviction
type connectionPair struct {
	connChan chan *rpcClient // channel to hold the connections to a node
	lastUsed time.Time       // used for evicting inactive connections based on LRU
}

func (cp *connectionPair) createConnections(nodeID string, numConn uint32) {
	log.Debug("connectionPair::createConnections: Creating %d connections for node %s", numConn, nodeID)

	cp.connChan = make(chan *rpcClient, numConn)
	cp.lastUsed = time.Now()

	// Create connections and add them to the channel
	for i := 0; i < int(numConn); i++ {
		// TODO: getNodeAddressFromID should be replaced with a function to get the node address from the config
		conn, err := newRPCClient(nodeID, getNodeAddressFromID(nodeID))
		if err != nil {
			log.Err("connectionPair::createConnections: Failed to create connection for nodeID %v [%v]", nodeID, err.Error())
			continue // skip this connection
		}
		cp.connChan <- conn
	}
}

// closeConnections closes all connections in the channel
func (cp *connectionPair) closeConnections() error {
	close(cp.connChan)

	for conn := range cp.connChan {
		err := conn.close()
		if err != nil {
			log.Err("connectionPair::closeConnections: Failed to close connection [%v]", err.Error())
			return err
		}
	}

	return nil
}

// TODO: call cluster manager to get the node address for the given node ID
func getNodeAddressFromID(nodeID string) string {
	return "localhost:9090"
}
