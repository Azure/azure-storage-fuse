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

package common

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// check whether blobfuse2 process is running for the given pid
func CheckProcessStatus(pid string) error {
	cmd := "ps -ef | grep " + pid
	cliOut, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	processes := strings.Split(string(cliOut), "\n")
	for _, process := range processes {
		l := strings.Fields(process)
		if len(l) >= 2 && l[1] == pid {
			return nil
		}
	}

	return fmt.Errorf("blobfuse2 is not running on pid %v", pid)
}

// check blobfuse2 pid status at every second
func MonitorPid() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		err := CheckProcessStatus(Pid)
		if err != nil {
			log.Err("util::MonitorPid : time = %v, [%v]", t.Format(time.RFC3339), err)
			// wait for 5 seconds for monitor threads to exit
			time.Sleep(5 * time.Second)
			break
		}
	}
}
