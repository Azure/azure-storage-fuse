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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type lruPolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
	policy *lruPolicy
}

var cache_path = filepath.Join(home_dir, "file_cache")

func (suite *lruPolicyTestSuite) SetupTest() {
	// err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	// if err != nil {
	// 	panic("Unable to set silent logger as default.")
	// }
	suite.assert = assert.New(suite.T())

	os.Mkdir(cache_path, fs.FileMode(0777))

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  0,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)
}

func (suite *lruPolicyTestSuite) setupTestHelper(config cachePolicyConfig) {
	suite.policy = NewLRUPolicy(config).(*lruPolicy)

	suite.policy.StartPolicy()
}

func (suite *lruPolicyTestSuite) cleanupTest() {
	suite.policy.ShutdownPolicy()

	os.RemoveAll(cache_path)
}

func (suite *lruPolicyTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.EqualValues("lru", suite.policy.Name())
	suite.assert.EqualValues(0, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(defaultMaxEviction, suite.policy.maxEviction)
	suite.assert.EqualValues(0, suite.policy.maxSizeMB)
	suite.assert.EqualValues(defaultMaxThreshold, suite.policy.highThreshold)
	suite.assert.EqualValues(defaultMinThreshold, suite.policy.lowThreshold)
}

func (suite *lruPolicyTestSuite) TestUpdateConfig() {
	defer suite.cleanupTest()
	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  120,
		maxEviction:   100,
		maxSizeMB:     10,
		highThreshold: 70,
		lowThreshold:  20,
		fileLocks:     &common.LockMap{},
	}
	suite.policy.UpdateConfig(config)

	suite.assert.NotEqualValues(120, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(0, suite.policy.cacheTimeout)      // cacheTimeout does not change
	suite.assert.EqualValues(100, suite.policy.maxEviction)
	suite.assert.EqualValues(10, suite.policy.maxSizeMB)
	suite.assert.EqualValues(70, suite.policy.highThreshold)
	suite.assert.EqualValues(20, suite.policy.lowThreshold)
}

func (suite *lruPolicyTestSuite) TestCacheValid() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.True(ok)
	suite.assert.NotNil(n)
	node := n.(*lruNode)
	suite.assert.EqualValues("temp", node.name)
	suite.assert.EqualValues(1, node.usage)
}

func (suite *lruPolicyTestSuite) TestCacheInvalidate() {
	defer suite.cleanupTest()
	f, _ := os.Create(cache_path + "/temp")
	f.Close()
	suite.policy.CacheValid("temp")
	suite.policy.CacheInvalidate("temp") // this is equivalent to purge since timeout=0

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.False(ok)
	suite.assert.Nil(n)
}

func (suite *lruPolicyTestSuite) TestCacheInvalidateTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)

	suite.policy.CacheValid("temp")
	suite.policy.CacheInvalidate("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.True(ok)
	suite.assert.NotNil(n)
	node := n.(*lruNode)
	suite.assert.EqualValues("temp", node.name)
	suite.assert.EqualValues(1, node.usage)
}

func (suite *lruPolicyTestSuite) TestCachePurge() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")
	suite.policy.CachePurge("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.False(ok)
	suite.assert.Nil(n)
}

func (suite *lruPolicyTestSuite) TestIsCached() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")

	suite.assert.True(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestIsCachedFalse() {
	defer suite.cleanupTest()
	suite.assert.False(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)

	suite.policy.CacheValid("temp")

	time.Sleep(5 * time.Second) // Wait for time > cacheTimeout, the file should no longer be cached

	suite.assert.False(suite.policy.IsCached("temp"))
}

func (suite *lruPolicyTestSuite) TestMaxEvictionDefault() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   defaultMaxEviction,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)

	for i := 1; i < 5000; i++ {
		suite.policy.CacheValid("temp" + fmt.Sprint(i))
	}

	time.Sleep(5 * time.Second) // Wait for time > cacheTimeout, the file should no longer be cached

	for i := 1; i < 5000; i++ {
		suite.assert.False(suite.policy.IsCached("temp" + fmt.Sprint(i)))
	}
}

func (suite *lruPolicyTestSuite) TestMaxEviction() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := cachePolicyConfig{
		tmpPath:       cache_path,
		cacheTimeout:  1,
		maxEviction:   5,
		maxSizeMB:     0,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	}

	suite.setupTestHelper(config)

	for i := 1; i < 5; i++ {
		suite.policy.CacheValid("temp" + fmt.Sprint(i))
	}

	time.Sleep(5 * time.Second) // Wait for time > cacheTimeout, the file should no longer be cached

	for i := 1; i < 5; i++ {
		suite.assert.False(suite.policy.IsCached("temp" + fmt.Sprint(i)))
	}
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
