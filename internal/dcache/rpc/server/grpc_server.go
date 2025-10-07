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
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	grpcservice "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/service"
)

// GrpcNodeServer holds the gRPC server
type GrpcNodeServer struct {
	address string
	server  *grpc.Server
}

// NewGrpcNodeServer creates a gRPC server for the node.
func NewGrpcNodeServer(rvMap map[string]dcache.RawVolume) (*GrpcNodeServer, error) {
	common.Assert(cm.IsValidRVMap(rvMap))

	nodeID, err := common.GetNodeUUID()
	if err != nil {
		log.Err("GrpcNodeServer::NewNodeServer: Failed to get node ID [%v]", err.Error())
		return nil, err
	}
	address := rpc.GetNodeAddressFromID(nodeID)
	if !common.IsValidHostPort(address) {
		log.Err("GrpcNodeServer::NewNodeServer: Invalid node address %s", address)
		return nil, fmt.Errorf("invalid node address %s", address)
	}

	if err = NewChunkServiceHandler(rvMap); err != nil {
		log.Err("GrpcNodeServer::NewNodeServer: NewChunkServiceHandler failed: %v", err)
		return nil, err
	}

	gs := grpc.NewServer()
	grpcservice.RegisterChunkServiceServer(gs, &grpcChunkService{})
	log.Info("GrpcNodeServer::NewNodeServer: gRPC server created at %s", address)
	return &GrpcNodeServer{address: address, server: gs}, nil
}

// Serve starts the gRPC server
func (s *GrpcNodeServer) Serve() error {
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		log.Err("GrpcNodeServer::Serve: Failed to listen on %s: %v", s.address, err)
		return err
	}
	log.Info("GrpcNodeServer::Serve: gRPC server listening on %s", s.address)
	return s.server.Serve(lis)
}

// Stop stops the gRPC server
func (s *GrpcNodeServer) Stop() {
	log.Info("GrpcNodeServer::Stop: Stopping gRPC server")
	s.server.GracefulStop()
}

// Service implementation moved to grpc_handler.go
