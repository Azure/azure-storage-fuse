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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// OtelMetricsTestSuite groups all OTel metrics unit tests.
type OtelMetricsTestSuite struct {
	suite.Suite
	tmpDir string
}

// SetupSuite creates a temporary directory for disk usage tests.
func (suite *OtelMetricsTestSuite) SetupSuite() {
	dir, err := os.MkdirTemp("", "blobfuse2-metrics-test")
	suite.Require().NoError(err, "Failed to create temp dir for tests")
	suite.tmpDir = dir
}

// TearDownSuite cleans up the temporary directory.
func (suite *OtelMetricsTestSuite) TearDownSuite() {
	_ = os.RemoveAll(suite.tmpDir)
}

// TearDownTest ensures metrics are stopped between tests to avoid leaking state.
func (suite *OtelMetricsTestSuite) TearDownTest() {
	_ = StopOtelMetrics()
}

// ---------- StartOtelMetrics / StopOtelMetrics lifecycle ----------

// TestStartStopMetrics verifies that the metrics pipeline can be started and stopped
// without errors, even when no real collector is running. The exporter will buffer data.
func (suite *OtelMetricsTestSuite) TestStartStopMetrics() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		Endpoint:           "localhost:4318",
		CollectionInterval: 1 * time.Second,
		Tag:                "test-blobfuse2",
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err, "StartOtelMetrics should succeed")
	assert.True(IsOtelMetricsRunning(), "Metrics should be running after start")

	err = StopOtelMetrics()
	// Shutdown may return an error if the collector is not reachable (expected in unit tests)
	if err != nil {
		suite.T().Logf("StopOtelMetrics returned error (expected if no collector): %v", err)
	}
	assert.False(IsOtelMetricsRunning(), "Metrics should not be running after stop")
}

// TestStartMetricsDefaultInterval verifies the default collection interval is applied.
func (suite *OtelMetricsTestSuite) TestStartMetricsDefaultInterval() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		Endpoint: "localhost:4318",
		// CollectionInterval intentionally 0 — should default to 30s
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err, "StartOtelMetrics with default interval should succeed")
	assert.True(IsOtelMetricsRunning())
}

// TestDoubleStart verifies that calling StartOtelMetrics twice returns an error
// instead of silently creating a second pipeline.
func (suite *OtelMetricsTestSuite) TestDoubleStart() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{Endpoint: "localhost:4318"}
	err := StartOtelMetrics(cfg)
	assert.NoError(err)

	err = StartOtelMetrics(cfg)
	assert.Error(err, "Starting metrics a second time should return an error")
	assert.Contains(err.Error(), "already started")
}

// TestStopWithoutStart verifies that calling Stop when metrics were never started
// does not return an error.
func (suite *OtelMetricsTestSuite) TestStopWithoutStart() {
	assert := assert.New(suite.T())

	err := StopOtelMetrics()
	assert.NoError(err, "Stopping metrics that were never started should be a no-op")
}

// TestStartMetricsNoEndpoint verifies that starting metrics without an explicit
// endpoint still works (uses default OTEL_EXPORTER_OTLP_ENDPOINT or SDK default).
func (suite *OtelMetricsTestSuite) TestStartMetricsNoEndpoint() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		CollectionInterval: 1 * time.Second,
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err, "StartOtelMetrics without endpoint should succeed")
	assert.True(IsOtelMetricsRunning())
}

// ---------- CPU usage helpers ----------

// TestReadProcStat verifies that /proc/stat can be read and parsed on Linux.
func (suite *OtelMetricsTestSuite) TestReadProcStat() {
	assert := assert.New(suite.T())

	total, idle, err := readProcStat()
	assert.NoError(err, "readProcStat should succeed on Linux")
	assert.Greater(total, float64(0), "Total CPU ticks should be > 0")
	assert.Greater(idle, float64(0), "Idle CPU ticks should be > 0")
	assert.GreaterOrEqual(total, idle, "Total should be >= idle")
}

// TestGetCPUUsage verifies that getCPUUsage returns a sane percentage.
func (suite *OtelMetricsTestSuite) TestGetCPUUsage() {
	assert := assert.New(suite.T())

	m := &OtelMetrics{}

	// First call establishes baseline, returns 0
	cpu, err := m.getCPUUsage()
	assert.NoError(err)
	assert.Equal(float64(0), cpu, "First CPU read should return 0 (baseline)")

	// Small sleep so CPU counters change
	time.Sleep(100 * time.Millisecond)

	cpu, err = m.getCPUUsage()
	assert.NoError(err)
	assert.GreaterOrEqual(cpu, float64(0), "CPU usage should be >= 0")
	assert.LessOrEqual(cpu, float64(100), "CPU usage should be <= 100")
}

// ---------- Memory usage helpers ----------

// TestGetMemoryUsage verifies that memory metrics return sensible values.
func (suite *OtelMetricsTestSuite) TestGetMemoryUsage() {
	assert := assert.New(suite.T())

	m := &OtelMetrics{}
	usage, total := m.getMemoryUsage()

	assert.Greater(usage, float64(0), "Process memory usage should be > 0")
	assert.Greater(total, float64(0), "Total system memory should be > 0")
	assert.LessOrEqual(usage, total, "Process memory should not exceed system total")
}

// TestGetTotalSystemMemory verifies that /proc/meminfo can be parsed.
func (suite *OtelMetricsTestSuite) TestGetTotalSystemMemory() {
	assert := assert.New(suite.T())

	mem := getTotalSystemMemory()
	// Any modern system has at least 128 MB
	assert.Greater(mem, uint64(128*1024*1024), "Total memory should be > 128 MB")
}

// ---------- Disk usage helpers ----------

// TestGetDiskUsageValidPath verifies disk metrics for a valid directory.
func (suite *OtelMetricsTestSuite) TestGetDiskUsageValidPath() {
	assert := assert.New(suite.T())

	m := &OtelMetrics{}
	used, total, percent, err := m.getDiskUsage(suite.tmpDir)

	assert.NoError(err, "getDiskUsage should succeed for a valid path")
	assert.Greater(total, float64(0), "Total disk should be > 0")
	assert.GreaterOrEqual(used, float64(0), "Used disk should be >= 0")
	assert.GreaterOrEqual(percent, float64(0), "Disk usage percent should be >= 0")
	assert.LessOrEqual(percent, float64(100), "Disk usage percent should be <= 100")
}

// TestGetDiskUsageInvalidPath verifies that getDiskUsage returns an error for a non-existent path.
func (suite *OtelMetricsTestSuite) TestGetDiskUsageInvalidPath() {
	assert := assert.New(suite.T())

	m := &OtelMetrics{}
	_, _, _, err := m.getDiskUsage("/this/path/does/not/exist")
	assert.Error(err, "getDiskUsage should fail for a non-existent path")
}

// TestGetDiskUsageEmptyPath verifies that getDiskUsage returns an error for an empty path.
func (suite *OtelMetricsTestSuite) TestGetDiskUsageEmptyPath() {
	assert := assert.New(suite.T())

	m := &OtelMetrics{}
	_, _, _, err := m.getDiskUsage("")
	assert.Error(err, "getDiskUsage should fail for an empty path")
}

// ---------- UpdateFileCachePath ----------

// TestUpdateFileCachePath verifies the runtime path update for disk metrics.
func (suite *OtelMetricsTestSuite) TestUpdateFileCachePath() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		Endpoint:           "localhost:4318",
		CollectionInterval: 1 * time.Second,
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err)

	// Initially no file cache path
	otelMetricsMu.Lock()
	assert.Empty(otelMetricsInstance.fileCachePath, "Initial file cache path should be empty")
	otelMetricsMu.Unlock()

	// Update with a real path
	UpdateFileCachePath(suite.tmpDir)

	otelMetricsMu.Lock()
	assert.Equal(suite.tmpDir, otelMetricsInstance.fileCachePath, "File cache path should be updated")
	otelMetricsMu.Unlock()

	// Update when stopped should not panic
	_ = StopOtelMetrics()
	assert.NotPanics(func() {
		UpdateFileCachePath("/some/path")
	}, "UpdateFileCachePath after stop should not panic")
}

// ---------- Full integration: start → collect → stop ----------

// TestMetricsCollectionCycle starts the metrics pipeline with a 1-second interval,
// waits for at least one collection cycle, and verifies the pipeline shuts down cleanly.
func (suite *OtelMetricsTestSuite) TestMetricsCollectionCycle() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		Endpoint:           "localhost:4318",
		CollectionInterval: 1 * time.Second,
		FileCachePath:      suite.tmpDir,
		Tag:                "test-blobfuse2",
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err)

	// Wait for at least two collection cycles
	time.Sleep(2500 * time.Millisecond)

	err = StopOtelMetrics()
	// Shutdown may return an error when the collector is not reachable (expected in unit tests)
	if err != nil {
		suite.T().Logf("StopOtelMetrics error (expected if no collector): %v", err)
	}
	assert.False(IsOtelMetricsRunning(), "Metrics should be stopped after StopOtelMetrics")
}

// TestMetricsWithoutFileCachePath verifies that metrics work when no file cache
// path is configured (disk metrics are simply skipped).
func (suite *OtelMetricsTestSuite) TestMetricsWithoutFileCachePath() {
	assert := assert.New(suite.T())

	cfg := OtelMetricsConfig{
		Endpoint:           "localhost:4318",
		CollectionInterval: 1 * time.Second,
		// No FileCachePath — disk metrics should be silently skipped
	}

	err := StartOtelMetrics(cfg)
	assert.NoError(err)

	// Wait for a collection cycle to ensure no panic
	time.Sleep(1500 * time.Millisecond)

	err = StopOtelMetrics()
	// Shutdown may return an error when the collector is not reachable (expected in unit tests)
	if err != nil {
		suite.T().Logf("StopOtelMetrics error (expected if no collector): %v", err)
	}
	assert.False(IsOtelMetricsRunning())
}

// TestIsOtelMetricsRunning verifies the state reporting function.
func (suite *OtelMetricsTestSuite) TestIsOtelMetricsRunning() {
	assert := assert.New(suite.T())

	assert.False(IsOtelMetricsRunning(), "Should be false before start")

	cfg := OtelMetricsConfig{Endpoint: "localhost:4318"}
	_ = StartOtelMetrics(cfg)
	assert.True(IsOtelMetricsRunning(), "Should be true after start")

	_ = StopOtelMetrics()
	assert.False(IsOtelMetricsRunning(), "Should be false after stop")
}

func TestOtelMetricsTestSuite(t *testing.T) {
	suite.Run(t, new(OtelMetricsTestSuite))
}
