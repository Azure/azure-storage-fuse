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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

func randomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
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

func (suite *utilTestSuite) TestThreadSafeBitmap() {
	var bitmap BitMap64

	start := make(chan bool)
	var wg sync.WaitGroup

	set := func() {
		defer wg.Done()
		<-start
		for i := range 100000 {
			bitmap.Set(uint64(i % 64))
		}
	}

	access := func() {
		defer wg.Done()
		<-start
		for i := range 100000 {
			bitmap.IsSet(uint64(i % 64))
		}
	}

	_clear := func() {
		defer wg.Done()
		<-start
		for i := range 100000 {
			bitmap.Clear(uint64(i % 64))
		}
	}

	resetBitmap := func() {
		defer wg.Done()
		<-start
		for range 100000 {
			bitmap.Reset()
		}
	}

	wg.Add(4)
	go set()
	go access()
	go _clear()
	go resetBitmap()
	close(start)
	wg.Wait()
}

func (suite *utilTestSuite) TestBitmapSetIsSetClear() {
	var bitmap BitMap64

	for i := uint64(0); i < 1000; i++ {
		j := i % 64
		ok := bitmap.Set(j)
		// first time setting the bit should return true
		suite.assert.True(ok)
		for k := uint64(0); k < 64; k++ {
			if k == j {
				suite.assert.True(bitmap.IsSet(k))
			} else {
				suite.assert.False(bitmap.IsSet(k))
			}
		}

		ok = bitmap.Set(j)
		// Second time setting the bit should return true
		suite.assert.False(ok)

		ok = bitmap.Clear(j)
		// first time clearing the bit should return true
		suite.assert.True(ok)
		suite.assert.False(bitmap.IsSet(j))

		ok = bitmap.Clear(j)
		// second time clearing the bit should return false
		suite.assert.False(ok)
		suite.assert.False(bitmap.IsSet(j))

		for k := uint64(0); k < 64; k++ {
			suite.assert.False(bitmap.IsSet(k))
		}
	}
}

func (suite *utilTestSuite) TestBitmapReset() {
	var bitmap BitMap64

	for i := uint64(0); i < 64; i++ {
		bitmap.Set(i)
	}

	ok := bitmap.Reset()
	// Reset should return true if any bit was set
	suite.assert.True(ok)

	for i := uint64(0); i < 64; i++ {
		suite.assert.False(bitmap.IsSet(i))
	}

	ok = bitmap.Reset()
	// Reset should return false if no bit was set
	suite.assert.False(ok)
}

func (suite *utilTestSuite) TestIsMountActiveNoMount() {
	var out bytes.Buffer
	cmd := exec.Command("../blobfuse2", "unmount", "all")
	cmd.Stdout = &out
	err := cmd.Run()
	suite.assert.NoError(err)
	cmd = exec.Command("pidof", "blobfuse2")
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Equal("exit status 1", err.Error())
	res, err := IsMountActive("/mnt/blobfuse")
	suite.assert.NoError(err)
	suite.assert.False(res)
}

func (suite *utilTestSuite) TestIsMountActiveTwoMounts() {
	var out bytes.Buffer

	// Define the file name and the content you want to write
	fileName := "config.yaml"

	lbpath := filepath.Join(home_dir, "lbpath")
	err := os.MkdirAll(lbpath, 0777)
	suite.assert.NoError(err)
	defer os.RemoveAll(lbpath)

	content := "components:\n" +
		"  - libfuse\n" +
		"  - loopbackfs\n\n" +
		"loopbackfs:\n" +
		"  path: " + lbpath + "\n\n"

	mntdir := filepath.Join(home_dir, "mountdir")
	err = os.MkdirAll(mntdir, 0777)
	suite.assert.NoError(err)
	defer os.RemoveAll(mntdir)

	dir, err := os.Getwd()
	suite.assert.NoError(err)
	configFile := filepath.Join(dir, "config.yaml")
	// Create or open the file. If it doesn't exist, it will be created.
	file, err := os.Create(fileName)
	suite.assert.NoError(err)
	defer file.Close() // Ensure the file is closed after we're done

	// Write the content to the file
	_, err = file.WriteString(content)
	suite.assert.NoError(err)

	err = os.Chdir("..")
	suite.assert.NoError(err)

	dir, err = os.Getwd()
	suite.assert.NoError(err)
	binary := filepath.Join(dir, "blobfuse2")
	cmd := exec.Command(binary, mntdir, "--config-file", configFile)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)

	res, err := IsMountActive(mntdir)
	suite.assert.NoError(err)
	suite.assert.True(res)

	res, err = IsMountActive("/mnt/blobfuse")
	suite.assert.NoError(err)
	suite.assert.False(res)

	cmd = exec.Command(binary, "unmount", mntdir)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
}

func (suite *typesTestSuite) TestDirectoryExists() {
	rand := randomString(8)
	dir := filepath.Join(home_dir, "dir"+rand)
	err := os.MkdirAll(dir, 0777)
	suite.assert.NoError(err)
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
	_, err := rand.Read(key)
	suite.assert.NoError(err)

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	suite.assert.NoError(err)

	_, err = EncryptData(data, key)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestDecryptBadKey() {
	// Generate a random key
	key := make([]byte, 20)
	_, err := rand.Read(key)
	suite.assert.NoError(err)

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	suite.assert.NoError(err)

	_, err = DecryptData(data, key)
	suite.assert.Error(err)
}

func (suite *typesTestSuite) TestEncryptDecrypt() {
	// Generate a random key
	key := make([]byte, 16)
	_, err := rand.Read(key)
	suite.assert.NoError(err)

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	suite.assert.NoError(err)

	cipher, err := EncryptData(data, key)
	suite.assert.NoError(err)

	d, err := DecryptData(cipher, key)
	suite.assert.NoError(err)
	suite.assert.Equal(data, d)
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
	suite.assert.NoError(err)

	data := make([]byte, 1024*1024)
	err = os.WriteFile(dirName+"/1.txt", data, 0777)
	suite.assert.NoError(err)

	err = os.WriteFile(dirName+"/2.txt", data, 0777)
	suite.assert.NoError(err)

	usage, err := GetUsage(dirName)
	suite.assert.NoError(err)
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
	suite.assert.NoError(err)

	usage, usagePercent, err := GetDiskUsageFromStatfs(dirName)
	suite.assert.NoError(err)
	suite.assert.NotEqual(0, usage)
	suite.assert.NotEqual(0, usagePercent)
	suite.assert.NotEqual(100, usagePercent)
	_ = os.RemoveAll(filepath.Join(pwd, "util_test"))
}

func (suite *utilTestSuite) TestDirectoryCleanup() {
	dirName := "./TestDirectoryCleanup"

	// Directory does not exists
	exists := DirectoryExists(dirName)
	suite.assert.False(exists)

	err := TempCacheCleanup(dirName)
	suite.assert.NoError(err)

	// Directory exists but is empty
	_ = os.MkdirAll(dirName, 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty := IsDirectoryEmpty(dirName)
	suite.assert.True(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.NoError(err)

	// Directory exists and is not empty
	_ = os.MkdirAll(dirName+"/A", 0777)
	exists = DirectoryExists(dirName)
	suite.assert.True(exists)

	empty = IsDirectoryEmpty(dirName)
	suite.assert.False(empty)

	err = TempCacheCleanup(dirName)
	suite.assert.NoError(err)

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
	suite.assert.NoError(err)

	// Check if file exists
	suite.assert.FileExists(filePath)

	// Check the content of the file
	data, err := os.ReadFile(filePath)
	suite.assert.NoError(err)
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

func (suite *utilTestSuite) TestGetMD5() {
	assert := assert.New(suite.T())

	f, err := os.Create("abc.txt")
	assert.NoError(err)

	_, err = f.Write([]byte(randomString(50)))
	assert.NoError(err)

	f.Close()

	f, err = os.Open("abc.txt")
	assert.NoError(err)

	md5Sum, err := GetMD5(f)
	assert.NoError(err)
	assert.NotZero(md5Sum)

	f.Close()
	os.Remove("abc.txt")
}

func (suite *utilTestSuite) TestComponentExists() {
	components := []string{
		"component1",
		"component2",
		"component3",
	}

	exists := ComponentInPipeline(components, "component1")
	suite.True(exists)

	exists = ComponentInPipeline(components, "component4")
	suite.False(exists)

}

func (suite *utilTestSuite) TestValidatePipeline() {
	err := ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "azstorage"})
	suite.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "xload", "azstorage"})
	suite.Error(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "xload", "azstorage"})
	suite.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "block_cache", "xload", "azstorage"})
	suite.Error(err)

	err = ValidatePipeline([]string{"libfuse", "file_cache", "azstorage"})
	suite.NoError(err)

	err = ValidatePipeline([]string{"libfuse", "block_cache", "azstorage"})
	suite.NoError(err)

	err = ValidatePipeline([]string{"libfuse", "xload", "attr_cache", "azstorage"})
	suite.NoError(err)
}

func (suite *utilTestSuite) TestUpdatePipeline() {
	pipeline := UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "xload")
	suite.NotNil(pipeline)
	suite.False(ComponentInPipeline(pipeline, "file_cache"))
	suite.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "block_cache", "azstorage"}, "xload")
	suite.NotNil(pipeline)
	suite.False(ComponentInPipeline(pipeline, "block_cache"))
	suite.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "file_cache", "azstorage"}, "block_cache")
	suite.NotNil(pipeline)
	suite.False(ComponentInPipeline(pipeline, "file_cache"))
	suite.Equal([]string{"libfuse", "block_cache", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "block_cache")
	suite.NotNil(pipeline)
	suite.False(ComponentInPipeline(pipeline, "xload"))
	suite.Equal([]string{"libfuse", "block_cache", "azstorage"}, pipeline)

	pipeline = UpdatePipeline([]string{"libfuse", "xload", "azstorage"}, "xload")
	suite.NotNil(pipeline)
	suite.Equal([]string{"libfuse", "xload", "azstorage"}, pipeline)
}

func TestPrettyOpenFlags(t *testing.T) {
	tests := []struct {
		name string
		flag int
		want string
	}{
		{
			name: "read only",
			flag: os.O_RDONLY,
			want: "[O_RDONLY]",
		},
		{
			name: "write only",
			flag: os.O_WRONLY,
			want: "[O_WRONLY]",
		},
		{
			name: "read write",
			flag: os.O_RDWR,
			want: "[O_RDWR]",
		},
		{
			name: "rdwr create trunc",
			flag: os.O_RDWR | os.O_CREATE | os.O_TRUNC,
			// access first, then flags in flagNames order
			want: "[O_RDWR | O_CREATE | O_TRUNC]",
		},
		{
			name: "wronly append",
			flag: os.O_WRONLY | os.O_APPEND,
			want: "[O_WRONLY | O_APPEND]",
		},
		{
			name: "rdwr append create excl sync trunc",
			flag: os.O_RDWR | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_SYNC | os.O_TRUNC,
			want: "[O_RDWR | O_APPEND | O_CREATE | O_EXCL | O_SYNC | O_TRUNC]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrettyOpenFlags(tt.flag)
			if got != tt.want {
				t.Fatalf("PrettyOpenFlags(%#x) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}
