// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
)

var check bool

var versionCmd = &cobra.Command{
	Use:   "version [--check]",
	Short: "Command to print the current version along with optional check for latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("blobfuse2 version: %s\n", common.Blobfuse2Version)
		if check {
			return VersionCheck()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().BoolVar(&check, "check", false, "To check whether latest version exists or not")
}
