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
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

func randomString(length int) string {
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

func (suite *utilTestSuite) TestIsMountActiveNoMount() {
	var out bytes.Buffer
	cmd := exec.Command("../blobfuse2", "unmount", "all")
	cmd.Stdout = &out
	err := cmd.Run()
	suite.assert.Nil(err)
	cmd = exec.Command("pidof", "blobfuse2")
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Equal("exit status 1", err.Error())
	res, err := IsMountActive("/mnt/blobfuse")
	suite.assert.Nil(err)
	suite.assert.False(res)
}

func (suite *utilTestSuite) TestIsMountActiveTwoMounts() {
	var out bytes.Buffer

	// Define the file name and the content you want to write
	fileName := "config.yaml"

	lbpath := filepath.Join(home_dir, "lbpath")
	os.MkdirAll(lbpath, 0777)
	defer os.RemoveAll(lbpath)

	content := "components:\n" +
		"  - libfuse\n" +
		"  - loopbackfs\n\n" +
		"loopbackfs:\n" +
		"  path: " + lbpath + "\n\n"

	mntdir := filepath.Join(home_dir, "mountdir")
	os.MkdirAll(mntdir, 0777)
	defer os.RemoveAll(mntdir)

	dir, err := os.Getwd()
	suite.assert.Nil(err)
	configFile := filepath.Join(dir, "config.yaml")
	// Create or open the file. If it doesn't exist, it will be created.
	file, err := os.Create(fileName)
	suite.assert.Nil(err)
	defer file.Close() // Ensure the file is closed after we're done

	// Write the content to the file
	_, err = file.WriteString(content)
	suite.assert.Nil(err)

	err = os.Chdir("..")
	suite.assert.Nil(err)

	dir, err = os.Getwd()
	suite.assert.Nil(err)
	binary := filepath.Join(dir, "blobfuse2")
	cmd := exec.Command(binary, mntdir, "--config-file", configFile)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Nil(err)

	res, err := IsMountActive(mntdir)
	suite.assert.Nil(err)
	suite.assert.True(res)

	res, err = IsMountActive("/mnt/blobfuse")
	suite.assert.Nil(err)
	suite.assert.False(res)

	cmd = exec.Command(binary, "unmount", mntdir)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Nil(err)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	path := "~/a/b/c/d"
	expandedPath := ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, path[2:])
	suite.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, path[5:])
	suite.assert.Contains(expandedPath, homeDir)

	path = "/a/b/c/d"
	expandedPath = ExpandPath(path)
	suite.assert.Equal(expandedPath, path)

	path = "./a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, pwd)

	path = "./a/../a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, pwd)

	path = "~/a/../$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = "$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = "/$HOME/a/b/c/d/../../../a/b/c/d/.././a"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(expandedPath, homeDir)

	path = ""
	expandedPath = ExpandPath(path)
	suite.assert.Equal(expandedPath, path)

	path = "$HOME/.blobfuse2/config_$web.yaml"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(path, "$web")

	path = "$HOME/.blobfuse2/$web"
	expandedPath = ExpandPath(path)
	suite.assert.NotEqual(expandedPath, path)
	suite.assert.Contains(path, "$web")
}

func (suite *utilTestSuite) TestGetUSage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test")
	err = os.Mkdir(dirName, 0777)
	suite.assert.Nil(err)

	data := make([]byte, 1024*1024)
	err = os.WriteFile(dirName+"/1.txt", data, 0777)
	suite.assert.Nil(err)

	err = os.WriteFile(dirName+"/2.txt", data, 0777)
	suite.assert.Nil(err)

	usage, err := GetUsage(dirName)
	suite.assert.Nil(err)
	suite.assert.GreaterOrEqual(int(usage), 2)
	suite.assert.LessOrEqual(int(usage), 4)

	_ = os.RemoveAll(dirName)
}

func (suite *utilTestSuite) TestGetDiskUsage() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dirName := filepath.Join(pwd, "util_test", "a", "b", "c")
	err = os.MkdirAll(dirName, 0777)
	suite.assert.Nil(err)

	usage, usagePercent, err := GetDiskUsageFromStatfs(dirName)
	suite.assert.Nil(err)
	suite.assert.NotEqual(usage, 0)
	suite.assert.NotEqual(usagePercent, 0)
	suite.assert.NotEqual(usagePercent, 100)
	_ = os.RemoveAll(filepath.Join(pwd, "util_test"))
}

func (suite *utilTestSuite) TestDirectoryCleanup() {
	dirName := "./TestDirectoryCleanup"

	// Directory does not exists
	exists := DirectoryExists(dirName)
	suite.assert.False(exists)

	err := TempCacheCleanup(dirName)
	suite.assert.Nil(err)

	// Directory exists but is empty
	_ = os.MkdirAll(dirName, 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty := IsDirectoryEmpty(dirName)
	suite.assert.True(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.Nil(err)

	// Directory exists and is not empty
	_ = os.MkdirAll(dirName+"/A", 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty = IsDirectoryEmpty(dirName)
	suite.assert.False(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.Nil(err)

	_ = os.RemoveAll(dirName)

}

func (suite *utilTestSuite) TestWriteToFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}
	filePath := fmt.Sprintf(".blobfuse2/test_%s.txt", randomString(8))
	content := "Hello World"
	filePath = homeDir + "/" + filePath

	defer os.Remove(filePath)

	err = WriteToFile(filePath, content, WriteToFileOptions{})
	suite.assert.Nil(err)

	// Check if file exists
	suite.assert.FileExists(filePath)

	// Check the content of the file
	data, err := os.ReadFile(filePath)
	suite.assert.Nil(err)
	suite.assert.Equal(content, string(data))

}

func (suite *utilTestSuite) TestCRC64() {
	data := []byte("Hello World")
	crc := GetCRC64(data, len(data))

	data = []byte("Hello World!")
	crc1 := GetCRC64(data, len(data))

	suite.assert.NotEqual(crc, crc1)
}

func (suite *utilTestSuite) TestGetFuseMinorVersion() {
	i := GetFuseMinorVersion()
	suite.assert.GreaterOrEqual(i, 0)
}
