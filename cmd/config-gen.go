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

package cmd

import (
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/spf13/cobra"
)

type configGenOptions struct {
	configComp string
}

var opts configGenOptions

var generateTestConfig = &cobra.Command{
	Use:               "gen-test-config",
	Short:             "Generate config file for testing given an output path.",
	Long:              "Generate config file for testing given an output path.",
	SuggestFor:        []string{"conv test config", "convert test config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		pipeline := []string{"libfuse"}
		if opts.configComp == "bc" {
			pipeline = append(pipeline, "block_cache")
		} else if opts.configComp == "fc" {
			pipeline = append(pipeline, "file_cache")
		}
		pipeline = append(pipeline, "attr_cache")
		pipeline = append(pipeline, "azstorage")
		options.Components = pipeline

		common.GenConfig = true

		newPipeline, _ := internal.NewPipeline(pipeline, true)
		print(newPipeline)

		// write the config with the params to the output file
		// err = os.WriteFile(opts.outputConfigPath, []byte(newConfig), 0700)
		// if err != nil {
		// 	return fmt.Errorf("failed to write file [%s]", err.Error())
		// }

		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateTestConfig)
	generateTestConfig.Flags().StringVar(&opts.configComp, "component", "", "Input bc or fc.")
	// generateTestConfig.Flags().StringVar(&opts.configComp, "component", "", "Input temppath")
}
