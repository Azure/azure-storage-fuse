// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/Azure/azure-storage-fuse/v2/internal/dist_cache_client/proto"
)

// discovery manages server list resolution and background refresh.
type discovery struct {
	mu      sync.RWMutex
	servers []string
	ring    *ConsistentHashRing
	cfg     *clientConfig
	connMgr *connManager
	stopCh  chan struct{}
	stopped bool
}

func newDiscovery(cfg *clientConfig, connMgr *connManager, vnodes int) (*discovery, error) {
	d := &discovery{
		cfg:     cfg,
		connMgr: connMgr,
		ring:    NewConsistentHashRing(nil, vnodes),
		stopCh:  make(chan struct{}),
	}

	servers, err := d.resolveServers()
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, ErrNoServers
	}

	d.servers = servers
	d.ring.UpdateServers(servers)

	// Start background refresh if using dynamic discovery
	if cfg.discoveryURL != "" || cfg.k8sService != "" {
		go d.refreshLoop()
	}

	return d, nil
}

// getServer returns the server responsible for the given cache key.
func (d *discovery) getServer(key string) (string, error) {
	return d.ring.GetServer(key)
}

// getServers returns a copy of the current server list.
func (d *discovery) getServers() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]string, len(d.servers))
	copy(out, d.servers)
	return out
}

// resolveServers determines the server list using the configured discovery method.
// Priority: discovery URL > K8s DNS > static list > environment variable.
func (d *discovery) resolveServers() ([]string, error) {
	// 1. Discovery endpoint (GetCacheServers RPC)
	if d.cfg.discoveryURL != "" {
		servers, err := d.discoverViaRPC(d.cfg.discoveryURL)
		if err == nil && len(servers) > 0 {
			return servers, nil
		}
		// Fall through to next method on error
	}

	// 2. Kubernetes DNS (headless StatefulSet)
	if d.cfg.k8sService != "" && d.cfg.k8sNamespace != "" {
		servers, err := d.discoverViaK8sDNS(d.cfg.k8sService, d.cfg.k8sNamespace, d.cfg.port)
		if err == nil && len(servers) > 0 {
			return servers, nil
		}
	}

	// 3. Static server list from config
	if len(d.cfg.servers) > 0 {
		return d.cfg.servers, nil
	}

	// 4. Environment variable
	if envList := os.Getenv("DIST_CACHE_SERVER_LIST"); envList != "" {
		servers := parseServerList(envList)
		if len(servers) > 0 {
			return servers, nil
		}
	}

	return nil, ErrNoServers
}

// discoverViaRPC connects to the discovery endpoint and calls GetCacheServers.
func (d *discovery) discoverViaRPC(endpoint string) ([]string, error) {
	c, err := d.connMgr.getConn(endpoint)
	if err != nil {
		return nil, err
	}
	defer d.connMgr.putConn(c)

	if err := c.setDeadline(time.Now().Add(d.cfg.requestTimeout)); err != nil {
		d.connMgr.discardConn(c)
		return nil, err
	}

	req := &pb.Request{
		Payload: &pb.Request_Getcacheserversrequest{
			Getcacheserversrequest: &pb.GetCacheServersRequest{},
		},
	}

	if err := c.sendRequest(req, nil); err != nil {
		d.connMgr.discardConn(c)
		return nil, fmt.Errorf("send GetCacheServers: %w", err)
	}

	var resp pb.GetCacheServersResponse
	if err := c.recvProto(&resp); err != nil {
		d.connMgr.discardConn(c)
		return nil, fmt.Errorf("recv GetCacheServers: %w", err)
	}

	if resp.Result != pb.GetCacheServersResponse_SUCCESS {
		return nil, fmt.Errorf("GetCacheServers failed: %v", resp.Result)
	}

	// Normalize server addresses to include port
	servers := make([]string, 0, len(resp.Serveraddresses))
	for _, addr := range resp.Serveraddresses {
		if !strings.Contains(addr, ":") {
			addr = fmt.Sprintf("%s:%d", addr, d.cfg.port)
		}
		servers = append(servers, addr)
	}

	sort.Strings(servers)
	return servers, nil
}

// discoverViaK8sDNS resolves servers from a Kubernetes headless StatefulSet service.
// DNS pattern: cacheserver-{N}.{service}.{namespace}.svc.cluster.local
func (d *discovery) discoverViaK8sDNS(service, namespace string, port int) ([]string, error) {
	// Try SRV record first for the headless service
	svcDNS := fmt.Sprintf("%s.%s.svc.cluster.local", service, namespace)
	_, addrs, err := net.LookupSRV("", "", svcDNS)
	if err == nil && len(addrs) > 0 {
		servers := make([]string, 0, len(addrs))
		for _, a := range addrs {
			servers = append(servers, fmt.Sprintf("%s:%d", strings.TrimSuffix(a.Target, "."), port))
		}
		sort.Strings(servers)
		return servers, nil
	}

	// Fall back to A record lookup
	ips, err := net.LookupHost(svcDNS)
	if err != nil {
		return nil, fmt.Errorf("k8s DNS lookup %s: %w", svcDNS, err)
	}

	servers := make([]string, 0, len(ips))
	for _, ip := range ips {
		servers = append(servers, fmt.Sprintf("%s:%d", ip, port))
	}
	sort.Strings(servers)
	return servers, nil
}

// refreshLoop periodically re-discovers the server list.
func (d *discovery) refreshLoop() {
	ticker := time.NewTicker(d.cfg.discoveryRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.refresh()
		}
	}
}

func (d *discovery) refresh() {
	servers, err := d.resolveServers()
	if err != nil || len(servers) == 0 {
		return // keep existing servers on refresh failure
	}

	d.mu.Lock()
	d.servers = servers
	d.mu.Unlock()

	d.ring.UpdateServers(servers)
}

// close stops the background refresh loop.
func (d *discovery) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.stopped {
		d.stopped = true
		close(d.stopCh)
	}
}

// parseServerList splits a comma-separated server list string.
func parseServerList(list string) []string {
	parts := strings.Split(list, ",")
	servers := make([]string, 0, len(parts))
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			servers = append(servers, s)
		}
	}
	return servers
}

// DiscoverServers is a standalone function for discovering servers without creating a full client.
// Useful for health checks and diagnostics.
func DiscoverServers(ctx context.Context, cfg *clientConfig) ([]string, error) {
	cm := newConnManager(2, cfg.dialTimeout)
	defer cm.closeAll()

	d := &discovery{cfg: cfg, connMgr: cm}
	return d.resolveServers()
}
