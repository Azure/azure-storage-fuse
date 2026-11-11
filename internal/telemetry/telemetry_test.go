// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testEvent() Event {
	return Event{Name: "blobfuse.mount.result", Time: time.Now().UTC(),
		Properties: map[string]string{"schema_version": "1", "event_id": newID(),
			"mount_session_id": newID()},
		Measurements: map[string]float64{}}
}

func TestRetiredPropertiesAreNotEncoded(t *testing.T) {
	for _, kind := range []string{"attempted", "result", "stopped", "recovery"} {
		t.Run(kind, func(t *testing.T) {
			e := testEvent()
			e.Name = "blobfuse.mount." + kind
			e.Measurements["count"] = 42
			for key := range retiredProperties {
				e.Properties[key] = "legacy"
			}
			body, err := encode(e, "test-key")
			if err != nil {
				t.Fatal(err)
			}
			var batch []envelope
			if err := json.Unmarshal(body, &batch); err != nil {
				t.Fatal(err)
			}
			got := batch[0].Data.Data
			for key := range retiredProperties {
				if _, exists := got.Properties[key]; exists {
					t.Errorf("retired field exported: %s", key)
				}
				if e.Properties[key] != "legacy" {
					t.Errorf("encoding mutated retained evidence: %s", key)
				}
			}
			if got.Properties["event_id"] != e.Properties["event_id"] || got.Measurements["count"] != 42 {
				t.Fatal("encoding changed event identity or measurements")
			}
			e.Properties["unapproved"] = "private"
			if _, err := encode(e, "test-key"); err == nil {
				t.Fatal("unapproved field was silently filtered instead of rejected")
			}
		})
	}
}

func TestEncodedEventSizeBoundary(t *testing.T) {
	const limit = 32 * 1024
	e := testEvent()
	e.Properties["failure_category"] = ""
	var body []byte
	var err error
	for i := 0; ; i++ {
		body, err = encode(e, "test-key")
		if err != nil {
			t.Fatal(err)
		}
		if len(body) >= limit-1024 {
			break
		}
		e.Measurements[fmt.Sprintf("counter_%04d", i)] = 1
	}
	e.Properties["failure_category"] = strings.Repeat("x", limit-len(body))
	body, err = encode(e, "test-key")
	if err != nil || len(body) != limit {
		t.Fatalf("32-KiB encoded event rejected: bytes=%d err=%v", len(body), err)
	}
	e.Properties["failure_category"] += "x"
	if _, err := encode(e, "test-key"); err == nil || err.Error() != "telemetry event exceeds limit" {
		t.Fatalf("expected event-size rejection above 32 KiB: %v", err)
	}
}

func TestDestinationValidation(t *testing.T) {
	for _, value := range []string{
		"", "InstrumentationKey=x;IngestionEndpoint=http://localhost",
		"InstrumentationKey=x;IngestionEndpoint=https://user:pass@host",
		"InstrumentationKey=x;IngestionEndpoint=https://host?q=x",
		"InstrumentationKey=x;IngestionEndpoint=https://host;Authorization=aad",
	} {
		if _, err := parseDestination(value); err == nil {
			t.Fatalf("accepted unsafe destination: %s", value)
		}
	}
	d, err := parseDestination("InstrumentationKey=x;IngestionEndpoint=https://example.com/")
	if err != nil || d.endpoint != "https://example.com/v2.1/track" {
		t.Fatalf("destination: %+v %v", d, err)
	}
}

func TestResponseHandling(t *testing.T) {
	status := atomic.Int64{}
	body := atomic.Value{}
	body.Store("")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(int(status.Load()))
		fmt.Fprint(w, body.Load().(string))
	}))
	defer server.Close()
	r := &reporter{dest: destination{endpoint: server.URL}, client: server.Client()}
	for _, tc := range []struct {
		code   int
		body   string
		failed bool
	}{
		{200, "", false},
		{400, "", true},
		{429, "", true},
		{503, "", true},
		{206, `{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":429}]}`, true},
		{206, `{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":400}]}`, true},
		{206, `{"itemsReceived":1,"itemsAccepted":1,"errors":[]}`, true},
		{206, `not JSON`, true},
	} {
		status.Store(int64(tc.code))
		body.Store(tc.body)
		err := r.send(context.Background(), []byte("[]"))
		if (err != nil) != tc.failed {
			t.Fatalf("status %d: err=%v", tc.code, err)
		}
	}
	t.Run("body-read-limit", func(t *testing.T) {
		body := strings.NewReader(`{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":429}]}` +
			strings.Repeat(" ", 100000))
		r := &reporter{dest: destination{endpoint: "https://test"}, client: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 206, Header: http.Header{}, Body: io.NopCloser(body)}, nil
			}),
		}}
		if err := r.send(context.Background(), []byte("[]")); err == nil {
			t.Fatal("expected item rejection")
		}
		if read := body.Size() - int64(body.Len()); read != 64*1024 {
			t.Fatalf("acknowledgement read %d bytes, want the 64-KiB limit", read)
		}
	})
}

func TestSessionLifecycle(t *testing.T) {
	var mu sync.Mutex
	var events []envelope
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var incoming []envelope
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			t.Error(err)
		}
		mu.Lock()
		events = append(events, incoming...)
		mu.Unlock()
	}))
	defer server.Close()
	s, err := Start(Options{ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory: filepath.Join(t.TempDir(), "microsoft-telemetry"), HTTPClient: server.Client(),
		MountPath: "/test/lifecycle-mount", Version: "test"})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Disable()
	s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
	Ready(map[string]string{"fuse_api_major": "3"}, nil)
	TeardownComplete()
	s.Close(nil)
	if Current() != nil {
		t.Fatal("session still active")
	}
	mu.Lock()
	defer mu.Unlock()
	kinds := map[string]int{}
	for _, e := range events {
		kinds[e.Data.Data.Name]++
		if strings.Contains(fmt.Sprint(e), "MUST-NOT-BE-SENT") {
			t.Fatal("unapproved configuration exported")
		}
	}
	if kinds["blobfuse.mount.attempted"] != 1 || kinds["blobfuse.mount.result"] != 1 ||
		kinds["blobfuse.mount.stopped"] != 1 {
		t.Fatalf("unexpected lifecycle: %+v", kinds)
	}
}

func testStore(t *testing.T) *stateStore {
	t.Helper()
	store, err := openStore(filepath.Join(t.TempDir(), "microsoft-telemetry"), func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.root.Close() })
	return store
}

// Legacy pending-event files are fixtures only; new reports are not spooled.
func (s *stateStore) persistEvent(e Event) error {
	return s.locked(func() error { return s.write("event-"+e.Properties["event_id"]+".json", e) })
}

func (s *stateStore) acknowledgeEvent(e Event) error { return s.consumeEvent(e, false) }

func (s *stateStore) discardEvent(e Event) error { return s.consumeEvent(e, false) }

func TestRecoveryIdentityAndStableReplay(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	dead := self
	dead.Start = "not-this-process"
	var session Session
	session.id, session.version = newID(), "test"
	old := sessionRecord{Session: newID(), Owner: dead, Updated: time.Now(), Ready: true, Uptime: 42}
	alive := sessionRecord{Session: newID(), Owner: self, Updated: time.Now(), Ready: true}
	for _, record := range []sessionRecord{old, alive} {
		if err := store.checkpoint(record); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := store.recover(self, session.id, session.event)
	if err != nil || len(pending) != 1 {
		t.Fatalf("recovery: %d %v", len(pending), err)
	}
	e := pending[0]
	if e.Properties["termination_class"] != "unclean_unknown" ||
		e.Measurements["previous_last_known_uptime_seconds"] != 42 ||
		e.Properties["previous_session_id"] != old.Session {
		t.Fatalf("bad recovery: %+v", e)
	}
	again, err := store.recover(self, session.id, session.event)
	if err != nil || len(again) != 1 || again[0].Properties["event_id"] != e.Properties["event_id"] {
		t.Fatalf("unstable recovery: %+v %v", again, err)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal(err)
	}
	again, err = store.recover(self, session.id, session.event)
	if err != nil || len(again) != 0 {
		t.Fatalf("recreated acknowledged event: %+v %v", again, err)
	}
	differentNS := self
	differentNS.Namespace = "other"
	if definitelyDead(differentNS, self) {
		t.Fatal("inferred death across PID namespaces")
	}
	if definitelyDead(processIdentity{}, self) {
		t.Fatal("inferred death without identity")
	}
}

func TestRecoveryAcceptsLegacySessionCheckpoint(t *testing.T) {
	store := testStore(t)
	current, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	record := sessionRecord{Session: newID(), Owner: current, Updated: time.Now(), Ready: true, Uptime: 42}
	record.Owner.Start = "old-process"
	legacy := struct {
		sessionRecord
		Attempt string `json:"attempt"`
	}{record, newID()}
	if err := store.locked(func() error {
		return store.write("session-"+record.Session+".json", legacy)
	}); err != nil {
		t.Fatal(err)
	}
	reporting := &Session{id: newID()}
	pending, err := store.recover(current, reporting.id, reporting.event)
	if err != nil || len(pending) != 1 {
		t.Fatalf("legacy checkpoint did not recover: %d %v", len(pending), err)
	}
	e := pending[0]
	if e.Properties["previous_session_id"] != record.Session ||
		e.Properties["mount_session_id"] != reporting.id ||
		e.Measurements["previous_last_known_uptime_seconds"] != 42 {
		t.Fatalf("recovery lost session association: %+v", e)
	}
	if err := store.acknowledgeEvent(e); err != nil {
		t.Fatal(err)
	}
	if _, err := store.root.Stat("session-" + record.Session + ".json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("legacy checkpoint was not retired:", err)
	}
}

func TestPendingCleanEvidenceOverridesUnfinishedCheckpoint(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{Session: newID(), Owner: self, Updated: time.Now(), Ready: true}
	old.Owner.Start = "old-process"
	if err := store.checkpoint(old); err != nil {
		t.Fatal(err)
	}
	e := testEvent()
	e.Name, e.Properties["mount_session_id"] = "blobfuse.mount.stopped", old.Session
	e.Properties["termination_class"] = "clean"
	if err := store.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	s := &Session{id: newID()}
	pending, err := store.recover(self, s.id, s.event)
	if err != nil || len(pending) != 1 || pending[0].Name != "blobfuse.mount.stopped" {
		t.Fatalf("misclassified clean stop: %+v %v", pending, err)
	}
}

func TestRecoveryWorkAndStateBudgets(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := self
	old.Start = "old-process"
	for range 12 {
		if err := store.checkpoint(sessionRecord{Session: newID(), Owner: old, Updated: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	s := &Session{id: newID()}
	pending, err := store.recover(self, s.id, s.event)
	if err != nil || len(pending) != 8 {
		t.Fatalf("recovery budget: %d %v", len(pending), err)
	}
	oversized := "event-" + newID() + ".json"
	f, err := store.root.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(strings.Repeat("x", MaxStateBytes)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := store.persistEvent(testEvent()); err == nil {
		t.Fatal("state byte budget not enforced")
	}
}

func TestDaemonHandoffPreservesIdentity(t *testing.T) {
	var mu sync.Mutex
	var names []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var incoming []envelope
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			t.Error(err)
		}
		mu.Lock()
		for _, e := range incoming {
			names = append(names, e.Data.Data.Name)
		}
		mu.Unlock()
	}))
	defer server.Close()
	o := Options{ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory: filepath.Join(t.TempDir(), "microsoft-telemetry"), HTTPClient: server.Client()}
	o.MountPath = "/test/daemon-mount"
	parent, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	o.SessionID, o.StartedAt = parent.Identity()
	o.StartMonotonic = parent.StartMonotonic()
	// Complete the one startup event before simulating the handoff so this
	// test checks producer ownership independently of network retries.
	parent.reporter.close(FlushTimeout)
	parent.Detach()
	parent.Close(nil)
	o.Child = true
	child, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close(nil)
	Ready(nil, nil)
	if child.id != o.SessionID || child.startMono != o.StartMonotonic {
		t.Fatal("handoff lost identity or startup timing")
	}
	TeardownComplete()
	child.Close(nil)
	mu.Lock()
	defer mu.Unlock()
	attempts, results := 0, 0
	for _, name := range names {
		if name == "blobfuse.mount.attempted" {
			attempts++
		}
		if name == "blobfuse.mount.result" {
			results++
		}
	}
	if attempts != 1 || results != 1 {
		t.Fatalf("duplicate/missing mount lifecycle: attempts=%d results=%d", attempts, results)
	}
}

func TestRuntimeOptOutPurgesState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	s, err := Start(Options{ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory: dir, HTTPClient: server.Client(), MountPath: "/test/runtime-opt-out"})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	Ready(nil, nil)
	s.Disable()
	if Current() != nil {
		t.Fatal("collection remained active after opt-out")
	}
	store, err := openMountStore(dir, "/test/runtime-opt-out", "", false, func(string) {})
	if err != nil || store == nil {
		t.Fatalf("reopen scoped store: %v", err)
	}
	defer store.root.Close()
	entries, err := store.entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if stateName(entry.Name()) {
			t.Fatal("retained telemetry after runtime opt-out")
		}
	}
	s.Close(nil)
}

func TestStateProtectionAndOptOut(t *testing.T) {
	base := filepath.Join(t.TempDir(), "microsoft-telemetry")
	store, err := openMountStore(base, "/test/protected-mount", "", true, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	defer store.root.Close()
	e := testEvent()
	if err := store.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	path := store.root.Name()
	s, err := Start(Options{Disabled: true, StateDirectory: base, MountPath: "/test/protected-mount"})
	if err != nil || s != nil {
		t.Fatalf("opt out: %v", err)
	}
	files, err := store.entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if stateName(file.Name()) {
			t.Fatal("retained telemetry after opt out")
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	id := newID()
	if err := os.Symlink(outside, filepath.Join(path, "event-"+id+".json")); err != nil {
		t.Fatal(err)
	}
	var value Event
	if err := store.read("event-"+id+".json", &value); err == nil {
		t.Fatal("followed symlink")
	}
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := openMountStore(base, "/test/protected-mount", "", true, func(string) {}); err == nil {
		t.Fatal("accepted public state directory")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestShutdownCancelsInflightAndDoesNotRedirect(t *testing.T) {
	r := newReporter(destination{endpoint: "https://example.test", key: "x"}, nil, func(string) {},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		})})
	r.report(testEvent(), false)
	start := time.Now()
	r.close(50 * time.Millisecond)
	if time.Since(start) > time.Second {
		t.Fatal("shutdown exceeded cancellation budget")
	}
	if r.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects are allowed")
	}
}

func TestBlankDestinationAndFingerprint(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	s, err := Start(Options{StateDirectory: dir})
	if err != nil || s != nil {
		t.Fatalf("blank destination: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created state without destination")
	}
	if fingerprint(map[string]string{"a": "1", "b": "2"}) != fingerprint(map[string]string{"b": "2", "a": "1"}) {
		t.Fatal("fingerprint depends on insertion order")
	}
}
