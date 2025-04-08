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

package server

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache"
	"github.com/apache/thrift/lib/go/thrift"
)

// NodeServer struct holds the Thrift server
type NodeServer struct {
	address string
	server  thrift.TServer
}

func NewNodeServer(address string, cacheDir string) (*NodeServer, error) {
	log.Debug("NodeServer::NewNodeServer: Creating NodeServer with address: %s, cacheDir: %s", address, cacheDir)

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTTransportFactory()

	var transport thrift.TServerTransport
	var err error

	// if secure {
	// 	cfg := new(tls.Config)
	// 	if cert, err := tls.LoadX509KeyPair("server.crt", "server.key"); err == nil {
	// 		cfg.Certificates = append(cfg.Certificates, cert)
	// 	} else {
	// 		return err
	// 	}
	// 	transport, err = thrift.NewTSSLServerSocket(addr, cfg)
	// }

	transport, err = thrift.NewTServerSocket(address)
	if err != nil {
		log.Err("NodeServer::NewNodeServer: Failed to create server socket [%v]", err.Error())
		return nil, err
	}

	handler := NewChunkServiceHandler(cacheDir)
	processor := dcache.NewChunkServiceProcessor(handler)
	server := thrift.NewTSimpleServer4(processor, transport, transportFactory, protocolFactory)

	return &NodeServer{
		address: address,
		server:  server,
	}, nil
}

func (ns *NodeServer) Start() error {
	log.Debug("NodeServer::Start: Starting NodeServer on address: %s", ns.address)
	err := ns.server.Serve()
	if err != nil {
		log.Err("NodeServer::Start: Failed to start server [%v]", err.Error())
		return err
	}

	return nil
}

func (ns *NodeServer) Stop() error {
	log.Debug("NodeServer::Stop: Stopping NodeServer on address: %s", ns.address)
	err := ns.server.Stop()
	if err != nil {
		log.Err("NodeServer::Stop: Failed to stop server [%v]", err.Error())
		return err
	}

	return nil
}
