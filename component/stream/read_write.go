package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"encoding/base64"
	"errors"
	"io"
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
		handlemap.CreateCacheObject(int64(rw.bufferSizePerHandle), handle)
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
	return rw.readWriteBlocks(options.Handle, options.Offset, options.Data, false)
}

func (rw *ReadWriteCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Trace("Stream::WriteFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
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
			atomic.AddInt32(&rw.cachedHandles, -1)
			handle.CacheObj.Unlock()
		}
		return true
	})
	return nil
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

func (rw *ReadWriteCache) createHandleCache(handle *handlemap.Handle) error {
	// if we hit handle limit then stream only on this new handle
	if rw.cachedHandles >= rw.handleLimit {
		handle.CacheObj.StreamOnly = true
		return nil
	}
	handlemap.CreateCacheObject(int64(rw.bufferSizePerHandle), handle)
	opts := internal.GetFileBlockOffsetsOptions{
		Name: handle.Path,
	}
	offsets, _ := rw.NextComponent().GetFileBlockOffsets(opts)
	handle.CacheObj.BlockOffsetList = offsets
	// if its a small file then download the file in its entirety if there is memory available, otherwise stream only
	if handle.CacheObj.SmallFile() {
		if uint64(handle.Size*mb) > memory.FreeMemory() {
			handle.CacheObj.StreamOnly = true
			return nil
		}
		block, _, err := rw.getBlock(handle, 0, &common.Block{StartIndex: 0, EndIndex: handle.Size})
		block.Id = base64.StdEncoding.EncodeToString(common.NewUUID().Bytes())
		if err != nil {
			rw.unlockBlock(block, false)
			return err
		}
		atomic.AddInt32(&rw.cachedHandles, 1)
		// our handle will consist of a single block locally for simpler logic
		handle.CacheObj.BlockList = append(handle.CacheObj.BlockList, block)
		rw.unlockBlock(block, false)
	} else {
		atomic.AddInt32(&rw.cachedHandles, 1)
	}
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
	blocks, _ := handle.CacheObj.FindBlocksToRead(offset, int64(len(data)))
	dataLeft := int64(len(data))
	dataRead, blk_index, dataCopied := 0, 0, int64(0)
	for dataLeft > 0 {
		if offset < handle.Size {
			block, exists, err := rw.getBlock(handle, blocks[blk_index].StartIndex, blocks[blk_index])
			if err != nil {
				rw.unlockBlock(block, exists)
				log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
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
			lastBlock := handle.CacheObj.BlockList[len(handle.CacheObj.BlockList)-1]
			if (lastBlock.EndIndex-lastBlock.StartIndex)+((offset-lastBlock.EndIndex)+dataLeft) <= rw.blockSize {
				_, exists, err := rw.getBlock(handle, lastBlock.StartIndex, lastBlock)
				if err != nil {
					rw.unlockBlock(lastBlock, exists)
					log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, lastBlock.StartIndex, err.Error())
					return dataRead, err
				}
				if offset-lastBlock.EndIndex > 0 {
					truncated := make([]byte, offset-lastBlock.EndIndex)
					lastBlock.Data = append(lastBlock.Data, truncated...)
				}
				lastBlock.Data = append(lastBlock.Data, data[dataRead:]...)
				lastBlock.EndIndex += dataLeft
				lastBlock.Flags.Set(common.DirtyBlock)
				rw.unlockBlock(lastBlock, exists)
				dataRead += int(dataLeft)
				return dataRead, nil
			}
			blockIdLength := common.GetIdLength(lastBlock.Id)
			blk := &common.Block{
				StartIndex: lastBlock.EndIndex,
				EndIndex:   lastBlock.EndIndex + dataLeft,
				Data:       make([]byte, dataLeft),
				Id:         base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(blockIdLength)),
			}
			dataCopied = int64(copy(blk.Data[offset-blocks[blk_index].StartIndex:], data[dataRead:]))
			dataRead += int(dataCopied)
			return dataRead, nil
		} else {
			return dataRead, nil
		}
	}
	return dataRead, nil
}
