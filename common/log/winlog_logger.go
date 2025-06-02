//go:build windows

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
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

type WinEventLogger struct {
	level     common.LogLevel
	tag       string
	logger    *log.Logger
	eventLog  syscall.Handle
}

var (
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	procRegisterEventSource = advapi32.NewProc("RegisterEventSourceW")
	procReportEvent         = advapi32.NewProc("ReportEventW")
	procDeregisterEventSource = advapi32.NewProc("DeregisterEventSource")
)

const (
	EVENTLOG_SUCCESS          = 0
	EVENTLOG_ERROR_TYPE       = 1
	EVENTLOG_WARNING_TYPE     = 2
	EVENTLOG_INFORMATION_TYPE = 4
)

var NoWinEventService = errors.New("failed to create Windows Event Log object")

func newWinEventLogger(lvl common.LogLevel, tag string) (*WinEventLogger, error) {
	l := &WinEventLogger{
		level: lvl,
		tag:   tag,
	}
	err := l.init()
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (l *WinEventLogger) GetLoggerObj() *log.Logger {
	return l.logger
}

func (l *WinEventLogger) SetLogLevel(level common.LogLevel) {
	l.level = level
	l.write(common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (l *WinEventLogger) GetType() string {
	return "winevent"
}

func (l *WinEventLogger) GetLogLevel() common.LogLevel {
	return l.level
}

func (l *WinEventLogger) init() error {
	// Register with Windows Event Log
	tagPtr, err := syscall.UTF16PtrFromString(l.tag)
	if err != nil {
		return NoWinEventService
	}

	ret, _, _ := procRegisterEventSource.Call(0, uintptr(unsafe.Pointer(tagPtr)))
	if ret == 0 {
		return NoWinEventService
	}

	l.eventLog = syscall.Handle(ret)
	
	// Create a standard logger that writes to a custom writer
	l.logger = log.New(&winEventWriter{eventLog: l.eventLog}, "", 0)
	if l.logger == nil {
		return errors.New("unable to create logger object")
	}

	return nil
}

type winEventWriter struct {
	eventLog syscall.Handle
}

func (w *winEventWriter) Write(p []byte) (n int, err error) {
	message := string(p)
	msgPtr, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return 0, err
	}

	// Write to Windows Event Log
	ret, _, _ := procReportEvent.Call(
		uintptr(w.eventLog),
		uintptr(EVENTLOG_INFORMATION_TYPE),
		0, // Category
		0, // EventID
		0, // UserSID
		1, // NumStrings
		0, // DataSize
		uintptr(unsafe.Pointer(&msgPtr)),
		0, // RawData
	)

	if ret == 0 {
		return 0, errors.New("failed to write to Windows Event Log")
	}

	return len(p), nil
}

func (l *WinEventLogger) write(lvl string, format string, args ...interface{}) {
	_, fn, ln, _ := runtime.Caller(3)
	msg := fmt.Sprintf(format, args...)
	l.logger.Print("[", common.MountPath, "] ", lvl, " [", filepath.Base(fn), " (", ln, ")]: ", msg)
}

func (l *WinEventLogger) Debug(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_DEBUG() {
		l.write(common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (l *WinEventLogger) Trace(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_TRACE() {
		l.write(common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (l *WinEventLogger) Info(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_INFO() {
		l.write(common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (l *WinEventLogger) Warn(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_WARNING() {
		l.write(common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (l *WinEventLogger) Err(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_ERR() {
		l.write(common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (l *WinEventLogger) Crit(format string, args ...interface{}) {
	if l.level >= common.ELogLevel.LOG_CRIT() {
		l.write(common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

func (l *WinEventLogger) SetLogFile(name string) error {
	return nil
}

func (l *WinEventLogger) SetMaxLogSize(size int) {
}

func (l *WinEventLogger) SetLogFileCount(count int) {
}

func (l *WinEventLogger) Destroy() error {
	if l.eventLog != 0 {
		procDeregisterEventSource.Call(uintptr(l.eventLog))
		l.eventLog = 0
	}
	return nil
}

func (l *WinEventLogger) LogRotate() error {
	return nil
}