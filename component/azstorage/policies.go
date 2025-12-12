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

	"golang.org/x/time/rate"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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
// Policy to limit the rate of requests
type rateLimitingPolicy struct {
	bandwidthLimiter *rate.Limiter
	opsLimiter       *rate.Limiter
}

func newRateLimitingPolicy(bytesPerSec int64, opsPerSec int64) policy.Policy {
	p := &rateLimitingPolicy{}

	if bytesPerSec > 0 {
		// Burst size is set to the limit itself to allow full utilization up to the limit
		p.bandwidthLimiter = rate.NewLimiter(rate.Limit(bytesPerSec), int(bytesPerSec))
		log.Info("RateLimitingPolicy : Bandwidth limit set to %d bytes/sec", bytesPerSec)
	}

	if opsPerSec > 0 {
		// Burst size is set to the limit itself
		p.opsLimiter = rate.NewLimiter(rate.Limit(opsPerSec), int(opsPerSec))
		log.Info("RateLimitingPolicy : Ops limit set to %d ops/sec", opsPerSec)
	}

	return p
}

func (p *rateLimitingPolicy) Do(req *policy.Request) (*http.Response, error) {
	ctx := req.Raw().Context()

	// Limit operations per second
	if p.opsLimiter != nil {
		// Wait for 1 token
		err := p.opsLimiter.Wait(ctx)
		if err != nil {
			log.Err("RateLimitingPolicy : Ops limit wait failed [%s]", err.Error())
			return nil, err
		}
	}

	// Limit bandwidth
	if p.bandwidthLimiter != nil {
		// Calculate the size of the request body
		var bodySize int64
		if req.Body() != nil {
			// This is an approximation. For exact size we might need to read the body which is not efficient.
			// For Seekable body we can get the size.
			if seeker, ok := req.Body().(interface {
				Seek(offset int64, whence int) (int64, error)
			}); ok {
				current, _ := seeker.Seek(0, 1)
				end, _ := seeker.Seek(0, 2)
				_, _ = seeker.Seek(current, 0)
				bodySize = end
			} else {
				// Fallback or estimation if needed.
				// For now, if we can't determine size, we might skip or assume a default.
				// However, for upload, the body should be seekable usually in SDK.
				// If not, we might just count the header size or similar?
				// Let's try to get Content-Length header if set.
				if req.Raw().ContentLength > 0 {
					bodySize = req.Raw().ContentLength
				}
			}
		}

		if bodySize > 0 {
			// Wait for tokens equal to body size
			// WaitN blocks until lim permits n events to happen.
			err := p.bandwidthLimiter.WaitN(ctx, int(bodySize))
			if err != nil {
				log.Err("RateLimitingPolicy : Bandwidth limit wait failed [%s]", err.Error())
				return nil, err
			}
		}
	}

	return req.Next()
}
