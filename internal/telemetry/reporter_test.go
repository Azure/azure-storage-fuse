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

func waitForSends(t *testing.T, r *reporter) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		idle := r.active == 0
		r.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("one-shot sends did not finish")
}

func TestOneShotFailuresDoNotStopFutureReports(t *testing.T) {
	for _, status := range []int{0, 408, 429, 503, 206} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int64
			r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, nil, func(string) {},
				&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					call := requests.Add(1)
					if call == 1 && status == 0 {
						return nil, errors.New("fixture transport failure")
					}
					code, body := 200, ""
					if call == 1 {
						code = status
						if code == 206 {
							body = `{"itemsReceived":1,"itemsAccepted":0,"errors":[{"index":0,"statusCode":429}]}`
						}
					}
					return &http.Response{StatusCode: code, Header: http.Header{"Retry-After": {"1"}},
						Body: io.NopCloser(strings.NewReader(body))}, nil
				})})
			defer r.close(0)
			r.report(testEvent(), false)
			waitForSends(t, r)
			time.Sleep(2200 * time.Millisecond)
			if requests.Load() != 1 || r.failures.Load() != 1 || r.dropped.Load() != 1 {
				t.Fatal("failed event retried or incorrectly accounted")
			}
			r.report(testEvent(), false)
			waitForSends(t, r)
			if requests.Load() != 2 || r.failures.Load() != 1 || r.dropped.Load() != 1 {
				t.Fatal("delivery failure stopped future reporting")
			}
		})
	}
}

func TestOneShotConcurrencyLimitDropsWithoutWaiting(t *testing.T) {
	started := make(chan struct{}, MaxConcurrentSends+1)
	release := make(chan struct{})
	var requests atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, nil, func(string) {},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			started <- struct{}{}
			select {
			case <-release:
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})})
	defer r.close(0)
	for range MaxConcurrentSends {
		r.report(testEvent(), false)
	}
	for range MaxConcurrentSends {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("concurrent sender was blocked behind another request")
		}
	}
	returned := make(chan struct{})
	go func() { r.report(testEvent(), false); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("excess event blocked instead of dropping")
	}
	if r.dropped.Load() != 1 {
		t.Fatal("excess event not dropped")
	}
	close(release)
	waitForSends(t, r)
	if requests.Load() != MaxConcurrentSends {
		t.Fatal("excess event was queued")
	}
	r.report(testEvent(), false)
	waitForSends(t, r)
	if requests.Load() != MaxConcurrentSends+1 {
		t.Fatal("slots were not released after sends completed")
	}
}

func TestRecoverySendOnceEvenOnFailureAndRepeatedDispatch(t *testing.T) {
	for _, name := range []string{"blobfuse.mount.stopped", "blobfuse.mount.result", "blobfuse.mount.recovery"} {
		t.Run(name, func(t *testing.T) {
			store, record, e, owner := completionFixture(t, name)
			if name == "blobfuse.mount.recovery" {
				if err := store.root.Remove("event-" + e.Properties["event_id"] + ".json"); err != nil {
					t.Fatal(err)
				}
			}
			var requests atomic.Int64
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				var batch []envelope
				if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
					return nil, err
				}
				if batch[0].Data.Data.Properties["event_id"] != e.Properties["event_id"] {
					t.Error("consumption changed event identity")
				}
				if _, err := store.root.Stat("session-" + record.Session + ".json"); err == nil {
					t.Error("terminal evidence was not consumed before network delivery")
				}
				return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, store, func(string) {}, client)
			defer r.close(0)
			other := newReporter(r.dest, store, func(string) {}, client)
			defer other.close(0)
			r.reportRetained(e)
			other.reportRetained(e)
			waitForSends(t, r)
			waitForSends(t, other)
			if requests.Load() != 1 {
				t.Fatalf("retained event sent %d times", requests.Load())
			}
			next := &Session{id: newID()}
			pending, err := store.recover(owner, next.id, next.event)
			if err != nil || len(pending) != 0 {
				t.Fatalf("failed delivery became later recovery/replay: %d %v", len(pending), err)
			}
		})
	}
}

func TestRecoveryConsumedWhenConcurrencyLimitDropsIt(t *testing.T) {
	store, record, e, owner := completionFixture(t, "blobfuse.mount.recovery")
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, store, func(string) {}, nil)
	defer r.close(0)
	r.active = MaxConcurrentSends
	r.reportRetained(e)
	r.active = 0
	if r.dropped.Load() != 1 {
		t.Fatal("recovery was not dropped at the admission limit")
	}

	if _, err := store.root.Stat("session-" + record.Session + ".json"); err == nil {
		t.Fatal("dropped recovery left a replayable session")
	}
	next := &Session{id: newID()}
	pending, err := store.recover(owner, next.id, next.event)
	if err != nil || len(pending) != 0 {
		t.Fatalf("admission drop became a later retry: %d %v", len(pending), err)
	}
}

func TestCancelledRecoveryIsNotRetriedAfterRestart(t *testing.T) {
	store, _, e, owner := completionFixture(t, "blobfuse.mount.recovery")
	started := make(chan struct{})
	var requests atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, store, func(string) {},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) > SendTimeout {
				t.Error("send lacks bounded deadline")
			}
			close(started)
			<-req.Context().Done()
			return nil, req.Context().Err()
		})})
	defer r.close(0)
	r.reportRetained(e)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("recovery send did not start")
	}
	begin := time.Now()
	r.close(10 * time.Millisecond)
	if time.Since(begin) > time.Second {
		t.Fatal("shutdown exceeded its cancellation budget")
	}
	next := &Session{id: newID()}
	pending, err := store.recover(owner, next.id, next.event)
	if err != nil || len(pending) != 0 || requests.Load() != 1 {
		t.Fatalf("cancelled recovery survived for retransmission: pending=%d requests=%d err=%v",
			len(pending), requests.Load(), err)
	}
}

func TestRecoveryNotSentUntilEvidenceCanBeConsumed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission fixture requires an unprivileged user")
	}
	store, record, e, _ := completionFixture(t, "blobfuse.mount.recovery")
	if err := os.Chmod(store.root.Name(), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(store.root.Name(), 0700); err != nil {
			t.Error(err)
		}
	})
	var requests atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"}, store, func(string) {},
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		})})
	defer r.close(0)
	r.reportRetained(e)
	waitForSends(t, r)
	if requests.Load() != 0 || r.dropped.Load() != 1 {
		t.Fatal("event sent despite failed durable consumption")
	}
	if _, err := store.root.Stat("session-" + record.Session + ".json"); err != nil {
		t.Fatal("failed consumption lost terminal evidence:", err)
	}
	if err := os.Chmod(store.root.Name(), 0700); err != nil {
		t.Fatal(err)
	}
	r.reportRetained(e)
	waitForSends(t, r)
	if requests.Load() != 1 {
		t.Fatal("unattempted retained event did not get its one send opportunity")
	}
}
