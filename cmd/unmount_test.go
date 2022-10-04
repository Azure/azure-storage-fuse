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

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var confFileUnMntTest string
var configUnMountLoopback string = `
logging:
  type: syslog
default-working-dir: ./
components:
  - libfuse
  - loopbackfs
libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 60
loopbackfs:
  path: /tmp/bfuseloopback
`

type unmountTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *unmountTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}

	os.RemoveAll("/tmp/bfuseloopback")
	os.MkdirAll("/tmp/bfuseloopback", 0777)
}

func (suite *unmountTestSuite) cleanupTest() {
	resetCLIFlags(*unmountCmd)
	os.RemoveAll("/tmp/bfuseloopback")
}

// mount failure test where the mount directory does not exists
func (suite *unmountTestSuite) TestUnMount() {
	defer suite.cleanupTest()

	mountDirectory := "TestUnMount_1"
	os.MkdirAll(mountDirectory, 0777)
	defer os.RemoveAll(mountDirectory)

	_, err := executeCommandC(rootCmd, "mount", mountDirectory, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory)
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) TestUnMountFail() {
	defer suite.cleanupTest()

	mountDirectory := "TestUnMount_2"
	os.MkdirAll(mountDirectory, 0777)
	defer os.RemoveAll(mountDirectory)

	_, err := executeCommandC(rootCmd, "mount", mountDirectory, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	os.Chdir(mountDirectory)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory)
	suite.assert.NotNil(err)

	os.Chdir("..")
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory)
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) TestUnMountWildcard() {
	defer suite.cleanupTest()

	mountDirectory := "TestUnMount_3"
	os.MkdirAll(mountDirectory, 0777)
	defer os.RemoveAll(mountDirectory)

	_, err := executeCommandC(rootCmd, "mount", mountDirectory, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory+"*")
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) TestUnMountWildcardFail() {
	defer suite.cleanupTest()

	mountDirectory := "TestUnMount_4"
	os.MkdirAll(mountDirectory, 0777)
	defer os.RemoveAll(mountDirectory)

	_, err := executeCommandC(rootCmd, "mount", mountDirectory, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	os.Chdir(mountDirectory)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory+"*")
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "failed to unmount")

	os.Chdir("..")
	_, err = executeCommandC(rootCmd, "unmount", mountDirectory+"*")
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) TestValidArg() {
	defer suite.cleanupTest()

	mountDirectory := "TestUnMount_5"
	os.MkdirAll(mountDirectory, 0777)
	defer os.RemoveAll(mountDirectory)

	_, err := executeCommandC(rootCmd, "mount", mountDirectory, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	lst, _ := unmountCmd.ValidArgsFunction(nil, nil, "")
	suite.assert.NotEmpty(lst)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectory+"*")
	suite.assert.Nil(err)

	lst, _ = unmountCmd.ValidArgsFunction(nil, nil, "")
	suite.assert.Empty(lst)

	lst, _ = unmountCmd.ValidArgsFunction(nil, nil, "abcd")
	suite.assert.Empty(lst)
}

func TestUnMountCommand(t *testing.T) {
	confFile, err := ioutil.TempFile("", "conf*.yaml")
	if err != nil {
		t.Error("Failed to create config file")
	}

	confFileUnMntTest = confFile.Name()
	defer os.Remove(confFileUnMntTest)

	_, err = confFile.WriteString(configUnMountLoopback)
	if err != nil {
		t.Error("Failed to write to config file")
	}
	confFile.Close()

	suite.Run(t, new(unmountTestSuite))
}
