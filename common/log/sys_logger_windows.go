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

package log

import (
	"blobfuse2/common"
	"log"
)

type SysLogger struct {
}

func newSysLogger(lvl common.LogLevel) (*SysLogger, error) {
	return nil, nil
}

func (l *SysLogger) GetLoggerObj() *log.Logger {
	return nil
}

func (l *SysLogger) SetLogLevel(level common.LogLevel) {
}

func (l *SysLogger) GetType() string {
	return "syslog"
}

func (l *SysLogger) GetLogLevel() common.LogLevel {
	return 0
}

func (l *SysLogger) init() error {
	return nil
}
func (l *SysLogger) Debug(format string, args ...interface{}) {
}

func (l *SysLogger) Trace(format string, args ...interface{}) {
}

func (l *SysLogger) Info(format string, args ...interface{}) {
}

func (l *SysLogger) Warn(format string, args ...interface{}) {
}

func (l *SysLogger) Err(format string, args ...interface{}) {
}

func (l *SysLogger) Crit(format string, args ...interface{}) {
}

// Methods not needed for syslog based logging
func (l *SysLogger) SetLogFile(name string) error {
	return nil
}

func (l *SysLogger) SetMaxLogSize(size int) {
	return
}

func (l *SysLogger) SetLogFileCount(count int) {
	return
}

func (l *SysLogger) Destroy() error {
	return nil
}

func (l *SysLogger) LogRotate() error {
	return nil
}
