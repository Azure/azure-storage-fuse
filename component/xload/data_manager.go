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
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// verify that the below types implement the xcomponent interfaces
var _ XComponent = &dataManager{}
var _ XComponent = &remoteDataManager{}

const DATA_MANAGER string = "DATA_MANAGER"

type dataManager struct {
	XBase
}

// --------------------------------------------------------------------------------------------------------

type remoteDataManager struct {
	dataManager
}

func NewRemoteDataManager(remote internal.Component) (*remoteDataManager, error) {
	log.Debug("data_manager::NewRemoteDataManager : create new remote data manager")

	rdm := &remoteDataManager{
		dataManager: dataManager{
			XBase: XBase{
				remote: remote,
			},
		},
	}

	rdm.SetName(DATA_MANAGER)
	rdm.Init()
	return rdm, nil
}

func (rdm *remoteDataManager) Init() {
	rdm.pool = NewThreadPool(MAX_WORKER_COUNT, rdm.Process)
	if rdm.pool == nil {
		log.Err("remoteDataManager::Init : fail to init thread pool")
	}
}

func (rdm *remoteDataManager) Start() {
	log.Debug("remoteDataManager::Start : start remote data manager")
	rdm.GetThreadPool().Start()
}

func (rdm *remoteDataManager) Stop() {
	log.Debug("remoteDataManager::Stop : stop remote data manager")
	if rdm.GetThreadPool() != nil {
		rdm.GetThreadPool().Stop()
	}
}

// upload or download block
func (rdm *remoteDataManager) Process(item *WorkItem) (int, error) {
	if item.Download {
		return rdm.ReadData(item)
	} else {
		return rdm.WriteData(item)
	}
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *WorkItem) (int, error) {
	log.Debug("remoteDataManager::ReadData : Scheduling download for %s offset %v", item.Path, item.Block.offset)

	h := handlemap.NewHandle(item.Path)
	h.Size = int64(item.DataLen)
	return rdm.GetRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: h,
		Offset: item.Block.offset,
		Data:   item.Block.data,
	})
}

// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *WorkItem) (int, error) {
	log.Debug("remoteDataManager::WriteData : Scheduling upload for %s offset %v", item.Path, item.Block.offset)

	return int(item.Block.length), rdm.GetRemote().StageData(internal.StageDataOptions{
		Name: item.Path,
		Data: item.Block.data[0:item.Block.length],
		// Offset: uint64(item.block.offset),
		Id: item.Block.id})
}
