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
	"path"
	"testing"
)

// TestReadPath_L2MissPopulatesAndHits verifies cold-L2 population followed by
// a warm-L2 hit. Remounting between reads clears the local cache; metrics
// distinguish L2 behavior while byte comparisons protect data integrity.
func TestReadPath_L2MissPopulatesAndHits(t *testing.T) {
	// Two 16 MiB chunks exercise multi-chunk population.
	const fileSize = 32 * 1024 * 1024

	testDirName := "dcache_e2e_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	activeMounter.Mount(t)
	t.Cleanup(func() { activeMounter.Unmount(t) })

	t.Logf("driver=%s: reading %s via %s", activeMounter.Kind(), blobPath, activeMounter.Kind())

	// A cold read falls back to Azure and populates L2 asynchronously.
	beforeMiss, haveMetrics := scrapeCacheServerMetrics(t)
	firstRead := activeMounter.ReadFile(t, blobPath)

	if len(firstRead) != len(original) {
		t.Fatalf("L2-miss read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if !bytes.Equal(firstRead, original) {
		t.Fatalf("L2-miss read: content mismatch (md5 got=%s want=%s)",
			md5Sum(firstRead), originalMD5)
	}
	t.Logf("L2-miss read: %d bytes, md5 matches", len(firstRead))

	if haveMetrics {
		afterMiss, ok := scrapeCacheServerMetrics(t)
		if !ok {
			t.Log("metrics: post-miss scrape returned no data; skipping populate assertion")
		} else {
			d := deltaCacheMetrics(beforeMiss, afterMiss)
			hits, misses, ratio := hitMissRatio(d)
			t.Logf("L2-miss window: Download hits=%d misses=%d ratio=%.3f, Upload/Success populates=%d (dup-populates=%d)",
				hits, misses, ratio, d.UploadSuccess, d.UploadInvalidTransition)
			if d.UploadSuccess <= 0 {
				t.Errorf("L2-miss read did not populate cache: Upload/Success delta = %d (expected >= 1 per 16 MiB chunk)", d.UploadSuccess)
			}
		}
	} else {
		t.Log("metrics scrape unavailable; skipping populate counter assertion (data integrity assertion still ran)")
	}

	// Drop local blocks so the next read must consult L2.
	activeMounter.Remount(t)

	beforeHit, _ := scrapeCacheServerMetrics(t)
	secondRead := activeMounter.ReadFile(t, blobPath)

	if len(secondRead) != len(original) {
		t.Fatalf("L2-hit read: size mismatch: got %d want %d", len(secondRead), len(original))
	}
	if !bytes.Equal(secondRead, original) {
		t.Fatalf("L2-hit read: content mismatch (md5 got=%s want=%s)",
			md5Sum(secondRead), originalMD5)
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
