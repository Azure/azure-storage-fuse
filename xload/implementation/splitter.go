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

package implementation

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/xload/contract"
	"github.com/Azure/azure-storage-fuse/v2/xload/core"
)

// verify that the below types implement the contract.Xcomponent interfaces
var _ contract.Xcomponent = &splitter{}
var _ contract.Xcomponent = &downloadSplitter{}

const SPLITTER string = "splitter"

type splitter struct {
	contract.Xbase
	blockSize uint64
	blockPool *core.BlockPool
	path      string
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

func NewDownloadSplitter(blockSize uint64, blockPool *core.BlockPool, path string, remote internal.Component) (*downloadSplitter, error) {
	log.Debug("splitter::newDownloadSplitter : create new download splitter for %s, block size %v", path, blockSize)

	d := &downloadSplitter{
		splitter: splitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
		},
	}
	d.SetRemote(remote)
	d.SetName(SPLITTER)
	d.init()
	return d, nil
}

func (d *downloadSplitter) init() {
	d.SetThreadPool(core.NewThreadPool(MAX_DATA_SPLITTER, d.process))
	if d.GetThreadPool() == nil {
		log.Err("downloadSplitter::init : fail to init thread pool")
	}
}

func (d *downloadSplitter) start() {
	log.Debug("downloadSplitter::start : start download splitter for %s", d.path)
	d.GetThreadPool().Start()
}

func (d *downloadSplitter) stop() {
	log.Debug("downloadSplitter::stop : stop download splitter for %s", d.path)
	if d.GetThreadPool() != nil {
		d.GetThreadPool().Stop()
	}
	d.GetNext().Stop()
}

// download data in chunks and then write to the local file
func (d *downloadSplitter) process(item *core.WorkItem) (int, error) {
	var err error

	log.Debug("downloadSplitter::process : Splitting data for %s", item.Path)
	if len(item.Path) == 0 {
		return 0, nil
	}

	numBlocks := ((item.DataLen - 1) / d.blockSize) + 1
	offset := int64(0)

	// TODO:: xload : should we delete the file if it already exists
	// TODO:: xload : what should be the flags and mode and should we allocate the full size to the file
	item.FileHandle, err = os.OpenFile(filepath.Join(d.path, item.Path), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		// create file
		return -1, fmt.Errorf("failed to open file %s [%v]", item.Path, err)
	}

	defer item.FileHandle.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *core.WorkItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.Err != nil {
				log.Err("downloadSplitter::process : Failed to download data for file %s", item.Path)
				operationSuccess = false
			} else {
				_, err := item.FileHandle.WriteAt(respSplitItem.Block.Data, respSplitItem.Block.Offset)
				if err != nil {
					log.Err("downloadSplitter::process : Failed to write data to file %s", item.Path)
					operationSuccess = false
				}
			}

			if respSplitItem.Block != nil {
				log.Debug("downloadSplitter::process : Download successful %s index %d offset %v", item.Path, respSplitItem.Block.Index, respSplitItem.Block.Offset)
				d.blockPool.Release(respSplitItem.Block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.Get()
		if block == nil {
			responseChannel <- &core.WorkItem{Err: fmt.Errorf("failed to get block from pool for file %s, offset %v", item.Path, offset)}
		} else {
			block.Index = i
			block.Offset = offset
			block.Length = int64(d.blockSize)

			splitItem := &core.WorkItem{
				CompName:        d.GetNext().GetName(),
				Path:            item.Path,
				DataLen:         item.DataLen,
				FileHandle:      item.FileHandle,
				Block:           block,
				ResponseChannel: responseChannel,
				Download:        true,
			}
			log.Debug("downloadSplitter::process : Scheduling download for %s offset %v", item.Path, offset)
			d.GetNext().GetThreadPool().Schedule(splitItem)
		}

		offset += int64(d.blockSize)
	}

	wg.Wait()
	err = item.FileHandle.Truncate(int64(item.DataLen))
	if err != nil {
		log.Err("downloadSplitter::process : Failed to truncate file %s [%s]", item.Path, err.Error())
		operationSuccess = false
	}

	if !operationSuccess {
		log.Err("downloadSplitter::process : Failed to download data for file %s", item.Path)
		log.Debug("downloadSplitter::process : deleting file %s", item.Path)

		// delete the file which failed to download from the local path
		err = os.Remove(filepath.Join(d.path, item.Path))
		if err != nil {
			log.Err("downloadSplitter::process : Unable to delete file %s [%s]", item.Path, err.Error())
		}

		return -1, fmt.Errorf("failed to download data for file %s", item.Path)
	}

	log.Debug("downloadSplitter::process : Download completed for file %s", item.Path)
	return 0, nil
}
