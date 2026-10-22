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
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	// Measure the cache usage at this interval.
	cacheUsageInterval = DiskUsageCheckInterval * time.Minute

	// Kill a measurement that runs longer than this and keep the last measured usage.
	cacheUsageTimeout = 15 * time.Minute

	// Warn when a measurement runs longer than this.
	cacheUsageSlowThreshold = 30 * time.Second
)

// cacheUsageMonitor measures the disk usage of the cache directory with du in a background goroutine. Callers read
// the last measured usage instead of running du themselves, so file operations never wait for it. du reads every
// file and directory in the cache, which can take minutes when the cache holds many entries that are not in the
// kernel's memory. Only one measurement runs at a time.
type cacheUsageMonitor struct {
	path     string
	interval time.Duration
	timeout  time.Duration
	measure  func(ctx context.Context, path string) (float64, error)

	started atomic.Uint64               // Number of measurements started
	last    atomic.Pointer[usageSample] // Last successful measurement, nil before the first

	refreshSignal chan struct{}
	updateSignal  chan struct{}

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// usageSample is the result of a successful measurement.
type usageSample struct {
	mb  float64
	seq uint64 // The measurement's number. Measurements are numbered from 1 in the order they start.
}

func newCacheUsageMonitor(path string) *cacheUsageMonitor {
	return &cacheUsageMonitor{
		path:          path,
		interval:      cacheUsageInterval,
		timeout:       cacheUsageTimeout,
		measure:       common.GetUsageWithContext,
		refreshSignal: make(chan struct{}, 1),
		updateSignal:  make(chan struct{}, 1),
	}
}

// start measures the usage once and then keeps measuring it in the background. If the first measurement fails,
// for example because du is not installed, start returns the error and does not start measuring.
func (m *cacheUsageMonitor) start() error {
	ctx, cancel := context.WithCancel(context.Background())
	if err := m.refresh(ctx); err != nil {
		cancel()
		return err
	}

	m.cancel = cancel
	m.wg.Add(1)
	go m.run(ctx)
	return nil
}

// stop stops measuring and kills a measurement in progress.
func (m *cacheUsageMonitor) stop() {
	if m.cancel == nil {
		return
	}

	m.cancel()
	m.wg.Wait()
	m.cancel = nil
}

func (m *cacheUsageMonitor) run(ctx context.Context) {
	defer m.wg.Done()

	// The interval counts from the end of the previous measurement, so a slow du is not started again right away.
	timer := time.NewTimer(m.interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-m.refreshSignal:
		}

		err := m.refresh(ctx)
		timer.Reset(m.interval)
		if err == nil {
			select {
			case m.updateSignal <- struct{}{}:
			default:
			}
		}
	}
}

func (m *cacheUsageMonitor) refresh(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	seq := m.started.Add(1)
	begin := time.Now()
	usage, err := m.measure(ctx, m.path)
	elapsed := time.Since(begin)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			log.Err("cacheUsageMonitor::refresh : measuring usage of %s did not finish in %v, keeping the last measured usage",
				m.path, m.timeout)
		case errors.Is(err, context.Canceled):
		default:
			log.Err("cacheUsageMonitor::refresh : failed to measure usage of %s [%s]", m.path, err.Error())
		}
		return err
	}

	if elapsed > cacheUsageSlowThreshold {
		log.Warn("cacheUsageMonitor::refresh : measuring usage of %s took %v, the cache holds many files or directories",
			m.path, elapsed.Round(time.Second))
	}

	m.last.Store(&usageSample{mb: usage, seq: seq})
	return nil
}

// usageMB returns the last measured usage in MB. It returns false if no measurement has succeeded.
func (m *cacheUsageMonitor) usageMB() (float64, bool) {
	sample := m.lastSample()
	if sample == nil {
		return 0, false
	}
	return sample.mb, true
}

// lastSample returns the last successful measurement, or nil if no measurement has succeeded.
func (m *cacheUsageMonitor) lastSample() *usageSample {
	return m.last.Load()
}

// nextSeq returns the number of the next measurement to start. A sample with at least this number was measured
// entirely after this call.
func (m *cacheUsageMonitor) nextSeq() uint64 {
	return m.started.Load() + 1
}

// requestRefresh asks for a new measurement without waiting for it. updates is signalled when it completes.
func (m *cacheUsageMonitor) requestRefresh() {
	select {
	case m.refreshSignal <- struct{}{}:
	default:
	}
}

// updates is signalled after each successful background measurement.
func (m *cacheUsageMonitor) updates() <-chan struct{} {
	return m.updateSignal
}
