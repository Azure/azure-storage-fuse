// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestTelemetryDaemonPolicyHandoff(t *testing.T) {
	for _, scenario := range []string{"enabled", "config-disabled-after-start", "initially-disabled", "child-disabled"} {
		t.Run(scenario, func(t *testing.T) {
			old := disableMicrosoftTelemetry
			disableMicrosoftTelemetry = false
			viper.Reset()
			t.Cleanup(func() {
				disableMicrosoftTelemetry = old
				viper.Reset()
			})
			t.Setenv("BLOBFUSE_DISABLE_TELEMETRY", "false")
			server, events := telemetryReceiver(t)
			connection := server.ConnectionString
			var parent *telemetry.Session
			if scenario != "initially-disabled" {
				var err error
				parent, err = telemetry.Start(telemetry.Options{
					ConnectionString: connection, HTTPClient: server.Client,
				})
				require.NoError(t, err)
				require.NotNil(t, parent)
				defer parent.Close()
				parent.Detach()
			}
			environment := os.Environ()
			if scenario == "config-disabled-after-start" {
				environment = telemetryDaemonEnvironment(parent, environment)
				viper.Set("microsoft-telemetry.disabled", true)
			}
			environment = telemetryDaemonEnvironment(parent, environment)
			parent.Close()
			count := 0
			var inherited telemetry.Options
			for _, entry := range environment {
				key, value, _ := strings.Cut(entry, "=")
				if key == telemetryHandoff {
					count++
					require.NoError(t, applyTelemetryHandoff(&inherited, value))
				}
			}
			require.Equal(t, 1, count, "handoff entries")
			if !inherited.Disabled && (inherited.SessionID != parent.Identity() ||
				inherited.StartMonotonic != parent.StartMonotonic()) {
				t.Fatal("handoff lost identity or original startup timing")
			}
			wantDisabled := scenario == "initially-disabled" || scenario == "child-disabled"
			child := exec.Command(os.Args[0], "-test.run=^TestTelemetryDaemonHandoffChild$")
			child.Env = append(environment, server.Env...)
			child.Env = append(child.Env,
				daemon.MARK_NAME+"="+daemon.MARK_VALUE,
				"BLOBFUSE_TEST_HANDOFF_CHILD=true",
				"BLOBFUSE_TEST_HANDOFF_DISABLED="+strconv.FormatBool(wantDisabled),
				"BLOBFUSE_TEST_HANDOFF_CHILD_CONFIG="+strconv.FormatBool(scenario == "child-disabled"))
			output, err := child.CombinedOutput()
			require.NoError(t, err, "child output: %s", output)
			if wantDisabled {
				require.Empty(t, events, "disabled daemon transmitted events")
				return
			}
			require.Len(t, events, 1)
			e := (<-events).Data.Base
			require.Equal(t, "blobfuse.mount.result", e.Name)
			require.Equal(t, parent.Identity(), e.Properties["mount_session_id"])
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
	_, exists := os.LookupEnv(telemetryHandoff)
	require.False(t, exists, "child did not clear private handoff")
	if os.Getenv("BLOBFUSE_TEST_HANDOFF_DISABLED") == "true" {
		require.Nil(t, session, "child bypassed inherited/local disable")
		return
	}
	require.NotNil(t, session)
	defer session.Close()
	telemetry.Ready(nil)
}

func TestTelemetryHandoffRejectsInvalidEnabledContext(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, value := range []string{
		"", "invalid", "id,1,false", id + ",bad,false",
		id + ",0,false", id + ",1,invalid", id + ",1,false,extra",
	} {
		var input telemetry.Options
		require.Error(t, applyTelemetryHandoff(&input, value), "handoff: %q", value)
	}
	for _, value := range []string{",0,true", id + ",1,true"} {
		var input telemetry.Options
		require.NoError(t, applyTelemetryHandoff(&input, value))
		require.True(t, input.Disabled)
	}
}
