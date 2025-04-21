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
	"math"
	"strconv"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

type ClusterManagerImpl struct {
	storageCallback dcache.StorageCallbacks
}

// Start implements ClusterManager.
func (c *ClusterManagerImpl) Start(*dcache.DCacheConfig, []dcache.RawVolume) error {
	return nil
}

// Stop implements ClusterManager.
func (c *ClusterManagerImpl) Stop() error {
	return nil
}

// GetActiveMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetActiveMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// GetDegradedMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetDegradedMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// GetRVs implements ClusterManager.
func (c *ClusterManagerImpl) GetRVs(mvName string) []dcache.RawVolume {
	return make([]dcache.RawVolume, 0)
}

// IsAlive implements ClusterManager.
func (c *ClusterManagerImpl) IsAlive(nodeId string) bool {
	return false
}

// LowestNumberRV implements ClusterManager.
func (c *ClusterManagerImpl) LowestNumberRV(rvNames []string) []string {
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

// ReportRVDown implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVDown(rvName string) error {
	return nil
}

// ReportRVFull implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVFull(rvName string) error {
	return nil
}

func evaluateMVRVMapping(NumReplicas int, MvsPerRv int) map[string]dcache.MirroredVolume {

	// Need to fetch latest MV Map from storage and update the local copy
	mvMap := fetchMVMap()
	// Need informtation about newest Rv's added
	rvMap := fetchRVMap()

	numRvs := len(rvMap)
	// Calculate the number of Mvs needed
	numMvs := int(math.Ceil(float64(numRvs) * float64(MvsPerRv) / float64(NumReplicas)))

	// Group Rvs by node for distribution
	nodeToRvs := make(map[string][]string)
	for rvID, rvInfo := range rvMap {
		nodeToRvs[rvInfo.NodeId] = append(nodeToRvs[rvInfo.NodeId], rvID)
	}

	// First pass: Direct distribution in a single scan
	// Assign Rvs to Mvs while maintaining constraints
	currentMvIndex := len(mvMap)
	startMvIndex := currentMvIndex

	// Create tracking maps
	rvAssignmentCount := make(map[string]int)                       // Track how many times each Rv has been assigned
	mvRvSet := make([]map[string]bool, numMvs)                      // Track which Rvs are in each Mv
	mvNodeCount := make([]map[string]int, numMvs)                   // Track how many Rvs from each node are in each Mv
	mvMap = append(mvMap, make([]dcache.MirroredVolume, numMvs)...) // Ensure mvMap has enough elements

	for i := currentMvIndex; i < currentMvIndex+numMvs; i++ {
		mvMap[i].RVWithStateMap = make(map[string]dcache.StateEnum)
		mv := mvMap[i]
		mv.State = dcache.StateOnline
		mvMap[i] = mv
	}

	for i := range numMvs {
		mvRvSet[i] = make(map[string]bool)
		mvNodeCount[i] = make(map[string]int)
	}

	nodesProcessed := 0
	nodeIDs := make([]string, 0, len(nodeToRvs))
	for nodeID := range nodeToRvs {
		nodeIDs = append(nodeIDs, nodeID)
	}

	// Ensure all Node's Rv's are distributed MvPerRv times
	for nodesProcessed < len(nodeToRvs)*MvsPerRv {

		// Iterate through each node
		for _, nodeID := range nodeIDs {
			// Check if all Rvs from this node have been assigned
			for _, rvID := range nodeToRvs[nodeID] {
				// Skip if this Rv has been fully assigned
				if rvAssignmentCount[rvID] >= MvsPerRv {
					continue
				}

				// Find next suitable Mv
				for attempts := range numMvs {
					// Calculate the Mv index in round robin fashion between startIndex and startIndex+numMvs
					mvIndex := startMvIndex + (currentMvIndex+attempts)%numMvs

					// Check if this Mv has space and doesn't already have this Rv
					if len(mvMap[mvIndex].RVWithStateMap) < NumReplicas {
						if mvRvSet[mvIndex%numMvs] != nil && !mvRvSet[mvIndex%numMvs][rvID] {
							// Assign the Rv to this Mv
							mv := mvMap[mvIndex]
							mv.RVWithStateMap[rvID] = rvMap[rvID].State
							mvMap[mvIndex] = mv
							// Update tracking maps
							// Mark Rv as assigned to this Mv
							mvRvSet[mvIndex%numMvs][rvID] = true
							// Track how many Rvs from this node are in this Mv
							mvNodeCount[mvIndex%numMvs][nodeID]++
							// Track how many times this Rv has been assigned
							rvAssignmentCount[rvID]++

							// Move to next Mv for better distribution
							currentMvIndex = mvIndex + 1
							if currentMvIndex >= numMvs {
								currentMvIndex = startMvIndex
							}
							break
						}
					}
				}
			}
			nodesProcessed++
		}
	}

	// Delete Mvs with less Rvs than NumReplicas
	for i := range mvMap {
		if len(mvMap[i].RVWithStateMap) < NumReplicas {
			// Delete mv from map
			mvMap = append(mvMap[:i], mvMap[i+1:]...)
			i-- // Adjust index after deletion
		}
	}

	// Make a new map of string to MirroredVolume lenght len(mvMap)
	mvRvMap := make(map[string]dcache.MirroredVolume, len(mvMap))
	for i := range mvMap {
		mvId := "mv" + strconv.Itoa(i)
		mvRvMap[mvId] = mvMap[i]
	}
	// Update the RVWithStateMap for each Mv

	return mvRvMap
}

func fetchMVMap() []dcache.MirroredVolume {
	// mvMap := make([]dcache.MirroredVolume, 2)
	// mvMap[0] = dcache.MirroredVolume{
	// 	RVWithStateMap: map[string]dcache.StateEnum{
	// 		"rv0": dcache.StateOnline,
	// 		"rv1": dcache.StateOnline,
	// 		"rv2": dcache.StateOnline,
	// 	},
	// 	State: dcache.StateOnline,
	// }
	// mvMap[1] = dcache.MirroredVolume{
	// 	RVWithStateMap: map[string]dcache.StateEnum{
	// 		"rv3": dcache.StateOnline,
	// 		"rv4": dcache.StateOnline,
	// 	},
	// 	State: dcache.StateOffline,
	// }

	return nil
}

func fetchRVMap() map[string]dcache.RawVolume {
	// is it possible to display only the new RV's that came up?

	return nil
}

func NewClusterManager(callback dcache.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{
		storageCallback: callback,
	}
}
