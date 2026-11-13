// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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

	processes := strings.SplitSeq(string(cliOut), "\n")
	for process := range processes {
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
