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

package cache_policy

import (
	"container/list"
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type lruCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *lruCacheTestSuite) SetupTest() {
	s.assert = assert.New(s.T())
}

// identity sizeOf: returns len(key) + len(val) for string caches.
func strSizeOf(k, v string) int64 {
	return int64(len(k) + len(v))
}

// zeroSizeOf ignores key and value; useful when we want to reason about overhead only.
func zeroSizeOf[K comparable, V any](K, V) int64 { return 0 }

// ---- basic operations ----

func (s *lruCacheTestSuite) TestBasicPutGet() {
	lru := NewLRU[string, string](0, strSizeOf)
	lru.Put("a", "1")
	lru.Put("b", "2")

	v, ok := lru.Get("a")
	s.assert.True(ok)
	s.assert.Equal("1", v)

	v, ok = lru.Get("b")
	s.assert.True(ok)
	s.assert.Equal("2", v)

	_, ok = lru.Get("missing")
	s.assert.False(ok)
}

func (s *lruCacheTestSuite) TestPeekDoesNotPromote() {
	lru := NewLRU[string, string](0, strSizeOf)
	lru.Put("a", "1")
	lru.Put("b", "2") // b is now MRU

	// Peek at "a" — should not promote it
	v, ok := lru.Peek("a")
	s.assert.True(ok)
	s.assert.Equal("1", v)

	// Without any limit the order won't matter for eviction, but verify via Range (MRU first)
	var order []string
	lru.Range(func(k, _ string) bool {
		order = append(order, k)
		return true
	})
	s.assert.Equal([]string{"b", "a"}, order) // b is still MRU
}

func (s *lruCacheTestSuite) TestHas() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	lru.Put("x", 42)
	s.assert.True(lru.Has("x"))
	s.assert.False(lru.Has("y"))
}

func (s *lruCacheTestSuite) TestDelete() {
	lru := NewLRU[string, string](0, strSizeOf)
	lru.Put("a", "1")
	lru.Put("b", "2")
	lru.Delete("a")

	s.assert.False(lru.Has("a"))
	s.assert.True(lru.Has("b"))
	s.assert.Equal(1, lru.Len())
}

func (s *lruCacheTestSuite) TestDeleteAbsent() {
	lru := NewLRU[string, string](0, strSizeOf)
	// Should not panic
	lru.Delete("nonexistent")
	s.assert.Equal(0, lru.Len())
}

func (s *lruCacheTestSuite) TestLen() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	s.assert.Equal(0, lru.Len())
	lru.Put("a", 1)
	s.assert.Equal(1, lru.Len())
	lru.Put("b", 2)
	s.assert.Equal(2, lru.Len())
	lru.Delete("a")
	s.assert.Equal(1, lru.Len())
}

func (s *lruCacheTestSuite) TestPurge() {
	lru := NewLRU[string, string](0, strSizeOf)
	lru.Put("a", "1")
	lru.Put("b", "2")
	lru.Purge()
	s.assert.Equal(0, lru.Len())
	s.assert.Equal(int64(0), lru.Size())
	s.assert.False(lru.Has("a"))
}

// ---- size accounting ----

func (s *lruCacheTestSuite) TestSizeAccounting() {
	lru := NewLRU[string, string](0, strSizeOf)
	overhead := lru.PerEntryOverhead()

	lru.Put("ab", "cd") // userSize = 2+2 = 4
	s.assert.Equal(overhead+4, lru.Size())

	lru.Put("ab", "cdef") // update: userSize changes from 4 to 6
	s.assert.Equal(overhead+6, lru.Size())

	lru.Delete("ab")
	s.assert.Equal(int64(0), lru.Size())
}

func (s *lruCacheTestSuite) TestPerEntryOverheadUsesUnsafeSizeof() {
	lru := NewLRU[string, string](0, strSizeOf)
	overhead := lru.PerEntryOverhead()

	// Overhead must cover at least: lruItem struct + list.Element struct
	var item lruItem[string, string]
	minExpected := int64(unsafe.Sizeof(item)) + int64(unsafe.Sizeof(list.Element{}))
	s.assert.GreaterOrEqual(overhead, minExpected,
		"perEntryOverhead should be >= sizeof(lruItem) + sizeof(list.Element)")
	// And must include the map bucket overhead constant
	s.assert.GreaterOrEqual(overhead, minExpected+mapBucketOverhead)
}

// ---- eviction ----

func (s *lruCacheTestSuite) TestNoLimitNoEviction() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int]) // maxSize=0 means no limit
	for i := 0; i < 10000; i++ {
		k := string(rune('a' + i%26))
		lru.Put(k, i)
	}
	// All distinct keys are "a".."z" (26 entries), none evicted
	s.assert.Equal(26, lru.Len())
}

func (s *lruCacheTestSuite) TestEvictionAtCapacity() {
	// Allow room for exactly 2 entries (overhead + 0 user bytes each)
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()

	lru.SetMaxSize(overhead * 2)

	lru.Put("a", 1)
	lru.Put("b", 2)
	s.assert.Equal(2, lru.Len())

	// Adding "c" should evict "a" (LRU)
	lru.Put("c", 3)
	s.assert.Equal(2, lru.Len())
	s.assert.False(lru.Has("a"), "LRU entry 'a' should have been evicted")
	s.assert.True(lru.Has("b"))
	s.assert.True(lru.Has("c"))
}

func (s *lruCacheTestSuite) TestEvictionPromotion() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	lru.SetMaxSize(overhead * 2)

	lru.Put("a", 1)
	lru.Put("b", 2) // order: b(MRU), a(LRU)

	// Promote "a" via Get
	lru.Get("a") // order: a(MRU), b(LRU)

	// Adding "c" should evict "b" (now LRU), not "a"
	lru.Put("c", 3)
	s.assert.Equal(2, lru.Len())
	s.assert.True(lru.Has("a"), "'a' was recently accessed and should survive")
	s.assert.False(lru.Has("b"), "'b' should be evicted (LRU)")
	s.assert.True(lru.Has("c"))
}

func (s *lruCacheTestSuite) TestEvictionOrder() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	lru.SetMaxSize(overhead * 3)

	lru.Put("a", 1) // LRU order: a
	lru.Put("b", 2) // LRU order: b, a
	lru.Put("c", 3) // LRU order: c, b, a — full

	lru.Put("d", 4) // evicts "a"
	s.assert.False(lru.Has("a"))
	lru.Put("e", 5) // evicts "b"
	s.assert.False(lru.Has("b"))

	s.assert.True(lru.Has("c"))
	s.assert.True(lru.Has("d"))
	s.assert.True(lru.Has("e"))
}

func (s *lruCacheTestSuite) TestSetMaxSizeTriggersEviction() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()

	// Fill with 5 entries (no limit yet)
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		lru.Put(k, 1)
	}
	s.assert.Equal(5, lru.Len())

	// Shrink to 2 entries
	lru.SetMaxSize(overhead * 2)
	s.assert.Equal(2, lru.Len())
	s.assert.LessOrEqual(lru.Size(), overhead*2)
}

func (s *lruCacheTestSuite) TestUpdateIncreasesSize() {
	lru := NewLRU[string, string](0, strSizeOf)
	overhead := lru.PerEntryOverhead()
	lru.SetMaxSize(overhead + 4) // room for 1 entry with 4 user bytes

	lru.Put("ab", "cd") // userSize=4, fits exactly
	s.assert.Equal(1, lru.Len())

	// Update with a value too large to ever fit — rejected upfront, original entry survives.
	retained := lru.Put("ab", "cdefgh") // userSize=8, exceeds limit
	s.assert.False(retained, "entry whose size exceeds maxSize should be rejected")
	s.assert.Equal(1, lru.Len(), "existing entry must not be displaced")
	v, ok := lru.Get("ab")
	s.assert.True(ok)
	s.assert.Equal("cd", v, "original value must be unchanged")
}

// ---- Range ----

func (s *lruCacheTestSuite) TestRange() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3) // MRU order: c, b, a

	var keys []string
	lru.Range(func(k string, _ int) bool {
		keys = append(keys, k)
		return true
	})
	s.assert.Equal([]string{"c", "b", "a"}, keys)
}

func (s *lruCacheTestSuite) TestRangeEarlyStop() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	for _, k := range []string{"a", "b", "c", "d"} {
		lru.Put(k, 1)
	}

	count := 0
	lru.Range(func(_ string, _ int) bool {
		count++
		return count < 2 // stop after 2
	})
	s.assert.Equal(2, count)
}

// ---- integration: integer keys ----

func (s *lruCacheTestSuite) TestIntKeys() {
	sizeOf := func(k int, v []byte) int64 { return int64(len(v)) }
	lru := NewLRU[int, []byte](0, sizeOf)
	lru.Put(1, []byte("hello"))
	lru.Put(2, []byte("world"))

	v, ok := lru.Get(1)
	s.assert.True(ok)
	s.assert.Equal([]byte("hello"), v)
	s.assert.Equal(2, lru.Len())
}

// ---- frozen userSize ----

func (s *lruCacheTestSuite) TestFrozenUserSizeOnMutation() {
	// sizeOf reports the length of the slice the pointer points to.
	// After insertion, we grow the slice — sizeOf would return a larger value,
	// but the LRU must still use the size frozen at insertion time.
	type buf struct{ data []byte }
	sizeOf := func(_ string, v *buf) int64 { return int64(len(v.data)) }

	lru := NewLRU[string, *buf](0, sizeOf)
	overhead := lru.PerEntryOverhead()

	b := &buf{data: []byte("hi")} // userSize = 2 at insertion
	lru.Put("k", b)
	s.assert.Equal(overhead+2, lru.Size())

	// Mutate the pointed-to value — sizeOf would now return 100
	b.data = make([]byte, 100)
	// Size must remain overhead+2 (frozen at insertion)
	s.assert.Equal(overhead+2, lru.Size())

	// Delete must subtract the frozen size, not the current size
	lru.Delete("k")
	s.assert.Equal(int64(0), lru.Size())
}

func (s *lruCacheTestSuite) TestFrozenUserSizeOnUpdate() {
	// When Put is called again for an existing key the new userSize replaces the old one.
	type buf struct{ data []byte }
	sizeOf := func(_ string, v *buf) int64 { return int64(len(v.data)) }

	lru := NewLRU[string, *buf](0, sizeOf)
	overhead := lru.PerEntryOverhead()

	b := &buf{data: []byte("hi")}
	lru.Put("k", b)

	// Update with a value that reports a different size at Put time
	b2 := &buf{data: make([]byte, 50)}
	lru.Put("k", b2)
	s.assert.Equal(overhead+50, lru.Size())

	// Now mutate b2 — size must stay at 50
	b2.data = make([]byte, 200)
	s.assert.Equal(overhead+50, lru.Size())

	lru.Delete("k")
	s.assert.Equal(int64(0), lru.Size())
}

// ---- delete size accounting ----

func (s *lruCacheTestSuite) TestDeleteSizeAccounting() {
	lru := NewLRU[string, string](0, strSizeOf)
	overhead := lru.PerEntryOverhead()

	lru.Put("ab", "cd") // userSize = 4
	s.assert.Equal(overhead+4, lru.Size())

	lru.Delete("ab")
	s.assert.Equal(int64(0), lru.Size())
	s.assert.Equal(0, lru.Len())
}

// ---- DeleteIf ----

func (s *lruCacheTestSuite) TestDeleteIf() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()

	for _, k := range []string{"a", "b", "c", "d"} {
		lru.Put(k, 1)
	}
	s.assert.Equal(int64(4)*overhead, lru.Size())

	// Remove entries whose key >= "c"
	lru.DeleteIf(func(k string, _ int) bool { return k >= "c" })

	s.assert.Equal(2, lru.Len())
	s.assert.True(lru.Has("a"))
	s.assert.True(lru.Has("b"))
	s.assert.False(lru.Has("c"))
	s.assert.False(lru.Has("d"))
	s.assert.Equal(int64(2)*overhead, lru.Size())
}

func (s *lruCacheTestSuite) TestDeleteIfAll() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	lru.Put("a", 1)
	lru.Put("b", 2)

	lru.DeleteIf(func(_ string, _ int) bool { return true })

	s.assert.Equal(0, lru.Len())
	s.assert.Equal(int64(0), lru.Size())
}

func (s *lruCacheTestSuite) TestDeleteIfNone() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	lru.Put("a", 1)
	lru.Put("b", 2)

	lru.DeleteIf(func(_ string, _ int) bool { return false })

	s.assert.Equal(2, lru.Len())
	s.assert.Equal(int64(2)*overhead, lru.Size())
}

func (s *lruCacheTestSuite) TestDeleteIfEmpty() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	// Should not panic on empty cache
	lru.DeleteIf(func(_ string, _ int) bool { return true })
	s.assert.Equal(0, lru.Len())
}

func (s *lruCacheTestSuite) TestReplaceIf() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	for _, k := range []string{"a", "b", "c", "d"} {
		lru.Put(k, 1)
	}

	lru.ReplaceIf(func(k string, _ int) bool { return k >= "c" }, func(_ string) int { return 99 })

	s.assert.Equal(4, lru.Len())
	s.assert.Equal(int64(4)*overhead, lru.Size())
	v, ok := lru.Peek("a")
	s.assert.True(ok)
	s.assert.Equal(1, v)
	v, ok = lru.Peek("c")
	s.assert.True(ok)
	s.assert.Equal(99, v)
	v, ok = lru.Peek("d")
	s.assert.True(ok)
	s.assert.Equal(99, v)
}

func (s *lruCacheTestSuite) TestReplaceIfAll() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	lru.Put("a", 1)
	lru.Put("b", 2)

	lru.ReplaceIf(func(_ string, _ int) bool { return true }, func(_ string) int { return 0 })

	s.assert.Equal(2, lru.Len())
	v, _ := lru.Peek("a")
	s.assert.Equal(0, v)
	v, _ = lru.Peek("b")
	s.assert.Equal(0, v)
}

func (s *lruCacheTestSuite) TestReplaceIfNone() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	lru.Put("a", 1)
	lru.Put("b", 2)

	lru.ReplaceIf(func(_ string, _ int) bool { return false }, func(_ string) int { return 99 })

	s.assert.Equal(2, lru.Len())
	s.assert.Equal(int64(2)*overhead, lru.Size())
	v, _ := lru.Peek("a")
	s.assert.Equal(1, v)
	v, _ = lru.Peek("b")
	s.assert.Equal(2, v)
}

func (s *lruCacheTestSuite) TestReplaceIfEmpty() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	// Should not panic on empty cache
	lru.ReplaceIf(func(_ string, _ int) bool { return true }, func(_ string) int { return 0 })
	s.assert.Equal(0, lru.Len())
}

func (s *lruCacheTestSuite) TestReplaceIfPromotesToMRU() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	lru.Put("a", 1)
	lru.Put("b", 2) // b is MRU

	lru.ReplaceIf(func(k string, _ int) bool { return k == "a" }, func(_ string) int { return 99 })

	// "a" should have been promoted to MRU after replacement
	var first string
	lru.Range(func(k string, _ int) bool { first = k; return false })
	s.assert.Equal("a", first)
}

// ---- purge then reuse ----

func (s *lruCacheTestSuite) TestPurgeAndReuse() {
	lru := NewLRU[string, string](0, strSizeOf)
	lru.Put("a", "1")
	lru.Put("b", "2")
	lru.Purge()

	// Cache must be fully functional after Purge
	lru.Put("c", "3")
	v, ok := lru.Get("c")
	s.assert.True(ok)
	s.assert.Equal("3", v)
	s.assert.Equal(1, lru.Len())
	s.assert.False(lru.Has("a"))
	s.assert.False(lru.Has("b"))
}

// ---- Range on empty cache ----

func (s *lruCacheTestSuite) TestRangeOnEmptyCache() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	calls := 0
	lru.Range(func(_ string, _ int) bool {
		calls++
		return true
	})
	s.assert.Equal(0, calls)
}

// ---- SetMaxSize(0) disables eviction ----

func (s *lruCacheTestSuite) TestSetMaxSizeZeroDisablesEviction() {
	lru := NewLRU[string, int](0, zeroSizeOf[string, int])
	overhead := lru.PerEntryOverhead()
	lru.SetMaxSize(overhead * 2)

	lru.Put("a", 1)
	lru.Put("b", 2)
	s.assert.Equal(2, lru.Len())

	// Remove the limit
	lru.SetMaxSize(0)
	s.assert.Equal(int64(0), lru.MaxSize())

	// Now we can add as many entries as we want without eviction
	for _, k := range []string{"c", "d", "e", "f"} {
		lru.Put(k, 1)
	}
	s.assert.Equal(6, lru.Len())
}

// ---- MaxSize accessor ----

func (s *lruCacheTestSuite) TestMaxSizeAccessor() {
	lru := NewLRU[string, int](1024, zeroSizeOf[string, int])
	s.assert.Equal(int64(1024), lru.MaxSize())

	lru.SetMaxSize(2048)
	s.assert.Equal(int64(2048), lru.MaxSize())

	lru.SetMaxSize(0)
	s.assert.Equal(int64(0), lru.MaxSize())
}

// ---- Peek on absent key ----

func (s *lruCacheTestSuite) TestPeekAbsent() {
	lru := NewLRU[string, string](0, strSizeOf)
	v, ok := lru.Peek("missing")
	s.assert.False(ok)
	s.assert.Empty(v)
}

// ---- concurrent access ----

func (s *lruCacheTestSuite) TestConcurrentAccess() {
	// Run under `go test -race` to detect data races.
	lru := NewLRU[int, int](0, zeroSizeOf[int, int])
	const goroutines = 8
	const ops = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := (id*ops + i) % 64
				lru.Put(key, id*ops+i)
				lru.Get(key)
				lru.Peek(key)
				lru.Has(key)
				if i%10 == 0 {
					lru.Delete(key)
				}
			}
		}(g)
	}
	wg.Wait()
	// Just verify the LRU is in a consistent state
	s.assert.GreaterOrEqual(lru.Len(), 0)
	s.assert.GreaterOrEqual(lru.Size(), int64(0))
}

func (s *lruCacheTestSuite) TestConcurrentAccessWithLimit() {
	lru := NewLRU[int, int](0, zeroSizeOf[int, int])
	overhead := lru.PerEntryOverhead()
	lru.SetMaxSize(overhead * 32)

	const goroutines = 8
	const ops = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := (id*ops + i) % 128
				lru.Put(key, id*ops+i)
				lru.Get(key)
			}
		}(g)
	}
	wg.Wait()

	s.assert.LessOrEqual(lru.Size(), overhead*32)
}

func TestLRUCacheTestSuite(t *testing.T) {
	suite.Run(t, new(lruCacheTestSuite))
}
