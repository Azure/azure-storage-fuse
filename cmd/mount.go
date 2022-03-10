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
	"blobfuse2/common/exectime"
	"blobfuse2/common/log"
	"blobfuse2/internal"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"

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

	Logging           LogOptions `config:"logging"`
	Components        []string   `config:"components"`
	Foreground        bool       `config:"foreground"`
	DefaultWorkingDir string     `config:"default-working-dir"`
	Debug             bool       `config:"debug"`
	DebugPath         string     `config:"debug-path"`
	CPUProfile        string     `config:"cpu-profile"`
	MemProfile        string     `config:"mem-profile"`
	PassPhrase        string     `config:"passphrase"`
	SecureConfig      bool       `config:"secure-config"`
}

var options mountOptions
var pipelineStarted bool

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

	// A user provided value of 0 doesnt make sense for MaxLogFileSize or LogFileCount.
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
	Use:        "mount [path]",
	Short:      "Mounts the azure container as a filesystem",
	Long:       "Mounts the azure container as a filesystem",
	SuggestFor: []string{"mnt", "mout"},
	Args:       cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Parent().Run(cmd.Parent(), args)

		options.MountPath = args[0]
		parseConfig()

		err := config.Unmarshal(&options)
		if err != nil {
			fmt.Printf("Init error config unmarshall [%s]", err)
			os.Exit(1)
		}

		if !config.IsSet("config-file") {
			options.ConfigFile = "config.yaml"
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

		log.Crit("Starting Blobfuse2 Mount : %s on (%s)", common.Blobfuse2Version, common.GetCurrentDistro())
		log.Crit("Logging level set to : %s", logLevel.String())
		pipeline, err := internal.NewPipeline(options.Components)
		if err != nil {
			log.Err("Mount: error initiliazing new pipeline [%v]", err)
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

			ctx, _ := context.WithCancel(context.Background())
			daemon.SetSigHandler(sigusrHandler(pipeline, ctx), syscall.SIGUSR1, syscall.SIGUSR2)
			child, err := dmnCtx.Reborn()
			if err != nil {
				log.Err("Mount: error daemonizing application [%v]", err)
				Destroy(1)
			}
			if child == nil {
				defer dmnCtx.Release()
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

	log.Destroy()
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

func init() {
	rootCmd.AddCommand(mountCmd)
	pipelineStarted = false

	options = mountOptions{}

	mountCmd.AddCommand(mountListCmd)
	mountCmd.AddCommand(mountAllCmd)

	mountCmd.PersistentFlags().StringVar(&options.ConfigFile, "config-file", "config.yaml",
		"Configures the path for the file where the account credentials are provided. Default is config.yaml")
	mountCmd.MarkPersistentFlagFilename("config-file", "yaml")

	mountCmd.PersistentFlags().BoolVar(&options.SecureConfig, "secure-config", false,
		"Encrypt auto generated config file for each container")

	mountCmd.PersistentFlags().StringVar(&options.PassPhrase, "passphrase", "",
		"Key to decrypt config file. Can also be specified by env-variable BLOBFUSE2_SECURE_CONFIG_PASSPHRASE.\nKey length shall be 16 (AES-128), 24 (AES-192), or 32 (AES-256) bytes in length.")

	mountCmd.PersistentFlags().String("log-level", "LOG_WARNING",
		"Enables logs written to syslog. Set to LOG_WARNING by default. Allowed values are LOG_OFF|LOG_CRIT|LOG_ERR|LOG_WARNING|LOG_INFO|LOG_DEBUG")
	config.BindPFlag("logging.level", mountCmd.PersistentFlags().Lookup("log-level"))
	mountCmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"LOG_OFF", "LOG_CRIT", "LOG_ERR", "LOG_WARNING", "LOG_INFO", "LOG_TRACE", "LOG_DEBUG"}, cobra.ShellCompDirectiveNoFileComp
	})

	mountCmd.PersistentFlags().String("log-file-path",
		common.DefaultLogFilePath, "Configures the path for log files. Default is "+common.DefaultLogFilePath)
	config.BindPFlag("logging.file-path", mountCmd.PersistentFlags().Lookup("log-file-path"))
	mountCmd.MarkPersistentFlagDirname("log-file-path")

	mountCmd.PersistentFlags().Bool("foreground", false, "Mount the system in foreground mode. Default value false.")
	config.BindPFlag("foreground", mountCmd.PersistentFlags().Lookup("foreground"))

	mountCmd.PersistentFlags().Bool("read-only", false, "Mount the system in read only mode. Default value false.")
	config.BindPFlag("read-only", mountCmd.PersistentFlags().Lookup("read-only"))

	mountCmd.PersistentFlags().String("default-working-dir", "", "Default working directory for storing log files and other blobfuse2 information")
	mountCmd.PersistentFlags().Lookup("default-working-dir").Hidden = true
	config.BindPFlag("default-working-dir", mountCmd.PersistentFlags().Lookup("default-working-dir"))
	mountCmd.MarkPersistentFlagDirname("default-working-dir")

	config.AttachToFlagSet(mountCmd.PersistentFlags())
	config.AttachFlagCompletions(mountCmd)
	config.AddConfigChangeEventListener(config.ConfigChangeEventHandlerFunc(OnConfigChange))
}

func Destroy(code int) {
	log.Destroy()
	os.Exit(code)
}
