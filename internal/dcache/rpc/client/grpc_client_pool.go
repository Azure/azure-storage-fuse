package rpc_client

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
)

const grpcPerNodeConns = 16

// grpcNodePool holds fixed connections for one node.
type grpcNodePool struct {
	conns []*grpcRPCClient
	rrIdx atomic.Uint32
}

// grpcClientPool manages per-node grpcNodePool.
type grpcClientPool struct {
	mu   sync.RWMutex
	pool map[string]*grpcNodePool // key: nodeID
}

func newGrpcClientPool() *grpcClientPool {
	return &grpcClientPool{pool: make(map[string]*grpcNodePool)}
}

// getClient returns a gRPC client for given nodeID in round-robin manner; creates pool if missing.
func (gp *grpcClientPool) getClient(nodeID string) (*grpcRPCClient, error) {
	address := rpc.GetNodeAddressFromID(nodeID)
	gp.mu.RLock()
	np := gp.pool[nodeID]
	gp.mu.RUnlock()
	if np == nil {
		// create
		gp.mu.Lock()
		// double-check
		np = gp.pool[nodeID]
		if np == nil {
			conns := make([]*grpcRPCClient, 0, grpcPerNodeConns)
			for i := 0; i < grpcPerNodeConns; i++ {
				c, err := newGrpcRPCClient(address)
				if err != nil {
					// close any created
					for _, pc := range conns {
						pc.Close()
					}
					gp.mu.Unlock()
					return nil, fmt.Errorf("newGrpcClientPool: dial %s failed: %w", address, err)
				}
				conns = append(conns, c)
			}
			np = &grpcNodePool{conns: conns}
			gp.pool[nodeID] = np
			log.Debug("grpcClientPool: created %d conns for node %s", len(conns), nodeID)
		}
		gp.mu.Unlock()
	}
	// Pick round-robin
	idx := int(np.rrIdx.Add(1)-1) % len(np.conns)
	return np.conns[idx], nil
}

// closeAll closes all connections (best effort).
func (gp *grpcClientPool) closeAll() {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	for id, np := range gp.pool {
		for _, c := range np.conns {
			c.Close()
		}
		delete(gp.pool, id)
	}
}
