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
	Last       bool // last block in the file?
}

type DiskBlock struct {
	StartIndex int64
	EndIndex   int64
	Path       string
}

//LRUCache definition for Least Recently Used Cache implementation.
type LRUCache struct {
	Capacity        int64                      //defines a cache object of the specified capacity.
	List            *list.List                 //DoublyLinkedList for backing the cache value.
	Elements        map[BlockKey]*list.Element //blockKey: KeyPair
	Occupied        int64
	DiskPersistence bool
	DiskCapacity    int64
	DiskList        *list.List
	DiskElements    map[BlockKey]*list.Element //blockKey: DiskBlock
	DiskPath        string
	DiskOccupied    int64
}

//KeyPair: defines the cache structure to be stored in LRUCache
type KeyPair struct {
	key   BlockKey
	value *CacheBlock
}

type DiskKeyPair struct {
	key   BlockKey
	value *DiskBlock
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

// get file name on disk
func (cache *LRUCache) getLocalFilePath(bk BlockKey) string {
	return filepath.Join(cache.DiskPath, bk.Handle.Path+"__"+fmt.Sprintf("%d", bk.StartIndex)+"__")
}

// get file/block from disk
func (cache *LRUCache) GetFromDisk(bk BlockKey, block *CacheBlock) (found bool) {
	if node, ok := cache.DiskElements[bk]; ok {
		diskBlock := node.Value.(*list.Element).Value.(*DiskKeyPair).value
		cache.DiskList.MoveToFront(node)
		f, _ := os.OpenFile(diskBlock.Path, os.O_RDONLY, 0775)
		f.Write(block.Data)
		f.Close()
		return true
	}
	return false
}

// put block on disk, given cache block and handle
func (cache *LRUCache) PutOnDisk(bk BlockKey, block *CacheBlock) {
	if cache.DiskOccupied >= cache.DiskCapacity {
		// find LRU disk block
		block := cache.DiskList.Back().Value.(*list.Element).Value.(*DiskKeyPair)
		cache.RemoveFromDisk(block)
	}
	// create local path for the new disk block
	localPath := cache.getLocalFilePath(bk)
	node := &list.Element{
		Value: DiskKeyPair{
			key: bk,
			value: &DiskBlock{
				StartIndex: block.StartIndex,
				EndIndex:   block.EndIndex,
				Path:       localPath,
			},
		},
	}
	pointer := cache.DiskList.PushFront(node)
	cache.DiskOccupied += (block.EndIndex - block.StartIndex)
	cache.DiskElements[bk] = pointer

	os.MkdirAll(filepath.Dir(localPath), os.FileMode(0775))
	f, _ := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	f.Write(node.Value.(*CacheBlock).Data)
	f.Close()
}

func (cache *LRUCache) PurgeDisk() {
	for _, value := range cache.DiskElements {
		cache.RemoveFromDisk(value.Value.(*list.Element).Value.(*DiskKeyPair))
	}
}

func (cache *LRUCache) RemoveFromDisk(pair *DiskKeyPair) {
	// clean the block data to not leak any memory
	localPath := cache.getLocalFilePath(pair.key)
	os.Remove(localPath)
	dirPath := filepath.Dir(localPath)
	if dirPath != cache.DiskPath {
		os.Remove(filepath.Dir(localPath))
	}
	if node, ok := cache.Elements[pair.key]; ok {
		cache.DiskOccupied -= (node.Value.(*CacheBlock).EndIndex - node.Value.(*CacheBlock).StartIndex)
		delete(cache.DiskElements, pair.key)
		cache.List.Remove(node)
	}
}

// STILL WORK TO DO: MOVING THE DISK CACHE HIT TO THE MEM CACHE
//Get: returns the cache value stored for the key, also moves the list pointer to front of the list
func (cache *LRUCache) Get(key BlockKey, handle *Handle, handleProvided bool) (*CacheBlock, bool) {
	if handleProvided {
		if node, ok := cache.Elements[key]; ok {
			value := node.Value.(*list.Element).Value.(KeyPair).value
			cache.List.MoveToFront(node)
			if handleProvided {
				handle.CacheObj.DataBuffer.Get(key, &Handle{}, false)
			}
			return value, true
		} else {
			if node, ok := cache.DiskElements[key]; ok {
				diskBlock := node.Value.(*list.Element).Value.(DiskKeyPair).value
				newBlock := &CacheBlock{
					StartIndex: diskBlock.StartIndex,
					EndIndex:   diskBlock.EndIndex,
					Data:       make([]byte, diskBlock.EndIndex-diskBlock.StartIndex),
					Last:       diskBlock.EndIndex >= handle.Size,
				}
				cache.DiskList.MoveToFront(node)
				f, _ := os.OpenFile(diskBlock.Path, os.O_RDONLY, 0775)
				f.Write(newBlock.Data)
				f.Close()
				return newBlock, true
			}
		}
		return &CacheBlock{}, false
	} else {
		if node, ok := cache.Elements[key]; ok {
			cache.List.MoveToFront(node)
		}
		return nil, true
	}
}

//Put: Inserts the key,value pair in LRUCache.
//If list capacity is full, entry at the last index of the list is deleted before insertion.
func (cache *LRUCache) Put(key BlockKey, value *CacheBlock, handle *Handle, handleProvided bool) {
	if cache.Occupied >= cache.Capacity {
		pair := cache.List.Back().Value.(*list.Element).Value.(KeyPair)
		cache.Remove(pair.key, true, cache.DiskPersistence)
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
func (cache *LRUCache) Keys() []interface{} {
	var keys []interface{}
	for k := range cache.Elements {
		keys = append(keys, k)
	}
	return keys
}

func (cache *LRUCache) RecentlyUsed() interface{} {
	return cache.List.Front().Value.(*list.Element).Value.(KeyPair).value
}

//Remove: removes the entry for the respective key
func (cache *LRUCache) Remove(key BlockKey, handleProvided, presistOnDisk bool) {
	// get the keyPair associated with the blockKey
	if node, ok := cache.Elements[key]; ok {
		// remove from capacity
		cache.Occupied -= node.Value.(KeyPair).value.EndIndex - node.Value.(KeyPair).value.StartIndex
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
	for _, block := range cache.Keys() {
		cache.Remove(block.(BlockKey), true, false)
	}
	cache.Capacity = 0
	cache.Elements = nil
	cache.List = nil
}

//Purge: clears handle LRUCache
func (cache *LRUCache) PurgeHandle(handle *Handle) {
	for _, key := range handle.CacheObj.DataBuffer.Keys() {
		cache.Remove(key.(BlockKey), true, false)
	}
	handle.CacheObj.DataBuffer.Purge()
}
