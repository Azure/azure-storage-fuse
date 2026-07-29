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

// TestNodeFailure_L2PartialLossFallsBackToBlob covers the "one L2 node
// dies mid-life" scenario. The design assertion under test is:
//
//	dist_cache is a *cache*, not a source of truth. Losing a single
//	cache-server pod must never lose data: chunks owned by the dead pod
//	have to fall back to azstorage on the next read, and the returned
//	bytes must remain byte-identical to what was originally uploaded.
//
// Flow (all through the pod driver -- host driver has no way to reach
// cluster-internal dist_cache endpoints):
//
//  1. Seed a multi-chunk payload to azstorage via the SDK. The payload
//     size is chosen to span *many* cache chunks so that consistent
//     hashing is essentially guaranteed to place at least one chunk on
//     every cache-server pod. Any pod we kill therefore owns at least
//     one chunk of the file.
//  2. Mount blobfuse2 (cycles the deployment -> cold local block_cache).
//  3. First read: L2 miss on every chunk. dist_cache serves from
//     azstorage and asynchronously populates L2 across every
//     cache-server pod. MD5 asserted.
//  4. Kill one cache-server pod with `kubectl delete --grace-period=0
//     --force`. The StatefulSet controller will schedule a replacement;
//     the replacement starts with an empty local store. Either way, the
//     chunks that used to live on that pod are no longer retrievable
//     from L2.
//  5. Remount blobfuse2. This drops block_cache so the second read is
//     forced through the dist_cache client. The remount also gives the
//     cluster a moment to reconcile the endpoints list before we issue
//     new reads.
//  6. Second read: L2 hit for chunks whose owner survived, L2 miss for
//     chunks owned by the killed pod. The misses must transparently
//     fall through to azstorage. MD5 asserted end-to-end -- this is the
//     data-integrity guarantee the test exists to protect.
//
// When -dcache-metrics-endpoints is wired up, two further per-endpoint
// invariants are checked on the second read:
//
//   - Survivor invariant: at least one non-victim cache-server endpoint
//     registers hit-family counter growth (`download` / `get` / `hit` /
//     `bytes_out` / `egress`). This proves surviving pods really did
//     serve chunks out of L2 rather than the whole file falling back to
//     blob.
//   - Victim invariant: the killed pod's endpoint registers no
//     hit-family counter growth. Either the port-forward died with the
//     pod, or the StatefulSet-scheduled replacement came back with an
//     empty store -- either way it must not have served cached bytes.
//
// The test also runs in single-pod clusters. There the "partial loss"
// degenerates to "total L2 loss": every chunk falls back to azstorage on
// the second read, and only the victim invariant is checked (no
// survivors to assert on). The MD5 assertion is still the same
// guarantee -- the mount must never surface a corrupt or short read
// regardless of how much of L2 is available.
//
// t.Cleanup waits for the StatefulSet rollout to complete so subsequent
// tests see a healthy Tachyon cluster.
func TestNodeFailure_L2PartialLossFallsBackToBlob(t *testing.T) {
	// 128 MiB @ chunk-size-mb=16 => 8 chunks. With
	// CACHE_SERVER_REPLICAS=3 (see nightly.config), that is enough
	// chunks that a uniformly-hashed distribution is overwhelmingly
	// likely to place a chunk on every cacheserver pod. If the ring is
	// ever tuned larger, bump this up so the invariant still holds.
	const fileSize = 128 * 1024 * 1024

	// ------------------------------------------------------------------
	// Step 1: seed the payload directly into azstorage.
	// ------------------------------------------------------------------
	testDirName := "dcache_e2e_" + randomTestDirName(t, 10)
	blobPath := path.Join(testDirName, "payload.bin")

	original := generateRandomBytes(t, fileSize)
	originalMD5 := md5Sum(original)
	t.Logf("seed %d bytes -> azstorage://%s/%s (md5=%s)",
		fileSize, testCfg.storageContainer, blobPath, originalMD5)
	uploadBlob(t, blobPath, original)
	t.Cleanup(func() { deleteBlob(t, blobPath) })

	// ------------------------------------------------------------------
	// Step 2: mount blobfuse2 with a cold local cache.
	// ------------------------------------------------------------------
	activeMounter.Mount(t)
	t.Cleanup(func() { activeMounter.Unmount(t) })

	// ------------------------------------------------------------------
	// Step 3: first read -- L2 miss on every chunk, populates L2 across
	// all cache-server pods.
	// ------------------------------------------------------------------
	firstRead := activeMounter.ReadFile(t, blobPath)
	if len(firstRead) != len(original) {
		t.Fatalf("populate read: size mismatch: got %d want %d", len(firstRead), len(original))
	}
	if !bytes.Equal(firstRead, original) {
		t.Fatalf("populate read: content mismatch (md5 got=%s want=%s)",
			md5Sum(firstRead), originalMD5)
	}
	t.Logf("populate read: %d bytes, md5 matches", len(firstRead))

	// ------------------------------------------------------------------
	// Step 4: kill one cache-server pod. Any pod is fine -- consistent
	// hashing means each pod owns roughly 1/N of the chunks, so killing
	// any of them guarantees a non-zero L2 loss for this file. With a
	// single-pod cluster this degenerates to "kill the whole L2", which
	// is still a valid data-integrity test: the read must fall entirely
	// back to azstorage and still return byte-identical data.
	//
	// We deliberately pick the first StatefulSet ordinal (`<sts>-0`) so
	// the pod maps to a deterministic index in testCfg.metricsEndpoints:
	// test/scripts/dcache/expose-metrics.sh iterates
	// `kubectl get pods -l app=cacheserver` in the order kubectl returns
	// them, which for a StatefulSet is ordinal order, so endpoint[0] is
	// this pod's scrape URL. That mapping is what lets the per-endpoint
	// metric assertions below distinguish "survivor served L2" from
	// "victim served nothing".
	//
	// We install the StatefulSet-rollout cleanup *before* issuing the
	// delete so that even if the second read fails, we still restore
	// the cluster to a healthy state for subsequent tests.
	// ------------------------------------------------------------------
	pods := listCacheserverPods(t)
	victim := testCfg.cacheserverStatefulSet + "-0"
	victimIdx := 0
	t.Logf("cacheserver: victim pod = %s (endpoint idx %d, of %d total pods: %v)",
		victim, victimIdx, len(pods), pods)

	t.Cleanup(func() { waitCacheserverStatefulSetReady(t) })

	killCacheserverPod(t, victim)
	waitCacheserverPodGone(t, victim)

	// ------------------------------------------------------------------
	// Step 5: remount blobfuse2. This drops the in-container
	// block_cache so the second read is guaranteed to consult
	// dist_cache. The remount also naturally overlaps with the
	// StatefulSet re-creating the killed pod (empty store), which is
	// equivalent to the pod being unreachable from the dist_cache
	// client's perspective for the purposes of this test.
	// ------------------------------------------------------------------
	activeMounter.Remount(t)

	// ------------------------------------------------------------------
	// Step 6: second read -- must succeed AND must return the exact
	// bytes we uploaded. This is the data-integrity assertion: even
	// with one L2 node's chunks unavailable from L2, the mixed
	// L2-hit + azstorage-fallback response must reconstruct the file
	// byte-for-byte.
	//
	// Snapshot per-endpoint metrics around this read so we can also
	// prove *which* server served what: surviving cache-servers must
	// register hit-family movement (they served their share from L2),
	// the victim's endpoint must not (it was down or came back empty).
	// ------------------------------------------------------------------
	beforePerEP, havePerEP := scrapePerEndpoint(t)

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

	if !havePerEP {
		t.Log("metrics endpoints not configured; skipping per-endpoint hit-attribution asserts (data integrity still verified)")
		return
	}
	afterPerEP, _ := scrapePerEndpoint(t)

	// Guard against a mis-sized endpoint list: if the operator wired up
	// fewer endpoints than there are cache-server pods, we can still
	// assert on the endpoints we do have, but we should log so the
	// discrepancy is obvious in CI output.
	if victimIdx >= len(beforePerEP) || victimIdx >= len(afterPerEP) {
		t.Fatalf("metrics endpoint list (len=%d) does not cover victim idx %d; "+
			"check test/scripts/dcache/expose-metrics.sh output",
			len(beforePerEP), victimIdx)
	}

	// Substrings that map to "this server served bytes out of L2". Same
	// list as the aggregate read-path test uses -- kept in one place
	// mentally so a Tachyon metric rename only needs to be tracked once.
	hitTerms := []string{"download", "get", "hit", "bytes_out", "egress"}

	// Survivor invariant: at least one non-victim endpoint must show
	// hit-family growth. In a single-pod cluster there are no survivors
	// -- the read fell back entirely to blob -- so we skip this half of
	// the check and only assert the victim invariant.
	survivorFound := false
	for i := range beforePerEP {
		if i == victimIdx {
			continue
		}
		d := deltaOne(beforePerEP[i], afterPerEP[i])
		hits, matched := sumMatching(d, hitTerms...)
		if hits > 0 {
			survivorFound = true
			t.Logf("survivor endpoint %d (%s): hit-family delta = %g (%v)",
				i, testCfg.metricsEndpoints[i], hits, matched)
		} else {
			t.Logf("survivor endpoint %d (%s): hit-family delta = %g (no matching counters moved)",
				i, testCfg.metricsEndpoints[i], hits)
			logTopDeltas(t, d, 10)
		}
	}
	if len(beforePerEP) > 1 && !survivorFound {
		t.Errorf("no surviving cache-server endpoint registered a hit-family counter delta; "+
			"expected at least one to have served chunks from L2 (victim idx=%d, endpoints=%v)",
			victimIdx, testCfg.metricsEndpoints)
	} else if len(beforePerEP) == 1 {
		t.Log("single-pod cluster: no survivors to check; skipping survivor-hit invariant")
	}

	// Victim invariant: the killed pod's endpoint must not register any
	// hit-family movement. `deltaOne` treats a silent (nil) endpoint as
	// an all-zero delta, so this same check cleanly covers both cases
	// -- the port-forward died with the pod, or the pod was rescheduled
	// with an empty store.
	dVictim := deltaOne(beforePerEP[victimIdx], afterPerEP[victimIdx])
	victimHits, victimMatched := sumMatching(dVictim, hitTerms...)
	if victimHits > 0 {
		logTopDeltas(t, dVictim, 15)
		t.Errorf("killed cache-server endpoint idx %d (%s) registered hit-family delta = %g (%v); "+
			"expected zero (the pod was force-deleted before this read)",
			victimIdx, testCfg.metricsEndpoints[victimIdx], victimHits, victimMatched)
	} else {
		t.Logf("victim endpoint %d (%s): hit-family delta = %g (silent as expected)",
			victimIdx, testCfg.metricsEndpoints[victimIdx], victimHits)
	}
}
