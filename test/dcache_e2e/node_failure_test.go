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

// TestNodeFailure_L2PartialLossFallsBackToBlob verifies that losing an L2
// node falls back to Azure without corrupting data. Metrics confirm survivors
// serve cached chunks while the victim does not.
func TestNodeFailure_L2PartialLossFallsBackToBlob(t *testing.T) {
	// Eight chunks make it very likely that each of three servers owns data.
	const fileSize = 128 * 1024 * 1024

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

	// Populate L2 from a cold local cache.
	firstRead := activeMounter.ReadFile(t, blobPath)
	if len(firstRead) != len(original) {
		t.Fatalf("populate read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if !bytes.Equal(firstRead, original) {
		t.Fatalf("populate read: content mismatch (md5 got=%s want=%s)",
			md5Sum(firstRead), originalMD5)
	}
	t.Logf("populate read: %d bytes, md5 matches", len(firstRead))

	// Ordinal zero is the victim. Register recovery before deletion so a
	// failed assertion cannot leave the cluster unhealthy.
	pods := listCacheserverPods(t)
	victim := testCfg.cacheserverStatefulSet + "-0"
	t.Logf("cacheserver: victim pod = %s (of %d total pods: %v)", victim, len(pods), pods)

	t.Cleanup(func() { waitCacheserverStatefulSetReady(t) })

	killCacheserverPod(t, victim)
	waitCacheserverPodGone(t, victim)

	// Drop the local cache so the next read must consult L2.
	activeMounter.Remount(t)

	// Aggregate scrape captures survivor L2 traffic. The victim's counters
	// are gone with the pod, so per-endpoint attribution would be
	// tautological here.
	before, haveMetrics := scrapeCacheServerMetrics(t)

	secondRead := activeMounter.ReadFile(t, blobPath)
	if len(secondRead) != len(original) {
		t.Fatalf("post-kill read: size mismatch: got %d want %d", len(secondRead), len(original))
	}
	if !bytes.Equal(secondRead, original) {
		t.Fatalf("post-kill read: content mismatch (md5 got=%s want=%s) -- "+
			"data integrity broken across the L2-hit / blob-fallback boundary",
			md5Sum(secondRead), originalMD5)
	}
	t.Logf("post-kill read: %d bytes, md5 matches (mixed L2-hit + blob-fallback served identical bytes)",
		len(secondRead))

	if !haveMetrics {
		t.Log("metrics scrape unavailable; skipping survivor-hit invariant (data integrity still verified)")
		return
	}
	after, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Log("metrics: post-kill scrape returned no data; skipping survivor-hit invariant")
		return
	}

	d := deltaCacheMetrics(before, after)
	hits, misses, ratio := hitMissRatio(d)
	t.Logf("post-kill aggregate: Download hits=%d misses=%d ratio=%.3f, Upload/Success populates=%d (dup-populates=%d)",
		hits, misses, ratio, d.UploadSuccess, d.UploadInvalidTransition)

	// A multi-pod cluster with a killed peer must still serve some chunk
	// from L2. Otherwise nothing distinguishes this from a blob-only read.
	if len(pods) > 1 && hits <= 0 {
		t.Errorf("no surviving cache-server registered a Download/Success delta; "+
			"expected at least one chunk to be served from L2 (hits=%d misses=%d)",
			hits, misses)
	} else if len(pods) == 1 {
		t.Log("single-pod cluster: no survivors to check; skipping survivor-hit invariant")
	}
}
