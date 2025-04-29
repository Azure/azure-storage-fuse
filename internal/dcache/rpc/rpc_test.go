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

package rpc_test

import (
	"context"
	"testing"

	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	rpc_server "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type rpcTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *rpcTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *rpcTestSuite) TestHelloRPC() {
	// start server
	server, err := rpc_server.NewNodeServer("localhost:9090")
	suite.assert.NoError(err)
	suite.assert.NotNil(server)

	err = server.Start()
	suite.assert.NoError(err)

	resp, err := rpc_client.Hello(context.Background(), "nodeID", &models.HelloRequest{})
	suite.assert.NoError(err)
	suite.assert.NotNil(resp)

	err = rpc_client.Cleanup()
	suite.assert.NoError(err)

	err = server.Stop()
	suite.assert.NoError(err)
}

func TestRPCTestSuite(t *testing.T) {
	suite.Run(t, new(rpcTestSuite))
}
