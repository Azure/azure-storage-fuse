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
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/xload/contract"
	"github.com/Azure/azure-storage-fuse/v2/xload/core"
)

// verify that the below types implement the contaract.Xcomponent interfaces
var _ contract.Xcomponent = &dataManager{}
var _ contract.Xcomponent = &remoteDataManager{}

const (
	compName                 = "xload"
	MAX_WORKER_COUNT         = 64
	MAX_DATA_SPLITTER        = 16
	MAX_LISTER               = 16
	defaultBlockSize         = 16
	DATA_MANAGER      string = "DATA_MANAGER"
)

type dataManager struct {
	contract.Xbase
}

// --------------------------------------------------------------------------------------------------------

type remoteDataManager struct {
	dataManager
}

func NewRemoteDataManager(remote internal.Component) (*remoteDataManager, error) {
	log.Debug("data_manager::newRemoteDataManager : create new remote data manager")

	remoteDM := &remoteDataManager{
		dataManager: dataManager{

			Xbase: contract.Xbase{},
		},
	}

	remoteDM.SetRemote(remote)
	remoteDM.SetName(DATA_MANAGER)
	remoteDM.init()
	return remoteDM, nil
}

func (rdm *remoteDataManager) init() {
	rdm.SetThreadPool(core.NewThreadPool(MAX_WORKER_COUNT, rdm.process))
	if rdm.GetThreadPool() == nil {
		log.Err("remoteDataManager::init : fail to init thread pool")
	}
}

func (rdm *remoteDataManager) start() {
	log.Debug("remoteDataManager::start : start remote data manager")
	rdm.GetThreadPool().Start()
}

func (rdm *remoteDataManager) stop() {
	log.Debug("remoteDataManager::stop : stop remote data manager")
	if rdm.GetThreadPool() != nil {
		rdm.GetThreadPool().Stop()
	}
}

// upload or download block
func (rdm *remoteDataManager) process(item *core.WorkItem) (int, error) {
	if item.IsDownloading() {
		return rdm.ReadData(item)
	} else {
		return rdm.WriteData(item)
	}
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *core.WorkItem) (int, error) {
	log.Debug("remoteDataManager::ReadData : Scheduling dow`nload for %s offset %v", item.Path, item.Block.Offset)

	h := handlemap.NewHandle(item.Path)
	h.Size = int64(item.DataLen)
	return rdm.GetRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: h,
		Offset: item.Block.Offset,
		Data:   item.Block.Data,
	})
}

// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *core.WorkItem) (int, error) {
	log.Debug("remoteDataManager::WriteData : Scheduling upload for %s offset %v", item.Path, item.Block.Offset)

	return int(item.Block.Length), rdm.GetRemote().StageData(internal.StageDataOptions{
		Name: item.Path,
		Data: item.Block.Data[0:item.Block.Length],
		// Offset: uint64(item.block.offset),
		Id: item.Block.Id})
}
