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
	"fmt"
	"os"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/spf13/cobra"
)

type generatedConfigOptions struct {
	configComp     string
	configTmp      string
	configDirectIO bool
}

var opts2 generatedConfigOptions

var generatedConfig = &cobra.Command{
	Use:               "gen-config",
	Short:             "Generate default config file.",
	Long:              "Generate default config file with the values pre-caculated by blobfuse2.",
	SuggestFor:        []string{"generate default config", "generate config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		common.GenConfig = true

		if opts2.configComp != "block_cache" && opts2.configComp != "file_cache" {
			return fmt.Errorf("component is required and should be either block_cache or file_cache")
		}

		// Check if configTmp is not provided when component is fc
		if opts2.configComp == "file_cache" && opts2.configTmp == "" {
			return fmt.Errorf("temp path is required for file cache mode. Use flag --tmp-path to provide the path")
		}

		pipeline := []string{"libfuse"}
		if opts2.configComp == "block_cache" {
			pipeline = append(pipeline, "block_cache")
			common.TmpPath = opts2.configTmp
		} else if opts2.configComp == "file_cache" {
			pipeline = append(pipeline, "file_cache")
			common.TmpPath = opts2.configTmp
		}
		if opts2.configDirectIO {
			common.DirectIO = true
		} else {
			pipeline = append(pipeline, "attr_cache")
		}

		yamlContent := "# Logger configuration\n#logging:\n  #  type: syslog|silent|base\n  #  level: log_off|log_crit|log_err|log_warning|log_info|log_trace|log_debug\n  #  file-path: <path where log files shall be stored. Default - '$HOME/.blobfuse2/blobfuse2.log'>\n"
		yamlContent += "\ncomponents:\n"

		// Iterate through the pipeline and add each component to the YAML content
		for _, component := range pipeline {
			yamlContent += fmt.Sprintf("  - %s\n", component)
		}
		yamlContent += "  - azstorage\n"

		_, err := internal.NewPipeline(pipeline, true)
		if err != nil {
			return fmt.Errorf("generatedConfig:: error creating pipeline [%s]", err.Error())
		}

		yamlContent += common.ConfigYaml

		yamlContent += "\n#Required\n#azstorage:\n  #  type: block|adls \n  #  account-name: <name of the storage account>\n  #  container: <name of the storage container to be mounted>\n  #  endpoint: <example - https://account-name.blob.core.windows.net>\n  #  mode: key|sas|spn|msi|azcli \n  #  account-key: <storage account key>\n  # OR\n  #  sas: <storage account sas>\n  # OR\n  #  appid: <storage account app id / client id for MSI>\n  # OR\n  #  tenantid: <storage account tenant id for SPN"

		// Open the file in append mode, create it if it doesn't exist
		file, err := os.OpenFile("generatedConfig.yaml", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return fmt.Errorf("error opening generated config file: [%s]", err.Error())
		}
		defer file.Close() // Ensure the file is closed when we're done

		// Write the YAML content to the file
		if _, err := file.WriteString(yamlContent); err != nil {
			return fmt.Errorf("error writing to generated config file [%s]", err.Error())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(generatedConfig)
	generatedConfig.Flags().StringVar(&opts2.configComp, "component", "", "Input block_cache or file_cache")
	generatedConfig.Flags().StringVar(&opts2.configTmp, "tmp-path", "", "Input path for caching")
	generatedConfig.Flags().BoolVar(&opts2.configDirectIO, "direct-io", false, "Choose direct-io mode")
}
