/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/spf13/viper"
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
  max-retries: 1
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

var configPriorityTest string = `
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
  - file_cache
  - libfuse
  - attr_cache
  - azstorage
`

var confFileMntTest, confFilePriorityTest string

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
	resetCLIFlags(*mountAllCmd)
	viper.Reset()

	common.DefaultWorkDir = "$HOME/.blobfuse2"
	common.DefaultLogFilePath = filepath.Join(common.DefaultWorkDir, "blobfuse2.log")
}

// mount failure test where the mount directory does not exists
func (suite *mountTestSuite) TestMountDirNotExists() {
	defer suite.cleanupTest()

	tempDir := randomString(8)
	op, err := executeCommandC(rootCmd, "mount", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory does not exists")

	op, err = executeCommandC(rootCmd, "mount", "all", tempDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory does not exists")
}

// mount failure test where the mount directory is not empty
func (suite *mountTestSuite) TestMountDirNotEmpty() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	tempDir := filepath.Join(mntDir, "tempdir")

	err = os.MkdirAll(tempDir, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount directory is not empty")

	op, err = executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "-o", "nonempty")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

// mount failure test where the mount path is not provided
func (suite *mountTestSuite) TestMountPathNotProvided() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "mount", "", fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")

	op, err = executeCommandC(rootCmd, "mount", "all", "", fmt.Sprintf("--config-file=%s", confFileMntTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "mount path not provided")
}

// mount failure test where the config file type is unsupported
func (suite *mountTestSuite) TestUnsupportedConfigFileType() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
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

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, "--config-file=cfgNotFound.yaml")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "no such file or directory")

	op, err = executeCommandC(rootCmd, "mount", "all", mntDir, "--config-file=cfgNotFound.yaml")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid config file")
	suite.assert.Contains(op, "no such file or directory")
}

// mount failure test where config file is not provided
func (suite *mountTestSuite) TestConfigFileNotProvided() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir)
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

// mount failure test where config file has components in wrong order
func (suite *mountTestSuite) TestComponentPrioritySetWrong() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	confFile, err := os.CreateTemp("", "conf*.yaml")
	suite.assert.Nil(err)
	confFilePriorityTest = confFile.Name()
	defer os.Remove(confFilePriorityTest)

	_, err = confFile.WriteString(configPriorityTest)
	suite.assert.Nil(err)
	confFile.Close()

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFilePriorityTest))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
	suite.assert.Contains(op, "component libfuse is out of order")
}

func (suite *mountTestSuite) TestDefaultConfigFile() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
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

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest), "--log-level=debug")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "invalid log level")
}

func (suite *mountTestSuite) TestCliParamsV1() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
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

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	tempLogDir := "/tmp/templogs_" + randomString(6)
	defer os.RemoveAll(tempLogDir)

	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--log-file-path=%s", tempLogDir+"/blobfuse2.log"),
		"--streaming", "--use-attr-cache", "--invalidate-on-sync", "--pre-mount-validate", "--basic-remount-check")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to initialize new pipeline")
}

// mount failure test where a libfuse option is incorrect
func (suite *mountTestSuite) TestInvalidLibfuseOption() {
	defer suite.cleanupTest()

	mntDir, err := os.MkdirTemp("", "mntdir")
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

	mntDir, err := os.MkdirTemp("", "mntdir")
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

	mntDir, err := os.MkdirTemp("", "mntdir")
	suite.assert.Nil(err)
	defer os.RemoveAll(mntDir)

	// incorrect umask value
	op, err := executeCommandC(rootCmd, "mount", mntDir, fmt.Sprintf("--config-file=%s", confFileMntTest),
		"-o allow_other", "-o attr_timeout=120", "-o entry_timeout=120", "-o negative_timeout=120",
		"-o ro", "-o allow_root", "-o default_permissions", "-o umask=abcd")
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to parse umask")
}

// fuse option parsing validation
func (suite *mountTestSuite) TestFuseOptions() {
	defer suite.cleanupTest()

	type fuseOpt struct {
		opt    string
		ignore bool
	}

	opts := []fuseOpt{
		{opt: "rw", ignore: true},
		{opt: "dev", ignore: true},
		{opt: "dev", ignore: true},
		{opt: "nodev", ignore: true},
		{opt: "suid", ignore: true},
		{opt: "nosuid", ignore: true},
		{opt: "delay_connect", ignore: true},
		{opt: "auto", ignore: true},
		{opt: "noauto", ignore: true},
		{opt: "user", ignore: true},
		{opt: "nouser", ignore: true},
		{opt: "exec", ignore: true},
		{opt: "noexec", ignore: true},

		{opt: "allow_other", ignore: false},
		{opt: "allow_other=true", ignore: false},
		{opt: "allow_other=false", ignore: false},
		{opt: "nonempty", ignore: false},
		{opt: "attr_timeout=10", ignore: false},
		{opt: "entry_timeout=10", ignore: false},
		{opt: "negative_timeout=10", ignore: false},
		{opt: "ro", ignore: false},
		{opt: "allow_root", ignore: false},
		{opt: "umask=777", ignore: false},
		{opt: "uid=1000", ignore: false},
		{opt: "gid=1000", ignore: false},
		{opt: "direct_io", ignore: false},
	}

	for _, val := range opts {
		ret := ignoreFuseOptions(val.opt)
		suite.assert.Equal(ret, val.ignore)
	}
}

func (suite *mountTestSuite) TestUpdateCliParams() {
	defer suite.cleanupTest()

	cliParams := []string{"blobfuse2", "mount", "~/mntdir/", "--foreground=false"}

	updateCliParams(&cliParams, "tmp-path", "tmpPath1")
	suite.assert.Equal(len(cliParams), 5)
	suite.assert.Equal(cliParams[4], "--tmp-path=tmpPath1")

	updateCliParams(&cliParams, "container-name", "testCnt1")
	suite.assert.Equal(len(cliParams), 6)
	suite.assert.Equal(cliParams[5], "--container-name=testCnt1")

	updateCliParams(&cliParams, "tmp-path", "tmpPath2")
	updateCliParams(&cliParams, "container-name", "testCnt2")
	suite.assert.Equal(len(cliParams), 6)
	suite.assert.Equal(cliParams[4], "--tmp-path=tmpPath2")
	suite.assert.Equal(cliParams[5], "--container-name=testCnt2")
}

func (suite *mountTestSuite) TestMountOptionVaildate() {
	defer suite.cleanupTest()
	opts := &mountOptions{}

	err := opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "mount path not provided")

	opts.MountPath, _ = os.UserHomeDir()
	err = opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid log level")

	opts.Logging.LogLevel = "log_junk"
	err = opts.validate(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid log level")

	opts.Logging.LogLevel = "log_debug"
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Empty(opts.Logging.LogFilePath)

	opts.DefaultWorkingDir, _ = os.UserHomeDir()
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Empty(opts.Logging.LogFilePath)
	suite.assert.Equal(common.DefaultWorkDir, opts.DefaultWorkingDir)

	opts.Logging.LogFilePath = common.DefaultLogFilePath
	err = opts.validate(true)
	suite.assert.Nil(err)
	suite.assert.Contains(opts.Logging.LogFilePath, opts.DefaultWorkingDir)
	suite.assert.Equal(common.DefaultWorkDir, opts.DefaultWorkingDir)
	suite.assert.Equal(common.DefaultLogFilePath, opts.Logging.LogFilePath)
}

func TestMountCommand(t *testing.T) {
	confFile, err := os.CreateTemp("", "conf*.yaml")
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
