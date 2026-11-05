// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
			"mount_session_id": newID(), "mount_attempt_id": newID()},
		Measurements: map[string]float64{}}
}

func TestEncodeAndSplit(t *testing.T) {
	e := testEvent()
	e.Name = "blobfuse.mount.snapshot"
	for i := 0; i < 2500; i++ {
		e.Measurements[fmt.Sprintf("fuse_read_errno_category_%04d", i)] = float64(i)
	}
	parts, err := splitSnapshot(e, "test-key")
	if err != nil || len(parts) < 2 {
		t.Fatalf("split: %d parts, %v", len(parts), err)
	}
	seen := map[string]float64{}
	ids := map[string]bool{}
	for _, part := range parts {
		b, err := encode(part, "test-key")
		if err != nil || len(b) > MaxEventBytes {
			t.Fatalf("oversized part: %d, %v", len(b), err)
		}
		id := part.Properties["event_id"]
		if ids[id] || part.Properties["snapshot_id"] != e.Properties["event_id"] {
			t.Fatal("invalid part identity")
		}
		ids[id] = true
		for k, v := range part.Measurements {
			seen[k] = v
		}
	}
	if len(seen) != len(e.Measurements) {
		t.Fatal("measurements lost")
	}
	for k, v := range e.Measurements {
		if seen[k] != v {
			t.Fatalf("measurement changed: %s", k)
		}
	}
	e.Measurements["invalid"] = math.NaN()
	if _, err := encode(e, "key"); err == nil {
		t.Fatal("accepted NaN")
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
		code          int
		body          string
		retry, failed bool
	}{
		{200, "", false, false},
		{400, "", false, true},
		{429, "", true, true},
		{503, "", true, true},
		{206, `{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":429}]}`, true, true},
		{206, `{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":400}]}`, false, true},
		{206, `{"itemsReceived":1,"itemsAccepted":1,"errors":[]}`, true, true},
		{206, `not JSON`, true, true},
	} {
		status.Store(int64(tc.code))
		body.Store(tc.body)
		retry, _, err := r.send(context.Background(), []byte("[]"))
		if retry != tc.retry || (err != nil) != tc.failed {
			t.Fatalf("status %d: retry=%v err=%v", tc.code, retry, err)
		}
	}
}

func TestSessionLifecycleAndCounters(t *testing.T) {
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
		Version: "test"})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { s.Close(nil) })
	HTTPResponse(503)
	SDKRequest(2, false)
	s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
	Ready(map[string]string{"fuse_api_major": "3"}, func() map[string]float64 {
		return map[string]float64{"fuse_read_completed": 1, "fuse_read_errno_EIO": 1}
	})
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 1000 {
				HTTPResponse(200)
			}
		})
	}
	workers.Wait()
	s.snapshot()
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
		if e.Data.Data.Name == "blobfuse.mount.snapshot" {
			m := e.Data.Data.Measurements
			if m["runtime_http_response_200"] != 8000 || m["startup_http_response_503"] != 1 ||
				m["startup_sdk_retry_attempts"] != 1 || m["fuse_read_errno_EIO"] != 1 {
				t.Fatalf("counter mismatch: %+v", m)
			}
		}
	}
	if kinds["blobfuse.mount.attempted"] != 1 || kinds["blobfuse.mount.result"] != 1 ||
		kinds["blobfuse.mount.stopped"] != 1 || kinds["blobfuse.mount.snapshot"] < 1 {
		t.Fatalf("wrong lifecycle: %+v", kinds)
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

func TestRecoveryIdentityAndStableReplay(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	dead := self
	dead.Start = "not-this-process"
	var session Session
	session.id, session.attempt, session.version = newID(), newID(), "test"
	old := sessionRecord{Session: newID(), Attempt: newID(), Owner: dead, Updated: time.Now(), Ready: true, Uptime: 42}
	alive := sessionRecord{Session: newID(), Attempt: newID(), Owner: self, Updated: time.Now(), Ready: true}
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
	store.removeEvent(e.Properties["event_id"])
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

func TestPendingCleanEvidenceOverridesUnfinishedCheckpoint(t *testing.T) {
	store := testStore(t)
	self, err := identity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	old := sessionRecord{Session: newID(), Attempt: newID(), Owner: self, Updated: time.Now(), Ready: true}
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
	s := &Session{id: newID(), attempt: newID()}
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
		if err := store.checkpoint(sessionRecord{Session: newID(), Attempt: newID(), Owner: old, Updated: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	s := &Session{id: newID(), attempt: newID()}
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

func TestDaemonHandoffPreservesIdentityAndStartupCounters(t *testing.T) {
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
	parent, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	HTTPResponse(200)
	SDKRequest(1, false)
	o.SessionID, o.AttemptID, o.StartedAt = parent.IDs()
	o.StartMonotonic = parent.StartMonotonic()
	// Complete the one startup event before simulating the handoff so this
	// test checks producer ownership independently of network retries.
	parent.reporter.close(FlushTimeout)
	parent.Detach()
	o.Child = true
	child, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close(nil)
	Ready(nil, nil)
	child.mu.Lock()
	values := child.measurements()
	child.mu.Unlock()
	if child.id != o.SessionID || child.attempt != o.AttemptID ||
		child.startMono != o.StartMonotonic || values["startup_http_response_200"] != 1 {
		t.Fatalf("handoff lost startup state: %+v", values)
	}
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

func TestRuntimeOptOutStopsNativeHooksAndPurgesState(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	dir := filepath.Join(t.TempDir(), "microsoft-telemetry")
	s, err := Start(Options{ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory: dir, HTTPClient: server.Client()})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	var enabled atomic.Bool
	s.NativeHooks(func() { enabled.Store(true) }, func() { enabled.Store(false) })
	Ready(nil, nil)
	s.Disable()
	if enabled.Load() || Current() != nil {
		t.Fatal("collection remained active after opt-out")
	}
	s.NativeHooks(func() { enabled.Store(true) }, func() { enabled.Store(false) })
	if enabled.Load() {
		t.Fatal("late initialization re-enabled native collection")
	}
	entries, err := os.ReadDir(dir)
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

func BenchmarkHTTPCounter(b *testing.B) {
	s := &Session{}
	active.Store(s)
	defer active.Store(nil)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			HTTPResponse(200)
		}
	})
}

func TestStateProtectionAndOptOut(t *testing.T) {
	store := testStore(t)
	e := testEvent()
	if err := store.persistEvent(e); err != nil {
		t.Fatal(err)
	}
	path := store.root.Name()
	s, err := Start(Options{Disabled: true, StateDirectory: path})
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
	if _, err := openStore(path, func(string) {}); err == nil {
		t.Fatal("accepted public state directory")
	}
}

func TestQueueBudget(t *testing.T) {
	r := &reporter{dest: destination{key: "x"}, diag: func(string) {}, wake: make(chan struct{}, 1)}
	for range 2000 {
		r.enqueue(testEvent(), false)
	}
	if r.bytes > MaxQueueBytes || r.dropped.Load() == 0 {
		t.Fatalf("queue not bounded: %d", r.bytes)
	}
	critical := testEvent()
	r.enqueue(critical, true)
	found := false
	for _, q := range r.queue {
		found = found || q.id == critical.Properties["event_id"]
	}
	if !found || r.bytes > MaxQueueBytes {
		t.Fatal("lifecycle not prioritized")
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
	r.enqueue(testEvent(), false)
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

func TestBodyReadBound(t *testing.T) {
	r := &reporter{dest: destination{endpoint: "https://test"}, client: &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 206, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 100000)))}, nil
		}),
	}}
	if _, _, err := r.send(context.Background(), []byte("[]")); err == nil {
		t.Fatal("accepted invalid large acknowledgement")
	}
}
