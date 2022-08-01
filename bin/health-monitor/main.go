package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	_ "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/monitor"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

func getMonitors() []hminternal.Monitor {
	compMap := map[string]bool{
		hmcommon.Blobfuse_stats:   hmcommon.NoBfsMon,
		hmcommon.Cpu_profiler:     hmcommon.NoCpuProf,
		hmcommon.Memory_profiler:  hmcommon.NoMemProf,
		hmcommon.Network_profiler: hmcommon.NoNetProf,
		hmcommon.File_cache:       hmcommon.NoFileCacheMon,
	}

	comps := make([]hminternal.Monitor, 0)

	for name, disable := range compMap {
		if !disable {
			obj, err := hminternal.GetMonitor(name)
			if err != nil {
				fmt.Printf("main::getMonitors : [%v]", err)
				continue
			}
			comps = append(comps, obj)
		}
	}

	return comps
}

func main() {
	flag.Parse()

	err := log.SetDefaultLogger("base", common.LogConfig{
		Level:       common.ELogLevel.LOG_DEBUG(),
		FilePath:    os.ExpandEnv(hmcommon.DefaultLogFile),
		MaxFileSize: common.DefaultMaxLogFileSize,
		FileCount:   common.DefaultLogFileCount,
		TimeTracker: false,
		Tag:         hmcommon.HealthMon,
	})

	if err != nil {
		fmt.Printf("Health Monitor: error initializing logger [%v]", err)
		log.Debug("main::main : error initializing logger [%v]", err)
		os.Exit(1)
	}

	if len(strings.TrimSpace(hmcommon.Pid)) == 0 {
		fmt.Printf("pid of blobfuse process not provided\n")
		log.Debug("main::main : pid of blobfuse process not provided")
		time.Sleep(1 * time.Second) // adding 1 second wait for adding to log before exiting
		os.Exit(1)
	}

	fmt.Println(hmcommon.Pid, hmcommon.BfsPollInterval, hmcommon.StatsPollinterval, hmcommon.NoBfsMon, hmcommon.NoCpuProf, hmcommon.NoMemProf, hmcommon.NoNetProf, hmcommon.NoFileCacheMon)

	comps := getMonitors()

	for _, obj := range comps {
		obj.ExportStats()
	}

}

func init() {
	flag.StringVar(&hmcommon.Pid, "pid", "", "Pid of Blobfuse")
	flag.IntVar(&hmcommon.BfsPollInterval, "blobfuse-poll-interval", 5, "Blobfuse stats polling interval in seconds")
	flag.IntVar(&hmcommon.StatsPollinterval, "stats-poll-interval", 10, "CPU, memory and network usage polling interval in seconds")

	flag.BoolVar(&hmcommon.NoBfsMon, "blobfuse-stats", false, "Enable blobfuse stats polling")
	flag.BoolVar(&hmcommon.NoCpuProf, "cpu-profiler", false, "Enable CPU profiling on blobfuse process")
	flag.BoolVar(&hmcommon.NoMemProf, "memory-profiler", false, "Enable memory profiling on blobfuse process")
	flag.BoolVar(&hmcommon.NoNetProf, "network-profiler", false, "Enable network profiling on blobfuse process")
	flag.BoolVar(&hmcommon.NoFileCacheMon, "file-cache", false, "Enable file cache directory monitor")
}
