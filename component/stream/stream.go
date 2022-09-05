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
	"context"
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type Stream struct {
	internal.BaseComponent
	cache               StreamConnection
	BlockSize           int64
	BufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	HandleLimit         int32
	CachedHandles       int32
	StreamOnly          bool // parameter used to check if its pure streaming
}

type StreamOptions struct {
	BlockSize         uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	BufferSizePerFile uint64 `config:"handle-buffer-size-mb" yaml:"handle-buffer-size-mb,omitempty"`
	HandleLimit       uint64 `config:"handle-limit" yaml:"handle-limit,omitempty"`
	readOnly          bool   `config:"read-only"`

	// v1 support
	StreamCacheMb    uint64 `config:"stream-cache-mb"`
	MaxBlocksPerFile uint64 `config:"max-blocks-per-file"`
}

const (
	compName = "stream"
	mb       = 1024 * 1024
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

func (st *Stream) Configure(_ bool) error {
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

	if config.IsSet(compName + ".max-blocks-per-file") {
		conf.BufferSizePerFile = conf.BlockSize * uint64(conf.MaxBlocksPerFile)
	}

	if config.IsSet(compName + ".stream-cache-mb") {
		conf.HandleLimit = conf.StreamCacheMb / conf.BufferSizePerFile
		if conf.HandleLimit == 0 {
			conf.HandleLimit = 1
		}
	}

	// if uint64((conf.BufferSizePerFile*conf.HandleLimit)*mb) > memory.FreeMemory() {
	// 	log.Err("Stream::Configure : config error, not enough free memory for provided configuration")
	// 	return errors.New("not enough free memory for provided stream configuration")
	// }
	st.cache = NewStreamConnection(conf, st)

	log.Info("Stream::Configure : Buffer size %v, Block size %v, Handle limit %v",
		conf.BufferSizePerFile, conf.BlockSize, conf.HandleLimit)

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *Stream) Stop() error {
	log.Trace("Stopping component : %s", st.Name())
	return st.cache.Stop()
}

func (st *Stream) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	return st.cache.CreateFile(options)
}

func (st *Stream) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	return st.cache.OpenFile(options)
}

func (st *Stream) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	// For files cached by file-cache calls will be served from libfuse layer and it will never come here
	// So its safe to assume that if call comes here then file is served by streaming layer
	return st.cache.ReadInBuffer(options)
}

func (st *Stream) WriteFile(options internal.WriteFileOptions) (int, error) {
	// For files cached by file-cache calls will be served from libfuse layer and it will never come here
	// So its safe to assume that if call comes here then file is served by streaming layer
	return st.cache.WriteFile(options)
}

func (st *Stream) FlushFile(options internal.FlushFileOptions) error {
	if options.Handle != nil && options.Handle.Cached() {
		// File is handled by file-cache, just forward the calls
		return st.NextComponent().FlushFile(options)
	}

	return st.cache.FlushFile(options)
}

func (st *Stream) CloseFile(options internal.CloseFileOptions) error {
	if options.Handle != nil && options.Handle.Cached() {
		// File is handled by file-cache, just forward the calls
		return st.NextComponent().CloseFile(options)
	}

	return st.cache.CloseFile(options)
}

func (st *Stream) DeleteFile(options internal.DeleteFileOptions) error {
	return st.cache.DeleteFile(options)
}

func (st *Stream) RenameFile(options internal.RenameFileOptions) error {
	return st.cache.RenameFile(options)
}

func (st *Stream) DeleteDir(options internal.DeleteDirOptions) error {
	return st.cache.DeleteDirectory(options)
}

func (st *Stream) RenameDir(options internal.RenameDirOptions) error {
	return st.cache.RenameDirectory(options)
}

func (st *Stream) TruncateFile(options internal.TruncateFileOptions) error {
	return st.cache.TruncateFile(options)
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
	blockSizeMb := config.AddUint64Flag("block-size-mb", 0, "Size (in MB) of a block to be downloaded during streaming.")
	config.BindPFlag(compName+".block-size-mb", blockSizeMb)

	maxBlocksMb := config.AddIntFlag("max-blocks-per-file", 0, "Maximum number of blocks to be cached in memory for streaming.")
	config.BindPFlag(compName+".max-blocks-per-file", maxBlocksMb)
	maxBlocksMb.Hidden = true

	streamCacheSize := config.AddUint64Flag("stream-cache-mb", 0, "Limit total amount of data being cached in memory to conserve memory footprint of blobfuse.")
	config.BindPFlag(compName+".stream-cache-mb", streamCacheSize)
	streamCacheSize.Hidden = true
}
