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
	"context"
	"crypto/tls"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache"
	"github.com/apache/thrift/lib/go/thrift"
)

// Connection struct holds the Thrift connection to a node
// This is used to make RPC calls to the node
type Connection struct {
	nodeID      string                     // Node ID of the node this connection is for, can be used for debug logs
	nodeAddress string                     // Address of the node this connection is for
	ctx         context.Context            // Context for the connection
	Transport   thrift.TTransport          // Transport is the Thrift transport layer
	Client      *dcache.ChunkServiceClient // Client is the Thrift client for the ChunkService
}

// NewConnection creates a new Thrift connection to a node
func NewConnection(nodeID string, nodeAddress string) (*Connection, error) {
	log.Debug("Connection::NewConnection: Creating new connection to node %s at %s", nodeID, nodeAddress)

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTTransportFactory()

	var transport thrift.TTransport
	cfg := &thrift.TConfiguration{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// if secure {
	// 	transport = thrift.NewTSSLSocketConf(addr, cfg)
	// }

	transport = thrift.NewTSocketConf(nodeAddress, cfg)
	transport, err := transportFactory.GetTransport(transport)
	if err != nil {
		log.Err("Connection::NewConnection: Failed to create transport [%v]", err.Error())
		return nil, err
	}

	iprot := protocolFactory.GetProtocol(transport)
	oprot := protocolFactory.GetProtocol(transport)
	client := dcache.NewChunkServiceClient(thrift.NewTStandardClient(iprot, oprot))

	conn := &Connection{
		nodeID:      nodeID,
		nodeAddress: nodeAddress,
		ctx:         context.Background(), // TODO: check if context with cancel is needed
		Transport:   transport,
		Client:      client,
	}

	err = conn.Transport.Open()
	if err != nil {
		log.Err("Connection::NewConnection: Failed to open transport [%v]", err.Error())
		return nil, err
	}

	return conn, nil
}

// Close closes the Thrift connection to the node
func (c *Connection) Close() error {
	err := c.Transport.Close()
	if err != nil {
		log.Err("Connection::Close: Failed to close transport [%v]", err.Error())
		return err
	}

	return nil
}

// IsAlive checks if the connection to the node is alive
func (c *Connection) IsAlive() bool {
	err := c.Client.Ping(c.ctx)
	if err != nil {
		log.Err("Connection::IsAlive: Failed to connect to node %s at %s  [%v]", c.nodeID, c.nodeAddress, err.Error())
		return false
	}

	return true
}

// GetChunk retrieves a chunk from the node
func (c *Connection) GetChunk(fileID string, fsID string, mirrorVolume int64, offset int64) (*dcache.Chunk, error) {
	log.Debug("Connection::GetChunk: Getting chunk from node %s at %s for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d", c.nodeID, c.nodeAddress, fileID, fsID, mirrorVolume, offset)

	chunk, err := c.Client.GetChunk(c.ctx, fileID, fsID, mirrorVolume, offset)
	if err != nil {
		log.Err("Connection::GetChunk: Failed to get chunk from node %s at %s [%v]", c.nodeID, c.nodeAddress, err.Error())
		return nil, err
	}

	// TODO: validate hash of the chunk

	return chunk, nil
}

// PutChunk writes a chunk to the node
func (c *Connection) PutChunk(chunk *dcache.Chunk) error {
	log.Debug("Connection::PutChunk: Putting chunk to node %s at %s for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d", c.nodeID, c.nodeAddress, chunk.FileID, chunk.FsID, chunk.MirrorVolume, chunk.Offset)

	err := c.Client.PutChunk(c.ctx, chunk)
	if err != nil {
		log.Err("Connection::PutChunk: Failed to put chunk to node %s at %s [%v]", c.nodeID, c.nodeAddress, err.Error())
		return err
	}

	return nil
}

// RemoveChunk deletes a chunk from the node
func (c *Connection) RemoveChunk(fileID string, fsID string, mirrorVolume int64, offset int64) error {
	log.Debug("Connection::RemoveChunk: Removing chunk from node %s at %s for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d", c.nodeID, c.nodeAddress, fileID, fsID, mirrorVolume, offset)

	err := c.Client.RemoveChunk(c.ctx, fileID, fsID, mirrorVolume, offset)
	if err != nil {
		log.Err("Connection::RemoveChunk: Failed to remove chunk from node %s at %s [%v]", c.nodeID, c.nodeAddress, err.Error())
		return err
	}

	return nil
}
