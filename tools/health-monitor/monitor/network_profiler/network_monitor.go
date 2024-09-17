/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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

package network_monitor

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"
)

type NetworkProfiler struct {
	name         string
	pid          string
	pollInterval int
}

func (nw *NetworkProfiler) GetName() string {
	return nw.name
}

func (nw *NetworkProfiler) SetName(name string) {
	nw.name = name
}

func (nw *NetworkProfiler) Monitor() error {
	err := nw.Validate()
	if err != nil {
		log.Err("network_monitor::Monitor : [%v]", err)
		return err
	}

	return nil
}

func (nw *NetworkProfiler) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("network_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(nw.GetName(), timestamp, st)
}

func (nw *NetworkProfiler) Validate() error {
	if len(nw.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if nw.pollInterval == 0 {
		return fmt.Errorf("process-monitor-interval-sec should be non-zero")
	}

	return nil
}

func NewNetworkMonitor() hminternal.Monitor {
	nw := &NetworkProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.ProcMonInterval,
	}

	nw.SetName(hmcommon.NetworkProfiler)

	return nw
}

func init() {
	// commenting this for now
	// hminternal.AddMonitor(hmcommon.NetworkProfiler, NewNetworkMonitor)
}
