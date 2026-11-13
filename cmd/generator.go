// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:        "generate [component name]",
	Hidden:     true,
	Short:      "Generate a new component for Blobfuse2",
	Long:       "Generate a new component for Blobfuse2",
	SuggestFor: []string{"gen", "gener"},
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		componentName := args[0]
		script := exec.Command("./cmd/componentGenerator.sh", componentName)
		script.Stdout = os.Stdout
		script.Stderr = os.Stdout

		if err := script.Run(); err != nil {
			return fmt.Errorf("failed to generate new component [%s]", err.Error())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
