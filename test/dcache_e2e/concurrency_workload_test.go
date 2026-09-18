//go:build !unittest
// +build !unittest

// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"fmt"
	"testing"
	"time"
)

type concurrentReadJob struct {
	pod         string
	blobPath    string
	readerIndex int
}

type concurrentReadResult struct {
	job      concurrentReadJob
	size     int
	checksum string
	duration time.Duration
	err      error
}

// runConcurrentReadWorkload releases all jobs through one barrier. Results
// retain only size and checksum so high fan-out does not hold every payload in
// the test process after validation.
func runConcurrentReadWorkload(t *testing.T, m *podMounter, jobs []concurrentReadJob) []concurrentReadResult {
	t.Helper()
	if len(jobs) == 0 {
		t.Fatal("concurrency workload has no jobs")
	}

	results := make(chan concurrentReadResult, len(jobs))
	barrier := make(chan struct{})
	for _, job := range jobs {
		go func(job concurrentReadJob) {
			<-barrier
			start := time.Now()
			data, err := m.readFileFromPodE(job.pod, job.blobPath)
			result := concurrentReadResult{
				job:      job,
				size:     len(data),
				duration: time.Since(start),
				err:      err,
			}
			if err == nil {
				result.checksum = md5Sum(data)
			}
			results <- result
		}(job)
	}
	close(barrier)

	deadline := time.NewTimer(podReadTimeout + 30*time.Second)
	defer deadline.Stop()

	out := make([]concurrentReadResult, 0, len(jobs))
	for len(out) < len(jobs) {
		select {
		case result := <-results:
			out = append(out, result)
		case <-deadline.C:
			t.Fatalf("concurrency workload timed out after %s (%d/%d reads completed)",
				podReadTimeout+30*time.Second, len(out), len(jobs))
		}
	}
	return out
}

func assertConcurrentReadResults(
	t *testing.T,
	results []concurrentReadResult,
	expected map[string][]byte,
) {
	t.Helper()
	for _, result := range results {
		want, ok := expected[result.job.blobPath]
		if !ok {
			t.Fatalf("unexpected workload path %q", result.job.blobPath)
		}
		if result.err != nil {
			t.Fatalf("pod %s reader %d read %s: %v",
				result.job.pod, result.job.readerIndex, result.job.blobPath, result.err)
		}
		wantMD5 := md5Sum(want)
		if result.size != len(want) || result.checksum != wantMD5 {
			t.Fatalf("pod %s reader %d read %s: got len=%d md5=%s, want len=%d md5=%s",
				result.job.pod, result.job.readerIndex, result.job.blobPath,
				result.size, result.checksum, len(want), wantMD5)
		}
	}
	t.Logf("concurrency workload: all %d reads returned expected bytes", len(results))
}

func fullFileReadJobs(pods, blobPaths []string, readersPerPod int) []concurrentReadJob {
	jobs := make([]concurrentReadJob, 0, len(pods)*len(blobPaths)*readersPerPod)
	for _, pod := range pods {
		for _, blobPath := range blobPaths {
			for reader := 0; reader < readersPerPod; reader++ {
				jobs = append(jobs, concurrentReadJob{
					pod:         pod,
					blobPath:    blobPath,
					readerIndex: reader,
				})
			}
		}
	}
	return jobs
}

func totalAzureGETsForBlob(t *testing.T, pods []string, blobPath string) int {
	t.Helper()
	gets, err := countAzureGETsForBlob(pods, blobPath)
	if err != nil {
		t.Fatalf("collect Azure GET evidence for %s: %v", blobPath, err)
	}
	total := 0
	for pod, count := range gets {
		t.Logf("concurrency: pod %s issued %d Azure GET(s) for %s", pod, count, blobPath)
		total += count
	}
	return total
}

// assertColdAzureGETBound allows readers that reach the three-second
// AlreadyLocked poll deadline to fall back to Azure. L1 coalescing should
// still bound traffic to at most one GET per pod per chunk.
func assertColdAzureGETBound(t *testing.T, pods []string, blobPath string, chunks int) {
	t.Helper()
	got := totalAzureGETsForBlob(t, pods, blobPath)
	minimum := chunks
	maximum := len(pods) * chunks
	if got < minimum || got > maximum {
		t.Fatalf("%s issued %d Azure GETs, want between %d and %d "+
			"(one owner fetch per chunk through one poll-timeout fallback per pod per chunk)",
			blobPath, got, minimum, maximum)
	}
}

func assertNoDuplicatePopulates(t *testing.T, before CacheServerMetrics, label string) CacheServerMetrics {
	t.Helper()
	after, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatalf("%s: post-workload cache-server metrics unavailable", label)
	}
	delta := deltaCacheMetrics(before, after)
	if delta.UploadInvalidTransition != 0 {
		t.Fatalf("%s: observed %d duplicate populate attempt(s)", label, delta.UploadInvalidTransition)
	}
	t.Logf("%s: hits=%d misses=%d uploads=%d duplicate-uploads=%d",
		label, delta.DownloadSuccess, delta.DownloadInvalidTransition,
		delta.UploadSuccess, delta.UploadInvalidTransition)
	return delta
}

func concurrencyBlobPath(t *testing.T, scenario string, index int) string {
	t.Helper()
	return fmt.Sprintf("dcache_e2e_concurrency_%s_%s/file_%02d.bin",
		scenario, randomTestDirName(t, 8), index)
}
