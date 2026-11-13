// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
	Use:        "stop",
	Short:      "Stops the health monitor binary associated with a given Blobfuse2 pid",
	Long:       "Stops the health monitor binary associated with a given Blobfuse2 pid",
	SuggestFor: []string{"stp", "st"},
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
