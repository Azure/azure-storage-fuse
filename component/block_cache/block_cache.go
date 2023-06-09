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
	"io"

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
	block  *Block
}

const compName = "block_cache"

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
//
//	Return failure if any config is not valid to exit the process
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

	if !config.IsSet(compName + ".block-size-mb") {
		conf.BlockSize = 8
	}

	if !config.IsSet(compName + ".mem-size-mb") {
		conf.MemSize = 1024
	}

	if !config.IsSet(compName + ".prefetch") {
		conf.PrefetchCount = 8
	}

	if !config.IsSet(compName + ".parallelism") {
		conf.Workers = 32
	}

	bc.blockSizeMB = conf.BlockSize
	bc.memSizeMB = conf.MemSize
	bc.workers = conf.Workers
	bc.prefetch = conf.PrefetchCount

	bc.blockPool = NewBlockPool((uint64)(bc.blockSizeMB*_1MB), (uint64)(bc.memSizeMB*_1MB))
	if bc.blockPool == nil {
		log.Err("BlockCache::Configure : fail to init Block pool")
		return fmt.Errorf("BlockCache: failed to init Block pool")
	}

	bc.threadPool = newThreadPool(bc.workers, bc.download)
	if bc.threadPool == nil {
		log.Err("BlockCache::Configure : fail to init thread pool")
		return fmt.Errorf("BlockCache: failed to init thread pool")
	}

	return nil
}

// OnConfigChange : When config file is changed, this will be called by pipeline. Refresh required config here
func (bc *BlockCache) OnConfigChange() {
	log.Trace("AzStorage::OnConfigChange : %s", bc.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := BlockCacheOptions{}
	err := config.UnmarshalKey(bc.Name(), &conf)
	if err != nil {
		log.Err("BlockCache::OnConfigChange : config error [invalid config attributes]")
		return
	}

	bc.blockPool.ReSize((uint64)(conf.BlockSize*_1MB), (uint64)(conf.MemSize*_1MB))
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
	prefetch := bc.blockPool.Available(bc.prefetch)
	for i := 0; uint32(i) < prefetch && int64(nextoffset) < handle.Size; i++ {
		handle.SetValue("#", nextoffset)
		success := bc.lineupDownload(handle, nextoffset, (i == 0))
		if !success {
			break
		}
		nextoffset += bc.blockPool.blockSize
	}

	handle.SetValue("#", nextoffset)

	return handle, nil
}

// ReadInBuffer: Read the local file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// The file should already be in the cache since CreateFile/OpenFile was called before and a shared lock was acquired.

	dataRead := int(0)
	for dataRead < len(options.Data) {

		block, err := bc.getBlock(options.Handle, uint64(options.Offset))
		if err != nil {
			if err != io.EOF {
				return 0, fmt.Errorf("BlockCache::ReadInBuffer : Failed to get the Block %s # %v [%v]", options.Handle.Path, options.Offset, err.Error())
			} else {
				return dataRead, err
			}
		}

		if block == nil {
			return dataRead, fmt.Errorf("BlockCache::ReadInBuffer : Failed to retreive block %s # %v", options.Handle.Path, options.Offset)
		}

		readOffset := uint64(options.Offset) - (block.id * bc.blockPool.blockSize)
		dataRead += copy(options.Data[dataRead:], block.data[readOffset:])

		options.Offset += int64(dataRead)
	}

	return dataRead, nil
}

// CloseFile: Flush the file and invalidate it from the cache.
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)

	options.Handle.CleanupWithCallback(func(key string, item interface{}) {
		if key != "#" {
			bc.blockPool.Release(item.(workItem).block)
		}
	})

	return nil
}

// download : Method to download the given amount of data
func (bc *BlockCache) lineupDownload(handle *handlemap.Handle, offset uint64, wait bool) bool {
	item := workItem{
		handle: handle,
		block:  bc.blockPool.Get(wait),
	}

	if item.block == nil {
		log.Err("BlockCache::lineupDownload : Failed to schedule prefetch of %s # %v, block: %v", handle.Path, offset, wait)
		return false
	}

	item.block.id = offset / bc.blockPool.blockSize

	handle.SetValue(fmt.Sprintf("%v", item.block.id), item)
	bc.threadPool.Schedule(offset == 0, item)

	return true
}

// download : Method to download the given amount of data
func (bc *BlockCache) download(i interface{}) {
	item := i.(workItem)
	n, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: item.handle,
		Offset: int64(item.block.id * bc.blockPool.blockSize),
		Data:   item.block.data,
	})

	if err != nil {
		// Fail to read the data so just reschedule this request
		log.Err("BlockCache::download : Failed to read %s from offset %v [%s]", item.handle.Path, item.block.id, err.Error())
		bc.threadPool.Schedule(false, item)
	}

	if n == 0 {
		log.Err("BlockCache::download : Failed to read %s from offset %v [0 bytes read]", item.handle.Path, item.block.id)
		bc.threadPool.Schedule(false, item)
	}

	// Unblock readers of this Block
	_ = item.block.ReadyForReading()
}

// getBlockIndex: From offset get the block index
func (bc *BlockCache) getBlockIndex(offset uint64) uint64 {
	return offset / bc.blockPool.blockSize
}

// getBlock: From offset generate the Block index and get the Block
func (bc *BlockCache) getBlock(handle *handlemap.Handle, readoffset uint64) (*Block, error) {
	if readoffset >= uint64(handle.Size) {
		return nil, io.EOF
	}

	index := bc.getBlockIndex(readoffset)
	item, found := handle.GetValue(fmt.Sprintf("%v", index))

	if !found {
		// This offset is not cached yet, so lineup the download
		success := bc.lineupDownload(handle, readoffset, true)
		if !success {
			return nil, fmt.Errorf("failed to schedule download")
		}

		item, found = handle.GetValue(fmt.Sprintf("%v", index))
		if !found {
			log.Err("BlockCache::ReadInBuffer : Something went wrong not able to find the Block %s # %v", handle.Path, index)
			return nil, fmt.Errorf("not able to find block immediatly after scheudling")
		}
	}

	block := item.(workItem).block

	// Wait for this block to complete the download
	t := int(0)
	t = <-block.state

	if t == 2 {
		_ = block.Unblock()

		// block is ready and we are the second reader so its time to schedule the next block
		lastoffset, found := handle.GetValue("#")
		if found && lastoffset.(uint64) < uint64(handle.Size) {
			if bc.blockPool.Available(1) > 0 {
				success := bc.lineupDownload(handle, lastoffset.(uint64), false)
				if success {
					handle.SetValue("#", lastoffset.(uint64)+bc.blockPool.blockSize)
				}
			}
		}
	} else if t == 1 {
		// block is ready and we are the first reader so its time to remove the second last block from here
		if block.id >= 2 {
			delId := block.id - 2
			delItem, delFound := handle.GetValue(fmt.Sprintf("%v", delId))
			if delFound {
				handle.RemoveValue(fmt.Sprintf("%v", delId))
				go bc.releaseBlock(delItem.(workItem).block)
			}
		}
	}

	return block, nil
}

// releaseBlock: Release this block and add back to the pool
func (bc *BlockCache) releaseBlock(b *Block) {
	<-b.state
	bc.blockPool.Release(b)
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{}
	comp.SetName(compName)
	config.AddConfigChangeEventListener(comp)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)
}
