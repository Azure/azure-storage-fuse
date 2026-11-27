// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry/telemetrytest"
)

type telemetryTestEvent struct {
	Time time.Time `json:"time"`
	Data struct {
		Base struct {
			Name       string            `json:"name"`
			Properties map[string]string `json:"properties"`
		} `json:"baseData"`
	} `json:"data"`
}

func telemetryReceiver(t *testing.T) (*telemetrytest.Receiver, <-chan telemetryTestEvent) {
	t.Helper()
	events := make(chan telemetryTestEvent, 16)
	server := telemetrytest.NewReceiver(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []telemetryTestEvent
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		for _, e := range batch {
			select {
			case events <- e:
			default:
				t.Error("unexpected excess telemetry events")
			}
		}
	}))
	return server, events
}
