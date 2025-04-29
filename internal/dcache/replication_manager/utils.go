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
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
)

const (
	// TODO: discuss if this is a good value for RPC timeout
	RPCClientTimeout = 2 // in seconds
)

func isComponentRVsValid(componentRVs []*models.RVNameAndState) bool {
	if len(componentRVs) == 0 {
		log.Err("utils::isComponentRVsValid: No component RVs found")
		common.Assert(false, "no component RVs found")
		return false
	}

	common.Assert(len(componentRVs) == getNumReplicas())

	for _, rv := range componentRVs {
		if rv == nil || rv.Name == "" || rv.State == "" {
			log.Err("utils::isComponentRVsValid: Invalid component RV found: %+v", rv)
			common.Assert(false, fmt.Sprintf("invalid component RV found: %+v", rv))
			return false
		}
	}

	return true
}

func getReaderRV(componentRVs []*models.RVNameAndState, excluseRVs []string) *models.RVNameAndState {
	log.Debug("utils::getReaderRV: Component RVs are: %v", rpc_server.ComponentRVsToString(componentRVs))

	// TODO:: integration: call cluster manager to get the node ID of this node
	myNodeID := getNodeUUID()

	onlineRVs := make([]*models.RVNameAndState, 0)
	for _, rv := range componentRVs {
		if rv.State != string(dcache.StateOnline) || slices.Contains(excluseRVs, rv.Name) {
			// this is not an online RV or is present in the exclude list
			// so skip this RV
			continue
		}

		// TODO:: integration: call cluster manager to get the node ID for the given rv
		nodeIDForRV := getNodeIDForRV(rv.Name)
		if nodeIDForRV == myNodeID {
			// this is the local RV in this node
			return rv
		}

		onlineRVs = append(onlineRVs, rv)
	}

	if len(onlineRVs) == 0 {
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

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the component RVs for the given MV
func getComponentRVsForMV(mvName string) []*models.RVNameAndState {
	return []*models.RVNameAndState{
		&models.RVNameAndState{Name: "rv0", State: string(dcache.StateOnline)},
		&models.RVNameAndState{Name: "rv1", State: string(dcache.StateOffline)},
		&models.RVNameAndState{Name: "rv2", State: string(dcache.StateOnline)},
	}
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the number of replicas for the given MV
func getNumReplicas() int {
	return 3
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the node ID of this node
func getNodeUUID() string {
	return "node-uuid" // TODO: get the node uuid from the config
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the node ID for the given rv
// this might not be needed if the "getComponentRVsForMV" method returns struct where all information of a RV is present
func getNodeIDForRV(rv string) string {
	return "node-uuid" // TODO: get the node uuid from the config
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the RV ID for the given RV name
// this might not be needed if the "getComponentRVsForMV" method returns struct where all information of a RV is present
func getRvIDFromRvName(rvName string) string {
	return "rvID"
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the node ID for the given RV name
// this might not be needed if the "getComponentRVsForMV" method returns struct where all information of a RV is present
func getNodeIDForRVName(rvName string) string {
	return "nodeID"
}
