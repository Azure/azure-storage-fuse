// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPanicDoesNotReportCleanShutdown(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPanicCleanupSubprocess$")
	cmd.Env = append(os.Environ(), "BLOBFUSE_TEST_PANIC_DIR="+dir)
	output, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("intentional teardown panic")) {
		t.Fatalf("helper did not panic as expected: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var batch []envelope
		if err := json.Unmarshal(line, &batch); err != nil {
			t.Fatal(err)
		}
		for _, event := range batch {
			if event.Data.Data.Name != "blobfuse.mount.stopped" {
				continue
			}
			found = true
			props := event.Data.Data.Properties
			t.Logf("Deferred cleanup ran during panic: termination_class=%s", props["termination_class"])
			if props["termination_class"] != "unclean_unknown" {
				t.Fatal("panic during incomplete teardown was incorrectly reported as clean")
			}
		}
	}
	if !found {
		t.Fatal("helper did not run deferred telemetry cleanup")
	}
}

func TestPanicCleanupSubprocess(t *testing.T) {
	dir := os.Getenv("BLOBFUSE_TEST_PANIC_DIR")
	if dir == "" {
		return
	}
	file, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
		StateDirectory:   filepath.Join(dir, "microsoft-telemetry"),
		MountPath:        filepath.Join(dir, "mount"),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				return nil, readErr
			}
			if _, writeErr := fmt.Fprintln(file, string(body)); writeErr != nil {
				return nil, writeErr
			}
			return &http.Response{StatusCode: 200, Header: http.Header{},
				Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	Ready(nil, nil)
	var mountErr error
	defer func() { s.Close(mountErr) }()
	panic("intentional teardown panic")
}

func TestExplicitTeardownClassification(t *testing.T) {
	for _, tc := range []struct {
		name, class     string
		complete, early bool
		exitErr         error
	}{
		{name: "completed", complete: true, class: "clean"},
		{name: "unfinished", class: "unclean_unknown"},
		{name: "early-marker", early: true, class: "unclean_unknown"},
		{name: "controlled-error", complete: true, exitErr: errors.New("teardown failed"), class: "runtime_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var terminal map[string]string
			s, err := Start(Options{
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=https://example.invalid",
				StateDirectory:   filepath.Join(t.TempDir(), "microsoft-telemetry"),
				MountPath:        "/test/classification",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					var batch []envelope
					if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
						return nil, err
					}
					for _, e := range batch {
						if e.Data.Data.Name == "blobfuse.mount.stopped" {
							mu.Lock()
							terminal = e.Data.Data.Properties
							mu.Unlock()
						}
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
				})},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close(nil)
			if tc.early {
				TeardownComplete()
			}
			Ready(nil, nil)
			if tc.complete {
				TeardownComplete()
			}
			s.Close(tc.exitErr)
			mu.Lock()
			defer mu.Unlock()
			if terminal["termination_class"] != tc.class {
				t.Fatalf("classification: %+v", terminal)
			}
			if _, present := terminal["shutdown_outcome"]; present {
				t.Fatal("retired shutdown_outcome was exported")
			}
		})
	}
}
