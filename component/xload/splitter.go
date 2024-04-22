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
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Interface to read and write data
type splitter interface {
	SplitData(item *workItem) (int, error)
}

// -----------------------------------------------------------------------------------

type UploadSplitter struct {
	blockSize uint64
	blockPool *BlockPool
	commiter  dataCommitter
	schedule  func(item *workItem)
	basePath  string
}

// SplitData reads data from the data manager
func (u *UploadSplitter) SplitData(item *workItem) (int, error) {
	var err error
	var ids []string

	numBlocks := ((item.dataLen - 1) / u.blockSize) + 1
	offset := int64(0)

	item.fileHandle, err = os.OpenFile(filepath.Join(u.basePath, item.path), os.O_RDONLY, 0644)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *workItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.err != nil {
				log.Err("UploadSplitter::SplitData : Failed to read data from file %s", item.path)
				operationSuccess = false
			}
			if respSplitItem.block != nil {
				log.Trace("UploadSplitter::SplitData : [%d] Upload successful for %s block[%d] %s offset %v", i, item.path, respSplitItem.block.index, respSplitItem.block.id, respSplitItem.block.offset)
				u.blockPool.Release(respSplitItem.block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := u.blockPool.Get()
		if block == nil {
			responseChannel <- &workItem{err: fmt.Errorf("failed to get block from pool for file %s %v", item.path, offset)}
		} else {
			dataLen, err := item.fileHandle.ReadAt(block.data, offset)
			if err != nil && err != io.EOF {
				responseChannel <- &workItem{err: fmt.Errorf("failed to read block from file %s %v", item.path, offset)}
			} else {
				block.index = i
				block.offset = offset
				block.length = int64(dataLen)
				block.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))

				splitItem := &workItem{
					path:            item.path,
					fileHandle:      nil,
					block:           block,
					responseChannel: responseChannel,
				}
				ids = append(ids, splitItem.block.id)
				log.Trace("UploadSplitter::SplitData : Scheduling %s block [%d] %s offset %v length %v", item.path, splitItem.block.index, splitItem.block.id, offset, splitItem.block.length)
				u.schedule(splitItem)
			}
		}

		offset += int64(u.blockSize)
	}

	wg.Wait()
	item.fileHandle.Close()

	if !operationSuccess {
		log.Err("UploadSplitter::SplitData : Failed to upload data from file %s", item.path)
	} else {
		u.commiter.CommitData(item.path, ids)
	}

	return 0, nil
}

// -----------------------------------------------------------------------------------

type DownloadSplitter struct {
	blockSize uint64
	blockPool *BlockPool
	commiter  dataCommitter
	schedule  func(item *workItem)
	basePath  string
}

// SplitData reads data from the data manager
func (d *DownloadSplitter) SplitData(item *workItem) (int, error) {
	var err error

	numBlocks := ((item.dataLen - 1) / d.blockSize) + 1
	offset := int64(0)

	item.fileHandle, err = os.OpenFile(filepath.Join(d.basePath, item.path), os.O_WRONLY, 0644)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *workItem, numBlocks)

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			respSplitItem := <-responseChannel
			if respSplitItem.err != nil {
				log.Err("DownloadSplitter::SplitData : Failed to read data from file %s", item.path)
				operationSuccess = false
			}

			_, err := item.fileHandle.WriteAt(respSplitItem.block.data, respSplitItem.block.offset)
			if err != nil {
				log.Err("DownloadSplitter::SplitData : Failed to write data to file %s", item.path)
				operationSuccess = false
			}

			if respSplitItem.block != nil {
				log.Trace("DownloadSplitter::SplitData : Download successful %s index %d offset %v", item.path, respSplitItem.block.index, respSplitItem.block.offset)
				d.blockPool.Release(respSplitItem.block)
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := d.blockPool.Get()
		if block == nil {
			responseChannel <- &workItem{err: fmt.Errorf("failed to get block from pool for file %s %v", item.path, offset)}
		} else {
			block.index = i
			block.offset = offset
			block.length = int64(d.blockSize)

			splitItem := &workItem{
				path:            item.path,
				fileHandle:      item.fileHandle,
				block:           block,
				responseChannel: responseChannel,
			}
			log.Trace("DownloadSplitter::SplitData : Scheduling %s offset %v", item.path, offset)
			d.schedule(splitItem)
		}

		offset += int64(d.blockSize)
	}

	wg.Wait()
	item.fileHandle.Close()

	if !operationSuccess {
		log.Err("UploadSplitter::SplitData : Failed to upload data from file %s", item.path)
	}

	return 0, nil
}
