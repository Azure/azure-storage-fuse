// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestCloseReleasesStateLockDuringDelivery(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int64
	s := testSession(t, Options{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			close(started)
			select {
			case <-release:
				return testResponse(200), nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})},
	})
	Stage("pipeline_initialization")
	s.Configure(map[string]string{"auth_mode": "MSI"})
	firstDone := make(chan struct{})
	go func() { s.Close(); close(firstDone) }()
	awaitSignal(t, started)

	// Late callbacks and repeated closes must neither block on delivery nor
	// change the session or send again.
	late := make(chan struct{})
	go func() {
		s.Configure(map[string]string{"auth_mode": "SPN"})
		Stage("late")
		Failure("late")
		Ready(map[string]string{"fuse_api_major": "2"})
		s.Detach()
		s.Close()
		s.HandoffFailed()
		close(late)
	}()
	awaitSignal(t, late)
	select {
	case <-firstDone:
		t.Fatal("Close returned before the outstanding result finished")
	default:
	}
	close(release)
	awaitSignal(t, firstDone)

	s.mu.Lock()
	ok := s.state == sessionClosed && s.stage == "pipeline_initialization" && s.failure == "" &&
		s.config["auth_mode"] == "MSI" && s.config["fuse_api_major"] == "" && s.resultRecorded
	s.mu.Unlock()
	if !ok || Current() != nil || active.Load() != nil || requests.Load() != 1 {
		t.Fatalf("late callbacks changed a closed session: ok=%t sends=%d", ok, requests.Load())
	}
}
