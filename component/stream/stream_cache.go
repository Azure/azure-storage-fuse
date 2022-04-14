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
	"blobfuse2/internal/handlemap"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bluele/gcache"
)

type blockKey struct {
	offset int64
	handle *handlemap.Handle
}
type cacheBlock struct {
	sync.RWMutex
	startIndex int64
	endIndex   int64
	data       []byte
	last       bool // last block in the file?
}

type cache struct {
	sync.RWMutex

	evictedBlock   blockKey // if a block gets removed from our block entries/main cache we need to delete the block reference from its respective file buffer
	evictionPolicy common.EvictionPolicy
	blocks         gcache.Cache // blocks stored: {blockKey(off1, fileKey1): cacheBlock1, blockKey(off1, fileKey2): cacheBlock2, ...}

	blockSize        int64
	blocksPerFileKey int // maximum number of blocks allowed to be stored for a file
	maxBlocks        int // maximum allowed configured number of blocks

	diskBlocks      gcache.Cache // blocks stored on disk when persistence is on
	diskPersistence bool         // When block is evicted from memory shall be stored on disk for some more time
	diskPath        string       // Location where persisted blocks will be stored
	diskCacheMB     int64        // Size of disk cache to be used for persistence
	diskTimeoutSec  float64      // Timeout in seconds for the block persisted on disk
}

// add a new file key and create a cache object for it to hold references to its current stored blocks
func (c *cache) addHandleCache(handle *handlemap.Handle) {
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
	cacheObj := handlemap.Cache{
		DataBuffer: fc,
	}
	handle.CacheObj = &cacheObj
}

// try to retrieve the block - return missing if it is not cached
func (c *cache) getBlock(handle *handlemap.Handle, offset int64) (*cacheBlock, bool) {
	blockSize := c.blockSize
	blockKeyObj := blockKey{handle: handle, offset: offset}
	c.Lock()
	defer c.Unlock()
	// this adds a cache hit to the file buffer if the block exists then down we're doing a cache hit on the block cache as well
	block, err := handle.CacheObj.DataBuffer.Get(blockKeyObj)
	// block was not found - create a new block, append it to cache and return it
	if err == gcache.KeyNotFoundError {
		if (offset + blockSize) > handle.Size {
			blockSize = handle.Size - offset
		}
		newBlock := &cacheBlock{
			startIndex: offset,
			endIndex:   offset + blockSize,
			data:       make([]byte, blockSize),
			last:       (offset + blockSize) >= handle.Size,
		}
		newBlock.Lock()
		// if we hit the max num of blocks stored for a given file then set it on the file enteries/buffer first
		// this would get it evicted on the file buffer level and the respective block from the block cache and avoid double evicting
		// this calls filePurgeOrEvict callback which will trigger the block cache to delete the corresponding block
		handle.CacheObj.DataBuffer.Set(blockKeyObj, newBlock)
		// clear the evicted block entry since it was already removed from its file buffer
		c.evictedBlock = blockKey{}
		c.blocks.Set(blockKeyObj, newBlock)
		// if a block was evicted in the process then we have to remove it from its file buffer as well
		if c.evictedBlock != (blockKey{}) {
			// get the evicted block file key and remove it from that file key's buffer
			c.evictedBlock.handle.CacheObj.DataBuffer.Remove(c.evictedBlock)
			// if this was the last block stored for this file then we can remove the entry from our map
			// TODO: we can add a cache timeout in the future
			c.evictedBlock = blockKey{}
		}
		dataFetched := false
		if c.diskPersistence {
			dataFetched = c.getBlockFromDisk(newBlock, blockKeyObj)
		}
		return newBlock, dataFetched
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
	for _, block := range c.blocks.Keys(false) {
		bk := block.(blockKey)
		c.blocks.Remove(bk)
		c.evictedBlock.handle.CacheObj.DataBuffer.Remove(c.evictedBlock)
		c.evictedBlock = blockKey{}
	}
	// Cleanup block residing on disk path
	if c.diskPersistence {
		c.wipeoutDiskCache()
	}
}

func (c *cache) removeCachedHandle(handle *handlemap.Handle) {
	c.Lock()
	defer c.Unlock()
	// remove all blocks stored for the file key
	// the purge walker callback will happen which will walk through all the block refs for this file and remove it from the main block cache
	handle.CacheObj.DataBuffer.Purge()
}

func (c *cache) fileEvict(key, value interface{}) {
	c.blocks.Remove(key)
}

func (c *cache) filePurge(key, value interface{}) {
	log.Trace("streamcache:: purging file key %s", key)
	c.blocks.Remove(key)
	c.evictedBlock = blockKey{}
}

// Using key construct a file name for persisted block
func (c *cache) getLocalFilePath(key blockKey) string {
	return filepath.Join(c.diskPath, key.handle.Path+"__"+fmt.Sprintf("%d", key.offset)+"__")
}

// Persist this block on disk
func (c *cache) persistBlockOnDisk(block *cacheBlock, key blockKey) {
	localPath := c.getLocalFilePath(key)

	log.Debug("streamcache::persistBlockOnDisk : Saving file %s offset %d to disk", key.handle, key.offset)

	err := os.MkdirAll(filepath.Dir(localPath), os.FileMode(0775))
	if err != nil {
		log.Err("streamcache::persistBlockOnDisk : unable to create local directory %s [%s]", localPath, err.Error())
		return
	}

	f, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		log.Err("streamcache::persistBlockOnDisk : Failed to create file for persisting block on disk %s (%s)", localPath, err.Error())
		return
	}
	f.Write(block.data)
	f.Close()

	c.diskBlocks.Set(key, true)
}

// Load data for this block from disk
func (c *cache) getBlockFromDisk(block *cacheBlock, key blockKey) bool {
	localPath := c.getLocalFilePath(key)
	info, err := os.Stat(localPath)

	if err != nil {
		return false
	}

	log.Debug("streamcache::getBlockFromDisk : Reading block for %s offset %d from disk", key.handle, key.offset)
	if time.Since(info.ModTime()).Seconds() > c.diskTimeoutSec {
		// File exists on local disk but disk cache timeout has elapsed
		os.Remove(localPath)
		c.diskBlocks.Remove(key)
		return false
	}

	f, err := os.OpenFile(localPath, os.O_RDONLY, 0775)
	if err != nil {
		return false
	}

	f.Read(block.data)
	f.Close()
	os.Remove(localPath)

	// As this block is read and loaded into memory, we just purge it from disk
	// as its in memory it will always be served from there and when its evicted
	// it will be pushed back to disk, so safe to remove it from disk here.
	c.diskBlocks.Remove(key)

	return true
}

// Remove this block from in-memory cache
func (c *cache) evictBlock(key blockKey, block *cacheBlock) {
	// clean the block data to not leak any memory
	block.Lock()

	if c.diskPersistence {
		c.persistBlockOnDisk(block, key)
	}

	block.data = nil
	c.evictedBlock = key
	block.Unlock()
}

// Remove this block from disk cache
func (c *cache) evictDiskBlock(key blockKey, val bool) {
	// clean the block data to not leak any memory
	localPath := c.getLocalFilePath(key)
	err := os.Remove(localPath)
	if err != nil {
		log.Err("streamcache::evictDiskBlock : Failed to delete file for persisted block %s (%s)", localPath, err.Error())
	}

	dirPath := filepath.Dir(localPath)
	if dirPath != c.diskPath {
		os.Remove(filepath.Dir(localPath))
	}
}

func (c *cache) wipeoutDiskCache() {
	log.Trace("streamcache::evictDiskBlock : Wipe out disk cache in progress")

	dirents, err := os.ReadDir(c.diskPath)
	if err != nil {
		return
	}

	for _, entry := range dirents {
		os.RemoveAll(filepath.Join(c.diskPath, entry.Name()))
	}
}
