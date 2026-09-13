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

// distributed_cache staleness / invalidation tests.
//
// Matrix (four tests, all fast, no TTL sleeps):
//
//	                     | same live mount           | after pod restart
//	---------------------+---------------------------+-------------------------------
//	blob overwrite (v2)  | expects v1 (stale)        | expects v2 (fresh + repopulate)
//	blob delete          | expects v1 (cached)       | expects ENOENT
//
// "Same live mount" tests document the caching contract users observe on a
// running pod: attr_cache pins the ObjAttr (including ETag), the kernel FUSE
// page cache is bound to the inode, and blobfuse2 mounts with `kernel_cache`
// without `auto_cache` and never calls `fuse_invalidate_inode` on ETag change.
// So neither an out-of-band overwrite nor an out-of-band delete is observable
// until the caches expire or the mount is dropped.
//
// "After pod restart" tests bypass every layer of the local cache (attr_cache,
// block_cache in-memory, and kernel FUSE) and exercise the distributed_cache
// invalidation invariant directly:
//
//   - fileGroupID(name, etag) versions the L2 group by blob ETag. A read
//     under a new ETag MUST miss the old chunks and repopulate under the
//     new group, so the overwrite scenario returns v2 (not v1).
//   - distributed_cache does not proactively evict on delete. Correctness
//     comes from OpenFile -> GetAttr -> 404 -> ENOENT at the azstorage layer,
//     so the delete scenario returns ENOENT (not phantom v1 bytes from L2).

package dcache_e2e

import (
	"path"
	"strings"
	"testing"
	"time"
)

const (
	// stalePayloadSize is a single block_cache chunk under the reference
	// pipeline (block-size-mb: 16), keeping each staleness test fast.
	stalePayloadSize = 16 * 1024 * 1024

	// staleChunkSize mirrors the reference block-size-mb=16, i.e. one
	// UploadSuccess per stalePayloadSize.
	staleChunkSize = 16 * 1024 * 1024

	// stalePopulateTimeout bounds the wait for an async L2 populate to
	// land on cache-server. Matches read_path_test.go.
	stalePopulateTimeout = 60 * time.Second

	// stalePopulatePollInterval is the poll cadence while waiting for a
	// populate to be visible in cache-server metrics.
	stalePopulatePollInterval = 500 * time.Millisecond
)

// TestStaleness_Rewrite_SameMount_ServesStaleV1 documents the same-mount
// contract for an out-of-band overwrite. distributed_cache is NOT expected
// to invalidate on the same live mount because attr_cache pins the ETag,
// kernel FUSE holds the page cache under the same inode, and blobfuse2
// never triggers inode invalidation on ETag change.
func TestStaleness_Rewrite_SameMount_ServesStaleV1(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := stalenessTestPath(t, "rewrite_same")

	_, v1MD5 := seedAndPrime(t, m, blobPath, "same-rewrite")

	// Overwrite in Azure with different content (guaranteed distinct ETag).
	v2 := generateRandomBytes(t, stalePayloadSize)
	v2MD5 := md5Sum(v2)
	if v2MD5 == v1MD5 {
		t.Fatalf("random payloads collided (md5=%s); rerun", v2MD5)
	}
	t.Logf("same-rewrite: overwriting %s with v2 (%d bytes, md5=%s)",
		blobPath, len(v2), v2MD5)
	uploadBlob(t, blobPath, v2)

	// Immediate re-read on the same mount: attr_cache still pins the v1
	// ETag; distributed_cache is queried under fileGroupID(name, v1_etag)
	// and returns v1 chunks.
	got := m.ReadFile(t, blobPath)
	switch md5Sum(got) {
	case v1MD5:
		t.Logf("same-rewrite: re-read returned v1 (stale) as required by the "+
			"attr_cache + kernel_cache contract on the same live mount (md5=%s)", v1MD5)
	case v2MD5:
		t.Fatalf("same-rewrite: re-read returned v2 (fresh) — attr_cache or the " +
			"kernel FUSE cache silently invalidated on ETag change, which is not " +
			"the current mount contract")
	default:
		t.Fatalf("same-rewrite: re-read returned unknown content (md5=%s); want v1=%s or v2=%s",
			md5Sum(got), v1MD5, v2MD5)
	}
}

// TestStaleness_Rewrite_AfterRestart_ServesFreshV2 exercises distributed_cache's
// fileGroupID(name, etag) versioning end-to-end. Restart drops attr_cache, the
// block_cache in-memory state, and the kernel FUSE page cache, so GetAttr goes
// to Azure and returns the new (v2) ETag. distributed_cache must miss L2 under
// the new group and populate v2 chunks; the read must return v2 bytes.
func TestStaleness_Rewrite_AfterRestart_ServesFreshV2(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := stalenessTestPath(t, "rewrite_restart")

	_, v1MD5 := seedAndPrime(t, m, blobPath, "restart-rewrite")

	// Overwrite in Azure with different content (guaranteed distinct ETag).
	v2 := generateRandomBytes(t, stalePayloadSize)
	v2MD5 := md5Sum(v2)
	if v2MD5 == v1MD5 {
		t.Fatalf("random payloads collided (md5=%s); rerun", v2MD5)
	}
	t.Logf("restart-rewrite: overwriting %s with v2 (%d bytes, md5=%s)",
		blobPath, len(v2), v2MD5)
	uploadBlob(t, blobPath, v2)

	// Restart to force a fresh GetAttr on next open.
	m.Remount(t)

	// Snapshot metrics so we can assert that the post-restart read
	// repopulated L2 under the new ETag (not just served a stale L1 slice).
	beforeV2, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatal("restart-rewrite: pre-read metrics scrape unavailable; cannot verify v2 populate")
	}

	got := m.ReadFile(t, blobPath)
	switch md5Sum(got) {
	case v2MD5:
		t.Logf("restart-rewrite: post-restart read returned v2 (fresh, md5=%s) as required "+
			"by distributed_cache ETag-keyed group versioning", v2MD5)
	case v1MD5:
		t.Fatalf("restart-rewrite: post-restart read returned v1 (stale) — either " +
			"L2 was consulted under the OLD ETag, or fileGroupID(name, etag) is no " +
			"longer keying on the current ETag. This is a distributed_cache regression")
	default:
		t.Fatalf("restart-rewrite: post-restart read returned unknown content (md5=%s); want v2=%s",
			md5Sum(got), v2MD5)
	}

	// Confirm v2 landed in L2 under its new group.
	waitAndAssertUploadSuccess(t, beforeV2, "restart-rewrite/v2-populate", 1)
}

// TestStaleness_Delete_SameMount_ServesCached documents the same-mount contract
// for an out-of-band delete. Within the caches' TTLs the kernel serves the
// pages it already has without asking FUSE, so the read succeeds with v1 even
// though the blob no longer exists in Azure.
func TestStaleness_Delete_SameMount_ServesCached(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := stalenessTestPath(t, "delete_same")

	v1, v1MD5 := seedAndPrime(t, m, blobPath, "same-delete")

	t.Logf("same-delete: deleting %s from Azure", blobPath)
	deleteBlob(t, blobPath)

	got := m.ReadFile(t, blobPath)
	if len(got) != len(v1) || md5Sum(got) != v1MD5 {
		t.Fatalf("same-delete: immediate re-read returned unexpected content "+
			"(len=%d md5=%s); expected cached v1 (len=%d md5=%s)",
			len(got), md5Sum(got), len(v1), v1MD5)
	}
	t.Logf("same-delete: re-read served cached v1 as required by the attr_cache "+
		"+ kernel_cache contract on the same live mount (md5=%s)", v1MD5)
}

// TestStaleness_Delete_AfterRestart_ReturnsENOENT exercises the delete side of
// distributed_cache's invalidation contract. After restart, GetAttr against
// Azure returns 404 and Open fails with ENOENT before distributed_cache is
// consulted. The read MUST NOT be silently served from orphan L2 chunks that
// physically remain in cache-server memory under the old ETag.
func TestStaleness_Delete_AfterRestart_ReturnsENOENT(t *testing.T) {
	m := newTestPodMounter(t)
	blobPath := stalenessTestPath(t, "delete_restart")

	_, _ = seedAndPrime(t, m, blobPath, "restart-delete")

	t.Logf("restart-delete: deleting %s from Azure", blobPath)
	deleteBlob(t, blobPath)

	m.Remount(t)

	pod := m.resolvePod(t)
	_, err := m.readFileFromPodE(pod, blobPath)
	if err == nil {
		t.Fatalf("restart-delete: read of deleted blob %s succeeded after pod restart — "+
			"distributed_cache appears to have served orphan L2 chunks under the old ETag",
			blobPath)
	}
	if !isENOENTError(err) {
		t.Fatalf("restart-delete: read of deleted blob %s failed with unexpected error: %v; "+
			"expected \"No such file or directory\"", blobPath, err)
	}
	t.Logf("restart-delete: post-restart read of deleted blob %s returned ENOENT as required",
		blobPath)
}

// stalenessTestPath returns a unique per-test blob path under a randomised
// directory so parallel test runs cannot collide on the same blob.
func stalenessTestPath(t *testing.T, tag string) string {
	t.Helper()
	return path.Join("dcache_e2e_stale_"+tag+"_"+randomTestDirName(t, 8), "payload.bin")
}

// seedAndPrime uploads a fresh random payload to Azure, registers a cleanup
// hook, reads it once through the mount to populate distributed_cache, and
// asserts the read matches. Returns the payload bytes and its MD5 so callers
// can assert stale-vs-fresh on subsequent re-reads. The label is used only in
// log output.
func seedAndPrime(t *testing.T, m *podMounter, blobPath, label string) ([]byte, string) {
	t.Helper()

	v1 := generateRandomBytes(t, stalePayloadSize)
	v1MD5 := md5Sum(v1)
	t.Logf("%s: seed v1 %d bytes -> azstorage://%s/%s (md5=%s)",
		label, stalePayloadSize, testCfg.storageContainer, blobPath, v1MD5)
	uploadBlob(t, blobPath, v1)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	before, ok := scrapeCacheServerMetrics(t)
	if !ok {
		t.Fatalf("%s: pre-populate metrics scrape unavailable", label)
	}

	got := m.ReadFile(t, blobPath)
	if len(got) != len(v1) || md5Sum(got) != v1MD5 {
		t.Fatalf("%s: prime read returned unexpected content "+
			"(len=%d md5=%s); expected v1 (len=%d md5=%s)",
			label, len(got), md5Sum(got), len(v1), v1MD5)
	}
	t.Logf("%s: prime read %d bytes, md5 matches", label, len(got))

	waitAndAssertUploadSuccess(t, before, label+"/v1-populate", stalePayloadSize/staleChunkSize)

	return v1, v1MD5
}

// waitAndAssertUploadSuccess polls cache-server metrics until the
// UploadSuccess delta from `before` reaches at least `want`, up to
// stalePopulateTimeout. Fails the test on timeout.
func waitAndAssertUploadSuccess(t *testing.T, before CacheServerMetrics, label string, want int) {
	t.Helper()

	deadline := time.Now().Add(stalePopulateTimeout)
	var d CacheServerMetrics
	var anyOK bool
	for {
		after, ok := scrapeCacheServerMetrics(t)
		if ok {
			anyOK = true
			d = deltaCacheMetrics(before, after)
			if d.UploadSuccess >= want {
				t.Logf("%s: cache-server delta UploadSuccess=%d (>= %d expected)",
					label, d.UploadSuccess, want)
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(stalePopulatePollInterval)
	}
	if !anyOK {
		t.Fatalf("%s: no successful cache-server metrics scrape within %s",
			label, stalePopulateTimeout)
	}
	t.Fatalf("%s: cache-server did not observe %d Upload/Success within %s "+
		"(got delta=%d); populate did not land under the expected ETag group",
		label, want, stalePopulateTimeout, d.UploadSuccess)
}

// isENOENTError reports whether an error returned by readFileFromPodE carries
// kubectl exec's stderr for a missing file. `cat` prints the canonical
// "No such file or directory" for ENOENT on the reference Ubuntu image, and
// the pod-mount helper embeds pod stderr in the returned error string.
func isENOENTError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "No such file or directory")
}
