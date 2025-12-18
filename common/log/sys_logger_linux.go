//go:build linux

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
	"log/syslog"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

func (l *SysLogger) init() error {
	// Configure logger to write to the syslog. You could do this in init(), too.
	logwriter, e := syslog.New(getSyslogLevel(l.level), l.tag)

	if e != nil {
		return ErrNoSyslogService
	}

	l.logger = log.New(logwriter, "", 0)
	if l.logger == nil {
		return errors.New("unable to create logger object")
	}

	return nil
}

// Convert our log levels to standard syslog levels
func getSyslogLevel(lvl common.LogLevel) syslog.Priority {
	// By default keep the log level to log warning and match the rest
	switch lvl {
	case common.ELogLevel.LOG_CRIT():
		return syslog.LOG_CRIT
	case common.ELogLevel.LOG_DEBUG():
		return syslog.LOG_DEBUG
	case common.ELogLevel.LOG_ERR():
		return syslog.LOG_ERR
	case common.ELogLevel.LOG_INFO():
		return syslog.LOG_INFO
	case common.ELogLevel.LOG_TRACE():
		return syslog.LOG_DEBUG
	default:
		return syslog.LOG_WARNING
	}
}
