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
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

// LogConfig : Configuration to be provided to logging infra
type LogFileConfig struct {
	LogFile      string
	LogSize      uint64
	LogFileCount int
	LogLevel     common.LogLevel
	LogTag       string

	currentLogSize uint64
}

type BaseLogger struct {
	channel    chan (string)
	workerDone sync.WaitGroup

	logger        *log.Logger
	logFileHandle io.WriteCloser
	procPID       int

	fileConfig LogFileConfig
}

func newBaseLogger(config LogFileConfig) (*BaseLogger, error) {
	l := &BaseLogger{fileConfig: config}
	err := l.init()
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (l *BaseLogger) GetLoggerObj() *log.Logger {
	return l.logger
}

func (l *BaseLogger) GetType() string {
	return "base"
}

func (l *BaseLogger) GetLogLevel() common.LogLevel {
	return l.fileConfig.LogLevel
}

func (l *BaseLogger) Debug(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_DEBUG() {
		l.logEvent(common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (l *BaseLogger) Trace(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_TRACE() {
		l.logEvent(common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (l *BaseLogger) Info(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_INFO() {
		l.logEvent(common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (l *BaseLogger) Warn(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_WARNING() {
		l.logEvent(common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (l *BaseLogger) Err(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_ERR() {
		l.logEvent(common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (l *BaseLogger) Crit(format string, args ...interface{}) {
	if l.fileConfig.LogLevel >= common.ELogLevel.LOG_CRIT() {
		l.logEvent(common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

func (l *BaseLogger) SetLogFile(name string) error {
	l.fileConfig.LogFile = name
	if l.logFileHandle != nil {
		if name == "stdout" {
			l.logFileHandle = os.Stdout
		} else {
			f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				l.logFileHandle = os.Stdout
				return err
			}
			l.logFileHandle = f
			fi, e := f.Stat()
			if e == nil {
				l.fileConfig.currentLogSize = uint64(fi.Size())
			}
		}
	}
	return nil
}

func (l *BaseLogger) SetMaxLogSize(size int) {
	l.fileConfig.LogSize = uint64(size) * 1024 * 1024
}

func (l *BaseLogger) SetLogFileCount(count int) {
	l.fileConfig.LogFileCount = count
}

func (l *BaseLogger) SetLogLevel(level common.LogLevel) {
	l.fileConfig.LogLevel = level
	l.logEvent(common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (l *BaseLogger) init() error {
	l.procPID = os.Getpid()

	// Set default for config
	if l.fileConfig.LogFile == "" {
		err := l.SetLogFile("stdout")
		if err != nil {
			return err
		}
	}
	if l.fileConfig.LogLevel == common.ELogLevel.INVALID() {
		l.SetLogLevel(common.ELogLevel.LOG_DEBUG())
	}
	if l.fileConfig.LogSize == 0 {
		l.SetMaxLogSize(common.DefaultMaxLogFileSize)
	}
	if l.fileConfig.LogFileCount == 0 {
		l.SetLogFileCount(common.DefaultLogFileCount)
	}

	if l.fileConfig.LogFile == "stdout" || l.fileConfig.LogFile == "" {
		l.logFileHandle = os.Stdout
	} else {
		fi, e := os.Stat(l.fileConfig.LogFile)
		if e == nil {
			l.fileConfig.currentLogSize = uint64(fi.Size())
		}
		var err error
		l.logFileHandle, err = os.OpenFile(l.fileConfig.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			l.logFileHandle = os.Stdout
		}
	}

	// init the log
	l.logger = log.New(l.logFileHandle, "", 0)

	// create channel for the dumper thread and start thread
	l.channel = make(chan string, 100000)
	l.workerDone.Add(1)
	go l.logDumper(1, l.channel)

	return nil
}

func (l *BaseLogger) Destroy() error {
	close(l.channel)
	l.workerDone.Wait()

	if err := l.logFileHandle.Close(); err != nil {
		return err
	}
	return nil
}

// logEvent : Enqueue the log to the channel
func (l *BaseLogger) logEvent(lvl string, format string, args ...interface{}) {
	// Only log if the log level matches the log request
	_, fn, ln, _ := runtime.Caller(3)
	msg := fmt.Sprintf(format, args...)
	msg = fmt.Sprintf("%s : %s[%d][%d] : [%s] %s [%s (%d)]: %s",
		time.Now().Format("Mon Jan _2 15:04:05.000 MST 2006"),
		l.fileConfig.LogTag,
		l.procPID,
		getGoRoutineID(),
		common.MountPath,
		lvl,
		filepath.Base(fn), ln,
		msg)

	l.channel <- msg
}

// Example goroutine 17 [running]: => This method will return 17
func getGoRoutineID() uint64 {
	// Grab up to 64 bytes of the current goroutine’s stack
	b := make([]byte, 64)

	// Write the current goroutine’s stack trace into the byte buffer.
	// Reslices bytes to only those n bytes (b = b[:n]), so you drop any unused capacity at the end of the buffer
	// and work only with the real stack‐trace bytes.
	b = b[:runtime.Stack(b, false)]

	// Strip the literal “goroutine ” prefix
	b = bytes.TrimPrefix(b, []byte("goroutine "))

	// Find the first space (everything before it is the ID)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		return 0
	}

	// Parse those digits into a number
	goRoutineId, err := strconv.ParseUint(string(b[:i]), 10, 64)
	if err != nil {
		return 0
	}

	return goRoutineId
}

// logDumper : logEvent just enqueues an event in the channel, this thread dumps that log to the file
func (l *BaseLogger) logDumper(id int, channel <-chan string) {
	defer l.workerDone.Done()

	for j := range channel {
		l.logger.Println(j)

		l.fileConfig.currentLogSize += (uint64)(len(j))
		if l.fileConfig.currentLogSize > l.fileConfig.LogSize {
			//fmt.Println("Calling logrotate : ", l.fileConfig.currentLogSize, " : ", l.fileConfig.logSize)
			_ = l.LogRotate()
		}
	}
}

func (l *BaseLogger) LogRotate() error {
	//fmt.Println("Log Rotation started")
	if err := l.logFileHandle.Close(); err != nil {
		return err
	}
	// skip if the file is standard output
	if l.fileConfig.LogFile == "stdout" {
		return nil
	}

	var fname string
	var fnameNew string
	fname = fmt.Sprintf("%s.%d", l.fileConfig.LogFile, (l.fileConfig.LogFileCount - 1))

	//fmt.Println("Deleting : ", fname)
	os.Remove(fname)

	for i := l.fileConfig.LogFileCount - 2; i > 0; i-- {
		fname = fmt.Sprintf("%s.%d", l.fileConfig.LogFile, i)
		fnameNew = fmt.Sprintf("%s.%d", l.fileConfig.LogFile, (i + 1))

		// Move each file to next number 8 -> 9, 7 -> 8, 6 -> 7 ...
		//fmt.Println("Renaming : ", fname, " : ", fnameNew)
		_ = os.Rename(fname, fnameNew)
	}

	//fmt.Println("Renaming : ", l.fileConfig.logFile, l.fileConfig.logFile+".1")
	_ = os.Rename(l.fileConfig.LogFile, l.fileConfig.LogFile+".1")

	var err error
	l.logFileHandle, err = os.OpenFile(l.fileConfig.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		l.logFileHandle = os.Stdout
	}

	// init the log
	l.logger.SetOutput(l.logFileHandle)
	l.fileConfig.currentLogSize = 0

	return nil
}
