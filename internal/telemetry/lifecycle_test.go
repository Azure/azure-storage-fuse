// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplicitTeardownClassification(t *testing.T) {
	for _, tc := range []struct {
		name, class             string
		complete, early, panics bool
	}{
		{name: "completed", complete: true, class: "clean"},
		{name: "unfinished", class: "unclean_unknown"},
		{name: "early-marker", early: true, class: "unclean_unknown"},
		{name: "panic", panics: true, class: "unclean_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, events := recordingSession(t, Options{})
			var terminal envelope
			if tc.early {
				TeardownComplete()
			}
			Ready(nil)
			if tc.complete {
				TeardownComplete()
			}
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				defer s.Close()
				if tc.panics {
					panic("intentional teardown panic")
				}
			}()
			if tc.panics && recovered != "intentional teardown panic" || !tc.panics && recovered != nil {
				t.Fatalf("unexpected panic outcome: %v", recovered)
			}
			for len(events) > 0 {
				if e := <-events; e.Data.Data.Name == "blobfuse.mount.stopped" {
					terminal = e
				}
			}
			require.Equal(t, tc.class, terminal.Data.Data.Properties["termination_class"])
			uptimeField, excludedField := "last_known_uptime_seconds", "final_uptime_seconds"
			if tc.complete {
				uptimeField, excludedField = excludedField, uptimeField
			}
			require.Contains(t, terminal.Data.Data.Measurements, uptimeField)
			require.NotContains(t, terminal.Data.Data.Measurements, excludedField)
		})
	}
}
