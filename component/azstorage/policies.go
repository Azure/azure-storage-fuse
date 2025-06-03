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
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
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

// In your metricsPolicy struct constructor or initializer, create the StatsCollector:
// var policystatscollector *stats_manager.StatsCollector
type metricsPolicy struct {
	mu                 sync.Mutex
	totalRequests      int
	informationalCount int
	successCount       int
	redirectCount      int
	clientErrorCount   int
	serverErrorCount   int
	failureCount       int64
	statsCollector     *stats_manager.StatsCollector
}

func NewMetricsPolicy() policy.Policy {
	if !common.EnableMonitoring {
		common.EnableMonitoring = true
	}

	// Create and return a metrics policy with its own stats collector
	statsCollector := stats_manager.NewStatsCollector("http-policy")
	return &metricsPolicy{
		statsCollector: statsCollector,
	}
}

func (p *metricsPolicy) Do(req *policy.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := req.Next()
	duration := time.Since(start)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalRequests++

	var statusCode int
	if resp != nil {
		statusCode = resp.StatusCode
	}

	switch {
	case statusCode >= 100 && statusCode < 200:
		p.informationalCount++
	case statusCode >= 200 && statusCode < 300:
		p.successCount++
	case statusCode >= 300 && statusCode < 400:
		p.redirectCount++
	case statusCode >= 400 && statusCode < 500:
		p.clientErrorCount++
		p.failureCount++
	case statusCode >= 500 && statusCode < 600:
		p.serverErrorCount++
		p.failureCount++
	default:
		if err != nil {
			p.failureCount++
		}
	}

	// Push updated metrics to the collector
	p.statsCollector.UpdateStats(stats_manager.Replace, "totalRequests", int64(p.totalRequests))
	p.statsCollector.UpdateStats(stats_manager.Replace, "informationalCount", int64(p.informationalCount))
	p.statsCollector.UpdateStats(stats_manager.Replace, "successCount", int64(p.successCount))
	p.statsCollector.UpdateStats(stats_manager.Replace, "redirectCount", int64(p.redirectCount))
	p.statsCollector.UpdateStats(stats_manager.Replace, "clientErrorCount", int64(p.clientErrorCount))
	p.statsCollector.UpdateStats(stats_manager.Replace, "serverErrorCount", int64(p.serverErrorCount))
	p.statsCollector.UpdateStats(stats_manager.Replace, "failureCount", p.failureCount)
	p.statsCollector.UpdateStats(stats_manager.Replace, "durationMs", duration.Milliseconds())

	return resp, err
}
