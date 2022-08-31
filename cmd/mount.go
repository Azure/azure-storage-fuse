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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/exectime"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
)

type LogOptions struct {
	Type           string `config:"type" yaml:"type,omitempty"`
	LogLevel       string `config:"level" yaml:"level,omitempty"`
	LogFilePath    string `config:"file-path" yaml:"file-path,omitempty"`
	MaxLogFileSize uint64 `config:"max-file-size-mb" yaml:"max-file-size-mb,omitempty"`
	LogFileCount   uint64 `config:"file-count" yaml:"file-count,omitempty"`
	TimeTracker    bool   `config:"track-time" yaml:"track-time,omitempty"`
}

type mountOptions struct {
	MountPath  string
	ConfigFile string

	Logging           LogOptions     `config:"logging"`
	Components        []string       `config:"components"`
	Foreground        bool           `config:"foreground"`
	DefaultWorkingDir string         `config:"default-working-dir"`
	Debug             bool           `config:"debug"`
	DebugPath         string         `config:"debug-path"`
	CPUProfile        string         `config:"cpu-profile"`
	MemProfile        string         `config:"mem-profile"`
	PassPhrase        string         `config:"passphrase"`
	SecureConfig      bool           `config:"secure-config"`
	DynamicProfiler   bool           `config:"dynamic-profile"`
	ProfilerPort      int            `config:"profiler-port"`
	ProfilerIP        string         `config:"profiler-ip"`
	MonitorOpt        monitorOptions `config:"health-monitor"`

	// v1 support
	Streaming      bool     `config:"streaming"`
	AttrCache      bool     `config:"use-attr-cache"`
	LibfuseOptions []string `config:"libfuse-options"`
}

var options mountOptions
var pipelineStarted bool //nolint

func (opt *mountOptions) validate(skipEmptyMount bool) error {
	if opt.MountPath == "" {
		return fmt.Errorf("argument error: mount path not provided")
	}

	if _, err := os.Stat(opt.MountPath); os.IsNotExist(err) {
		return fmt.Errorf("argument error: mount directory does not exists")
	} else if common.IsDirectoryMounted(opt.MountPath) {
		return fmt.Errorf("argument error: directory is already mounted")
	} else if !skipEmptyMount && !common.IsDirectoryEmpty(opt.MountPath) {
		return fmt.Errorf("argument error: mount directory is not empty")
	}

	if err := common.ELogLevel.Parse(opt.Logging.LogLevel); err != nil {
		return fmt.Errorf("argument error: invalid log-level = %s", opt.Logging.LogLevel)
	}
	opt.Logging.LogFilePath = os.ExpandEnv(opt.Logging.LogFilePath)
	if !common.DirectoryExists(filepath.Dir(opt.Logging.LogFilePath)) {
		err := os.MkdirAll(filepath.Dir(opt.Logging.LogFilePath), os.FileMode(0666)|os.ModeDir)
		if err != nil {
			return fmt.Errorf("argument error: invalid log-file-path = %s", opt.Logging.LogFilePath)
		}
	}

	// A user provided value of 0 doesn't make sense for MaxLogFileSize or LogFileCount.
	if opt.Logging.MaxLogFileSize == 0 {
		opt.Logging.MaxLogFileSize = common.DefaultMaxLogFileSize
	}

	if opt.Logging.LogFileCount == 0 {
		opt.Logging.LogFileCount = common.DefaultLogFileCount
	}

	if opt.DefaultWorkingDir != "" {
		common.DefaultWorkDir = opt.DefaultWorkingDir
		common.DefaultLogFilePath = filepath.Join(common.DefaultWorkDir, "blobfuse2.log")
	}

	if opt.Debug {
		_, err := os.Stat(opt.DebugPath)
		if os.IsNotExist(err) {
			err := os.MkdirAll(opt.DebugPath, os.FileMode(0755))
			if err != nil {
				return fmt.Errorf("argument error: invalid debug path")
			}
		}
	}
	return nil
}

func OnConfigChange() {
	newLogOptions := &LogOptions{}
	err := config.UnmarshalKey("logging", newLogOptions)
	if err != nil {
		log.Err("Mount.go::OnConfigChange : Invalid logging options [%s]", err)
	}

	var logLevel common.LogLevel
	err = logLevel.Parse(newLogOptions.LogLevel)
	if err != nil {
		log.Err("Mount.go::OnConfigChange : Invalid log level [%s]", newLogOptions.LogLevel)
	}

	err = log.SetConfig(common.LogConfig{
		Level:       logLevel,
		FilePath:    os.ExpandEnv(newLogOptions.LogFilePath),
		MaxFileSize: newLogOptions.MaxLogFileSize,
		FileCount:   newLogOptions.LogFileCount,
		TimeTracker: newLogOptions.TimeTracker,
	})

	if err != nil {
		log.Err("Mount.go::OnConfigChange : Unable to reset Logging options [%s]", err)
	}
}

// parseConfig : Based on config file or encrypted data parse the provided config
func parseConfig() {
	// Based on extension decide file is encrypted or not
	if options.SecureConfig ||
		filepath.Ext(options.ConfigFile) == SecureConfigExtension {
		fmt.Println("Secure config provided, going for decryption")

		// Validate config is to be secured on write or not
		if options.PassPhrase == "" {
			options.PassPhrase = os.Getenv(SecureConfigEnvName)
		}

		if options.PassPhrase == "" {
			fmt.Println("argument error: No passphrase provided to decrypt the config file.",
				"Either use --passphrase cli option or store passphrase in BLOBFUSE2_SECURE_CONFIG_PASSPHRASE environment variable.")
			os.Exit(1)
		}

		cipherText, err := ioutil.ReadFile(options.ConfigFile)
		if err != nil {
			fmt.Println("failed to read encrypted config file ", options.ConfigFile, "[", err.Error(), "]")
			os.Exit(1)
		}

		plainText, err := common.DecryptData(cipherText, []byte(options.PassPhrase))
		if err != nil {
			fmt.Println("failed to decrypt config file ", options.ConfigFile, "[", err.Error(), "]")
			os.Exit(1)
		}

		config.SetConfigFile(options.ConfigFile)
		config.SetSecureConfigOptions(options.PassPhrase)
		err = config.ReadFromConfigBuffer(plainText)
		if err != nil {
			fmt.Printf("invalid decrypted config file [%v]", err)
			os.Exit(1)
		}

	} else {
		err := config.ReadFromConfigFile(options.ConfigFile)
		if err != nil {
			fmt.Printf("invalid config file [%v]", err)
			os.Exit(1)
		}
	}
}

var mountCmd = &cobra.Command{
	Use:               "mount [path]",
	Short:             "Mounts the azure container as a filesystem",
	Long:              "Mounts the azure container as a filesystem",
	SuggestFor:        []string{"mnt", "mout"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Parent().Run(cmd.Parent(), args)

		options.MountPath = args[0]
		configFileExists := true

		if options.ConfigFile == "" {
			// Config file is not set in cli parameters
			// Blobfuse2 defaults to config.yaml in current directory
			// If the file does not exists then user might have configured required things in env variables
			// Fall back to defaults and let components fail if all required env variables are not set.
			_, err := os.Stat(common.DefaultConfigFilePath)
			if err != nil && os.IsNotExist(err) {
				configFileExists = false
			} else {
				options.ConfigFile = common.DefaultConfigFilePath
			}
		}

		if configFileExists {
			parseConfig()
		}

		err := config.Unmarshal(&options)
		if err != nil {
			fmt.Printf("Init error config unmarshall [%s]", err)
			os.Exit(1)
		}

		if !configFileExists || len(options.Components) == 0 {
			pipeline := []string{"libfuse"}

			if config.IsSet("streaming") && options.Streaming {
				pipeline = append(pipeline, "stream")
			} else {
				pipeline = append(pipeline, "file_cache")
			}

			// by default attr-cache is enable in v2
			// only way to disable is to pass cli param and set it to false
			if options.AttrCache {
				pipeline = append(pipeline, "attr_cache")
			}

			pipeline = append(pipeline, "azstorage")
			options.Components = pipeline
		}

		if config.IsSet("libfuse-options") {
			allowedFlags := "Mount: error allowed FUSE configurations are: `-o attr_timeout=TIMEOUT`, `-o negative_timeout=TIMEOUT`, `-o entry_timeout=TIMEOUT` `-o allow_other`, `-o allow_root`, `-o umask=PERMISSIONS -o default_permissions`, `-o ro`"
			// there are only 8 available options for -o so if we have more we should throw
			if len(options.LibfuseOptions) > 8 {
				fmt.Print(allowedFlags)
				os.Exit(1)
			}
			for _, v := range options.LibfuseOptions {
				parameter := strings.Split(v, "=")
				if len(parameter) > 2 || len(parameter) <= 0 {
					fmt.Print(allowedFlags)
					os.Exit(1)
				}
				v = strings.TrimSpace(v)
				if v == "default_permissions" {
					continue
				} else if v == "allow_other" || v == "allow_other=true" {
					config.Set("allow-other", "true")
				} else if strings.HasPrefix(v, "attr_timeout=") {
					config.Set("libfuse.attribute-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "entry_timeout=") {
					config.Set("libfuse.entry-expiration-sec", parameter[1])
				} else if strings.HasPrefix(v, "negative_timeout=") {
					config.Set("libfuse.negative-entry-expiration-sec", parameter[1])
				} else if v == "ro" || v == "ro=true" {
					config.Set("read-only", "true")
				} else if v == "allow_root" {
					config.Set("libfuse.default-permission", "700")
				} else if strings.HasPrefix(v, "umask=") {
					permission, err := strconv.ParseUint(parameter[1], 10, 32)
					if err != nil {
						fmt.Printf("Mount: %s", err)
						os.Exit(1)
					}
					perm := ^uint32(permission) & 777
					config.Set("libfuse.default-permission", fmt.Sprint(perm))
				} else {
					fmt.Print(allowedFlags)
					os.Exit(1)
				}
			}
		}

		if !config.IsSet("logging.file-path") {
			options.Logging.LogFilePath = common.DefaultLogFilePath
		}

		if !config.IsSet("logging.level") {
			options.Logging.LogLevel = "LOG_WARNING"
		}

		err = options.validate(false)
		if err != nil {
			fmt.Printf("Mount: error invalid options [%v]", err)
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

		if config.IsSet("invalidate-on-sync") {
			log.Warn("unsupported v1 CLI parameter: invalidate-on-sync is always true in blobfuse2.")
		}
		if config.IsSet("pre-mount-validate") {
			log.Warn("unsupported v1 CLI parameter: pre-mount-validate is always true in blobfuse2.")
		}
		if config.IsSet("basic-remount-check") {
			log.Warn("unsupported v1 CLI parameter: basic-remount-check is always true in blobfuse2.")
		}

		common.EnableMonitoring = options.MonitorOpt.EnableMon

		// check if blobfuse stats monitor is added in the disable list
		for _, mon := range options.MonitorOpt.DisableList {
			if mon == common.BfuseStats {
				common.BfsDisabled = true
				break
			}
		}

		if options.Debug {
			f, err := os.OpenFile(filepath.Join(options.DebugPath, "times.log"), os.O_CREATE|os.O_APPEND|os.O_RDWR, os.FileMode(0755))
			if err != nil {
				fmt.Printf("unable to open times.log file for exectime reporting [%s]", err)
			}
			exectime.SetDefault(f, true)
		} else {
			exectime.SetDefault(nil, false)
		}
		defer exectime.PrintStats()

		config.Set("mount-path", options.MountPath)

		var pipeline *internal.Pipeline

		log.Crit("Starting Blobfuse2 Mount : %s on (%s)", common.Blobfuse2Version, common.GetCurrentDistro())
		log.Crit("Logging level set to : %s", logLevel.String())
		pipeline, err = internal.NewPipeline(options.Components, !daemon.WasReborn())
		if err != nil {
			log.Err("Mount: error initializing new pipeline [%v]", err)
			fmt.Println("failed to mount :", err)
			Destroy(1)
		}

		if !options.Foreground {
			pidFile := strings.Replace(options.MountPath, "/", "_", -1) + ".pid"
			dmnCtx := &daemon.Context{
				PidFileName: filepath.Join(os.ExpandEnv(common.DefaultWorkDir), pidFile),
				PidFilePerm: 0644,
				Umask:       027,
			}

			ctx, _ := context.WithCancel(context.Background()) //nolint
			daemon.SetSigHandler(sigusrHandler(pipeline, ctx), syscall.SIGUSR1, syscall.SIGUSR2)
			child, err := dmnCtx.Reborn()
			if err != nil {
				log.Err("Mount: error daemonizing application [%v]", err)
				Destroy(1)
			}
			log.Debug("mount: foreground disabled, child = %v", daemon.WasReborn())
			if child == nil {
				defer dmnCtx.Release() // nolint
				setGOConfig()
				go startDynamicProfiler()
				runPipeline(pipeline, ctx)
			}
		} else {
			if options.CPUProfile != "" {
				os.Remove(options.CPUProfile)
				f, err := os.Create(options.CPUProfile)
				if err != nil {
					fmt.Printf("error opening file for cpuprofile [%s]", err)
				}
				defer f.Close()
				if err := pprof.StartCPUProfile(f); err != nil {
					fmt.Printf("failed to start cpuprofile [%s]", err)
				}
				defer pprof.StopCPUProfile()
			}

			setGOConfig()
			go startDynamicProfiler()

			log.Debug("mount: foreground enabled")
			runPipeline(pipeline, context.Background())
			if options.MemProfile != "" {
				os.Remove(options.MemProfile)
				f, err := os.Create(options.MemProfile)
				if err != nil {
					fmt.Printf("error opening file for memprofile [%s]", err)
				}
				defer f.Close()
				runtime.GC()
				if err = pprof.WriteHeapProfile(f); err != nil {
					fmt.Printf("error memory profiling [%s]", err)
				}
			}
		}

	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
}

func runPipeline(pipeline *internal.Pipeline, ctx context.Context) {
	pid := fmt.Sprintf("%v", os.Getpid())
	common.TransferPipe += "_" + pid
	common.PollingPipe += "_" + pid
	log.Debug("Mount::runPipeline : blobfuse2 pid = %v, transfer pipe = %v, polling pipe = %v", pid, common.TransferPipe, common.PollingPipe)

	go startMonitor(os.Getpid())

	pipelineStarted = true
	err := pipeline.Start(ctx)
	if err != nil {
		log.Err("Mount: error unable to start pipeline [%v]", err)
		fmt.Printf("Mount: error unable to start pipeline [%v]", err)
		Destroy(1)
	}

	pipelineStarted = false
	err = pipeline.Stop()
	if err != nil {
		log.Err("Mount: error unable to stop pipeline [%v]", err)
		fmt.Printf("Mount: error unable to stop pipeline [%v]", err)
		Destroy(1)
	}

	_ = log.Destroy()
}

func startMonitor(pid int) {
	if common.EnableMonitoring {
		log.Debug("mount::startMonitor : pid = %v, config-file = %v", pid, options.ConfigFile)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"health-monitor", fmt.Sprintf("--pid=%v", pid), fmt.Sprintf("--config-file=%s", options.ConfigFile)})
		err := rootCmd.Execute()
		if err != nil {
			common.EnableMonitoring = false
			log.Err("mount::startMonitor : [%v]", err)
		}
	}
}

func sigusrHandler(pipeline *internal.Pipeline, ctx context.Context) daemon.SignalHandlerFunc {
	return func(sig os.Signal) error {
		log.Crit("sigusrHandler: Signal %d received", sig)

		var err error
		if sig == syscall.SIGUSR1 {
			log.Crit("sigusrHandler: SIGUSR1 received")
			config.OnConfigChange()
		}

		return err
	}
}

func setGOConfig() {
	// Ensure we always have more than 1 OS thread running goroutines, since there are issues with having just 1.
	isOnlyOne := runtime.GOMAXPROCS(0) == 1
	if isOnlyOne {
		runtime.GOMAXPROCS(2)
	}

	// Golang's default behaviour is to GC when new objects = (100% of) total of objects surviving previous GC.
	// Set it to lower level so that memory if freed up early
	debug.SetGCPercent(70)
}

func startDynamicProfiler() {
	if !options.DynamicProfiler {
		return
	}

	if options.ProfilerIP == "" {
		// By default enable profiler on 127.0.0.1
		options.ProfilerIP = "localhost"
	}

	if options.ProfilerPort == 0 {
		// This is default go profiler port
		options.ProfilerPort = 6060
	}

	connStr := fmt.Sprintf("%s:%d", options.ProfilerIP, options.ProfilerPort)
	log.Info("startDynamicProfiler : Staring profiler on [%s]", connStr)

	// To check dynamic profiling info http://<ip>:<port>/debug/pprof
	// for e.g. for default config use http://localhost:6060/debug/pprof
	// Also CLI based profiler can be used
	// e.g. go tool pprof http://localhost:6060/debug/pprof/heap
	//      go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
	//      go tool pprof http://localhost:6060/debug/pprof/block
	//
	err := http.ListenAndServe(connStr, nil)
	if err != nil {
		log.Err("startDynamicProfiler : Failed to start dynamic profiler [%s]", err.Error())
	}
}

func init() {
	rootCmd.AddCommand(mountCmd)
	pipelineStarted = false

	options = mountOptions{}

	mountCmd.AddCommand(mountListCmd)
	mountCmd.AddCommand(mountAllCmd)

	mountCmd.PersistentFlags().StringVar(&options.ConfigFile, "config-file", "",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml in current directory.")
	_ = mountCmd.MarkPersistentFlagFilename("config-file", "yaml")

	mountCmd.PersistentFlags().BoolVar(&options.SecureConfig, "secure-config", false,
		"Encrypt auto generated config file for each container")

	mountCmd.PersistentFlags().StringVar(&options.PassPhrase, "passphrase", "",
		"Key to decrypt config file. Can also be specified by env-variable BLOBFUSE2_SECURE_CONFIG_PASSPHRASE.\nKey length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")

	mountCmd.PersistentFlags().String("log-level", "LOG_WARNING",
		"Enables logs written to syslog. Set to LOG_WARNING by default. Allowed values are LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG")
	config.BindPFlag("logging.level", mountCmd.PersistentFlags().Lookup("log-level"))
	_ = mountCmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"LOG_OFF", "LOG_CRIT", "LOG_ERR", "LOG_WARNING", "LOG_INFO", "LOG_TRACE", "LOG_DEBUG"}, cobra.ShellCompDirectiveNoFileComp
	})

	mountCmd.PersistentFlags().String("log-file-path",
		common.DefaultLogFilePath, "Configures the path for log files. Default is "+common.DefaultLogFilePath)
	config.BindPFlag("logging.file-path", mountCmd.PersistentFlags().Lookup("log-file-path"))
	_ = mountCmd.MarkPersistentFlagDirname("log-file-path")

	mountCmd.PersistentFlags().Bool("foreground", false, "Mount the system in foreground mode. Default value false.")
	config.BindPFlag("foreground", mountCmd.PersistentFlags().Lookup("foreground"))

	mountCmd.PersistentFlags().Bool("read-only", false, "Mount the system in read only mode. Default value false.")
	config.BindPFlag("read-only", mountCmd.PersistentFlags().Lookup("read-only"))

	mountCmd.PersistentFlags().String("default-working-dir", "", "Default working directory for storing log files and other blobfuse2 information")
	mountCmd.PersistentFlags().Lookup("default-working-dir").Hidden = true
	config.BindPFlag("default-working-dir", mountCmd.PersistentFlags().Lookup("default-working-dir"))
	_ = mountCmd.MarkPersistentFlagDirname("default-working-dir")

	mountCmd.Flags().BoolVar(&options.Streaming, "streaming", false, "Enable Streaming.")
	config.BindPFlag("streaming", mountCmd.Flags().Lookup("streaming"))
	mountCmd.Flags().Lookup("streaming").Hidden = true

	mountCmd.Flags().BoolVar(&options.AttrCache, "use-attr-cache", true, "Use attribute caching.")
	config.BindPFlag("use-attr-cache", mountCmd.Flags().Lookup("use-attr-cache"))
	mountCmd.Flags().Lookup("use-attr-cache").Hidden = true

	mountCmd.Flags().Bool("invalidate-on-sync", true, "Invalidate file/dir on sync/fsync.")
	config.BindPFlag("invalidate-on-sync", mountCmd.Flags().Lookup("invalidate-on-sync"))
	mountCmd.Flags().Lookup("invalidate-on-sync").Hidden = true

	mountCmd.Flags().Bool("pre-mount-validate", true, "Validate blobfuse2 is mounted.")
	config.BindPFlag("pre-mount-validate", mountCmd.Flags().Lookup("pre-mount-validate"))
	mountCmd.Flags().Lookup("pre-mount-validate").Hidden = true

	mountCmd.Flags().Bool("basic-remount-check", true, "Validate blobfuse2 is mounted by reading /etc/mtab.")
	config.BindPFlag("basic-remount-check", mountCmd.Flags().Lookup("basic-remount-check"))
	mountCmd.Flags().Lookup("basic-remount-check").Hidden = true

	mountCmd.PersistentFlags().StringSliceVarP(&options.LibfuseOptions, "o", "o", []string{}, "FUSE options.")
	config.BindPFlag("libfuse-options", mountCmd.PersistentFlags().ShorthandLookup("o"))
	mountCmd.PersistentFlags().ShorthandLookup("o").Hidden = true

	config.AttachToFlagSet(mountCmd.PersistentFlags())
	config.AttachFlagCompletions(mountCmd)
	config.AddConfigChangeEventListener(config.ConfigChangeEventHandlerFunc(OnConfigChange))
}

func Destroy(code int) {
	_ = log.Destroy()
	os.Exit(code)
}
