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
	"context"
	"fmt"

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
type BlockCache struct {
	internal.BaseComponent

	blockSizeMB uint32
	memSizeMB   uint32
	workers     uint32
	prefetch    uint32

	blockPool  *BlockPool
	threadPool *ThreadPool
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize     uint32 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize       uint32 `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	PrefetchCount uint32 `config:"prefetch" yaml:"prefetch,omitempty"`
	Workers       uint32 `config:"parallelism" yaml:"parallelism,omitempty"`
}

// One workitem to be scheduled
type workItem struct {
	handle *handlemap.Handle
	offset uint64
	val    *block
}

const compName = "block_cache"

//  Verification to check satisfaction criteria with Component Interface
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
//  this shall not block the call otherwise pipeline will not start
func (bc *BlockCache) Start(ctx context.Context) error {
	log.Trace("BlockCache::Start : Starting component %s", bc.Name())

	bc.threadPool.Start()

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component %s", bc.Name())

	bc.threadPool.Stop()

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//  Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())

	readonly := false
	err := config.UnmarshalKey("read-only", &readonly)
	if err != nil {
		log.Err("BlockCache::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("BlockCache: unable to obtain read-only")
	}

	if !readonly {
		log.Err("BlockCache::Configure : config error [filesystem is not mounted in read-only mode]")
		return fmt.Errorf("BlockCache: filesystem is not mounted in read-only mode")
	}

	// >> If you do not need any config parameters remove below code and return nil
	conf := BlockCacheOptions{}
	err = config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("BlockCache: config error [invalid config attributes]")
	}

	bc.blockSizeMB = conf.BlockSize
	bc.memSizeMB = conf.MemSize
	bc.workers = conf.Workers
	bc.prefetch = conf.PrefetchCount

	bc.blockPool = newBlockPool((uint64)(bc.blockSizeMB*_1MB), (uint64)(bc.memSizeMB*_1MB))
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init block pool")
		return fmt.Errorf("BlockCache: failed to init block pool")
	}

	bc.threadPool = newThreadPool(bc.workers, bc.Download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	return nil
}

// OpenFile: Makes the file available in the local cache for further file operations.
func (bc *BlockCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	attr, err := bc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	handle := handlemap.NewHandle(options.Name)
	handle.Size = attr.Size

	// Schedule the download of first N blocks for this file here
	nextoffset := uint64(0)
	count := uint64(0)
	for i := uint32(0); i < bc.prefetch && int64(nextoffset) < handle.Size; i++ {
		count, err = bc.LineupDownload(handle, nextoffset)
		if err != nil {
			log.Err(err.Error())
			return nil, err
		}
		nextoffset += count
	}
	handle.SetValue("#", nextoffset)

	return handle, nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.

	block := bc.GetBlock(options.Handle, uint64(options.Offset))
	if block == nil {
		return 0, fmt.Errorf("BlockCache::ReadInBuffer : Failed to get the block %s # %v", options.Handle.Path, options.Offset)
	}

	readOffset := uint64(options.Offset) - block.offset
	n := copy(options.Data, block.data[readOffset:])
	return n, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	options.Handle.CleanupWithCallback(func(item interface{}) {
		bc.blockPool.Release(item.(workItem).val)
	})

	return nil
}

// GetBlockID: From offset generate the block index
func (bc *BlockCache) GetBlockID(offset uint64) uint64 {
	if offset < bc.blockPool.firstBlockSize {
		return 0
	}

	return (offset / bc.blockPool.blockSize) * bc.blockPool.blockSize
}

// Download : Method to download the given amount of data
func (bc *BlockCache) LineupDownload(handle *handlemap.Handle, offset uint64) (uint64, error) {
	item := workItem{
		handle: handle,
		offset: offset,
		val:    bc.blockPool.Get(offset == 0),
	}

	if item.val == nil {
		return 0, fmt.Errorf("BlockCache::LineupDownload : Failed to schedule prefetch of %s # %v", handle.Path, offset)
	}

	handle.SetValue(fmt.Sprintf("%v", offset), item)

	bc.threadPool.Schedule(offset == 0, item)

	return item.val.size(), nil
}

// Download : Method to download the given amount of data
func (bc *BlockCache) Download(i interface{}) {
	item := i.(workItem)
	n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.offset),
		Data:   item.val.data,
	})

	item.val.length = n
	item.val.offset = item.offset

	if err != nil {
		// Fail to read the data so just reschedule this request
		log.Err("BlockCache::Download : Failed to read %s from offset %v [%s]", item.handle.Path, item.offset, err.Error())
		bc.threadPool.Schedule(false, item)
	} else {
		// Unblock readers of this block
		item.val.ready()
	}
}

// GetBlock: From offset generate the block index and get the block
func (bc *BlockCache) GetBlock(handle *handlemap.Handle, readoffset uint64) *block {
	offset := bc.GetBlockID(readoffset)
	item, found := handle.GetValue(fmt.Sprintf("%v", offset))

	if !found {
		// This offset is not cached yet, so lineup the download
		_, err := bc.LineupDownload(handle, offset)
		if err != nil {
			log.Err("BlockCache::ReadInBuffer : Failed to schedule new block download%s # %v", handle.Path, offset)
			return nil
		}
		item, found = handle.GetValue(fmt.Sprintf("%v", offset))
		if !found {
			log.Err("BlockCache::ReadInBuffer : Something went wrong not able to find the block %s # %v", handle.Path, offset)
			return nil
		}
	}

	block := item.(workItem).val

	// Wait for this block to complete the download
	t := int(0)
	select {
	case t = <-block.state:
		// This block is now ready to be read
		break
	default:
		// channel is closed so just exit
		break
	}

	if t == 1 {
		// Block is ready and we are the first reader so its time to schedule the next block
		lastoffset, found := handle.GetValue("#")
		if found && lastoffset.(uint64) < uint64(handle.Size) {
			count, err := bc.LineupDownload(handle, lastoffset.(uint64))
			if err != nil {
				log.Err(err.Error())
			} else {
				handle.SetValue("#", lastoffset.(uint64)+count)
			}
		}
	} else if t == 2 {
		// Block is ready and we are the second reader so its time to remove the second last block from here
		if offset > bc.blockPool.blockSize*2 {
			offset -= bc.blockPool.blockSize * 2
			if offset < bc.blockPool.firstBlockSize {
				offset = 0
			}
			delItem, delFound := handle.GetValue(fmt.Sprintf("%v", offset))
			if delFound {
				bc.blockPool.Release(delItem.(workItem).val)
			}
		}
	}

	return block
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)
}
