// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
)

func TestTelemetryDaemonPolicyHandoff(t *testing.T) {
	for _, scenario := range []string{"enabled", "disabled-then-enabled", "disabled-between-launches", "initially-disabled", "child-disabled"} {
		t.Run(scenario, func(t *testing.T) {
			old := disableMicrosoftTelemetry
			disableMicrosoftTelemetry = false
			viper.Reset()
			t.Cleanup(func() {
				disableMicrosoftTelemetry = old
				viper.Reset()
			})
			t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
			var mu sync.Mutex
			var results []map[string]string
			var attemptTimes []time.Time
			var attemptSessions []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var batch []struct {
					Time time.Time `json:"time"`
					Data struct {
						Base struct {
							Name       string            `json:"name"`
							Properties map[string]string `json:"properties"`
						} `json:"baseData"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				mu.Lock()
				for _, item := range batch {
					if item.Data.Base.Name == "blobfuse.mount.attempted" {
						attemptTimes = append(attemptTimes, item.Time)
						attemptSessions = append(attemptSessions, item.Data.Base.Properties["mount_session_id"])
					}
					if item.Data.Base.Name == "blobfuse.mount.result" {
						results = append(results, item.Data.Base.Properties)
					}
				}
				mu.Unlock()
			}))
			defer server.Close()
			connection := "InstrumentationKey=test;IngestionEndpoint=" + server.URL
			var parent *telemetry.Session
			if scenario != "initially-disabled" {
				var err error
				parent, err = telemetry.Start(telemetry.Options{
					ConnectionString: connection, HTTPClient: server.Client(), DeferAttempt: true,
				})
				if err != nil || parent == nil {
					t.Fatalf("parent: %v", err)
				}
				defer parent.Disable()
				parent.Detach()
			}
			environment := os.Environ()
			if scenario == "disabled-between-launches" {
				environment = telemetryDaemonEnvironment(parent, environment)
			}
			if scenario == "disabled-then-enabled" || scenario == "disabled-between-launches" {
				viper.Set("microsoft-telemetry.disabled", true)
				disableTelemetryOnConfigChange()
				viper.Set("microsoft-telemetry.disabled", false)
				// Rebuilding the environment on a later launch must replace
				// the stale enabled snapshot, not append a second handoff.
			}
			environment = telemetryDaemonEnvironment(parent, environment)
			// A parent's sticky decision must not rely on the child's latest
			// YAML value: the child below reads disabled=false by default.
			parent.Close()
			count := 0
			var inherited telemetry.Options
			for _, entry := range environment {
				key, value, _ := strings.Cut(entry, "=")
				if key == telemetryHandoff {
					count++
					if err := applyTelemetryHandoff(&inherited, value); err != nil {
						t.Fatal(err)
					}
				}
			}
			if count != 1 {
				t.Fatalf("handoff environment has %d policy entries", count)
			}
			if !inherited.Disabled && (inherited.SessionID != parent.Identity() ||
				inherited.StartMonotonic != parent.StartMonotonic() || !inherited.AttemptTime.Equal(parent.AttemptTime())) {
				t.Fatal("handoff lost identity or original startup timing")
			}
			wantDisabled := scenario != "enabled"
			dir := t.TempDir()
			cert := filepath.Join(dir, "ca.pem")
			if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{
				Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
			}), 0600); err != nil {
				t.Fatal(err)
			}
			filtered := make([]string, 0, len(environment))
			for _, entry := range environment {
				key, _, _ := strings.Cut(entry, "=")
				if key != daemon.MARK_NAME && key != "BLOBFUSE_TELEMETRY_CONNECTION_STRING" && key != "SSL_CERT_FILE" {
					filtered = append(filtered, entry)
				}
			}
			child := exec.Command(os.Args[0], "-test.run=^TestTelemetryDaemonHandoffChild$")
			child.Env = append(filtered,
				daemon.MARK_NAME+"="+daemon.MARK_VALUE,
				"BLOBFUSE_TELEMETRY_CONNECTION_STRING="+connection,
				"SSL_CERT_FILE="+cert,
				"BLOBFUSE_TEST_HANDOFF_CHILD=true",
				"BLOBFUSE_TEST_HANDOFF_DISABLED="+strconv.FormatBool(wantDisabled),
				"BLOBFUSE_TEST_HANDOFF_CHILD_CONFIG="+strconv.FormatBool(scenario == "child-disabled"))
			output, err := child.CombinedOutput()
			if err != nil {
				t.Fatalf("child handoff failed: %v\n%s", err, output)
			}
			mu.Lock()
			defer mu.Unlock()
			if wantDisabled && (len(results) != 0 || len(attemptTimes) != 0) {
				t.Fatal("disabled daemon transmitted lifecycle events")
			}
			if !wantDisabled && (len(results) != 1 || results[0]["mount_session_id"] != parent.Identity()) {
				t.Fatalf("enabled daemon lost result/identity: %+v", results)
			}
			if !wantDisabled && (len(attemptTimes) != 1 || !attemptTimes[0].Equal(parent.AttemptTime()) ||
				attemptSessions[0] != parent.Identity()) {
				t.Fatalf("enabled daemon lost attempt ownership, identity or time: times=%v sessions=%v", attemptTimes, attemptSessions)
			}
		})
	}
}

func TestTelemetryDaemonHandoffChild(t *testing.T) {
	if os.Getenv("BLOBFUSE_TEST_HANDOFF_CHILD") != "true" {
		return
	}
	viper.Reset()
	disableMicrosoftTelemetry = false
	viper.Set("microsoft-telemetry.disabled", os.Getenv("BLOBFUSE_TEST_HANDOFF_CHILD_CONFIG") == "true")
	session := beginMountTelemetry()
	if _, exists := os.LookupEnv(telemetryHandoff); exists {
		t.Fatal("child did not clear the private handoff")
	}
	if os.Getenv("BLOBFUSE_TEST_HANDOFF_DISABLED") == "true" {
		if session != nil {
			session.Disable()
			t.Fatal("child re-enabled telemetry despite inherited/local disable")
		}
		return
	}
	if session == nil {
		t.Fatal("enabled child did not initialize telemetry")
	}
	defer session.Disable()
	telemetry.Ready(nil)
	telemetry.TeardownComplete()
	session.Close()
}

func TestTelemetryHandoffRejectsInvalidEnabledContext(t *testing.T) {
	id := strings.Repeat("a", 32)
	validTime := time.Now().UTC().Format(time.RFC3339Nano)
	for _, value := range []string{
		"", "invalid", "id,1,false," + validTime, id + ",bad,false," + validTime,
		id + ",0,false," + validTime, id + ",1,invalid," + validTime,
		id + ",1,false", id + ",1,false,", id + ",1,false,bad",
		id + ",1,false," + time.Time{}.Format(time.RFC3339Nano),
	} {
		var input telemetry.Options
		if err := applyTelemetryHandoff(&input, value); err == nil {
			t.Fatalf("accepted invalid handoff: %q", value)
		}
	}
	for _, value := range []string{",0,true,", id + ",1,true," + validTime} {
		var input telemetry.Options
		if err := applyTelemetryHandoff(&input, value); err != nil || !input.Disabled {
			t.Fatalf("disabled policy was not inherited: input=%+v err=%v", input, err)
		}
	}
}
