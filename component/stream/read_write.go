package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"errors"
	"io"
	"sync/atomic"

	"github.com/pbnjay/memory"
)

type ReadWriteCache struct {
	*Stream
	StreamConnection
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	cachedHandles       int32
	streamOnly          bool
}

func (rw *ReadWriteCache) Configure(conf StreamOptions) error {
	if conf.BufferSizePerFile <= 0 || conf.HandleLimit <= 0 {
		rw.streamOnly = true
	}
	rw.bufferSizePerHandle = conf.BufferSizePerFile
	rw.handleLimit = int32(conf.HandleLimit)
	rw.cachedHandles = 0
	return nil
}

func (rw *ReadWriteCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)
	handle, err := rw.NextComponent().OpenFile(options)
	if err != nil {
		log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
		return handle, err
	}
	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}
	if !rw.streamOnly {
		// if we hit handle limit then stream only on this new handle
		if rw.cachedHandles >= rw.handleLimit {
			log.Trace("Stream::OpenFile : file handle limit exceeded - switch handle to stream only mode %s [%s]", options.Name, handle.ID)
			handle.CacheObj.StreamOnly = true
			return handle, nil
		}
		atomic.AddInt32(&rw.cachedHandles, 1)
		handlemap.CreateCacheObject(int64(rw.bufferSizePerHandle), handle)
		opts := internal.GetFileBlockOffsetsOptions{
			Name: handle.Path,
		}
		offsets, _ := rw.NextComponent().GetFileBlockOffsets(opts)
		offsets.Cached = true
		handle.CacheObj.BlockOffsetList = offsets
		// if its a small file then download the file in its entirety if there is memory available, otherwise stream only
		if handle.CacheObj.SmallFile {
			if uint64(handle.Size*mb) > memory.FreeMemory() {
				handle.CacheObj.StreamOnly = true
				return handle, err
			}
			block, _, err := rw.getBlock(handle, 0, &common.Block{StartIndex: 0, EndIndex: handle.Size})
			if err != nil {
				log.Err("Stream::OpenFile : error downloading small file %s [%s]", options.Name, err.Error())
				rw.unlockBlock(block, false)
				return handle, err
			}
			// our handle will consist of a single block locally for simpler logic
			handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, block)
			rw.unlockBlock(block, false)
		}
	}
	return handle, err
}

func (rw *ReadWriteCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	if rw.streamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return rw.readWriteBlocks(options.Handle, options.Offset, options.Data, false)
}

func (rw *ReadWriteCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("Stream::WriteFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	if rw.streamOnly || options.Handle.CacheObj.StreamOnly {
		data, err := rw.NextComponent().WriteFile(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::WriteFile : error failed to write data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return rw.readWriteBlocks(options.Handle, options.Offset, options.Data, true)
}

func (rw *ReadWriteCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	rw.NextComponent().CloseFile(options)
	if !rw.streamOnly && !options.Handle.CacheObj.StreamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: options.Handle})
		options.Handle.CacheObj.Purge()
		atomic.AddInt32(&rw.cachedHandles, -1)
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
			handle.CacheObj.Unlock()
		}
		return true
	})
	return nil
}

func (rw *ReadWriteCache) getBlock(handle *handlemap.Handle, offset int64, block *common.Block) (*common.Block, bool, error) {
	blockKeyObj := offset
	handle.CacheObj.Lock()
	cached_block, found := handle.CacheObj.Get(blockKeyObj)
	if !found {
		block.Lock()
		block.Data = make([]byte, block.EndIndex-block.StartIndex)
		ok := handle.CacheObj.Put(blockKeyObj, block)
		// if the cache is full and we couldn't evict - we need to do a flush
		if !ok {
			rw.NextComponent().FlushFile(internal.FlushFileOptions{Handle: handle})
			ok = handle.CacheObj.Put(blockKeyObj, block)
			if !ok {
				return block, false, errors.New("flushed and still unable to put block in cache")
			}
		}
		handle.CacheObj.Unlock()
		options := internal.ReadInBufferOptions{
			Handle: handle,
			Offset: block.StartIndex,
			Data:   block.Data,
		}
		_, err := rw.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			return nil, false, err
		}
		return block, false, nil
	} else {
		cached_block.RLock()
		handle.CacheObj.Unlock()
		return block, true, nil
	}
}

func (rw *ReadWriteCache) readWriteBlocks(handle *handlemap.Handle, offset int64, data []byte, write bool) (int, error) {
	// if it's not a small file then we look the blocks it consistts of
	if !handle.CacheObj.SmallFile {
		blocks, found := handle.CacheObj.FindBlocksToRead(offset, int64(len(data)))
		if !found {
			return 0, errors.New("block does not exist")
		}
		dataLeft := int64(len(data))
		dataRead := 0
		blk_index := 0
		dataCopied := int64(0)
		for dataLeft > 0 && offset < handle.Size {
			block, exists, err := rw.getBlock(handle, blocks[blk_index].StartIndex, blocks[blk_index])
			if err != nil {
				rw.unlockBlock(block, exists)
				log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
				return dataRead, err
			}
			if write {
				dataCopied = int64(copy(block.Data[offset-blocks[blk_index].StartIndex:], data[dataRead:]))
				block.Dirty = true
			} else {
				dataCopied = int64(copy(data[dataRead:], block.Data[offset-blocks[blk_index].StartIndex:]))
			}
			rw.unlockBlock(block, exists)
			dataLeft -= dataCopied
			offset += dataCopied
			dataRead += int(dataCopied)
			blk_index += 1
		}
		return dataRead, nil
	} else {
		// we know a small, i.e., file consists of a single block
		// small files don't have delayed flushes
		block, exists, err := rw.getBlock(handle, 0, handle.CacheObj.BlockList[0])
		if err != nil {
			rw.unlockBlock(block, exists)
			log.Err("Stream::ReadInBuffer : failed to retrieve small file %s block with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
		}
		if write {
			_ = int64(copy(block.Data[offset:], data))
			block.Dirty = true
		} else {
			_ = int64(copy(data, block.Data[offset:]))
		}
		rw.unlockBlock(block, exists)
	}
	return len(data), nil
}
