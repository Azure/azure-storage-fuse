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
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"
)

// podMounter drives a blobfuse2 pod inside a Kubernetes cluster. The pod is
// expected to already be Running (typically deployed from
// docker/k8s/blobfuse2-dist-cache-deployment.yaml). This driver never
// creates or deletes the Deployment — that is the operator's job — it only
// cycles the pod between reads so block_cache in the pod's container starts
// cold.
type podMounter struct{}

// podRolloutTimeout bounds how long we wait for a Deployment rollout to
// finish after Remount. Kept in sync with the livenessProbe on
// blobfuse2-dist-cache-deployment.yaml (initialDelaySeconds=30 +
// periodSeconds=30 + a safety margin for slow FUSE init).
const podRolloutTimeout = 120 * time.Second

// podReadTimeout bounds a single read from the mount via kubectl exec.
// A 32 MiB payload streams in ~1-2s on a local kind cluster; give it a
// generous margin to accommodate slow discovery-URL fallback on the cache
// server side without turning a genuine hang into an eternal wait.
const podReadTimeout = 5 * time.Minute

func (*podMounter) Kind() string { return "pod" }

// resolvePod returns the name of the currently-Ready pod for the target
// Deployment. It re-resolves on every call because rollouts change pod
// names, so caching would be a footgun after Remount.
func (m *podMounter) resolvePod(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"get", "pod",
		"-l", testCfg.podSelector,
		"--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[0].metadata.name}",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: resolve: kubectl get pod: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		t.Fatalf("pod: resolve: no Running pod found for selector %q in namespace %q",
			testCfg.podSelector, testCfg.podNamespace)
	}
	return name
}

// Mount cycles the Deployment so the first read after Mount is guaranteed
// to hit a cold local cache — otherwise a pod that has been up for a while
// may already have prefetched blocks from a prior interactive session,
// which would defeat the L2-miss assertion.
func (m *podMounter) Mount(t *testing.T) {
	m.Remount(t)
}

// Unmount is deliberately a no-op. The Deployment lifecycle is owned by
// the operator; the test only cycles pods within it. Tearing it down on
// every test teardown would surprise anyone running the suite twice in a
// row (image would still be side-loaded, but the config Secret would
// re-apply, etc.).
func (m *podMounter) Unmount(t *testing.T) {}

// Remount triggers a rollout restart and waits for the new pod to become
// Ready. In practice this drops:
//   - block_cache's in-memory blocks (pod restart destroys the container FS)
//   - any file_cache backing store held in the container's overlay
//   - any open FUSE handles from prior reads
//
// which is exactly the guarantee the host driver provides via
// unmount+mount and directory wipe.
func (m *podMounter) Remount(t *testing.T) {
	t.Helper()

	t.Logf("pod: rollout restart deployment/%s in namespace %s",
		testCfg.podDeployment, testCfg.podNamespace)
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"rollout", "restart", "deployment/"+testCfg.podDeployment,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: kubectl rollout restart: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}

	// Block on the new ReplicaSet reaching Ready. This subsumes both the
	// image pull (no-op on kind because we side-load) and blobfuse2's own
	// mount time (the livenessProbe checks /proc/mounts, so Ready implies
	// the FUSE mount is live).
	statusArgs := []string{
		"-n", testCfg.podNamespace,
		"rollout", "status", "deployment/" + testCfg.podDeployment,
		fmt.Sprintf("--timeout=%ds", int(podRolloutTimeout.Seconds())),
	}
	out, err = exec.Command(testCfg.kubectlBin, statusArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: kubectl rollout status: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}
	t.Logf("pod: rollout complete (%s)", strings.TrimSpace(string(out)))
}

// ReadFile streams a mount-relative file out of the pod over
// `kubectl exec ... -- cat`. kubectl's exec transport is binary-safe
// (raw byte streams via SPDY / websocket), so this works for arbitrary
// payloads including random binary content.
func (m *podMounter) ReadFile(t *testing.T, blobPath string) []byte {
	t.Helper()

	pod := m.resolvePod(t)
	// path.Join, not filepath.Join: the destination is a linux path inside
	// the container regardless of the host OS running the test.
	full := path.Join(testCfg.podMountPath, blobPath)

	cmd := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"exec", pod,
		"--", "cat", full,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Enforce a per-read deadline so a stuck FUSE mount in the pod (e.g.
	// dist_cache client hung waiting on a dead cacheserver) does not tie
	// up the entire `go test` timeout budget.
	done := make(chan error, 1)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatalf("pod: kubectl exec start: %v", err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("pod: kubectl exec cat %s: %v (stderr: %s)",
				full, err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(podReadTimeout):
		_ = cmd.Process.Kill()
		t.Fatalf("pod: kubectl exec cat %s: timed out after %s (stderr so far: %s)",
			full, podReadTimeout, strings.TrimSpace(stderr.String()))
	}

	t.Logf("pod: kubectl exec cat %s: %d bytes in %s",
		full, stdout.Len(), time.Since(start).Round(time.Millisecond))
	return stdout.Bytes()
}
