// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/spf13/viper"
)

func TestTelemetryPipelineDerivesAttributeCacheEnablement(t *testing.T) {
	for _, components := range [][]string{
		{"libfuse", "attr_cache", "file_cache", "azstorage"},
		{"libfuse", "file_cache", "azstorage"},
	} {
		t.Run(strings.Join(components, ","), func(t *testing.T) {
			results := make(chan map[string]string, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var batch []struct {
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
				for _, item := range batch {
					if item.Data.Base.Name == "blobfuse.mount.result" {
						results <- item.Data.Base.Properties
					}
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			s, err := telemetry.Start(telemetry.Options{
				ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
				StateDirectory:   filepath.Join(t.TempDir(), "microsoft-telemetry"), MountPath: "/test/pipeline",
				HTTPClient: server.Client(),
			})
			if err != nil || s == nil {
				t.Fatalf("start: %v", err)
			}
			defer s.Disable()
			telemetryPipeline(components)
			telemetry.Ready(nil, nil)
			telemetry.TeardownComplete()
			s.Close(nil)
			select {
			case got := <-results:
				if got["pipeline_components"] != strings.Join(components, ",") {
					t.Fatalf("effective pipeline changed: %+v", got)
				}
				if _, exists := got["attr_cache_enabled"]; exists {
					t.Fatal("redundant attribute-cache flag was exported")
				}
			default:
				t.Fatal("mount result was not delivered")
			}
		})
	}
}

func TestTelemetryDisableWins(t *testing.T) {
	old := disableMicrosoftTelemetry
	defer func() { disableMicrosoftTelemetry = old }()
	t.Cleanup(viper.Reset)
	for _, tc := range []struct {
		name       string
		file, flag bool
		env        string
		want       bool
	}{
		{"default on", false, false, "false", false},
		{"config wins", true, false, "false", true},
		{"flag wins", false, true, "false", true},
		{"environment wins", false, false, "true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("microsoft-telemetry.disabled", tc.file)
			disableMicrosoftTelemetry = tc.flag
			t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", tc.env)
			o, err := telemetryOptions()
			if err != nil || o.Disabled != tc.want {
				t.Fatalf("options: %+v %v", o, err)
			}
		})
	}
}

func TestTelemetryOptionsAndAllowlist(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	if err := config.ReadConfigFromReader(strings.NewReader(`
default-working-dir: /tmp/telemetry-test
microsoft-telemetry:
  snapshot-interval-sec: 300
`)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	o, err := telemetryOptions()
	if err != nil || o.IntervalSeconds != 300 ||
		o.StateDirectory != filepath.Join("/tmp/telemetry-test", "microsoft-telemetry") {
		t.Fatalf("options: %+v %v", o, err)
	}

	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "invalid")
	if _, err := telemetryOptions(); err == nil {
		t.Fatal("invalid opt-out silently accepted")
	}
	if normalizedPlatform("/sensitive/path") != "unknown" {
		t.Fatal("unsafe platform text accepted")
	}
}

func TestTelemetryDestinationSelection(t *testing.T) {
	for _, tc := range []struct {
		name, override, embedded, want string
		testBinary                     bool
	}{
		{name: "compiled-default", embedded: "production", want: "production"},
		{name: "environment-override", embedded: "production", override: "test", want: "test"},
		{name: "empty-environment-fallback", embedded: "production", override: "  ", want: "production"},
		{name: "test-binary-ignores-default", embedded: "production", testBinary: true},
		{name: "test-binary-explicit-override", embedded: "production", override: "test", testBinary: true, want: "test"},
		{name: "unconfigured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := configuredTelemetryConnectionString(tc.override, tc.embedded, tc.testBinary)
			if got != tc.want {
				t.Fatalf("destination %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTelemetryOptOutRetriesCleanupAfterPathValidation(t *testing.T) {
	oldPath, oldDisable := options.MountPath, disableMicrosoftTelemetry
	t.Cleanup(func() {
		options.MountPath, disableMicrosoftTelemetry = oldPath, oldDisable
		viper.Reset()
	})
	viper.Reset()
	disableMicrosoftTelemetry = false
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	directory := filepath.Join(t.TempDir(), "microsoft-telemetry")
	options.MountPath = "/test/deferred-opt-out"
	previous, err := telemetry.Start(telemetry.Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory:   directory, MountPath: options.MountPath, HTTPClient: server.Client(),
	})
	if err != nil || previous == nil {
		t.Fatalf("previous session: %v", err)
	}
	previous.Detach()
	previous.Close(nil)
	// Initial opt-out cannot inspect the protected state; validation makes it
	// accessible again. No new reporter may be started during the retry.
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	viper.Set("microsoft-telemetry.disabled", true)
	viper.Set("microsoft-telemetry.state-directory", directory)
	if session := beginMountTelemetry(); session != nil {
		session.Disable()
		t.Fatal("opt-out initialized telemetry")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	retryMountTelemetryState(nil)
	if telemetry.Current() != nil {
		t.Fatal("purge retry initialized a collector")
	}
	if err := filepath.WalkDir(directory, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && (strings.HasPrefix(entry.Name(), "session-") || strings.HasPrefix(entry.Name(), "event-")) {
			t.Errorf("deferred opt-out left retained state: %s", entry.Name())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTelemetryConfigOptOutDuringDaemonHandoff(t *testing.T) {
	old := disableMicrosoftTelemetry
	disableMicrosoftTelemetry = false
	t.Cleanup(func() {
		disableMicrosoftTelemetry = old
		viper.Reset()
	})
	viper.Reset()
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	directory := filepath.Join(t.TempDir(), "microsoft-telemetry")
	session, err := telemetry.Start(telemetry.Options{
		ConnectionString: "InstrumentationKey=test;IngestionEndpoint=" + server.URL,
		StateDirectory:   directory, MountPath: "/test/config-handoff",
		HTTPClient: server.Client(),
	})
	if err != nil || session == nil {
		t.Fatalf("start: %v", err)
	}
	defer session.Disable()
	session.Detach()
	if telemetry.Current() != nil {
		t.Fatal("parent collection did not detach")
	}
	viper.Set("microsoft-telemetry.disabled", true)
	disableTelemetryOnConfigChange()
	session.HandoffFailed(errors.New("simulated daemonization failure"))
	if err := filepath.WalkDir(directory, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && (strings.HasPrefix(entry.Name(), "session-") || strings.HasPrefix(entry.Name(), "event-")) {
			t.Errorf("config opt-out did not purge the detached mount: %s", entry.Name())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
