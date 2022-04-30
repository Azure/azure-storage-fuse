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

	"github.com/pbnjay/memory"
)

type Stream struct {
	internal.BaseComponent
	cache      StreamConnection
	streamOnly bool // parameter used to check if its pure streaming
}

type StreamOptions struct {
	BlockSize         uint64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	BufferSizePerFile uint64 `config:"handle-buffer-size-mb" yaml:"handle-buffer-size-mb,omitempty"`
	HandleLimit       uint64 `config:"handle-limit" yaml:"handle-limit,omitempty"`
	readOnly          bool   `config:"read-only"`
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
	if uint64((conf.BufferSizePerFile*conf.HandleLimit)*mb) > memory.FreeMemory() {
		log.Err("Stream::Configure : config error, not enough free memory for provided configuration")
		return errors.New("not enough free memory for provided stream configuration")
	}
	st.cache = NewStreamConnection(conf, st)
	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (st *Stream) Stop() error {
	log.Trace("Stopping component : %s", st.Name())
	return st.cache.Stop()
}

func (st *Stream) unlockBlock(block *common.Block, exists bool) {
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
	return st.cache.OpenFile(options)
}

func (st *Stream) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	return st.cache.ReadInBuffer(options)
}

func (st *Stream) WriteFile(options internal.WriteFileOptions) (int, error) {
	return st.cache.WriteFile(options)
}

func (st *Stream) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("Stream::CloseFile : name=%s, handle=%d", options.Handle.Path, options.Handle.ID)
	return st.cache.CloseFile(options)
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
