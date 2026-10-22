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
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	// Walk the cache directory at this interval, counted from the end of the previous walk.
	cacheUsageCorrectionInterval = 10 * time.Minute

	// Stop a walk that runs longer than this and keep the current usage.
	cacheUsageTimeout = 15 * time.Minute

	// Warn when a walk runs longer than this.
	cacheUsageSlowThreshold = 30 * time.Second
)

// cacheUsageMonitor keeps the disk usage of the cache directory in memory, so reading it is instant and current.
//
// It keeps the space allocated to each file in the cache. The file cache calls track for a file, while holding the
// file's lock, after each operation that can change the file's size or remove it, so the usage follows those
// operations right away.
//
// A walk of the cache directory, at start and then every correction interval in a background goroutine, measures
// the directories and measures every file again under the file's lock. That corrects what the operations do not
// show: files cached before mount, files written while open (including after being renamed while open), and changes
// made by anything other than the file cache. A walk reads every directory and file in the cache, as du does, which
// can take minutes on a large cache, so only one runs at a time and nothing waits for it.
type cacheUsageMonitor struct {
	path     string
	interval time.Duration
	timeout  time.Duration
	measure  func(ctx context.Context) (cacheWalk, error)

	// lockFile takes the lock of the file at path without waiting. It returns false if the lock is busy. Nil means
	// files are measured without their lock.
	lockFile func(path string) (unlock func(), ok bool)

	mu        sync.Mutex
	files     map[string]int64 // Allocated bytes of each file with any allocated
	tracked   int64            // Sum of files
	untracked int64            // Bytes allocated to directories and other entries at the last walk

	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// cacheWalk is the result of walking the cache directory.
type cacheWalk struct {
	untracked int64               // Bytes allocated to directories and to entries that are not regular files
	seen      map[string]struct{} // Regular files found. Nil if not known.
}

func newCacheUsageMonitor(path string) *cacheUsageMonitor {
	m := &cacheUsageMonitor{
		path:     path,
		interval: cacheUsageCorrectionInterval,
		timeout:  cacheUsageTimeout,
		files:    make(map[string]int64),
	}
	m.measure = m.walk
	return m
}

// start measures the cache directory once and then keeps measuring it in the background.
func (m *cacheUsageMonitor) start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.started.Store(true)

	_ = m.correct(ctx)

	m.wg.Add(1)
	go m.run(ctx)
}

// stop stops measuring and stops a walk in progress. Tracking still keeps the usage up to date.
func (m *cacheUsageMonitor) stop() {
	if m == nil || m.cancel == nil {
		return
	}

	m.cancel()
	m.wg.Wait()
	m.cancel = nil
}

func (m *cacheUsageMonitor) run(ctx context.Context) {
	defer m.wg.Done()

	timer := time.NewTimer(m.interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		_ = m.correct(ctx)
		timer.Reset(m.interval)
	}
}

// correct walks the cache directory, and stops tracking files that are gone.
func (m *cacheUsageMonitor) correct(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	begin := time.Now()
	result, err := m.measure(ctx)
	elapsed := time.Since(begin)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			log.Err("cacheUsageMonitor::correct : measuring usage of %s did not finish in %v", m.path, m.timeout)
		case errors.Is(err, context.Canceled):
		default:
			log.Err("cacheUsageMonitor::correct : failed to measure usage of %s [%s]", m.path, err.Error())
		}
		return err
	}

	if elapsed > cacheUsageSlowThreshold {
		log.Warn("cacheUsageMonitor::correct : measuring usage of %s took %v, the cache holds many files or directories",
			m.path, elapsed.Round(time.Second))
	}

	m.mu.Lock()
	m.untracked = result.untracked
	var unseen map[string]int64
	for path, allocated := range m.files {
		if _, ok := result.seen[path]; !ok {
			if unseen == nil {
				unseen = make(map[string]int64)
			}
			unseen[path] = allocated
		}
	}
	m.mu.Unlock()

	// A tracked file that the walk did not find was removed by something other than the file cache, or was created
	// after the walk read its directory.
	for path, allocated := range unseen {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			continue
		}
		m.mu.Lock()
		if m.files[path] == allocated {
			delete(m.files, path)
			m.tracked -= allocated
		}
		m.mu.Unlock()
	}

	return nil
}

// walk measures the directories and the entries that are not regular files under the cache directory, and tracks
// every regular file, so the usage counts what du counts.
func (m *cacheUsageMonitor) walk(ctx context.Context) (cacheWalk, error) {
	result := cacheWalk{seen: make(map[string]struct{})}
	err := m.walkDir(ctx, m.path, &result)
	return result, err
}

func (m *cacheUsageMonitor) walkDir(ctx context.Context, dir string, result *cacheWalk) error {
	f, err := os.Open(dir)
	if err != nil {
		if dir == m.path {
			return err
		}
		// The directory was removed or cannot be read, so skip it as du does.
		log.Debug("cacheUsageMonitor::walkDir : failed to open %s [%s]", dir, err.Error())
		return nil
	}
	defer f.Close()

	if info, err := f.Stat(); err == nil {
		result.untracked += info.Sys().(*syscall.Stat_t).Blocks * 512
	}

	for {
		entries, err := f.ReadDir(1024)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}

			path := filepath.Join(dir, entry.Name())
			switch {
			case entry.IsDir():
				if err := m.walkDir(ctx, path, result); err != nil {
					return err
				}

			case entry.Type().IsRegular():
				result.seen[path] = struct{}{}
				m.trackUnlocked(path)

			default:
				// du also counts the space used by other entries, such as a symlink with a long target.
				var stat syscall.Stat_t
				if syscall.Lstat(path, &stat) == nil {
					result.untracked += stat.Blocks * 512
				}
			}
		}

		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Debug("cacheUsageMonitor::walkDir : failed to read %s [%s]", dir, err.Error())
			return nil
		}
	}
}

// trackUnlocked tracks the file at path for the walk, taking the file's lock. If the lock is busy, the operation
// holding it tracks the file when it is done.
func (m *cacheUsageMonitor) trackUnlocked(path string) {
	if m.lockFile != nil {
		unlock, ok := m.lockFile(path)
		if !ok {
			return
		}
		defer unlock()
	}
	m.track(path)
}

// allocatedSize returns the space allocated to the regular file at path, or 0 for anything else. st_blocks counts
// 512-byte units, as du does, so a sparse file counts only the space it uses.
func allocatedSize(path string) int64 {
	var stat syscall.Stat_t
	err := syscall.Lstat(path, &stat)
	if err == nil && stat.Mode&syscall.S_IFMT == syscall.S_IFREG {
		return stat.Blocks * 512
	}
	if err != nil && !os.IsNotExist(err) {
		log.Debug("cacheUsageMonitor::allocatedSize : failed to stat %s [%s]", path, err.Error())
	}
	return 0
}

// track updates the usage for the file at path after an operation that can change its size or remove it. Call it
// while holding the file's lock, so the size read here is not replaced by an older one.
func (m *cacheUsageMonitor) track(path string) {
	if m == nil {
		return
	}

	allocated := allocatedSize(path)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.tracked += allocated - m.files[path]
	if allocated > 0 {
		m.files[path] = allocated
	} else {
		delete(m.files, path)
	}
}

// usageMB returns the usage in MB. It returns false if the monitor was not started.
func (m *cacheUsageMonitor) usageMB() (float64, bool) {
	if m == nil || !m.started.Load() {
		return 0, false
	}

	m.mu.Lock()
	usage := m.tracked + m.untracked
	m.mu.Unlock()

	return float64(max(usage, 0)) / MB, true
}
