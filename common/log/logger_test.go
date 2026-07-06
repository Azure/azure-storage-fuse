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
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

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

// resetCrashOutputState clears process-global state used by the crash-output / rotation-hook
// machinery so each test starts from a known baseline. logObj is swapped for a fresh silent logger
// because earlier tests may have called Destroy() on a base logger, which closes its channel and
// would cause subsequent Warn/Info calls (e.g. from setCrashOutput) to panic.
//
// The previously-installed SIGHUP listener goroutine (if any) is stopped: signal.Stop unregisters
// the channel from the runtime signal dispatch, and closing the channel lets the "for range"
// goroutine drain the buffer and exit. Without this, tests that call SetupCrashOutput repeatedly
// would leak goroutines still registered for SIGHUP, and future SIGHUPs would fan out to all of
// them, making later tests flaky.
func resetCrashOutputState() {
	rotateHooksMu.Lock()
	rotateHooks = nil
	rotateHooksMu.Unlock()

	if sighupInstalled.Load() && sighupCh != nil {
		signal.Stop(sighupCh)
		close(sighupCh)
		sighupCh = nil
	}
	sighupOnce = sync.Once{}
	sighupInstalled.Store(false)
	sighupHandled.Store(0)
	setupCrashOutputOnce = sync.Once{}

	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	logObj = &SilentLogger{}
}

func hookCount() int {
	rotateHooksMu.Lock()
	defer rotateHooksMu.Unlock()
	return len(rotateHooks)
}

func (lts *LoggerTestSuite) TestRegisterLogRotateHook() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	// nil hook must be a no-op.
	registerLogRotateHook(nil)
	assert.Equal(0, hookCount())

	var order []int
	registerLogRotateHook(func() { order = append(order, 1) })
	registerLogRotateHook(func() { order = append(order, 2) })
	registerLogRotateHook(func() { order = append(order, 3) })
	assert.Equal(3, hookCount())

	invokeRotateHooks()
	assert.Equal([]int{1, 2, 3}, order, "hooks must fire in registration order")

	// Re-invocation must re-run all hooks (they are not one-shot).
	invokeRotateHooks()
	assert.Equal([]int{1, 2, 3, 1, 2, 3}, order)
}

// TestInvokeRotateHooksIsolatesPanics validates that a panicking hook does not prevent later
// hooks from running and does not propagate out of invokeRotateHooks. This matters because
// invokeRotateHooks runs inline on BaseLogger's log-rotation path; a bare panic there would
// take down the log-dumper goroutine and effectively break logging.
func (lts *LoggerTestSuite) TestInvokeRotateHooksIsolatesPanics() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	var ran []int
	registerLogRotateHook(func() { ran = append(ran, 1) })
	registerLogRotateHook(func() { panic("hook 2 exploded") })
	registerLogRotateHook(func() { ran = append(ran, 3) })

	assert.NotPanics(invokeRotateHooks, "invokeRotateHooks must swallow per-hook panics")
	assert.Equal([]int{1, 3}, ran, "hooks after a panicking hook must still run")
}

func (lts *LoggerTestSuite) TestBaseLoggerRotateInvokesHook() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	tmpDir := lts.T().TempDir()
	cfg := common.LogConfig{
		FilePath:    filepath.Join(tmpDir, "rotate.log"),
		MaxFileSize: 1,
		FileCount:   3,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	err := SetDefaultLogger("base", cfg)
	assert.NoError(err)
	defer func() { _ = Destroy() }()

	var fired int32
	registerLogRotateHook(func() { atomic.AddInt32(&fired, 1) })

	assert.NoError(LogRotate())
	assert.Equal(int32(1), atomic.LoadInt32(&fired))

	assert.NoError(LogRotate())
	assert.Equal(int32(2), atomic.LoadInt32(&fired))
}

func (lts *LoggerTestSuite) TestSetCrashOutput() {
	assert := assert.New(lts.T())

	// "base" with a real, writable file -- success path; runtime crash output is updated.
	resetCrashOutputState()
	tmp, err := os.CreateTemp("", "blobfuse2-crash-base-*.log")
	assert.NoError(err)
	defer os.Remove(tmp.Name())
	assert.NoError(tmp.Close())
	assert.NotPanics(func() { setCrashOutput("base", tmp.Name()) })

	// "base" with empty path or "stdout" -- early no-op, no panic.
	assert.NotPanics(func() { setCrashOutput("base", "") })
	assert.NotPanics(func() { setCrashOutput("base", "stdout") })

	// "silent" and unknown logger types -- early no-op, no panic.
	assert.NotPanics(func() { setCrashOutput("silent", "ignored") })
	assert.NotPanics(func() { setCrashOutput("not-a-real-type", "ignored") })

	// "base" pointing at a non-existent path -- must Warn and return (no panic, no crash).
	// O_CREATE is intentionally not used, so missing files are tolerated.
	assert.NotPanics(func() {
		setCrashOutput("base", filepath.Join(lts.T().TempDir(), "does-not-exist.log"))
	})

	// "syslog"/""/"default" branches all target common.SyslogFilePath. In a test environment that
	// file is usually not writable; the call must still not panic and must not return an error.
	assert.NotPanics(func() { setCrashOutput("", "") })
	assert.NotPanics(func() { setCrashOutput("default", "") })
	assert.NotPanics(func() { setCrashOutput("syslog", "") })
}

func (lts *LoggerTestSuite) TestCrashOutputTarget() {
	assert := assert.New(lts.T())

	// "base" with a real path returns that path.
	assert.Equal("/tmp/blobfuse2.log", crashOutputTarget("base", "/tmp/blobfuse2.log"))

	// "base" with empty / stdout returns "" (no file to mirror to).
	assert.Empty(crashOutputTarget("base", ""))
	assert.Empty(crashOutputTarget("base", "stdout"))

	// syslog family always routes to common.SyslogFilePath regardless of logFilePath.
	assert.Equal(common.SyslogFilePath, crashOutputTarget("", ""))
	assert.Equal(common.SyslogFilePath, crashOutputTarget("default", ""))
	assert.Equal(common.SyslogFilePath, crashOutputTarget("syslog", ""))
	assert.Equal(common.SyslogFilePath, crashOutputTarget("syslog", "/ignored"))

	// silent and unknown types -- no target.
	assert.Empty(crashOutputTarget("silent", "ignored"))
	assert.Empty(crashOutputTarget("not-a-real-type", "ignored"))
}

// TestSetCrashOutputPanicSafeAfterDestroy validates the defer/recover guard in setCrashOutput:
// the SIGHUP handler goroutine outlives Destroy(), which closes BaseLogger's channel. Any Warn()
// from setCrashOutput after Destroy would panic on send-to-closed-channel; the recover swallows
// it so a SIGHUP during teardown cannot crash the process.
func (lts *LoggerTestSuite) TestSetCrashOutputPanicSafeAfterDestroy() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	// Install a base logger and immediately destroy it -- the channel is now closed.
	cfg := common.LogConfig{
		FilePath:    filepath.Join(lts.T().TempDir(), "destroyed.log"),
		MaxFileSize: 10,
		FileCount:   3,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	assert.NoError(SetDefaultLogger("base", cfg))
	assert.NoError(Destroy())

	// setCrashOutput's error path calls Warn() -- which would attempt to send on the closed
	// channel. Point it at a non-existent file so the error path is exercised. The deferred
	// recover inside setCrashOutput must absorb the panic.
	assert.NotPanics(func() {
		setCrashOutput("base", filepath.Join(lts.T().TempDir(), "does-not-exist.log"))
	})

	// Restore a healthy logger so subsequent tests are not affected.
	logObj = &SilentLogger{}
}

// TestSetupCrashOutputSkipsUnsupportedConfigs validates that SetupCrashOutput does not register
// a rotate hook and does not install the SIGHUP handler for logger configurations that have no
// meaningful file target to mirror crash dumps to.
func (lts *LoggerTestSuite) TestSetupCrashOutputSkipsUnsupportedConfigs() {
	assert := assert.New(lts.T())

	cases := []struct {
		loggerType, logFilePath string
	}{
		{"silent", "ignored"},
		{"base", ""},
		{"base", "stdout"},
		{"unknown-logger", "/tmp/x"},
	}
	for _, c := range cases {
		resetCrashOutputState()
		before := hookCount()
		SetupCrashOutput(c.loggerType, c.logFilePath)
		assert.Equalf(before, hookCount(),
			"SetupCrashOutput must not register a rotate hook for (%q, %q)", c.loggerType, c.logFilePath)
		assert.Falsef(sighupInstalled.Load(),
			"SetupCrashOutput must not install SIGHUP handler for (%q, %q)", c.loggerType, c.logFilePath)
	}
}

// TestSetupCrashOutputBaseSkipsSighupHandler validates that "base" mode registers the in-process
// rotate hook but does not install the SIGHUP handler: BaseLogger owns its file and rotates in
// process, so hijacking SIGHUP would be inappropriate.
func (lts *LoggerTestSuite) TestSetupCrashOutputBaseSkipsSighupHandler() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	tmp, err := os.CreateTemp("", "blobfuse2-crash-base-nosighup-*.log")
	assert.NoError(err)
	defer os.Remove(tmp.Name())
	assert.NoError(tmp.Close())

	before := hookCount()
	SetupCrashOutput("base", tmp.Name())
	assert.Equal(before+1, hookCount(), "base mode must register exactly one rotate hook")
	assert.False(sighupInstalled.Load(), "base mode must not install SIGHUP handler")

	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
}

func (lts *LoggerTestSuite) TestSetupCrashOutputRegistersHookAndHandler() {
	assert := assert.New(lts.T())
	resetCrashOutputState()

	tmp, err := os.CreateTemp("", "blobfuse2-crash-setup-*.log")
	assert.NoError(err)
	defer os.Remove(tmp.Name())
	assert.NoError(tmp.Close())

	// Pre-arm a sentinel SIGHUP listener so the test can confirm signal delivery without risk of
	// the default SIGHUP action (process termination) firing if installation somehow failed.
	sentinel := make(chan os.Signal, 1)
	signal.Notify(sentinel, syscall.SIGHUP)
	defer signal.Stop(sentinel)

	before := hookCount()
	// Use "syslog" so SetupCrashOutput actually installs the SIGHUP handler (base mode skips it).
	SetupCrashOutput("syslog", tmp.Name())
	assert.Equal(before+1, hookCount(), "SetupCrashOutput must register exactly one rotate hook")
	assert.True(sighupInstalled.Load(), "syslog mode must install SIGHUP handler")

	// Invoking rotate hooks must not panic (the registered closure re-runs setCrashOutput).
	assert.NotPanics(invokeRotateHooks)

	// Idempotence: a second call must not register another rotate hook and must not panic. The
	// outer sync.Once in SetupCrashOutput ensures repeated invocations are cheap no-ops.
	hooksAfterFirst := hookCount()
	assert.NotPanics(func() { SetupCrashOutput("syslog", tmp.Name()) })
	assert.Equal(hooksAfterFirst, hookCount(), "second SetupCrashOutput must not register another rotate hook")

	// Signal delivery sanity check: SIGHUP reaches the process (proving signal.Notify was wired).
	handledBefore := sighupHandled.Load()
	assert.NoError(syscall.Kill(syscall.Getpid(), syscall.SIGHUP))
	select {
	case <-sentinel:
		// delivered to the sentinel; our listener may or may not have run yet
	case <-time.After(2 * time.Second):
		lts.T().Fatal("SIGHUP was not delivered to the process within 2s")
	}

	// Also verify that the installed crash-output handler goroutine actually ran, not just that
	// the signal reached the process. Poll the counter since the handler runs asynchronously.
	deadline := time.Now().Add(2 * time.Second)
	for sighupHandled.Load() == handledBefore {
		if time.Now().After(deadline) {
			lts.T().Fatalf("crash-output SIGHUP handler did not execute within 2s (before=%d, after=%d)",
				handledBefore, sighupHandled.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Greater(sighupHandled.Load(), handledBefore, "SIGHUP handler goroutine must run at least once")

	// Clean up runtime crash output so it doesn't leak to other tests.
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
}

func TestLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(LoggerTestSuite))
}
