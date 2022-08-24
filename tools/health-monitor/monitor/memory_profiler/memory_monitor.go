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

package memory_profiler

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type MemoryProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (mem *MemoryProfiler) GetName() string {
	return mem.name
}

func (mem *MemoryProfiler) SetName(name string) {
	mem.name = name
}

func (mem *MemoryProfiler) Monitor() error {
	defer hmcommon.Wg.Done()

	err := mem.Validate()
	if err != nil {
		log.Err("memory_monitor::Monitor : [%v]", err)
		return err
	}

	ticker := time.NewTicker(time.Duration(mem.pollInterval) * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		c, err := mem.getMemoryUsage()
		if err != nil {
			log.Err("memory_monitor::Monitor : [%v]", err)
			return err
		}

		log.Debug("Memory Usage : %v at %v", c, t.Format(time.RFC3339))
		mem.ExportStats(t.Format(time.RFC3339), c)
	}

	return nil
}

func (mem *MemoryProfiler) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("memory_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(mem.GetName(), timestamp, st)
}

func (mem *MemoryProfiler) Validate() error {
	if len(mem.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if mem.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func (mem *MemoryProfiler) getMemoryUsage() (string, error) {
	topCmd := "top -b -n 1 -d 0.2 -p " + mem.pid + " | tail -1 | awk '{print $10}'"

	cliOut, err := exec.Command("bash", "-c", topCmd).Output()
	if err != nil {
		log.Err("memory_monitor::getMemoryUsage : Blobfuse2 is not running on pid %v [%v]", mem.pid, err)
		return "", err
	}

	stats := strings.Split(strings.Split(string(cliOut), "\n")[0], " ")

	if stats[0] == "%MEM" {
		log.Err("memory_monitor::getMemoryUsage : Blobfuse2 is not running on pid %v", mem.pid)
		return "", fmt.Errorf("blobfuse2 is not running on pid %v", mem.pid)
	}

	return stats[0], nil
}

func NewMemoryMonitor() hminternal.Monitor {
	mem := &MemoryProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	mem.SetName(hmcommon.MemoryProfiler)

	return mem
}

func init() {
	hminternal.AddMonitor(hmcommon.MemoryProfiler, NewMemoryMonitor)
}
