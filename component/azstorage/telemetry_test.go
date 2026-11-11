// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azstorage

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

type testTelemetryTransport func(*http.Request) (*http.Response, error)

func (fn testTelemetryTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestTelemetrySDKRetryScope(t *testing.T) {
	var mu sync.Mutex
	var snapshots []map[string]float64
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []struct {
			Data struct {
				BaseData struct {
					Name         string             `json:"name"`
					Measurements map[string]float64 `json:"measurements"`
				} `json:"baseData"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
		}
		mu.Lock()
		for _, e := range batch {
			if e.Data.BaseData.Name == "blobfuse.mount.snapshot" {
				snapshots = append(snapshots, e.Data.BaseData.Measurements)
			}
		}
		mu.Unlock()
	}))
	defer destination.Close()
	s, err := telemetry.Start(telemetry.Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + destination.URL,
		StateDirectory:   filepath.Join(t.TempDir(), "microsoft-telemetry"), HTTPClient: destination.Client(),
		MountPath: "/test/sdk-retry-mount",
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close(nil)
	telemetry.Ready(nil, nil)
	var attempts atomic.Int64
	client := &http.Client{Transport: observedTransport{next: testTelemetryTransport(func(req *http.Request) (*http.Response, error) {
		status := 503
		if attempts.Add(1) > 1 {
			status = 200
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Request: req,
			Body: io.NopCloser(strings.NewReader(""))}, nil
	})}}
	pipeline := runtime.NewPipeline("test", "v1", runtime.PipelineOptions{}, &azcore.ClientOptions{
		Transport:        client,
		Retry:            policy.RetryOptions{MaxRetries: 1, RetryDelay: time.Millisecond},
		PerCallPolicies:  []policy.Policy{requestObservationPolicy{}},
		PerRetryPolicies: []policy.Policy{attemptObservationPolicy{}},
	})
	req, err := runtime.NewRequest(context.Background(), http.MethodGet, "https://storage.example.test")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := pipeline.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	telemetry.TeardownComplete()
	s.Close(nil)
	mu.Lock()
	defer mu.Unlock()
	if len(snapshots) == 0 {
		t.Fatal("no final snapshot")
	}
	m := snapshots[len(snapshots)-1]
	for k, want := range map[string]float64{"runtime_http_response_503": 1, "runtime_http_response_200": 1,
		"runtime_sdk_retry_attempts": 1, "runtime_sdk_retried_requests": 1,
		"runtime_sdk_requests": 1, "runtime_sdk_terminal_failures": 0} {
		if m[k] != want {
			t.Fatalf("%s=%v want %v", k, m[k], want)
		}
	}
	if _, ok := m["retry_exhausted"]; ok {
		t.Fatal("invented retry exhaustion")
	}
}

func TestTelemetryErrorCategories(t *testing.T) {
	for _, tc := range []struct {
		err      error
		category int
	}{
		{&net.DNSError{Err: "failed", Name: "never-export-this"}, 0},
		{x509.UnknownAuthorityError{}, 1},
		{context.DeadlineExceeded, 2},
		{fmt.Errorf("wrapped: %w", syscall.ECONNRESET), 3},
		{context.Canceled, 4},
		{errors.New("unclassified private text"), 5},
	} {
		if got := networkCategory(tc.err); got != tc.category {
			t.Fatalf("%T: category %d want %d", tc.err, got, tc.category)
		}
	}
}

type telemetryErrorReader struct{ err error }

func (r telemetryErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestTelemetryNetworkCounters(t *testing.T) {
	snapshots := make(chan map[string]float64, 1)
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []struct {
			Data struct {
				Base struct {
					Name         string             `json:"name"`
					Measurements map[string]float64 `json:"measurements"`
				} `json:"baseData"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		for _, item := range batch {
			if item.Data.Base.Name == "blobfuse.mount.snapshot" {
				snapshots <- item.Data.Base.Measurements
			}
		}
	}))
	defer destination.Close()
	s, err := telemetry.Start(telemetry.Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + destination.URL,
		StateDirectory:   filepath.Join(t.TempDir(), "microsoft-telemetry"), MountPath: "/test/network",
		HTTPClient: destination.Client(),
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	req, err := http.NewRequest(http.MethodGet, "https://storage.example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	startup := observedTransport{next: testTelemetryTransport(func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "fixture"}
	})}
	if _, err := startup.RoundTrip(req); err == nil {
		t.Fatal("startup DNS fixture did not fail")
	}
	telemetry.Ready(nil, nil)
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"dns", &net.DNSError{Err: "fixture"}},
		{"tls", x509.UnknownAuthorityError{}},
		{"timeout", context.DeadlineExceeded},
		{"reset", fmt.Errorf("wrapped: %w", syscall.ECONNRESET)},
		{"cancelled", context.Canceled},
		{"other", errors.New("private fixture text")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := observedTransport{next: testTelemetryTransport(func(*http.Request) (*http.Response, error) {
				return nil, tc.err
			})}
			if _, err := transport.RoundTrip(req); err == nil {
				t.Fatal("transport fixture did not fail")
			}
			transport.next = testTelemetryTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(telemetryErrorReader{tc.err})}, nil
			})
			response, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			for range 2 {
				if _, err := response.Body.Read(make([]byte, 1)); err == nil {
					t.Fatal("body fixture did not fail")
				}
			}
		})
	}
	eofTransport := observedTransport{next: testTelemetryTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	response, err := eofTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := response.Body.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("expected normal EOF: %v", err)
	}
	response.Body.Close()
	telemetry.TeardownComplete()
	s.Close(nil)
	select {
	case got := <-snapshots:
		for key, want := range map[string]float64{
			"startup_transport_dns": 1, "startup_http_no_response": 1,
			"runtime_transport_dns": 2, "runtime_transport_tls": 2,
			"runtime_transport_timeout": 2, "runtime_transport_reset": 2,
			"runtime_transport_cancelled": 2, "runtime_transport_other": 2,
			"runtime_transport_body_read": 6, "runtime_http_no_response": 6,
			"runtime_http_transport_attempts": 13, "runtime_http_response_200": 7,
		} {
			if got[key] != want {
				t.Errorf("%s=%v, want %v", key, got[key], want)
			}
		}
	default:
		t.Fatal("final snapshot was not delivered")
	}
}
