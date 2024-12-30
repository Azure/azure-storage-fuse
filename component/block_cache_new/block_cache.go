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
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

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

var bc *BlockCache
var logy *os.File
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
	if blkList == nil {
		return nil, false
	}
	listLen := len(*blkList)
	var newblkList blockList
	for idx, blk := range *blkList {
		if (idx < (listLen-1) && blk.Size != bc.blockSize) || (idx == (listLen-1) && blk.Size > bc.blockSize) || (len(blk.Id) != StdBlockIdLength) {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format ")
			return nil, false
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, remote_block))
	}
	return newblkList, true
}

// CreateFile: Create a new file
func (bc *BlockCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%s", options.Name, options.Mode)
	logy.Write([]byte(fmt.Sprintf("BlockCache::CreateFile : name=%s, mode=%d\n", options.Name, options.Mode)))
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
	log.Trace("BlockCache::OpenFile : name=%s, flags=%X, mode=%s", options.Name, options.Flags, options.Mode)
	logy.Write([]byte(fmt.Sprintf("BlockCache::OpenFile : name=%s, flags=%d, mode=%s\n", options.Name, options.Flags, options.Mode)))
	attr, err := bc.NextComponent().GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	if options.Flags&os.O_TRUNC != 0 {
		attr.Size = 0
	}

	f, first_open := GetFileFromPath(options.Name)
	var blockList blockList
	var valid bool = false //Invalid blocklist blobs can only be read and can't be modified
	if attr.Size == 0 {
		valid = true
	}

	if first_open && attr.Size > 0 {
		blkList, err := bc.NextComponent().GetCommittedBlockList(options.Name)
		if err != nil {
			log.Err("BlockCache::OpenFile : Failed to get block list of %s [%v]", options.Name, err)
			return nil, fmt.Errorf("failed to retrieve block list for %s", options.Name)
		}
		blockList, valid = bc.validateBlockList(blkList)
		if !valid {
			blockList = nil
		}
	}

	f.Lock()
	if valid {
		f.readOnly = false // This file can be read and modified too
	}

	if f.readOnly && ((options.Flags&os.O_WRONLY != 0) || (options.Flags&os.O_RDWR != 0)) {
		log.Err("BlockCache::OpenFile : Cannot Write to file %s whose blocklist is invalid", options.Name)
		DeleteFile(f)
		f.Unlock()
		return nil, errors.New("cannot write to file whose blocklist is invalid")
	}

	if f.size == -1 {
		populateFileInfo(f, attr)
	}
	handle := CreateFreshHandleForFile(f.Name, f.size, attr.Mtime)
	f.handles[handle] = true
	if blockList != nil {
		f.blockList = blockList
	}
	f.Unlock()
	PutHandleIntoMap(handle, f)

	return handle, nil
}

// ReadInBuffer: Read the file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	log.Trace("BlockCache::ReadFile : handle=%d, path=%s, offset: %d\n", options.Handle.ID, options.Handle.Path, options.Offset)
	logy.Write([]byte(fmt.Sprintf("BlockCache::ReadFile : handle=%d, path=%s, offset: %d\n", options.Handle.ID, options.Handle.Path, options.Offset)))
	h := options.Handle
	if h.Prev_offset == options.Offset {
		h.Is_seq++
	} else {
		h.Is_seq = 0
	}
	f := GetFileFromHandle(options.Handle)

	offset := options.Offset
	dataRead := 0
	len_of_copy := len(options.Data)
	f.Lock()
	options.Handle.Size = f.size // This is updated here as it is used by the nxt comp for upload usually not necessary!
	f.Unlock()

	if options.Offset >= options.Handle.Size {
		// EOF reached so early exit
		return 0, io.EOF
	}

	for dataRead < len_of_copy {
		idx := getBlockIndex(offset)
		var blk *block
		var err error
		// if options.Handle.Is_seq != 0 {
		// 	blk, err = getBlockWithReadAhead(idx, options.Handle, f)
		// } else {
		blk, err = getBlockForRead(idx, options.Handle, f)
		//		}
		if err != nil {
			return dataRead, err
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		blk.Lock()
		block_buf := blk.buf
		len_of_block_buf := getBlockSize(options.Handle.Size, idx)
		bytesCopied := copy(options.Data[dataRead:], block_buf.data[blockOffset:len_of_block_buf])
		// if blockOffset+int64(bytesCopied) == int64(len_of_block_buf) {
		// 	releaseBufferForBlock(blk) // THis should handle the uncommited buffers
		// }
		blk.Unlock()

		dataRead += bytesCopied
		offset += int64(bytesCopied)
		if offset >= options.Handle.Size { //this should be protected by lock ig, idk
			return dataRead, io.EOF
		}
	}

	h.Prev_offset = options.Offset + int64(dataRead)
	return dataRead, nil

}

// WriteFile: Write to the local file
func (bc *BlockCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("BlockCache::WriteFile : handle=%d, path=%s, offset= %d", options.Handle.ID, options.Handle.Path, options.Offset)
	logy.Write([]byte(fmt.Sprintf("BlockCache::WriteFile : handle=%d, path=%s, offset= %d\n", options.Handle.ID, options.Handle.Path, options.Offset)))
	f := GetFileFromHandle(options.Handle)
	offset := options.Offset
	len_of_copy := len(options.Data)
	dataWritten := 0
	for dataWritten < len_of_copy {
		idx := getBlockIndex(offset)
		blk, err := getBlockForWrite(idx, options.Handle, f)
		if err != nil {
			return dataWritten, err
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		blk.Lock()
		block_buf := blk.buf
		bytesCopied := copy(block_buf.data[blockOffset:BlockSize], options.Data[dataWritten:])
		block_buf.synced = 0
		blk.Unlock()

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
	logy.Write([]byte(fmt.Sprintf("BlockCache::SyncFile : handle=%d, path=%s\n", options.Handle.ID, options.Handle.Path)))
	f, _ := GetFileFromPath(options.Handle.Path)
	fileChanged, err := syncBuffersForFile(options.Handle, f)
	if err == nil {
		if fileChanged {
			err = commitBuffersForFile(options.Handle, f)
		}
	}
	return err
}

// FlushFile: Flush the local file to storage
func (bc *BlockCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("BlockCache::FlushFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	logy.Write([]byte(fmt.Sprintf("BlockCache::FlushFile : handle=%d, path=%s\n", options.Handle.ID, options.Handle.Path)))
	err := bc.SyncFile(internal.SyncFileOptions{Handle: options.Handle})
	return err
}

// CloseFile: File is closed by application so release all the blocks and submit back to blockPool
func (bc *BlockCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("BlockCache::CloseFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	logy.Write([]byte(fmt.Sprintf("BlockCache::CloseFile : handle=%d, path=%s\n", options.Handle.ID, options.Handle.Path)))
	//err := bc.SyncFile(internal.SyncFileOptions{Handle: options.Handle})
	DeleteHandleForFile(options.Handle)
	return nil
}

// TruncateFile: Truncate the file to the given size
func (bc *BlockCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("BlockCache::Truncate File : path=%s, size = %d", options.Name, options.Size)
	logy.Write([]byte(fmt.Sprintf("BlockCache::Truncate File : path=%s, size = %d\n", options.Name, options.Size)))
	h, err := bc.OpenFile(internal.OpenFileOptions{Name: options.Name, Flags: os.O_RDWR, Mode: 0666})
	defer bc.CloseFile(internal.CloseFileOptions{Handle: h})
	if err != nil {
		return err
	}

	f, _ := GetFileFromPath(options.Name)
	f.Lock()
	defer f.Unlock()
	f.size = options.Size
	len_of_blocklst := len(f.blockList)
	// Modify the blocklist
	total_blocks := (options.Size + int64(BlockSize) - 1) / int64(BlockSize)
	if total_blocks <= int64(len_of_blocklst) {
		f.blockList = f.blockList[:total_blocks] //here memory of the blocks is not given to the pool, Modify it.
	} else {
		for i := len_of_blocklst; i < int(total_blocks); i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, local_block)
			close(blk.downloadStatus)
			f.blockList = append(f.blockList, blk)
			if i == int(total_blocks)-1 {
				blk.buf = bPool.getBuffer()
			}
		}
	}
	return nil
}

// DeleteDir: Recursively invalidate the directory and its children
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : %s", options.Name)
	logy.Write([]byte(fmt.Sprintf("BlockCache::DeleteDir : %s\n", options.Name)))
	err := bc.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("BlockCache::DeleteDir : %s failed", options.Name)
		return err
	}
	return err
}

// RenameDir: Recursively invalidate the source directory and its children
func (bc *BlockCache) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("BlockCache::RenameDir : src=%s, dst=%s", options.Src, options.Dst)
	logy.Write([]byte(fmt.Sprintf("BlockCache::RenameDir : src=%s, dst=%s\n", options.Src, options.Dst)))
	err := bc.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("BlockCache::RenameDir : error %s [%s]", options.Src, err.Error())
		return err
	}
	return nil
}

// DeleteFile: Invalidate the file in local cache.
func (bc *BlockCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("BlockCache::DeleteFile : name=%s", options.Name)
	logy.Write([]byte(fmt.Sprintf("BlockCache::DeleteFile : name=%s\n", options.Name)))
	err := bc.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

// RenameFile: Invalidate the file in local cache.
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)
	logy.Write([]byte(fmt.Sprintf("BlockCache::RenameFile : src=%s, dst=%s\n", options.Src, options.Dst)))
	err := bc.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	} else {
		f, _ := GetFileFromPath(options.Src)
		if isDstPathTempFile(options.Dst) {
			log.Trace("BlockCache::RenameFile : Deleting Src opened File src=%s, dst=%s", options.Src, options.Dst)
			f.Name = options.Dst
			f.Lock()
			for h := range f.handles {
				h.Path = options.Dst
			}
			f.Unlock()
		}
		DeleteFile(f)
	}

	return nil
}

func (bc *BlockCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	attr, err := bc.NextComponent().GetAttr(options)
	file, ok := checkFileExistsInOpen(options.Name)
	if ok {
		attr.Size = file.size
	}
	return attr, err
}

// ------------------------- Factory -------------------------------------------
// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewBlockCacheComponent() internal.Component {
	comp := &BlockCache{}
	logy, _ = os.OpenFile("/home/syeleti/logs/logy.txt", os.O_RDWR, 0666)
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
