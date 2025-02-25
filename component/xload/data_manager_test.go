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
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type dataManagerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *dataManagerTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())
}

func (suite *dataManagerTestSuite) TestNewRemoteDataManager() {
	rdm, err := newRemoteDataManager(nil, nil)
	suite.assert.NotNil(err)
	suite.assert.Nil(rdm)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote data manager")

	remote := loopback.NewLoopbackFSComponent()
	statsMgr, err := NewStatsManager(1, false)
	suite.assert.Nil(err)
	suite.assert.NotNil(statsMgr)

	rdm, err = newRemoteDataManager(remote, statsMgr)
	suite.assert.Nil(err)
	suite.assert.NotNil(rdm)
}

func (suite *dataManagerTestSuite) TestProcessErrors() {
	rdm := &remoteDataManager{}

	ctx, cancel := context.WithCancel(context.Background())
	item := &WorkItem{
		CompName: DATA_MANAGER,
		Path:     "test",
		Block:    &Block{},
		Download: false,
		Ctx:      ctx,
	}

	n, err := rdm.Process(item)
	suite.assert.NotNil(err)
	suite.assert.Equal(n, 0)

	// cancel the context
	cancel()

	n, err = rdm.Process(item)
	suite.assert.NotNil(err)
	suite.assert.Equal(n, 0)
}

func TestDatamanagerSuite(t *testing.T) {
	suite.Run(t, new(dataManagerTestSuite))
}
