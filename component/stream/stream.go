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
	"math"
	"strconv"
	"strings"

	"github.com/bluele/gcache"
	"github.com/pbnjay/memory"
	"go.uber.org/atomic"
)

type Stream struct {
	internal.BaseComponent
	streamCache       *cache
	streamOnly        bool // parameter used to check if its pure streaming
	fileKeyLocks      *common.LockMap
	cacheOnFileHandle bool // much better option for prefetching
}

type StreamOptions struct {
	BlockSize         uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	BlocksPerFile     int    `config:"blocks-per-file" yaml:"blocks-per-file,omitempty"`
	StreamCacheSize   uint64 `config:"cache-size-mb" yaml:"cache-size-mb,omitempty"`
	Policy            string `config:"policy" yaml:"policy,omitempty"`
	CacheOnFileHandle bool   `config:"cache-on-file-handle" yaml:"cache-on-file-handle,omitempty"`
	readOnly          bool   `config:"read-only"`
}

const (
	compName = "stream"
	mb       = 1024 * 1024
)

var _ internal.Component = &Stream{}
var nextHandleID = *atomic.NewUint64(uint64(0))

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

func (st *Stream) evictBlock(key, value interface{}) {
	// clean the block data to not leak any memory
	value.(*cacheBlock).Lock()
	value.(*cacheBlock).data = nil
	st.streamCache.evictedBlock = key.(blockKey)
	value.(*cacheBlock).Unlock()
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

	if conf.BlocksPerFile <= 0 || conf.BlockSize <= 0 || conf.StreamCacheSize <= 0 || conf.StreamCacheSize < conf.BlockSize {
		st.streamOnly = true
	} else {
		if uint64(conf.StreamCacheSize*mb) > memory.FreeMemory() {
			log.Err("Stream::Configure : config error, not enough free memory for provided configuration")
			return errors.New("not enough free memory for provided stream configuration")
		}

		// In the future when we can write streams we will allow only caching on file names for that case
		// Since if we enable write mode on handle caching it can cause issues when writing to a blob on the same block
		if !conf.readOnly && conf.CacheOnFileHandle {
			log.Err("Stream::Configure : config error, handle level caching is available for read-only mode")
			return errors.New("handle level caching is available for read-only mode")
		}

		st.cacheOnFileHandle = conf.CacheOnFileHandle
		maxBlocks := int(math.Floor(float64(conf.StreamCacheSize) / float64(conf.BlockSize)))

		// default eviction policy is LRU
		var evictionPolicy common.EvictionPolicy
		evictionPolicy.Parse(strings.ToLower(conf.Policy))

		var bc gcache.Cache
		switch evictionPolicy {
		case common.EPolicy.LFU():
			bc = gcache.New(maxBlocks).LFU().EvictedFunc(st.evictBlock).Build()
		case common.EPolicy.ARC():
			bc = gcache.New(maxBlocks).ARC().EvictedFunc(st.evictBlock).Build()
		default:
			bc = gcache.New(maxBlocks).LRU().EvictedFunc(st.evictBlock).Build()
		}
		log.Trace("Stream::Configure : cache eviction policy %s", evictionPolicy)

		st.streamCache = &cache{
			files:            make(map[string]*cacheFile),
			blockSize:        int64(conf.BlockSize) * mb,
			maxBlocks:        maxBlocks,
			blocksPerFileKey: conf.BlocksPerFile,
			blocks:           bc,
			evictionPolicy:   evictionPolicy,
		}
	}
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *Stream) Stop() error {
	log.Trace("Stopping component : %s", st.Name())
	if !st.streamOnly {
		st.streamCache.teardown()
	}
	return nil
}

func (st *Stream) getFileKey(fileName string, handleID handlemap.HandleID) string {
	if st.cacheOnFileHandle {
		return strconv.FormatUint((uint64(handleID)), 10)
	}
	return fileName
}

func (st *Stream) unlockBlock(block *cacheBlock, exists bool) {
	if exists {
		block.RUnlock()
	} else {
		block.Unlock()
	}
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
		fileKey := options.Name
		if st.cacheOnFileHandle {
			handle.ID = handlemap.HandleID(nextHandleID.Inc())
			fileKey = strconv.FormatUint((uint64(handle.ID)), 10)
		}
		st.fileKeyLocks.Lock(fileKey)
		st.streamCache.addFileKey(fileKey)
		block, exists, _ := st.fetchBlock(fileKey, handle, 0)
		// if it exists then we can just RUnlock since we didn't manipulate its data buffer
		st.unlockBlock(block, exists)
		st.streamCache.incrementHandles(fileKey)
		st.fileKeyLocks.Unlock(fileKey)
	}
	return handle, err
}

func (st *Stream) fetchBlock(fileKey string, handle *handlemap.Handle, offset int64) (*cacheBlock, bool, error) {
	block, exists := st.streamCache.getBlock(fileKey, offset, handle.Size)
	if !exists {
		// if the block does not exist fetch it from the next component
		options := internal.ReadInBufferOptions{
			Handle: handle,
			Offset: block.startIndex,
			Data:   block.data,
		}
		_, err := st.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			return nil, exists, err
		}
	}
	return block, exists, nil
}

func (st *Stream) copyCachedBlock(fileKey string, handle *handlemap.Handle, offset int64, data []byte) (int, error) {
	dataLeft := int64(len(data))
	// counter to track how much we have copied into our request buffer thus far
	dataRead := 0
	// covers the case if we get a call that is bigger than the file size
	for dataLeft > 0 && offset < handle.Size {
		// round all offsets to the specific blocksize offsets
		cachedBlockStartIndex := (offset - (offset % st.streamCache.blockSize))
		// Lock on requested block and fileName to ensure it is not being rerequested or manipulated
		block, exists, err := st.fetchBlock(fileKey, handle, cachedBlockStartIndex)
		if err != nil {
			st.unlockBlock(block, exists)
			log.Err("Stream::ReadInBuffer : failed to download block of %s with offset %d: [%s]", fileKey, block.startIndex, err.Error())
			return dataRead, err
		}
		dataCopied := int64(copy(data[dataRead:], block.data[offset-cachedBlockStartIndex:]))
		st.unlockBlock(block, exists)
		dataLeft -= dataCopied
		offset += dataCopied
		dataRead += int(dataCopied)
	}
	return dataRead, nil
}

func (st *Stream) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	fileKey := st.getFileKey(options.Handle.Path, options.Handle.ID)
	// if we're only streaming then avoid using the cache
	if st.streamOnly {
		data, err := st.NextComponent().ReadInBuffer(options)
		if err != nil && err != io.EOF {
			log.Err("Stream::ReadInBuffer : error failed to download requested data for %s: [%s]", options.Handle.Path, err.Error())
		}
		return data, err
	}
	st.streamCache.addFileKey(fileKey)
	return st.copyCachedBlock(fileKey, options.Handle, options.Offset, options.Data)
}

func (st *Stream) WriteFile(options internal.WriteFileOptions) (int, error) {
	if len(options.FileOffsets) == 0 {

	}
	return st.NextComponent().WriteFile(options)
}

func (st *Stream) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	st.NextComponent().CloseFile(options)
	if !st.streamOnly {
		fileKey := st.getFileKey(options.Handle.Path, options.Handle.ID)
		remainingHandles := st.streamCache.decrementHandles(fileKey)
		if remainingHandles <= 0 {
			st.streamCache.removeFileKey(fileKey)

		}
	}
	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewStreamComponent() internal.Component {
	comp := &Stream{}
	comp.SetName(compName)
	comp.fileKeyLocks = common.NewLockMap()
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewStreamComponent)
}
