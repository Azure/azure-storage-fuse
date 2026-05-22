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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ---- original suite --------------------------------------------------------

type LoggerTestSuite struct {
	suite.Suite
	log_rotate_test_count int
}

func fastTestDebug(lts *LoggerTestSuite) {
	for i := 0; i < lts.log_rotate_test_count; i++ {
		Debug("hello %d", i)
	}
}

func fastTestCrit(lts *LoggerTestSuite) {
	for i := 0; i < lts.log_rotate_test_count; i++ {
		Crit("hello %d", i)
	}
}

func simpleTest(lts *LoggerTestSuite) {
	Crit("Running Simple Test")
	for l := range 3 {
		switch l {
		case 0:
			SetLogLevel(common.ELogLevel.LOG_DEBUG())
		case 1:
			SetLogLevel(common.ELogLevel.LOG_INFO())
		case 2:
			SetLogLevel(common.ELogLevel.LOG_WARNING())
		default:
			SetLogLevel(common.ELogLevel.LOG_ERR())
		}

		Debug("hello %d", l)
		Trace("hello %d", l)
		Info("hello %d", l)
		Warn("hello %d", l)
		Err("hello %d", l)
		Crit("hello %d", l)
	}
}

func (lts *LoggerTestSuite) SetupTest() {
	lts.log_rotate_test_count = (10 * 1000 * 10)
}

func (lts *LoggerTestSuite) TestBaseLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	err := SetDefaultLogger("base", cfg)
	assert.NoError(err, "Failed to set base logger")

	simpleTest(lts)

	SetLogLevel(common.ELogLevel.LOG_DEBUG())
	fastTestDebug(lts)

	SetLogLevel(common.ELogLevel.LOG_CRIT())
	fastTestCrit(lts)

	err = Destroy()
	assert.NoError(err, "Failed to release base logger")
}

func (lts *LoggerTestSuite) TestSilentLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{}

	err := SetDefaultLogger("silent", cfg)
	assert.NoError(err, "Failed to set silent logger")

	simpleTest(lts)
}

func (lts *LoggerTestSuite) TestSysLogger() {
	assert := assert.New(lts.T())

	cfg := common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	}

	err := SetDefaultLogger("syslog", cfg)
	assert.NoError(err, "Failed to set silent logger")

	simpleTest(lts)
}

func (lts *LoggerTestSuite) TestNegative() {
	assert := assert.New(lts.T())
	cfg := common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	}

	err := SetDefaultLogger("negative", cfg)
	assert.Error(err, "Negative : did not get logger object")
}

func TestLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}

// ---- helpers ---------------------------------------------------------------

func makeLogger(t *testing.T, cfg LogFileConfig) (*BaseLogger, string) {
	t.Helper()
	dir := t.TempDir()
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join(dir, "test.log")
	} else {
		cfg.LogFile = filepath.Join(dir, cfg.LogFile)
	}
	if cfg.LogLevel == common.ELogLevel.INVALID() {
		cfg.LogLevel = common.ELogLevel.LOG_DEBUG()
	}
	if cfg.LogFileCount == 0 {
		cfg.LogFileCount = 5
	}
	if cfg.LogSize == 0 {
		cfg.LogSize = 1024 * 1024
	}
	l, err := newBaseLogger(cfg)
	if err != nil {
		t.Fatalf("newBaseLogger: %v", err)
	}
	t.Cleanup(func() {
		close(l.channel)
		l.workerDone.Wait()
		if l.logFileHandle != os.Stdout {
			_ = l.logFileHandle.Close()
		}
	})
	return l, cfg.LogFile
}

func fileContent(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// drainLogger flushes the async channel by closing it and waiting for the
// worker, then re-initialises it so the logger remains usable.
func drainLogger(l *BaseLogger) {
	close(l.channel)
	l.workerDone.Wait()
	l.channel = make(chan string, 100000)
	l.workerDone.Add(1)
	go l.logDumper(1, l.channel)
}

// ---- init / defaults -------------------------------------------------------

func TestInit_DefaultLogLevel(t *testing.T) {
	l, _ := makeLogger(t, LogFileConfig{})
	if l.fileConfig.LogLevel != common.ELogLevel.LOG_DEBUG() {
		t.Errorf("default log level: got %v, want LOG_DEBUG", l.fileConfig.LogLevel)
	}
}

func TestInit_DefaultLogSize(t *testing.T) {
	dir := t.TempDir()
	l, err := newBaseLogger(LogFileConfig{LogFile: filepath.Join(dir, "test.log")})
	if err != nil {
		t.Fatalf("newBaseLogger: %v", err)
	}
	defer func() { close(l.channel); l.workerDone.Wait(); l.logFileHandle.Close() }()
	expected := uint64(common.DefaultMaxLogFileSize) * 1024 * 1024
	if l.fileConfig.LogSize != expected {
		t.Errorf("default log size: got %d, want %d", l.fileConfig.LogSize, expected)
	}
}

func TestInit_DefaultLogFileCount(t *testing.T) {
	dir := t.TempDir()
	l, err := newBaseLogger(LogFileConfig{LogFile: filepath.Join(dir, "test.log")})
	if err != nil {
		t.Fatalf("newBaseLogger: %v", err)
	}
	defer func() { close(l.channel); l.workerDone.Wait(); l.logFileHandle.Close() }()
	if l.fileConfig.LogFileCount != common.DefaultLogFileCount {
		t.Errorf("default log file count: got %d, want %d", l.fileConfig.LogFileCount, common.DefaultLogFileCount)
	}
}

func TestInit_StdoutFallback(t *testing.T) {
	cfg := LogFileConfig{
		LogFile:      "stdout",
		LogLevel:     common.ELogLevel.LOG_DEBUG(),
		LogFileCount: 5,
		LogSize:      1024 * 1024,
	}
	l, err := newBaseLogger(cfg)
	if err != nil {
		t.Fatalf("newBaseLogger stdout: %v", err)
	}
	if l.logFileHandle != os.Stdout {
		t.Error("expected stdout handle")
	}
	close(l.channel)
	l.workerDone.Wait()
}

// ---- log level filtering ---------------------------------------------------

func TestLevelFilter_Debug(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})
	l.Debug("debug-msg")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "debug-msg") {
		t.Error("DEBUG message missing at LOG_DEBUG level")
	}
}

func TestLevelFilter_DebugSuppressedAtInfo(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_INFO()})
	l.Debug("hidden-debug")
	drainLogger(l)
	if strings.Contains(fileContent(t, path), "hidden-debug") {
		t.Error("DEBUG message should be suppressed at LOG_INFO level")
	}
}

func TestLevelFilter_InfoVisible(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_INFO()})
	l.Info("info-msg")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "info-msg") {
		t.Error("INFO message missing at LOG_INFO level")
	}
}

func TestLevelFilter_WarnSuppressedAtErr(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_ERR()})
	l.Warn("hidden-warn")
	drainLogger(l)
	if strings.Contains(fileContent(t, path), "hidden-warn") {
		t.Error("WARN message should be suppressed at LOG_ERR level")
	}
}

func TestLevelFilter_CritAlwaysVisible(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_CRIT()})
	l.Crit("crit-msg")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "crit-msg") {
		t.Error("CRIT message missing at LOG_CRIT level")
	}
}

func TestLevelFilter_Trace(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_TRACE()})
	l.Trace("trace-msg")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "trace-msg") {
		t.Error("TRACE message missing at LOG_TRACE level")
	}
}

func TestSetLogLevel_ChangesFiltering(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})
	l.SetLogLevel(common.ELogLevel.LOG_ERR())
	l.Info("suppressed-info")
	drainLogger(l)
	if strings.Contains(fileContent(t, path), "suppressed-info") {
		t.Error("INFO should be suppressed after SetLogLevel(ERR)")
	}
}

// ---- output format ---------------------------------------------------------

func TestOutputFormat_ContainsTag(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogTag: "mytag", LogLevel: common.ELogLevel.LOG_DEBUG()})
	l.Info("tagged-line")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "mytag") {
		t.Error("log tag missing from output")
	}
}

func TestOutputFormat_ContainsLevel(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})
	l.Err("err-line")
	drainLogger(l)
	if !strings.Contains(fileContent(t, path), "LOG_ERR") {
		t.Error("log level string missing from output")
	}
}

func TestOutputFormat_ContainsPID(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})
	l.Info("pid-line")
	drainLogger(l)
	pid := fmt.Sprintf("[%d]", os.Getpid())
	if !strings.Contains(fileContent(t, path), pid) {
		t.Error("PID missing from output")
	}
}

func TestOutputFormat_GoroutineID(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogGoroutineID: true})
	l.Info("gid-line")
	drainLogger(l)
	content := fileContent(t, path)
	if !strings.Contains(content, "gid-line") {
		t.Error("message missing when LogGoroutineID=true")
	}
}

// ---- SetLogFile ------------------------------------------------------------

func TestSetLogFile_RedirectsOutput(t *testing.T) {
	l, _ := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})

	dir := t.TempDir()
	newPath := filepath.Join(dir, "new.log")

	if err := l.SetLogFile(newPath); err != nil {
		t.Fatalf("SetLogFile: %v", err)
	}

	l.Info("after-redirect")
	drainLogger(l)

	if !strings.Contains(fileContent(t, newPath), "after-redirect") {
		t.Error("message not written to new log file after SetLogFile")
	}
}

func TestSetLogFile_InvalidPath(t *testing.T) {
	l, _ := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG()})
	err := l.SetLogFile("/nonexistent/path/log.txt")
	if err == nil {
		t.Error("expected error for invalid log file path")
	}
}

// ---- LogRotate mechanics ---------------------------------------------------

func TestLogRotate_CreatesRotatedFile(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogFileCount: 5})
	l.Info("before-rotate")
	drainLogger(l)

	if err := l.LogRotate(); err != nil {
		t.Fatalf("LogRotate: %v", err)
	}

	rotated := path + ".1"
	if !fileExists(rotated) {
		t.Errorf("expected rotated file %s to exist", rotated)
	}
}

func TestLogRotate_NewFileIsWritable(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogFileCount: 5})
	if err := l.LogRotate(); err != nil {
		t.Fatalf("LogRotate: %v", err)
	}
	l.Info("after-rotate")
	drainLogger(l)

	if !strings.Contains(fileContent(t, path), "after-rotate") {
		t.Error("messages after rotation not written to new log file")
	}
}

func TestLogRotate_ShiftsFiles(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogFileCount: 5})

	// Create two rotations so .1 becomes .2
	l.Info("rot1")
	drainLogger(l)
	if err := l.LogRotate(); err != nil {
		t.Fatalf("rotate 1: %v", err)
	}
	l.Info("rot2")
	drainLogger(l)
	if err := l.LogRotate(); err != nil {
		t.Fatalf("rotate 2: %v", err)
	}

	if !fileExists(path + ".2") {
		t.Error("expected .2 file after two rotations")
	}
}

func TestLogRotate_DeletesOldestFile(t *testing.T) {
	l, path := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogFileCount: 3})

	// 3 rotations with count=3 should delete the oldest
	for i := 0; i < 3; i++ {
		drainLogger(l)
		if err := l.LogRotate(); err != nil {
			t.Fatalf("rotate %d: %v", i+1, err)
		}
	}

	// .3 should not exist (only 3 files allowed, index 1-2 + current)
	if fileExists(path + ".3") {
		t.Error(".3 file should have been deleted with LogFileCount=3")
	}
}

func TestLogRotate_ResetsCurrentSize(t *testing.T) {
	l, _ := makeLogger(t, LogFileConfig{LogLevel: common.ELogLevel.LOG_DEBUG(), LogFileCount: 5})
	l.fileConfig.currentLogSize = 999999
	if err := l.LogRotate(); err != nil {
		t.Fatalf("LogRotate: %v", err)
	}
	if l.fileConfig.currentLogSize != 0 {
		t.Error("currentLogSize should be reset to 0 after rotation")
	}
}

func TestLogRotate_StdoutIsNoop(t *testing.T) {
	cfg := LogFileConfig{
		LogFile:      "stdout",
		LogLevel:     common.ELogLevel.LOG_DEBUG(),
		LogFileCount: 5,
		LogSize:      1024 * 1024,
	}
	l, err := newBaseLogger(cfg)
	if err != nil {
		t.Fatalf("newBaseLogger: %v", err)
	}
	defer func() {
		close(l.channel)
		l.workerDone.Wait()
	}()

	if err := l.LogRotate(); err != nil {
		t.Errorf("LogRotate on stdout should not error: %v", err)
	}
}

// ---- auto-rotation ---------------------------------------------------------

func TestAutoRotate_TriggersOnSizeExceeded(t *testing.T) {
	smallSize := uint64(1) // 1 byte — rotate after first message
	l, path := makeLogger(t, LogFileConfig{
		LogLevel:     common.ELogLevel.LOG_DEBUG(),
		LogFileCount: 5,
		LogSize:      smallSize,
	})

	l.Info("trigger-rotate")
	drainLogger(l)

	if !fileExists(path + ".1") {
		t.Error("auto-rotation did not produce .1 file")
	}
}

// ---- Destroy ---------------------------------------------------------------

func TestDestroy_FlushesMessages(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "destroy.log")
	cfg := LogFileConfig{
		LogFile:      logPath,
		LogLevel:     common.ELogLevel.LOG_DEBUG(),
		LogFileCount: 5,
		LogSize:      1024 * 1024,
	}
	l, err := newBaseLogger(cfg)
	if err != nil {
		t.Fatalf("newBaseLogger: %v", err)
	}

	l.Info("flush-me")
	if err := l.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if !strings.Contains(fileContent(t, logPath), "flush-me") {
		t.Error("message not flushed before Destroy returned")
	}
}

// ---- NewLogger dispatch / public API ---------------------------------------

func TestNewLogger_BaseType(t *testing.T) {
	dir := t.TempDir()
	cfg := common.LogConfig{
		FilePath:    filepath.Join(dir, "base.log"),
		MaxFileSize: 1,
		FileCount:   5,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	l, err := NewLogger("base", cfg)
	if err != nil {
		t.Fatalf("NewLogger base: %v", err)
	}
	defer func() { _ = l.Destroy() }()
	if l.GetType() != "base" {
		t.Errorf("expected type 'base', got %s", l.GetType())
	}
}

func TestNewLogger_SilentType(t *testing.T) {
	l, err := NewLogger("silent", common.LogConfig{})
	if err != nil {
		t.Fatalf("NewLogger silent: %v", err)
	}
	if l.GetType() != "silent" {
		t.Errorf("expected type 'silent', got %s", l.GetType())
	}
}

func TestNewLogger_InvalidType(t *testing.T) {
	_, err := NewLogger("bogus", common.LogConfig{})
	if err == nil {
		t.Error("expected error for invalid logger type")
	}
}

func TestNewLogger_DefaultTag(t *testing.T) {
	dir := t.TempDir()
	cfg := common.LogConfig{
		FilePath:    filepath.Join(dir, "tag.log"),
		MaxFileSize: 1,
		FileCount:   5,
		Level:       common.ELogLevel.LOG_DEBUG(),
		Tag:         "",
	}
	l, err := NewLogger("base", cfg)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer func() { _ = l.Destroy() }()
	// Smoke test: logger should be usable (tag defaulted to FileSystemName)
	l.Info("tag-default-check")
}

// ---- SilentLogger ----------------------------------------------------------

func TestSilentLogger_NoPanic(t *testing.T) {
	l := &SilentLogger{}
	l.Debug("d")
	l.Trace("t")
	l.Info("i")
	l.Warn("w")
	l.Err("e")
	l.Crit("c")
	_ = l.LogRotate()
	_ = l.Destroy()
}

func TestSilentLogger_GetType(t *testing.T) {
	l := &SilentLogger{}
	if l.GetType() != "silent" {
		t.Errorf("expected 'silent', got %s", l.GetType())
	}
}

func TestSilentLogger_GetLogLevel(t *testing.T) {
	l := &SilentLogger{}
	if l.GetLogLevel() != common.ELogLevel.LOG_OFF() {
		t.Errorf("expected LOG_OFF, got %v", l.GetLogLevel())
	}
}
