/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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
	"blobfuse2/common"
	"blobfuse2/common/config"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/pbnjay/memory"
)

type Stream struct {
	internal.BaseComponent
	cache      *cache
	streamOnly bool // parameter used to check if its pure streaming
}

type StreamOptions struct {
	BlockSize         uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	BufferSizePerFile uint64 `config:"handle-buffer-size-mb" yaml:"handle-buffer-size-mb,omitempty"`
	HandleLimit       uint64 `config:"handle-limit" yaml:"handle-limit,omitempty"`
	readOnly          bool   `config:"read-only"`
}

type cache struct {
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
}

const (
	compName              = "stream"
	mb                    = 1024 * 1024
	defaultDiskTimeoutSec = (30 * 60)
)

var _ internal.Component = &Stream{}

func (st *Stream) Name() string {
	return compName
}

func (st *Stream) SetName(name string) {
	st.BaseComponent.SetName(name)
}

func (st *Stream) SetNextComponent(nc internal.Component) {
	st.BaseComponent.SetNextComponent(nc)
}

func (st *Stream) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

func (st *Stream) Start(ctx context.Context) error {
	log.Trace("Starting component : %s", st.Name())
	return nil
}

func (st *Stream) Configure() error {
	log.Trace("Stream::Configure : %s", st.Name())
	conf := StreamOptions{}
	err := config.UnmarshalKey(compName, &conf)
	if err != nil {
		log.Err("Stream::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", st.Name(), err.Error())
	}

	err = config.UnmarshalKey("read-only", &conf.readOnly)
	if err != nil {
		log.Err("Stream::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("config error in %s [%s]", st.Name(), err.Error())
	}

	if !conf.readOnly {
		log.Err("Stream::Configure : config error, Stream component is available for read-only mode")
		return errors.New("stream component is available only for read-only mount")
	}

	if conf.BufferSizePerFile <= 0 || conf.BlockSize <= 0 || conf.HandleLimit <= 0 {
		st.streamOnly = true
	} else {
		if uint64((conf.BufferSizePerFile*conf.HandleLimit)*mb) > memory.FreeMemory() {
			log.Err("Stream::Configure : config error, not enough free memory for provided configuration")
			return errors.New("not enough free memory for provided stream configuration")
		}
		if !conf.readOnly {
			log.Err("Stream::Configure : config error, handle level caching is available for read-only mode")
			return errors.New("handle level caching is available for read-only mode")
		}
	}
	st.cache = &cache{
		blockSize:           int64(conf.BlockSize) * mb,
		bufferSizePerHandle: conf.BufferSizePerFile * mb,
		handleLimit:         int32(conf.HandleLimit),
		openHandles:         0,
	}
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *Stream) Stop() error {
	log.Trace("Stopping component : %s", st.Name())
	handleMap := handlemap.GetHandles()
	handleMap.Range(func(key, value interface{}) bool {
		handle := value.(*handlemap.Handle)
		handle.CacheObj.Lock()
		handle.CacheObj.Purge()
		handle.CacheObj.Unlock()
		return true
	})
	return nil
}

func (st *Stream) unlockBlock(block *common.CacheBlock, exists bool) {
	if exists {
		block.RUnlock()
	} else {
		block.Unlock()
	}
	return
}

func (st *Stream) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Stream::OpenFile : name=%s, flags=%d, mode=%s", options.Name, options.Flags, options.Mode)
	handle, err := st.NextComponent().OpenFile(options)
	if err != nil {
		log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
		return handle, err
	}
	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}
	if !st.streamOnly {
		if st.cache.openHandles >= st.cache.handleLimit {
			err = errors.New("handle limit exceeded")
			log.Err("Stream::OpenFile : error %s [%s]", options.Name, err.Error())
			return handle, err
		}
		atomic.AddInt32(&st.cache.openHandles, 1)
		handlemap.CreateCacheObject(int64(st.cache.bufferSizePerHandle), handle)
		block, exists, _ := st.getBlock(handle, 0)
		// if it exists then we can just RUnlock since we didn't manipulate its data buffer
		st.unlockBlock(block, exists)
	}
	return handle, err
}

func (st *Stream) getBlock(handle *handlemap.Handle, offset int64) (*common.CacheBlock, bool, error) {
	blockSize := st.cache.blockSize
	blockKeyObj := offset
	handle.CacheObj.Lock()
	block, found := handle.CacheObj.Get(blockKeyObj)
	if !found {
		if (offset + blockSize) > handle.Size {
			blockSize = handle.Size - offset
		}
		block = &common.CacheBlock{
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
		_, err := st.NextComponent().ReadInBuffer(options)
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

func (st *Stream) copyCachedBlock(handle *handlemap.Handle, offset int64, data []byte) (int, error) {
	dataLeft := int64(len(data))
	// counter to track how much we have copied into our request buffer thus far
	dataRead := 0
	// covers the case if we get a call that is bigger than the file size
	for dataLeft > 0 && offset < handle.Size {
		// round all offsets to the specific blocksize offsets
		cachedBlockStartIndex := (offset - (offset % st.cache.blockSize))
		// Lock on requested block and fileName to ensure it is not being rerequested or manipulated
		block, exists, err := st.getBlock(handle, cachedBlockStartIndex)
		if err != nil {
			st.unlockBlock(block, exists)
			log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", handle.Path, block.StartIndex, err.Error())
			return dataRead, err
		}
		dataCopied := int64(copy(data[dataRead:], block.Data[offset-cachedBlockStartIndex:]))
		st.unlockBlock(block, exists)
		dataLeft -= dataCopied
		offset += dataCopied
		dataRead += int(dataCopied)
	}
	return dataRead, nil
}

func (st *Stream) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// if we're only streaming then avoid using the cache
	if st.streamOnly {
		data, err := st.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	return st.copyCachedBlock(options.Handle, options.Offset, options.Data)
}

func (st *Stream) WriteFile(options internal.WriteFileOptions) (int, error) {
	return st.NextComponent().WriteFile(options)
}

func (st *Stream) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	st.NextComponent().CloseFile(options)
	if !st.streamOnly {
		options.Handle.CacheObj.Lock()
		defer options.Handle.CacheObj.Unlock()
		options.Handle.CacheObj.Purge()
		atomic.AddInt32(&st.cache.openHandles, -1)
	}
	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewStreamComponent() internal.Component {
	comp := &Stream{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewStreamComponent)
}
