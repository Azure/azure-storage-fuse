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

package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
)

var unmountCmd = &cobra.Command{
	Use:               "unmount <mount path>",
	Short:             "Unmount Blobfuse2",
	Long:              "Unmount Blobfuse2",
	SuggestFor:        []string{"unmount", "unmnt"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.Contains(args[0], "*") {
			mntPathPrefix := args[0]

			lstMnt, err := common.ListMountPoints()
			if err != nil {
				return fmt.Errorf("failed to list mount points [%s]", err.Error())
			}

			for _, mntPath := range lstMnt {
				match, err := regexp.MatchString(mntPathPrefix, mntPath)
				if err != nil {
					fmt.Printf("Pattern matching failed for mount point %s [%s]\n", mntPath, err.Error())
				}
				if match {
					err := unmountBlobfuse2(mntPath)
					if err != nil {
						return fmt.Errorf("failed to unmount %s [%s]", mntPath, err.Error())
					}
				}
			}
		} else {
			err := unmountBlobfuse2(args[0])
			if err != nil {
				return fmt.Errorf("failed to unmount %s [%s]", args[0], err.Error())
			}
		}

		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if toComplete == "" {
			mntPts, err := common.ListMountPoints()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return mntPts, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
}

// Attempts to unmount the directory and returns true if the operation succeeded
func unmountBlobfuse2(mntPath string) error {
	cliOut := exec.Command("fusermount", "-u", mntPath)
	_, err := cliOut.Output()
	if err != nil {
		return err
	} else {
		fmt.Println("Successfully unmounted", mntPath)
		return nil
	}
}

func init() {
	rootCmd.AddCommand(unmountCmd)
	unmountCmd.AddCommand(umntAllCmd)
}
