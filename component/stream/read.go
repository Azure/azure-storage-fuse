package stream

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"io"
	"sync/atomic"
	"syscall"
)

type ReadCache struct {
	*Stream
	StreamConnection
}

func (r *ReadCache) Configure(conf StreamOptions) error {
	if conf.BufferSizePerFile <= 0 || conf.BlockSize <= 0 || conf.HandleLimit <= 0 {
		r.StreamOnly = true
	}
	r.BlockSize = int64(conf.BlockSize) * mb
	r.BufferSizePerHandle = conf.BufferSizePerFile * mb
	r.HandleLimit = int32(conf.HandleLimit)
	r.CachedHandles = 0
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
	if !r.StreamOnly {
		if r.CachedHandles >= r.HandleLimit {
			log.Trace("Stream::OpenFile : file handle limit exceeded - switch handle to stream only mode %s [%s]", options.Name, handle.ID)
			handle.CacheObj.StreamOnly = true
			return handle, nil
		}
		atomic.AddInt32(&r.CachedHandles, 1)
		handlemap.CreateCacheObject(int64(r.BufferSizePerHandle), handle)
		block, exists, _ := r.getBlock(handle, 0)
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
	r.NextComponent().CloseFile(options)
	if !r.StreamOnly && !options.Handle.CacheObj.StreamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		options.Handle.CacheObj.Purge()
		atomic.AddInt32(&r.CachedHandles, -1)
	}
	return nil
}

func (r *ReadCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	return 0, syscall.ENOTSUP
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
