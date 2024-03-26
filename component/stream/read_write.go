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
	"encoding/base64"
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/pbnjay/memory"
)

type ReadWriteCache struct {
	*Stream
	StreamConnection
}

func (rw *ReadWriteCache) Configure(conf StreamOptions) error {
	if conf.BufferSize <= 0 || conf.BlockSize <= 0 || conf.CachedObjLimit <= 0 {
		rw.StreamOnly = true
		log.Info("ReadWriteCache::Configure : Streamonly set to true")
	}
	rw.BlockSize = int64(conf.BlockSize) * mb
	rw.BufferSize = conf.BufferSize * mb
	rw.CachedObjLimit = int32(conf.CachedObjLimit)
	rw.CachedObjects = 0
	return nil
}

func (rw *ReadWriteCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::CreateFile : name=%s, mode=%s", options.Name, options.Mode)
	handle, err := rw.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("Stream::CreateFile : error failed to create file %s: [%s]", options.Name, err.Error())
		return handle, err
	}
	if !rw.StreamOnly {
		err = rw.createHandleCache(handle)
		if err != nil {
			log.Err("Stream::CreateFile : error creating cache object %s [%s]", options.Name, err.Error())
		}
	}
	return handle, err
}

func (rw *ReadWriteCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)
	handle, err := rw.NextComponent().OpenFile(options)
	if err != nil {
		log.Err("Stream::OpenFile : error failed to open file %s [%s]", options.Name, err.Error())
		return handle, err
	}

	if options.Flags&os.O_TRUNC != 0 {
		handle.Size = 0
	}

	if !rw.StreamOnly {
		err = rw.createHandleCache(handle)
		if err != nil {
			log.Err("Stream::OpenFile : error failed to create cache object %s [%s]", options.Name, err.Error())
		}
	}

	return handle, err
}

func (rw *ReadWriteCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// log.Trace("Stream::ReadInBuffer : name=%s, handle=%d, offset=%d", options.Handle.Path, options.Handle.ID, options.Offset)
	if !rw.StreamOnly && options.Handle.CacheObj.StreamOnly {
		err := rw.createHandleCache(options.Handle)
		if err != nil {
			log.Err("Stream::ReadInBuffer : error failed to create cache object  %s [%s]", options.Handle.Path, err.Error())
			return 0, err
		}
	}
	if rw.StreamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	options.Handle.CacheObj.Lock()
	defer options.Handle.CacheObj.Unlock()
	if atomic.LoadInt64(&options.Handle.Size) == 0 {
		return 0, nil
	}
	read, err := rw.readWriteBlocks(options.Handle, options.Offset, options.Data, false)
	if err != nil {
		log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
	}
	return read, err
}

func (rw *ReadWriteCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	// log.Trace("Stream::WriteFile : name=%s, handle=%d, offset=%d", options.Handle.Path, options.Handle.ID, options.Offset)
	if !rw.StreamOnly && options.Handle.CacheObj.StreamOnly {
		err := rw.createHandleCache(options.Handle)
		if err != nil {
			log.Err("Stream::WriteFile : error failed to create cache object %s [%s]", options.Handle.Path, err.Error())
			return 0, err
		}
	}
	if rw.StreamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().WriteFile(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::WriteFile : error failed to write data to %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	options.Handle.CacheObj.Lock()
	defer options.Handle.CacheObj.Unlock()
	written, err := rw.readWriteBlocks(options.Handle, options.Offset, options.Data, true)
	if err != nil {
		log.Err("Stream::WriteFile : error failed to write data to %s: [%s]", options.Handle.Path, err.Error())
	}
	options.Handle.Flags.Set(handlemap.HandleFlagDirty)
	return written, err
}

func (rw *ReadWriteCache) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("Stream::TruncateFile : name=%s, size=%d", options.Name, options.Size)
	// if !rw.StreamOnly {
	// 	handleMap := handlemap.GetHandles()
	// 	handleMap.Range(func(key, value interface{}) bool {
	// 		handle := value.(*handlemap.Handle)
	// 		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
	// 			if handle.Path == options.Name {
	// 				err := rw.purge(handle, options.Size, true)
	// 				if err != nil {
	// 					log.Err("Stream::TruncateFile : failed to flush and purge handle cache %s [%s]", handle.Path, err.Error())
	// 					return false
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	err := rw.NextComponent().TruncateFile(options)
	if err != nil {
		log.Err("Stream::TruncateFile : error truncating file %s [%s]", options.Name, err.Error())
	}
	return err
}

func (rw *ReadWriteCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("Stream::RenameFile : name=%s", options.Src)
	// if !rw.StreamOnly {
	// 	var err error
	// 	handleMap := handlemap.GetHandles()
	// 	handleMap.Range(func(key, value interface{}) bool {
	// 		handle := value.(*handlemap.Handle)
	// 		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
	// 			if handle.Path == options.Src {
	// 				err := rw.purge(handle, -1, true)
	// 				if err != nil {
	// 					log.Err("Stream::RenameFile : failed to flush and purge handle cache %s [%s]", handle.Path, err.Error())
	// 					return false
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	err := rw.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("Stream::RenameFile : error renaming file %s [%s]", options.Src, err.Error())
	}
	return err
}

func (rw *ReadWriteCache) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("Stream::FlushFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	if rw.StreamOnly || options.Handle.CacheObj.StreamOnly {
		return nil
	}
	if options.Handle.Dirty() {
		err := rw.NextComponent().FlushFile(options)
		if err != nil {
			log.Err("Stream::FlushFile : error flushing file %s [%s]", options.Handle.Path, err.Error())
			return err
		}
		options.Handle.Flags.Clear(handlemap.HandleFlagDirty)
	}
	return nil
}

func (rw *ReadWriteCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	// try to flush again to make sure it's cleaned up
	err := rw.FlushFile(internal.FlushFileOptions{Handle: options.Handle})
	if err != nil {
		log.Err("Stream::CloseFile : error flushing file %s [%s]", options.Handle.Path, err.Error())
		return err
	}
	if !rw.StreamOnly && !options.Handle.CacheObj.StreamOnly {
		err = rw.purge(options.Handle, -1)
		if err != nil {
			log.Err("Stream::CloseFile : error purging file %s [%s]", options.Handle.Path, err.Error())
		}
	}
	err = rw.NextComponent().CloseFile(options)
	if err != nil {
		log.Err("Stream::CloseFile : error closing file %s [%s]", options.Handle.Path, err.Error())
	}
	return err
}

func (rw *ReadWriteCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("Stream::DeleteFile : name=%s", options.Name)
	// if !rw.StreamOnly {
	// 	handleMap := handlemap.GetHandles()
	// 	handleMap.Range(func(key, value interface{}) bool {
	// 		handle := value.(*handlemap.Handle)
	// 		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
	// 			if handle.Path == options.Name {
	// 				err := rw.purge(handle, -1, false)
	// 				if err != nil {
	// 					log.Err("Stream::DeleteFile : failed to purge handle cache %s [%s]", handle.Path, err.Error())
	// 					return false
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// }
	err := rw.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("Stream::DeleteFile : error deleting file %s [%s]", options.Name, err.Error())
	}
	return err
}

func (rw *ReadWriteCache) DeleteDirectory(options internal.DeleteDirOptions) error {
	log.Trace("Stream::DeleteDirectory : name=%s", options.Name)
	// if !rw.StreamOnly {
	// 	handleMap := handlemap.GetHandles()
	// 	handleMap.Range(func(key, value interface{}) bool {
	// 		handle := value.(*handlemap.Handle)
	// 		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
	// 			if strings.HasPrefix(handle.Path, options.Name) {
	// 				err := rw.purge(handle, -1, false)
	// 				if err != nil {
	// 					log.Err("Stream::DeleteDirectory : failed to purge handle cache %s [%s]", handle.Path, err.Error())
	// 					return false
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// }
	err := rw.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("Stream::DeleteDirectory : error deleting directory %s [%s]", options.Name, err.Error())
	}
	return err
}

func (rw *ReadWriteCache) RenameDirectory(options internal.RenameDirOptions) error {
	log.Trace("Stream::RenameDirectory : name=%s", options.Src)
	// if !rw.StreamOnly {
	// 	var err error
	// 	handleMap := handlemap.GetHandles()
	// 	handleMap.Range(func(key, value interface{}) bool {
	// 		handle := value.(*handlemap.Handle)
	// 		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
	// 			if strings.HasPrefix(handle.Path, options.Src) {
	// 				err := rw.purge(handle, -1, true)
	// 				if err != nil {
	// 					log.Err("Stream::RenameDirectory : failed to flush and purge handle cache %s [%s]", handle.Path, err.Error())
	// 					return false
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	err := rw.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("Stream::RenameDirectory : error renaming directory %s [%s]", options.Src, err.Error())
	}
	return err
}

// Stop : Stop the component functionality and kill all threads started
func (rw *ReadWriteCache) Stop() error {
	log.Trace("Stream::Stop : stopping component : %s", rw.Name())
	if !rw.StreamOnly {
		handleMap := handlemap.GetHandles()
		handleMap.Range(func(key, value interface{}) bool {
			handle := value.(*handlemap.Handle)
			if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
				err := rw.purge(handle, -1)
				if err != nil {
					log.Err("Stream::Stop : failed to purge handle cache %s [%s]", handle.Path, err.Error())
					return false
				}
			}
			return true
		})
	}
	return nil
}

func (rw *ReadWriteCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	// log.Trace("AttrCache::GetAttr : %s", options.Name)
	return rw.NextComponent().GetAttr(options)
}

func (rw *ReadWriteCache) purge(handle *handlemap.Handle, size int64) error {
	handle.CacheObj.Lock()
	defer handle.CacheObj.Unlock()
	handle.CacheObj.Purge()
	// if size isn't -1 then we're resizing
	if size != -1 {
		atomic.StoreInt64(&handle.Size, size)
	}
	handle.CacheObj.StreamOnly = true
	atomic.AddInt32(&rw.CachedObjects, -1)
	return nil
}

func (rw *ReadWriteCache) createHandleCache(handle *handlemap.Handle) error {
	handlemap.CreateCacheObject(int64(rw.BufferSize), handle)
	// if we hit handle limit then stream only on this new handle
	if atomic.LoadInt32(&rw.CachedObjects) >= rw.CachedObjLimit {
		handle.CacheObj.StreamOnly = true
		return nil
	}
	opts := internal.GetFileBlockOffsetsOptions{
		Name: handle.Path,
	}
	var offsets *common.BlockOffsetList
	var err error
	if handle.Size == 0 {
		offsets = &common.BlockOffsetList{}
		offsets.Flags.Set(common.SmallFile)
	} else {
		offsets, err = rw.NextComponent().GetFileBlockOffsets(opts)
		if err != nil {
			return err
		}
	}
	handle.CacheObj.BlockOffsetList = offsets
	// if its a small file then download the file in its entirety if there is memory available, otherwise stream only
	if handle.CacheObj.SmallFile() {
		if uint64(atomic.LoadInt64(&handle.Size)) > memory.FreeMemory() {
			handle.CacheObj.StreamOnly = true
			return nil
		}
		block, _, err := rw.getBlock(handle, &common.Block{StartIndex: 0, EndIndex: handle.Size})
		if err != nil {
			return err
		}
		block.Id = base64.StdEncoding.EncodeToString(common.NewUUID().Bytes())
		// our handle will consist of a single block locally for simpler logic
		handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, block)
		handle.CacheObj.BlockIdLength = common.GetIdLength(block.Id)
		// now consists of a block - clear the flag
		handle.CacheObj.Flags.Clear(common.SmallFile)
	}
	atomic.AddInt32(&rw.CachedObjects, 1)
	return nil
}

func (rw *ReadWriteCache) putBlock(handle *handlemap.Handle, block *common.Block) error {
	ok := handle.CacheObj.Put(block.StartIndex, block)
	// if the cache is full and we couldn't evict - we need to do a flush
	if !ok {
		err := rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
		if err != nil {
			return err
		}
		ok = handle.CacheObj.Put(block.StartIndex, block)
		if !ok {
			return errors.New("flushed and still unable to put block in cache")
		}
	}
	return nil
}

func (rw *ReadWriteCache) getBlock(handle *handlemap.Handle, block *common.Block) (*common.Block, bool, error) {
	cached_block, found := handle.CacheObj.Get(block.StartIndex)
	if !found {
		block.Data = make([]byte, block.EndIndex-block.StartIndex)
		err := rw.putBlock(handle, block)
		if err != nil {
			return block, false, err
		}
		options := internal.ReadInBufferOptions{
			Handle: handle,
			Offset: block.StartIndex,
			Data:   block.Data,
		}
		// check if its a create operation
		if len(block.Data) != 0 {
			_, err = rw.NextComponent().ReadInBuffer(options)
			if err != nil && err != io.EOF {
				return nil, false, err
			}
		}
		return block, false, nil
	}
	return cached_block, true, nil
}

func (rw *ReadWriteCache) readWriteBlocks(handle *handlemap.Handle, offset int64, data []byte, write bool) (int, error) {
	// if it's not a small file then we look the blocks it consistts of
	blocks, found := handle.CacheObj.FindBlocks(offset, int64(len(data)))
	if !found && !write {
		return 0, nil
	}
	dataLeft := int64(len(data))
	dataRead, blk_index, dataCopied := 0, 0, int64(0)
	lastBlock := handle.CacheObj.BlockList[len(handle.CacheObj.BlockList)-1]
	for dataLeft > 0 {
		if offset < int64(lastBlock.EndIndex) {
			block, _, err := rw.getBlock(handle, blocks[blk_index])
			if err != nil {
				return dataRead, err
			}
			if write {
				dataCopied = int64(copy(block.Data[offset-blocks[blk_index].StartIndex:], data[dataRead:]))
				block.Flags.Set(common.DirtyBlock)
			} else {
				dataCopied = int64(copy(data[dataRead:], block.Data[offset-blocks[blk_index].StartIndex:]))
			}
			dataLeft -= dataCopied
			offset += dataCopied
			dataRead += int(dataCopied)
			blk_index += 1
			//if appending to file
		} else if write {
			emptyByteLength := offset - lastBlock.EndIndex
			// if the data to append + our last block existing data do not exceed block size - just append to last block
			if (lastBlock.EndIndex-lastBlock.StartIndex)+(emptyByteLength+dataLeft) <= rw.BlockSize || lastBlock.EndIndex == 0 {
				_, _, err := rw.getBlock(handle, lastBlock)
				if err != nil {
					return dataRead, err
				}
				// if no overwrites and pure append - then we need to create an empty buffer in between
				if emptyByteLength > 0 {
					truncated := make([]byte, emptyByteLength)
					lastBlock.Data = append(lastBlock.Data, truncated...)
				}
				lastBlock.Data = append(lastBlock.Data, data[dataRead:]...)
				newLastBlockEndIndex := lastBlock.EndIndex + dataLeft + emptyByteLength
				handle.CacheObj.Resize(lastBlock.StartIndex, newLastBlockEndIndex)
				lastBlock.Flags.Set(common.DirtyBlock)
				atomic.StoreInt64(&handle.Size, lastBlock.EndIndex)
				dataRead += int(dataLeft)
				return dataRead, nil
			}
			blk := &common.Block{
				StartIndex: lastBlock.EndIndex,
				EndIndex:   lastBlock.EndIndex + dataLeft + emptyByteLength,
				Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(handle.CacheObj.BlockIdLength)),
			}
			blk.Data = make([]byte, blk.EndIndex-blk.StartIndex)
			dataCopied = int64(copy(blk.Data[offset-blk.StartIndex:], data[dataRead:]))
			blk.Flags.Set(common.DirtyBlock)
			handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, blk)
			err := rw.putBlock(handle, blk)
			if err != nil {
				return dataRead, err
			}
			atomic.StoreInt64(&handle.Size, blk.EndIndex)
			dataRead += int(dataCopied)
			return dataRead, nil
		} else {
			return dataRead, nil
		}
	}
	return dataRead, nil
}

func (rw *ReadWriteCache) SyncFile(options internal.SyncFileOptions) error {
	log.Trace("ReadWriteCache::SyncFile : handle=%d, path=%s", options.Handle.ID, options.Handle.Path)

	err := rw.FlushFile(internal.FlushFileOptions{Handle: options.Handle})
	if err != nil {
		log.Err("Stream::SyncFile : error flushing file %s [%s]", options.Handle.Path, err.Error())
		return err
	}

	return nil
}
