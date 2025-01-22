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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/attr_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	"github.com/Azure/azure-storage-fuse/v2/component/block_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/file_cache"
	"github.com/Azure/azure-storage-fuse/v2/component/libfuse"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type blobfuseCliOptions struct {
	configFile   string
	logLevel     string
	fuseLogging  bool
	useAttrCache bool
	useStreaming bool
	// fuseAttrTimeout   uint32
	// fuseEntryTimeout  uint32
	tmpPath           string
	cacheSize         float64
	fileCacheTimeout  uint32
	maxEviciton       uint32
	highDiskThreshold uint32
	lowDiskThreshold  uint32
	emptyDirCheck     bool
	blockSize         uint64
	maxBlocksPerFile  int
	streamCacheSize   uint64
	noSymlinks        bool
	cacheOnList       bool
	useAdls           bool
	useHttps          bool
	containerName     string
	maxConcurrency    uint16
	cancelListOnMount uint16
	maxRetry          int32
	maxRetryInterval  int32
	retryDelayFactor  int32
	httpProxy         string
	httpsProxy        string
	ignoreOpenFlags   bool
}
type ComponentsConfig []string
type PipelineConfig struct {
	ForegroundOption            bool `yaml:"foreground,omitempty"`
	ReadOnlyOption              bool `yaml:"read-only,omitempty"`
	AllowOtherOption            bool `yaml:"allow-other,omitempty"`
	NonEmptyMountOption         bool `yaml:"nonempty,omitempty"`
	LogOptions                  `yaml:"logging,omitempty"`
	libfuse.LibfuseOptions      `yaml:"libfuse,omitempty"`
	block_cache.StreamOptions   `yaml:"stream,omitempty"`
	file_cache.FileCacheOptions `yaml:"file_cache,omitempty"`
	attr_cache.AttrCacheOptions `yaml:"attr_cache,omitempty"`
	azstorage.AzStorageOptions  `yaml:"azstorage,omitempty"`
	ComponentsConfig            `yaml:"components,omitempty"`
}

var outputFilePath string
var mountPath string
var libfuseOptions []string
var bfConfCliOptions blobfuseCliOptions
var bfv2StorageConfigOptions azstorage.AzStorageOptions
var bfv2LoggingConfigOptions LogOptions
var bfv2FuseConfigOptions libfuse.LibfuseOptions
var bfv2FileCacheConfigOptions file_cache.FileCacheOptions
var bfv2AttrCacheConfigOptions attr_cache.AttrCacheOptions
var bfv2ComponentsConfigOptions ComponentsConfig
var bfv2StreamConfigOptions block_cache.StreamOptions
var bfv2ForegroundOption bool
var bfv2ReadOnlyOption bool
var bfv2NonEmptyMountOption bool
var bfv2AllowOtherOption bool
var useAttrCache bool
var useStream bool
var useFileCache bool = true
var convertConfigOnly bool
var enableGen1 bool
var reqFreeSpaceMB int

func resetOptions() {
	bfv2StorageConfigOptions = azstorage.AzStorageOptions{}
	bfv2LoggingConfigOptions = LogOptions{}
	bfv2FuseConfigOptions = libfuse.LibfuseOptions{}
	bfv2FileCacheConfigOptions = file_cache.FileCacheOptions{}
	bfv2AttrCacheConfigOptions = attr_cache.AttrCacheOptions{}
	bfv2ComponentsConfigOptions = ComponentsConfig{}
	bfv2StreamConfigOptions = block_cache.StreamOptions{}
	bfv2ForegroundOption = false
	bfv2ReadOnlyOption = false
	bfv2NonEmptyMountOption = false
	bfv2AllowOtherOption = false
	useAttrCache = false
	useStream = false
	useFileCache = true
}

var generateConfigCmd = &cobra.Command{
	Use:               "mountv1",
	Short:             "Generate a configuration file for Blobfuse2 from Blobfuse configuration file/flags",
	Long:              "Generate a configuration file for Blobfuse2 from Blobfuse configuration file/flags",
	SuggestFor:        []string{"conv config", "convert config"},
	Args:              cobra.MaximumNArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				log.Err(err.Error())
			}
		}
		resetOptions()
		// If we are only converting the config without mounting then we do not need the mount path and therefore the args length would be 0
		if len(args) == 1 {
			mountPath = args[0]
		}

		file, err := os.Open(bfConfCliOptions.configFile)
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				// some users may have a commented out config
				linePieces := strings.SplitN(scanner.Text(), "#", 2)
				line := linePieces[0]
				configParam := strings.Fields(line)
				if len(configParam) == 0 {
					continue
				}
				if len(configParam) != 2 {
					return fmt.Errorf("failed to read configuration file. Configuration %s is incorrect. Make sure your configuration file parameters are of the format `key value`", configParam)
				}

				// get corresponding Blobfuse2 configurations from the config file parameters
				err := convertBfConfigParameter(cmd.Flags(), configParam[0], configParam[1])
				if err != nil {
					return fmt.Errorf("failed to convert configuration parameters [%s]", err.Error())
				}

			}
		}

		bfv2ComponentsConfigOptions = append(bfv2ComponentsConfigOptions, "libfuse")
		// get corresponding Blobfuse2 configurations from the cli parameters - these supersede the config options
		err = convertBfCliParameters(cmd.Flags())
		if err != nil {
			return fmt.Errorf("failed to convert CLI parameters [%s]", err.Error())
		}

		// if we have the o being passed then parse it
		if cmd.Flags().Lookup("o").Changed {
			err := parseFuseConfig(libfuseOptions)
			if err != nil {
				return err
			}
		}
		if useStream {
			bfv2ComponentsConfigOptions = append(bfv2ComponentsConfigOptions, "stream")
		}
		if useFileCache {
			bfv2ComponentsConfigOptions = append(bfv2ComponentsConfigOptions, "file_cache")
		}
		if useAttrCache {
			bfv2ComponentsConfigOptions = append(bfv2ComponentsConfigOptions, "attr_cache")
		}
		bfv2ComponentsConfigOptions = append(bfv2ComponentsConfigOptions, "azstorage")

		// Set the endpoint if not explicitly provided
		if bfv2StorageConfigOptions.Endpoint == "" {
			accountName := bfv2StorageConfigOptions.AccountName
			if accountName == "" {
				res, ok := os.LookupEnv(azstorage.EnvAzStorageAccount)
				if !ok {
					return fmt.Errorf("invalid account name")
				} else {
					accountName = res
				}
			}
			http := "https"
			if bfv2StorageConfigOptions.UseHTTP {
				http = "http"
			}

			accountType := ""
			if bfv2StorageConfigOptions.AccountType == "" || bfv2StorageConfigOptions.AccountType == "blob" {
				accountType = "blob"
			} else if bfv2StorageConfigOptions.AccountType == "adls" {
				accountType = "dfs"
			} else {
				return fmt.Errorf("invalid account type")
			}
			bfv2StorageConfigOptions.Endpoint = fmt.Sprintf("%s://%s.%s.core.windows.net", http, accountName, accountType)
		}
		bfv2StorageConfigOptions.VirtualDirectory = true

		pConf := PipelineConfig{
			bfv2ForegroundOption,
			bfv2ReadOnlyOption,
			bfv2AllowOtherOption,
			bfv2NonEmptyMountOption,
			bfv2LoggingConfigOptions,
			bfv2FuseConfigOptions,
			bfv2StreamConfigOptions,
			bfv2FileCacheConfigOptions,
			bfv2AttrCacheConfigOptions,
			bfv2StorageConfigOptions,
			bfv2ComponentsConfigOptions}

		data, _ := yaml.Marshal(&pConf)
		err2 := os.WriteFile(outputFilePath, data, 0700)
		if err2 != nil {
			return fmt.Errorf("failed to write file [%s]", err2.Error())
		}

		if !convertConfigOnly {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			if enableGen1 {
				rootCmd.SetArgs([]string{"mountgen1", mountPath, fmt.Sprintf("--config-file=%s", outputFilePath), fmt.Sprintf("--required-free-space-mb=%v", reqFreeSpaceMB)})
			} else {
				rootCmd.SetArgs([]string{"mount", mountPath, fmt.Sprintf("--config-file=%s", outputFilePath), "--disable-version-check=true"})
			}
			err := rootCmd.Execute()
			if err != nil {
				return fmt.Errorf("failed to execute command [%s]", err.Error())
			}
		}
		return nil
	},
}

// `-o negative_timeout`: files that do not exist
// `-o ro`: read-only mode
// `-o entry_timeout`: timeout in seconds for which name lookups will be cached
// `-o attr_timeout`: The timeout in seconds for which file/directory attributes
// `-o umask`: inverse of default permissions being set, so 0000 is 0777
// `-d` : enable debug logs and foreground on
func parseFuseConfig(config []string) error {
	for _, v := range config {
		parameter := strings.Split(v, "=")
		if len(parameter) > 2 || len(parameter) <= 0 {
			return errors.New(common.FuseAllowedFlags)
		}

		v = strings.TrimSpace(v)
		if ignoreFuseOptions(v) {
			continue
		} else if v == "allow_other" || v == "allow_other=true" {
			bfv2AllowOtherOption = true
		} else if v == "allow_other=false" {
			bfv2AllowOtherOption = false
		} else if v == "nonempty" {
			bfv2NonEmptyMountOption = true
		} else if strings.HasPrefix(v, "attr_timeout=") {
			timeout, err := strconv.ParseUint(parameter[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse attr_timeout [%s]", err.Error())
			}
			bfv2FuseConfigOptions.AttributeExpiration = uint32(timeout)
		} else if strings.HasPrefix(v, "entry_timeout=") {
			timeout, err := strconv.ParseUint(parameter[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse entry_timeout [%s]", err.Error())
			}
			bfv2FuseConfigOptions.EntryExpiration = uint32(timeout)
		} else if strings.HasPrefix(v, "negative_timeout=") {
			timeout, err := strconv.ParseUint(parameter[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse negative_timeout [%s]", err.Error())
			}
			bfv2FuseConfigOptions.NegativeEntryExpiration = uint32(timeout)
		} else if v == "ro" {
			bfv2ReadOnlyOption = true
		} else if v == "allow_root" {
			bfv2FuseConfigOptions.DefaultPermission = 700
		} else if strings.HasPrefix(v, "umask=") {
			permission, err := strconv.ParseUint(parameter[1], 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse umask [%s]", err.Error())
			}
			perm := ^uint32(permission) & 777
			bfv2FuseConfigOptions.DefaultPermission = perm
		} else {
			return errors.New(common.FuseAllowedFlags)
		}
	}

	return nil
}

// helper method: converts config file options
func convertBfConfigParameter(flags *pflag.FlagSet, configParameterKey string, configParameterValue string) error {
	switch configParameterKey {
	case "logLevel":
		if !flags.Lookup("log-level").Changed {
			bfv2LoggingConfigOptions.LogLevel = configParameterValue
		}
	case "accountName":
		bfv2StorageConfigOptions.AccountName = configParameterValue
	case "accountKey":
		bfv2StorageConfigOptions.AccountKey = configParameterValue
	case "accountType":
		if !flags.Lookup("use-adls").Changed {
			bfv2StorageConfigOptions.AccountType = configParameterValue
		}
	case "aadEndpoint":
		bfv2StorageConfigOptions.ActiveDirectoryEndpoint = configParameterValue
	case "authType":
		bfv2StorageConfigOptions.AuthMode = strings.ToLower(configParameterValue)
	case "blobEndpoint":
		bfv2StorageConfigOptions.Endpoint = configParameterValue
	case "containerName":
		if !flags.Lookup("container-name").Changed {
			bfv2StorageConfigOptions.Container = configParameterValue
		}
	case "httpProxy":
		if !flags.Lookup("http-proxy").Changed {
			bfv2StorageConfigOptions.HttpProxyAddress = configParameterValue
		}
	case "identityClientId":
		bfv2StorageConfigOptions.ApplicationID = configParameterValue
	case "httpsProxy":
		bfv2StorageConfigOptions.HttpsProxyAddress = configParameterValue
	case "identityObjectId":
		bfv2StorageConfigOptions.ObjectID = configParameterValue
	case "identityResourceId":
		bfv2StorageConfigOptions.ResourceID = configParameterValue
	case "sasToken":
		bfv2StorageConfigOptions.SaSKey = configParameterValue
	case "servicePrincipalClientId":
		bfv2StorageConfigOptions.ClientID = configParameterValue
	case "servicePrincipalClientSecret":
		bfv2StorageConfigOptions.ClientSecret = configParameterValue
	case "servicePrincipalTenantId":
		bfv2StorageConfigOptions.TenantID = configParameterValue

	case "msiEndpoint":
		// msiEndpoint is not supported config in V2, this needs to be given as MSI_ENDPOINT env variable
		return nil

	default:
		return fmt.Errorf("failed to parse configuration file. Configuration parameter `%s` is not supported in Blobfuse2", configParameterKey)
	}

	return nil
}

// helper method: converts cli options - cli options that overlap with config file take precedence
func convertBfCliParameters(flags *pflag.FlagSet) error {
	if flags.Lookup("set-content-type").Changed || flags.Lookup("ca-cert-file").Changed || flags.Lookup("basic-remount-check").Changed || flags.Lookup(
		"background-download").Changed || flags.Lookup("cache-poll-timeout-msec").Changed || flags.Lookup("upload-modified-only").Changed || flags.Lookup("debug-libcurl").Changed {
		logWriter, _ := syslog.New(syslog.LOG_WARNING, "")
		_ = logWriter.Warning("one or more unsupported v1 parameters [set-content-type, ca-cert-file, basic-remount-check, background-download, cache-poll-timeout-msec, upload-modified-only, debug-libcurl] have been passed, ignoring and proceeding to mount")
	}

	bfv2LoggingConfigOptions.Type = "syslog"
	if flags.Lookup("log-level").Changed {
		bfv2LoggingConfigOptions.LogLevel = bfConfCliOptions.logLevel
	}

	if flags.Lookup("streaming").Changed {
		if bfConfCliOptions.useStreaming {
			useStream = true
			useFileCache = false
			if flags.Lookup("block-size-mb").Changed {
				bfv2StreamConfigOptions.BlockSize = bfConfCliOptions.blockSize
			}
			if flags.Lookup("max-blocks-per-file").Changed {
				bfv2StreamConfigOptions.BufferSize = bfConfCliOptions.blockSize * uint64(bfConfCliOptions.maxBlocksPerFile)
			}
			if flags.Lookup("stream-cache-mb").Changed {
				bfv2StreamConfigOptions.CachedObjLimit = bfConfCliOptions.streamCacheSize / bfv2StreamConfigOptions.BufferSize
				if bfv2StreamConfigOptions.CachedObjLimit == 0 {
					bfv2StreamConfigOptions.CachedObjLimit = 1
				}
			}
		} else {
			useStream = false
			useFileCache = true
		}
	}

	if flags.Lookup("use-attr-cache").Changed {
		useAttrCache = true
		if bfConfCliOptions.useAttrCache {
			if flags.Lookup("cache-on-list").Changed {
				if bfConfCliOptions.cacheOnList {
					bfv2AttrCacheConfigOptions.NoCacheOnList = !bfConfCliOptions.cacheOnList
				}
			}
			if flags.Lookup("no-symlinks").Changed {
				if bfConfCliOptions.noSymlinks {
					bfv2AttrCacheConfigOptions.NoSymlinks = bfConfCliOptions.noSymlinks
				}
			}
		}
	}
	if flags.Lookup("tmp-path").Changed {
		bfv2FileCacheConfigOptions.TmpPath = bfConfCliOptions.tmpPath
	}
	if flags.Lookup("cache-size-mb").Changed {
		bfv2FileCacheConfigOptions.MaxSizeMB = bfConfCliOptions.cacheSize
	}
	if flags.Lookup("file-cache-timeout-in-seconds").Changed {
		bfv2FileCacheConfigOptions.Timeout = bfConfCliOptions.fileCacheTimeout
	}
	if flags.Lookup("max-eviction").Changed {
		bfv2FileCacheConfigOptions.MaxEviction = bfConfCliOptions.maxEviciton
	}
	if flags.Lookup("high-disk-threshold").Changed {
		bfv2FileCacheConfigOptions.HighThreshold = bfConfCliOptions.highDiskThreshold
	}
	if flags.Lookup("low-disk-threshold").Changed {
		bfv2FileCacheConfigOptions.LowThreshold = bfConfCliOptions.lowDiskThreshold
	}
	if flags.Lookup("empty-dir-check").Changed {
		bfv2FileCacheConfigOptions.AllowNonEmpty = !bfConfCliOptions.emptyDirCheck
	}
	if flags.Lookup("use-adls").Changed {
		if bfConfCliOptions.useAdls {
			bfv2StorageConfigOptions.AccountType = "adls"
		} else {
			bfv2StorageConfigOptions.AccountType = "block"
		}
	}
	if flags.Lookup("use-https").Changed {
		bfv2StorageConfigOptions.UseHTTP = !bfConfCliOptions.useHttps
	}
	if flags.Lookup("container-name").Changed {
		bfv2StorageConfigOptions.Container = bfConfCliOptions.containerName
	}
	if flags.Lookup("max-concurrency").Changed {
		bfv2StorageConfigOptions.MaxConcurrency = bfConfCliOptions.maxConcurrency
	}
	if flags.Lookup("cancel-list-on-mount-seconds").Changed {
		bfv2StorageConfigOptions.CancelListForSeconds = bfConfCliOptions.cancelListOnMount
	}
	if flags.Lookup("max-retry").Changed {
		bfv2StorageConfigOptions.MaxRetries = bfConfCliOptions.maxRetry
	}
	if flags.Lookup("max-retry-interval-in-seconds").Changed {
		bfv2StorageConfigOptions.MaxTimeout = bfConfCliOptions.maxRetryInterval
	}
	if flags.Lookup("retry-delay-factor").Changed {
		bfv2StorageConfigOptions.BackoffTime = bfConfCliOptions.retryDelayFactor
	}
	if flags.Lookup("http-proxy").Changed {
		bfv2StorageConfigOptions.HttpProxyAddress = bfConfCliOptions.httpProxy
	}
	if flags.Lookup("https-proxy").Changed {
		bfv2StorageConfigOptions.HttpsProxyAddress = bfConfCliOptions.httpsProxy
	}
	if flags.Lookup("d").Changed {
		bfv2FuseConfigOptions.EnableFuseTrace = bfConfCliOptions.fuseLogging
		bfv2ForegroundOption = bfConfCliOptions.fuseLogging
	}
	if flags.Lookup("ignore-open-flags").Changed {
		bfv2FuseConfigOptions.IgnoreOpenFlags = bfConfCliOptions.ignoreOpenFlags
	}
	return nil
}

func init() {
	rootCmd.AddCommand(generateConfigCmd)
	generateConfigCmd.Flags().StringVar(&outputFilePath, "output-file", "config.yaml", "Output Blobfuse configuration file.")

	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.tmpPath, "tmp-path", "", "Tmp location for the file cache.")
	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.configFile, "config-file", "", "Input Blobfuse configuration file.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.useHttps, "use-https", false, "Enables HTTPS communication with Blob storage.")
	generateConfigCmd.Flags().Uint32Var(&bfConfCliOptions.fileCacheTimeout, "file-cache-timeout-in-seconds", 0, "During this time, blobfuse will not check whether the file is up to date or not.")
	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.containerName, "container-name", "", "Required if no configuration file is specified.")
	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.logLevel, "log-level", "LOG_WARNING", "Logging level.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.useAttrCache, "use-attr-cache", false, "Enable attribute cache.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.useAdls, "use-adls", false, "Enables blobfuse to access Azure DataLake storage account.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.noSymlinks, "no-symlinks", false, "Disables symlink support.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.cacheOnList, "cache-on-list", true, "Cache attributes on listing.")
	generateConfigCmd.Flags().Uint16Var(&bfConfCliOptions.maxConcurrency, "max-concurrency", 0, "Option to override default number of concurrent storage connections")
	generateConfigCmd.Flags().Float64Var(&bfConfCliOptions.cacheSize, "cache-size-mb", 0, "File cache size.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.emptyDirCheck, "empty-dir-check", false, "Disallows remounting using a non-empty tmp-path.")
	generateConfigCmd.Flags().Uint16Var(&bfConfCliOptions.cancelListOnMount, "cancel-list-on-mount-seconds", 0, "A list call to the container is by default issued on mount.")
	generateConfigCmd.Flags().Uint32Var(&bfConfCliOptions.highDiskThreshold, "high-disk-threshold", 0, "High disk threshold percentage.")
	generateConfigCmd.Flags().Uint32Var(&bfConfCliOptions.lowDiskThreshold, "low-disk-threshold", 0, "Low disk threshold percentage.")
	generateConfigCmd.Flags().Uint32Var(&bfConfCliOptions.maxEviciton, "max-eviction", 0, "Number of files to be evicted from cache at once.")
	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.httpsProxy, "https-proxy", "", "HTTPS Proxy address.")
	generateConfigCmd.Flags().StringVar(&bfConfCliOptions.httpProxy, "http-proxy", "", "HTTP Proxy address.")
	generateConfigCmd.Flags().Int32Var(&bfConfCliOptions.maxRetry, "max-retry", 0, "Maximum retry count if the failure codes are retryable.")
	generateConfigCmd.Flags().Int32Var(&bfConfCliOptions.maxRetryInterval, "max-retry-interval-in-seconds", 0, "Maximum number of seconds between 2 retries.")
	generateConfigCmd.Flags().Int32Var(&bfConfCliOptions.retryDelayFactor, "retry-delay-factor", 0, "Retry delay between two tries")
	//invalidate-on-sync is always on - accept it as an arg and just ignore it
	generateConfigCmd.Flags().Bool("invalidate-on-sync", true, "Invalidate file/dir on sync/fsync")
	//pre-mount-validate is always on - accept it as an arg and just ignore it
	generateConfigCmd.Flags().Bool("pre-mount-validate", true, "Validate blobfuse2 is mounted")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.useStreaming, "streaming", false, "Enable Streaming.")
	generateConfigCmd.Flags().Uint64Var(&bfConfCliOptions.streamCacheSize, "stream-cache-mb", 0, "Limit total amount of data being cached in memory to conserve memory footprint of blobfuse.")
	generateConfigCmd.Flags().IntVar(&bfConfCliOptions.maxBlocksPerFile, "max-blocks-per-file", 0, "Maximum number of blocks to be cached in memory for streaming.")
	generateConfigCmd.Flags().Uint64Var(&bfConfCliOptions.blockSize, "block-size-mb", 0, "Size (in MB) of a block to be downloaded during streaming.")

	generateConfigCmd.Flags().StringSliceVarP(&libfuseOptions, "o", "o", []string{}, "FUSE options.")
	generateConfigCmd.Flags().BoolVarP(&bfConfCliOptions.fuseLogging, "d", "d", false, "Mount with foreground and FUSE logs on.")
	generateConfigCmd.Flags().BoolVar(&convertConfigOnly, "convert-config-only", false, "Don't mount - only convert v1 configuration to v2.")
	generateConfigCmd.Flags().BoolVar(&bfConfCliOptions.ignoreOpenFlags, "ignore-open-flags", false, "Flag to ignore open flags unsupported by blobfuse.")

	// options that are not available in V2:
	generateConfigCmd.Flags().Bool("set-content-type", false, "Turns on automatic 'content-type' property based on the file extension.")
	generateConfigCmd.Flags().String("ca-cert-file", "", "Specifies the proxy pem certificate path if its not in the default path.")
	generateConfigCmd.Flags().Bool("basic-remount-check", false, "Check for an already mounted status using /etc/mtab.")
	generateConfigCmd.Flags().Bool("background-download", false, "File download to run in the background on open call.")
	generateConfigCmd.Flags().Uint64("cache-poll-timeout-msec", 0, "Time in milliseconds in order to poll for possible expired files awaiting cache eviction.")
	generateConfigCmd.Flags().Bool("upload-modified-only", false, "Flag to turn off unnecessary uploads to storage.")
	generateConfigCmd.Flags().Bool("debug-libcurl", false, "Flag to allow users to debug libcurl calls.")

	// flags for gen1 mount
	generateConfigCmd.Flags().BoolVar(&enableGen1, "enable-gen1", false, "To enable Gen1 mount")
	generateConfigCmd.Flags().IntVar(&reqFreeSpaceMB, "required-free-space-mb", 0, "Required free space in MB")
}
