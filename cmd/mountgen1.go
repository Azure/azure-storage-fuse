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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/libfuse"

	"github.com/spf13/cobra"
)

var azStorageOpt azstorage.AzStorageOptions
var libFuseOpt libfuse.LibfuseOptions
var fileCacheOpt file_cache.FileCacheOptions
var requiredFreeSpace int
var configFile string
var generateJsonOnly bool
var gen1ConfigFilePath string

func resetGenOneOptions() {
	azStorageOpt = azstorage.AzStorageOptions{}
	libFuseOpt = libfuse.LibfuseOptions{}
	fileCacheOpt = file_cache.FileCacheOptions{}
}

var gen1Cmd = &cobra.Command{
	Use:               "mountgen1",
	Short:             "Mounts Azure Storage ADLS Gen 1 account using SPN auth",
	Long:              "Mounts Azure Storage ADLS Gen 1 account using SPN auth",
	SuggestFor:        []string{"mntgen1", "gen1 mount"},
	Args:              cobra.ExactArgs(1),
	Hidden:            true,
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		resetGenOneOptions()
		options.MountPath = args[0]
		options.ConfigFile = configFile
		err := parseConfig()
		if err != nil {
			return err
		}

		err = config.Unmarshal(&options)
		if err != nil {
			return fmt.Errorf("failed to unmarshal config [%s]", err.Error())
		}

		if !config.IsSet("logging.file-path") {
			options.Logging.LogFilePath = common.DefaultLogFilePath
		}

		if !config.IsSet("logging.level") {
			options.Logging.LogLevel = "LOG_WARNING"
		}

		err = options.validate(false)
		if err != nil {
			return err
		}

		err = config.UnmarshalKey("azstorage", &azStorageOpt)
		if err != nil {
			return fmt.Errorf("invalid azstorage config [%s]", err.Error())
		}

		// not checking ClientSecret since adlsgen1fuse will be reading secret from env variable (ADL_CLIENT_SECRET)
		if azStorageOpt.ClientID == "" || azStorageOpt.TenantID == "" || azStorageOpt.AccountName == "" {
			log.Err("mountgen1 : clientId, tenantId or accountName can't be empty")
			return fmt.Errorf("clientId, tenantId or accountName can't be empty")
		}

		// changing authMode to spn since clientId and tenantId are not empty
		if azStorageOpt.AuthMode == "" {
			azStorageOpt.AuthMode = "spn"
		}

		err = config.UnmarshalKey("libfuse", &libFuseOpt)
		if err != nil {
			return fmt.Errorf("invalid libfuse config [%s]", err.Error())
		}

		err = config.UnmarshalKey("file_cache", &fileCacheOpt)
		if err != nil {
			return fmt.Errorf("invalid file_cache config [%s]", err.Error())
		}

		var logLevel common.LogLevel
		err = logLevel.Parse(options.Logging.LogLevel)
		if err != nil {
			return fmt.Errorf("invalid log level [%s]", err.Error())
		}

		err = generateAdlsGenOneJson()
		if err != nil {
			return err
		}

		if !generateJsonOnly {
			err = runAdlsGenOneBinary()
			if err != nil {
				return err
			}
		}

		return nil
	},
}

// code to generate json file for rustfuse
func generateAdlsGenOneJson() error {
	rustFuseMap := make(map[string]interface{})
	if strings.ToLower(azStorageOpt.AuthMode) == "spn" {
		// adlsgen1fuse will be reading secret from env variable (ADL_CLIENT_SECRET) hence no reason to include this.
		// rustFuseMap["clientsecret"] = azStorageOpt.ClientSecret

		rustFuseMap["clientid"] = azStorageOpt.ClientID
		rustFuseMap["tenantid"] = azStorageOpt.TenantID
		if azStorageOpt.ActiveDirectoryEndpoint != "" {
			rustFuseMap["authorityurl"] = azStorageOpt.ActiveDirectoryEndpoint
		} else {
			rustFuseMap["authorityurl"] = "https://login.microsoftonline.com"
		}

		rustFuseMap["credentialtype"] = "servicePrincipal"
	} else {
		return fmt.Errorf("for Gen1 account only SPN auth is supported")
	}

	rustFuseMap["resourceurl"] = "https://datalake.azure.net/"

	if libFuseOpt.AttributeExpiration != 0 {
		rustFuseMap["fuseattrtimeout"] = libFuseOpt.AttributeExpiration
	}

	if libFuseOpt.EntryExpiration != 0 {
		rustFuseMap["fuseentrytimeout"] = libFuseOpt.EntryExpiration
	}

	var allowOther bool
	err := config.UnmarshalKey("allow-other", &allowOther)
	if err != nil {
		log.Err("mountgen1 : generateAdlsGenOneJson:allow-other config error (invalid config attributes) [%s]", err.Error())
		return fmt.Errorf("unable to parse allow-other config [%s]", err.Error())
	}

	rustFuseMap["fuseallowother"] = allowOther

	if options.Logging.LogLevel != "" {
		rustFuseMap["loglevel"] = strings.ToUpper(options.Logging.LogLevel)
	}

	if azStorageOpt.MaxRetries != 0 {
		rustFuseMap["retrycount"] = azStorageOpt.MaxRetries
	}

	if fileCacheOpt.MaxSizeMB != 1000 {
		rustFuseMap["maxcachesizeinmb"] = fileCacheOpt.MaxSizeMB
	}

	if requiredFreeSpace != 0 {
		rustFuseMap["requiredfreespaceinmb"] = requiredFreeSpace
	}

	if fileCacheOpt.TmpPath != "" {
		rustFuseMap["cachedir"] = common.ExpandPath(fileCacheOpt.TmpPath)
	}

	if azStorageOpt.Container != "" {
		rustFuseMap["resourceid"] = "adl://" + azStorageOpt.AccountName + ".azuredatalakestore.net/" + azStorageOpt.Container + "/"
	} else {
		rustFuseMap["resourceid"] = "adl://" + azStorageOpt.AccountName + ".azuredatalakestore.net/"
	}

	rustFuseMap["mountdir"] = options.MountPath

	jsonData, _ := json.MarshalIndent(rustFuseMap, "", "\t")

	err = os.WriteFile(gen1ConfigFilePath, jsonData, 0777)
	if err != nil {
		log.Err("mountgen1 : generateAdlsGenOneJson:failed to write adlsgen1fuse.json [%s]", err.Error())
		return fmt.Errorf("failed to write adlsgen1fuse.json [%s]", err.Error())
	}

	return nil
}

// run the adlsgen1fuse binary
func runAdlsGenOneBinary() error {
	adlsgen1fuseCmd := exec.Command("adlsgen1fuse", gen1ConfigFilePath)
	var errb bytes.Buffer
	adlsgen1fuseCmd.Stderr = &errb
	_, err := adlsgen1fuseCmd.Output()

	if err != nil {
		log.Err("mountgen1 : runAdlsGenOneBinary: unable to run adlsgen1fuse binary (%s : %s)", err.Error(), errb.String())
		return fmt.Errorf("unable to run adlsgen1fuse binary (%s : %s)", err.Error(), errb.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(gen1Cmd)

	gen1Cmd.Flags().StringVar(&configFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")

	gen1Cmd.Flags().IntVar(&requiredFreeSpace, "required-free-space-mb", 0, "Required free space in MB")

	gen1Cmd.Flags().BoolVar(&generateJsonOnly, "generate-json-only", false, "Don't mount, only generate the JSON file needed for gen1 mount")

	gen1Cmd.Flags().StringVar(&gen1ConfigFilePath, "output-file", "/tmp/adlsgen1fuse.json", "Output JSON file needed for gen1 mount")
}
