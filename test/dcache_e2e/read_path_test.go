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
	beforeMiss, haveMetrics := scrapeCacheServerMetrics(t)
	firstRead := m.ReadFile(t, blobPath)

	if len(firstRead) != len(original) {
		t.Fatalf("L2-miss read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if firstReadMD5 := md5Sum(firstRead); firstReadMD5 != originalMD5 {
		t.Fatalf("L2-miss read: content mismatch (md5 got=%s want=%s)",
			firstReadMD5, originalMD5)
	}
	t.Logf("L2-miss read: %d bytes, md5 matches", len(firstRead))

	if haveMetrics {
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
			t.Log("metrics: post-miss scrape returned no data; skipping populate assertion")
		} else {
			hits, misses, ratio := hitMissRatio(d)
			t.Logf("L2-miss window: Download hits=%d misses=%d ratio=%.3f, Upload/Success populates=%d (dup-populates=%d)",
				hits, misses, ratio, d.UploadSuccess, d.UploadInvalidTransition)
			if d.UploadSuccess < expectedChunks {
				t.Errorf("L2-miss read did not populate cache within %s: Upload/Success delta = %d (expected >= %d, one per %d-byte chunk)",
					populateTimeout, d.UploadSuccess, expectedChunks, chunkSize)
			}
		}
	} else {
		t.Log("metrics scrape unavailable; skipping populate counter assertion (data integrity assertion still ran)")
	}

	// Drop local blocks so the next read must consult L2.
	m.Remount(t)

	beforeHit, _ := scrapeCacheServerMetrics(t)
	secondRead := m.ReadFile(t, blobPath)

	if len(secondRead) != len(original) {
		t.Fatalf("L2-hit read: size mismatch: got %d want %d", len(secondRead), len(original))
	}
	if secondReadMD5 := md5Sum(secondRead); secondReadMD5 != originalMD5 {
		t.Fatalf("L2-hit read: content mismatch (md5 got=%s want=%s)",
			secondReadMD5, originalMD5)
	}
	t.Logf("L2-hit read: %d bytes, md5 matches (data returned from dist_cache is identical to what was uploaded)",
		len(secondRead))

	if haveMetrics {
		afterHit, ok := scrapeCacheServerMetrics(t)
		if !ok {
			t.Log("metrics: post-hit scrape returned no data; skipping hit assertion")
		} else {
			d := deltaCacheMetrics(beforeHit, afterHit)
			hits, misses, ratio := hitMissRatio(d)
			t.Logf("L2-hit window: Download hits=%d misses=%d ratio=%.3f", hits, misses, ratio)
			if hits <= 0 {
				t.Errorf("L2-hit read did not register any Download/Success on cache-server: hits=%d misses=%d", hits, misses)
			}
		}
	}
}
