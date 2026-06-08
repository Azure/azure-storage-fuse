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
	"unsafe"
)

// mapBucketOverhead estimates the per-entry overhead from Go's internal hash map structure.
// Go's map uses a bucket-based design; 64 B covers the amortised cost of key/value copies
// and tophash bookkeeping per entry.
const mapBucketOverhead = 64

// lruItem is the value stored inside each list.Element in an LRU[K,V].
type lruItem[K comparable, V any] struct {
	key      K
	val      V
	userSize int64 // sizeOf(key, val) frozen at insertion time
}

// LRU is a memory-bounded, generic least-recently-used cache.
//
// It is safe for concurrent use by multiple goroutines.
//
// Memory accounting works in two layers:
//   - perEntryOverhead: fixed structural bytes per entry (list.Element + lruItem + map
//     bucket), computed once in NewLRU via unsafe.Sizeof for the concrete K/V types.
//   - sizeOf(k, v): caller-provided estimate of the heap bytes for one key-value pair,
//     covering any heap allocations reachable from k and v (e.g. string data, pointed-to
//     structs).  It should NOT include the structural overhead above.
type LRU[K comparable, V any] struct {
	mu               sync.RWMutex
	items            map[K]*list.Element
	list             *list.List
	maxSize          int64 // 0 = no limit
	currSize         int64
	perEntryOverhead int64
	sizeOf           func(K, V) int64
}

// NewLRU creates an LRU with the given memory limit (bytes) and size estimator.
// maxSize == 0 disables memory-based eviction.
func NewLRU[K comparable, V any](maxSize int64, sizeOf func(K, V) int64) *LRU[K, V] {
	var zeroItem lruItem[K, V]
	overhead := int64(unsafe.Sizeof(zeroItem)) +
		int64(unsafe.Sizeof(list.Element{})) +
		mapBucketOverhead
	return &LRU[K, V]{
		items:            make(map[K]*list.Element),
		list:             list.New(),
		maxSize:          maxSize,
		sizeOf:           sizeOf,
		perEntryOverhead: overhead,
	}
}

// Put inserts or updates a key-value pair and promotes it to MRU.
// After insertion, LRU entries are evicted until currSize <= maxSize.
func (l *LRU[K, V]) Put(key K, val V) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.put(key, val)
}

// put is the lock-free inner implementation, called with l.mu write-locked.
func (l *LRU[K, V]) put(key K, val V) {
	userSize := l.sizeOf(key, val)
	if elem, ok := l.items[key]; ok {
		item := elem.Value.(*lruItem[K, V])
		l.currSize -= item.userSize
		item.val = val
		item.userSize = userSize
		l.currSize += userSize
		l.list.MoveToFront(elem)
		l.evictIfNeeded()
		return
	}
	item := &lruItem[K, V]{key: key, val: val, userSize: userSize}
	elem := l.list.PushFront(item)
	l.items[key] = elem
	l.currSize += l.perEntryOverhead + userSize
	l.evictIfNeeded()
}

// Get retrieves a value and promotes it to MRU.  Returns (zero, false) on miss.
func (l *LRU[K, V]) Get(key K) (V, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.items[key]; ok {
		l.list.MoveToFront(elem)
		return elem.Value.(*lruItem[K, V]).val, true
	}
	var zero V
	return zero, false
}

// Peek retrieves a value without changing LRU order.  Returns (zero, false) on miss.
func (l *LRU[K, V]) Peek(key K) (V, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if elem, ok := l.items[key]; ok {
		return elem.Value.(*lruItem[K, V]).val, true
	}
	var zero V
	return zero, false
}

// Has reports whether key is present without changing LRU order.
func (l *LRU[K, V]) Has(key K) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.items[key]
	return ok
}

// Delete removes an entry from the cache.  No-op if the key is absent.
func (l *LRU[K, V]) Delete(key K) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.delete(key)
}

// delete is the lock-free inner implementation, called with l.mu write-locked.
func (l *LRU[K, V]) delete(key K) {
	if elem, ok := l.items[key]; ok {
		item := elem.Value.(*lruItem[K, V])
		l.currSize -= l.perEntryOverhead + item.userSize
		delete(l.items, key)
		l.list.Remove(elem)
	}
}

// Len returns the number of entries currently in the cache.
func (l *LRU[K, V]) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.items)
}

// Size returns the current estimated memory usage in bytes.
func (l *LRU[K, V]) Size() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currSize
}

// MaxSize returns the configured memory limit (0 = no limit).
func (l *LRU[K, V]) MaxSize() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.maxSize
}

// PerEntryOverhead returns the structural bytes added per entry (useful for testing).
// This value is immutable after construction and requires no locking.
func (l *LRU[K, V]) PerEntryOverhead() int64 {
	return l.perEntryOverhead
}

// SetMaxSize changes the memory limit and immediately evicts LRU entries if over the new limit.
func (l *LRU[K, V]) SetMaxSize(newMax int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxSize = newMax
	l.evictIfNeeded()
}

// Range calls fn for every entry in MRU→LRU order.
//
// The cache is read-locked for the duration of the call. fn must not call any
// LRU methods (Put, Get, Delete, etc.) — doing so will deadlock. Mutating a
// value pointed to by V is safe as long as the caller manages value-level
// concurrency separately.
func (l *LRU[K, V]) Range(fn func(K, V) bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for elem := l.list.Front(); elem != nil; elem = elem.Next() {
		item := elem.Value.(*lruItem[K, V])
		if !fn(item.key, item.val) {
			return
		}
	}
}

// Purge removes all entries.
func (l *LRU[K, V]) Purge() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[K]*list.Element)
	l.list.Init()
	l.currSize = 0
}

// DeleteIf removes all entries for which pred returns true.
// It holds the write lock for the entire scan — single pass, no intermediate allocation.
func (l *LRU[K, V]) DeleteIf(pred func(K, V) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for elem := l.list.Front(); elem != nil; {
		item := elem.Value.(*lruItem[K, V])
		next := elem.Next()
		if pred(item.key, item.val) {
			l.currSize -= l.perEntryOverhead + item.userSize
			delete(l.items, item.key)
			l.list.Remove(elem)
		}
		elem = next
	}
}

// ReplaceIf replaces the value of every entry for which pred returns true with newVal(key).
// The factory is called once per matching entry so each gets its own value.
// It holds the write lock for the entire scan — no entry matching the predicate can be
// inserted or removed by another goroutine between the scan and the replacements.
func (l *LRU[K, V]) ReplaceIf(pred func(K, V) bool, newVal func(K) V) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for elem := l.list.Front(); elem != nil; {
		next := elem.Next()
		item := elem.Value.(*lruItem[K, V])
		if pred(item.key, item.val) {
			v := newVal(item.key)
			newSize := l.sizeOf(item.key, v)
			l.currSize += newSize - item.userSize
			item.val = v
			item.userSize = newSize
			l.list.MoveToFront(elem)
		}
		elem = next
	}
	l.evictIfNeeded()
}

// evictIfNeeded removes the least-recently-used entries until currSize <= maxSize.
// Must be called with l.mu write-locked.
func (l *LRU[K, V]) evictIfNeeded() {
	if l.maxSize <= 0 {
		return
	}
	for l.currSize > l.maxSize && l.list.Len() > 0 {
		l.evictLast()
	}
}

// evictLast removes the least-recently-used entry.
// Must be called with l.mu write-locked.
func (l *LRU[K, V]) evictLast() {
	elem := l.list.Back()
	if elem == nil {
		return
	}
	item := elem.Value.(*lruItem[K, V])
	l.currSize -= l.perEntryOverhead + item.userSize
	delete(l.items, item.key)
	l.list.Remove(elem)
}
