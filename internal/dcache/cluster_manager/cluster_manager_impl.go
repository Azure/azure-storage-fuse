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
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

type ClusterManagerImpl struct {
	storageCallback  dcache.StorageCallbacks
	hbTicker         *time.Ticker
	clusterMapticker *time.Ticker
	nodeId           string
	storageCachePath string
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
func (cmi *ClusterManagerImpl) Start(clusterManagerConfig *ClusterManagerConfig) error {
	cmi.storageCachePath = clusterManagerConfig.StorageCachePath
	cmi.nodeId = clusterManagerConfig.RVList[0].NodeId
	cmi.createClusterConfig(clusterManagerConfig)
	cmi.hbTicker = time.NewTicker(time.Duration(clusterManagerConfig.HeartbeatSeconds) * time.Second)
	go func() {
		for range cmi.hbTicker.C {
			log.Trace("Scheduled task Heartbeat Punch triggered")
			cmi.punchHeartBeat(clusterManagerConfig)
		}
	}()
	cmi.clusterMapticker = time.NewTicker(time.Duration(clusterManagerConfig.ClustermapEpoch) * time.Second)
	go func() {
		for range cmi.clusterMapticker.C {
			log.Trace("Scheduled Cluster Map update task triggered")
			cmi.UpdateStorageConfigIfRequired()
			cmi.UpdateClusterMapCacheCopy()
		}
	}()
	return nil
}

func (c *ClusterManagerImpl) UpdateClusterMapCacheCopy() {
	//update my local copy of cluster map if anythig is change
	//Notify to replication manager if there is any change
}

func (cmi *ClusterManagerImpl) punchHeartBeat(clusterManagerConfig *ClusterManagerConfig) {
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

func (cmi *ClusterManagerImpl) createClusterConfig(clusterManagerConfig *ClusterManagerConfig) error {
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

func (cmi *ClusterManagerImpl) checkAndUpdateRVMap(clusterMapRVMap map[string]dcache.RawVolume) (bool, bool, error) {
	dirListAttr, err := cmi.storageCallback.ReadDirFromStorage(internal.ReadDirOptions{Name: cmi.storageCachePath + "/Nodes"})
	isRVMapUpdated := false
	isMVsUpdateNeeded := false
	if err != nil {
		log.Err("ClusterManagerImpl::checkAndUpdateRVMap: Failed to read directory from Storage %s, error: %v", cmi.storageCachePath+"/Nodes", err)
		return isRVMapUpdated, isMVsUpdateNeeded, err
	}
	log.Trace("ClusterManagerImpl::checkAndUpdateRVMap: Heartbeat files in storage %s", dirListAttr)

	rVsByBlkID := make(map[string]dcache.RawVolume)

	for _, fileAttr := range dirListAttr {

		bytes, err := cmi.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{Path: fileAttr.Path})
		if err != nil {
			log.Err("ClusterManagerImpl::checkAndUpdateRVMap: Failed to read heartbeat file %s, error: %v", fileAttr.Path, err)
			return isRVMapUpdated, isMVsUpdateNeeded, err
		}
		var hbData dcache.HeartbeatData
		if err := json.Unmarshal(bytes, &hbData); err != nil {
			log.Err("ClusterManagerImpl::checkAndUpdateRVMap: Failed to parse JSON, error: %v", err)
			return isRVMapUpdated, isMVsUpdateNeeded, err
		}
		for _, rv := range hbData.RVList {
			rVsByBlkID[rv.FSID] = rv
		}
	}
	//There can be 3 scenarios
	//1. There is nothing in clusterMap and RVs are present in heartbeat
	//2. There is something in clusterMap which needs to be updated
	//3. There is something in heartbeat which needs to be added to clusterMap

	if len(clusterMapRVMap) == 0 && len(rVsByBlkID) != 0 {
		isRVMapUpdated = true
		isMVsUpdateNeeded = true
		i := 0
		for _, rv := range rVsByBlkID {
			clusterMapRVMap[fmt.Sprintf("rv%d", i)] = rv
			i++
		}

		return isRVMapUpdated, isMVsUpdateNeeded, nil
	}
	rVsExistsInClusterMapByBlkID := make(map[string]dcache.RawVolume)
	rvNameList := make([]string, 0, len(clusterMapRVMap))
	for rvName, rvInClusterMap := range clusterMapRVMap {
		if rvHb, found := rVsByBlkID[rvInClusterMap.FSID]; found {
			if rvInClusterMap.State != rvHb.State {
				isRVMapUpdated = true
				isMVsUpdateNeeded = true
				rvInClusterMap.State = rvHb.State
			}
			if rvInClusterMap.AvailableSpace != rvHb.AvailableSpace {
				isRVMapUpdated = true
				rvInClusterMap.AvailableSpace = rvHb.AvailableSpace
				if rvInClusterMap.AvailableSpace < (rvInClusterMap.TotalSpace / 10) {
					isMVsUpdateNeeded = true
				}

			}
			rVsExistsInClusterMapByBlkID[rvInClusterMap.FSID] = rvHb
		} else {
			log.Trace("ClusterManagerImpl::checkAndUpdateRVMap: FSID=%s missing in new heartbeats", rvName)
			rvInClusterMap.State = dcache.StateOffline
			isRVMapUpdated = true
			isMVsUpdateNeeded = true

		}
		clusterMapRVMap[rvName] = rvInClusterMap
		rvNameList = append(rvNameList, rvName)
	}
	if len(rvNameList) != 0 {
		sort.Strings(rvNameList)
		lastRVName := rvNameList[len(rvNameList)-1]
		i, _ := strconv.Atoi(strings.Split(lastRVName, "rv")[1])
		for blkId, rv := range rVsByBlkID {
			if _, exists := rVsExistsInClusterMapByBlkID[blkId]; !exists {
				i++
				clusterMapRVMap[fmt.Sprintf("rv%d", i)] = rv
				isRVMapUpdated = true
				isMVsUpdateNeeded = true
			}
		}
	}
	return isRVMapUpdated, isMVsUpdateNeeded, nil
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
func (cmi *ClusterManagerImpl) UpdateStorageConfigIfRequired() {
	bytes, err := cmi.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{Path: cmi.storageCachePath + "/ClusterMap.json"})
	if err != nil {
		log.Err("UpdateStorageConfigIfRequired: bytes %v, err %v", bytes, err)
		return
	}
	var clusterCfg dcache.ClusterConfig
	if err := json.Unmarshal(bytes, &clusterCfg); err != nil {
		log.Err("UpdateStorageConfigIfRequired: failed to parse JSON, error: %v", err)
		return
	}

	if (clusterCfg.LastUpdatedBy == cmi.nodeId) || (time.Now().Unix()-clusterCfg.LastUpdatedAt > int64(clusterCfg.Config.ClustermapEpoch)) {
		log.Trace("I am the leader or Cluster map is stale. Proceed with updating the storage cluster map.")
		isRVMapUpdated, isMVsUpdateNeeded, err := cmi.checkAndUpdateRVMap(clusterCfg.RVMap)
		log.Err("UpdateStorageConfigIfRequired: isRVMapUpdated %v, isMVsUpdateNeeded %v, err %v", isRVMapUpdated, isMVsUpdateNeeded, err)

		if isMVsUpdateNeeded {
			//TODO{Akku}: evaluateMVsRVMapping()
		}
		clusterCfg.LastUpdatedBy = cmi.nodeId
		clusterCfg.LastUpdatedAt = time.Now().Unix()
		clusterCfgByte, _ := json.Marshal(clusterCfg)
		cmi.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
			Name: cmi.storageCachePath + "/ClusterMap.json",
			Data: clusterCfgByte,
			// EtagMatchConditions: ,
			//TODO{Akku}: ADD Etag condition
		})

		//If required trigger replication manager
	}

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
