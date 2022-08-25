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

package cpu_profiler

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type CpuProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (cpu *CpuProfiler) GetName() string {
	return cpu.name
}

func (cpu *CpuProfiler) SetName(name string) {
	cpu.name = name
}

func (cpu *CpuProfiler) Monitor() error {
	err := cpu.Validate()
	if err != nil {
		log.Err("cpu_monitor::Monitor : [%v]", err)
		return err
	}

	ticker := time.NewTicker(time.Duration(cpu.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := cpu.getCpuUsage()
		if err != nil {
			log.Err("cpu_monitor::Monitor : [%v]", err)
			return err
		}

		log.Debug("CPU Usage : %v at %v", c, t.Format(time.RFC3339))
		cpu.ExportStats(t.Format(time.RFC3339), c)
	}

	return nil
}

func (cpu *CpuProfiler) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("cpu_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(cpu.GetName(), timestamp, st)
}

func (cpu *CpuProfiler) Validate() error {
	if len(cpu.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if cpu.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func (cpu *CpuProfiler) getCpuUsage() (string, error) {
	topCmd := "top -b -n 1 -d 0.2 -p " + cpu.pid + " | tail -1 | awk '{print $9}'"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err("cpu_monitor::getCpuUsage : Blobfuse2 is not running on pid %v [%v]", cpu.pid, err)
		return "", err
	}

	stats := strings.Split(strings.Split(string(cliOut), "\n")[0], " ")

	if stats[0] == "%CPU" {
		log.Err("cpu_monitor::getCpuUsage : Blobfuse2 is not running on pid %v", cpu.pid)
		return "", fmt.Errorf("blobfuse2 is not running on pid %v", cpu.pid)
	}

	return stats[0], nil
}

func NewCpuMonitor() hminternal.Monitor {
	cpu := &CpuProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	cpu.SetName(hmcommon.CpuProfiler)

	return cpu
}

func init() {
	hminternal.AddMonitor(hmcommon.CpuProfiler, NewCpuMonitor)
}
