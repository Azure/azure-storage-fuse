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
	"time"
	"unsafe"

	cachepolicy "github.com/Azure/azure-storage-fuse/v2/common/cache_policy"
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
		sz += int64(unsafe.Sizeof(*item))

		if item.attr != nil {
			sz += int64(unsafe.Sizeof(*item.attr))

			sz += int64(len(item.attr.Path))
			sz += int64(len(item.attr.Name))
			sz += int64(len(item.attr.ETag))
			sz += int64(len(item.attr.MD5))

			for k, v := range item.attr.Metadata {
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
type attrCacheLRU struct {
	*cachepolicy.LRU[string, *attrCacheItem]
}

func newAttrCacheLRU(maxSizeBytes int64) *attrCacheLRU {
	return &attrCacheLRU{cachepolicy.NewLRU(maxSizeBytes, estimateAttrCacheEntrySize)}
}

func (l *attrCacheLRU) cachePositiveEntry(path string, attr *internal.ObjAttr) {
	l.Put(path, &attrCacheItem{attr: attr, exists: true, cachedAt: time.Now()})
}

func (l *attrCacheLRU) cacheNegativeEntry(path string) {
	l.Put(path, &attrCacheItem{cachedAt: time.Now()})
}

func (l *attrCacheLRU) cacheAttributes(pathList []*internal.ObjAttr) {
	for _, attr := range pathList {
		key := internal.TruncateDirName(attr.Path)
		l.cachePositiveEntry(key, attr)
	}
}

// Marks the entry as negative
func (l *attrCacheLRU) deletePath(path string, t time.Time) {
	l.Put(internal.TruncateDirName(path), &attrCacheItem{cachedAt: t})
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
	prefix := internal.ExtendDirName(path)
	l.DeleteIf(func(key string, _ *attrCacheItem) bool {
		return strings.HasPrefix(key, prefix)
	})

	// Invalidate directory entry itself
	l.invalidatePath(path)
}

func (l *attrCacheLRU) updateCacheEntry(path string, attr *internal.ObjAttr) {
	if l.Has(path) {
		if attr != nil {
			copied := *attr // copy so we don't mutate the caller's struct
			copied.Path = path
			attr = &copied
		}
		l.Put(path, &attrCacheItem{attr: attr, exists: attr != nil, cachedAt: time.Now()})
	}
}
