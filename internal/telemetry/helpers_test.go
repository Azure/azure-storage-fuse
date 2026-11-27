// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testSession(t *testing.T, o Options) *Session {
	t.Helper()
	o.ConnectionString = "InstrumentationKey=test;IngestionEndpoint=https://test.in.applicationinsights.azure.com"
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return testResponse(200), nil
		})}
	}
	s, err := Start(o)
	require.NoError(t, err)
	require.NotNil(t, s)
	t.Cleanup(func() {
		s.reporter.cancel()
		s.Close()
	})
	return s
}

func testResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func requestEvent(req *http.Request) (envelope, error) {
	var batch []envelope
	if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
		return envelope{}, err
	}
	if len(batch) != 1 {
		return envelope{}, errors.New("expected exactly one telemetry event")
	}
	return batch[0], nil
}

func recordingSession(t *testing.T, o Options) (*Session, <-chan envelope) {
	t.Helper()
	events := make(chan envelope, 16)
	o.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		e, err := requestEvent(req)
		if err != nil {
			t.Error(err)
			return nil, err
		}
		select {
		case events <- e:
		default:
			t.Error("unexpected excess telemetry events")
		}
		return testResponse(200), nil
	})}
	return testSession(t, o), events
}

func awaitSignal(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}
}

func concurrently(fn func()) {
	var workers sync.WaitGroup
	for range 20 {
		workers.Add(1)
		go func() { defer workers.Done(); fn() }()
	}
	workers.Wait()
}
