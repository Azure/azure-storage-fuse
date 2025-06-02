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

package log

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

// Logger : Interface to define a generic Logger. Implement this to create your new logging lib
type Logger interface {
	GetLoggerObj() *log.Logger

	SetLogFile(name string) error
	SetMaxLogSize(size int)
	SetLogFileCount(count int)
	SetLogLevel(level common.LogLevel)

	Destroy() error

	GetType() string
	GetLogLevel() common.LogLevel
	Debug(format string, args ...interface{})
	Trace(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Err(format string, args ...interface{})
	Crit(format string, args ...interface{})
	LogRotate() error
}

// newLogger : Method to create Logger object
func NewLogger(name string, config common.LogConfig) (Logger, error) {
	timeTracker = config.TimeTracker

	if len(strings.TrimSpace(config.Tag)) == 0 {
		config.Tag = common.FileSystemName
	}

	if name == "base" {
		baseLogger, err := newBaseLogger(LogFileConfig{
			LogFile:      config.FilePath,
			LogLevel:     config.Level,
			LogSize:      config.MaxFileSize * 1024 * 1024,
			LogFileCount: int(config.FileCount),
			LogTag:       config.Tag,
		})
		if err != nil {
			return nil, err
		}
		return baseLogger, nil
	} else if name == "silent" {
		silentLogger := &SilentLogger{}
		return silentLogger, nil
	} else if name == "" || name == "default" || name == "syslog" {
		// Try platform-specific system logger first
		return newSystemLogger(config)
	} else if name == "winevent" {
		// Windows Event Log logger (Windows only)
		return newPlatformEventLogger(config)
	}
	return nil, errors.New("invalid logger type")
}

var logObj Logger
var timeTracker bool

// ------------------ Public methods to use logging lib ------------------

func GetLoggerObj() *log.Logger {
	return logObj.GetLoggerObj()
}

func GetLogLevel() common.LogLevel {
	return logObj.GetLogLevel()
}

func GetType() string { // TODO: Should I make an enum for this instead?
	return logObj.GetType()
}

func TimeTracker() bool {
	return timeTracker
}

func SetDefaultLogger(name string, config common.LogConfig) error {
	var err error
	logObj, err = NewLogger(name, config)
	if err != nil || logObj == nil {
		return err
	}
	return nil
}

func SetConfig(config common.LogConfig) error {
	timeTracker = config.TimeTracker

	if logObj != nil {
		if config.FilePath != "" {
			err := logObj.SetLogFile(config.FilePath)
			if err != nil {
				return err
			}
		}
		if config.Level != common.ELogLevel.INVALID() {
			logObj.SetLogLevel(config.Level)
		}
		if config.MaxFileSize != 0 {
			logObj.SetMaxLogSize(int(config.MaxFileSize))
		}
		if config.FileCount != 0 {
			logObj.SetLogFileCount(int(config.FileCount))
		}
	}

	return nil
}

func SetLogFile(name string) error {
	if logObj != nil {
		return logObj.SetLogFile(name)
	}
	return nil
}

func SetMaxLogSize(size int) {
	if logObj != nil {
		logObj.SetMaxLogSize(size)
	}
}

func SetLogFileCount(count int) {
	if logObj != nil {
		logObj.SetLogFileCount(count)
	}
}

// SetLogLevel : Reset the log level
func SetLogLevel(lvl common.LogLevel) {
	if logObj != nil {
		logObj.SetLogLevel(lvl)
		Crit("SetLogLevel : Log level reset to : %s", lvl.String())
	}
}

// Destroy : DeInitialize the logging library
func Destroy() error {
	return logObj.Destroy()
}

// ------------------ Public methods for logging events ------------------

// Debug : Debug message logging
func Debug(msg string, args ...interface{}) {
	logObj.Debug(msg, args...)
}

// Trace : Trace message logging
func Trace(msg string, args ...interface{}) {
	logObj.Trace(msg, args...)
}

// Info : Info message logging
func Info(msg string, args ...interface{}) {
	logObj.Info(msg, args...)
}

// Warn : Warning message logging
func Warn(msg string, args ...interface{}) {
	logObj.Warn(msg, args...)
}

// Err : Error message logging
func Err(msg string, args ...interface{}) {
	logObj.Err(msg, args...)
}

// Crit : Critical message logging
func Crit(msg string, args ...interface{}) {
	logObj.Crit(msg, args...)
}

// LogRotate : Rotate the log files explicitly
func LogRotate() error {
	return logObj.LogRotate()
}

func init() {
	logObj, _ = NewLogger("syslog", common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	})
}

// TimeTracker : Dump time taken by a call
func TimeTrack(start time.Time, location string, name string) {
	if timeTracker {
		elapsed := time.Since(start)
		logObj.Crit("TimeTracker :: [%s] %s => %s", location, name, elapsed)
	}
}

// TimeTracker : Dump time taken by a call
func TimeTrackDiff(diff time.Duration, location string, name string) {
	if timeTracker {
		logObj.Crit("TimeTracker :: [%s] %s => %s", location, name, diff)
	}
}
