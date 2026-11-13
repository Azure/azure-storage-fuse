// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
)

var umntAllCmd = &cobra.Command{
	Use:        "all",
	Short:      "Unmount all instances of Blobfuse2",
	Long:       "Unmount all instances of Blobfuse2",
	SuggestFor: []string{"al", "all"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		lazy, _ := cmd.Flags().GetBool("lazy")
		lstMnt, err := common.ListMountPoints()
		if err != nil {
			return fmt.Errorf("failed to list mount points [%s]", err.Error())
		}

		mountfound := 0
		unmounted := 0
		errMsg := "failed to unmount - \n"

		for _, mntPath := range lstMnt {
			mountfound += 1
			err := unmountBlobfuse2(mntPath, lazy, false)
			if err == nil {
				unmounted += 1
			} else {
				errMsg += " " + mntPath + " - [" + err.Error() + "]\n"
			}
		}

		if mountfound == 0 {
			fmt.Println("Nothing to unmount")
		} else {
			fmt.Printf("%d of %d mounts were successfully unmounted\n", unmounted, mountfound)
		}

		if unmounted < mountfound {
			return errors.New(errMsg)
		}

		return nil
	},
}
