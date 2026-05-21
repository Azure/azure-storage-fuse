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
	"os"
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

func makeAttrWithMetadata(path string) *internal.ObjAttr {
	v := "val"
	return &internal.ObjAttr{
		Path:     path,
		Name:     path,
		ETag:     "etag123",
		Metadata: map[string]*string{"key": &v},
	}
}

// ---- attrCacheItem ----

func (s *cacheMapTestSuite) TestNewAttrCacheItemPositive() {
	attr := makeAttr("f")
	t := time.Now()
	item := newAttrCacheItem(attr, true, t)

	s.assert.True(item.valid)
	s.assert.True(item.exists)
	s.assert.Equal(attr, item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestNewAttrCacheItemNegative() {
	t := time.Now()
	item := newAttrCacheItem(nil, false, t)

	s.assert.True(item.valid)
	s.assert.False(item.exists)
	s.assert.Nil(item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestGetAttr() {
	attr := makeAttr("f")
	item := newAttrCacheItem(attr, true, time.Now())
	s.assert.Equal(attr, item.getAttr())
}

func (s *cacheMapTestSuite) TestInvalidate() {
	item := newAttrCacheItem(makeAttr("f"), true, time.Now())
	item.invalidate()

	s.assert.False(item.valid)
	s.assert.Nil(item.attr)
	// exists is unchanged — invalidate is about staleness, not presence
	s.assert.True(item.exists)
}

func (s *cacheMapTestSuite) TestIsNegativeEntry() {
	positive := newAttrCacheItem(makeAttr("f"), true, time.Now())
	s.assert.False(positive.isNegativeEntry())

	negative := newAttrCacheItem(nil, false, time.Now())
	s.assert.True(negative.isNegativeEntry())
}

func (s *cacheMapTestSuite) TestMarkAsNegativeEntry() {
	item := newAttrCacheItem(makeAttr("f"), true, time.Now())
	before := time.Now()
	item.markAsNegativeEntry(before)

	s.assert.False(item.exists)
	s.assert.True(item.valid)
	s.assert.Nil(item.attr)
	s.assert.Equal(before, item.cachedAt)
}

func (s *cacheMapTestSuite) TestSetSize() {
	attr := makeAttr("f")
	item := newAttrCacheItem(attr, true, time.Now())
	before := time.Now()

	item.setSize(999)

	s.assert.Equal(int64(999), item.attr.Size)
	s.assert.False(item.attr.Mtime.Before(before))
	s.assert.False(item.cachedAt.Before(before))
}

func (s *cacheMapTestSuite) TestSetMode() {
	attr := makeAttr("f")
	item := newAttrCacheItem(attr, true, time.Now())
	before := time.Now()

	item.setMode(os.FileMode(0644))

	s.assert.Equal(os.FileMode(0644), item.attr.Mode)
	s.assert.False(item.attr.Ctime.Before(before))
	s.assert.False(item.cachedAt.Before(before))
}

// ---- estimateAttrCacheEntrySize ----

func (s *cacheMapTestSuite) TestEstimateSizeNilItem() {
	key := "somepath"
	sz := estimateAttrCacheEntrySize(key, nil)
	expected := int64(len(key))
	expected += int64(float64(expected) * heapOverheadFactor)
	s.assert.Equal(expected, sz)
}

func (s *cacheMapTestSuite) TestEstimateSizeNilAttr() {
	key := "p"
	item := newAttrCacheItem(nil, false, time.Now())
	sz := estimateAttrCacheEntrySize(key, item)

	base := int64(len(key)) + int64(unsafe.Sizeof(*item))
	expected := base + int64(float64(base)*heapOverheadFactor)
	s.assert.Equal(expected, sz)
}

func (s *cacheMapTestSuite) TestEstimateSizeWithAttr() {
	key := "file"
	attr := makeAttr("file")
	item := newAttrCacheItem(attr, true, time.Now())
	sz := estimateAttrCacheEntrySize(key, item)

	// Must be strictly larger than key + item struct alone
	base := int64(len(key)) + int64(unsafe.Sizeof(*item))
	s.assert.Greater(sz, base)
}

func (s *cacheMapTestSuite) TestEstimateSizeGrowsWithMetadata() {
	key := "f"
	attr := makeAttr("f")
	item := newAttrCacheItem(attr, true, time.Now())
	szWithout := estimateAttrCacheEntrySize(key, item)

	v := "value"
	attr.Metadata = map[string]*string{"key": &v}
	szWith := estimateAttrCacheEntrySize(key, item)

	s.assert.Greater(szWith, szWithout)
}

func (s *cacheMapTestSuite) TestEstimateSizeGrowsWithLongerStrings() {
	short := estimateAttrCacheEntrySize("a", newAttrCacheItem(makeAttr("a"), true, time.Now()))
	long := estimateAttrCacheEntrySize("averylongpathname", newAttrCacheItem(makeAttr("averylongpathname"), true, time.Now()))
	s.assert.Greater(long, short)
}

// ---- attrCacheLRU ----

func newTestLRU() *attrCacheLRU {
	return newAttrCacheLRU(0) // no memory limit
}

func (s *cacheMapTestSuite) TestCachePositiveEntry() {
	lru := newTestLRU()
	attr := makeAttr("file")

	lru.cachePositiveEntry("file", attr)

	item, ok := lru.Peek("file")
	s.assert.True(ok)
	s.assert.True(item.valid)
	s.assert.True(item.exists)
	s.assert.Equal(attr, item.attr)
}

func (s *cacheMapTestSuite) TestCacheNegativeEntry() {
	lru := newTestLRU()

	lru.cacheNegativeEntry("missing")

	item, ok := lru.Peek("missing")
	s.assert.True(ok)
	s.assert.True(item.valid)
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
		s.assert.True(item.valid, key)
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
	s.assert.True(item.valid)
	s.assert.True(item.isNegativeEntry())
	s.assert.Nil(item.attr)
	s.assert.Equal(t, item.cachedAt)
}

func (s *cacheMapTestSuite) TestDeletePathAbsent() {
	lru := newTestLRU()
	// Should not panic when path is not in cache
	lru.deletePath("nonexistent", time.Now())
	s.assert.Equal(0, lru.Len())
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

	item, ok := lru.Peek("file")
	s.assert.True(ok)
	s.assert.False(item.valid)
	s.assert.Nil(item.attr)
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

	item, ok := lru.Peek("dir")
	s.assert.True(ok)
	s.assert.False(item.valid)
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

func (s *cacheMapTestSuite) TestInvalidateDirectory() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dir", makeAttr("dir"))
	lru.cachePositiveEntry("dir/a", makeAttr("dir/a"))
	lru.cachePositiveEntry("dir/b", makeAttr("dir/b"))
	lru.cachePositiveEntry("other", makeAttr("other"))

	lru.invalidateDirectory("dir")

	for _, key := range []string{"dir", "dir/a", "dir/b"} {
		item, ok := lru.Peek(key)
		s.assert.True(ok, key)
		s.assert.False(item.valid, key)
	}
	item, ok := lru.Peek("other")
	s.assert.True(ok)
	s.assert.True(item.valid)
}

func (s *cacheMapTestSuite) TestInvalidateDirectoryPreservesNonChildren() {
	lru := newTestLRU()
	lru.cachePositiveEntry("dira", makeAttr("dira"))
	lru.cachePositiveEntry("dir/x", makeAttr("dir/x"))

	lru.invalidateDirectory("dir")

	item, ok := lru.Peek("dira")
	s.assert.True(ok)
	s.assert.True(item.valid)

	item, ok = lru.Peek("dir/x")
	s.assert.True(ok)
	s.assert.False(item.valid)
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

func TestCacheMapTestSuite(t *testing.T) {
	suite.Run(t, new(cacheMapTestSuite))
}
