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
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// KeyPair: the list node containing both block key and cache block values
type KeyPair struct {
	key   int64
	value *common.Block
}

// LRUCache definition for Least Recently Used Cache implementation
type LRUCache struct {
	sync.RWMutex
	Capacity int64
	List     *list.List              //DoublyLinkedList: node1->node2.... node:=KeyPair
	Elements map[int64]*list.Element //blockKey:KeyPair
	Occupied int64
}

// NewLRUCache: creates a new LRUCache object with the defined capacity
func NewLRUCache(capacity int64) *LRUCache {
	return &LRUCache{
		Capacity: capacity,
		List:     new(list.List),
		Elements: make(map[int64]*list.Element),
	}
}

// Get: returns the cache value stored for the key, cache hits the handle and moves the list pointer to front of the list
func (cache *LRUCache) Get(bk int64) (*common.Block, bool) {
	found := false
	var cb *common.Block
	if node, ok := cache.Elements[bk]; ok {
		cb = getKeyPair(node).value
		cache.List.MoveToFront(node)
		found = true
	}
	return cb, found
}

// Resize: resizes a cached block and adjusts occupied size
func (cache *LRUCache) Resize(bk, newEndIndex int64) bool {
	var cb *common.Block
	if node, ok := cache.Elements[bk]; ok {
		cb = getKeyPair(node).value
		sizeDiff := newEndIndex - cb.EndIndex
		cache.Occupied += sizeDiff
		cb.EndIndex = newEndIndex
		return true
	}
	return false
}

// Put: Inserts the key,value pair in LRUCache. Return false if failed.
func (cache *LRUCache) Put(key int64, value *common.Block) bool {
	if cache.Occupied >= cache.Capacity {
		evicted := cache.evict()
		if !evicted {
			return false
		}
	}
	node := &list.Element{
		Value: KeyPair{
			key:   key,
			value: value,
		},
	}
	pointer := cache.List.PushFront(node)
	cache.Occupied += (node.Value.(KeyPair).value.EndIndex - node.Value.(KeyPair).value.StartIndex)
	cache.Elements[key] = pointer
	return true
}

func (cache *LRUCache) Print() {
	for _, value := range cache.Elements {
		log.Debug("Key:%+v,Value:%+v\n", getKeyPair(value).value.StartIndex, getKeyPair(value).value.EndIndex)
	}
}

// Keys: returns all the keys present in LRUCache
func (cache *LRUCache) Keys() []int64 {
	var keys []int64
	for k := range cache.Elements {
		keys = append(keys, k)
	}
	return keys
}

func (cache *LRUCache) RecentlyUsed() *common.Block {
	return getKeyPair(cache.List.Front()).value
}

func (cache *LRUCache) LeastRecentlyUsed() *common.Block {
	return getKeyPair(cache.List.Back()).value
}

// Remove: removes the entry for the respective key
func (cache *LRUCache) Remove(key int64) {
	// get the keyPair associated with the blockKey
	if node, ok := cache.Elements[key]; ok {
		nodeKeyPair := getKeyPair(node)
		nodeKeyPair.value.Lock()
		defer nodeKeyPair.value.Unlock()
		// remove from capacity
		cache.Occupied -= nodeKeyPair.value.EndIndex - nodeKeyPair.value.StartIndex
		// if handle is not provided then we're on the handle cache we can just remove it from cache
		nodeKeyPair.value.Data = nil
		delete(cache.Elements, key)
		cache.List.Remove(node)
	}
}

// Purge: clears LRUCache
func (cache *LRUCache) Purge() {
	for _, bk := range cache.Keys() {
		cache.Remove(bk)
	}
	cache.Capacity = 0
	cache.Elements = nil
	cache.List = nil
}

func getKeyPair(node *list.Element) KeyPair {
	// uncast the keypair
	return node.Value.(*list.Element).Value.(KeyPair)
}

// return true if eviction happened, return false otherwise
func (cache *LRUCache) evict() bool {
	node := cache.List.Back()
	pair := getKeyPair(node)
	for i := 0; i < cache.List.Len(); i++ {
		if !pair.value.Dirty() {
			cache.Remove(pair.key)
			return true
		}
		node = node.Prev()
		if node == nil {
			return false
		}
		pair = getKeyPair(node)
	}
	return false
}
