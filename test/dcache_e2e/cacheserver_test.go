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

// cacheserverRolloutTimeout bounds how long we wait for the Tachyon
// StatefulSet to become fully Ready after we deliberately kill one of its
// pods. Kept large enough to accommodate a fresh container start plus the
// cache-server's own readiness/liveness probes.
const cacheserverRolloutTimeout = 3 * time.Minute

// cacheserverTerminateTimeout bounds how long we wait for a deleted
// cache-server pod to actually leave the Ready set. This is deliberately
// short: `kubectl delete --grace-period=0 --force` returns almost
// immediately, and the endpoint drop should follow within a few seconds.
const cacheserverTerminateTimeout = 30 * time.Second

// listCacheserverPods returns the names of every cache-server pod in the
// configured namespace, in the order kubectl reports them. Order is not
// guaranteed to be stable across calls, but it does not need to be: the
// caller picks one pod, and consistent-hashing means any choice exercises
// the same code path.
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

// killCacheserverPod force-deletes the named pod. --grace-period=0 +
// --force skips the SIGTERM window, matching a "node crash" as closely as
// kubectl allows -- the pod object is removed immediately and the
// StatefulSet controller will schedule a replacement. The replacement
// starts with an empty local L2 store, which is precisely what we want
// for a "some chunks fall back to blob" assertion.
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

// waitCacheserverPodGone polls until either the named pod is missing from
// the API, or it exists but is not Ready. Either state is sufficient to
// prove the L2 owner of some chunks is temporarily unreachable. A short
// timeout is fine: `delete --grace-period=0 --force` completes quickly.
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
			// Most likely NotFound -- the pod object has been removed.
			// Anything else (RBAC, transient API error) still counts as
			// "not observable as Ready", which is what we care about.
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
	// Even if the API keeps reporting Ready=True (fast reschedule with the
	// same name is possible under a StatefulSet), the *new* pod's cache
	// store is empty, which still satisfies the "some chunks fall back to
	// blob" invariant. Log and continue.
	t.Logf("cacheserver: pod %s appeared to stay Ready within %s; continuing "+
		"(a StatefulSet-scheduled replacement starts with an empty store, "+
		"which still exercises the fall-back path)", pod, cacheserverTerminateTimeout)
}

// waitCacheserverStatefulSetReady blocks until the StatefulSet reports
// that every replica is Ready again. Used in t.Cleanup so the next test
// in the suite sees a healthy Tachyon cluster.
func waitCacheserverStatefulSetReady(t *testing.T) {
	t.Helper()
	args := []string{
		"-n", testCfg.cacheserverNamespace,
		"rollout", "status", "statefulset/" + testCfg.cacheserverStatefulSet,
		fmt.Sprintf("--timeout=%ds", int(cacheserverRolloutTimeout.Seconds())),
	}
	out, err := exec.Command(testCfg.kubectlBin, args...).CombinedOutput()
	if err != nil {
		// t.Logf, not Fatalf: this runs from t.Cleanup, so failing here
		// would mask the real test outcome. The subsequent test that
		// actually needs cache-server will fail loudly if the cluster is
		// still broken.
		t.Logf("cacheserver: rollout status failed: %v (out: %s)",
			err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cacheserver: rollout complete (%s)", strings.TrimSpace(string(out)))
}
