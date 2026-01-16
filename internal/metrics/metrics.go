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

package metrics

import (
	"context"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	meterName = "github.com/Azure/azure-storage-fuse/v2"
)

// MetricsCollector provides OpenTelemetry metrics collection for blobfuse2
type MetricsCollector struct {
	meter         metric.Meter
	meterProvider *sdkmetric.MeterProvider
	componentName string
	enabled       bool
	shutdownOnce  sync.Once

	// Operation counters
	operationCounter metric.Int64Counter

	// Cache metrics
	cacheUsageGauge      metric.Float64ObservableGauge
	cacheHitCounter      metric.Int64Counter
	cacheMissCounter     metric.Int64Counter
	cacheEvictionCounter metric.Int64Counter

	// System metrics
	memoryUsageGauge metric.Int64ObservableGauge
	cpuUsageGauge    metric.Float64ObservableGauge

	// Network metrics (Azure Storage)
	requestCounter   metric.Int64Counter
	responseCounter  metric.Int64Counter
	errorCounter     metric.Int64Counter
	bytesTransferred metric.Int64Counter
	requestDuration  metric.Float64Histogram

	// Cache state tracking
	cacheUsageMB    float64
	cacheUsageMutex sync.RWMutex
}

var (
	globalCollector *MetricsCollector
	initOnce        sync.Once
)

// InitMetrics initializes the OpenTelemetry metrics infrastructure
func InitMetrics(ctx context.Context, endpoint string, enabled bool) error {
	var initErr error

	initOnce.Do(func() {
		if !enabled || endpoint == "" {
			log.Info("metrics::InitMetrics : Metrics collection disabled")
			globalCollector = &MetricsCollector{enabled: false}
			return
		}

		// Create OTLP metric exporter
		exporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			log.Err("metrics::InitMetrics : Failed to create OTLP exporter [%v]", err)
			initErr = err
			globalCollector = &MetricsCollector{enabled: false}
			return
		}

		// Create resource with service information
		res, err := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String("blobfuse2"),
				semconv.ServiceVersionKey.String(common.Blobfuse2Version),
			),
		)
		if err != nil {
			log.Err("metrics::InitMetrics : Failed to create resource [%v]", err)
			initErr = err
			globalCollector = &MetricsCollector{enabled: false}
			return
		}

		// Create meter provider
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(10*time.Second))),
		)

		otel.SetMeterProvider(meterProvider)

		globalCollector = &MetricsCollector{
			meterProvider: meterProvider,
			meter:         meterProvider.Meter(meterName),
			enabled:       true,
		}

		// Initialize metrics
		if err := globalCollector.initializeMetrics(); err != nil {
			log.Err("metrics::InitMetrics : Failed to initialize metrics [%v]", err)
			initErr = err
			return
		}

		log.Info("metrics::InitMetrics : OpenTelemetry metrics initialized successfully")
	})

	return initErr
}

// GetGlobalCollector returns the global metrics collector instance
func GetGlobalCollector() *MetricsCollector {
	if globalCollector == nil {
		return &MetricsCollector{enabled: false}
	}
	return globalCollector
}

// NewMetricsCollector creates a component-specific metrics collector
func NewMetricsCollector(componentName string) *MetricsCollector {
	global := GetGlobalCollector()
	if !global.enabled {
		return &MetricsCollector{enabled: false}
	}

	return &MetricsCollector{
		meter:                global.meter,
		meterProvider:        global.meterProvider,
		componentName:        componentName,
		enabled:              true,
		operationCounter:     global.operationCounter,
		cacheUsageGauge:      global.cacheUsageGauge,
		cacheHitCounter:      global.cacheHitCounter,
		cacheMissCounter:     global.cacheMissCounter,
		cacheEvictionCounter: global.cacheEvictionCounter,
		memoryUsageGauge:     global.memoryUsageGauge,
		cpuUsageGauge:        global.cpuUsageGauge,
		requestCounter:       global.requestCounter,
		responseCounter:      global.responseCounter,
		errorCounter:         global.errorCounter,
		bytesTransferred:     global.bytesTransferred,
		requestDuration:      global.requestDuration,
	}
}

// initializeMetrics creates all metric instruments
func (mc *MetricsCollector) initializeMetrics() error {
	var err error

	// Operation counter
	mc.operationCounter, err = mc.meter.Int64Counter(
		"blobfuse.operations",
		metric.WithDescription("Number of filesystem operations"),
		metric.WithUnit("{operations}"),
	)
	if err != nil {
		return err
	}

	// Cache hit counter
	mc.cacheHitCounter, err = mc.meter.Int64Counter(
		"blobfuse.cache.hits",
		metric.WithDescription("Number of cache hits"),
		metric.WithUnit("{hits}"),
	)
	if err != nil {
		return err
	}

	// Cache miss counter
	mc.cacheMissCounter, err = mc.meter.Int64Counter(
		"blobfuse.cache.misses",
		metric.WithDescription("Number of cache misses"),
		metric.WithUnit("{misses}"),
	)
	if err != nil {
		return err
	}

	// Cache eviction counter
	mc.cacheEvictionCounter, err = mc.meter.Int64Counter(
		"blobfuse.cache.evictions",
		metric.WithDescription("Number of cache evictions"),
		metric.WithUnit("{evictions}"),
	)
	if err != nil {
		return err
	}

	// Cache usage gauge
	mc.cacheUsageGauge, err = mc.meter.Float64ObservableGauge(
		"blobfuse.cache.usage_mb",
		metric.WithDescription("Current cache usage in megabytes"),
		metric.WithUnit("MB"),
		metric.WithFloat64Callback(func(ctx context.Context, observer metric.Float64Observer) error {
			mc.cacheUsageMutex.RLock()
			usage := mc.cacheUsageMB
			mc.cacheUsageMutex.RUnlock()
			observer.Observe(usage)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Memory usage gauge
	mc.memoryUsageGauge, err = mc.meter.Int64ObservableGauge(
		"blobfuse.system.memory_bytes",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			observer.Observe(int64(m.Alloc))
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// CPU usage gauge
	mc.cpuUsageGauge, err = mc.meter.Float64ObservableGauge(
		"blobfuse.system.cpu_usage",
		metric.WithDescription("Current CPU usage percentage"),
		metric.WithUnit("%"),
		metric.WithFloat64Callback(func(ctx context.Context, observer metric.Float64Observer) error {
			var usage syscall.Rusage
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
				cpuTime := float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6 +
					float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6
				observer.Observe(cpuTime)
			}
			return nil
		}),
	)
	if err != nil {
		return err
	}

	// Request counter
	mc.requestCounter, err = mc.meter.Int64Counter(
		"blobfuse.azure.requests",
		metric.WithDescription("Number of requests to Azure Storage"),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return err
	}

	// Response counter
	mc.responseCounter, err = mc.meter.Int64Counter(
		"blobfuse.azure.responses",
		metric.WithDescription("Number of responses from Azure Storage"),
		metric.WithUnit("{responses}"),
	)
	if err != nil {
		return err
	}

	// Error counter
	mc.errorCounter, err = mc.meter.Int64Counter(
		"blobfuse.azure.errors",
		metric.WithDescription("Number of errors from Azure Storage"),
		metric.WithUnit("{errors}"),
	)
	if err != nil {
		return err
	}

	// Bytes transferred counter
	mc.bytesTransferred, err = mc.meter.Int64Counter(
		"blobfuse.azure.bytes_transferred",
		metric.WithDescription("Number of bytes transferred to/from Azure Storage"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	// Request duration histogram
	mc.requestDuration, err = mc.meter.Float64Histogram(
		"blobfuse.azure.request_duration",
		metric.WithDescription("Duration of Azure Storage requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordOperation records a filesystem operation
func (mc *MetricsCollector) RecordOperation(operation string, count int64) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.operationCounter.Add(context.Background(), count, metric.WithAttributes(attrs...))
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit(cacheType string) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("cache_type", cacheType),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.cacheHitCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss(cacheType string) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("cache_type", cacheType),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.cacheMissCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordCacheEviction records a cache eviction
func (mc *MetricsCollector) RecordCacheEviction(cacheType string, count int64) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("cache_type", cacheType),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.cacheEvictionCounter.Add(context.Background(), count, metric.WithAttributes(attrs...))
}

// SetCacheUsage sets the current cache usage in MB
func (mc *MetricsCollector) SetCacheUsage(usageMB float64) {
	if !mc.enabled {
		return
	}

	mc.cacheUsageMutex.Lock()
	mc.cacheUsageMB = usageMB
	mc.cacheUsageMutex.Unlock()
}

// RecordAzureRequest records an Azure Storage request
func (mc *MetricsCollector) RecordAzureRequest(operation string) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.requestCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
}

// RecordAzureResponse records an Azure Storage response
func (mc *MetricsCollector) RecordAzureResponse(operation string, statusCode int, duration float64) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.Int("status_code", statusCode),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.responseCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))

	// Record duration
	mc.requestDuration.Record(context.Background(), duration, metric.WithAttributes(attrs...))

	// Record error if status code indicates failure
	if statusCode >= 400 {
		errorAttrs := []attribute.KeyValue{
			attribute.String("operation", operation),
			attribute.String("error_type", getErrorType(statusCode)),
			attribute.Int("status_code", statusCode),
		}
		if mc.componentName != "" {
			errorAttrs = append(errorAttrs, attribute.String("component", mc.componentName))
		}
		mc.errorCounter.Add(context.Background(), 1, metric.WithAttributes(errorAttrs...))
	}
}

// RecordBytesTransferred records bytes transferred to/from Azure Storage
func (mc *MetricsCollector) RecordBytesTransferred(operation string, bytes int64, direction string) {
	if !mc.enabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("direction", direction),
	}
	if mc.componentName != "" {
		attrs = append(attrs, attribute.String("component", mc.componentName))
	}

	mc.bytesTransferred.Add(context.Background(), bytes, metric.WithAttributes(attrs...))
}

// Shutdown gracefully shuts down the metrics collector
func (mc *MetricsCollector) Shutdown(ctx context.Context) error {
	if !mc.enabled || mc.meterProvider == nil {
		return nil
	}

	var shutdownErr error
	mc.shutdownOnce.Do(func() {
		shutdownErr = mc.meterProvider.Shutdown(ctx)
		if shutdownErr != nil {
			log.Err("metrics::Shutdown : Failed to shutdown meter provider [%v]", shutdownErr)
		}
	})

	return shutdownErr
}

// getErrorType categorizes HTTP status codes into error types
func getErrorType(statusCode int) string {
	if statusCode >= 400 && statusCode < 500 {
		return "client_error"
	} else if statusCode >= 500 {
		return "server_error"
	}
	return "unknown"
}
