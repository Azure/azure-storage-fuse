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
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	cachepolicy "github.com/Azure/azure-storage-fuse/v2/common/cache_policy"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// defaultKernelListCacheMaxSizeMB caps the LRU memory usage (~160k entries at avg 200B).
const defaultKernelListCacheMaxSizeMB = 32

type dirCacheItem struct {
	cachedAt time.Time
}

func dirCacheItemSize(key string, _ *dirCacheItem) int64 {
	// 2× for GC overhead, matching the same multiplier used in attr_cache.
	return int64(len(key)+int(unsafe.Sizeof(dirCacheItem{}))) * 2
}

type kernelListCacheTracker struct {
	lru    *cachepolicy.LRU[string, *dirCacheItem]
	ttl    time.Duration
	lastOp atomic.Int64 // Unix seconds of last trackDir; used as idle gate in sweeper
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func newKernelListCacheTracker(ttlSec uint32) *kernelListCacheTracker {
	return &kernelListCacheTracker{
		lru:    cachepolicy.NewLRU[string, *dirCacheItem](defaultKernelListCacheMaxSizeMB*1024*1024, dirCacheItemSize),
		ttl:    time.Duration(ttlSec) * time.Second,
		stopCh: make(chan struct{}),
	}
}

func (t *kernelListCacheTracker) start() {
	t.wg.Add(1)
	go func() { defer t.wg.Done(); t.ttlSweeper() }()
}

func (t *kernelListCacheTracker) stop() {
	close(t.stopCh)
	t.wg.Wait()
}

// trackDir records an opendir for the given directory name in blobfuse internal
// format: "" for root, "dir/" for a subdirectory (libfuse_opendir appends the
// trailing slash; the raw FUSE path has none). Returns whether the cached listing
// is still fresh.
//
// Returns true  → within TTL; caller should set cache_readdir=1, keep_cache=1.
// Returns false → expired or first access; caller should set cache_readdir=1, keep_cache=0
// so the kernel discards any stale listing and fetches fresh data via READDIRPLUS.
//
// When the LRU evicts an entry due to the size cap, fuse_invalidate_path is NOT called.
// This is safe: the next opendir for that path hits !found here, returns false, and the
// caller sets keep_cache=0, causing the kernel to discard any stale listing and call
// READDIRPLUS for fresh data.
func (t *kernelListCacheTracker) trackDir(name string) bool {
	t.lastOp.Store(time.Now().Unix())

	now := time.Now()
	item, found := t.lru.Get(name)
	if !found {
		t.lru.Put(name, &dirCacheItem{cachedAt: now})
		return false
	}
	if now.Sub(item.cachedAt) >= t.ttl {
		t.lru.Put(name, &dirCacheItem{cachedAt: now})
		return false
	}
	return true
}

// ttlSweeper ticks every TTL and evicts expired entries when the cache is idle.
// Mirrors AttrCache.ttlSweeper.
func (t *kernelListCacheTracker) ttlSweeper() {
	ticker := time.NewTicker(t.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.sweepExpired()
		case <-t.stopCh:
			return
		}
	}
}

// sweepExpired removes TTL-expired entries from the LRU and calls fuse_invalidate_path
// for each, evicting stale listings from the kernel's dentry cache for directories
// that have not been accessed since their TTL elapsed.
// The idle gate (identical to AttrCache.sweepExpired) skips the sweep when the cache
// is under active use to avoid write-lock contention with opendir traffic.
func (t *kernelListCacheTracker) sweepExpired() {
	if last := t.lastOp.Load(); last != 0 {
		if time.Now().Unix()-last < int64(t.ttl/(2*time.Second)) {
			return
		}
	}
	ttl := t.ttl
	var expired []string
	t.lru.DeleteIf(func(path string, item *dirCacheItem) bool {
		if time.Since(item.cachedAt) >= ttl {
			expired = append(expired, path)
			return true
		}
		return false
	})
	for _, path := range expired {
		// Proactive optimization: evict the kernel's cached listing for this path so
		// it can reclaim page cache memory sooner.  Correctness does not depend on
		// this call — if it is skipped or fails, the next opendir for the path will
		// have trackDir return false (either !found after LRU eviction, or expired),
		// causing the caller to set keep_cache=0 and the kernel to call READDIRPLUS
		// for fresh data.
		if err := fuseFS.InvalidateKernelListCache("/" + strings.TrimSuffix(path, "/")); err != nil {
			log.Warn("kernelListCacheTracker::sweepExpired : failed to invalidate %s [%s]", path, err)
		}
	}
	log.Debug("kernelListCacheTracker::sweepExpired : evicted %d entries, lru %d MB / %d MB",
		len(expired), t.lru.Size()>>20, t.lru.MaxSize()>>20)
}
