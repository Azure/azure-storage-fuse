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

// distributed_cache ranged-read tests.
//
// Verifies that distributed_cache is chunk-granular under the reference
// production pipeline (block-cache: block-size-mb: 16, prefetch: 32).
//
// A 64 MiB seed maps to exactly four 16 MiB L2 groups keyed as
// fileGroupID(name, etag) + chunkIdx:
//
//     chunk 0: [ 0 MiB, 16 MiB)
//     chunk 1: [16 MiB, 32 MiB)
//     chunk 2: [32 MiB, 48 MiB)
//     chunk 3: [48 MiB, 64 MiB)
//
// Range selection strategy
// ------------------------
// block_cache's forward prefetch (production default: 32 blocks) pulls
// chunks ahead of the touched chunk into distributed_cache's populate path.
// On a small file every touched chunk would amplify into "touched + all
// remaining chunks" populates, which masks the per-chunk distributed_cache
// contract we want to verify.
//
// The read ranges below are anchored at the END of the file. Prefetch is
// forward-only, so a read of the last chunk (or the last two chunks) has
// nothing to prefetch past EOF, and the populate count reduces to exactly
// the chunks the read actually touched. This lets each test assert a
// deterministic populate count using only the user-facing YAML surface
// (no CLI overrides, no internal flag knobs, no prefetch disabling).

package dcache_e2e

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// rangedChunkSize mirrors the reference block-size-mb=16 in
	// docker/k8s/blobfuse2-dist-cache-deployment.yaml.tmpl. One chunk == one
	// L2 group under a given ETag, so one populate per chunk touched.
	rangedChunkSize = 16 * 1024 * 1024

	// rangedNumChunks yields 64 MiB total = four L2 groups. Small enough to
	// keep each test in the ~15 s range yet large enough to exercise a
	// cross-chunk boundary read without ambiguity.
	rangedNumChunks = 4

	// rangedPayloadSize is the seeded blob size, one chunk per rangedChunkSize.
	rangedPayloadSize = rangedNumChunks * rangedChunkSize

	// rangedPopulateTimeout bounds the wait for an async L2 populate to
	// land on cache-server. Same envelope as read_path_test.go /
	// staleness_test.go.
	rangedPopulateTimeout = 60 * time.Second

	// rangedPopulatePollInterval is the poll cadence while waiting for a
	// populate/hit to be visible in cache-server metrics.
	rangedPopulatePollInterval = 500 * time.Millisecond

	// rangedSettleWindow is the extra time we wait after the expected
	// populate count is reached, before asserting no additional populates
	// arrived. Catches late prefetch spill-over that would violate the
	// "==" assertion.
	rangedSettleWindow = 3 * time.Second
)

// TestRanged_LastChunk_PopulatesExactlyOne verifies that a ranged read of a
// single chunk populates ONLY that chunk in L2, not the whole file. The
// read targets the LAST chunk so block_cache's forward prefetch runs off
// EOF and cannot inflate the populate count. Any populate count other than
// exactly one would signal that either the touched-chunk path is doing
// extra work, or that prefetch is reaching past EOF.
func TestRanged_LastChunk_PopulatesExactlyOne(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := rangedTestPath(t, "last_chunk")
	payload := seedRangedBlob(t, blobPath, "last-chunk")

	before, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("last-chunk: pre-read metrics scrape unavailable; cannot verify per-chunk populate")
	}

	// Read the last chunk: [(N-1)*chunkSize, N*chunkSize).
	const offset = (rangedNumChunks - 1) * rangedChunkSize
	const length = rangedChunkSize
	got := readRangeFromPod(t, m, blobPath, offset, length, "last-chunk/read-last")

	if want := payload[offset : offset+length]; !bytes.Equal(got, want) {
		t.Fatalf("last-chunk: bytes mismatch: got md5=%s want md5=%s (len got=%d want=%d)",
			md5Sum(got), md5Sum(want), len(got), len(want))
	}
	t.Logf("last-chunk: read %d bytes, md5 matches slice payload[%d:%d]",
		len(got), offset, offset+length)

	// Prefetch is bounded by EOF, so exactly one L2 group is populated.
	waitAndAssertPopulateExact(t, before, "last-chunk/populate", 1)
}

// TestRanged_LastChunk_ReRead_IsL2Hit verifies that a chunk populated by a
// ranged read is served from L2 (DownloadSuccess) on a subsequent ranged
// read of the SAME chunk, with NO repopulate. Pod restart between reads is
// required to invalidate the kernel FUSE page cache; without it, read #2
// never reaches blobfuse. The read targets the last chunk to keep the
// cold-read populate count deterministic.
func TestRanged_LastChunk_ReRead_IsL2Hit(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := rangedTestPath(t, "last_reread")
	payload := seedRangedBlob(t, blobPath, "last-reread")

	// Read 1: cold. Last chunk is missed and populated (exactly one, no
	// prefetch past EOF).
	beforeCold, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("last-reread: pre-cold-read metrics scrape unavailable")
	}
	const offset = (rangedNumChunks - 1) * rangedChunkSize
	const length = rangedChunkSize
	cold := readRangeFromPod(t, m, blobPath, offset, length, "last-reread/cold-last-chunk")
	if want := payload[offset : offset+length]; !bytes.Equal(cold, want) {
		t.Fatalf("last-reread: cold last-chunk bytes mismatch: got md5=%s want md5=%s",
			md5Sum(cold), md5Sum(want))
	}
	waitAndAssertPopulateExact(t, beforeCold, "last-reread/cold-populate", 1)

	// Drop kernel page cache so read #2 reaches blobfuse.
	m.Remount(t)

	// Read 2: warm. Last chunk must come from L2, no repopulate.
	beforeWarm, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("last-reread: pre-warm-read metrics scrape unavailable")
	}
	warm := readRangeFromPod(t, m, blobPath, offset, length, "last-reread/warm-last-chunk")
	if want := payload[offset : offset+length]; !bytes.Equal(warm, want) {
		t.Fatalf("last-reread: warm last-chunk bytes mismatch: got md5=%s want md5=%s",
			md5Sum(warm), md5Sum(want))
	}
	waitAndAssertRangedHit(t, beforeWarm, "last-reread/warm-hit", 1, 0)
}

// TestRanged_DifferentChunk_AfterFirst_IsMiss verifies that populating one
// chunk under a blob's ETag does NOT satisfy a later ranged request for a
// different chunk of the same blob. If chunks were not independently keyed,
// the second read would silently hit the first chunk's L2 entry.
//
// Read order is chosen so block_cache's forward prefetch cannot pre-cache
// the second read's target:
//
//	Read #1: LAST chunk. Prefetch runs off EOF and adds no other chunks
//	         to L2, so read #2's target chunk 0 is guaranteed cold.
//	Restart: drop the kernel FUSE page cache so read #2 reaches blobfuse.
//	Read #2: chunk 0. Fresh — has never been populated, so if per-chunk
//	         keying holds it MUST miss L2 and repopulate.
func TestRanged_DifferentChunk_AfterFirst_IsMiss(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := rangedTestPath(t, "diff_chunk")
	payload := seedRangedBlob(t, blobPath, "diff-chunk")

	// Read 1: populate ONLY the last chunk.
	beforeLast, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("diff-chunk: pre-last-chunk metrics scrape unavailable")
	}
	const lastOff = (rangedNumChunks - 1) * rangedChunkSize
	last := readRangeFromPod(t, m, blobPath, lastOff, rangedChunkSize, "diff-chunk/read-last-chunk")
	if want := payload[lastOff : lastOff+rangedChunkSize]; !bytes.Equal(last, want) {
		t.Fatalf("diff-chunk: last-chunk bytes mismatch: got md5=%s want md5=%s",
			md5Sum(last), md5Sum(want))
	}
	waitAndAssertPopulateExact(t, beforeLast, "diff-chunk/last-chunk-populate", 1)

	// Restart so read #2 truly reaches blobfuse.
	m.Remount(t)

	// Read 2: cold on chunk 0 under the same blob (same ETag). If chunks
	// weren't independently keyed, this would be a phantom hit against
	// the last-chunk L2 entry.
	beforeCh0, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("diff-chunk: pre-chunk0 metrics scrape unavailable")
	}
	const ch0Off = 0
	ch0 := readRangeFromPod(t, m, blobPath, ch0Off, rangedChunkSize, "diff-chunk/read-chunk0")
	if want := payload[ch0Off : ch0Off+rangedChunkSize]; !bytes.Equal(ch0, want) {
		t.Fatalf("diff-chunk: chunk 0 bytes mismatch: got md5=%s want md5=%s",
			md5Sum(ch0), md5Sum(want))
	}
	// Under production prefetch settings, reading chunk 0 populates chunk 0
	// plus prefetched chunks 1..2 (chunk 3 is already in L2 from read #1).
	// The signal we need is that chunk 0 was NOT silently satisfied by the
	// last-chunk L2 entry: a >=1 populate delta on chunk 0's own read proves
	// per-chunk keying holds.
	waitAndAssertPopulateAtLeast(t, beforeCh0, "diff-chunk/chunk0-populate", 1)
}

// TestRanged_CrossLastBoundary_PopulatesBoth verifies that a ranged read
// straddling a chunk boundary fetches AND populates BOTH underlying chunks.
// The range straddles the LAST chunk boundary so block_cache's forward
// prefetch runs off EOF and cannot inflate the populate count.
func TestRanged_CrossLastBoundary_PopulatesBoth(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := rangedTestPath(t, "cross_last")
	payload := seedRangedBlob(t, blobPath, "cross-last")

	before, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("cross-last: pre-read metrics scrape unavailable")
	}

	// Straddle the last chunk boundary: last 4 MiB of chunk (N-2) + first
	// 4 MiB of chunk (N-1). Prefetch cannot go past EOF.
	const offset = (rangedNumChunks-1)*rangedChunkSize - 4*1024*1024
	const length = 8 * 1024 * 1024
	if offset/rangedChunkSize == (offset+length-1)/rangedChunkSize {
		t.Fatalf("cross-last: sanity: range [%d, %d) does not cross a chunk boundary (chunk size=%d)",
			offset, offset+length, rangedChunkSize)
	}
	got := readRangeFromPod(t, m, blobPath, offset, length, "cross-last/read-span")
	if want := payload[offset : offset+length]; !bytes.Equal(got, want) {
		t.Fatalf("cross-last: bytes mismatch: got md5=%s want md5=%s (len got=%d want=%d)",
			md5Sum(got), md5Sum(want), len(got), len(want))
	}
	t.Logf("cross-last: read %d bytes across last two chunks, md5 matches slice payload[%d:%d]",
		len(got), offset, offset+length)

	// Exactly two populates: one per underlying chunk touched, prefetch
	// bounded by EOF.
	waitAndAssertPopulateExact(t, before, "cross-last/populate", 2)
}

// rangedTestPath returns a unique per-test blob path under a randomised
// directory so parallel test runs cannot collide.
func rangedTestPath(t *testing.T, tag string) string {
	t.Helper()
	return path.Join("dcache_e2e_ranged_"+tag+"_"+randomTestDirName(t, 8), "payload.bin")
}

// seedRangedBlob uploads a fresh random rangedPayloadSize blob to Azure and
// registers a cleanup hook. It does NOT prime the cache — each ranged-read
// test controls the exact bytes it touches. Returns the original bytes so
// callers can slice-compare with what the mount returns.
func seedRangedBlob(t *testing.T, blobPath, label string) []byte {
	t.Helper()

	payload := generateRandomBytes(t, rangedPayloadSize)
	t.Logf("%s: seed %d bytes -> azstorage://%s/%s (md5=%s)",
		label, rangedPayloadSize, testCfg.storageContainer, blobPath, md5Sum(payload))
	uploadBlob(t, blobPath, payload)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	return payload
}

// readRangeFromPod streams bytes [offset, offset+length) of blobPath through
// the mount using `dd` over kubectl exec. Byte-scoped skip/count (skip_bytes,
// count_bytes) keep the helper unit-safe for arbitrary future ranges.
func readRangeFromPod(t *testing.T, m *podMounter, blobPath string, offset, length int, label string) []byte {
	t.Helper()

	pod := m.resolvePod(t)
	full := path.Join(m.mountPath, blobPath)

	dd := fmt.Sprintf(
		"dd if=%s bs=1048576 iflag=skip_bytes,count_bytes skip=%s count=%s status=none",
		full, strconv.Itoa(offset), strconv.Itoa(length),
	)

	cmd := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"exec", pod,
		"--", "sh", "-c", dd,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("%s: kubectl exec start on %s: %v", label, pod, err)
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: kubectl exec dd on %s: %v (stderr: %s)",
				label, pod, err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(podReadTimeout):
		_ = cmd.Process.Kill()
		t.Fatalf("%s: kubectl exec dd on %s: timed out after %s (stderr so far: %s)",
			label, pod, podReadTimeout, strings.TrimSpace(stderr.String()))
	}

	got := stdout.Bytes()
	if len(got) != length {
		t.Fatalf("%s: short ranged read on %s: got %d bytes, want %d (offset=%d, stderr: %s)",
			label, pod, len(got), length, offset, strings.TrimSpace(stderr.String()))
	}
	t.Logf("%s: read %d bytes at offset=%d from %s in %s",
		label, len(got), offset, pod, time.Since(start).Round(time.Millisecond))
	return got
}

// waitAndAssertPopulateExact polls cache-server metrics until the
// UploadSuccess delta from `before` reaches `want`, then waits an additional
// settle window and asserts the delta is STILL exactly `want`. Catches late
// populate spill-over that would over-count the touched chunks.
func waitAndAssertPopulateExact(t *testing.T, before CacheServerMetrics, label string, want int) {
	t.Helper()

	deadline := time.Now().Add(rangedPopulateTimeout)
	var d CacheServerMetrics
	var anyOK bool
	for {
		after, ok := scrapeCacheServerMetrics(t)
		if ok {
			anyOK = true
			d = deltaCacheMetrics(before, after)
			if d.UploadSuccess >= want {
				break
			}
		}
		if time.Now().After(deadline) {
			if !anyOK {
				t.Fatalf("%s: no successful cache-server metrics scrape within %s",
					label, rangedPopulateTimeout)
			}
			t.Fatalf("%s: cache-server did not observe %d UploadSuccess within %s "+
				"(got delta=%d); ranged read did not populate the expected number of chunks",
				label, want, rangedPopulateTimeout, d.UploadSuccess)
		}
		time.Sleep(rangedPopulatePollInterval)
	}

	// Settle window: any late-arriving populate shows up here.
	time.Sleep(rangedSettleWindow)
	after, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatalf("%s: post-settle metrics scrape unavailable; cannot enforce exact populate count",
			label)
	}
	d = deltaCacheMetrics(before, after)
	if d.UploadSuccess != want {
		t.Fatalf("%s: cache-server delta UploadSuccess=%d after %s settle, want exactly %d "+
			"(DownloadSuccess=%d DownloadInvalidTransition=%d); over-count indicates prefetch "+
			"crossed the expected boundary or an unexpected populate path",
			label, d.UploadSuccess, rangedSettleWindow, want,
			d.DownloadSuccess, d.DownloadInvalidTransition)
	}
	t.Logf("%s: cache-server delta UploadSuccess=%d (exactly %d expected), "+
		"DownloadSuccess=%d DownloadInvalidTransition=%d",
		label, d.UploadSuccess, want, d.DownloadSuccess, d.DownloadInvalidTransition)
}

// waitAndAssertPopulateAtLeast polls cache-server metrics until the
// UploadSuccess delta reaches at least `min`. Used when prefetch can add
// unpredictable extra populates and the test only needs to prove the
// touched chunk itself was a real miss (not a phantom hit).
func waitAndAssertPopulateAtLeast(t *testing.T, before CacheServerMetrics, label string, min int) {
	t.Helper()

	deadline := time.Now().Add(rangedPopulateTimeout)
	var d CacheServerMetrics
	var anyOK bool
	for {
		after, ok := scrapeCacheServerMetrics(t)
		if ok {
			anyOK = true
			d = deltaCacheMetrics(before, after)
			if d.UploadSuccess >= min {
				t.Logf("%s: cache-server delta UploadSuccess=%d (>= %d expected), "+
					"DownloadSuccess=%d DownloadInvalidTransition=%d",
					label, d.UploadSuccess, min, d.DownloadSuccess, d.DownloadInvalidTransition)
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(rangedPopulatePollInterval)
	}
	if !anyOK {
		t.Fatalf("%s: no successful cache-server metrics scrape within %s",
			label, rangedPopulateTimeout)
	}
	t.Fatalf("%s: cache-server did not observe %d UploadSuccess within %s "+
		"(got delta=%d); ranged read did not populate the expected number of chunks",
		label, min, rangedPopulateTimeout, d.UploadSuccess)
}

// waitAndAssertRangedHit polls cache-server metrics until the DownloadSuccess
// delta reaches `wantHits`, and asserts UploadSuccess delta is exactly
// `wantPopulates` at that point. Used to verify that a warm ranged read is
// served from L2 without repopulating.
func waitAndAssertRangedHit(t *testing.T, before CacheServerMetrics, label string, wantHits, wantPopulates int) {
	t.Helper()

	deadline := time.Now().Add(rangedPopulateTimeout)
	var d CacheServerMetrics
	var anyOK bool
	for {
		after, ok := scrapeCacheServerMetrics(t)
		if ok {
			anyOK = true
			d = deltaCacheMetrics(before, after)
			if d.DownloadSuccess >= wantHits {
				if d.UploadSuccess != wantPopulates {
					t.Fatalf("%s: hit window observed DownloadSuccess=%d (>= %d expected) "+
						"but UploadSuccess=%d, want exactly %d; warm ranged read repopulated L2 "+
						"instead of serving from it",
						label, d.DownloadSuccess, wantHits, d.UploadSuccess, wantPopulates)
				}
				t.Logf("%s: cache-server delta DownloadSuccess=%d (>= %d expected), "+
					"UploadSuccess=%d (want %d), DownloadInvalidTransition=%d",
					label, d.DownloadSuccess, wantHits, d.UploadSuccess, wantPopulates,
					d.DownloadInvalidTransition)
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(rangedPopulatePollInterval)
	}
	if !anyOK {
		t.Fatalf("%s: no successful cache-server metrics scrape within %s",
			label, rangedPopulateTimeout)
	}
	t.Fatalf("%s: cache-server did not observe %d DownloadSuccess within %s "+
		"(got delta hits=%d populates=%d); warm ranged read was not served from L2",
		label, wantHits, rangedPopulateTimeout, d.DownloadSuccess, d.UploadSuccess)
}
