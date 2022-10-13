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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// var configMountTest string = `
// logging:
//   type: syslog
// default-working-dir: /tmp/blobfuse2
// file_cache:
//   path: /tmp/fileCachePath
// libfuse:
//   attribute-expiration-sec: 120
//   entry-expiration-sec: 60
// azstorage:
//   account-name: myAccountName
//   account-key: myAccountKey
//   mode: key
//   endpoint: myEndpoint
//   container: myContainer
// components:
//   - libfuse
//   - file_cache
//   - attr_cache
//   - azstorage
// health_monitor:
//   monitor-disable-list:
//     - network_profiler
//     - blobfuse_stats
// `

// var configMountLoopback string = `
// logging:
//   type: syslog
// default-working-dir: /tmp/blobfuse2
// components:
//   - libfuse
//   - loopbackfs
// libfuse:
//   attribute-expiration-sec: 120
//   entry-expiration-sec: 60
// loopbackfs:
//   path: /tmp/bfuseloopback
// `

// var confFileMntTest string

type mountAllTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *mountAllTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *mountAllTestSuite) cleanupTest() {
	resetCLIFlags(*mountCmd)
}

// mount failure test where the mount directory does not exists
func (suite *mountAllTestSuite) TestMountAllDirNotExists() {
	defer suite.cleanupTest()

	tempDir := randomString(8)
	op, err := executeCommandC(rootCmd, "mount", "all", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory does not exists")
}

// mount failure test where the mount path is not provided
func (suite *mountAllTestSuite) TestMountAllPathNotProvided() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "mount", "all", "", fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")
}

// mount failure test where the config file type is unsupported
func (suite *mountAllTestSuite) TestUnsupportedConfigFileType() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", "all", mntDir, "--config-file=cfgInvalid.yam")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "Unsupported Config Type")
}

// mount failure test where the config file is not present
func (suite *mountAllTestSuite) TestConfigFileNotFound() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", "all", mntDir, "--config-file=cfgNotFound.yaml")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "no such file or directory")
}

// mount failure test where config file is not provided
func (suite *mountAllTestSuite) TestConfigFileNotProvided() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", "all", mntDir)
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to configure AzureStorage object")
}

func (suite *mountAllTestSuite) TestDefaultConfigFile() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	currDir, err := os.Getwd()
	suite.assert.Nil(err)
	defaultCfgPath := filepath.Join(currDir, common.DefaultConfigFilePath)

	// create default config file
	src, err := os.Open(confFileMntTest)
	suite.Equal(nil, err)

	dest, err := os.Create(defaultCfgPath)
	suite.Equal(nil, err)
	defer os.Remove(defaultCfgPath)

	bytesCopied, err := io.Copy(dest, src)
	suite.Equal(nil, err)
	suite.NotEqual(0, bytesCopied)

	err = dest.Close()
	suite.Equal(nil, err)
	err = src.Close()
	suite.Equal(nil, err)

	op, err := executeCommandC(rootCmd, "mount", "all", mntDir)
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to get container list from storage")
}

func (suite *mountAllTestSuite) TestInvalidLogLevel() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", "all", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--log-level=debug")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid log level")
}

func TestMountAllCommand(t *testing.T) {
	confFile, err := ioutil.TempFile("", "conf*.yaml")
	if err != nil {
		t.Error("Failed to create config file")
	}
	confFileMntTest = confFile.Name()
	defer os.Remove(confFileMntTest)

	_, err = confFile.WriteString(configMountTest)
	if err != nil {
		t.Error("Failed to write to config file")
	}
	confFile.Close()

	suite.Run(t, new(mountAllTestSuite))
}
