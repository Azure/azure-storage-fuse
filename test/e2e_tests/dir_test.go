//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package e2e_tests

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type dirTestSuite struct {
	suite.Suite
	testPath      string
	adlsTest      bool
	testCachePath string
	minBuff       []byte
	medBuff       []byte
	hugeBuff      []byte
}

var pathPtr string
var tempPathPtr string
var adlsPtr string
var clonePtr string
var streamDirectPtr string
var enableSymlinkADLS string

func regDirTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getDirTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initDirFlags() {
	pathPtr = getDirTestFlag("mnt-path")
	adlsPtr = getDirTestFlag("adls")
	tempPathPtr = getDirTestFlag("tmp-path")
	clonePtr = getDirTestFlag("clone")
	streamDirectPtr = getDirTestFlag("stream-direct-test")
	enableSymlinkADLS = getDirTestFlag("enable-symlink-adls")
}

func getTestDirName(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dirTestSuite) dirTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.NoError(err)
	}
}

// -------------- Directory Tests -------------------

// # Create Directory with a simple name
func (suite *dirTestSuite) TestDirCreateSimple() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory that already exists
func (suite *dirTestSuite) TestDirCreateDuplicate() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)
	// duplicate dir - we expect to throw
	err = os.Mkdir(dirName, 0777)
	suite.Contains(err.Error(), "file exists")

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory with special characters in name
func (suite *dirTestSuite) TestDirCreateSplChar() {
	dirName := suite.testPath + "/" + "@#$^&*()_+=-{}[]|?><.,~"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory with slash in name
func (suite *dirTestSuite) TestDirCreateSlashChar() {
	dirName := suite.testPath + "/" + "PRQ\\STUV"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Rename a directory
func (suite *dirTestSuite) TestDirRename() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	newName := suite.testPath + "/test1_new"
	err = os.Rename(dirName, newName)
	suite.NoError(err)

	_, err = os.Stat(dirName)
	suite.True(os.IsNotExist(err))

	// cleanup
	suite.dirTestCleanup([]string{newName})
}

// # Move an empty directory
func (suite *dirTestSuite) TestDirMoveEmpty() {
	dir2Name := suite.testPath + "/test2"
	err := os.Mkdir(dir2Name, 0777)
	suite.NoError(err)

	dir3Name := suite.testPath + "/test3"
	err = os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Rename(dir2Name, dir3Name+"/test2")
	time.Sleep(1 * time.Second)
	suite.NoError(err)

	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// # Move an non-empty directory
func (suite *dirTestSuite) TestDirMoveNonEmpty() {
	dir2Name := suite.testPath + "/test2NE"
	err := os.Mkdir(dir2Name, 0777)
	suite.NoError(err)

	file1Name := dir2Name + "/test.txt"
	f, err := os.Create(file1Name)
	suite.NoError(err)
	f.Close()

	dir3Name := suite.testPath + "/test3NE"
	err = os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Mkdir(dir3Name+"/abcdTest", 0777)
	suite.NoError(err)

	err = os.Rename(dir2Name, dir3Name+"/test2")
	time.Sleep(1 * time.Second)
	suite.NoError(err)

	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteEmpty() {
	dirName := suite.testPath + "/test1_new"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	suite.dirTestCleanup([]string{dirName})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteNonEmpty() {
	dir3Name := suite.testPath + "/test3NE"
	err := os.Mkdir(dir3Name, 0777)
	suite.NoError(err)

	err = os.Mkdir(dir3Name+"/abcdTest", 0777)
	suite.NoError(err)

	err = os.Remove(dir3Name)
	suite.Error(err)
	suite.Contains(err.Error(), "directory not empty")

	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// // # Delete non-empty directory recursively
// func (suite *dirTestSuite) TestDirDeleteRecursive() {
// 	dirName := suite.testPath + "/testREC"

// 	err := os.Mkdir(dirName, 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(dirName+"/level1", 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(dirName+"/level2", 0777)
// 	suite.Equal(nil, err)

// 	err = os.Mkdir(dirName+"/level1/l1", 0777)
// 	suite.Equal(nil, err)

// 	srcFile, err := os.OpenFile(dirName+"/level2/abc.txt", os.O_CREATE, 0777)
// 	suite.Equal(nil, err)
// 	srcFile.Close()

// 	suite.dirTestCleanup([]string{dirName})
// }

// # Get stats of a directory
func (suite *dirTestSuite) TestDirGetStats() {
	dirName := suite.testPath + "/test3"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)
	// time.Sleep(2 * time.Second)

	stat, err := os.Stat(dirName)
	suite.NoError(err)
	modTineDiff := time.Since(stat.ModTime())

	// for directory block blob may still return timestamp as 0
	// So compare the time only if epoch is non-zero
	if stat.ModTime().Unix() != 0 {
		suite.True(stat.IsDir())
		suite.Equal("test3", stat.Name())
		suite.GreaterOrEqual(float64(1), modTineDiff.Hours())
	}
	// Cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Change mod of directory
func (suite *dirTestSuite) TestDirChmod() {
	if suite.adlsTest == true {
		dirName := suite.testPath + "/testchmod"
		err := os.Mkdir(dirName, 0777)
		suite.NoError(err)

		err = os.Chmod(dirName, 0744)
		suite.NoError(err)

		stat, err := os.Stat(dirName)
		suite.NoError(err)
		suite.Equal("-rwxr--r--", stat.Mode().Perm().String())

		suite.dirTestCleanup([]string{dirName})
	}
}

// # List directory
func (suite *dirTestSuite) TestDirList() {
	testDir := suite.testPath + "/bigTestDir"
	err := os.Mkdir(testDir, 0777)
	suite.NoError(err)

	dir := filepath.Join(testDir + "/Dir1")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)
	dir = filepath.Join(testDir + "/Dir2")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)
	dir = filepath.Join(testDir + "/Dir3")
	err = os.Mkdir(dir, 0777)
	suite.NoError(err)

	srcFile, err := os.OpenFile(testDir+"/abc.txt", os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	files, err := os.ReadDir(testDir)
	suite.NoError(err)
	suite.Len(files, 4)

	// Cleanup
	suite.dirTestCleanup([]string{testDir})
}

// // # List directory recursively
// func (suite *dirTestSuite) TestDirListRecursive() {
// 	testDir := suite.testPath + "/bigTestDir"
// 	err := os.Mkdir(testDir, 0777)
// 	suite.Equal(nil, err)

// 	dir := filepath.Join(testDir + "/Dir1")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	dir = filepath.Join(testDir + "/Dir2")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	dir = filepath.Join(testDir + "/Dir3")
// 	err = os.Mkdir(dir, 0777)
// 	suite.Equal(nil, err)

// 	srcFile, err := os.OpenFile(testDir+"/abc.txt", os.O_CREATE, 0777)
// 	suite.Equal(nil, err)
// 	srcFile.Close()

// 	var files []string
// 	err = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
// 		files = append(files, path)
// 		return nil
// 	})
// 	suite.Equal(nil, err)

// 	testFiles, err := os.ReadDir(testDir)
// 	suite.Equal(nil, err)
// 	suite.Equal(4, len(testFiles))

// 	// Cleanup
// 	suite.dirTestCleanup([]string{testDir})
// }

// // # Rename directory with data
func (suite *dirTestSuite) TestDirRenameFull() {
	if strings.ToLower(streamDirectPtr) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	dirName := suite.testPath + "/full_dir"
	newName := suite.testPath + "/full_dir_rename"
	fileName := dirName + "/test_file_"

	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	err = os.Mkdir(dirName+"/tmp", 0777)
	suite.NoError(err)

	for i := range 10 {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.medBuff, 0777)
		suite.NoError(err)
	}

	err = os.Rename(dirName, newName)
	suite.NoError(err)

	//  Deleted directory shall not be present in the container now
	_, err = os.Stat(dirName)
	suite.True(os.IsNotExist(err))

	_, err = os.Stat(newName)
	suite.False(os.IsNotExist(err))

	// this should fail as the new dir should be filled
	err = os.Remove(newName)
	suite.Error(err)

	// cleanup
	suite.dirTestCleanup([]string{newName})

}

func (suite *dirTestSuite) TestGitStash() {
	if strings.ToLower(streamDirectPtr) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if clonePtr == "true" || clonePtr == "True" {
		dirName := suite.testPath + "/stash"
		tarName := suite.testPath + "/tardir.tar.gz"

		cmd := exec.Command("git", "clone", "https://github.com/wastore/azure-storage-samples-for-net", dirName)
		_, err := cmd.Output()
		suite.NoError(err)

		_, err = os.Stat(dirName)
		suite.NoError(err)

		err = os.Chdir(dirName)
		suite.NoError(err)

		cmd = exec.Command("git", "status")
		cliOut, err := cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "nothing to commit, working")
		}

		f, err := os.OpenFile("README.md", os.O_WRONLY, 0644)
		suite.NoError(err)
		suite.NotZero(f)
		info, err := f.Stat()
		suite.NoError(err)
		_, err = f.WriteAt([]byte("TestString"), info.Size())
		suite.NoError(err)
		_ = f.Close()

		f, err = os.OpenFile("README.md", os.O_RDONLY, 0644)
		suite.NoError(err)
		suite.NotZero(f)
		new_info, err := f.Stat()
		suite.NoError(err)
		suite.Equal(info.Size()+10, new_info.Size())
		data := make([]byte, 10)
		n, err := f.ReadAt(data, info.Size())
		suite.NoError(err)
		suite.Equal(10, n)
		suite.Equal("TestString", string(data))
		err = f.Close()
		suite.NoError(err)

		cmd = exec.Command("git", "status")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Changes not staged for commit")
		}

		cmd = exec.Command("git", "stash")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Saved working directory and index state WIP")
		}

		cmd = exec.Command("git", "stash", "list")
		_, err = cmd.Output()
		suite.NoError(err)

		cmd = exec.Command("git", "stash", "pop")
		cliOut, err = cmd.Output()
		suite.NoError(err)
		if len(cliOut) > 0 {
			suite.Contains(string(cliOut), "Changes not staged for commit")
		}

		err = os.Chdir(suite.testPath)
		suite.NoError(err)

		// As Tar is taking long time first to clone and then to tar just mixing both the test cases
		cmd = exec.Command("tar", "-zcvf", tarName, dirName)
		cliOut, _ = cmd.Output()
		if len(cliOut) > 0 {
			suite.NotContains(cliOut, "file changed as we read it")
		}

		cmd = exec.Command("tar", "-zxvf", tarName, "--directory", dirName)
		_, _ = cmd.Output()

		os.RemoveAll(dirName)
		os.Remove("libfuse.tar.gz")
	}
}

func (suite *dirTestSuite) TestReadDirLink() {
	if suite.adlsTest && strings.ToLower(enableSymlinkADLS) != "true" {
		fmt.Printf("Skipping this test case for adls : %v, enable-symlink-adls : %v\n", suite.adlsTest, enableSymlinkADLS)
		return
	}

	dirName := suite.testPath + "/test_hns"
	err := os.Mkdir(dirName, 0777)
	suite.NoError(err)

	fileName := dirName + "/small_file.txt"
	f, err := os.Create(fileName)
	suite.NoError(err)
	f.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.NoError(err)

	symName := suite.testPath + "/dirlink.lnk"
	err = os.Symlink(dirName, symName)
	suite.NoError(err)

	dl, err := os.ReadDir(suite.testPath)
	suite.NoError(err)
	suite.NotEmpty(dl)

	// list operation on symlink
	dirLinkList, err := os.ReadDir(symName)
	suite.NoError(err)
	suite.NotEmpty(dirLinkList)

	dirList, err := os.ReadDir(dirName)
	suite.NoError(err)
	suite.NotEmpty(dirList)

	suite.Len(dirList, len(dirLinkList))

	// comparing list values since they are sorted by file name
	for i := range dirLinkList {
		suite.Equal(dirLinkList[i].Name(), dirList[i].Name())
	}

	// temp cache cleanup
	suite.dirTestCleanup([]string{suite.testCachePath + "/test_hns/small_file.txt", suite.testCachePath + "/dirlink.lnk"})

	data1, err := os.ReadFile(symName + "/small_file.txt")
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data1))

	// temp cache cleanup
	suite.dirTestCleanup([]string{suite.testCachePath + "/test_hns/small_file.txt", suite.testCachePath + "/dirlink.lnk"})

	data2, err := os.ReadFile(fileName)
	suite.NoError(err)
	suite.Len(suite.minBuff, len(data2))

	// validating data
	suite.Equal(data1, data2)

	suite.dirTestCleanup([]string{dirName})
	err = os.Remove(symName)
	suite.NoError(err)
}

// -------------- Main Method -------------------
func TestDirTestSuite(t *testing.T) {
	initDirFlags()
	dirTest := dirTestSuite{
		minBuff:  make([]byte, 1024),
		medBuff:  make([]byte, (10 * 1024 * 1024)),
		hugeBuff: make([]byte, (500 * 1024 * 1024)),
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getTestDirName(10)
	fmt.Println(testDirName)

	// Create directory for testing the End to End test on mount path
	dirTest.testPath = pathPtr + "/" + testDirName
	fmt.Println(dirTest.testPath)

	dirTest.testCachePath = tempPathPtr + "/" + testDirName
	fmt.Println(dirTest.testCachePath)

	if adlsPtr == "true" || adlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		dirTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(dirTest.testPath)
	if err != nil {
		fmt.Printf("Could not cleanup feature dir before testing [%s]\n", err.Error())
	}

	err = os.Mkdir(dirTest.testPath, 0777)
	if err != nil {
		t.Errorf("Failed to create test directory [%s]\n", err.Error())
	}
	_, _ = rand.Read(dirTest.minBuff)
	_, _ = rand.Read(dirTest.medBuff)
	_, _ = rand.Read(dirTest.hugeBuff)

	// Run the actual End to End test
	suite.Run(t, &dirTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(dirTest.testPath)
}

func init() {
	regDirTestFlag(&pathPtr, "mnt-path", "", "Mount Path of Container")
	regDirTestFlag(&adlsPtr, "adls", "", "Account is ADLS or not")
	regDirTestFlag(&clonePtr, "clone", "", "Git clone test is enable or not")
	regDirTestFlag(&tempPathPtr, "tmp-path", "", "Cache dir path")
	regDirTestFlag(&enableSymlinkADLS, "enable-symlink-adls", "false", "Enable symlink support for ADLS accounts")
}
