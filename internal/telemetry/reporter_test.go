// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	for _, status := range []int{0, 200, 400, 408, 429, 503, 206} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int64
			body := strings.NewReader("partial response must not be read")
			r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
				&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
					if code == http.StatusPartialContent {
						return &http.Response{StatusCode: code, Body: io.NopCloser(body)}, nil
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"1"}},
						Body: io.NopCloser(strings.NewReader(""))}, nil
				})})
			defer r.close(0)
			if err := r.send(context.Background(), []byte("[]")); (err == nil) != (status == http.StatusOK) {
				t.Fatalf("wrong delivery result for status %d: %v", status, err)
			}
			requests.Store(0)
			require.NoError(t, r.report(testEvent()))
			waitForSends(t, r)
			if requests.Load() != 1 {
				t.Fatal("failed event retried")
			}
			if body.Len() != int(body.Size()) {
				t.Fatal("one-shot rejection unnecessarily read the partial response")
			}
			require.NoError(t, r.report(testEvent()))
			waitForSends(t, r)
			if requests.Load() != 2 {
				t.Fatal("delivery failure stopped future reporting")
			}
		})
	}
}

func TestSessionRejectsUnapprovedMeasurementsBeforeDelivery(t *testing.T) {
	var requests, diagnostics atomic.Int64
	s := testSession(t, Options{
		Diagnostic: func(message string) {
			diagnostics.Add(1)
			if message != "telemetry: invalid or oversized event dropped" {
				t.Errorf("unexpected invalid-payload diagnostic: %s", message)
			}
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return testResponse(200), nil
		})},
	})
	e := testEvent()
	e.Measurements["mount_latency_ms"] = 1
	e.Measurements["private/fixture"] = 2
	s.mu.Lock()
	for range 2 {
		s.reportLocked(e)
	}
	s.mu.Unlock()
	waitForSends(t, s.reporter)
	if requests.Load() != 0 || diagnostics.Load() != 2 {
		t.Fatalf("invalid events reached delivery or lacked diagnostics: requests=%d diagnostics=%d", requests.Load(), diagnostics.Load())
	}
	delete(e.Measurements, "private/fixture")
	s.mu.Lock()
	s.reportLocked(e)
	s.mu.Unlock()
	waitForSends(t, s.reporter)
	if requests.Load() != 1 || diagnostics.Load() != 2 {
		t.Fatal("invalid measurement blocked a later approved event")
	}
}

func TestReporterCloseFinishesBeforeBudgetWhenSendsComplete(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			close(started)
			select {
			case <-release:
				return testResponse(200), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})})
	defer r.cancel()
	require.NoError(t, r.report(testEvent()))
	awaitSignal(t, started)
	returned := make(chan struct{})
	go func() { r.close(2 * time.Second); close(returned) }()
	select {
	case <-returned:
		close(release)
		t.Fatal("close did not wait for outstanding send")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	awaitSignal(t, returned)
}

func TestReporterCloseIsBoundedEvenIfTransportIgnoresCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			close(started)
			<-release
			return nil, errors.New("fixture delayed failure")
		})})
	defer func() {
		close(release)
		waitForSends(t, r)
		r.cancel()
	}()
	require.NoError(t, r.report(testEvent()))
	awaitSignal(t, started)
	returned := make(chan struct{})
	go func() { r.close(20 * time.Millisecond); close(returned) }()
	awaitSignal(t, returned)
	if r.ctx.Err() == nil {
		t.Fatal("close did not cancel outstanding request context")
	}
	if r.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects are allowed")
	}
}

func TestReporterCancellationInterruptsShutdownWait(t *testing.T) {
	started := make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			close(started)
			<-req.Context().Done()
			return nil, req.Context().Err()
		})})
	defer r.cancel()
	require.NoError(t, r.report(testEvent()))
	awaitSignal(t, started)
	returned := make(chan struct{})
	go func() { r.close(time.Hour); close(returned) }()
	r.cancel()
	awaitSignal(t, returned)
	waitForSends(t, r)
}
