// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type configGenOptions struct {
	configFilePath   string
	outputConfigPath string
	containerName    string
	tempDirPath      string
}

var opts configGenOptions
var templatesDir = "testdata/config/"

var generateTestConfig = &cobra.Command{
	Use:        "gen-test-config",
	Short:      "Generate config file for testing given an output path.",
	Long:       "Generate config file for testing given an output path.",
	SuggestFor: []string{"conv test config", "convert test config"},
	Hidden:     true,
	Args:       cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		var templateConfig []byte
		var err error

		if strings.Contains(opts.configFilePath, templatesDir) {
			templateConfig, err = os.ReadFile(opts.configFilePath)
		} else {
			templateConfig, err = os.ReadFile(templatesDir + opts.configFilePath)
		}

		if err != nil {
			return fmt.Errorf("failed to read file [%s]", err.Error())
		}

		// match all parameters in { }
		re := regexp.MustCompile("{.*?}")
		templateParams := re.FindAll(templateConfig, -1)
		newConfig := string(templateConfig)

		for _, param := range templateParams {
			// { 0 } -> container name
			// { 1 } -> temp path
			if string(param) == "{ 0 }" {
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, opts.containerName)
			} else if string(param) == "{ 1 }" {
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, opts.tempDirPath)
			} else {
				envVar := os.Getenv(string(param)[2 : len(string(param))-2])
				re := regexp.MustCompile(string(param))
				newConfig = re.ReplaceAllString(newConfig, envVar)
			}
		}

		// write the config with the params to the output file
		err = os.WriteFile(opts.outputConfigPath, []byte(newConfig), 0700)
		if err != nil {
			return fmt.Errorf("failed to write file [%s]", err.Error())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateTestConfig)
	generateTestConfig.Flags().StringVar(&opts.configFilePath, "config-file", "", "Input config file.")
	generateTestConfig.Flags().StringVar(&opts.outputConfigPath, "output-file", "", "Output config file path.")
	generateTestConfig.Flags().StringVar(&opts.containerName, "container-name", "", "Container name.")
	generateTestConfig.Flags().StringVar(&opts.tempDirPath, "temp-path", "", "Temporary file path.")

}
