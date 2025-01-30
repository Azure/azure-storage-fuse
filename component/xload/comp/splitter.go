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

package comp

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/xload/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ common.XComponent = &splitter{}
var _ common.XComponent = &downloadSplitter{}

const SPLITTER string = "splitter"

type splitter struct {
	common.XBase
	blockPool *common.BlockPool
	path      string
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

func NewDownloadSplitter(blockPool *common.BlockPool, path string, remote internal.Component) (*downloadSplitter, error) {
	log.Debug("splitter::NewDownloadSplitter : create new download splitter for %s, block size %v", path, blockPool.GetBlockSize())

	d := &downloadSplitter{
		splitter: splitter{
			blockPool: blockPool,
			path:      path,
		},
	}

	d.SetName(SPLITTER)
	d.SetRemote(remote)
	d.Init()
	return d, nil
}

func (d *downloadSplitter) Init() {
	d.SetThreadPool(common.NewThreadPool(common.MAX_DATA_SPLITTER, d.Process))
	if d.GetThreadPool() == nil {
		log.Err("downloadSplitter::Init : fail to init thread pool")
	}
}

func (d *downloadSplitter) Start() {
	log.Debug("downloadSplitter::Start : start download splitter for %s", d.path)
	d.GetThreadPool().Start()
}

func (d *downloadSplitter) Stop() {
	log.Debug("downloadSplitter::Stop : stop download splitter for %s", d.path)
	if d.GetThreadPool() != nil {
		d.GetThreadPool().Stop()
	}
	d.GetNext().Stop()
}

// download data in chunks and then write to the local file
func (d *downloadSplitter) Process(item *common.WorkItem) (int, error) {
	log.Debug("downloadSplitter::Process : Splitting data for %s", item.Path)
	var err error
	localPath := filepath.Join(d.path, item.Path)

	if len(item.Path) == 0 {
		return 0, nil
	}

	// TODO:: xload : should we delete the file if it already exists
	// TODO:: xload : what should be the flags and mode and should we allocate the full size to the file
	// TODO:: xload : handle case if blob is a symlink
	item.FileHandle, err = os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE, item.Mode)
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to create file %s [%s]", item.Path, err.Error())
		return -1, fmt.Errorf("failed to open file %s [%s]", item.Path, err.Error())
	}

	defer item.FileHandle.Close()

	if item.DataLen == 0 {
		log.Debug("downloadSplitter::Process : 0 byte file %s", item.Path)
		return 0, nil
	}

	// truncate the file to its size
	err = item.FileHandle.Truncate(int64(item.DataLen))
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to truncate file %s, so deleting it from local path [%s]", item.Path, err.Error())

		// delete the file which failed to truncate from the local path
		err1 := os.Remove(localPath)
		if err1 != nil {
			log.Err("downloadSplitter::Process : Failed to delete file %s [%s]", item.Path, err1.Error())
		}

		return -1, fmt.Errorf("failed to truncate file %s [%s]", item.Path, err.Error())
	}

	numBlocks := ((item.DataLen - 1) / d.blockPool.GetBlockSize()) + 1
	offset := int64(0)

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *common.WorkItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.Err != nil {
				log.Err("downloadSplitter::Process : Failed to download data for file %s", item.Path)
				operationSuccess = false
			} else {
				_, err := item.FileHandle.WriteAt(respSplitItem.Block.Data[:respSplitItem.DataLen], respSplitItem.Block.Offset)
				if err != nil {
					log.Err("downloadSplitter::Process : Failed to write data to file %s", item.Path)
					operationSuccess = false
				}
			}

			if respSplitItem.Block != nil {
				log.Debug("downloadSplitter::Process : Download successful %s index %d offset %v", item.Path, respSplitItem.Block.Index, respSplitItem.Block.Offset)
				d.blockPool.Release(respSplitItem.Block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.Get()
		if block == nil {
			responseChannel <- &common.WorkItem{Err: fmt.Errorf("failed to get block from pool for file %s, offset %v", item.Path, offset)}
		} else {
			block.Index = i
			block.Offset = offset
			block.Length = int64(d.blockPool.GetBlockSize())

			splitItem := &common.WorkItem{
				CompName:        d.GetNext().GetName(),
				Path:            item.Path,
				DataLen:         item.DataLen,
				FileHandle:      item.FileHandle,
				Block:           block,
				ResponseChannel: responseChannel,
				Download:        true,
			}
			log.Debug("downloadSplitter::Process : Scheduling download for %s offset %v", item.Path, offset)
			d.GetNext().Schedule(splitItem)
		}

		offset += int64(d.blockPool.GetBlockSize())
	}

	wg.Wait()

	// update the last modified time
	err = os.Chtimes(localPath, item.Atime, item.Mtime)
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to change times of file %s [%s]", item.Path, err.Error())
		operationSuccess = false
	}

	if !operationSuccess {
		log.Err("downloadSplitter::Process : Failed to download data for file %s, so deleting it from local path", item.Path)

		// delete the file which failed to download from the local path
		err = os.Remove(localPath)
		if err != nil {
			log.Err("downloadSplitter::Process : Failed to delete file %s [%s]", item.Path, err.Error())
		}

		return -1, fmt.Errorf("failed to download data for file %s", item.Path)
	}

	log.Debug("downloadSplitter::Process : Download completed for file %s", item.Path)
	return 0, nil
}
