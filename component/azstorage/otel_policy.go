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

// otel_policy.go implements an Azure SDK per-call pipeline policy that captures
// OpenTelemetry metrics for every HTTP request made to Azure Storage.
//
// Metrics emitted:
//   - blobfuse2.storage.request.duration_ms  (Float64Histogram) — round-trip latency in milliseconds
//   - blobfuse2.storage.request.count        (Int64Counter)     — total number of requests
//   - blobfuse2.storage.request.retry_count  (Int64Histogram)   — retry count per logical operation
//
// Each metric is tagged with:
//   - operation  : classified REST operation (e.g. ListBlobs, GetBlob, PutBlob)
//   - status     : success / error / throttled
//   - status_code: HTTP status code as a string (e.g. "200", "404")
//
// The policy sits in the PerCallPolicies chain, which means it wraps the entire
// retry loop. A single Do() invocation may result in multiple HTTP round-trips
// if the SDK retries. The policy measures total wall-clock time (including retries)
// and counts retries via the x-ms-client-request-id / x-ms-retry-count headers.
//
// Design decisions:
//   - Singleton: only one instance is created; getOtelPerCallPolicy() returns it.
//   - Lazy init: instruments are created on NewOtelRequestPolicy using the meter
//     from log.GetOtelMeter(). If metrics are not enabled, a nil policy is returned.
//   - Thread-safe: the policy is stateless per-request; histograms/counters are
//     concurrency-safe in the OTel SDK.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// otelRequestPolicy is an Azure SDK pipeline policy that records per-request
// OpenTelemetry metrics (latency, count, retries) for Azure Storage REST calls.
type otelRequestPolicy struct {
	// durationHist records request round-trip time in milliseconds (including retries).
	durationHist otelmetric.Float64Histogram

	// countCounter records the total number of requests issued.
	countCounter otelmetric.Int64Counter

	// retryHist records how many retries were performed for each logical operation.
	retryHist otelmetric.Int64Histogram
}

var (
	otelPolicyInstance *otelRequestPolicy
	otelPolicyMu       sync.Mutex
)

// NewOtelRequestPolicy creates (or returns the existing) singleton OTel request policy.
// It uses the meter from log.GetOtelMeter(). If the meter is nil (metrics not enabled),
// this function returns nil and the caller should skip adding the policy.
func NewOtelRequestPolicy() policy.Policy {
	otelPolicyMu.Lock()
	defer otelPolicyMu.Unlock()

	if otelPolicyInstance != nil {
		return otelPolicyInstance
	}

	meter := log.GetOtelMeter()
	if meter == nil {
		log.Info("OtelRequestPolicy::NewOtelRequestPolicy : OTel metrics not enabled, skipping request policy")
		return nil
	}

	p := &otelRequestPolicy{}

	var err error

	// Duration histogram: measures total wall-clock time of each request including retries
	p.durationHist, err = meter.Float64Histogram(
		"blobfuse2.storage.request.duration_ms",
		otelmetric.WithDescription("Round-trip latency of Azure Storage REST calls in milliseconds (includes retries)"),
		otelmetric.WithUnit("ms"),
	)
	if err != nil {
		log.Err("OtelRequestPolicy::NewOtelRequestPolicy : Failed to create duration histogram [%s]", err.Error())
		return nil
	}

	// Request count: total number of REST operations
	p.countCounter, err = meter.Int64Counter(
		"blobfuse2.storage.request.count",
		otelmetric.WithDescription("Total number of Azure Storage REST requests"),
		otelmetric.WithUnit("{request}"),
	)
	if err != nil {
		log.Err("OtelRequestPolicy::NewOtelRequestPolicy : Failed to create request counter [%s]", err.Error())
		return nil
	}

	// Retry histogram: number of retries per logical operation
	p.retryHist, err = meter.Int64Histogram(
		"blobfuse2.storage.request.retry_count",
		otelmetric.WithDescription("Number of retries per Azure Storage REST operation"),
		otelmetric.WithUnit("{retry}"),
	)
	if err != nil {
		log.Err("OtelRequestPolicy::NewOtelRequestPolicy : Failed to create retry histogram [%s]", err.Error())
		return nil
	}

	log.Info("OtelRequestPolicy::NewOtelRequestPolicy : OTel request metrics policy initialized successfully")
	otelPolicyInstance = p
	return otelPolicyInstance
}

// ResetOtelRequestPolicy resets the singleton for testing purposes.
func ResetOtelRequestPolicy() {
	otelPolicyMu.Lock()
	defer otelPolicyMu.Unlock()
	otelPolicyInstance = nil
}

// Do implements policy.Policy. It wraps the next policy in the chain, measuring
// the total wall-clock duration. Because this sits in PerCallPolicies, the call
// to req.Next() traverses the retry policy and all per-retry policies.
//
// The retry count is inferred from the x-ms-retry-count response header when
// available, or defaults to 0.
func (p *otelRequestPolicy) Do(req *policy.Request) (*http.Response, error) {
	start := time.Now()
	rawReq := req.Raw()

	// Classify the operation from the HTTP method + URL path + query params
	operation := classifyOperation(rawReq.Method, rawReq.URL.Path, rawReq.URL.RawQuery)

	log.Debug("OtelRequestPolicy::Do : operation=%s method=%s url=%s", operation, rawReq.Method, rawReq.URL.Path)

	// Execute the rest of the pipeline (including retries)
	resp, err := req.Next()

	durationMs := float64(time.Since(start).Milliseconds())

	// Determine status and status code
	statusCode := 0
	status := "error"
	retryCount := int64(0)

	if resp != nil {
		statusCode = resp.StatusCode
		status = classifyStatus(statusCode)

		// Azure SDK sets x-ms-client-request-id for correlating retries.
		// The retry count, if tracked at the transport level, can be inferred
		// from response headers or from the SDK's retry policy metadata.
		// Since the per-call policy wraps the retry loop, retry count is 0 here
		// unless we read it from the response. We check for a custom header that
		// some middleware may inject, or default to 0.
		if rc := resp.Header.Get("x-ms-retry-count"); rc != "" {
			_, _ = fmt.Sscanf(rc, "%d", &retryCount)
		}
	} else if err != nil {
		// Request completely failed (no response at all)
		status = "error"
		statusCode = 0
	}

	// Build attributes for all three metrics
	attrs := otelmetric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
		attribute.String("status_code", fmt.Sprintf("%d", statusCode)),
		attribute.String("mount_path", common.MountPath),
	)

	ctx := context.Background()
	p.durationHist.Record(ctx, durationMs, attrs)
	p.countCounter.Add(ctx, 1, attrs)
	p.retryHist.Record(ctx, retryCount, attrs)

	log.Debug("OtelRequestPolicy::Do : operation=%s status=%s code=%d duration_ms=%.1f retries=%d",
		operation, status, statusCode, durationMs, retryCount)

	return resp, err
}

// classifyOperation maps an HTTP method + URL path + query string to a human-readable
// Azure Storage operation name. This is best-effort: unknown patterns fall back to
// "METHOD /path_suffix".
//
// URL structure for Azure Blob Storage:
//
//	/<container>                                                      — container-level
//	/<container>/<blob>                                               — blob-level
//	/<container>/<blob>?comp=blocklist                                — PutBlockList / GetBlockList
//	/<container>/<blob>?comp=block&blockid=...                       — PutBlock / StageBlock
//	/<container>/<blob>?comp=page, comp=appendblock, comp=metadata   — various operations
//	/<container>?restype=container&comp=list                         — ListBlobs
//	/?comp=list                                                       — ListContainers
//
// Query parameters used for classification:
//
//	comp        — block, blocklist, page, appendblock, metadata, list, ...
//	restype     — container
func classifyOperation(method, urlPath, rawQuery string) string {
	query := strings.ToLower(rawQuery)
	comp := extractQueryParam(query, "comp")
	restype := extractQueryParam(query, "restype")

	// Count path segments (skip the leading empty segment from the leading /)
	// Example: "/container/blob/dir/file.txt" → ["container", "blob", "dir", "file.txt"] → segCount = 4
	segments := strings.Split(strings.TrimPrefix(urlPath, "/"), "/")
	segCount := len(segments)
	if segCount == 1 && segments[0] == "" {
		segCount = 0
	}

	switch method {
	case http.MethodGet:
		// GET /?comp=list → ListContainers
		if comp == "list" && restype == "" && segCount == 0 {
			return "ListContainers"
		}

		// GET /<container>?restype=container&comp=list → ListBlobs
		if comp == "list" && restype == "container" {
			return "ListBlobs"
		}

		// GET /<container>/<blob>?comp=blocklist → GetBlockList
		if comp == "blocklist" && segCount >= 2 {
			return "GetBlockList"
		}

		// GET /<container>/<blob>?comp=metadata → GetBlobMetadata
		if comp == "metadata" && segCount >= 2 {
			return "GetBlobMetadata"
		}
		// GET /<container>/<blob>?comp=tags → GetBlobTags
		if comp == "tags" && segCount >= 2 {
			return "GetBlobTags"
		}
		// GET /<container>?restype=container → GetContainerProperties
		if restype == "container" && comp == "" {
			return "GetContainerProperties"
		}
		// GET /<container>/<blob> (no comp) → GetBlob (download)
		if segCount >= 2 && comp == "" {
			return "GetBlob"
		}
		return "Get"

	case http.MethodPut:
		// PUT /<container>/<blob>?comp=block&blockid=... → PutBlock (StageBlock)
		if comp == "block" && segCount >= 2 {
			return "PutBlock"
		}
		// PUT /<container>/<blob>?comp=blocklist → PutBlockList (CommitBlockList)
		if comp == "blocklist" && segCount >= 2 {
			return "PutBlockList"
		}
		// PUT /<container>/<blob>?comp=page → PutPage
		if comp == "page" && segCount >= 2 {
			return "PutPage"
		}
		// PUT /<container>/<blob>?comp=appendblock → AppendBlock
		if comp == "appendblock" && segCount >= 2 {
			return "AppendBlock"
		}
		// PUT /<container>/<blob>?comp=metadata → SetBlobMetadata
		if comp == "metadata" && segCount >= 2 {
			return "SetBlobMetadata"
		}
		// PUT /<container>/<blob>?comp=tags → SetBlobTags
		if comp == "tags" && segCount >= 2 {
			return "SetBlobTags"
		}
		// PUT /<container>/<blob>?comp=copy → CopyBlob (start copy)
		if comp == "copy" && segCount >= 2 {
			return "CopyBlob"
		}
		// PUT /<container>/<blob>?comp=tier → SetBlobTier
		if comp == "tier" && segCount >= 2 {
			return "SetBlobTier"
		}
		// PUT /<container>?restype=container → CreateContainer
		if restype == "container" && segCount >= 1 {
			return "CreateContainer"
		}
		// PUT /<container>/<blob> (no comp) → PutBlob
		if segCount >= 2 && comp == "" {
			return "PutBlob"
		}
		return "Put"

	case http.MethodHead:
		// HEAD /<container>/<blob> → GetBlobProperties
		if segCount >= 2 {
			return "GetBlobProperties"
		}
		// HEAD /<container>?restype=container → GetContainerProperties
		if restype == "container" {
			return "GetContainerProperties"
		}
		return "Head"

	case http.MethodDelete:
		// DELETE /<container>/<blob> → DeleteBlob
		if segCount >= 2 {
			return "DeleteBlob"
		}
		// DELETE /<container>?restype=container → DeleteContainer
		if restype == "container" {
			return "DeleteContainer"
		}
		return "Delete"

	case http.MethodPatch:
		// PATCH is used by Data Lake operations
		// PATCH /<filesystem>/<path>?action=... → various DFS operations
		action := extractQueryParam(query, "action")
		switch action {
		case "append":
			return "DfsAppend"
		case "flush":
			return "DfsFlush"
		case "setaccesscontrol":
			return "DfsSetAccessControl"
		case "getaccesscontrol":
			return "DfsGetAccessControl"
		default:
			if action != "" {
				return "DfsPatch_" + action
			}
		}
		return "Patch"

	default:
		return method
	}
}

// classifyStatus maps an HTTP status code to a simplified status string.
func classifyStatus(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "success"
	case statusCode == 304:
		return "not_modified"
	case statusCode == 404:
		return "not_found"
	case statusCode == 409:
		return "conflict"
	case statusCode == 412:
		return "precondition_failed"
	case statusCode == 429:
		return "throttled"
	case statusCode >= 500:
		return "server_error"
	default:
		return "error"
	}
}

// extractQueryParam does a lightweight parse of a lowercased raw query string
// to pull out the value for a given key. This avoids a full url.ParseQuery()
// allocation on every request.
func extractQueryParam(rawQuery, key string) string {
	search := key + "="
	idx := strings.Index(rawQuery, search)
	if idx == -1 {
		return ""
	}

	val := rawQuery[idx+len(search):]
	if end := strings.IndexByte(val, '&'); end != -1 {
		val = val[:end]
	}

	return val
}
