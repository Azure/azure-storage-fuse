// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryMakesProgressAtRecordLimit(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := self
	old.Start = "previous-process-start"
	for range MaxStateRecords {
		if err := store.checkpoint(sessionRecord{Session: newID(), Owner: old, Updated: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	reporting := &Session{id: newID()}
	pending, err := store.recover(self, reporting.id, reporting.event)
	if err != nil || len(pending) == 0 {
		t.Fatalf("full store cannot recover: events=%d err=%v", len(pending), err)
	}
	if len(pending) != 8 {
		t.Fatalf("recovery work limit changed: %d", len(pending))
	}
	entries, err := store.entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxStateRecords+1 { // Includes the lock file.
		t.Fatal("recovery allocated additional files at the record limit")
	}
	var accepted atomic.Int64
	d, err := parseDestination("InstrumentationKey=test;IngestionEndpoint=https://example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	reporter := newReporter(d, store, func(string) {}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		accepted.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	for _, e := range pending {
		reporter.reportRetained(e)
	}
	reporter.close(time.Second)
	if accepted.Load() != int64(len(pending)) {
		t.Fatal("checkpoint-only events were not delivered")
	}
	if err := store.checkpoint(sessionRecord{Session: newID(), Owner: self, Updated: time.Now()}); err != nil {
		t.Fatal("recovery did not free room for a new session:", err)
	}
}

func TestRecoveryReserveAndPendingReplay(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{Session: newID(), Owner: self, Updated: time.Now()}
	old.Owner.Start = "old-process"
	if err := store.checkpoint(old); err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for {
		e := testEvent()
		for _, k := range []string{"distro_name", "distro_version", "kernel_release", "blobfuse_version",
			"pipeline_components", "auth_mode", "config_fingerprint"} {
			e.Properties[k] = strings.Repeat("x", 900)
		}
		err := store.persistEvent(e)
		if err != nil {
			if err.Error() != "telemetry state capacity exceeded" {
				t.Fatal(err)
			}
			break
		}
		existing[e.Properties["event_id"]] = true
	}
	used := func() int64 {
		t.Helper()
		entries, err := store.entries()
		if err != nil {
			t.Fatal(err)
		}
		var bytes int64
		for _, entry := range entries {
			if stateName(entry.Name()) {
				info, err := entry.Info()
				if err != nil {
					t.Fatal(err)
				}
				bytes += info.Size()
			}
		}
		return bytes
	}
	before := used()
	if before <= MaxStateBytes-RecoveryReserveBytes-MaxEventBytes || before > MaxStateBytes-RecoveryReserveBytes {
		t.Fatalf("normal writes did not preserve recovery headroom: %d bytes", before)
	}
	reporting := &Session{id: newID()}
	pending, err := store.recover(self, reporting.id, reporting.event)
	if err != nil || len(pending) != len(existing)+1 {
		t.Fatalf("full normal store lost replay: events=%d err=%v", len(pending), err)
	}
	if after := used(); after > MaxStateBytes || after <= before {
		t.Fatalf("recovery did not use only bounded reserved space: before=%d after=%d", before, after)
	}
	recoveredID := ""
	for _, e := range pending {
		id := e.Properties["event_id"]
		if e.Name == "blobfuse.mount.recovery" {
			recoveredID = id
		} else if !existing[id] {
			t.Fatal("replay changed event identity")
		}
	}
	again, err := store.recover(self, reporting.id, reporting.event)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range again {
		found = found || e.Name == "blobfuse.mount.recovery" && e.Properties["event_id"] == recoveredID
	}
	if !found {
		t.Fatal("checkpoint-only recovery was not stable across restarts")
	}
}

func TestRecoveryReturnsPendingWhenCheckpointCannotBeUpdated(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission fixture needs an unprivileged user")
	}
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{Session: newID(), Owner: self, Updated: time.Now()}
	old.Owner.Start = "old-process"
	if err := store.checkpoint(old); err != nil {
		t.Fatal(err)
	}
	e := testEvent()
	if err := store.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.root.Name(), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(store.root.Name(), 0700); err != nil {
			t.Error(err)
		}
	})
	reporting := &Session{id: newID()}
	pending, err := store.recover(self, reporting.id, reporting.event)
	if err == nil || len(pending) != 1 || pending[0].Properties["event_id"] != e.Properties["event_id"] {
		t.Fatalf("failed checkpoint hid existing pending work: events=%d err=%v", len(pending), err)
	}
	var retained sessionRecord
	if err := store.read("session-"+old.Session+".json", &retained); err != nil || retained.Terminal != nil {
		t.Fatal("failed checkpoint was treated as durable terminal evidence")
	}
}

func TestCheckpointOnlyReplayAndCompletionWithoutNewWrites(t *testing.T) {
	store, record, e, self := completionFixture(t, "blobfuse.mount.recovery")
	if err := store.locked(func() error { return store.removeDurable("event-" + e.Properties["event_id"] + ".json") }); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.root.Name(), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(store.root.Name(), 0700); err != nil {
			t.Error(err)
		}
	})
	reporting := &Session{id: newID()}
	pending, err := store.recover(self, reporting.id, reporting.event)
	if err != nil || len(pending) != 1 || pending[0].Properties["event_id"] != e.Properties["event_id"] {
		t.Fatalf("durable terminal replay required a new write: %+v %v", pending, err)
	}
	if err := os.Chmod(store.root.Name(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.acknowledgeEvent(pending[0]); err != nil {
		t.Fatal("checkpoint-only acknowledgement failed:", err)
	}
	var retained sessionRecord
	if err := store.read("session-"+record.Session+".json", &retained); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("acknowledged checkpoint was not retired:", err)
	}
}
