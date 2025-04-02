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

package distributed_cache

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type hbManagerTestSuite struct {
	suite.Suite
	assert    *assert.Assertions
	hbManager *HeartbeatManager

	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

func (suite *hbManagerTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	cachePath, _ := os.UserHomeDir()
	suite.hbManager = &HeartbeatManager{
		cachePath: cachePath,
		comp:      suite.mock,
		hbPath:    "__CACHE__mycache1",
	}
}

func (suite *hbManagerTestSuite) TestStartHbSuccess() {
	suite.mock.EXPECT().WriteFromBuffer(gomock.Any()).Return(nil)
	err := suite.hbManager.Starthb()
	suite.assert.Nil(err)
}

func (suite *hbManagerTestSuite) TestStartHbFail() {
	suite.mock.EXPECT().WriteFromBuffer(gomock.Any()).Return(errors.New("test error"))
	err := suite.hbManager.Starthb()
	suite.assert.Equal("test error", err.Error())
}

func (suite *hbManagerTestSuite) TestStopHbSuccess() {
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)
	err := suite.hbManager.Stop()
	suite.assert.Nil(err)
}

func (suite *hbManagerTestSuite) TestStopHbFail() {
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(errors.New("test error"))
	err := suite.hbManager.Stop()
	suite.assert.Equal("test error", err.Error())
}
func (suite *hbManagerTestSuite) TestStartDiscoverySuccess() {
	attrs := []*internal.ObjAttr{
		{Name: "node1.hb", Path: "__CACHE__mycache1/Nodes/node1.hb"},
		{Name: "node2.hb", Path: "__CACHE__mycache1/Nodes/node2.hb"},
	}

	peerData := HeartbeatData{
		NodeID:        "node1",
		LastHeartbeat: uint64(time.Now().Unix()),
	}
	data, _ := json.Marshal(peerData)

	suite.mock.EXPECT().ReadDir(gomock.Any()).Return(attrs, nil)
	suite.mock.EXPECT().ReadFileWithName(gomock.Any()).Return(data, nil).Times(len(attrs))

	suite.hbManager.hbDuration = 30
	suite.hbManager.maxMissedHbs = 3

	suite.hbManager.StartDiscovery()

	suite.assert.NotNil(PeersByNodeId["node1"])
	suite.assert.NotNil(PeersByName["__CACHE__mycache1/Nodes/node1.hb"])
}

func (suite *hbManagerTestSuite) TestStartDiscoveryReadDirFail() {
	suite.mock.EXPECT().ReadDir(gomock.Any()).Return(nil, errors.New("read dir error"))

	suite.hbManager.StartDiscovery()

	suite.assert.Empty(PeersByNodeId)
	suite.assert.Empty(PeersByName)
}

func (suite *hbManagerTestSuite) TestStartDiscoveryReadFileFail() {
	attrs := []*internal.ObjAttr{
		{Name: "node1.hb", Path: "__CACHE__mycache1/Nodes/node1.hb"},
	}

	suite.mock.EXPECT().ReadDir(gomock.Any()).Return(attrs, nil)
	suite.mock.EXPECT().ReadFileWithName(gomock.Any()).Return(nil, errors.New("read file error"))

	suite.hbManager.StartDiscovery()

	suite.assert.Empty(PeersByNodeId)
	suite.assert.Empty(PeersByName)
}

func (suite *hbManagerTestSuite) TestStartDiscoveryOldHeartbeat() {
	attrs := []*internal.ObjAttr{
		{Name: "node1.hb", Path: "__CACHE__mycache1/Nodes/node1.hb"},
	}

	peerData := HeartbeatData{
		NodeID:        "node1",
		LastHeartbeat: uint64(time.Now().Add(-time.Hour).Unix()),
	}
	data, _ := json.Marshal(peerData)

	suite.mock.EXPECT().ReadDir(gomock.Any()).Return(attrs, nil)
	suite.mock.EXPECT().ReadFileWithName(gomock.Any()).Return(data, nil)
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)

	suite.hbManager.hbDuration = 30
	suite.hbManager.maxMissedHbs = 3

	suite.hbManager.StartDiscovery()

	suite.assert.Empty(PeersByNodeId)
	suite.assert.Empty(PeersByName)
}

func (suite *hbManagerTestSuite) TestStartDiscoveryDeleteFileFail() {
	attrs := []*internal.ObjAttr{
		{Name: "node1.hb", Path: "__CACHE__mycache1/Nodes/node1.hb"},
	}

	peerData := HeartbeatData{
		NodeID:        "node1",
		LastHeartbeat: uint64(time.Now().Add(-time.Hour).Unix()),
	}
	data, _ := json.Marshal(peerData)

	suite.mock.EXPECT().ReadDir(gomock.Any()).Return(attrs, nil)
	suite.mock.EXPECT().ReadFileWithName(gomock.Any()).Return(data, nil)
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(errors.New("delete file error"))

	suite.hbManager.hbDuration = 10
	suite.hbManager.maxMissedHbs = 3

	suite.hbManager.StartDiscovery()

	suite.assert.Empty(PeersByNodeId)
	suite.assert.Empty(PeersByName)
}
func TestHeartbeatManagerTestSuite(t *testing.T) {

	suite.Run(t, new(hbManagerTestSuite))
}
