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

package e2e_tests

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var dataValidationMntPathPtr string
var dataValidationTempPathPtr string
var dataValidationAdlsPtr string
var quickTest string
var streamDirectTest string
var distro string

var minBuff, medBuff, largeBuff, hugeBuff []byte

type dataValidationTestSuite struct {
	suite.Suite
	testMntPath   string
	testLocalPath string
	testCachePath string
	adlsTest      bool
}

func regDataValidationTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getDataValidationTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initDataValidationFlags() {
	dataValidationMntPathPtr = getDataValidationTestFlag("mnt-path")
	dataValidationAdlsPtr = getDataValidationTestFlag("adls")
	dataValidationTempPathPtr = getDataValidationTestFlag("tmp-path")
	quickTest = getDataValidationTestFlag("quick-test")
	streamDirectTest = getDataValidationTestFlag("stream-direct-test")
	distro = getDataValidationTestFlag("distro-name")
}

func getDataValidationTestDirName(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dataValidationTestSuite) dataValidationTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.Equal(nil, err)
	}
}

func (suite *dataValidationTestSuite) copyToMountDir(localFilePath string, remoteFilePath string) {
	// copy to mounted directory
	cpCmd := exec.Command("cp", localFilePath, remoteFilePath)
	cliOut, err := cpCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.Equal(nil, err)
}

func (suite *dataValidationTestSuite) validateData(localFilePath string, remoteFilePath string) {
	// compare the local and mounted files
	diffCmd := exec.Command("diff", localFilePath, remoteFilePath)
	cliOut, err := diffCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)
}

// -------------- Data Validation Tests -------------------

// Test correct overwrite of file using echo command
func (suite *dataValidationTestSuite) TestFileOverwriteWithEchoCommand() {

	if strings.Contains(strings.ToUpper(distro), "UBUNTU-20.04") {
		fmt.Println("Skipping this test case for UBUNTU-20.04")
		return
	}

	remoteFilePath := filepath.Join(suite.testMntPath, "TESTFORECHO.txt")
	text := "Hello, this is a test."
	command := "echo \"" + text + "\" > " + remoteFilePath
	cmd := exec.Command("/bin/bash", "-c", command)
	_, err := cmd.Output()
	suite.Equal(err, nil)

	data, err := os.ReadFile(remoteFilePath)
	suite.Nil(err)
	suite.Equal(string(data), text+"\n")

	newtext := "End of test."
	newcommand := "echo \"" + newtext + "\" > " + remoteFilePath
	newcmd := exec.Command("/bin/bash", "-c", newcommand)
	_, err = newcmd.Output()
	suite.Equal(err, nil)

	data, err = os.ReadFile(remoteFilePath)
	suite.Nil(err)
	suite.Equal(string(data), newtext+"\n")
}

// data validation for small sized files
func (suite *dataValidationTestSuite) TestSmallFileData() {
	fileName := "small_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for medium sized files
func (suite *dataValidationTestSuite) TestMediumFileData() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "medium_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, medBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for large sized files
func (suite *dataValidationTestSuite) TestLargeFileData() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "large_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, largeBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// negative test case for data validation where the local file is updated
func (suite *dataValidationTestSuite) TestDataValidationNegative() {
	fileName := "updated_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.Equal(nil, err)

	// copy local file to mounted directory
	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	// update local file
	srcFile, err = os.OpenFile(localFilePath, os.O_APPEND|os.O_WRONLY, 0777)
	suite.Equal(nil, err)
	_, err = srcFile.WriteString("Added text")
	srcFile.Close()
	suite.Equal(nil, err)

	// compare local file and mounted files
	diffCmd := exec.Command("diff", localFilePath, remoteFilePath)
	cliOut, err := diffCmd.Output()
	fmt.Println("Negative test case where files should differ")
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

func validateMultipleFilesData(jobs <-chan int, results chan<- string, fileSize string, suite *dataValidationTestSuite) {
	for i := range jobs {
		fileName := fileSize + strconv.Itoa(i) + ".txt"
		localFilePath := suite.testLocalPath + "/" + fileName
		remoteFilePath := suite.testMntPath + "/" + fileName
		fmt.Println("Local file path: " + localFilePath)

		// create the file in local directory
		srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
		suite.Equal(nil, err)
		srcFile.Close()

		// write to file in the local directory
		if fileSize == "huge" {
			err = os.WriteFile(localFilePath, hugeBuff, 0777)
		} else if fileSize == "large" {
			if strings.ToLower(quickTest) == "true" {
				err = os.WriteFile(localFilePath, hugeBuff, 0777)
			} else {
				err = os.WriteFile(localFilePath, largeBuff, 0777)
			}
		} else if fileSize == "medium" {
			err = os.WriteFile(localFilePath, medBuff, 0777)
		} else {
			err = os.WriteFile(localFilePath, minBuff, 0777)
		}
		suite.Equal(nil, err)

		suite.copyToMountDir(localFilePath, remoteFilePath)
		suite.dataValidationTestCleanup([]string{suite.testCachePath + "/" + fileName})
		suite.validateData(localFilePath, remoteFilePath)

		suite.dataValidationTestCleanup([]string{localFilePath, suite.testCachePath + "/" + fileName})

		results <- remoteFilePath
	}
}

func createThreadPool(noOfFiles int, noOfWorkers int, fileSize string, suite *dataValidationTestSuite) {
	jobs := make(chan int, noOfFiles)
	results := make(chan string, noOfFiles)

	for i := 1; i <= noOfWorkers; i++ {
		go validateMultipleFilesData(jobs, results, fileSize, suite)
	}

	for i := 1; i <= noOfFiles; i++ {
		jobs <- i
	}
	close(jobs)

	for i := 1; i <= noOfFiles; i++ {
		filePath := <-results
		os.Remove(filePath)
	}
	close(results)

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

func (suite *dataValidationTestSuite) TestMultipleSmallFiles() {
	noOfFiles := 16
	noOfWorkers := 4
	createThreadPool(noOfFiles, noOfWorkers, "small", suite)
}

func (suite *dataValidationTestSuite) TestMultipleMediumFiles() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.Contains(strings.ToUpper(distro), "RHEL") {
		fmt.Println("Skipping this test case for RHEL")
		return
	}

	noOfFiles := 8
	noOfWorkers := 4
	createThreadPool(noOfFiles, noOfWorkers, "medium", suite)
}

func (suite *dataValidationTestSuite) TestMultipleLargeFiles() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.Contains(strings.ToUpper(distro), "RHEL") {
		fmt.Println("Skipping this test case for RHEL")
		return
	}

	noOfFiles := 4
	noOfWorkers := 2
	createThreadPool(noOfFiles, noOfWorkers, "large", suite)
}

func (suite *dataValidationTestSuite) TestMultipleHugeFiles() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	if strings.ToLower(quickTest) == "true" {
		fmt.Println("Quick test is enabled. Skipping this test case")
		return
	}

	noOfFiles := 2
	noOfWorkers := 2
	createThreadPool(noOfFiles, noOfWorkers, "huge", suite)
}

// -------------- Main Method -------------------
func TestDataValidationTestSuite(t *testing.T) {
	initDataValidationFlags()
	fmt.Println("Distro Name: " + distro)

	// Ignore data validation test on all distros other than UBN
	if strings.ToLower(quickTest) == "true" {
		fmt.Println("Skipping Data Validation test suite...")
		return
	}

	dataValidationTest := dataValidationTestSuite{}

	minBuff = make([]byte, 1024)
	medBuff = make([]byte, (10 * 1024 * 1024))
	largeBuff = make([]byte, (500 * 1024 * 1024))
	if strings.ToLower(quickTest) == "true" {
		hugeBuff = make([]byte, (100 * 1024 * 1024))
	} else {
		hugeBuff = make([]byte, (750 * 1024 * 1024))
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getDataValidationTestDirName(10)

	// Create directory for testing the End to End test on mount path
	dataValidationTest.testMntPath = dataValidationMntPathPtr + "/" + testDirName
	fmt.Println(dataValidationTest.testMntPath)

	dataValidationTest.testLocalPath, _ = filepath.Abs(dataValidationMntPathPtr + "/..")
	fmt.Println(dataValidationTest.testLocalPath)

	dataValidationTest.testCachePath = dataValidationTempPathPtr + "/" + testDirName
	fmt.Println(dataValidationTest.testCachePath)

	if dataValidationAdlsPtr == "true" || dataValidationAdlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		dataValidationTest.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(dataValidationTest.testMntPath)
	if err != nil {
		fmt.Println("Could not cleanup feature dir before testing")
	}
	err = os.RemoveAll(dataValidationTest.testCachePath)
	if err != nil {
		fmt.Println("Could not cleanup cache dir before testing")
	}

	err = os.Mkdir(dataValidationTest.testMntPath, 0777)
	if err != nil {
		t.Error("Failed to create test directory")
	}
	rand.Read(minBuff)
	rand.Read(medBuff)
	rand.Read(largeBuff)
	rand.Read(hugeBuff)

	// Run the actual End to End test
	suite.Run(t, &dataValidationTest)

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(dataValidationTest.testMntPath)
}

func init() {
	regDataValidationTestFlag(&dataValidationMntPathPtr, "mnt-path", "", "Mount Path of Container")
	regDataValidationTestFlag(&dataValidationAdlsPtr, "adls", "", "Account is ADLS or not")
	regDataValidationTestFlag(&dataValidationTempPathPtr, "tmp-path", "", "Cache dir path")
	regDataValidationTestFlag(&quickTest, "quick-test", "true", "Run quick tests")
	regDataValidationTestFlag(&streamDirectTest, "stream-direct-test", "false", "Run stream direct tests")
	regDataValidationTestFlag(&distro, "distro-name", "", "Name of the distro")
}
