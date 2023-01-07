/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package common

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

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

func (suite *typesTestSuite) TestDirectoryExists() {
	rand := randomString(8)
	dir := filepath.Join(home_dir, "dir"+rand)
	os.MkdirAll(dir, 0777)
	defer os.RemoveAll(dir)

	exists := DirectoryExists(dir)
	suite.assert.True(exists)
}

func (suite *typesTestSuite) TestDirectoryDoesNotExist() {
	rand := randomString(8)
	dir := filepath.Join(home_dir, "dir"+rand)

	exists := DirectoryExists(dir)
	suite.assert.False(exists)
}

func (suite *typesTestSuite) TestEncryptBadKey() {
	// Generate a random key
	key := make([]byte, 20)
	rand.Read(key)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := EncryptData(data, key)
	suite.assert.NotNil(err)
}

func (suite *typesTestSuite) TestDecryptBadKey() {
	// Generate a random key
	key := make([]byte, 20)
	rand.Read(key)

	data := make([]byte, 1024)
	rand.Read(data)

	_, err := DecryptData(data, key)
	suite.assert.NotNil(err)
}

func (suite *typesTestSuite) TestEncryptDecrypt() {
	// Generate a random key
	key := make([]byte, 16)
	rand.Read(key)

	data := make([]byte, 1024)
	rand.Read(data)

	cipher, err := EncryptData(data, key)
	suite.assert.Nil(err)

	d, err := DecryptData(cipher, key)
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, d)
}

func (suite *utilTestSuite) TestMonitorBfs() {
	monitor := MonitorBfs()
	suite.assert.False(monitor)
}

func (suite *utilTestSuite) TestExpandPath() {
	path := "~/a/b/c/d"
	expandedPath := ExpandPath(path)
	suite.assert.Contains(expandedPath, path[2:])
}
