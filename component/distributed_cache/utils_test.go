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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type utilTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *utilTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestUtil(t *testing.T) {
	suite.Run(t, new(utilTestSuite))
}

func (suite *utilTestSuite) TestisPathContainsDcacheVirtualComponent() {
	isAzurePath, isDcachePath, rawPath := getFS("fs=dcache/foo")
	suite.assert.False(isAzurePath)
	suite.assert.True(isDcachePath)
	suite.assert.Equal("foo", rawPath)

	isAzurePath, isDcachePath, rawPath = getFS("fs=dcache/foo/bar")
	suite.assert.False(isAzurePath)
	suite.assert.True(isDcachePath)
	suite.assert.Equal("foo/bar", rawPath)

	isAzurePath, isDcachePath, rawPath = getFS("fs=dcacheee/foo/bar")
	suite.assert.False(isAzurePath)
	suite.assert.False(isDcachePath)
	suite.assert.Equal("fs=dcacheee/foo/bar", rawPath)
}

func (suite *utilTestSuite) TestisPathContainsAzureVirtualComponent() {
	isAzurePath, isDcachePath, rawPath := getFS("fs=azure/foo")
	suite.assert.True(isAzurePath)
	suite.assert.False(isDcachePath)
	suite.assert.Equal("foo", rawPath)
	isAzurePath, isDcachePath, rawPath = getFS("fs=azure/foo/bar")
	suite.assert.True(isAzurePath)
	suite.assert.False(isDcachePath)
	suite.assert.Equal("foo/bar", rawPath)
	isAzurePath, isDcachePath, rawPath = getFS("fs=azureeee/foo/bar")
	suite.assert.False(isAzurePath)
	suite.assert.False(isDcachePath)
	suite.assert.Equal("fs=azureeee/foo/bar", rawPath)
}
