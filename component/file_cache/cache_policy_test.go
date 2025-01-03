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

package file_cache

import (
	"io/fs"
	"math"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cachePolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *cachePolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	suite.assert = assert.New(suite.T())
	os.Mkdir(cache_path, fs.FileMode(0777))
}

func (suite *cachePolicyTestSuite) cleanupTest() {
	os.RemoveAll(cache_path)
}

func (suite *cachePolicyTestSuite) TestGetUsage() {
	defer suite.cleanupTest()
	f, _ := os.Create(cache_path + "/test")
	data := make([]byte, 1024*1024)
	f.Write(data)
	result, _ := common.GetUsage(cache_path)
	suite.assert.Equal(float64(1), math.Floor(result))
	f.Close()
}

func (suite *cachePolicyTestSuite) TestGetUsagePercentage() {
	defer suite.cleanupTest()
	data := make([]byte, 1024*1024)

	f, _ := os.Create(cache_path + "/test")
	f.Write(data)
	result := getUsagePercentage(cache_path, 4)
	// since the value might defer a little distro to distro
	suite.assert.GreaterOrEqual(result, float64(25))
	suite.assert.LessOrEqual(result, float64(30))
	f.Close()

	result = getUsagePercentage("/", 0)
	// since the value might defer a little distro to distro
	suite.assert.GreaterOrEqual(result, float64(0))
	suite.assert.LessOrEqual(result, float64(90))
}

func (suite *cachePolicyTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	f, _ := os.Create(cache_path + "/test")
	result := deleteFile(f.Name() + "not_exist")
	suite.assert.Equal(nil, result)
	f.Close()
}

func TestCachePolicyTestSuite(t *testing.T) {
	suite.Run(t, new(cachePolicyTestSuite))
}
