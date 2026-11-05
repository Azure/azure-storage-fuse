// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type destination struct{ endpoint, key string }

func parseDestination(connection string) (destination, error) {
	var d destination
	for _, field := range strings.Split(connection, ";") {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "instrumentationkey":
			d.key = strings.TrimSpace(v)
		case "ingestionendpoint":
			d.endpoint = strings.TrimRight(strings.TrimSpace(v), "/")
		case "authorization":
			if !strings.EqualFold(strings.TrimSpace(v), "ikey") {
				return d, errors.New("unsupported telemetry authorization")
			}
		}
	}
	u, err := url.Parse(d.endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || d.key == "" || len(d.key) > 128 {
		return destination{}, errors.New("telemetry requires an HTTPS ingestion endpoint and instrumentation key")
	}
	d.endpoint += "/v2.1/track"
	return d, nil
}

type queued struct {
	body     []byte
	id       string
	durable  bool
	created  time.Time
	attempts int
	next     time.Time
}

type reporter struct {
	mu       sync.Mutex
	queue    []queued
	bytes    int
	wake     chan struct{}
	done     chan struct{}
	cancel   context.CancelFunc
	dest     destination
	client   *http.Client
	store    *stateStore
	diag     func(string)
	dropped  atomic.Uint64
	failures atomic.Uint64
	inFlight atomic.Bool
}

func newReporter(d destination, store *stateStore, diag func(string), client *http.Client) *reporter {
	ctx, cancel := context.WithCancel(context.Background())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns, transport.MaxIdleConnsPerHost = 1, 1
	transport.MaxConnsPerHost = 1
	if client == nil {
		client = &http.Client{Transport: transport}
	} else {
		cloned := *client
		client = &cloned
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r := &reporter{
		wake: make(chan struct{}, 1), done: make(chan struct{}), cancel: cancel,
		dest: d, store: store, diag: diag,
		client: client,
	}
	go r.run(ctx)
	return r
}

func (r *reporter) enqueue(e Event, durable bool) {
	r.enqueueRecord(e, durable, true)
}

func (r *reporter) replay(e Event) {
	r.enqueueRecord(e, true, false)
}

func (r *reporter) enqueueRecord(e Event, durable, persist bool) {
	body, err := encode(e, r.dest.key)
	if err != nil {
		r.dropped.Add(1)
		r.diag("telemetry: invalid or oversized event dropped")
		return
	}
	if durable && persist && r.store != nil {
		if err := r.store.persistEvent(e); err != nil {
			r.diag("telemetry: lifecycle persistence failed")
		}
	}
	r.mu.Lock()
	if !persist && (r.bytes+len(body) > MaxQueueBytes/2 || len(r.queue) >= MaxQueueRecords/2) {
		r.mu.Unlock()
		r.diag("telemetry: remaining replay records deferred to another startup")
		return
	}
	full := func() bool { return r.bytes+len(body) > MaxQueueBytes || len(r.queue) >= MaxQueueRecords }
	if full() && durable {
		for i := len(r.queue) - 1; i >= 0 && full(); i-- {
			if !r.queue[i].durable {
				r.bytes -= len(r.queue[i].body)
				r.queue = append(r.queue[:i], r.queue[i+1:]...)
				r.dropped.Add(1)
			}
		}
	}
	if full() {
		r.mu.Unlock()
		r.dropped.Add(1)
		r.diag("telemetry: reporting queue full")
		return
	}
	r.queue = append(r.queue, queued{body: body, id: e.Properties["event_id"], durable: durable, created: e.Time})
	r.bytes += len(body)
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *reporter) take(now time.Time) (queued, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	choice := -1
	for i, q := range r.queue {
		if now.Before(q.next) {
			continue
		}
		if choice == -1 || q.durable {
			choice = i
			if q.durable {
				break
			}
		}
	}
	if choice < 0 {
		return queued{}, false
	}
	q := r.queue[choice]
	r.inFlight.Store(true)
	r.bytes -= len(q.body)
	r.queue = append(r.queue[:choice], r.queue[choice+1:]...)
	return q, true
}

func (r *reporter) run(ctx context.Context) {
	defer close(r.done)
	defer r.client.CloseIdleConnections()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		q, ok := r.take(time.Now())
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-r.wake:
			}
			continue
		}
		if time.Since(q.created) > Retention {
			r.dropped.Add(1)
			if q.durable && r.store != nil {
				r.store.removeEvent(q.id)
			}
			r.inFlight.Store(false)
			continue
		}
		retry, delay, err := r.send(ctx, q.body)
		if err == nil {
			if q.durable && r.store != nil {
				r.store.removeEvent(q.id)
			}
			r.inFlight.Store(false)
			continue
		}
		r.failures.Add(1)
		r.diag("telemetry: delivery failed")
		if !retry {
			r.dropped.Add(1)
			if q.durable && r.store != nil {
				r.store.removeEvent(q.id)
			}
			r.inFlight.Store(false)
			continue
		}
		q.attempts++
		if delay == 0 {
			delay = time.Second * time.Duration(1<<min(q.attempts, 8))
			delay += time.Duration(rand.Int64N(int64(delay/2) + 1))
		}
		q.next = time.Now().Add(delay)
		r.mu.Lock()
		if r.bytes+len(q.body) <= MaxQueueBytes && len(r.queue) < MaxQueueRecords {
			r.queue = append(r.queue, q)
			r.bytes += len(q.body)
		} else {
			r.dropped.Add(1)
		}
		r.mu.Unlock()
		r.inFlight.Store(false)
	}
}

func (r *reporter) send(parent context.Context, body []byte) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(parent, SendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.dest.endpoint, bytes.NewReader(body))
	if err != nil {
		return false, 0, errors.New("invalid telemetry request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return true, 0, errors.New("telemetry transport failure")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPartialContent {
		var result struct {
			Received int `json:"itemsReceived"`
			Accepted int `json:"itemsAccepted"`
			Errors   []struct {
				Index  int `json:"index"`
				Status int `json:"statusCode"`
			} `json:"errors"`
		}
		decoder := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
		if err := decoder.Decode(&result); err != nil || result.Received != 1 || result.Accepted != 0 ||
			len(result.Errors) != 1 || result.Errors[0].Index != 0 {
			return true, 0, errors.New("invalid partial telemetry acknowledgement")
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return true, 0, errors.New("invalid trailing partial telemetry acknowledgement")
		}
		code := result.Errors[0].Status
		return code == 429 || code >= 500, retryAfter(resp.Header), errors.New("telemetry item rejected")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return false, 0, nil
	}
	return resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode >= 500,
		retryAfter(resp.Header), errors.New("telemetry ingestion rejected request")
}

func retryAfter(h http.Header) time.Duration {
	if seconds, err := strconv.Atoi(h.Get("Retry-After")); err == nil && seconds > 0 {
		return min(time.Duration(min(seconds, 86400))*time.Second, Retention)
	}
	if until, err := http.ParseTime(h.Get("Retry-After")); err == nil {
		return min(max(time.Until(until), 0), Retention)
	}
	return 0
}

func (r *reporter) close(budget time.Duration) {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		empty := len(r.queue) == 0
		r.mu.Unlock()
		// The worker may have removed one item for an in-flight send.
		if empty && !r.inFlight.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.cancel()
	<-r.done
}
