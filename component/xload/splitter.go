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
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ xcomponent = &splitter{}
var _ xcomponent = &downloadSplitter{}

const SPLITTER string = "splitter"

type splitter struct {
	xbase
	blockSize uint64
	blockPool *BlockPool
	path      string
	fileLocks *common.LockMap
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

func newDownloadSplitter(blockSize uint64, blockPool *BlockPool, path string, remote internal.Component, statsMgr *statsManager, fileLocks *common.LockMap) (*downloadSplitter, error) {
	log.Debug("splitter::newDownloadSplitter : create new download splitter for %s, block size %v", path, blockSize)

	d := &downloadSplitter{
		splitter: splitter{
			blockSize: blockSize,
			blockPool: blockPool,
			path:      path,
			fileLocks: fileLocks,
			xbase: xbase{
				remote:   remote,
				statsMgr: statsMgr,
			},
		},
	}

	d.setName(SPLITTER)
	d.init()
	return d, nil
}

func (d *downloadSplitter) init() {
	d.pool = newThreadPool(MAX_DATA_SPLITTER, d.process)
	if d.pool == nil {
		log.Err("downloadSplitter::init : fail to init thread pool")
	}
}

func (d *downloadSplitter) start() {
	log.Debug("downloadSplitter::start : start download splitter for %s", d.path)
	d.getThreadPool().Start()
}

func (d *downloadSplitter) stop() {
	log.Debug("downloadSplitter::stop : stop download splitter for %s", d.path)
	if d.getThreadPool() != nil {
		d.getThreadPool().Stop()
	}
	d.getNext().stop()
}

// download data in chunks and then write to the local file
func (d *downloadSplitter) process(item *workItem) (int, error) {
	log.Debug("downloadSplitter::process : Splitting data for %s", item.path)

	var err error
	localPath := filepath.Join(d.path, item.path)

	// if priority is false, it means that it has been scheduled by the lister and not OpenFile call
	// so get a lock and wait if file is already under download by the OpenFile thread
	// OpenFile thread already has a lock on the file, so don't take it again
	if !item.priority {
		flock := d.fileLocks.Get(item.path)
		flock.Lock()
		defer flock.Unlock()
	}

	filePresent, size := isFilePresent(localPath)
	if filePresent && item.dataLen == uint64(size) {
		return int(size), nil
	}

	if len(item.path) == 0 {
		return 0, nil
	}

	numBlocks := ((item.dataLen - 1) / d.blockSize) + 1
	offset := int64(0)

	// TODO:: xload : should we delete the file if it already exists
	// TODO:: xload : what should be the flags and mode and should we allocate the full size to the file
	item.fileHandle, err = os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		// create file
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	defer item.fileHandle.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *workItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.err != nil {
				log.Err("downloadSplitter::process : Failed to download data for file %s", item.path)
				operationSuccess = false
			} else {
				_, err := item.fileHandle.WriteAt(respSplitItem.block.data, respSplitItem.block.offset)
				if err != nil {
					log.Err("downloadSplitter::process : Failed to write data to file %s", item.path)
					operationSuccess = false
				}
			}

			if respSplitItem.block != nil {
				// log.Debug("downloadSplitter::process : Download successful %s index %d offset %v", item.path, respSplitItem.block.index, respSplitItem.block.offset)
				d.blockPool.Release(respSplitItem.block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.GetBlock(item.priority)
		if block == nil {
			responseChannel <- &workItem{err: fmt.Errorf("failed to get block from pool for file %s, offset %v", item.path, offset)}
		} else {
			block.index = i
			block.offset = offset
			block.length = int64(d.blockSize)

			splitItem := &workItem{
				compName:        d.getNext().getName(),
				path:            item.path,
				dataLen:         item.dataLen,
				fileHandle:      item.fileHandle,
				block:           block,
				responseChannel: responseChannel,
				download:        true,
			}
			// log.Debug("downloadSplitter::process : Scheduling download for %s offset %v", item.path, offset)
			d.getNext().getThreadPool().Schedule(item.priority, splitItem)
		}

		offset += int64(d.blockSize)
	}

	wg.Wait()
	err = item.fileHandle.Truncate(int64(item.dataLen))
	if err != nil {
		log.Err("downloadSplitter::process : Failed to truncate file %s [%s]", item.path, err.Error())
		operationSuccess = false
	}

	// send the download status to stats manager
	d.getStatsManager().addStats(&statsItem{
		component: SPLITTER,
		name:      item.path,
		success:   operationSuccess,
		download:  true,
	})

	if !operationSuccess {
		log.Err("downloadSplitter::process : Failed to download data for file %s", item.path)
		log.Debug("downloadSplitter::process : deleting file %s", item.path)

		// delete the file which failed to download from the local path
		err = os.Remove(filepath.Join(d.path, item.path))
		if err != nil {
			log.Err("downloadSplitter::process : Unable to delete file %s [%s]", item.path, err.Error())
		}

		return -1, fmt.Errorf("failed to download data for file %s", item.path)
	}

	log.Debug("downloadSplitter::process : Download completed for file %s", item.path)
	return 0, nil
}
