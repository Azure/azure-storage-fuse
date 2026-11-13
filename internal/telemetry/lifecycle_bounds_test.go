// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLifecycleBoundsOutstandingSendsWithoutAdmissionCounter(t *testing.T) {
	for _, scenario := range []string{"foreground-mounted", "startup-failed", "handoff-failed", "child-mounted", "parent-startup-failed"} {
		t.Run(scenario, func(t *testing.T) {
			received := make(chan string, 20)
			release := make(chan struct{})
			s, err := Start(Options{
				DeferAttempt:     scenario == "foreground-mounted" || scenario == "handoff-failed" || scenario == "parent-startup-failed",
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var batch []envelope
					if err := json.NewDecoder(req.Body).Decode(&batch); err != nil {
						return nil, err
					}
					received <- batch[0].Data.Data.Name
					select {
					case <-release:
						return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
					case <-req.Context().Done():
						return nil, req.Context().Err()
					}
				})},
			})
			if err != nil || s == nil {
				t.Fatalf("start: %v", err)
			}
			defer s.Disable()
			parallel := func(fn func()) {
				var workers sync.WaitGroup
				for range 20 {
					workers.Add(1)
					go func() { defer workers.Done(); fn() }()
				}
				workers.Wait()
			}
			mounted := scenario == "foreground-mounted" || scenario == "child-mounted"
			if mounted {
				parallel(s.ReportAttempt)
				parallel(func() { Ready(nil) })
				TeardownComplete()
			}
			if scenario == "handoff-failed" {
				s.Detach()
			}
			closed := make(chan struct{})
			go func() {
				parallel(func() {
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
				case name := <-received:
					got[name]++
				case <-time.After(time.Second):
					close(release)
					t.Fatal("expected lifecycle send did not start concurrently")
				}
			}
			close(release)
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("repeated completion did not finish")
			}
			waitForSends(t, s.reporter)
			for name, count := range want {
				if got[name] != count {
					t.Fatalf("wrong lifecycle count: %+v", got)
				}
			}
			select {
			case extra := <-received:
				t.Fatalf("unguarded duplicate lifecycle send: %s", extra)
			default:
			}
		})
	}
}
