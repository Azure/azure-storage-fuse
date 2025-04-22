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
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

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
		lazy, _ := cmd.Flags().GetBool("lazy")
		if strings.Contains(args[0], "*") {
			mntPathPrefix := args[0]

			lstMnt, _ := common.ListMountPoints()
			for _, mntPath := range lstMnt {
				match, _ := regexp.MatchString(mntPathPrefix, mntPath)
				if match {
					err := unmountBlobfuse2(mntPath, lazy)
					if err != nil {
						return fmt.Errorf("failed to unmount %s [%s]", mntPath, err.Error())
					}
				}
			}
		} else {
			err := unmountBlobfuse2(args[0], lazy)
			if err != nil {
				return err
			}
		}

		return nil
	},
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if toComplete == "" {
			mntPts, _ := common.ListMountPoints()
			return mntPts, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
}

// Attempts to unmount the directory and returns true if the operation succeeded
func unmountBlobfuse2(mntPath string, lazy bool) error {
	unmountCmd := []string{"fusermount3", "fusermount"}

	var errb bytes.Buffer
	var err error
	for _, umntCmd := range unmountCmd {
		var args []string
		if lazy {
			args = append(args, "-z")
		}
		args = append(args, "-u", mntPath)
		cliOut := exec.Command(umntCmd, args...)
		cliOut.Stderr = &errb
		_, err = cliOut.Output()

		if err == nil {
			fmt.Println("Successfully unmounted", mntPath)
			return nil
		}

		if !strings.Contains(err.Error(), "executable file not found") {
			log.Err("unmountBlobfuse2 : failed to unmount (%s : %s)", err.Error(), errb.String())
			break
		}
	}

	return fmt.Errorf("%s", errb.String()+" "+err.Error())
}

func init() {
	rootCmd.AddCommand(unmountCmd)
	unmountCmd.AddCommand(umntAllCmd)

	unmountCmd.PersistentFlags().BoolP("lazy", "z", false, "Use lazy unmount")
}
