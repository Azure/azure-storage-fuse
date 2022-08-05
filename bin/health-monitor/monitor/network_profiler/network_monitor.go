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

package network_monitor

import (
	"fmt"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
	defer hmcommon.Wg.Done()

	err := nw.Validate()
	if err != nil {
		log.Err("network_monitor::Monitor : [%v]", err)
		return err
	}

	return nil
}

func (nw *NetworkProfiler) ExportStats() {
	fmt.Println("Inside network export stats")
}

func (nw *NetworkProfiler) Validate() error {
	if len(nw.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if nw.pollInterval == 0 {
		return fmt.Errorf("stats-poll-interval should be non-zero")
	}

	return nil
}

func NewNetworkMonitor() hminternal.Monitor {
	nw := &NetworkProfiler{
		pid:          hmcommon.Pid,
		pollInterval: hmcommon.StatsPollinterval,
	}

	nw.SetName(hmcommon.NetworkProfiler)

	return nw
}

func init() {
	hminternal.AddMonitor(hmcommon.NetworkProfiler, NewNetworkMonitor)
}
