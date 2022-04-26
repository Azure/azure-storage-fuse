// +build !unittest

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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var dataValidationMntPathPtr string
var dataValidationTempPathPtr string
var dataValidationAdlsPtr string
var quickTest string

var wg sync.WaitGroup

type dataValidationTestSuite struct {
	suite.Suite
	testMntPath   string
	testLocalPath string
	testCachePath string
	adlsTest      bool
	minBuff       []byte
	medBuff       []byte
	hugeBuff      []byte
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
	err = ioutil.WriteFile(localFilePath, suite.minBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for medium sized files
func (suite *dataValidationTestSuite) TestMediumFileData() {
	fileName := "medium_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = ioutil.WriteFile(localFilePath, suite.medBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// data validation for large sized files
func (suite *dataValidationTestSuite) TestLargeFileData() {
	fileName := "large_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = ioutil.WriteFile(localFilePath, suite.hugeBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{suite.testCachePath})

	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

// negative test case for data validation where the local file is updated
func (suite *dataValidationTestSuite) TestFileUpdate() {
	fileName := "updated_data.txt"
	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	err = ioutil.WriteFile(localFilePath, suite.minBuff, 0777)
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
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, suite.testCachePath})
}

func validateMultipleFilesData(fileName string, fileSize string, suite *dataValidationTestSuite) {
	defer wg.Done()

	localFilePath := suite.testLocalPath + "/" + fileName
	remoteFilePath := suite.testMntPath + "/" + fileName
	fmt.Println("Local file path: " + localFilePath)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	srcFile.Close()

	// write to file in the local directory
	var fileBuff []byte
	if fileSize == "huge" {
		fileBuff = make([]byte, (2000 * 1024 * 1024))
	} else if fileSize == "large" {
		if quickTest == "true" {
			fileBuff = make([]byte, (100 * 1024 * 1024))
		} else {
			fileBuff = make([]byte, (500 * 1024 * 1024))
		}
	} else if fileSize == "medium" {
		fileBuff = make([]byte, (10 * 1024 * 1024))
	} else {
		fileBuff = make([]byte, 1024)
	}
	rand.Read(fileBuff)
	err = ioutil.WriteFile(localFilePath, fileBuff, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)
	suite.dataValidationTestCleanup([]string{suite.testCachePath + "/" + fileName})
	suite.validateData(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, suite.testCachePath + "/" + fileName})
}

func (suite *dataValidationTestSuite) TestMultipleSmallFiles() {
	for i := 1; i <= 100; i++ {
		wg.Add(1)

		fileName := "small_data_" + strconv.Itoa(i) + ".txt"
		go validateMultipleFilesData(fileName, "small", suite)
	}

	wg.Wait()

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

func (suite *dataValidationTestSuite) TestMultipleMediumFiles() {
	for i := 1; i <= 50; i++ {
		wg.Add(1)

		fileName := "medium_data_" + strconv.Itoa(i) + ".txt"
		go validateMultipleFilesData(fileName, "medium", suite)
	}

	wg.Wait()

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

func (suite *dataValidationTestSuite) TestMultipleLargeFiles() {
	for i := 1; i <= 5; i++ {
		wg.Add(1)

		fileName := "large_data_" + strconv.Itoa(i) + ".txt"
		go validateMultipleFilesData(fileName, "large", suite)
	}

	wg.Wait()

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

func (suite *dataValidationTestSuite) TestMultipleHugeFiles() {
	if quickTest == "true" {
		fmt.Println("Quick test is enabled. Skipping this test case")
		return
	}

	for i := 1; i <= 2; i++ {
		wg.Add(1)

		fileName := "huge_data_" + strconv.Itoa(i) + ".txt"
		go validateMultipleFilesData(fileName, "huge", suite)
	}

	wg.Wait()

	suite.dataValidationTestCleanup([]string{suite.testCachePath})
}

// -------------- Main Method -------------------
func TestDataValidationTestSuite(t *testing.T) {
	initDataValidationFlags()
	dataValidationTest := dataValidationTestSuite{
		minBuff:  make([]byte, 1024),
		medBuff:  make([]byte, (10 * 1024 * 1024)),
		hugeBuff: make([]byte, (500 * 1024 * 1024)),
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
	rand.Read(dataValidationTest.minBuff)
	rand.Read(dataValidationTest.medBuff)
	rand.Read(dataValidationTest.hugeBuff)

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
}
