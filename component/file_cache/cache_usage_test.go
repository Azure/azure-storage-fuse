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
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/suite"
)

type usageFunc = func(ctx context.Context, path string) (float64, error)

// constUsage returns a measure function that always reports mb.
func constUsage(mb float64) usageFunc {
	return func(context.Context, string) (float64, error) { return mb, nil }
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

// randomData returns size random bytes. Zeros could take less space on a filesystem that compresses.
func randomData(size int) []byte {
	data := make([]byte, size)
	_, _ = rand.Read(data)
	return data
}

// newTestUsageMonitor returns a monitor of dir that does not measure on a timer. Instead of walking dir, it takes
// what tracking does not count from measure.
func newTestUsageMonitor(dir string, measure usageFunc) *cacheUsageMonitor {
	m := newCacheUsageMonitor(dir)
	m.interval = time.Hour
	m.measure = func(ctx context.Context) (cacheWalk, error) {
		mb, err := measure(ctx, dir)
		return cacheWalk{untracked: int64(mb * MB)}, err
	}
	return m
}

// newWalkingUsageMonitor returns a monitor of dir that walks it and does not walk on a timer.
func newWalkingUsageMonitor(dir string) *cacheUsageMonitor {
	m := newCacheUsageMonitor(dir)
	m.interval = time.Hour
	return m
}

// diskUsageMB returns the space allocated to dir and everything under it in MB, as du counts it.
func diskUsageMB(dir string) float64 {
	var total int64
	// The trailing slash follows a symlinked dir.
	_ = filepath.WalkDir(dir+"/", func(path string, d fs.DirEntry, err error) error {
		var stat syscall.Stat_t
		if err == nil && syscall.Lstat(path, &stat) == nil {
			total += stat.Blocks * 512
		}
		return nil
	})
	return float64(total) / MB
}

// allocatedMB returns the space allocated to the regular files under dir in MB, which is what tracking counts.
func allocatedMB(dir string) float64 {
	var total int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			var stat syscall.Stat_t
			if syscall.Lstat(path, &stat) == nil {
				total += stat.Blocks * 512
			}
		}
		return nil
	})
	return float64(total) / MB
}

type cacheUsageMonitorTestSuite struct {
	suite.Suite
	dir string
}

func (suite *cacheUsageMonitorTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.Require().NoError(err)
	suite.dir = suite.T().TempDir()
}

func (suite *cacheUsageMonitorTestSuite) usage(m *cacheUsageMonitor) float64 {
	usage, ok := m.usageMB()
	suite.Require().True(ok)
	return usage
}

// writeFile writes size bytes of data to name in the test directory and returns its path.
func (suite *cacheUsageMonitorTestSuite) writeFile(name string, size int) string {
	path := filepath.Join(suite.dir, name)
	suite.Require().NoError(os.MkdirAll(filepath.Dir(path), 0777))
	suite.Require().NoError(os.WriteFile(path, randomData(size), 0600))
	return path
}

func (suite *cacheUsageMonitorTestSuite) TestUsageUnknownBeforeStart() {
	m := newTestUsageMonitor(suite.dir, constUsage(1))
	usage, ok := m.usageMB()
	suite.False(ok)
	suite.Zero(usage)
	m.stop()
}

// A nil monitor is ignored, as for a cache policy created without one.
func (suite *cacheUsageMonitorTestSuite) TestNilMonitor() {
	var m *cacheUsageMonitor
	m.track(filepath.Join(suite.dir, "file"))
	usage, ok := m.usageMB()
	suite.False(ok)
	suite.Zero(usage)
	m.stop()
}

func (suite *cacheUsageMonitorTestSuite) TestStartMeasuresUsage() {
	m := newTestUsageMonitor(suite.dir, constUsage(42))
	m.start()
	defer m.stop()

	suite.InDelta(42, suite.usage(m), 0.001)
}

// The walk at start counts what was in the cache before mount, including the directories.
func (suite *cacheUsageMonitorTestSuite) TestStartMeasuresExistingFiles() {
	suite.writeFile("existing", MB)
	suite.writeFile("dir/sub/file", 256*1024)

	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
	suite.GreaterOrEqual(suite.usage(m), 1.25)
}

// track counts the space allocated to a file and follows it as it grows, shrinks and is removed.
func (suite *cacheUsageMonitorTestSuite) TestTrackFollowsFileSize() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	file := suite.writeFile("file", 2*MB)
	m.track(file)
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)
	suite.GreaterOrEqual(suite.usage(m), float64(2))

	// Tracking a file again without a change does not count it twice.
	m.track(file)
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)

	suite.Require().NoError(os.WriteFile(file, randomData(5*MB), 0600))
	m.track(file)
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)
	suite.GreaterOrEqual(suite.usage(m), float64(5))

	suite.Require().NoError(os.Truncate(file, MB))
	m.track(file)
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)
	suite.Less(suite.usage(m), float64(1.1))

	suite.Require().NoError(os.Remove(file))
	m.track(file)
	suite.Zero(suite.usage(m))
	suite.Empty(m.files)
}

func (suite *cacheUsageMonitorTestSuite) TestTrackSumsFiles() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	for i := range 5 {
		m.track(suite.writeFile(fmt.Sprintf("dir%d/file", i), (i+1)*256*1024))
	}
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)
	suite.Len(m.files, 5)
}

// A sparse file counts only the space allocated to it, as du counts it.
func (suite *cacheUsageMonitorTestSuite) TestTrackSparseFile() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	file := filepath.Join(suite.dir, "sparse")
	f, err := os.Create(file)
	suite.Require().NoError(err)
	suite.Require().NoError(f.Truncate(100 * MB))
	suite.Require().NoError(f.Close())

	m.track(file)
	suite.Less(suite.usage(m), float64(1))
}

// Only regular files are tracked. The walk measures directories and other entries.
func (suite *cacheUsageMonitorTestSuite) TestTrackIgnoresOtherEntries() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	dir := filepath.Join(suite.dir, "dir")
	suite.Require().NoError(os.Mkdir(dir, 0777))
	target := suite.writeFile("target", MB)
	link := filepath.Join(suite.dir, "link")
	suite.Require().NoError(os.Symlink(target, link))

	m.track(dir)
	m.track(link)
	m.track(filepath.Join(suite.dir, "missing"))
	suite.Zero(suite.usage(m))
	suite.Empty(m.files)
}

// The usage is what tracking counts plus what the last walk measured that tracking does not count.
func (suite *cacheUsageMonitorTestSuite) TestUsageAddsUntrackedToTracked() {
	var measured atomic.Int32
	m := newTestUsageMonitor(suite.dir, func(context.Context, string) (float64, error) {
		return float64(measured.Load()), nil
	})
	m.start()
	defer m.stop()

	first := suite.writeFile("first", MB)
	m.track(first)

	measured.Store(10)
	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(10+allocatedMB(suite.dir), suite.usage(m), 0.001)

	m.track(suite.writeFile("second", 2*MB))
	suite.InDelta(10+allocatedMB(suite.dir), suite.usage(m), 0.001)

	suite.Require().NoError(os.Remove(first))
	m.track(first)
	suite.InDelta(10+allocatedMB(suite.dir), suite.usage(m), 0.001)
}

// After a walk, the usage equals what du reports for the cache directory, whether files are tracked or not.
func (suite *cacheUsageMonitorTestSuite) TestWalkMatchesDu() {
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	for i := range 6 {
		path := suite.writeFile(fmt.Sprintf("d%d/e%d/file%d", i%2, i%3, i), (i+1)*100*1024)
		if i%2 == 0 {
			m.track(path)
		}
	}
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.dir, "empty/dirs/only"), 0777))
	sparse, err := os.Create(filepath.Join(suite.dir, "sparse"))
	suite.Require().NoError(err)
	suite.Require().NoError(sparse.Truncate(50 * MB))
	suite.Require().NoError(sparse.Close())
	suite.Require().NoError(os.Symlink(filepath.Join(suite.dir, "sparse"), filepath.Join(suite.dir, "link")))

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)

	out, err := exec.Command("du", "-s", "-B1", suite.dir).Output()
	if err != nil {
		suite.T().Skip("du is not available")
	}
	duBytes, err := strconv.ParseInt(strings.Fields(string(out))[0], 10, 64)
	suite.Require().NoError(err)
	suite.InDelta(float64(duBytes)/MB, suite.usage(m), 1e-9)
}

// Files cached, written, renamed or evicted while the walk runs are counted once. Operations track the files they
// change, and the walk tracks each file again under its lock, so it does not matter whether the walk read a file's
// directory before or after the change.
func (suite *cacheUsageMonitorTestSuite) TestTrackingDuringWalk() {
	m := newWalkingUsageMonitor(suite.dir)
	evicted := suite.writeFile("a/evicted", MB)
	m.start()
	defer m.stop()

	walk := m.measure
	m.measure = func(ctx context.Context) (cacheWalk, error) {
		result, err := walk(ctx)
		// The walk read the directory already.
		m.track(suite.writeFile("a/cached", 2*MB))
		suite.Require().NoError(os.Remove(evicted))
		m.track(evicted)
		return result, err
	}

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A temporary file that is written while open, so tracking has not counted its data yet, then closed and renamed
// after the walk read it, is counted once.
func (suite *cacheUsageMonitorTestSuite) TestTempFileRenamedDuringWalk() {
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	tmp := filepath.Join(suite.dir, "dir/file.tmp")
	final := filepath.Join(suite.dir, "dir/file")
	suite.Require().NoError(os.MkdirAll(filepath.Dir(tmp), 0777))
	suite.Require().NoError(os.WriteFile(tmp, nil, 0600))
	m.track(tmp) // created empty
	suite.Require().NoError(os.WriteFile(tmp, randomData(MB), 0600))

	walk := m.measure
	m.measure = func(ctx context.Context) (cacheWalk, error) {
		result, err := walk(ctx)
		m.track(tmp) // closed
		suite.Require().NoError(os.Rename(tmp, final))
		m.track(tmp)
		m.track(final)
		return result, err
	}

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// Files cached before mount are counted by the walk at start, and then followed by tracking, so downloading them
// again or deleting them does not count them twice.
func (suite *cacheUsageMonitorTestSuite) TestFileCachedBeforeStart() {
	existing := suite.writeFile("dir/existing", 4*MB)
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)

	// Download the file again, as OpenFile does for a file cached before mount.
	suite.Require().NoError(os.Remove(existing))
	suite.writeFile("dir/existing", 4*MB)
	m.track(existing)
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)

	suite.Require().NoError(os.Remove(existing))
	m.track(existing)
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A file written while open is counted when it is flushed or closed. If that does not happen, the walk counts it.
func (suite *cacheUsageMonitorTestSuite) TestWalkCountsFileWrittenWhileOpen() {
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	path := suite.writeFile("log", MB)
	m.track(path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	suite.Require().NoError(err)
	defer f.Close()
	_, err = f.Write(randomData(4 * MB))
	suite.Require().NoError(err)
	suite.Less(suite.usage(m), diskUsageMB(suite.dir))

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A file renamed while open is written through a handle that still has the old name, so closing it tracks the old
// name. The walk counts what was written after the rename.
func (suite *cacheUsageMonitorTestSuite) TestWalkCountsFileRenamedWhileOpen() {
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	oldPath := suite.writeFile("old", MB)
	newPath := filepath.Join(suite.dir, "new")
	m.track(oldPath)
	f, err := os.OpenFile(oldPath, os.O_WRONLY|os.O_APPEND, 0600)
	suite.Require().NoError(err)
	defer f.Close()

	suite.Require().NoError(os.Rename(oldPath, newPath))
	m.track(oldPath)
	m.track(newPath)
	_, err = f.Write(randomData(8 * MB))
	suite.Require().NoError(err)
	m.track(oldPath) // close
	suite.Less(suite.usage(m), diskUsageMB(suite.dir))

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// The walk measures a file only under its lock. A busy file is left to the operation holding its lock, which tracks
// it when done.
func (suite *cacheUsageMonitorTestSuite) TestWalkSkipsBusyFiles() {
	m := newWalkingUsageMonitor(suite.dir)
	busy := suite.writeFile("busy", MB)
	free := suite.writeFile("free", MB)
	var locked []string
	m.lockFile = func(path string) (func(), bool) {
		if path == busy {
			return nil, false
		}
		locked = append(locked, path)
		return func() {}, true
	}
	m.start()
	defer m.stop()

	suite.Equal([]string{free}, locked)
	suite.Equal(map[string]int64{free: m.files[free]}, m.files)

	m.track(busy)
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A tracked file removed by something other than the file cache stops being tracked at the next walk.
func (suite *cacheUsageMonitorTestSuite) TestCorrectionRemovesFilesRemovedOutsideFileCache() {
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()

	file := suite.writeFile("dir/file", 3*MB)
	kept := suite.writeFile("dir/kept", MB)
	m.track(file)
	m.track(kept)
	suite.Require().NoError(os.Remove(file))

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
	suite.Len(m.files, 1)

	m.track(file)
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A tracked file that the walk did not find but that exists, because it was created after the walk read its
// directory, stays tracked.
func (suite *cacheUsageMonitorTestSuite) TestCorrectionKeepsFilesCreatedDuringWalk() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	file := suite.writeFile("file", MB)
	m.measure = func(context.Context) (cacheWalk, error) {
		m.track(file)
		return cacheWalk{seen: map[string]struct{}{}}, nil
	}

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 1e-9)
	suite.Len(m.files, 1)
}

// The walk follows a symlinked cache directory, and files tracked under the link are not counted twice.
func (suite *cacheUsageMonitorTestSuite) TestSymlinkedCacheDirectory() {
	link := filepath.Join(suite.T().TempDir(), "link")
	suite.Require().NoError(os.Symlink(suite.dir, link))
	suite.writeFile("dir/untracked", MB)
	suite.writeFile("dir/tracked", 2*MB)

	m := newWalkingUsageMonitor(link)
	m.start()
	defer m.stop()
	m.track(filepath.Join(link, "dir/tracked"))

	suite.Require().NoError(m.correct(context.Background()))
	suite.InDelta(diskUsageMB(suite.dir), suite.usage(m), 1e-9)
}

// A walk stops when it is cancelled and keeps the usage.
func (suite *cacheUsageMonitorTestSuite) TestWalkStopsWhenCancelled() {
	for i := range 10 {
		suite.writeFile(fmt.Sprintf("dir%d/file", i), 4096)
	}
	m := newWalkingUsageMonitor(suite.dir)
	m.start()
	defer m.stop()
	usage := suite.usage(m)

	suite.writeFile("new", MB)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	suite.ErrorIs(m.correct(ctx), context.Canceled)
	suite.InDelta(usage, suite.usage(m), 1e-9)
}

// Without the cache directory, the walk fails, and the usage counts the tracked files.
func (suite *cacheUsageMonitorTestSuite) TestMissingCacheDirectory() {
	m := newWalkingUsageMonitor(filepath.Join(suite.dir, "missing"))
	m.start()
	defer m.stop()

	suite.Zero(suite.usage(m))
	suite.Error(m.correct(context.Background()))
	m.track(suite.writeFile("file", MB))
	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 1e-9)
}

func (suite *cacheUsageMonitorTestSuite) TestFailedCorrectionKeepsUsage() {
	m := newTestUsageMonitor(suite.dir, firstThen(5, func(context.Context, string) (float64, error) {
		return 0, errors.New("walk failed")
	}))
	m.start()
	defer m.stop()

	suite.Error(m.correct(context.Background()))
	suite.InDelta(5, suite.usage(m), 0.001)
}

// A failed measurement is retried on the next interval.
func (suite *cacheUsageMonitorTestSuite) TestFailedStartMeasurementIsRetried() {
	var calls atomic.Int32
	m := newTestUsageMonitor(suite.dir, func(context.Context, string) (float64, error) {
		if calls.Add(1) == 1 {
			return 0, errors.New("walk failed")
		}
		return 7, nil
	})
	m.interval = 10 * time.Millisecond
	m.start()
	defer m.stop()

	suite.Zero(suite.usage(m))
	suite.Eventually(func() bool {
		usage, _ := m.usageMB()
		return usage == 7
	}, 5*time.Second, 10*time.Millisecond)
}

func (suite *cacheUsageMonitorTestSuite) TestMeasuresOnInterval() {
	var calls atomic.Int32
	m := newTestUsageMonitor(suite.dir, func(context.Context, string) (float64, error) {
		return float64(calls.Add(1)), nil
	})
	m.interval = 10 * time.Millisecond
	m.start()
	defer m.stop()

	suite.Eventually(func() bool {
		usage, _ := m.usageMB()
		return usage >= 3
	}, 5*time.Second, 10*time.Millisecond)
}

// A measurement that runs past the timeout is killed and the usage is kept.
func (suite *cacheUsageMonitorTestSuite) TestTimeoutKeepsUsage() {
	m := newTestUsageMonitor(suite.dir, firstThen(3, func(ctx context.Context, _ string) (float64, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}))
	m.timeout = 50 * time.Millisecond
	m.start()
	defer m.stop()

	suite.ErrorIs(m.correct(context.Background()), context.DeadlineExceeded)
	suite.InDelta(3, suite.usage(m), 0.001)
}

// Reading and tracking the usage never wait for a measurement in progress.
func (suite *cacheUsageMonitorTestSuite) TestUsageDoesNotWaitForMeasurement() {
	entered := make(chan struct{})
	release := make(chan struct{})
	m := newTestUsageMonitor(suite.dir, firstThen(0, func(context.Context, string) (float64, error) {
		close(entered)
		<-release
		return 0, nil
	}))
	m.interval = time.Millisecond
	m.start()
	defer m.stop()
	defer close(release)
	<-entered

	file := suite.writeFile("file", MB)
	usage := make(chan float64, 1)
	go func() {
		m.track(file)
		mb, _ := m.usageMB()
		usage <- mb
	}()
	select {
	case mb := <-usage:
		suite.InDelta(allocatedMB(suite.dir), mb, 0.001)
	case <-time.After(5 * time.Second):
		suite.FailNow("tracking waited for the measurement")
	}
}

// Only one measurement runs at a time, and the interval counts from the end of a measurement, so a walk slower than
// the interval is not restarted right away.
func (suite *cacheUsageMonitorTestSuite) TestOneMeasurementAtATime() {
	const interval = 20 * time.Millisecond
	var running, maxRunning atomic.Int32
	var mu sync.Mutex
	var starts, ends []time.Time
	m := newTestUsageMonitor(suite.dir, func(context.Context, string) (float64, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			highest := maxRunning.Load()
			if n <= highest || maxRunning.CompareAndSwap(highest, n) {
				break
			}
		}
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
	m.start()

	suite.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ends) >= 4
	}, 5*time.Second, 10*time.Millisecond)
	m.stop()

	suite.Equal(int32(1), maxRunning.Load())
	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(ends) && i < len(starts); i++ {
		suite.GreaterOrEqual(starts[i].Sub(ends[i-1]), interval-5*time.Millisecond, "measurement %d", i)
	}
}

// stop cancels a measurement in progress instead of waiting for it, so unmount does not wait for the walk.
func (suite *cacheUsageMonitorTestSuite) TestStopCancelsMeasurement() {
	entered := make(chan struct{})
	errs := make(chan error, 1)
	m := newTestUsageMonitor(suite.dir, firstThen(1, func(ctx context.Context, _ string) (float64, error) {
		close(entered)
		<-ctx.Done()
		errs <- ctx.Err()
		return 0, ctx.Err()
	}))
	m.interval = time.Millisecond
	m.start()
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

// Concurrent tracking of different files, and of the same file, keeps the usage equal to the files' sizes.
func (suite *cacheUsageMonitorTestSuite) TestConcurrentTracking() {
	m := newTestUsageMonitor(suite.dir, constUsage(0))
	m.start()
	defer m.stop()

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			path := suite.writeFile(fmt.Sprintf("file%d", i), (i+1)*64*1024)
			for range 50 {
				m.track(path)
				m.track(filepath.Join(suite.dir, "shared"))
				_, _ = m.usageMB()
			}
		})
	}
	wg.Wait()

	suite.InDelta(allocatedMB(suite.dir), suite.usage(m), 0.001)
	suite.Len(m.files, 16)
}

func TestCacheUsageMonitorTestSuite(t *testing.T) {
	suite.Run(t, new(cacheUsageMonitorTestSuite))
}
