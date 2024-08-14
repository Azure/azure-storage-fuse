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
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

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

	lazyWrite    bool           // Flag to indicate if lazy write is enabled
	fileCloseOpt sync.WaitGroup // Wait group to wait for all async close operations to complete
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize      float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize        uint64  `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	TmpPath        string  `config:"path" yaml:"path,omitempty"`
	DiskSize       uint64  `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout    uint32  `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`
	PrefetchCount  uint32  `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers        uint32  `config:"parallelism" yaml:"parallelism,omitempty"`
	PrefetchOnOpen bool    `config:"prefetch-on-open" yaml:"prefetch-on-open,omitempty"`
}

const (
	compName               = "block_cache"
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

	if bc.lazyWrite {
		// Wait for all async upload to complete if any
		log.Info("BlockCache::Stop : Waiting for async close to complete")
		bc.fileCloseOpt.Wait()
	}

	// Wait for thread pool to stop
	bc.threadPool.Stop()

	// Clear the disk cache on exit
	if bc.tmpPath != "" {
		_ = bc.diskPolicy.Stop()
		_ = common.TempCacheCleanup(bc.tmpPath)
	}

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())

	defaultMemSize := false
	conf := BlockCacheOptions{}
	err := config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	}

	bc.blockSize = uint64(16) * _1MB
	if config.IsSet(compName + ".block-size-mb") {
		bc.blockSize = uint64(conf.BlockSize * float64(_1MB))
	}

	if config.IsSet(compName + ".mem-size-mb") {
		bc.memSize = conf.MemSize * _1MB
	} else {
		var sysinfo syscall.Sysinfo_t
		err = syscall.Sysinfo(&sysinfo)
		if err != nil {
			log.Err("BlockCache::Configure : config error %s [%s]. Assigning a pre-defined value of 4GB.", bc.Name(), err.Error())
			bc.memSize = uint64(4192) * _1MB
		} else {
			bc.memSize = uint64(0.8 * (float64)(sysinfo.Freeram) * float64(sysinfo.Unit))
			defaultMemSize = true
		}
	}

	bc.diskTimeout = defaultTimeout
	if config.IsSet(compName + ".disk-timeout-sec") {
		bc.diskTimeout = conf.DiskTimeout
	}

	bc.prefetchOnOpen = conf.PrefetchOnOpen
	bc.prefetch = uint32(math.Max((MIN_PREFETCH*2)+1, (float64)(2*runtime.NumCPU())))
	bc.noPrefetch = false

	if defaultMemSize && (uint64(bc.prefetch)*uint64(bc.blockSize)) > bc.memSize {
		bc.prefetch = (MIN_PREFETCH * 2) + 1
	}

	err = config.UnmarshalKey("lazy-write", &bc.lazyWrite)
	if err != nil {
		log.Err("BlockCache: config error [unable to obtain lazy-write]")
		return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
	}

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

	bc.workers = uint32(3 * runtime.NumCPU())
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
		var stat syscall.Statfs_t
		err = syscall.Statfs(bc.tmpPath, &stat)
		if err != nil {
			log.Err("BlockCache::Configure : config error %s [%s]. Assigning a default value of 4GB or if any value is assigned to .disk-size-mb in config.", bc.Name(), err.Error())
			bc.diskSize = uint64(4192) * _1MB
		} else {
			bc.diskSize = uint64(0.8 * float64(stat.Bavail) * float64(stat.Bsize))
		}
	}

	if config.IsSet(compName + ".disk-size-mb") {
		bc.diskSize = conf.DiskSize * _1MB
	}

	if (uint64(bc.prefetch) * uint64(bc.blockSize)) > bc.memSize {
		log.Err("BlockCache::Configure : config error [memory limit too low for configured prefetch]")
		return fmt.Errorf("config error in %s [memory limit too low for configured prefetch]", bc.Name())
	}

	log.Info("BlockCache::Configure : block size %v, mem size %v, worker %v, prefetch %v, disk path %v, max size %v, disk timeout %v, prefetch-on-open %t, maxDiskUsageHit %v, noPrefetch %v",
		bc.blockSize, bc.memSize, bc.workers, bc.prefetch, bc.tmpPath, bc.diskSize, bc.diskTimeout, bc.prefetchOnOpen, bc.maxDiskUsageHit, bc.noPrefetch)

	bc.blockPool = NewBlockPool(bc.blockSize, bc.memSize)
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("config error in %s [fail to init block pool]", bc.Name())
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download, bc.upload)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("config error in %s [fail to init thread pool]", bc.Name())
	}

	if bc.tmpPath != "" {
		bc.diskPolicy, err = tlru.New(uint32((bc.diskSize)/bc.blockSize), bc.diskTimeout, bc.diskEvict, 60, bc.checkDiskUsage)
		if err != nil {
			log.Err("BlockCache::Configure : fail to create LRU for memory nodes [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", bc.Name(), err.Error())
		}
	}

	return nil
}

// CreateFile: Create a new file
func (bc *BlockCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	_, err := bc.NextComponent().CreateFile(options)
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
	bc.prepareHandleForBlockCache(handle)
	return handle, nil
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
	handle.Mtime = attr.Mtime
	handle.Size = attr.Size

	bc.prepareHandleForBlockCache(handle)

	if options.Flags&os.O_TRUNC != 0 || options.Flags&os.O_WRONLY != 0 {
		// If file is opened in truncate or wronly mode then we need to wipe out the data consider current file size as 0
		log.Debug("BlockCache::OpenFile : Truncate %v to 0", options.Name)
		handle.Size = 0
		handle.Flags.Set(handlemap.HandleFlagDirty)
	} else if options.Flags&os.O_RDWR != 0 && handle.Size != 0 {
		// File is not opened in read-only mode so we need to get the list of blocks and validate the size
		// As there can be a potential write on this file, currently configured block size and block size of the file in container
		// has to match otherwise it will corrupt the file. Fail the open call if this is not the case.
		blockList, err := bc.NextComponent().GetCommittedBlockList(options.Name)
		if err != nil || blockList == nil {
			log.Err("BlockCache::OpenFile : Failed to get block list of %s [%v]", options.Name, err)
			return nil, fmt.Errorf("failed to retrieve block list for %s", options.Name)
		}

		valid := bc.validateBlockList(handle, options, blockList)
		if !valid {
			return nil, fmt.Errorf("block size mismatch for %s", options.Name)
		}
	}

	if handle.Size > 0 {
		// This shall be done after the refresh only as this will populate the queues created by above method
		if handle.Size < int64(bc.blockSize) {
			// File is small and can fit in one block itself
			_ = bc.refreshBlock(handle, 0, false)
		} else if bc.prefetchOnOpen && !bc.noPrefetch {
			// Prefetch to start on open
			_ = bc.startPrefetch(handle, 0, false)
		}
	}

	return handle, nil
}

// validateBlockList: Validates the blockList and populates the blocklist inside the handle for a file.
// This method is only called when the file is opened in O_RDWR mode.
// Each Block's size must equal to blockSize set in config and last block size <= config's blockSize
// returns true, if blockList is valid
func (bc *BlockCache) validateBlockList(handle *handlemap.Handle, options internal.OpenFileOptions, blockList *internal.CommittedBlockList) bool {
	lst, _ := handle.GetValue("blockList")
	listMap := lst.(map[int64]*blockInfo)
	listLen := len(*blockList)

	for idx, block := range *blockList {
		if (idx < (listLen-1) && block.Size != bc.blockSize) || (idx == (listLen-1) && block.Size > bc.blockSize) {
			log.Err("BlockCache::validateBlockList : Block size mismatch for %s [block: %v, size: %v]", options.Name, block.Id, block.Size)
			return false
		}
		listMap[int64(idx)] = &blockInfo{
			id:        block.Id,
			committed: true,
			size:      block.Size,
		}
	}
	return true
}

func (bc *BlockCache) prepareHandleForBlockCache(handle *handlemap.Handle) {
	// Allocate a block pool object for this handle
	// Actual linked list to hold the nodes
	handle.Buffers = &handlemap.Buffers{
		Cooked:  list.New(), // List to hold free blocks
		Cooking: list.New(), // List to hold blocks still under download
	}

	// Create map to hold the block-ids for this file
	listMap := make(map[int64]*blockInfo, 0)
	handle.SetValue("blockList", listMap)

	// Set next offset to download as 0
	// We may not download this if first read starts with some other offset
	handle.SetValue("#", (uint64)(0))
}

// FlushFile: Flush the local file to storage
func (bc *BlockCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("BlockCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	if bc.lazyWrite && !options.CloseInProgress {
		// As lazy-write is enable, upload will be scheduled when file is closed.
		log.Info("BlockCache::FlushFile : %s will be flushed when handle %d is closed", options.Handle.Path, options.Handle.ID)
		return nil
	}

	options.Handle.Lock()
	defer options.Handle.Unlock()

	// call commit blocks only if the handle is dirty
	if options.Handle.Dirty() {
		err := bc.commitBlocks(options.Handle)
		if err != nil {
			log.Err("BlockCache::FlushFile : Failed to commit blocks for %s [%s]", options.Handle.Path, err.Error())
			return err
		}
	}

	return nil
}

// CloseFile: File is closed by application so release all the blocks and submit back to blockPool
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	bc.fileCloseOpt.Add(1)
	if !bc.lazyWrite {
		// Sync close is called so wait till the upload completes
		return bc.closeFileInternal(options)
	}

	// Async close is called so schedule the upload and return here
	go bc.closeFileInternal(options) //nolint
	return nil
}

// closeFileInternal: Actual handling of the close file goes here
func (bc *BlockCache) closeFileInternal(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	defer bc.fileCloseOpt.Done()

	if options.Handle.Dirty() {
		log.Info("BlockCache::CloseFile : name=%s, handle=%d dirty. Flushing the file.", options.Handle.Path, options.Handle.ID)
		err := bc.FlushFile(internal.FlushFileOptions{Handle: options.Handle, CloseInProgress: true}) //nolint
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
		block.node = nil
		block.ReUse()
		bc.blockPool.Release(block)
	}
	options.Handle.Buffers.Cooking = nil

	// Release the blocks that are ready to be reused
	blockList = options.Handle.Buffers.Cooked
	node = blockList.Front()
	for ; node != nil; node = blockList.Front() {
		block := blockList.Remove(node).(*Block)
		// block.Unblock()
		block.node = nil
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
		bytesRead := copy(options.Data[dataRead:], block.data[readOffset:(block.endIndex-block.offset)])

		// Move offset forward in case we need to copy more data
		options.Offset += int64(bytesRead)
		dataRead += bytesRead

		if options.Offset >= options.Handle.Size {
			// EOF reached so early exit
			return dataRead, io.EOF
		}
	}

	return dataRead, nil
}

func (bc *BlockCache) addToCooked(handle *handlemap.Handle, block *Block) {
	if block.node != nil {
		_ = handle.Buffers.Cooking.Remove(block.node)
		_ = handle.Buffers.Cooked.Remove(block.node)
	}
	block.node = handle.Buffers.Cooked.PushBack(block)
}

func (bc *BlockCache) addToCooking(handle *handlemap.Handle, block *Block) {
	if block.node != nil {
		_ = handle.Buffers.Cooked.Remove(block.node)
		_ = handle.Buffers.Cooking.Remove(block.node)
	}
	block.node = handle.Buffers.Cooking.PushBack(block)
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

		// block is not present in the buffer list, check if it is uncommitted
		// If yes, commit all the uncommitted blocks first and then download this block
		shouldCommit, _ := shouldCommitAndDownload(int64(index), handle)
		if shouldCommit {
			// commit all the uncommitted blocks to storage
			log.Debug("BlockCache::getBlock : Downloading an uncommitted block %v, so committing all the staged blocks for %v=>%s", index, handle.ID, handle.Path)
			err := bc.commitBlocks(handle)
			if err != nil {
				log.Err("BlockCache::getBlock : Failed to commit blocks for %v=>%s [%s]", handle.ID, handle.Path, err.Error())
				return nil, err
			}
		}

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
	t, ok := <-block.state
	if ok {
		// this block is now open to read and process
		block.Unblock()

		switch t {
		case BlockStatusDownloaded:
			log.Debug("BlockCache::getBlock : Downloaded block %v for %v=>%s (read offset %v)", index, handle.ID, handle.Path, readoffset)

			block.flags.Clear(BlockFlagDownloading)

			// Download complete and you are first reader of this block
			if !bc.noPrefetch && handle.OptCnt <= MIN_RANDREAD {
				// So far this file has been read sequentially so prefetch more
				val, _ := handle.GetValue("#")
				if int64(val.(uint64)*bc.blockSize) < handle.Size {
					_ = bc.startPrefetch(handle, val.(uint64), true)
				}
			}

			// This block was moved to in-process queue as download is complete lets move it back to normal queue
			bc.addToCooked(handle, block)

			// mark this block as synced so that if it can used for write later
			// which will move it back to cooking list as per the synced flag
			block.flags.Set(BlockFlagSynced)

		case BlockStatusUploaded:
			log.Debug("BlockCache::getBlock : Staged block %v for %v=>%s (read offset %v)", index, handle.ID, handle.Path, readoffset)
			block.flags.Clear(BlockFlagUploading)

		case BlockStatusDownloadFailed:
			log.Err("BlockCache::getBlock : Failed to download block %v for %v=>%s (read offset %v)", index, handle.ID, handle.Path, readoffset)

			// Remove this node from handle so that next read retries to download the block again
			bc.releaseDownloadFailedBlock(handle, block)
			return nil, fmt.Errorf("failed to download block")

		case BlockStatusUploadFailed:
			// Local data is still valid so continue using this buffer
			log.Err("BlockCache::getBlock : Failed to upload block %v for %v=>%s (read offset %v)", index, handle.ID, handle.Path, readoffset)
			block.flags.Clear(BlockFlagUploading)

			// Move this block to end of queue as this is still modified and un-staged
			bc.addToCooking(handle, block)
		}
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
		// Check if the block exists in the local cache or not
		// If not, download the block from storage
		_, found := handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			// Check if the block is an uncommitted block or not
			// For uncommitted block we need to commit the block first
			shouldCommit, _ := shouldCommitAndDownload(int64(index), handle)
			if shouldCommit {
				// This shall happen only for the first uncommitted block and shall flush all the uncommitted blocks to storage
				log.Debug("BlockCache::startPrefetch : Fetching an uncommitted block %v, so committing all the staged blocks for %v=>%s", index, handle.ID, handle.Path)
				err := bc.commitBlocks(handle)
				if err != nil {
					log.Err("BlockCache::startPrefetch : Failed to commit blocks for %v=>%s [%s]", handle.ID, handle.Path, err.Error())
					return err
				}
			}

			// push the block for download
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
	log.Trace("BlockCache::refreshBlock : Request to download %v=>%s (index %v, prefetch %v)", handle.ID, handle.Path, index, prefetch)

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
		upload:   false,
	}

	// Remove this block from free block list and add to in-process list
	bc.addToCooking(handle, block)

	block.flags.Set(BlockFlagDownloading)

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
				item.block.Ready(BlockStatusDownloaded)
				return
			}
		}
	}

	item.block.endIndex = item.block.offset
	// If file does not exists then download the block from the container
	n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.block.offset),
		Data:   item.block.data,
	})

	if item.failCnt > MAX_FAIL_CNT {
		// If we failed to read the data 3 times then just give up
		log.Err("BlockCache::download : 3 attempts to download a block have failed %v=>%s (index %v, offset %v)", item.handle.ID, item.handle.Path, item.block.id, item.block.offset)
		item.block.Failed()
		item.block.Ready(BlockStatusDownloadFailed)
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

	item.block.endIndex = item.block.offset + uint64(n)

	if bc.tmpPath != "" {
		err := os.MkdirAll(filepath.Dir(localPath), 0777)
		if err != nil {
			log.Err("BlockCache::download : error creating directory structure for file %s [%s]", localPath, err.Error())
			return
		}

		// Dump this block to local disk cache
		f, err := os.Create(localPath)
		if err == nil {
			_, err := f.Write(item.block.data[:n])
			if err != nil {
				log.Err("BlockCache::download : Failed to write %s to disk [%v]", localPath, err.Error())
				_ = os.Remove(localPath)
			}

			f.Close()
			bc.diskPolicy.Refresh(diskNode.(*list.Element))
		}
	}

	// Just mark the block that download is complete
	item.block.Ready(BlockStatusDownloaded)
}

// WriteFile: Write to the local file
func (bc *BlockCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	// log.Debug("BlockCache::WriteFile : Writing %v bytes from %s", len(options.Data), options.Handle.Path)

	options.Handle.Lock()
	defer options.Handle.Unlock()

	// log.Debug("BlockCache::WriteFile : Writing handle %v=>%v: offset %v, %v bytes", options.Handle.ID, options.Handle.Path, options.Offset, len(options.Data))

	// Keep getting next blocks until you read the request amount of data
	dataWritten := int(0)
	for dataWritten < len(options.Data) {
		block, err := bc.getOrCreateBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			// Failed to get block for writing
			log.Err("BlockCache::WriteFile : Unable to allocate block for %s [%s]", options.Handle.Path, err.Error())
			return dataWritten, err
		}

		// log.Debug("BlockCache::WriteFile : Writing to block %v, offset %v for handle %v=>%v", block.id, options.Offset, options.Handle.ID, options.Handle.Path)

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
		}

		if options.Handle.Size < options.Offset {
			options.Handle.Size = options.Offset
		}
	}

	return dataWritten, nil
}

func (bc *BlockCache) getOrCreateBlock(handle *handlemap.Handle, offset uint64) (*Block, error) {
	// Check the given block index is already available or not
	index := bc.getBlockIndex(offset)
	if index >= MAX_BLOCKS {
		log.Err("BlockCache::getOrCreateBlock : Failed to get Block %v=>%s offset %v", handle.ID, handle.Path, offset)
		return nil, fmt.Errorf("block index out of range. Increase your block size")
	}

	// log.Debug("FilBlockCacheCache::getOrCreateBlock : Get block for %s, index %v", handle.Path, index)

	var block *Block
	var err error

	node, found := handle.GetValue(fmt.Sprintf("%v", index))
	if !found {
		// If too many buffers are piled up for this file then try to evict some of those which are already uploaded
		if handle.Buffers.Cooked.Len()+handle.Buffers.Cooking.Len() >= int(bc.prefetch) {
			bc.waitAndFreeUploadedBlocks(handle, 1)
		}

		// Either the block is not fetched yet or offset goes beyond the file size
		block = bc.blockPool.MustGet()
		if block == nil {
			log.Err("BlockCache::getOrCreateBlock : Unable to allocate block %v=>%s (index %v)", handle.ID, handle.Path, index)
			return nil, fmt.Errorf("unable to allocate block")
		}

		block.node = nil
		block.id = int64(index)
		block.offset = index * bc.blockSize

		if block.offset < uint64(handle.Size) {
			shouldCommit, shouldDownload := shouldCommitAndDownload(block.id, handle)

			// if a block has been staged and deleted from the buffer list, then we should commit the existing blocks
			// commit the dirty blocks and download the given block
			if shouldCommit {
				log.Debug("BlockCache::getOrCreateBlock : Fetching an uncommitted block %v, so committing all the staged blocks for %v=>%s", block.id, handle.ID, handle.Path)
				err = bc.commitBlocks(handle)
				if err != nil {
					log.Err("BlockCache::getOrCreateBlock : Failed to commit blocks for %v=>%s [%s]", handle.ID, handle.Path, err.Error())
					return nil, err
				}
			}

			// download the block if,
			//    - it was already committed, or
			//    - it was committed by the above commit blocks operation
			if shouldDownload || shouldCommit {
				// We are writing somewhere in between so just fetch this block
				log.Debug("BlockCache::getOrCreateBlock : Downloading block %v for %v=>%v", block.id, handle.ID, handle.Path)
				bc.lineupDownload(handle, block, false)

				// Now wait for download to complete
				<-block.state

				// if the block failed to download, it can't be used for overwriting
				if block.IsFailed() {
					log.Err("BlockCache::getOrCreateBlock : Failed to download block %v for %v=>%s", block.id, handle.ID, handle.Path)

					// Remove this node from handle so that next read retries to download the block again
					bc.releaseDownloadFailedBlock(handle, block)
					return nil, fmt.Errorf("failed to download block")
				}
			} else {
				log.Debug("BlockCache::getOrCreateBlock : push block %v to the cooking list for %v=>%v", block.id, handle.ID, handle.Path)
				block.node = handle.Buffers.Cooking.PushBack(block)
			}
		} else {
			block.node = handle.Buffers.Cooking.PushBack(block)
		}

		handle.SetValue(fmt.Sprintf("%v", index), block)
		block.flags.Clear(BlockFlagDownloading)
		block.Unblock()

		// As we are creating new blocks here, we need to push the block for upload and remove them from list here
		if handle.Buffers.Cooking.Len() > MIN_WRITE_BLOCK {
			err = bc.stageBlocks(handle, 1)
			if err != nil {
				log.Err("BlockCache::getOrCreateBlock : Unable to stage blocks for %s [%s]", handle.Path, err.Error())
			}
		}

	} else {
		// We have the block now which we wish to write
		block = node.(*Block)

		// If the block was staged earlier then we are overwriting it here so move it back to cooking queue
		if block.flags.IsSet(BlockFlagSynced) {
			log.Debug("BlockCache::getOrCreateBlock : Overwriting back to staged block %v for %v=>%s", block.id, handle.ID, handle.Path)

		} else if block.flags.IsSet(BlockFlagDownloading) {
			log.Debug("BlockCache::getOrCreateBlock : Waiting for download to finish for committed block %v for %v=>%s", block.id, handle.ID, handle.Path)
			<-block.state
			block.Unblock()

			// if the block failed to download, it can't be used for overwriting
			if block.IsFailed() {
				log.Err("BlockCache::getOrCreateBlock : Failed to download block %v for %v=>%s", block.id, handle.ID, handle.Path)

				// Remove this node from handle so that next read retries to download the block again
				bc.releaseDownloadFailedBlock(handle, block)
				return nil, fmt.Errorf("failed to download block")
			}
		} else if block.flags.IsSet(BlockFlagUploading) {
			// If the block is being staged, then wait till it is uploaded,
			// and then write to the same block and move it back to cooking queue
			log.Debug("BlockCache::getOrCreateBlock : Waiting for the block %v to upload for %v=>%s", block.id, handle.ID, handle.Path)
			<-block.state
			block.Unblock()
		}

		bc.addToCooking(handle, block)

		block.flags.Clear(BlockFlagUploading)
		block.flags.Clear(BlockFlagDownloading)
		block.flags.Clear(BlockFlagSynced)
	}

	return block, nil
}

// Stage the given number of blocks from this handle
func (bc *BlockCache) stageBlocks(handle *handlemap.Handle, cnt int) error {
	//log.Debug("BlockCache::stageBlocks : Staging blocks for %s, cnt %v", handle.Path, cnt)

	nodeList := handle.Buffers.Cooking
	node := nodeList.Front()

	lst, _ := handle.GetValue("blockList")
	listMap := lst.(map[int64]*blockInfo)

	for node != nil && cnt > 0 {
		nextNode := node.Next()
		block := node.Value.(*Block)

		if block.IsDirty() {
			bc.lineupUpload(handle, block, listMap)
			cnt--
		}

		node = nextNode
	}

	return nil
}

// remove the block which failed to download so that it can be used again
func (bc *BlockCache) releaseDownloadFailedBlock(handle *handlemap.Handle, block *Block) {
	if block.node != nil {
		_ = handle.Buffers.Cooking.Remove(block.node)
		_ = handle.Buffers.Cooked.Remove(block.node)
	}

	handle.RemoveValue(fmt.Sprintf("%v", block.id))
	block.node = nil
	block.ReUse()
	bc.blockPool.Release(block)
}

func (bc *BlockCache) printCooking(handle *handlemap.Handle) { //nolint
	nodeList := handle.Buffers.Cooking
	node := nodeList.Front()
	cookedId := []int64{}
	cookingId := []int64{}
	for node != nil {
		nextNode := node.Next()
		block := node.Value.(*Block)
		cookingId = append(cookingId, block.id)
		node = nextNode
	}
	nodeList = handle.Buffers.Cooked
	node = nodeList.Front()
	for node != nil {
		nextNode := node.Next()
		block := node.Value.(*Block)
		cookedId = append(cookedId, block.id)
		node = nextNode
	}
	log.Debug("BlockCache::printCookingnCooked : %v=>%s \n Cooking: [%v] \n Cooked: [%v]", handle.ID, handle.Path, cookingId, cookedId)

}

// shouldCommitAndDownload is used to check if we should commit the existing blocks and download the given block.
// There can be a case where a block has been partially written, staged and cleared from the buffer list.
// If write call comes for that block, we cannot get the previous staged data
// since the block is not yet committed. So, we have to commit it.
// If the block is staged and cleared from the buffer list, return true for commit and false for downloading.
// if the block is already committed, return false for commit and true for downloading.
func shouldCommitAndDownload(blockID int64, handle *handlemap.Handle) (bool, bool) {
	lst, ok := handle.GetValue("blockList")
	if !ok {
		return false, false
	}

	listMap := lst.(map[int64]*blockInfo)
	val, ok := listMap[blockID]
	if ok {
		// block id exists
		// If block is staged, return true for commit and false for downloading
		// If block is committed, return false for commit and true for downloading
		return !val.committed, val.committed
	} else {
		return false, false
	}
}

// lineupUpload : Create a work item and schedule the upload
func (bc *BlockCache) lineupUpload(handle *handlemap.Handle, block *Block, listMap map[int64]*blockInfo) {
	// if a block has data less than block size and is not the last block,
	// add null at the end and upload the full block
	// bc.printCooking(handle)
	if block.endIndex < uint64(handle.Size) {
		log.Debug("BlockCache::lineupUpload : Appending null for block %v, size %v for %v=>%s", block.id, (block.endIndex - block.offset), handle.ID, handle.Path)
		block.endIndex = block.offset + bc.blockSize
	} else if block.endIndex == uint64(handle.Size) {
		// TODO: random write scenario where this block is not the last block
		log.Debug("BlockCache::lineupUpload : Last block %v, size %v for %v=>%s", block.id, (block.endIndex - block.offset), handle.ID, handle.Path)
	}

	// id := listMap[block.id]
	// if id == "" {
	id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	listMap[block.id] = &blockInfo{
		id:        id,
		committed: false,
		size:      block.endIndex - block.offset,
	}
	//}

	log.Debug("BlockCache::lineupUpload : block %v, size %v for %v=>%s, blockId %v", block.id, (block.endIndex - block.offset), handle.ID, handle.Path, id)
	item := &workItem{
		handle:   handle,
		block:    block,
		prefetch: false,
		failCnt:  0,
		upload:   true,
		blockId:  id,
	}

	// log.Debug("BlockCache::lineupUpload : Upload block %v=>%s (index %v, offset %v, data %v)", handle.ID, handle.Path, block.id, block.offset, (block.endIndex - block.offset))

	block.Uploading()
	block.flags.Clear(BlockFlagFailed)
	block.flags.Set(BlockFlagUploading)

	// Remove this block from free block list and add to in-process list
	bc.addToCooked(handle, block)

	// Send the work item to worker pool to schedule download
	bc.threadPool.Schedule(false, item)
}

func (bc *BlockCache) waitAndFreeUploadedBlocks(handle *handlemap.Handle, cnt int) {
	nodeList := handle.Buffers.Cooked
	node := nodeList.Front()
	nextNode := node

	wipeoutBlock := false
	if cnt == 1 {
		wipeoutBlock = true
	}

	for nextNode != nil && cnt > 0 {
		node = nextNode
		nextNode = node.Next()

		block := node.Value.(*Block)
		if block.id != -1 {
			// Wait for upload of this block to complete
			_, ok := <-block.state
			block.flags.Clear(BlockFlagDownloading)
			block.flags.Clear(BlockFlagUploading)

			if ok {
				block.Unblock()
			}
		} else {
			block.Unblock()
		}

		if block.IsFailed() {
			log.Err("BlockCache::waitAndFreeUploadedBlocks : Failed to upload block, posting back to cooking list %v=>%s (index %v, offset %v)", handle.ID, handle.Path, block.id, block.offset)
			bc.addToCooking(handle, block)
			continue
		}
		cnt--

		if wipeoutBlock || block.id == -1 {
			log.Debug("BlockCache::waitAndFreeUploadedBlocks : Block cleanup for block %v=>%s (index %v, offset %v)", handle.ID, handle.Path, block.id, block.offset)
			handle.RemoveValue(fmt.Sprintf("%v", block.id))
			nodeList.Remove(node)
			block.node = nil
			block.ReUse()
			bc.blockPool.Release(block)
		}
	}
}

// upload : Method to stage the given amount of data
func (bc *BlockCache) upload(item *workItem) {
	fileName := fmt.Sprintf("%s::%v", item.handle.Path, item.block.id)

	// filename_blockindex is the key for the lock
	// this ensure that at a given time a block from a file is downloaded only once across all open handles
	flock := bc.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()
	// log.Debug("BlockCache::Upload : block %v, size %v for %v=>%s, blockId %v", item.block.id, (item.block.endIndex - item.block.offset), item.handle.ID, item.handle.Path, item.blockId)

	// This block is updated so we need to stage it now
	err := bc.NextComponent().StageData(internal.StageDataOptions{
		Name: item.handle.Path,
		Data: item.block.data[0 : item.block.endIndex-item.block.offset],
		Id:   item.blockId})
	if err != nil {
		// Fail to write the data so just reschedule this request
		log.Err("BlockCache::upload : Failed to write %v=>%s from offset %v [%s]", item.handle.ID, item.handle.Path, item.block.id, err.Error())
		item.failCnt++

		if item.failCnt > MAX_FAIL_CNT {
			// If we failed to write the data 3 times then just give up
			log.Err("BlockCache::upload : 3 attempts to upload a block have failed %v=>%s (index %v, offset %v)", item.handle.ID, item.handle.Path, item.block.id, item.block.offset)
			item.block.Failed()
			item.block.Ready(BlockStatusUploadFailed)
			return
		}

		bc.threadPool.Schedule(false, item)
		return
	}

	if bc.tmpPath != "" {
		localPath := filepath.Join(bc.tmpPath, fileName)

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
			diskNode, found := bc.fileNodeMap.Load(fileName)
			if !found {
				diskNode = bc.diskPolicy.Add(fileName)
				bc.fileNodeMap.Store(fileName, diskNode)
			} else {
				bc.diskPolicy.Refresh(diskNode.(*list.Element))
			}
		}
	}

return_safe:
	item.block.flags.Set(BlockFlagSynced)
	item.block.NoMoreDirty()
	item.block.Ready(BlockStatusUploaded)
}

// Stage the given number of blocks from this handle
func (bc *BlockCache) commitBlocks(handle *handlemap.Handle) error {
	log.Debug("BlockCache::commitBlocks : Staging blocks for %s", handle.Path)

	// Make three attempts to upload all pending blocks
	cnt := 0
	for cnt = 0; cnt < 3; cnt++ {
		if handle.Buffers.Cooking.Len() == 0 {
			break
		}

		err := bc.stageBlocks(handle, MAX_BLOCKS)
		if err != nil {
			log.Err("BlockCache::commitBlocks : Failed to stage blocks for %s [%s]", handle.Path, err.Error())
			return err
		}

		bc.waitAndFreeUploadedBlocks(handle, MAX_BLOCKS)
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

	blockIDList, err := bc.getBlockIDList(handle)
	if err != nil {
		log.Err("BlockCache::commitBlocks : Failed to get block id list for %v [%v]", handle.Path, err.Error())
		return err
	}

	log.Debug("BlockCache::commitBlocks : Committing blocks for %s", handle.Path)

	// Commit the block list now
	err = bc.NextComponent().CommitData(internal.CommitDataOptions{Name: handle.Path, List: blockIDList, BlockSize: bc.blockSize})
	if err != nil {
		log.Err("BlockCache::commitBlocks : Failed to commit blocks for %s [%s]", handle.Path, err.Error())
		return err
	}

	// set all the blocks as committed
	list, _ := handle.GetValue("blockList")
	listMap := list.(map[int64]*blockInfo)
	for k := range listMap {
		listMap[k].committed = true
	}

	handle.Flags.Clear(handlemap.HandleFlagDirty)
	return nil
}

func (bc *BlockCache) getBlockIDList(handle *handlemap.Handle) ([]string, error) {
	// generate the block id list order
	list, _ := handle.GetValue("blockList")
	listMap := list.(map[int64]*blockInfo)

	offsets := make([]int64, 0)
	blockIDList := make([]string, 0)

	for k := range listMap {
		offsets = append(offsets, k)
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })

	zeroBlockStaged := false
	zeroBlockID := ""
	index := int64(0)
	i := 0

	for i < len(offsets) {
		if index == offsets[i] {
			// TODO: when a staged block (not last block) has data less than block size
			if i != len(offsets)-1 && listMap[offsets[i]].size != bc.blockSize {
				log.Err("BlockCache::getBlockIDList : Staged block %v has less data %v for %v=>%s\n%v", offsets[i], listMap[offsets[i]].size, handle.ID, handle.Path, common.BlockCacheRWErrMsg)
				return nil, fmt.Errorf("staged block %v has less data %v for %v=>%s\n%v", offsets[i], listMap[offsets[i]].size, handle.ID, handle.Path, common.BlockCacheRWErrMsg)
			}

			blockIDList = append(blockIDList, listMap[offsets[i]].id)
			log.Debug("BlockCache::getBlockIDList : Preparing blocklist for %v=>%s (%v :  %v, size %v)", handle.ID, handle.Path, offsets[i], listMap[offsets[i]].id, listMap[offsets[i]].size)
			index++
			i++
		} else {
			for index < offsets[i] {
				if !zeroBlockStaged {
					id, err := bc.stageZeroBlock(handle, 1)
					if err != nil {
						return nil, err
					}

					zeroBlockStaged = true
					zeroBlockID = id
				}

				blockIDList = append(blockIDList, zeroBlockID)
				listMap[index] = &blockInfo{
					id:        zeroBlockID,
					committed: false,
					size:      bc.blockPool.blockSize,
				}
				log.Debug("BlockCache::getBlockIDList : Adding zero block for %v=>%s, index %v", handle.ID, handle.Path, index)
				log.Debug("BlockCache::getBlockIDList : Preparing blocklist for %v=>%s (%v :  %v, zero block size %v)", handle.ID, handle.Path, index, zeroBlockID, bc.blockPool.blockSize)
				index++
			}
		}
	}

	return blockIDList, nil
}

func (bc *BlockCache) stageZeroBlock(handle *handlemap.Handle, tryCnt int) (string, error) {
	if tryCnt > MAX_FAIL_CNT {
		// If we failed to write the data 3 times then just give up
		log.Err("BlockCache::stageZeroBlock : 3 attempts to upload zero block have failed %v=>%v", handle.ID, handle.Path)
		return "", fmt.Errorf("3 attempts to upload zero block have failed for %v=>%v", handle.ID, handle.Path)
	}

	id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))

	log.Debug("BlockCache::stageZeroBlock : Staging zero block for %v=>%v, try = %v", handle.ID, handle.Path, tryCnt)
	err := bc.NextComponent().StageData(internal.StageDataOptions{
		Name: handle.Path,
		Data: bc.blockPool.zeroBlock.data[:],
		Id:   id,
	})

	if err != nil {
		log.Err("BlockCache::stageZeroBlock : Failed to write zero block for %v=>%v, try %v [%v]", handle.ID, handle.Path, tryCnt, err.Error())
		return bc.stageZeroBlock(handle, tryCnt+1)
	}

	log.Debug("BlockCache::stageZeroBlock : Zero block id for %v=>%v = %v", handle.ID, handle.Path, id)
	return id, nil
}

// diskEvict : Callback when a node from disk expires
func (bc *BlockCache) diskEvict(node *list.Element) {
	fileName := node.Value.(string)

	// If this block is already locked then return otherwise Lock() will hung up
	if bc.fileLocks.Locked(fileName) {
		log.Info("BlockCache::diskEvict : File %s is locked so skipping eviction", fileName)
		return
	}

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
	usage := uint32((data * 100) / float64(bc.diskSize/_1MB))

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

// invalidateDirectory: Recursively invalidates a directory in the file cache.
func (bc *BlockCache) invalidateDirectory(name string) {
	log.Trace("BlockCache::invalidateDirectory : %s", name)

	if bc.tmpPath == "" {
		return
	}

	localPath := filepath.Join(bc.tmpPath, name)
	_ = os.RemoveAll(localPath)
}

// DeleteDir: Recursively invalidate the directory and its children
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : %s", options.Name)

	err := bc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("BlockCache::DeleteDir : %s failed", options.Name)
		return err
	}

	bc.invalidateDirectory(options.Name)
	return err
}

// RenameDir: Recursively invalidate the source directory and its children
func (bc *BlockCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)

	err := bc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("BlockCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}

	bc.invalidateDirectory(options.Src)
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (bc *BlockCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name=%s", options.Name)

	flock := bc.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	err := bc.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}

	localPath := filepath.Join(bc.tmpPath, options.Name)
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
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)

	sflock := bc.fileLocks.Get(options.Src)
	sflock.Lock()
	defer sflock.Unlock()

	dflock := bc.fileLocks.Get(options.Dst)
	dflock.Lock()
	defer dflock.Unlock()

	err := bc.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	}

	localSrcPath := filepath.Join(bc.tmpPath, options.Src)
	localDstPath := filepath.Join(bc.tmpPath, options.Dst)

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

func (bc *BlockCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("BlockCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	err := bc.FlushFile(internal.FlushFileOptions{Handle: options.Handle, CloseInProgress: true}) //nolint
	if err != nil {
		log.Err("BlockCache::SyncFile : failed to flush file %s", options.Handle.Path)
		return err
	}

	return nil
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
