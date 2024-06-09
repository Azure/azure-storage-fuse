package main

import (
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

func callme(filterArr *[][]Filter, dirs *[][]os.FileInfo) {
	fv := &fileValidator{
		workers:    16,
		atomicflag: 0,
		fileCnt:    0,
		filterArr:  *filterArr,
	}
	fv.wgo.Add(1) //kept outside thread
	fv.outputChan = make(chan opdata, fv.workers)
	fv.fileInpQueue = make(chan os.FileInfo, fv.workers)

	go fv.RecieveOutput()

	for w := 1; w <= fv.workers; w++ {
		// fv.wgi.Add(1)
		go fv.ChkFile() //go routines for each worker (thread) are called
	}
	for _, fileInfos := range *dirs {
		for _, fileinfo := range fileInfos {
			fv.fileInpQueue <- fileinfo //push all files one by one in channel , if channel is full , it will wait
			fv.fileCnt++
		}
	}
	atomic.StoreInt32(&fv.atomicflag, 1)
	close(fv.fileInpQueue) //close channel once all files have been processed
	// fv.wgi.Wait()
	fv.wgo.Wait()                       //wait for completion of all threads
	fmt.Println("All workers stopped ") //exit

	for _, finallist := range fv.finalFiles {
		fmt.Println("List O/P: ", finallist.filenmae)
	}

}
func main() {
	fmt.Println(time.Now())
	filter := flag.String("filter", "!", "enter your filter here") //used to take filter input from the user
	flag.Parse()
	str := (*filter)                        //assign value stored in filter to a string
	filterArr, isInpValid := ParseInp(&str) //parse the string and get an array (splitted on basis of ||) of array(splitted on basis of &&) of filters
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
	var dirs [][]os.FileInfo
	dirPath := "../../../test"
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
	dirs = append(dirs, fileInfos)
	callme(&filterArr, &dirs)
	// fv := &fileValidator{
	// 	workers:    16,
	// 	atomicflag: 0,
	// 	fileCnt:    0,
	// 	filterArr:  filterArr,
	// }
	// fv.wgo.Add(1) //kept outside thread
	// fv.outputChan = make(chan opdata, fv.workers)
	// fv.fileInpQueue = make(chan os.FileInfo, fv.workers)

	// go fv.RecieveOutput()

	// for w := 1; w <= fv.workers; w++ {
	// 	// fv.wgi.Add(1)
	// 	go fv.ChkFile() //go routines for each worker (thread) are called
	// }
	// for _, fileInfos := range dirs {
	// 	for _, fileinfo := range fileInfos {
	// 		fv.fileInpQueue <- fileinfo //push all files one by one in channel , if channel is full , it will wait
	// 		fv.fileCnt++
	// 	}
	// }
	// atomic.StoreInt32(&fv.atomicflag, 1)
	// close(fv.fileInpQueue) //close channel once all files have been processed
	// // fv.wgi.Wait()
	// fv.wgo.Wait()                       //wait for completion of all threads
	// fmt.Println("All workers stopped ") //exit

	// for _, finallist := range fv.finalFiles {
	// 	fmt.Println("List O/P: ", finallist.filenmae)
	// }

	fmt.Println(time.Now())
}
