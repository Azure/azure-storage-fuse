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
	found, resPath := isPathContainsDcacheVirtualComponent("fs=dcache/foo")
	suite.assert.True(found)
	suite.assert.Equal("foo", resPath)
	found, resPath = isPathContainsDcacheVirtualComponent("fs=dcache/foo/bar")
	suite.assert.True(found)
	suite.assert.Equal("foo/bar", resPath)
	found, resPath = isPathContainsDcacheVirtualComponent("fs=dcacheee/foo/bar")
	suite.assert.False(found)
}

func (suite *utilTestSuite) TestisPathContainsAzureVirtualComponent() {
	found, resPath := isPathContainsAzureVirtualComponent("fs=azure/foo")
	suite.assert.True(found)
	suite.assert.Equal("foo", resPath)
	found, resPath = isPathContainsAzureVirtualComponent("fs=azure/foo/bar")
	suite.assert.True(found)
	suite.assert.Equal("foo/bar", resPath)
	found, resPath = isPathContainsAzureVirtualComponent("fs=azureeee/foo/bar")
	suite.assert.False(found)
}

func (suite *utilTestSuite) TestisValidPath() {
	ok := isPathValid("foo/bar/baz")
	suite.assert.True(ok)
	ok = isPathValid("fs=azure/foo/bar/baz")
	suite.assert.True(ok)
	ok = isPathValid("fs=dcache/foo/bar/baz")
	suite.assert.True(ok)
	ok = isPathValid("fs=azure/foo/fs=dcache/bar/baz")
	suite.assert.False(ok)
	ok = isPathValid("fs=azure/foo/fs=azure/bar/baz")
	suite.assert.False(ok)
	ok = isPathValid("fs=dcache/foo/fs=dcache/bar/baz")
	suite.assert.False(ok)
}
