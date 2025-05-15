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

package xload

import (
	"context"
	"fmt"
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

type xloadTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	xload             *Xload
	loopback          internal.Component
	local_path        string
	fake_storage_path string
}

// var home_dir, _ = os.UserHomeDir()

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	loopback.Configure(true)

	return loopback
}

func newTestXload(next internal.Component) (*Xload, error) {
	xload := NewXloadComponent()
	xload.SetNextComponent(next)
	err := xload.Configure(true)
	if err != nil {
		return nil, err
	}

	return xload.(*Xload), nil
}

func (suite *xloadTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	rand := randomString(8)
	suite.local_path = filepath.Join("/tmp/", "xload_"+rand)
	suite.fake_storage_path = filepath.Join("/tmp/", "fake_storage_"+rand)
	defaultConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, suite.fake_storage_path)
	log.Debug(defaultConfig)

	// Delete the temp directories created
	os.RemoveAll(suite.local_path)
	os.RemoveAll(suite.fake_storage_path)
	err = suite.setupTestHelper(defaultConfig, false)
	suite.assert.Nil(err)
}

func (suite *xloadTestSuite) setupTestHelper(configuration string, startComponents bool) error {
	suite.assert = assert.New(suite.T())

	var err error
	config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.loopback = newLoopbackFS()
	suite.xload, err = newTestXload(suite.loopback)
	if err != nil {
		return err
	}

	if startComponents {
		suite.loopback.Start(context.Background())
		err := suite.xload.Start(context.Background())
		if err != nil {
			return err
		}
	}

	return nil
}

func (suite *xloadTestSuite) cleanupTest(stopComp bool) {
	config.ResetConfig()
	if stopComp {
		suite.loopback.Stop()
		err := suite.xload.Stop()
		if err != nil {
			suite.assert.Nil(err)
		}
	}

	// Delete the temp directories created
	os.RemoveAll(suite.local_path)
	os.RemoveAll(suite.fake_storage_path)
}

func (suite *xloadTestSuite) TestConfigEmpty() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated
	emptyConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, suite.fake_storage_path)
	err := suite.setupTestHelper(emptyConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)

	suite.assert.Equal(suite.xload.Name(), compName)
	suite.assert.Equal(suite.xload.path, suite.local_path)
	suite.assert.Equal(suite.xload.blockSize, uint64(defaultBlockSize*float64(MB)))
	suite.assert.Equal(suite.xload.mode, EMode.PRELOAD())
	suite.assert.Equal(suite.xload.exportProgress, false)
	suite.assert.Equal(suite.xload.defaultPermission, common.DefaultFilePermissionBits)
	suite.assert.NotEqual(suite.xload.workerCount, uint32(0))
	suite.assert.Nil(suite.xload.blockPool)
	suite.assert.Nil(suite.xload.statsMgr)
	suite.assert.NotNil(suite.xload.fileLocks)
	suite.assert.Len(suite.xload.comps, 0)
}

func (suite *xloadTestSuite) TestConfigNotReadOnly() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated
	testConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s", suite.local_path, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "should be used in only in read-only mode")
}

func (suite *xloadTestSuite) TestConfigBlockSize() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	blockSize := (float64)(5.5)
	testConfig := fmt.Sprintf("xload:\n  path: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)
	suite.assert.Equal(suite.xload.path, suite.local_path)
	suite.assert.Equal(suite.xload.blockSize, uint64(blockSize*float64(MB)))
}

func (suite *xloadTestSuite) TestConfigBlockSizeInCLI() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	blockSize := 4.8
	testConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, suite.fake_storage_path)
	err := config.ReadConfigFromReader(strings.NewReader(testConfig))
	suite.assert.Nil(err)

	config.Set("stream.block-size-mb", fmt.Sprintf("%v", blockSize))

	xload := (NewXloadComponent()).(*Xload)
	err = xload.Configure(true)
	suite.assert.Nil(err)
	suite.assert.Equal(xload.path, suite.local_path)
	suite.assert.Equal(xload.blockSize, uint64(blockSize*float64(MB)))
}

func (suite *xloadTestSuite) TestConfigNoPath() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	blockSize := (float64)(5.5)
	testConfig := fmt.Sprintf("xload:\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "path not given")
}

func (suite *xloadTestSuite) TestConfigPathInCLI() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	blockSize := (float64)(4)
	testConfig := fmt.Sprintf("xload:\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", blockSize, suite.fake_storage_path)
	err := config.ReadConfigFromReader(strings.NewReader(testConfig))
	suite.assert.Nil(err)

	config.Set("file_cache.path", suite.local_path)

	xload := (NewXloadComponent()).(*Xload)
	err = xload.Configure(true)
	suite.assert.Nil(err)
	suite.assert.Equal(xload.path, suite.local_path)
	suite.assert.Equal(xload.blockSize, uint64(blockSize*float64(MB)))
}

func (suite *xloadTestSuite) TestConfigPathSameAsMountPath() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	testConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, suite.fake_storage_path)
	err := config.ReadConfigFromReader(strings.NewReader(testConfig))
	suite.assert.Nil(err)

	config.Set("mount-path", suite.local_path)

	xload := (NewXloadComponent()).(*Xload)
	err = xload.Configure(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "xload path is same as mount path")
}

func (suite *xloadTestSuite) TestConfigPathNotEmpty() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	// create file in local path
	err := os.Mkdir(suite.local_path, 0755)
	suite.assert.Nil(err)
	_, err = os.Create(filepath.Join(suite.local_path, "testFile"))
	suite.assert.Nil(err)

	testConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, suite.fake_storage_path)
	err = suite.setupTestHelper(testConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "temp directory not empty")
}

func (suite *xloadTestSuite) TestConfigMode() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	modes := []struct {
		val  string
		mode Mode
	}{
		{val: "preload", mode: EMode.PRELOAD()},
		{val: "upload", mode: EMode.UPLOAD()},
		{val: "sync", mode: EMode.SYNC()},
		{val: "PRELOAD", mode: EMode.PRELOAD()},
		{val: "UpLoad", mode: EMode.UPLOAD()},
		{val: "sYNC", mode: EMode.SYNC()},
		{val: "invalid_mode", mode: EMode.INVALID_MODE()},
		{val: "Invalid_MODE", mode: EMode.INVALID_MODE()},
		{val: "invalid", mode: EMode.INVALID_MODE()},
		{val: "RANDOM", mode: EMode.INVALID_MODE()},
	}

	for i, m := range modes {
		testConfig := fmt.Sprintf("xload:\n  path: %s\n  mode: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, m.val, suite.fake_storage_path)
		err := suite.setupTestHelper(testConfig, false)
		if i < len(modes)-4 {
			suite.assert.Nil(err)
			suite.assert.Equal(suite.xload.path, suite.local_path)
			suite.assert.Equal(suite.xload.mode, m.mode)
		} else {
			suite.assert.NotNil(err)
		}
	}
}

func (suite *xloadTestSuite) TestConfigAllowOther() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	testConfig := fmt.Sprintf("xload:\n  path: %s\n\nloopbackfs:\n  path: %s\n\nread-only: true\n\nallow-other: true", suite.local_path, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, false) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)
	suite.assert.Equal(suite.xload.defaultPermission, common.DefaultAllowOtherPermissionBits)
}

func (suite *xloadTestSuite) TestUnsupportedModes() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	modes := []string{"upload", "sync", "invalid_mode"}
	blockSize := float64(0.001)
	for _, m := range modes {
		testConfig := fmt.Sprintf("xload:\n  path: %s\n  mode: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, m, blockSize, suite.fake_storage_path)
		err := suite.setupTestHelper(testConfig, true)
		suite.assert.NotNil(err)
	}

	testConfig := fmt.Sprintf("xload:\n  path: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, false)
	suite.assert.Nil(err)

	suite.xload.mode = EMode.INVALID_MODE()
	err = suite.xload.Start(context.Background())
	suite.assert.NotNil(err)
}

func (suite *xloadTestSuite) TestPriority() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{}
	suite.assert.Equal(xl.Priority(), internal.EComponentPriority.LevelMid())
}

func (suite *xloadTestSuite) TestBlockPoolError() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{}
	err := xl.Start(context.Background())
	suite.assert.NotNil(err)
}

func (suite *xloadTestSuite) TestXComponentDefault() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	type testCmp struct {
		XBase
	}

	t := &testCmp{}

	t.Schedule(nil)

	n, err := t.Process(nil)
	suite.assert.Nil(err)
	suite.assert.Equal(n, 0)
}

func (suite *xloadTestSuite) TestCreateDownloader() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{}
	err := xl.createDownloader()
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")
	suite.assert.Len(xl.comps, 0)

	xl.path = suite.local_path
	xl.workerCount = 4
	xl.SetNextComponent(xl)
	xl.statsMgr = &StatsManager{}
	err = xl.createDownloader()
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create download splitter")
	suite.assert.Len(xl.comps, 0)

	xl.blockPool = &BlockPool{}
	xl.fileLocks = common.NewLockMap()
	err = xl.createDownloader()
	suite.assert.Nil(err)
	suite.assert.Len(xl.comps, 3)
}

func (suite *xloadTestSuite) TestCreateChain() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{
		path:        suite.local_path,
		statsMgr:    &StatsManager{},
		blockPool:   &BlockPool{},
		fileLocks:   common.NewLockMap(),
		workerCount: 4,
	}
	xl.SetNextComponent(xl)

	err := xl.createChain()
	suite.assert.NotNil(err)

	err = xl.startComponents()
	suite.assert.NotNil(err)

	err = xl.createDownloader()
	suite.assert.Nil(err)
	suite.assert.Len(xl.comps, 3)

	err = xl.createChain()
	suite.assert.Nil(err)
	suite.assert.NotNil(xl.comps[0].GetNext())
	suite.assert.NotNil(xl.comps[1].GetNext())
	suite.assert.Nil(xl.comps[2].GetNext())
}

func (suite *xloadTestSuite) TestDownloadFileError() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{}
	err := xl.downloadFile("file0")
	suite.assert.NotNil(err)
}

func (suite *xloadTestSuite) TestDownloadFileGetAttrError() {
	defer suite.cleanupTest(false)
	suite.cleanupTest(false) // teardown the default xload generated

	xl := &Xload{
		path:        suite.local_path,
		statsMgr:    &StatsManager{},
		blockPool:   &BlockPool{},
		fileLocks:   common.NewLockMap(),
		workerCount: 4,
	}

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", suite.fake_storage_path)
	config.ReadConfigFromReader(strings.NewReader(cfg))
	loopback := newLoopbackFS()

	xl.SetNextComponent(loopback)

	err := xl.createDownloader()
	suite.assert.Nil(err)
	suite.assert.Len(xl.comps, 3)

	err = xl.createChain()
	suite.assert.Nil(err)

	err = xl.downloadFile("file0")
	suite.assert.NotNil(err)
}

func (suite *xloadTestSuite) TestXloadStartStop() {
	defer suite.cleanupTest(true)
	config.ResetConfig()

	createTestDirsAndFiles(suite.fake_storage_path, suite.assert)

	blockSize := (float64)(0.00001)
	testConfig := fmt.Sprintf("xload:\n  path: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, true) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)
	suite.assert.Equal(suite.xload.path, suite.local_path)
	suite.assert.Equal(suite.xload.blockSize, uint64(blockSize*float64(MB)))

	time.Sleep(5 * time.Second)

	validateMD5(suite.local_path, suite.fake_storage_path, suite.assert)
}

func (suite *xloadTestSuite) TestOpenFileAlreadyDownloaded() {
	defer suite.cleanupTest(true)
	config.ResetConfig()

	createTestDirsAndFiles(suite.fake_storage_path, suite.assert)

	blockSize := (float64)(0.00001)
	testConfig := fmt.Sprintf("xload:\n  path: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, true) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)
	suite.assert.Equal(suite.xload.path, suite.local_path)
	suite.assert.Equal(suite.xload.blockSize, uint64(blockSize*float64(MB)))

	time.Sleep(5 * time.Second)

	fh, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: "file_4"})
	suite.assert.Nil(err)
	suite.assert.NotNil(fh)
	suite.assert.Equal(fh.Size, (int64)(36))

	err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh})
	suite.assert.Nil(err)

	fh2, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: "dir_0/file_3"})
	suite.assert.Nil(err)
	suite.assert.NotNil(fh2)
	suite.assert.Equal(fh2.Size, (int64)(27))

	err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh2})
	suite.assert.Nil(err)

	validateMD5(suite.local_path, suite.fake_storage_path, suite.assert)
}

func (suite *xloadTestSuite) TestOpenFileWithDownload() {
	defer suite.cleanupTest(true)
	config.ResetConfig()

	blockSize := (float64)(0.00001)
	testConfig := fmt.Sprintf("xload:\n  path: %s\n  block-size-mb: %v\n\nloopbackfs:\n  path: %s\n\nread-only: true", suite.local_path, blockSize, suite.fake_storage_path)
	err := suite.setupTestHelper(testConfig, true) // setup a new xload with a custom config (teardown will occur after the test as usual)
	suite.assert.Nil(err)
	suite.assert.Equal(suite.xload.path, suite.local_path)
	suite.assert.Equal(suite.xload.blockSize, uint64(blockSize*float64(MB)))

	time.Sleep(5 * time.Second)

	createTestDirsAndFiles(suite.fake_storage_path, suite.assert)

	// open file error
	fh, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: "dir_1/file_0"})
	suite.assert.Nil(err)
	suite.assert.NotNil(fh)
	suite.assert.Equal(fh.Size, (int64)(0))

	err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh})
	suite.assert.Nil(err)

	fh1, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: "file_4", Flags: os.O_RDONLY, Mode: common.DefaultFilePermissionBits})
	suite.assert.Nil(err)
	suite.assert.NotNil(fh1)
	suite.assert.Equal(fh1.Size, (int64)(36))

	err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh1})
	suite.assert.Nil(err)

	fh2, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: "dir_0/file_3", Flags: os.O_RDONLY, Mode: common.DefaultFilePermissionBits})
	suite.assert.Nil(err)
	suite.assert.NotNil(fh2)
	suite.assert.Equal(fh2.Size, (int64)(27))

	err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh2})
	suite.assert.Nil(err)

	suite.validateMD5WithOpenFile(suite.local_path, suite.fake_storage_path)
}

func (suite *xloadTestSuite) validateMD5WithOpenFile(localPath string, remotePath string) {
	entries, err := os.ReadDir(remotePath)
	suite.assert.Nil(err)

	for _, entry := range entries {
		localFilePath := filepath.Join(localPath, entry.Name())
		remoteFilePath := filepath.Join(remotePath, entry.Name())

		if entry.IsDir() {
			suite.validateMD5WithOpenFile(localFilePath, remoteFilePath)
		} else {
			relPath := strings.TrimPrefix(localFilePath, suite.local_path+"/")
			fh, err := suite.xload.OpenFile(internal.OpenFileOptions{Name: relPath, Flags: os.O_RDONLY, Mode: common.DefaultFilePermissionBits})
			suite.assert.Nil(err)
			suite.assert.NotNil(fh)

			localMD5, err := computeMD5(localFilePath)
			suite.assert.Nil(err)

			remoteMD5, err := computeMD5(remoteFilePath)
			suite.assert.Nil(err)

			suite.assert.Equal(localMD5, remoteMD5)

			err = suite.xload.CloseFile(internal.CloseFileOptions{Handle: fh})
			suite.assert.Nil(err)
		}
	}
}

func TestXloadTestSuite(t *testing.T) {
	suite.Run(t, new(xloadTestSuite))
}
