// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitForSends(t *testing.T, r *reporter) {
	t.Helper()
	done := make(chan struct{})
	go func() { r.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("one-shot sends did not finish")
	}
}

func TestOneShotFailuresAreSilentAndDoNotStopFutureReports(t *testing.T) {
	for _, status := range []int{0, 408, 429, 503, 206} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int64
			r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, func(message string) {
				t.Errorf("delivery unexpectedly called diagnostic: %s", message)
			}, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				deadline, ok := req.Context().Deadline()
				if !ok || time.Until(deadline) > SendTimeout {
					t.Error("send lacks bounded deadline")
				}
				call := requests.Add(1)
				if call == 1 && status == 0 {
					return nil, errors.New("fixture transport failure")
				}
				code := http.StatusOK
				if call == 1 {
					code = status
				}
				return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"1"}},
					Body: io.NopCloser(strings.NewReader(""))}, nil
			})})
			defer r.close(0)
			r.report(testEvent())
			waitForSends(t, r)
			if requests.Load() != 1 {
				t.Fatal("failed event retried")
			}
			r.report(testEvent())
			waitForSends(t, r)
			if requests.Load() != 2 {
				t.Fatal("delivery failure stopped future reporting")
			}
		})
	}
}

func TestReporterRejectsUnapprovedMeasurementsBeforeDelivery(t *testing.T) {
	var requests, diagnostics atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, func(message string) {
		diagnostics.Add(1)
		if message != "telemetry: invalid or oversized event dropped" {
			t.Errorf("unexpected invalid-payload diagnostic: %s", message)
		}
	}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	defer r.close(0)
	for _, tc := range []struct {
		event, measurement string
	}{
		{"blobfuse.mount.attempted", "time_to_mount_ms"},
		{"blobfuse.mount.result", "final_uptime_seconds"},
		{"blobfuse.mount.stopped", "time_to_failure_ms"},
		{"blobfuse.mount.result", "private/fixture"},
	} {
		e := testEvent()
		e.Name = tc.event
		e.Measurements["time_to_mount_ms"] = 1
		e.Measurements[tc.measurement] = 2
		r.report(e)
	}
	waitForSends(t, r)
	if requests.Load() != 0 || diagnostics.Load() != 4 {
		t.Fatalf("invalid events reached delivery or lacked diagnostics: requests=%d diagnostics=%d", requests.Load(), diagnostics.Load())
	}
	e := testEvent()
	e.Measurements["time_to_mount_ms"] = 12.5
	r.report(e)
	waitForSends(t, r)
	if requests.Load() != 1 || diagnostics.Load() != 4 {
		t.Fatal("invalid measurement blocked a later approved event")
	}
}

func TestReporterCloseFinishesBeforeBudgetWhenSendsComplete(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, func(string) {},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			close(started)
			select {
			case <-release:
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})})
	defer r.close(0)
	r.report(testEvent())
	<-started
	returned := make(chan struct{})
	go func() { r.close(2 * time.Second); close(returned) }()
	select {
	case <-returned:
		close(release)
		t.Fatal("close did not wait for outstanding send")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("close waited for the full budget after delivery completed")
	}
}

func TestReporterCloseIsBoundedEvenIfTransportIgnoresCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, func(message string) {
		t.Errorf("background failure reached logger: %s", message)
	}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(started)
		<-release
		return nil, errors.New("fixture delayed failure")
	})})
	defer func() {
		close(release)
		waitForSends(t, r)
		r.close(0)
	}()
	r.report(testEvent())
	<-started
	returned := make(chan struct{})
	go func() { r.close(20 * time.Millisecond); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("close waited indefinitely for worker cleanup")
	}
	if r.ctx.Err() == nil {
		t.Fatal("close did not cancel outstanding request context")
	}
	if r.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects are allowed")
	}
}

func TestReporterCancellationInterruptsShutdownWait(t *testing.T) {
	started := make(chan struct{})
	var requests atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, func(string) {},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			close(started)
			<-req.Context().Done()
			return nil, req.Context().Err()
		})})
	defer r.close(0)
	r.report(testEvent())
	<-started
	returned := make(chan struct{})
	go func() { r.close(time.Hour); close(returned) }()
	r.cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt shutdown wait")
	}
	waitForSends(t, r)
	r.report(testEvent())
	if requests.Load() != 1 {
		t.Fatal("closed reporter accepted another event")
	}
}
