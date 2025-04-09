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

package dcachelib

import (
	"encoding/json"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	. "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/api"
)

type ClusterManagerImpl struct {
	StorageCallback StorageCallbacks
}

func (cmi *ClusterManagerImpl) Start() error {
	return nil
}

func (cmi *ClusterManagerImpl) Stop() error {
	return nil
}

func NewClusterManager(callback StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{}
}

func (cmi *ClusterManagerImpl) CreateClusterConfig() error {
	rv0 := RawVolume{
		Name:             "rv0",
		HostNode:         "node-uuid",
		FSID:             "filesystem-guid",
		FDID:             "fault-domain-id",
		State:            "online",
		TotalSpaceGB:     1000,
		AvailableSpaceGB: 3415,
	}

	clusterConfig := ClusterConfig{
		Readonly:      false,
		State:         StateReady,
		Epoch:         123,
		CreatedAt:     1741008135,
		LastUpdatedAt: 1741190809,
		LastUpdatedBy: "<GUID>",
		// Config:        dcacheConfig,
		RVList: []RawVolume{
			rv0,
		},
		MVList: []MirroredVolume{
			{
				Name:           "mv0",
				RVmapWithState: []VolumeState{{Volume: rv0, State: "online"}},
			},
		},
	}
	log.Trace("ClusterManager::CreateClusterConfig : ClusterConfig: %v", clusterConfig)
	clusterConfigJson, err := json.Marshal(clusterConfig)
	log.Err("ClusterManager::CreateClusterConfig : ClusterConfigJson: %v", err)
	return cmi.StorageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{Name: "ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
}

func (cmi *ClusterManagerImpl) GetActiveMVs() []MirroredVolume {
	return nil
}

func (cmi *ClusterManagerImpl) IsAlive(peerId string) bool {
	return false
}

func (cmi *ClusterManagerImpl) UpdateMVs(mvs []MirroredVolume) {
}

func (cmi *ClusterManagerImpl) UpdateStroageConfigIfRequired() error {
	checkForRVs()
	evaluateMVsRVMapping()
	//Mark the Mv degraded
	return nil
}

func (cmi *ClusterManagerImpl) WatchForConfigChanges() error {

	// Update your local cluster config in the Path
	return nil
}

func checkForRVs() {
}

func evaluateMVsRVMapping() {}

func IsAlive(peerId string) bool {
	return false
}

func GetActiveMVs() []MirroredVolume {
	return nil
}
