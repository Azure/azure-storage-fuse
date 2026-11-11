// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
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

func TestRetryStateRecoversWithoutSpoolingInflightAttempt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	mount := "/test/state-retry"
	previous := mountStore(t, dir, mount, "")
	owner, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{Session: newID(), Owner: owner, Ready: true, Uptime: 42, Updated: time.Now()}
	old.Owner.Start = "old-process"
	if err := previous.checkpoint(old); err != nil {
		t.Fatal(err)
	}
	// Safely simulate unavailable persistent state without filling a real disk
	// or disturbing any mount; restoring permissions represents path repair.
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	attempt := make(chan Event, 1)
	recovered := make(chan Event, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		StateDirectory:   dir, MountPath: mount,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var batch []envelope
			if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
				return nil, err
			}
			item := batch[0]
			e := Event{Name: item.Data.Data.Name, Properties: item.Data.Data.Properties,
				Time: item.Time, Measurements: item.Data.Data.Measurements}
			if e.Name == "blobfuse.mount.attempted" {
				attempt <- e
				select {
				case <-release:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
			}
			if e.Name == "blobfuse.mount.recovery" {
				recovered <- e
			}
			return &http.Response{StatusCode: 200, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	if s.store != nil {
		t.Fatal("initial state unexpectedly available")
	}
	var original Event
	select {
	case original = <-attempt:
	case <-time.After(time.Second):
		t.Fatal("original attempt did not start delivery")
	}
	session, started := s.Identity()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s.RetryState()
	if s.store == nil || s.reporter.store.Load() != s.store {
		t.Fatal("repaired state not attached to session and reporter")
	}
	var retained Event
	if err := s.store.read("event-"+original.Properties["event_id"]+".json", &retained); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("late attachment spooled an already attempted event:", err)
	}
	currentSession, currentStarted := s.Identity()
	if session != currentSession || !started.Equal(currentStarted) {
		t.Fatal("state retry changed startup identity or timing")
	}
	var current sessionRecord
	if err := s.store.read("session-"+session+".json", &current); err != nil {
		t.Fatal("replacement session was not checkpointed:", err)
	}
	unblock()
	select {
	case e := <-recovered:
		if e.Properties["previous_session_id"] != old.Session || e.Properties["termination_class"] != "unclean_unknown" ||
			e.Measurements["previous_last_known_uptime_seconds"] != 42 {
			t.Fatalf("wrong recovery: %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("previous unfinished session not reported")
	}
	attached := s.store
	s.RetryState()
	if s.store != attached {
		t.Fatal("successful attachment was needlessly replaced")
	}
	select {
	case <-attempt:
		t.Fatal("state retry produced another attempted event")
	default:
	}
}

func TestRetryStateDoesNotUndoDisableOrCreateNewScope(t *testing.T) {
	s := handoffSession(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	s.Disable()
	s.RetryState()
	assertNoHandoffState(t, s)
}

func TestRetryStateFailureRemainsNonfatal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		StateDirectory:   dir, MountPath: "/test/unavailable-state",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
	})
	if err != nil || s == nil {
		t.Fatalf("telemetry initialization: %v", err)
	}
	defer s.Disable()
	s.RetryState()
	if s.store != nil || Current() != s {
		t.Fatal("failed state retry replaced or stopped the in-memory collector")
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("unsafe state directory was modified: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".scope-key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("scope key created inside unsafe state directory")
	}
}

func TestRetryStateCompletesEarlierRuntimeOptOutPurge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	mount := "/test/disabled-state-retry"
	previous := mountStore(t, dir, mount, "")
	e := testEvent()
	if err := previous.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		StateDirectory:   dir, MountPath: mount,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	s.Disable() // Path/state is still unavailable during this first purge.
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s.RetryState()
	if Current() != nil || s.store != nil {
		t.Fatal("deferred purge reactivated collection or persistence")
	}
	var retained Event
	if err := previous.read("event-"+e.Properties["event_id"]+".json", &retained); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deferred opt-out retained prior event: %v", err)
	}
}
