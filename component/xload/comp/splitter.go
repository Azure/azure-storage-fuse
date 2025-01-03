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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	bcommon "github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/xload/common"
	xinternal "github.com/Azure/azure-storage-fuse/v2/component/xload/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xinternal.XComponent = &splitter{}
var _ xinternal.XComponent = &downloadSplitter{}

type splitter struct {
	xinternal.XBase
	blockSize uint64
	blockPool *common.BlockPool
	path      string
	fileLocks *bcommon.LockMap
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

func NewDownloadSplitter(blockSize uint64, blockPool *common.BlockPool, path string, remote internal.Component, statsMgr *xinternal.StatsManager, fileLocks *bcommon.LockMap) (*downloadSplitter, error) {
	log.Debug("splitter::NewDownloadSplitter : create new download splitter for %s, block size %v", path, blockSize)

	d := &downloadSplitter{
		splitter: splitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
			fileLocks: fileLocks,
		},
	}

	d.SetName(common.SPLITTER)
	d.SetRemote(remote)
	d.SetStatsManager(statsMgr)
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

	// if priority is false, it means that it has been scheduled by the lister and not OpenFile call
	// so get a lock and wait if file is already under download by the OpenFile thread
	// OpenFile thread already has a lock on the file, so don't take it again
	if !item.Priority {
		flock := d.fileLocks.Get(item.Path)
		flock.Lock()
		defer flock.Unlock()
	}

	filePresent, size := common.IsFilePresent(localPath)
	if filePresent && item.DataLen == uint64(size) {
		return int(size), nil
	}

	if len(item.Path) == 0 {
		return 0, nil
	}

	numBlocks := ((item.DataLen - 1) / d.blockSize) + 1
	offset := int64(0)

	// TODO:: xload : should we delete the file if it already exists
	// TODO:: xload : what should be the flags and mode and should we allocate the full size to the file
	item.FileHandle, err = os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		// create file
		return -1, fmt.Errorf("failed to open file %s [%v]", item.Path, err)
	}

	defer item.FileHandle.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *common.WorkItem, numBlocks)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.Err != nil {
				log.Err("downloadSplitter::Process : Failed to download data for file %s", item.Path)
				operationSuccess = false
				cancel() // cancel the context to stop download of other chunks
			} else {
				_, err := item.FileHandle.WriteAt(respSplitItem.Block.Data, respSplitItem.Block.Offset)
				if err != nil {
					log.Err("downloadSplitter::Process : Failed to write data to file %s", item.Path)
					operationSuccess = false
					cancel() // cancel the context to stop download of other chunks
				}
			}

			if respSplitItem.Block != nil {
				// log.Debug("downloadSplitter::process : Download successful %s index %d offset %v", item.path, respSplitItem.block.index, respSplitItem.block.offset)
				d.blockPool.Release(respSplitItem.Block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.GetBlock(item.Priority)
		if block == nil {
			responseChannel <- &common.WorkItem{Err: fmt.Errorf("failed to get block from pool for file %s, offset %v", item.Path, offset)}
		} else {
			block.Index = i
			block.Offset = offset
			block.Length = int64(d.blockSize)

			splitItem := &common.WorkItem{
				CompName:        d.GetNext().GetName(),
				Path:            item.Path,
				DataLen:         item.DataLen,
				FileHandle:      item.FileHandle,
				Block:           block,
				ResponseChannel: responseChannel,
				Download:        true,
				Priority:        item.Priority,
				Ctx:             ctx,
			}
			// log.Debug("downloadSplitter::Process : Scheduling download for %s offset %v", item.Path, offset)
			d.GetNext().GetThreadPool().Schedule(splitItem)
		}

		offset += int64(d.blockSize)
	}

	wg.Wait()
	err = item.FileHandle.Truncate(int64(item.DataLen))
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to truncate file %s [%s]", item.Path, err.Error())
		operationSuccess = false
	}

	// send the download status to stats manager
	d.GetStatsManager().AddStats(&xinternal.StatsItem{
		Component: common.SPLITTER,
		Name:      item.Path,
		Success:   operationSuccess,
		Download:  true,
	})

	if !operationSuccess {
		log.Err("downloadSplitter::Process : Failed to download data for file %s", item.Path)
		log.Debug("downloadSplitter::Process : deleting file %s", item.Path)

		// delete the file which failed to download from the local path
		err = os.Remove(filepath.Join(d.path, item.Path))
		if err != nil {
			log.Err("downloadSplitter::Process : Unable to delete file %s [%s]", item.Path, err.Error())
		}

		return -1, fmt.Errorf("failed to download data for file %s", item.Path)
	}

	log.Debug("downloadSplitter::Process : Download completed for file %s", item.Path)
	return 0, nil
}
