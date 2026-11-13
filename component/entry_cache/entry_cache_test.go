// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package entry_cache

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type entryCacheTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	entryCache        *EntryCache
	loopback          internal.Component
	fake_storage_path string
}

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	_ = loopback.Configure(true)

	return loopback
}

func newEntryCache(next internal.Component) *EntryCache {
	entryCache := NewEntryCacheComponent()
	entryCache.SetNextComponent(next)
	err := entryCache.Configure(true)
	if err != nil {
		panic("Unable to configure entry cache.")
	}

	return entryCache.(*EntryCache)
}

func randomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *entryCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	rand := randomString(8)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf("read-only: true\n\nentry_cache:\n  timeout-sec: 7\n\nloopbackfs:\n  path: %s", suite.fake_storage_path)
	log.Debug("%s", defaultConfig)

	// Delete the temp directories created
	os.RemoveAll(suite.fake_storage_path)
	suite.setupTestHelper(defaultConfig)
}

func (suite *entryCacheTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.assert.NoError(err)
	suite.loopback = newLoopbackFS()
	suite.entryCache = newEntryCache(suite.loopback)
	err = suite.loopback.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.entryCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start file cache [%s]", err.Error()))
	}

}

func (suite *entryCacheTestSuite) cleanupTest() {
	err := suite.loopback.Stop()
	suite.assert.NoError(err)
	err = suite.entryCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	os.RemoveAll(suite.fake_storage_path)
}

func (suite *entryCacheTestSuite) TestEmpty() {
	defer suite.cleanupTest()

	objs, token, err := suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "", Token: ""})
	suite.assert.NoError(err)
	suite.assert.NotNil(objs)
	suite.assert.Empty(token)

	_, found := suite.entryCache.pathMap.Load("##")
	suite.assert.False(found)

	objs, token, err = suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "ABCD", Token: ""})
	suite.assert.Error(err)
	suite.assert.Nil(objs)
	suite.assert.Empty(token)
}

func (suite *entryCacheTestSuite) TestWithEntry() {
	defer suite.cleanupTest()

	// Create a file
	filePath := filepath.Join(suite.fake_storage_path, "testfile1")
	h, err := os.Create(filePath)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	h.Close()

	objs, token, err := suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "", Token: ""})
	suite.assert.NoError(err)
	suite.assert.NotNil(objs)
	suite.assert.Empty(token)

	cachedObjs, found := suite.entryCache.pathMap.Load("##")
	suite.assert.True(found)
	suite.assert.Len(objs, 1)

	suite.assert.Equal(objs, cachedObjs.(pathCacheItem).children)
}

func (suite *entryCacheTestSuite) TestCachedEntry() {
	defer suite.cleanupTest()

	// Create a file
	filePath := filepath.Join(suite.fake_storage_path, "testfile1")
	h, err := os.Create(filePath)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	h.Close()

	objs, token, err := suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "", Token: ""})
	suite.assert.NoError(err)
	suite.assert.NotNil(objs)
	suite.assert.Empty(token)

	cachedObjs, found := suite.entryCache.pathMap.Load("##")
	suite.assert.True(found)
	suite.assert.Len(objs, 1)

	suite.assert.Equal(objs, cachedObjs.(pathCacheItem).children)

	filePath = filepath.Join(suite.fake_storage_path, "testfile2")
	h, err = os.Create(filePath)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	h.Close()

	objs, token, err = suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "", Token: ""})
	suite.assert.NoError(err)
	suite.assert.NotNil(objs)
	suite.assert.Empty(token)
	suite.assert.Len(objs, 1)

	time.Sleep(40 * time.Second)
	_, found = suite.entryCache.pathMap.Load("##")
	suite.assert.False(found)

	objs, token, err = suite.entryCache.StreamDir(internal.StreamDirOptions{Name: "", Token: ""})
	suite.assert.NoError(err)
	suite.assert.NotNil(objs)
	suite.assert.Empty(token)
	suite.assert.Len(objs, 2)

}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestEntryCacheTestSuite(t *testing.T) {
	suite.Run(t, new(entryCacheTestSuite))
}
