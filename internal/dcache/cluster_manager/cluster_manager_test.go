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
	cmi              ClusterManager
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

// // replace the old mockHeartbeat with this:
// func mockHeartbeat(nodeID, rvId string, available, total uint64, staleHbduration uint64) {
// 	hb := dcache.HeartbeatData{
// 		NodeID:        nodeID,
// 		LastHeartbeat: uint64(time.Now().Unix()) - staleHbduration,
// 		RVList: []dcache.RawVolume{
// 			{RvId: rvId, State: dcache.StateOnline, AvailableSpace: available, TotalSpace: total},
// 		},
// 	}
// 	hbBytes, _ := json.Marshal(hb)
// 	mockHeartbeatData[nodeID] = hbBytes
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_AddNewRV() {

// 	// Mock data
// 	mockNodeIDs := []string{"node1"}
// 	mockHeartbeat("node1", "rv1", 50, 100, 0)
// 	getAllNodes = func() ([]string, error) {
// 		return mockNodeIDs, nil
// 	}

// 	initialClusterMap := map[string]dcache.RawVolume{}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.True(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_AddNewRVWithExistingRVUpdated() {

// 	// Mock data
// 	mockNodeIDs := []string{"node1", "node2"}
// 	mockHeartbeat("node1", "rvId0", 50, 100, 0)
// 	mockHeartbeat("node2", "rvId1", 50, 100, 0)
// 	getAllNodes = func() ([]string, error) {
// 		return mockNodeIDs, nil
// 	}

// 	initialClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rvId0", State: dcache.StateOnline, AvailableSpace: 20, TotalSpace: 100},
// 	}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rvId0", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 		"rv1": {RvId: "rvId1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.True(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_UpdateExistingRV() {

// 	// Mock data
// 	mockNodeIDs := []string{"node1"}
// 	mockHeartbeat("node1", "rv1", 50, 100, 0)

// 	// Mock functions
// 	getAllNodes = func() ([]string, error) {
// 		return mockNodeIDs, nil
// 	}

// 	initialClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOffline, AvailableSpace: 20, TotalSpace: 100},
// 	}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.True(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_MarkMissingRVOffline() {
// 	// Mock functions
// 	getAllNodes = func() ([]string, error) {
// 		return nil, nil
// 	}
// 	initialClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOffline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.True(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_NoChangesRequired() {
// 	// Mock data
// 	mockNodeIDs := []string{"node1"}

// 	// Mock functions
// 	mockHeartbeat("node1", "rv1", 50, 100, 0)
// 	getAllNodes = func() ([]string, error) {
// 		return mockNodeIDs, nil
// 	}

// 	initialClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.False(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

// func (suite *ClusterManagerImplTestSuite) TestUpdateRVList_MarksStaleHeartbeatOffline() {
// 	// Mock data
// 	mockNodeIDs := []string{"node1"}

// 	// Mock functions
// 	mockHeartbeat("node1", "rv1", 50, 100, 3*60)
// 	getAllNodes = func() ([]string, error) {
// 		return mockNodeIDs, nil
// 	}

// 	initialClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOnline, AvailableSpace: 50, TotalSpace: 100},
// 	}
// 	expectedClusterMap := map[string]dcache.RawVolume{
// 		"rv0": {RvId: "rv1", State: dcache.StateOffline, AvailableSpace: 50, TotalSpace: 100},
// 	}

// 	changed, err := suite.cmi.updateRVList(initialClusterMap, 3, 30)
// 	suite.NoError(err)
// 	suite.True(changed)
// 	suite.Equal(expectedClusterMap, initialClusterMap)
// }

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_EmptyRvMap() {
	mvMap := mockMvMap()
	rvMap := map[string]dcache.RawVolume{} // Empty rvMap

	suite.cmi.config = suite.mockConfig(2, 2)
	suite.cmi.updateMVList(rvMap, mvMap)

	suite.True(len(mvMap) == len(mockMvMap()), "No MVs should be updated when rvMap is empty")
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_EmptyMvMap() {
	mvMap := map[string]dcache.MirroredVolume{} // Empty mvMap
	rvMap := mockRvMap()

	suite.cmi.config = suite.mockConfig(2, 2)
	suite.cmi.updateMVList(rvMap, mvMap)

	suite.updateMvList(mvMap, rvMap, 2, 2)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_MaxMVs() {
	mvMap := map[string]dcache.MirroredVolume{}
	rvMap := mockRvMap()

	suite.cmi.config = suite.mockConfig(1, 1)
	suite.cmi.updateMVList(rvMap, mvMap)

	suite.Equal(len(mvMap), len(rvMap), "Number of updated MVs should be equal to number of RVs")
	suite.updateMvList(mvMap, rvMap, suite.cmi.config.NumReplicas, suite.cmi.config.MvsPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_OfflineMv() {
	mvMap := mockMvMap()
	rvMap := mockRvMap()

	rv := rvMap["rv0"]
	rv.State = dcache.StateOffline
	rvMap["rv0"] = rv

	rv = rvMap["rv1"]
	rv.State = dcache.StateOffline
	rvMap["rv1"] = rv

	suite.cmi.config = suite.mockConfig(2, 2)
	suite.cmi.updateMVList(rvMap, mvMap)

	suite.Equal(mvMap["mv0"].State, dcache.StateOffline, "Updated MV0 should be offline")
	suite.updateMvList(mvMap, rvMap, suite.cmi.config.NumReplicas, suite.cmi.config.MvsPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_OfflineRv() {
	mvMap := mockMvMap()
	mvMap["mv0"].RVs["rv6"] = dcache.StateOnline
	mvMap["mv1"].RVs["rv5"] = dcache.StateOnline
	rvMap := mockRvMap()
	rv := rvMap["rv4"]
	rv.State = dcache.StateOffline
	rvMap["rv4"] = rv

	suite.cmi.config = suite.mockConfig(3, 5)
	suite.cmi.updateMVList(rvMap, mvMap)

	for _, mv := range mvMap {
		_, ok := mv.RVs["rv4"]
		suite.False(ok, "RV4 should not be present in any MV")
	}
	suite.updateMvList(mvMap, rvMap, suite.cmi.config.NumReplicas, suite.cmi.config.MvsPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_DegradedMv() {
	mvMap := mockMvMap()
	rvMap := mockRvMap()

	rv := rvMap["rv0"]
	rv.State = dcache.StateOffline
	rvMap["rv0"] = rv

	suite.cmi.config = suite.mockConfig(2, 2)
	suite.cmi.updateMVList(rvMap, mvMap)

	suite.Equal(mvMap["mv0"].State, dcache.StateDegraded, "Updated MV0 should be degraded")
	suite.updateMvList(mvMap, rvMap, suite.cmi.config.NumReplicas, suite.cmi.config.MvsPerRv)
}

func (suite *ClusterManagerImplTestSuite) TestUpdateMvList_FixMv() {

	mvMap := mockMvMap()
	rvMap := mockRvMap()

	rv := rvMap["rv0"]
	rv.State = dcache.StateOffline
	rvMap["rv0"] = rv
	rv2 := rvMap["rv2"]
	rv2.State = dcache.StateOffline
	rvMap["rv2"] = rv2
	rv3 := rvMap["rv3"]
	rv3.State = dcache.StateOffline
	rvMap["rv3"] = rv3

	suite.cmi.config = suite.mockConfig(2, 2)
	suite.cmi.updateMVList(rvMap, mvMap)
	suite.Equal(mvMap["mv0"].State, dcache.StateDegraded, "Updated MV0 should be degraded")
	suite.Equal(mvMap["mv1"].State, dcache.StateOffline, "Updated MV1 should be offline")
	suite.updateMvList(mvMap, rvMap, suite.cmi.config.NumReplicas, suite.cmi.config.MvsPerRv)
}

func (suite *ClusterManagerImplTestSuite) updateMvList(updated map[string]dcache.MirroredVolume, rvMap map[string]dcache.RawVolume, numReplicas uint32, mvPerRv uint64) {
	suite.True(len(updated) > 0)

	// Check if all the mv's have numReplica rvs
	for _, mv := range updated {
		suite.Equal(numReplicas, uint32(len(mv.RVs)))
	}

	// Iterate over mvMap and check if any rv is repeated more than mvsPerRv times overall
	count := make([]int, len(rvMap))
	for i := range count {
		count[i] = int(mvPerRv)
	}
	for _, mv := range updated {
		for rv := range mv.RVs {
			index, err := strconv.Atoi(rv[2:])
			suite.Nil(err)
			count[index]--
			suite.GreaterOrEqual(count[index], 0)
		}
	}

	// Check if node diversity is maintained
	for _, mv := range updated {
		nodeMap := make(map[string]bool)
		for rv := range mv.RVs {
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
			RVs: map[string]dcache.StateEnum{
				"rv0": dcache.StateOnline,
				"rv1": dcache.StateOnline,
			},
			State: dcache.StateOnline,
		},
		"mv1": {
			RVs: map[string]dcache.StateEnum{
				"rv2": dcache.StateOnline,
				"rv3": dcache.StateOnline,
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

func (suite *ClusterManagerImplTestSuite) mockConfig(i int, j int) *dcache.DCacheConfig {
	if suite.cmi.config == nil {
		suite.cmi.config = &dcache.DCacheConfig{}
	}
	suite.cmi.config.NumReplicas = uint32(i)
	suite.cmi.config.MvsPerRv = uint64(j)
	return &dcache.DCacheConfig{
		NumReplicas: suite.cmi.config.NumReplicas,
		MvsPerRv:    suite.cmi.config.MvsPerRv,
	}
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(ClusterManagerImplTestSuite))
}
