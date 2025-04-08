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
