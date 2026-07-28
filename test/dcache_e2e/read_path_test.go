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

// TestReadPath_L2MissPopulatesAndHits is the canonical read-path E2E for
// the dist_cache read-only integration. dist_cache is a read-only L2 layer
// between blobfuse2 and Azure Storage: writes never traverse it, so the
// mount is opened --read-only=true and all seeding happens out-of-band via
// the Azure SDK. The test walks through the two dist_cache states we care
// about for a given file:
//
//  1. Cold L2, cold local cache: read must fall through to azstorage and
//     asynchronously populate L2 with the fetched chunks.
//  2. Warm L2, cold local cache: read must hit L2 (no download from
//     azstorage), and the returned bytes must be byte-identical to what we
//     originally uploaded via the SDK.
//
// Data integrity is asserted at each stage by MD5 of the read bytes vs the
// original payload. Cache behaviour is asserted by diffing Tachyon
// cacheserver Prometheus counters between snapshots -- an upload-family
// counter must move on the L2-miss read, and a download-family counter
// must move on the L2-hit read. When metrics are not wired up (local
// iteration), the metric assertions are logged as skipped and the
// data-integrity assertions still run, so the test remains useful without
// the full pipeline.
//
// The test *owns* the mount lifecycle: it remounts between reads so that
// block_cache's in-memory blocks (which would otherwise serve read #2 from
// RAM without ever touching dist_cache) are dropped. This is what actually
// distinguishes an L2-hit read from a same-handle in-memory read.
func TestReadPath_L2MissPopulatesAndHits(t *testing.T) {
	// 32 MiB exercises multi-chunk paths in the block_cache + dist_cache
	// config shipped in testdata/config/azure_key_dist_cache_block_e2e.yaml
	// (chunk-size-mb=16). Anything smaller than one chunk defeats the
	// multi-chunk L2-populate assertion we care about.
	const fileSize = 32 * 1024 * 1024

	// ------------------------------------------------------------------
	// Step 1: seed the payload directly into azstorage. Reads through
	// the mount will find this blob and their first access will miss L2.
	// ------------------------------------------------------------------
	testDirName := "dcache_e2e_" + randomTestDirName(t, 10)
	// Blob path uses forward slashes regardless of host OS: azstorage
	// object names are '/'-delimited even when the mount presents them
	// on a Linux filesystem.
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	// ------------------------------------------------------------------
	// Step 2: mount blobfuse2 --read-only with dist_cache in the
	// pipeline. Fresh mount -> cold local block_cache -> the first
	// read will *have* to consult dist_cache (which is also cold).
	//
	// In host driver mode this spawns a local blobfuse2 process. In pod
	// driver mode this cycles the Deployment so its container starts
	// with a fresh block_cache (image side-loaded, no pull cost).
	// ------------------------------------------------------------------
	activeMounter.Mount(t)
	t.Cleanup(func() { activeMounter.Unmount(t) })

	t.Logf("driver=%s: reading %s via %s", activeMounter.Kind(), blobPath, activeMounter.Kind())

	// ------------------------------------------------------------------
	// Step 3: first read after mount -- L2 miss. dist_cache fetches
	// from azstorage, returns the bytes to the reader, and (in the
	// background) uploads the fetched chunks to L2.
	// ------------------------------------------------------------------
	beforeMiss, haveMetrics := scrapeMetrics(t)
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
		afterMiss, _ := scrapeMetrics(t)
		if afterMiss == nil {
			t.Log("metrics: post-miss scrape returned no data; skipping populate assertion")
		} else {
			d := delta(beforeMiss, afterMiss)
			// upload-family counters cover Tachyon's set{,_chunk} /
			// upload_bytes / put_chunk emissions. bytes_in is included
			// in case the server counts ingress bytes without a
			// dedicated upload counter.
			uploaded, matched := sumMatching(d, "upload", "put", "set_chunk", "bytes_in", "ingress")
			if uploaded <= 0 {
				logTopDeltas(t, d, 15)
				t.Errorf("L2-miss read did not appear to populate the cache: matching counters delta = %g (%v)",
					uploaded, matched)
			} else {
				t.Logf("L2-miss populate confirmed: matching-counter delta = %g (%v)", uploaded, matched)
			}
		}
	} else {
		t.Log("metrics endpoints not configured; skipping populate counter assertion (data integrity assertion still ran)")
	}

	// ------------------------------------------------------------------
	// Step 4: drop the local cache. block_cache holds cooked blocks in
	// RAM keyed on the (inode, handle) pair; without a remount the next
	// read is very likely served from RAM and never reaches dist_cache.
	// Remounting is the only way to guarantee we exercise the dist_cache
	// hit path. Host driver: unmount + wipe temp + mount. Pod driver:
	// kubectl rollout restart (container FS is fresh on each Ready pod).
	// ------------------------------------------------------------------
	activeMounter.Remount(t)

	// ------------------------------------------------------------------
	// Step 5: second read after remount -- L2 hit. Bytes come from
	// dist_cache, not azstorage. Confirm the bytes returned from L2 are
	// byte-identical to the original upload (no chunking-boundary
	// corruption, no ETag mix-up, no truncation on the wire).
	// ------------------------------------------------------------------
	beforeHit, _ := scrapeMetrics(t)
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
		afterHit, _ := scrapeMetrics(t)
		if afterHit == nil {
			t.Log("metrics: post-hit scrape returned no data; skipping hit assertion")
		} else {
			d := delta(beforeHit, afterHit)
			downloaded, matched := sumMatching(d, "download", "get", "hit", "bytes_out", "egress")
			if downloaded <= 0 {
				logTopDeltas(t, d, 15)
				t.Errorf("L2-hit read did not appear to hit the cache: matching counters delta = %g (%v)",
					downloaded, matched)
			} else {
				t.Logf("L2-hit confirmed: matching-counter delta = %g (%v)", downloaded, matched)
			}
		}
	}
}
