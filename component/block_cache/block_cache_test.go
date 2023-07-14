/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package block_cache

import (
	"context"
	"fmt"
	"io/ioutil"
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

type blockCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockCacheTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

type testObj struct {
	fake_storage_path string
	disk_cache_path   string
	loopback          internal.Component
	blockCache        *BlockCache
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func getFakeStoragePath() string {
	rand := randomString(8)
	fake_storage_path := filepath.Join(home_dir, "fake_storage"+rand)
	_ = os.Mkdir(fake_storage_path, 0777)
	return fake_storage_path
}

func setupPipeline(cfg string) (*testObj, error) {
	rand := randomString(8)
	fake_storage_path := filepath.Join(home_dir, "fake_storage"+rand)
	_ = os.Mkdir(fake_storage_path, 0777)

	tobj := &testObj{
		fake_storage_path: getFakeStoragePath(),
		disk_cache_path:   getFakeStoragePath(),
	}

	if cfg == "" {
		cfg = fmt.Sprintf("read-only: true\n\nloopbackfs:\n  path: %s\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10\n  path: %s\n  disk-size-mb: 50\n  disk-timeout-sec: 20", tobj.fake_storage_path, tobj.disk_cache_path)
	} else {
		cfg = fmt.Sprintf("%s\n\nloopbackfs:\n  path: %s\n", cfg, tobj.fake_storage_path)
	}

	config.ReadConfigFromReader(strings.NewReader(cfg))

	tobj.loopback = loopback.NewLoopbackFSComponent()
	err := tobj.loopback.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure loopback [%s]", err.Error())
	}

	tobj.blockCache = NewBlockCacheComponent().(*BlockCache)
	tobj.blockCache.SetNextComponent(tobj.loopback)
	err = tobj.blockCache.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure blockcache [%s]", err.Error())
	}

	err = tobj.loopback.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start loopback [%s]", err.Error())
	}

	err = tobj.blockCache.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start blockcache [%s]", err.Error())
	}

	return tobj, nil
}

func (tobj *testObj) cleanupPipeline() error {
	if tobj == nil {
		return nil
	}

	if tobj.loopback != nil {
		err := tobj.loopback.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop loopback [%s]", err.Error())
		}
	}

	if tobj.blockCache != nil {
		err := tobj.blockCache.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop block cache [%s]", err.Error())
		}
	}

	os.RemoveAll(tobj.fake_storage_path)
	os.RemoveAll(tobj.disk_cache_path)

	return nil
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	emptyConfig := "read-only: true"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")
	suite.assert.EqualValues(tobj.blockCache.blockSize, 16*_1MB)
	suite.assert.EqualValues(tobj.blockCache.memSize, 4192*_1MB)
	suite.assert.EqualValues(tobj.blockCache.diskSize, 4192*_1MB)
	suite.assert.EqualValues(tobj.blockCache.diskTimeout, defaultTimeout)
	suite.assert.EqualValues(tobj.blockCache.workers, 128)
	suite.assert.EqualValues(tobj.blockCache.prefetch, MIN_PREFETCH)
	suite.assert.EqualValues(tobj.blockCache.noPrefetch, false)
	suite.assert.NotNil(tobj.blockCache.blockPool)
	suite.assert.NotNil(tobj.blockCache.threadPool)
}

func (suite *blockCacheTestSuite) TestNonROMount() {
	emptyConfig := "read-only: false"
	tobj, err := setupPipeline(emptyConfig)

	suite.assert.NotNil(err)
	suite.assert.Nil(tobj)
	suite.assert.Contains(err.Error(), "filesystem is not mounted in read-only mode")
}

func (suite *blockCacheTestSuite) TestInvalidPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 8\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid config for prefetch count")
}

func (suite *blockCacheTestSuite) TestNoPrefetchConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 500\n  prefetch: 0\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)
	suite.assert.Equal(tobj.blockCache.noPrefetch, true)
}

func (suite *blockCacheTestSuite) TestInvalidDiskPath() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: /abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "permission denied")
}

func (suite *blockCacheTestSuite) TestManualConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")
	suite.assert.EqualValues(tobj.blockCache.blockSize, 16*_1MB)
	suite.assert.EqualValues(tobj.blockCache.memSize, 500*_1MB)
	suite.assert.EqualValues(tobj.blockCache.workers, 10)
	suite.assert.EqualValues(tobj.blockCache.diskSize, 100*_1MB)
	suite.assert.EqualValues(tobj.blockCache.diskTimeout, 5)
	suite.assert.EqualValues(tobj.blockCache.prefetch, 12)
	suite.assert.EqualValues(tobj.blockCache.workers, 10)

	suite.assert.NotNil(tobj.blockCache.blockPool)
}

func (suite *blockCacheTestSuite) TestOpenFileFail() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "a"
	options := internal.OpenFileOptions{Name: path}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NotNil(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "no such file or directory")
}

func (suite *blockCacheTestSuite) TestFileOpneClose() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheTestSuite(t *testing.T) {
	bcsuite := new(blockCacheTestSuite)
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}

	suite.Run(t, bcsuite)
}
