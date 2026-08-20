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
	"strconv"
	"strings"
	"testing"
	"time"
)

// Kept in sync with docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl.
const blobfuseLogPath = "/var/log/blobfuse2/blobfuse2-block-logs.txt"

const azureGETLogGrep = 15 * time.Second

// stampedeReplicas is the fleet size raced against a single Azure GET.
const stampedeReplicas = 3

// Concurrent cold reads from N pods must produce exactly one Azure GET:
// one pod wins GotLock and populates L2, others see AlreadyLocked and poll.
func TestStampedePrevention_ConcurrentColdReadCoalesces(t *testing.T) {
	// One 16 MiB read == one chunk at block-size-mb=16 == one expected Upload.
	const fileSize = 16 * 1024 * 1024

	testDirName := "dcache_e2e_stampede_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	m := newTestPodMounter(t)

	// Fresh Deployment starts at 1 replica with a cold cache; scale up to the
	// racer count directly.
	m.ScaleWaitTo(t, stampedeReplicas)

	pods := m.ListPods(t)
	if len(pods) < 2 {
		t.Fatalf("stampede test requires >= 2 blobfuse pods after scale-out, found %d (%v)",
			len(pods), pods)
	}
	t.Logf("stampede: %d blobfuse pods will race on %s: %v", len(pods), blobPath, pods)

	before, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("stampede: pre-read cache-server metrics scrape unavailable; cannot verify UploadSuccess invariant")
	}

	results := m.ConcurrentReadFile(t, pods, blobPath)

	for _, pod := range pods {
		data := results[pod]
		if len(data) != len(original) {
			t.Fatalf("pod %s: size mismatch: got %d want %d", pod, len(data), len(original))
		}
		if dataMD5 := md5Sum(data); dataMD5 != originalMD5 {
			t.Fatalf("pod %s: content mismatch (md5 got=%s want=%s)",
				pod, dataMD5, originalMD5)
		}
	}
	t.Logf("stampede: all %d pods returned identical bytes (md5=%s)", len(pods), originalMD5)

	// Log evidence attributes Azure traffic to a specific pod for this exact blob.
	gets, err := countAzureGETsForBlob(pods, blobPath)
	if err != nil {
		t.Fatalf("stampede: collect Azure GET evidence: %v", err)
	}
	var azurePods []string
	total := 0
	for _, pod := range pods {
		n := gets[pod]
		t.Logf("stampede: pod %s issued %d Azure GET(s) for %s", pod, n, blobPath)
		if n > 0 {
			azurePods = append(azurePods, pod)
		}
		total += n
	}
	switch {
	case len(azurePods) == 0:
		t.Errorf("stampede: no pod recorded an Azure GET for %s; log evidence is missing "+
			"(is BLOBFUSE_DISABLE_SDK_LOG set? is logging.level=log_debug?)", blobPath)
	case len(azurePods) > 1:
		t.Errorf("stampede: expected exactly 1 pod to hit Azure, got %d (%v); "+
			"the other pods should have observed AlreadyLocked and waited on L2. "+
			"Total Azure GET attempts across fleet: %d",
			len(azurePods), azurePods, total)
	default:
		t.Logf("stampede: exactly 1 pod (%s) hit Azure for %s; other %d pod(s) served from L2",
			azurePods[0], blobPath, len(pods)-1)
	}

	after, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("stampede: post-read cache-server metrics scrape unavailable; cannot verify UploadSuccess invariant")
	}
	d := deltaCacheMetrics(before, after)
	hits, misses, ratio := hitMissRatio(d)
	t.Logf("stampede: cache-server delta: DownloadSuccess=%d DownloadInvalidTransition=%d "+
		"UploadSuccess=%d UploadInvalidTransition=%d (hits=%d misses=%d ratio=%.3f)",
		d.DownloadSuccess, d.DownloadInvalidTransition,
		d.UploadSuccess, d.UploadInvalidTransition,
		hits, misses, ratio)
	if d.UploadSuccess != 1 {
		t.Errorf("stampede: expected exactly 1 successful populate, got UploadSuccess=%d "+
			"(one 16 MiB read == one chunk == one Upload)", d.UploadSuccess)
	}
	if d.UploadInvalidTransition > 0 {
		t.Errorf("stampede: %d duplicate populate(s) rejected by cache-server; "+
			"lock protocol did not prevent concurrent Upload attempts",
			d.UploadInvalidTransition)
	}
}

// Returns pod-name -> Azure GET count for blobPath.
func countAzureGETsForBlob(pods []string, blobPath string) (map[string]int, error) {
	out := make(map[string]int, len(pods))
	for _, pod := range pods {
		n, err := grepAzureGETsInPod(pod, blobPath)
		if err != nil {
			return nil, fmt.Errorf("pod %s: %w", pod, err)
		}
		out[pod] = n
	}
	return out, nil
}

// Counts `SDK(Retry) : =====> Try=N for GET <url containing blobPath>` lines
// in the pod's log for any retry number N.
func grepAzureGETsInPod(pod, blobPath string) (int, error) {
	// The SDK URL-encodes '/' as '%2F' in blob names, so match the encoded form.
	encodedBlobPath := strings.ReplaceAll(blobPath, "/", "%2F")
	// `wc -l` always exits 0 with a single integer, avoiding the `grep -c`
	// quirk of returning "0" with a non-zero exit code on empty input.
	shell := fmt.Sprintf(
		`grep -E %s %s 2>/dev/null | grep -F %s 2>/dev/null | wc -l`,
		shellSingleQuote(`SDK\(Retry\) : =====> Try=[0-9]+ for GET`),
		blobfuseLogPath,
		shellSingleQuote(encodedBlobPath),
	)

	cmd := exec.Command(testCfg.kubectlBin,
		"-n", testCfg.podNamespace,
		"exec", pod,
		"--", "sh", "-c", shell,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("kubectl exec start on %s: %w", pod, err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return 0, fmt.Errorf("kubectl exec grep on %s: %w (stderr: %s)",
				pod, err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(azureGETLogGrep):
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("kubectl exec grep on %s: timed out after %s",
			pod, azureGETLogGrep)
	}

	line := strings.TrimSpace(stdout.String())
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0, fmt.Errorf("parse grep count %q on %s: %w", line, pod, err)
	}
	return n, nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
