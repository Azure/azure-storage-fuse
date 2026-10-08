/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

	err := os.Mkdir(cache_path, fs.FileMode(0777))
	suite.assert.NoError(err)

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

	err := suite.policy.StartPolicy()
	suite.assert.NoError(err)
}

func (suite *lruPolicyTestSuite) cleanupTest() {
	err := suite.policy.ShutdownPolicy()
	suite.assert.NoError(err)

	os.RemoveAll(cache_path)
}

func (suite *lruPolicyTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("lru", suite.policy.Name())
	suite.assert.EqualValues(0, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(defaultMaxEviction, suite.policy.maxEviction)
	suite.assert.Equal(0, int(suite.policy.maxSizeMB))
	suite.assert.Equal(defaultMaxThreshold, int(suite.policy.highThreshold))
	suite.assert.Equal(defaultMinThreshold, int(suite.policy.lowThreshold))
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
	err := suite.policy.UpdateConfig(config)
	suite.assert.NoError(err)

	suite.assert.NotEqualValues(120, suite.policy.cacheTimeout) // cacheTimeout does not change
	suite.assert.EqualValues(0, suite.policy.cacheTimeout)      // cacheTimeout does not change
	suite.assert.EqualValues(100, suite.policy.maxEviction)
	suite.assert.Equal(10, int(suite.policy.maxSizeMB))
	suite.assert.Equal(70, int(suite.policy.highThreshold))
	suite.assert.Equal(20, int(suite.policy.lowThreshold))
}

func (suite *lruPolicyTestSuite) TestCacheValid() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.True(ok)
	suite.assert.NotNil(n)
	node := n.(*lruNode)
	suite.assert.Equal("temp", node.name)
	suite.assert.Equal(1, node.usage)
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
	suite.assert.Equal("temp", node.name)
	suite.assert.Equal(1, node.usage)
}

func (suite *lruPolicyTestSuite) TestCachePurge() {
	defer suite.cleanupTest()
	suite.policy.CacheValid("temp")
	suite.policy.CachePurge("temp")

	n, ok := suite.policy.nodeMap.Load("temp")
	suite.assert.False(ok)
	suite.assert.Nil(n)
}

func (suite *lruPolicyTestSuite) TestPurgeRetriesBusyFile() {
	defer suite.cleanupTest()

	name := filepath.Join(cache_path, "busy")
	suite.Require().NoError(os.WriteFile(name, []byte("content"), 0600))

	flock := suite.policy.fileLocks.Get("busy")
	flock.Lock()
	suite.False(suite.policy.deleteItem(name), "deleteItem must not wait for a busy file lock")

	suite.policy.CachePurge(name)
	suite.Never(func() bool {
		_, err := os.Stat(name)
		return os.IsNotExist(err)
	}, 300*time.Millisecond, 10*time.Millisecond)

	flock.Unlock()
	suite.Eventually(func() bool {
		_, err := os.Stat(name)
		return os.IsNotExist(err)
	}, 5*time.Second, 10*time.Millisecond)
}

// A busy file must wait for the retry timer instead of being retried on every new cleanup request.
func (suite *lruPolicyTestSuite) TestNewPurgesDoNotRetryBusyFile() {
	interval := purgeRetryInterval
	defer func() { purgeRetryInterval = interval }()
	defer suite.cleanupTest()
	suite.cleanupTest()

	purgeRetryInterval = time.Hour
	suite.setupTestHelper(cachePolicyConfig{
		tmpPath:       cache_path,
		maxEviction:   defaultMaxEviction,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	})
	suite.Require().NoError(os.MkdirAll(cache_path, 0777))

	purgeAndWait := func(name string) {
		path := filepath.Join(cache_path, name)
		suite.Require().NoError(os.WriteFile(path, []byte("content"), 0600))
		suite.policy.CachePurge(path)
		suite.Require().Eventually(func() bool {
			_, err := os.Stat(path)
			return os.IsNotExist(err)
		}, 5*time.Second, 10*time.Millisecond)
	}

	busy := filepath.Join(cache_path, "busy")
	suite.Require().NoError(os.WriteFile(busy, []byte("content"), 0600))
	flock := suite.policy.fileLocks.Get("busy")
	flock.Lock()
	suite.policy.CachePurge(busy)
	// The worker handles one batch at a time, so once two later requests are done it has tried the busy file.
	purgeAndWait("first")
	purgeAndWait("second")
	flock.Unlock()

	purgeAndWait("third")
	_, err := os.Stat(busy)
	suite.NoError(err, "a new cleanup request retried the busy file before its retry timer")
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

// When an expiry pass stops at max-eviction, the files it leaves behind must stay correctly linked,
// including after one of them is used again or purged.
func (suite *lruPolicyTestSuite) TestPartialEvictionKeepsListLinked() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	suite.setupTestHelper(cachePolicyConfig{
		tmpPath:       cache_path,
		maxEviction:   2,
		highThreshold: defaultMaxThreshold,
		lowThreshold:  defaultMinThreshold,
		fileLocks:     &common.LockMap{},
	})
	p := suite.policy
	path := func(name string) string { return filepath.Join(cache_path, name) }

	for _, name := range []string{"a", "b", "c", "d"} {
		p.CacheValid(path(name))
	}
	// Expire all four files, then evict only the two newest because max-eviction is 2.
	p.updateMarker()
	p.updateMarker()
	p.deleteExpiredNodes()
	suite.Equal([]string{"__", "##", path("b"), path("a")}, suite.listNodes())

	p.cacheValidate(path("b"))
	suite.Equal([]string{path("b"), "__", "##", path("a")}, suite.listNodes())

	p.removeNode(path("a"))
	suite.Equal([]string{path("b"), "__", "##"}, suite.listNodes())
}

// listNodes returns the names in the LRU list, failing on a loop or a broken back link.
func (suite *lruPolicyTestSuite) listNodes() []string {
	var names []string
	var prev *lruNode
	seen := make(map[*lruNode]bool)
	for node := suite.policy.head; node != nil; node = node.next {
		suite.Require().False(seen[node], "LRU list loops back to %s", node.name)
		suite.Require().Same(prev, node.prev, "wrong back link at %s", node.name)
		seen[node] = true
		names = append(names, node.name)
		prev = node
	}
	return names
}

func TestLRUPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(lruPolicyTestSuite))
}
