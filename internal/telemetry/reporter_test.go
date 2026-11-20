// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReporterSendsOnce(t *testing.T) {
	var requests atomic.Int64
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) > SendTimeout {
				t.Error("send lacks bounded deadline")
			}
			return testResponse(http.StatusServiceUnavailable), nil
		})})
	defer r.cancel()
	require.NoError(t, r.report(testEvent()))
	awaitSignal(t, r.done)
	r.close(0)
	require.EqualValues(t, 1, requests.Load())
}

func TestReporterCloseIsBoundedEvenIfTransportIgnoresCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	r := newReporter(destination{endpoint: "https://example.invalid", key: "test"},
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			close(started)
			<-release
			return nil, req.Context().Err()
		})})
	defer r.cancel()
	require.NoError(t, r.report(testEvent()))
	defer func() {
		close(release)
		awaitSignal(t, r.done)
	}()
	awaitSignal(t, started)
	returned := make(chan struct{})
	go func() { r.close(20 * time.Millisecond); close(returned) }()
	awaitSignal(t, returned)
	require.ErrorIs(t, r.ctx.Err(), context.Canceled)
	require.ErrorIs(t, r.client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}
