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

package rpc_client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type rpcClientTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *rpcClientTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *rpcClientTestSuite) TestNewRPCClientTimeout() {
	nodeID := "test-node-id"
	nodeAddress := "10.0.0.5:9090"

	client, err := newRPCClient(nodeID, 0, nodeAddress)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "timeout")
	suite.assert.Nil(client)
}

func (suite *rpcClientTestSuite) TestConvertBytesToReadable() {
	suite.assert.Equal("512 B", bytesToReadable(512))
	suite.assert.Equal("1.00 KB", bytesToReadable(1024))
	suite.assert.Equal("1.00 MB", bytesToReadable(1048576))
	suite.assert.Equal("1.00 GB", bytesToReadable(1073741824))
	suite.assert.Equal("12.02 GB", bytesToReadable(12911104000))
	suite.assert.Equal("1.00 TB", bytesToReadable(1099511627776))
	suite.assert.Equal("1.00 PB", bytesToReadable(1125899906842624))
}

func TestRPCClientTestSuite(t *testing.T) {
	suite.Run(t, new(rpcClientTestSuite))
}
