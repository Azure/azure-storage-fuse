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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
	mu sync.Mutex
}

func NewMetricsPolicy() policy.Policy {
	return &metricsPolicy{}
}

func (p *metricsPolicy) Do(req *policy.Request) (*http.Response, error) {
	resp, err := req.Next()

	p.mu.Lock()
	defer p.mu.Unlock()

	if azStatsCollector != nil {
		azStatsCollector.UpdateStats(stats_manager.Increment, TotalRequests, (int64)(1))

		var statusCode int
		if resp != nil {
			statusCode = resp.StatusCode
		}

		switch {
		case statusCode >= 100 && statusCode < 200:
			azStatsCollector.UpdateStats(stats_manager.Increment, InformationalCount, (int64)(1))
		case statusCode >= 200 && statusCode < 300:
			azStatsCollector.UpdateStats(stats_manager.Increment, SuccessCount, (int64)(1))
		case statusCode >= 300 && statusCode < 400:
			azStatsCollector.UpdateStats(stats_manager.Increment, RedirectCount, (int64)(1))
		case statusCode >= 400 && statusCode < 500:
			azStatsCollector.UpdateStats(stats_manager.Increment, ClientErrorCount, (int64)(1))
			azStatsCollector.UpdateStats(stats_manager.Increment, FailureCount, (int64)(1))
		case statusCode >= 500 && statusCode < 600:
			azStatsCollector.UpdateStats(stats_manager.Increment, ServerErrorCount, (int64)(1))
			azStatsCollector.UpdateStats(stats_manager.Increment, FailureCount, (int64)(1))
		default:
			if err != nil {
				azStatsCollector.UpdateStats(stats_manager.Increment, FailureCount, (int64)(1))
			}
		}
	} else {
		log.Warn("azStatsCollector is nil in metricsPolicy.Do - skipping stats update")
	}

	return resp, err
}
