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

// podMounter drives a pre-existing blobfuse2 Deployment.
type podMounter struct{}

// podRolloutTimeout includes the deployment's probe delay.
const podRolloutTimeout = 120 * time.Second

// podReadTimeout bounds a mount read through kubectl exec.
const podReadTimeout = 5 * time.Minute

func (*podMounter) Kind() string { return "pod" }

// resolvePod re-resolves the current pod because rollouts change its name.
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

// Mount restarts the Deployment to guarantee a cold local cache.
func (m *podMounter) Mount(t *testing.T) {
	m.Remount(t)
}

// Unmount is a no-op because the suite does not own the Deployment.
func (m *podMounter) Unmount(t *testing.T) {}

// Remount restarts the pod, clearing local caches and open FUSE handles.
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

	// Readiness implies the liveness probe sees the FUSE mount.
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

// ReadFile streams a mount-relative file over binary-safe kubectl exec.
func (m *podMounter) ReadFile(t *testing.T, blobPath string) []byte {
	t.Helper()

	pod := m.resolvePod(t)
	// The destination is always a Linux path inside the container.
	full := path.Join(testCfg.podMountPath, blobPath)

	cmd := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"exec", pod,
		"--", "cat", full,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Bound hangs without consuming the package test timeout.
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
