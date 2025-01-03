/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/spf13/cobra"
)

type secureOptions struct {
	Operation  string
	ConfigFile string
	PassPhrase string
	OutputFile string
	Key        string
	Value      string
}

const SecureConfigEnvName string = "BLOBFUSE2_SECURE_CONFIG_PASSPHRASE"
const SecureConfigExtension string = ".azsec"

var secOpts secureOptions

// Section defining all the command that we have in secure feature
var secureCmd = &cobra.Command{
	Use:               "secure",
	Short:             "Encrypt / Decrypt your config file",
	Long:              "Encrypt / Decrypt your config file",
	SuggestFor:        []string{"sec", "secre"},
	Example:           "blobfuse2 secure encrypt --config-file=config.yaml --passphrase=PASSPHRASE",
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}
		return nil
	},
}

var encryptCmd = &cobra.Command{
	Use:               "encrypt",
	Short:             "Encrypt your config file",
	Long:              "Encrypt your config file",
	SuggestFor:        []string{"en", "enc"},
	Example:           "blobfuse2 secure encrypt --config-file=config.yaml --passphrase=PASSPHRASE",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		_, err = encryptConfigFile(true)
		if err != nil {
			return fmt.Errorf("failed to encrypt config file [%s]", err.Error())
		}

		return nil
	},
}

var decryptCmd = &cobra.Command{
	Use:               "decrypt",
	Short:             "Decrypt your config file",
	Long:              "Decrypt your config file",
	SuggestFor:        []string{"de", "dec"},
	Example:           "blobfuse2 secure decrypt --config-file=config.yaml --passphrase=PASSPHRASE",
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateOptions()
		if err != nil {
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		_, err = decryptConfigFile(true)
		if err != nil {
			return fmt.Errorf("failed to decrypt config file [%s]", err.Error())
		}

		return nil
	},
}

//--------------- command section ends

func validateOptions() error {
	if secOpts.PassPhrase == "" {
		secOpts.PassPhrase = os.Getenv(SecureConfigEnvName)
	}

	if secOpts.ConfigFile == "" {
		return errors.New("config file not provided, check usage")
	}

	if _, err := os.Stat(secOpts.ConfigFile); os.IsNotExist(err) {
		return errors.New("config file does not exist")
	}

	if secOpts.PassPhrase == "" {
		return errors.New("provide the passphrase as a cli parameter or configure the BLOBFUSE2_SECURE_CONFIG_PASSPHRASE environment variable")
	}

	return nil
}

// encryptConfigFile: Encrypt config file using the passphrase provided by user
func encryptConfigFile(saveConfig bool) ([]byte, error) {
	plaintext, err := os.ReadFile(secOpts.ConfigFile)
	if err != nil {
		return nil, err
	}

	cipherText, err := common.EncryptData(plaintext, []byte(secOpts.PassPhrase))
	if err != nil {
		return nil, err
	}

	if saveConfig {
		outputFileName := ""
		if secOpts.OutputFile == "" {
			outputFileName = filepath.Join(common.ExpandPath(common.DefaultWorkDir), filepath.Base(secOpts.ConfigFile))
			outputFileName += SecureConfigExtension
		} else {
			outputFileName = secOpts.OutputFile
		}

		return cipherText, saveToFile(outputFileName, cipherText, true)
	}

	return cipherText, nil
}

// decryptConfigFile: Decrypt config file using the passphrase provided by user
func decryptConfigFile(saveConfig bool) ([]byte, error) {
	cipherText, err := os.ReadFile(secOpts.ConfigFile)
	if err != nil {
		return nil, err
	}

	plainText, err := common.DecryptData(cipherText, []byte(secOpts.PassPhrase))
	if err != nil {
		return nil, err
	}

	if saveConfig {
		outputFileName := ""
		if secOpts.OutputFile == "" {
			outputFileName = filepath.Join(os.ExpandEnv(common.DefaultWorkDir), filepath.Base(secOpts.ConfigFile))
			extension := filepath.Ext(outputFileName)
			outputFileName = outputFileName[0 : len(outputFileName)-len(extension)]
		} else {
			outputFileName = secOpts.OutputFile
		}

		return plainText, saveToFile(outputFileName, plainText, false)
	}

	return plainText, nil
}

// saveToFile: Save the newly generated config file and delete the source if requested
func saveToFile(configFileName string, data []byte, deleteSource bool) error {
	err := os.WriteFile(configFileName, data, 0777)
	if err != nil {
		return err
	}

	if deleteSource {
		// Delete the original file as we now have a encrypted config file
		err = os.Remove(secOpts.ConfigFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(secureCmd)
	secureCmd.AddCommand(encryptCmd)
	secureCmd.AddCommand(decryptCmd)
	secureCmd.AddCommand(getKeyCmd)
	secureCmd.AddCommand(setKeyCmd)

	getKeyCmd.Flags().StringVar(&secOpts.Key, "key", "",
		"Config key to be searched in encrypted config file")

	setKeyCmd.Flags().StringVar(&secOpts.Key, "key", "",
		"Config key to be updated in encrypted config file")
	setKeyCmd.Flags().StringVar(&secOpts.Value, "value", "",
		"New value for the given config key to be set in ecrypted config file")

	// Flags that needs to be accessible at all subcommand level shall be defined in persistentflags only
	secureCmd.PersistentFlags().StringVar(&secOpts.ConfigFile, "config-file", "",
		"Configuration file to be encrypted / decrypted")

	secureCmd.PersistentFlags().StringVar(&secOpts.PassPhrase, "passphrase", "",
		"Key to be used for encryption / decryption. Can also be specified by env-variable BLOBFUSE2_SECURE_CONFIG_PASSPHRASE.\nKey length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")

	secureCmd.PersistentFlags().StringVar(&secOpts.OutputFile, "output-file", "",
		"Path and name for the output file")
}
