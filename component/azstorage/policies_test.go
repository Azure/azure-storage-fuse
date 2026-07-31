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

package azstorage

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type mockTransport struct{}

func (m *mockTransport) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

type policiesTestSuite struct {
	suite.Suite
}

func (s *policiesTestSuite) SetupTest() {
	// Initialize the logger
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (s *policiesTestSuite) TearDownTest() {
	_ = log.Destroy()
}

func (s *policiesTestSuite) TestRateLimitingPolicy_OpsLimit() {
	assert := assert.New(s.T())

	// Limit to 1 op/sec. Burst will be 1 * 10 = 10 ops.
	p := newRateLimitingPolicy(-1, 1)
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerRetry: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	req, err := runtime.NewRequest(context.Background(), http.MethodGet, "http://localhost")
	assert.NoError(err)

	// Consume burst
	for i := 0; i < 10; i++ {
		_, err := pipeline.Do(req)
		assert.NoError(err)
	}

	// Next request should be delayed by ~1 sec
	start := time.Now()
	_, err = pipeline.Do(req)
	assert.NoError(err)
	duration := time.Since(start)

	// It should take at least 1 second (minus some tolerance)
	assert.GreaterOrEqual(duration, 900*time.Millisecond, "Expected delay of ~1s, got %v", duration)
}

func (s *policiesTestSuite) TestRateLimitingPolicy_BandwidthLimit() {
	assert := assert.New(s.T())

	// Limit 100 bytes/sec. Burst will be 100 * 10 = 1000 bytes.
	p := newRateLimitingPolicy(100, -1)
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerRetry: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	req, err := runtime.NewRequest(context.Background(), http.MethodGet, "http://localhost")
	assert.NoError(err)

	req.Raw().Header["Range"] = []string{"bytes=0-99"} // 100 bytes

	// Consume burst (10 requests of 100 bytes = 1000 bytes)
	for i := 0; i < 10; i++ {
		_, err := pipeline.Do(req)
		assert.NoError(err)
	}

	// Next request of 100 bytes should be delayed by ~1 sec
	start := time.Now()
	_, err = pipeline.Do(req)
	assert.NoError(err)
	duration := time.Since(start)

	assert.GreaterOrEqual(duration, 900*time.Millisecond, "Expected delay of ~1s, got %v", duration)
}

func (s *policiesTestSuite) TestRateLimitingPolicy_NoLimit() {
	assert := assert.New(s.T())

	p := newRateLimitingPolicy(-1, -1)
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerRetry: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	req, _ := runtime.NewRequest(context.Background(), http.MethodGet, "http://localhost")
	req.Raw().Header["Range"] = []string{"bytes=0-99"}

	start := time.Now()
	for i := 0; i < 20; i++ {
		_, err := pipeline.Do(req)
		assert.NoError(err)
	}
	duration := time.Since(start)

	// Should be very fast
	assert.Less(duration, 100*time.Millisecond, "Expected fast execution, got %v", duration)
}

func (s *policiesTestSuite) TestRateLimitingPolicy_BandwidthLimit_XMsRange() {
	assert := assert.New(s.T())

	// Limit 100 bytes/sec. Burst 1000 bytes.
	p := newRateLimitingPolicy(100, -1)
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerRetry: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	req, _ := runtime.NewRequest(context.Background(), http.MethodGet, "http://localhost")
	req.Raw().Header["x-ms-range"] = []string{"bytes=0-99"} // 100 bytes

	// Consume burst
	for i := 0; i < 10; i++ {
		_, err := pipeline.Do(req)
		assert.NoError(err)
	}

	// Next request should be delayed
	start := time.Now()
	_, err := pipeline.Do(req)
	assert.NoError(err)
	duration := time.Since(start)

	assert.GreaterOrEqual(duration, 900*time.Millisecond, "Expected delay of ~1s, got %v", duration)
}

func (s *policiesTestSuite) TestRateLimitingPolicy_BandwidthLimit_SkipNonGet() {
	assert := assert.New(s.T())

	// Limit 100 bytes/sec. burst 1000.
	p := newRateLimitingPolicy(100, -1)
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerRetry: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	// Create requests for checkable methods
	methods := []string{http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodHead}

	for _, method := range methods {
		req, _ := runtime.NewRequest(context.Background(), method, "http://localhost")
		// Even if we have a range header that implies a large payload
		// the policy should ignore it because it's not GET.
		req.Raw().Header["Range"] = []string{"bytes=0-999"} // 1000 bytes

		start := time.Now()
		// Execute multiple times - if limited, this would take ~10 seconds (1000 bytes * 10 / 100 bytes/sec)
		// But since we are skipping non-GET, it should be instant.
		for i := 0; i < 11; i++ {
			_, err := pipeline.Do(req)
			assert.NoError(err)
		}
		duration := time.Since(start)

		// Each request should be effectively instant, so total should be very fast
		assert.Less(duration, 100*time.Millisecond, "Expected fast execution for method %s, got %v", method, duration)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// layoutPolicy tests

func (s *policiesTestSuite) TestWithLayoutEndpoint() {
	assert := assert.New(s.T())

	ctx := context.Background()
	endpoint := "https://layout.blob.core.windows.net"

	ctxWithEndpoint := WithLayoutEndpoint(ctx, endpoint)

	value := ctxWithEndpoint.Value(ctxLayoutEndpointKey{})
	assert.NotNil(value)
	assert.Equal(endpoint, value.(string))
}

func (s *policiesTestSuite) TestWithLayoutEndpointEmptyString() {
	assert := assert.New(s.T())

	ctx := context.Background()
	ctxWithEndpoint := WithLayoutEndpoint(ctx, "")

	value := ctxWithEndpoint.Value(ctxLayoutEndpointKey{})
	assert.Nil(value)
}

func (s *policiesTestSuite) TestLayoutPolicyWithLayoutEndpoint() {
	assert := assert.New(s.T())

	layoutHost := "layout.blob.core.windows.net:443"
	originalHost := "original.blob.core.windows.net"

	p := NewLayoutPolicy()
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerCall: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	ctx := WithLayoutEndpoint(context.Background(), "https://"+layoutHost)
	req, err := runtime.NewRequest(ctx, http.MethodGet, "https://"+originalHost+"/container/blob")
	assert.NoError(err)

	_, err = pipeline.Do(req)
	assert.NoError(err)

	// Host header should be set to original host
	assert.Equal(originalHost, req.Raw().Host)
	// URL host should be redirected to the layout endpoint
	assert.Equal(layoutHost, req.Raw().URL.Host)
}

func (s *policiesTestSuite) TestLayoutPolicyWithLayoutEndpointEmpty() {
	assert := assert.New(s.T())

	originalHost := "original.blob.core.windows.net"

	p := NewLayoutPolicy()
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerCall: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	// Bypass WithLayoutEndpoint to set an empty string directly in context
	ctx := context.WithValue(context.Background(), ctxLayoutEndpointKey{}, "")
	req, err := runtime.NewRequest(ctx, http.MethodGet, "https://"+originalHost+"/container/blob")
	assert.NoError(err)

	_, err = pipeline.Do(req)
	assert.NoError(err)

	// URL host should remain unchanged when endpoint is empty
	assert.Equal(originalHost, req.Raw().URL.Host)
}

func (s *policiesTestSuite) TestLayoutPolicyWithLayoutEndpointInvalid() {
	assert := assert.New(s.T())

	p := NewLayoutPolicy()
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerCall: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	// Use an invalid URL that will fail url.Parse
	ctx := context.WithValue(context.Background(), ctxLayoutEndpointKey{}, "://invalid-url")
	req, err := runtime.NewRequest(ctx, http.MethodGet, "https://original.blob.core.windows.net/container/blob")
	assert.NoError(err)

	_, err = pipeline.Do(req)
	assert.Error(err)
}

func (s *policiesTestSuite) TestLayoutPolicyWithoutLayoutEndpoint() {
	assert := assert.New(s.T())

	originalHost := "original.blob.core.windows.net"

	p := NewLayoutPolicy()
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{
		PerCall: []policy.Policy{p},
	}, &policy.ClientOptions{Transport: &mockTransport{}})

	req, err := runtime.NewRequest(context.Background(), http.MethodGet, "https://"+originalHost+"/container/blob")
	assert.NoError(err)

	originalURLHost := req.Raw().URL.Host
	originalReqHost := req.Raw().Host

	_, err = pipeline.Do(req)
	assert.NoError(err)

	// Without a layout endpoint in context, Host and URL host should remain unchanged
	assert.Equal(originalURLHost, req.Raw().URL.Host)
	assert.Equal(originalReqHost, req.Raw().Host)
}

func (s *policiesTestSuite) TestNewLayoutPolicy() {
	assert := assert.New(s.T())

	p := NewLayoutPolicy()
	assert.NotNil(p)
	_, ok := p.(*layoutPolicy)
	assert.True(ok)
}

func TestPoliciesSuite(t *testing.T) {
	suite.Run(t, new(policiesTestSuite))
}
