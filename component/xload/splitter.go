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
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ XComponent = &splitter{}
var _ XComponent = &downloadSplitter{}

type splitter struct {
	XBase
	blockPool   *BlockPool
	path        string
	fileLocks   *common.LockMap
	validateMD5 bool
}

// --------------------------------------------------------------------------------------------------------

type downloadSplitter struct {
	splitter
}

type downloadSplitterOptions struct {
	blockPool   *BlockPool
	path        string
	workerCount uint32
	remote      internal.Component
	statsMgr    *StatsManager
	fileLocks   *common.LockMap
	validateMD5 bool
}

func newDownloadSplitter(opts *downloadSplitterOptions) (*downloadSplitter, error) {
	if opts == nil || opts.blockPool == nil || opts.path == "" || opts.remote == nil || opts.statsMgr == nil || opts.fileLocks == nil || opts.workerCount == 0 {
		log.Err("lister::NewRemoteLister : invalid parameters sent to create download splitter")
		return nil, fmt.Errorf("invalid parameters sent to create download splitter")
	}

	log.Debug("splitter::NewDownloadSplitter : create new download splitter for %s, block size %v, workers %v", opts.path, opts.blockPool.GetBlockSize(), opts.workerCount)

	ds := &downloadSplitter{
		splitter: splitter{
			blockPool:   opts.blockPool,
			path:        opts.path,
			fileLocks:   opts.fileLocks,
			validateMD5: opts.validateMD5,
		},
	}

	ds.SetName(SPLITTER)
	ds.SetWorkerCount(opts.workerCount)
	ds.SetRemote(opts.remote)
	ds.SetStatsManager(opts.statsMgr)
	ds.Init()
	return ds, nil
}

func (ds *downloadSplitter) Init() {
	ds.SetThreadPool(NewThreadPool(ds.GetWorkerCount(), ds.Process))
	if ds.GetThreadPool() == nil {
		log.Err("downloadSplitter::Init : fail to init thread pool")
	}
}

func (ds *downloadSplitter) Start(ctx context.Context) {
	log.Debug("downloadSplitter::Start : start download splitter for %s", ds.path)
	ds.GetThreadPool().Start(ctx)
}

func (ds *downloadSplitter) Stop() {
	log.Debug("downloadSplitter::Stop : stop download splitter for %s", ds.path)
	if ds.GetThreadPool() != nil {
		ds.GetThreadPool().Stop()
	}
	log.Debug("downloadSplitter::Stop : stop successful")
}

// download data in chunks and then write to the local file
func (ds *downloadSplitter) Process(item *WorkItem) (int, error) {
	log.Debug("downloadSplitter::Process : Splitting data for %s, size %v, mode %v, priority %v, access time %v, modified time %v", item.Path, item.DataLen,
		item.Mode, item.Priority, item.Atime.Format(time.DateTime), item.Mtime.Format(time.DateTime))

	var err error
	localPath := filepath.Join(ds.path, item.Path)

	// if priority is false, it means that it has been scheduled by the lister and not by the OpenFile call.
	// So, get a lock. If the locking goes into wait state, it means the file is already under download by the OpenFile thread.
	// Otherwise, if there are no other locks, acquire a lock to prevent any OpenFile call from adding a request again.
	// OpenFile thread already takes a lock on the file in its code, so don't take it again here.
	if !item.Priority {
		flock := ds.fileLocks.Get(item.Path)
		flock.Lock()
		defer flock.Unlock()
	}

	filePresent, isDir, size := isFilePresent(localPath)
	if filePresent {
		if isDir {
			log.Err("downloadSplitter::Process : %s is a directory", item.Path)
			return -1, fmt.Errorf("%s is a directory", item.Path)
		} else if item.DataLen == uint64(size) {
			log.Debug("downloadSplitter::Process : %s will be served from local path, priority %v", item.Path, item.Priority)
			return int(size), nil
		}
	}

	// TODO:: xload : should we delete the file if it already exists
	// TODO:: xload : what should be the flags
	// TODO:: xload : verify if the mode is set correctly
	// TODO:: xload : handle case if blob is a symlink
	item.FileHandle, err = os.OpenFile(localPath, os.O_RDWR|os.O_CREATE, item.Mode)
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to create file %s [%s]", item.Path, err.Error())
		return -1, fmt.Errorf("failed to open file %s [%s]", item.Path, err.Error())
	}

	defer item.FileHandle.Close()

	if item.DataLen == 0 {
		log.Debug("downloadSplitter::Process : 0 byte file %s", item.Path)
		// send the status to stats manager
		ds.GetStatsManager().AddStats(&StatsItem{
			Component: SPLITTER,
			Name:      item.Path,
			Success:   true,
			Download:  true,
		})
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

	numBlocks := ((item.DataLen - 1) / ds.blockPool.GetBlockSize()) + 1
	offset := int64(0)

	wg := sync.WaitGroup{}
	wg.Add(1)

	responseChannel := make(chan *WorkItem, numBlocks)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	operationSuccess := true
	go func() {
		defer wg.Done()

		for i := 0; i < int(numBlocks); i++ {
			select {
			case <-ds.GetThreadPool().ctx.Done(): // check if the thread pool is closed
				operationSuccess = false
				return
			case respSplitItem := <-responseChannel:
				if respSplitItem.Err != nil {
					log.Err("downloadSplitter::Process : Failed to download data for file %s", item.Path)
					operationSuccess = false
					cancel() // cancel the context to stop download of other chunks
				} else {
					_, err := item.FileHandle.WriteAt(respSplitItem.Block.Data[:respSplitItem.DataLen], respSplitItem.Block.Offset)
					if err != nil {
						log.Err("downloadSplitter::Process : Failed to write data to file %s [%s]", item.Path, err.Error())
						operationSuccess = false
						cancel() // cancel the context to stop download of other chunks
					}

					// send the download status to stats manager
					ds.GetStatsManager().AddStats(&StatsItem{
						Component:        SPLITTER,
						Name:             item.Path,
						Success:          false,
						Download:         false,
						DiskIO:           true,
						BytesTransferred: respSplitItem.DataLen,
					})
				}

				if respSplitItem.Block != nil {
					// log.Debug("downloadSplitter::process : Download successful %s index %d offset %v", item.path, respSplitItem.block.index, respSplitItem.block.offset)
					ds.blockPool.Release(respSplitItem.Block)
				}
			}
		}
	}()

	for i := 0; i < int(numBlocks); i++ {
		block := ds.blockPool.GetBlock(item.Priority)
		if block == nil {
			responseChannel <- &WorkItem{Err: fmt.Errorf("failed to get block from pool for file %s, offset %v", item.Path, offset)}
		} else {
			block.Index = i
			block.Offset = offset
			block.Length = int64(ds.blockPool.GetBlockSize())

			splitItem := &WorkItem{
				CompName:        ds.GetNext().GetName(),
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
			err := ds.GetNext().Schedule(splitItem)
			if err != nil {
				log.Err("downloadSplitter::Process : Failed to schedule download for %s [%s]", item.Path, err.Error())
				responseChannel <- &WorkItem{Err: fmt.Errorf("failed to schedule download for %s [%s]", item.Path, err.Error())}
			}
		}

		offset += int64(ds.blockPool.GetBlockSize())
	}

	wg.Wait()

	// update the last modified time
	// TODO:: xload : verify if the lmt is updated correctly
	err = os.Chtimes(localPath, item.Atime, item.Mtime)
	if err != nil {
		log.Err("downloadSplitter::Process : Failed to change times of file %s [%s]", item.Path, err.Error())
	}

	if ds.validateMD5 && operationSuccess {
		err = ds.checkConsistency(item)
		if err != nil {
			// TODO:: xload : retry if md5 validation fails
			log.Err("downloadSplitter::Process : unable to validate md5 for %s [%s]", item.Path, err.Error())
			operationSuccess = false
		}
	}

	// send the download status to stats manager
	ds.GetStatsManager().AddStats(&StatsItem{
		Component: SPLITTER,
		Name:      item.Path,
		Success:   operationSuccess,
		Download:  true,
	})

	if !operationSuccess {
		log.Err("downloadSplitter::Process : Failed to download data for file %s, so deleting it from local path", item.Path)

		// delete the file which failed to download from the local path
		err = os.Remove(localPath)
		if err != nil {
			log.Err("downloadSplitter::Process : Failed to delete file %s [%s]", item.Path, err.Error())
		}

		return -1, fmt.Errorf("failed to download data for file %s", item.Path)
	}

	log.Debug("downloadSplitter::Process : Download completed for file %s, priority %v", item.Path, item.Priority)
	return 0, nil
}

func (ds *downloadSplitter) checkConsistency(item *WorkItem) error {
	if item.MD5 == nil {
		log.Warn("downloadSplitter::checkConsistency : Unable to get MD5Sum for blob %s", item.Path)
	} else {
		// Compute md5 of local file
		fileMD5, err := common.GetMD5(item.FileHandle)
		if err != nil {
			log.Err("downloadSplitter::checkConsistency : Failed to generate MD5Sum for %s [%s]", item.Path, err.Error())
			return err
		}
		// compare md5 and fail is not match
		if !reflect.DeepEqual(fileMD5, item.MD5) {
			log.Err("downloadSplitter::checkConsistency : MD5Sum mismatch on download for file %s", item.Path)
			return fmt.Errorf("md5sum mismatch on download for file %s", item.Path)
		}
	}

	return nil
}
