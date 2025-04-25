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
	"fmt"
	"math/rand"
	"time"

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
	// const (
	// 	NumReplicas = 3
	// 	MVsPerRV    = 10
	// )
	rvMap := fetchRVMap()
	existingMVMap := fetchMVMap()

	// Local types
	type rv struct {
		rvName string
		slots  int // initialized with MVsPerRV
	}

	type node struct {
		nodeId string
		rvs    []rv
		active bool // to mark if node still has available RVs
	}

	// Helper function to find node by ID
	findNode := func(nodes []node, nodeId string) int {
		for i, n := range nodes {
			if n.nodeId == nodeId {
				return i
			}
		}
		return -1
	}

	// Phase 1: Initialize nodes and process existing MVs
	var nodes []node

	// Create initial node structure from RV map
	nodeMap := make(map[string][]rv)
	for rvId, rvInfo := range rvMap {
		nodeMap[rvInfo.NodeId] = append(nodeMap[rvInfo.NodeId], rv{
			rvName: rvId,
			slots:  MvsPerRv,
		})
	}

	// Convert map to slice and initialize nodes
	for nodeId, rvs := range nodeMap {
		nodes = append(nodes, node{
			nodeId: nodeId,
			rvs:    rvs,
			active: true,
		})
	}

	// Process existing MVs
	for _, mv := range existingMVMap {
		for rvId := range mv.RVWithStateMap {
			// Find the node containing this RV
			for nodeIdx := range nodes {
				for rvIdx := range nodes[nodeIdx].rvs {
					if nodes[nodeIdx].rvs[rvIdx].rvName == rvId {
						// Decrease available slots
						nodes[nodeIdx].rvs[rvIdx].slots--
						break
					}
				}
			}
		}
	}

	// Phase 2: Create new MVs
	newMVs := make([]dcache.MirroredVolume, 0)
	currentMVIndex := len(existingMVMap)

	// Helper function to count active nodes
	countActiveNodes := func(nodes []node) int {
		count := 0
		for _, n := range nodes {
			if n.active {
				count++
			}
		}
		return count
	}

	// Helper function to get random active nodes
	getRandomActiveNodes := func(nodes []node, count int) []int {
		if countActiveNodes(nodes) < count {
			return nil
		}

		activeIndices := make([]int, 0)
		used := make(map[int]bool)
		rand.Seed(time.Now().UnixNano())

		for len(activeIndices) < count {
			idx := rand.Intn(len(nodes))
			if !used[idx] && nodes[idx].active {
				activeIndices = append(activeIndices, idx)
				used[idx] = true
			}
		}
		return activeIndices
	}

	// Create new MVs until we can't fill them completely
	for countActiveNodes(nodes) >= NumReplicas {
		// Create new MV
		mv := dcache.MirroredVolume{
			RVWithStateMap: make(map[string]string),
			State:          dcache.StateOnline,
		}

		// Get random active nodes
		selectedNodes := getRandomActiveNodes(nodes, NumReplicas)
		if selectedNodes == nil {
			break
		}

		// Add one RV from each selected node
		for _, nodeIdx := range selectedNodes {
			// Find first RV with available slots
			for rvIdx := range nodes[nodeIdx].rvs {
				if nodes[nodeIdx].rvs[rvIdx].slots > 0 {
					// Add RV to MV
					rvName := nodes[nodeIdx].rvs[rvIdx].rvName
					mv.RVWithStateMap[rvName] = rvMap[rvName].State

					// Decrease slots
					nodes[nodeIdx].rvs[rvIdx].slots--

					// Check if this RV is exhausted
					if nodes[nodeIdx].rvs[rvIdx].slots == 0 {
						// Remove exhausted RV
						nodes[nodeIdx].rvs = append(nodes[nodeIdx].rvs[:rvIdx], nodes[nodeIdx].rvs[rvIdx+1:]...)
					}
					break
				}
			}

			// Check if node has any RVs left
			if len(nodes[nodeIdx].rvs) == 0 {
				nodes[nodeIdx].active = false
			}
		}

		// Add the new MV if it has exactly NumReplicas RVs
		if len(mv.RVWithStateMap) == NumReplicas {
			newMVs = append(newMVs, mv)
			currentMVIndex++
		}
	}

	// Combine existing and new MVs into final result
	result := make(map[string]dcache.MirroredVolume, len(existingMVMap)+len(newMVs))

	// Copy existing MVs
	for mvId, mv := range existingMVMap {
		result[mvId] = mv
	}

	// Add new MVs
	for i, mv := range newMVs {
		mvId := fmt.Sprintf("mv%d", currentMVIndex-len(newMVs)+i)
		result[mvId] = mv
	}

	return result
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
