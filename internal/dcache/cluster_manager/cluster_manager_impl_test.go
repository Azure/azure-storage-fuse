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
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/stretchr/testify/suite"
)

var (
	// shared map for heartbeat mocks
	mockHeartbeatData map[string][]byte
)

type ClusterManagerImplTestSuite struct {
	suite.Suite
	cmi              ClusterManagerImpl
	origGetAllNodes  func() ([]string, error)
	origGetHeartbeat func(string) ([]byte, error)
}

func (suite *ClusterManagerImplTestSuite) SetupTest() {
	mockHeartbeatData = make(map[string][]byte)

	// save originals
	suite.origGetAllNodes = getAllNodes
	suite.origGetHeartbeat = getHeartbeat

	// override getHeartbeat to use mockHeartbeatData
	getHeartbeat = func(nodeId string) ([]byte, error) {
		if b, ok := mockHeartbeatData[nodeId]; ok {
			return b, nil
		}
		return nil, fmt.Errorf("heartbeat not found for node %s", nodeId)
	}
}

func (suite *ClusterManagerImplTestSuite) TearDownTest() {
	// restore originals
	getAllNodes = suite.origGetAllNodes
	getHeartbeat = suite.origGetHeartbeat
}

func (suite *ClusterManagerImplTestSuite) TestCheckIfClusterMapExists() {
	orig := getClusterMap
	defer func() { getClusterMap = orig }()

	// 1) success
	getClusterMap = func() ([]byte, *string, error) { return nil, nil, nil }
	exists, err := suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.True(exists)

	// 2) os.ErrNotExist
	getClusterMap = func() ([]byte, *string, error) { return nil, nil, os.ErrNotExist }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 3) syscall.ENOENT
	getClusterMap = func() ([]byte, *string, error) { return nil, nil, syscall.ENOENT }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 4) other error
	testErr := errors.New("boom")
	getClusterMap = func() ([]byte, *string, error) { return nil, nil, testErr }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.EqualError(err, "boom")
	suite.False(exists)
}

// replace the old mockHeartbeat with this:
func mockHeartbeat(nodeID, rvId string, available, total uint64) {
	hb := dcache.HeartbeatData{
		NodeID: nodeID,
		RVList: []dcache.RawVolume{
			{RvId: rvId, State: dcache.StateOnline, AvailableSpace: available, TotalSpace: total},
		},
	}
	hbBytes, _ := json.Marshal(hb)
	mockHeartbeatData[nodeID] = hbBytes
}

func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_AddNewRV() {

	// Mock data
	mockNodeIDs := []string{"node1"}
	mockHeartbeat("node1", "rv1", 50, 100)
	getAllNodes = func() ([]string, error) {
		return mockNodeIDs, nil
	}

	initialClusterMap := map[string]dcache.RawVolume{}
	expectedClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}

	changed, err := suite.cmi.updateRVList(initialClusterMap)
	suite.NoError(err)
	suite.True(changed)
	suite.Equal(expectedClusterMap, initialClusterMap)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_AddNewRVWithExisting() {

	// Mock data
	mockNodeIDs := []string{"node1", "node2"}
	mockHeartbeat("node1", "rvId0", 50, 100)
	mockHeartbeat("node2", "rvId1", 50, 100)
	getAllNodes = func() ([]string, error) {
		return mockNodeIDs, nil
	}

	initialClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rvId0", State: dcache.StateOnline, AvailableSpace: 20, TotalSpace: 100},
	}
	expectedClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rvId0", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
		"rv1": {RvId: "rvId1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}

	changed, err := suite.cmi.updateRVList(initialClusterMap)
	suite.NoError(err)
	suite.True(changed)
	suite.Equal(expectedClusterMap, initialClusterMap)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_UpdateExistingRV() {

	// Mock data
	mockNodeIDs := []string{"node1"}
	mockHeartbeat("node1", "rv1", 50, 100)

	// Mock functions
	getAllNodes = func() ([]string, error) {
		return mockNodeIDs, nil
	}

	initialClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOffline, AvailableSpace: 20, TotalSpace: 100},
	}
	expectedClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}

	changed, err := suite.cmi.updateRVList(initialClusterMap)
	suite.NoError(err)
	suite.True(changed)
	suite.Equal(expectedClusterMap, initialClusterMap)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_MarkMissingRVOffline() {
	// Mock functions
	getAllNodes = func() ([]string, error) {
		return nil, nil
	}
	initialClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}
	expectedClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOffline, AvailableSpace: 50, TotalSpace: 100},
	}

	changed, err := suite.cmi.updateRVList(initialClusterMap)
	suite.NoError(err)
	suite.True(changed)
	suite.Equal(expectedClusterMap, initialClusterMap)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_NoChangesRequired() {
	// Mock data
	mockNodeIDs := []string{"node1"}

	// Mock functions
	mockHeartbeat("node1", "rv1", 50, 100)
	getAllNodes = func() ([]string, error) {
		return mockNodeIDs, nil
	}

	initialClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}
	expectedClusterMap := map[string]dcache.RawVolume{
		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}

	changed, err := suite.cmi.updateRVList(initialClusterMap)
	suite.NoError(err)
	suite.False(changed)
	suite.Equal(expectedClusterMap, initialClusterMap)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_EmptyRvMap() {
	mvMap := mockMvMap()
	rvMap := map[string]dcache.RawVolume{} // Empty rvMap

	numReplicas := 2
	mvPerRv := 2
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	suite.True(len(updated) == len(mvMap), "No MVs should be updated when rvMap is empty")
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_EmptyMvMap() {
	mvMap := map[string]dcache.MirroredVolume{} // Empty mvMap
	rvMap := mockRvMap()

	numReplicas := 2
	mvPerRv := 2
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	suite.TestUpdateMvList(updated, rvMap, numReplicas, mvPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_MaxMVs() {
	mvMap := map[string]dcache.MirroredVolume{}
	rvMap := mockRvMap()

	numReplicas := 1
	mvPerRv := 1
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	suite.Equal(len(updated), len(rvMap), "Number of updated MVs should be equal to number of RVs")
	suite.TestUpdateMvList(updated, rvMap, numReplicas, mvPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_OfflineMv() {
	mvMap := mockMvMap()
	// Remove rv1 from mv0
	// delete(mvMap["mv0"].RVWithStateMap, "rv1")
	// // Remove rv3 from mv1
	// delete(mvMap["mv1"].RVWithStateMap, "rv3")
	rvMap := mockRvMap()

	rv := rvMap["rv0"]
	rv.State = dcache.StateOffline
	rvMap["rv0"] = rv

	rv = rvMap["rv1"]
	rv.State = dcache.StateOffline
	rvMap["rv1"] = rv

	numReplicas := 2
	mvPerRv := 2
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	suite.Equal(updated["mv0"].State, dcache.StateOffline, "Updated MV0 should be offline")
	suite.TestUpdateMvList(updated, rvMap, numReplicas, mvPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_OfflineRv() {
	mvMap := mockMvMap()
	mvMap["mv0"].RVWithStateMap["rv6"] = string(dcache.StateOnline)
	mvMap["mv1"].RVWithStateMap["rv5"] = string(dcache.StateOnline)
	rvMap := mockRvMap()
	rv := rvMap["rv4"]
	rv.State = dcache.StateOffline
	rvMap["rv4"] = rv

	numReplicas := 3
	mvPerRv := 5
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	for _, mv := range updated {
		_, ok := mv.RVWithStateMap["rv4"]
		suite.False(ok, "RV4 should not be present in any MV")
	}
	suite.TestUpdateMvList(updated, rvMap, numReplicas, mvPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_DegradedMv() {
	mvMap := mockMvMap()
	rvMap := mockRvMap()

	rv := rvMap["rv0"]
	rv.State = dcache.StateOffline
	rvMap["rv0"] = rv

	numReplicas := 2
	mvPerRv := 2
	updated := suite.cmi.updateMVList(rvMap, mvMap, numReplicas, mvPerRv)

	suite.Equal(updated["mv0"].State, dcache.StateDegraded, "Updated MV0 should be degraded")
	suite.TestUpdateMvList(updated, rvMap, numReplicas, mvPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList(updated map[string]dcache.MirroredVolume, rvMap map[string]dcache.RawVolume, numReplicas int, mvPerRv int) {
	suite.True(len(updated) > 0)

	// Check if all the mv's have numReplica rvs
	for _, mv := range updated {
		suite.Equal(numReplicas, len(mv.RVWithStateMap))
	}

	// Iterate over mvMap and check if any rv is repeated more than mvsPerRv times overall
	count := make([]int, len(rvMap))
	for i := range count {
		count[i] = mvPerRv
	}
	for _, mv := range updated {
		for rv, _ := range mv.RVWithStateMap {
			index, err := strconv.Atoi(rv[2:])
			suite.Nil(err)
			count[index]--
			suite.GreaterOrEqual(count[index], 0)
		}
	}

	// Check if node diversity is maintained
	for _, mv := range updated {
		nodeMap := make(map[string]bool)
		for rv, _ := range mv.RVWithStateMap {
			nodeId := rvMap[rv].NodeId
			_, ok := nodeMap[nodeId]
			suite.False(ok, "Node diversity not maintained")
			nodeMap[nodeId] = true
		}
	}
}

func mockMvMap() map[string]dcache.MirroredVolume {
	return map[string]dcache.MirroredVolume{
		"mv0": {
			RVWithStateMap: map[string]string{
				"rv0": string(dcache.StateOnline),
				"rv1": string(dcache.StateOnline),
			},
			State: dcache.StateOnline,
		},
		"mv1": {
			RVWithStateMap: map[string]string{
				"rv2": string(dcache.StateOnline),
				"rv3": string(dcache.StateOnline),
			},
			State: dcache.StateOnline,
		},
	}
}

func mockRvMap() map[string]dcache.RawVolume {
	return map[string]dcache.RawVolume{
		"rv0": {RvId: "rv0", NodeId: "node0", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
		"rv1": {RvId: "rv1", NodeId: "node1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
		"rv2": {RvId: "rv2", NodeId: "node1", State: dcache.StateOnline, AvailableSpace: 30, TotalSpace: 100}, // Duplicate nodeId
		"rv3": {RvId: "rv3", NodeId: "node3", State: dcache.StateOnline, AvailableSpace: 70, TotalSpace: 100},
		"rv4": {RvId: "rv4", NodeId: "node4", State: dcache.StateOnline, AvailableSpace: 0, TotalSpace: 100},
		"rv5": {RvId: "rv5", NodeId: "node5", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
		"rv6": {RvId: "rv6", NodeId: "node3", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100}, // Duplicate nodeId
		"rv7": {RvId: "rv7", NodeId: "node5", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
		"rv8": {RvId: "rv8", NodeId: "node4", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
	}
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(ClusterManagerImplTestSuite))
}
