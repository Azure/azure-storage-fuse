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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// podMounter drives a per-test blobfuse2 Deployment. Every test that needs a
// mount clones the reference Deployment via newTestPodMounter(t), giving
// concurrent tests fully isolated pod fleets.
type podMounter struct {
	namespace   string
	deployment  string
	selector    string
	mountPath   string
	logSequence int
}

const blobfuseDiagnosticLogPath = "/var/log/blobfuse2/blobfuse2-block-logs.txt"

// podRolloutTimeout includes the deployment's probe delay.
const podRolloutTimeout = 120 * time.Second

// scaleWaitTimeout bounds one scale-up or scale-down wait.
const scaleWaitTimeout = 120 * time.Second

// podReadTimeout bounds a mount read through kubectl exec.
const podReadTimeout = 5 * time.Minute

// newTestPodMounter clones the reference Deployment configured in testCfg into
// a per-test Deployment (unique name, unique labels/selector) so tests never
// share pods with each other. The Deployment is torn down in t.Cleanup.
func newTestPodMounter(t *testing.T) *podMounter {
	t.Helper()
	name := uniqueDeploymentName(t)
	m := &podMounter{
		namespace:  testCfg.podNamespace,
		deployment: name,
		selector:   "app=" + name,
		mountPath:  testCfg.podMountPath,
	}
	cloneReferenceDeployment(t, testCfg.podDeployment, name)
	t.Cleanup(func() {
		m.collectBlobfuseLogs(t, "cleanup")
		m.deleteDeployment(t)
	})
	m.WaitDeploymentReady(t)
	m.assertMountReadOnly(t)
	return m
}

// collectBlobfuseLogs copies each live pod's file log to the pipeline artifact
// staging directory. Collection is best-effort and disabled outside CI unless
// DCACHE_LOG_DIR is set explicitly.
func (m *podMounter) collectBlobfuseLogs(t *testing.T, phase string) {
	t.Helper()
	logRoot := os.Getenv("DCACHE_LOG_DIR")
	if logRoot == "" {
		return
	}

	pods, err := m.listLivePodsE()
	if err != nil {
		t.Logf("logs: list blobfuse2 pods before %s: %v", phase, err)
		return
	}

	m.logSequence++
	outDir := filepath.Join(logRoot, "blobfuse2", m.deployment)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Logf("logs: create artifact directory %s: %v", outDir, err)
		return
	}

	for _, pod := range pods {
		out, err := exec.Command(testCfg.kubectlBin,
			"-n", m.namespace,
			"exec", pod,
			"--", "cat", blobfuseDiagnosticLogPath,
		).CombinedOutput()
		fileName := fmt.Sprintf("%02d-%s-%s.log", m.logSequence, phase, pod)
		filePath := filepath.Join(outDir, fileName)
		if err != nil {
			filePath += ".error"
			t.Logf("logs: collect blobfuse2 log from %s before %s: %v", pod, phase, err)
		}
		if writeErr := os.WriteFile(filePath, out, 0644); writeErr != nil {
			t.Logf("logs: write %s: %v", filePath, writeErr)
		}
	}
}

// uniqueDeploymentName returns a DNS-1123 label derived from t.Name plus a
// random suffix, capped at 63 characters.
func uniqueDeploymentName(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, r := range strings.ToLower(t.Name()) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	base := "b2-dcache-" + strings.Trim(b.String(), "-")
	suffix := "-" + randomTestDirName(t, 6)
	const dnsLabelMax = 63
	if len(base)+len(suffix) > dnsLabelMax {
		base = base[:dnsLabelMax-len(suffix)]
		base = strings.TrimRight(base, "-")
	}
	return base + suffix
}

// cloneReferenceDeployment fetches the reference Deployment as JSON, rewrites
// its name / labels / selector to newName, strips server-managed metadata,
// resets replicas to 1, and applies the result. Reuses the shared Secret and
// hostPath volumes in the same namespace.
func cloneReferenceDeployment(t *testing.T, refName, newName string) {
	t.Helper()

	raw, err := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"get", "deployment", refName,
		"-o", "json",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("clone: get reference deployment %s: %v (out: %s)",
			refName, err, strings.TrimSpace(string(raw)))
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("clone: unmarshal reference deployment: %v", err)
	}

	meta, _ := obj["metadata"].(map[string]any)
	if meta == nil {
		t.Fatalf("clone: reference deployment has no metadata")
	}
	for _, k := range []string{"uid", "resourceVersion", "generation", "creationTimestamp", "managedFields", "ownerReferences", "annotations"} {
		delete(meta, k)
	}
	meta["name"] = newName
	labels, _ := meta["labels"].(map[string]any)
	if labels == nil {
		labels = map[string]any{}
		meta["labels"] = labels
	}
	labels["app"] = newName

	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		t.Fatalf("clone: reference deployment has no spec")
	}
	spec["replicas"] = 1
	if sel, ok := spec["selector"].(map[string]any); ok {
		if ml, ok := sel["matchLabels"].(map[string]any); ok {
			ml["app"] = newName
		}
	}
	if tmpl, ok := spec["template"].(map[string]any); ok {
		if tmeta, ok := tmpl["metadata"].(map[string]any); ok {
			if tlabels, ok := tmeta["labels"].(map[string]any); ok {
				tlabels["app"] = newName
			}
		}
	}
	delete(obj, "status")

	body, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("clone: marshal patched deployment: %v", err)
	}

	apply := exec.Command(testCfg.kubectlBin, "apply", "-f", "-")
	apply.Stdin = bytes.NewReader(body)
	if out, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("clone: apply deployment %s: %v (out: %s)",
			newName, err, strings.TrimSpace(string(out)))
	}
	t.Logf("clone: created deployment %s (cloned from %s)", newName, refName)
}

// deleteDeployment tears the per-test Deployment down; best-effort so a
// cleanup failure never masks a real test result.
func (m *podMounter) deleteDeployment(t *testing.T) {
	t.Helper()
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"delete", "deployment", m.deployment,
		"--wait=false",
		"--ignore-not-found",
	).CombinedOutput()
	if err != nil {
		t.Logf("cleanup: delete deployment %s: %v (out: %s)",
			m.deployment, err, strings.TrimSpace(string(out)))
		return
	}
	t.Logf("cleanup: delete deployment %s: %s", m.deployment, strings.TrimSpace(string(out)))
}

// resolvePod returns a live (non-terminating) Running pod. During a rollout,
// both new and old pods can report status.phase=Running, so the terminating
// filter is essential — reading from the old pod would serve stale block_cache.
func (m *podMounter) resolvePod(t *testing.T) string {
	t.Helper()
	pods, err := m.listLivePodsE()
	if err != nil {
		t.Fatalf("pod: resolve: %v", err)
	}
	if len(pods) == 0 {
		t.Fatalf("pod: resolve: no live pod found for selector %q in namespace %q",
			m.selector, m.namespace)
	}
	return pods[0]
}

func (m *podMounter) assertMountReadOnly(t *testing.T) {
	t.Helper()
	pod := m.resolvePod(t)
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"exec", pod,
		"--", "cat", "/proc/mounts",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: inspect mount options on %s: %v (out: %s)",
			pod, err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != m.mountPath {
			continue
		}
		for _, option := range strings.Split(fields[3], ",") {
			if option == "ro" {
				t.Logf("pod: verified read-only mount %s on %s", m.mountPath, pod)
				return
			}
		}
		t.Fatalf("pod: mount %s on %s is not read-only (options: %s)",
			m.mountPath, pod, fields[3])
	}
	t.Fatalf("pod: mount %s not found in /proc/mounts on %s", m.mountPath, pod)
}

// Remount restarts the pod, clearing local caches and open FUSE handles.
func (m *podMounter) Remount(t *testing.T) {
	t.Helper()
	m.collectBlobfuseLogs(t, "before-remount")

	t.Logf("pod: rollout restart deployment/%s in namespace %s",
		m.deployment, m.namespace)
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"rollout", "restart", "deployment/"+m.deployment,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: kubectl rollout restart: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}

	// Readiness implies the readiness probe sees the FUSE mount.
	statusArgs := []string{
		"-n", m.namespace,
		"rollout", "status", "deployment/" + m.deployment,
		fmt.Sprintf("--timeout=%ds", int(podRolloutTimeout.Seconds())),
	}
	out, err = exec.Command(testCfg.kubectlBin, statusArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: kubectl rollout status: %v (out: %s)", err, strings.TrimSpace(string(out)))
	}
	t.Logf("pod: rollout complete (%s)", strings.TrimSpace(string(out)))

	// rollout status returns once the new pods are Ready, but the old pods may
	// still be terminating (phase=Running with a deletionTimestamp). Wait until
	// only live pods remain so resolvePod cannot pick a Terminating pod whose
	// block_cache is still warm.
	m.WaitDeploymentReady(t)
}

// ReadFile streams a mount-relative file over binary-safe kubectl exec.
func (m *podMounter) ReadFile(t *testing.T, blobPath string) []byte {
	t.Helper()

	pod := m.resolvePod(t)
	start := time.Now()
	data, err := m.readFileFromPodE(pod, blobPath)
	if err != nil {
		t.Fatalf("pod: %v", err)
	}
	t.Logf("pod: read %s from %s: %d bytes in %s",
		blobPath, pod, len(data), time.Since(start).Round(time.Millisecond))
	return data
}

// ListPods returns every Running pod backing the Deployment, excluding
// pods that are terminating (they still report status.phase=Running until
// their grace period ends but no longer count against the Deployment).
func (m *podMounter) ListPods(t *testing.T) []string {
	t.Helper()
	pods, err := m.listLivePodsE()
	if err != nil {
		t.Fatalf("pod: list: %v", err)
	}
	if len(pods) == 0 {
		t.Fatalf("pod: list: no live pods for selector %q in namespace %q",
			m.selector, m.namespace)
	}
	return pods
}

// listLivePodsE returns live (non-terminating) Running pod names, or an error.
func (m *podMounter) listLivePodsE() ([]string, error) {
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"get", "pod",
		"-l", m.selector,
		"--field-selector=status.phase=Running",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.deletionTimestamp}{"\n"}{end}`,
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get pod: %w (out: %s)", err, strings.TrimSpace(string(out)))
	}
	var pods []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, delTS, _ := strings.Cut(line, "\t")
		if strings.TrimSpace(delTS) != "" {
			continue
		}
		if name = strings.TrimSpace(name); name != "" {
			pods = append(pods, name)
		}
	}
	return pods, nil
}

// ConcurrentReadFile fires one goroutine per pod, released together through
// a barrier so their reads start within scheduler jitter of each other.
func (m *podMounter) ConcurrentReadFile(t *testing.T, pods []string, blobPath string) map[string][]byte {
	t.Helper()
	type result struct {
		pod  string
		data []byte
		err  error
	}
	resCh := make(chan result, len(pods))
	barrier := make(chan struct{})
	for _, pod := range pods {
		go func(p string) {
			<-barrier
			data, err := m.readFileFromPodE(p, blobPath)
			resCh <- result{p, data, err}
		}(pod)
	}
	close(barrier)

	out := make(map[string][]byte, len(pods))
	for range pods {
		r := <-resCh
		if r.err != nil {
			t.Fatalf("pod: concurrent read: %v", r.err)
		}
		out[r.pod] = r.data
	}
	return out
}

// Error-returning variant; safe to call from goroutines (no t.Fatal).
func (m *podMounter) readFileFromPodE(pod, blobPath string) ([]byte, error) {
	full := path.Join(m.mountPath, blobPath)

	cmd := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"exec", pod,
		"--", "cat", full,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kubectl exec start on %s: %w", pod, err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("kubectl exec cat %s on %s: %w (stderr: %s)",
				full, pod, err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(podReadTimeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("kubectl exec cat %s on %s: timed out after %s (stderr so far: %s)",
			full, pod, podReadTimeout, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}

// ScaleWaitTo scales the Deployment and blocks until it is ready.
func (m *podMounter) ScaleWaitTo(t *testing.T, replicas int) {
	t.Helper()
	if pods, err := m.listLivePodsE(); err == nil && replicas < len(pods) {
		m.collectBlobfuseLogs(t, "before-scale-down")
	}
	t.Logf("pod: scale deployment/%s to %d replicas", m.deployment, replicas)
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"scale", "deployment/"+m.deployment,
		fmt.Sprintf("--replicas=%d", replicas),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: kubectl scale to %d: %v (out: %s)",
			replicas, err, strings.TrimSpace(string(out)))
	}
	m.WaitDeploymentReady(t)
}

// WaitDeploymentReady blocks until .status.readyReplicas == .spec.replicas
// AND no terminating pods remain (their kube-scheduler grace can outlive the
// Deployment's status transition).
func (m *podMounter) WaitDeploymentReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(scaleWaitTimeout)
	for {
		out, err := exec.Command(testCfg.kubectlBin,
			"-n", m.namespace,
			"get", "deployment", m.deployment,
			"-o", `jsonpath={.spec.replicas} {.status.readyReplicas} {.status.replicas}`,
		).CombinedOutput()
		if err != nil {
			t.Fatalf("pod: wait ready: kubectl get deployment: %v (out: %s)",
				err, strings.TrimSpace(string(out)))
		}
		fields := strings.Fields(strings.TrimSpace(string(out)))
		spec := fieldOrZero(fields, 0)
		ready := fieldOrZero(fields, 1)
		total := fieldOrZero(fields, 2)

		livePods, listErr := m.listLivePodsE()
		live := len(livePods)

		if listErr == nil && spec == ready && total == spec && live == spec {
			t.Logf("pod: deployment ready at replicas=%d (live pods=%d)", spec, live)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pod: deployment not ready after %s (spec=%d ready=%d total=%d live=%d listErr=%v)",
				scaleWaitTimeout, spec, ready, total, live, listErr)
		}
		time.Sleep(2 * time.Second)
	}
}

// fieldOrZero returns fields[i] as int, or 0 when the field is missing/empty.
// A missing readyReplicas serialises as an absent jsonpath field.
func fieldOrZero(fields []string, i int) int {
	if i >= len(fields) {
		return 0
	}
	n, err := strconv.Atoi(fields[i])
	if err != nil {
		return 0
	}
	return n
}
