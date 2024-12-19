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

package block_cache_new

import (
	"context"
	"fmt"
	"io"
	"os"

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

var bc *BlockCache
var BlockSize int
var bPool *BufferPool
var memory int = 1024 * 1024 * 1024

// Common structure for Component
type BlockCache struct {
	internal.BaseComponent

	blockSize uint64 // Size of each block to be cached
}

// Structure defining your config parameters
type BlockCacheOptions struct {
	BlockSize      float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize        uint64  `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	TmpPath        string  `config:"path" yaml:"path,omitempty"`
	DiskSize       uint64  `config:"disk-size-mb" yaml:"disk-size-mb,omitempty"`
	DiskTimeout    uint32  `config:"disk-timeout-sec" yaml:"timeout-sec,omitempty"`
	PrefetchCount  uint32  `config:"prefetch" yaml:"prefetch,omitempty"`
	PrefetchOnOpen bool    `config:"prefetch-on-open" yaml:"prefetch-on-open,omitempty"`
}

const (
	compName         = "block_cache_new"
	defaultTimeout   = 120
	defaultBlockSize = 4
	MAX_BLOCKS       = 50000
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
	log.Trace("BlockCache::Start : Starting component block_cache new %s", bc.Name())
	bPool = createBufferPool(memory)
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component block_cache_new %s", bc.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())
	return nil
}

func (bc *BlockCache) validateBlockList(blkList *internal.CommittedBlockList) (blockList, bool) {
	listLen := len(*blkList)
	var newblkList blockList
	for idx, blk := range *blkList {
		if (idx < (listLen-1) && blk.Size != bc.blockSize) || (idx == (listLen-1) && blk.Size > bc.blockSize) {
			log.Err("BlockCache:: Unqual sized blocklist")
			return nil, false
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, remote_block))
	}
	return newblkList, true
}

// CreateFile: Create a new file
func (bc *BlockCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%d", options.Name, options.Mode)

	_, err := bc.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("BlockCache::CreateFile : Failed to create file %s", options.Name)
		return nil, err
	}

	return bc.OpenFile(internal.OpenFileOptions{
		Name:  options.Name,
		Flags: os.O_RDWR | os.O_TRUNC, //TODO: Standard says to open in O_WRONLY|O_TRUNC due to the writeback cache I defaulted it, it shoudl be change in future
		Mode:  options.Mode})
}

// OpenFile: Create a handle for the file user has requested to open
func (bc *BlockCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)

	attr, err := bc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	f, first_open := GetFile(options.Name)
	var blockList blockList

	if first_open && attr.Size > 0 {
		blkList, err := bc.NextComponent().GetCommittedBlockList(options.Name)
		if err != nil || blkList == nil {
			log.Err("BlockCache::OpenFile : Failed to get block list of %s [%v]", options.Name, err)
			return nil, fmt.Errorf("failed to retrieve block list for %s", options.Name)
		}
		var valid bool
		blockList, valid = bc.validateBlockList(blkList)
		if !valid {
			return nil, fmt.Errorf("block size mismatch for %s", options.Name)
		}

	}

	f.Lock()
	if f.size == -1 {
		populateFileInfo(f, attr)
	}
	handle := CreateFreshHandleForFile(f.Name, f.size, attr.Mtime)
	f.handles[handle] = true
	f.blockList = blockList
	f.Unlock()

	return handle, nil
}

// ReadInBuffer: Read the file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	log.Trace("BlockCache::ReadFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	if options.Offset >= options.Handle.Size {
		// EOF reached so early exit
		return 0, io.EOF
	}
	f, _ := GetFile(options.Handle.Path)

	offset := options.Offset
	dataRead := 0
	len_of_copy := len(options.Data)
	for dataRead < len_of_copy {
		idx := getBlockIndex(offset)
		block_buf, err := getBlockForRead(idx, options.Handle, f)
		if err != nil {
			return dataRead, err
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		block_buf.RLock()
		len_of_block_buf := block_buf.dataSize
		bytesCopied := copy(options.Data[dataRead:], block_buf.data[blockOffset:len_of_block_buf])
		block_buf.RUnlock()

		dataRead += bytesCopied
		offset += int64(bytesCopied)
		if offset >= f.size { //this should be protected by lock ig, idk
			return dataRead, io.EOF
		}
	}
	return dataRead, nil

}

// WriteFile: Write to the local file
func (bc *BlockCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("BlockCache::WriteFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	f, _ := GetFile(options.Handle.Path)
	offset := options.Offset
	len_of_copy := len(options.Data)
	dataWritten := 0
	for dataWritten < len_of_copy {
		idx := getBlockIndex(offset)
		block_buf, err := getBlockForWrite(idx, options.Handle, f)
		if err != nil {
			return dataWritten, err
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		block_buf.Lock()
		bytesCopied := copy(block_buf.data[blockOffset:BlockSize], options.Data[dataWritten:])
		block_buf.synced = 0
		block_buf.Unlock()

		dataWritten += bytesCopied
		offset += int64(dataWritten)
		//Update the file size if it fall outside
		f.Lock()
		if offset > f.size {
			f.size = offset
		}
		f.Unlock()
	}
	return dataWritten, nil

}

func (bc *BlockCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("BlockCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	f, _ := GetFile(options.Handle.Path)
	err := syncBuffersForFile(options.Handle, f)
	if err == nil {
		err = commitBuffersForFile(options.Handle, f)
	}
	return err
}

// FlushFile: Flush the local file to storage
func (bc *BlockCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("BlockCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	err := bc.SyncFile(internal.SyncFileOptions{Handle: options.Handle})
	return err
}

// CloseFile: File is closed by application so release all the blocks and submit back to blockPool
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	err := bc.SyncFile(internal.SyncFileOptions{Handle: options.Handle})
	DeleteHandleForFile(options.Handle)
	return err
}

// TruncateFile: Truncate the file to the given size
func (bc *BlockCache) TruncateFile(options internal.TruncateFileOptions) error {
	return nil
}

// DeleteDir: Recursively invalidate the directory and its children
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : %s", options.Name)
	return nil
}

// RenameDir: Recursively invalidate the source directory and its children
func (bc *BlockCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (bc *BlockCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name=%s", options.Name)
	return nil
}

// RenameFile: Invalidate the file in local cache.
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)
	return nil
}

// ------------------------- Factory -------------------------------------------
// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{}
	comp.blockSize = 8 * 1024 * 1024
	BlockSize = int(comp.blockSize)
	comp.SetName(compName)
	bc = comp
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewBlockCacheComponent)

	blockSizeMb := config.AddFloat64Flag("block-cache-new-block-size", 0.0, "Size (in MB) of a block to be downloaded for block-cache-new.")
	config.BindPFlag(compName+".block-size-mb", blockSizeMb)

	blockPoolMb := config.AddUint64Flag("block-cache-new-pool-size", 0, "Size (in MB) of total memory preallocated for block-cache-new.")
	config.BindPFlag(compName+".mem-size-mb", blockPoolMb)

	blockCachePath := config.AddStringFlag("block-cache-new-path", "", "Path to store downloaded blocks.")
	config.BindPFlag(compName+".path", blockCachePath)

	blockDiskMb := config.AddUint64Flag("block-cache-new-disk-size", 0, "Size (in MB) of total disk capacity that block-cache-new can use.")
	config.BindPFlag(compName+".disk-size-mb", blockDiskMb)

	blockDiskTimeout := config.AddUint32Flag("block-cache-new-disk-timeout", 0, "Timeout (in seconds) for which persisted data remains in disk cache-new.")
	config.BindPFlag(compName+".disk-timeout-sec", blockDiskTimeout)

	blockCachePrefetch := config.AddUint32Flag("block-cache-new-prefetch", 0, "Max number of blocks to prefetch.")
	config.BindPFlag(compName+".prefetch", blockCachePrefetch)

	blockParallelism := config.AddUint32Flag("block-cache-new-parallelism", 128, "Number of worker thread responsible for upload/download jobs.")
	config.BindPFlag(compName+".parallelism", blockParallelism)

	blockCachePrefetchOnOpen := config.AddBoolFlag("block-cache-new-prefetch-on-open", false, "Start prefetching on open or wait for first read.")
	config.BindPFlag(compName+".prefetch-on-open", blockCachePrefetchOnOpen)
}
