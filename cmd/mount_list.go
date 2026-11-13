// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
)

var mountListCmd = &cobra.Command{
	Use:        "list",
	Short:      "List all blobfuse2 mountpoints",
	Long:       "List all blobfuse2 mountpoints",
	SuggestFor: []string{"lst", "list"},
	Example:    "blobfuse2 mount list",
	RunE: func(cmd *cobra.Command, args []string) error {
		lstMnt, err := common.ListMountPoints()
		if err != nil {
			return fmt.Errorf("failed to list mount points [%s]", err.Error())
		}

		for i, mntPath := range lstMnt {
			fmt.Println(i+1, ":", mntPath)
		}

		return nil
	},
}
