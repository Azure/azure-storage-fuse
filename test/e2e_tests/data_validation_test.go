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
	"crypto/md5"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/suite"
)

var dataValidationMntPathPtr string
var dataValidationTempPathPtr string
var dataValidationAdlsPtr string
var quickTest string
var streamDirectTest string
var distro string
var blockSizeMB int = 16

var minBuff, medBuff, largeBuff, hugeBuff []byte

const _1MB uint64 = (1024 * 1024)

type dataValidationTestSuite struct {
	suite.Suite
}

type testObj struct {
	testMntPath   string
	testLocalPath string
	testCachePath string
	adlsTest      bool
}

var tObj testObj

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
	blockSizeMB = flag.Lookup("block-size-mb").Value.(flag.Getter).Get().(int)
}

func getDataValidationTestDirName(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)[:n]
}

func (suite *dataValidationTestSuite) dataValidationTestCleanup(toRemove []string) {
	for _, path := range toRemove {
		err := os.RemoveAll(path)
		suite.NoError(err)
	}
}

func (suite *dataValidationTestSuite) copyToMountDir(localFilePath string, remoteFilePath string) {
	// copy to mounted directory
	cpCmd := exec.Command("cp", localFilePath, remoteFilePath)
	cliOut, err := cpCmd.Output()
	if len(cliOut) != 0 {
		fmt.Println(string(cliOut))
	}
	suite.NoError(err)
}

// Computes MD5 and returns the 32byte slice which represents the hash value
func (suite *dataValidationTestSuite) computeMD5(filePath string) []byte {
	fh, err := os.Open(filePath)
	suite.NoError(err)

	fi, err := fh.Stat()
	suite.NoError(err)
	size := fi.Size()

	hash := md5.New()
	bytesCopied, err := io.Copy(hash, fh)
	suite.NoError(err)
	suite.Equal(size, bytesCopied)

	err = fh.Close()
	suite.NoError(err)

	return hash.Sum(nil)
}

func (suite *dataValidationTestSuite) helperValidateFileContent(localFilePath string, remoteFilePath string) {
	// check if file sizes are same
	localFileInfo, err := os.Stat(localFilePath)
	suite.NoError(err)
	remoteFileInfo, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(localFileInfo.Size(), remoteFileInfo.Size())

	localMD5sum := suite.computeMD5(localFilePath)
	remoteMD5sum := suite.computeMD5(remoteFilePath)
	suite.Equal(localMD5sum, remoteMD5sum)
}

func (suite *dataValidationTestSuite) helperCreateFile(localFilePath string, remoteFilePath string, size int64) {
	buffer := make([]byte, 1*1024*1024)
	_, _ = rand.Read(buffer)

	writeFile := func(file *os.File) {
		originalSize := size
		for originalSize > 0 {
			bytesToWrite := min(int64(len(buffer)), originalSize)
			n, err := file.Write(buffer[0:bytesToWrite])
			suite.Equal(int(bytesToWrite), n)
			ok := suite.NoError(err)
			if !ok {
				break
			}
			originalSize -= int64(n)
		}
	}

	localFile, err := os.Create(localFilePath)
	suite.NoError(err)
	writeFile(localFile)
	err = localFile.Close()
	suite.NoError(err)

	remoteFile, err := os.Create(remoteFilePath)
	suite.NoError(err)
	writeFile(remoteFile)
	err = remoteFile.Close()
	suite.NoError(err)
}

func (suite *dataValidationTestSuite) helperTruncateFile(localFilePath string, remoteFilePath string, size int64) {
	srcFile, err := os.OpenFile(localFilePath, os.O_RDWR, 0666)
	suite.NoError(err)
	err = srcFile.Truncate(size)
	suite.NoError(err)

	dstFile, err := os.OpenFile(remoteFilePath, os.O_RDWR, 0666)
	suite.NoError(err)
	err = dstFile.Truncate(size)
	suite.NoError(err)

	err = srcFile.Close()
	suite.NoError(err)
	err = dstFile.Close()
	suite.NoError(err)
}

func (suite *dataValidationTestSuite) helperWriteToFile(localFilePath string, remoteFilePath string, offset int64, size int) {
	buffer := make([]byte, 1*1024*1024)

	localFile, err := os.OpenFile(localFilePath, os.O_RDWR, 0666)
	suite.NoError(err)

	remoteFile, err := os.OpenFile(remoteFilePath, os.O_RDWR, 0666)
	suite.NoError(err)

	n, err := localFile.WriteAt(buffer[0:size], offset)
	suite.Equal(size, n)
	suite.NoError(err)

	n, err = remoteFile.WriteAt(buffer[0:size], offset)
	suite.Equal(size, n)
	suite.NoError(err)

	err = localFile.Close()
	suite.NoError(err)
	err = remoteFile.Close()
	suite.NoError(err)
}

//----------------Utility Functions-----------------------

// pass the file name and the function returns the LocalFilePath and MountedFilePath
func convertFileNameToFilePath(fileName string) (localFilePath string, remoteFilePath string) {
	localFilePath = tObj.testLocalPath + "/" + fileName
	remoteFilePath = tObj.testMntPath + "/" + fileName
	return localFilePath, remoteFilePath
}

// creates File in Local and Mounted Directories and returns there file handles the associated fd has O_RDWR mode
func createFileHandleInLocalAndRemote(suite *dataValidationTestSuite, localFilePath, remoteFilePath string) (lfh *os.File, rfh *os.File) {
	lfh, err := os.Create(localFilePath)
	suite.NoError(err)

	rfh, err = os.Create(remoteFilePath)
	suite.NoError(err)

	return lfh, rfh
}

// closes the file handles, This ensures that data is flushed to disk/Azure Storage from the cache
func closeFileHandles(suite *dataValidationTestSuite, handles ...*os.File) {
	for _, h := range handles {
		err := h.Close()
		suite.NoError(err)
	}
}

// Writes the data at the given Offsets for the given file
func writeSparseData(suite *dataValidationTestSuite, fh *os.File, offsets []int64) {
	ind := uint64(0)
	for _, o := range offsets {
		// write 1MB data at offset o
		n, err := fh.WriteAt(medBuff[ind*_1MB:(ind+1)*_1MB], o)
		suite.NoError(err)
		suite.Equal(n, int(_1MB))

		ind = (ind + 1) % 10
	}
}

// Creates the file with filePath and puts random data of size bytes
func generateFileWithRandomData(suite *dataValidationTestSuite, filePath string, size int) {
	fh, err := os.Create(filePath)
	suite.NoError(err)
	bufferSize := 4 * 1024
	buffer := make([]byte, 4*1024)
	_, _ = rand.Read(buffer)
	blocks := size / bufferSize
	for range blocks {
		bytesToWrite := min(bufferSize, size)
		bytesWritten, err := fh.Write(buffer[0:bytesToWrite])
		suite.NoError(err)
		suite.Equal(bytesToWrite, bytesWritten)
		size -= bytesWritten
	}
	closeFileHandles(suite, fh)
}

// -------------- Data Validation Tests -------------------

// Test correct overwrite of file using echo command
func (suite *dataValidationTestSuite) TestFileOverwriteWithEchoCommand() {
	remoteFilePath := filepath.Join(tObj.testMntPath, "TESTFORECHO.txt")
	text := "Hello, this is a test."
	command := "echo \"" + text + "\" > " + remoteFilePath
	cmd := exec.Command("/bin/bash", "-c", command)
	_, err := cmd.Output()
	suite.NoError(err)

	data, err := os.ReadFile(remoteFilePath)
	suite.NoError(err)
	suite.Equal(string(data), text+"\n")

	newtext := "End of test."
	newcommand := "echo \"" + newtext + "\" > " + remoteFilePath
	newcmd := exec.Command("/bin/bash", "-c", newcommand)
	_, err = newcmd.Output()
	suite.NoError(err)

	data, err = os.ReadFile(remoteFilePath)
	suite.NoError(err)
	suite.Equal(string(data), newtext+"\n")
}

// data validation for small sized files
func (suite *dataValidationTestSuite) TestSmallFileData() {
	fileName := "small_data.txt"
	localFilePath := tObj.testLocalPath + "/" + fileName
	remoteFilePath := tObj.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{tObj.testCachePath})

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

// data validation for medium sized files
func (suite *dataValidationTestSuite) TestMediumFileData() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "medium_data.txt"
	localFilePath := tObj.testLocalPath + "/" + fileName
	remoteFilePath := tObj.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, medBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{tObj.testCachePath})

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

// data validation for large sized files
func (suite *dataValidationTestSuite) TestLargeFileData() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
		return
	}
	fileName := "large_data.txt"
	localFilePath := tObj.testLocalPath + "/" + fileName
	remoteFilePath := tObj.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, largeBuff, 0777)
	suite.NoError(err)

	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{tObj.testCachePath})

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

// negative test case for data validation where the local file is updated
func (suite *dataValidationTestSuite) TestDataValidationNegative() {
	fileName := "updated_data.txt"
	localFilePath := tObj.testLocalPath + "/" + fileName
	remoteFilePath := tObj.testMntPath + "/" + fileName

	// create the file in local directory
	srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
	suite.NoError(err)
	srcFile.Close()

	// write to file in the local directory
	err = os.WriteFile(localFilePath, minBuff, 0777)
	suite.NoError(err)

	// copy local file to mounted directory
	suite.copyToMountDir(localFilePath, remoteFilePath)

	// delete the cache directory
	suite.dataValidationTestCleanup([]string{tObj.testCachePath})

	// update local file
	srcFile, err = os.OpenFile(localFilePath, os.O_APPEND|os.O_WRONLY, 0777)
	suite.NoError(err)
	_, err = srcFile.WriteString("Added text")
	srcFile.Close()
	suite.NoError(err)

	// compare local file and mounted files
	diffCmd := exec.Command("diff", localFilePath, remoteFilePath)
	cliOut, err := diffCmd.Output()
	fmt.Println("Negative test case where files should differ")
	fmt.Println(string(cliOut))
	suite.NotEmpty(cliOut)
	suite.Error(err)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func validateMultipleFilesData(jobs <-chan int, results chan<- string, fileSize string, suite *dataValidationTestSuite) {
	for i := range jobs {
		fileName := fileSize + strconv.Itoa(i) + ".txt"
		localFilePath := tObj.testLocalPath + "/" + fileName
		remoteFilePath := tObj.testMntPath + "/" + fileName
		fmt.Println("Local file path: " + localFilePath)

		// create the file in local directory
		srcFile, err := os.OpenFile(localFilePath, os.O_CREATE, 0777)
		suite.NoError(err)
		srcFile.Close()

		// write to file in the local directory
		switch fileSize {
		case "huge":
			err = os.WriteFile(localFilePath, hugeBuff, 0777)
		case "large":
			if strings.ToLower(quickTest) == "true" {
				err = os.WriteFile(localFilePath, hugeBuff, 0777)
			} else {
				err = os.WriteFile(localFilePath, largeBuff, 0777)
			}
		case "medium":
			err = os.WriteFile(localFilePath, medBuff, 0777)
		default:
			err = os.WriteFile(localFilePath, minBuff, 0777)
		}
		suite.NoError(err)

		suite.copyToMountDir(localFilePath, remoteFilePath)
		suite.dataValidationTestCleanup([]string{tObj.testCachePath + "/" + fileName})
		suite.helperValidateFileContent(localFilePath, remoteFilePath)

		suite.dataValidationTestCleanup([]string{localFilePath, tObj.testCachePath + "/" + fileName})

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

	suite.dataValidationTestCleanup([]string{tObj.testCachePath})
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

	noOfFiles := 8
	noOfWorkers := 4
	createThreadPool(noOfFiles, noOfWorkers, "medium", suite)
}

func (suite *dataValidationTestSuite) TestMultipleLargeFiles() {
	if strings.ToLower(streamDirectTest) == "true" {
		fmt.Println("Skipping this test case for stream direct")
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

func (suite *dataValidationTestSuite) TestSparseFileRandomWrite() {
	fileName := "sparseFile"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
	lfh, rfh := createFileHandleInLocalAndRemote(suite, localFilePath, remoteFilePath)

	// write to local file
	writeSparseData(suite, lfh, []int64{0, 164 * int64(_1MB), 100 * int64(_1MB), 65 * int64(_1MB), 129 * int64(_1MB)})

	// write to remote file
	writeSparseData(suite, rfh, []int64{0, 164 * int64(_1MB), 100 * int64(_1MB), 65 * int64(_1MB), 129 * int64(_1MB)})

	closeFileHandles(suite, lfh, rfh)
	// check size of blob uploaded
	fi, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(165*int64(_1MB), fi.Size())

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestSparseFileRandomWriteBlockOverlap() {
	fileName := "sparseFileBlockOverlap"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
	lfh, rfh := createFileHandleInLocalAndRemote(suite, localFilePath, remoteFilePath)

	// write to local file
	writeSparseData(suite, lfh, []int64{0, 170 * int64(_1MB), 63*int64(_1MB) + 1024*512, 129 * int64(_1MB), 100 * int64(_1MB)})

	// write to remote file
	writeSparseData(suite, rfh, []int64{0, 170 * int64(_1MB), 63*int64(_1MB) + 1024*512, 129 * int64(_1MB), 100 * int64(_1MB)})

	closeFileHandles(suite, lfh, rfh)

	// check size of blob uploaded
	fi, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(171*int64(_1MB), fi.Size())

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestFileReadBytesMultipleBlocks() {
	fileName := "bytesReadMultipleBlock"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
	lfh, rfh := createFileHandleInLocalAndRemote(suite, localFilePath, remoteFilePath)

	// write 65MB data
	n, err := lfh.WriteAt(largeBuff[0:65*_1MB], 0)
	suite.NoError(err)
	suite.Equal(n, int(65*_1MB))

	// write 7 bytes at offset 65MB
	n, err = lfh.WriteAt(largeBuff[0:7], int64(65*_1MB))
	suite.NoError(err)
	suite.Equal(7, n)

	// write 65MB data
	n, err = rfh.WriteAt(largeBuff[0:65*_1MB], 0)
	suite.NoError(err)
	suite.Equal(n, int(65*_1MB))

	// write 7 bytes at offset 65MB
	n, err = rfh.WriteAt(largeBuff[0:7], int64(65*_1MB))
	suite.NoError(err)
	suite.Equal(7, n)

	closeFileHandles(suite, lfh, rfh)

	// check size of blob uploaded using os.Stat()
	fi, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(65*int64(_1MB)+7, fi.Size())

	// count the total bytes uploaded
	fh, err := os.Open(remoteFilePath)
	suite.NoError(err)

	totalBytesread := int64(0)
	dataBuff := make([]byte, int(_1MB))
	for {
		bytesRead, err := fh.Read(dataBuff)
		totalBytesread += int64(bytesRead)
		if err != nil {
			suite.Contains(err.Error(), "EOF")
			break
		}
	}
	suite.Equal(65*int64(_1MB)+7, totalBytesread)

	closeFileHandles(suite, fh)

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestFileReadBytesOneBlock() {
	fileName := "bytesReadOneBlock"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
	lfh, rfh := createFileHandleInLocalAndRemote(suite, localFilePath, remoteFilePath)

	// write 13 bytes data to local file
	n, err := lfh.WriteAt(largeBuff[0:13], 0)
	suite.NoError(err)
	suite.Equal(13, n)

	// write 13 bytes data to remote file
	n, err = rfh.WriteAt(largeBuff[0:13], 0)
	suite.NoError(err)
	suite.Equal(13, n)

	closeFileHandles(suite, lfh, rfh)

	// check size of blob uploaded using os.Stat()
	fi, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(int64(13), fi.Size())

	// count the total bytes uploaded
	fh, err := os.Open(remoteFilePath)
	suite.NoError(err)

	totalBytesread := int64(0)
	dataBuff := make([]byte, 1000)
	for {
		bytesRead, err := fh.Read(dataBuff)
		totalBytesread += int64(bytesRead)
		if err != nil {
			suite.Contains(err.Error(), "EOF")
			break
		}
	}
	suite.Equal(int64(13), totalBytesread)

	closeFileHandles(suite, fh)

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestRandomWriteRaceCondition() {
	fileName := "randomWriteRaceCondition"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
	lfh, rfh := createFileHandleInLocalAndRemote(suite, localFilePath, remoteFilePath)

	offsetList := []int64{}
	for i := range 10 {
		offsetList = append(offsetList, int64(i*16*int(_1MB)))
	}
	// at the end write back at block 0 at offset 1MB
	offsetList = append(offsetList, int64(_1MB))

	// write to local file
	writeSparseData(suite, lfh, offsetList)

	// write to remote file
	writeSparseData(suite, rfh, offsetList)

	closeFileHandles(suite, lfh, rfh)

	// check size of blob uploaded
	fi, err := os.Stat(remoteFilePath)
	suite.NoError(err)
	suite.Equal(145*int64(_1MB), fi.Size())

	suite.helperValidateFileContent(localFilePath, remoteFilePath)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestPanicOnClosingFile() {
	fileName := "panicOnClosingFile"
	_, remoteFilePath := convertFileNameToFilePath(fileName)
	blockSizeBytes := blockSizeMB * int(_1MB)
	buffer := make([]byte, blockSizeBytes)
	generateFileWithRandomData(suite, remoteFilePath, blockSizeBytes*10)

	rfh, err := os.OpenFile(remoteFilePath, syscall.O_RDWR, 0666)
	suite.NoError(err)

	//Read 1st block
	bytes_read, err := rfh.Read(buffer)
	suite.NoError(err)
	suite.Equal(bytes_read, blockSizeBytes)

	//Write to 2nd block
	bytes_written, err := rfh.Write(buffer)
	suite.Equal(bytes_written, blockSizeBytes)
	suite.NoError(err)

	closeFileHandles(suite, rfh)
}

// This tests takes default 1MB blockSize as other blockSizes could cause the test to take some time.
func (suite *dataValidationTestSuite) TestPanicOnWritingToFile() {
	fileName := "panicOnWritingToFile"
	_, remoteFilePath := convertFileNameToFilePath(fileName)
	blockSizeBytes := blockSizeMB * int(_1MB)
	buffer := make([]byte, blockSizeBytes)
	generateFileWithRandomData(suite, remoteFilePath, blockSizeBytes*20)

	rfh, err := os.OpenFile(remoteFilePath, syscall.O_RDWR, 0666)
	suite.NoError(err)

	//Make the cooking+cooked=prefetchCount
	for i := range 3 {
		offset := 4 * int64(i) * int64(_1MB)
		bytes_read, err := rfh.ReadAt(buffer, offset)
		suite.NoError(err)
		suite.Equal(bytes_read, blockSizeBytes)
	}

	for i := 18; i < 20; i++ {
		bytes_written, err := rfh.WriteAt(buffer, 18*int64(blockSizeBytes))
		suite.Equal(bytes_written, blockSizeBytes)
		suite.NoError(err)
	}

	closeFileHandles(suite, rfh)
}

// This tests takes default 1MB blockSize as other blockSizes could cause the test to take some time.
func (suite *dataValidationTestSuite) TestPanicOnReadingFileInRandReadMode() {
	fileName := "panicOnReadingFileInRandReadMode"
	_, remoteFilePath := convertFileNameToFilePath(fileName)
	blockSizeBytes := int(_1MB)
	buffer := make([]byte, blockSizeBytes)
	generateFileWithRandomData(suite, remoteFilePath, blockSizeBytes*84)

	rfh, err := os.OpenFile(remoteFilePath, syscall.O_RDWR, 0666)
	suite.NoError(err)

	//Write at some offset
	bytes_written, err := rfh.WriteAt(buffer, 0)
	suite.Equal(bytes_written, blockSizeBytes)
	suite.NoError(err)

	//Make the file handle goto random read mode in block cache(This is causing panic)
	for i := range 14 {
		offset := int64(_1MB) * 6 * int64(i)
		bytes_read, err := rfh.ReadAt(buffer, offset)
		suite.NoError(err)
		suite.Equal(bytes_read, blockSizeBytes)
	}

	closeFileHandles(suite, rfh)
}

// func (suite *dataValidationTestSuite) TestReadDataAtBlockBoundaries() {
// 	fileName := "testReadDataAtBlockBoundaries"
// 	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)
// 	fileSize := 35 * int(_1MB)
// 	generateFileWithRandomData(suite, localFilePath, fileSize)
// 	suite.copyToMountDir(localFilePath, remoteFilePath)
// 	suite.validateData(localFilePath, remoteFilePath)

// 	lfh, rfh := openFileHandleInLocalAndRemote(suite, os.O_RDWR, localFilePath, remoteFilePath)
// 	var offset int64 = 0
// 	//tests run in 16MB block size config.
// 	//Data in File 35MB(3blocks)
// 	//block1->16MB, block2->16MB, block3->3MB

// 	//getting 4MB data from 1st block
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//getting 4MB data from overlapping blocks
// 	offset = int64(15 * int(_1MB))
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//getting 4MB data from last block
// 	offset = int64(32 * int(_1MB))
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//getting 4MB data from overlapping block with last block
// 	offset = int64(30 * int(_1MB))
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//Read at some random offset
// 	for i := 0; i < 10; i++ {
// 		offset = mrand.Int63() % int64(fileSize)
// 		compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	}

// 	//write at end of file
// 	offset = int64(fileSize)
// 	compareWriteOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//Check the previous write with read
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)

// 	//Write at Random offset in the file
// 	offset = mrand.Int63() % int64(fileSize)
// 	compareWriteOperInLocalAndRemote(suite, lfh, rfh, offset)
// 	//Check the previous write with read
// 	compareReadOperInLocalAndRemote(suite, lfh, rfh, offset)

// 	closeFileHandles(suite, lfh, rfh)
// }

// -------------- Main Method -------------------
func TestDataValidationTestSuite(t *testing.T) {
	initDataValidationFlags()
	fmt.Println("Distro Name: " + distro)

	// Ignore data validation test on all distros other than UBN
	if strings.ToLower(quickTest) == "true" || (!strings.Contains(strings.ToUpper(distro), "UBUNTU") && !strings.Contains(strings.ToUpper(distro), "UBN")) {
		fmt.Println("Skipping Data Validation test suite...")
		return
	}

	tObj = testObj{}

	minBuff = make([]byte, 1024)
	medBuff = make([]byte, (10 * _1MB))
	largeBuff = make([]byte, (500 * _1MB))
	if strings.ToLower(quickTest) == "true" {
		hugeBuff = make([]byte, (100 * _1MB))
	} else {
		hugeBuff = make([]byte, (750 * _1MB))
	}

	// Generate random test dir name where our End to End test run is contained
	testDirName := getDataValidationTestDirName(10)

	// Create directory for testing the End to End test on mount path
	tObj.testMntPath = dataValidationMntPathPtr + "/" + testDirName
	fmt.Println(tObj.testMntPath)

	tObj.testLocalPath, _ = filepath.Abs(dataValidationMntPathPtr + "/..")
	fmt.Println(tObj.testLocalPath)

	tObj.testCachePath = dataValidationTempPathPtr + "/" + testDirName
	fmt.Println(tObj.testCachePath)

	if dataValidationAdlsPtr == "true" || dataValidationAdlsPtr == "True" {
		fmt.Println("ADLS Testing...")
		tObj.adlsTest = true
	} else {
		fmt.Println("BLOCK Blob Testing...")
	}

	fmt.Println("Block Size MB Used for the tests ", blockSizeMB)

	// Sanity check in the off chance the same random name was generated twice and was still around somehow
	err := os.RemoveAll(tObj.testMntPath)
	if err != nil {
		fmt.Printf("Could not cleanup feature dir before testing [%s]\n", err.Error())
	}
	err = os.RemoveAll(tObj.testCachePath)
	if err != nil {
		fmt.Printf("Could not cleanup cache dir before testing [%s]\n", err.Error())
	}

	err = os.Mkdir(tObj.testMntPath, 0777)
	if err != nil {
		t.Errorf("Failed to create test directory [%s]\n", err.Error())
	}
	_, _ = rand.Read(minBuff)
	_, _ = rand.Read(medBuff)
	_, _ = rand.Read(largeBuff)
	_, _ = rand.Read(hugeBuff)

	// Run the actual End to End test
	suite.Run(t, new(dataValidationTestSuite))

	//  Wipe out the test directory created for End to End test
	os.RemoveAll(tObj.testMntPath)
}

func init() {
	regDataValidationTestFlag(&dataValidationMntPathPtr, "mnt-path", "", "Mount Path of Container")
	regDataValidationTestFlag(&dataValidationAdlsPtr, "adls", "", "Account is ADLS or not")
	regDataValidationTestFlag(&dataValidationTempPathPtr, "tmp-path", "", "Cache dir path")
	regDataValidationTestFlag(&quickTest, "quick-test", "true", "Run quick tests")
	regDataValidationTestFlag(&streamDirectTest, "stream-direct-test", "false", "Run stream direct tests")
	regDataValidationTestFlag(&distro, "distro-name", "", "Name of the distro")
	flag.IntVar(&blockSizeMB, "block-size-mb", 16, "Block size MB in block cache")
}
