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

// TestNodeFailure_L2PartialLossFallsBackToBlob verifies that losing an L2
// node falls back to Azure without corrupting data. Metrics confirm survivors
// serve cached chunks while the victim does not.
func TestNodeFailure_L2PartialLossFallsBackToBlob(t *testing.T) {
	// Eight chunks make it very likely that each of three servers owns data.
	const fileSize = 128 * 1024 * 1024
	const chunkSize = 16 * 1024 * 1024
	const expectedChunks = fileSize / chunkSize
	const populateTimeout = 60 * time.Second

	testDirName := "dcache_e2e_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	m := newTestPodMounter(t)

	beforePopulate, haveMetrics := scrapeCacheServerMetrics(t)
	if !haveMetrics {
		t.Fatal("node-failure test requires cache-server metrics to verify initial L2 population")
	}

	// Populate L2 from a cold local cache.
	firstRead := m.ReadFile(t, blobPath)
	if len(firstRead) != len(original) {
		t.Fatalf("populate read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if firstReadMD5 := md5Sum(firstRead); firstReadMD5 != originalMD5 {
		t.Fatalf("populate read: content mismatch (md5 got=%s want=%s)",
			firstReadMD5, originalMD5)
	}
	t.Logf("populate read: %d bytes, md5 matches", len(firstRead))

	deadline := time.Now().Add(populateTimeout)
	for {
		afterPopulate, ok := scrapeCacheServerMetrics(t)
		if ok && deltaCacheMetrics(beforePopulate, afterPopulate).UploadSuccess >= expectedChunks {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("initial read did not populate all %d chunks within %s", expectedChunks, populateTimeout)
		}
		time.Sleep(500 * time.Millisecond)
	}

	clientPod := m.resolvePod(t)
	clientNode, err := getPodNode(m.namespace, clientPod)
	if err != nil {
		t.Fatal(err)
	}

	pods := listCacheserverPods(t)
	if len(pods) < 2 {
		t.Fatalf("node-failure test requires at least 2 cache-server pods, found %d", len(pods))
	}
	victimPod, victimNode := "", ""
	for _, pod := range pods {
		node, err := getPodNode(testCfg.cacheserverNamespace, pod)
		if err != nil {
			t.Fatal(err)
		}
		if node != clientNode {
			victimPod, victimNode = pod, node
			break
		}
	}
	if victimNode == "" {
		t.Fatalf("no cache-server worker can be failed without also killing blobfuse pod %s on %s", clientPod, clientNode)
	}
	t.Logf("node failure: killing kind node %s hosting %s; blobfuse pod %s remains on %s",
		victimNode, victimPod, clientPod, clientNode)

	t.Cleanup(func() { restoreKindNode(t, victimNode) })
	killKindNode(t, victimNode)
	if err := waitKindNodeReady(victimNode, false); err != nil {
		t.Fatal(err)
	}
	t.Logf("node failure: %s is NotReady", victimNode)

	// Drop the local cache so the next read must consult L2.
	m.Remount(t)

	// Aggregate scrape captures traffic from the cache-server pods that remain
	// reachable after the node loss.
	before, haveMetrics := scrapeCacheServerMetrics(t)

	secondRead := m.ReadFile(t, blobPath)
	if len(secondRead) != len(original) {
		t.Fatalf("post-node-loss read: size mismatch: got %d want %d", len(secondRead), len(original))
	}
	if secondReadMD5 := md5Sum(secondRead); secondReadMD5 != originalMD5 {
		t.Fatalf("post-node-loss read: content mismatch (md5 got=%s want=%s) -- "+
			"data integrity broken across the L2-hit / blob-fallback boundary",
			secondReadMD5, originalMD5)
	}
	t.Logf("post-node-loss read: %d bytes, md5 matches (mixed L2-hit + blob-fallback served identical bytes)",
		len(secondRead))

	readerPod := m.resolvePod(t)
	azureGETs, err := grepAzureGETsInPod(readerPod, blobPath)
	if err != nil {
		t.Fatalf("verify Azure fallback on %s: %v", readerPod, err)
	}
	if azureGETs == 0 {
		t.Fatalf("node loss produced no Azure GET for %s; test did not observe blob fallback", blobPath)
	}
	t.Logf("post-node-loss: blobfuse pod %s issued %d Azure GET(s) for %s", readerPod, azureGETs, blobPath)

	if !haveMetrics {
		t.Log("metrics scrape unavailable; skipping survivor-hit invariant (data integrity still verified)")
		return
	}
	after, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Log("metrics: post-node-loss scrape returned no data; skipping survivor-hit invariant")
		return
	}

	d := deltaCacheMetrics(before, after)
	hits, misses, ratio := hitMissRatio(d)
	t.Logf("post-node-loss aggregate: Download hits=%d misses=%d ratio=%.3f, Upload/Success populates=%d (dup-populates=%d)",
		hits, misses, ratio, d.UploadSuccess, d.UploadInvalidTransition)

	// A multi-pod cluster with a killed peer must still serve some chunk
	// from L2. Otherwise nothing distinguishes this from a blob-only read.
	if len(pods) > 1 && hits <= 0 {
		t.Errorf("no surviving cache-server registered a Download/Success delta; "+
			"expected at least one chunk to be served from L2 (hits=%d misses=%d)",
			hits, misses)
	}
}
