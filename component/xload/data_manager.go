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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// verify that the below types implement the xcomponent interfaces
var _ XComponent = &dataManager{}
var _ XComponent = &remoteDataManager{}

type dataManager struct {
	XBase
}

// --------------------------------------------------------------------------------------------------------

type remoteDataManager struct {
	dataManager
}

type remoteDataManagerOptions struct {
	workerCount uint32
	remote      internal.Component
	statsMgr    *StatsManager
}

func newRemoteDataManager(opts *remoteDataManagerOptions) (*remoteDataManager, error) {
	if opts == nil || opts.remote == nil || opts.statsMgr == nil || opts.workerCount == 0 {
		log.Err("data_manager::NewRemoteDataManager : invalid parameters sent to create remote data manager")
		return nil, fmt.Errorf("invalid parameters sent to create remote data manager")
	}

	log.Debug("data_manager::NewRemoteDataManager : create new remote data manager, workers %v", opts.workerCount)

	rdm := &remoteDataManager{}

	rdm.SetName(DATA_MANAGER)
	rdm.SetWorkerCount(opts.workerCount)
	rdm.SetRemote(opts.remote)
	rdm.SetStatsManager(opts.statsMgr)
	rdm.Init()
	return rdm, nil
}

func (rdm *remoteDataManager) Init() {
	rdm.SetThreadPool(NewThreadPool(rdm.GetWorkerCount(), rdm.Process))
	if rdm.GetThreadPool() == nil {
		log.Err("remoteDataManager::Init : fail to init thread pool")
	}
}

func (rdm *remoteDataManager) Start(ctx context.Context) {
	log.Debug("remoteDataManager::Start : start remote data manager")
	rdm.GetThreadPool().Start(ctx)
}

func (rdm *remoteDataManager) Stop() {
	log.Debug("remoteDataManager::Stop : stop remote data manager")
	if rdm.GetThreadPool() != nil {
		rdm.GetThreadPool().Stop()
	}
	log.Debug("remoteDataManager::Stop : stop successful")
}

// upload or download block
func (rdm *remoteDataManager) Process(item *WorkItem) (int, error) {
	select {
	case <-item.Ctx.Done(): // listen for cancellation signal
		log.Err("remoteDataManager::Process : Cancelling download for offset %v of %v", item.Block.Offset, item.Path)
		return 0, fmt.Errorf("cancelling download for offset %v of %v", item.Block.Offset, item.Path)

	default:
		if item.Download {
			return rdm.ReadData(item)
		} else {
			// return rdm.WriteData(item)
			return 0, fmt.Errorf("uploads are currently not supported, path %v", item.Path)
		}
	}
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *WorkItem) (int, error) {
	// log.Debug("remoteDataManager::ReadData : Scheduling download for %s offset %v", item.Path, item.Block.Offset)

	bytesTransferred, err := rdm.GetRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Offset: item.Block.Offset,
		Data:   item.Block.Data,
		Path:   item.Path,
		Size:   (int64)(item.DataLen),
	})

	// send the block download status to stats manager
	rdm.sendStats(item.Path, true, uint64(bytesTransferred), err == nil)

	return bytesTransferred, err
}

// uncomment this when the support for upload is added
/*
// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *WorkItem) (int, error) {
	// log.Debug("remoteDataManager::WriteData : Scheduling upload for %s offset %v", item.path, item.block.offset)

	bytesTransferred := int(item.Block.Length)
	err := rdm.GetRemote().StageData(internal.StageDataOptions{
		Name: item.Path,
		Data: item.Block.Data[0:item.Block.Length],
		// Offset: uint64(item.block.offset),
		Id: item.Block.Id,
	})
	if err != nil {
		log.Err("remoteDataManager::WriteData : upload failed for %s offset %v [%v]", item.Path, item.Block.Offset, err.Error())
		bytesTransferred = 0
	}

	// send the block upload status to stats manager
	rdm.sendStats(item.Path, false, uint64(bytesTransferred), err == nil)

	return bytesTransferred, err
}
*/

// send stats to stats manager
func (rdm *remoteDataManager) sendStats(path string, isDownload bool, bytesTransferred uint64, isSuccess bool) {
	rdm.GetStatsManager().AddStats(&StatsItem{
		Component:        DATA_MANAGER,
		Name:             path,
		Success:          isSuccess,
		Download:         isDownload,
		BytesTransferred: bytesTransferred,
	})
}
