// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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

type reporter struct {
	mu       sync.Mutex
	active   int
	closed   bool
	workers  sync.WaitGroup
	shutdown sync.Once
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	dest     destination
	client   *http.Client
	store    atomic.Pointer[stateStore]
	diag     func(string)
	dropped  atomic.Uint64
	failures atomic.Uint64
}

func newReporter(d destination, store *stateStore, diag func(string), client *http.Client) *reporter {
	ctx, cancel := context.WithCancel(context.Background())
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns, transport.MaxIdleConnsPerHost = MaxConcurrentSends, MaxConcurrentSends
		transport.MaxConnsPerHost = MaxConcurrentSends
		client = &http.Client{Transport: transport}
	} else {
		cloned := *client
		client = &cloned
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r := &reporter{
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
		dest: d, diag: diag, client: client,
	}
	r.store.Store(store)
	return r
}

// report consumes retained lifecycle evidence before the single send opportunity.
// A crash after consumption can lose the event, but cannot replay that send.
func (r *reporter) report(e Event, lifecycle bool) {
	r.reportEvent(e, lifecycle, false)
}

func (r *reporter) reportRetained(e Event) {
	r.reportEvent(e, true, true)
}

func (r *reporter) reportEvent(e Event, lifecycle, retained bool) {
	body, err := encode(e, r.dest.key)
	if err != nil {
		r.dropped.Add(1)
		r.diag("telemetry: invalid or oversized event dropped")
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.ctx.Err() != nil {
		r.dropped.Add(1)
		return
	}
	if lifecycle {
		if store := r.store.Load(); store != nil {
			if err := store.consumeEvent(e, retained); err != nil {
				r.dropped.Add(1)
				r.diag("telemetry: cannot consume lifecycle evidence; event not sent")
				return
			}
		}
	}
	if time.Since(e.Time) > Retention {
		r.dropped.Add(1)
		r.diag("telemetry: expired event discarded")
		return
	}
	if r.active >= MaxConcurrentSends {
		r.dropped.Add(1)
		r.diag("telemetry: concurrent send limit reached; event dropped")
		return
	}
	r.active++
	r.workers.Add(1)
	go func() {
		defer r.workers.Done()
		defer func() {
			r.mu.Lock()
			r.active--
			r.mu.Unlock()
		}()
		if err := r.send(r.ctx, body); err != nil {
			r.failures.Add(1)
			r.dropped.Add(1)
			r.diag("telemetry: delivery failed; event discarded without retry")
		}
	}()
}

func (r *reporter) attachStore(store *stateStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store.Store(store)
}

func (r *reporter) send(parent context.Context, body []byte) error {
	ctx, cancel := context.WithTimeout(parent, SendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.dest.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid telemetry request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return errors.New("telemetry transport failure")
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
			return errors.New("invalid partial telemetry acknowledgement")
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return errors.New("invalid trailing partial telemetry acknowledgement")
		}
		return errors.New("telemetry item rejected")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.New("telemetry ingestion rejected request")
}

func (r *reporter) close(budget time.Duration) {
	r.shutdown.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
		go func() {
			r.workers.Wait()
			r.client.CloseIdleConnections()
			close(r.done)
		}()
	})
	if budget > 0 {
		timer := time.NewTimer(budget)
		defer timer.Stop()
		select {
		case <-r.done:
		case <-timer.C:
		}
	}
	r.cancel()
	<-r.done
}
