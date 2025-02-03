/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	xcommon "github.com/Azure/azure-storage-fuse/v2/component/xload/common"
	"github.com/Azure/azure-storage-fuse/v2/component/xload/comp"
	xinternal "github.com/Azure/azure-storage-fuse/v2/component/xload/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Common structure for Component
type Xload struct {
	internal.BaseComponent
	blockSize         uint64                  // Size of each block to be cached
	mode              xcommon.Mode            // Mode of the Xload component
	exportProgress    bool                    // Export the progess of xload operation to json file
	workerCount       uint32                  // Number of workers running
	blockPool         *xcommon.BlockPool      // Pool of blocks
	path              string                  // Path on local disk where Xload will operate
	defaultPermission os.FileMode             // Default permissions of files and directories in the xload path
	comps             []xinternal.XComponent  // list of components in xload
	statsMgr          *xinternal.StatsManager // stats manager
}

// Structure defining your config parameters
type XloadOptions struct {
	BlockSize      float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	Mode           string  `config:"mode" yaml:"mode,omitempty"`
	Path           string  `config:"path" yaml:"path,omitempty"`
	ExportProgress bool    `config:"export-progress" yaml:"path,omitempty"`
	// TODO:: xload : add parallelism parameter
}

const (
	compName         = "xload"
	defaultBlockSize = 16
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

func (xl *Xload) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (xl *Xload) Configure(_ bool) error {
	log.Trace("Xload::Configure : %s", xl.Name())

	// xload component should be used only in readonly mode
	var readonly bool
	err := config.UnmarshalKey("read-only", &readonly)
	if err != nil {
		log.Err("Xload::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
	}

	if !readonly {
		log.Err("Xload::Configure : Xload component should be used only in read-only mode")
		return fmt.Errorf("Xload component should be used in only in read-only mode")
	}

	conf := XloadOptions{}
	err = config.UnmarshalKey(xl.Name(), &conf)
	if err != nil {
		log.Err("Xload::Configure : config error [invalid config attributes]")
		return fmt.Errorf("Xload: config error [invalid config attributes]")
	}

	blockSize := (float64)(defaultBlockSize) // 16 MB as deafult block size
	if config.IsSet(compName + ".block-size-mb") {
		blockSize = conf.BlockSize
	} else if config.IsSet("stream.block-size-mb") {
		err = config.UnmarshalKey("stream.block-size-mb", &blockSize)
		if err != nil {
			log.Err("Xload::Configure : Failed to unmarshal block-size-mb [%s]", err.Error())
		}
	}

	xl.blockSize = uint64(blockSize * float64(xcommon.MB))

	localPath := strings.TrimSpace(conf.Path)
	if localPath == "" {
		if config.IsSet("file_cache.path") {
			err = config.UnmarshalKey("file_cache.path", &localPath)
			if err != nil {
				log.Err("Xload::Configure : Failed to unmarshal tmp-path [%s]", err.Error())
			}
		}
	}

	xl.path = common.ExpandPath(localPath)
	if xl.path == "" {
		// TODO:: xload : should we use current working directory in this case
		log.Err("Xload::Configure : config error [path not given in xload]")
		return fmt.Errorf("config error in %s [path not given]", xl.Name())
	} else {
		//check mnt path is not same as xload path
		mntPath := ""
		err = config.UnmarshalKey("mount-path", &mntPath)
		if err != nil {
			log.Err("Xload::Configure : config error [unable to obtain Mount Path [%s]]", err.Error())
			return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
		}

		if xl.path == mntPath {
			log.Err("Xload::Configure : config error [xload path is same as mount path]")
			return fmt.Errorf("config error in %s error [xload path is same as mount path]", xl.Name())
		}

		_, err = os.Stat(xl.path)
		if os.IsNotExist(err) {
			log.Info("Xload::Configure : config error [xload path does not exist, attempting to create path]")
			err := os.Mkdir(xl.path, os.FileMode(0755))
			if err != nil {
				log.Err("Xload::Configure : config error creating directory of xload path [%s]", err.Error())
				return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
			}
		}

		if !common.IsDirectoryEmpty(xl.path) {
			log.Err("Xload::Configure : config error %s directory is not empty", xl.path)
			return fmt.Errorf("config error in %s [temp directory not empty]", xl.Name())
		}
	}

	var mode xcommon.Mode = xcommon.EMode.PRELOAD() // using preload as the default mode
	if len(conf.Mode) > 0 {
		err = mode.Parse(conf.Mode)
		if err != nil {
			log.Err("Xload::Configure : Failed to parse mode %s [%s]", conf.Mode, err.Error())
			return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
		}

		if mode == xcommon.EMode.INVALID_MODE() {
			log.Err("Xload::Configure : Invalid mode : %s", conf.Mode)
			return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
		}
	}

	xl.mode = mode
	xl.exportProgress = conf.ExportProgress

	allowOther := false
	err = config.UnmarshalKey("allow-other", &allowOther)
	if err != nil {
		log.Err("Xload::Configure : config error [unable to obtain allow-other]")
	}

	if allowOther {
		xl.defaultPermission = common.DefaultAllowOtherPermissionBits
	} else {
		xl.defaultPermission = common.DefaultFilePermissionBits
	}

	log.Debug("Xload::Configure : block size %v, mode %v, path %v, default permission %v", xl.blockSize, xl.mode.String(), xl.path, xl.defaultPermission)

	return nil
}

// Start : Pipeline calls this method to start the component functionality
func (xl *Xload) Start(ctx context.Context) error {
	log.Trace("Xload::Start : Starting component %s", xl.Name())

	xl.workerCount = xcommon.MAX_WORKER_COUNT
	xl.blockPool = xcommon.NewBlockPool(xl.blockSize, xl.workerCount*3)
	if xl.blockPool == nil {
		log.Err("Xload::Start : Failed to create block pool")
		return fmt.Errorf("failed to create block pool")
	}

	var err error

	// create stats manager
	xl.statsMgr, err = xinternal.NewStatsmanager(xcommon.MAX_WORKER_COUNT*2, xl.exportProgress)
	if err != nil {
		log.Err("Xload::Start : Failed to create stats manager [%s]", err.Error())
		return err
	}

	// Xload : start code goes here
	switch xl.mode {
	case xcommon.EMode.PRELOAD():
		// Start downloader here
		err = xl.createDownloader()
		if err != nil {
			log.Err("Xload::Start : Failed to start downloader [%s]", err.Error())
			return err
		}
	case xcommon.EMode.UPLOAD():
		// Start uploader here
		return fmt.Errorf("uploader is currently unsupported")
	case xcommon.EMode.SYNC():
		//Start syncer here
		return fmt.Errorf("sync is currently unsupported")
	default:
		log.Err("Xload::Start : Invalid mode : %s", xl.mode.String())
		return fmt.Errorf("invalid mode in xload : %s", xl.mode.String())
	}

	xl.statsMgr.Start()
	return xl.startComponents()
}

// Stop : Stop the component functionality and kill all threads started
func (xl *Xload) Stop() error {
	log.Trace("Xload::Stop : Stopping component %s", xl.Name())

	xl.comps[0].Stop()
	xl.statsMgr.Stop()
	xl.blockPool.Terminate()

	// TODO:: xload : should we delete the files from local path
	err := common.TempCacheCleanup(xl.path)
	if err != nil {
		log.Err("unable to clean xload local path [%s]", err.Error())
		return err
	}

	return nil
}

func (xl *Xload) createDownloader() error {
	log.Trace("Xload::createDownloader : Starting downloader")

	// Create remote lister pool to list remote files
	rl, err := comp.NewRemoteLister(xl.path, xl.defaultPermission, xl.NextComponent(), xl.statsMgr)
	if err != nil {
		log.Err("Xload::createDownloader : Unable to create remote lister [%s]", err.Error())
		return err
	}

	ds, err := comp.NewDownloadSplitter(xl.blockPool, xl.path, xl.NextComponent(), xl.statsMgr)
	if err != nil {
		log.Err("Xload::createDownloader : Unable to create download splitter [%s]", err.Error())
		return err
	}

	rdm, err := comp.NewRemoteDataManager(xl.NextComponent(), xl.statsMgr)
	if err != nil {
		log.Err("Xload::startUploader : failed to create remote data manager [%s]", err.Error())
		return err
	}

	xl.comps = []xinternal.XComponent{rl, ds, rdm}
	return nil
}

func (xl *Xload) createChain() error {
	if len(xl.comps) == 0 {
		log.Err("Xload::createChain : no component initialized in xload")
		return fmt.Errorf("no component initialized in xload")
	}

	currComp := xl.comps[0]

	for i := 1; i < len(xl.comps); i++ {
		nextComp := xl.comps[i]
		currComp.SetNext(nextComp)
		currComp = nextComp
	}

	return nil
}

func (xl *Xload) startComponents() error {
	err := xl.createChain()
	if err != nil {
		log.Err("Xload::startComponents : failed to create chain [%s]", err.Error())
		return err
	}

	for i := len(xl.comps) - 1; i >= 0; i-- {
		xl.comps[i].Start()
	}

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
