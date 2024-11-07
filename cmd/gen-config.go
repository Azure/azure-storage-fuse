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
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/spf13/cobra"
)

type genConfigParams struct {
	blockCache bool   `config:"block-cache" yaml:"block-cache,omitempty"`
	directIO   bool   `config:"direct-io" yaml:"direct-io,omitempty"`
	readOnly   bool   `config:"ro" yaml:"ro,omitempty"`
	tmpPath    string `config:"tmp-path" yaml:"tmp-path,omitempty"`
	outputFile string `config:"o" yaml:"o,omitempty"`
}

var optsGenCfg genConfigParams

var generatedConfig = &cobra.Command{
	Use:               "gen-config",
	Short:             "Generate default config file.",
	Long:              "Generate default config file with the values pre-caculated by blobfuse2.",
	SuggestFor:        []string{"generate default config", "generate config"},
	Hidden:            true,
	Args:              cobra.ExactArgs(0),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		// Check if configTmp is not provided when component is fc
		if (!optsGenCfg.blockCache) && optsGenCfg.tmpPath == "" {
			return fmt.Errorf("temp path is required for file cache mode. Use flag --tmp-path to provide the path")
		}

		// Set the configs
		if optsGenCfg.readOnly {
			config.Set("read-only", "true")
		}

		if optsGenCfg.directIO {
			config.Set("direct-io", "true")
		}

		config.Set("tmp-path", optsGenCfg.tmpPath)

		// Create the pipeline
		pipeline := []string{"libfuse"}
		if optsGenCfg.blockCache {
			pipeline = append(pipeline, "block_cache")
		} else {
			pipeline = append(pipeline, "file_cache")
		}

		if !optsGenCfg.directIO {
			pipeline = append(pipeline, "attr_cache")
		}
		pipeline = append(pipeline, "azstorage")

		var sb strings.Builder

		if optsGenCfg.directIO {
			sb.WriteString("direct-io: true\n")
		}

		if optsGenCfg.readOnly {
			sb.WriteString("read-only: true\n\n")
		}

		sb.WriteString("# Logger configuration\n#logging:\n  #  type: syslog|silent|base\n  #  level: log_off|log_crit|log_err|log_warning|log_info|log_trace|log_debug\n")
		sb.WriteString("  #  file-path: <path where log files shall be stored. Default - '$HOME/.blobfuse2/blobfuse2.log'>\n")

		sb.WriteString("\ncomponents:\n")
		for _, component := range pipeline {
			sb.WriteString(fmt.Sprintf("  - %s\n", component))
		}

		for _, component := range pipeline {
			c := internal.GetComponent(component)
			if c == nil {
				return fmt.Errorf("generatedConfig:: error getting component [%s]", component)
			}
			sb.WriteString("\n")
			sb.WriteString(c.GenConfig())
		}

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

		var err error = nil
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

	generatedConfig.Flags().BoolVar(&optsGenCfg.blockCache, "block-cache", false, "Block-Cache shall be used as caching strategy")
	generatedConfig.Flags().BoolVar(&optsGenCfg.directIO, "direct-io", false, "Direct-io mode shall be used")
	generatedConfig.Flags().BoolVar(&optsGenCfg.readOnly, "ro", false, "Mount in read-only mode")
	generatedConfig.Flags().StringVar(&optsGenCfg.tmpPath, "tmp-path", "", "Temp cache path to be used")
	generatedConfig.Flags().StringVar(&optsGenCfg.outputFile, "o", "", "Output file location")
}
