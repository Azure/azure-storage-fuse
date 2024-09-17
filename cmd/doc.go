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
