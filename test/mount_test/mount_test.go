// +build !unittest

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

package mount_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

var blobfuseBinary string = "blobfuse2"
var mntDir string = "mntdir"
var configFile string

type mountSuite struct {
	suite.Suite
}

func remountCheck(suite *mountSuite) {
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "directory is already mounted")
}

// list blobfuse mounted directories
func listBlobfuseMounts(suite *mountSuite) []byte {
	mntListCmd := exec.Command(blobfuseBinary, "mount", "list")
	cliOut, err := mntListCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(nil, err)
	return cliOut
}

// unmount blobfuse
func blobfuseUnmount(suite *mountSuite, unmountOutput string) {
	unmountCmd := exec.Command(blobfuseBinary, "unmount", "all")
	cliOut, err := unmountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.Equal(nil, err)
	suite.Contains(string(cliOut), unmountOutput)

	// wait after unmount
	time.Sleep(5 * time.Second)

	// validate unmount
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))
}

// mount command test along with remount on the same path
func (suite *mountSuite) TestMountCmd() {
	// run mount command
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)

	// wait for mount
	time.Sleep(10 * time.Second)

	// validate mount
	cliOut = listBlobfuseMounts(suite)
	suite.NotEqual(0, len(cliOut))
	suite.Contains(string(cliOut), mntDir)

	remountCheck(suite)

	// unmount
	blobfuseUnmount(suite, mntDir)
}

// mount failure test where the mount directory does not exists
func (suite *mountSuite) TestMountDirNotExists() {
	tempDir := filepath.Join(mntDir, "tempdir")
	mountCmd := exec.Command(blobfuseBinary, "mount", tempDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount directory does not exists")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where the mount directory is not empty
func (suite *mountSuite) TestMountDirNotEmpty() {
	tempDir := filepath.Join(mntDir, "tempdir")
	_ = os.Mkdir(tempDir, 0777)
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount directory is not empty")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	os.RemoveAll(tempDir)

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where the mount path is not provided
func (suite *mountSuite) TestMountPathNotProvided() {
	mountCmd := exec.Command(blobfuseBinary, "mount", "", "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount path not provided")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where config file is not provided
func (suite *mountSuite) TestConfigFileNotProvided() {
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "failed to mount")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where config file is not provided and environment variables have incorrect credentials
func (suite *mountSuite) TestEnvVarMountFailure() {
	tempDir := filepath.Join(mntDir, "..", "tempdir")
	os.Mkdir(tempDir, 0777)

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", "myAccount")
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", "myKey")
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "https://myAccount.dfs.core.windows.net")

	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--tmp-path="+tempDir, "--container-name=myContainer")
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(nil, err)

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")

	os.RemoveAll(tempDir)
}

// mount test using environment variables for mounting
func (suite *mountSuite) TestEnvVarMount() {
	// read config file
	configData, err := os.ReadFile(configFile)
	suite.Equal(nil, err)

	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBuffer(configData))

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", viper.GetString("azstorage.account-name"))
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", viper.GetString("azstorage.account-key"))
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", viper.GetString("azstorage.endpoint"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_CONTAINER", viper.GetString("azstorage.container"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_TYPE", viper.GetString("azstorage.type"))

	tempFile := viper.GetString("file_cache.path")

	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--tmp-path="+tempFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)

	// wait for mount
	time.Sleep(10 * time.Second)

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.NotEqual(0, len(cliOut))
	suite.Contains(string(cliOut), mntDir)

	// unmount
	blobfuseUnmount(suite, mntDir)

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_CONTAINER")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_TYPE")
}

// mount test using environment variables for mounting with cli options
func (suite *mountSuite) TestEnvVarMountCliParams() {
	// read config file
	configData, err := os.ReadFile(configFile)
	suite.Equal(nil, err)

	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBuffer(configData))

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", viper.GetString("azstorage.account-name"))
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", viper.GetString("azstorage.account-key"))
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", viper.GetString("azstorage.endpoint"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_CONTAINER", viper.GetString("azstorage.container"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_TYPE", viper.GetString("azstorage.type"))

	tempFile := viper.GetString("file_cache.path")

	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--tmp-path="+tempFile, "--allow-other",
		"--file-cache-timeout=120", "--cancel-list-on-mount-seconds=10", "--attr-timeout=120", "--entry-timeout=120",
		"--negative-timeout=120", "--log-level=LOG_WARNING", "--cache-size-mb=1000")
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)

	// wait for mount
	time.Sleep(4 * time.Second)

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.NotEqual(0, len(cliOut))
	suite.Contains(string(cliOut), mntDir)

	// unmount
	blobfuseUnmount(suite, mntDir)

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_CONTAINER")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_TYPE")
}

// mountv1 test using CSI driver cli options
func (suite *mountSuite) TestEnvVarMountCSIParams() {
	// read config file
	configData, err := os.ReadFile(configFile)
	suite.Equal(nil, err)

	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBuffer(configData))

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", viper.GetString("azstorage.account-name"))
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", viper.GetString("azstorage.account-key"))
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", viper.GetString("azstorage.endpoint"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_CONTAINER", viper.GetString("azstorage.container"))
	os.Setenv("AZURE_STORAGE_ACCOUNT_TYPE", viper.GetString("azstorage.type"))

	tempFile := viper.GetString("file_cache.path")

	mountCmd := exec.Command(blobfuseBinary, "mountv1", mntDir, "--tmp-path="+tempFile, "-o", "allow_other",
		"--file-cache-timeout-in-seconds=120", "--use-attr-cache=true", "--cancel-list-on-mount-seconds=10",
		"-o", "attr_timeout=120", "-o", "entry_timeout=120", "-o", "negative_timeout=120",
		"--log-level=LOG_WARNING", "--cache-size-mb=1000")
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)

	// wait for mount
	time.Sleep(4 * time.Second)

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.NotEqual(0, len(cliOut))
	suite.Contains(string(cliOut), mntDir)

	// unmount
	blobfuseUnmount(suite, mntDir)

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_CONTAINER")
	os.Unsetenv("AZURE_STORAGE_ACCOUNT_TYPE")
}

func TestMountSuite(t *testing.T) {
	suite.Run(t, new(mountSuite))
}

func TestMain(m *testing.M) {
	workingDirPtr := flag.String("working-dir", "", "Directory containing the blobfuse binary")
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	configPtr := flag.String("config-file", "", "Config file for mounting")

	flag.Parse()

	blobfuseBinary = filepath.Join(*workingDirPtr, blobfuseBinary)
	mntDir = filepath.Join(*pathPtr, mntDir)
	configFile = *configPtr

	err := os.RemoveAll(mntDir)
	if err != nil {
		fmt.Println("Could not cleanup mount directory before testing")
	}
	os.Mkdir(mntDir, 0777)

	m.Run()

	os.RemoveAll(mntDir)
}
