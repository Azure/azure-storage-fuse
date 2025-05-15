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
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type statsMgrTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *statsMgrTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.Nil(err)
}

func (suite *statsMgrTestSuite) TestNewStatsManager() {
	sm, err := NewStatsManager(5, false, nil)
	suite.assert.Nil(err)
	suite.assert.NotNil(sm)
	suite.assert.Nil(sm.fileHandle)

	sm, err = NewStatsManager(5, true, nil)
	suite.assert.Nil(err)
	suite.assert.NotNil(sm)
	suite.assert.NotNil(sm.fileHandle)

	err = os.Remove(sm.fileHandle.Name())
	suite.assert.Nil(err)
}

func (suite *statsMgrTestSuite) TestStatsManagerStartStop() {
	sm, err := NewStatsManager(10, true, nil)
	suite.assert.Nil(err)
	suite.assert.NotNil(sm)
	suite.assert.NotNil(sm.fileHandle)

	// start the stats manager
	sm.Start()

	defer func() {
		err = os.Remove(sm.fileHandle.Name())
		suite.assert.Nil(err)
	}()

	// push lister stats
	sm.AddStats(&StatsItem{Component: LISTER, Name: "", ListerCount: uint64(10)})
	sm.AddStats(&StatsItem{Component: LISTER, Name: "dirName", Dir: true, Success: true, Download: true})

	// export stats
	sm.AddStats(&StatsItem{Component: STATS_MANAGER})

	// incorrect component
	sm.AddStats(&StatsItem{Component: "random component"})

	// push data manager stats
	for i := 0; i < 5; i++ {
		fileName := fmt.Sprintf("file_%v", i)
		download := false
		if i%2 == 0 {
			download = true
		}
		sm.AddStats(&StatsItem{Component: DATA_MANAGER, Name: fileName, Success: true, Download: download, BytesTransferred: uint64(1024 * i)})
	}

	// add sleep for exporting stats
	time.Sleep(5 * time.Second)

	// push splitter stats
	for i := 0; i < 9; i++ {
		fileName := fmt.Sprintf("file_%v", i)
		success := false
		if i%2 == 0 {
			success = true
		}
		sm.AddStats(&StatsItem{Component: SPLITTER, Name: fileName, Success: success, Download: true})
	}

	time.Sleep(10 * time.Second)

	// stop the stats manager
	sm.Stop()

	suite.assert.Equal(sm.dirs, uint64(1))
	suite.assert.Equal(sm.totalFiles, sm.success+sm.failed)
	suite.assert.Greater(sm.bytesDownloaded, uint64(0))
	suite.assert.Greater(sm.bytesUploaded, uint64(0))
}

func TestStatsMgrSuite(t *testing.T) {
	suite.Run(t, new(statsMgrTestSuite))
}
