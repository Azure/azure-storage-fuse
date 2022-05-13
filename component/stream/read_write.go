package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync/atomic"

	"github.com/pbnjay/memory"
)

type ReadWriteCache struct {
	*Stream
	StreamConnection
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	cachedHandles       int32
	streamOnly          bool
}

func (rw *ReadWriteCache) Configure(conf StreamOptions) error {
	if conf.BufferSizePerFile <= 0 || conf.BlockSize <= 0 || conf.HandleLimit <= 0 {
		rw.streamOnly = true
	}
	rw.blockSize = int64(conf.BlockSize) * mb
	rw.bufferSizePerHandle = conf.BufferSizePerFile
	rw.handleLimit = int32(conf.HandleLimit)
	rw.cachedHandles = 0
	return nil
}

func (rw *ReadWriteCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	handle, err := rw.NextComponent().CreateFile(options)
	if err != nil {
		log.Err("Stream::CreateFile : error failed to create file %s: [%s]", options.Name, err.Error())
	}
	if !rw.streamOnly {
		err = rw.createHandleCache(handle)
		if err != nil {
			log.Err("Stream::OpenFile : error opening file %s [%s]", options.Name, err.Error())
			return handle, err
		}
	}
	return handle, nil
}

func (rw *ReadWriteCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)
	handle, err := rw.NextComponent().OpenFile(options)
	if err != nil {
		log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
		return handle, err
	}
	if !rw.streamOnly {
		err = rw.createHandleCache(handle)
		if err != nil {
			log.Err("Stream::OpenFile : error opening file %s [%s]", options.Name, err.Error())
			return handle, err
		}
	}
	return handle, err
}

func (rw *ReadWriteCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	log.Trace("Stream::ReadInBuffer : name=%s, handle=%d, offset=%d", options.Handle.Path, options.Handle.ID, options.Offset)
	// if we're not in stream only mode and our handle is stream only - check if memory cleared up
	if !rw.streamOnly && options.Handle.CacheObj.StreamOnly {
		err := rw.createHandleCache(options.Handle)
		if err != nil {
			log.Err("Stream::ReadInBuffer : error reading file %s [%s]", options.Handle.Path, err.Error())
			return 0, err
		}
	}
	if rw.streamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	if atomic.LoadInt64(&options.Handle.Size) == 0 {
		return 0, nil
	}
	return rw.readWriteBlocks(options.Handle, options.Offset, options.Data, false)
}

func (rw *ReadWriteCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("Stream::WriteFile : name=%s, handle=%d, offset=%d", options.Handle.Path, options.Handle.ID, options.Offset)
	if !rw.streamOnly && options.Handle.CacheObj.StreamOnly {
		err := rw.createHandleCache(options.Handle)
		if err != nil {
			log.Err("Stream::ReadInBuffer : error reading file %s [%s]", options.Handle.Path, err.Error())
			return 0, err
		}
	}
	if rw.streamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().WriteFile(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::WriteFile : error failed to write data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return rw.readWriteBlocks(options.Handle, options.Offset, options.Data, true)
}

func (rw *ReadWriteCache) TruncateFile(options internal.TruncateFileOptions) error {
	if !rw.streamOnly {
		handleMap := handlemap.GetHandles()
		handleMap.Range(func(key, value interface{}) bool {
			handle := value.(*handlemap.Handle)
			if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
				if handle.Path == options.Name {
					handle.CacheObj.Lock()
					rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
					handle.CacheObj.Purge()
					atomic.StoreInt64(&handle.Size, options.Size)
					handle.CacheObj.StreamOnly = true
					atomic.AddInt32(&rw.cachedHandles, -1)
					handle.CacheObj.Unlock()
				}
			}
			return true
		})
	}
	err := rw.NextComponent().TruncateFile(options)
	if err != nil {
		log.Err("Stream::TruncateFile : error truncating file %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

func (rw *ReadWriteCache) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("Stream::RenameFile : name=%s", options.Src)
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
			if handle.Path == options.Src {
				log.Trace("Stream::RenameFile : found matching handle handle=%d", handle)
				handle.CacheObj.Lock()
				rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
				handle.CacheObj.Purge()
				handle.CacheObj.StreamOnly = true
				atomic.AddInt32(&rw.cachedHandles, -1)
				handle.CacheObj.Unlock()
			}
		}
		return true
	})
	err := rw.NextComponent().RenameFile(options)
	if err != nil {
		log.Err("Stream::RenameFile : error renaming file %s [%s]", options.Src, err.Error())
		return err
	}
	return nil
}

func (rw *ReadWriteCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	rw.NextComponent().CloseFile(options)
	if !rw.streamOnly && !options.Handle.CacheObj.StreamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: options.Handle})
		options.Handle.CacheObj.Purge()
		options.Handle.CacheObj.StreamOnly = true
		atomic.AddInt32(&rw.cachedHandles, -1)
	}
	return nil
}

func (rw *ReadWriteCache) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("Stream::DeleteFile : name=%s", options.Name)
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
			if handle.Path == options.Name {
				log.Trace("Stream::DeleteFile : found matching handle=%d", handle)
				handle.CacheObj.Lock()
				handle.CacheObj.Purge()
				handle.CacheObj.StreamOnly = true
				atomic.AddInt32(&rw.cachedHandles, -1)
				handle.CacheObj.Unlock()
			}
		}
		return true
	})
	err := rw.NextComponent().DeleteFile(options)
	if err != nil {
		log.Err("Stream::DeleteFile : error deleting file %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

func (rw *ReadWriteCache) DeleteDirectory(options internal.DeleteDirOptions) error {
	log.Trace("Stream::DeleteDirectory : name=%s", options.Name)
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
			if strings.HasPrefix(handle.Path, options.Name) {
				log.Trace("Stream::DeleteDir : found matching handle %d", handle.Path)
				handle.CacheObj.Lock()
				handle.CacheObj.Purge()
				handle.CacheObj.StreamOnly = true
				atomic.AddInt32(&rw.cachedHandles, -1)
				handle.CacheObj.Unlock()
			}
		}
		return true
	})
	err := rw.NextComponent().DeleteDir(options)
	if err != nil {
		log.Err("Stream::DeleteDirectory : error deleting directory %s [%s]", options.Name, err.Error())
		return err
	}
	return nil
}

func (rw *ReadWriteCache) RenameDirectory(options internal.RenameDirOptions) error {
	log.Trace("Stream::RenameDirectory : name=%s", options.Src)
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
			if strings.HasPrefix(handle.Path, options.Src) {
				handle.CacheObj.Lock()
				rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
				handle.CacheObj.Purge()
				handle.CacheObj.StreamOnly = true
				atomic.AddInt32(&rw.cachedHandles, -1)
				handle.CacheObj.Unlock()
			}
		}
		return true
	})
	err := rw.NextComponent().RenameDir(options)
	if err != nil {
		log.Err("Stream::RenameDirectory : error renaming directory %s [%s]", options.Src, err.Error())
		return err
	}
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (rw *ReadWriteCache) Stop() error {
	log.Trace("Stopping component : %s", rw.Name())
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != nil && !handle.CacheObj.StreamOnly {
			handle.CacheObj.Lock()
			handle.CacheObj.Purge()
			handle.CacheObj.StreamOnly = true
			atomic.AddInt32(&rw.cachedHandles, -1)
			handle.CacheObj.Unlock()
		}
		return true
	})
	return nil
}

func (rw *ReadWriteCache) createHandleCache(handle *handlemap.Handle) error {

	handlemap.CreateCacheObject(int64(rw.bufferSizePerHandle), handle)
	handle.CacheObj.Lock()
	defer handle.CacheObj.Unlock()
	// if we hit handle limit then stream only on this new handle
	if atomic.LoadInt32(&rw.cachedHandles) >= rw.handleLimit {
		handle.CacheObj.StreamOnly = true
		return nil
	}
	opts := internal.GetFileBlockOffsetsOptions{
		Name: handle.Path,
	}
	offsets, _ := rw.NextComponent().GetFileBlockOffsets(opts)
	handle.CacheObj.BlockOffsetList = offsets
	// if its a small file then download the file in its entirety if there is memory available, otherwise stream only
	if handle.CacheObj.SmallFile() {
		if uint64(atomic.LoadInt64(&handle.Size)*mb) > memory.FreeMemory() {
			handle.CacheObj.StreamOnly = true
			return nil
		}
		block, _, err := rw.getBlock(handle, &common.Block{StartIndex: 0, EndIndex: handle.Size})
		block.Id = base64.StdEncoding.EncodeToString(common.NewUUID().Bytes())
		if err != nil {
			rw.unlockBlock(block, false)
			return err
		}
		atomic.AddInt32(&rw.cachedHandles, 1)
		// our handle will consist of a single block locally for simpler logic
		handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, block)
		handle.CacheObj.BlockIdLength = common.GetIdLength(block.Id)
		rw.unlockBlock(block, false)
		return nil
	}
	atomic.AddInt32(&rw.cachedHandles, 1)
	return nil
}

func (rw *ReadWriteCache) putBlock(handle *handlemap.Handle, block *common.Block) error {
	ok := handle.CacheObj.Put(block.StartIndex, block)
	// if the cache is full and we couldn't evict - we need to do a flush
	if !ok {
		rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
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
		block.Lock()
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
	} else {
		cached_block.RLock()
		return block, true, nil
	}
}

func (rw *ReadWriteCache) readWriteBlocks(handle *handlemap.Handle, offset int64, data []byte, write bool) (int, error) {
	handle.CacheObj.Lock()
	defer handle.CacheObj.Unlock()
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
			block, exists, err := rw.getBlock(handle, blocks[blk_index])
			if err != nil {
				rw.unlockBlock(block, exists)
				err := errors.New("could not retrieve required block")
				return dataRead, err
			}
			if write {
				dataCopied = int64(copy(block.Data[offset-blocks[blk_index].StartIndex:], data[dataRead:]))
				block.Flags.Set(common.DirtyBlock)
			} else {
				dataCopied = int64(copy(data[dataRead:], block.Data[offset-blocks[blk_index].StartIndex:]))
			}
			rw.unlockBlock(block, exists)
			dataLeft -= dataCopied
			offset += dataCopied
			dataRead += int(dataCopied)
			blk_index += 1
			//if appending to file
		} else if write {
			emptyByteLength := offset - lastBlock.EndIndex
			// if the data to append + our last block existing data do not exceed block size - just append to last block
			if (lastBlock.EndIndex-lastBlock.StartIndex)+(emptyByteLength+dataLeft) <= rw.blockSize {
				_, exists, err := rw.getBlock(handle, lastBlock)
				if err != nil {
					rw.unlockBlock(lastBlock, exists)
					log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, lastBlock.StartIndex, err.Error())
					return dataRead, err
				}
				// if no overwrites and pure append - then we need to create an empty buffer in between
				if emptyByteLength > 0 {
					truncated := make([]byte, emptyByteLength)
					lastBlock.Data = append(lastBlock.Data, truncated...)
				}
				lastBlock.Data = append(lastBlock.Data, data[dataRead:]...)
				lastBlock.EndIndex += dataLeft + emptyByteLength
				lastBlock.Flags.Set(common.DirtyBlock)
				atomic.StoreInt64(&handle.Size, lastBlock.EndIndex)
				rw.unlockBlock(lastBlock, exists)
				dataRead += int(dataLeft)
				rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
				return dataRead, nil
			}
			blk := &common.Block{
				StartIndex: lastBlock.EndIndex,
				EndIndex:   lastBlock.EndIndex + dataLeft + emptyByteLength,
				Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(handle.CacheObj.BlockIdLength)),
			}
			blk.Data = make([]byte, blk.EndIndex-blk.StartIndex)
			dataCopied = int64(copy(blk.Data[offset-blocks[blk_index].StartIndex:], data[dataRead:]))
			handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, blk)
			rw.putBlock(handle, blk)
			atomic.StoreInt64(&handle.Size, blk.EndIndex)
			dataRead += int(dataCopied)
			rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
			return dataRead, nil
		}
		return dataRead, nil
	}
	return dataRead, nil
}
