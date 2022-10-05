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
	"path/filepath"
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
  #level: log_debug
  #file-path: blobfuse2.log
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

var currentDir string
var mountDirectoryPrefix string

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

	os.MkdirAll(mountDirectoryPrefix, 0777)
	os.MkdirAll("/tmp/bfuseloopback", 0777)
}

func (suite *unmountTestSuite) cleanupTest() {
	resetCLIFlags(*unmountCmd)
	resetCLIFlags(*mountCmd)
	resetCLIFlags(*rootCmd)
	time.Sleep(2 * time.Second)
}

// mount failure test where the mount directory does not exists
func (suite *unmountTestSuite) unmountCmd() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount", mountDirectoryPrefix, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix)
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) unmountCmdFail() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount", mountDirectoryPrefix, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	err = os.Chdir(mountDirectoryPrefix)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix)
	suite.assert.NotNil(err)

	err = os.Chdir(currentDir)
	suite.assert.Nil(err)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix)
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) unmountCmdWildcard() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount", mountDirectoryPrefix, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix+"*")
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) unmountCmdWildcardFail() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount", mountDirectoryPrefix, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	err = os.Chdir(mountDirectoryPrefix)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix+"*")
	suite.assert.NotNil(err)
	if err != nil {
		suite.assert.Contains(err.Error(), "failed to unmount")
	}

	err = os.Chdir(currentDir)
	suite.assert.Nil(err)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix+"*")
	suite.assert.Nil(err)
}

func (suite *unmountTestSuite) unmountCmdValidArg() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "mount", mountDirectoryPrefix, fmt.Sprintf("--config-file=%s", confFileUnMntTest))
	suite.assert.Nil(err)

	time.Sleep(2 * time.Second)
	lst, _ := unmountCmd.ValidArgsFunction(nil, nil, "")
	suite.assert.NotEmpty(lst)

	_, err = executeCommandC(rootCmd, "unmount", mountDirectoryPrefix+"*")
	suite.assert.Nil(err)

	lst, _ = unmountCmd.ValidArgsFunction(nil, nil, "")
	suite.assert.Empty(lst)

	lst, _ = unmountCmd.ValidArgsFunction(nil, nil, "abcd")
	suite.assert.Empty(lst)
}

func (suite *unmountTestSuite) TestUnMountCmd() {
	suite.unmountCmd()
	suite.unmountCmdFail()
	suite.unmountCmdWildcard()
	suite.unmountCmdWildcardFail()
	suite.unmountCmdValidArg()
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

	currentDir, _ = os.Getwd()
	mountDirectoryPrefix = filepath.Join(currentDir, "TestUnMount")

	err = os.MkdirAll(mountDirectoryPrefix, 0777)
	if err != nil {
		t.Error("Failed to create mount dir ", err.Error())
	}

	err = os.MkdirAll("/tmp/bfuseloopback", 0777)
	if err != nil {
		t.Error("Failed to create loopback dir ", err.Error())
	}

	defer os.RemoveAll(mountDirectoryPrefix)
	defer os.RemoveAll("/tmp/bfuseloopback")

	suite.Run(t, new(unmountTestSuite))
}
