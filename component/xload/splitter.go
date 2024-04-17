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
	"os"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Interface to read and write data
type splitter interface {
	SplitData(item *workItem) (int, error)
}

// -----------------------------------------------------------------------------------

// LocalDataManager is a data manager for local data
type UploadSplitter struct {
	blockSize uint64
	blockPool *BlockPool
	commiter  dataCommitter
	schedule  func(item *workItem)
}

// SplitData reads data from the data manager
func (u *UploadSplitter) SplitData(item *workItem) (int, error) {
	var err error
	var ids []string

	numBlocks := ((item.length - 1) / u.blockSize) + 1
	offset := uint64(0)

	item.fileHandle, err = os.Open(item.path)
	if err != nil {
		return -1, fmt.Errorf("failed to open file %s [%v]", item.path, err)
	}

	responseChannel := make(chan workItemResp, numBlocks)
	finalStatus := true

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			splitItem := <-responseChannel

			if splitItem.err != nil {
				log.Err("UploadSplitter::SplitData : Failed to read data from file %s", item.path)
				finalStatus = false
			}

			u.blockPool.Release(splitItem.block)
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := u.blockPool.Get()
		if block == nil {
			return -1, fmt.Errorf("failed to get block from pool for file %s", item.path)
		}

		dataLen, err := item.fileHandle.ReadAt(block.data, int64(offset))
		if err != nil {
			return -1, fmt.Errorf("failed to read data from file %s [%v]", item.path, err)
		}

		splitItem := &workItem{
			path:            item.path,
			offset:          offset,
			length:          uint64(dataLen),
			fileHandle:      nil,
			id:              base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16)),
			block:           block,
			responseChannel: responseChannel,
		}

		offset += u.blockSize
		ids = append(ids, splitItem.id)
		u.schedule(splitItem)
	}

	wg.Wait()
	close(responseChannel)

	item.fileHandle.Close()

	if finalStatus != true {
		log.Err("UploadSplitter::SplitData : Failed to upload data from file %s", item.path)
	} else {
		u.commiter.CommitData(item.path, ids)
	}

	return 0, nil
}
