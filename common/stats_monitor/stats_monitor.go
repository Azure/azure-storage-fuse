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

package stats_monitor

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
)

type StatsOptions struct {
	MountPath    string                      `json:"mount_dir"`
	FileCacheOpt file_cache.FileCacheOptions `json:"file_cache"`
	AzStorageOpt StatsAzStorageOptions       `json:"azstorage"`
}

type StatsAzStorageOptions struct {
	AccountType string `config:"type" yaml:"type,omitempty"`
	AccountName string `config:"account-name" yaml:"account-name,omitempty"`
	Endpoint    string `config:"endpoint" yaml:"endpoint,omitempty"`
	Container   string `config:"container" yaml:"container,omitempty"`
}

func parseStatsConfig(statsOpt *StatsOptions, mountPath string) error {
	statsOpt.MountPath = mountPath
	err := config.UnmarshalKey("file_cache", &statsOpt.FileCacheOpt)
	if err != nil {
		log.Err("stats_monitor: FileCache config error [invalid config attributes]")
		return err
	}

	err = config.UnmarshalKey("azstorage", &statsOpt.AzStorageOpt)
	if err != nil {
		log.Err("stats_monitor: Unable to unmarshal container")
		return err
	}

	return nil
}

// read the stats config file and store it in json list
func getConfigData(configFile string, statsOptList *[]StatsOptions) error {
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Err("stats_monitor: Unable to read stats config file %s", configFile)
		return err
	}

	if err = json.Unmarshal(configData, statsOptList); err != nil {
		log.Err("stats_monitor: Unable to unmarshal stats config file %s", configFile)
		return err
	}

	return nil
}

// check whether the file exists
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return (err == nil)
}

// add the config of new mount to the stats config file. If already present, just update it
func addMountConfig(statsOptList *[]StatsOptions, statsOpt *StatsOptions) {
	var isPresent bool = false

	for i, opt := range *statsOptList {
		if opt.AzStorageOpt.Container == statsOpt.AzStorageOpt.Container && opt.AzStorageOpt.Endpoint == statsOpt.AzStorageOpt.Endpoint {
			(*statsOptList)[i] = *statsOpt
			isPresent = true
			break
		}
	}
	if !isPresent {
		*statsOptList = append(*statsOptList, *statsOpt)
	}
	log.Trace("Number of mounts to monitor = %d", len(*statsOptList))
}

func GenerateMonitorConfig(mountPath string) error {
	var statsOpt StatsOptions = StatsOptions{}
	var statsOptList []StatsOptions
	var statsConfigFile string = os.ExpandEnv(common.StatsConfigFilePath)

	workDir := os.ExpandEnv(common.DefaultWorkDir)
	_ = os.MkdirAll(workDir, os.ModeDir|os.FileMode(0777))

	err := parseStatsConfig(&statsOpt, mountPath)
	if err != nil {
		return err
	}

	if FileExists(statsConfigFile) {
		err = getConfigData(statsConfigFile, &statsOptList)
		if err != nil {
			return err
		}
	} else {
		statsOptList = make([]StatsOptions, 0)
	}

	addMountConfig(&statsOptList, &statsOpt)

	cfgFile, err := json.MarshalIndent(statsOptList, "", "\t")
	if err != nil {
		log.Err("stats_monitor: Failed to marshal file cache options")
		return err
	}

	err = ioutil.WriteFile(statsConfigFile, cfgFile, 0755)
	if err != nil {
		log.Err("stats_monitor: Failed to write the config to the file, %s", statsConfigFile)
		return err
	}

	return nil
}
