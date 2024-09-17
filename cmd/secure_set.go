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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var setKeyCmd = &cobra.Command{
	Use:               "set",
	Short:             "Update encrypted config by setting new value for the given config parameter",
	Long:              "Update encrypted config by setting new value for the given config parameter",
	SuggestFor:        []string{"s", "set"},
	Example:           "blobfuse2 secure set --config-file=config.yaml --passphrase=PASSPHRASE --key=logging.log_level --value=log_debug",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		plainText, err := decryptConfigFile(false)
		if err != nil {
			return fmt.Errorf("failed to decrypt config file [%s]", err.Error())
		}

		viper.SetConfigType("yaml")
		err = viper.ReadConfig(strings.NewReader(string(plainText)))
		if err != nil {
			return fmt.Errorf("failed to load config [%s]", err.Error())
		}

		value := viper.Get(secOpts.Key)
		if value != nil {
			valType := reflect.TypeOf(value)
			if strings.HasPrefix(valType.String(), "map") ||
				strings.HasPrefix(valType.String(), "[]") {
				return errors.New("invalid option, only allowed to modify a scalar config")
			}

			fmt.Println("Current value : ", secOpts.Key, "=", value)
			fmt.Println("Setting value : ", secOpts.Key, "=", secOpts.Value)
		} else {
			fmt.Println("Key not found in config file, adding now")
		}

		viper.Set(secOpts.Key, secOpts.Value)

		allConf := viper.AllSettings()
		confStream, err := yaml.Marshal(allConf)
		if err != nil {
			return fmt.Errorf("failed to marshal config [%s]", err.Error())
		}

		cipherText, err := common.EncryptData(confStream, []byte(secOpts.PassPhrase))
		if err != nil {
			return fmt.Errorf("failed to encrypt config [%s]", err.Error())
		}

		if err = saveToFile(secOpts.ConfigFile, cipherText, false); err != nil {
			return fmt.Errorf("failed save config file [%s]", err.Error())
		}

		return nil
	},
}
