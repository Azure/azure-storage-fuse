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

package block_cache

import (
	"container/list"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type ParallelUpload struct {
	internal.BaseComponent
	threadPool      *ThreadPool // Pool of threads
	targetDirectory string      // Disk path to the target directory
	workers         uint32      // Number of threads working to fetch the blocks
}

// Structure defining your config parameters
type ParallelUploadOptions struct {
	BlockSize       float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize         uint64  `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	TmpPath         string  `config:"path" yaml:"path,omitempty"`
	TargetDirectory string  `config:"path" yaml:"path,omitempty"`
}

const (
	compName               = "parallel_upload"
	defaultTimeout         = 120
	MAX_POOL_USAGE  uint32 = 80
	MIN_POOL_USAGE  uint32 = 50
	MIN_PREFETCH           = 5
	MIN_WRITE_BLOCK        = 3
	MIN_RANDREAD           = 10
	MAX_FAIL_CNT           = 3
	MAX_BLOCKS             = 50000
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &ParallelUpload{}

func (pu *ParallelUpload) Name() string {
	return compName
}

func (pu *ParallelUpload) SetName(name string) {
	pu.BaseComponent.SetName(name)
}

func (pu *ParallelUpload) SetNextComponent(nc internal.Component) {
	pu.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not Block the call otherwise pipeline will not start
func (pu *ParallelUpload) Start(ctx context.Context) error {
	log.Trace("BlockCache::Start : Starting component %s", pu.Name())

	// Start the thread pool and keep it ready for download
	pu.threadPool.Start()

	// If disk caching is enabled then start the disk eviction policy
	// if pu.tmpPath != "" {
	// 	err := pu.diskPolicy.Start()
	// 	if err != nil {
	// 		log.Err("BlockCache::Start : failed to start diskpolicy [%s]", err.Error())
	// 		return fmt.Errorf("failed to start  disk-policy for block-cache")
	// 	}
	// }

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (pu *ParallelUpload) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", pu.Name())

	// Wait for thread pool to stop
	pu.threadPool.Stop()

	// Clear the disk cache on exit
	// if pu.tmpPath != "" {
	// 	_ = pu.diskPolicy.Stop()
	// 	_ = pu.TempCacheCleanup()
	// }

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (pu *ParallelUpload) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", pu.Name())

	conf := ParallelUploadOptions{}
	err := config.UnmarshalKey(pu.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", pu.Name(), err.Error())
	}

	pu.targetDirectory = "./" //review
	if config.IsSet(compName + ".target-directory") {
		pu.targetDirectory = common.ExpandPath(conf.TargetDirectory)
		// Extract values from 'conf' and store them as you wish here
		_, err = os.Stat(pu.targetDirectory)
		if os.IsNotExist(err) {
			log.Info("ParallelUpload: config error [target-directory does not exist.]")
			return fmt.Errorf("config error in %s [%s]", pu.Name(), err.Error())
		}
	}

	pu.threadPool = newThreadPool(pu.workers, pu.download, pu.upload)
	if pu.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("config error in %s [fail to init thread pool]", pu.Name())
	}

	return nil
}

// CreateFile: Create a new file
func (pu *ParallelUpload) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	_, err := pu.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("BlockCache::CreateFile : Failed to create file %s", options.Name)
		return nil, err
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = 0
	handle.Mtime = time.Now()

	// As file is created on storage as well there is no need to mark this as dirty
	// Any write operation to file will mark it dirty and flush will then reupload
	// handle.Flags.Set(handlemap.HandleFlagDirty)
	pu.prepareHandleForBlockCache(handle)
	return handle, nil
}

// OpenFile: Create a handle for the file user has requested to open
func (pu *ParallelUpload) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	attr, err := pu.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Mtime = attr.Mtime
	handle.Size = attr.Size

	pu.prepareHandleForBlockCache(handle)

	if options.Flags&os.O_TRUNC != 0 || options.Flags&os.O_WRONLY != 0 {
		// If file is opened in truncate or wronly mode then we need to wipe out the data consider current file size as 0
		handle.Size = 0
		handle.Flags.Set(handlemap.HandleFlagDirty)
	} else if options.Flags&os.O_RDWR != 0 && handle.Size != 0 {
		// File is not opened in read-only mode so we need to get the list of blocks and validate the size
		// As there can be a potential write on this file, currently configured block size and block size of the file in container
		// has to match otherwise it will corrupt the file. Fail the open call if this is not the case.
		blockList, err := pu.NextComponent().GetCommittedBlockList(options.Name)
		if err != nil || blockList == nil {
			log.Err("BlockCache::OpenFile : Failed to get block list of %s [%v]", options.Name, err)
			return nil, fmt.Errorf("failed to retrieve block list for %s", options.Name)
		}

		lst, _ := handle.GetValue("blockList")
		listMap := lst.(map[int64]string)

		listLen := len(*blockList)
		for idx, block := range *blockList {
			listMap[int64(idx)] = block.Id
			// All blocks shall of same size otherwise fail the open call
			// Last block is allowed to be of smaller size as it can be partial block
			if block.Size != pu.blockSize && idx != (listLen-1) {
				log.Err("BlockCache::OpenFile : Block size mismatch for %s [block: %v, size: %v]", options.Name, block.Id, block.Size)
				return nil, fmt.Errorf("block size mismatch for %s", options.Name)
			}
		}
	}

	if handle.Size > 0 {
		// This shall be done after the refresh only as this will populate the queues created by above method
		if handle.Size < int64(pu.blockSize) {
			// File is small and can fit in one block itself
			_ = pu.refreshBlock(handle, 0, false)
		} else if pu.prefetchOnOpen && !pu.noPrefetch {
			// Prefetch to start on open
			_ = pu.startPrefetch(handle, 0, false)
		}
	}

	return handle, nil
}

func (pu *ParallelUpload) prepareHandleForBlockCache(handle *handlemap.Handle) {
	// Allocate a block pool object for this handle
	// Actual linked list to hold the nodes
	handle.Buffers = &handlemap.Buffers{
		Cooked:  list.New(), // List to hold free blocks
		Cooking: list.New(), // List to hold blocks still under download
	}

	// Create map to hold the block-ids for this file
	listMap := make(map[int64]string, 0)
	handle.SetValue("blockList", listMap)

	// Set next offset to download as 0
	// We may not download this if first read starts with some other offset
	handle.SetValue("#", (uint64)(0))
}

// FlushFile: Flush the local file to storage
func (pu *ParallelUpload) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("BlockCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	options.Handle.Lock()
	defer options.Handle.Unlock()

	err := pu.commitBlocks(options.Handle)
	if err != nil {
		log.Err("BlockCache::FlushFile : Failed to commit blocks for %s [%s]", options.Handle.Path, err.Error())
		return err
	}

	return nil
}

// CloseFile: File is closed by application so release all the blocks and submit back to blockPool
func (pu *ParallelUpload) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	if options.Handle.Dirty() {
		log.Info("BlockCache::CloseFile : name=%s, handle=%d dirty. Flushing the file.", options.Handle.Path, options.Handle.ID)
		err := pu.FlushFile(internal.FlushFileOptions{Handle: options.Handle}) //nolint
		if err != nil {
			log.Err("BlockCache::CloseFile : failed to flush file %s", options.Handle.Path)
			return err
		}
	}

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
		pu.blockPool.Release(block)
	}
	options.Handle.Buffers.Cooking = nil

	// Release the blocks that are ready to be reused
	blockList = options.Handle.Buffers.Cooked
	node = blockList.Front()
	for ; node != nil; node = blockList.Front() {
		block := blockList.Remove(node).(*Block)
		block.ReUse()
		pu.blockPool.Release(block)
	}
	options.Handle.Buffers.Cooked = nil

	return nil
}

// ReadInBuffer: Read the file into a buffer
func (pu *ParallelUpload) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
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
		block, err := pu.getBlock(options.Handle, uint64(options.Offset))
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
func (pu *ParallelUpload) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	// Check the given block index is already available or not
	index := pu.getBlockIndex(readoffset)
	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If this is the first read request then prefetch all required nodes
		val, _ := handle.GetValue("#")
		if !pu.noPrefetch && val.(uint64) == 0 {
			log.Debug("BlockCache::getBlock : Starting the prefetch %v=>%s (offset %v, index %v)", handle.ID, handle.Path, readoffset, index)

			// This is the first read for this file handle so start prefetching all the nodes
			err := pu.startPrefetch(handle, index, false)
			if err != nil && err != io.EOF {
				log.Err("BlockCache::getBlock : Unable to start prefetch  %v=>%s (offset %v, index %v) [%s]", handle.ID, handle.Path, readoffset, index, err.Error())
				return nil, err
			}
		} else {
			// This is a case of random read so increment the random read count
			handle.OptCnt++

			log.Debug("BlockCache::getBlock : Unable to get block %v=>%s (offset %v, index %v) Random %v", handle.ID, handle.Path, readoffset, index, handle.OptCnt)

			// This block is not present even after prefetch so lets download it now
			err := pu.startPrefetch(handle, index, false)
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
		block.flags.Clear(BlockFlagDownloading)

		if block.IsFailed() {
			log.Err("BlockCache::getBlock : Failed to download block %v=>%s (offset %v, index %v)", handle.ID, handle.Path, readoffset, index)

			// Remove this node from handle so that next read retries to download the block again
			_ = handle.Buffers.Cooking.Remove(block.node)
			handle.RemoveValue(fmt.Sprintf("%v", block.id))
			block.ReUse()
			pu.blockPool.Release(block)
			return nil, fmt.Errorf("failed to download block")
		}

		// Download complete and you are first reader of this block
		if handle.OptCnt <= MIN_RANDREAD {
			// So far this file has been read sequentially so prefetch more
			val, _ := handle.GetValue("#")
			if int64(val.(uint64)*pu.blockSize) < handle.Size {
				_ = pu.startPrefetch(handle, val.(uint64), true)
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
func (pu *ParallelUpload) getBlockIndex(offset uint64) uint64 {
	return offset / pu.blockSize
}

// startPrefetch: Start prefetchign the blocks from given offset. Same method is used to download currently required block as well
func (pu *ParallelUpload) startPrefetch(handle *handlemap.Handle, index uint64, prefetch bool) error {
	// Calculate how many buffers we have in free and in-process queue
	currentCnt := handle.Buffers.Cooked.Len() + handle.Buffers.Cooking.Len()
	cnt := uint32(0)

	if handle.OptCnt > MIN_RANDREAD {
		// This handle has been read randomly and we have reached the threshold to declare a random read case

		if currentCnt > MIN_PREFETCH {
			// As this file is in random read mode now, release the excess buffers. Just keep 5 buffers for it to work
			log.Info("BlockCache::startPrefetch : Cleanup excessive blocks  %v=>%s index %v", handle.ID, handle.Path, index)

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
					block.flags.Clear(BlockFlagDownloading)
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
				pu.blockPool.Release(block)

				currentCnt--
			}
		}
		// As we were asked to download a block, for random read case download only the requested block
		// This is where prefetching is blocked now as we download just the block which is requested
		cnt = 1
	} else {
		// This handle is having sequential reads so far
		// Allocate more buffers if required until we hit the prefetch count limit
		for ; currentCnt < int(pu.prefetch) && cnt < MIN_PREFETCH; currentCnt++ {
			block := pu.blockPool.TryGet()
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
			err := pu.refreshBlock(handle, index, prefetch || i > 0)
			if err != nil {
				return err
			}
			index++
		}
	}

	return nil
}

// refreshBlock: Get a block from the list and prepare it for download
func (pu *ParallelUpload) refreshBlock(handle *handlemap.Handle, index uint64, prefetch bool) error {
	log.Trace("BlockCache::refreshBlock : Request to download %v=>%s (index %v, prefetch %v)", handle.ID, handle.Path, index, prefetch)

	// Convert index to offset
	offset := index * pu.blockSize
	if int64(offset) >= handle.Size {
		// We have reached EOF so return back no need to download anything here
		return io.EOF
	}

	nodeList := handle.Buffers.Cooked
	if nodeList.Len() == 0 && !prefetch {
		// User needs a block now but there is no free block available right now
		// this might happen when all blocks are under download and no first reader is hit for any of them
		block := pu.blockPool.MustGet()
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

		pu.lineupDownload(handle, block, prefetch)
	}

	return nil
}

// lineupDownload : Create a work item and schedule the download
func (pu *ParallelUpload) lineupDownload(handle *handlemap.Handle, block *Block, prefetch bool) {
	item := &workItem{
		handle:   handle,
		block:    block,
		prefetch: prefetch,
		failCnt:  0,
		upload:   false,
	}

	// Remove this block from free block list and add to in-process list
	if block.node != nil {
		_ = handle.Buffers.Cooked.Remove(block.node)
	}

	block.node = handle.Buffers.Cooking.PushFront(block)
	block.flags.Set(BlockFlagDownloading)

	// Send the work item to worker pool to schedule download
	pu.threadPool.Schedule(!prefetch, item)
}

// download : Method to download the given amount of data
func (pu *ParallelUpload) download(item *workItem) {
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.id)

	// filename_blockindex is the key for the lock
	// this ensure that at a given time a block from a file is downloaded only once across all open handles
	flock := pu.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	var diskNode any
	found := false
	localPath := ""

	if pu.tmpPath != "" {
		// Update diskpolicy to reflect the new file
		diskNode, found = pu.fileNodeMap.Load(fileName)
		if !found {
			diskNode = pu.diskPolicy.Add(fileName)
			pu.fileNodeMap.Store(fileName, diskNode)
		} else {
			pu.diskPolicy.Refresh(diskNode.(*list.Element))
		}

		// Check local file exists for this offset and file combination or not
		localPath = filepath.Join(pu.tmpPath, fileName)
		_, err := os.Stat(localPath)

		if err == nil {
			// If file exists then read the block from the local file
			f, err := os.Open(localPath)
			if err != nil {
				// On any disk failure we do not fail the download flow
				log.Err("BlockCache::download : Failed to open file %s [%s]", fileName, err.Error())
				_ = os.Remove(localPath)
			} else {
				n, err := f.Read(item.block.data)
				if err != nil {
					log.Err("BlockCache::download : Failed to read data from disk cache %s [%s]", fileName, err.Error())
					f.Close()
					_ = os.Remove(localPath)
				}

				f.Close()
				// We have read the data from disk so there is no need to go over network
				// Just mark the block that download is complete

				item.block.endIndex = item.block.offset + uint64(n)
				item.block.Ready()
				return
			}
		}
	}

	item.block.endIndex = item.block.offset
	// If file does not exists then download the block from the container
	n, err := pu.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.block.offset),
		Data:   item.block.data,
	})

	if item.failCnt > MAX_FAIL_CNT {
		// If we failed to read the data 3 times then just give up
		log.Err("BlockCache::download : 3 attempts to download a block have failed %v=>%s (index %v, offset %v)", item.handle.ID, item.handle.Path, item.block.id, item.block.offset)
		item.block.Failed()
		item.block.Ready()
		return
	}

	if err != nil {
		// Fail to read the data so just reschedule this request
		log.Err("BlockCache::download : Failed to read %v=>%s from offset %v [%s]", item.handle.ID, item.handle.Path, item.block.id, err.Error())
		item.failCnt++
		pu.threadPool.Schedule(false, item)
		return
	} else if n == 0 {
		// No data read so just reschedule this request
		log.Err("BlockCache::download : Failed to read %v=>%s from offset %v [0 bytes read]", item.handle.ID, item.handle.Path, item.block.id)
		item.failCnt++
		pu.threadPool.Schedule(false, item)
		return
	}

	item.block.endIndex = item.block.offset + uint64(n)

	if pu.tmpPath != "" {
		err := os.MkdirAll(filepath.Dir(localPath), 0777)
		if err != nil {
			log.Err("BlockCache::download : error creating directory structure for file %s [%s]", localPath, err.Error())
			return
		}

		// Dump this block to local disk cache
		f, err := os.Create(localPath)
		if err == nil {
			_, err := f.Write(item.block.data)
			if err != nil {
				log.Err("BlockCache::download : Failed to write %s to disk [%v]", localPath, err.Error())
				_ = os.Remove(localPath)
			}

			f.Close()
			pu.diskPolicy.Refresh(diskNode.(*list.Element))
		}
	}

	// Just mark the block that download is complete
	item.block.Ready()
}

// WriteFile: Write to the local file
func (pu *ParallelUpload) WriteFile(options internal.WriteFileOptions) (int, error) {
	//log.Debug("BlockCache::WriteFile : Writing %v bytes from %s", len(options.Data), options.Handle.Path)

	options.Handle.Lock()
	defer options.Handle.Unlock()

	// Keep getting next blocks until you read the request amount of data
	dataWritten := int(0)
	for dataWritten < len(options.Data) {
		block, err := pu.getOrCreateBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			// Failed to get block for writing
			log.Err("BlockCache::WriteFile : Unable to allocate block for %s [%s]", options.Handle.Path, err.Error())
			return dataWritten, err
		}

		// Copy the incoming data to block
		writeOffset := uint64(options.Offset) - block.offset
		bytesWritten := copy(block.data[writeOffset:], options.Data[dataWritten:])

		// Mark this block has been updated
		block.Dirty()
		options.Handle.Flags.Set(handlemap.HandleFlagDirty)

		// Move offset forward in case we need to copy more data
		options.Offset += int64(bytesWritten)
		dataWritten += bytesWritten

		if block.endIndex < uint64(options.Offset) {
			block.endIndex = uint64(options.Offset)
			options.Handle.Size = int64(block.endIndex)
		}
	}

	return dataWritten, nil
}

func (pu *ParallelUpload) getOrCreateBlock(handle *handlemap.Handle, offset uint64) (*Block, error) {
	// Check the given block index is already available or not
	index := pu.getBlockIndex(offset)
	if index >= MAX_BLOCKS {
		log.Err("BlockCache::getOrCreateBlock : Failed to get Block %v=>%s offset %v", handle.ID, handle.Path, offset)
		return nil, fmt.Errorf("block index out of range. Increase your block size.")
	}

	//log.Debug("FilBlockCacheCache::getOrCreateBlock : Get block for %s, index %v", handle.Path, index)

	var block *Block
	var err error

	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If too many buffers are piled up for this file then try to evict some of those which are already uploaded
		if handle.Buffers.Cooked.Len()+handle.Buffers.Cooking.Len() >= int(pu.prefetch) {
			pu.waitAndFreeUploadedBlocks(handle, 1)
		}

		// Either the block is not fetched yet or offset goes beyond the file size
		block = pu.blockPool.MustGet()
		if block == nil {
			log.Err("BlockCache::getOrCreateBlock : Unable to allocate block %v=>%s (index %v)", handle.ID, handle.Path, index)
			return nil, fmt.Errorf("unable to allocate block")
		}

		block.node = nil
		block.id = int64(index)
		block.offset = index * pu.blockSize

		if block.offset < uint64(handle.Size) {
			// We are writing somewhere in between so just fetch this block
			pu.lineupDownload(handle, block, false)

			// Now wait for download to complete
			<-block.state
		} else {
			block.node = handle.Buffers.Cooking.PushBack(block)
		}

		handle.SetValue(fmt.Sprintf("%v", index), block)
		block.flags.Clear(BlockFlagDownloading)
		block.Unblock()

		// As we are creating new blocks here, we need to push the block for upload and remove them from list here
		if handle.Buffers.Cooking.Len() > MIN_WRITE_BLOCK {
			err = pu.stageBlocks(handle, 1)
			if err != nil {
				log.Err("BlockCache::getOrCreateBlock : Unable to stage blocks for %s [%s]", handle.Path, err.Error())
			}
		}

	} else {
		// We have the block now which we wish to write
		block = node.(*Block)

		// If the block was staged earlier then we are overwriting it here so move it back to cooking queue
		if block.flags.IsSet(BlockFlagSynced) {
			if block.node != nil {
				_ = handle.Buffers.Cooked.Remove(block.node)
			}

			block.node = handle.Buffers.Cooking.PushBack(block)
			block.flags.Clear(BlockFlagSynced)
		} else if block.flags.IsSet(BlockFlagDownloading) {
			<-block.state
			block.flags.Clear(BlockFlagDownloading)
			block.Unblock()
		}
	}

	return block, nil
}

// Stage the given number of blocks from this handle
func (pu *ParallelUpload) stageBlocks(handle *handlemap.Handle, cnt int) error {
	//log.Debug("BlockCache::stageBlocks : Stageing blocks for %s, cnt %v", handle.Path, cnt)

	nodeList := handle.Buffers.Cooking
	node := nodeList.Front()

	lst, _ := handle.GetValue("blockList")
	listMap := lst.(map[int64]string)

	for node != nil && cnt > 0 {
		nextNode := node.Next()
		block := node.Value.(*Block)

		if block.IsDirty() {
			pu.lineupUpload(handle, block, listMap)
			cnt--
		}

		node = nextNode
	}

	return nil
}

// lineupUpload : Create a work item and schedule the upload
func (pu *ParallelUpload) lineupUpload(handle *handlemap.Handle, block *Block, listMap map[int64]string) {
	// id := listMap[block.id]
	// if id == "" {
	id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	listMap[block.id] = id
	//}

	item := &workItem{
		handle:   handle,
		block:    block,
		prefetch: false,
		failCnt:  0,
		upload:   true,
		blockId:  id,
	}

	log.Debug("BlockCache::lineupUpload : Upload block %v=>%s (index %v, offset %v, data %v)", handle.ID, handle.Path, block.id, block.offset, (block.endIndex - block.offset))

	if (block.endIndex - block.offset) == 0 {
		log.Err("BlockCache::lineupUpload : Upload block %v=>%s (index %v, offset %v, data %v) 0 byte block formed", handle.ID, handle.Path, block.id, block.offset, (block.endIndex - block.offset))
	}

	// Remove this block from free block list and add to in-process list
	if block.node != nil {
		_ = handle.Buffers.Cooking.Remove(block.node)
	}

	block.Uploading()
	block.flags.Clear(BlockFlagFailed)
	block.flags.Set(BlockFlagUploading)
	block.node = handle.Buffers.Cooked.PushBack(block)

	// Send the work item to worker pool to schedule download
	pu.threadPool.Schedule(false, item)
}

func (pu *ParallelUpload) waitAndFreeUploadedBlocks(handle *handlemap.Handle, cnt int) {
	nodeList := handle.Buffers.Cooked
	node := nodeList.Front()
	nextNode := node

	for nextNode != nil && cnt > 0 {
		node = nextNode
		nextNode = node.Next()

		block := node.Value.(*Block)
		if block.id != -1 {
			// Wait for upload of this block to complete
			<-block.state
			block.flags.Clear(BlockFlagUploading)
		}

		block.Unblock()

		if block.IsFailed() {
			log.Err("BlockCache::waitAndFreeUploadedBlocks : Failed to upload block, posting back to cooking list %v=>%s (index %v, offset %v)", handle.ID, handle.Path, block.id, block.offset)
			_ = handle.Buffers.Cooked.Remove(block.node)
			block.node = handle.Buffers.Cooking.PushFront(block)
			continue
		}
		cnt--

		log.Debug("BlockCache::waitAndFreeUploadedBlocks : Block cleanup for block %v=>%s (index %v, offset %v)", handle.ID, handle.Path, block.id, block.offset)
		handle.RemoveValue(fmt.Sprintf("%v", block.id))
		nodeList.Remove(node)
		block.node = nil
		block.ReUse()
		pu.blockPool.Release(block)
	}
}

// upload : Method to stage the given amount of data
func (pu *ParallelUpload) upload(item *workItem) {
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.id)

	// filename_blockindex is the key for the lock
	// this ensure that at a given time a block from a file is downloaded only once across all open handles
	flock := pu.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	// This block is updated so we need to stage it now
	err := pu.NextComponent().StageData(internal.StageDataOptions{
		Name:   item.handle.Path,
		Data:   item.block.data[0 : item.block.endIndex-item.block.offset],
		Offset: uint64(item.block.id),
		Id:     item.blockId})
	if err != nil {
		// Fail to write the data so just reschedule this request
		log.Err("BlockCache::upload : Failed to write %v=>%s from offset %v [%s]", item.handle.ID, item.handle.Path, item.block.id, err.Error())
		item.failCnt++

		if item.failCnt > MAX_FAIL_CNT {
			// If we failed to write the data 3 times then just give up
			log.Err("BlockCache::upload : 3 attempts to upload a block have failed %v=>%s (index %v, offset %v)", item.handle.ID, item.handle.Path, item.block.id, item.block.offset)
			item.block.Failed()
			item.block.Ready()
			return
		}

		pu.threadPool.Schedule(false, item)
		return
	}

	if pu.tmpPath != "" {
		localPath := filepath.Join(pu.tmpPath, fileName)

		err := os.MkdirAll(filepath.Dir(localPath), 0777)
		if err != nil {
			log.Err("BlockCache::upload : error creating directory structure for file %s [%s]", localPath, err.Error())
			goto return_safe
		}

		// Dump this block to local disk cache
		f, err := os.Create(localPath)
		if err == nil {
			_, err := f.Write(item.block.data[0 : item.block.endIndex-item.block.offset])
			if err != nil {
				log.Err("BlockCache::upload : Failed to write %s to disk [%v]", localPath, err.Error())
				_ = os.Remove(localPath)
				goto return_safe
			}

			f.Close()
			diskNode, found := pu.fileNodeMap.Load(fileName)
			if !found {
				diskNode = pu.diskPolicy.Add(fileName)
				pu.fileNodeMap.Store(fileName, diskNode)
			} else {
				pu.diskPolicy.Refresh(diskNode.(*list.Element))
			}
		}
	}

return_safe:
	item.block.flags.Set(BlockFlagSynced)
	item.block.NoMoreDirty()
	item.block.Ready()
}

// Stage the given number of blocks from this handle
func (pu *ParallelUpload) commitBlocks(handle *handlemap.Handle) error {
	log.Debug("BlockCache::commitBlocks : Stageing blocks for %s", handle.Path)

	// Make three attempts to upload all pending blocks
	cnt := 0
	for cnt = 0; cnt < 3; cnt++ {
		if handle.Buffers.Cooking.Len() == 0 {
			break
		}

		err := pu.stageBlocks(handle, MAX_BLOCKS)
		if err != nil {
			log.Err("BlockCache::commitBlocks : Failed to stage blocks for %s [%s]", handle.Path, err.Error())
			return err
		}

		pu.waitAndFreeUploadedBlocks(handle, MAX_BLOCKS)
	}

	if cnt == 3 {
		nodeList := handle.Buffers.Cooking
		node := nodeList.Front()
		for node != nil {
			block := node.Value.(*Block)
			node = node.Next()

			if block.IsDirty() {
				log.Err("BlockCache::commitBlocks : Failed to stage blocks for %s after 3 attempts", handle.Path)
				return fmt.Errorf("failed to stage blocks")
			}
		}
	}

	// Generate the block id list order now
	list, _ := handle.GetValue("blockList")
	listMap := list.(map[int64]string)

	offsets := make([]int64, 0)
	blockIdList := make([]string, 0)

	for k := range listMap {
		offsets = append(offsets, k)
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })

	for i := 0; i < len(offsets); i++ {
		blockIdList = append(blockIdList, listMap[offsets[i]])
		log.Debug("BlockCache::commitBlocks : Preparing blocklist for %v=>%s (%v :  %v)", handle.ID, handle.Path, i, listMap[offsets[i]])
	}

	log.Debug("BlockCache::commitBlocks : Committing blocks for %s", handle.Path)

	// Commit the block list now
	err := pu.NextComponent().CommitData(internal.CommitDataOptions{Name: handle.Path, List: blockIdList, BlockSize: pu.blockSize})
	if err != nil {
		log.Err("BlockCache::commitBlocks : Failed to commit blocks for %s [%s]", handle.Path, err.Error())
		return err
	}

	handle.Flags.Clear(handlemap.HandleFlagDirty)
	return nil
}

// diskEvict : Callback when a node from disk expires
func (pu *ParallelUpload) diskEvict(node *list.Element) {
	fileName := node.Value.(string)

	// Lock the file name so that its not downloaded when deletion is going on
	flock := pu.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	pu.fileNodeMap.Delete(fileName)

	localPath := filepath.Join(pu.tmpPath, fileName)
	_ = os.Remove(localPath)
}

// checkDiskUsage : Callback to check usage of disk and decide whether eviction is needed
func (pu *ParallelUpload) checkDiskUsage() bool {
	data, _ := common.GetUsage(pu.tmpPath)
	usage := uint32((data * 100) / float64(pu.diskSize))

	if pu.maxDiskUsageHit {
		if usage >= MIN_POOL_USAGE {
			return true
		}
		pu.maxDiskUsageHit = false
	} else {
		if usage >= MAX_POOL_USAGE {
			pu.maxDiskUsageHit = true
			return true
		}
	}

	log.Info("BlockCache::checkDiskUsage : current disk usage : %fMB %v%%", data, usage)
	log.Info("BlockCache::checkDiskUsage : current cache usage : %v%%", pu.blockPool.Usage())
	return false
}

// invalidateDirectory: Recursively invalidates a directory in the file cache.
func (pu *ParallelUpload) invalidateDirectory(name string) {
	log.Trace("BlockCache::invalidateDirectory : %s", name)

	if pu.tmpPath == "" {
		return
	}

	localPath := filepath.Join(pu.tmpPath, name)
	_ = os.RemoveAll(localPath)
}

// DeleteDir: Recursively invalidate the directory and its children
func (pu *ParallelUpload) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : %s", options.Name)

	err := pu.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("BlockCache::DeleteDir : %s failed", options.Name)
		return err
	}

	pu.invalidateDirectory(options.Name)
	return err
}

// RenameDir: Recursively invalidate the source directory and its children
func (pu *ParallelUpload) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)

	err := pu.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("BlockCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	pu.invalidateDirectory(options.Src)
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (pu *ParallelUpload) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name=%s", options.Name)

	flock := pu.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	err := pu.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	localPath := filepath.Join(pu.tmpPath, options.Name)
	files, err := filepath.Glob(localPath + "*")
	if err == nil {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				break
			}
		}
	}

	return err
}

// RenameFile: Invalidate the file in local cache.
func (pu *ParallelUpload) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)

	sflock := pu.fileLocks.Get(options.Src)
	sflock.Lock()
	defer sflock.Unlock()

	dflock := pu.fileLocks.Get(options.Dst)
	dflock.Lock()
	defer dflock.Unlock()

	err := pu.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	localSrcPath := filepath.Join(pu.tmpPath, options.Src)
	localDstPath := filepath.Join(pu.tmpPath, options.Dst)

	files, err := filepath.Glob(localSrcPath + "*")
	if err == nil {
		for _, f := range files {
			err = os.Rename(f, strings.Replace(f, localSrcPath, localDstPath, 1))
			if err != nil {
				break
			}
		}
	}

	return err
}

func (pu *ParallelUpload) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("BlockCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	options.Handle.Lock()
	defer options.Handle.Unlock()

	return pu.commitBlocks(options.Handle)
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

	blockSizeMb := config.AddFloat64Flag("block-cache-block-size", 0.0, "Size (in MB) of a block to be downloaded for block-cache.")
	config.BindPFlag(compName+".block-size-mb", blockSizeMb)

	blockPoolMb := config.AddUint64Flag("block-cache-pool-size", 0, "Size (in MB) of total memory preallocated for block-cache.")
	config.BindPFlag(compName+".mem-size-mb", blockPoolMb)

	blockCachePath := config.AddStringFlag("block-cache-path", "", "Path to store downloaded blocks.")
	config.BindPFlag(compName+".path", blockCachePath)

	blockDiskMb := config.AddUint64Flag("block-cache-disk-size", 0, "Size (in MB) of total disk capacity that block-cache can use.")
	config.BindPFlag(compName+".disk-size-mb", blockDiskMb)

	blockDiskTimeout := config.AddUint32Flag("block-cache-disk-timeout", 0, "Timeout (in seconds) for which persisted data remains in disk cache.")
	config.BindPFlag(compName+".disk-timeout-sec", blockDiskTimeout)

	blockCachePrefetch := config.AddUint32Flag("block-cache-prefetch", 0, "Max number of blocks to prefetch.")
	config.BindPFlag(compName+".prefetch", blockCachePrefetch)

	blockParallelism := config.AddUint32Flag("block-cache-parallelism", 128, "Number of worker thread responsible for upload/download jobs.")
	config.BindPFlag(compName+".parallelism", blockParallelism)

	blockCachePrefetchOnOpen := config.AddBoolFlag("block-cache-prefetch-on-open", false, "Start prefetching on open or wait for first read.")
	config.BindPFlag(compName+".prefetch-on-open", blockCachePrefetchOnOpen)

}
