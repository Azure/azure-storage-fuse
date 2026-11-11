// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func completionFixture(t *testing.T, name string) (*stateStore, sessionRecord, Event, processIdentity) {
	t.Helper()
	store := testStore(t)
	current, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{
		Session: newID(), Owner: current,
		Ready: name != "blobfuse.mount.result", Updated: time.Now(), Uptime: 42,
	}
	old.Owner.Start = "previous-process-start"
	e := testEvent()
	e.Name = name
	e.Properties["mount_session_id"] = old.Session
	switch name {
	case "blobfuse.mount.stopped":
		e.Properties["termination_class"] = "clean"
	case "blobfuse.mount.result":
		e.Properties["outcome"] = "failure"
	case "blobfuse.mount.recovery":
		e.Properties["mount_session_id"] = newID()
		e.Properties["previous_session_id"] = old.Session
		e.Properties["termination_class"] = "unclean_unknown"
		old.Terminal = &e
	}
	if err := store.checkpoint(old); err != nil {
		t.Fatal(err)
	}
	if err := store.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	return store, old, e, current
}

func TestAcknowledgedTerminalDoesNotBecomeUnclean(t *testing.T) {
	for _, name := range []string{"blobfuse.mount.stopped", "blobfuse.mount.result", "blobfuse.mount.recovery"} {
		t.Run(name, func(t *testing.T) {
			store, _, e, current := completionFixture(t, name)
			// For stop/startup-failure, the final session checkpoint deliberately
			// remains unfinished while its terminal event was persisted.
			if err := store.acknowledgeEvent(e); err != nil {
				t.Fatal(err)
			}
			replacement := &Session{id: newID()}
			pending, err := store.recover(current, replacement.id, replacement.event)
			if err != nil {
				t.Fatal(err)
			}
			if len(pending) != 0 {
				t.Fatalf("acknowledged terminal produced later recovery: %+v", pending)
			}
		})
	}
}

func TestLegacyTerminalReplayOmitsRetiredProperties(t *testing.T) {
	for _, name := range []string{"blobfuse.mount.stopped", "blobfuse.mount.result", "blobfuse.mount.recovery"} {
		t.Run(name, func(t *testing.T) {
			store, record, e, current := completionFixture(t, name)
			for key := range retiredProperties {
				e.Properties[key] = "legacy"
			}
			record.Terminal = &e
			legacy := struct {
				sessionRecord
				Attempt string `json:"attempt"`
			}{record, newID()}
			if err := store.locked(func() error {
				return store.write("session-"+record.Session+".json", legacy)
			}); err != nil {
				t.Fatal(err)
			}
			if err := store.persistEvent(e); err != nil {
				t.Fatal(err)
			}
			reporting := &Session{id: newID()}
			pending, err := store.recover(current, reporting.id, reporting.event)
			if err != nil || len(pending) != 1 {
				t.Fatalf("legacy terminal not replayable: %d %v", len(pending), err)
			}
			var sent atomic.Int64
			r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, store, func(string) {},
				&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var batch []envelope
					if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
						return nil, err
					}
					got := batch[0].Data.Data.Properties
					for key := range retiredProperties {
						if _, exists := got[key]; exists {
							t.Errorf("retired field replayed: %s", key)
						}
					}
					if got["event_id"] != e.Properties["event_id"] {
						t.Error("replay changed event identity")
					}
					sent.Add(1)
					return &http.Response{StatusCode: 200, Header: http.Header{},
						Body: io.NopCloser(strings.NewReader(""))}, nil
				})})
			r.reportRetained(pending[0])
			r.close(time.Second)
			if sent.Load() != 1 {
				t.Fatal("legacy event not delivered")
			}
			for _, filename := range []string{"session-" + record.Session + ".json", "event-" + e.Properties["event_id"] + ".json"} {
				if _, err := store.root.Stat(filename); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("legacy terminal cleanup failed for %s: %v", filename, err)
				}
			}
		})
	}
}
func TestDiscardedTerminalDoesNotBecomeUnclean(t *testing.T) {
	for _, name := range []string{"blobfuse.mount.stopped", "blobfuse.mount.result", "blobfuse.mount.recovery"} {
		t.Run(name, func(t *testing.T) {
			store, _, e, current := completionFixture(t, name)
			if err := store.discardEvent(e); err != nil {
				t.Fatal(err)
			}
			replacement := &Session{id: newID()}
			pending, err := store.recover(current, replacement.id, replacement.event)
			if err != nil || len(pending) != 0 {
				t.Fatalf("discard lost the known termination outcome: %+v %v", pending, err)
			}
		})
	}
}

func TestNonTerminalAcknowledgementRetainsRunningSession(t *testing.T) {
	for _, name := range []string{"blobfuse.mount.attempted", "blobfuse.mount.result"} {
		t.Run(name, func(t *testing.T) {
			store := testStore(t)
			current, err := identity(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			record := sessionRecord{Session: newID(), Owner: current, Ready: true, Updated: time.Now()}
			e := testEvent()
			e.Name = name
			e.Properties["mount_session_id"] = record.Session
			e.Properties["outcome"] = "success"
			if err := store.checkpoint(record); err != nil {
				t.Fatal(err)
			}
			if err := store.persistEvent(e); err != nil {
				t.Fatal(err)
			}
			if err := store.acknowledgeEvent(e); err != nil {
				t.Fatal(err)
			}
			var retained sessionRecord
			if err := store.read("session-"+record.Session+".json", &retained); err != nil {
				t.Fatal("ordinary acknowledgement removed the running session:", err)
			}
			var pending Event
			if err := store.read("event-"+e.Properties["event_id"]+".json", &pending); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("ordinary acknowledged event retained:", err)
			}
		})
	}
}

func TestTerminalCleanupRetainsEvidenceWhenSessionRemovalFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-denial fixture requires an unprivileged user")
	}
	store, record, e, current := completionFixture(t, "blobfuse.mount.stopped")
	path := store.root.Name()
	if err := os.Chmod(path, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0700); err != nil {
			t.Error(err)
		}
	})
	if err := store.acknowledgeEvent(e); err == nil {
		t.Fatal("expected session removal to fail on a non-writable directory")
	}
	var pending Event
	if err := store.read("event-"+e.Properties["event_id"]+".json", &pending); err != nil {
		t.Fatal("lost completion evidence when cleanup failed:", err)
	}
	var saved sessionRecord
	if err := store.read("session-"+record.Session+".json", &saved); err != nil {
		t.Fatal("unexpected session state after failed removal:", err)
	}
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal("cleanup did not recover:", err)
	}
	replacement := &Session{id: newID()}
	events, err := store.recover(current, replacement.id, replacement.event)
	if err != nil || len(events) != 0 {
		t.Fatalf("cleanup retry produced false recovery: %+v %v", events, err)
	}
}

func TestTerminalCleanupInterruptedAfterSessionRemoval(t *testing.T) {
	store, record, e, current := completionFixture(t, "blobfuse.mount.stopped")
	if err := store.locked(func() error { return store.removeDurable("session-" + record.Session + ".json") }); err != nil {
		t.Fatal(err)
	}
	// Simulate restart after retiring the session but before event removal.
	replacement := &Session{id: newID()}
	pending, err := store.recover(current, replacement.id, replacement.event)
	if err != nil || len(pending) != 1 || pending[0].Properties["event_id"] != e.Properties["event_id"] ||
		pending[0].Properties["termination_class"] != "clean" {
		t.Fatalf("interrupted cleanup lost terminal evidence: %+v %v", pending, err)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal("cannot finish cleanup with an already absent session:", err)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal("repeated acknowledgement was not idempotent:", err)
	}
}

func TestTerminalCleanupRejectsConflictingEvidence(t *testing.T) {
	for _, scenario := range []string{"conflicting-pending", "conflicting-terminal", "wrong-session", "startup-after-ready", "recovery-without-marker"} {
		t.Run(scenario, func(t *testing.T) {
			name := "blobfuse.mount.stopped"
			if scenario == "startup-after-ready" {
				name = "blobfuse.mount.result"
			}
			if scenario == "recovery-without-marker" {
				name = "blobfuse.mount.recovery"
			}
			store, record, e, _ := completionFixture(t, name)
			filename := "session-" + record.Session + ".json"
			switch scenario {
			case "conflicting-pending":
				other := testEvent()
				other.Properties["event_id"] = e.Properties["event_id"]
				if err := store.persistEvent(other); err != nil {
					t.Fatal(err)
				}
			case "conflicting-terminal":
				other := testEvent()
				record.Terminal = &other
			case "wrong-session":
				record.Session = newID()
			case "startup-after-ready":
				record.Ready = true
			case "recovery-without-marker":
				record.Terminal = nil
			}
			if err := store.locked(func() error { return store.write(filename, record) }); err != nil {
				t.Fatal(err)
			}
			if err := store.acknowledgeEvent(e); err == nil {
				t.Fatal("conflicting completion evidence was accepted")
			}
			var retained Event
			if err := store.read("event-"+e.Properties["event_id"]+".json", &retained); err != nil {
				t.Fatal("conflicting evidence was discarded:", err)
			}
			var kept sessionRecord
			if err := store.read(filename, &kept); err != nil {
				t.Fatal("conflicting session was removed:", err)
			}
		})
	}
}

func TestRecoveryAcknowledgementRetiresPreviousSessionOnly(t *testing.T) {
	store, old, e, current := completionFixture(t, "blobfuse.mount.recovery")
	active := sessionRecord{Session: e.Properties["mount_session_id"],
		Owner: current, Ready: true, Updated: time.Now()}
	if err := store.checkpoint(active); err != nil {
		t.Fatal(err)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal(err)
	}
	var record sessionRecord
	if err := store.read("session-"+old.Session+".json", &record); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("previous recovered session was not retired:", err)
	}
	if err := store.read("session-"+active.Session+".json", &record); err != nil {
		t.Fatal("new reporting session was incorrectly retired:", err)
	}
}

func TestReporterDistinguishesTerminalAcceptanceAndDiscard(t *testing.T) {
	for _, scenario := range []string{"accepted", "rejected", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			store, record, e, _ := completionFixture(t, "blobfuse.mount.stopped")
			if scenario == "expired" {
				e.Time = time.Now().Add(-Retention - time.Second)
			}
			d, err := parseDestination("InstrumentationKey=test;IngestionEndpoint=https://example.invalid")
			if err != nil {
				t.Fatal(err)
			}
			var requests atomic.Int64
			r := newReporter(d, store, func(string) {}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				status := 200
				if scenario == "rejected" {
					status = 400
				}
				return &http.Response{StatusCode: status, Header: http.Header{},
					Body: io.NopCloser(strings.NewReader(""))}, nil
			})})
			if err := store.persistEvent(e); err != nil {
				t.Fatal(err)
			}
			r.reportRetained(e)
			r.close(time.Second)
			if _, err := store.root.Stat("session-" + record.Session + ".json"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("terminal checkpoint not retired:", err)
			}
			if _, err := store.root.Stat("event-" + e.Properties["event_id"] + ".json"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("terminal event not retired:", err)
			}
			wantRequests, wantFailures, wantDropped := int64(1), uint64(0), uint64(0)
			switch scenario {
			case "rejected":
				wantFailures, wantDropped = 1, 1
			case "expired":
				wantRequests, wantDropped = 0, 1
			}
			if requests.Load() != wantRequests || r.failures.Load() != wantFailures || r.dropped.Load() != wantDropped {
				t.Fatalf("wrong delivery disposition: requests=%d failures=%d dropped=%d",
					requests.Load(), r.failures.Load(), r.dropped.Load())
			}
		})
	}
}
