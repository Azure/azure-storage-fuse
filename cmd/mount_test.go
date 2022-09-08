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

var configMountTest string = `
logging:
  type: syslog
default-working-dir: /tmp/blobfuse2
file_cache:
  path: /tmp/fileCachePath
libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 60
azstorage:
  account-name: myAccountName
  account-key: myAccountKey
  mode: key
  endpoint: myEndpoint
  container: myContainer
components:
  - libfuse
  - file_cache
  - attr_cache
  - azstorage
health_monitor:
  monitor-disable-list:
    - network_profiler
    - blobfuse_stats
`

var configMountLoopback string = `
logging:
  type: syslog
default-working-dir: /tmp/blobfuse2
components:
  - libfuse
  - loopbackfs
libfuse:
  attribute-expiration-sec: 120
  entry-expiration-sec: 60
loopbackfs:
  path: /tmp/bfuseloopback
`

var confFileMntTest string

type mountTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *mountTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *mountTestSuite) cleanupTest() {
	resetCLIFlags(*mountCmd)
}

// mount failure test where the mount directory does not exists
func (suite *mountTestSuite) TestMountDirNotExists() {
	defer suite.cleanupTest()

	tempDir := randomString(8)
	op, err := executeCommandC(rootCmd, "mount", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory does not exists")
}

// mount failure test where the mount directory is not empty
func (suite *mountTestSuite) TestMountDirNotEmpty() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	tempDir := filepath.Join(mntDir, "tempdir")

	err = os.MkdirAll(tempDir, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory is not empty")
}

// mount failure test where the mount path is not provided
func (suite *mountTestSuite) TestMountPathNotProvided() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "mount", "", fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")
}

// mount failure test where the config file type is unsupported
func (suite *mountTestSuite) TestUnsupportedConfigFileType() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=cfgInvalid.yam")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "Unsupported Config Type")
}

// mount failure test where the config file is not present
func (suite *mountTestSuite) TestConfigFileNotFound() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=cfgNotFound.yaml")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "no such file or directory")
}

// mount failure test where config file is not provided
func (suite *mountTestSuite) TestConfigFileNotProvided() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir)
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

func (suite *mountTestSuite) TestDefaultConfigFile() {
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

	op, err := executeCommandC(rootCmd, "mount", mntDir)
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

func (suite *mountTestSuite) TestInvalidLogLevel() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--log-level=debug")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid log level")
}

func (suite *mountTestSuite) TestCliParamsV1() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	tempLogDir := "/tmp/templogs_" + randomString(6)
	defer os.RemoveAll(tempLogDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		fmt.Sprintf("--log-file-path=%s", tempLogDir+"/blobfuse2.log"), "--invalidate-on-sync", "--pre-mount-validate", "--basic-remount-check")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

func (suite *mountTestSuite) TestStreamAttrCacheOptionsV1() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	tempLogDir := "/tmp/templogs_" + randomString(6)
	defer os.RemoveAll(tempLogDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--log-file-path=%s", tempLogDir+"/blobfuse2.log"),
		"--streaming", "--use-attr-cache", "--invalidate-on-sync", "--pre-mount-validate", "--basic-remount-check")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

// libfuse options test
func (suite *mountTestSuite) TestLibfuseOptions() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	tempLogDir := "/tmp/templogs_" + randomString(6)
	defer os.RemoveAll(tempLogDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		fmt.Sprintf("--log-file-path=%s", tempLogDir+"/blobfuse2.log"), "--invalidate-on-sync", "--pre-mount-validate", "--basic-remount-check",
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o default_permissions", "-o umask=755")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

// mount failure test where libfuse options are greater than 8
func (suite *mountTestSuite) TestLibfuseOptionsMoreThan8() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	// greater than 8 libfuse options
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o default_permissions", "-o umask=755", "-o random_option")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid FUSE options")
}

// mount failure test where a libfuse option is incorrect
func (suite *mountTestSuite) TestInvalidLibfuseOption() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	// incorrect option
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o default_permissions", "-o umask=755", "-o a=b=c")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid FUSE options")
}

// mount failure test where a libfuse option is undefined
func (suite *mountTestSuite) TestUndefinedLibfuseOption() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	// undefined option
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o umask=755", "-o random_option")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid FUSE options")
}

// mount failure test where umask value is invalid
func (suite *mountTestSuite) TestInvalidUmaskValue() {
	defer suite.cleanupTest()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	// incorrect umask value
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o default_permissions", "-o umask=abcd")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to parse umask")
}

func (suite *mountTestSuite) TestMountUsingLoopbackFailure() {
	defer suite.cleanupTest()

	confFile, err := ioutil.TempFile("", "conf*.yaml")
	suite.assert.Nil(err)
	confFileName := confFile.Name()
	defer os.Remove(confFileName)

	_, err = confFile.WriteString(configMountLoopback)
	suite.assert.Nil(err)
	confFile.Close()

	mntDir, err := ioutil.TempDir("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileName))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to daemonize application")
}

func TestMountCommand(t *testing.T) {
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

	suite.Run(t, new(mountTestSuite))
}
