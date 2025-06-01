/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package azstorage

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-storage-fuse/v2/common"
)

// blobfuseTelemetryPolicy is a custom pipeline policy to prepend the blobfuse user agent string to the one coming from SDK.
// This is added in the PerCallPolicies which executes after the SDK's default telemetry policy.
type blobfuseTelemetryPolicy struct {
	telemetryValue string
}

// newBlobfuseTelemetryPolicy creates an object which prepends the blobfuse user agent string to the User-Agent request header
func newBlobfuseTelemetryPolicy(telemetryValue string) policy.Policy {
	return &blobfuseTelemetryPolicy{telemetryValue: telemetryValue}
}

func (p blobfuseTelemetryPolicy) Do(req *policy.Request) (*http.Response, error) {
	userAgent := p.telemetryValue

	// prepend the blobfuse user agent string
	if ua := req.Raw().Header.Get(common.UserAgentHeader); ua != "" {
		userAgent = fmt.Sprintf("%s %s", userAgent, ua)
	}
	req.Raw().Header.Set(common.UserAgentHeader, userAgent)
	return req.Next()
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Policy to override the service version if requested by user
type serviceVersionPolicy struct {
	serviceApiVersion string
}

func newServiceVersionPolicy(version string) policy.Policy {
	return &serviceVersionPolicy{
		serviceApiVersion: version,
	}
}

func (r *serviceVersionPolicy) Do(req *policy.Request) (*http.Response, error) {
	req.Raw().Header["x-ms-version"] = []string{r.serviceApiVersion}
	return req.Next()
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Policy to track all http requests and responses

type metricsPolicy struct {
	namespace    string
	requestCount int64
	failureCount int64
	mu           sync.Mutex
	exporter     *StatsExporter // Must be injected/initialized externally
}

type PolicyMetric struct {
	RequestCount int64  `json:"request_count"`
	FailureCount int64  `json:"failure_count"`
	DurationMs   int64  `json:"duration_ms"`
	Timestamp    string `json:"timestamp"`
}

func newmetricsPolicy(namespace string) policy.Policy {
	return &metricsPolicy{
		namespace: namespace,
		//monitorPusher: PushMetricsToAzureMonitor, // Function from stats_exports.go
	}
}

func (p *metricsPolicy) Do(req *policy.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := req.Next()
	duration := time.Since(start)

	p.mu.Lock()
	p.requestCount++
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		p.failureCount++
	}

	// Define metric struct
	metric := PolicyMetric{
		RequestCount: p.requestCount,
		FailureCount: p.failureCount,
		DurationMs:   duration.Milliseconds(),
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
	p.mu.Unlock()

	// Use the defined metric
	if p.exporter != nil {
		p.exporter.AddMonitorStats("AzureMonitorPolicy", metric.Timestamp, metric)
	}

	return resp, err
}
