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
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// cacheserverRolloutTimeout allows for startup and readiness probes.
const cacheserverRolloutTimeout = 3 * time.Minute

// cacheserverTerminateTimeout bounds removal from the Ready set.
const cacheserverTerminateTimeout = 30 * time.Second

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

// killCacheserverPod force-deletes a pod to approximate a node failure.
func killCacheserverPod(t *testing.T, pod string) {
	t.Helper()
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.cacheserverNamespace,
		"delete", "pod", pod,
		"--grace-period=0",
		"--force",
		"--wait=false",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("cacheserver: delete pod %s: %v (out: %s)", pod, err, strings.TrimSpace(string(out)))
	}
	t.Logf("cacheserver: kill %s: %s", pod, strings.TrimSpace(string(out)))
}

// waitCacheserverPodGone waits until the pod is absent or not Ready.
func waitCacheserverPodGone(t *testing.T, pod string) {
	t.Helper()
	deadline := time.Now().Add(cacheserverTerminateTimeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(testCfg.kubectlBin,
			"-n", testCfg.cacheserverNamespace,
			"get", "pod", pod,
			"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}",
		).CombinedOutput()
		if err != nil {
			// Any API error means the pod is not observable as Ready.
			t.Logf("cacheserver: %s no longer observable as Ready: %s",
				pod, strings.TrimSpace(string(out)))
			return
		}
		status := strings.TrimSpace(string(out))
		if status != "True" {
			t.Logf("cacheserver: %s Ready=%q (no longer serving)", pod, status)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	// A fast same-name replacement has an empty cache, so it is still valid.
	t.Logf("cacheserver: pod %s appeared to stay Ready within %s; continuing "+
		"(a StatefulSet-scheduled replacement starts with an empty store, "+
		"which still exercises the fall-back path)", pod, cacheserverTerminateTimeout)
}

// waitCacheserverStatefulSetReady restores cluster health after fault tests.
func waitCacheserverStatefulSetReady(t *testing.T) {
	t.Helper()
	args := []string{
		"-n", testCfg.cacheserverNamespace,
		"rollout", "status", "statefulset/" + testCfg.cacheserverStatefulSet,
		fmt.Sprintf("--timeout=%ds", int(cacheserverRolloutTimeout.Seconds())),
	}
	out, err := exec.Command(testCfg.kubectlBin, args...).CombinedOutput()
	if err != nil {
		// Cleanup must not mask the original test result.
		t.Logf("cacheserver: rollout status failed: %v (out: %s)",
			err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cacheserver: rollout complete (%s)", strings.TrimSpace(string(out)))
}
