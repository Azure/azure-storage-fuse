// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package log

import (
	"log"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

type SilentLogger struct {
}

func (*SilentLogger) GetLoggerObj() *log.Logger {
	return nil
}

func (*SilentLogger) GetType() string {
	return "silent"
}

func (*SilentLogger) GetLogLevel() common.LogLevel {
	return common.ELogLevel.LOG_OFF()
}

func (*SilentLogger) Debug(_ string, _ ...any) {

}

func (*SilentLogger) Trace(_ string, _ ...any) {

}

func (*SilentLogger) Info(_ string, _ ...any) {

}

func (*SilentLogger) Warn(_ string, _ ...any) {

}

func (*SilentLogger) Err(_ string, _ ...any) {

}

func (*SilentLogger) Crit(_ string, _ ...any) {

}

func (*SilentLogger) LogRotate() error {
	return nil
}

func (*SilentLogger) Destroy() error {
	return nil
}

func (*SilentLogger) SetLogFile(_ string) error {
	return nil
}

func (*SilentLogger) SetMaxLogSize(_ int) {
}

func (*SilentLogger) SetLogFileCount(_ int) {

}

func (*SilentLogger) SetLogLevel(_ common.LogLevel) {

}
