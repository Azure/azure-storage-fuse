// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func handoffSession(t *testing.T, transport http.RoundTripper) *Session {
	t.Helper()
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		HTTPClient:       &http.Client{Transport: transport},
		DeferAttempt:     true,
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		s.Disable()
		if current := Current(); current != nil && current != s {
			current.Disable()
		}
	})
	return s
}

func assertDisabledHandoff(t *testing.T, s *Session) {
	t.Helper()
	current := Current()
	s.mu.Lock()
	defer s.mu.Unlock()
	if current != nil || !s.disabled.Load() || s.state != sessionClosed {
		t.Fatal("handoff reactivated telemetry after opt-out")
	}
}

func TestHandoffFailureCannotUndoOptOut(t *testing.T) {
	for _, detached := range []bool{false, true} {
		name := "active"
		if detached {
			name = "detached"
		}
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			requests := 0
			s := handoffSession(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				mu.Lock()
				requests++
				mu.Unlock()
				return &http.Response{StatusCode: 200, Header: http.Header{},
					Body: io.NopCloser(strings.NewReader(""))}, nil
			}))
			if detached {
				s.Detach()
			}
			DisableCurrent()
			mu.Lock()
			before := requests
			mu.Unlock()
			s.HandoffFailed()
			s.HandoffFailed()
			s.ReportAttempt()
			s.Close()
			mu.Lock()
			after := requests
			mu.Unlock()
			if before != after {
				t.Fatalf("requests sent after opt-out: %d -> %d", before, after)
			}
			assertDisabledHandoff(t, s)
		})
	}
}

func TestPermittedHandoffFailureReportsOnce(t *testing.T) {
	var mu sync.Mutex
	var results []map[string]string
	var attemptTime time.Time
	attempts := 0
	s := handoffSession(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var incoming []envelope
		if err := json.NewDecoder(req.Body).Decode(&incoming); err != nil {
			return nil, err
		}
		mu.Lock()
		for _, e := range incoming {
			if e.Data.Data.Name == "blobfuse.mount.attempted" {
				attempts++
				if !e.Time.Equal(attemptTime) {
					t.Error("failed handoff lost original attempt time")
				}
			}
			if e.Data.Data.Name == "blobfuse.mount.result" {
				results = append(results, e.Data.Data.Properties)
			}
		}
		mu.Unlock()
		return &http.Response{StatusCode: 200, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	attemptTime = s.AttemptTime()
	s.Configure(map[string]string{"auth_mode": "MSI"})
	s.Detach()
	s.Configure(map[string]string{"auth_mode": "SPN"})
	s.HandoffFailed()
	s.HandoffFailed()
	s.Close()
	mu.Lock()
	defer mu.Unlock()
	if len(results) != 1 || attempts != 1 {
		t.Fatalf("failure lifecycle counts: attempts=%d results=%d, want one each", attempts, len(results))
	}
	if p := results[0]; p["outcome"] != "failure" || p["failure_stage"] != "daemonization" ||
		p["mount_session_id"] != s.id || p["auth_mode"] != "MSI" {
		t.Fatalf("incorrect failure identity or outcome: %+v", p)
	}
}

func TestOptOutCancelsConcurrentHandoffDelivery(t *testing.T) {
	sending := make(chan struct{})
	cancelled := make(chan struct{})
	var once sync.Once
	s := handoffSession(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var incoming []envelope
		if err := json.NewDecoder(req.Body).Decode(&incoming); err != nil {
			return nil, err
		}
		for _, e := range incoming {
			if e.Data.Data.Name == "blobfuse.mount.result" {
				once.Do(func() { close(sending) })
				<-req.Context().Done()
				close(cancelled)
				return nil, req.Context().Err()
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	s.Detach()
	handoffDone := make(chan struct{})
	go func() {
		s.HandoffFailed()
		close(handoffDone)
	}()
	select {
	case <-sending:
	case <-time.After(2 * time.Second):
		t.Fatal("handoff reporter did not start")
	}
	disabled := make(chan struct{})
	go func() { DisableCurrent(); close(disabled) }()
	for _, done := range []<-chan struct{}{cancelled, handoffDone, disabled} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("opt-out did not cancel and join concurrent handoff delivery")
		}
	}
	assertDisabledHandoff(t, s)
}

func TestParentExitDoesNotWaitForOrCancelChildAttempt(t *testing.T) {
	var parentRequests int
	parent := handoffSession(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		parentRequests++
		return nil, errors.New("parent must not send")
	}))
	parent.Detach()
	parent.ReportAttempt()
	started := make(chan (<-chan struct{}), 1)
	release := make(chan struct{})
	child, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		SessionID:        parent.Identity(),
		StartMonotonic:   parent.StartMonotonic(),
		AttemptTime:      parent.AttemptTime(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			started <- req.Context().Done()
			select {
			case <-release:
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})},
	})
	if err != nil || child == nil {
		t.Fatalf("child: %v", err)
	}
	defer child.Disable()
	if child.Identity() != parent.Identity() || child.StartMonotonic() != parent.StartMonotonic() ||
		!child.AttemptTime().Equal(parent.AttemptTime()) {
		t.Fatal("handoff lost identity or original startup timing")
	}
	var cancelled <-chan struct{}
	select {
	case cancelled = <-started:
	case <-time.After(time.Second):
		t.Fatal("child did not start its attempted request before readiness")
	}
	start := time.Now()
	parent.Close()
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("parent exit waited for child telemetry: %s", elapsed)
	}
	select {
	case <-parent.reporter.done:
	case <-time.After(time.Second):
		t.Fatal("parent reporter did not complete cleanup")
	}
	if parentRequests != 0 || parent.attemptReported {
		t.Fatal("background parent transmitted the child's attempt")
	}
	select {
	case <-cancelled:
		t.Fatal("parent exit cancelled child delivery")
	default:
	}
	if Current() != child {
		t.Fatal("parent exit removed the child's active collector")
	}
	close(release)
	waitForSends(t, child.reporter)
}
