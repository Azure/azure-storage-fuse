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

package distributed_cache

import (
	"errors"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type hbManagerTestSuite struct {
	suite.Suite
	assert    *assert.Assertions
	hbManager *HeartbeatManager

	mockCtrl *gomock.Controller
	mock     *internal.MockComponent
}

func (suite *hbManagerTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	cachePath, _ := os.UserHomeDir()
	suite.hbManager = &HeartbeatManager{
		cachePath: cachePath,
		comp:      suite.mock,
		hbPath:    "__CACHE__mycache1",
	}
}

func (suite *hbManagerTestSuite) TestStartHbSuccess() {
	suite.mock.EXPECT().WriteFromBuffer(gomock.Any()).Return(nil)
	err := suite.hbManager.Starthb()
	suite.assert.Nil(err)
}

func (suite *hbManagerTestSuite) TestStartHbFail() {
	suite.mock.EXPECT().WriteFromBuffer(gomock.Any()).Return(errors.New("test error"))
	err := suite.hbManager.Starthb()
	suite.assert.Equal("test error", err.Error())
}

func (suite *hbManagerTestSuite) TestStopHbSuccess() {
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)
	err := suite.hbManager.Stop()
	suite.assert.Nil(err)
}

func (suite *hbManagerTestSuite) TestStopHbFail() {
	suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(errors.New("test error"))
	err := suite.hbManager.Stop()
	suite.assert.Equal("test error", err.Error())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHeartbeatManagerTestSuite(t *testing.T) {

	suite.Run(t, new(hbManagerTestSuite))
}
