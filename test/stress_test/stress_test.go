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

package stress_test

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

var retryCount = 3
var noOfWorkers int = 20
var baseDir string = "stress"
var quickStressTest bool = false

type workItem struct {
	optType  int // 1: Create Directory,  2 : Create File
	baseDir  string
	dirName  string
	fileName string
	fileData []byte
}

func downloadWorker(t *testing.T, id int, jobs <-chan string, results chan<- int) {
	//var data []byte
	for item := range jobs {
		i := 0
		for ; i < retryCount; i++ {
			f, errFile := os.Open(item)
			if errFile == nil {
				f.Close()
				_, _ = os.ReadFile(item)
				//t.Log(data)
				//t.Log(".")
				break
			} else {
				t.Log("F")
			}
		}
		if i == retryCount {
			t.FailNow()
		}

		//t.Log("Opened File : %s/%s.tst \n", item.baseDir, item.fileName)
		results <- 1
	}
}

func uploadWorker(t *testing.T, id int, jobs <-chan workItem, results chan<- int) {
	for item := range jobs {
		if item.optType == 1 {
			errDir := os.MkdirAll(item.baseDir+"/"+item.dirName, 0755)
			if errDir != nil {
				t.FailNow()
			}
			//t.Log("#")
			//t.Log("Created Directory : %s/%s \n", item.baseDir, item.dirName)
		} else if item.optType == 2 {
			i := 0
			var errFile error
			for ; i < retryCount; i++ {
				errFile = os.WriteFile(item.baseDir+"/"+item.fileName+".tst", item.fileData, 0666)
				if errFile == nil {
					//t.Log(".")
					break
				} else {
					t.Log("F")
				}
			}

			if i == retryCount {
				t.FailNow()
			}

			//t.Log("Created File : %s/%s.tst \n", item.baseDir, item.fileName)
		}
		results <- 1
	}
}

func BytesCount(bytes float64, postfix string) (byteStr string) {
	if postfix == "rate" {
		bytes = (bytes * 8)
	}

	if bytes < 1024 {
		if postfix == "" {
			postfix = " bytes"
		} else {
			postfix = " bps"
		}
		byteStr = fmt.Sprintf("%.2f", (float64)(bytes))
	} else if bytes < (1024 * 1024) {
		if postfix == "" {
			postfix = " KB"
		} else {
			postfix = " Kbps"
		}
		byteStr = fmt.Sprintf("%.2f", (float64)(bytes/1024))
	} else if bytes < (1024 * 1024 * 1024) {
		if postfix == "" {
			postfix = " MB"
		} else {
			postfix = " Mbps"
		}
		byteStr = fmt.Sprintf("%.2f", (float64)(bytes/(1024*1024)))
	} else {
		if postfix == "" {
			postfix = " GB"
		} else {
			postfix = " Gbps"
		}
		byteStr = fmt.Sprintf("%.2f", (float64)(bytes/(1024*1024*1024)))
	}

	byteStr += postfix
	return
}

func stressTestUpload(t *testing.T, name string, noOfDir int, noOfFiles int, fileSize int) {
	t.Log("\nStarting test : '" + name + "' \n")

	if noOfDir < noOfWorkers {
		noOfWorkers = noOfDir
	}
	var workItemCnt = noOfDir + (noOfDir * noOfFiles)

	jobs := make(chan workItem, workItemCnt)
	results := make(chan int, workItemCnt)

	for w := 1; w <= noOfWorkers; w++ {
		go uploadWorker(t, w, jobs, results)
	}
	t.Logf("Number of workders started : %d \n", noOfWorkers)

	var dirItem workItem
	dirItem.optType = 1
	dirItem.baseDir = baseDir + "/" + name

	var fileBuff = make([]byte, fileSize)
	rand.Read(fileBuff)
	//t.Log(fileBuff)

	var fileItem workItem
	fileItem.optType = 2
	fileItem.baseDir = baseDir + "/" + name
	fileItem.fileData = fileBuff

	startTime := time.Now()
	//  Create given number of directories in parallel
	for j := 1; j <= noOfDir; j++ {
		dirItem.dirName = strconv.Itoa(j)
		jobs <- dirItem
	}
	for a := 1; a <= noOfDir; a++ {
		<-results
	}

	//  Create given number of files in each directory in parallel
	for j := 1; j <= noOfDir; j++ {
		fileItem.dirName = strconv.Itoa(j)
		for k := 1; k <= noOfFiles; k++ {
			fileItem.fileName = strconv.Itoa(j) + "/" + name + "_" + strconv.Itoa(k)
			jobs <- fileItem
		}
	}
	close(jobs)
	for a := 1; a <= (noOfDir * noOfFiles); a++ {
		<-results
	}
	elapsed := time.Since(startTime)
	close(results)

	t.Logf("\n-----------------------------------------------------------------------------------------")
	t.Logf("Number of directories created : %d \n", noOfDir)
	t.Logf("Number of files created : %d  each of %s\n", noOfDir*noOfFiles, BytesCount((float64)(fileSize), ""))
	t.Logf("%s bytes created in %f secs\n", BytesCount((float64)(fileSize*noOfDir*noOfFiles), ""), elapsed.Seconds())
	if elapsed.Seconds() >= 1 {
		t.Logf("Upload Speed %s \n",
			BytesCount(
				(float64)((float64)(fileSize*noOfDir*noOfFiles)/(float64)(elapsed.Seconds())),
				"rate"))
	} else {
		t.Logf("Upload Speed %s \n",
			BytesCount(
				(float64)(fileSize*noOfDir*noOfFiles),
				"rate"))
	}
}

func stressTestDownload(t *testing.T, name string, noOfDir int, noOfFiles int, fileSize int) {
	t.Log("Starting Download test...\n")

	var workItemCnt = noOfDir + (noOfDir * noOfFiles)

	jobs := make(chan string, workItemCnt)
	results := make(chan int, workItemCnt)

	for w := 1; w <= noOfWorkers; w++ {
		go downloadWorker(t, w, jobs, results)
	}

	totalBytes := 0
	totalFiles := 0
	startTime := time.Now()

	err := filepath.Walk(baseDir+"/"+name,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				//t.Log(path, info.Size())
				jobs <- path
				totalFiles++
				totalBytes += (int)(info.Size())
			}
			return nil
		})
	if err != nil {
		t.Log(err)
	}
	close(jobs)
	for a := 1; a <= (noOfDir * noOfFiles); a++ {
		<-results
	}
	close(results)

	elapsed := time.Since(startTime)

	t.Logf("\nTotal files downloaded : %d\n", totalFiles)
	t.Logf("%s bytes read in %.2f secs\n", BytesCount((float64)(totalBytes), ""), (float64)(elapsed.Seconds()))
	if elapsed.Seconds() >= 1 {
		t.Logf("Download Speed %s \n",
			BytesCount(
				(float64)((float64)(totalBytes)/(float64)(elapsed.Seconds())),
				"rate"))
	} else {
		t.Logf("Download Speed %s \n",
			BytesCount(
				(float64)(totalBytes),
				"rate"))
	}
	t.Log("Cleaning up...")
	os.RemoveAll(baseDir + "/" + name)
	t.Log("-----------------------------------------------------------------------------------------")

}

func TestStress(t *testing.T) {
	StressSmall(t)
	StressBig(t)
	StressHuge(t)
}

func StressSmall(t *testing.T) {
	var numSmallDirs, numSmallFiles, smallFileSize int

	if quickStressTest == false {
		numSmallDirs = 50
		numSmallFiles = 40
		smallFileSize = (1024 * 1024)
	} else {
		numSmallDirs = 5
		numSmallFiles = 4
		smallFileSize = (100)
	}

	stressTestUpload(t, "small", numSmallDirs, numSmallFiles, smallFileSize)
	stressTestDownload(t, "small", numSmallDirs, numSmallFiles, smallFileSize)
}

func StressBig(t *testing.T) {
	var numBigDirs, numBigFiles, bigFileSize int

	if quickStressTest == false {
		numBigDirs = 10
		numBigFiles = 2
		bigFileSize = (200 * 1024 * 1024)
	} else {
		numBigDirs = 1
		numBigFiles = 2
		bigFileSize = (500)
	}

	stressTestUpload(t, "big", numBigDirs, numBigFiles, bigFileSize)
	stressTestDownload(t, "big", numBigDirs, numBigFiles, bigFileSize)
}

func StressHuge(t *testing.T) {
	var numHugeDirs, numHugeFiles, hugeFileSize int

	if quickStressTest == false {
		numHugeDirs = 2
		numHugeFiles = 1
		hugeFileSize = (2 * 1024 * 1024 * 1024)
	} else {
		numHugeDirs = 2
		numHugeFiles = 1
		hugeFileSize = (2 * 1024)
	}

	stressTestUpload(t, "huge", numHugeDirs, numHugeFiles, hugeFileSize)
	stressTestDownload(t, "huge", numHugeDirs, numHugeFiles, hugeFileSize)
}

func TestMain(m *testing.M) {
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	quickTest := flag.String("quick", "false", "Run a quick test")

	flag.Parse()

	baseDir = filepath.Join(*pathPtr, baseDir)
	if *quickTest == "true" || *quickTest == "True" {
		quickStressTest = true
		fmt.Println("Quick test running..")
	} else {
		fmt.Println("Full stress test running..")
	}

	err := os.RemoveAll(baseDir)
	if err != nil {
		fmt.Println("Could not cleanup stress dir before testing")
	}

	err = os.Mkdir(baseDir, 0777)
	if err != nil {
		fmt.Println("Could not create dir before testing")
	}

	m.Run()

	os.RemoveAll(baseDir)
}
