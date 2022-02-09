/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"sync"

	"github.com/bluele/gcache"
)

type blockKey struct {
	offset  int64
	fileKey string
}
type cacheBlock struct {
	sync.RWMutex
	startIndex int64
	endIndex   int64
	data       []byte
	last       bool // last block in the file?
}

type cacheFile struct {
	openHandles     int
	fileBlockBuffer gcache.Cache // contains all block pointers stored for a given file: {blockKey(off1, fileKey3): cacheBlock1, blockKey(off1, fileKey3): cacheBlock2, ...}
}
type cache struct {
	blocks           gcache.Cache          // blocks stored: {blockKey(off1, fileKey1): cacheBlock1, blockKey(off1, fileKey2): cacheBlock2, ...}
	files            map[string]*cacheFile // current files stored and their respective blocks references/pointers currently stored: {fileName1: cacheFile, fileName2: cacheFile2, ....}
	blockSize        int64
	blocksPerFileKey int      // maximum number of blocks allowed to be stored for a file
	maxBlocks        int      // maximum allowed configured number of blocks
	evictedBlock     blockKey // if a block gets removed from our block entries/main cache we need to delete the block reference from its respective file buffer
	evictionPolicy   common.EvictionPolicy
	sync.RWMutex
}

// on file handle closures decrement handles
func (c *cache) decrementHandles(fileKey string) int {
	c.Lock()
	defer c.Unlock()
	f := c.files[fileKey]
	f.openHandles -= 1
	return f.openHandles
}

// on file opens we increment handles for a given file
func (c *cache) incrementHandles(fileKey string) {
	c.Lock()
	defer c.Unlock()
	f := c.files[fileKey]
	f.openHandles += 1
}

// add a new file key and create a cache object for it to hold references to its current stored blocks
func (c *cache) addFileKey(fileKey string) {
	c.Lock()
	defer c.Unlock()
	_, ok := c.files[fileKey]
	if !ok {
		log.Trace("streamcache:: file key %s not found initializing new key", fileKey)
		//EvictedFunc is a callback that allows us to make sure we remove the corresponding blocks from the block cache
		// PurgeVisitorFunc is a callback that allows us to walk through each block cache entry as we're purging it and perform a cleanup operation we want
		var fc gcache.Cache
		switch c.evictionPolicy {
		case common.EPolicy.LFU():
			fc = gcache.New(c.blocksPerFileKey).LRU().EvictedFunc(c.fileEvict).PurgeVisitorFunc(c.filePurge).Build()
		case common.EPolicy.ARC():
			fc = gcache.New(c.blocksPerFileKey).LFU().EvictedFunc(c.fileEvict).PurgeVisitorFunc(c.filePurge).Build()
		default:
			fc = gcache.New(c.blocksPerFileKey).ARC().EvictedFunc(c.fileEvict).PurgeVisitorFunc(c.filePurge).Build()
		}
		c.files[fileKey] = &cacheFile{
			fileBlockBuffer: fc,
		}
	}
}

func (c *cache) removeFileKey(fileKey string) {
	c.Lock()
	defer c.Unlock()
	// remove all blocks stored for the file key
	// the purge walker callback will happen which will walk through all the block refs for this file and remove it from the main block cache
	c.files[fileKey].fileBlockBuffer.Purge()
	// delete the entry from our map once complete
	delete(c.files, fileKey)
}

// remove file keys with a given prefix
// func (c *cache) removeWithPrefix(prefix string) {
// 	log.Trace("streamcache:: checking cached file keys within %s", prefix)
// 	c.Lock()
// 	defer c.Unlock()
// 	for fileKey := range c.files {
// 		if strings.HasPrefix(fileKey, prefix) {
// 			c.files[fileKey].fileBlockBuffer.Purge()
// 			delete(c.files, fileKey)
// 		}
// 	}
// }

// try to retrieve the block - return missing if it is not cached
func (c *cache) getBlock(fileKey string, offset int64, fileSize int64) (*cacheBlock, bool) {
	blockSize := c.blockSize
	blockKeyObj := blockKey{fileKey: fileKey, offset: offset}
	c.Lock()
	defer c.Unlock()
	// this adds a cache hit to the file buffer if the block exists then down we're doing a cache hit on the block cache as well
	block, err := c.files[fileKey].fileBlockBuffer.Get(blockKeyObj)

	// block was not found - create a new block, append it to cache and return it
	if err == gcache.KeyNotFoundError {
		if (offset + blockSize) > fileSize {
			blockSize = fileSize - offset
		}
		newBlock := &cacheBlock{
			startIndex: offset,
			endIndex:   offset + blockSize,
			data:       make([]byte, blockSize),
			last:       (offset + blockSize) >= fileSize,
		}
		// lock because we will be changing the block since its data buffer is currently empty
		newBlock.Lock()

		// if we hit the max num of blocks stored for a given file then set it on the file enteries/buffer first
		// this would get it evicted on the file buffer level and the respective block from the block cache and avoid double evicting
		// this calls filePurgeOrEvict callback which will trigger the block cache to delete the corresponding block
		c.files[fileKey].fileBlockBuffer.Set(blockKeyObj, newBlock)
		// clear the evicted block entry since it was already removed from its file buffer
		c.evictedBlock = blockKey{}
		c.blocks.Set(blockKeyObj, newBlock)
		// if a block was evicted in the process then we have to remove it from its file buffer as well
		if c.evictedBlock != (blockKey{}) {
			// get the evicted block file key and remove it from that file key's buffer
			c.files[c.evictedBlock.fileKey].fileBlockBuffer.Remove(c.evictedBlock)
			// if this was the last block stored for this file then we can remove the entry from our map
			// TODO: we can add a cache timeout in the future
			if c.files[c.evictedBlock.fileKey].fileBlockBuffer.Len(false) == 0 && c.evictedBlock.fileKey != blockKeyObj.fileKey {
				delete(c.files, c.evictedBlock.fileKey)
			}
			c.evictedBlock = blockKey{}
		}
		return newBlock, false
	} else {
		block.(*cacheBlock).RLock()
		c.blocks.Get(blockKeyObj)
	}
	return block.(*cacheBlock), true
}

func (c *cache) teardown() {
	log.Trace("streamcache:: tearing down stream cache")
	c.Lock()
	defer c.Unlock()
	for fileKey := range c.files {
		c.files[fileKey].fileBlockBuffer.Purge()
		delete(c.files, fileKey)
	}
}

func (c *cache) fileEvict(key, value interface{}) {
	c.blocks.Remove(key)
}

func (c *cache) filePurge(key, value interface{}) {
	log.Trace("streamcache:: purging file key %s", key)
	c.blocks.Remove(key)
	c.evictedBlock = blockKey{}
}
