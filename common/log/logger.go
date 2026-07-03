/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
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

package log

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	Debug(format string, args ...any)
	Trace(format string, args ...any)
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Err(format string, args ...any)
	Crit(format string, args ...any)
	LogRotate() error
}

// newLogger : Method to create Logger object
func NewLogger(name string, config common.LogConfig) (Logger, error) {
	timeTracker = config.TimeTracker

	if len(strings.TrimSpace(config.Tag)) == 0 {
		config.Tag = common.FileSystemName
	}

	switch name {
	case "base":
		baseLogger, err := newBaseLogger(LogFileConfig{
			LogFile:        config.FilePath,
			LogLevel:       config.Level,
			LogSize:        config.MaxFileSize * 1024 * 1024,
			LogFileCount:   int(config.FileCount),
			LogTag:         config.Tag,
			LogGoroutineID: config.LogGoroutineID,
		})
		if err != nil {
			return nil, err
		}
		return baseLogger, nil
	case "silent":
		silentLogger := &SilentLogger{}
		return silentLogger, nil
	case "", "default", "syslog":
		sysLogger, err := newSysLogger(config.Level, config.Tag, config.LogGoroutineID)
		if err != nil {
			if err == ErrNoSyslogService {
				// Syslog service does not exists on this system
				// fallback to file based logging.
				return NewLogger("base", config)
			}
			return nil, err
		}
		return sysLogger, nil
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
// This should only be called from the main function.
func Destroy() error {
	if logObj != nil {
		return logObj.Destroy()
	}
	return fmt.Errorf("Logger is not initialized")
}

// ------------------ Public methods for logging events ------------------

// Debug : Debug message logging
func Debug(msg string, args ...any) {
	logObj.Debug(msg, args...)
}

// Trace : Trace message logging
func Trace(msg string, args ...any) {
	logObj.Trace(msg, args...)
}

// Info : Info message logging
func Info(msg string, args ...any) {
	logObj.Info(msg, args...)
}

// Warn : Warning message logging
func Warn(msg string, args ...any) {
	logObj.Warn(msg, args...)
}

// Err : Error message logging
func Err(msg string, args ...any) {
	logObj.Err(msg, args...)
}

// Crit : Critical message logging
func Crit(msg string, args ...any) {
	logObj.Crit(msg, args...)
}

// LogRotate : Rotate the log files explicitly
func LogRotate() error {
	return logObj.LogRotate()
}

// rotateHooksMu guards rotateHooks against concurrent registration and invocation.
var rotateHooksMu sync.Mutex
var rotateHooks []func()

// registerLogRotateHook registers fn to be invoked after each successful log rotation performed by the
// underlying logger. Useful for callers that hold a file descriptor to the log file (e.g. runtime
// crash output) and need to re-open it after rotation. Hooks are invoked sequentially and must be
// cheap and non-blocking; they run inline on the rotation path.
func registerLogRotateHook(fn func()) {
	if fn == nil {
		return
	}
	rotateHooksMu.Lock()
	rotateHooks = append(rotateHooks, fn)
	rotateHooksMu.Unlock()
}

// invokeRotateHooks runs all registered post-rotation hooks. Called by logger implementations
// after a successful rotation.
func invokeRotateHooks() {
	rotateHooksMu.Lock()
	hooks := append([]func(){}, rotateHooks...)
	rotateHooksMu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}

// SetupCrashOutput wires the Go runtime crash output (panics, fatal errors from any goroutine) to the
// Blobfuse2 log file in addition to stderr (which the daemon library redirects to the per-mount
// .trace file). Supported logger modes:
//
//   - "base"  -> writes to logFilePath (the configured log file). Skipped when logFilePath is empty
//     or "stdout".
//   - "" / "default" / "syslog" -> writes to common.SyslogFilePath (the rsyslog sink for blobfuse2
//     messages, declared in setup/11-blobfuse2.conf).
//
// A no-op for the "silent" logger, unknown logger types, and base-with-stdout -- in those cases no
// rotate hook is registered and no SIGHUP handler is installed.
//
// The crash output fd is kept attached to the live log file across rotations via two mechanisms
// selected per logger mode:
//  1. "base"   -> BaseLogger's in-process size-based rotation invokes the registered rotate hook.
//     No SIGHUP handler is installed because BaseLogger owns its file and does not participate in
//     external rotation.
//  2. syslog family -> external rotators (logrotate's postrotate, the AKS Blob CSI driver, ...)
//     signal the process via SIGHUP after rotating /var/log/blobfuse2.log aside. SysLogger has no
//     in-process rotation, so SIGHUP is the only trigger.
func SetupCrashOutput(loggerType, logFilePath string) {
	// Skip the whole setup (crash-output fd, rotate hook, SIGHUP goroutine) when the logger
	// configuration has no meaningful file target to mirror crash dumps to.
	if crashOutputTarget(loggerType, logFilePath) == "" {
		return
	}

	setCrashOutput(loggerType, logFilePath)

	// Re-attach the crash output fd after BaseLogger's in-process size-based rotation. The rename
	// moves the original file aside while our dup'd fd stays pinned to the old inode, so we must
	// re-open the live file. Registered unconditionally: it is a no-op for logger types whose
	// LogRotate() implementations never invoke the hook (SysLogger, SilentLogger).
	registerLogRotateHook(func() {
		setCrashOutput(loggerType, logFilePath)
	})

	// SIGHUP is only meaningful for the syslog family: SysLogger has no in-process rotation, so
	// external rotators (logrotate + postrotate, AKS Blob CSI driver, ...) are the only way to
	// know that /var/log/blobfuse2.log has been rotated. For "base" mode the in-process hook above
	// already covers rotation and we don't want to hijack SIGHUP for other consumers.
	if isSyslogFamily(loggerType) {
		installCrashSighupHandler(loggerType, logFilePath)
	}
}

// isSyslogFamily reports whether loggerType routes crash output to common.SyslogFilePath and
// therefore relies on external rotators + SIGHUP for rotation notification.
func isSyslogFamily(loggerType string) bool {
	switch loggerType {
	case "", "default", "syslog":
		return true
	default:
		return false
	}
}

// crashOutputTarget returns the filesystem path where runtime crash dumps should be mirrored for
// the given logger configuration, or "" if the configuration does not support mirroring crash
// dumps to a log file (silent logger, stdout base logger, unknown types, ...).
func crashOutputTarget(loggerType, logFilePath string) string {
	switch loggerType {
	case "base":
		// BaseLogger may be configured with "stdout" or no file at all; nothing useful to also write to.
		if logFilePath == "" || logFilePath == "stdout" {
			return ""
		}
		return logFilePath
	case "", "default", "syslog":
		// syslog can't be redirected to via a file descriptor, so target the rsyslog sink for blobfuse2 messages.
		return common.SyslogFilePath
	default:
		// "silent" or unknown logger.
		return ""
	}
}

func setCrashOutput(loggerType, logFilePath string) {
	// Panic-safe: the SIGHUP handler goroutine outlives log.Destroy(), which closes BaseLogger's
	// channel. A subsequent Warn() from here would then panic on send-to-closed-channel. Swallow
	// any panic so a SIGHUP during teardown (or any other edge case) cannot crash the process --
	// the .trace file still captures runtime panics via stderr redirection.
	defer func() { _ = recover() }()

	crashFilePath := crashOutputTarget(loggerType, logFilePath)
	if crashFilePath == "" {
		return
	}

	// Open without O_CREATE: in base mode BaseLogger has already created the file; in syslog mode rsyslog owns
	// the file and must have created it first. If the file is missing (e.g. rsyslog not restarted yet on a fresh
	// install), let it fail -- the per-mount .trace file still captures the panic via stderr redirection.
	f, err := os.OpenFile(crashFilePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		Warn("log: failed to open %s for crash output [%s]", crashFilePath, err.Error())
		return
	}
	// SetCrashOutput dups the fd, so the file handle can be closed immediately.
	defer f.Close()

	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		Warn("log: failed to set crash output to %s [%s]", crashFilePath, err.Error())
	}
}

// sighupOnce ensures the SIGHUP listener goroutine is only started once per process.
var sighupOnce sync.Once

// sighupCh is the channel signal.Notify writes to; retained at package scope so tests can shut
// the listener goroutine down cleanly (production never reads it back).
var sighupCh chan os.Signal

// sighupInstalled reports whether the SIGHUP listener has been installed. Read from tests via
// sync/atomic; production code has no reason to read it.
var sighupInstalled atomic.Bool

// sighupHandled counts the number of SIGHUPs the listener goroutine has processed. Used by tests
// to verify that the crash-output handler actually ran (as opposed to merely observing that the
// signal was delivered to the process).
var sighupHandled atomic.Uint64

// installCrashSighupHandler arranges for the crash output fd to be re-attached when the process receives
// SIGHUP. Intended for the syslog family only: SysLogger has no in-process rotation, so external
// rotators (logrotate's postrotate, the AKS Blob CSI driver's rotation hook, etc.) are the only
// way to be notified that /var/log/blobfuse2.log has been rotated aside. Guarded by sync.Once so the
// listener is installed at most once per process.
func installCrashSighupHandler(loggerType, logFilePath string) {
	sighupOnce.Do(func() {
		sighupCh = make(chan os.Signal, 1)
		signal.Notify(sighupCh, syscall.SIGHUP)
		sighupInstalled.Store(true)
		go func() {
			for range sighupCh {
				setCrashOutput(loggerType, logFilePath)
				sighupHandled.Add(1)
			}
		}()
	})
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
