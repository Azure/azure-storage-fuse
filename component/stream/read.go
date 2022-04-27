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

type ReadCache struct {
	*Stream
	StreamConnection
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
	streamOnly          bool
}

func (r *ReadCache) Configure(conf StreamOptions) error {
	if conf.BufferSizePerFile <= 0 || conf.BlockSize <= 0 || conf.HandleLimit <= 0 {
		r.streamOnly = true
	}
	r.blockSize = int64(conf.BlockSize) * mb
	r.bufferSizePerHandle = conf.BufferSizePerFile
	r.handleLimit = int32(conf.HandleLimit)
	r.openHandles = 0
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (r *ReadCache) Stop() error {
	log.Trace("Stopping component : %s", r.Name())
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		if handle.CacheObj != (handlemap.Cache{}) {
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
	return
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
	if !r.streamOnly {
		if r.openHandles >= r.handleLimit {
			err = errors.New("handle limit exceeded")
			log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
			return handle, err
		}
		atomic.AddInt32(&r.openHandles, 1)
		handlemap.CreateCacheObject(int64(r.bufferSizePerHandle), handle)
		block, exists, _ := r.getBlock(handle, 0)
		// if it exists then we can just RUnlock since we didn't manipulate its data buffer
		r.unlockBlock(block, exists)
	}
	return handle, err
}

func (r *ReadCache) getBlock(handle *handlemap.Handle, offset int64) (*common.Block, bool, error) {
	blockSize := r.blockSize
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
			Last:       (offset + blockSize) >= handle.Size,
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
		cachedBlockStartIndex := (offset - (offset % r.blockSize))
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
	if r.streamOnly {
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
	r.NextComponent().CloseFile(options)
	if !r.streamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		options.Handle.CacheObj.Purge()
		atomic.AddInt32(&r.openHandles, -1)
	}
	return nil
}
