package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"time"

	"github.com/Azure/azure-storage-azcopy/v10/common"
	"github.com/Azure/azure-storage-azcopy/v10/ste"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

func initSTE() (err error) {
	jobID := common.NewJobID()
	tuner := ste.NullConcurrencyTuner{FixedValue: 128}
	var pacer ste.PacerAdmin = ste.NewNullAutoPacer()
	var logLevel common.LogLevel
	common.AzcopyJobPlanFolder = "/home/vikas/blobfusetmp"

	if err := logLevel.Parse("debug"); err != nil {
		logLevel = common.ELogLevel.Info()
	}
	logger := common.NewSysLogger(jobID, logLevel, "blobfuse")
	logger.OpenLog()

	os.MkdirAll(common.AzcopyJobPlanFolder, 0666)
	jobMgr = ste.NewJobMgr(ste.NewConcurrencySettings(math.MaxInt32, false),
		jobID,
		context.Background(),
		common.NewNullCpuMonitor(),
		common.ELogLevel.Error(),
		"Lustre",
		"/home/vikas/blobfusetmp",
		&tuner,
		pacer,
		common.NewMultiSizeSlicePool(4*1024*1024*1024 /* 4GiG */),
		common.NewCacheLimiter(int64(2*1024*1024*1024)),
		common.NewCacheLimiter(int64(64)),
		logger)

	/*
		This needs to be moved to a better location
	*/
	go func() {
		time.Sleep(20 * time.Second) // wait a little, so that our initial pool of buffers can get allocated without heaps of (unnecessary) GC activity
		debug.SetGCPercent(20)       // activate more aggressive/frequent GC than the default
	}()

	//util.SetJobMgr(jobMgr)
	//util.RestPartNum()
	//common.GetLifecycleMgr().E2EEnableAwaitAllowOpenFiles(false)
	//common.GetLifecycleMgr().SetForceLogging()

	return nil
}

func main() {
	fmt.Println(os.Getenv("AZCOPY_BUFFER_GB"))

	initSTE()

	/*
		err := Download(<sas url>,
			"./mnt.sh",
			(8 * 1024 * 1024))
		if err != nil {
			fmt.Println(err.Error())
		}
	*/

	err := Upload("/home/vikas/mnt.sh",
	<sas url>,
		(8 * 1024 * 1024),
		azblob.Metadata{})
	if err != nil {
		fmt.Println(err.Error())
	}

	/*
		err = Upload("/home/vikas/clear.sh",
			<sas url>,
			(8 * 1024 * 1024),
			azblob.Metadata{})
		if err != nil {
			fmt.Println(err.Error())
		}
	*/

}
