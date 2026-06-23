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

package attr_cache

import (
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	cachepolicy "github.com/Azure/azure-storage-fuse/v2/common/cache_policy"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// attrCacheItem : Structure of each item in attr cache
type attrCacheItem struct {
	attr     *internal.ObjAttr
	cachedAt time.Time
	exists   bool // true if the entry exists in storage; false marks a negative cache entry (path confirmed absent)
}

func (value *attrCacheItem) isNegativeEntry() bool {
	return !value.exists
}

// positiveEntrySize is the approximate heap cost of a positive (non-tombstone) cache entry
// with a typical path (52 bytes), ETag (20 bytes), and MD5 (16 bytes), after GOGC doubling.
// Used to convert a legacy max-files count into an equivalent max-size-mb value.
const positiveEntrySize int64 = 964

// estimateAttrCacheEntrySize estimates the heap bytes for one attr-cache key-value pair.
// The result is doubled to account for Go's GOGC=100 behaviour where RSS ≈ 2× live heap.
//
// Typical sizes (52-byte path, 20-byte ETag, 16-byte MD5):
//   - negative entry (tombstone): ~320 B/entry
//   - positive entry (no metadata): ~964 B/entry
//
// See TestNegativeEntryCapacityMatchesEstimate and TestPositiveEntryCapacityMatchesEstimate
// in cache_item_test.go for exact accounting and how to recalculate for different path lengths.
func estimateAttrCacheEntrySize(key string, item *attrCacheItem) int64 {
	// Key string data (the string header is already in LRU's lruItem struct)
	sz := int64(len(key))

	if item != nil {
		// unsafe.Sizeof returns only the struct header size — it does not include
		// heap-allocated data referenced by pointer fields (strings, slices, maps).
		// Those are accounted for explicitly below.
		sz += int64(unsafe.Sizeof(*item))

		if item.attr != nil {
			sz += int64(unsafe.Sizeof(*item.attr))

			sz += int64(len(item.attr.Path))
			sz += int64(len(item.attr.Name))
			sz += int64(len(item.attr.ETag))
			sz += int64(len(item.attr.MD5))

			for k, v := range item.attr.Metadata {
				// 32 bytes covers the per-entry overhead of a Go map bucket:
				// key pointer (8) + value pointer (8) + map internal bookkeeping (~16).
				sz += 32 + int64(len(k))
				if v != nil {
					sz += int64(len(*v))
				}
			}
		}
	}

	// With GOGC=100 (Go's default), the runtime lets garbage accumulate up to 100% of the
	// live heap before collecting, so RSS can reach ~2× the live heap. Doubling the estimate
	// keeps the LRU limit meaningful in RSS terms rather than raw heap terms.
	return sz * 2
}

// attrCacheLRU is an LRU cache specialised for attr-cache entries. Embedding the generic
// LRU promotes all its methods (Get, Put, Peek, Delete, …) while allowing cache-specific
// helpers to be defined here alongside the data model they operate on.
// lastOp records the Unix second of the last cache operation for the TTL sweeper's idle gate.
type attrCacheLRU struct {
	*cachepolicy.LRU[string, *attrCacheItem]
	lastOp atomic.Int64
}

func newAttrCacheLRU(maxSizeBytes int64) *attrCacheLRU {
	return &attrCacheLRU{LRU: cachepolicy.NewLRU(maxSizeBytes, estimateAttrCacheEntrySize)}
}

func (l *attrCacheLRU) touch() {
	l.lastOp.Store(time.Now().Unix())
}

// Get promotes the entry to MRU and records cache activity for the idle gate.
func (l *attrCacheLRU) Get(key string) (*attrCacheItem, bool) {
	l.touch()
	return l.LRU.Get(key)
}

// Has checks whether a key exists and records cache activity for the idle gate.
func (l *attrCacheLRU) Has(key string) bool {
	l.touch()
	return l.LRU.Has(key)
}

// Put inserts or updates an entry and records cache activity for the idle gate.
// Returns false (without modifying the cache) if the entry's size exceeds maxCacheSize.
func (l *attrCacheLRU) Put(key string, val *attrCacheItem) bool {
	l.touch()
	return l.LRU.Put(key, val)
}

// Delete removes an entry and records cache activity for the idle gate.
func (l *attrCacheLRU) Delete(key string) {
	l.touch()
	l.LRU.Delete(key)
}

// ReplaceIf replaces matching entries and records cache activity for the idle gate.
// DeleteIf is intentionally not overridden because the TTL sweeper relies on it without
// resetting the idle gate. Call l.touch() explicitly before using DeleteIf elsewhere.
func (l *attrCacheLRU) ReplaceIf(pred func(string, *attrCacheItem) bool, newVal func(string) *attrCacheItem) {
	l.touch()
	l.LRU.ReplaceIf(pred, newVal)
}

func (l *attrCacheLRU) cachePositiveEntry(path string, attr *internal.ObjAttr) {
	if !l.Put(path, &attrCacheItem{attr: attr, exists: true, cachedAt: time.Now()}) {
		log.Err("attrCacheLRU::cachePositiveEntry : entry too large for cache, skipping path %s", path)
	}
}

func (l *attrCacheLRU) cacheNegativeEntry(path string) {
	if !l.Put(path, &attrCacheItem{cachedAt: time.Now()}) {
		log.Err("attrCacheLRU::cacheNegativeEntry : entry too large for cache, skipping path %s", path)
	}
}

func (l *attrCacheLRU) cacheAttributes(pathList []*internal.ObjAttr) {
	if len(pathList) == 0 {
		return
	}
	// Bulk caching can involve thousands of entries; avoid a time.Now()/atomic.Store per item.
	l.touch()
	currTime := time.Now()
	for _, attr := range pathList {
		key := internal.TruncateDirName(attr.Path)
		if !l.LRU.Put(key, &attrCacheItem{attr: attr, exists: true, cachedAt: currTime}) {
			log.Err("attrCacheLRU::cacheAttributes : entry too large for cache, skipping path %s", key)
		}
	}
}

// Marks the entry as negative
func (l *attrCacheLRU) deletePath(path string, t time.Time) {
	if !l.Put(internal.TruncateDirName(path), &attrCacheItem{cachedAt: t}) {
		log.Err("attrCacheLRU::deletePath : tombstone too large for cache, skipping path %s", path)
	}
}

// Removes the entry from the cache.
func (l *attrCacheLRU) invalidatePath(path string) {
	l.Delete(internal.TruncateDirName(path))
}

func (l *attrCacheLRU) deleteDirectory(path string, t time.Time) {
	// Mark all the child entries as negative entries
	prefix := internal.ExtendDirName(path)
	l.ReplaceIf(func(key string, _ *attrCacheItem) bool {
		return strings.HasPrefix(key, prefix)
	}, func(_ string) *attrCacheItem {
		return &attrCacheItem{cachedAt: t}
	})

	// Mark the directory entry itself as a negative entry
	l.deletePath(path, t)
}

func (l *attrCacheLRU) invalidateDirectory(path string) {
	// Invalidate all the child entries
	l.touch()
	prefix := internal.ExtendDirName(path)
	l.DeleteIf(func(key string, _ *attrCacheItem) bool {
		return strings.HasPrefix(key, prefix)
	})

	// Invalidate directory entry itself
	l.invalidatePath(path)
}

func (l *attrCacheLRU) refreshEntry(path string, attr *internal.ObjAttr) {
	path = internal.TruncateDirName(path)
	if attr == nil {
		// Src attributes unavailable — invalidate Dst so the next GetAttr refetches
		// from the next component rather than poisoning it with a negative entry.
		l.Delete(path)
		return
	}
	if l.Has(path) {
		copied := *attr // copy so we don't mutate the caller's struct
		copied.Path = path
		if !l.Put(path, &attrCacheItem{attr: &copied, exists: true, cachedAt: time.Now()}) {
			log.Err("attrCacheLRU::refreshEntry : entry too large for cache, skipping path %s", path)
		}
	}
}
