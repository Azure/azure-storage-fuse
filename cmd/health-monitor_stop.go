/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var blobfuse2Pid string

var healthMonStop = &cobra.Command{
	Use:               "stop",
	Short:             "Stops the health monitor binary associated with a given Blobfuse2 pid",
	Long:              "Stops the health monitor binary associated with a given Blobfuse2 pid",
	SuggestFor:        []string{"stp", "st"},
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		blobfuse2Pid = strings.TrimSpace(blobfuse2Pid)

		if len(blobfuse2Pid) == 0 {
			return fmt.Errorf("pid of blobfuse2 process not given")
		}

		pid, err := getPid(blobfuse2Pid)
		if err != nil {
			return fmt.Errorf("failed to get health monitor pid")
		}

		err = stop(pid)
		if err != nil {
			return fmt.Errorf("failed to stop health monitor")
		}

		return nil
	},
}

// Attempts to get pid of the health monitor
func getPid(blobfuse2Pid string) (string, error) {
	psAux := exec.Command("ps", "aux")
	out, err := psAux.Output()
	if err != nil {
		return "", err
	}
	processes := strings.SplitSeq(string(out), "\n")
	for process := range processes {
		if strings.Contains(process, "bfusemon") && strings.Contains(process, fmt.Sprintf("--pid=%s", blobfuse2Pid)) {
			re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
			pids := re.FindAllString(process, 1)
			if pids == nil {
				return "", fmt.Errorf("failed to process PID from %s", process)
			}
			pid := pids[0]
			fmt.Printf("Successfully got health monitor PID %s.\n", pid)
			return pid, nil
		}
	}
	return "", fmt.Errorf("no corresponding health monitor PID found")

}

// Attempts to kill all health monitors
func stop(pid string) error {
	cliOut := exec.Command("kill", "-9", pid)
	_, err := cliOut.Output()
	if err != nil {
		return err
	} else {
		fmt.Println("Successfully stopped health monitor binary.")
		return nil
	}
}

func init() {
	healthMonCmd.AddCommand(healthMonStop)
	healthMonStop.AddCommand(healthMonStopAll)

	healthMonStop.Flags().StringVar(&blobfuse2Pid, "pid", "", "Blobfuse2 PID associated with the health monitor that should be stopped")
	_ = healthMonStop.MarkFlagRequired("pid")
}
