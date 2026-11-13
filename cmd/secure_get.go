// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
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
		if value == nil {
			return fmt.Errorf("key not found in config")
		}

		valType := reflect.TypeOf(value)
		if strings.HasPrefix(valType.String(), "map") {
			fmt.Println("Fetching group level configuration")
		} else if strings.HasPrefix(valType.String(), "[]") {
			fmt.Println("Fetching options level configuration")
		} else {
			fmt.Println("Fetching scalar configuration")
		}

		fmt.Println(secOpts.Key, "=", value)
		return nil
	},
}
