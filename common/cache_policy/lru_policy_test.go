/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package cache_policy

import (
	"container/list"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type typesTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *typesTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func assertBlockCached(suite *typesTestSuite, key, endIndex int64, cache *LRUCache) {
	blk := &common.Block{StartIndex: key, EndIndex: endIndex}
	block, found := cache.Get(key)
	suite.assert.Equal(blk, block)
	suite.assert.Equal(true, found)
}

func assertBlockNotCached(suite *typesTestSuite, key int64, cache *LRUCache) {
	_, found := cache.Get(key)
	suite.assert.Equal(false, found)
}

func TestGenerateConfig(t *testing.T) {
	suite.Run(t, new(typesTestSuite))
}

func (suite *typesTestSuite) TestNewLRUCache() {
	lruCache := NewLRUCache(4)
	suite.assert.Equal(int64(4), lruCache.Capacity)
}

func (suite *typesTestSuite) TestPutBlock() {
	lruCache := NewLRUCache(4)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	lruCache.Put(0, blk1)

	assertBlockCached(suite, int64(0), int64(1), lruCache)
	suite.assert.Equal(int64(1), lruCache.Occupied)
}

func (suite *typesTestSuite) TestCachePurge() {
	lruCache := NewLRUCache(4)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	lruCache.Put(0, blk1)

	suite.assert.Equal(int64(4), lruCache.Capacity)
	suite.assert.Equal(int64(1), lruCache.Occupied)
	assertBlockCached(suite, int64(0), int64(1), lruCache)

	lruCache.Purge()
	suite.assert.Equal(0, len(lruCache.Keys()))
	suite.assert.Equal(int64(0), lruCache.Capacity)
	suite.assert.Equal(map[int64]*list.Element(map[int64]*list.Element(nil)), lruCache.Elements)
}

func (suite *typesTestSuite) TestBlockNotFound() {
	lruCache := NewLRUCache(4)
	suite.assert.Equal(int64(4), lruCache.Capacity)
	assertBlockNotCached(suite, 0, lruCache)
}

func (suite *typesTestSuite) TestResizeBlock() {
	lruCache := NewLRUCache(5)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	blk2 := &common.Block{StartIndex: 1, EndIndex: 3}

	lruCache.Put(0, blk1)
	lruCache.Put(1, blk2)

	suite.assert.Equal(int64(5), lruCache.Capacity)
	suite.assert.Equal(int64(3), lruCache.Occupied)
	suite.assert.Equal(2, len(lruCache.Keys()))

	//resize to larger
	lruCache.Resize(1, 4)
	suite.assert.Equal(int64(4), lruCache.Occupied)
	suite.assert.Equal(2, len(lruCache.Keys()))

	// resize to smaller
	lruCache.Resize(1, 2)
	suite.assert.Equal(int64(2), lruCache.Occupied)
	suite.assert.Equal(2, len(lruCache.Keys()))
}

func (suite *typesTestSuite) TestLRUPolicy() {
	lruCache := NewLRUCache(5)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	blk2 := &common.Block{StartIndex: 1, EndIndex: 2}
	blk3 := &common.Block{StartIndex: 2, EndIndex: 3}

	lruCache.Put(0, blk1)
	lruCache.Put(1, blk2)
	lruCache.Put(2, blk3)

	lruCache.Get(blk1.StartIndex)
	lruCache.Get(blk3.StartIndex)
	lruCache.Get(blk2.StartIndex)
	lruCache.Get(blk1.StartIndex)

	suite.assert.Equal(blk1, lruCache.RecentlyUsed())
	suite.assert.Equal(blk3, lruCache.LeastRecentlyUsed())
	suite.assert.Equal(3, len(lruCache.Keys()))
}

func (suite *typesTestSuite) TestEvictionSingleBlock() {
	lruCache := NewLRUCache(1)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	blk2 := &common.Block{StartIndex: 1, EndIndex: 2}

	lruCache.Put(0, blk1)
	lruCache.Put(1, blk2)

	assertBlockCached(suite, blk2.StartIndex, blk2.EndIndex, lruCache)
	assertBlockNotCached(suite, blk1.StartIndex, lruCache)

}

func (suite *typesTestSuite) TestEviction() {
	lruCache := NewLRUCache(3)
	blk1 := &common.Block{StartIndex: 0, EndIndex: 1}
	blk2 := &common.Block{StartIndex: 1, EndIndex: 2}
	blk3 := &common.Block{StartIndex: 2, EndIndex: 3}
	blk4 := &common.Block{StartIndex: 3, EndIndex: 8}

	lruCache.Put(0, blk1)
	lruCache.Put(1, blk2)
	lruCache.Put(2, blk3)

	lruCache.Get(blk1.StartIndex)
	lruCache.Get(blk3.StartIndex)
	lruCache.Get(blk2.StartIndex)
	lruCache.Get(blk1.StartIndex)
	suite.assert.Equal(int64(3), lruCache.Occupied)

	lruCache.Put(3, blk4)

	suite.assert.Equal(blk4, lruCache.RecentlyUsed())
	suite.assert.Equal(blk2, lruCache.LeastRecentlyUsed())
	assertBlockNotCached(suite, blk3.StartIndex, lruCache)
	suite.assert.Equal(3, len(lruCache.Keys()))
	suite.assert.Equal(int64(7), lruCache.Occupied)
}

func (suite *typesTestSuite) TestDirtyBlockEviction() {
	lruCache := NewLRUCache(3)
	blk0 := &common.Block{StartIndex: 0, EndIndex: 1}
	blk1 := &common.Block{StartIndex: 1, EndIndex: 2}
	blk2 := &common.Block{StartIndex: 2, EndIndex: 3}
	blk3 := &common.Block{StartIndex: 3, EndIndex: 8}

	lruCache.Put(0, blk0)
	lruCache.Put(1, blk1)
	lruCache.Put(2, blk2)

	lruCache.Get(blk0.StartIndex)
	lruCache.Get(blk2.StartIndex)
	lruCache.Get(blk1.StartIndex)
	lruCache.Get(blk0.StartIndex)

	blk0.Flags.Set(common.DirtyBlock)
	blk1.Flags.Set(common.DirtyBlock)
	blk2.Flags.Set(common.DirtyBlock)

	success := lruCache.Put(3, blk3)

	suite.assert.Equal(false, success)
	// assert operation was not successful since all blocks are dirty
	suite.assert.Equal(blk0, lruCache.RecentlyUsed())
	suite.assert.Equal(blk2, lruCache.LeastRecentlyUsed())

	assertBlockNotCached(suite, blk3.StartIndex, lruCache)
	suite.assert.Equal(3, len(lruCache.Keys()))
	suite.assert.Equal(int64(3), lruCache.Occupied)

	// clear the recently used block and ensure that it ends up evicting it over others
	blk0.Flags.Clear(common.DirtyBlock)
	success = lruCache.Put(3, blk3)

	suite.assert.Equal(true, success)
	suite.assert.Equal(blk3, lruCache.RecentlyUsed())
	suite.assert.Equal(blk2, lruCache.LeastRecentlyUsed())

	assertBlockNotCached(suite, blk0.StartIndex, lruCache)
	suite.assert.Equal(3, len(lruCache.Keys()))
	suite.assert.Equal(int64(7), lruCache.Occupied)

	blk4 := &common.Block{StartIndex: 8, EndIndex: 9}
	blk1.Flags.Clear(common.DirtyBlock)
	success = lruCache.Put(8, blk4)

	suite.assert.Equal(true, success)
	suite.assert.Equal(blk4, lruCache.RecentlyUsed())
	suite.assert.Equal(blk2, lruCache.LeastRecentlyUsed())

	assertBlockNotCached(suite, blk1.StartIndex, lruCache)
	suite.assert.Equal(3, len(lruCache.Keys()))
	suite.assert.Equal(int64(7), lruCache.Occupied)

}
