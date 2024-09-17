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

package stream

import (
	"io"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type ReadCache struct {
	*Stream
	StreamConnection
}

func (r *ReadCache) Configure(conf StreamOptions) error {
	if conf.BufferSize <= 0 || conf.BlockSize <= 0 || conf.CachedObjLimit <= 0 {
		r.StreamOnly = true
		log.Info("ReadCache::Configure : Streamonly set to true")
	}
	r.BlockSize = int64(conf.BlockSize) * mb
	r.BufferSize = conf.BufferSize * mb
	r.CachedObjLimit = int32(conf.CachedObjLimit)
	r.CachedObjects = 0
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (r *ReadCache) Stop() error {
	log.Trace("Stopping component : %s", r.Name())
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil {
			handle.CacheObj.Lock()
			handle.CacheObj.Purge()
			handle.CacheObj.Unlock()
		}
		return true
	})
	return nil
}

func (r *ReadCache) unlockBlock(block *common.Block, exists bool) {
	if exists {
		block.RUnlock()
	} else {
		block.Unlock()
	}
}

func (r *ReadCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)
	handle, err := r.NextComponent().OpenFile(options)
	if err != nil {
		log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
		return handle, err
	}
	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}
	if !r.StreamOnly {
		handlemap.CreateCacheObject(int64(r.BufferSize), handle)
		if r.CachedObjects >= r.CachedObjLimit {
			log.Trace("Stream::OpenFile : file handle limit exceeded - switch handle to stream only mode %s [%s]", options.Name, handle.ID)
			handle.CacheObj.StreamOnly = true
			return handle, nil
		}
		atomic.AddInt32(&r.CachedObjects, 1)
		block, exists, err := r.getBlock(handle, 0)
		if err != nil {
			log.Err("Stream::OpenFile : error failed to get block on open %s [%s]", options.Name, err.Error())
			return handle, err
		}
		// if it exists then we can just RUnlock since we didn't manipulate its data buffer
		r.unlockBlock(block, exists)
	}
	return handle, err
}

func (r *ReadCache) getBlock(handle *handlemap.Handle, offset int64) (*common.Block, bool, error) {
	blockSize := r.BlockSize
	blockKeyObj := offset
	handle.CacheObj.Lock()
	block, found := handle.CacheObj.Get(blockKeyObj)
	if !found {
		if (offset + blockSize) > handle.Size {
			blockSize = handle.Size - offset
		}
		block = &common.Block{
			StartIndex: offset,
			EndIndex:   offset + blockSize,
			Data:       make([]byte, blockSize),
		}
		block.Lock()
		handle.CacheObj.Put(blockKeyObj, block)
		handle.CacheObj.Unlock()
		// if the block does not exist fetch it from the next component
		options := internal.ReadInBufferOptions{
			Handle: handle,
			Offset: block.StartIndex,
			Data:   block.Data,
		}
		_, err := r.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			return nil, false, err
		}
		return block, false, nil
	} else {
		block.RLock()
		handle.CacheObj.Unlock()
		return block, true, nil
	}
}

func (r *ReadCache) copyCachedBlock(handle *handlemap.Handle, offset int64, data []byte) (int, error) {
	dataLeft := int64(len(data))
	// counter to track how much we have copied into our request buffer thus far
	dataRead := 0
	// covers the case if we get a call that is bigger than the file size
	for dataLeft > 0 && offset < handle.Size {
		// round all offsets to the specific blocksize offsets
		cachedBlockStartIndex := (offset - (offset % r.BlockSize))
		// Lock on requested block and fileName to ensure it is not being rerequested or manipulated
		block, exists, err := r.getBlock(handle, cachedBlockStartIndex)
		if err != nil {
			r.unlockBlock(block, exists)
			log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
			return dataRead, err
		}
		dataCopied := int64(copy(data[dataRead:], block.Data[offset-cachedBlockStartIndex:]))
		r.unlockBlock(block, exists)
		dataLeft -= dataCopied
		offset += dataCopied
		dataRead += int(dataCopied)
	}
	return dataRead, nil
}

func (r *ReadCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// if we're only streaming then avoid using the cache
	if r.StreamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := r.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return r.copyCachedBlock(options.Handle, options.Offset, options.Data)
}

func (r *ReadCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	err := r.NextComponent().CloseFile(options)
	if err != nil {
		log.Err("Stream::CloseFile : error closing file %s [%s]", options.Handle.Path, err.Error())
	}
	if !r.StreamOnly && !options.Handle.CacheObj.StreamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		options.Handle.CacheObj.Purge()
		options.Handle.CacheObj.StreamOnly = true
		atomic.AddInt32(&r.CachedObjects, -1)
	}
	return nil
}

func (r *ReadCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	// log.Trace("AttrCache::GetAttr : %s", options.Name)
	return r.NextComponent().GetAttr(options)
}

func (r *ReadCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	return 0, syscall.ENOTSUP
}

func (r *ReadCache) FlushFile(options internal.FlushFileOptions) error {
	// log.Trace("Stream::FlushFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	return nil
}

func (r *ReadCache) TruncateFile(options internal.TruncateFileOptions) error {
	return syscall.ENOTSUP
}

func (r *ReadCache) RenameFile(options internal.RenameFileOptions) error {
	return syscall.ENOTSUP

}

func (r *ReadCache) DeleteFile(options internal.DeleteFileOptions) error {
	return syscall.ENOTSUP

}
func (r *ReadCache) DeleteDirectory(options internal.DeleteDirOptions) error {
	return syscall.ENOTSUP

}
func (r *ReadCache) RenameDirectory(options internal.RenameDirOptions) error {
	return syscall.ENOTSUP

}
func (r *ReadCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	return nil, syscall.ENOTSUP

}

func (r *ReadCache) SyncFile(_ internal.SyncFileOptions) error {
	return nil
}
