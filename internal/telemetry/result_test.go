// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionReportsOneMountResult(t *testing.T) {
	for _, scenario := range []string{"foreground-mounted", "startup-failed", "handoff-failed", "child-mounted"} {
		t.Run(scenario, func(t *testing.T) {
			var options Options
			var parent *Session
			var parentEvents <-chan envelope
			if scenario == "child-mounted" {
				parent, parentEvents = recordingSession(t, Options{})
				parent.Detach()
				options.SessionID, options.StartMonotonic = parent.Identity(), parent.StartMonotonic()
			}
			s, received := recordingSession(t, options)
			s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
			Failure("authentication")
			mounted := scenario == "foreground-mounted" || scenario == "child-mounted"
			if mounted {
				concurrently(func() { Ready(map[string]string{"fuse_api_major": "3"}) })
				concurrently(func() { Failure("dns_resolution") })
				require.Equal(t, "authentication", s.failure, "failure after readiness changed startup category")
			}
			if parent != nil {
				parent.Close()
				require.Empty(t, parentEvents)
				require.Same(t, s, Current())
			}
			if scenario == "handoff-failed" {
				s.Detach()
				s.Configure(map[string]string{"auth_mode": "SPN"})
			}
			concurrently(func() {
				if scenario == "handoff-failed" {
					s.HandoffFailed()
				} else {
					s.Close()
				}
			})
			s.HandoffFailed()
			s.Close()
			want := float64(0)
			if mounted {
				want = 1
			}
			require.Len(t, received, 1)
			e := (<-received).Data.Data
			properties := e.Properties
			require.Equal(t, mountResultEvent, e.Name)
			require.Equal(t, "1", properties["schema_version"])
			require.Equal(t, 2, e.Version)
			require.NotContains(t, properties, "account_name")
			require.Equal(t, s.Identity(), properties["mount_session_id"])
			require.Equal(t, "MSI", properties["auth_mode"])
			require.Contains(t, e.Measurements, "result")
			require.Equal(t, want, e.Measurements["result"])
			require.Contains(t, e.Measurements, "mount_latency_ms")
			if scenario == "handoff-failed" {
				require.Equal(t, "daemonization", properties["failure_stage"])
			}
			if mounted {
				require.Equal(t, "3", properties["fuse_api_major"])
			}
		})
	}
}
