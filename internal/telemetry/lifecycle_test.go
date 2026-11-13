// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
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
)

func TestExplicitTeardownClassification(t *testing.T) {
	for _, tc := range []struct {
		name, class             string
		complete, early, panics bool
		exitErr                 error
	}{
		{name: "completed", complete: true, class: "clean"},
		{name: "unfinished", class: "unclean_unknown"},
		{name: "early-marker", early: true, class: "unclean_unknown"},
		{name: "panic", panics: true, class: "unclean_unknown"},
		{name: "incomplete-error", exitErr: errors.New("teardown failed"), class: "unclean_unknown"},
		{name: "completed-with-unrelated-error", complete: true, exitErr: errors.New("post-teardown failure"), class: "clean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var terminal envelope
			s, err := Start(Options{
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					var batch []envelope
					if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
						return nil, err
					}
					for _, e := range batch {
						if e.Data.Data.Name == "blobfuse.mount.stopped" {
							mu.Lock()
							terminal = e
							mu.Unlock()
						}
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
				})},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if tc.early {
				TeardownComplete()
			}
			Ready(nil)
			if tc.complete {
				TeardownComplete()
			}
			var recovered any
			returned := func() error {
				defer func() { recovered = recover() }()
				defer s.Close()
				if tc.panics {
					panic("intentional teardown panic")
				}
				return tc.exitErr
			}()
			if tc.panics && recovered != "intentional teardown panic" || !tc.panics && recovered != nil {
				t.Fatalf("unexpected panic outcome: %v", recovered)
			}
			if returned != tc.exitErr {
				t.Fatal("telemetry cleanup changed the caller's error")
			}
			mu.Lock()
			defer mu.Unlock()
			if terminal.Data.Data.Properties["termination_class"] != tc.class {
				t.Fatalf("classification: %+v", terminal)
			}
			if _, present := terminal.Data.Data.Properties["shutdown_outcome"]; present {
				t.Fatal("retired shutdown_outcome was exported")
			}
			uptimeField, excludedField := "last_known_uptime_seconds", "final_uptime_seconds"
			if tc.complete {
				uptimeField, excludedField = excludedField, uptimeField
			}
			if _, present := terminal.Data.Data.Measurements[uptimeField]; !present {
				t.Fatalf("shutdown missing %s: %+v", uptimeField, terminal.Data.Data.Measurements)
			}
			if _, present := terminal.Data.Data.Measurements[excludedField]; present {
				t.Fatalf("shutdown incorrectly includes %s", excludedField)
			}
		})
	}
}

func TestFailureOnlyChangesCategoryBeforeReadiness(t *testing.T) {
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	})
	if err != nil || s == nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Close()
	Failure("authentication")
	s.mu.Lock()
	before := s.failure
	s.mu.Unlock()
	if before != "authentication" {
		t.Fatalf("startup category was not recorded: %q", before)
	}
	Ready(nil)
	var workers sync.WaitGroup
	for range 20 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			Ready(nil)
			Failure("dns_resolution")
		}()
	}
	workers.Wait()
	s.mu.Lock()
	after := s.failure
	s.mu.Unlock()
	if after != before {
		t.Fatalf("failure after readiness changed startup category: %q -> %q", before, after)
	}
}
