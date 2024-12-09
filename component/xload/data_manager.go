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
var _ xcomponent = &dataManager{}
var _ xcomponent = &remoteDataManager{}

const DATA_MANAGER string = "DATA_MANAGER"

type dataManager struct {
	xbase
}

// --------------------------------------------------------------------------------------------------------

type remoteDataManager struct {
	dataManager
}

func newRemoteDataManager(remote internal.Component, statsMgr *statsManager) (*remoteDataManager, error) {
	log.Debug("data_manager::newRemoteDataManager : create new remote data manager")

	rdm := &remoteDataManager{
		dataManager: dataManager{
			xbase: xbase{
				remote:   remote,
				statsMgr: statsMgr,
			},
		},
	}

	rdm.setName(DATA_MANAGER)
	rdm.init()
	return rdm, nil
}

func (rdm *remoteDataManager) init() {
	rdm.pool = newThreadPool(MAX_WORKER_COUNT, rdm.process)
	if rdm.pool == nil {
		log.Err("remoteDataManager::init : fail to init thread pool")
	}
}

func (rdm *remoteDataManager) start() {
	log.Debug("remoteDataManager::start : start remote data manager")
	rdm.getThreadPool().Start()
}

func (rdm *remoteDataManager) stop() {
	log.Debug("remoteDataManager::stop : stop remote data manager")
	if rdm.getThreadPool() != nil {
		rdm.getThreadPool().Stop()
	}
}

// upload or download block
func (rdm *remoteDataManager) process(item *workItem) (int, error) {
	if item.download {
		return rdm.ReadData(item)
	} else {
		return rdm.WriteData(item)
	}
}

// ReadData reads data from the data manager
func (rdm *remoteDataManager) ReadData(item *workItem) (int, error) {
	log.Debug("remoteDataManager::ReadData : Scheduling download for %s offset %v", item.path, item.block.offset)

	h := handlemap.NewHandle(item.path)
	h.Size = int64(item.dataLen)
	n, err := rdm.getRemote().ReadInBuffer(internal.ReadInBufferOptions{
		Handle: h,
		Offset: item.block.offset,
		Data:   item.block.data,
	})

	// send the block download status to stats manager
	rdm.getStatsManager().addStats(&statsItem{
		name:             item.path,
		success:          err == nil,
		download:         true,
		bytesTransferred: uint64(n),
	})

	return n, err
}

// WriteData writes data to the data manager
func (rdm *remoteDataManager) WriteData(item *workItem) (int, error) {
	log.Debug("remoteDataManager::WriteData : Scheduling upload for %s offset %v", item.path, item.block.offset)

	n := int(item.block.length)
	err := rdm.getRemote().StageData(internal.StageDataOptions{
		Name: item.path,
		Data: item.block.data[0:item.block.length],
		// Offset: uint64(item.block.offset),
		Id: item.block.id,
	})
	if err != nil {
		log.Err("remoteDataManager::WriteData : upload failed for %s offset %v [%v]", item.path, item.block.offset, err.Error())
		n = 0
	}

	// send the block upload status to stats manager
	rdm.getStatsManager().addStats(&statsItem{
		name:             item.path,
		success:          err == nil,
		download:         false,
		bytesTransferred: uint64(n),
	})

	return n, err
}
