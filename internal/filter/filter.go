package filter

import (
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func (fl *UserInputFilters) ApplyFilterOnBlobs(fileInfos []*internal.ObjAttr) []*internal.ObjAttr { //function called from azstorage.go streamDir func
	log.Debug("came inside filter")
	if len(fileInfos) == 0 {
		return fileInfos
	}
	fv := &FileValidator{
		workers:   16,
		fileCnt:   int64(len(fileInfos)),
		FilterArr: fl.FilterArr,
	}
	fv.wgo.Add(1) //kept outside thread
	fv.outputChan = make(chan *opdata, fv.workers)
	fv.fileInpQueue = make(chan *internal.ObjAttr, fv.workers)

	go fv.RecieveOutput() //thread parellely reading from ouput channel

	for w := 1; w <= fv.workers; w++ {
		go fv.ChkFile() //go routines for each worker (thread) are called
	}
	for _, fileinfo := range fileInfos {
		// fmt.Println("passedFile: ", *fileinfo)
		fv.fileInpQueue <- fileinfo //push all files one by one in channel , if channel is full , it will wait
	}

	close(fv.fileInpQueue) //close channel once all files have been processed

	fv.wgo.Wait() //wait for completion of all threads
	// fmt.Println("All workers stopped ") //exit
	log.Debug("moved out of filter")

	return fv.finalFiles
}
