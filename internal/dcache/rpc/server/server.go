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

package rpc_server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
	"github.com/apache/thrift/lib/go/thrift"
)

//go:generate $ASSERT_REMOVER $GOFILE

// NodeServer struct holds the Thrift server
type NodeServer struct {
	address string
	server  thrift.TServer
}

// NewNodeServer creates a Thrift server for the node.
// rvMap is a map of raw volumes that the node will serve.
func NewNodeServer(rvMap map[string]dcache.RawVolume) (*NodeServer, error) {
	common.Assert(cm.IsValidRVMap(rvMap))

	nodeID, err := common.GetNodeUUID()
	if err != nil {
		log.Err("NodeServer::NewNodeServer: Failed to get node ID [%v]", err)
		common.Assert(false, err)
		return nil, err
	}

	address := rpc.GetNodeAddressFromID(nodeID)

	if !common.IsValidHostPort(address) {
		log.Err("NodeServer::NewNodeServer: Invalid node address %s", address)
		common.Assert(false, address)
		return nil, fmt.Errorf("invalid node address %s", address)
	}

	log.Debug("NodeServer::NewNodeServer: Creating server with address: %s, RVs %+v", address, rvMap)

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTTransportFactory()

	var transport thrift.TServerTransport

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
		log.Err("NodeServer::NewNodeServer: Failed to create server socket [%v]", err)
		common.Assert(false, err)
		return nil, err
	}

	//
	// Create the chunk service handler.
	// This must set the global var handler.
	//
	err = NewChunkServiceHandler(rvMap)
	if err != nil {
		common.Assert(false, err)
		return nil, err
	}
	common.Assert(handler != nil)

	processor := service.NewChunkServiceProcessor(handler)
	server := thrift.NewTSimpleServer4(processor, transport, transportFactory, protocolFactory)

	return &NodeServer{
		address: address,
		server:  server,
	}, nil
}

func (ns *NodeServer) Start() error {
	log.Debug("NodeServer::Start: Starting NodeServer on address: %s", ns.address)

	go func() {
		err := ns.server.Serve() // this is a blocking call
		if err != nil {
			log.Err("NodeServer::Start: PANIC: failed to start server [%v]", err.Error())
			log.GetLoggerObj().Panicf("PANIC: failed to start server [%v]", err.Error())
		}
	}()

	return nil
}

func (ns *NodeServer) Stop() error {
	log.Debug("NodeServer::Stop: Stopping NodeServer on address: %s", ns.address)
	err := ns.server.Stop()
	if err != nil {
		log.Err("NodeServer::Stop: Failed to stop server [%v]", err.Error())
		common.Assert(false, err)
		return err
	}

	return nil
}

// Thrift server that uses one go routine per connection.
//
// TODO: Make it use a pool of goroutines instead of one per connection.
//
// Note: For IO intensive workloads, simple NodeServer is better than ThreadedNodeServer as too many
//       go routines can cause excessive context switching and CPU thrashing, reducing overall throughput.

type ThreadedNodeServer struct {
	address          string
	transport        thrift.TServerTransport
	transportFactory thrift.TTransportFactory
	protocolFactory  thrift.TProtocolFactory
	processor        *service.ChunkServiceProcessor
	context          context.Context
	cancel           context.CancelFunc
}

// NewThreadedNodeServer creates a threadded Thrift server that uses one go rouitne per connection.
// rvMap is a map of raw volumes that the node will serve.
func NewThreadedNodeServer(rvMap map[string]dcache.RawVolume) (*ThreadedNodeServer, error) {
	common.Assert(cm.IsValidRVMap(rvMap))

	nodeID, err := common.GetNodeUUID()
	if err != nil {
		log.Err("NodeServer::NewThreadedNodeServer: Failed to get node ID [%v]", err.Error())
		common.Assert(false, err)
		return nil, err
	}

	address := rpc.GetNodeAddressFromID(nodeID)

	if !common.IsValidHostPort(address) {
		log.Err("NodeServer::NewThreadedNodeServer: Invalid node address %s", address)
		common.Assert(false, address)
		return nil, fmt.Errorf("invalid node address %s", address)
	}

	log.Debug("NodeServer::NewThreadedNodeServer: Creating server with address: %s, RVs %+v", address, rvMap)

	ctx, cancel := context.WithCancel(context.Background())
	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	transportFactory := thrift.NewTTransportFactory()

	var transport thrift.TServerTransport
	transport, err = thrift.NewTServerSocket(address)
	if err != nil {
		log.Err("NodeServer::NewThreadedNodeServer: Failed to create server socket [%v]", err)
		common.Assert(false, err)
		return nil, err
	}

	err = transport.Listen()
	if err != nil {
		log.Err("NodeServer::NewThreadedNodeServer: Failed to listen on server socket [%v]", err)
		common.Assert(false, err)
		return nil, err
	}

	//
	// Create the chunk service handler.
	// This must set the global var handler.
	//
	err = NewChunkServiceHandler(rvMap)
	if err != nil {
		common.Assert(false, err)
		return nil, err
	}
	common.Assert(handler != nil)

	processor := service.NewChunkServiceProcessor(handler)

	return &ThreadedNodeServer{
		address:          address,
		transport:        transport,
		transportFactory: transportFactory,
		protocolFactory:  protocolFactory,
		processor:        processor,
		context:          ctx,
		cancel:           cancel,
	}, nil
}

func (ns *ThreadedNodeServer) Start() error {
	log.Debug("ThreadedNodeServer::Start: Starting ThreadedNodeServer on address: %s", ns.address)

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Warn("ThreadedNodeServer::Start: Shutting down...")
		ns.cancel()
		ns.transport.Interrupt()
	}()

	go func() {
		log.Info("ThreadedNodeServer::Start: Accepting client connections on: %s", ns.address)

		for {
			client, err := ns.transport.Accept()
			if err != nil {
				log.Err("ThreadedNodeServer::Start: PANIC: thrift accept error [%v]", err)
				log.GetLoggerObj().Panicf("ThreadedNodeServer::Start: PANIC: thrift accept error [%v]", err)
				break
			}

			log.Debug("ThreadedNodeServer::Start: Accepted new client connection!")
			go ns.processConn(client, ns.processor, ns.transportFactory, ns.protocolFactory)
		}
	}()

	return nil
}

func (ns *ThreadedNodeServer) Stop() error {
	log.Debug("ThreadedNodeServer::Stop: Stopping ThreadedNodeServer on address: %s", ns.address)
	ns.cancel()
	ns.transport.Interrupt()

	return nil
}

func (ns *ThreadedNodeServer) processConn(client thrift.TTransport, processor thrift.TProcessor,
	transportFactory thrift.TTransportFactory, protocolFactory thrift.TProtocolFactory) {

	defer client.Close()

	inputTransport, err := transportFactory.GetTransport(client)
	if err != nil {
		log.Err("ThreadedNodeServer::processConn: Failed to get input transport [%v]", err)
		return
	}

	outputTransport, err := transportFactory.GetTransport(client)
	if err != nil {
		log.Err("ThreadedNodeServer::processConn: Failed to get output transport [%v]", err)
		return
	}

	inputProtocol := protocolFactory.GetProtocol(inputTransport)
	outputProtocol := protocolFactory.GetProtocol(outputTransport)

	for {
		ok, err := processor.Process(ns.context, inputProtocol, outputProtocol)
		if err != nil {
			log.Err("ThreadedNodeServer::processConn: Client disconnected or error [%v]", err)
			break
		}
		if !ok {
			log.Debug("ThreadedNodeServer::processConn: no more work...")
		}
	}
}

// Silence unused import errors for release builds.
func init() {
	cm.IsValidMVName("mv0")
}
