// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type destination struct{ endpoint, key string }

var ingestionHostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.in\.applicationinsights\.azure\.com$`)

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
		u.RawQuery != "" || u.ForceQuery || strings.Contains(d.endpoint, "#") || u.Path != "" ||
		u.Port() != "" && u.Port() != "443" || strings.HasSuffix(u.Host, ":") ||
		d.key == "" || len(d.key) > 128 {
		return destination{}, errors.New("telemetry requires an HTTPS ingestion endpoint and instrumentation key")
	}
	if !ingestionHostname.MatchString(strings.ToLower(u.Hostname())) {
		return destination{}, errors.New("telemetry requires an Azure Public regional Application Insights ingestion hostname")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = "/v2.1/track"
	d.endpoint = u.String()
	return d, nil
}

type reporter struct {
	workers sync.WaitGroup
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	dest    destination
	client  *http.Client
}

func newReporter(d destination, client *http.Client) *reporter {
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
		dest: d, client: client,
	}
	return r
}

func (r *reporter) report(e Event) error {
	body, err := encode(e, r.dest.key)
	if err != nil {
		return err
	}
	if r.ctx.Err() != nil {
		return nil
	}
	// Session serializes registration and prevents reports once closing starts.
	r.workers.Add(1)
	go func() {
		defer r.workers.Done()
		// Delivery is best-effort. In particular, never call the logger from
		// a worker that may outlive the bounded shutdown wait.
		_ = r.send(r.ctx, body)
	}()
	return nil
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
	// Session calls close once, after registering its final event.
	go func() {
		r.workers.Wait()
		r.client.CloseIdleConnections()
		close(r.done)
	}()
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
