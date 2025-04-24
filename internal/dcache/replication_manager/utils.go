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
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	RPCClientTimeout = 2 // in seconds
)

func selectOnlineRVForMV(mvName string) (string, error) {
	// TODO:: integration: call cluster manager to get the component RVs for the given MV
	componentRVs := getComponentRVsForMV(mvName)
	if len(componentRVs) == 0 {
		log.Err("utils::selectOnlineRVForMV: No component RVs found for the given MV %s", mvName)
		common.Assert(false, "no component RVs found for the given MV", mvName)
		return "", fmt.Errorf("no component RVs found for the given MV %s", mvName)
	}

	log.Debug("utils::selectOnlineRVForMV: Component RVs for the given MV %s are: %v", mvName, componentRVs)

	// TODO:: integration: call cluster manager to get the node ID of this node
	myNodeID := getNodeUUID()

	onlineRVs := make([]string, 0)
	for _, rv := range componentRVs {
		if strings.Contains(rv, "=") {
			// this is not an online RV if flags are present, so skip this RV
			// For example, rv1=offline, rv5=outofsync, etc.
			continue
		}

		// TODO:: integration: call cluster manager to get the node ID for the given rv
		nodeIDForRV := getNodeIDForRV(rv)
		if nodeIDForRV == myNodeID {
			// this is the local RV in this node
			return rv, nil
		}

		onlineRVs = append(onlineRVs, rv)
	}

	if len(onlineRVs) == 0 {
		return "", fmt.Errorf("no online RVs found for the given MV %s", mvName)
	}

	// select random online RV
	// TODO: add logic for sending Hello RPC call to check if the node hosting this RV is online
	// If not, select another RV from the list
	index := rand.Intn(len(onlineRVs))
	return onlineRVs[index], nil
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the component RVs for the given MV
func getComponentRVsForMV(mvName string) []string {
	return []string{"rv0", "rv1", "rv2"}
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
