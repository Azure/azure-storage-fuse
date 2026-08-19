//go:build !fuse2

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

package libfuse

import (
	"testing"
	"time"

	cachepolicy "github.com/Azure/azure-storage-fuse/v2/common/cache_policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type kernelListCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *kernelListCacheTestSuite) SetupTest() {
	s.assert = assert.New(s.T())
	// sweepExpired calls fuseFS.InvalidateKernelListCache; set a non-nil instance so
	// the call reaches C (which returns an error safely when g_fuse == NULL) instead
	// of panicking on a nil dereference.
	fuseFS = &Libfuse{}
}

func (s *kernelListCacheTestSuite) TeardownTest() {
	fuseFS = nil
}

func newTestTracker(ttl time.Duration) *kernelListCacheTracker {
	return &kernelListCacheTracker{
		lru:    cachepolicy.NewLRU[string, *dirCacheItem](defaultKernelListCacheMaxSizeMB*1024*1024, dirCacheItemSize),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
}

// ---- trackDir ---------------------------------------------------------------

// TestTrackDirNewPath verifies that a first-time opendir returns false and stores the entry.
func (s *kernelListCacheTestSuite) TestTrackDirNewPath() {
	t := newTestTracker(time.Hour)

	result := t.trackDir("dir/")

	s.assert.False(result)
	s.assert.True(t.lru.Has("dir/"))
}

// TestTrackDirRoot verifies that root ("") is handled identically to any other path.
func (s *kernelListCacheTestSuite) TestTrackDirRoot() {
	t := newTestTracker(time.Hour)

	result := t.trackDir("")

	s.assert.False(result)
	s.assert.True(t.lru.Has(""))
}

// TestTrackDirWithinTTL verifies that a second opendir within TTL returns true.
func (s *kernelListCacheTestSuite) TestTrackDirWithinTTL() {
	t := newTestTracker(time.Hour)
	t.lru.Put("dir/", &dirCacheItem{cachedAt: time.Now()})

	result := t.trackDir("dir/")

	s.assert.True(result)
}

// TestTrackDirWithinTTLDoesNotResetCachedAt verifies that a cache hit does not reset
// cachedAt, which would silently extend the TTL on every access.
func (s *kernelListCacheTestSuite) TestTrackDirWithinTTLDoesNotResetCachedAt() {
	t := newTestTracker(time.Hour)
	original := time.Now().Add(-30 * time.Minute)
	t.lru.Put("dir/", &dirCacheItem{cachedAt: original})

	t.trackDir("dir/")

	item, ok := t.lru.Peek("dir/")
	s.assert.True(ok)
	s.assert.Equal(original, item.cachedAt)
}

// TestTrackDirExpired verifies that an opendir after TTL returns false and resets cachedAt.
func (s *kernelListCacheTestSuite) TestTrackDirExpired() {
	t := newTestTracker(time.Hour)
	t.lru.Put("dir/", &dirCacheItem{cachedAt: time.Now().Add(-2 * time.Hour)})

	before := time.Now()
	result := t.trackDir("dir/")

	s.assert.False(result)
	item, ok := t.lru.Peek("dir/")
	s.assert.True(ok)
	s.assert.False(item.cachedAt.Before(before), "cachedAt must be reset to approximately now")
}

// TestTrackDirUpdatesLastOp verifies that trackDir refreshes the idle-gate timestamp.
func (s *kernelListCacheTestSuite) TestTrackDirUpdatesLastOp() {
	t := newTestTracker(time.Hour)
	before := time.Now().Unix()

	t.trackDir("dir/")

	s.assert.GreaterOrEqual(t.lastOp.Load(), before)
}

// ---- LRU size-cap eviction --------------------------------------------------

// TestTrackDirAfterLRUEviction verifies that when the LRU silently evicts an entry
// due to the size cap, the next trackDir for that path returns false (treated as new)
// rather than a stale cache hit.
func (s *kernelListCacheTestSuite) TestTrackDirAfterLRUEviction() {
	// 1-byte limit forces immediate eviction of every inserted entry.
	t := &kernelListCacheTracker{
		lru:    cachepolicy.NewLRU[string, *dirCacheItem](1, dirCacheItemSize),
		ttl:    time.Hour,
		stopCh: make(chan struct{}),
	}
	t.trackDir("a/")
	t.trackDir("b/") // "a/" evicted at or before this point

	result := t.trackDir("a/") // must be treated as a new path, not a TTL hit

	s.assert.False(result)
}

// ---- sweepExpired -----------------------------------------------------------

// TestSweepExpiredRemovesExpiredEntries verifies that entries past their TTL are
// deleted and entries still within TTL are kept.
func (s *kernelListCacheTestSuite) TestSweepExpiredRemovesExpiredEntries() {
	t := newTestTracker(100 * time.Millisecond)
	t.lru.Put("expired/", &dirCacheItem{cachedAt: time.Now().Add(-time.Hour)})
	t.lru.Put("fresh/", &dirCacheItem{cachedAt: time.Now()})
	t.lastOp.Store(time.Now().Add(-time.Hour).Unix()) // idle

	t.sweepExpired()

	s.assert.False(t.lru.Has("expired/"))
	s.assert.True(t.lru.Has("fresh/"))
	s.assert.Equal(1, t.lru.Len())
}

// TestSweepExpiredKeepsFreshEntries verifies that no entries are removed when all
// are within TTL.
func (s *kernelListCacheTestSuite) TestSweepExpiredKeepsFreshEntries() {
	t := newTestTracker(time.Hour)
	t.lru.Put("a/", &dirCacheItem{cachedAt: time.Now()})
	t.lru.Put("b/", &dirCacheItem{cachedAt: time.Now()})
	t.lastOp.Store(time.Now().Add(-2 * time.Hour).Unix()) // idle

	t.sweepExpired()

	s.assert.Equal(2, t.lru.Len())
}

// TestSweepExpiredSkipsWhenActive verifies the idle gate: sweep is skipped when
// lastOp is within TTL/2, to avoid write-lock contention during active traffic.
func (s *kernelListCacheTestSuite) TestSweepExpiredSkipsWhenActive() {
	t := newTestTracker(time.Hour)
	t.lru.Put("expired/", &dirCacheItem{cachedAt: time.Now().Add(-2 * time.Hour)})
	t.lastOp.Store(time.Now().Unix()) // very recent activity

	t.sweepExpired()

	s.assert.Equal(1, t.lru.Len(), "sweep must be skipped when cache is active")
}

// TestSweepExpiredRunsWhenIdle verifies that the sweep runs and removes expired
// entries when the cache has been idle longer than TTL/2.
func (s *kernelListCacheTestSuite) TestSweepExpiredRunsWhenIdle() {
	t := newTestTracker(time.Hour)
	t.lru.Put("expired/", &dirCacheItem{cachedAt: time.Now().Add(-2 * time.Hour)})
	t.lastOp.Store(time.Now().Add(-2 * time.Hour).Unix()) // idle for 2h >> TTL/2

	t.sweepExpired()

	s.assert.Equal(0, t.lru.Len())
}

// ---- start / stop -----------------------------------------------------------

// TestStartStop verifies that start and stop complete without deadlock.
func (s *kernelListCacheTestSuite) TestStartStop() {
	t := newTestTracker(50 * time.Millisecond)
	t.start()
	t.stop()
}

// TestSweeperGoroutineEvictsEntries is an integration test that runs the sweeper
// goroutine end-to-end and verifies it removes expired entries after a few TTL cycles.
func (s *kernelListCacheTestSuite) TestSweeperGoroutineEvictsEntries() {
	const ttl = 50 * time.Millisecond
	t := newTestTracker(ttl)
	t.lru.Put("dir/", &dirCacheItem{cachedAt: time.Now()})
	// lastOp == 0 bypasses the idle gate so the sweeper always runs.

	t.start()
	defer t.stop()

	time.Sleep(4 * ttl)

	s.assert.Equal(0, t.lru.Len(), "sweeper goroutine must have removed the expired entry")
}

func TestKernelListCacheTestSuite(t *testing.T) {
	suite.Run(t, new(kernelListCacheTestSuite))
}
