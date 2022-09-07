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
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getKeyCmd = &cobra.Command{
	Use:               "get",
	Short:             "Get value of requested config parameter from your encrypted config file",
	Long:              "Get value of requested config parameter from your encrypted config file",
	SuggestFor:        []string{"g", "get"},
	Example:           "blobfuse2 secure get --config-file=config.yaml --passphrase=PASSPHRASE --key=logging.log_level",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateOptions()
		if err != nil {
			fmt.Printf("secure get : failed to validate options (%s)", err.Error())
			return fmt.Errorf("secure get : failed to validate options (%s)", err.Error())
		}

		plainText, err := decryptConfigFile(false)
		if err != nil {
			fmt.Printf("secure get : failed to decrypt config file (%s)", err.Error())
			return fmt.Errorf("secure get : failed to decrypt config file (%s)", err.Error())
		}

		viper.SetConfigType("yaml")
		err = viper.ReadConfig(strings.NewReader(string(plainText)))
		if err != nil {
			fmt.Printf("secure get : failed to load config (%s)", err.Error())
			return fmt.Errorf("secure get : failed to load config (%s)", err.Error())
		}

		value := viper.Get(secOpts.Key)
		if value == nil {
			fmt.Printf("secure get : key not found in config (%s)", err.Error())
			return fmt.Errorf("secure get : key not found in config (%s)", err.Error())
		}

		valType := reflect.TypeOf(value)
		if strings.HasPrefix(valType.String(), "map") {
			fmt.Println("secure get : Fetching group level configuration")
		} else if strings.HasPrefix(valType.String(), "[]") {
			fmt.Println("secure get : Fetching options level configuration")
		} else {
			fmt.Println("secure get : Fetching scalar configuration")
		}

		fmt.Println("secure get : ", secOpts.Key, "=", value)
		return nil
	},
}
