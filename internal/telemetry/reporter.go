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
	done   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	dest   destination
	client *http.Client
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
	return &reporter{
		ctx: ctx, cancel: cancel,
		dest: d, client: client,
	}
}

func (r *reporter) report(e Event) error {
	body, err := encode(e, r.dest.key)
	if err != nil {
		return err
	}
	// Session registers at most one result, before closing the reporter.
	r.done = make(chan struct{})
	done := r.done
	go func() {
		defer close(done)
		defer r.client.CloseIdleConnections()
		r.send(body)
	}()
	return nil
}

func (r *reporter) send(body []byte) {
	ctx, cancel := context.WithTimeout(r.ctx, SendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.dest.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err == nil {
		// Best effort: no response body processing, retries, or worker logging.
		_ = resp.Body.Close()
	}
}

func (r *reporter) close(budget time.Duration) {
	defer r.cancel()
	if r.done != nil && budget > 0 {
		timer := time.NewTimer(budget)
		defer timer.Stop()
		select {
		case <-r.done:
		case <-r.ctx.Done():
		case <-timer.C:
		}
	}
}
