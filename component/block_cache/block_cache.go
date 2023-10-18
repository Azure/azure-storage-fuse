/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package block_cache

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/vibhansa-msft/tlru"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type BlockCache struct {
	internal.BaseComponent

	blockSize       uint64          // Size of each block to be cached
	memSize         uint64          // Mem size to be used for caching at the startup
	tmpPath         string          // Disk path where these blocks will be cached
	diskSize        uint64          // Size of disk space allocated for the caching
	diskTimeout     uint32          // Timeout for which disk blocks will be cached
	workers         uint32          // Number of threads working to fetch the blocks
	prefetch        uint32          // Number of blocks to be prefetched
	diskPolicy      *tlru.TLRU      // Disk cache eviction policy
	blockPool       *BlockPool      // Pool of blocks
	threadPool      *ThreadPool     // Pool of threads
	fileLocks       *common.LockMap // Locks for each file_blockid to avoid multiple threads to fetch same block
	fileNodeMap     sync.Map        // Map holding files that are there in our cache
	maxDiskUsageHit bool            // Flag to indicate if we have hit max disk usage
	noPrefetch      bool            // Flag to indicate if prefetch is disabled
	prefetchOnOpen  bool            // Start prefetching on file open call instead of waiting for first read
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize      uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize        uint64 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	TmpPath        string `config:"path" yaml:"path,omitempty"`
	DiskSize       uint64 `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout    uint32 `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`
	PrefetchCount  uint32 `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers        uint32 `config:"parallelism" yaml:"parallelism,omitempty"`
	PrefetchOnOpen bool   `config:"prefetch-on-open" yaml:"prefetch-on-open,omitempty"`
}

const (
	compName              = "block_cache"
	defaultTimeout        = 120
	MAX_POOL_USAGE uint32 = 80
	MIN_POOL_USAGE uint32 = 50
	MIN_PREFETCH          = 5
	MIN_RANDREAD          = 10
	MAX_FAIL_CNT          = 3
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &BlockCache{}

func (bc *BlockCache) Name() string {
	return compName
}

func (bc *BlockCache) SetName(name string) {
	bc.BaseComponent.SetName(name)
}

func (bc *BlockCache) SetNextComponent(nc internal.Component) {
	bc.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not Block the call otherwise pipeline will not start
func (bc *BlockCache) Start(ctx context.Context) error {
	log.Trace("BlockCache::Start : Starting component %s", bc.Name())

	// Start the thread pool and keep it ready for download
	bc.threadPool.Start()

	// If disk caching is enabled then start the disk eviction policy
	if bc.tmpPath != "" {
		err := bc.diskPolicy.Start()
		if err != nil {
			log.Err("BlockCache::Start : failed to start diskpolicy [%s]", err.Error())
			return fmt.Errorf("failed to start  disk-policy for block-cache")
		}
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", bc.Name())

	// Wait for thread pool to stop
	bc.threadPool.Stop()

	// Clear the disk cache on exit
	if bc.tmpPath != "" {
		_ = bc.diskPolicy.Stop()
		_ = bc.TempCacheCleanup()
	}

	return nil
}

// TempCacheCleanup cleans up the local cached contents
func (bc *BlockCache) TempCacheCleanup() error {
	if bc.tmpPath == "" {
		return nil
	}

	log.Info("BlockCache::TempCacheCleanup : Cleaning up temp directory %s", bc.tmpPath)

	dirents, err := os.ReadDir(bc.tmpPath)
	if err != nil {
		log.Err("BlockCache::TempCacheCleanup : Failed to list directory %s [%v]", bc.tmpPath, err.Error())
		return nil
	}

	for _, entry := range dirents {
		os.RemoveAll(filepath.Join(bc.tmpPath, entry.Name()))
	}

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())

	readonly := false
	err := config.UnmarshalKey("read-only", &readonly)
	if err != nil {
		log.Err("BlockCache::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("config error in %s [filesystem is not mounted in read-only mode]", bc.Name())
	}

	// Currently we support readonly mode
	if !readonly {
		log.Err("BlockCache::Configure : config error [filesystem is not mounted in read-only mode]")
		return fmt.Errorf("config error in %s [filesystem is not mounted in read-only mode]", bc.Name())
	}

	conf := BlockCacheOptions{}
	err = config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	}

	bc.blockSize = uint64(16) * _1MB
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSize = conf.BlockSize * _1MB

	}

	bc.memSize = uint64(4192) * _1MB
	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSize = conf.MemSize * _1MB
	}

	bc.diskSize = uint64(4192)
	if config.IsSet(compName + ".disk-size-mb") {
		bc.diskSize = conf.DiskSize
	}
	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
	}

	bc.prefetchOnOpen = conf.PrefetchOnOpen
	bc.prefetch = MIN_PREFETCH
	bc.noPrefetch = false

	if config.IsSet(compName + ".prefetch") {
		bc.prefetch = conf.PrefetchCount
		if bc.prefetch == 0 {
			bc.noPrefetch = true
		} else if conf.PrefetchCount <= (MIN_PREFETCH * 2) {
			log.Err("BlockCache::Configure : Prefetch count can not be less then %v", (MIN_PREFETCH*2)+1)
			return fmt.Errorf("config error in %s [invalid prefetch count]", bc.Name())
		}
	}

	bc.maxDiskUsageHit = false

	bc.workers = 128
	if config.IsSet(compName + ".parallelism") {
		bc.workers = conf.Workers
	}

	bc.tmpPath = ""
	if conf.TmpPath != "" {
		bc.tmpPath = common.ExpandPath(conf.TmpPath)

		// Extract values from 'conf' and store them as you wish here
		_, err = os.Stat(bc.tmpPath)
		if os.IsNotExist(err) {
			log.Info("BlockCache: config error [tmp-path does not exist. attempting to create tmp-path.]")
			err := os.Mkdir(bc.tmpPath, os.FileMode(0755))
			if err != nil {
				log.Err("BlockCache: config error creating directory after clean [%s]", err.Error())
				return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
			}
		}
	}

	if (uint64(bc.prefetch) * uint64(bc.blockSize)) > bc.memSize {
		log.Err("BlockCache::Configure : config error [memory limit too low for configured prefetch]")
		return fmt.Errorf("config error in %s [memory limit too low for configured prefetch]", bc.Name())
	}

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefeth %v, disk path %v, max size %vMB, disk timeout %v",
		bc.blockSize, bc.memSize, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSize, bc.diskTimeout)

	bc.blockPool = NewBlockPool(bc.blockSize, bc.memSize)
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("config error in %s [fail to init block pool]", bc.Name())
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("config error in %s [fail to init thread pool]", bc.Name())
	}

	if bc.tmpPath != "" {
		bc.diskPolicy, err = tlru.New(uint32((bc.diskSize*_1MB)/bc.blockSize), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
		if err != nil {
			log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
		}
	}

	return nil
}

// OpenFile: Create a handle for the file user has requested to open
func (bc *BlockCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	attr, err := bc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = attr.Size
	handle.Mtime = attr.Mtime

	// Set next offset to download as 0
	// We may not download this if first read starts with some other offset
	handle.SetValue("#", (uint64)(0))

	// Allocate a block pool object for this handle
	// Actual linked list to hold the nodes
	handle.Buffers = &handlemap.Buffers{
		Cooked:  list.New(), // List to hold free blocks
		Cooking: list.New(), // List to hold blocks still under download
	}

	if handle.Size < int64(bc.blockSize) {
		// File is small and can fit in one block itself
		_ = bc.refreshBlock(handle, 0, false)
	} else if bc.prefetchOnOpen && !bc.noPrefetch {
		// Prefetch to start on open
		_ = bc.startPrefetch(handle, 0, false)
	}

	return handle, nil
}

// CloseFile: File is closed by application so release all the blocks and submit back to blockPool
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	// Release the blocks that are in use and wipe out handle map
	options.Handle.Cleanup()

	// Release the buffers which are still under download after they have been written
	blockList := options.Handle.Buffers.Cooking
	node := blockList.Front()
	for ; node != nil; node = blockList.Front() {
		// Due to prefetch there might be some downloads still going on
		block := blockList.Remove(node).(*Block)

		// Wait for download to complete and then free up this block
		<-block.state
		block.ReUse()
		bc.blockPool.Release(block)
	}
	options.Handle.Buffers.Cooking = nil

	// Release the blocks that are ready to be reused
	blockList = options.Handle.Buffers.Cooked
	node = blockList.Front()
	for ; node != nil; node = blockList.Front() {
		block := blockList.Remove(node).(*Block)
		block.ReUse()
		bc.blockPool.Release(block)
	}
	options.Handle.Buffers.Cooked = nil

	return nil
}

// ReadInBuffer: Read the file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	if options.Offset >= options.Handle.Size {
		// EOF reached so early exit
		return 0, io.EOF
	}

	// As of now we allow only one operation on a handle at a time
	// This simplifies the logic of block-cache otherwise we will have to handle
	// a lot of race conditions and logic becomes complex and sub-performant
	options.Handle.Lock()
	defer options.Handle.Unlock()

	// Keep getting next blocks until you read the request amount of data
	dataRead := int(0)
	for dataRead < len(options.Data) {
		block, err := bc.getBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			if err != io.EOF {
				log.Err("BlockCache::ReadInBuffer : Failed to get Block %v=>%s offset %v [%v]", options.Handle.ID, options.Handle.Path, options.Offset, err.Error())
			}
			return dataRead, err
		}

		// Copy data from this block to user buffer
		readOffset := uint64(options.Offset) - block.offset
		bytesRead := copy(options.Data[dataRead:], block.data[readOffset:])

		// Move offset forward in case we need to copy more data
		options.Offset += int64(bytesRead)
		dataRead += bytesRead
	}

	return dataRead, nil
}

// getBlock: From offset generate the Block index and get the Block corresponding to it
/* Base logic of getBlock:
Check if the given block is already available or not
if not
	if this is the first read for this file start prefetching of blocks from given offset
	if this is not first read, consider this to be a random read case and start prefetch from given offset
		once the random read count reaches a limit, this prefetching will be turned off
	in either case this prefetching will add the block index to the map
	so search the map again now
Once block is available
if you are first reader of this block
	its time to prefetch next block(s) based on how much we can prefetch
	Once you queue  up the required prefetch mark this block as open to read
	so that others can come and freely read this block
	First reader here has responsibility to remove an old used block and lineup download for next blocks
Return this block once prefetch is queued and block is marked open for all
*/
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	// Check the given block index is already available or not
	index := bc.getBlockIndex(readoffset)
	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If this is the first read request then prefetch all required nodes
		val, _ := handle.GetValue("#")
		if !bc.noPrefetch && val.(uint64) == 0 {
			log.Debug("BlockCache::getBlock : Starting the prefetch %v=>%s (offset %v, index %v)", handle.ID, handle.Path, readoffset, index)

			// This is the first read for this file handle so start prefetching all the nodes
			err := bc.startPrefetch(handle, index, false)
			if err != nil && err != io.EOF {
				log.Err("BlockCache::getBlock : Unable to start prefetch  %v=>%s (offset %v, index %v) [%s]", handle.ID, handle.Path, readoffset, index, err.Error())
				return nil, err
			}
		} else {
			// This is a case of random read so increment the random read count
			handle.OptCnt++

			log.Debug("BlockCache::getBlock : Unable to get block %v=>%s (offset %v, index %v) Random %v", handle.ID, handle.Path, readoffset, index, handle.OptCnt)

			// This block is not present even after prefetch so lets download it now
			err := bc.startPrefetch(handle, index, false)
			if err != nil && err != io.EOF {
				log.Err("BlockCache::getBlock : Unable to start prefetch  %v=>%s (offset %v, index %v) [%s]", handle.ID, handle.Path, readoffset, index, err.Error())
				return nil, err
			}
		}

		// This node was not found so above logic should have queued it up, retry searching now
		node, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::getBlock : Failed to get the required block %v=>%s (offset %v, index %v)", handle.ID, handle.Path, readoffset, index)
			return nil, fmt.Errorf("not able to find block immediately after scheudling")
		}
	}

	// We have the block now which we wish to read
	block := node.(*Block)

	// Wait for this block to complete the download
	t := int(0)
	t = <-block.state

	if t == 1 {
		// Download complete and you are first reader of this block
		if handle.OptCnt <= MIN_RANDREAD {
			// So far this file has been read sequentially so prefetch more
			val, _ := handle.GetValue("#")
			if int64(val.(uint64)*bc.blockSize) < handle.Size {
				_ = bc.startPrefetch(handle, val.(uint64), true)
			}
		}

		// This block was moved to in-process queue as download is complete lets move it back to normal queue
		_ = handle.Buffers.Cooking.Remove(block.node)
		block.node = handle.Buffers.Cooked.PushBack(block)

		// Mark this block is now open for everyone to read and process
		// Once unblocked and moved to original queue, any instance can delete this block to reuse as well
		block.Unblock()
	}

	return block, nil
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / bc.blockSize
}

// startPrefetch: Start prefetchign the blocks from given offset. Same method is used to download currently required block as well
func (bc *BlockCache) startPrefetch(handle *handlemap.Handle, index uint64, prefetch bool) error {
	// Calculate how many buffers we have in free and in-process queue
	currentCnt := handle.Buffers.Cooked.Len() + handle.Buffers.Cooking.Len()
	cnt := uint32(0)

	if handle.OptCnt > MIN_RANDREAD {
		// This handle has been read randomly and we have reached the threshold to declare a random read case

		if currentCnt > MIN_PREFETCH {
			// As this file is in random read mode now, release the excess buffers. Just keep 5 buffers for it to work
			log.Debug("BlockCache::startPrefetch : Cleanup excessive blocks  %v=>%s index %v", handle.ID, handle.Path, index)

			// As this is random read move all in process blocks to free list
			nodeList := handle.Buffers.Cooking
			currentCnt = nodeList.Len()
			node := nodeList.Front()

			for i := 0; node != nil && i < currentCnt; node = nodeList.Front() {
				// Test whether this block is already downloaded or still under download
				block := handle.Buffers.Cooking.Remove(node).(*Block)
				block.node = nil
				i++

				select {
				case <-block.state:
					// As we are first reader of this block here its important to unblock any future readers on this block
					block.Unblock()

					// Block is downloaded so it's safe to ready it for reuse
					block.node = handle.Buffers.Cooked.PushBack(block)

				default:
					// Block is still under download so can not reuse this
					block.node = handle.Buffers.Cooking.PushBack(block)
				}
			}

			// Now remove excess blocks from cooked list
			nodeList = handle.Buffers.Cooked
			currentCnt = nodeList.Len()
			node = nodeList.Front()

			for ; node != nil && currentCnt > MIN_PREFETCH; node = nodeList.Front() {
				block := node.Value.(*Block)
				_ = nodeList.Remove(node)

				// Remove entry of this block from map so that no one can find it
				handle.RemoveValue(fmt.Sprintf("%v", block.id))
				block.node = nil

				// Submit this block back to pool for reuse
				block.ReUse()
				bc.blockPool.Release(block)

				currentCnt--
			}
		}
		// As we were asked to download a block, for random read case download only the requested block
		// This is where prefetching is blocked now as we download just the block which is requested
		cnt = 1
	} else {
		// This handle is having sequential reads so far
		// Allocate more buffers if required until we hit the prefetch count limit
		for ; currentCnt < int(bc.prefetch) && cnt < MIN_PREFETCH; currentCnt++ {
			block := bc.blockPool.TryGet()
			if block != nil {
				block.node = handle.Buffers.Cooked.PushFront(block)
				cnt++
			}
		}

		// If no new buffers were allocated then we have all buffers allocated to this handle already
		// time to switch to a sliding window where we remove one block and lineup a new block for download
		if cnt == 0 {
			cnt = 1
		}
	}

	for i := uint32(0); i < cnt; i++ {
		// Revalidate this node does not exists in the block map
		_, found := handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			// Block not found so lets push it for download
			err := bc.refreshBlock(handle, index, prefetch || i > 0)
			if err != nil {
				return err
			}
			index++
		}
	}

	return nil
}

// refreshBlock: Get a block from the list and prepare it for download
func (bc *BlockCache) refreshBlock(handle *handlemap.Handle, index uint64, prefetch bool) error {
	log.Debug("BlockCache::refreshBlock : Request to download %v=>%s (index %v, prefetch %v)", handle.ID, handle.Path, index, prefetch)

	// Convert index to offset
	offset := index * bc.blockSize
	if int64(offset) >= handle.Size {
		// We have reached EOF so return back no need to download anything here
		return io.EOF
	}

	nodeList := handle.Buffers.Cooked
	if nodeList.Len() == 0 && !prefetch {
		// User needs a block now but there is no free block available right now
		// this might happen when all blocks are under download and no first reader is hit for any of them
		block := bc.blockPool.MustGet()
		if block == nil {
			log.Err("BlockCache::refreshBlock : Unable to allocate block %v=>%s (index %v, prefetch %v)", handle.ID, handle.Path, index, prefetch)
			return fmt.Errorf("unable to allocate block")
		}

		block.node = handle.Buffers.Cooked.PushFront(block)
	}

	node := nodeList.Front()
	if node != nil {
		// Now there is at least one free block available in the list
		block := node.Value.(*Block)

		if block.id != -1 {
			// This is a reuse of a block case so we need to remove old entry from the map
			handle.RemoveValue(fmt.Sprintf("%v", block.id))
		}

		// Reuse this block and lineup for download
		block.ReUse()
		block.id = int64(index)
		block.offset = offset

		// Add this entry to handle map so that others can refer to the same block if required
		handle.SetValue(fmt.Sprintf("%v", index), block)
		handle.SetValue("#", (index + 1))

		bc.lineupDownload(handle, block, prefetch)
	}

	return nil
}

// lineupDownload : Create a work item and schedule the download
func (bc *BlockCache) lineupDownload(handle *handlemap.Handle, block *Block, prefetch bool) {
	item := &workItem{
		handle:   handle,
		block:    block,
		prefetch: prefetch,
		failCnt:  0,
	}

	// Remove this block from free block list and add to in-process list
	if block.node != nil {
		_ = handle.Buffers.Cooked.Remove(block.node)
	}

	block.node = handle.Buffers.Cooking.PushFront(block)

	// Send the work item to worker pool to schedule download
	bc.threadPool.Schedule(!prefetch, item)
}

// download : Method to download the given amount of data
func (bc *BlockCache) download(item *workItem) {
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.id)

	// filename_blockindex is the key for the lock
	// this ensure that at a given time a block from a file is downloaded only once across all open handles
	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	var diskNode any
	found := false
	localPath := ""

	if bc.tmpPath != "" {
		// Update diskpolicy to reflect the new file
		diskNode, found = bc.fileNodeMap.Load(fileName)
		if !found {
			diskNode = bc.diskPolicy.Add(fileName)
			bc.fileNodeMap.Store(fileName, diskNode)
		} else {
			bc.diskPolicy.Refresh(diskNode.(*list.Element))
		}

		// Check local file exists for this offset and file combination or not
		localPath = filepath.Join(bc.tmpPath, fileName)
		_, err := os.Stat(localPath)

		if err == nil {
			// If file exists then read the block from the local file
			f, err := os.Open(localPath)
			if err != nil {
				// On any disk failure we do not fail the download flow
				log.Err("BlockCache::download : Failed to open file %s [%s]", fileName, err.Error())
				_ = os.Remove(localPath)
			} else {
				_, err = f.Read(item.block.data)
				if err != nil {
					log.Err("BlockCache::download : Failed to read data from disk cache %s [%s]", fileName, err.Error())
					f.Close()
					_ = os.Remove(localPath)
				}

				f.Close()
				// We have read the data from disk so there is no need to go over network
				// Just mark the block that download is complete
				item.block.ReadyForReading()
				return
			}
		}
	}

	// If file does not exists then download the block from the container
	n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.block.offset),
		Data:   item.block.data,
	})

	if item.failCnt > MAX_FAIL_CNT {
		// If we failed to read the data 3 times then just give up
		log.Err("BlockCache::download : 3 attempts to download a block have failed %v=>%s (index %v, offset %v)", item.handle.ID, item.handle.Path, item.block.id, item.block.offset)
		return
	}

	if err != nil {
		// Fail to read the data so just reschedule this request
		log.Err("BlockCache::download : Failed to read %v=>%s from offset %v [%s]", item.handle.ID, item.handle.Path, item.block.id, err.Error())
		item.failCnt++
		bc.threadPool.Schedule(false, item)
		return
	} else if n == 0 {
		// No data read so just reschedule this request
		log.Err("BlockCache::download : Failed to read %v=>%s from offset %v [0 bytes read]", item.handle.ID, item.handle.Path, item.block.id)
		item.failCnt++
		bc.threadPool.Schedule(false, item)
		return
	}

	if bc.tmpPath != "" {
		// Dump this block to local disk cache
		f, err := os.Create(localPath)
		if err == nil {
			_, err := f.Write(item.block.data)
			if err != nil {
				log.Err("BlockCache::download : Failed to write %s to disk [%v]", localPath, err.Error())
				_ = os.Remove(localPath)
			}

			f.Close()
			bc.diskPolicy.Refresh(diskNode.(*list.Element))
		}
	}

	// Just mark the block that download is complete
	item.block.ReadyForReading()
}

// diskEvict : Callback when a node from disk expires
func (bc *BlockCache) diskEvict(node *list.Element) {
	fileName := node.Value.(string)

	// Lock the file name so that its not downloaded when deletion is going on
	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	bc.fileNodeMap.Delete(fileName)

	localPath := filepath.Join(bc.tmpPath, fileName)
	_ = os.Remove(localPath)
}

// checkDiskUsage : Callback to check usage of disk and decide whether eviction is needed
func (bc *BlockCache) checkDiskUsage() bool {
	data, _ := common.GetUsage(bc.tmpPath)
	usage := uint32((data * 100) / float64(bc.diskSize))

	if bc.maxDiskUsageHit {
		if usage >= MIN_POOL_USAGE {
			return true
		}
		bc.maxDiskUsageHit = false
	} else {
		if usage >= MAX_POOL_USAGE {
			bc.maxDiskUsageHit = true
			return true
		}
	}

	log.Info("BlockCache::checkDiskUsage : current disk usage : %fMB %v%%", data, usage)
	log.Info("BlockCache::checkDiskUsage : current cache usage : %v%%", bc.blockPool.Usage())
	return false
}

// ------------------------- Factory -------------------------------------------
// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{
		fileLocks: common.NewLockMap(),
	}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)

	blockSizeMb := config.AddUint64Flag("block-cache-block-size", 0, "Size (in MB) of a block to be downloaded for block-cache.")
	config.BindPFlag(compName+".block-size-mb", blockSizeMb)

	blockPoolMb := config.AddUint64Flag("block-cache-pool-size", 0, "Size (in MB) of total memory preallocated for block-cache.")
	config.BindPFlag(compName+".mem-size-mb", blockPoolMb)

	blockCachePath := config.AddStringFlag("block-cache-path", "", "Path to store downloaded blocks.")
	config.BindPFlag(compName+".path", blockCachePath)

	blockDiskMb := config.AddUint64Flag("block-cache-disk-size", 0, "Size (in MB) of total disk capacity that block-cache can use.")
	config.BindPFlag(compName+".disk-size-mb", blockDiskMb)

	blockCachePrefetch := config.AddUint32Flag("block-cache-prefetch", 0, "Max number of blocks to prefetch.")
	config.BindPFlag(compName+".prefetch", blockCachePrefetch)

	blockCachePrefetchOnOpen := config.AddBoolFlag("block-cache-prefetch-on-open", false, "Start prefetching on open or wait for first read.")
	config.BindPFlag(compName+".prefetch-on-open", blockCachePrefetchOnOpen)

}
