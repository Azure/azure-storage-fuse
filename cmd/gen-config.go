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
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/spf13/cobra"
)

type generatedConfigOptions struct {
	configComp     string
	configTmp      string
	configDirectIO bool
	outputFile     string
}

var optsGenCfg generatedConfigOptions

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

		if optsGenCfg.configComp != "block_cache" && optsGenCfg.configComp != "file_cache" {
			return fmt.Errorf("component is required and should be either block_cache or file_cache. Please use --component flag to specify component name")
		}

		// Check if configTmp is not provided when component is fc
		if optsGenCfg.configComp == "file_cache" && optsGenCfg.configTmp == "" {
			return fmt.Errorf("temp path is required for file cache mode. Use flag --tmp-path to provide the path")
		}

		pipeline := []string{"libfuse", optsGenCfg.configComp}
		common.TmpPath = optsGenCfg.configTmp

		if optsGenCfg.configDirectIO {
			common.DirectIO = true
		} else {
			pipeline = append(pipeline, "attr_cache")
		}

		var sb strings.Builder
		sb.WriteString("# Logger configuration\n#logging:\n  #  type: syslog|silent|base\n  #  level: log_off|log_crit|log_err|log_warning|log_info|log_trace|log_debug\n")
		sb.WriteString("  #  file-path: <path where log files shall be stored. Default - '$HOME/.blobfuse2/blobfuse2.log'>\n")
		sb.WriteString("\ncomponents:\n")

		// Iterate through the pipeline and add each component to the YAML content
		for _, component := range pipeline {
			sb.WriteString(fmt.Sprintf("  - %s\n", component))
		}
		sb.WriteString("  - azstorage\n")

		_, err := internal.NewPipeline(pipeline, true)
		if err != nil {
			return fmt.Errorf("generatedConfig:: error creating pipeline [%s]", err.Error())
		}

		sb.WriteString(common.ConfigYaml)

		sb.WriteString("\n#Required\n#azstorage:\n  #  type: block|adls \n  #  account-name: <name of the storage account>\n  #  container: <name of the storage container to be mounted>\n  #  endpoint: <example - https://account-name.blob.core.windows.net>\n  ")
		sb.WriteString("#  mode: key|sas|spn|msi|azcli \n  #  account-key: <storage account key>\n  # OR\n  #  sas: <storage account sas>\n  # OR\n  #  appid: <storage account app id / client id for MSI>\n  # OR\n  #  tenantid: <storage account tenant id for SPN")

		filePath := ""
		if optsGenCfg.outputFile == "" {
			// DefaultWorkDir := "$HOME/.blobfuse2"
			// DefaultLogFile := filepath.Join(DefaultWorkDir, "generatedConfig.yaml")
			homeDir, err := os.UserHomeDir()
			if err != nil {
				fmt.Println("Error getting home directory:", err)
				return err
			}
			filePath = homeDir + "/.blobfuse2/generatedConfig.yaml"
		} else {
			filePath = optsGenCfg.outputFile
		}

		if optsGenCfg.outputFile == "console" {
			fmt.Println(sb.String())
		} else {
			err = common.WriteToFile(filePath, sb.String(), common.WriteToFileOptions{Flags: os.O_TRUNC, Permission: 0644})
		}

		return err
	},
}

func init() {
	rootCmd.AddCommand(generatedConfig)
	generatedConfig.Flags().StringVar(&optsGenCfg.configComp, "component", "", "Input block_cache or file_cache")
	generatedConfig.Flags().StringVar(&optsGenCfg.configTmp, "tmp-path", "", "Input path for caching")
	generatedConfig.Flags().BoolVar(&optsGenCfg.configDirectIO, "direct_io", false, "Choose direct-io mode")
	generatedConfig.Flags().StringVar(&optsGenCfg.outputFile, "o", "", "Output file location")
}
