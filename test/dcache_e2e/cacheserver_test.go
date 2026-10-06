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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// cacheserverRolloutTimeout allows for startup and readiness probes.
const cacheserverRolloutTimeout = 3 * time.Minute

// kindNodeStateTimeout allows the kubelet heartbeat lease to expire after a
// kind node container is stopped.
const kindNodeStateTimeout = 2 * time.Minute

// nodeNetworkProbeTimeout bounds the post-restore data-plane probes. Route and
// iptables reprogramming is normally seconds; this only has to outlast a
// kindnet/kube-proxy pod restart.
//
// The probes report rather than fail (see restoreKindNode), so this is kept
// short: on a kernel where kube-proxy cannot restart the wait is pure dead
// time, and on a healthy kernel recovery is observed well inside it.
const nodeNetworkProbeTimeout = 60 * time.Second

// kindNodeDialTimeout bounds a single TCP connect attempt issued inside a kind
// node. A blackholed route manifests as a connect timeout rather than a
// refusal, so this must be short enough to poll against.
const kindNodeDialTimeout = 3 * time.Second

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

func killKindNode(t *testing.T, node string) {
	t.Helper()
	label, err := exec.Command(testCfg.dockerBin, "inspect",
		"--format", `{{index .Config.Labels "io.x-k8s.kind.cluster"}}`, node,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("kind: inspect node container %s: %v (out: %s)", node, err, strings.TrimSpace(string(label)))
	}
	if strings.TrimSpace(string(label)) == "" {
		t.Fatalf("kind: refusing to kill Docker container %s: missing io.x-k8s.kind.cluster label", node)
	}
	out, err := exec.Command(testCfg.dockerBin, "kill", node).CombinedOutput()
	if err != nil {
		t.Fatalf("kind: kill node %s: %v (out: %s)", node, err, strings.TrimSpace(string(out)))
	}
	t.Logf("kind: killed node container %s", node)
}

func restoreKindNode(t *testing.T, node string) {
	t.Helper()
	out, err := exec.Command(testCfg.dockerBin, "start", node).CombinedOutput()
	if err != nil {
		t.Errorf("cleanup: start kind node %s: %v (out: %s)", node, err, strings.TrimSpace(string(out)))
		return
	}
	out, err = exec.Command(testCfg.dockerBin,
		"exec", node, "sh", "-c",
		"if [ ! -c /dev/fuse ]; then mknod -m 0666 /dev/fuse c 10 229; else chmod 0666 /dev/fuse; fi; test -c /dev/fuse",
	).CombinedOutput()
	if err != nil {
		t.Errorf("cleanup: restore /dev/fuse on kind node %s: %v (out: %s)",
			node, err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cleanup: kind node %s has a usable /dev/fuse device", node)
	if err := waitKindNodeReady(node, true); err != nil {
		t.Errorf("cleanup: %v", err)
		return
	}
	t.Logf("cleanup: kind node %s is Ready", node)
	// Node-local readiness says nothing about the data plane, so reprogram the
	// node's routes and Service rules and then observe both layers.
	//
	// These probes report; they do not fail the test. On some kernels kube-proxy
	// cannot complete startup inside a restarted container and crash-loops, which
	// leaves every Service ClusterIP - including kube-dns - unreachable from this
	// node for as long as it lives. Nothing a test can do repairs that. The suite
	// therefore runs this test last (see the split invocation in
	// azure-pipeline-templates/dist-cache-e2e.yml) and the cluster is torn down
	// immediately afterwards, so a node left in this state harms nothing. The
	// probe output is kept because it is the only in-band record of the damage.
	recycleNodeNetworkDaemons(t, node)
	waitCacheserverStatefulSetReady(t)
	waitClusterIPReachableFromNode(t, node)
	waitCacheserverPodsReachable(t)
}

// recycleNodeNetworkDaemons deletes the kindnet and kube-proxy pods bound to
// node so both reprogram the node's pod-CIDR routes and Service iptables rules
// from a clean slate. Both are DaemonSet members, so the control plane recreates
// them immediately.
//
// This is a remedy, not an assertion: the reachability gates below decide
// whether the node actually recovered, so a failure here is only logged.
func recycleNodeNetworkDaemons(t *testing.T, node string) {
	t.Helper()
	for _, app := range []string{"kindnet", "kube-proxy"} {
		out, err := exec.Command(testCfg.kubectlBin,
			"-n", "kube-system",
			"delete", "pod",
			"-l", "k8s-app="+app,
			"--field-selector", "spec.nodeName="+node,
			"--ignore-not-found",
		).CombinedOutput()
		if err != nil {
			t.Logf("cleanup: recycle %s on %s: %v (out: %s)",
				app, node, err, strings.TrimSpace(string(out)))
			continue
		}
		t.Logf("cleanup: recycled %s on %s to force network reprogramming", app, node)
	}
}

// waitClusterIPReachableFromNode reports whether node can open a TCP connection
// to the kube-dns ClusterIP, waiting up to nodeNetworkProbeTimeout.
//
// A restarted kind node rebuilds its iptables from scratch. Until kube-proxy
// reprograms them the node reaches no Service at all, including DNS. A blobfuse2
// pod scheduled there then blocks in azstorage's TestPipeline inside
// internal.NewPipeline - before the mount exists - so its readiness probe can
// never pass and its liveness probe eventually kills it.
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
				"pods scheduled there cannot resolve DNS and will hang before mounting. "+
				"Expected when kube-proxy fails to restart on this node; harmless because "+
				"this test runs last and the cluster is torn down next: %v",
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
// server wherever it is scheduled. After `docker start`, kubelet reports the
// node and its pods Ready well before kindnet reinstalls the cross-node
// pod-CIDR routes, and traffic to the restored node is silently blackholed -
// a connect timeout rather than a refusal.
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
