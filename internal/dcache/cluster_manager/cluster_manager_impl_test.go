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

package clustermanager

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClusterManagerImplTestSuite struct {
	suite.Suite
	cmi ClusterManagerImpl
}

func (suite *ClusterManagerImplTestSuite) TestCheckIfClusterMapExists() {
	orig := getClusterMap
	defer func() { getClusterMap = orig }()

	// 1) success
	getClusterMap = func() error { return nil }
	exists, err := suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.True(exists)

	// 2) os.ErrNotExist
	getClusterMap = func() error { return os.ErrNotExist }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 3) syscall.ENOENT
	getClusterMap = func() error { return syscall.ENOENT }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 4) other error
	testErr := errors.New("boom")
	getClusterMap = func() error { return testErr }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.EqualError(err, "boom")
	suite.False(exists)
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(ClusterManagerImplTestSuite))
}
