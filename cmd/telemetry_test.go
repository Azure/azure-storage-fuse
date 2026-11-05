// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/spf13/viper"
)

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
	t.Setenv("BLOBFUSE_TELEMETRY_CONNECTION_STRING", "test-destination")
	o, err := telemetryOptions()
	if err != nil || o.IntervalSeconds != 300 || o.ConnectionString != "test-destination" ||
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
