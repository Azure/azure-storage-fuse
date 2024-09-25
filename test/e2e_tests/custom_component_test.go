package e2e_tests

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var customComponentMntPathPtr string
var customComponentTempPathPtr string
var customComponentStoragePathPtr string

var buffer []byte

const MB int64 = (1024 * 1024)

type customComponentTestSuite struct {
	suite.Suite
}

type testConf struct {
	testMntPath     string
	testLocalPath   string
	testStoragePath string // path which is backed by the storage
}

var tConf testConf

func regcustomComponentTestFlag(p *string, name string, value string, usage string) {
	if flag.Lookup(name) == nil {
		flag.StringVar(p, name, value, usage)
	}
}

func getcustomComponentTestFlag(name string) string {
	return flag.Lookup(name).Value.(flag.Getter).Get().(string)
}

func initcustomComponentFlags() {
	customComponentMntPathPtr = getcustomComponentTestFlag("mnt-path")
	customComponentTempPathPtr = getcustomComponentTestFlag("tmp-path")
	customComponentStoragePathPtr = getcustomComponentTestFlag("storage-path")
	blockSizeMB = flag.Lookup("block-size-mb").Value.(flag.Getter).Get().(int)
}

func getcustomComponentTestDirName(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *customComponentTestSuite) copyToMountDir(localFilePath string, remoteFilePath string) {
	// copy to mounted directory
	cpCmd := exec.Command("cp", localFilePath, remoteFilePath)
	cliOut, err := cpCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.Equal(nil, err)
}

// Computes MD5 and returns the 32byte slice which represents the hash value
func (suite *customComponentTestSuite) computeMD5(filePath string) []byte {
	fh, err := os.Open(filePath)
	suite.Nil(err)

	fi, err := fh.Stat()
	suite.Nil(err)
	size := fi.Size()

	hash := md5.New()
	bytesCopied, err := io.Copy(hash, fh)
	suite.Nil(err)
	suite.Equal(size, bytesCopied)

	err = fh.Close()
	suite.Nil(err)

	return hash.Sum(nil)
}

func (suite *customComponentTestSuite) validateData(localFilePath string, remoteFilePath string) {
	localMD5sum := suite.computeMD5(localFilePath)
	remoteMD5sum := suite.computeMD5(remoteFilePath)
	suite.Equal(localMD5sum, remoteMD5sum)
}

func (suite *customComponentTestSuite) TestCustomComponentIOValidation() {
	// Test for writing to the custom component
	fileName := "test_file.txt"
	localFilePath := filepath.Join(tConf.testLocalPath, fileName)
	storagefilePath := filepath.Join(tConf.testStoragePath, fileName)
	remoteFilePath := filepath.Join(tConf.testMntPath, fileName)

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.Equal(nil, err)
	defer srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, buffer, 0777)
	suite.Equal(nil, err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	suite.validateData(localFilePath, storagefilePath)
	suite.validateData(localFilePath, remoteFilePath)
}

// -------------- Main Method -------------------
func TestCustomComponentTestSuite(t *testing.T) {
	initcustomComponentFlags()
	tConf = testConf{}

	buffer = make([]byte, 9*int64(blockSizeMB)*MB+2*MB)

	// Generate random test dir name where our End to End test run is contained
	testDirName := getcustomComponentTestDirName(10)

	// Create directory for testing the End to End test on mount path
	tConf.testMntPath = filepath.Join(customComponentMntPathPtr, testDirName)
	fmt.Println(tConf.testMntPath)
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory: ", err)
	}
	tConf.testLocalPath = filepath.Join(wd, testDirName)
	fmt.Println(tConf.testLocalPath)

	tConf.testStoragePath = filepath.Join(customComponentStoragePathPtr, testDirName)
	rand.Read(buffer)

	err = os.Mkdir(tConf.testMntPath, 0777)
	if err != nil {
		fmt.Println("Error creating mount directory: ", err)
	}

	err = os.Mkdir(tConf.testLocalPath, 0777)
	if err != nil {
		fmt.Println("Error creating local directory: ", err)
	}

	// Run the actual End to End test
	suite.Run(t, new(customComponentTestSuite))

	// Cleanup the test directories
	err = os.RemoveAll(filepath.Dir(tConf.testStoragePath))
	if err != nil {
		fmt.Println("Error removing mount directory: ", err)
	}

	err = os.RemoveAll(tConf.testLocalPath)
	if err != nil {
		fmt.Println("Error removing local directory: ", err)
	}
}

func init() {
	regcustomComponentTestFlag(&customComponentMntPathPtr, "mnt-path", "/mnt/test", "Mount Path of Container")
	regcustomComponentTestFlag(&customComponentTempPathPtr, "tmp-path", "/tmp/cache", "Cache dir path")
	regcustomComponentTestFlag(&customComponentStoragePathPtr, "storage-path", "/mnt/storage", "Storage path")
}
