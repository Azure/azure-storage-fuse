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

package file_cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/suite"
)

type usageFunc = func(ctx context.Context, path string) (float64, error)

type cacheUsageMonitorTestSuite struct {
	suite.Suite
}

func (suite *cacheUsageMonitorTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.Require().NoError(err)
}

// newTestUsageMonitor returns a monitor that measures with measure and does not measure on a timer.
func newTestUsageMonitor(measure usageFunc) *cacheUsageMonitor {
	m := newCacheUsageMonitor("unused")
	m.interval = time.Hour
	m.measure = measure
	return m
}

// firstThen returns a measure function that reports first on its first call and runs later on every other call.
func firstThen(first float64, later usageFunc) usageFunc {
	var calls atomic.Int32
	return func(ctx context.Context, path string) (float64, error) {
		if calls.Add(1) == 1 {
			return first, nil
		}
		return later(ctx, path)
	}
}

func (suite *cacheUsageMonitorTestSuite) requireUpdate(m *cacheUsageMonitor) {
	select {
	case <-m.updates():
	case <-time.After(5 * time.Second):
		suite.FailNow("no usage update")
	}
}

func (suite *cacheUsageMonitorTestSuite) requireNoUpdate(m *cacheUsageMonitor) {
	select {
	case <-m.updates():
		suite.FailNow("unexpected usage update")
	case <-time.After(200 * time.Millisecond):
	}
}

func (suite *cacheUsageMonitorTestSuite) TestStartMeasuresUsage() {
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) { return 42, nil })
	suite.Require().NoError(m.start())
	defer m.stop()

	usage, ok := m.usageMB()
	suite.True(ok)
	suite.InDelta(42, usage, 0)
}

// Without du, start fails, the usage stays unknown and stop must not hang.
func (suite *cacheUsageMonitorTestSuite) TestStartFailsWhenUsageCannotBeMeasured() {
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) {
		return 0, errors.New("failed to find du")
	})
	suite.Error(m.start())

	usage, ok := m.usageMB()
	suite.False(ok)
	suite.Zero(usage)

	m.requestRefresh()
	suite.requireNoUpdate(m)
	m.stop()
}

func (suite *cacheUsageMonitorTestSuite) TestStopWithoutStart() {
	m := newTestUsageMonitor(nil)
	m.stop()
}

func (suite *cacheUsageMonitorTestSuite) TestMeasuresDirectoryWithDu() {
	dir := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(dir, "file"), make([]byte, MB), 0600))

	m := newCacheUsageMonitor(dir)
	suite.Require().NoError(m.start())
	defer m.stop()

	usage, ok := m.usageMB()
	suite.True(ok)
	suite.GreaterOrEqual(usage, float64(1))
	suite.Less(usage, float64(2))
}

func (suite *cacheUsageMonitorTestSuite) TestMeasuresOnInterval() {
	var calls atomic.Int32
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) {
		return float64(calls.Add(1)), nil
	})
	m.interval = 10 * time.Millisecond
	suite.Require().NoError(m.start())
	defer m.stop()

	suite.requireUpdate(m)
	suite.Eventually(func() bool {
		usage, _ := m.usageMB()
		return usage >= 3
	}, 5*time.Second, 10*time.Millisecond)
}

func (suite *cacheUsageMonitorTestSuite) TestRequestRefresh() {
	var value atomic.Int32
	value.Store(1)
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) {
		return float64(value.Load()), nil
	})
	suite.Require().NoError(m.start())
	defer m.stop()

	value.Store(7)
	m.requestRefresh()
	suite.requireUpdate(m)

	usage, ok := m.usageMB()
	suite.True(ok)
	suite.InDelta(7, usage, 0)
}

func (suite *cacheUsageMonitorTestSuite) TestFailedMeasurementKeepsLastUsage() {
	var failures atomic.Int32
	m := newTestUsageMonitor(firstThen(5, func(context.Context, string) (float64, error) {
		failures.Add(1)
		return 0, errors.New("du failed")
	}))
	suite.Require().NoError(m.start())
	defer m.stop()

	m.requestRefresh()
	suite.Eventually(func() bool { return failures.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	suite.requireNoUpdate(m)

	usage, ok := m.usageMB()
	suite.True(ok)
	suite.InDelta(5, usage, 0)
}

// A measurement that runs past the timeout is killed and the last measured usage is kept.
func (suite *cacheUsageMonitorTestSuite) TestTimeoutKeepsLastUsage() {
	errs := make(chan error, 1)
	m := newTestUsageMonitor(firstThen(3, func(ctx context.Context, _ string) (float64, error) {
		<-ctx.Done()
		errs <- ctx.Err()
		return 0, ctx.Err()
	}))
	m.timeout = 50 * time.Millisecond
	suite.Require().NoError(m.start())
	defer m.stop()

	m.requestRefresh()
	select {
	case err := <-errs:
		suite.ErrorIs(err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		suite.FailNow("measurement was not timed out")
	}
	suite.requireNoUpdate(m)

	usage, ok := m.usageMB()
	suite.True(ok)
	suite.InDelta(3, usage, 0)
}

// Reading the usage never waits for a measurement in progress.
func (suite *cacheUsageMonitorTestSuite) TestUsageDoesNotWaitForMeasurement() {
	entered := make(chan struct{})
	release := make(chan struct{})
	m := newTestUsageMonitor(firstThen(2, func(context.Context, string) (float64, error) {
		close(entered)
		<-release
		return 9, nil
	}))
	suite.Require().NoError(m.start())
	defer m.stop()

	m.requestRefresh()
	<-entered
	usage, ok := m.usageMB()
	suite.True(ok)
	suite.InDelta(2, usage, 0)

	close(release)
	suite.requireUpdate(m)
	usage, _ = m.usageMB()
	suite.InDelta(9, usage, 0)
}

// However often a measurement is requested, only one runs at a time.
func (suite *cacheUsageMonitorTestSuite) TestOneMeasurementAtATime() {
	var running, maxRunning, calls atomic.Int32
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) {
		calls.Add(1)
		n := running.Add(1)
		defer running.Add(-1)
		for {
			highest := maxRunning.Load()
			if n <= highest || maxRunning.CompareAndSwap(highest, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		return 1, nil
	})
	m.interval = time.Millisecond
	suite.Require().NoError(m.start())

	var wg sync.WaitGroup
	deadline := time.Now().Add(300 * time.Millisecond)
	for range 8 {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				m.requestRefresh()
			}
		})
	}
	wg.Wait()
	m.stop()

	suite.Greater(calls.Load(), int32(2))
	suite.Equal(int32(1), maxRunning.Load())
}

// The interval counts from the end of a measurement, so a du slower than the interval is not restarted right away.
func (suite *cacheUsageMonitorTestSuite) TestIntervalCountsFromEndOfMeasurement() {
	const interval = 30 * time.Millisecond
	var mu sync.Mutex
	var starts, ends []time.Time
	m := newTestUsageMonitor(func(context.Context, string) (float64, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		time.Sleep(2 * interval)
		mu.Lock()
		ends = append(ends, time.Now())
		mu.Unlock()
		return 1, nil
	})
	m.interval = interval
	suite.Require().NoError(m.start())

	suite.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ends) >= 4
	}, 5*time.Second, 10*time.Millisecond)
	m.stop()

	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(ends) && i < len(starts); i++ {
		suite.GreaterOrEqual(starts[i].Sub(ends[i-1]), interval-5*time.Millisecond, "measurement %d", i)
	}
}

// stop kills a measurement in progress instead of waiting for it, so unmount does not wait for du.
func (suite *cacheUsageMonitorTestSuite) TestStopCancelsMeasurement() {
	entered := make(chan struct{})
	errs := make(chan error, 1)
	m := newTestUsageMonitor(firstThen(1, func(ctx context.Context, _ string) (float64, error) {
		close(entered)
		<-ctx.Done()
		errs <- ctx.Err()
		return 0, ctx.Err()
	}))
	suite.Require().NoError(m.start())

	m.requestRefresh()
	<-entered

	stopped := make(chan struct{})
	go func() {
		m.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		suite.FailNow("stop waited for the measurement")
	}
	suite.ErrorIs(<-errs, context.Canceled)
}

func TestCacheUsageMonitorTestSuite(t *testing.T) {
	suite.Run(t, new(cacheUsageMonitorTestSuite))
}
