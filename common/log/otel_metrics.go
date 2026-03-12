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
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// OtelMetricsConfig holds the configuration for OpenTelemetry metrics collection.
type OtelMetricsConfig struct {
	// Endpoint is the OTLP HTTP endpoint (e.g., "localhost:4318").
	// If empty, the OTEL_EXPORTER_OTLP_ENDPOINT environment variable is used.
	Endpoint string

	// CollectionInterval is how frequently metrics are collected and exported.
	// Defaults to 30 seconds if not specified.
	CollectionInterval time.Duration

	// FileCachePath is the path to the file cache directory.
	// If non-empty, disk usage metrics will be collected for this path.
	FileCachePath string

	// Tag is the application tag used for metric attributes (e.g., "blobfuse2").
	Tag string
}

// OtelMetrics manages the OpenTelemetry metrics pipeline for system resource monitoring.
// It collects CPU, memory, and optionally disk (file cache) usage metrics and exports
// them via OTLP HTTP to an OpenTelemetry Collector.
type OtelMetrics struct {
	meterProvider *sdkmetric.MeterProvider
	meter         otelmetric.Meter

	// Metrics instruments
	cpuUsageGauge         otelmetric.Float64ObservableGauge
	memoryUsageGauge      otelmetric.Float64ObservableGauge
	memoryTotalGauge      otelmetric.Float64ObservableGauge
	diskUsageGauge        otelmetric.Float64ObservableGauge
	diskTotalGauge        otelmetric.Float64ObservableGauge
	diskUsagePercentGauge otelmetric.Float64ObservableGauge

	// Configuration
	fileCachePath string
	tag           string

	// Lifecycle management
	cancel context.CancelFunc
	mu     sync.Mutex

	// Previous CPU stats for delta calculation
	prevCPUTotal float64
	prevCPUIdle  float64
}

// Global singleton for the metrics collector, protected by a mutex.
var (
	otelMetricsInstance *OtelMetrics
	otelMetricsMu       sync.Mutex
)

// StartOtelMetrics initializes and starts the OpenTelemetry metrics pipeline.
// It creates an OTLP HTTP exporter, sets up metric instruments for CPU, memory,
// and optionally disk usage, and begins periodic collection.
// Returns an error if the pipeline cannot be initialized.
func StartOtelMetrics(cfg OtelMetricsConfig) error {
	otelMetricsMu.Lock()
	defer otelMetricsMu.Unlock()

	// Prevent duplicate initialization
	if otelMetricsInstance != nil {
		return fmt.Errorf("OTel metrics already started")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set default collection interval to 30s if not configured
	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = 30 * time.Second
	}

	// Set default tag
	tag := cfg.Tag
	if tag == "" {
		tag = common.FileSystemName
	}

	// Build OTLP HTTP exporter options
	opts := []otlpmetrichttp.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
		// Use insecure mode (matching the OTel logger pattern)
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Create a periodic reader that collects and exports metrics at the configured interval
	reader := sdkmetric.NewPeriodicReader(exporter,
		sdkmetric.WithInterval(cfg.CollectionInterval),
	)

	// Create the meter provider with the periodic reader
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	meter := mp.Meter("blobfuse2.system.metrics",
		otelmetric.WithInstrumentationVersion(common.Blobfuse2Version),
	)

	m := &OtelMetrics{
		meterProvider: mp,
		meter:         meter,
		fileCachePath: cfg.FileCachePath,
		tag:           tag,
		cancel:        cancel,
	}

	// Register all metric instruments using asynchronous (observable) gauges.
	// These are callback-based: the OTel SDK invokes the callbacks during each collection cycle.
	if err := m.registerMetrics(); err != nil {
		cancel()
		_ = mp.Shutdown(ctx)
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	otelMetricsInstance = m
	return nil
}

// StopOtelMetrics gracefully shuts down the OpenTelemetry metrics pipeline.
// It flushes any pending metrics and releases resources.
func StopOtelMetrics() error {
	otelMetricsMu.Lock()
	defer otelMetricsMu.Unlock()

	if otelMetricsInstance == nil {
		return nil
	}

	m := otelMetricsInstance
	otelMetricsInstance = nil

	// Cancel the context to signal shutdown
	if m.cancel != nil {
		m.cancel()
	}

	// Gracefully shutdown the meter provider (flushes pending metrics)
	if m.meterProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return m.meterProvider.Shutdown(ctx)
	}

	return nil
}

// IsOtelMetricsRunning returns true if the OTel metrics pipeline is currently active.
func IsOtelMetricsRunning() bool {
	otelMetricsMu.Lock()
	defer otelMetricsMu.Unlock()
	return otelMetricsInstance != nil
}

// GetOtelMeter returns the OTel meter instance if the metrics pipeline is running.
// External components (e.g., the Azure SDK request metrics policy) use this to register
// their own instruments on the same meter provider, ensuring all metrics flow through
// a single OTLP exporter.
// Returns nil if metrics are not running.
func GetOtelMeter() otelmetric.Meter {
	otelMetricsMu.Lock()
	defer otelMetricsMu.Unlock()

	if otelMetricsInstance == nil {
		return nil
	}

	return otelMetricsInstance.meter
}

// UpdateFileCachePath updates the file cache path for disk usage monitoring at runtime.
// This allows the metrics module to start monitoring disk usage after file cache is configured.
func UpdateFileCachePath(path string) {
	otelMetricsMu.Lock()
	defer otelMetricsMu.Unlock()
	if otelMetricsInstance != nil {
		otelMetricsInstance.mu.Lock()
		defer otelMetricsInstance.mu.Unlock()
		otelMetricsInstance.fileCachePath = path
	}
}

// registerMetrics creates all OTel metric instruments and registers their observation callbacks.
// CPU usage is computed as a percentage across all cores using /proc/stat.
// Memory usage is the process RSS read from /proc/self/status (matches top RES).
// Disk usage is read via syscall.Statfs on the file cache directory.
func (m *OtelMetrics) registerMetrics() error {
	// Common attributes attached to every metric observation
	commonAttrs := []attribute.KeyValue{
		attribute.String("tag", m.tag),
		attribute.String("mount_path", common.MountPath),
		attribute.String("hostname", common.GetHostName()),
		attribute.String("host_ip", common.GetHostIP()),
	}

	var err error

	// ---- CPU Usage Gauge (percentage across all cores) ----
	m.cpuUsageGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.cpu.usage_percent",
		otelmetric.WithDescription("Current CPU usage percentage across all cores"),
		otelmetric.WithUnit("%"),
	)
	if err != nil {
		return fmt.Errorf("failed to create CPU usage gauge: %w", err)
	}

	// ---- Memory Usage Gauge (bytes used by this process) ----
	m.memoryUsageGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.memory.usage_bytes",
		otelmetric.WithDescription("Current memory usage of the blobfuse2 process in bytes — matches RES in top"),
		otelmetric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create memory usage gauge: %w", err)
	}

	// ---- Total Memory Gauge (bytes available on the system) ----
	m.memoryTotalGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.memory.total_bytes",
		otelmetric.WithDescription("Total system memory in bytes"),
		otelmetric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create memory total gauge: %w", err)
	}

	// ---- Disk Usage Gauge (bytes used in file cache directory) ----
	m.diskUsageGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.disk.usage_bytes",
		otelmetric.WithDescription("Disk space used in the file cache directory in bytes"),
		otelmetric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create disk usage gauge: %w", err)
	}

	// ---- Disk Total Gauge (total bytes on the file cache volume) ----
	m.diskTotalGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.disk.total_bytes",
		otelmetric.WithDescription("Total disk space on the file cache volume in bytes"),
		otelmetric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("failed to create disk total gauge: %w", err)
	}

	// ---- Disk Usage Percent Gauge ----
	m.diskUsagePercentGauge, err = m.meter.Float64ObservableGauge(
		"blobfuse2.system.disk.usage_percent",
		otelmetric.WithDescription("Disk usage percentage in the file cache directory"),
		otelmetric.WithUnit("%"),
	)
	if err != nil {
		return fmt.Errorf("failed to create disk usage percent gauge: %w", err)
	}

	// Register a single batch callback for all gauges.
	// The OTel SDK calls this function during each collection cycle.
	_, err = m.meter.RegisterCallback(
		func(ctx context.Context, o otelmetric.Observer) error {
			attrSet := otelmetric.WithAttributes(commonAttrs...)

			// Collect CPU usage
			cpuPercent, cpuErr := m.getCPUUsage()
			if cpuErr == nil {
				o.ObserveFloat64(m.cpuUsageGauge, cpuPercent, attrSet)
			}

			// Collect memory usage (process RSS from /proc/self/status)
			memUsage, memTotal := m.getMemoryUsage()
			o.ObserveFloat64(m.memoryUsageGauge, memUsage, attrSet)
			o.ObserveFloat64(m.memoryTotalGauge, memTotal, attrSet)

			// Collect disk usage only when file cache path is configured
			m.mu.Lock()
			cachePath := m.fileCachePath
			m.mu.Unlock()

			if cachePath != "" {
				diskUsed, diskTotal, diskPercent, diskErr := m.getDiskUsage(cachePath)
				if diskErr == nil {
					o.ObserveFloat64(m.diskUsageGauge, diskUsed, attrSet)
					o.ObserveFloat64(m.diskTotalGauge, diskTotal, attrSet)
					o.ObserveFloat64(m.diskUsagePercentGauge, diskPercent, attrSet)
				}
			}

			return nil
		},
		m.cpuUsageGauge,
		m.memoryUsageGauge,
		m.memoryTotalGauge,
		m.diskUsageGauge,
		m.diskTotalGauge,
		m.diskUsagePercentGauge,
	)
	if err != nil {
		return fmt.Errorf("failed to register metric callback: %w", err)
	}

	return nil
}

// getCPUUsage reads /proc/stat and computes overall CPU usage percentage
// by comparing the delta between successive reads (idle vs total ticks).
// Returns a value between 0.0 and 100.0.
func (m *OtelMetrics) getCPUUsage() (float64, error) {
	total, idle, err := readProcStat()
	if err != nil {
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// On the first read we can only store the baseline and return 0
	if m.prevCPUTotal == 0 && m.prevCPUIdle == 0 {
		m.prevCPUTotal = total
		m.prevCPUIdle = idle
		return 0, nil
	}

	totalDelta := total - m.prevCPUTotal
	idleDelta := idle - m.prevCPUIdle

	m.prevCPUTotal = total
	m.prevCPUIdle = idle

	if totalDelta == 0 {
		return 0, nil
	}

	// CPU usage = 100 * (1 - idle_delta / total_delta)
	usage := 100.0 * (1.0 - idleDelta/totalDelta)
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}

	return usage, nil
}

// readProcStat parses /proc/stat to extract aggregate CPU counters.
// Returns (total, idle) jiffie counts. The "cpu" line in /proc/stat has format:
//
//	cpu  user nice system idle iowait irq softirq steal guest guest_nice
func readProcStat() (total, idle float64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0, fmt.Errorf("unexpected /proc/stat cpu line: %s", line)
			}
			// Parse each counter
			var sum float64
			for i := 1; i < len(fields); i++ {
				val, parseErr := strconv.ParseFloat(fields[i], 64)
				if parseErr != nil {
					return 0, 0, fmt.Errorf("failed to parse /proc/stat field: %w", parseErr)
				}
				sum += val
				if i == 4 { // 4th field (index 4) is idle
					idle = val
				}
			}
			return sum, idle, nil
		}
	}

	return 0, 0, fmt.Errorf("/proc/stat does not contain cpu line")
}

// getMemoryUsage returns the current process memory usage (RSS) and total system memory.
// Process memory is obtained from /proc/self/status (VmRSS) which matches the RES column
// in top/htop. This includes all memory held in RAM: Go heap, Go stacks, CGo/C allocations,
// libfuse buffers, mmap'd regions, etc.
// Total system memory is read from /proc/meminfo.
func (m *OtelMetrics) getMemoryUsage() (usageBytes float64, totalBytes float64) {
	usageBytes = float64(getProcessRSS())
	totalBytes = float64(getTotalSystemMemory())
	return
}

// getProcessRSS reads VmRSS from /proc/self/status to get the Resident Set Size
// of the current process in bytes. This includes all memory actually held in RAM:
// Go heap, Go stacks, CGo/C allocations, libfuse buffers, mmap'd regions, etc.
// This matches the RES column shown by top/htop.
func getProcessRSS() uint64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					// /proc/self/status reports VmRSS in kB
					return val * 1024
				}
			}
		}
	}
	return 0
}

// getTotalSystemMemory reads MemTotal from /proc/meminfo to determine total RAM in bytes.
func getTotalSystemMemory() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					// /proc/meminfo reports in kB
					return val * 1024
				}
			}
		}
	}
	return 0
}

// getDiskUsage returns disk space metrics for the given path using syscall.Statfs.
// Returns (usedBytes, totalBytes, usagePercent, error).
func (m *OtelMetrics) getDiskUsage(path string) (usedBytes, totalBytes, usagePercent float64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to statfs %s: %w", path, err)
	}

	// Total space = total blocks * block size
	total := float64(stat.Blocks) * float64(stat.Bsize)
	// Available space = available blocks * block size (available to unprivileged users)
	avail := float64(stat.Bavail) * float64(stat.Bsize)
	// Used = total - available
	used := total - avail

	var percent float64
	if total > 0 {
		percent = (used / total) * 100.0
	}

	return used, total, percent, nil
}
