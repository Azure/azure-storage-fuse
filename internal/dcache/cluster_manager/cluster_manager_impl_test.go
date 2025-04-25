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

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(ClusterManagerImplTestSuite))
}
