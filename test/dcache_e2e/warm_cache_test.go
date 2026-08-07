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
	"path"
	"testing"
	"time"
)

// TestWarmCache_LateJoinerPodsReadFromL2 seeds L2 with a single "warmer" pod,
// then scales the Deployment up so that fresh pods join with an empty local
// block_cache and must serve the blob from L2 rather than Azure Blob Storage.
//
// The invariant is verified three ways:
//  1. MD5 of the bytes returned by each late-joiner matches the seed data.
//  2. blobfuse2's SDK GET log line for the exact blob path is absent on
//     every late-joiner pod (log evidence, authoritative).
//  3. Cache-server metrics delta over the phase-2 window shows
//     DownloadSuccess > 0 (L2 hits) and UploadSuccess == 0 (no repopulate).
func TestWarmCache_LateJoinerPodsReadFromL2(t *testing.T) {
	// 32 MiB spans two 16 MiB chunks so multi-chunk hits are exercised.
	const fileSize = 32 * 1024 * 1024
	const chunkSize = 16 * 1024 * 1024
	const expectedChunks = fileSize / chunkSize
	const warmerReplicas = 1
	const finalReplicas = 3
	// block_cache dispatches L2 populate asynchronously to the read
	// completing, so we must wait for the warmer's uploads to land on
	// cache-server before scaling up the late-joiners.
	const warmerPopulateTimeout = 60 * time.Second
	const warmerPopulatePollInterval = 500 * time.Millisecond

	testDirName := "dcache_e2e_warm_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	m := newTestPodMounter(t)

	// Fresh Deployment starts at 1 replica with a cold block_cache; that IS
	// the warmer. Scale up in phase 2 below to admit late-joiners.
	warmerPods := m.ListPods(t)
	if len(warmerPods) != warmerReplicas {
		t.Fatalf("warm-cache: expected %d warmer pod, got %d (%v)",
			warmerReplicas, len(warmerPods), warmerPods)
	}
	warmerPod := warmerPods[0]
	t.Logf("warm-cache: warmer pod = %s", warmerPod)

	beforeWarm, haveMetrics := scrapeCacheServerMetrics(t)

	warmerData, err := m.readFileFromPodE(warmerPod, blobPath)
	if err != nil {
		t.Fatalf("warm-cache: warmer read failed: %v", err)
	}
	if len(warmerData) != len(original) {
		t.Fatalf("warm-cache: warmer size mismatch: got %d want %d",
			len(warmerData), len(original))
	}
	if warmerMD5 := md5Sum(warmerData); warmerMD5 != originalMD5 {
		t.Fatalf("warm-cache: warmer content mismatch (md5 got=%s want=%s)",
			warmerMD5, originalMD5)
	}
	t.Logf("warm-cache: warmer read %d bytes, md5 matches", len(warmerData))

	if haveMetrics {
		// Poll until the warmer's async L2 populate has landed on cache-server,
		// otherwise late-joiners may race the warmer and re-populate themselves,
		// flipping both the warmer-phase and late-joiner-phase assertions.
		deadline := time.Now().Add(warmerPopulateTimeout)
		var d CacheServerMetrics
		var lastOK bool
		for {
			after, ok := scrapeCacheServerMetrics(t)
			if ok {
				lastOK = true
				d = deltaCacheMetrics(beforeWarm, after)
				if d.UploadSuccess >= expectedChunks {
					break
				}
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(warmerPopulatePollInterval)
		}
		if !lastOK {
			t.Log("warm-cache: post-warmer metrics scrape returned no data; skipping populate assertion")
		} else {
			t.Logf("warm-cache: warm window delta: UploadSuccess=%d UploadInvalidTransition=%d "+
				"DownloadSuccess=%d DownloadInvalidTransition=%d",
				d.UploadSuccess, d.UploadInvalidTransition,
				d.DownloadSuccess, d.DownloadInvalidTransition)
			if d.UploadSuccess < expectedChunks {
				t.Errorf("warm-cache: warmer did not populate L2 within %s: "+
					"UploadSuccess delta = %d (expected >= %d, one per %d-byte chunk)",
					warmerPopulateTimeout, d.UploadSuccess, expectedChunks, chunkSize)
			}
		}
	} else {
		t.Log("warm-cache: metrics unavailable; skipping populate assertion")
	}

	// Phase 2: add late-joiner pods without disturbing the warmer.
	m.ScaleWaitTo(t, finalReplicas)

	allPods := m.ListPods(t)
	lateJoiners := make([]string, 0, len(allPods))
	for _, p := range allPods {
		if p != warmerPod {
			lateJoiners = append(lateJoiners, p)
		}
	}
	if len(lateJoiners) == 0 {
		t.Fatalf("warm-cache: no late-joiner pods appeared after scale-up "+
			"(all=%v warmer=%s); Deployment may have replaced the warmer",
			allPods, warmerPod)
	}
	t.Logf("warm-cache: %d late-joiner pod(s) will read %s: %v",
		len(lateJoiners), blobPath, lateJoiners)

	beforeHit, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("warm-cache: pre-hit metrics scrape unavailable; cannot measure late-joiner L2 behavior")
	}

	for _, pod := range lateJoiners {
		start := time.Now()
		data, err := m.readFileFromPodE(pod, blobPath)
		if err != nil {
			t.Fatalf("warm-cache: late-joiner %s read failed: %v", pod, err)
		}
		if len(data) != len(original) {
			t.Fatalf("warm-cache: late-joiner %s size mismatch: got %d want %d",
				pod, len(data), len(original))
		}
		if dataMD5 := md5Sum(data); dataMD5 != originalMD5 {
			t.Fatalf("warm-cache: late-joiner %s content mismatch (md5 got=%s want=%s)",
				pod, dataMD5, originalMD5)
		}
		t.Logf("warm-cache: late-joiner %s read %d bytes in %s, md5 matches",
			pod, len(data), time.Since(start).Round(time.Millisecond))
	}

	// Log evidence: any late-joiner hitting Azure means L2 lookup failed.
	azureGETs, err := countAzureGETsForBlob(lateJoiners, blobPath)
	if err != nil {
		t.Fatalf("warm-cache: collect Azure GET evidence: %v", err)
	}
	var azurePods []string
	total := 0
	for _, pod := range lateJoiners {
		n := azureGETs[pod]
		t.Logf("warm-cache: late-joiner %s issued %d Azure GET(s) for %s", pod, n, blobPath)
		if n > 0 {
			azurePods = append(azurePods, pod)
		}
		total += n
	}
	if len(azurePods) > 0 {
		t.Errorf("warm-cache: %d late-joiner pod(s) hit Azure directly (%v, total GETs=%d); "+
			"expected all late-joiners to serve %s from L2",
			len(azurePods), azurePods, total, blobPath)
	}

	afterHit, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Log("warm-cache: post-hit metrics scrape returned no data; skipping L2-hit assertions")
		return
	}
	d := deltaCacheMetrics(beforeHit, afterHit)
	hits, misses, ratio := hitMissRatio(d)
	t.Logf("warm-cache: hit window delta: DownloadSuccess=%d DownloadInvalidTransition=%d "+
		"UploadSuccess=%d UploadInvalidTransition=%d (hits=%d misses=%d ratio=%.3f)",
		d.DownloadSuccess, d.DownloadInvalidTransition,
		d.UploadSuccess, d.UploadInvalidTransition,
		hits, misses, ratio)
	if hits <= 0 {
		t.Errorf("warm-cache: late-joiners produced no L2 hits: DownloadSuccess delta = %d "+
			"(misses=%d)", d.DownloadSuccess, d.DownloadInvalidTransition)
	}
	if d.UploadSuccess > 0 {
		t.Errorf("warm-cache: L2 was repopulated during the hit window: UploadSuccess delta = %d "+
			"(expected 0; late-joiners should have found chunks already present)",
			d.UploadSuccess)
	}
}
