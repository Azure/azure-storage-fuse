/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package xload

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Common structure for Component
type Xload struct {
	internal.BaseComponent
	blockSize uint64 // Size of each block to be cached
	memSize   uint64 // Mem size to be used for caching at the startup
	mode      Mode   // Mode of the Xload component
	path      string // path to local disk containing the files to be uploaded
	en        enumerator
	fs        *fileSpiltter
}

// Structure defining your config parameters
type XloadOptions struct {
	BlockSize float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	MemSize   uint64  `config:"mem-size-mb" yaml:"mem-size-mb,omitempty"`
	Mode      string  `config:"mode" yaml:"mode,omitempty"`
	Path      string  `config:"path" yaml:"mode,omitempty"`
}

const compName = "xload"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &Xload{}

func (xl *Xload) Name() string {
	return compName
}

func (xl *Xload) SetName(name string) {
	xl.BaseComponent.SetName(name)
}

func (xl *Xload) SetNextComponent(nc internal.Component) {
	xl.BaseComponent.SetNextComponent(nc)
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (xl *Xload) Configure(_ bool) error {
	log.Trace("Xload::Configure : %s", xl.Name())

	conf := XloadOptions{}
	err := config.UnmarshalKey(xl.Name(), &conf)
	if err != nil {
		log.Err("Xload::Configure : config error [invalid config attributes]")
		return fmt.Errorf("Xload: config error [invalid config attributes]")
	}

	xl.blockSize = uint64(16) * _1MB // 16 MB as deafult block size
	if config.IsSet(compName + ".block-size-mb") {
		xl.blockSize = uint64(conf.BlockSize * float64(_1MB))
	}

	xl.memSize = uint64(4192) * _1MB // 4 GB as default mem size
	if config.IsSet(compName + ".mem-size-mb") {
		xl.memSize = conf.MemSize * _1MB
	}

	var mode Mode
	err = mode.Parse(conf.Mode)
	if err != nil {
		log.Err("Xload::Configure : Failed to parse mode %s [%s]", conf.Mode, err.Error())
		return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
	}

	if mode == EMode.INVALID_MODE() {
		log.Err("Xload::Configure : Invalid mode : %s", conf.Mode)
		return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
	}

	xl.mode = mode

	xl.path = strings.TrimSpace(conf.Path)

	return nil
}

// Start : Pipeline calls this method to start the component functionality
func (xl *Xload) Start(ctx context.Context) error {
	log.Trace("Xload::Start : Starting component %s", xl.Name())

	// Xload : start code goes here
	switch xl.mode {
	case EMode.CHECKPOINT():
		// Start checkpoint thread here
	case EMode.DOWNLOAD():
		// Start downloader here
	case EMode.UPLOAD():
		// Start uploader here
		go xl.Upload()

	case EMode.SYNC():
		//Start syncer here
	default:
		log.Err("Xload::Start : Invalid mode : %s", xl.mode.String())
		return fmt.Errorf("invalid mode in xload : %s", xl.mode.String())
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (xl *Xload) Stop() error {
	log.Trace("Xload::Stop : Stopping component %s", xl.Name())

	if xl.en.getInputPool() != nil {
		xl.en.getInputPool().Stop()
	}

	if xl.fs.inputPool != nil {
		xl.fs.inputPool.Stop()
	}

	return nil
}

func (xl *Xload) Upload() error {
	if len(xl.path) == 0 {
		log.Err("Xload::Upload : Path not given for upload")
		return fmt.Errorf("local path not given for upload")
	}

	// create local lister
	var err error
	xl.en, err = newLocalLister(xl.path, xl.NextComponent())
	if err != nil {
		log.Err("Xload::Upload : Unable to create local lister [%s]", err.Error())
		return err
	}

	// start input threadpool
	xl.en.getInputPool().Start()

	// create file splitter
	xl.fs, err = newFileSpiltter()
	if err != nil {
		log.Err("Xload::Upload : Unable to create file splitter [%s]", err.Error())
		return err
	}

	xl.fs.inputPool.Start()

	xl.en.setOutputPool(xl.fs.inputPool)

	// add base path in the input channel for listing
	xl.en.getInputPool().Schedule(&workItem{
		basePath: xl.path,
	})

	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
func NewXloadComponent() internal.Component {
	comp := &Xload{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewXloadComponent)
}
