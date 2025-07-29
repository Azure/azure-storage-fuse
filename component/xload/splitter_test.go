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
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
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

type splitterTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

var remote internal.Component
var remote_path string

func (suite *splitterTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.Nil(err)

	remote_path = filepath.Join("/tmp/", "xload_"+randomString(8))
	err = os.MkdirAll(remote_path, 0777)
	suite.assert.Nil(err)

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", remote_path)
	config.ReadConfigFromReader(strings.NewReader(cfg))

	remote = loopback.NewLoopbackFSComponent()
	err = remote.Configure(true)
	suite.assert.Nil(err)

	createTestDirsAndFiles(remote_path, suite.assert)
}

func (suite *splitterTestSuite) TearDownSuite() {
	err := os.RemoveAll(remote_path)
	suite.assert.Nil(err)
}

func createTestDirsAndFiles(path string, assert *assert.Assertions) {
	createTestFiles(path, assert)

	for i := 0; i < 2; i++ {
		dirName := filepath.Join(path, fmt.Sprintf("dir_%v", i))
		err := os.MkdirAll(dirName, 0777)
		assert.Nil(err)

		createTestFiles(dirName, assert)
	}
}

func createTestFiles(path string, assert *assert.Assertions) {
	for i := 0; i < 5; i++ {
		filePath := filepath.Join(path, fmt.Sprintf("file_%v", i))
		f, err := os.Create(filePath)
		defer func() {
			err = f.Close()
			assert.Nil(err)
		}()
		assert.Nil(err)

		n, err := f.Write([]byte(randomString(9 * i)))
		assert.Nil(err)
		assert.Equal(n, 9*i)

		err = os.Truncate(filePath, int64(9*i))
		assert.Nil(err)
	}
}

type testSplitter struct {
	path      string
	blockSize uint64
	blockPool *BlockPool
	locks     *common.LockMap
	stMgr     *StatsManager
}

func setupTestSplitter() (*testSplitter, error) {
	ts := &testSplitter{}
	ts.path = filepath.Join("/tmp/", fmt.Sprintf("xsplitter_%v", randomString(8)))
	err := os.MkdirAll(ts.path, 0777)
	if err != nil {
		return nil, err
	}

	ts.blockSize = 10
	ts.blockPool = NewBlockPool(ts.blockSize, 10, context.TODO())
	ts.locks = common.NewLockMap()

	ts.stMgr, err = NewStatsManager(10, false, nil)
	if err != nil {
		return nil, err
	}

	ts.stMgr.Start()
	return ts, nil
}

func (ts *testSplitter) cleanup() error {
	ts.stMgr.Stop()
	ts.blockPool.Terminate()

	err := os.RemoveAll(ts.path)
	return err
}

func (suite *splitterTestSuite) TestNewDownloadSplitter() {
	ds, err := newDownloadSplitter(nil)
	suite.assert.NotNil(err)
	suite.assert.Nil(ds)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create download splitter")

	ds, err = newDownloadSplitter(&downloadSplitterOptions{})
	suite.assert.NotNil(err)
	suite.assert.Nil(ds)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create download splitter")

	statsMgr, err := NewStatsManager(1, false, nil)
	suite.assert.Nil(err)
	suite.assert.NotNil(statsMgr)

	ds, err = newDownloadSplitter(&downloadSplitterOptions{
		blockPool:   NewBlockPool(1, 1, context.TODO()),
		path:        "/home/user/random_path",
		workerCount: 4,
		remote:      remote,
		statsMgr:    statsMgr,
		fileLocks:   common.NewLockMap(),
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)
}

func (suite *splitterTestSuite) TestProcessFilePresent() {
	ts, err := setupTestSplitter()
	suite.assert.Nil(err)
	suite.assert.NotNil(ts)

	defer func() {
		err = ts.cleanup()
		suite.assert.Nil(err)
	}()

	ds, err := newDownloadSplitter(&downloadSplitterOptions{ts.blockPool, ts.path, 4, remote, ts.stMgr, ts.locks, false})
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)

	n, err := ds.Process(&WorkItem{})
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "is a directory")
	suite.assert.Equal(n, -1)

	fileName := "file_4"
	cpCmd := exec.Command("cp", filepath.Join(remote_path, fileName), ts.path)
	_, err = cpCmd.Output()
	suite.assert.Nil(err)

	n, err = ds.Process(&WorkItem{Path: fileName, DataLen: uint64(36)})
	suite.assert.Nil(err)
	suite.assert.Equal(n, 36)
}

func (suite *splitterTestSuite) TestSplitterStartStop() {
	ts, err := setupTestSplitter()
	suite.assert.Nil(err)
	suite.assert.NotNil(ts)

	defer func() {
		err = ts.cleanup()
		suite.assert.Nil(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		path:              ts.path,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            remote,
		statsMgr:          ts.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)

	ds, err := newDownloadSplitter(&downloadSplitterOptions{ts.blockPool, ts.path, 4, remote, ts.stMgr, ts.locks, true})
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)

	rdm, err := newRemoteDataManager(&remoteDataManagerOptions{
		workerCount: 8,
		remote:      remote,
		statsMgr:    ts.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rdm)

	// create chain
	rl.SetNext(ds)
	ds.SetNext(rdm)

	// start components
	rdm.Start(context.TODO())
	ds.Start(context.TODO())
	rl.Start(context.TODO())

	time.Sleep(5 * time.Second)

	// stop comoponents
	rl.Stop()

	validateMD5(ts.path, remote_path, suite.assert)
}

func (suite *splitterTestSuite) TestSplitterConsistency() {
	ts, err := setupTestSplitter()
	suite.assert.Nil(err)
	suite.assert.NotNil(ts)

	remote.(*loopback.LoopbackFS).SetConsistency(true)

	defer func() {
		remote.(*loopback.LoopbackFS).SetConsistency(false)
		err = ts.cleanup()
		suite.assert.Nil(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		path:              ts.path,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            remote,
		statsMgr:          ts.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)

	ds, err := newDownloadSplitter(&downloadSplitterOptions{ts.blockPool, ts.path, 4, remote, ts.stMgr, ts.locks, true})
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)

	rdm, err := newRemoteDataManager(&remoteDataManagerOptions{
		workerCount: 8,
		remote:      remote,
		statsMgr:    ts.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rdm)

	// create chain
	rl.SetNext(ds)
	ds.SetNext(rdm)

	// start components
	rdm.Start(context.TODO())
	ds.Start(context.TODO())
	rl.Start(context.TODO())

	time.Sleep(5 * time.Second)

	// stop comoponents
	rl.Stop()

	validateMD5(ts.path, remote_path, suite.assert)
}

func validateMD5(localPath string, remotePath string, assert *assert.Assertions) {
	entries, err := os.ReadDir(remotePath)
	assert.Nil(err)

	for _, entry := range entries {
		localFile := filepath.Join(localPath, entry.Name())
		remoteFile := filepath.Join(remotePath, entry.Name())

		if entry.IsDir() {
			f, err := os.Stat(localFile)
			assert.Nil(err)
			assert.True(f.IsDir())

			validateMD5(localFile, remoteFile, assert)
		} else {
			l, err := computeMD5(localFile)
			assert.Nil(err)

			r, err := computeMD5(remoteFile)
			assert.Nil(err)

			assert.Equal(l, r)
		}
	}
}

func computeMD5(filePath string) ([]byte, error) {
	fh, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	hash := md5.New()
	if _, err := io.Copy(hash, fh); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func TestSplitterSuite(t *testing.T) {
	suite.Run(t, new(splitterTestSuite))
}

// TODO:: xload : add tests for failure cases
