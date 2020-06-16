package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var retryCount = 3
var noOfWorkers int = 20
var baseDir string = "stress"

type workItem struct {
	optType  int // 1: Create Directory,  2 : Create File
	baseDir  string
	dirName  string
	fileName string
	fileData []byte
}

func downloadWorker(id int, jobs <-chan string, results chan<- int) {
	var errFile error
	//var data []byte
	for item := range jobs {
		i := 0
		for ; i < retryCount; i++ {
			f, errFile := os.Open(item)
			if errFile == nil {
				f.Close()
				_, _ = ioutil.ReadFile(item)
				//fmt.Println(data)
				fmt.Printf(".")
				break
			} else {
				fmt.Printf("F")
			}
		}
		if i == retryCount {
			log.Fatal(errFile)
		}

		//fmt.Printf("Opened File : %s/%s.tst \n", item.baseDir, item.fileName)
		results <- 1
	}
}

func uploadWorker(id int, jobs <-chan workItem, results chan<- int) {
	for item := range jobs {
		if item.optType == 1 {
			errDir := os.MkdirAll(item.baseDir+"/"+item.dirName, 0755)
			if errDir != nil {
				log.Fatal(errDir)
			}
			fmt.Printf("#")
			//fmt.Printf("Created Directory : %s/%s \n", item.baseDir, item.dirName)
		} else if item.optType == 2 {
			i := 0
			var errFile error
			for ; i < retryCount; i++ {
				errFile = ioutil.WriteFile(item.baseDir+"/"+item.fileName+".tst", item.fileData, 0666)
				if errFile == nil {
					fmt.Printf(".")
					break
				} else {
					fmt.Printf("F")
				}
			}

			if i == retryCount {
				log.Fatal(errFile)
			}
			//fmt.Printf("Created File : %s/%s.tst \n", item.baseDir, item.fileName)
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

func stressTestUpload(name string, noOfDir int, noOfFiles int, fileSize int) {
	fmt.Println("\nStarting test : '" + name + "' \n")

	if noOfDir < noOfWorkers {
		noOfWorkers = noOfDir
	}
	var workItemCnt = noOfDir + (noOfDir * noOfFiles)

	jobs := make(chan workItem, workItemCnt)
	results := make(chan int, workItemCnt)

	for w := 1; w <= noOfWorkers; w++ {
		go uploadWorker(w, jobs, results)
	}
	fmt.Printf("Number of workders started : %d \n", noOfWorkers)

	var dirItem workItem
	dirItem.optType = 1
	dirItem.baseDir = baseDir + "/" + name

	var fileBuff = make([]byte, fileSize)
	rand.Read(fileBuff)
	//fmt.Println(fileBuff)

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

	fmt.Println("\n-----------------------------------------------------------------------------------------")
	fmt.Printf("Number of directories created : %d \n", noOfDir)
	fmt.Printf("Number of files created : %d  each of %s\n", noOfDir*noOfFiles, BytesCount((float64)(fileSize), ""))
	fmt.Printf("%s bytes created in %f secs\n", BytesCount((float64)(fileSize*noOfDir*noOfFiles), ""), elapsed.Seconds())
	if elapsed.Seconds() >= 1 {
		fmt.Printf("Upload Speed %s \n",
			BytesCount(
				(float64)((float64)(fileSize*noOfDir*noOfFiles)/(float64)(elapsed.Seconds())),
				"rate"))
	} else {
		fmt.Printf("Upload Speed %s \n",
			BytesCount(
				(float64)(fileSize*noOfDir*noOfFiles),
				"rate"))
	}
}

func stressTestDownload(name string, noOfDir int, noOfFiles int, fileSize int) {
	fmt.Printf("Starting Download test...\n")

	var workItemCnt = noOfDir + (noOfDir * noOfFiles)

	jobs := make(chan string, workItemCnt)
	results := make(chan int, workItemCnt)

	for w := 1; w <= noOfWorkers; w++ {
		go downloadWorker(w, jobs, results)
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
				//fmt.Println(path, info.Size())
				jobs <- path
				totalFiles++
				totalBytes += (int)(info.Size())
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
	close(jobs)
	for a := 1; a <= (noOfDir * noOfFiles); a++ {
		<-results
	}
	close(results)

	elapsed := time.Since(startTime)

	fmt.Printf("\nTotal files downloaded : %d\n", totalFiles)
	fmt.Printf("%s bytes read in %.2f secs\n", BytesCount((float64)(totalBytes), ""), (float64)(elapsed.Seconds()))
	if elapsed.Seconds() >= 1 {
		fmt.Printf("Download Speed %s \n",
			BytesCount(
				(float64)((float64)(totalBytes)/(float64)(elapsed.Seconds())),
				"rate"))
	} else {
		fmt.Printf("Download Speed %s \n",
			BytesCount(
				(float64)(totalBytes),
				"rate"))
	}
	fmt.Println("Cleaning up...")
	os.RemoveAll(baseDir + "/" + name)
	fmt.Println("-----------------------------------------------------------------------------------------\n")

}

func main() {
	baseDir = os.Args[1] + "/" + baseDir
	//fmt.Println("Creating temp directories in " + baseDir)

	//  Small file test
	var numSmallDirs int = 50
	var numSmallFiles int = 40
	var smallFileSize int = (1024 * 1024)

	stressTestUpload("small", numSmallDirs, numSmallFiles, smallFileSize)
	stressTestDownload("small", numSmallDirs, numSmallFiles, smallFileSize)

	//  Big file test
	var numBigDirs int = 10
	var numBigFiles int = 5
	var bigFileSize int = (200 * 1024 * 1024)

	stressTestUpload("big", numBigDirs, numBigFiles, bigFileSize)
	stressTestDownload("big", numBigDirs, numBigFiles, bigFileSize)

	//  Big file test
	var numHugeDirs int = 3
	var numHugeFiles int = 1
	var hugeFileSize int = (2 * 1024 * 1024 * 1024)

	stressTestUpload("huge", numHugeDirs, numHugeFiles, hugeFileSize)
	stressTestDownload("huge", numHugeDirs, numHugeFiles, hugeFileSize)

}
