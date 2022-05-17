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
	"blobfuse2/common"
	"blobfuse2/common/config"
	"blobfuse2/common/log"
	"blobfuse2/component/file_cache"
	"encoding/json"
	"os"
)

type StatsOptions struct {
	MountPath    string                      `json:"mount_dir"`
	FileCacheOpt file_cache.FileCacheOptions `json:"file_cache"`
}

func GenerateMonitorConfig(mountPath string) error {
	var statsOpt StatsOptions = StatsOptions{}
	statsOpt.MountPath = mountPath
	err := config.UnmarshalKey("file_cache", &statsOpt.FileCacheOpt)
	if err != nil {
		log.Err("stats_monitor: FileCache config error [invalid config attributes]")
		return err
	}

	cfgFile, err := json.MarshalIndent(statsOpt, "", "\t")
	if err != nil {
		log.Err("stats_monitor: Failed to marshal file cache options")
		return err
	}

	workDir := os.ExpandEnv(common.DefaultWorkDir)
	_ = os.MkdirAll(workDir, os.ModeDir|os.FileMode(0777))

	err = os.WriteFile(os.ExpandEnv(common.StatsConfigFilePath), cfgFile, 0755)
	if err != nil {
		log.Err("stats_monitor: Failed to write the config to the file, %s", common.StatsConfigFilePath)
		return err
	}

	return nil
}
