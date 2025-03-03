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
var BlockSize int
var bPool *BufferPool
var wp *workerPool
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
	bPool = newBufferPool(memory)
	wp = createWorkerPool(32)
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (bc *BlockCache) Stop() error {
	log.Trace("BlockCache::Stop : Stopping component block_cache_new %s", bc.Name())
	wp.destroyWorkerPool()
	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (bc *BlockCache) Configure(_ bool) error {
	log.Trace("BlockCache::Configure : %s", bc.Name())
	return nil
}

// CreateFile: Create a new file
func (bc *BlockCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("BlockCache::CreateFile : name=%s, mode=%s", options.Name, options.Mode)
	// logy.Write([]byte(fmt.Sprintf("BlockCache::CreateFile : name=%s, mode=%d\n", options.Name, options.Mode)))
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
	// This call will be an overhead if attr cache is not present in the pipeline. There are somethings to reconsider here.
	attr, err := bc.GetAttr(internal.GetAttrOptions{Name: options.Name})
	if err != nil {
		log.Err("BlockCache::OpenFile : Failed to get attr of %s [%s]", options.Name, err.Error())
		return nil, err
	}

	f, first_open := GetFileFromPath(options.Name)
	f.Lock()
	defer f.Unlock()
	if f.size == -1 {
		populateFileInfo(f, attr)
	}

	if first_open {
		f.blockList = createBlockListForReadOnlyFile(f)
	}

	// todo: O_TRUNC is not supported currently.
	if attr.Size == 0 || options.Flags&os.O_TRUNC != 0 {
		f.size = 0
		f.blockList = make([]*block, 0) //todo: return memory to pool
		attr.Size = 0
		f.blkListState = blockListValid
	}

	if attr.Size > 0 {
		if f.blkListState == blockListNotRetrieved && ((options.Flags&os.O_WRONLY != 0) || (options.Flags&os.O_RDWR != 0)) {
			blkList, err := bc.NextComponent().GetCommittedBlockList(options.Name)
			if err != nil {
				log.Err("BlockCache::OpenFile : Failed to get block list of %s [%v]", options.Name, err)
				return nil, fmt.Errorf("failed to retrieve block list for %s", options.Name)
			}
			blockList, valid := validateBlockList(blkList, f)
			if valid {
				f.blkListState = blockListValid
			} else {
				f.blkListState = blockListInvalid
			}
			f.blockList = blockList
		}
	}

	if f.blkListState == blockListInvalid && ((options.Flags&os.O_WRONLY != 0) || (options.Flags&os.O_RDWR != 0)) {
		log.Err("BlockCache::OpenFile : Cannot Write to file %s whose blocklist is invalid", options.Name)
		DeleteFile(f)
		return nil, errors.New("cannot write to file whose blocklist is invalid")
	}

	handle := CreateFreshHandleForFile(f.Name, f.size, attr.Mtime)
	f.handles[handle] = true

	PutHandleIntoMap(handle, f)

	return handle, nil
}

// ReadInBuffer: Read some data of the file into a buffer
func (bc *BlockCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// log.Trace("BlockCache::ReadFile : handle=%d, path=%s, offset: %d\n", options.Handle.ID, options.Handle.Path, options.Offset)
	if options.Handle.Prev_offset == options.Offset {
		if options.Handle.Is_seq == 0 {
			options.Handle.Is_seq = getBlockIndex(options.Offset) + 1
			log.Debug("BlockCache::ReadInBuffer : Seq read detected: at offset %d, Is_seq : %d", options.Offset, options.Handle.Is_seq)
		}
	} else {
		options.Handle.Is_seq = 0
		log.Debug("BlockCache::ReadInBuffer : Random read detected Prev Offset: %d, cur offset: %d, Is_seq : %d", options.Handle.Prev_offset, options.Offset, options.Handle.Is_seq)

	}
	f := GetFileFromHandle(options.Handle)

	offset := options.Offset
	dataRead := 0
	len_of_copy := len(options.Data)
	f.Lock()
	fileSize := f.size
	f.Unlock()

	if options.Offset >= fileSize {
		// EOF reached so early exit
		log.Err("BlockCache::ReadInBuffer : EOF reached before reading the file")
		return 0, io.EOF
	}

	for dataRead < len_of_copy {
		idx := getBlockIndex(offset)
		var blk *block
		var err error
		if (options.Handle.Is_seq != 0) && ((offset % int64(BlockSize)) == 0) && (options.Handle.Is_seq < idx+5) {
			log.Debug("BlockCache::ReadInBuffer : Read ahead starting at idx: %d, Is_seq : %d", idx, options.Handle.Is_seq)
			blk, err = getBlockWithReadAhead(idx, int(options.Handle.Is_seq), f)
			options.Handle.Is_seq += 3
		} else {
			blk, err = getBlockForRead(idx, f, syncRequest)
		}
		if err != nil {
			log.Err("BlockCache::ReadInBuffer : Failed to read the block, idx:%d, file:%s", blk.id, blk.file.Name)
			currefCnt := blk.refCnt.Add(-1)
			if currefCnt < 0 {
				panic("BlockCache::ReadInBuffer : Ref cnt for the blk is not getting modififed correctly")
			}
			return dataRead, err
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		blk.Lock()
		block_buf := blk.buf
		len_of_block_buf := getBlockSize(fileSize, idx)
		bytesCopied := copy(options.Data[dataRead:], block_buf.data[blockOffset:len_of_block_buf])
		currefCnt := blk.refCnt.Add(-1)
		if currefCnt < 0 {
			panic(" BlockCache::ReadInBuffer : Ref cnt for the blk is not getting modififed correctly")
		}
		blk.Unlock()

		dataRead += bytesCopied
		offset += int64(bytesCopied)
		if offset >= fileSize { //this should be protected by lock ig, idk
			return dataRead, io.EOF
		}
	}

	options.Handle.Prev_offset = options.Offset + int64(dataRead)
	return dataRead, nil

}

// WriteFile: Write to the local file
func (bc *BlockCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("BlockCache::WriteFile : handle=%d, path=%s, offset= %d", options.Handle.ID, options.Handle.Path, options.Offset)
	f := GetFileFromHandle(options.Handle)
	offset := options.Offset
	len_of_copy := len(options.Data)
	dataWritten := 0
	for dataWritten < len_of_copy {
		idx := getBlockIndex(offset)
		blk, err := getBlockForWrite(idx, f)
		if err != nil {
			currefCnt := blk.refCnt.Add(-1)
			if currefCnt < 0 {
				panic("BlockCache::WriteFile : Ref cnt for the blk is not getting modififed correctly")
			}
			return dataWritten, err
		}
		blk.cancelOngoingAsyncUpload()
		blk.resetAsyncUploadTimer()
		blockOffset := convertOffsetIntoBlockOffset(offset)

		blk.Lock()
		// What if write comes on a hole? currenlty not handled
		if blk.buf == nil {
			panic(fmt.Sprintf("BlockCache::WriteFile : Culprit Blk idx : %d, file name: %s", blk.idx, f.Name))
		}
		bytesCopied := copy(blk.buf.data[blockOffset:BlockSize], options.Data[dataWritten:])
		updateModifiedBlock(blk)
		currefCnt := blk.refCnt.Add(-1)
		if currefCnt < 0 {
			panic("BlockCache::WriteFile : Ref cnt for the blk is not getting modififed correctly")
		}
		blk.Unlock()

		dataWritten += bytesCopied
		offset += int64(bytesCopied)
		//Update the file size if it fall outside
		f.Lock()
		if offset > f.size {
			log.Debug("BlockCache::WriteFile : Size MODIFIED after write handle=%d, path=%s, offset= %d, prev size=%d, cur size=%d", options.Handle.ID, options.Handle.Path, options.Offset, f.size, offset)
			f.size = offset
		}
		f.Unlock()
	}

	return dataWritten, nil
}

func (bc *BlockCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("BlockCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)
	f := GetFileFromHandle(options.Handle)
	fileChanged, err := syncBuffersForFile(f)
	if err == nil {
		if fileChanged {
			err = commitBuffersForFile(f)
			if err != nil {
				log.Err("BlockCache::SyncFile : Commiting buffers failed handle=%d, path=%s, err=%s", options.Handle.ID, options.Handle.Path, err.Error())
			}
		}
	} else {
		log.Err("BlockCache::SyncFile : Syncing buffers failed handle=%d, path=%s, err=%s", options.Handle.ID, options.Handle.Path, err.Error())
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
	DeleteHandleForFile(options.Handle)
	DeleteHandleFromMap(options.Handle)
	return nil
}

// TruncateFile: Truncate the file to the given size
func (bc *BlockCache) TruncateFile(options internal.TruncateFileOptions) (err error) {
	log.Trace("BlockCache::Truncate File : path=%s, size = %d", options.Name, options.Size)
	var h *handlemap.Handle = options.Handle
	if h == nil {
		// Truncate on Path, as there might exists some open handles we cannot pass on the call.
		h, err = bc.OpenFile(internal.OpenFileOptions{Name: options.Name, Flags: os.O_RDWR, Mode: 0666})
		if err != nil {
			log.Err("BlockCache::Truncate File : Error Opening the file path=%s, size = %d, err = %s", options.Name, options.Size, err.Error())
			return
		}
		defer bc.CloseFile(
			internal.CloseFileOptions{
				Handle: h,
			},
		)
		// It's important to flush file as there maynot be flush call after this.
		defer func() {
			err = bc.FlushFile(
				internal.FlushFileOptions{
					Handle: h,
				},
			)
			if err != nil {
				log.Err("BlockCache::Truncate File : Error Flushing the file path=%s, size = %d, err = %s", options.Name, options.Size, err.Error())
				return
			}
		}()
	}

	f := GetFileFromHandle(h)
	f.Lock()
	defer f.Unlock()
	if f.size == options.Size {
		return nil
	}
	f.size = options.Size
	lenOfBlkLst := len(f.blockList)

	// Modify the blocklist
	finalBlocksCnt := (options.Size + int64(BlockSize) - 1) / int64(BlockSize)
	if finalBlocksCnt <= int64(lenOfBlkLst) { //shrink
		f.blockList = f.blockList[:finalBlocksCnt] //here memory of the blocks is not given to the pool, Modify it.
		// Update the state of the last block.
		lastBlkIdx := int(finalBlocksCnt - 1)
		if finalBlocksCnt > 0 {
			err = changeStateOfBlockToLocal(lastBlkIdx, f.blockList[lastBlkIdx])
			if err != nil {
				log.Err("BlockCache::Truncate File : failed to convert the last block to local, file path=%s, size = %d, err = %s", options.Name, options.Size, err.Error())
			}
		}
	} else { //expand
		//Update the state of the last block before expanding.
		if lenOfBlkLst > 0 {
			changeStateOfBlockToLocal(lenOfBlkLst-1, f.blockList[lenOfBlkLst-1])
			if err != nil {
				log.Err("BlockCache::Truncate File : failed to convert the last block to local, file path=%s, size = %d, err = %s", options.Name, options.Size, err.Error())
			}
		}
		for i := lenOfBlkLst; i < int(finalBlocksCnt); i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, localBlock, f)
			f.blockList = append(f.blockList, blk)
			if i == int(finalBlocksCnt)-1 {
				// We are allocating buffer here as there might not be full hole for last block
				bPool.getBufferForBlock(blk)
			} else {
				blk.hole = true
			}
		}
	}
	//todo: revert back to the prev state if any error occurs
	return nil
}

// DeleteDir: Recursively invalidate the directory and its children
func (bc *BlockCache) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("BlockCache::DeleteDir : %s", options.Name)
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
	err := bc.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("BlockCache::DeleteFile : error  %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

// RenameFile: Invalidate the file in local cache.
// We support soft deletes. more on this in lib
func (bc *BlockCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("BlockCache::RenameFile : src=%s, dst=%s", options.Src, options.Dst)
	err := bc.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("BlockCache::RenameFile : %s failed to rename file [%s]", options.Src, err.Error())
		return err
	} else {
		f, _ := GetFileFromPath(options.Src)
		if isDstPathTempFile(options.Dst) {
			log.Info("BlockCache::RenameFile : Deleting an opened File src=%s, dst=%s", options.Src, options.Dst)
			f.Lock()
			f.Name = options.Dst
			for h := range f.handles {
				h.Path = options.Dst
			}
			f.Unlock()
		}
		HardDeleteFile(options.Src)
	}

	return nil
}

func (bc *BlockCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	log.Trace("BlockCache::GetAttr : file=%s", options.Name)

	attr, err := bc.NextComponent().GetAttr(options)
	if err != nil {
		return attr, err
	}
	file, ok := checkFileExistsInOpen(options.Name)
	if ok {
		file.Lock()
		attr.Size = file.size
		file.Unlock()
	}

	return attr, err
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
