//go:build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackgroundStartupOwnedByDaemon(t *testing.T) {
	type event struct {
		Time time.Time `json:"time"`
		Data struct {
			Base struct {
				Name         string             `json:"name"`
				Properties   map[string]string  `json:"properties"`
				Measurements map[string]float64 `json:"measurements"`
			} `json:"baseData"`
		} `json:"data"`
	}
	var mu sync.Mutex
	var events []event
	resultStarted, release, cancelled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var resultOnce, releaseOnce, cancelOnce sync.Once
	m := startTelemetryMount(t, func(w http.ResponseWriter, r *http.Request) {
		var batch []event
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxItem)).Decode(&batch); err != nil {
			t.Error("invalid local telemetry payload:", err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, batch...)
		mu.Unlock()
		for _, e := range batch {
			if e.Data.Base.Name == "blobfuse.mount.result" {
				resultOnce.Do(func() { close(resultStarted) })
				select {
				case <-release:
				case <-r.Context().Done():
					cancelOnce.Do(func() { close(cancelled) })
					return
				}
			}
		}
		w.WriteHeader(200)
	})
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	select {
	case <-resultStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not start its mount result request")
	}
	select {
	case <-m.done:
		require.NoError(t, m.exitErr)
	case <-time.After(3 * time.Second):
		t.Fatal("background parent waited for blocked telemetry")
	}
	require.Less(t, time.Since(m.started), 4*time.Second, "startup waited for telemetry")
	mounted, err := isMounted(m.mountPath)
	require.NoError(t, err)
	require.True(t, mounted, "daemon exited with parent")
	select {
	case <-cancelled:
		t.Fatal("parent exit cancelled the mount result request")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	e := events[0]
	require.Equal(t, "blobfuse.mount.result", e.Data.Base.Name)
	require.NotEmpty(t, e.Data.Base.Properties["mount_session_id"])
	require.False(t, e.Time.Before(m.started) || e.Time.After(time.Now()), "invalid result observation time")
	require.EqualValues(t, 1, e.Data.Base.Measurements["result"])
	require.Positive(t, e.Data.Base.Measurements["mount_latency_ms"])
}
