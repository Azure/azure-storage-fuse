// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

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
		{val: "preload", mode: EMode.PRELOAD()},
		{val: "upload", mode: EMode.UPLOAD()},
		{val: "sync", mode: EMode.SYNC()},
		{val: "invalid_mode", mode: EMode.INVALID_MODE()},
		{val: "PRELOAD", mode: EMode.PRELOAD()},
		{val: "UpLoad", mode: EMode.UPLOAD()},
		{val: "sYNC", mode: EMode.SYNC()},
		{val: "invalid", mode: EMode.INVALID_MODE()},
		{val: "RANDOM", mode: EMode.INVALID_MODE()},
	}

	for i, m := range modes {
		var mode Mode
		err := mode.Parse(m.val)
		if i < len(modes)-2 {
			suite.assert.NoError(err)
		} else {
			suite.assert.Error(err)
		}

		suite.assert.Equal(mode, m.mode)
	}
}

func (suite *utilsTestSuite) TestModeString() {
	modes := []struct {
		mode Mode
		val  string
	}{
		{mode: EMode.PRELOAD(), val: "PRELOAD"},
		{mode: EMode.UPLOAD(), val: "UPLOAD"},
		{mode: EMode.SYNC(), val: "SYNC"},
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
		suite.assert.InEpsilon(RoundFloat(v.val, v.precision), v.res, 0.00001)
	}
}

func (suite *utilsTestSuite) TestIsFilePresent() {
	path := "/home/randomFile1234"
	isPresent, isDir, size := isFilePresent(path)
	suite.assert.False(isPresent)
	suite.assert.False(isDir)
	suite.assert.EqualValues(0, size)

	currDir, err := os.Getwd()
	suite.assert.NoError(err)

	isPresent, isDir, size = isFilePresent(currDir)
	suite.assert.True(isPresent)
	suite.assert.True(isDir)
	suite.assert.Positive(size)

	path = filepath.Join(currDir, "testFile1234")
	_, err = os.Create(path)
	defer os.Remove(path)
	suite.assert.NoError(err)

	isPresent, isDir, size = isFilePresent(path)
	suite.assert.True(isPresent)
	suite.assert.False(isDir)
	suite.assert.EqualValues(0, size)

	err = os.Truncate(path, 10)
	suite.assert.NoError(err)

	isPresent, isDir, size = isFilePresent(path)
	suite.assert.True(isPresent)
	suite.assert.False(isDir)
	suite.assert.EqualValues(10, size)
}

func TestUtilsSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
