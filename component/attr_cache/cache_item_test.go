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
	"fmt"
	"testing"
	"time"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cacheMapTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *cacheMapTestSuite) SetupTest() {
	s.assert = assert.New(s.T())
}

func makeAttr(path string) *internal.ObjAttr {
	return &internal.ObjAttr{Path: path, Name: path, Size: 1024, Mode: 0755}
}

// ---- attrCacheItem ----

func (s *cacheMapTestSuite) TestNewAttrCacheItemPositive() {
	attr := makeAttr("f")
	t := time.Now()
	item := &attrCacheItem{attr: attr, exists: true, cachedAt: t}

	s.assert.True(item.exists)
	s.assert.Equal(attr, item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestNewAttrCacheItemNegative() {
	t := time.Now()
	item := &attrCacheItem{cachedAt: t}

	s.assert.False(item.exists)
	s.assert.Nil(item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestGetAttr() {
	attr := makeAttr("f")
	item := &attrCacheItem{attr: attr, exists: true, cachedAt: time.Now()}
	s.assert.Equal(attr, item.attr)
}

func (s *cacheMapTestSuite) TestIsNegativeEntry() {
	positive := &attrCacheItem{attr: makeAttr("f"), exists: true, cachedAt: time.Now()}
	s.assert.False(positive.isNegativeEntry())

	negative := &attrCacheItem{cachedAt: time.Now()}
	s.assert.True(negative.isNegativeEntry())
}

// ---- estimateAttrCacheEntrySize ----

func (s *cacheMapTestSuite) TestEstimateSizeNilItem() {
	key := "somepath"
	sz := estimateAttrCacheEntrySize(key, nil)
	expected := int64(len(key))
	expected += expected
	s.assert.Equal(expected, sz)
}

func (s *cacheMapTestSuite) TestEstimateSizeNilAttr() {
	key := "p"
	item := &attrCacheItem{cachedAt: time.Now()}
	sz := estimateAttrCacheEntrySize(key, item)

	base := int64(len(key)) + int64(unsafe.Sizeof(*item))
	expected := base * 2
	s.assert.Equal(expected, sz)
}

func (s *cacheMapTestSuite) TestEstimateSizeWithAttr() {
	key := "file"
	attr := makeAttr("file")
	item := &attrCacheItem{attr: attr, exists: true, cachedAt: time.Now()}
	sz := estimateAttrCacheEntrySize(key, item)

	// Must be strictly larger than key + item struct alone
	base := int64(len(key)) + int64(unsafe.Sizeof(*item))
	s.assert.Greater(sz, base)
}

func (s *cacheMapTestSuite) TestEstimateSizeGrowsWithMetadata() {
	key := "f"
	attr := makeAttr("f")
	item := &attrCacheItem{attr: attr, exists: true, cachedAt: time.Now()}
	szWithout := estimateAttrCacheEntrySize(key, item)

	v := "value"
	attr.Metadata = map[string]*string{"key": &v}
	szWith := estimateAttrCacheEntrySize(key, item)

	s.assert.Greater(szWith, szWithout)
}

func (s *cacheMapTestSuite) TestEstimateSizeGrowsWithLongerStrings() {
	short := estimateAttrCacheEntrySize("a", &attrCacheItem{attr: makeAttr("a"), exists: true, cachedAt: time.Now()})
	long := estimateAttrCacheEntrySize("averylongpathname", &attrCacheItem{attr: makeAttr("averylongpathname"), exists: true, cachedAt: time.Now()})
	s.assert.Greater(long, short)
}

// ---- attrCacheLRU ----

func newTestLRU() *attrCacheLRU {
	return newAttrCacheLRU(0, nil) // nil = no idle-gate tracking
}

func (s *cacheMapTestSuite) TestCachePositiveEntry() {
	lru := newTestLRU()
	attr := makeAttr("file")

	lru.cachePositiveEntry("file", attr)

	item, ok := lru.Peek("file")
	s.assert.True(ok)
	s.assert.True(item.exists)
	s.assert.Equal(attr, item.attr)
}

func (s *cacheMapTestSuite) TestCacheNegativeEntry() {
	lru := newTestLRU()

	lru.cacheNegativeEntry("missing")

	item, ok := lru.Peek("missing")
	s.assert.True(ok)
	s.assert.False(item.exists)
	s.assert.Nil(item.attr)
}

func (s *cacheMapTestSuite) TestCacheAttributes() {
	lru := newTestLRU()
	attrs := []*internal.ObjAttr{
		makeAttr("dir/a"),
		makeAttr("dir/b"),
		makeAttr("dir/c/"),
	}

	lru.cacheAttributes(attrs)

	s.assert.Equal(3, lru.Len())
	_, ok := lru.Peek("dir/a")
	s.assert.True(ok)
	_, ok = lru.Peek("dir/b")
	s.assert.True(ok)
	// trailing slash is truncated
	_, ok = lru.Peek("dir/c")
	s.assert.True(ok)
}

func (s *cacheMapTestSuite) TestCacheAttributesAllPositive() {
	lru := newTestLRU()
	lru.cacheAttributes([]*internal.ObjAttr{makeAttr("a"), makeAttr("b")})

	for _, key := range []string{"a", "b"} {
		item, ok := lru.Peek(key)
		s.assert.True(ok, key)
		s.assert.True(item.exists, key)
	}
}

func (s *cacheMapTestSuite) TestDeletePath() {
	lru := newTestLRU()
	lru.cachePositiveEntry("file", makeAttr("file"))
	t := time.Now()

	lru.deletePath("file", t)

	item, ok := lru.Peek("file")
	s.assert.True(ok)
	s.assert.True(item.isNegativeEntry())
	s.assert.Nil(item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestDeletePathAbsent() {
	lru := newTestLRU()
	t := time.Now()

	lru.deletePath("nonexistent", t)

	// Always inserts a tombstone — callers confirmed the path is gone from storage
	item, ok := lru.Peek("nonexistent")
	s.assert.True(ok)
	s.assert.True(item.isNegativeEntry())
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestDeletePathTruncatesTrailingSlash() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))

	lru.deletePath("dir/", time.Now())

	item, ok := lru.Peek("dir")
	s.assert.True(ok)
	s.assert.True(item.isNegativeEntry())
}

func (s *cacheMapTestSuite) TestInvalidatePath() {
	lru := newTestLRU()
	lru.cachePositiveEntry("file", makeAttr("file"))

	lru.invalidatePath("file")

	_, ok := lru.Peek("file")
	s.assert.False(ok)
}

func (s *cacheMapTestSuite) TestInvalidatePathAbsent() {
	lru := newTestLRU()
	// Should not panic
	lru.invalidatePath("nonexistent")
	s.assert.Equal(0, lru.Len())
}

func (s *cacheMapTestSuite) TestInvalidatePathTruncatesTrailingSlash() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))

	lru.invalidatePath("dir/")

	_, ok := lru.Peek("dir")
	s.assert.False(ok)
}

func (s *cacheMapTestSuite) TestDeleteDirectory() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))
	lru.cachePositiveEntry("dir/a", makeAttr("dir/a"))
	lru.cachePositiveEntry("dir/b", makeAttr("dir/b"))
	lru.cachePositiveEntry("other", makeAttr("other"))

	lru.deleteDirectory("dir", time.Now())

	for _, key := range []string{"dir", "dir/a", "dir/b"} {
		item, ok := lru.Peek(key)
		s.assert.True(ok, key)
		s.assert.True(item.isNegativeEntry(), key)
	}
	// sibling not affected
	item, ok := lru.Peek("other")
	s.assert.True(ok)
	s.assert.False(item.isNegativeEntry())
}

func (s *cacheMapTestSuite) TestDeleteDirectoryPreservesNonChildren() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dira", makeAttr("dira")) // "dira" != "dir/"
	lru.cachePositiveEntry("dir/x", makeAttr("dir/x"))

	lru.deleteDirectory("dir", time.Now())

	// "dira" starts with "dir" but not "dir/" — must be untouched
	item, ok := lru.Peek("dira")
	s.assert.True(ok)
	s.assert.False(item.isNegativeEntry())

	item, ok = lru.Peek("dir/x")
	s.assert.True(ok)
	s.assert.True(item.isNegativeEntry())
}

func (s *cacheMapTestSuite) TestDeleteDirectoryAlsoDeletesDirectoryItself() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))

	lru.deleteDirectory("dir", time.Now())

	item, ok := lru.Peek("dir")
	s.assert.True(ok)
	s.assert.True(item.isNegativeEntry())
}

func (s *cacheMapTestSuite) TestInvalidateDirectoryAlsoDeletesDirectoryItself() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))

	lru.invalidateDirectory("dir")

	_, ok := lru.Peek("dir")
	s.assert.False(ok)
}

func (s *cacheMapTestSuite) TestInvalidateDirectory() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))
	lru.cachePositiveEntry("dir/a", makeAttr("dir/a"))
	lru.cachePositiveEntry("dir/b", makeAttr("dir/b"))
	lru.cachePositiveEntry("other", makeAttr("other"))

	lru.invalidateDirectory("dir")

	for _, key := range []string{"dir", "dir/a", "dir/b"} {
		_, ok := lru.Peek(key)
		s.assert.False(ok, key)
	}
	item, ok := lru.Peek("other")
	s.assert.True(ok)
	s.assert.True(item.exists)
}

func (s *cacheMapTestSuite) TestInvalidateDirectoryPreservesNonChildren() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dira", makeAttr("dira"))
	lru.cachePositiveEntry("dir/x", makeAttr("dir/x"))

	lru.invalidateDirectory("dir")

	item, ok := lru.Peek("dira")
	s.assert.True(ok)
	s.assert.True(item.exists)

	_, ok = lru.Peek("dir/x")
	s.assert.False(ok)
}

func (s *cacheMapTestSuite) TestUpdateCacheEntry() {
	lru := newTestLRU()
	old := makeAttr("file")
	lru.cachePositiveEntry("file", old)

	newAttr := &internal.ObjAttr{Size: 9999, Mode: 0600}
	before := time.Now()
	lru.updateCacheEntry("file", newAttr)

	item, ok := lru.Peek("file")
	s.assert.True(ok)
	s.assert.Equal(int64(9999), item.attr.Size)
	s.assert.Equal("file", item.attr.Path) // path is set from the key
	s.assert.True(item.exists)
	s.assert.False(item.cachedAt.Before(before))
}

func (s *cacheMapTestSuite) TestUpdateCacheEntryAbsent() {
	lru := newTestLRU()
	// Should be a no-op for a path not in cache
	lru.updateCacheEntry("missing", makeAttr("missing"))
	s.assert.Equal(0, lru.Len())
}

// ---- Memory limit and capacity tests ----

// typicalPath returns a deterministic 52-byte path for index i, representative of
// real Azure blob paths (container + directory depth + filename).
// Fixed length keeps every entry's estimated size identical, making capacity
// arithmetic exact and regression-friendly.
//
//	"logs/service/2024/" (18 B) + 34-digit zero-padded index (34 B) = 52 B
func typicalPath(i int) string { return fmt.Sprintf("logs/service/2024/%034d", i) }

// typicalName returns the fixed-length filename portion of typicalPath.
func typicalName(i int) string { return fmt.Sprintf("%034d", i) }

// fixedETag is a representative Azure ETag (quoted hex string, 20 bytes).
const fixedETag = "\"0x8DA1B2C3D4E5F6A7\""

// fixedMD5 is a representative 16-byte MD5 digest.
var fixedMD5 = []byte{
	0x9b, 0x74, 0xc9, 0x89, 0x7b, 0xac, 0x77, 0x0f,
	0xfc, 0x9f, 0x8a, 0xd0, 0x7f, 0x36, 0x2e, 0x15,
}

func makeTypicalAttr(i int) *internal.ObjAttr {
	return &internal.ObjAttr{
		Path: typicalPath(i),
		Name: typicalName(i),
		ETag: fixedETag,
		MD5:  fixedMD5,
	}
}

// TestAttrCacheLRUHonorsMemoryLimit verifies the hard invariant: Size() never
// exceeds MaxSize() at any point during a fill that goes well past the limit.
func (s *cacheMapTestSuite) TestAttrCacheLRUHonorsMemoryLimit() {
	const maxSize = 64 * 1024 * 1024
	lru := newAttrCacheLRU(maxSize, nil)

	for i := 0; i < 200_000; i++ {
		lru.cacheNegativeEntry(typicalPath(i))
		if i%10_000 == 0 {
			s.assert.LessOrEqualf(lru.Size(), int64(maxSize),
				"Size() exceeded MaxSize() after %d inserts", i+1)
		}
	}
	s.assert.LessOrEqual(lru.Size(), int64(maxSize))
}

// TestNegativeEntryCapacityMatchesEstimate verifies that the actual number of
// entries the LRU holds at steady state equals the theoretical capacity derived
// from estimateAttrCacheEntrySize.  A mismatch means the size estimator and the
// LRU accounting have drifted apart.
// Run with -v to see the per-entry byte budget and capacity for documentation.
func (s *cacheMapTestSuite) TestNegativeEntryCapacityMatchesEstimate() {
	const maxSize = 64 * 1024 * 1024
	lru := newAttrCacheLRU(maxSize, nil)

	k := typicalPath(0)
	userSize := estimateAttrCacheEntrySize(k, &attrCacheItem{cachedAt: time.Now()})
	bytesPerEntry := lru.PerEntryOverhead() + userSize
	expectedLen := int(int64(maxSize) / bytesPerEntry)

	s.T().Logf("negative entry: %d B/entry → 64 MB holds ~%d entries", bytesPerEntry, expectedLen)

	for i := 0; i < expectedLen*2; i++ {
		lru.cacheNegativeEntry(typicalPath(i))
	}

	s.assert.Equal(expectedLen, lru.Len(),
		"negative-entry capacity mismatch: bytesPerEntry=%d expectedLen=%d", bytesPerEntry, expectedLen)
}

// TestPositiveEntryCapacityMatchesEstimate is the same check for positive entries
// with realistic ETag and MD5 fields populated.
// Run with -v to see the per-entry byte budget and capacity for documentation.
func (s *cacheMapTestSuite) TestPositiveEntryCapacityMatchesEstimate() {
	const maxSize = 64 * 1024 * 1024
	lru := newAttrCacheLRU(maxSize, nil)

	probe := makeTypicalAttr(0)
	userSize := estimateAttrCacheEntrySize(typicalPath(0), &attrCacheItem{attr: probe, exists: true, cachedAt: time.Now()})
	bytesPerEntry := lru.PerEntryOverhead() + userSize
	expectedLen := int(int64(maxSize) / bytesPerEntry)

	s.T().Logf("positive entry (ETag+MD5, no metadata): %d B/entry → 64 MB holds ~%d entries", bytesPerEntry, expectedLen)

	for i := 0; i < expectedLen*2; i++ {
		lru.cachePositiveEntry(typicalPath(i), makeTypicalAttr(i))
	}

	s.assert.Equal(expectedLen, lru.Len(),
		"positive-entry capacity mismatch: bytesPerEntry=%d expectedLen=%d", bytesPerEntry, expectedLen)
}

// TestEntrySizesAreInExpectedRange guards against silent struct bloat or formula
// changes.  The bounds are intentionally loose (~2× headroom) so they survive
// minor struct additions while still catching large regressions.
func (s *cacheMapTestSuite) TestEntrySizesAreInExpectedRange() {
	lru := newAttrCacheLRU(0, nil)

	negUserSize := estimateAttrCacheEntrySize(typicalPath(0), &attrCacheItem{cachedAt: time.Now()})
	negTotal := lru.PerEntryOverhead() + negUserSize
	s.assert.GreaterOrEqualf(negTotal, int64(300), "negative entry shrank below 300 B (got %d B)", negTotal)
	s.assert.LessOrEqualf(negTotal, int64(600), "negative entry grew past 600 B (got %d B)", negTotal)

	posUserSize := estimateAttrCacheEntrySize(typicalPath(0), &attrCacheItem{attr: makeTypicalAttr(0), exists: true, cachedAt: time.Now()})
	posTotal := lru.PerEntryOverhead() + posUserSize
	s.assert.GreaterOrEqualf(posTotal, int64(700), "positive entry shrank below 700 B (got %d B)", posTotal)
	s.assert.LessOrEqualf(posTotal, int64(1500), "positive entry grew past 1500 B (got %d B)", posTotal)
}

func TestCacheMapTestSuite(t *testing.T) {
	suite.Run(t, new(cacheMapTestSuite))
}
