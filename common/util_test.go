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

func (suite *utilTestSuite) TestGetUsage() {
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

func (suite *utilTestSuite) TestGetUsageWithSymlinks() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	dir1 := filepath.Join(pwd, "util_test_dir1")
	dir2 := filepath.Join(pwd, "util_test_dir2")
	err = os.Mkdir(dir1, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(dir1)

	err = os.Mkdir(dir2, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(dir2)

	data := make([]byte, 1024*1024)
	file1 := filepath.Join(dir1, "file1.txt")
	file2 := filepath.Join(dir2, "file2.txt")

	err = os.WriteFile(file1, data, 0777)
	suite.assert.Nil(err)

	err = os.WriteFile(file2, data, 0777)
	suite.assert.Nil(err)

	symlink := filepath.Join(dir1, "link_to_file2")
	err = os.Symlink(file2, symlink)
	suite.assert.Nil(err)

	linkInfo, err := os.Lstat(symlink)
	suite.assert.Nil(err)
	symlinkSize := linkInfo.Size()

	usage, err := GetUsage(dir1)
	suite.assert.Nil(err)

	file1ExpectedSize := float64(1024 * 1024)
	expectedUsageMB := (file1ExpectedSize + float64(symlinkSize)) / (1024 * 1024)

	/* Usage should be greater than 1MB (size of the file plus the symlink size)
	   but less than 1.5MB since dereferencing the symlink will result in a size
	   over this amount. The results of InDelta() may depend on the underlying
	   file system. */
	suite.assert.InDelta(expectedUsageMB, usage, 0.1)
	suite.assert.Less(usage, 1.5)           // Should be much less than 2MB
	suite.assert.GreaterOrEqual(usage, 1.0) // Should be at least 1MB
}

func (suite *utilTestSuite) TestGetUsageWithSubdirectories() {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	tempDir := filepath.Join(pwd, "util_test_subdir")
	err = os.Mkdir(tempDir, 0777)
	suite.assert.Nil(err)
	defer os.RemoveAll(tempDir)

	data := make([]byte, 1024*1024)
	file1 := filepath.Join(tempDir, "file1.txt")
	err = os.WriteFile(file1, data, 0777)
	suite.assert.Nil(err)

	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0777)
	suite.assert.Nil(err)

	data2 := make([]byte, 2*1024*1024)
	file2 := filepath.Join(subDir, "file2.txt")
	err = os.WriteFile(file2, data2, 0777)
	suite.assert.Nil(err)

	subDir2 := filepath.Join(tempDir, "subdir2")
	err = os.Mkdir(subDir2, 0777)
	suite.assert.Nil(err)

	data3 := make([]byte, 512*1024)
	file3 := filepath.Join(subDir2, "file3.txt")
	err = os.WriteFile(file3, data3, 0777)
	suite.assert.Nil(err)

	dirInfo, err := os.Lstat(subDir)
	suite.assert.Nil(err)
	dirSize := dirInfo.Size()

	dirInfo2, err := os.Lstat(subDir2)
	suite.assert.Nil(err)
	dirSize2 := dirInfo2.Size()

	usage, err := GetUsage(tempDir)
	suite.assert.Nil(err)

	file1ExpectedSize := float64(1024 * 1024)
	file2ExpectedSize := float64(2 * 1024 * 1024)
	file3ExpectedSize := float64(512 * 1024)
	expectedSizeMB := (file1ExpectedSize + file2ExpectedSize + file3ExpectedSize + float64(dirSize+dirSize2)) / (1024 * 1024)

	suite.assert.InDelta(expectedSizeMB, usage, 0.1)
	suite.assert.GreaterOrEqual(usage, 3.5)
	suite.assert.Less(usage, 4.5)
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

func (s *utilTestSuite) TestGetMD5() {
	assert := assert.New(s.T())

	f, err := os.Create("abc.txt")
	assert.Nil(err)

	_, err = f.Write([]byte(randomString(50)))
	assert.Nil(err)

	f.Close()

	f, err = os.Open("abc.txt")
	assert.Nil(err)

	md5Sum, err := GetMD5(f)
	assert.Nil(err)
	assert.NotZero(md5Sum)

	f.Close()
	os.Remove("abc.txt")
}

func (s *utilTestSuite) TestComponentExists() {
	components := []string{
		"component1",
		"component2",
		"component3",
	}

	exists := ComponentInPipeline(components, "component1")
	s.Assert().True(exists)

	exists = ComponentInPipeline(components, "component4")
	s.Assert().False(exists)

}

func (s *utilTestSuite) TestValidatePipeline() {
	err := ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "azstorage"})
	s.Assert().NotNil(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "xload", "azstorage"})
	s.Assert().NotNil(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "xload", "azstorage"})
	s.Assert().NotNil(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "xload", "azstorage"})
	s.Assert().NotNil(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "azstorage"})
	s.Assert().Nil(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "azstorage"})
	s.Assert().Nil(err)

	err = ValidatePipeline([]string{"libfuse", "xload", "attr_cache", "azstorage"})
	s.Assert().Nil(err)
}

func (s *utilTestSuite) TestUpdatePipeline() {
	pipeline := UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "xload")
	s.Assert().NotNil(pipeline)
	s.Assert().False(ComponentInPipeline(pipeline, "file_cache"))
	s.Assert().Equal(pipeline, []string{"libfuse", "xload", "azstorage"})

	pipeline = UpdatePipeline([]string{"libfuse", "block_cache", "azstorage"}, "xload")
	s.Assert().NotNil(pipeline)
	s.Assert().False(ComponentInPipeline(pipeline, "block_cache"))
	s.Assert().Equal(pipeline, []string{"libfuse", "xload", "azstorage"})

	pipeline = UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "block_cache")
	s.Assert().NotNil(pipeline)
	s.Assert().False(ComponentInPipeline(pipeline, "file_cache"))
	s.Assert().Equal(pipeline, []string{"libfuse", "block_cache", "azstorage"})

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "block_cache")
	s.Assert().NotNil(pipeline)
	s.Assert().False(ComponentInPipeline(pipeline, "xload"))
	s.Assert().Equal(pipeline, []string{"libfuse", "block_cache", "azstorage"})

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "xload")
	s.Assert().NotNil(pipeline)
	s.Assert().Equal(pipeline, []string{"libfuse", "xload", "azstorage"})
}
