// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOptOutDuringInitialization(t *testing.T) {
	var requests atomic.Int64
	atPolicy := make(chan struct{})
	release := make(chan struct{})
	result := make(chan *Session, 1)
	startErr := make(chan error, 1)
	go func() {
		s, err := Start(Options{
			ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
			PolicyDisabled: func() bool {
				close(atPolicy)
				<-release
				return false
			},
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
			})},
		})
		startErr <- err
		result <- s
	}()
	select {
	case <-atPolicy:
	case <-time.After(2 * time.Second):
		t.Fatal("initialization did not reach policy gate")
	}
	s := active.Load()
	if s == nil {
		close(release)
		t.Fatal("initializing session has no policy owner")
	}
	t.Cleanup(s.Disable)
	if Current() != nil {
		close(release)
		t.Fatal("partially initialized session exposed to collectors")
	}
	disabled := make(chan struct{})
	go func() { DisableCurrent(); close(disabled) }()
	select {
	case <-s.reporter.ctx.Done():
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("opt-out did not cancel before initialization completed")
	}
	select {
	case <-disabled:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("opt-out cleanup did not complete")
	}
	close(release)
	if err := <-startErr; err != nil {
		t.Fatal(err)
	}
	if got := <-result; got != nil {
		t.Fatal("startup returned an enabled session after opt-out")
	}
	if requests.Load() != 0 || Current() != nil || active.Load() != nil {
		t.Fatal("initialization sent or reactivated telemetry after opt-out")
	}
}

func TestInitializationRechecksPolicyBeforeDispatch(t *testing.T) {
	var requests atomic.Int64
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		PolicyDisabled:   func() bool { return true },
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	})
	if s != nil {
		defer s.Disable()
	}
	if err != nil || s != nil || requests.Load() != 0 || active.Load() != nil {
		t.Fatalf("changed policy was ignored: session=%v err=%v requests=%d", s, err, requests.Load())
	}
}
