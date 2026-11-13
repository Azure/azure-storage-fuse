// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionClosingReleasesStateLockAndCoordinatesCallers(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		disable, ignoreCancellation bool
	}{
		{name: "delivery-completes"},
		{name: "opt-out-cancels", disable: true},
		{name: "opt-out-ignores-transport-cleanup", disable: true, ignoreCancellation: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var startOnce, cancelOnce, releaseOnce sync.Once
			var requests atomic.Int64
			s, err := Start(Options{
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var batch []envelope
					if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
						return nil, err
					}
					requests.Add(1)
					if batch[0].Data.Data.Name == "blobfuse.mount.stopped" {
						startOnce.Do(func() { close(started) })
						select {
						case <-release:
						case <-req.Context().Done():
							cancelOnce.Do(func() { close(cancelled) })
							if tc.ignoreCancellation {
								<-release
							}
							return nil, req.Context().Err()
						}
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
				})},
			})
			if err != nil || s == nil {
				t.Fatalf("start: %v", err)
			}
			defer func() {
				releaseOnce.Do(func() { close(release) })
				s.Disable()
			}()
			Stage("pipeline_initialization")
			s.Configure(map[string]string{"auth_mode": "MSI"})
			Ready(nil)
			waitForSends(t, s.reporter)
			firstDone := make(chan struct{})
			go func() { s.Close(); close(firstDone) }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("shutdown did not start its final send")
			}
			unchanged := make(chan bool, 1)
			go func() {
				current := Current()
				s.Configure(map[string]string{"auth_mode": "SPN"})
				Stage("late")
				Failure("late")
				Ready(map[string]string{"fuse_api_major": "2"})
				TeardownComplete()
				s.Detach()
				s.ReportAttempt()
				s.mu.Lock()
				ok := current == nil && active.Load() == s && s.state == sessionClosing &&
					s.stage == "pipeline_initialization" && s.failure == "" &&
					s.config["auth_mode"] == "MSI" && s.config["fuse_api_major"] == "" &&
					!s.teardownComplete
				s.mu.Unlock()
				unchanged <- ok
			}()
			select {
			case ok := <-unchanged:
				if !ok {
					t.Fatal("late callbacks changed a closing session")
				}
			case <-time.After(time.Second):
				t.Fatal("shutdown held the state lock while waiting for delivery")
			}
			var callers sync.WaitGroup
			for range 8 {
				callers.Add(1)
				go func() { defer callers.Done(); s.Close() }()
			}
			allDone := make(chan struct{})
			go func() { callers.Wait(); close(allDone) }()
			for _, done := range []<-chan struct{}{firstDone, allDone} {
				select {
				case <-done:
					t.Fatal("Close returned before the shared shutdown completed")
				case <-time.After(20 * time.Millisecond):
				}
			}
			if tc.disable {
				disabled := make(chan struct{})
				go func() { DisableCurrent(); close(disabled) }()
				for _, done := range []<-chan struct{}{cancelled, disabled} {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Fatal("opt-out did not interrupt the shutdown wait")
					}
				}
			} else {
				releaseOnce.Do(func() { close(release) })
			}
			for _, done := range []<-chan struct{}{firstDone, allDone, s.closeDone} {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("concurrent Close calls did not share shutdown completion")
				}
			}
			if tc.ignoreCancellation {
				select {
				case <-s.reporter.done:
					t.Fatal("fixture transport unexpectedly finished cleanup")
				default:
				}
				releaseOnce.Do(func() { close(release) })
			}
			waitForSends(t, s.reporter)
			s.Configure(map[string]string{"auth_mode": "SPN"})
			s.ReportAttempt()
			s.HandoffFailed()
			s.mu.Lock()
			closed := s.state == sessionClosed && s.config["auth_mode"] == "MSI"
			s.mu.Unlock()
			if !closed || Current() != nil || active.Load() != nil || requests.Load() != 3 {
				t.Fatalf("shutdown changed state or lifecycle count: closed=%t sends=%d", closed, requests.Load())
			}
		})
	}
}

func TestSessionDisableCancelsBeforeAcquiringStateLock(t *testing.T) {
	started := make(chan struct{})
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			close(started)
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("attempted send did not start")
	}
	s.mu.Lock()
	done := make(chan struct{})
	go func() { s.Disable(); close(done) }()
	select {
	case <-s.reporter.ctx.Done():
		s.mu.Unlock()
	case <-time.After(time.Second):
		s.mu.Unlock()
		t.Fatal("opt-out waited for the state lock before cancelling delivery")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disable did not complete after the state lock was released")
	}
	waitForSends(t, s.reporter)
}
