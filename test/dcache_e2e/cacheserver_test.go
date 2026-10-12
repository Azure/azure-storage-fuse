//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package dcache_e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// cacheserverRolloutTimeout allows for startup and readiness probes.
const cacheserverRolloutTimeout = 3 * time.Minute

// kindNodeStateTimeout allows the kubelet heartbeat lease to expire after
// kubelet on a kind node is stopped.
const kindNodeStateTimeout = 2 * time.Minute

// nodeNetworkProbeTimeout bounds the post-restore data-plane probes. Stopping
// kubelet leaves the node's network namespace, pod IPs, iptables and kindnet
// routes untouched, so these only have to outlast kubelet restarting the one
// cache-server container it finds stopped.
//
// The probes still report rather than fail (see restoreKindNode): they gate the
// health of the restore path, not what the test proves.
const nodeNetworkProbeTimeout = 60 * time.Second

// kindNodeDialTimeout bounds a single TCP connect attempt issued inside a kind
// node. A blackholed route manifests as a connect timeout rather than a
// refusal, so this must be short enough to poll against.
const kindNodeDialTimeout = 3 * time.Second

// discoveryStateTimeout bounds a single diagnostic kubectl query. These hit
// the API server rather than a kubelet, so they stay responsive even while a
// node is down, but a bounded wait keeps a wedged query from stalling the test.
const discoveryStateTimeout = 15 * time.Second

// logDiscoveryBackends records which pods the discovery Service would route a
// GetCacheServers RPC to at this instant.
//
// Discovery is a single RPC to a ClusterIP Service whose selector matches every
// cache-server pod -- including one whose node has just been killed, since the
// server list is generated from a ConfigMap with no liveness filtering. The
// client library discards the failure reason, so when discovery returns zero
// servers the only way to tell "the RPC was routed to a dead backend" from "the
// RPC reached a live server that returned nothing" is to know whether the dead
// pod was still a routable endpoint at that moment.
//
// Purely diagnostic: every failure here is logged and swallowed, never fatal.
func logDiscoveryBackends(t *testing.T, phase string) {
	t.Helper()

	svc := testCfg.cacheserverStatefulSet + "-discovery"
	run := func(desc string, args ...string) {
		ctx, cancel := context.WithTimeout(context.Background(), discoveryStateTimeout)
		defer cancel()

		out, err := exec.CommandContext(ctx, testCfg.kubectlBin, args...).CombinedOutput()
		text := strings.TrimSpace(string(out))
		if err != nil {
			t.Logf("discovery-state [%s]: %s: query failed: %v (out: %s)", phase, desc, err, text)
			return
		}
		if text == "" {
			text = "(none)"
		}
		t.Logf("discovery-state [%s]: %s:\n%s", phase, desc, text)
	}

	run("endpoints behind service "+svc,
		"-n", testCfg.cacheserverNamespace,
		"get", "endpointslices",
		"-l", "kubernetes.io/service-name="+svc,
		"-o", `jsonpath={range .items[*]}{range .endpoints[*]}{"  "}{.targetRef.name}`+
			`{" addr="}{.addresses[0]}{" ready="}{.conditions.ready}`+
			`{" serving="}{.conditions.serving}{" node="}{.nodeName}{"\n"}{end}{end}`,
	)

	run("cache-server pods",
		"-n", testCfg.cacheserverNamespace,
		"get", "pod",
		"-l", testCfg.cacheserverSelector,
		"-o", `jsonpath={range .items[*]}{"  "}{.metadata.name}{" node="}{.spec.nodeName}`+
			`{" ip="}{.status.podIP}{" phase="}{.status.phase}{"\n"}{end}`,
	)
}

// listCacheserverPods returns cache-server pod names in kubectl order.
func listCacheserverPods(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.cacheserverNamespace,
		"get", "pod",
		"-l", testCfg.cacheserverSelector,
		"-o", "jsonpath={.items[*].metadata.name}",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("cacheserver: list pods: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		t.Fatalf("cacheserver: no pods found for selector %q in namespace %q",
			testCfg.cacheserverSelector, testCfg.cacheserverNamespace)
	}
	return strings.Fields(name)
}

func getPodNode(namespace, pod string) (string, error) {
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", namespace,
		"get", "pod", pod,
		"-o", "jsonpath={.spec.nodeName}",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get pod %s node: %w (out: %s)", pod, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// failKindNode takes a worker out of service without destroying it.
//
// Stopping kubelet makes the node miss its heartbeat lease, so the node
// controller marks it NotReady and the cache-server endpoint behind the
// headless Service flips not-ready. That withdraws the pod's per-pod DNS
// record, and cache clients address servers purely by name
// (<sts>-<ordinal>.<svc>.<ns>.svc.cluster.local), so the victim becomes
// unaddressable exactly as it would be had the machine died.
//
// The cache-server container is then stopped directly. kubelet is already down
// and cannot restart it, so the server is genuinely absent rather than merely
// unnamed: a client still holding a cached DNS answer is refused instead of
// served, which keeps the blob-fallback assertion honest.
//
// Nothing is destroyed. The node container, its network namespace, pod IPs,
// iptables, kindnet routes and /dev/fuse all survive, so restoreKindNode is one
// systemctl start rather than the cluster-wide rebuild a `docker kill` forces.
func failKindNode(t *testing.T, node string) {
	t.Helper()
	label, err := exec.Command(testCfg.dockerBin, "inspect",
		"--format", `{{index .Config.Labels "io.x-k8s.kind.cluster"}}`, node,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("kind: inspect node container %s: %v (out: %s)", node, err, strings.TrimSpace(string(label)))
	}
	if strings.TrimSpace(string(label)) == "" {
		t.Fatalf("kind: refusing to disrupt Docker container %s: missing io.x-k8s.kind.cluster label", node)
	}

	// kubelet.service is Restart=always, but systemd does not resurrect a unit
	// that was stopped explicitly, so this holds until restoreKindNode.
	out, err := exec.Command(testCfg.dockerBin, "exec", node,
		"systemctl", "stop", "kubelet").CombinedOutput()
	if err != nil {
		t.Fatalf("kind: stop kubelet on node %s: %v (out: %s)", node, err, strings.TrimSpace(string(out)))
	}
	t.Logf("kind: stopped kubelet on node %s", node)

	ids, err := exec.Command(testCfg.dockerBin, "exec", node,
		"crictl", "ps", "--name", "cacheserver", "--state", "Running", "-q",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("kind: list cache-server containers on node %s: %v (out: %s)",
			node, err, strings.TrimSpace(string(ids)))
	}
	running := strings.Fields(string(ids))
	if len(running) == 0 {
		t.Logf("kind: WARNING: no running cache-server container on node %s; "+
			"the victim is unaddressable by name but nothing was stopped", node)
	}
	for _, id := range running {
		out, err := exec.Command(testCfg.dockerBin, "exec", node,
			"crictl", "stop", "--timeout", "5", id).CombinedOutput()
		if err != nil {
			t.Fatalf("kind: stop cache-server container %s on node %s: %v (out: %s)",
				id, node, err, strings.TrimSpace(string(out)))
		}
		t.Logf("kind: stopped cache-server container %s on node %s", id, node)
	}
}

// restoreKindNode undoes failKindNode by starting kubelet again.
//
// Because nothing was destroyed, kubelet resumes against the state it left:
// the same network namespace, pod IPs, iptables and kindnet routes. It then
// restarts the cache-server container it finds stopped. No /dev/fuse recreation
// and no kube-proxy or kindnet restart is involved, so the node is usable by
// later tests instead of being stranded.
func restoreKindNode(t *testing.T, node string) {
	t.Helper()
	out, err := exec.Command(testCfg.dockerBin, "exec", node,
		"systemctl", "start", "kubelet").CombinedOutput()
	if err != nil {
		t.Errorf("cleanup: start kubelet on kind node %s: %v (out: %s)",
			node, err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cleanup: started kubelet on kind node %s", node)
	if err := waitKindNodeReady(node, true); err != nil {
		t.Errorf("cleanup: %v", err)
		return
	}
	t.Logf("cleanup: kind node %s is Ready", node)

	// Node-local readiness says nothing about the cache-server container coming
	// back or about the data plane, so observe both before the next test runs.
	// These probes report rather than fail: the restore path is no longer
	// expected to damage anything, and a false failure here would be charged to
	// whichever test happened to own the cleanup.
	waitCacheserverStatefulSetReady(t)
	waitClusterIPReachableFromNode(t, node)
	waitCacheserverPodsReachable(t)
}

// waitClusterIPReachableFromNode reports whether node can open a TCP connection
// to the kube-dns ClusterIP, waiting up to nodeNetworkProbeTimeout.
//
// A restarted kind node rebuilds its iptables from scratch. Stopping kubelet
// does neither: it leaves this node's iptables in place and never restarts
// kube-proxy, so Service routing should be intact throughout and this should
// pass on the first attempt. It is kept as a regression gate, because a
// blobfuse2 pod scheduled on a node that reaches no Service blocks in
// azstorage's TestPipeline inside internal.NewPipeline - before the mount
// exists - so its readiness probe never passes and its liveness probe
// eventually kills it, surfacing as a failure in some unrelated later test.
//
// Observed and logged, not asserted: see restoreKindNode for why.
func waitClusterIPReachableFromNode(t *testing.T, node string) {
	t.Helper()
	dnsIP, err := clusterDNSIP()
	if err != nil {
		t.Logf("cleanup: look up kube-dns ClusterIP: %v", err)
		return
	}
	deadline := time.Now().Add(nodeNetworkProbeTimeout)
	for {
		dialErr := dialFromKindNode(node, dnsIP, 53)
		if dialErr == nil {
			t.Logf("cleanup: kind node %s reaches kube-dns ClusterIP %s", node, dnsIP)
			return
		}
		if time.Now().After(deadline) {
			t.Logf("cleanup: WARNING: kind node %s could not reach kube-dns ClusterIP %s within %s; "+
				"pods scheduled there cannot resolve DNS and will hang before mounting, "+
				"which would fail some later test rather than this one: %v",
				node, dnsIP, nodeNetworkProbeTimeout, dialErr)
			return
		}
		time.Sleep(3 * time.Second)
	}
}

// waitCacheserverPodsReachable reports whether every cache-server pod accepts a
// TCP connection issued from every *other* cache-server node, waiting up to
// nodeNetworkProbeTimeout.
//
// The distributed cache is cross-node by construction: one shared StatefulSet
// spreads the hash ring over all nodes, so a blobfuse2 pod must reach every
// server wherever it is scheduled. This confirms that the cache-server
// container kubelet restarted is accepting connections again, and that the
// cross-node pod-CIDR routes - which stopping kubelet leaves in place - still
// carry traffic to it.
//
// Observed and logged, not asserted: see restoreKindNode for why.
func waitCacheserverPodsReachable(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(nodeNetworkProbeTimeout)
	for {
		pods, err := listCacheserverNetInfo()
		if err == nil {
			err = probeCacheserverPods(pods)
			if err == nil {
				t.Logf("cacheserver: all %d pods reachable from every peer node", len(pods))
				return
			}
		}
		if time.Now().After(deadline) {
			t.Logf("cacheserver: WARNING: cross-node pod networking did not recover within %s: %v",
				nodeNetworkProbeTimeout, err)
			return
		}
		time.Sleep(3 * time.Second)
	}
}

// probeCacheserverPods dials every pod's advertised ports from each peer node.
func probeCacheserverPods(pods []podNetInfo) error {
	for _, pod := range pods {
		if pod.ip == "" {
			return fmt.Errorf("pod %s has no IP assigned yet", pod.name)
		}
		for _, peer := range pods {
			if peer.node == pod.node {
				continue
			}
			for _, port := range pod.ports {
				if err := dialFromKindNode(peer.node, pod.ip, port); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// podNetInfo is a cache-server pod's scheduling and addressing, as needed by the
// post-restore reachability probes.
type podNetInfo struct {
	name  string
	node  string
	ip    string
	ports []int
}

func listCacheserverNetInfo() ([]podNetInfo, error) {
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.cacheserverNamespace,
		"get", "pod",
		"-l", testCfg.cacheserverSelector,
		"-o", "json",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list cacheserver pods: %w (out: %s)", err, strings.TrimSpace(string(out)))
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Ports []struct {
						ContainerPort int `json:"containerPort"`
					} `json:"ports"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				PodIP string `json:"podIP"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("decode cacheserver pod list: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no pods found for selector %q in namespace %q",
			testCfg.cacheserverSelector, testCfg.cacheserverNamespace)
	}
	pods := make([]podNetInfo, 0, len(list.Items))
	for _, item := range list.Items {
		info := podNetInfo{
			name: item.Metadata.Name,
			node: item.Spec.NodeName,
			ip:   item.Status.PodIP,
		}
		for _, container := range item.Spec.Containers {
			for _, port := range container.Ports {
				info.ports = append(info.ports, port.ContainerPort)
			}
		}
		pods = append(pods, info)
	}
	return pods, nil
}

func clusterDNSIP() (string, error) {
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", "kube-system",
		"get", "svc", "kube-dns",
		"-o", "jsonpath={.spec.clusterIP}",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get kube-dns service: %w (out: %s)", err, strings.TrimSpace(string(out)))
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("kube-dns service has no ClusterIP")
	}
	return ip, nil
}

// dialFromKindNode attempts one TCP connect to ip:port from inside node's
// network namespace. kind node images ship bash but neither nc nor nslookup,
// so bash's /dev/tcp is the only dependency-free probe available.
func dialFromKindNode(node, ip string, port int) error {
	script := fmt.Sprintf("timeout %d bash -c 'echo > /dev/tcp/%s/%d'",
		int(kindNodeDialTimeout.Seconds()), ip, port)
	out, err := exec.Command(testCfg.dockerBin, "exec", node, "bash", "-c", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dial %s:%d from node %s: %w (out: %s)",
			ip, port, node, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitKindNodeReady(node string, wantReady bool) error {
	deadline := time.Now().Add(kindNodeStateTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(testCfg.kubectlBin,
			"get", "node", node,
			"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`,
		).CombinedOutput()
		if err == nil {
			ready := strings.TrimSpace(string(out)) == "True"
			if ready == wantReady {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("kind node %s did not become Ready=%t within %s", node, wantReady, kindNodeStateTimeout)
}

// waitCacheserverStatefulSetReady restores cluster health after fault tests.
//
// `kubectl rollout status` alone is not enough here: for a StatefulSet with a
// partitioned update strategy it exits 0 as soon as the pods have been
// *updated*, printing "partitioned roll out complete" even while it is still
// "Waiting for N pods to be ready". A test that mounts in that window races
// the cache servers re-registering with discovery: reads still return correct
// bytes via blob fallback, but nothing is populated into or served from L2,
// so the next test fails its metric assertions for no visible reason.
// Gate on readyReplicas == replicas instead.
func waitCacheserverStatefulSetReady(t *testing.T) {
	t.Helper()
	args := []string{
		"-n", testCfg.cacheserverNamespace,
		"rollout", "status", "statefulset/" + testCfg.cacheserverStatefulSet,
		fmt.Sprintf("--timeout=%ds", int(cacheserverRolloutTimeout.Seconds())),
	}
	out, err := exec.Command(testCfg.kubectlBin, args...).CombinedOutput()
	if err != nil {
		t.Errorf("cacheserver: rollout status failed: %v (out: %s)",
			err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cacheserver: rollout reported (%s)", strings.TrimSpace(string(out)))

	deadline := time.Now().Add(cacheserverRolloutTimeout)
	var desired, ready string
	for time.Now().Before(deadline) {
		o, err := exec.Command(testCfg.kubectlBin,
			"-n", testCfg.cacheserverNamespace,
			"get", "statefulset", testCfg.cacheserverStatefulSet,
			"-o", "jsonpath={.status.replicas}/{.status.readyReplicas}",
		).CombinedOutput()
		if err == nil {
			// readyReplicas is omitted entirely when zero, so the right
			// hand side can legitimately come back empty.
			desired, ready, _ = strings.Cut(strings.TrimSpace(string(o)), "/")
			if desired != "" && desired == ready {
				t.Logf("cacheserver: all %s replicas ready", desired)
				return
			}
		}
		time.Sleep(2 * time.Second)
	}

	t.Errorf("cacheserver: statefulset %s not fully ready after %s (replicas=%q readyReplicas=%q); "+
		"mounts scheduled next are likely to race discovery and see an incomplete cache ring",
		testCfg.cacheserverStatefulSet, cacheserverRolloutTimeout, desired, ready)
}
