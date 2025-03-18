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
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

type listTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

var lb internal.Component
var lb_path string
var r *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(length int) string {
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *listTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.Nil(err)

	lb_path = filepath.Join("/tmp/", "xload_"+randomString(8))
	err = os.MkdirAll(lb_path, 0777)
	suite.assert.Nil(err)

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", lb_path)
	config.ReadConfigFromReader(strings.NewReader(cfg))

	lb = loopback.NewLoopbackFSComponent()
	err = lb.Configure(true)
	suite.assert.Nil(err)

	suite.createDirsAndFiles(lb_path)
}

func (suite *listTestSuite) TearDownSuite() {
	err := os.RemoveAll(lb_path)
	suite.assert.Nil(err)
}

func (suite *listTestSuite) createFile(filePath string, size int64) {
	f, err := os.Create(filePath)
	defer func() {
		err = f.Close()
		suite.assert.Nil(err)
	}()
	suite.assert.Nil(err)

	err = os.Truncate(filePath, size)
	suite.assert.Nil(err)
}

func (suite *listTestSuite) createDirsAndFiles(path string) {
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(path, fmt.Sprintf("file_%v", i))
		suite.createFile(filePath, int64(9*i))
	}

	for i := 0; i < 10; i++ {
		dirName := filepath.Join(path, fmt.Sprintf("dir_%v", i))
		err := os.MkdirAll(dirName, 0777)
		suite.assert.Nil(err)

		for j := 0; j < 5; j++ {
			filePath := filepath.Join(dirName, fmt.Sprintf("file_%v%v", i, j))
			suite.createFile(filePath, int64(i*j))
		}
	}
}

type testComponent struct {
	XBase
	ctr atomic.Int64
}

func getTestcomponent() *testComponent {
	tc := &testComponent{}
	tc.SetThreadPool(NewThreadPool(1, tc.Process))
	tc.GetThreadPool().Start()
	return tc
}

func (tcmp *testComponent) Stop() {
	tcmp.GetThreadPool().Stop()
}

func (tcmp *testComponent) Process(item *WorkItem) (int, error) {
	tcmp.ctr.Add(1)
	return int(tcmp.ctr.Load()), nil
}

type testLister struct {
	path  string
	stMgr *StatsManager
}

func setupTestLister() (*testLister, error) {
	tl := &testLister{}
	tl.path = filepath.Join("/tmp/", fmt.Sprintf("xlister_%v", randomString(8)))
	err := os.MkdirAll(tl.path, 0777)
	if err != nil {
		return nil, err
	}

	tl.stMgr, err = NewStatsManager(1, false)
	if err != nil {
		return nil, err
	}

	tl.stMgr.Start()
	return tl, nil
}

func (tl *testLister) cleanup() error {
	tl.stMgr.Stop()

	err := os.RemoveAll(tl.path)
	return err
}

func (suite *listTestSuite) TestNewRemoteLister() {
	rl, err := newRemoteLister(nil)
	suite.assert.NotNil(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		path:              "",
		workerCount:       0,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            nil,
		statsMgr:          nil,
	})
	suite.assert.NotNil(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		path:              "home/user/random_path",
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            nil,
		statsMgr:          nil,
	})
	suite.assert.NotNil(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		path:              "home/user/random_path",
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          nil,
	})
	suite.assert.NotNil(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	statsMgr, err := NewStatsManager(1, false)
	suite.assert.Nil(err)
	suite.assert.NotNil(statsMgr)

	rl, err = newRemoteLister(&remoteListerOptions{
		path:              "home/user/random_path",
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          statsMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)
}

func (suite *listTestSuite) TestListerStartStop() {
	tl, err := setupTestLister()
	suite.assert.Nil(err)
	suite.assert.NotNil(tl)

	defer func() {
		err = tl.cleanup()
		suite.assert.Nil(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		path:              tl.path,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          tl.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)

	testComp := getTestcomponent()
	rl.SetNext(testComp)

	rl.Start()
	time.Sleep(5 * time.Second)
	rl.Stop()

	suite.assert.Equal(testComp.ctr.Load(), int64(60))

	entries, err := os.ReadDir(tl.path)
	suite.assert.Nil(err)
	suite.assert.Len(entries, 10)
}

func (suite *listTestSuite) TestListerMkdir() {
	tl, err := setupTestLister()
	suite.assert.Nil(err)
	suite.assert.NotNil(tl)

	defer func() {
		err = tl.cleanup()
		suite.assert.Nil(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		path:              tl.path,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          tl.stMgr,
	})
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)

	for i := 0; i < 5; i++ {
		dirPath := filepath.Join(tl.path, fmt.Sprintf("dir%v", i))
		err = rl.mkdir(dirPath)
		suite.assert.Nil(err)
	}

	entries, err := os.ReadDir(tl.path)
	suite.assert.Nil(err)
	suite.assert.Len(entries, 5)
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(listTestSuite))
}
