package common

import (
	"container/list"
	"fmt"
	"sync"
)

type CacheBlock struct {
	sync.RWMutex
	StartIndex int64
	EndIndex   int64
	Data       []byte
	Last       bool //last block in the file?
}

//KeyPair: the list node containing both block key and cache block values
type KeyPair struct {
	key   int64
	value *CacheBlock
}

//LRUCache definition for Least Recently Used Cache implementation
type LRUCache struct {
	sync.RWMutex
	Capacity int64
	List     *list.List              //DoublyLinkedList: node1->node2.... node:=KeyPair
	Elements map[int64]*list.Element //blockKey:KeyPair
	Occupied int64
}

//NewLRUCache: creates a new LRUCache object with the defined capacity
func NewLRUCache(capacity int64) *LRUCache {
	return &LRUCache{
		Capacity: capacity,
		List:     new(list.List),
		Elements: make(map[int64]*list.Element),
	}
}

//Get: returns the cache value stored for the key, cache hits the handle and moves the list pointer to front of the list
func (cache *LRUCache) Get(bk int64) (*CacheBlock, bool) {
	found := false
	var cb *CacheBlock
	if node, ok := cache.Elements[bk]; ok {
		cb = node.Value.(*list.Element).Value.(KeyPair).value
		cache.List.MoveToFront(node)
		found = true
		cb.RLock()
	}
	return cb, found
}

//Put: Inserts the key,value pair in LRUCache.
func (cache *LRUCache) Put(key int64, value *CacheBlock) {
	if cache.Occupied >= cache.Capacity {
		pair := cache.List.Back().Value.(*list.Element).Value.(KeyPair)
		cache.Remove(pair.key)
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
}

func (cache *LRUCache) Print() {
	for _, value := range cache.Elements {
		fmt.Printf("Key:%+v,Value:%+v\n", value.Value.(*list.Element).Value.(KeyPair).value.StartIndex, value.Value.(*list.Element).Value.(KeyPair).value.EndIndex)
	}
}

//Keys: returns all the keys present in LRUCache
func (cache *LRUCache) Keys() []int64 {
	var keys []int64
	for k := range cache.Elements {
		keys = append(keys, k)
	}
	return keys
}

func (cache *LRUCache) RecentlyUsed() *CacheBlock {
	return cache.List.Front().Value.(*list.Element).Value.(KeyPair).value
}

//Remove: removes the entry for the respective key
func (cache *LRUCache) Remove(key int64) {
	// get the keyPair associated with the blockKey
	if node, ok := cache.Elements[key]; ok {
		node.Value.(*list.Element).Value.(KeyPair).value.Lock()
		defer node.Value.(*list.Element).Value.(KeyPair).value.Unlock()
		// remove from capacity
		cache.Occupied -= node.Value.(*list.Element).Value.(KeyPair).value.EndIndex - node.Value.(*list.Element).Value.(KeyPair).value.StartIndex
		//if handle is not provided then we're on the handle cache we can just remove it from cache
		node.Value.(*list.Element).Value.(KeyPair).value.Data = nil
		delete(cache.Elements, key)
		cache.List.Remove(node)
	}
}

//Purge: clears LRUCache
func (cache *LRUCache) Purge() {
	for _, bk := range cache.Keys() {
		cache.Remove(bk)
	}
	cache.Capacity = 0
	cache.Elements = nil
	cache.List = nil
}
