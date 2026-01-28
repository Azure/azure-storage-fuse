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
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
)

// OtelLogger : OpenTelemetry-based logger that exports logs via OTLP
type OtelLogger struct {
	loggerProvider *sdklog.LoggerProvider
	logger         otellog.Logger
	stdLogger      *log.Logger
	procPID        int
	logLevel       common.LogLevel
	logTag         string
	logGoroutineID bool
}

// OtelLoggerConfig : Configuration for OpenTelemetry logger
type OtelLoggerConfig struct {
	Endpoint       string // OTLP endpoint (e.g., "http://localhost:4318")
	LogLevel       common.LogLevel
	LogTag         string
	LogGoroutineID bool
}

func newOtelLogger(config OtelLoggerConfig) (*OtelLogger, error) {
	l := &OtelLogger{
		procPID:        os.Getpid(),
		logLevel:       config.LogLevel,
		logTag:         config.LogTag,
		logGoroutineID: config.LogGoroutineID,
	}

	// Set default log level if not provided
	if l.logLevel == common.ELogLevel.INVALID() {
		l.logLevel = common.ELogLevel.LOG_DEBUG()
	}

	// Create OTLP HTTP exporter
	ctx := context.Background()
	
	var exporter *otlploghttp.Exporter
	var err error
	
	if config.Endpoint != "" {
		exporter, err = otlploghttp.New(ctx,
			otlploghttp.WithEndpoint(config.Endpoint),
			otlploghttp.WithInsecure(),
		)
	} else {
		// Use default endpoint from environment variables (OTEL_EXPORTER_OTLP_ENDPOINT)
		exporter, err = otlploghttp.New(ctx)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create logger provider with batch processor
	l.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	// Create logger instance
	l.logger = l.loggerProvider.Logger(config.LogTag)

	// Create a standard logger for compatibility
	l.stdLogger = log.New(os.Stdout, "", 0)

	return l, nil
}

func (l *OtelLogger) GetLoggerObj() *log.Logger {
	return l.stdLogger
}

func (l *OtelLogger) GetType() string {
	return "otel"
}

func (l *OtelLogger) GetLogLevel() common.LogLevel {
	return l.logLevel
}

func (l *OtelLogger) Debug(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_DEBUG() {
		l.logEvent(otellog.SeverityDebug, common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (l *OtelLogger) Trace(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_TRACE() {
		l.logEvent(otellog.SeverityTrace, common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (l *OtelLogger) Info(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_INFO() {
		l.logEvent(otellog.SeverityInfo, common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (l *OtelLogger) Warn(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_WARNING() {
		l.logEvent(otellog.SeverityWarn, common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (l *OtelLogger) Err(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_ERR() {
		l.logEvent(otellog.SeverityError, common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (l *OtelLogger) Crit(format string, args ...any) {
	if l.logLevel >= common.ELogLevel.LOG_CRIT() {
		l.logEvent(otellog.SeverityFatal, common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

func (l *OtelLogger) SetLogFile(name string) error {
	// Not applicable for OTLP logger - logs are sent to remote endpoint
	return nil
}

func (l *OtelLogger) SetMaxLogSize(size int) {
	// Not applicable for OTLP logger - no local file rotation
}

func (l *OtelLogger) SetLogFileCount(count int) {
	// Not applicable for OTLP logger - no local file rotation
}

func (l *OtelLogger) SetLogLevel(level common.LogLevel) {
	l.logLevel = level
	l.logEvent(otellog.SeverityFatal, common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (l *OtelLogger) Destroy() error {
	if l.loggerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return l.loggerProvider.Shutdown(ctx)
	}
	return nil
}

func (l *OtelLogger) LogRotate() error {
	// Not applicable for OTLP logger
	return nil
}

// logEvent : Emit log record via OpenTelemetry
func (l *OtelLogger) logEvent(severity otellog.Severity, lvl string, format string, args ...any) {
	_, fn, ln, _ := runtime.Caller(3)
	msg := fmt.Sprintf(format, args...)

	ctx := context.Background()
	
	// Build attributes
	attrs := []otellog.KeyValue{
		otellog.String("level", lvl),
		otellog.String("source.file", filepath.Base(fn)),
		otellog.Int("source.line", ln),
		otellog.Int("pid", l.procPID),
		otellog.String("tag", l.logTag),
		otellog.String("mount_path", common.MountPath),
	}

	if l.logGoroutineID {
		attrs = append(attrs, otellog.Int("goroutine_id", int(common.GetGoroutineID())))
	}

	// Create log record
	var record otellog.Record
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(msg))
	record.SetSeverity(severity)
	record.AddAttributes(attrs...)

	// Emit the log record
	l.logger.Emit(ctx, record)

	// Also write to stdout for immediate visibility during debugging
	timestamp := time.Now().Format(common.UnixDateMillis)
	if l.logGoroutineID {
		l.stdLogger.Printf("%s : %s[%d][%d] : [%s] %s [%s (%d)]: %s",
			timestamp, l.logTag, l.procPID, common.GetGoroutineID(),
			common.MountPath, lvl, filepath.Base(fn), ln, msg)
	} else {
		l.stdLogger.Printf("%s : %s[%d] : [%s] %s [%s (%d)]: %s",
			timestamp, l.logTag, l.procPID,
			common.MountPath, lvl, filepath.Base(fn), ln, msg)
	}
}
