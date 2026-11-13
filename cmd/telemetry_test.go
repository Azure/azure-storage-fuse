// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestTelemetryPipelineNormalizesComponents(t *testing.T) {
	for _, tc := range []struct {
		name       string
		components []string
		pipeline   string
		stream     string
	}{
		{"approved-order", []string{"libfuse", "file_cache", "attr_cache", "azstorage"}, "libfuse,file_cache,attr_cache,azstorage", "false"},
		{"stream-alias", []string{"libfuse", "stream", "azstorage"}, "libfuse,block_cache,azstorage", "true"},
		{"custom-component", []string{"libfuse", "private-plugin", "azstorage"}, "libfuse,other,azstorage", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, events := telemetryReceiver(t)
			s, err := telemetry.Start(telemetry.Options{
				ConnectionString: server.ConnectionString,
				HTTPClient:       server.Client,
			})
			require.NoError(t, err)
			require.NotNil(t, s)
			defer s.Close()
			telemetryPipeline(tc.components)
			telemetry.Ready(nil)
			telemetry.TeardownComplete()
			s.Close()
			var result map[string]string
			for len(events) > 0 {
				if e := <-events; e.Data.Base.Name == "blobfuse.mount.result" {
					result = e.Data.Base.Properties
				}
			}
			require.Equal(t, tc.pipeline, result["pipeline_components"])
			require.Equal(t, tc.stream, result["stream_mode"])
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
`)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
	o, err := telemetryOptions()
	if err != nil || o.Disabled {
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
