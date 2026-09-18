//go:build !unittest
// +build !unittest

// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"testing"
)

const (
	concurrencyChunkSize = 16 * 1024 * 1024
	concurrencyFileSize  = 2 * concurrencyChunkSize
)

// TestConcurrency_MultipleFilesAndChunks exercises independent cache keys
// across several files while multiple pods race every chunk.
func TestConcurrency_MultipleFilesAndChunks(t *testing.T) {
	paths, payloads := seedConcurrencyFiles(t, "multi_file", testCfg.concurrencyFiles)
	m := newTestPodMounter(t)
	m.ScaleWaitTo(t, testCfg.concurrencyPods)
	pods := m.ListPods(t)

	before := requireConcurrencyMetrics(t, "multi-file")
	results := runConcurrentReadWorkload(t, m, fullFileReadJobs(pods, paths, 1))
	assertConcurrentReadResults(t, results, payloads)

	expectedChunks := len(paths) * concurrencyFileSize / concurrencyChunkSize
	waitAndAssertPopulateExact(t, before, "multi-file/populate", expectedChunks)
	delta := assertNoDuplicatePopulates(t, before, "multi-file")
	if delta.UploadSuccess != expectedChunks {
		t.Fatalf("multi-file: got %d successful populates, want %d",
			delta.UploadSuccess, expectedChunks)
	}
	for _, blobPath := range paths {
		assertColdAzureGETBound(t, pods, blobPath, concurrencyFileSize/concurrencyChunkSize)
	}
}

// TestConcurrency_SameFileHighContention sends multiple readers per pod
// against the same multi-chunk file. Each chunk must have one population
// winner and all readers must receive identical bytes.
func TestConcurrency_SameFileHighContention(t *testing.T) {
	paths, payloads := seedConcurrencyFiles(t, "same_file", 1)
	blobPath := paths[0]
	m := newTestPodMounter(t)
	m.ScaleWaitTo(t, testCfg.concurrencyPods)
	pods := m.ListPods(t)

	before := requireConcurrencyMetrics(t, "same-file")
	jobs := fullFileReadJobs(pods, paths, testCfg.concurrencyReadersPerPod)
	results := runConcurrentReadWorkload(t, m, jobs)
	assertConcurrentReadResults(t, results, payloads)

	expectedChunks := concurrencyFileSize / concurrencyChunkSize
	waitAndAssertPopulateExact(t, before, "same-file/populate", expectedChunks)
	delta := assertNoDuplicatePopulates(t, before, "same-file")
	if delta.UploadSuccess != expectedChunks {
		t.Fatalf("same-file: got %d successful populates, want %d",
			delta.UploadSuccess, expectedChunks)
	}
	assertColdAzureGETBound(t, pods, blobPath, expectedChunks)
}

// TestConcurrency_MixedHotAndColdReads verifies that warm L2 reads remain
// hits while a different file is populated under concurrent cold contention.
func TestConcurrency_MixedHotAndColdReads(t *testing.T) {
	paths, payloads := seedConcurrencyFiles(t, "mixed", 2)
	hotPath, coldPath := paths[0], paths[1]
	m := newTestPodMounter(t)

	beforeWarm := requireConcurrencyMetrics(t, "mixed/warm")
	hot := m.ReadFile(t, hotPath)
	if len(hot) != len(payloads[hotPath]) || md5Sum(hot) != md5Sum(payloads[hotPath]) {
		t.Fatalf("mixed: warmer returned unexpected bytes")
	}
	expectedChunks := concurrencyFileSize / concurrencyChunkSize
	waitAndAssertPopulateExact(t, beforeWarm, "mixed/warm-populate", expectedChunks)

	// Replace the warmer pod to clear its kernel and local block caches while
	// preserving L2, then add fresh readers.
	m.Remount(t)
	m.ScaleWaitTo(t, testCfg.concurrencyPods)
	pods := m.ListPods(t)

	beforeMixed := requireConcurrencyMetrics(t, "mixed/read")
	results := runConcurrentReadWorkload(t, m, fullFileReadJobs(pods, paths, 1))
	assertConcurrentReadResults(t, results, payloads)
	waitAndAssertPopulateExact(t, beforeMixed, "mixed/cold-populate", expectedChunks)

	delta := assertNoDuplicatePopulates(t, beforeMixed, "mixed")
	if delta.UploadSuccess != expectedChunks {
		t.Fatalf("mixed: cold file produced %d successful populates, want %d",
			delta.UploadSuccess, expectedChunks)
	}
	if delta.DownloadSuccess < len(pods) {
		t.Fatalf("mixed: got %d L2 hits, want at least one hot-file hit per pod (%d)",
			delta.DownloadSuccess, len(pods))
	}
	if got := totalAzureGETsForBlob(t, pods, hotPath); got != 0 {
		t.Fatalf("mixed: hot file unexpectedly issued %d Azure GET(s)", got)
	}
	assertColdAzureGETBound(t, pods, coldPath, expectedChunks)
}

func seedConcurrencyFiles(t *testing.T, scenario string, count int) ([]string, map[string][]byte) {
	t.Helper()
	paths := make([]string, 0, count)
	payloads := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		blobPath := concurrencyBlobPath(t, scenario, i)
		payload := generateRandomBytes(t, concurrencyFileSize)
		uploadBlob(t, blobPath, payload)
		t.Cleanup(func() { deleteBlobBestEffort(t, blobPath) })
		paths = append(paths, blobPath)
		payloads[blobPath] = payload
	}
	return paths, payloads
}

func requireConcurrencyMetrics(t *testing.T, label string) CacheServerMetrics {
	t.Helper()
	before, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatalf("%s: cache-server metrics unavailable", label)
	}
	return before
}
