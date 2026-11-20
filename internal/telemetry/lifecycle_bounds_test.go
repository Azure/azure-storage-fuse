// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycleBoundsOutstandingSendsWithoutAdmissionCounter(t *testing.T) {
	for _, scenario := range []string{"foreground-mounted", "startup-failed", "handoff-failed", "child-mounted"} {
		t.Run(scenario, func(t *testing.T) {
			received := make(chan envelope, 20)
			release := make(chan struct{})
			options := Options{
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					e, err := requestEvent(req)
					if err != nil {
						return nil, err
					}
					received <- e
					select {
					case <-release:
						return testResponse(200), nil
					case <-req.Context().Done():
						return nil, req.Context().Err()
					}
				})},
			}
			var parent *Session
			var parentEvents <-chan envelope
			if scenario == "child-mounted" {
				parent, parentEvents = recordingSession(t, Options{})
				parent.Detach()
				options.SessionID, options.StartMonotonic = parent.Identity(), parent.StartMonotonic()
			}
			s := testSession(t, options)
			s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
			Failure("authentication")
			mounted := scenario == "foreground-mounted" || scenario == "child-mounted"
			if mounted {
				concurrently(func() { Ready(map[string]string{"fuse_api_major": "3"}) })
				concurrently(func() { Failure("dns_resolution") })
				require.Equal(t, "authentication", s.failure, "failure after readiness changed startup category")
				TeardownComplete()
			}
			if parent != nil {
				parent.Close()
				awaitSignal(t, parent.reporter.done)
				require.Empty(t, parentEvents)
				require.Same(t, s, Current())
			}
			if scenario == "handoff-failed" {
				s.Detach()
				s.Configure(map[string]string{"auth_mode": "SPN"})
			}
			closed := make(chan struct{})
			go func() {
				concurrently(func() {
					if scenario == "handoff-failed" {
						s.HandoffFailed()
					} else {
						s.Close()
					}
				})
				close(closed)
			}()
			want := map[string]int{"blobfuse.mount.result": 1}
			if mounted {
				want["blobfuse.mount.stopped"] = 1
			}
			total := 0
			for _, count := range want {
				total += count
			}
			got := map[string]int{}
			for range total {
				select {
				case e := <-received:
					name, properties := e.Data.Data.Name, e.Data.Data.Properties
					got[name]++
					require.Equal(t, "1", properties["schema_version"])
					require.Equal(t, 2, e.Data.Data.Version)
					require.NotContains(t, properties, "account_name")
					require.Equal(t, s.Identity(), properties["mount_session_id"])
					require.Equal(t, "MSI", properties["auth_mode"])
					if name == "blobfuse.mount.result" {
						require.Contains(t, e.Data.Data.Measurements, "result")
						if mounted {
							require.EqualValues(t, 1, e.Data.Data.Measurements["result"])
						} else {
							require.Zero(t, e.Data.Data.Measurements["result"])
						}
						require.Contains(t, e.Data.Data.Measurements, "mount_latency_ms")
					}
					if scenario == "handoff-failed" {
						require.Equal(t, "daemonization", properties["failure_stage"])
					}
					if mounted {
						require.Equal(t, "3", properties["fuse_api_major"])
					}
				case <-time.After(time.Second):
					close(release)
					t.Fatal("expected lifecycle send did not start concurrently")
				}
			}
			close(release)
			awaitSignal(t, closed)
			s.HandoffFailed()
			s.Close()
			waitForSends(t, s.reporter)
			for name, count := range want {
				if got[name] != count {
					t.Fatalf("wrong lifecycle count: %+v", got)
				}
			}
			select {
			case extra := <-received:
				t.Fatalf("unguarded duplicate lifecycle send: %s", extra.Data.Data.Name)
			default:
			}
		})
	}
}
