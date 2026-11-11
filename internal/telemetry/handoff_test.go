// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func handoffSession(t *testing.T, transport http.RoundTripper) *Session {
	t.Helper()
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		StateDirectory:   filepath.Join(t.TempDir(), "microsoft-telemetry"),
		MountPath:        "/test/handoff",
		HTTPClient:       &http.Client{Transport: transport},
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

func assertNoHandoffState(t *testing.T, s *Session) {
	t.Helper()
	store, err := openMountStore(s.stateDirectory, s.mountPath, s.mountScope, false, func(string) {})
	if err != nil || store == nil {
		t.Fatalf("open mount store: %v", err)
	}
	defer store.root.Close()
	files, err := store.entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if stateName(file.Name()) {
			t.Fatalf("opt-out left or recreated retained state: %s", file.Name())
		}
	}
	if Current() != nil {
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
			s.Disable()
			mu.Lock()
			before := requests
			mu.Unlock()
			s.HandoffFailed(errors.New("simulated daemon launch failure"))
			s.HandoffFailed(errors.New("repeated failure callback"))
			s.Close(nil)
			mu.Lock()
			after := requests
			mu.Unlock()
			if before != after {
				t.Fatalf("requests sent after opt-out: %d -> %d", before, after)
			}
			assertNoHandoffState(t, s)
		})
	}
}

func TestPermittedHandoffFailureReportsOnce(t *testing.T) {
	var mu sync.Mutex
	var results []map[string]string
	s := handoffSession(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var incoming []envelope
		if err := json.NewDecoder(req.Body).Decode(&incoming); err != nil {
			return nil, err
		}
		mu.Lock()
		for _, e := range incoming {
			if e.Data.Data.Name == "blobfuse.mount.result" {
				results = append(results, e.Data.Data.Properties)
			}
		}
		mu.Unlock()
		return &http.Response{StatusCode: 200, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	s.Detach()
	s.HandoffFailed(errors.New("simulated daemon launch failure"))
	s.HandoffFailed(errors.New("repeated failure callback"))
	s.Close(nil)
	mu.Lock()
	defer mu.Unlock()
	if len(results) != 1 {
		t.Fatalf("failure event count=%d, want one", len(results))
	}
	if p := results[0]; p["outcome"] != "failure" || p["failure_stage"] != "daemonization" ||
		p["mount_session_id"] != s.id {
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
		s.HandoffFailed(errors.New("simulated daemon launch failure"))
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
	assertNoHandoffState(t, s)
}

func TestDisableCurrentReachesDetachedPolicyOwner(t *testing.T) {
	s := handoffSession(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, context.Cause(req.Context())
	}))
	s.Detach()
	if Current() != nil {
		t.Fatal("detached parent still exposed as an active collector")
	}
	DisableCurrent()
	s.HandoffFailed(errors.New("simulated daemon launch failure"))
	assertNoHandoffState(t, s)
	if _, err := os.Stat(s.stateDirectory); err != nil {
		t.Fatal("opt-out removed the shared mount root:", err)
	}
}
