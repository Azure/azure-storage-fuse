// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionClosingReleasesStateLockAndCoordinatesCallers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handoff bool
	}{
		{name: "delivery-completes"},
		{name: "handoff-delivery-completes", handoff: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var startOnce, releaseOnce sync.Once
			var requests atomic.Int64
			s := testSession(t, Options{
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					e, err := requestEvent(req)
					if err != nil {
						return nil, err
					}
					requests.Add(1)
					if e.Data.Data.Name == "blobfuse.mount.stopped" ||
						tc.handoff && e.Data.Data.Name == "blobfuse.mount.result" {
						startOnce.Do(func() { close(started) })
						select {
						case <-release:
						case <-req.Context().Done():
							return nil, req.Context().Err()
						}
					}
					return testResponse(200), nil
				})},
			})
			defer func() {
				releaseOnce.Do(func() { close(release) })
				s.Close()
			}()
			Stage("pipeline_initialization")
			s.Configure(map[string]string{"auth_mode": "MSI"})
			if tc.handoff {
				s.Detach()
			} else {
				Ready(nil)
			}
			waitForSends(t, s.reporter)
			firstDone := make(chan struct{})
			go func() {
				if tc.handoff {
					s.HandoffFailed()
				} else {
					s.Close()
				}
				close(firstDone)
			}()
			awaitSignal(t, started)
			unchanged := make(chan bool, 1)
			go func() {
				current := Current()
				s.Configure(map[string]string{"auth_mode": "SPN"})
				Stage("late")
				Failure("late")
				Ready(map[string]string{"fuse_api_major": "2"})
				TeardownComplete()
				s.Detach()
				s.mu.Lock()
				stage := "pipeline_initialization"
				if tc.handoff {
					stage = "daemonization"
				}
				ok := current == nil && active.Load() == s && s.state == sessionClosing &&
					s.stage == stage && s.failure == "" &&
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
			releaseOnce.Do(func() { close(release) })
			for _, done := range []<-chan struct{}{firstDone, allDone, s.closeDone} {
				awaitSignal(t, done)
			}
			waitForSends(t, s.reporter)
			s.Configure(map[string]string{"auth_mode": "SPN"})
			s.HandoffFailed()
			s.mu.Lock()
			closed := s.state == sessionClosed && s.config["auth_mode"] == "MSI"
			s.mu.Unlock()
			want := int64(2)
			if tc.handoff {
				want = 1
			}
			if !closed || Current() != nil || active.Load() != nil || requests.Load() != want {
				t.Fatalf("shutdown changed state or lifecycle count: closed=%t sends=%d", closed, requests.Load())
			}
		})
	}
}
