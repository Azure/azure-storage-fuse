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
	"math"
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

	//schedule Punch heartbeat
	//Schedule clustermap config update at storage and local copy
	return nil
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

func EvaluateMVsRVMapping() map[int]dcache.MirroredVolume {

	mvMap := map[int]dcache.MirroredVolume{}
	rvMap := fetchRVMap()
	// mvMap := fecthMVMap()

	// Calculate number of MVs
	numRVs := len(rvMap)

	NumReplicas := 1
	MvsPerRv := 1

	numMVs := int(math.Ceil(float64(numRVs) * float64(MvsPerRv) / float64(NumReplicas)))

	// Group RVs by node for distribution
	nodeToRVs := make(map[string][]string)
	for rvID, rvInfo := range rvMap {
		nodeToRVs[rvInfo.NodeId] = append(nodeToRVs[rvInfo.NodeId], rvID)
	}

	// Create tracking maps
	rvAssignmentCount := make(map[string]int)     // Track how many times each RV has been assigned
	mvRVSet := make([]map[string]bool, numMVs)    // Track which RVs are in each MV
	mvNodeCount := make([]map[string]int, numMVs) // Track how many RVs from each node are in each MV

	// TODO :: Check from which Mv state is offline or iterate through all see which is more secure
	for i := range mvMap {
		mvRVSet[i] = make(map[string]bool)
		mvNodeCount[i] = make(map[string]int)
	}

	// First pass: Direct distribution in a single scan
	// Assign RVs to MVs while maintaining constraints
	currentMVIndex := 0

	// Process nodes in a round-robin fashion to ensure diversity
	nodesProcessed := 0
	nodeIDs := make([]string, 0, len(nodeToRVs))
	for nodeID := range nodeToRVs {
		nodeIDs = append(nodeIDs, nodeID)
	}

	for nodesProcessed < len(nodeToRVs)*MvsPerRv {
		for _, nodeID := range nodeIDs {
			for _, rvID := range nodeToRVs[nodeID] {
				// Skip if this RV has been fully assigned
				if rvAssignmentCount[rvID] >= MvsPerRv {
					continue
				}

				// Find next suitable MV
				for attempts := 0; attempts < numMVs; attempts++ {
					mvIndex := (currentMVIndex + attempts) % numMVs

					// Check if this MV has space and doesn't already have this RV
					if len(mvMap[mvIndex].RVWithStateMap) < NumReplicas {
						if mvRVSet[mvIndex] == nil || (mvRVSet[mvIndex] != nil && !mvRVSet[mvIndex][rvID]) {
							// Assign the RV to this MV
							mv := mvMap[mvIndex]
							mv.RVWithStateMap[rvID] = rvMap[rvID].State
							mvMap[mvIndex] = mv
							mvRVSet[mvIndex][rvID] = true
							// mvMap[mvIndex].Nodes[nodeID] = true
							mvNodeCount[mvIndex][nodeID]++
							rvAssignmentCount[rvID]++

							// Move to next MV for better distribution
							currentMVIndex = (mvIndex + 1) % numMVs
							break
						}
					}
				}
			}

			nodesProcessed++
			// Break early if we've assigned all RVs
			// if len(rvInstances) == 0 {
			// 	break
			// }
		}
	}

	// Mark MVs with fewer RVs as special
	for i := range mvMap {
		if len(mvMap[i].RVWithStateMap) < NumReplicas {
			mv := mvMap[i]
			mv.State = dcache.StateOffline
			mvMap[i] = mv
		}
	}

	return mvMap
}

// go through the RVMap and find

// rvStateMap := map[string]string{
// 	"rv0": "online",
// 	"rv1": "offline",
// 	"rv2": "syncing"}
// mv0 := dcache.MirroredVolume{
// 	RVWithStateMap: rvStateMap,
// 	State:          dcache.StateOffline,
// }
// mvRvMap["mv0"] = mv0

func fetchRVMap() map[string]dcache.RawVolume {
	rvMap := map[string]dcache.RawVolume{
		"rv0": {
			NodeId:         "Node1",
			IPAddress:      "192.168.1.1",
			FSID:           "FSID1",
			FDID:           "FDID1",
			State:          dcache.StateOnline,
			TotalSpace:     100,
			AvailableSpace: 50,
			LocalCachePath: "/path/to/cache/rv0",
		},
		"rv1": {
			NodeId:         "Node1",
			IPAddress:      "192.168.1.1",
			FSID:           "FSID2",
			FDID:           "FDID2",
			State:          dcache.StateOnline,
			TotalSpace:     200,
			AvailableSpace: 150,
			LocalCachePath: "/path/to/cache/rv1",
		},

		// Node 2 RVs
		"rv2": {
			NodeId:         "Node2",
			IPAddress:      "192.168.1.2",
			FSID:           "FSID3",
			FDID:           "FDID3",
			State:          dcache.StateOnline,
			TotalSpace:     300,
			AvailableSpace: 250,
			LocalCachePath: "/path/to/cache/rv2",
		},
		"rv3": {
			NodeId:         "Node2",
			IPAddress:      "192.168.1.2",
			FSID:           "FSID4",
			FDID:           "FDID4",
			State:          dcache.StateOnline,
			TotalSpace:     400,
			AvailableSpace: 350,
			LocalCachePath: "/path/to/cache/rv3",
		},

		// Node 3 RVs
		"rv4": {
			NodeId:         "Node3",
			IPAddress:      "192.168.1.3",
			FSID:           "FSID5",
			FDID:           "FDID5",
			State:          dcache.StateOnline,
			TotalSpace:     500,
			AvailableSpace: 450,
			LocalCachePath: "/path/to/cache/rv4",
		},
		"rv5": {
			NodeId:         "Node3",
			IPAddress:      "192.168.1.3",
			FSID:           "FSID6",
			FDID:           "FDID6",
			State:          dcache.StateOnline,
			TotalSpace:     600,
			AvailableSpace: 550,
			LocalCachePath: "/path/to/cache/rv5",
		},
	}

	return rvMap
}

// Example RVs

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
func (c *ClusterManagerImpl) UpdateStorageConfigIfRequired() {
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
