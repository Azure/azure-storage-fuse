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
	loopback          internal.Component
	blockCache        *BlockCache
}

func getFakeStoragePath() string {
	rand := randomString(8)
	fake_storage_path := filepath.Join(home_dir, "fake_storage"+rand)
	_ = os.Mkdir(fake_storage_path, 0777)
	return fake_storage_path
}

func setupPipeline(cfg string) *testObj {
	rand := randomString(8)
	fake_storage_path := filepath.Join(home_dir, "fake_storage"+rand)
	_ = os.Mkdir(fake_storage_path, 0777)

	tobj := &testObj{fake_storage_path: getFakeStoragePath()}

	if cfg == "" {
		cfg = fmt.Sprintf("read-only: true\n\nloopbackfs:\n  path: %s\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 2\n  parallelism: 8\n", tobj.fake_storage_path)
	} else {
		cfg = fmt.Sprintf("%s\n\nloopbackfs:\n  path: %s\n", cfg, tobj.fake_storage_path)
	}

	config.ReadConfigFromReader(strings.NewReader(cfg))

	tobj.loopback = loopback.NewLoopbackFSComponent()
	err := tobj.loopback.Configure(true)
	if err != nil {
		panic("Unable to configure loopback.")
	}

	tobj.blockCache = NewBlockCacheComponent().(*BlockCache)
	tobj.blockCache.SetNextComponent(tobj.loopback)
	err = tobj.blockCache.Configure(true)
	if err != nil {
		panic("Unable to configure block cache.")
	}

	err = tobj.loopback.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start loopback [%s]", err.Error()))
	}

	err = tobj.blockCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start block cache [%s]", err.Error()))
	}

	return tobj
}

func (tobj *testObj) cleanupPipeline() {
	err := tobj.loopback.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop loopback [%s]", err.Error()))
	}

	err = tobj.blockCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop block cache [%s]", err.Error()))
	}

	os.RemoveAll(tobj.fake_storage_path)
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	emptyConfig := fmt.Sprintf("read-only: true")
	tobj := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")

	suite.assert.EqualValues(tobj.blockCache.blockSizeMB, 8)
	suite.assert.EqualValues(tobj.blockCache.memSizeMB, 1024)
	suite.assert.EqualValues(tobj.blockCache.workers, 32)
	suite.assert.EqualValues(tobj.blockCache.prefetch, 8)
	suite.assert.NotNil(tobj.blockCache.blockPool)
}

// Tests the configuration of block cache
func (suite *blockCacheTestSuite) TestConfig() {
	cfg := fmt.Sprintf("read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 10\n  parallelism: 10")
	tobj := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")

	suite.assert.EqualValues(tobj.blockCache.blockSizeMB, 16)
	suite.assert.EqualValues(tobj.blockCache.memSizeMB, 500)
	suite.assert.EqualValues(tobj.blockCache.workers, 10)
	suite.assert.EqualValues(tobj.blockCache.prefetch, 10)
	suite.assert.NotNil(tobj.blockCache.blockPool)
}

// Tests CreateDir
func (suite *blockCacheTestSuite) TestOpenFileFail() {
	tobj := setupPipeline("")
	defer tobj.cleanupPipeline()

	path := "a"
	options := internal.OpenFileOptions{Name: path}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NotNil(err)
	suite.assert.Nil(h)
}

func (suite *blockCacheTestSuite) TestOpenFilePass() {
	tobj := setupPipeline("")
	defer tobj.cleanupPipeline()

	path := "b"

	f, err := tobj.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(f)

	err = tobj.loopback.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	f, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path})
	suite.assert.Nil(err)
	suite.assert.NotNil(f)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestOpenFilePreFetch() {
	tobj := setupPipeline("")
	defer tobj.cleanupPipeline()

	path := "tst1"

	buf := make([]byte, 2048)
	_, err := rand.Read(buf)
	suite.assert.Nil(err)

	err = ioutil.WriteFile(filepath.Join(tobj.fake_storage_path, path), buf, 0777)
	suite.assert.Nil(err)

	f, err := tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path})
	suite.assert.Nil(err)
	suite.assert.NotNil(f)

	item, found := f.GetValue("0")
	suite.assert.Equal(found, true)

	block := item.(workItem).block
	suite.assert.NotNil(block)
	suite.assert.Equal(block.id, uint64(0))

	item, found = f.GetValue("#")
	suite.assert.Equal(found, true)
	suite.assert.Equal(item.(uint64), uint64(_1MB))

	h, err := os.OpenFile(filepath.Join(tobj.fake_storage_path, path), os.O_RDWR, 0777)
	suite.assert.Nil(err)
	f.SetFileObject(h)

	n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: buf})
	suite.assert.NotNil(err)
	suite.assert.NotEqual(n, 0)
	suite.assert.Equal(n, 2048)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestOpenMultiblockFile() {
	tobj := setupPipeline("")
	defer tobj.cleanupPipeline()

	path := "tst2"

	buffSize := uint32(_1MB * 3)
	buf := make([]byte, buffSize)
	_, err := rand.Read(buf)
	suite.assert.Nil(err)

	err = ioutil.WriteFile(filepath.Join(tobj.fake_storage_path, path), buf, 0777)
	suite.assert.Nil(err)

	f, err := tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path})
	suite.assert.Nil(err)
	suite.assert.NotNil(f)

	item, found := f.GetValue("0")
	suite.assert.Equal(found, true)

	block := item.(workItem).block
	suite.assert.NotNil(block)
	suite.assert.Equal(block.id, uint64(0))

	_, found = f.GetValue("1")
	suite.assert.Equal(found, true)

	_, found = f.GetValue("2")
	suite.assert.Equal(found, true)

	_, found = f.GetValue("#")
	suite.assert.Equal(found, true)

	n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: buf})
	suite.assert.NotNil(err)
	suite.assert.NotEqual(n, 0)
	suite.assert.Equal(uint32(n), buffSize)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
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

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}
