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
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type OtelLoggerTestSuite struct {
	suite.Suite
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerCreation() {
	assert := assert.New(suite.T())

	// Test creating OTel logger without endpoint (will use default/env)
	config := OtelLoggerConfig{
		LogLevel:       common.ELogLevel.LOG_INFO(),
		LogTag:         "test-blobfuse2",
		LogGoroutineID: false,
	}

	logger, err := newOtelLogger(config)
	assert.NoError(err, "Failed to create OTel logger")
	assert.NotNil(logger, "OTel logger should not be nil")
	assert.Equal("otel", logger.GetType(), "Logger type should be 'otel'")
	assert.Equal(common.ELogLevel.LOG_INFO(), logger.GetLogLevel(), "Log level should match")

	// Cleanup
	err = logger.Destroy()
	assert.NoError(err, "Failed to destroy OTel logger")
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerWithEndpoint() {
	assert := assert.New(suite.T())

	// Test creating OTel logger with explicit endpoint
	config := OtelLoggerConfig{
		Endpoint:       "localhost:4318",
		LogLevel:       common.ELogLevel.LOG_DEBUG(),
		LogTag:         "test-blobfuse2",
		LogGoroutineID: true,
	}

	logger, err := newOtelLogger(config)
	// Note: This may fail if no collector is running, which is expected in unit tests
	// The test validates the configuration and initialization logic
	if err != nil {
		// If initialization fails, that's OK for unit tests as there may be no collector
		suite.T().Logf("OTel logger initialization failed (expected if no collector): %v", err)
	} else {
		assert.NotNil(logger, "OTel logger should not be nil")
		assert.Equal(common.ELogLevel.LOG_DEBUG(), logger.GetLogLevel(), "Log level should match")

		// Cleanup
		err = logger.Destroy()
		assert.NoError(err, "Failed to destroy OTel logger")
	}
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerLoggingLevels() {
	assert := assert.New(suite.T())

	config := OtelLoggerConfig{
		LogLevel:       common.ELogLevel.LOG_DEBUG(),
		LogTag:         "test-blobfuse2",
		LogGoroutineID: false,
	}

	logger, err := newOtelLogger(config)
	if err != nil {
		suite.T().Skipf("Skipping test - OTel logger initialization failed: %v", err)
		return
	}
	defer func() {
		err := logger.Destroy()
		assert.NoError(err)
	}()

	// Test all logging levels - these should not panic
	assert.NotPanics(func() {
		logger.Debug("Debug message: %s", "test")
		logger.Trace("Trace message: %d", 123)
		logger.Info("Info message")
		logger.Warn("Warning message")
		logger.Err("Error message")
		logger.Crit("Critical message")
	}, "Logging should not panic")
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerSetLogLevel() {
	assert := assert.New(suite.T())

	config := OtelLoggerConfig{
		LogLevel:       common.ELogLevel.LOG_INFO(),
		LogTag:         "test-blobfuse2",
		LogGoroutineID: false,
	}

	logger, err := newOtelLogger(config)
	if err != nil {
		suite.T().Skipf("Skipping test - OTel logger initialization failed: %v", err)
		return
	}
	defer func() {
		err := logger.Destroy()
		assert.NoError(err)
	}()

	// Test setting log level
	assert.Equal(common.ELogLevel.LOG_INFO(), logger.GetLogLevel())
	logger.SetLogLevel(common.ELogLevel.LOG_ERR())
	assert.Equal(common.ELogLevel.LOG_ERR(), logger.GetLogLevel())
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerDefaultLogLevel() {
	assert := assert.New(suite.T())

	// Test with invalid log level - should default to LOG_DEBUG
	config := OtelLoggerConfig{
		LogLevel:       common.ELogLevel.INVALID(),
		LogTag:         "test-blobfuse2",
		LogGoroutineID: false,
	}

	logger, err := newOtelLogger(config)
	if err != nil {
		suite.T().Skipf("Skipping test - OTel logger initialization failed: %v", err)
		return
	}
	defer func() {
		err := logger.Destroy()
		assert.NoError(err)
	}()

	assert.Equal(common.ELogLevel.LOG_DEBUG(), logger.GetLogLevel(), "Should default to LOG_DEBUG")
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerInterface() {
	assert := assert.New(suite.T())

	config := OtelLoggerConfig{
		LogLevel: common.ELogLevel.LOG_INFO(),
		LogTag:   "test-blobfuse2",
	}

	logger, err := newOtelLogger(config)
	if err != nil {
		suite.T().Skipf("Skipping test - OTel logger initialization failed: %v", err)
		return
	}
	defer func() {
		err := logger.Destroy()
		assert.NoError(err)
	}()

	// Test that OtelLogger implements Logger interface
	var _ Logger = logger

	// Test GetLoggerObj
	stdLogger := logger.GetLoggerObj()
	assert.NotNil(stdLogger, "Standard logger should not be nil")

	// Test GetType
	assert.Equal("otel", logger.GetType())

	// Test LogRotate (should be no-op for OTel logger)
	err = logger.LogRotate()
	assert.NoError(err, "LogRotate should be no-op for OTel logger")

	// Test SetLogFile (should be no-op for OTel logger)
	err = logger.SetLogFile("dummy.log")
	assert.NoError(err, "SetLogFile should be no-op for OTel logger")

	// Test SetMaxLogSize (should be no-op for OTel logger)
	assert.NotPanics(func() {
		logger.SetMaxLogSize(100)
	}, "SetMaxLogSize should not panic")

	// Test SetLogFileCount (should be no-op for OTel logger)
	assert.NotPanics(func() {
		logger.SetLogFileCount(10)
	}, "SetLogFileCount should not panic")
}

func (suite *OtelLoggerTestSuite) TestOtelLoggerViaFactory() {
	assert := assert.New(suite.T())

	cfg := common.LogConfig{
		Level:        common.ELogLevel.LOG_INFO(),
		Tag:          "test-blobfuse2",
		OtelEndpoint: "localhost:4318",
	}

	err := SetDefaultLogger("otel", cfg)
	// May fail if no collector is running, which is OK for unit tests
	if err != nil {
		suite.T().Logf("OTel logger initialization via factory failed (expected if no collector): %v", err)
	} else {
		assert.Equal("otel", GetType(), "Logger type should be 'otel'")

		// Test logging through global functions
		assert.NotPanics(func() {
			Info("Test info message")
			Warn("Test warning message")
			Err("Test error message")
		}, "Global logging functions should not panic")

		// Cleanup - may fail if no collector is running, which is OK
		err = Destroy()
		if err != nil {
			suite.T().Logf("OTel logger cleanup failed (expected if no collector): %v", err)
		}
	}
}

func TestOtelLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(OtelLoggerTestSuite))
}
