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
	// TODO: discuss if this is a good value for RPC timeout
	RPCClientTimeout = 2 // in seconds

	ChunkIndexUpperBound = 1e9
)

func getReaderRV(componentRVs []*models.RVNameAndState, excludeRVs []string) *models.RVNameAndState {
	log.Debug("utils::getReaderRV: Component RVs are: %v, excludeRVs: %v", rpc.ComponentRVsToString(componentRVs), excludeRVs)

	myNodeID := rpc.GetMyNodeUUID()
	onlineRVs := make([]*models.RVNameAndState, 0)
	for _, rv := range componentRVs {
		if rv.State != string(dcache.StateOnline) || slices.Contains(excludeRVs, rv.Name) {
			// this is not an online RV or is present in the exclude list
			// so skip this RV
			log.Debug("utils::getReaderRV: skipping RV %s with state %s", rv.Name, rv.State)
			continue
		}

		nodeIDForRV := getNodeIDFromRVName(rv.Name)
		common.Assert(common.IsValidUUID(nodeIDForRV))
		if nodeIDForRV == myNodeID {
			// this is the local RV in this node
			return rv
		}

		onlineRVs = append(onlineRVs, rv)
	}

	if len(onlineRVs) == 0 {
		log.Debug("utils::getReaderRV: no online RVs found for component RVs %v", rpc.ComponentRVsToString(componentRVs))
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
