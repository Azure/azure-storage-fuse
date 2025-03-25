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
	"fmt"
	"os/exec"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	"github.com/spf13/cobra"
)

type monitorOptions struct {
	EnableMon       bool     `config:"enable-monitoring"`
	DisableList     []string `config:"monitor-disable-list"`
	BfsPollInterval int      `config:"stats-poll-interval-sec"`
	ProcMonInterval int      `config:"process-monitor-interval-sec"`
	OutputPath      string   `config:"output-path"`
}

var pid string
var cacheMonitorOptions file_cache.FileCacheOptions

func resetMonitorOptions() {
	options.MonitorOpt = monitorOptions{}
	cacheMonitorOptions = file_cache.FileCacheOptions{}
}

var healthMonCmd = &cobra.Command{
	Use:               "health-monitor",
	Short:             "Monitor blobfuse2 mount",
	Long:              "Monitor blobfuse2 mount",
	SuggestFor:        []string{"bfusemon", "monitor health"},
	Args:              cobra.ExactArgs(0),
	Hidden:            true,
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(_ *cobra.Command, _ []string) error {
		resetMonitorOptions()

		err := validateHMonOptions()
		if err != nil {
			log.Err("health-monitor : failed to validate options [%s]", err.Error())
			return fmt.Errorf("failed to validate options [%s]", err.Error())
		}

		options.ConfigFile = configFile
		err = parseConfig()
		if err != nil {
			log.Err("health-monitor : failed to parse config [%s]", err.Error())
			return err
		}

		err = config.UnmarshalKey("file_cache", &cacheMonitorOptions)
		if err != nil {
			log.Err("health-monitor : file_cache config error (invalid config attributes) [%s]", err.Error())
			return fmt.Errorf("invalid file_cache config [%s]", err.Error())
		}

		err = config.UnmarshalKey("health_monitor", &options.MonitorOpt)
		if err != nil {
			log.Err("health-monitor : health_monitor config error (invalid config attributes) [%s]", err.Error())
			return fmt.Errorf("invalid health_monitor config [%s]", err.Error())
		}

		cliParams := buildCliParamForMonitor()
		log.Debug("health-monitor : Options = %v", cliParams)
		log.Debug("health-monitor : Starting health-monitor for blobfuse2 pid = %s", pid)

		hmcmd := exec.Command(hmcommon.BfuseMon, cliParams...)
		cliOut, err := hmcmd.Output()
		if len(cliOut) > 0 {
			log.Debug("health-monitor : cliout = %v", string(cliOut))
		}

		if err != nil {
			common.EnableMonitoring = false
			log.Err("health-monitor : failed to start health monitor [%s]", err.Error())
			return fmt.Errorf("failed to start health monitor [%s]", err.Error())
		}

		return nil
	},
}

func validateHMonOptions() error {
	pid = strings.TrimSpace(pid)
	configFile = strings.TrimSpace(configFile)
	errMsg := ""

	if len(pid) == 0 {
		errMsg = "pid of blobfuse2 process not given. "
	}

	if len(configFile) == 0 {
		errMsg += "config file not given."
	}

	if len(errMsg) != 0 {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func buildCliParamForMonitor() []string {
	var cliParams []string

	cliParams = append(cliParams, "--pid="+pid)
	if options.MonitorOpt.BfsPollInterval != 0 {
		cliParams = append(cliParams, fmt.Sprintf("--stats-poll-interval-sec=%v", options.MonitorOpt.BfsPollInterval))
	}
	if options.MonitorOpt.ProcMonInterval != 0 {
		cliParams = append(cliParams, fmt.Sprintf("--process-monitor-interval-sec=%v", options.MonitorOpt.ProcMonInterval))
	}

	if options.MonitorOpt.OutputPath != "" {
		cliParams = append(cliParams, fmt.Sprintf("--output-path=%v", options.MonitorOpt.OutputPath))
	}

	cliParams = append(cliParams, "--cache-path="+common.ExpandPath(cacheMonitorOptions.TmpPath))
	cliParams = append(cliParams, fmt.Sprintf("--max-size-mb=%v", cacheMonitorOptions.MaxSizeMB))

	for _, v := range options.MonitorOpt.DisableList {
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
			cliParams = append(cliParams, "--no-file-cache-monitor")
		default:
			log.Debug("health-monitor::buildCliParamForMonitor: Invalid health monitor option %v", v)
		}
	}

	return cliParams
}

func init() {
	rootCmd.AddCommand(healthMonCmd)

	healthMonCmd.Flags().StringVar(&pid, "pid", "", "Pid of blobfuse2 process")
	_ = healthMonCmd.MarkFlagRequired("pid")

	healthMonCmd.Flags().StringVar(&configFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")
	_ = healthMonCmd.MarkFlagRequired("config-file")
}
