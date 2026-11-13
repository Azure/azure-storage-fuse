// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testEvent() Event {
	return Event{Name: "blobfuse.mount.result", Time: time.Now().UTC(),
		Properties: map[string]string{"schema_version": "1", "event_id": newID(),
			"mount_session_id": newID()},
		Measurements: map[string]float64{}}
}

func TestEncodeRejectsUnapprovedSchema(t *testing.T) {
	e := testEvent()
	e.Name = "unexpected.event"
	if _, err := encode(e, "test-key"); err == nil {
		t.Fatal("encoder accepted an unapproved event")
	}
	e.Name = "blobfuse.mount.result"
	for _, field := range []string{"account_name", "raw_error"} {
		e.Properties[field] = "not-allowed"
		if _, err := encode(e, "test-key"); err == nil {
			t.Fatalf("encoder accepted unapproved property %s", field)
		}
		delete(e.Properties, field)
	}
}

func TestEncodedEventSizeBoundary(t *testing.T) {
	const limit = 32 * 1024
	e := testEvent()
	e.Properties["failure_category"] = ""
	// JSON escaping reaches the envelope cap using only approved fields.
	for _, field := range []string{"pipeline_components", "auth_mode", "distro_name", "distro_version", "kernel_release"} {
		e.Properties[field] = strings.Repeat("<", 1024)
	}
	body, err := encode(e, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	padding := limit - len(body)
	if padding < 0 || padding/6+padding%6 >= 1024 {
		t.Fatalf("fixture cannot reach size boundary within property limits: padding=%d", padding)
	}
	e.Properties["failure_category"] = strings.Repeat("<", padding/6) + strings.Repeat("x", padding%6)
	body, err = encode(e, "test-key")
	if err != nil || len(body) != limit {
		t.Fatalf("32-KiB encoded event rejected: bytes=%d err=%v", len(body), err)
	}
	e.Properties["failure_category"] += "x"
	if _, err := encode(e, "test-key"); err == nil || err.Error() != "telemetry event exceeds limit" {
		t.Fatalf("expected event-size rejection above 32 KiB: %v", err)
	}
}

func TestMeasurementNamesAreEventSpecific(t *testing.T) {
	for _, tc := range []struct {
		event   string
		allowed map[string]bool
	}{
		{event: "blobfuse.mount.attempted"},
		{event: "blobfuse.mount.result", allowed: map[string]bool{
			"time_to_mount_ms": true, "time_to_failure_ms": true,
		}},
		{event: "blobfuse.mount.stopped", allowed: map[string]bool{
			"final_uptime_seconds": true, "last_known_uptime_seconds": true,
		}},
	} {
		t.Run(tc.event, func(t *testing.T) {
			e := testEvent()
			e.Name = tc.event
			for _, measurements := range []map[string]float64{nil, {}} {
				e.Measurements = measurements
				if _, err := encode(e, "test-key"); err != nil {
					t.Fatalf("measurement-free event rejected: %v", err)
				}
			}
			for _, name := range []string{
				"time_to_mount_ms", "time_to_failure_ms", "final_uptime_seconds", "last_known_uptime_seconds",
				"counter_0001", "", "Time_to_mount_ms", "private/fixture",
			} {
				t.Run(name, func(t *testing.T) {
					e.Measurements = map[string]float64{name: 12.5}
					body, err := encode(e, "test-key")
					if !tc.allowed[name] {
						if err == nil || err.Error() != "unapproved telemetry measurement" || body != nil {
							t.Fatalf("unapproved measurement was not rejected: err=%v body=%s", err, body)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					var batch []envelope
					if err := json.Unmarshal(body, &batch); err != nil {
						t.Fatal(err)
					}
					if len(batch) != 1 || len(batch[0].Data.Data.Measurements) != 1 ||
						batch[0].Data.Data.Measurements[name] != 12.5 {
						t.Fatalf("approved measurement changed during encoding: %s", body)
					}
				})
			}
		})
	}
}

func TestMeasurementValuesMustBeFinite(t *testing.T) {
	for _, tc := range []struct {
		event, measurement string
	}{
		{"blobfuse.mount.result", "time_to_mount_ms"},
		{"blobfuse.mount.result", "time_to_failure_ms"},
		{"blobfuse.mount.stopped", "final_uptime_seconds"},
		{"blobfuse.mount.stopped", "last_known_uptime_seconds"},
	} {
		t.Run(tc.measurement, func(t *testing.T) {
			e := testEvent()
			e.Name = tc.event
			for _, value := range []float64{0, 12.5, math.MaxFloat64, math.NaN(), math.Inf(1), math.Inf(-1)} {
				e.Measurements = map[string]float64{tc.measurement: value}
				body, err := encode(e, "test-key")
				if math.IsNaN(value) || math.IsInf(value, 0) {
					if err == nil || err.Error() != "invalid telemetry measurement" || body != nil {
						t.Fatalf("nonfinite measurement accepted: value=%v err=%v", value, err)
					}
				} else if err != nil {
					t.Fatalf("finite measurement rejected: value=%v err=%v", value, err)
				}
			}
		})
	}
}

func TestDestinationValidation(t *testing.T) {
	for _, value := range []string{
		"", "InstrumentationKey=x;IngestionEndpoint=http://localhost",
		"InstrumentationKey=x;IngestionEndpoint=https://user:pass@host",
		"InstrumentationKey=x;IngestionEndpoint=https://host?q=x",
		"InstrumentationKey=x;IngestionEndpoint=https://host;Authorization=aad",
	} {
		if _, err := parseDestination(value); err == nil {
			t.Fatalf("accepted unsafe destination: %s", value)
		}
	}
	d, err := parseDestination("InstrumentationKey=x;IngestionEndpoint=https://example.com/")
	if err != nil || d.endpoint != "https://example.com/v2.1/track" {
		t.Fatalf("destination: %+v %v", d, err)
	}
}

func TestResponseHandling(t *testing.T) {
	status := atomic.Int64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(int(status.Load()))
	}))
	defer server.Close()
	r := &reporter{dest: destination{endpoint: server.URL}, client: server.Client()}
	for _, code := range []int{200, 400, 429, 503, 206} {
		status.Store(int64(code))
		err := r.send(context.Background(), []byte("[]"))
		if (err == nil) != (code == 200) {
			t.Fatalf("status %d: err=%v", code, err)
		}
	}
	t.Run("partial-response-body-not-read", func(t *testing.T) {
		body := strings.NewReader(`{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":429}]}` +
			strings.Repeat(" ", 100000))
		r := &reporter{dest: destination{endpoint: "https://test"}, client: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 206, Header: http.Header{}, Body: io.NopCloser(body)}, nil
			}),
		}}
		if err := r.send(context.Background(), []byte("[]")); err == nil {
			t.Fatal("expected item rejection")
		}
		if read := body.Size() - int64(body.Len()); read != 0 {
			t.Fatalf("one-shot rejection unnecessarily read %d response bytes", read)
		}
	})
}

func TestEncodePreservesEvent(t *testing.T) {
	e := testEvent()
	e.Properties["outcome"] = "success"
	e.Measurements["time_to_mount_ms"] = 25
	before, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	body, err := encode(e, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	var batch []envelope
	if err := json.Unmarshal(body, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch) != 1 || batch[0].Data.Data.Properties["outcome"] != "success" ||
		batch[0].Data.Data.Measurements["time_to_mount_ms"] != 25 {
		t.Fatalf("event content changed during encoding: %s", body)
	}
	after, err := json.Marshal(e)
	if err != nil || string(before) != string(after) {
		t.Fatal("encoding mutated the input event:", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	var mu sync.Mutex
	var events []envelope
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var incoming []envelope
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			t.Error(err)
		}
		mu.Lock()
		events = append(events, incoming...)
		mu.Unlock()
	}))
	defer server.Close()
	s, err := Start(Options{ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		HTTPClient: server.Client(),
		Version:    "test"})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
	Ready(map[string]string{"fuse_api_major": "3"})
	TeardownComplete()
	s.Close()
	if Current() != nil {
		t.Fatal("session still active")
	}
	mu.Lock()
	defer mu.Unlock()
	kinds := map[string]int{}
	for _, e := range events {
		kinds[e.Data.Data.Name]++
		if strings.Contains(fmt.Sprint(e), "MUST-NOT-BE-SENT") {
			t.Fatal("unapproved configuration exported")
		}
	}
	if kinds["blobfuse.mount.attempted"] != 1 || kinds["blobfuse.mount.result"] != 1 ||
		kinds["blobfuse.mount.stopped"] != 1 {
		t.Fatalf("unexpected lifecycle: %+v", kinds)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestBlankDestinationAndFingerprint(t *testing.T) {
	for _, options := range []Options{{}, {Disabled: true}} {
		s, err := Start(options)
		if err != nil || s != nil {
			t.Fatalf("inactive session: %v", err)
		}
	}
	if fingerprint(map[string]string{"a": "1", "b": "2"}) != fingerprint(map[string]string{"b": "2", "a": "1"}) {
		t.Fatal("fingerprint depends on insertion order")
	}
}
