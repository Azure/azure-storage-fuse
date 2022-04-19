package handlemap

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type BlockKey struct {
	StartIndex int64
	Handle     *Handle
}

type CacheBlock struct {
	sync.RWMutex
	StartIndex int64
	EndIndex   int64
	Data       []byte
	Last       bool //last block in the file?
}

type DiskBlock struct {
	StartIndex int64
	EndIndex   int64
	Path       string //disk path to the block
}

//Key Pair: the list node containing both block key and cache block values
type KeyPair struct {
	key   BlockKey
	value *CacheBlock
}

type DiskKeyPair struct {
	key   BlockKey
	value *DiskBlock
}

//LRUCache definition for Least Recently Used Cache implementation
type LRUCache struct {
	Capacity        int64
	List            *list.List                 //DoublyLinkedList: node1->node2.... node:=Key Pair
	Elements        map[BlockKey]*list.Element //blockKey:Key Pair
	Occupied        int64
	DiskPersistence bool
	DiskCapacity    int64
	DiskList        *list.List                 //DoublyLinkedList: node1->node2.... node:=Disk Key Pair
	DiskElements    map[BlockKey]*list.Element //blockKey:Disk Key Pair
	DiskPath        string
	DiskOccupied    int64
}

//NewLRUCache: creates a new LRUCache object with the defined capacity
func NewLRUCache(diskPresistence bool, diskPath string, capacity, diskCapacity int64) LRUCache {
	var diskElements map[BlockKey]*list.Element = nil
	if diskPresistence {
		diskElements = make(map[BlockKey]*list.Element)
	}
	return LRUCache{
		Capacity:        capacity,
		List:            new(list.List),
		Elements:        make(map[BlockKey]*list.Element),
		DiskPersistence: diskPresistence,
		DiskCapacity:    diskCapacity,
		DiskElements:    diskElements,
		DiskPath:        diskPath,
		DiskOccupied:    0,
	}
}

//get file name on disk
func (cache *LRUCache) getLocalFilePath(bk BlockKey) string {
	return filepath.Join(cache.DiskPath, bk.Handle.Path+"__"+fmt.Sprintf("%d", bk.StartIndex)+"__")
}

//get file/block from disk
func (cache *LRUCache) GetFromDisk(bk BlockKey, handle *Handle) (*CacheBlock, bool) {
	if node, ok := cache.DiskElements[bk]; ok {
		//get the disk block
		diskKeyPair := node.Value.(*list.Element).Value.(DiskKeyPair)
		cb := &CacheBlock{
			StartIndex: diskKeyPair.value.StartIndex,
			EndIndex:   diskKeyPair.value.EndIndex,
			Data:       make([]byte, diskKeyPair.value.EndIndex-diskKeyPair.value.StartIndex),
			Last:       diskKeyPair.value.EndIndex >= handle.Size,
		}
		cache.DiskList.MoveToFront(node)
		f, _ := os.OpenFile(diskKeyPair.value.Path, os.O_RDONLY, 0775)
		//write to cache block (moving data to L1 cache)
		f.Read(cb.Data)
		f.Close()
		return cb, true
	}
	return &CacheBlock{}, false
}

//put block on disk, given cache block and handle
func (cache *LRUCache) PutOnDisk(bk BlockKey, cb *CacheBlock) {
	if cache.DiskOccupied >= cache.DiskCapacity {
		//find LRU disk diskKeyPair
		bk := cache.DiskList.Back().Value.(*list.Element).Value.(*DiskKeyPair).key
		cache.RemoveFromDisk(bk)
	}
	// create local path for the new disk block
	localPath := cache.getLocalFilePath(bk)
	node := &list.Element{
		Value: DiskKeyPair{
			key: bk,
			value: &DiskBlock{
				StartIndex: cb.StartIndex,
				EndIndex:   cb.EndIndex,
				Path:       localPath,
			},
		},
	}
	pointer := cache.DiskList.PushFront(node)
	cache.DiskOccupied += (cb.EndIndex - cb.StartIndex)
	cache.DiskElements[bk] = pointer

	os.MkdirAll(filepath.Dir(localPath), os.FileMode(0775))
	f, _ := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	f.Write(node.Value.(*CacheBlock).Data)
	f.Close()
}

//purge disk cache
func (cache *LRUCache) PurgeDisk() {
	for _, value := range cache.DiskElements {
		cache.RemoveFromDisk(value.Value.(*list.Element).Value.(*DiskKeyPair).key)
	}
}

//remove block from disk
func (cache *LRUCache) RemoveFromDisk(bk BlockKey) {
	// clean the block data to not leak any memory
	localPath := cache.getLocalFilePath(bk)
	os.Remove(localPath)
	dirPath := filepath.Dir(localPath)
	if dirPath != cache.DiskPath {
		os.Remove(filepath.Dir(localPath))
	}
	if node, ok := cache.Elements[bk]; ok {
		cache.DiskOccupied -= (node.Value.(*CacheBlock).EndIndex - node.Value.(*CacheBlock).StartIndex)
		delete(cache.DiskElements, bk)
		cache.List.Remove(node)
	}
}

//Get: returns the cache value stored for the key, cache hits the handle and moves the list pointer to front of the list
func (cache *LRUCache) Get(bk BlockKey, handle *Handle, handleProvided bool) (*CacheBlock, bool) {
	var found bool
	var cb *CacheBlock
	//if handle is provided then we're still looking for the block
	if handleProvided {
		// check L1 cache for block
		if node, ok := cache.Elements[bk]; ok {
			cb = node.Value.(*list.Element).Value.(KeyPair).value
			cache.List.MoveToFront(node)
			//cache hit the handle
			if handleProvided {
				handle.CacheObj.DataBuffer.Get(bk, &Handle{}, false)
			}
			found = true
		} else {
			//block was not found in L1 cache - look on disk
			cb, found = cache.GetFromDisk(bk, handle)
			if found {
				//bring back to L1
				cache.Put(bk, cb, handle, true)
				//remove from L2
				cache.RemoveFromDisk(bk)
			}
		}
		//block not found
		return cb, found
	} else {
		//handle is not provided therefore we just want to do a cache hit on the handle cache
		if node, ok := cache.Elements[bk]; ok {
			cache.List.MoveToFront(node)
		}
		return nil, true
	}
}

//Put: Inserts the key,value pair in LRUCache.
func (cache *LRUCache) Put(key BlockKey, value *CacheBlock, handle *Handle, handleProvided bool) {
	if cache.Occupied >= cache.Capacity {
		pair := cache.List.Back().Value.(*list.Element).Value.(KeyPair)
		cache.Remove(pair.key, handleProvided, cache.DiskPersistence)
	}
	node := &list.Element{
		Value: KeyPair{
			key:   key,
			value: value,
		},
	}
	pointer := cache.List.PushFront(node)
	cache.Occupied += (node.Value.(*CacheBlock).EndIndex - node.Value.(*CacheBlock).StartIndex)
	cache.Elements[key] = pointer
	if handleProvided {
		handle.CacheObj.DataBuffer.Put(key, value, &Handle{}, false)
	}
}

func (cache *LRUCache) Print() {
	for key, value := range cache.Elements {
		fmt.Printf("Key:%s,Value:%+v\n", key.Handle.Path, value.Value.(*list.Element).Value.(KeyPair).value.StartIndex)
	}
}

//Keys: returns all the keys present in LRUCache
func (cache *LRUCache) Keys() []BlockKey {
	var keys []BlockKey
	for k := range cache.Elements {
		keys = append(keys, k)
	}
	return keys
}

func (cache *LRUCache) RecentlyUsed() *CacheBlock {
	return cache.List.Front().Value.(*list.Element).Value.(KeyPair).value
}

//Remove: removes the entry for the respective key
func (cache *LRUCache) Remove(key BlockKey, handleProvided, presistOnDisk bool) {
	// get the key Pair associated with the blockKey
	if node, ok := cache.Elements[key]; ok {
		// remove from capacity
		cache.Occupied -= node.Value.(KeyPair).value.EndIndex - node.Value.(KeyPair).value.StartIndex
		//if handle is not provided then we're on the handle cache we can just remove it from cache
		if handleProvided {
			// put block on disk if we want to presist it
			if presistOnDisk {
				cache.PutOnDisk(key, node.Value.(*list.Element).Value.(KeyPair).value)
			}
			node.Value.(*CacheBlock).Data = nil
		}
		delete(cache.Elements, key)
		cache.List.Remove(node)
	}
	if handleProvided {
		key.Handle.CacheObj.DataBuffer.Remove(key, false, false)
	}
}

//Purge: clears LRUCache
func (cache *LRUCache) Purge() {
	for _, bk := range cache.Keys() {
		cache.Remove(bk, true, false)
	}
	cache.Capacity = 0
	cache.Elements = nil
	cache.List = nil
	cache.PurgeDisk()
}

//Purge: clears handle LRUCache
func (cache *LRUCache) PurgeHandle(handle *Handle) {
	for _, bk := range handle.CacheObj.DataBuffer.Keys() {
		//we don't want to presist on disk when purging a handle's blocks
		cache.Remove(bk, true, false)
	}
}
