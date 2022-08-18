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
	"os/exec"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	"github.com/spf13/cobra"
)

type monitorOptions struct {
	EnableMon         bool     `config:"enable-monitoring"`
	DisableList       []string `config:"monitor-disable-list"`
	BfsPollInterval   int      `config:"blobfuse2-poll-interval"`
	StatsPollinterval int      `config:"stats-poll-interval"`
}

var pid string
var cacheMonitorOptions file_cache.FileCacheOptions
var hmonOptions monitorOptions

func resetMonitorOptions() {
	hmonOptions = monitorOptions{}
	cacheMonitorOptions = file_cache.FileCacheOptions{}
}

var healthMonCmd = &cobra.Command{
	Use:               "health-monitor",
	Short:             "Monitor blobfuse2 mount",
	Long:              "Monitor blobfuse2 mount",
	SuggestFor:        []string{"healthmon", "monitor health"},
	Args:              cobra.ExactArgs(0),
	Hidden:            true,
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		resetMonitorOptions()

		err := validateMonOptions()
		if err != nil {
			log.Err("health-monitor: [%v]", err)
			return err
		}

		options.ConfigFile = configFile
		parseConfig()

		err = config.UnmarshalKey("file_cache", &cacheMonitorOptions)
		if err != nil {
			log.Err("health-monitor: FileCache config error [invalid config attributes]")
			return err
		}

		err = config.UnmarshalKey("health-monitor", &hmonOptions)
		if err != nil {
			log.Err("health-monitor: FileCache config error [invalid config attributes]")
			return err
		}

		cliParams := buildCliParamForMonitor()
		log.Debug("health-monitor: options = %v", cliParams)

		log.Debug("Starting health-monitor for blobfuse2 pid = %s", pid)

		hmcmd := exec.Command(hmcommon.HealthMon, cliParams...)
		cliOut, err := hmcmd.Output()
		if len(cliOut) > 0 {
			log.Debug("health-monitor: cliout = %v", string(cliOut))
			fmt.Println(string(cliOut))
		}
		if err != nil {
			log.Err("health-monitor: [%v]", err)
			return fmt.Errorf("failed to start health monitor: [%v]", err)
		}

		return nil
	},
}

func validateMonOptions() error {
	pid = strings.TrimSpace(pid)
	configFile = strings.TrimSpace(configFile)
	errMsg := ""

	if len(pid) == 0 {
		errMsg = "Pid of blobfuse2 process not given\n"
	}

	if len(configFile) == 0 {
		errMsg += "Config file not given\n"
	}

	if len(errMsg) != 0 {
		errMsg += "Failed to start health-monitor"
		return fmt.Errorf(errMsg)
	}

	return nil
}

func buildCliParamForMonitor() []string {
	var cliParams []string

	cliParams = append(cliParams, "--pid="+pid)
	if hmonOptions.BfsPollInterval != 0 {
		cliParams = append(cliParams, fmt.Sprintf("--blobfuse2-poll-interval=%v", hmonOptions.BfsPollInterval))
	}
	if hmonOptions.StatsPollinterval != 0 {
		cliParams = append(cliParams, fmt.Sprintf("--stats-poll-interval=%v", hmonOptions.StatsPollinterval))
	}
	cliParams = append(cliParams, "--cache-path="+cacheMonitorOptions.TmpPath)
	cliParams = append(cliParams, fmt.Sprintf("--max-size-mb=%v", cacheMonitorOptions.MaxSizeMB))

	for _, v := range hmonOptions.DisableList {
		switch v {
		case hmcommon.BlobfuseStats:
			cliParams = append(cliParams, "--no-blobfuse2-stats")
		case hmcommon.CpuProfiler:
			cliParams = append(cliParams, "--no-cpu-profiler")
		case hmcommon.MemoryProfiler:
			cliParams = append(cliParams, "--no-memory-profiler")
		case hmcommon.NetworkProfiler:
			cliParams = append(cliParams, "--no-network-profiler")
		case hmcommon.FileCacheMon:
			cliParams = append(cliParams, "--no-cache-monitor")
		default:
			log.Debug("health-monitor::buildCliParamForMonitor: Invalid health monitor option %v", v)
		}
	}

	return cliParams
}

func init() {
	rootCmd.AddCommand(healthMonCmd)

	healthMonCmd.Flags().StringVar(&pid, "pid", "", "Pid of blobfuse2 process")
	healthMonCmd.MarkFlagRequired("pid")

	healthMonCmd.Flags().StringVar(&configFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")
	healthMonCmd.MarkFlagRequired("config-file")
}
