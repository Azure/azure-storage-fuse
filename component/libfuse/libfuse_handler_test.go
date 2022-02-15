/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package libfuse

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Tests the default configuration of libfuse
func (suite *libfuseTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal(suite.libfuse.Name(), "libfuse")
}

// getattr

func (suite *libfuseTestSuite) TestMkDir() {
	testMkDir(suite)
}

func (suite *libfuseTestSuite) TestMkDirError() {
	testMkDirError(suite)
}

// readdir

func (suite *libfuseTestSuite) TestRmDir() {
	testRmDir(suite)
}

func (suite *libfuseTestSuite) TestRmDirNotEmpty() {
	testRmDirNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestRmDirError() {
	testRmDirError(suite)
}

func (suite *libfuseTestSuite) TestCreate() {
	testCreate(suite)
}

func (suite *libfuseTestSuite) TestCreateError() {
	testCreateError(suite)
}

func (suite *libfuseTestSuite) TestOpen() {
	testOpen(suite)
}

func (suite *libfuseTestSuite) TestOpenSyncFlag() {
	testOpenSyncFlag(suite)
}

func (suite *libfuseTestSuite) TestOpenNotExists() {
	testOpenNotExists(suite)
}

func (suite *libfuseTestSuite) TestOpenError() {
	testOpenError(suite)
}

// read

// write

// flush

func (suite *libfuseTestSuite) TestTruncate() {
	testTruncate(suite)
}

func (suite *libfuseTestSuite) TestTruncateError() {
	testTruncateError(suite)
}

// release

func (suite *libfuseTestSuite) TestUnlink() {
	testUnlink(suite)
}

func (suite *libfuseTestSuite) TestUnlinkNotExists() {
	testUnlinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestUnlinkError() {
	testUnlinkError(suite)
}

// rename

func (suite *libfuseTestSuite) TestSymlink() {
	testSymlink(suite)
}

func (suite *libfuseTestSuite) TestSymlinkError() {
	testSymlinkError(suite)
}

func (suite *libfuseTestSuite) TestReadLink() {
	testReadLink(suite)
}

func (suite *libfuseTestSuite) TestReadLinkNotExists() {
	testReadLinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestReadLinkError() {
	testReadLinkError(suite)
}

func (suite *libfuseTestSuite) TestFsync() {
	testFsync(suite)
}

func (suite *libfuseTestSuite) TestFsyncHandleError() {
	testFsyncHandleError(suite)
}

func (suite *libfuseTestSuite) TestFsyncError() {
	testFsyncError(suite)
}

func (suite *libfuseTestSuite) TestFsyncDir() {
	testFsyncDir(suite)
}

func (suite *libfuseTestSuite) TestFsyncDirError() {
	testFsyncDirError(suite)
}

func (suite *libfuseTestSuite) TestChmod() {
	testChmod(suite)
}

func (suite *libfuseTestSuite) TestChmodNotExists() {
	testChmodNotExists(suite)
}

func (suite *libfuseTestSuite) TestChmodError() {
	testChmodError(suite)
}

func (suite *libfuseTestSuite) TestChown() {
	testChown(suite)
}

func (suite *libfuseTestSuite) TestUtimens() {
	testUtimens(suite)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLibfuseTestSuite(t *testing.T) {
	suite.Run(t, new(libfuseTestSuite))
}
