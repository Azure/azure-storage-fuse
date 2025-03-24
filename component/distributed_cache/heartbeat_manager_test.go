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
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// var home_dir, _ = os.UserHomeDir()
// var mountpoint = home_dir + "mountpoint"
// var dataBuff []byte
// var random *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

type heartbeatManagerTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	fake_storage_path string
	disk_cache_path   string
	loopback          internal.Component
	hbManager         *HeartbeatManager
}

func (suite *heartbeatManagerTestSuite) TestHeartbeatManagerAddHeartBeat() {
	suite.hbManager.Starthb()
	_, err := suite.hbManager.storage.GetAttr(suite.hbManager.hbPath + "/Nodes/" + suite.hbManager.nodeId + ".hb")
	suite.assert.Nil(err)
}

func (suite *heartbeatManagerTestSuite) TestDistributedCacheRemoveHeartBeat() {
	suite.hbManager.Stop()
	_, err := suite.hbManager.storage.GetAttr(suite.hbManager.hbPath + "/Nodes/" + suite.hbManager.nodeId + ".hb")
	suite.assert.NotNil(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHeartbeatManagerTestSuite(t *testing.T) {

	suite.Run(t, new(heartbeatManagerTestSuite))
}
