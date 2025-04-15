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

package clustermanager

import (
	"encoding/json"
	"time"

	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type ClusterManagerImpl struct {
	storageCallback dcachelib.StorageCallbacks
}

// GetPeer implements ClusterManager.
func (cmi *ClusterManagerImpl) GetPeer(nodeId string) dcachelib.Peer {
	return dcachelib.Peer{}
}

// GetPeerRVs implements ClusterManager.
func (cmi *ClusterManagerImpl) GetPeerRVs(mvName string) []dcachelib.RawVolume {
	return nil
}

func (cmi *ClusterManagerImpl) Start() error {
	return nil
}

func (cmi *ClusterManagerImpl) Stop() error {
	return nil
}

func NewClusterManager(callback dcachelib.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{
		storageCallback: callback,
	}
}

func (cmi *ClusterManagerImpl) CreateClusterConfig(dcacheConfig dcachelib.DCacheConfig, storageCachepath string) error {
	uuidVal, err := common.GetUUID()
	if err != nil {
		log.Err("AddHeartBeat: Failed to retrieve UUID, error: %v", err)
		return err
	}
	clusterConfig := dcachelib.ClusterConfig{
		Readonly:      evaluateReadOnlyState(),
		State:         dcachelib.StateOffline,
		Epoch:         1,
		CreatedAt:     time.Now().Unix(),
		LastUpdatedAt: time.Now().Unix(),
		LastUpdatedBy: uuidVal,
		Config:        dcacheConfig,
		RVList:        fetchRVList(),
		MVList:        evaluateMVsRVMapping(),
	}
	clusterConfigJson, err := json.Marshal(clusterConfig)
	log.Err("ClusterManager::CreateClusterConfig : ClusterConfigJson: %v", err)
	err = cmi.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{Name: storageCachepath + "/ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
	return err
}

func evaluateReadOnlyState() bool {
	return true
}

func (cmi *ClusterManagerImpl) GetActiveMVs() []dcachelib.MirroredVolume {
	return nil
}

func (cmi *ClusterManagerImpl) IsAlive(peerId string) bool {
	return false
}

func (cmi *ClusterManagerImpl) UpdateMVs(mvs []dcachelib.MirroredVolume) {
}

func (cmi *ClusterManagerImpl) UpdateStroageConfigIfRequired() error {
	fetchRVList()
	evaluateMVsRVMapping()
	//Mark the Mv degraded
	return nil
}

func (cmi *ClusterManagerImpl) WatchForConfigChanges() error {

	// Update your local cluster config in the Path
	return nil
}

func fetchRVList() []dcachelib.RawVolume {
	rvList := []dcachelib.RawVolume{}
	//iterate through heartbeat file and get the list of RVs
	//add RV names to the list
	//return the list

	// example
	// rvName := "rv0"
	// rv0 := RawVolume{
	// 	Name:             rvName,
	// 	HostNode:         "node-uuid",
	// 	FSID:             "filesystem-guid",
	// 	FDID:             "fault-domain-id",
	// 	State:            "online",
	// 	TotalSpaceGB:     1000,
	// 	AvailableSpaceGB: 3415,
	// }
	return rvList
}

func evaluateMVsRVMapping() []dcachelib.MirroredVolume {

	mVList := []dcachelib.MirroredVolume{}
	// sample MV list
	// a := []MirroredVolume{
	// 	{
	// 		Name:           "mv0",
	// 		RVmapWithState: []VolumeState{{Volume: rv0, State: "online"}},
	// 	},
	// }
	return mVList
}

func IsAlive(peerId string) bool {
	return false
}

func GetActiveMVs() []dcachelib.MirroredVolume {
	return nil
}
