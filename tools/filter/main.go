package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	fmt.Println(time.Now())
	filter := flag.String("filter", "!", "enter your filter here") //used to take filter input from the user
	flag.Parse()
	str := (*filter)                       //assign value stored in filter to a string
	filterArr, isInpValid := ParseInp(str) //parse the string and get an array (splitted on basis of ||) of array(splitted on basis of &&) of filters
	fmt.Println(time.Now())
	if !isInpValid { //if input given by user is not valid, display and return
		fmt.Println("Wrong input format, Try again.")
		return
	}
	for i, innerArray := range filterArr {
		fmt.Println("Inner array: ", i+1)
		for _, data := range innerArray {
			fmt.Println(data)
		}
	}
	dirPath := "../../../TstData"
	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Println("Error opening directory:", err)
		return
	}
	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		fmt.Println("error reading directory:", err)
		return
	}
	const workers = 16
	var atomicFlag int32 = 0
	var wgo sync.WaitGroup //Number of threads that will be working concurrently
	outputChan := make(chan opdata, workers)
	var cnt int64 = 0
	wgo.Add(1)
	go func() {
		defer wgo.Done()
		var counter int64 = 0
		for data := range outputChan {
			counter++
			fmt.Println("OutPut Channel: ", data.filenmae, " ", data.ispassed)
			// Check if the atomic variable is true
			if atomic.LoadInt32(&atomicFlag) == 1 && counter == cnt {
				close(outputChan)
				break
			}
		}
	}()

	fileInpQueue := make(chan os.FileInfo, workers) //made a channel to store input files
	var wgi sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wgi.Add(1)
		go ChkFile(w, fileInpQueue, &wgi, filterArr, outputChan) //go routines for each worker (thread) are called
	}
	for _, fileinfo := range fileInfos {
		fileInpQueue <- fileinfo //push all files one by one in channel , if channel is full , it will wait
		cnt++
	}
	atomic.StoreInt32(&atomicFlag, 1)
	close(fileInpQueue) //close channel once all files have been processed
	wgi.Wait()
	wgo.Wait()                          //wait for completion of all threads
	fmt.Println("All workers stopped ") //exit
	fmt.Println(time.Now())
}
