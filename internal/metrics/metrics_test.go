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
	"testing"
	"time"
)

func TestMetricsInitialization(t *testing.T) {
	// Test that metrics can be initialized without errors
	ctx := context.Background()

	// Initialize with disabled metrics
	err := InitMetrics(ctx, "", false)
	if err != nil {
		t.Fatalf("Failed to initialize disabled metrics: %v", err)
	}

	collector := GetGlobalCollector()
	if collector == nil {
		t.Fatal("Global collector should not be nil")
	}

	if collector.enabled {
		t.Error("Collector should be disabled when initialized with enabled=false")
	}
}

func TestMetricsCollectorCreation(t *testing.T) {
	// Test creating component-specific metrics collectors
	ctx := context.Background()

	// Initialize metrics (disabled for this test)
	_ = InitMetrics(ctx, "", false)

	// Create component-specific collectors
	collector1 := NewMetricsCollector("test_component_1")
	collector2 := NewMetricsCollector("test_component_2")

	if collector1 == nil {
		t.Fatal("Component collector 1 should not be nil")
	}

	if collector2 == nil {
		t.Fatal("Component collector 2 should not be nil")
	}
}

func TestMetricsOperations(t *testing.T) {
	// Test that metric operations don't panic when disabled
	ctx := context.Background()
	_ = InitMetrics(ctx, "", false)

	collector := NewMetricsCollector("test_component")

	// These should not panic even when metrics are disabled
	collector.RecordOperation("test_op", 1)
	collector.RecordCacheHit("test_cache")
	collector.RecordCacheMiss("test_cache")
	collector.RecordCacheEviction("test_cache", 1)
	collector.SetCacheUsage(100.0)
	collector.RecordAzureRequest("test_request")
	collector.RecordAzureResponse("test_response", 200, 0.5)
	collector.RecordBytesTransferred("test_transfer", 1024, "upload")
}

func TestMetricsShutdown(t *testing.T) {
	// Test that shutdown works correctly
	ctx := context.Background()
	_ = InitMetrics(ctx, "", false)

	collector := GetGlobalCollector()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := collector.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestErrorTypeClassification(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "client_error"},
		{401, "client_error"},
		{404, "client_error"},
		{499, "client_error"},
		{500, "server_error"},
		{502, "server_error"},
		{503, "server_error"},
		{599, "server_error"},
		{200, "unknown"},
		{300, "unknown"},
	}

	for _, tt := range tests {
		result := getErrorType(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getErrorType(%d) = %s, want %s", tt.statusCode, result, tt.expected)
		}
	}
}
