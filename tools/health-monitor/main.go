/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
	_ "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/monitor"
)

func getMonitors() []hminternal.Monitor {
	compMap := map[string]bool{
		hmcommon.BlobfuseStats:   hmcommon.NoBfsMon,
		hmcommon.CpuProfiler:     hmcommon.NoCpuProf,
		hmcommon.MemoryProfiler:  hmcommon.NoMemProf,
		hmcommon.NetworkProfiler: hmcommon.NoNetProf,
		hmcommon.FileCacheMon:    hmcommon.NoFileCacheMon,
	}

	comps := make([]hminternal.Monitor, 0)

	for name, disabled := range compMap {
		if !disabled {
			obj, err := hminternal.GetMonitor(name)
			if err != nil {
				log.Err("main::getMonitors : [%v]", err)
				continue
			}
			comps = append(comps, obj)
		}
	}

	return comps
}

func main() {
	flag.Parse()

	err := log.SetDefaultLogger("syslog", common.LogConfig{
		Level:       common.ELogLevel.LOG_DEBUG(),
		FilePath:    os.ExpandEnv(hmcommon.DefaultLogFile),
		MaxFileSize: common.DefaultMaxLogFileSize,
		FileCount:   common.DefaultLogFileCount,
		TimeTracker: false,
		Tag:         hmcommon.HealthMon,
	})

	if err != nil {
		fmt.Printf("Health Monitor: error initializing logger [%v]", err)
		os.Exit(1)
	}

	if len(strings.TrimSpace(hmcommon.Pid)) == 0 {
		fmt.Printf("pid of blobfuse2 process not provided\n")
		log.Err("main::main : pid of blobfuse2 process not provided")
		time.Sleep(1 * time.Second) // adding 1 second wait for adding to log(base type) before exiting
		os.Exit(1)
	}

	common.TransferPipe += "_" + hmcommon.Pid
	common.PollingPipe += "_" + hmcommon.Pid

	log.Debug("Blobfuse2 Pid: %v \n"+
		"Transfer Pipe: %v \n"+
		"Polling Pipe: %v \n"+
		"Blobfuse2 Stats poll interval: %v \n"+
		"Health Stats poll interval: %v \n"+
		"Cache Path: %v \n"+
		"Max cache size in MB: %v",
		hmcommon.Pid, common.TransferPipe, common.PollingPipe, hmcommon.BfsPollInterval,
		hmcommon.StatsPollinterval, hmcommon.TempCachePath, hmcommon.MaxCacheSize)

	comps := getMonitors()

	for _, obj := range comps {
		hmcommon.Wg.Add(1)
		go obj.Monitor()
	}

	hmcommon.Wg.Wait()

	err = hminternal.CloseExporter()
	if err != nil {
		log.Err("main::main : Unable to close exporter [%v]", err)
		os.Exit(1)
	}

	log.Debug("Monitoring ended")
}

func init() {
	flag.StringVar(&hmcommon.Pid, "pid", "", "Pid of blobfuse2 process")
	flag.IntVar(&hmcommon.BfsPollInterval, "blobfuse2-poll-interval", 5, "Blobfuse2 stats polling interval in seconds")
	flag.IntVar(&hmcommon.StatsPollinterval, "stats-poll-interval", 10, "CPU, memory and network usage polling interval in seconds")

	flag.BoolVar(&hmcommon.NoBfsMon, "no-blobfuse2-stats", false, "Disable blobfuse2 stats polling")
	flag.BoolVar(&hmcommon.NoCpuProf, "no-cpu-profiler", false, "Disable CPU monitoring on blobfuse2 process")
	flag.BoolVar(&hmcommon.NoMemProf, "no-memory-profiler", false, "Disable memory monitoring on blobfuse2 process")
	flag.BoolVar(&hmcommon.NoNetProf, "no-network-profiler", false, "Disable network monitoring on blobfuse2 process")
	flag.BoolVar(&hmcommon.NoFileCacheMon, "no-cache-monitor", false, "Disable file cache directory monitor")

	flag.StringVar(&hmcommon.TempCachePath, "cache-path", "", "path to local disk cache")
	flag.Float64Var(&hmcommon.MaxCacheSize, "max-size-mb", 0, "maximum cache size allowed. Default - 0 (unlimited)")
}
