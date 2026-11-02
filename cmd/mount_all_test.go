/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     | |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMountAllRejectsDistributedCache(t *testing.T) {
	const discovery = "distributed_cache:\n  discovery-endpoint: discovery:9065\n"
	const discoveryFlag = "--distributed-cache-discovery-endpoint=discovery:9065"

	tests := []struct {
		name          string
		yaml          string
		args          []string
		env           string
		serverList    string
		configFile    bool
		defaultConfig bool
		secure        bool
		allowed       bool
	}{
		{name: "components", yaml: "components: [libfuse, distributed_cache, azstorage]\n", configFile: true},
		{name: "discovery YAML", yaml: discovery, configFile: true},
		{name: "default config file", yaml: discovery, configFile: true, defaultConfig: true},
		{name: "read-only config", yaml: "read-only: true\n" + discovery, configFile: true},
		{name: "server list YAML", yaml: "distributed_cache:\n  server-list: cache:9065\n", configFile: true},
		{name: "discovery CLI with config", args: []string{discoveryFlag}, configFile: true},
		{name: "discovery environment with config", env: "discovery:9065", configFile: true},
		{name: "server list environment with config", serverList: "cache:9065", configFile: true},
		{name: "encrypted config", yaml: discovery, configFile: true, secure: true},
		{name: "discovery CLI without config", args: []string{discoveryFlag}},
		{name: "discovery environment without config", env: "discovery:9065"},
		{name: "server list environment without config", serverList: "cache:9065"},
		{name: "conflicting file cache config", yaml: discovery + "file_cache:\n  path: /user-cache\n", configFile: true},
		{name: "conflicting tmp path", args: []string{discoveryFlag, "--tmp-path=/user-cache"}},
		{name: "file cache config allowed", yaml: "components: [libfuse, file_cache, azstorage]\n", configFile: true, allowed: true},
		{name: "file cache CLI allowed", args: []string{"--tmp-path=/user-cache"}, allowed: true},
		{name: "empty discovery allowed", args: []string{"--distributed-cache-discovery-endpoint="}, allowed: true},
		{name: "tuning alone allowed", yaml: "distributed_cache:\n  block-size-mb: 32\n", configFile: true, allowed: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldOptions := options
			resetMountCLIFlags()
			t.Cleanup(func() {
				resetMountCLIFlags()
				options = oldOptions
			})
			t.Setenv("DISTRIBUTED_CACHE_DISCOVERY_ENDPOINT", tc.env)
			if tc.env == "" {
				require.NoError(t, os.Unsetenv("DISTRIBUTED_CACHE_DISCOVERY_ENDPOINT"))
			}
			t.Setenv("DISTRIBUTED_CACHE_SERVER_LIST", tc.serverList)
			if tc.serverList == "" {
				require.NoError(t, os.Unsetenv("DISTRIBUTED_CACHE_SERVER_LIST"))
			}

			t.Chdir(t.TempDir())
			options = mountOptions{}
			if tc.configFile {
				options.ConfigFile = "input.yaml"
				if tc.defaultConfig {
					options.ConfigFile = common.DefaultConfigFilePath
				}
				data := []byte(tc.yaml)
				if tc.secure {
					options.ConfigFile += SecureConfigExtension
					options.PassPhrase = "1234567890123456"
					var err error
					data, err = common.EncryptData(data, []byte(options.PassPhrase))
					require.NoError(t, err)
				}
				require.NoError(t, os.WriteFile(options.ConfigFile, data, 0600))
				if tc.defaultConfig {
					options.ConfigFile = ""
				}
			}
			require.NoError(t, mountCmd.PersistentFlags().Parse(tc.args))

			err := processCommand()
			if tc.allowed {
				require.EqualError(t, err, "mount path not provided", "ordinary mounts must reach normal validation")
			} else {
				require.EqualError(t, err, "mount all isn't supported with distributed cache; mount each container separately")
			}
			assert.False(t, config.IsSet("mount-all-containers"), "reject before container listing and child generation")
		})
	}
}

func TestMountAllContainersFileCacheChildConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		configFile bool
		secure     bool
	}{
		{name: "config file", configFile: true},
		{name: "encrypted config file", configFile: true, secure: true},
		{name: "CLI only"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldOptions, oldMountAllOpts := options, mountAllOpts
			oldArgs, oldWorkDir := os.Args, common.DefaultWorkDir
			resetMountCLIFlags()
			t.Cleanup(func() {
				resetMountCLIFlags()
				options, mountAllOpts = oldOptions, oldMountAllOpts
				os.Args, common.DefaultWorkDir = oldArgs, oldWorkDir
			})

			workDir := t.TempDir()
			common.DefaultWorkDir = workDir
			t.Setenv("BLOBFUSE_MOUNT_ALL_TEST_ARGS", workDir)
			executable := filepath.Join(workDir, "capture-mount")
			require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$BLOBFUSE_MOUNT_ALL_TEST_ARGS/${2##*/}\"\n"), 0700))
			mountAllOpts.blobfuse2BinPath = executable
			options = mountOptions{SecureConfig: tc.secure, PassPhrase: "1234567890123456"}
			mountPath := filepath.Join(workDir, "mounts")
			os.Args = []string{"blobfuse2", "mount", "all", mountPath}
			configFile := ""
			if tc.configFile {
				configFile = "input.yaml"
				viper.Set("file_cache.path", "/user-cache")
			} else {
				require.NoError(t, mountCmd.PersistentFlags().Parse([]string{"--tmp-path=/user-cache"}))
				os.Args = append(os.Args, "--tmp-path=/user-cache")
			}
			containers := []string{"photos", "videos"}
			require.NoError(t, mountAllContainers(containers, configFile, mountPath, tc.configFile))

			for _, container := range containers {
				data, err := os.ReadFile(filepath.Join(workDir, container))
				require.NoError(t, err)
				args := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
				require.GreaterOrEqual(t, len(args), 3)
				assert.Equal(t, "mount", args[0])
				assert.Equal(t, filepath.Join(mountPath, container), args[1])
				assert.DirExists(t, args[1])
				assert.Contains(t, args, "--disable-version-check=true")

				if tc.configFile {
					childPath := filepath.Join(workDir, "config_"+container+".yaml")
					if tc.secure {
						childPath += SecureConfigExtension
					}
					assert.Equal(t, "--config-file="+childPath, args[2])
					data, err := os.ReadFile(childPath)
					require.NoError(t, err)
					if tc.secure {
						data, err = common.DecryptData(data, []byte("1234567890123456"))
						require.NoError(t, err)
					}
					childConfig := viper.New()
					childConfig.SetConfigType("yaml")
					require.NoError(t, childConfig.ReadConfig(strings.NewReader(string(data))))
					assert.Equal(t, args[1], childConfig.GetString("mount-path"))
					assert.Equal(t, container, childConfig.GetString("azstorage.container"))
					assert.False(t, childConfig.GetBool("foreground"))
					assert.Equal(t, filepath.Join("/user-cache", container), childConfig.GetString("file_cache.path"))
				} else {
					assert.Equal(t, "--foreground=false", args[2])
					assert.Contains(t, args, "--container-name="+container)
					assert.Contains(t, args, "--tmp-path="+filepath.Join("/user-cache", container))
				}
			}
		})
	}
}
