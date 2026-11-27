//go:build !unittest
// +build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"path"
	"testing"
	"time"
)

// TestReadPath_L2MissPopulatesAndHits verifies cold-L2 population followed by
// a warm-L2 hit. Remounting between reads clears the local cache; metrics
// distinguish L2 behavior while byte comparisons protect data integrity.
func TestReadPath_L2MissPopulatesAndHits(t *testing.T) {
	// Two 16 MiB chunks exercise multi-chunk population.
	const fileSize = 32 * 1024 * 1024
	const chunkSize = 16 * 1024 * 1024
	const expectedChunks = fileSize / chunkSize
	// block_cache dispatches L2 populate asynchronously to the read; poll
	// the metric until it lands so the assertion doesn't race the upload.
	const populateTimeout = 60 * time.Second
	const populatePollInterval = 500 * time.Millisecond

	testDirName := "dcache_e2e_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	m := newTestPodMounter(t)

	// A cold read falls back to Azure and populates L2 asynchronously.
	beforeMiss, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("L2-miss: pre-read cache-server metrics scrape unavailable; cannot verify populate")
	}
	firstRead := m.ReadFile(t, blobPath)

	if len(firstRead) != len(original) {
		t.Fatalf("L2-miss read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if firstReadMD5 := md5Sum(firstRead); firstReadMD5 != originalMD5 {
		t.Fatalf("L2-miss read: content mismatch (md5 got=%s want=%s)",
			firstReadMD5, originalMD5)
	}
	t.Logf("L2-miss read: %d bytes, md5 matches", len(firstRead))

	deadline := time.Now().Add(populateTimeout)
	var d CacheServerMetrics
	var lastOK bool
	for {
		after, ok := scrapeCacheServerMetrics(t)
		if ok {
			lastOK = true
			d = deltaCacheMetrics(beforeMiss, after)
			if d.UploadSuccess >= expectedChunks {
				break
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(populatePollInterval)
	}
	if !lastOK {
		t.Fatalf("L2-miss: no successful cache-server metrics scrape within %s; cannot verify populate", populateTimeout)
	}
	hits, misses, ratio := hitMissRatio(d)
	t.Logf("L2-miss window: Download hits=%d misses=%d ratio=%.3f, Upload/Success populates=%d (dup-populates=%d)",
		hits, misses, ratio, d.UploadSuccess, d.UploadInvalidTransition)
	if d.UploadSuccess < expectedChunks {
		t.Errorf("L2-miss read did not populate cache within %s: Upload/Success delta = %d (expected >= %d, one per %d-byte chunk)",
			populateTimeout, d.UploadSuccess, expectedChunks, chunkSize)
	}

	// Drop local blocks so the next read must consult L2.
	m.Remount(t)

	beforeHit, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("L2-hit: pre-read cache-server metrics scrape unavailable; cannot verify hits")
	}
	secondRead := m.ReadFile(t, blobPath)

	if len(secondRead) != len(original) {
		t.Fatalf("L2-hit read: size mismatch: got %d want %d", len(secondRead), len(original))
	}
	if secondReadMD5 := md5Sum(secondRead); secondReadMD5 != originalMD5 {
		t.Fatalf("L2-hit read: content mismatch (md5 got=%s want=%s)",
			secondReadMD5, originalMD5)
	}
	t.Logf("L2-hit read: %d bytes, md5 matches (data returned from distributed_cache is identical to what was uploaded)",
		len(secondRead))

	afterHit, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("L2-hit: post-read cache-server metrics scrape unavailable; cannot verify hits")
	}
	dHit := deltaCacheMetrics(beforeHit, afterHit)
	hitsHit, missesHit, ratioHit := hitMissRatio(dHit)
	t.Logf("L2-hit window: Download hits=%d misses=%d ratio=%.3f", hitsHit, missesHit, ratioHit)
	if hitsHit <= 0 {
		t.Errorf("L2-hit read did not register any Download/Success on cache-server: hits=%d misses=%d", hitsHit, missesHit)
	}
}
