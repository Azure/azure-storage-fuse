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

package e2e_tests

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type dirTestSuite struct {
	suite.Suite
	testPath string
	adlsTest bool
	sasTest  bool
	minBuff  []byte
	medBuff  []byte
	hugeBuff []byte
}

var pathPtr string
var adlsPtr string
var sasPtr string
var clonePtr string

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
	sasPtr = getDirTestFlag("sas")
	clonePtr = getDirTestFlag("clone")
}

func getTestDirName(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dirTestSuite) dirTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.Equal(nil, err)
	}
}

// -------------- Directory Tests -------------------

// # Create Directory with a simple name
func (suite *dirTestSuite) TestDirCreateSimple() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory that already exists
func (suite *dirTestSuite) TestDirCreateDuplicate() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)
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
	suite.Equal(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Create Directory with slash in name
func (suite *dirTestSuite) TestDirCreateSlashChar() {
	dirName := suite.testPath + "/" + "PRQ\\STUV"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Rename a directory
func (suite *dirTestSuite) TestDirRename() {
	dirName := suite.testPath + "/test1"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	newName := suite.testPath + "/test1_new"
	err = os.Rename(dirName, newName)
	suite.Equal(nil, err)

	_, err = os.Stat(dirName)
	suite.Equal(true, os.IsNotExist(err))

	// cleanup
	suite.dirTestCleanup([]string{newName})
}

// # Move an empty directory
func (suite *dirTestSuite) TestDirMoveEmpty() {
	dir2Name := suite.testPath + "/test2"
	err := os.Mkdir(dir2Name, 0777)
	suite.Equal(nil, err)

	dir3Name := suite.testPath + "/test3"
	err = os.Mkdir(dir3Name, 0777)
	suite.Equal(nil, err)

	err = os.Rename(dir2Name, dir3Name+"/test2")
	time.Sleep(1 * time.Second)
	suite.Equal(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// # Move an non-empty directory
func (suite *dirTestSuite) TestDirMoveNonEmpty() {
	dir2Name := suite.testPath + "/test2NE"
	err := os.Mkdir(dir2Name, 0777)
	suite.Equal(nil, err)

	file1Name := dir2Name + "/test.txt"
	f, err := os.Create(file1Name)
	suite.Equal(nil, err)
	f.Close()

	dir3Name := suite.testPath + "/test3NE"
	err = os.Mkdir(dir3Name, 0777)
	suite.Equal(nil, err)

	err = os.Mkdir(dir3Name+"/abcdTest", 0777)
	suite.Equal(nil, err)

	err = os.Rename(dir2Name, dir3Name+"/test2")
	time.Sleep(1 * time.Second)
	suite.Equal(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{dir3Name})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteEmpty() {
	dirName := suite.testPath + "/test1_new"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	suite.dirTestCleanup([]string{dirName})
}

// # Delete non-empty directory
func (suite *dirTestSuite) TestDirDeleteNonEmpty() {
	dir3Name := suite.testPath + "/test3NE"
	err := os.Mkdir(dir3Name, 0777)
	suite.Equal(nil, err)

	err = os.Mkdir(dir3Name+"/abcdTest", 0777)
	suite.Equal(nil, err)

	err = os.Remove(dir3Name)
	suite.NotNil(err)
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
	suite.Equal(nil, err)
	// time.Sleep(2 * time.Second)

	stat, err := os.Stat(dirName)
	suite.Equal(nil, err)
	modTineDiff := time.Now().Sub(stat.ModTime())

	// for directory block blob may still return timestamp as 0
	// So compare the time only if epoch is non-zero
	if stat.ModTime().Unix() != 0 {
		suite.Equal(true, stat.IsDir())
		suite.Equal("test3", stat.Name())
		suite.GreaterOrEqual(float64(1), modTineDiff.Hours())
	}
	// Cleanup
	suite.dirTestCleanup([]string{dirName})
}

// # Change mod of directory
func (suite *dirTestSuite) TestDirChmod() {
	if suite.adlsTest == true {
		dirName := suite.testPath + "/test3"
		err := os.Mkdir(dirName, 0777)
		suite.Equal(nil, err)

		err = os.Chmod(dirName, 0744)
		suite.Equal(nil, err)

		stat, err := os.Stat(dirName)
		suite.Equal(nil, err)
		suite.Equal("-rwxr--r--", stat.Mode().Perm().String())

		suite.dirTestCleanup([]string{dirName})
	}
}

// # List directory
func (suite *dirTestSuite) TestDirList() {
	testDir := suite.testPath + "/bigTestDir"
	err := os.Mkdir(testDir, 0777)
	suite.Equal(nil, err)

	dir := filepath.Join(testDir + "/Dir1")
	err = os.Mkdir(dir, 0777)
	suite.Equal(nil, err)
	dir = filepath.Join(testDir + "/Dir2")
	err = os.Mkdir(dir, 0777)
	suite.Equal(nil, err)
	dir = filepath.Join(testDir + "/Dir3")
	err = os.Mkdir(dir, 0777)
	suite.Equal(nil, err)

	srcFile, err := os.OpenFile(testDir+"/abc.txt", os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	files, err := ioutil.ReadDir(testDir)
	suite.Equal(nil, err)
	suite.Equal(4, len(files))

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

// 	testFiles, err := ioutil.ReadDir(testDir)
// 	suite.Equal(nil, err)
// 	suite.Equal(4, len(testFiles))

// 	// Cleanup
// 	suite.dirTestCleanup([]string{testDir})
// }

// // # Rename directory with data
func (suite *dirTestSuite) TestDirRenameFull() {
	dirName := suite.testPath + "/full_dir"
	newName := suite.testPath + "/full_dir_rename"
	fileName := dirName + "/test_file_"

	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	err = os.Mkdir(dirName+"/tmp", 0777)
	suite.Equal(nil, err)

	for i := 0; i < 10; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := ioutil.WriteFile(newFile, suite.medBuff, 0777)
		suite.Equal(nil, err)
	}

	err = os.Rename(dirName, newName)
	suite.Equal(nil, err)

	//  Deleted directory shall not be present in the container now
	_, err = os.Stat(dirName)
	suite.Equal(true, os.IsNotExist(err))

	_, err = os.Stat(newName)
	suite.Equal(false, os.IsNotExist(err))

	// this should fail as the new dir should be filled
	err = os.Remove(newName)
	suite.NotEqual(nil, err)

	// cleanup
	suite.dirTestCleanup([]string{newName})

}

func (suite *dirTestSuite) TestTarDir() {
	dirName := suite.testPath + "/clone"
	tarName := suite.testPath + "/libfuse.tar.gz"

	cmd := exec.Command("git", "clone", "https://github.com/libfuse/libfuse", dirName)
	_, err := cmd.Output()
	suite.Equal(nil, err)

	_, err = os.Stat(dirName)
	suite.Equal(nil, err)

	cmd = exec.Command("tar", "-zcvf", tarName, dirName)
	cliOut, err := cmd.Output()
	suite.Equal(nil, err)
	suite.NotContains(cliOut, "file changed as we read it")

	cmd = exec.Command("tar", "-zxvf", tarName, "--directory", dirName)
	_, err = cmd.Output()
	suite.Equal(nil, err)

	os.RemoveAll(dirName)
	os.Remove("libfuse.tar.gz")
}

func (suite *dirTestSuite) TestGitClone() {
	if clonePtr == "true" || clonePtr == "True" {
		dirName := suite.testPath + "/clone"

		cmd := exec.Command("git", "clone", "https://github.com/libfuse/libfuse", dirName)
		_, err := cmd.Output()
		suite.Equal(nil, err)

		_, err = os.Stat(dirName)
		suite.Equal(nil, err)

		_, err = os.Stat(dirName + "/.git")
		suite.Equal(nil, err)

		os.RemoveAll(dirName)
	}
}

func (suite *dirTestSuite) TestGitStatus() {
	if clonePtr == "true" || clonePtr == "True" {
		dirName := suite.testPath + "/clone"

		cmd := exec.Command("git", "clone", "https://github.com/libfuse/libfuse", dirName)
		_, err := cmd.Output()
		suite.Equal(nil, err)

		_, err = os.Stat(dirName)
		suite.Equal(nil, err)

		_, err = os.Stat(dirName + "/.git")
		suite.Equal(nil, err)

		err = os.Chdir(dirName)
		suite.Equal(nil, err)

		cmd = exec.Command("git", "status")
		cliOut, err := cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "nothing to commit, working tree clean")

		f, err := os.OpenFile("README.md", os.O_APPEND|os.O_WRONLY, 0644)
		suite.Equal(nil, err)
		suite.NotZero(f)
		_, err = f.WriteString("TestString")
		suite.Equal(nil, err)
		_ = f.Close()

		cmd = exec.Command("git", "status")
		cliOut, err = cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "Changes not staged for commit")

		os.Chdir(suite.testPath)
		os.RemoveAll(dirName)
	}
}

func (suite *dirTestSuite) TestGitStash() {
	if clonePtr == "true" || clonePtr == "True" {
		dirName := suite.testPath + "/clone"

		cmd := exec.Command("git", "clone", "https://github.com/libfuse/libfuse", dirName)
		_, err := cmd.Output()
		suite.Equal(nil, err)

		_, err = os.Stat(dirName)
		suite.Equal(nil, err)

		_, err = os.Stat(dirName + "/.git")
		suite.Equal(nil, err)

		err = os.Chdir(dirName)
		suite.Equal(nil, err)

		cmd = exec.Command("git", "status")
		cliOut, err := cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "nothing to commit, working tree clean")

		f, err := os.OpenFile("README.md", os.O_APPEND|os.O_WRONLY, 0644)
		suite.Equal(nil, err)
		suite.NotZero(f)
		_, err = f.WriteString("TestString")
		suite.Equal(nil, err)
		_ = f.Close()

		cmd = exec.Command("git", "status")
		cliOut, err = cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "Changes not staged for commit")

		cmd = exec.Command("git", "stash")
		cliOut, err = cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "Saved working directory and index state WIP")

		cmd = exec.Command("git", "stash", "list")
		_, err = cmd.Output()
		suite.Equal(nil, err)

		cmd = exec.Command("git", "stash", "pop")
		cliOut, err = cmd.Output()
		suite.Equal(nil, err)
		suite.Contains(string(cliOut), "Changes not staged for commit")

		os.Chdir(suite.testPath)
		os.RemoveAll(dirName)
	}
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

	// Create directory for testing the End to End test on mount path
	dirTest.testPath = pathPtr + "/" + testDirName
	if adlsPtr == "true" || adlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		dirTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	if sasPtr == "true" || sasPtr == "True" {
		fmt.Println("SAS Testing...")
		dirTest.sasTest = true
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(dirTest.testPath)
	if err != nil {
		fmt.Println("Could not cleanup feature dir before testing")
	}

	err = os.Mkdir(dirTest.testPath, 0777)
	if err != nil {
		t.Error("Failed to create test directory")
	}
	rand.Read(dirTest.minBuff)
	rand.Read(dirTest.medBuff)
	rand.Read(dirTest.hugeBuff)

	// Run the actual End to End test
	suite.Run(t, &dirTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(dirTest.testPath)
}

func init() {
	regDirTestFlag(&pathPtr, "mnt-path", "", "Mount Path of Container")
	regDirTestFlag(&adlsPtr, "adls", "", "Account is ADLS or not")
	regDirTestFlag(&sasPtr, "sas", "", "Auth is SAS or not")
	regFileTestFlag(&fileTestGitClonePtr, "clone", "", "Git clone test is enable or not")
}
