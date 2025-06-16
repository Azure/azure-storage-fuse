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

package gc

import (
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

const (
	// TODO: For prod we should increase it for resilience, but not too much as to affect
	// our responsiveness.
	RPCClientTimeout = 2 // in seconds
)

// return the RV ID for the given RV name
func getRvIDFromRvName(rvName string) string {
	return cm.RvNameToId(rvName)
}

// return the node ID for the given rvName
func getNodeIDFromRVName(rvName string) string {
	return cm.RVNameToNodeId(rvName)
}

// Return list of component RVs (name and state) for the given MV, and its state, and also the clustermap Epoch.
// The epoch should be used by the caller to correctly refresh the clustermap on receiving a NeedToRefreshClusterMap
// error.
func getComponentRVsForMV(mvName string) (dcache.StateEnum, []*models.RVNameAndState, int64) {
	mvState, rvMap, epoch := cm.GetRVsEx(mvName)
	return mvState, convertRVMapToList(mvName, rvMap), epoch
}

func convertRVMapToList(mvName string, rvMap map[string]dcache.StateEnum) []*models.RVNameAndState {
	var componentRVs []*models.RVNameAndState

	for rvName, rvState := range rvMap {
		common.Assert(cm.IsValidRVName(rvName), rvName)
		common.Assert(cm.IsValidComponentRVState(rvState), rvName, rvState)

		componentRVs = append(componentRVs,
			&models.RVNameAndState{Name: rvName, State: string(rvState)})
	}

	common.Assert(len(componentRVs) == int(getNumReplicas()),
		mvName, len(componentRVs), getNumReplicas(), rpc.ComponentRVsToString(componentRVs))

	return componentRVs
}

// return the number of replicas
func getNumReplicas() uint32 {
	return cm.GetCacheConfig().NumReplicas
}

func getMyNodeId() string {
	nodeId, _ := common.GetNodeUUID()
	return nodeId
}

func getNumChunksForFile(file *dcache.FileMetadata) int64 {
	return (file.Size + file.FileLayout.ChunkSize - 1) / file.FileLayout.ChunkSize
}

func getNextChunkIdxInMV(curChunkIdx int64, numMvs int64) int64 {
	return curChunkIdx + numMvs
}
