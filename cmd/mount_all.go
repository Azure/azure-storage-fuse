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
	RunE: func(cmd *cobra.Command, args []string) error {
		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				return err
			}
		}

		mountAllOpts.blobfuse2BinPath = os.Args[0]
		options.MountPath = args[0]
		return processCommand()
	},

	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
}

func processCommand() error {
	err := parseConfig()
	if err != nil {
		fmt.Printf("mount all : failed to parse config [%s]", err.Error())
		return fmt.Errorf("mount all : failed to parse config [%s]", err.Error())
	}

	err = config.Unmarshal(&options)
	if err != nil {
		fmt.Printf("mount all : failed to unmarshal config [%s]", err.Error())
		return fmt.Errorf("mount all : failed to unmarshal config [%s]", err.Error())
	}

	err = options.validate(true)
	if err != nil {
		fmt.Printf("mount all : invalid options [%s]", err.Error())
		return fmt.Errorf("mount all : invalid options [%s]", err.Error())
	}

	var logLevel common.LogLevel
	err = logLevel.Parse(options.Logging.LogLevel)
	if err != nil {
		fmt.Printf("mount all : invalid log level [%s]", err.Error())
		return fmt.Errorf("mount all : invalid log level [%s]", err.Error())
	}

	err = log.SetDefaultLogger(options.Logging.Type, common.LogConfig{
		FilePath:    options.Logging.LogFilePath,
		MaxFileSize: options.Logging.MaxLogFileSize,
		FileCount:   options.Logging.LogFileCount,
		Level:       logLevel,
		TimeTracker: options.Logging.TimeTracker,
	})

	if err != nil {
		fmt.Printf("mount all : failed to initialize logger [%s]", err.Error())
		return fmt.Errorf("mount all : failed to initialize logger [%s]", err.Error())
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
		log.Warn("mount all: mountall config error (invalid config attributes) [%s]\n", err.Error())
	}

	// Validate config is to be secured on write or not
	if options.PassPhrase == "" {
		options.PassPhrase = os.Getenv(SecureConfigEnvName)
	}

	if options.SecureConfig && options.PassPhrase == "" {
		fmt.Printf("mount all : key not provided to decrypt config file")
		return fmt.Errorf("mount all : key not provided to decrypt config file")
	}

	containerList, err := getContainerList()
	if err != nil {
		fmt.Printf("mount all : failed to get container list [%s]", err.Error())
		return fmt.Errorf("mount all : failed to get container list [%s]", err.Error())
	}

	if len(containerList) > 0 {
		containerList = filterAllowedContainerList(containerList)
		err = mountAllContainers(containerList, options.ConfigFile, options.MountPath)
		if err != nil {
			fmt.Printf("mount all : failed to mount all containers [%s]", err.Error())
			return fmt.Errorf("mount all : failed to mount all containers [%s]", err.Error())
		}
	} else {
		fmt.Println("No containers to mount from this account")
	}
	return nil
}

// getContainerList : Get list of containers from storage account
func getContainerList() ([]string, error) {
	var containerList []string

	// Create AzStorage component to get container list
	azComponent := &azstorage.AzStorage{}
	azComponent.SetName("azstorage")
	azComponent.SetNextComponent(nil)

	// Configure AzStorage component
	err := azComponent.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("failed to configure AzureStorage object [%s]", err.Error())
	}

	//  Start AzStorage the component so that credentials are verified
	err = azComponent.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AzureStorage object [%s]", err.Error())
	}

	// Get the list of containers from the component
	containerList, err = azComponent.ListContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to get container list from storage [%s]", err.Error())
	}

	// Stop the azStorage component as its no more needed now
	_ = azComponent.Stop()
	return containerList, nil
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
func mountAllContainers(containerList []string, configFile string, mountPath string) error {
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
			err = os.MkdirAll(contMountPath, 0777)
			if err != nil {
				fmt.Printf("Failed to create directory %s : %s\n", contMountPath, err.Error())
			}
		}

		// NOTE : Add all the configs that need replacement based on container here

		// If next instance is not mounted in background then mountall will hang up hence always mount in background
		viper.Set("foreground", false)
		viper.Set("azstorage.container", container)
		viper.Set("file_cache.path", filepath.Join(fileCachePath, container))

		// Create config file with container specific configs
		err := writeConfigFile(contConfigFile)
		if err != nil {
			return err
		}

		// Now that we have mount path and config file for this container fire a mount command for this one
		cliParams[1] = contMountPath
		cliParams[2] = "--config-file=" + contConfigFile

		fmt.Println("Mounting container :", container, "to path :", contMountPath)
		cmd := exec.Command(mountAllOpts.blobfuse2BinPath, cliParams...)

		cliOut, err := cmd.Output()
		fmt.Println(string(cliOut))
		if err != nil {
			fmt.Printf("Failed to mount container %s : %s\n", container, err.Error())
		}
	}
	return nil
}

func writeConfigFile(contConfigFile string) error {
	if options.SecureConfig {
		allConf := viper.AllSettings()
		confStream, err := yaml.Marshal(allConf)
		if err != nil {
			return fmt.Errorf("failed to marshall yaml content")
		}

		cipherText, err := common.EncryptData(confStream, []byte(options.PassPhrase))
		if err != nil {
			return fmt.Errorf("failed to encrypt yaml content [%s]", err.Error())
		}

		err = ioutil.WriteFile(contConfigFile, cipherText, 0777)
		if err != nil {
			return fmt.Errorf("failed to write encrypted file [%s]", err.Error())
		}
	} else {
		// Write modified config as per container to a new config file
		err := viper.WriteConfigAs(contConfigFile)
		if err != nil {
			return fmt.Errorf("failed to write config file [%s]", err.Error())
		}
	}
	return nil
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
	return strings.HasPrefix(opt, "--config-file")
}
