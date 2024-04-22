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
	"os"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Common structure for Component
type Xload struct {
	internal.BaseComponent
	blockSize uint64 // Size of each block to be cached
	mode      Mode   // Mode of the Xload component

	workerCount   uint32 // Number of workers running
	en            enumerator
	dataMgrPool   *ThreadPool // Thread Pool for data upload download
	dataSplitPool *ThreadPool // Thread Pool for chunking of a file
	blockPool     *BlockPool  // Pool of blocks
	path          string      // Path on local disk where Xload will operate
}

// Structure defining your config parameters
type XloadOptions struct {
	BlockSize float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	Mode      string  `config:"mode" yaml:"mode,omitempty"`
	Path      string  `config:"path" yaml:"path,omitempty"`
}

const (
	compName          = "xload"
	MAX_WORKER_COUNT  = 64
	MAX_DATA_SPLITTER = 16
	MAX_LISTER        = 16
)

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

	xl.path = strings.TrimSpace(conf.Path)
	if xl.path == "" {
		xl.path, err = os.Getwd()
		if err != nil {
			log.Err("Xload::Configure : Failed to get current directory [%s]", err.Error())
			return err
		}
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

	return nil
}

// Start : Pipeline calls this method to start the component functionality
func (xl *Xload) Start(ctx context.Context) error {
	log.Trace("Xload::Start : Starting component %s", xl.Name())

	xl.workerCount = MAX_WORKER_COUNT
	xl.blockPool = NewBlockPool(xl.blockSize, xl.workerCount*3)
	if xl.blockPool == nil {
		log.Err("Xload::Start : Failed to create block pool")
		return fmt.Errorf("failed to create block pool")
	}

	// Xload : start code goes here
	switch xl.mode {
	case EMode.CHECKPOINT():
		// Start checkpoint thread here
	case EMode.DOWNLOAD():
		// Start downloader here
	case EMode.UPLOAD():
		// Start uploader here
		err := xl.StartUploader()
		if err != nil {
			log.Err("Xload::Start : Failed to start uploader [%s]", err.Error())
			return err
		}
	case EMode.SYNC():
		//Start syncer here
	default:
		log.Err("Xload::Start : Invalid mode : %s", xl.mode.String())
		return fmt.Errorf("invalid mode in xload : %s", xl.mode.String())
	}

	// Start the data upload download thread pool
	if xl.dataMgrPool != nil {
		xl.dataMgrPool.Start()
	}

	// Start the pool to chunk each file
	if xl.dataSplitPool != nil {
		xl.dataSplitPool.Start()
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (xl *Xload) Stop() error {
	log.Trace("Xload::Stop : Stopping component %s", xl.Name())

	if xl.en.getInputPool() != nil {
		xl.en.getInputPool().Stop()
	}

	// Stop of thread pool shall be in reverse order of start
	if xl.dataSplitPool != nil {
		xl.dataSplitPool.Stop()
	}

	if xl.dataMgrPool != nil {
		xl.dataMgrPool.Stop()
	}

	xl.blockPool.Terminate()

	return nil
}

// StartUploader : Start the uploader thread
func (xl *Xload) StartUploader() error {
	log.Trace("Xload::StartUploader : Starting uploader")

	// Create remote data manager to upload blocks
	dataMgr := RemoteDataManager{
		remote: xl.NextComponent(),
	}

	// Create a thread-pool to run workers which will call the uploader
	xl.dataMgrPool = newThreadPool(MAX_WORKER_COUNT, dataMgr.WriteData)

	// Create a block splitter
	splitter := UploadSplitter{
		blockSize: xl.blockSize,
		blockPool: xl.blockPool,
		commiter:  &dataMgr,
		schedule:  xl.dataMgrPool.Schedule,
		basePath:  xl.path,
	}

	// Create a thread-pool to split file into blocks
	xl.dataSplitPool = newThreadPool(MAX_DATA_SPLITTER, splitter.SplitData)

	// Create lister pool to list local files
	var err error
	xl.en, err = newLocalLister(xl.path, xl.NextComponent())
	if err != nil {
		log.Err("Xload::StartUploader : Unable to create local lister [%s]", err.Error())
		return err
	}

	// start input threadpool
	xl.en.getInputPool().Start()
	xl.en.setOutputPool(xl.dataSplitPool)

	// Kick off the local lister here
	xl.en.getInputPool().Schedule(&workItem{})
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
