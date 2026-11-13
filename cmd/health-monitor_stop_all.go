// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
)

var healthMonStopAll = &cobra.Command{
	Use:        "all",
	Short:      "Stop all health monitor binaries",
	Long:       "Stop all health monitor binaries",
	SuggestFor: []string{"al", "all"},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := stopAll()
		if err != nil {
			return fmt.Errorf("failed to stop all health monitor binaries [%s]", err.Error())
		}
		return nil
	},
}

// Attempts to kill all health monitors
func stopAll() error {
	cliOut := exec.Command("killall", hmcommon.BfuseMon)
	_, err := cliOut.Output()
	if err != nil {
		return err
	} else {
		fmt.Println("Successfully stopped all health monitor binaries.")
		return nil
	}
}
