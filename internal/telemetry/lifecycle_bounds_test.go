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
	for _, scenario := range []string{"foreground-mounted", "startup-failed", "handoff-failed", "child-mounted", "parent-startup-failed"} {
		t.Run(scenario, func(t *testing.T) {
			received := make(chan envelope, 20)
			release := make(chan struct{})
			attemptStarted := make(chan (<-chan struct{}), 1)
			options := Options{
				DeferAttempt: scenario == "foreground-mounted" || scenario == "handoff-failed" || scenario == "parent-startup-failed",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					e, err := requestEvent(req)
					if err != nil {
						return nil, err
					}
					received <- e
					if scenario == "child-mounted" && e.Data.Data.Name == "blobfuse.mount.attempted" {
						select {
						case attemptStarted <- req.Context().Done():
						default:
						}
					}
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
				parent, parentEvents = recordingSession(t, Options{DeferAttempt: true})
				parent.Detach()
				parent.ReportAttempt()
				options.SessionID, options.StartMonotonic, options.AttemptTime =
					parent.Identity(), parent.StartMonotonic(), parent.AttemptTime()
			}
			s := testSession(t, options)
			if parent != nil {
				var cancelled <-chan struct{}
				select {
				case cancelled = <-attemptStarted:
				case <-time.After(time.Second):
					t.Fatal("child did not start its attempted request before readiness")
				}
				start := time.Now()
				parent.Close()
				require.Less(t, time.Since(start), 500*time.Millisecond, "parent waited for child delivery")
				awaitSignal(t, parent.reporter.done)
				require.Empty(t, parentEvents)
				require.False(t, parent.attemptReported)
				require.Equal(t, parent.Identity(), s.Identity())
				require.Equal(t, parent.StartMonotonic(), s.StartMonotonic())
				require.True(t, parent.AttemptTime().Equal(s.AttemptTime()))
				require.Same(t, s, Current())
				select {
				case <-cancelled:
					t.Fatal("parent exit cancelled child delivery")
				default:
				}
			}
			s.Configure(map[string]string{"auth_mode": "MSI", "account_name": "MUST-NOT-BE-SENT"})
			Failure("authentication")
			mounted := scenario == "foreground-mounted" || scenario == "child-mounted"
			if mounted {
				concurrently(s.ReportAttempt)
				concurrently(func() { Ready(map[string]string{"fuse_api_major": "3"}) })
				concurrently(func() { Failure("dns_resolution") })
				require.Equal(t, "authentication", s.failure, "failure after readiness changed startup category")
				TeardownComplete()
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
			want := map[string]int{"blobfuse.mount.attempted": 1, "blobfuse.mount.result": 1}
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
					require.NotContains(t, properties, "account_name")
					require.Equal(t, s.Identity(), properties["mount_session_id"])
					if name == "blobfuse.mount.attempted" {
						require.True(t, e.Time.Equal(s.AttemptTime()), "original attempt time lost")
					} else {
						require.Equal(t, "MSI", properties["auth_mode"])
						if scenario == "handoff-failed" {
							require.Equal(t, "failure", properties["outcome"])
							require.Equal(t, "daemonization", properties["failure_stage"])
						}
						if mounted {
							require.Equal(t, "3", properties["fuse_api_major"])
						}
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
