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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getKeyCmd = &cobra.Command{
	Use:        "get",
	Short:      "Get value of requested config parameter from your encrypted config file",
	Long:       "Get value of requested config parameter from your encrypted config file",
	SuggestFor: []string{"g", "get"},
	Example:    "blobfuse2 secure get --config-file=config.yaml --passphrase=PASSPHRASE --key=logging.log_level",
	RunE: func(cmd *cobra.Command, args []string) error {
		validateOptions()

		plainText, err := decryptConfigFile(false)
		if err != nil {
			return err
		}

		viper.SetConfigType("yaml")
		err = viper.ReadConfig(strings.NewReader(string(plainText)))
		if err != nil {
			return errors.New("failed to load config")
		}

		value := viper.Get(secOpts.Key)
		if value == nil {
			return errors.New("search key not found in config")
		}

		valType := reflect.TypeOf(value)
		if strings.HasPrefix(valType.String(), "map") {
			fmt.Println("fetching group level configuration")
		} else if strings.HasPrefix(valType.String(), "[]") {
			fmt.Println("fetching options level configuration")
		} else {
			fmt.Println("fetching scalar configuration")
		}

		fmt.Println(secOpts.Key, "=", value)
		return nil
	},
}
