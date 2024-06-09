package filter

import (
	"fmt"
	"os"
	"sync/atomic"
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
