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

package replication_manager

import (
	"fmt"
	"math/rand"
	"slices"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

const (
	// TODO: For prod we should increase it for resilience, but not too much as to affect
	// our responsiveness.
	RPCClientTimeout = 2 // in seconds

	// This is a practically infeasible chunk index, for sanity checks.
	ChunkIndexUpperBound = 1e9

	// Time interval in seconds for resyncing the degraded MV.
	ResyncInterval = 30
)

func getReaderRV(componentRVs []*models.RVNameAndState, excludeRVs []string) *models.RVNameAndState {
	log.Debug("utils::getReaderRV: Component RVs are: %v, excludeRVs: %v",
		rpc.ComponentRVsToString(componentRVs), excludeRVs)

	// componentRVs must have exactly NumReplicas RVs.
	common.Assert(len(componentRVs) == int(getNumReplicas()), len(componentRVs), getNumReplicas())
	// excludeRVs can have at max all the RVs in componentRVs.
	common.Assert(len(excludeRVs) <= len(componentRVs), len(excludeRVs), len(componentRVs))

	myNodeID := rpc.GetMyNodeUUID()
	onlineRVs := make([]*models.RVNameAndState, 0)
	for _, rv := range componentRVs {
		if rv.State != string(dcache.StateOnline) || slices.Contains(excludeRVs, rv.Name) {
			// Not an online RV or present in the exclude list, skip.
			log.Debug("utils::getReaderRV: skipping RV %s with state %s", rv.Name, rv.State)
			continue
		}

		nodeIDForRV := getNodeIDFromRVName(rv.Name)
		common.Assert(common.IsValidUUID(nodeIDForRV))
		if nodeIDForRV == myNodeID {
			// Prefer local RV.
			return rv
		}

		onlineRVs = append(onlineRVs, rv)
	}

	if len(onlineRVs) == 0 {
		log.Debug("utils::getReaderRV: no suitable RVs found for component RVs %v",
			rpc.ComponentRVsToString(componentRVs))
		return nil
	}

	// select random online RV
	// TODO: add logic for sending Hello RPC call to check if the node hosting this RV is online
	// If not, select another RV from the list
	index := rand.Intn(len(onlineRVs))
	return onlineRVs[index]
}

// TODO: hash validation will be done later
// TODO: should byte array be used for storing hash instead of string?
// check is md5sum can be used for hash or crc should be used
// func getMD5Sum(data []byte) string {
// 	hash := md5.Sum(data)
// 	return hex.EncodeToString(hash[:])
// }

// Return list of component RVs (name and state) for the given MV.
func getComponentRVsForMV(mvName string) []*models.RVNameAndState {
	rvMap := cm.GetRVs(mvName)
	return convertRVMapToList(mvName, rvMap)
}

func convertRVMapToList(mvName string, rvMap map[string]dcache.StateEnum) []*models.RVNameAndState {
	var componentRVs []*models.RVNameAndState
	for rvName, rvState := range rvMap {
		common.Assert(cm.IsValidRVName(rvName), rvName)
		common.Assert(rvState == dcache.StateOnline ||
			rvState == dcache.StateOffline ||
			rvState == dcache.StateOutOfSync ||
			rvState == dcache.StateSyncing, rvName, rvState)

		componentRVs = append(componentRVs,
			&models.RVNameAndState{Name: rvName, State: string(rvState)})
	}

	common.Assert(len(componentRVs) == int(getNumReplicas()),
		fmt.Sprintf("number of component RVs %d is not same as number of replicas %d for MV %s: %v",
			len(componentRVs), getNumReplicas(), mvName, rpc.ComponentRVsToString(componentRVs)))

	return componentRVs
}

func convertRVListToMap(mvName string, componentRVs []*models.RVNameAndState) map[string]dcache.StateEnum {
	rvMap := make(map[string]dcache.StateEnum)
	for _, rv := range componentRVs {
		common.Assert(rv != nil, "Component RV is nil")
		common.Assert(cm.IsValidRVName(rv.Name), rv.Name)
		common.Assert(rv.State == string(dcache.StateOnline) ||
			rv.State == string(dcache.StateOffline) ||
			rv.State == string(dcache.StateOutOfSync) ||
			rv.State == string(dcache.StateSyncing), rv.Name, rv.State)

		rvMap[rv.Name] = dcache.StateEnum(rv.State)
	}

	common.Assert(len(rvMap) == int(getNumReplicas()),
		fmt.Sprintf("number of component RVs %d is not same as number of replicas %d for MV %s: %v",
			len(rvMap), getNumReplicas(), mvName, rvMap))

	return rvMap
}

// return the number of replicas
func getNumReplicas() uint32 {
	return cm.GetCacheConfig().NumReplicas
}

// return the RV ID for the given RV name
func getRvIDFromRvName(rvName string) string {
	return cm.RvNameToId(rvName)
}

// return the node ID for the given rvName
func getNodeIDFromRVName(rvName string) string {
	return cm.RVNameToNodeId(rvName)
}

// return the local cache path for the given RV name
// Note: this RV should be hosted by the this node
func getCachePathForRVName(rvName string) string {
	myRVs := cm.GetMyRVs()
	common.Assert(myRVs != nil, "nodes's raw volumes cannot be nil")
	common.Assert(len(myRVs) > 0, "nodes's raw volumes cannot be empty")

	rv, ok := myRVs[rvName]
	common.Assert(ok, fmt.Sprintf("RV %s is not hosted by the node. Raw volumes: %+v", rvName, myRVs))
	common.Assert(rv.LocalCachePath != "", fmt.Sprintf("RV %s local cache path is empty", rvName))
	common.Assert(common.DirectoryExists(rv.LocalCachePath),
		fmt.Sprintf("RV %s local cache path %s does not exist", rvName, rv.LocalCachePath))

	return rv.LocalCachePath
}

// Update the state of the target RV in the cluster map.
func updateComponentRVState(mvName string, targetRVName string, targetRVState dcache.StateEnum, componentRVs []*models.RVNameAndState) error {
	common.Assert(cm.IsValidMVName(mvName), mvName)
	common.Assert(cm.IsValidRVName(targetRVName), targetRVName)
	common.Assert(targetRVState == dcache.StateOnline ||
		targetRVState == dcache.StateOffline ||
		targetRVState == dcache.StateOutOfSync ||
		targetRVState == dcache.StateSyncing, targetRVName, targetRVState)

	log.Debug("utils::updateComponentRVState: updating component RV state for MV %s, target RV %s, state %s, component RVs: %v",
		mvName, targetRVName, targetRVState, rpc.ComponentRVsToString(componentRVs))

	rvMap := convertRVListToMap(mvName, componentRVs)

	common.Assert(rvMap[targetRVName] != targetRVState,
		fmt.Sprintf("target RV %s state is already %s", targetRVName, targetRVState))

	// update the state of the target RV in the map
	rvMap[targetRVName] = targetRVState

	log.Debug("utils::updateComponentRVState: MV %s, updated component RVs : %v",
		mvName, rvMap)

	err := cm.UpdateComponentRVState(mvName, dcache.MirroredVolume{
		State: dcache.StateDegraded, // NOTE: state of MV is ignored, as it is taken care by the UpdateComponentRVState() method in cluster manager
		RVs:   rvMap,
	})
	if err != nil {
		errStr := fmt.Sprintf("failed to update component RV state for MV %s, RV %s, state %s: %v",
			mvName, targetRVName, targetRVState, err)
		log.Err("utils::updateComponentRVState: %v", errStr)
		common.Assert(false, errStr)
		return err
	}

	return nil
}
