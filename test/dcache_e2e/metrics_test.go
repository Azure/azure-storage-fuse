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
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// metricsScrapeTimeout bounds one kubectl-exec scrape of a single pod.
const metricsScrapeTimeout = 5 * time.Second

// cacheServerContainerName is the container inside a cacheserver pod that
// exposes the Prometheus port.
const cacheServerContainerName = "cacheserver"

// cacheServerRequestCounter is the Tachyon Prometheus counter name.
const cacheServerRequestCounter = "cache_server_request_counter"

// CacheServerMetrics aggregates cache_server_request_counter samples across
// every cacheserver pod, keyed by the labels that distinguish L2 outcomes.
type CacheServerMetrics struct {
	DownloadSuccess           int // L2 hits
	DownloadInvalidTransition int // L2 misses (chunk absent)
	UploadSuccess             int // L2 populates
	UploadInvalidTransition   int // duplicate populates (FILE_EXISTS)
}

// scrapeCacheServerMetrics discovers cacheserver pods via kubectl and scrapes
// each pod's /metrics endpoint via `kubectl exec ... curl` (no port-forward).
// Returns false when scraping is not viable (no pods found).
func scrapeCacheServerMetrics(t *testing.T) (CacheServerMetrics, bool) {
	t.Helper()
	pods, err := scrapePodNames(context.Background())
	if err != nil {
		t.Logf("metrics: pod discovery failed: %v", err)
		return CacheServerMetrics{}, false
	}
	if len(pods) == 0 {
		t.Log("metrics: no cacheserver pods found; skipping scrape")
		return CacheServerMetrics{}, false
	}
	var total CacheServerMetrics
	for _, pod := range pods {
		m, err := scrapeCacheServerPod(context.Background(), pod)
		if err != nil {
			// Silent pods are treated as zero-contribution; tests only assert
			// on aggregate deltas.
			t.Logf("metrics: pod %s scrape failed: %v (treated as zero)", pod, err)
			continue
		}
		total.DownloadSuccess += m.DownloadSuccess
		total.DownloadInvalidTransition += m.DownloadInvalidTransition
		total.UploadSuccess += m.UploadSuccess
		total.UploadInvalidTransition += m.UploadInvalidTransition
	}
	return total, true
}

// scrapePodNames lists cacheserver pod names using the configured selector.
func scrapePodNames(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, metricsScrapeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, testCfg.kubectlBin, "get", "pods",
		"-n", testCfg.cacheserverNamespace,
		"-l", testCfg.cacheserverSelector,
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl get pods: %w (stderr=%s)", err, strings.TrimSpace(errBuf.String()))
	}
	var pods []string
	for _, line := range strings.Split(out.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			pods = append(pods, line)
		}
	}
	return pods, nil
}

// scrapeCacheServerPod runs curl inside the target pod against its own
// /metrics endpoint and parses the response.
func scrapeCacheServerPod(ctx context.Context, pod string) (CacheServerMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, metricsScrapeTimeout)
	defer cancel()
	url := fmt.Sprintf("http://localhost:%d/metrics", testCfg.cacheserverMetricsPort)
	cmd := exec.CommandContext(ctx, testCfg.kubectlBin, "exec",
		"-n", testCfg.cacheserverNamespace, pod,
		"-c", cacheServerContainerName, "--",
		"curl", "-fsS", "--max-time", "3", url,
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return CacheServerMetrics{}, fmt.Errorf("kubectl exec curl: %w (stderr=%s)", err, strings.TrimSpace(errBuf.String()))
	}
	return parseCacheServerMetrics(out.String()), nil
}

// parseCacheServerMetrics extracts labeled cache_server_request_counter
// samples from a Prometheus exposition body.
func parseCacheServerMetrics(body string) CacheServerMetrics {
	var m CacheServerMetrics
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, cacheServerRequestCounter+"{") {
			continue
		}
		val, ok := lastFieldInt(line)
		if !ok {
			continue
		}
		isDownload := strings.Contains(line, `request_type="Download"`)
		isUpload := strings.Contains(line, `request_type="Upload"`)
		if !isDownload && !isUpload {
			continue
		}
		isSuccess := strings.Contains(line, `status="Success"`)
		isInvalidTransition := strings.Contains(line, `status="InvalidTransition"`)
		switch {
		case isDownload && isSuccess:
			m.DownloadSuccess += val
		case isDownload && isInvalidTransition:
			m.DownloadInvalidTransition += val
		case isUpload && isSuccess:
			m.UploadSuccess += val
		case isUpload && isInvalidTransition:
			m.UploadInvalidTransition += val
		}
	}
	return m
}

// lastFieldInt returns the integer value of the trailing field of a
// Prometheus sample line. Fractional Prometheus literals are truncated.
func lastFieldInt(line string) (int, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	s := fields[len(fields)-1]
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f), true
	}
	return 0, false
}

// deltaCacheMetrics returns after minus before, field by field.
func deltaCacheMetrics(before, after CacheServerMetrics) CacheServerMetrics {
	return CacheServerMetrics{
		DownloadSuccess:           after.DownloadSuccess - before.DownloadSuccess,
		DownloadInvalidTransition: after.DownloadInvalidTransition - before.DownloadInvalidTransition,
		UploadSuccess:             after.UploadSuccess - before.UploadSuccess,
		UploadInvalidTransition:   after.UploadInvalidTransition - before.UploadInvalidTransition,
	}
}

// hitMissRatio returns L2 Download hits, misses, and hit-ratio in [0, 1].
// ratio is 0 when hits+misses == 0.
func hitMissRatio(d CacheServerMetrics) (hits, misses int, ratio float64) {
	hits = d.DownloadSuccess
	misses = d.DownloadInvalidTransition
	total := hits + misses
	if total > 0 {
		ratio = float64(hits) / float64(total)
	}
	return hits, misses, ratio
}
