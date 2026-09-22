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

package cpumemprofiler

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type CPUMemProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (cm *CPUMemProfiler) GetName() string {
	return cm.name
}

func (cm *CPUMemProfiler) SetName(name string) {
	cm.name = name
}

func (cm *CPUMemProfiler) Monitor() error {
	err := cm.Validate()
	if err != nil {
		log.Err("cpu_mem_monitor::Monitor : [%v]", err)
		return err
	}
	log.Debug("cpu_mem_monitor::Monitor : started")

	ticker := time.NewTicker(time.Duration(cm.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := cm.getCPUMemoryUsage()
		if err != nil {
			log.Err("cpu_mem_monitor::Monitor : [%v]", err)
			return err
		}

		if !hmcommon.NoCPUProf {
			cm.ExportStats(t.Format(time.RFC3339), c.CPUUsage)
		}
		if !hmcommon.NoMemProf {
			cm.ExportStats(t.Format(time.RFC3339), c.MemUsage)
		}
	}

	return nil
}

func (cm *CPUMemProfiler) ExportStats(timestamp string, st any) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("cpu_mem_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	s := st.(string)
	if s[len(s)-1] == '%' {
		se.AddMonitorStats(hmcommon.CPUProfiler, timestamp, st)
	} else {
		se.AddMonitorStats(hmcommon.MemoryProfiler, timestamp, st)
	}
}

func (cm *CPUMemProfiler) Validate() error {
	if len(cm.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if cm.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func (cm *CPUMemProfiler) getCPUMemoryUsage() (*hmcommon.CPUMemStat, error) {
	topCmd := "top -b -n 1 -d 0.2 -p " + cm.pid + " | tail -2"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err("cpu_mem_monitor::getCPUMemoryUsage : Blobfuse2 is not running on pid %v [%v]", cm.pid, err)
		return nil, err
	}

	processes := strings.Split(strings.Trim(string(cliOut), "\n"), "\n")
	if len(processes) < 2 {
		log.Err("cpu_mem_monitor::getCPUMemoryUsage : Blobfuse2 is not running on pid %v", cm.pid)
		return nil, fmt.Errorf("blobfuse2 is not running on pid %v", cm.pid)
	}

	cpuIndex, memIndex := getCPUMemIndex(processes[0])
	stats := strings.Fields(processes[1])
	if cpuIndex == -1 || memIndex == -1 || len(stats) <= int(math.Max(float64(cpuIndex), float64(memIndex))) || len(stats[cpuIndex]) == 0 || len(stats[memIndex]) == 0 {
		log.Debug("cpu_mem_monitor::getCPUMemoryUsage : %v", processes)
		log.Err("cpu_mem_monitor::getCPUMemoryUsage : Blobfuse2 is not running on pid %v", cm.pid)
		return nil, fmt.Errorf("blobfuse2 is not running on pid %v", cm.pid)
	}

	cpuMemStat := &hmcommon.CPUMemStat{
		CPUUsage: stats[cpuIndex],
		MemUsage: stats[memIndex],
	}
	cpuMemStat.CPUUsage += "%"
	if cpuMemStat.MemUsage[len(cpuMemStat.MemUsage)-1] >= '0' && cpuMemStat.MemUsage[len(cpuMemStat.MemUsage)-1] <= '9' {
		cpuMemStat.MemUsage += "k"
	}

	return cpuMemStat, nil
}

func getCPUMemIndex(process string) (int, int) {
	cols := strings.Fields(process)
	var cpuIndex, memIndex = -1, -1
	for i, col := range cols {
		switch col {
		case "%CPU":
			cpuIndex = i
		case "VIRT":
			memIndex = i
		}
	}
	return cpuIndex, memIndex
}

func NewCPUMemoryMonitor() hminternal.Monitor {
	cm := &CPUMemProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	cm.SetName(hmcommon.CPUMemoryProfiler)

	return cm
}

func init() {
	hminternal.AddMonitor(hmcommon.CPUMemoryProfiler, NewCPUMemoryMonitor)
}
