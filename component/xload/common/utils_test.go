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

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type utilsTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *utilsTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *utilsTestSuite) TestModeParse() {
	modes := []struct {
		val  string
		mode Mode
	}{
		{val: "download", mode: EMode.DOWNLOAD()},
		{val: "upload", mode: EMode.UPLOAD()},
		{val: "sync", mode: EMode.SYNC()},
		{val: "checkpoint", mode: EMode.CHECKPOINT()},
		{val: "invalid_mode", mode: EMode.INVALID_MODE()},
		{val: "DOWNLOAD", mode: EMode.DOWNLOAD()},
		{val: "UpLoad", mode: EMode.UPLOAD()},
		{val: "sYNC", mode: EMode.SYNC()},
		{val: "checkPOINT", mode: EMode.CHECKPOINT()},
		{val: "invalid", mode: EMode.INVALID_MODE()},
		{val: "RANDOM", mode: EMode.INVALID_MODE()},
	}

	for i, m := range modes {
		var mode Mode
		err := mode.Parse(m.val)
		if i < len(modes)-2 {
			suite.assert.Nil(err)
		} else {
			suite.assert.NotNil(err)
		}

		suite.assert.Equal(mode, m.mode)
	}
}

func (suite *utilsTestSuite) TestModeString() {
	modes := []struct {
		mode Mode
		val  string
	}{
		{mode: EMode.DOWNLOAD(), val: "DOWNLOAD"},
		{mode: EMode.UPLOAD(), val: "UPLOAD"},
		{mode: EMode.SYNC(), val: "SYNC"},
		{mode: EMode.CHECKPOINT(), val: "CHECKPOINT"},
		{mode: EMode.INVALID_MODE(), val: "INVALID_MODE"},
	}

	for _, m := range modes {
		suite.assert.Equal(m.mode.String(), m.val)
	}
}

func (suite *utilsTestSuite) TestRoundFloat() {
	values := []struct {
		val       float64
		precision int
		res       float64
	}{
		{val: 3.14159265359, precision: 2, res: 3.14},
		{val: 3.14159265359, precision: 4, res: 3.1416},
		{val: 3.14159265359, precision: 5, res: 3.14159},
		{val: 3.14159265359, precision: 0, res: 3},
		{val: 5.9245, precision: 0, res: 6},
		{val: 5.19645, precision: 2, res: 5.20},
	}

	for _, v := range values {
		suite.assert.Equal(RoundFloat(v.val, v.precision), v.res)
	}
}

func (suite *utilsTestSuite) TestIsFilePresent() {
	path := "/home/randomFile1234"
	isPresent, size := IsFilePresent(path)
	suite.assert.Equal(isPresent, false)
	suite.assert.EqualValues(size, 0)

	currDir, err := os.Getwd()
	suite.assert.Nil(err)

	path = filepath.Join(currDir, "testFile1234")
	_, err = os.Create(path)
	defer os.Remove(path)
	suite.assert.Nil(err)

	isPresent, size = IsFilePresent(path)
	suite.assert.Equal(isPresent, true)
	suite.assert.EqualValues(size, 0)

	err = os.Truncate(path, 10)
	suite.assert.Nil(err)

	isPresent, size = IsFilePresent(path)
	suite.assert.Equal(isPresent, true)
	suite.assert.EqualValues(size, 10)
}

func TestUtilsSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
