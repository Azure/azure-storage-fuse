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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type containerListingOptions struct {
	AllowList        []string `config:"container-allowlist"`
	DenyList         []string `config:"container-denylist"`
	blobfuse2BinPath string
}

var mountAllOpts containerListingOptions

var mountAllCmd = &cobra.Command{
	Use:               "all [path] <flags>",
	Short:             "Mounts all azure blob container for a given account as a filesystem",
	Long:              "Mounts all azure blob container for a given account as a filesystem",
	SuggestFor:        []string{"mnta", "mout"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	Run: func(cmd *cobra.Command, args []string) {
		VersionCheck()

		mountAllOpts.blobfuse2BinPath = os.Args[0]
		options.MountPath = args[0]
		processCommand()
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
}

func processCommand() {
	parseConfig()

	err := config.Unmarshal(&options)
	if err != nil {
		fmt.Printf("MountAll : Init error config unmarshall [%s]", err)
		os.Exit(1)
	}

	err = options.validate(true)
	if err != nil {
		fmt.Printf("MountAll : error invalid options [%v]", err)
		os.Exit(1)
	}

	var logLevel common.LogLevel
	err = logLevel.Parse(options.Logging.LogLevel)
	if err != nil {
		fmt.Println("error: invalid log level")
	}

	err = log.SetDefaultLogger(options.Logging.Type, common.LogConfig{
		FilePath:    options.Logging.LogFilePath,
		MaxFileSize: options.Logging.MaxLogFileSize,
		FileCount:   options.Logging.LogFileCount,
		Level:       logLevel,
		TimeTracker: options.Logging.TimeTracker,
	})

	if err != nil {
		fmt.Printf("Mount: error initializing logger [%v]", err)
		os.Exit(1)
	}

	config.Set("mount-path", options.MountPath)

	// Add this flag in config map so that other components be aware that we are running
	// in 'mount all' command mode. This is used by azstorage component for certain cofig checks
	config.SetBool("mount-all-containers", true)

	log.Crit("Starting Blobfuse2 Mount All: %s", common.Blobfuse2Version)
	log.Crit("Logging level set to : %s", logLevel.String())

	// Get allowlist/denylist containers from the config
	err = config.UnmarshalKey("mountall", &mountAllOpts)
	if err != nil {
		fmt.Printf("MountAll : Failed to get container listing options (%s)\n", err.Error())
	}

	// Validate config is to be secured on write or not
	if options.PassPhrase == "" {
		options.PassPhrase = os.Getenv(SecureConfigEnvName)
	}

	if options.SecureConfig && options.PassPhrase == "" {
		fmt.Println("Key not provided for decrypt config file")
		os.Exit(1)
	}

	containerList := getContainerList()
	if len(containerList) > 0 {
		containerList = filterAllowedContainerList(containerList)
		mountAllContainers(containerList, options.ConfigFile, options.MountPath)
	} else {
		fmt.Println("MountAll : There is nothing to mount from this account")
		os.Exit(1)
	}
}

// getContainerList : Get list of containers from storage account
func getContainerList() []string {
	var containerList []string

	// Create AzStorage component to get container list
	azComponent := &azstorage.AzStorage{}
	if azComponent == nil {
		fmt.Printf("MountAll : Failed to create AzureStorage object")
		os.Exit(1)
	}
	azComponent.SetName("azstorage")
	azComponent.SetNextComponent(nil)

	// Configure AzStorage component
	err := azComponent.Configure()
	if err != nil {
		fmt.Printf("MountAll : Failed to configure AzureStorage object (%s)", err.Error())
		os.Exit(1)
	}

	//  Start AzStorage the component so that credentials are verified
	err = azComponent.Start(context.Background())
	if err != nil {
		fmt.Printf("MountAll : Failed to initialize AzureStorage object (%s)", err.Error())
		os.Exit(1)
	}

	// Get the list of containers from the component
	containerList, err = azComponent.ListContainers()
	if err != nil {
		fmt.Printf("MountAll : Failed to get container list from storage (%s)", err.Error())
		os.Exit(1)
	}

	// Stop the azStorage component as its no more needed now
	azComponent.Stop()
	return containerList
}

// FiterAllowedContainer : Filter which containers are allowed to be mounted
func filterAllowedContainerList(containers []string) []string {
	allowListing := false
	if len(mountAllOpts.AllowList) > 0 {
		allowListing = true
	}

	// Convert the entire container list into a map
	var filterContainer = make(map[string]bool)
	for _, container := range containers {
		filterContainer[container] = !allowListing
	}

	// Now based on allow or deny list mark the containers
	if allowListing {
		// Only containers in this list shall be allowed
		for _, container := range mountAllOpts.AllowList {
			_, found := filterContainer[container]
			if found {
				filterContainer[container] = true
			}
		}
	} else {
		// Containers in this list shall not be allowed
		for _, container := range mountAllOpts.DenyList {
			_, found := filterContainer[container]
			if found {
				filterContainer[container] = false
			}
		}
	}

	// Consolidate only containers that are allowed now
	var filterList []string
	for container, allowed := range filterContainer {
		if allowed {
			filterList = append(filterList, container)
		}
	}

	return filterList
}

// mountAllContainers : Iterate allowed container list and create config file and mount path for them
func mountAllContainers(containerList []string, configFile string, mountPath string) {
	// Now iterate filtered container list and prepare mount path, temp path, and config file for them
	fileCachePath := viper.GetString("file_cache.path")

	// Generate slice containing all the argument which we need to pass to each mount command
	cliParams := buildCliParamForMount()

	// Change the config file name per container
	ext := filepath.Ext(configFile)
	if ext == SecureConfigExtension {
		ext = ".yaml"
	}

	//configFileName := configFile[:(len(configFile) - len(ext))]
	configFileName := filepath.Join(os.ExpandEnv(common.DefaultWorkDir), "config")

	for _, container := range containerList {
		contMountPath := filepath.Join(mountPath, container)
		contConfigFile := configFileName + "_" + container + ext

		if options.SecureConfig {
			contConfigFile = contConfigFile + SecureConfigExtension
		}

		if _, err := os.Stat(contMountPath); os.IsNotExist(err) {
			os.MkdirAll(contMountPath, 0777)
		}

		// NOTE : Add all the configs that need replacement based on container here

		// If next instance is not mounted in background then mountall will hang up hence always mount in background
		viper.Set("foreground", false)
		viper.Set("azstorage.container", container)
		viper.Set("file_cache.path", filepath.Join(fileCachePath, container))

		// Create config file with container specific configs
		writeConfigFile(contConfigFile)

		// Now that we have mount path and config file for this container fire a mount command for this one
		cliParams[1] = contMountPath
		cliParams[2] = "--config-file=" + contConfigFile

		fmt.Println("Mounting container :", container, "to path :", contMountPath)
		cmd := exec.Command(mountAllOpts.blobfuse2BinPath, cliParams...)

		cliOut, err := cmd.Output()
		fmt.Println(string(cliOut))
		if err != nil {
			fmt.Printf("failed to mount container %s : %s\n", container, err.Error())
		}
	}
}

func writeConfigFile(contConfigFile string) {
	if options.SecureConfig {
		allConf := viper.AllSettings()
		confStream, err := yaml.Marshal(allConf)
		if err != nil {
			fmt.Println("Failed to marshall yaml content")
			os.Exit(1)
		}

		cipherText, err := common.EncryptData(confStream, []byte(options.PassPhrase))
		if err != nil {
			fmt.Println("Failed to marshall yaml content ", err.Error())
			os.Exit(1)
		}

		err = ioutil.WriteFile(contConfigFile, cipherText, 0777)
		if err != nil {
			fmt.Println("Failed to write encrypted file : ", err.Error())
			os.Exit(1)
		}
	} else {
		// Write modified config as per container to a new config file
		viper.WriteConfigAs(contConfigFile)
	}
}

func buildCliParamForMount() []string {
	var cliParam []string

	cliParam = append(cliParam, "mount")
	cliParam = append(cliParam, "<mount-path>")
	cliParam = append(cliParam, "--config-file=<conf_file>")
	for _, opt := range os.Args[4:] {
		if !ignoreCliParam(opt) {
			cliParam = append(cliParam, opt)
		}
	}
	cliParam = append(cliParam, "--disable-version-check=true")

	return cliParam
}

func ignoreCliParam(opt string) bool {
	if strings.HasPrefix(opt, "--config-file") {
		return true
	}

	return false
}
