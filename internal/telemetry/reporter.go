// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	closed   bool
	workers  sync.WaitGroup
	shutdown sync.Once
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	dest     destination
	client   *http.Client
	diag     func(string)
}

func newReporter(d destination, diag func(string), client *http.Client) *reporter {
	ctx, cancel := context.WithCancel(context.Background())
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
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
	return r
}

func (r *reporter) report(e Event) {
	body, err := encode(e, r.dest.key)
	if err != nil {
		r.diag("telemetry: invalid or oversized event dropped")
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.ctx.Err() != nil {
		return
	}
	// Phase 1 produces at most attempted, result and stopped per process.
	// Serialize Add with close so shutdown never races new worker registration.
	r.workers.Add(1)
	go func() {
		defer r.workers.Done()
		// Delivery is best-effort. In particular, never call the logger from
		// a worker that may outlive the bounded shutdown wait.
		_ = r.send(r.ctx, body)
	}()
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
		// Each request contains one event; partial acceptance is a failure.
		return errors.New("telemetry item rejected")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.New("telemetry ingestion rejected request")
}

func (r *reporter) close(budget time.Duration) {
	defer r.cancel()
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
		case <-r.ctx.Done():
		case <-timer.C:
		}
	}
}
