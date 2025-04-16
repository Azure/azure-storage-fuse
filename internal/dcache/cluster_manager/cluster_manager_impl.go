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
	"os"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

type ClusterManagerImpl struct {
	storageCallback dcache.StorageCallbacks
	hbTicker        *time.Ticker
	nodeId          string
}

// GetActiveMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetActiveMVs() []dcache.MirroredVolume {
	return nil
}

// GetPeer implements ClusterManager.
func (c *ClusterManagerImpl) GetPeer(nodeId string) dcache.Peer {
	return dcache.Peer{}
}

// GetPeerRVs implements ClusterManager.
func (c *ClusterManagerImpl) GetPeerRVs(mvName string) []dcache.RawVolume {
	return nil
}

// IsAlive implements ClusterManager.
func (c *ClusterManagerImpl) IsAlive(peerId string) bool {
	return false
}

// Start implements ClusterManager.
func (cmi *ClusterManagerImpl) Start(clusterManagerConfig ClusterManagerConfig) error {
	cmi.createClusterConfig(clusterManagerConfig)
	cmi.hbTicker = time.NewTicker(time.Duration(clusterManagerConfig.HeartbeatSeconds) * time.Second)
	go func() {
		for range cmi.hbTicker.C {
			log.Trace("Scheduled task triggered")
			cmi.punchHeartBeat(clusterManagerConfig)
		}
	}()
	//Schedule clustermap config update at storage and local copy
	return nil
}

func (cmi *ClusterManagerImpl) punchHeartBeat(clusterManagerConfig ClusterManagerConfig) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Err("Error getting hostname:", err)
	}
	listMyRVs(clusterManagerConfig.RVList)
	cmi.nodeId = clusterManagerConfig.RVList[0].NodeId
	hbData := dcache.HeartbeatData{
		IPAddr:        clusterManagerConfig.RVList[0].IPAddress,
		NodeID:        cmi.nodeId,
		Hostname:      hostname,
		LastHeartbeat: uint64(time.Now().Unix()),
		RVList:        clusterManagerConfig.RVList,
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	if err != nil {
		log.Err("AddHeartBeat: Failed to marshal heartbeat data")
	}

	// Create a heartbeat file in storage with <nodeId>.hb
	if err := cmi.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{Name: clusterManagerConfig.StorageCachePath + "/Nodes/" + cmi.nodeId + ".hb", Data: data}); err != nil {
		log.Err("AddHeartBeat: Failed to write heartbeat file: ", err)
	}
	log.Trace("AddHeartBeat: Heartbeat file updated successfully")
}

func listMyRVs(rvList []dcache.RawVolume) {
	for index, rv := range rvList {
		log.Trace("RV %d: %s", index, rv)
		usage, err := common.GetUsage(rv.LocalCachePath)
		if err != nil {
			log.Err("failed to get usage for path %s: %v", rv.LocalCachePath, err)
		}
		rvList[index].AvailableSpace = rv.TotalSpace - uint64(usage)*1024
		// TODO{Akku}: If available space is less than 10% of total space, set state to offline
		rvList[index].State = dcache.StateOnline
	}
}

func (cmi *ClusterManagerImpl) createClusterConfig(clusterManagerConfig ClusterManagerConfig) error {
	if cmi.checkIfClusterMapExists(clusterManagerConfig.StorageCachePath) {
		log.Trace("ClusterManager::createClusterConfig : ClusterMap.json already exists")
		return nil
	}
	dcacheConfig := dcache.DCacheConfig{
		MinNodes:  clusterManagerConfig.MinNodes,
		ChunkSize: clusterManagerConfig.ChunkSize}
	clusterConfig := dcache.ClusterConfig{
		Readonly:      evaluateReadOnlyState(),
		State:         dcache.StateOffline,
		Epoch:         1,
		CreatedAt:     time.Now().Unix(),
		LastUpdatedAt: time.Now().Unix(),
		LastUpdatedBy: clusterManagerConfig.RVList[0].NodeId,
		Config:        dcacheConfig,
		RVMap:         map[string]dcache.RawVolume{},
		MVMap:         map[string]dcache.MirroredVolume{},
	}
	clusterConfigJson, err := json.Marshal(clusterConfig)
	log.Err("ClusterManager::CreateClusterConfig : ClusterConfigJson: %v, err %v", clusterConfigJson, err)
	// err = cmi.metaManager.PutMetaFile(internal.WriteFromBufferOptions{Name: clusterManagerConfig.StorageCachePath + "/ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
	err = cmi.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{Name: clusterManagerConfig.StorageCachePath + "/ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
	return err
	// return nil
}

func (cmi *ClusterManagerImpl) checkIfClusterMapExists(path string) bool {
	_, err := cmi.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{Name: path + "/ClusterMap.json"})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			return false
		}
		log.Err("ClusterManagerImpl::checkIfClusterMapExists: Failed to check configFile presence in Storage path %s error: %v", path+"/ClusterMap.json", err)
	}
	return true
}

func evaluateMVsRVMapping() map[string]dcache.MirroredVolume {

	mvRvMap := map[string]dcache.MirroredVolume{}
	// rvStateMap := map[string]string{
	// 	"rv0": "online",
	// 	"rv1": "offline",
	// 	"rv2": "syncing"}
	// mv0 := dcache.MirroredVolume{
	// 	RVWithStateMap: rvStateMap,
	// 	State:          dcache.StateOffline,
	// }
	// mvRvMap["mv0"] = mv0
	return mvRvMap
}

func fetchRVMap() map[string]dcache.RawVolume {
	//Iterate over Heartbeat File
	rvMap := map[string]dcache.RawVolume{}
	// rv0 := dcache.RawVolume{
	// 	HostNode:         "Node1",
	// 	FSID:             "FSID1",
	// 	FDID:             "FDID1",
	// 	State:            "Active",
	// 	TotalSpaceGB:     100,
	// 	AvailableSpaceGB: 50,
	// 	LocalCachePath:   "/path/to/cache",
	// }
	// rvMap["rv0"] = rv0
	return rvMap
}

func evaluateReadOnlyState() bool {
	return false
}

// Stop implements ClusterManager.
func (c *ClusterManagerImpl) Stop() error {
	return nil
}

// UpdateMVs implements ClusterManager.
func (c *ClusterManagerImpl) UpdateMVs(mvs []dcache.MirroredVolume) {
}

// UpdateStorageConfigIfRequired implements ClusterManager.
func (c *ClusterManagerImpl) UpdateStorageConfigIfRequired() error {
	return nil
}

// WatchForConfigChanges implements ClusterManager.
func (c *ClusterManagerImpl) WatchForConfigChanges() error {
	return nil
}

func NewClusterManager(callback dcache.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{
		storageCallback: callback,
	}
}
