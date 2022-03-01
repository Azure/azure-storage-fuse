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
	"blobfuse2/common"
	"blobfuse2/common/config"
	"blobfuse2/common/log"
	"blobfuse2/component/azstorage"
	"blobfuse2/component/file_cache"
	"blobfuse2/component/libfuse"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// type gen1Options struct {
// 	MountPath  string
// 	ConfigFile string

// 	Components                  []string `config:"components"`
// 	LogOptions                  `config:"logging"`
// 	azstorage.AzStorageOptions  `config:"azstorage"`
// 	libfuse.LibfuseOptions      `config:"libfuse"`
// 	file_cache.FileCacheOptions `config:"file-cache"`
// }

// var gen1Opt gen1Options

var AzStorageOpt azstorage.AzStorageOptions
var LibFuseOpt libfuse.LibfuseOptions
var FileCacheOpt file_cache.FileCacheOptions
var requiredFreeSpace int
var configFile string

const gen1ConfigFilePath string = "/tmp/adlsgen1fuse.json"

var gen1Cmd = &cobra.Command{
	Use:        "mountgen1",
	Short:      "Mounts Azure Storage ADLS Gen 1 account using SPN auth",
	Long:       "Mounts Azure Storage ADLS Gen 1 account using SPN auth",
	SuggestFor: []string{"mntgen1", "gen1 mount"},
	Args:       cobra.ExactArgs(1),
	Hidden:     true,
	Run: func(cmd *cobra.Command, args []string) {
		options.MountPath = args[0]
		options.ConfigFile = configFile
		parseConfig()

		err := config.Unmarshal(&options)
		if err != nil {
			fmt.Printf("Init error config unmarshall [%s]", err)
			os.Exit(1)
		}

		err = options.validate(false)
		if err != nil {
			fmt.Printf("mountgen1: error invalid options [%v]", err)
			os.Exit(1)
		}

		err = config.UnmarshalKey("azstorage", &AzStorageOpt)
		if err != nil {
			log.Err("mountgen1: AzStorage config error [invalid config attributes]")
			os.Exit(1)
		}

		// not checking ClientSecret since adlsgen1fuse will be reading secret from env variable (ADL_CLIENT_SECRET)
		if AzStorageOpt.AuthMode == "" && AzStorageOpt.ClientID != "" && AzStorageOpt.TenantID != "" {
			AzStorageOpt.AuthMode = "spn"
		}

		err = config.UnmarshalKey("libfuse", &LibFuseOpt)
		if err != nil {
			log.Err("mountgen1: Libfuse config error [invalid config attributes]")
			os.Exit(1)
		}

		err = config.UnmarshalKey("file_cache", &FileCacheOpt)
		if err != nil {
			log.Err("mountgen1: FileCache config error [invalid config attributes]")
			os.Exit(1)
		}

		var logLevel common.LogLevel
		err = logLevel.Parse(options.Logging.LogLevel)
		if err != nil {
			fmt.Println("error: invalid log level")
		}

		err = mountRustFuse()
		if err != nil {
			log.Err("Unable to mount Gen1: " + err.Error())
			os.Exit(1)
		}

		// run the adlsgen1fuse binary
		adlsgen1fuseCmd := exec.Command("adlsgen1fuse", gen1ConfigFilePath)
		cliOut, err := adlsgen1fuseCmd.Output()
		fmt.Println(string(cliOut))
		if err != nil {
			fmt.Printf("Unable to run adlsgen1fuse binary: %s\n", err.Error())
		}
	},
}

// code to generate json file for rustfuse
func mountRustFuse() error {
	rustFuseMap := make(map[string]interface{})
	if strings.ToLower(AzStorageOpt.AuthMode) == "spn" {
		// adlsgen1fuse will be reading secret from env variable (ADL_CLIENT_SECRET) hence no reason to include this.
		// rustFuseMap["clientsecret"] = AzStorageOpt.ClientSecret

		rustFuseMap["clientid"] = AzStorageOpt.ClientID
		rustFuseMap["tenantid"] = AzStorageOpt.TenantID
		if AzStorageOpt.ActiveDirectoryEndpoint != "" {
			rustFuseMap["authorityurl"] = AzStorageOpt.ActiveDirectoryEndpoint
		} else {
			rustFuseMap["authorityurl"] = "https://login.microsoftonline.com"
		}

		rustFuseMap["credentialtype"] = "servicePrincipal"
	} else {
		log.Err("mountgen1::MountRustFuse : For Gen1 account only SPN auth is supported")
		return fmt.Errorf("for Gen1 account only SPN auth is supported")
	}

	rustFuseMap["resourceurl"] = "https://datalake.azure.net/"

	if LibFuseOpt.AttributeExpiration != 0 {
		rustFuseMap["fuseattrtimeout"] = LibFuseOpt.AttributeExpiration
	}

	if LibFuseOpt.EntryExpiration != 0 {
		rustFuseMap["fuseentrytimeout"] = LibFuseOpt.EntryExpiration
	}

	var allowOther bool
	err := config.UnmarshalKey("allow-other", &allowOther)
	if err != nil {
		log.Err("mountgen1::MountRustFuse : config error [unable to obtain allow-other]")
		return fmt.Errorf("config error in [%s]", err.Error())
	}
	rustFuseMap["fuseallowother"] = allowOther

	if options.Logging.LogLevel != "" {
		rustFuseMap["loglevel"] = strings.ToUpper(options.Logging.LogLevel)
	}

	if AzStorageOpt.MaxRetries != 0 {
		rustFuseMap["retrycount"] = AzStorageOpt.MaxRetries
	}

	if FileCacheOpt.MaxSizeMB != 1000 {
		rustFuseMap["maxcachesizeinmb"] = FileCacheOpt.MaxSizeMB
	}

	if requiredFreeSpace != 0 {
		rustFuseMap["requiredfreespaceinmb"] = requiredFreeSpace
	}

	if FileCacheOpt.TmpPath != "" {
		rustFuseMap["cachedir"] = FileCacheOpt.TmpPath
	}

	if AzStorageOpt.Container != "" {
		rustFuseMap["resourceid"] = "adl://" + AzStorageOpt.AccountName + ".azuredatalakestore.net/" + AzStorageOpt.Container + "/"
	} else {
		rustFuseMap["resourceid"] = "adl://" + AzStorageOpt.AccountName + ".azuredatalakestore.net/"
	}

	rustFuseMap["mountdir"] = options.MountPath

	jsonData, _ := json.MarshalIndent(rustFuseMap, "", "\t")

	err = os.WriteFile(gen1ConfigFilePath, jsonData, 0777)
	if err != nil {
		log.Err("mountgen1::MountRustFuse : Unable to write to adlsgen1fuse.json")
		return fmt.Errorf("unable to write to adlsgen1fuse.json: [%s]", err.Error())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(gen1Cmd)

	gen1Cmd.Flags().StringVar(&configFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")

	gen1Cmd.Flags().IntVar(&requiredFreeSpace, "required-free-space-mb", 0, "Required free space in MB")

}
