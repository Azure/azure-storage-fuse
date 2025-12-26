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
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type policiesTestSuite struct {
	suite.Suite
}

type mockTransport struct{}

func (m *mockTransport) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
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

	req.Raw().Header.Set("Range", "bytes=0-99") // 100 bytes

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
	req.Raw().Header.Set("Range", "bytes=0-99")

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
	req.Raw().Header.Set("x-ms-range", "bytes=0-99") // 100 bytes

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

func TestPoliciesSuite(t *testing.T) {
	suite.Run(t, new(policiesTestSuite))
}
