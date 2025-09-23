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

package log

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LoggerTestSuite struct {
	suite.Suite
	log_rotate_test_count int
}

func fastTestDebug(lts *LoggerTestSuite) {
	for i := 0; i < lts.log_rotate_test_count; i++ {
		Debug("hello %d", i)
	}
}

func fastTestCrit(lts *LoggerTestSuite) {
	for i := 0; i < lts.log_rotate_test_count; i++ {
		Crit("hello %d", i)
	}
}

func simpleTest(lts *LoggerTestSuite) {
	Crit("Running Simple Test")
	for l := range 3 {
		switch l {
		case 0:
			SetLogLevel(common.ELogLevel.LOG_DEBUG())
		case 1:
			SetLogLevel(common.ELogLevel.LOG_INFO())
		case 2:
			SetLogLevel(common.ELogLevel.LOG_WARNING())
		default:
			SetLogLevel(common.ELogLevel.LOG_ERR())
		}

		Debug("hello %d", l)
		Trace("hello %d", l)
		Info("hello %d", l)
		Warn("hello %d", l)
		Err("hello %d", l)
		Crit("hello %d", l)
	}
}

func (lts *LoggerTestSuite) SetupTest() {
	lts.log_rotate_test_count = (10 * 1000 * 10)
}

func (lts *LoggerTestSuite) TestBaseLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	err := SetDefaultLogger("base", cfg)
	assert.Nil(err, "Failed to set base logger")

	simpleTest(lts)

	SetLogLevel(common.ELogLevel.LOG_DEBUG())
	fastTestDebug(lts)

	SetLogLevel(common.ELogLevel.LOG_CRIT())
	fastTestCrit(lts)

	err = Destroy()
	assert.Nil(err, "Failed to release base logger")
}

func (lts *LoggerTestSuite) TestSilentLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{}

	err := SetDefaultLogger("silent", cfg)
	assert.Nil(err, "Failed to set silent logger")

	simpleTest(lts)
}

func (lts *LoggerTestSuite) TestSysLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	}

	err := SetDefaultLogger("syslog", cfg)
	assert.Nil(err, "Failed to set silent logger")

	simpleTest(lts)
}

func (lts *LoggerTestSuite) TestNegative() {
	assert := assert.New(lts.T())
	cfg := common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	}

	err := SetDefaultLogger("negative", cfg)
	assert.NotNil(err, "Negative : did not get logger object")
}

func TestLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}
