package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"errors"
	"io"
	"sync/atomic"
)

type ReadWriteCache struct {
	*Stream
	StreamConnection
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
	streamOnly          bool
}

func (rw *ReadWriteCache) Configure(conf StreamOptions) error {
	if conf.BufferSizePerFile <= 0 || conf.HandleLimit <= 0 {
		rw.streamOnly = true
	}
	rw.bufferSizePerHandle = conf.BufferSizePerFile
	rw.handleLimit = int32(conf.HandleLimit)
	rw.openHandles = 0
	return nil
}

func (rw *ReadWriteCache) getBlock(handle *handlemap.Handle, offset int64, block *common.Block) (*common.Block, bool, error) {
	blockKeyObj := offset
	handle.CacheObj.Lock()
	block, found := handle.CacheObj.Get(blockKeyObj)
	if !found {
		block.Data = make([]byte, block.EndIndex-block.StartIndex)
		block.Lock()
		handle.CacheObj.Put(blockKeyObj, block)
		handle.CacheObj.Unlock()
		// if the block does not exist fetch it from the next component
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
		block.RLock()
		handle.CacheObj.Unlock()
		return block, true, nil
	}
}

func (rw *ReadWriteCache) copyCachedBlock(handle *handlemap.Handle, offset int64, data []byte) (int, error) {
	blocks, found := handle.CacheObj.BlockOffsetList.FindBlocksToRead(offset, int64(len(data)))
	if !found {
		return 0, errors.New("yo")
	}
	dataLeft := int64(len(data))
	// counter to track how much we have copied into our request buffer thus far
	dataRead := 0
	i := 0
	// covers the case if we get a call that is bigger than the file size
	for dataLeft > 0 && offset < handle.Size {
		// Lock on requested block and fileName to ensure it is not being rerequested or manipulated
		block, exists, err := rw.getBlock(handle, blocks[i].StartIndex, blocks[i])
		if err != nil {
			rw.unlockBlock(block, exists)
			log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
			return dataRead, err
		}
		dataCopied := int64(copy(data[dataRead:], block.Data[offset-blocks[i].StartIndex:]))
		rw.unlockBlock(block, exists)
		dataLeft -= dataCopied
		offset += dataCopied
		dataRead += int(dataCopied)
		i += 1
	}
	return dataRead, nil
}

func (rw *ReadWriteCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// if we're only streaming then avoid using the cache
	if rw.streamOnly {
		data, err := rw.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return rw.copyCachedBlock(options.Handle, options.Offset, options.Data)
}

func (rw *ReadWriteCache) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	rw.NextComponent().CloseFile(options)
	if !rw.streamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		options.Handle.CacheObj.Purge()
		atomic.AddInt32(&rw.openHandles, -1)
	}
	return nil
}
func (rw *ReadWriteCache) GetFileBlockOffsets(options internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	return rw.NextComponent().GetFileBlockOffsets(options)

}

func (rw *ReadWriteCache) Write(options internal.WriteFileOptions) (*handlemap.Handle, error) {
	return nil, nil
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
		if rw.openHandles >= rw.handleLimit {
			err = errors.New("handle limit exceeded")
			log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
			return handle, err
		}
		atomic.AddInt32(&rw.openHandles, 1)
		handlemap.CreateCacheObject(int64(rw.bufferSizePerHandle), handle)
		opts := internal.GetFileBlockOffsetsOptions{
			Name: handle.Path,
		}
		offsets, _ := rw.GetFileBlockOffsets(opts)
		offsets.Cached = true
		handle.CacheObj.BlockOffsetList = offsets
	}
	return handle, err
}

// Stop : Stop the component functionality and kill all threads started
func (rw *ReadWriteCache) Stop() error {
	log.Trace("Stopping component : %s", rw.Name())
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
