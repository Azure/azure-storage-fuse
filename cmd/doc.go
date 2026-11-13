// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docCmdInput = struct {
	outputLocation string
}{}

// docCmd represents the doc command
var docCmd = &cobra.Command{
	Use:    "doc",
	Hidden: true,
	Short:  "Generates documentation for the tool in Markdown format",
	Long:   "Generates documentation for the tool in Markdown format, and stores them in the designated location",
	RunE: func(cmd *cobra.Command, args []string) error {
		// verify the output location
		f, err := os.Stat(docCmdInput.outputLocation)
		if err != nil && os.IsNotExist(err) {
			// create the output location if it does not exist yet
			if err = os.MkdirAll(docCmdInput.outputLocation, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create output location [%s]", err.Error())
			}
		} else if err != nil {
			return fmt.Errorf("cannot access output location [%s]", err.Error())
		} else if !f.IsDir() {
			return fmt.Errorf("output location is invalid as it is pointing to a file")
		}

		// dump the entire command tree's doc into the folder
		// it will include this command too, which is intended
		err = doc.GenMarkdownTree(rootCmd, docCmdInput.outputLocation)
		if err != nil {
			return fmt.Errorf("cannot generate command tree [%s]. Please contact the dev team", err.Error())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(docCmd)
	docCmd.PersistentFlags().StringVar(&docCmdInput.outputLocation, "output-location", "./doc",
		"where to put the generated markdown files")
}
