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

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

type ClusterManagerImpl struct {
	storageCallback dcache.StorageCallbacks
}

// GetRVs implements ClusterManager.
func (c *ClusterManagerImpl) GetRVs(mvName string) []dcache.RawVolume {
	panic("unimplemented")
}

// ReportRVDown implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVDown(rvName string) error {
	panic("unimplemented")
}

// ReportRVFull implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVFull(rvName string) error {
	panic("unimplemented")
}

// GetDegradedMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetDegradedMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// LowestNumberRV implements ClusterManager.
func (c *ClusterManagerImpl) LowestNumberRV(rvs []string) []string {
	return make([]string, 0)
}

// NodeIdToIP implements ClusterManager.
func (c *ClusterManagerImpl) NodeIdToIP(nodeId string) string {
	return ""
}

// RVFsidToName implements ClusterManager.
func (c *ClusterManagerImpl) RVFsidToName(rvFsid string) string {
	return ""
}

// RVNameToFsid implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToFsid(rvName string) string {
	return ""
}

// RVNameToIp implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToIp(rvName string) string {
	return ""
}

// RVNameToNodeId implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToNodeId(rvName string) string {
	return ""
}

// GetActiveMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetActiveMVs() []dcache.MirroredVolume {
	return nil
}

// IsAlive implements ClusterManager.
func (c *ClusterManagerImpl) IsAlive(nodeId string) bool {
	return false
}

// Start implements ClusterManager.
func (cmi *ClusterManagerImpl) Start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	cmi.createClusterMapIfRequired(dCacheConfig, rvs)
	//schedule Punch heartbeat
	//Schedule clustermap config update at storage and local copy
	return nil
}

func (cmi *ClusterManagerImpl) createClusterMapIfRequired(dCacheConfig *dcache.DCacheConfig, rvList []dcache.RawVolume) error {
	if cmi.checkIfClusterMapExists(dCacheConfig.CacheId) {
		log.Trace("ClusterManager::createClusterConfig : ClusterMap.json already exists")
		return nil
	}
	clusterConfig := dcache.ClusterMap{
		Readonly:      evaluateReadOnlyState(),
		State:         dcache.StateReady,
		Epoch:         1,
		CreatedAt:     time.Now().Unix(),
		LastUpdatedAt: time.Now().Unix(),
		LastUpdatedBy: rvList[0].NodeId,
		Config:        *dCacheConfig,
		RVMap:         map[string]dcache.RawVolume{},
		MVMap:         map[string]dcache.MirroredVolume{},
	}
	clusterConfigJson, err := json.Marshal(clusterConfig)
	log.Err("ClusterManager::CreateClusterConfig : ClusterConfigJson: %v, err %v", clusterConfigJson, err)
	// err = cmi.metaManager.PutMetaFile(internal.WriteFromBufferOptions{Name: clusterManagerConfig.StorageCachePath + "/ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
	err = cmi.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{Name: "__CACHE__" + dCacheConfig.CacheId + "/ClusterMap.json", Data: []byte(clusterConfigJson), IsNoneMatchEtagEnabled: true})
	return err
	// return nil
}

func (cmi *ClusterManagerImpl) checkIfClusterMapExists(cacheId string) bool {
	_, err := cmi.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{Name: "__CACHE__" + cacheId + "/ClusterMap.json"})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			return false
		}
		log.Err("ClusterManagerImpl::checkIfClusterMapExists: Failed to check configFile presence in Storage path %s error: %v", "__CACHE__"+cacheId+"/ClusterMap.json", err)
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
func NewClusterManager(callback dcache.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{
		storageCallback: callback,
	}
}
