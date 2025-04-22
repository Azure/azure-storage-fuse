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

package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// ConnectionPool manages multiple connections efficiently
type ConnectionPool struct {
	mu          sync.Mutex
	connections map[string]*ConnectionPair // map of node ID to connection
	maxPerNode  uint32                     // Maximum number of open connections per node
	maxNodes    uint32                     // Maximum number of nodes for which connections are open
	timeout     uint32                     // Duration in seconds after which a connection is closed
}

// NewConnectionPool creates a new connection pool with the specified parameters
// maxPerNode: Maximum number of open connections per node
// maxNodes: Maximum number of nodes for which connections are open
// timeout: Duration in seconds after which a connection is closed
func NewConnectionPool(maxPerNode uint32, maxNodes uint32, timeout uint32) *ConnectionPool {
	log.Debug("ConnectionPool::NewConnectionPool: Creating new connection pool with maxPerNode: %d, maxNodes: %d, timeout: %d", maxPerNode, maxNodes, timeout)
	return &ConnectionPool{
		connections: make(map[string]*ConnectionPair),
		maxPerNode:  maxPerNode,
		maxNodes:    maxNodes,
		timeout:     timeout,
	}

	// TODO: start a goroutine to periodically close inactive connections
}

// GetConnection retrieves a connection from the pool for the specified node ID
// If no connection is available, a new one is created
func (cp *ConnectionPool) GetConnection(nodeID string) (*Connection, error) {
	log.Debug("ConnectionPool::GetConnection: Retrieving connection for node %s", nodeID)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	var connPair *ConnectionPair
	connPair, exists := cp.connections[nodeID]
	if !exists {
		if len(cp.connections) >= int(cp.maxNodes) {
			log.Debug("ConnectionPool::GetConnection: Maximum number of nodes reached, evict LRU connection")
			err := cp.closeLRUConnection()
			if err != nil {
				log.Err("ConnectionPool::GetConnection: Failed to close LRU connection [%v]", err.Error())
				return nil, err
			}
		}

		connPair = &ConnectionPair{}
		connPair.createConnections(nodeID, cp.maxPerNode)
		cp.connections[nodeID] = connPair
	}

	select {
	case conn := <-connPair.connChan:
		connPair.lastUsed = time.Now()
		return conn, nil
	default:
		log.Err("ConnectionPool::GetConnection: No available connections in the pool for node %s", nodeID)
		return nil, fmt.Errorf("no available connections in the pool for node %s", nodeID)
	}
}

// ReleaseConnection releases a connection back to the pool for the specified node ID
func (cp *ConnectionPool) ReleaseConnection(nodeID string, conn *Connection) error {
	log.Debug("ConnectionPool::ReleaseConnection: Releasing connection for node %s", nodeID)

	connPair, exists := cp.connections[nodeID]
	if !exists {
		log.Err("ConnectionPool::ReleaseConnection: No connection pair found for node %s", nodeID)
		return fmt.Errorf("no connection pair found for node %s", nodeID)
	}

	connPair.connChan <- conn
	return nil
}

// Close the least recently used connection from the connections pool
func (cp *ConnectionPool) closeLRUConnection() error {
	// Find the least recently used connection and close it
	var lruConnPair *ConnectionPair
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
			log.Err("ConnectionPool::closeLRUConnection: Failed to close LRU connection for node %s [%v]", lruNodeID, err.Error())
			return err
		}
		delete(cp.connections, lruNodeID)
	}

	return nil
}

// closeInactiveConnections closes connections that have not been used for a specified timeout
func (cp *ConnectionPool) closeInactiveConnections() {
	// Cleanup old connections based on the LastUsed timestamp
	// This will run in a separate goroutine and will periodically close the connections based on LRU strategy
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() error {
	// see if this is needed
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for key, connPair := range cp.connections {
		err := connPair.closeConnections()
		if err != nil {
			log.Err("ConnectionPool::Close: Failed to close connection for node %s [%v]", key, err.Error())
			return err
		}
		delete(cp.connections, key)
	}

	cp.connections = make(map[string]*ConnectionPair)
	return nil
}

// ------------------------------------------------------------------------------------------------------------------------------------------------------

// ConnectionPair holds a channel of connections to a node
// and the last used timestamp for LRU eviction
type ConnectionPair struct {
	connChan chan *Connection // channel to hold the connections to a node
	lastUsed time.Time        // used for evicting inactive connections based on LRU
}

func (cp *ConnectionPair) createConnections(nodeID string, numConn uint32) {
	log.Debug("ConnectionPair::createConnections: Creating %d connections for node %s", numConn, nodeID)

	cp.connChan = make(chan *Connection, numConn)
	cp.lastUsed = time.Now()

	// Create connections and add them to the channel
	for i := 0; i < int(numConn); i++ {
		// TODO: getNodeAddress should be replaced with a function to get the node address from the config
		conn, err := NewConnection(nodeID, getNodeAddress(nodeID))
		if err != nil {
			log.Err("ConnectionPair::createConnections: Failed to create connection for nodeID %v [%v]", nodeID, err.Error())
			continue // skip this connection
		}
		cp.connChan <- conn
	}
}

// closeConnections closes all connections in the channel
func (cp *ConnectionPair) closeConnections() error {
	close(cp.connChan)

	for conn := range cp.connChan {
		err := conn.Close()
		if err != nil {
			log.Err("ConnectionPair::closeConnections: Failed to close connection [%v]", err.Error())
			return err
		}
	}

	return nil
}

// TODO: this will be replaced with a function to get the node address from the config
func getNodeAddress(nodeID string) string {
	return "localhost:9090"
}
