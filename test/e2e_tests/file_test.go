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

package e2e_tests

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var fileTestPathPtr string
var fileTestTempPathPtr string
var fileTestAdlsPtr string
var fileTestGitClonePtr string
var fileTestStreamDirectPtr string
var fileTestDistroName string
var fileTestEnableSymlinkADLS string

type fileTestSuite struct {
	suite.Suite
	testPath      string
	adlsTest      bool
	testCachePath string
	minBuff       []byte
	medBuff       []byte
	hugeBuff      []byte
}

func regFileTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getFileTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initFileFlags() {
	fileTestPathPtr = getFileTestFlag("mnt-path")
	fileTestAdlsPtr = getFileTestFlag("adls")
	fileTestTempPathPtr = getFileTestFlag("tmp-path")
	fileTestGitClonePtr = getFileTestFlag("clone")
	fileTestStreamDirectPtr = getFileTestFlag("stream-direct-test")
	fileTestDistroName = getFileTestFlag("distro-name")
	fileTestEnableSymlinkADLS = getFileTestFlag("enable-symlink-adls")
}

func getFileTestDirName(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *fileTestSuite) fileTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.Equal(nil, err)
	}
}

// // -------------- File Tests -------------------

// # Create file test
func (suite *fileTestSuite) TestFileCreate() {
	fileName := suite.testPath + "/small_write.txt"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestOpenFlag_O_TRUNC() {
	fileName := suite.testPath + "/test_on_open"
	buf := "foo"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	suite.Nil(err)
	bytesWritten, err := srcFile.Write([]byte(buf))
	suite.Equal(len(buf), bytesWritten)
	suite.Nil(err)
	err = srcFile.Close()
	suite.Nil(err)

	srcFile, err = os.OpenFile(fileName, os.O_WRONLY, 0666)
	suite.Nil(err)
	err = srcFile.Close()
	suite.Nil(err)

	fileInfo, err := os.Stat(fileName)
	suite.Equal(int64(len(buf)), fileInfo.Size())
	suite.Nil(err)

	srcFile, err = os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	suite.Nil(err)
	err = srcFile.Close()
	suite.Nil(err)

	fileInfo, err = os.Stat(fileName)
	suite.Equal(int64(0), fileInfo.Size())
	suite.Nil(err)
}

func (suite *fileTestSuite) TestFileCreateUtf8Char() {
	fileName := suite.testPath + "/भारत.txt"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreatSpclChar() {
	speclChar := "abcd%23ABCD%34123-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत.txt"
	fileName := suite.testPath + "/" + speclChar

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()
	time.Sleep(time.Second * 2)

	_, err = os.Stat(fileName)
	suite.Equal(nil, err)

	files, err := os.ReadDir(suite.testPath)
	suite.Equal(nil, err)
	suite.GreaterOrEqual(len(files), 1)

	found := false
	for _, file := range files {
		if file.Name() == speclChar {
			found = true
		}
	}
	suite.Equal(true, found)

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreatEncodeChar() {
	speclChar := "%282%29+class_history_by_item.log"
	fileName := suite.testPath + "/" + speclChar

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()
	time.Sleep(time.Second * 2)

	_, err = os.Stat(fileName)
	suite.Equal(nil, err)

	files, err := os.ReadDir(suite.testPath)
	suite.Equal(nil, err)
	suite.GreaterOrEqual(len(files), 1)

	found := false
	for _, file := range files {
		if file.Name() == speclChar {
			found = true
		}
	}
	suite.Equal(true, found)

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreateMultiSpclCharWithinSpclDir() {
	speclChar := "abcd%23ABCD%34123-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत.txt"
	speclDirName := suite.testPath + "/" + "abc%23%24%25efg-._~!$&'()*+,;=!@ΣΑΠΦΩ$भारत"
	secFile := speclDirName + "/" + "abcd123~!@#$%^&*()_+=-{}][\":;'?><,.|abcd123~!@#$%^&*()_+=-{}][\":;'?><,.|.txt"
	fileName := speclDirName + "/" + speclChar

	err := os.Mkdir(speclDirName, 0777)
	suite.Equal(nil, err)

	srcFile, err := os.OpenFile(secFile, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	srcFile, err = os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()
	time.Sleep(time.Second * 2)

	_, err = os.Stat(fileName)
	suite.Equal(nil, err)

	files, err := os.ReadDir(speclDirName)
	suite.Equal(nil, err)
	suite.GreaterOrEqual(len(files), 1)

	found := false
	for _, file := range files {
		if file.Name() == speclChar {
			found = true
		}
	}
	suite.Equal(true, found)

	suite.fileTestCleanup([]string{speclDirName})
}

func (suite *fileTestSuite) TestFileCreateLongName() {
	fileName := suite.testPath + "/Higher Call_ An Incredible True Story of Combat and Chivalry in the War-Torn Skies of World War II, A - Adam Makos & Larry Alexander.epub"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

func (suite *fileTestSuite) TestFileCreateSlashName() {
	fileName := suite.testPath + "/abcd\\efg.txt"

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName, suite.testPath + "/abcd"})
}

func (suite *fileTestSuite) TestFileCreateLabel() {
	fileName := suite.testPath + "/chunk_f13c48d4-5c1e-11ea-b41d-000d3afe1867.label"

	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	suite.fileTestCleanup([]string{fileName})
}

// # Write a small file
func (suite *fileTestSuite) TestFileWriteSmall() {
	fileName := suite.testPath + "/small_write.txt"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Equal(nil, err)

	suite.fileTestCleanup([]string{fileName})
}

// # Read a small file
func (suite *fileTestSuite) TestFileReadSmall() {
	fileName := suite.testPath + "/small_write.txt"
	srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Equal(nil, err)

	data, err := os.ReadFile(fileName)
	suite.Equal(nil, err)
	suite.Equal(len(data), len(suite.minBuff))

	suite.fileTestCleanup([]string{fileName})
}

// # Create duplicate file
func (suite *fileTestSuite) TestFileCreateDuplicate() {
	fileName := suite.testPath + "/small_write.txt"
	f, err := os.OpenFile(fileName, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	f.Close()

	f, err = os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()

	suite.fileTestCleanup([]string{fileName})
}

// # Truncate a file
func (suite *fileTestSuite) TestFileTruncate() {
	fileName := suite.testPath + "/small_write.txt"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()

	err = os.Truncate(fileName, 2)
	suite.Equal(nil, err)

	data, err := os.ReadFile(fileName)
	suite.Equal(nil, err)
	suite.LessOrEqual(2, len(data))

	suite.fileTestCleanup([]string{fileName})
}

// # Create file matching directory name
func (suite *fileTestSuite) TestFileNameConflict() {
	dirName := suite.testPath + "/test"
	fileName := suite.testPath + "/test.txt"

	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()

	err = os.RemoveAll(dirName)
	suite.Equal(nil, err)
}

// # Copy file from once directory to another
func (suite *fileTestSuite) TestFileCopy() {
	dirName := suite.testPath + "/test123"
	fileName := suite.testPath + "/test"
	dstFileName := dirName + "/test_copy.txt"

	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	srcFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0777)
	suite.Equal(nil, err)
	defer srcFile.Close()

	dstFile, err := os.Create(dstFileName)
	suite.Equal(nil, err)
	defer dstFile.Close()

	_, err = io.Copy(srcFile, dstFile)
	suite.Equal(nil, err)
	dstFile.Close()

	suite.fileTestCleanup([]string{dirName})
}

// # Get stats of a file
func (suite *fileTestSuite) TestFileGetStat() {
	fileName := suite.testPath + "/test"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()
	time.Sleep(time.Second * 3)

	stat, err := os.Stat(fileName)
	suite.Equal(nil, err)
	modTineDiff := time.Now().Sub(stat.ModTime())

	suite.Equal(false, stat.IsDir())
	suite.Equal("test", stat.Name())
	suite.LessOrEqual(modTineDiff.Hours(), float64(1))

	suite.fileTestCleanup([]string{fileName})
}

// # Change mod of file
func (suite *fileTestSuite) TestFileChmod() {
	if suite.adlsTest {
		fileName := suite.testPath + "/test"
		f, err := os.Create(fileName)
		suite.Equal(nil, err)
		f.Close()

		err = os.Chmod(fileName, 0744)
		suite.Equal(nil, err)
		stat, err := os.Stat(fileName)
		suite.Equal(nil, err)
		suite.Equal("-rwxr--r--", stat.Mode().Perm().String())

		suite.fileTestCleanup([]string{fileName})
	}
}

// # Create multiple med files
func (suite *fileTestSuite) TestFileCreateMulti() {
	if strings.ToLower(fileTestStreamDirectPtr) == "true" && strings.ToLower(fileTestDistroName) == "ubuntu-20.04" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	dirName := suite.testPath + "/multi_dir"
	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)
	fileName := dirName + "/multi"
	for i := 0; i < 10; i++ {
		newFile := fileName + strconv.Itoa(i)
		err := os.WriteFile(newFile, suite.medBuff, 0777)
		suite.Equal(nil, err)
	}
	suite.fileTestCleanup([]string{dirName})
}

// TODO: this test would always pass since its dependent on above tests - resources should be created only for it
// # Delete single files
func (suite *fileTestSuite) TestFileDeleteSingle() {
	fileName := suite.testPath + "/multi0"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()
	suite.fileTestCleanup([]string{fileName})
}

// // -------------- SymLink Tests -------------------

// # Create a symlink to a file
func (suite *fileTestSuite) TestLinkCreate() {
	fileName := suite.testPath + "/small_write1.txt"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()
	symName := suite.testPath + "/small.lnk"
	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Equal(nil, err)

	err = os.Symlink(fileName, symName)
	suite.Equal(nil, err)
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

// # Read a small file using symlink
func (suite *fileTestSuite) TestLinkRead() {
	fileName := suite.testPath + "/small_write1.txt"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()

	symName := suite.testPath + "/small.lnk"
	err = os.Symlink(fileName, symName)
	suite.Equal(nil, err)

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Equal(nil, err)
	data, err := os.ReadFile(fileName)
	suite.Equal(nil, err)
	suite.Equal(len(data), len(suite.minBuff))
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

// # Write a small file using symlink
func (suite *fileTestSuite) TestLinkWrite() {
	targetName := suite.testPath + "/small_write1.txt"
	f, err := os.Create(targetName)
	suite.Equal(nil, err)
	f.Close()
	symName := suite.testPath + "/small.lnk"
	err = os.Symlink(targetName, symName)
	suite.Equal(nil, err)

	stat, err := os.Stat(targetName)
	modTineDiff := time.Now().Sub(stat.ModTime())
	suite.Equal(nil, err)
	suite.LessOrEqual(modTineDiff.Minutes(), float64(1))
	suite.fileTestCleanup([]string{targetName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

// # Rename the target file and validate read on symlink fails
func (suite *fileTestSuite) TestLinkRenameTarget() {
	fileName := suite.testPath + "/small_write1.txt"
	symName := suite.testPath + "/small.lnk"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()
	err = os.Symlink(fileName, symName)
	suite.Equal(nil, err)

	fileNameNew := suite.testPath + "/small_write_new.txt"
	err = os.Rename(fileName, fileNameNew)
	suite.Equal(nil, err)

	_, err = os.ReadFile(symName)
	// we expect that to fail
	suite.NotEqual(nil, err)

	// rename back to original name
	err = os.Rename(fileNameNew, fileName)
	suite.Equal(nil, err)

	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

// # Delete the symklink and check target file is still intact
func (suite *fileTestSuite) TestLinkDeleteReadTarget() {
	fileName := suite.testPath + "/small_write1.txt"
	symName := suite.testPath + "/small.lnk"
	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()
	err = os.Symlink(fileName, symName)
	suite.Equal(nil, err)
	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Equal(nil, err)

	err = os.Remove(symName)
	suite.Equal(nil, err)

	data, err := os.ReadFile(fileName)
	suite.Equal(nil, err)
	suite.Equal(len(data), len(suite.minBuff))

	err = os.Symlink(fileName, symName)
	suite.Equal(nil, err)
	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

func (suite *fileTestSuite) TestListDirReadLink() {
	if suite.adlsTest && strings.ToLower(fileTestEnableSymlinkADLS) != "true" {
		fmt.Printf("Skipping this test case for adls : %v, enable-symlink-adls : %v\n", suite.adlsTest, fileTestEnableSymlinkADLS)
		return
	}

	fileName := suite.testPath + "/small_hns.txt"
	f, err := os.Create(fileName)
	suite.Nil(err)
	f.Close()

	err = os.WriteFile(fileName, suite.minBuff, 0777)
	suite.Nil(err)

	symName := suite.testPath + "/small_hns.lnk"
	err = os.Symlink(fileName, symName)
	suite.Nil(err)

	dl, err := os.ReadDir(suite.testPath)
	suite.Nil(err)
	suite.Greater(len(dl), 0)

	// temp cache cleanup
	suite.fileTestCleanup([]string{suite.testCachePath + "/small_hns.txt", suite.testCachePath + "/small_hns.lnk"})

	data1, err := os.ReadFile(symName)
	suite.Nil(err)
	suite.Equal(len(data1), len(suite.minBuff))

	// temp cache cleanup
	suite.fileTestCleanup([]string{suite.testCachePath + "/small_hns.txt", suite.testCachePath + "/small_hns.lnk"})

	data2, err := os.ReadFile(fileName)
	suite.Nil(err)
	suite.Equal(len(data2), len(suite.minBuff))

	// validating data
	suite.Equal(data1, data2)

	suite.fileTestCleanup([]string{fileName})
	err = os.Remove(symName)
	suite.Equal(nil, err)
}

/*
func (suite *fileTestSuite) TestReadOnlyFile() {
	if suite.adlsTest == true {
		fileName := suite.testPath + "/readOnlyFile.txt"
		srcFile, err := os.Create(fileName)
		suite.Equal(nil, err)
		srcFile.Close()
		// make it read only permissions
		err = os.Chmod(fileName, 0444)
		suite.Equal(nil, err)
		_, err = os.OpenFile(fileName, os.O_RDONLY, 0444)
		suite.Equal(nil, err)
		_, err = os.OpenFile(fileName, os.O_RDWR, 0444)
		suite.NotNil(err)
		suite.fileTestCleanup([]string{fileName})
	}
} */

func (suite *fileTestSuite) TestCreateReadOnlyFile() {
	if suite.adlsTest == true {
		fileName := suite.testPath + "/createReadOnlyFile.txt"
		srcFile, err := os.OpenFile(fileName, os.O_CREATE, 0444)
		srcFile.Close()
		suite.Equal(nil, err)
		_, err = os.OpenFile(fileName, os.O_RDONLY, 0444)
		suite.Equal(nil, err)
		suite.fileTestCleanup([]string{fileName})
	}
}

// # Rename with special character in name
func (suite *fileTestSuite) TestRenameSpecial() {
	dirName := suite.testPath + "/" + "Alcaldía"
	newDirName := suite.testPath + "/" + "Alδaδcaldía"
	fileName := dirName + "/" + "भारत.txt"
	newFileName := dirName + "/" + "भारतabcd.txt"

	err := os.Mkdir(dirName, 0777)
	suite.Equal(nil, err)

	f, err := os.Create(fileName)
	suite.Equal(nil, err)
	f.Close()

	err = os.Rename(fileName, newFileName)
	suite.Equal(nil, err)

	err = os.Rename(newFileName, fileName)
	suite.Equal(nil, err)

	err = os.Rename(dirName, newDirName)
	suite.Equal(nil, err)

	err = os.RemoveAll(newDirName)
	suite.Equal(nil, err)
}

// -------------- Main Method -------------------
func TestFileTestSuite(t *testing.T) {
	initFileFlags()
	fileTest := fileTestSuite{
		minBuff:  make([]byte, 1024),
		medBuff:  make([]byte, (10 * 1024 * 1024)),
		hugeBuff: make([]byte, (500 * 1024 * 1024)),
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getFileTestDirName(10)
	fmt.Println(testDirName)

	// Create directory for testing the End to End test on mount path
	fileTest.testPath = fileTestPathPtr + "/" + testDirName
	fmt.Println(fileTest.testPath)

	fileTest.testCachePath = fileTestTempPathPtr + "/" + testDirName
	fmt.Println(fileTest.testCachePath)

	if fileTestAdlsPtr == "true" || fileTestAdlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		fileTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(fileTest.testPath)
	if err != nil {
		fmt.Printf("Could not cleanup feature dir before testing [%s]\n", err.Error())
	}

	err = os.Mkdir(fileTest.testPath, 0777)
	if err != nil {
		t.Errorf("Failed to create test directory [%s]\n", err.Error())
	}
	rand.Read(fileTest.minBuff)
	rand.Read(fileTest.medBuff)

	// Run the actual End to End test
	suite.Run(t, &fileTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(fileTest.testPath)
}

func init() {
	regFileTestFlag(&fileTestPathPtr, "mnt-path", "", "Mount Path of Container")
	regFileTestFlag(&fileTestAdlsPtr, "adls", "", "Account is ADLS or not")
	regFileTestFlag(&fileTestGitClonePtr, "clone", "", "Git clone test is enable or not")
	regFileTestFlag(&fileTestTempPathPtr, "tmp-path", "", "Cache dir path")
	regFileTestFlag(&fileTestEnableSymlinkADLS, "enable-symlink-adls", "false", "Enable symlink support for ADLS accounts")
}
