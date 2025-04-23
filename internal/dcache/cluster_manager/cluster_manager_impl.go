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
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
)

type ClusterManagerImpl struct {
	hbTicker  *time.Ticker
	nodeId    string
	hostname  string
	ipAddress string
}

// It will return online MVs as per local cache copy of cluster map
func GetActiveMVs() []dcache.MirroredVolume {
	return clusterManagerInstance.getActiveMVs()
}

// It will return the cache config as per local cache copy of cluster map
func GetCacheConfig() *dcache.DCacheConfig {
	return clusterManagerInstance.getCacheConfig()
}

// It will return offline/down MVs as per local cache copy of cluster map
func GetDegradedMVs() []dcache.MirroredVolume {
	return clusterManagerInstance.getDegradedMVs()
}

// It will return all the RVs for particular mv name as per local cache copy of cluster map
func GetRVs(mvName string) []dcache.RawVolume {
	return clusterManagerInstance.getRVs(mvName)
}

// It will check if the given nodeId is online as per local cache copy of cluster map
func IsOnline(nodeId string) bool {
	return clusterManagerInstance.isOnline(nodeId)
}

// It will evaluate the lowest number of RVs for given rv Names
func LowestNumberRV(rvNames []string) []string {
	return clusterManagerInstance.lowestNumberRV(rvNames)
}

// It will return the IP address of the given nodeId as per local cache copy of cluster map
func NodeIdToIP(nodeId string) string {
	return clusterManagerInstance.nodeIdToIP(nodeId)
}

// It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map
func RvIdToName(rvId string) string {
	return clusterManagerInstance.rvIdToName(rvId)
}

// It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map
func RvNameToId(rvName string) string {
	return clusterManagerInstance.rvNameToId(rvName)
}

// It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map
func RVNameToNodeId(rvName string) string {
	return clusterManagerInstance.rVNameToNodeId(rvName)
}

// It will return the IP address of the given RV name as per local cache copy of cluster map
func RVNameToIp(rvName string) string {
	return clusterManagerInstance.rVNameToIp(rvName)
}

// Update RV state to down and update MVs
func ReportRVDown(rvName string) error {
	return clusterManagerInstance.reportRVDown(rvName)
}

// Update RV state to offline and update MVs
func ReportRVFull(rvName string) error {
	return clusterManagerInstance.reportRVFull(rvName)
}

func Stop() error {
	return clusterManagerInstance.stop()
}

// start implements ClusterManager.
func (cmi *ClusterManagerImpl) start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	cmi.nodeId = rvs[0].NodeId

	//TODO{Akku}: fix this assert to just work with 1 return value
	common.Assert(common.IsValidUUID(cmi.nodeId))
	err := cmi.checkAndCreateInitialClusterMap(dCacheConfig)
	if err != nil {
		return err
	}
	cmi.hostname, err = os.Hostname()
	if err != nil {
		return err
	}
	cmi.ipAddress = rvs[0].IPAddress
	common.Assert(common.IsValidIP(cmi.ipAddress), fmt.Sprintf("Invalid Ip[%s] for nodeId[%s]", cmi.ipAddress, cmi.nodeId))
	cmi.hbTicker = time.NewTicker(time.Duration(dCacheConfig.HeartbeatSeconds) * time.Second)
	go func() {
		for range cmi.hbTicker.C {
			log.Debug("Scheduled task Heartbeat Punch triggered")
			cmi.punchHeartBeat(rvs)
		}
		log.Info("Scheduled task Heartbeat Punch stopped")
	}()
	//Schedule clustermap update at storage and local copy
	return nil
}

// Stop implements ClusterManager.
func (cmi *ClusterManagerImpl) stop() error {
	if cmi.hbTicker != nil {
		cmi.hbTicker.Stop()
	}
	// TODO{Akku}: Delete the heartbeat file
	// mm.DeleteHeartbeat(cmi.nodeId)
	return nil
}

// getActiveMVs implements ClusterManager.
func (c *ClusterManagerImpl) getActiveMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// getCacheConfig implements ClusterManager.
func (cmi *ClusterManagerImpl) getCacheConfig() *dcache.DCacheConfig {
	return nil
}

// getDegradedMVs implements ClusterManager.
func (c *ClusterManagerImpl) getDegradedMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// getRVs implements ClusterManager.
func (c *ClusterManagerImpl) getRVs(mvName string) []dcache.RawVolume {
	return make([]dcache.RawVolume, 0)
}

func (c *ClusterManagerImpl) isOnline(nodeId string) bool {
	return false
}

// lowestNumberRV implements ClusterManager.
func (c *ClusterManagerImpl) lowestNumberRV(rvNames []string) []string {
	return make([]string, 0)
}

// nodeIdToIP implements ClusterManager.
func (c *ClusterManagerImpl) nodeIdToIP(nodeId string) string {
	return ""
}

// rvIdToName implements ClusterManager.
func (c *ClusterManagerImpl) rvIdToName(rvId string) string {
	return ""
}

// rvNameToId implements ClusterManager.
func (c *ClusterManagerImpl) rvNameToId(rvName string) string {
	return ""
}

// rVNameToIp implements ClusterManager.
func (c *ClusterManagerImpl) rVNameToIp(rvName string) string {
	return ""
}

// rVNameToNodeId implements ClusterManager.
func (c *ClusterManagerImpl) rVNameToNodeId(rvName string) string {
	return ""
}

// reportRVDown implements ClusterManager.
func (c *ClusterManagerImpl) reportRVDown(rvName string) error {
	return nil
}

// reportRVFull implements ClusterManager.
func (c *ClusterManagerImpl) reportRVFull(rvName string) error {
	return nil
}

func (cmi *ClusterManagerImpl) checkAndCreateInitialClusterMap(dCacheConfig *dcache.DCacheConfig) error {
	isClusterMapExists, err := cmi.checkIfClusterMapExists()
	if err != nil {
		log.Err("ClusterManagerImpl::checkAndCreateInitialClusterMap: Failed to check clusterMap file presence in Storage. error -: %v", err)
		return err
	}
	if isClusterMapExists {
		log.Info("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap already exists")
		return nil
	}
	currentTime := time.Now().Unix()
	clusterMap := dcache.ClusterMap{
		Readonly:      true,
		State:         dcache.StateReady,
		Epoch:         1,
		CreatedAt:     currentTime,
		LastUpdatedAt: currentTime,
		LastUpdatedBy: cmi.nodeId,
		Config:        *dCacheConfig,
		RVMap:         map[string]dcache.RawVolume{},
		MVMap:         map[string]dcache.MirroredVolume{},
	}
	clusterMapBytes, err := json.Marshal(clusterMap)
	if err != nil {
		log.Err("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap Marshalling fail : %+v, err %v", clusterMap, err)
		return err
	}

	err = mm.CreateInitialClusterMap(clusterMapBytes)
	if err != nil {
		log.Err("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap creation fail : %+v, err %v", clusterMap, err)
		return err
	} else {
		log.Info("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap created successfully : %+v", clusterMap)
	}
	return err
}

func (cmi *ClusterManagerImpl) checkIfClusterMapExists() (bool, error) {
	err := getClusterMap()
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			return false, nil
		} else {
			return false, err
		}
	}
	//TODO: Save the cluster map in local copy
	return true, nil
}

var getClusterMap = func() error {
	_, _, err := mm.GetClusterMap()
	return err
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

func (cmi *ClusterManagerImpl) punchHeartBeat(rvList []dcache.RawVolume) {

	listMyRVs(rvList)
	hbData := dcache.HeartbeatData{
		IPAddr:        cmi.ipAddress,
		NodeID:        cmi.nodeId,
		Hostname:      cmi.hostname,
		LastHeartbeat: uint64(time.Now().Unix()),
		RVList:        rvList,
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	//Adding Assert because error capturing can just log the error and continue because it's a schedule method
	common.Assert(err == nil, fmt.Sprintf("Error marshalling heartbeat data %+v : error - %v", hbData, err))
	if err == nil {
		// Create and update heartbeat file in storage with <nodeId>.hb
		err = mm.UpdateHeartbeat(cmi.nodeId, data)
		common.Assert(err == nil, fmt.Sprintf("Error updating heartbeat file with nodeId %s in storage: %v", cmi.nodeId, err))
		log.Debug("AddHeartBeat: Heartbeat file updated successfully %+v", hbData)
	} else {
		log.Warn("Error Updating heartbeat for nodeId %s with data %+v : error - %v", cmi.nodeId, hbData, err)
	}
}

func listMyRVs(rvList []dcache.RawVolume) {
	for index, rv := range rvList {
		_, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(rv.LocalCachePath)
		common.Assert(err == nil, fmt.Sprintf("Error getting disk space metrics for path %s for punching heartbeat: %v", rv.LocalCachePath, err))
		if err != nil {
			availableSpace = 0
			log.Warn("Error getting disk space metrics for path %s for punching heartbeat that's why forcing available Space to set to zero : %v", rv.LocalCachePath, err)
		}
		rvList[index].AvailableSpace = availableSpace
		rvList[index].State = dcache.StateOnline
	}
}

var (
	// clusterManagerInstance is the singleton instance of the ClusterManagerImpl
	clusterManagerInstance ClusterManager = &ClusterManagerImpl{}
	initCalled                            = false
)

func Init(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	common.Assert(!initCalled, "Cluster Manager Init should only be called once")
	initCalled = true
	err := clusterManagerInstance.start(dCacheConfig, rvs)
	return err
}
