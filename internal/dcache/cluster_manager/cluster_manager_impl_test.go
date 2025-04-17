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
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/stretchr/testify/suite"
)

type clusterManagerImplTestSuite struct {
	suite.Suite
	mockStorage dcache.StorageCallbacks
}

// MockStorageCallback is a fake storage callbacks implementation for tests
type MockStorageCallback struct {
	ReadDirAttr           []internal.ObjAttr
	Storage               map[string][]byte
	ReadDirFromStorageErr error
	GetBlobFromStorageErr error
}

func (m *MockStorageCallback) DeleteBlob(opt internal.DeleteFileOptions) error          { return nil }
func (m *MockStorageCallback) DeleteBlobInStorage(opt internal.DeleteFileOptions) error { return nil }
func (m *MockStorageCallback) GetBlob(opt internal.ReadFileWithNameOptions) ([]byte, error) {
	return nil, nil
}
func (m *MockStorageCallback) GetBlobFromStorage(opt internal.ReadFileWithNameOptions) ([]byte, error) {
	if m.GetBlobFromStorageErr != nil {
		return nil, m.GetBlobFromStorageErr
	}
	if content, ok := m.Storage[opt.Path]; ok {
		return content, nil
	}
	return []byte{}, nil
}
func (m *MockStorageCallback) GetProperties(opt internal.GetAttrOptions) (*internal.ObjAttr, error) {
	return nil, nil
}
func (m *MockStorageCallback) GetPropertiesFromStorage(opt internal.GetAttrOptions) (*internal.ObjAttr, error) {
	return nil, nil
}
func (m *MockStorageCallback) PutBlob(opt internal.WriteFromBufferOptions) error          { return nil }
func (m *MockStorageCallback) PutBlobInStorage(opt internal.WriteFromBufferOptions) error { return nil }
func (m *MockStorageCallback) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	return nil, nil
}
func (m *MockStorageCallback) ReadDirFromStorage(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	if m.ReadDirFromStorageErr != nil {
		return nil, m.ReadDirFromStorageErr
	}
	var dirListing []*internal.ObjAttr
	for i := range m.ReadDirAttr {
		dirListing = append(dirListing, &m.ReadDirAttr[i])
	}
	return dirListing, nil
}
func (m *MockStorageCallback) SetProperties(path string, properties map[string]string) error {
	return nil
}
func (m *MockStorageCallback) SetPropertiesInStorage(path string, properties map[string]string) error {
	return nil
}

func (s *clusterManagerImplTestSuite) SetupTest() {
	s.mockStorage = &MockStorageCallback{
		Storage: make(map[string][]byte),
	}
}

func createMockHeartbeat(nodeID, fsid string, available, total uint64) (internal.ObjAttr, string, []byte) {
	attr := internal.ObjAttr{Name: nodeID + ".hb", Path: "/fakeStorage/Nodes/" + nodeID + ".hb"}
	hbData := dcache.HeartbeatData{
		NodeID: nodeID,
		RVList: []dcache.RawVolume{
			{FSID: fsid, State: dcache.StateOnline, AvailableSpace: available, TotalSpace: total},
		},
	}
	hbBytes, _ := json.Marshal(hbData)
	return attr, attr.Path, hbBytes
}

// TestCheckAndUpdateRVMapEmptyClusterMap tests scenario 1:
// clusterMap having no RVs, but we have new data from heartbeats.
func (suite *clusterManagerImplTestSuite) TestCheckAndUpdateRVMapNoRVInClusterMap() {
	attrA, pathA, hbA := createMockHeartbeat("nodeA", "fsidA", 500, 1000)
	attrB, pathB, hbB := createMockHeartbeat("nodeB", "fsidB", 300, 600)
	mockStorage := &MockStorageCallback{
		ReadDirAttr: []internal.ObjAttr{attrA, attrB},
		Storage:     map[string][]byte{pathA: hbA, pathB: hbB},
	}
	// Create a new ClusterManagerImpl
	cmi := &ClusterManagerImpl{
		storageCallback:  mockStorage,
		storageCachePath: "/fakeStorage",
	}
	// Scenario: clusterMap having no RVs
	clusterMapRVMap := make(map[string]dcache.RawVolume)
	isRVMapUpdated, isMVsUpdateNeeded, err := cmi.checkAndUpdateRVMap(clusterMapRVMap)

	suite.Require().NoError(err, "Expected no error from checkAndUpdateRVMap")
	suite.Assert().True(isRVMapUpdated, "Expected isRVMapUpdated=true for new entries")
	suite.Assert().True(isMVsUpdateNeeded, "Expected isMVsUpdateNeeded=true for new entries")
	suite.Assert().Equal(2, len(clusterMapRVMap), "Two new volumes should be added from heartbeats")
	suite.Assert().NotNil(clusterMapRVMap["rv0"])
	suite.Assert().NotNil(clusterMapRVMap["rv1"])
}

// TestCheckAndUpdateRVMapExisting checks scenario 2/3:
// Some entries exist in clusterMapRVMap, and new data must be merged or updated.
func (suite *clusterManagerImplTestSuite) TestCheckAndUpdateRVMapExisting() {

	// Old cluster map: volume "fsidA" is offline with 0 space
	clusterMapRVMap := map[string]dcache.RawVolume{
		"rv0": {FSID: "fsidA", State: dcache.StateOffline, AvailableSpace: 0, TotalSpace: 1000},
	}

	attr, path, hb := createMockHeartbeat("nodeA", "fsidA", 700, 1000)
	mockStorage := &MockStorageCallback{
		ReadDirAttr: []internal.ObjAttr{attr},
		Storage:     map[string][]byte{path: hb},
	}
	cmi := &ClusterManagerImpl{
		storageCallback:  mockStorage,
		storageCachePath: "/fakeStorage",
	}

	isRVMapUpdated, isMVsUpdateNeeded, err := cmi.checkAndUpdateRVMap(clusterMapRVMap)

	suite.Require().NoError(err)
	suite.Assert().True(isRVMapUpdated, "Volume changed from offline to online, so it must be updated")
	suite.Assert().True(isMVsUpdateNeeded, "Online state changes typically require MVs update")

	// Ensure the clusterMap reflected the new data
	suite.Assert().Equal(1, len(clusterMapRVMap))
	updatedRV := clusterMapRVMap["rv0"]
	suite.Assert().Equal(dcache.StateOnline, updatedRV.State, "Should be updated to online")
	suite.Assert().Equal(uint64(700), updatedRV.AvailableSpace, "Should reflect the new available space (700)")
}

// TestCheckAndUpdateRVMapMissing tests the scenario where clusterMap has an entry not found in the heartbeat (becomes offline).
func (suite *clusterManagerImplTestSuite) TestCheckAndUpdateRVMapMissing() {

	// clusterMap has volume fsidA, fsidB
	clusterMapRVMap := map[string]dcache.RawVolume{
		"rv0": {FSID: "fsidA", State: dcache.StateOnline, AvailableSpace: 500, TotalSpace: 1000},
		"rv1": {FSID: "fsidB", State: dcache.StateOnline, AvailableSpace: 200, TotalSpace: 800},
	}

	attr, path, hb := createMockHeartbeat("nodeA", "fsidA", 600, 1000)
	mockStorage := &MockStorageCallback{
		ReadDirAttr: []internal.ObjAttr{attr},
		Storage:     map[string][]byte{path: hb},
	}
	cmi := &ClusterManagerImpl{
		storageCallback:  mockStorage,
		storageCachePath: "/fakeStorage",
	}

	isRVMapUpdated, isMVsUpdateNeeded, err := cmi.checkAndUpdateRVMap(clusterMapRVMap)

	suite.Require().NoError(err)
	suite.Assert().True(isRVMapUpdated, "fsidB should go offline, which is an update")
	suite.Assert().True(isMVsUpdateNeeded, "Offline changes typically require MVs update")

	// fsidA updated from 500 to 600
	suite.Assert().Equal(dcache.StateOnline, clusterMapRVMap["rv0"].State)
	suite.Assert().Equal(uint64(600), clusterMapRVMap["rv0"].AvailableSpace)

	// fsidB is missing in heartbeat -> offline
	suite.Assert().Equal(dcache.StateOffline, clusterMapRVMap["rv1"].State)
}

// TestCheckAndUpdateRVMapMissing tests the scenario where clusterMap has an entries and new rv added(becomes online).
func (suite *clusterManagerImplTestSuite) TestCheckAndUpdateRVMapNewRVAdded() {

	// clusterMap has volume fsidA, fsidB
	clusterMapRVMap := map[string]dcache.RawVolume{
		"rv0": {FSID: "fsidA", State: dcache.StateOnline, AvailableSpace: 500, TotalSpace: 1000},
	}

	attr1, path1, hb1 := createMockHeartbeat("nodeA", "fsidA", 400, 1000)
	attr, path, hb := createMockHeartbeat("nodeB", "fsidB", 200, 800)
	mockStorage := &MockStorageCallback{
		ReadDirAttr: []internal.ObjAttr{attr1, attr},
		Storage:     map[string][]byte{path: hb, path1: hb1},
	}
	cmi := &ClusterManagerImpl{
		storageCallback:  mockStorage,
		storageCachePath: "/fakeStorage",
	}

	isRVMapUpdated, isMVsUpdateNeeded, err := cmi.checkAndUpdateRVMap(clusterMapRVMap)

	suite.Require().NoError(err)
	suite.Assert().True(isRVMapUpdated, "fsidB should go online, which is an new addition")
	suite.Assert().True(isMVsUpdateNeeded, "New RV addition means MV RV mapping update")

	// fsidA updated from 500 to 400
	suite.Assert().Equal(dcache.StateOnline, clusterMapRVMap["rv0"].State)
	suite.Assert().Equal(uint64(400), clusterMapRVMap["rv0"].AvailableSpace)

	// fsidB is added in clusterMap
	suite.Assert().Equal(dcache.StateOnline, clusterMapRVMap["rv1"].State)
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(clusterManagerImplTestSuite))
}
