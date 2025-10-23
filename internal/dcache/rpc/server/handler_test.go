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

package rpc_server

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type handlerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *handlerTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *handlerTestSuite) TestCreateLogTar() {
	logFilePath := common.ExpandPath("$HOME/.blobfuse2/blobfuse2.log") // update this path to where log file is located
	logFilePath = "/home/sourav/go/src/azure-storage-fuse/blobfuse2.log"
	config.Set("logging.file-path", logFilePath)

	h := &ChunkServiceHandler{}
	err := h.createLogTarLocked(4)
	suite.assert.Nil(err)
}

func (suite *handlerTestSuite) TestGetMemoryInfo() {
	total, used, err := getMemoryInfo()
	suite.assert.NoError(err)
	suite.assert.Greater(total, int64(0))
	suite.assert.Greater(used, int64(0))
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(handlerTestSuite))
}
