// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/sevlyar/go-daemon"
)

var disableMicrosoftTelemetry bool

const telemetryHandoff = "_BLOBFUSE_TELEMETRY_HANDOFF"

// telemetryConnectionString currently targets the personal TEST resource only.
// TODO: Before merging to main or releasing, remove this test connection string
// and replace it with the approved BlobFuse production destination.
// BLOBFUSE_TELEMETRY_CONNECTION_STRING is the only runtime destination override.
const telemetryConnectionString = "InstrumentationKey=5caab1e6-8d2e-4ba1-b211-94ba93ba0308;" +
	"IngestionEndpoint=https://eastus-8.in.applicationinsights.azure.com/;" +
	"LiveEndpoint=https://eastus.livediagnostics.monitor.azure.com/;" +
	"ApplicationId=8357c71a-edea-4079-9250-d83849380ec5"

type microsoftTelemetryOptions struct {
	Disabled        bool   `config:"disabled"`
	StateDirectory  string `config:"state-directory"`
	MountScope      string `config:"mount-scope"`
	IntervalSeconds int    `config:"snapshot-interval-sec"`
}

func configuredTelemetryConnectionString(override, embedded string, testBinary bool) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	if testBinary {
		return ""
	}
	return strings.TrimSpace(embedded)
}

func telemetryOptions() (microsoftTelemetryOptions, error) {
	var o microsoftTelemetryOptions
	if err := config.UnmarshalKey("microsoft-telemetry", &o); err != nil {
		return o, err
	}
	// Deliberately do not bind opt-out inputs to normal override precedence.
	o.Disabled = o.Disabled || disableMicrosoftTelemetry
	if value, ok := os.LookupEnv("BLOBFUSE_DISABLE_TELEMETRY"); ok {
		disabled, err := strconv.ParseBool(value)
		if err != nil {
			return o, fmt.Errorf("invalid BLOBFUSE_DISABLE_TELEMETRY boolean")
		}
		o.Disabled = o.Disabled || disabled
	}
	work := common.DefaultWorkDir
	var configuredWork string
	if err := config.UnmarshalKey("default-working-dir", &configuredWork); err != nil {
		return o, err
	}
	if configuredWork != "" {
		work = configuredWork
	}
	if o.StateDirectory == "" {
		o.StateDirectory = filepath.Join(common.ExpandPath(work), "microsoft-telemetry")
	} else {
		o.StateDirectory = common.ExpandPath(o.StateDirectory)
	}
	if !o.Disabled && o.IntervalSeconds != 0 && (o.IntervalSeconds < 60 || o.IntervalSeconds > 3600) {
		return o, fmt.Errorf("telemetry snapshot interval must be between 60 and 3600 seconds")
	}
	return o, nil
}

func beginMountTelemetry() *telemetry.Session {
	o, err := telemetryOptions()
	if err != nil {
		log.Warn("Microsoft telemetry inactive: invalid telemetry configuration")
		return nil
	}
	input := telemetry.Options{
		Disabled: o.Disabled, ConnectionString: configuredTelemetryConnectionString(
			os.Getenv("BLOBFUSE_TELEMETRY_CONNECTION_STRING"), telemetryConnectionString, testing.Testing()),
		StateDirectory: o.StateDirectory, Interval: time.Duration(o.IntervalSeconds) * time.Second,
		MountPath: options.MountPath, MountScope: o.MountScope,
		Version: common.Blobfuse2Version, Child: daemon.WasReborn(),
		Diagnostic: func(message string) { log.Warn("%s", message) },
	}
	if daemon.WasReborn() {
		fields := strings.Split(os.Getenv(telemetryHandoff), ",")
		if len(fields) == 3 {
			input.SessionID = fields[0]
			input.StartedAt, err = time.Parse(time.RFC3339Nano, fields[1])
			if err != nil {
				log.Warn("Microsoft telemetry: invalid daemon handoff timestamp")
				input.StartedAt = time.Time{}
			}
			input.StartMonotonic, err = strconv.ParseInt(fields[2], 10, 64)
			if err != nil {
				log.Warn("Microsoft telemetry: invalid daemon startup clock")
			}
		}
		if err := os.Unsetenv(telemetryHandoff); err != nil {
			log.Warn("Microsoft telemetry: cannot clear daemon handoff")
		}
	}
	session, err := telemetry.Start(input)
	if err != nil {
		log.Warn("Microsoft telemetry inactive: configuration or state initialization failed")
		return nil
	}
	return session
}

func retryMountTelemetryState(session *telemetry.Session) {
	o, err := telemetryOptions()
	if err != nil {
		telemetry.DisableCurrent()
		log.Warn("Microsoft telemetry: invalid configuration after mount-path validation")
		return
	}
	if o.Disabled {
		if session != nil {
			session.Disable()
		}
		// Initial opt-out may have been unable to resolve the stale mount.
		// Retry its scoped purge now, without creating any reporter.
		if _, err := telemetry.Start(telemetry.Options{
			Disabled: true, StateDirectory: o.StateDirectory,
			MountPath: options.MountPath, MountScope: o.MountScope,
			Diagnostic: func(message string) { log.Warn("%s", message) },
		}); err != nil {
			log.Warn("Microsoft telemetry: opt-out state cleanup still unavailable")
		}
		return
	}
	session.RetryState()
}

func telemetryPlatform() map[string]string {
	properties := map[string]string{}
	var info syscall.Utsname
	if syscall.Uname(&info) == nil {
		var b []byte
		for _, c := range info.Release {
			if c == 0 {
				break
			}
			b = append(b, byte(c))
		}
		properties["kernel_release"] = normalizedPlatform(string(b))
	}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch key {
			case "ID":
				properties["distro_name"] = normalizedPlatform(strings.Trim(value, `"`))
			case "VERSION_ID":
				properties["distro_version"] = normalizedPlatform(strings.Trim(value, `"`))
			}
		}
	}
	return properties
}

func normalizedPlatform(value string) string {
	if len(value) > 128 {
		return "unknown"
	}
	for _, c := range value {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || strings.ContainsRune("._+-", c) {
			continue
		}
		return "unknown"
	}
	return value
}

func telemetryPipeline(components []string) {
	approved := map[string]bool{"libfuse": true, "file_cache": true, "block_cache": true,
		"attr_cache": true, "entry_cache": true, "xload": true, "distributed_cache": true, "azstorage": true}
	names := make([]string, 0, len(components))
	stream := false
	for _, name := range components {
		if name == "stream" {
			name, stream = "block_cache", true
		}
		if !approved[name] {
			name = "other"
		}
		names = append(names, name)
	}
	p := telemetryPlatform()
	p["pipeline_components"] = strings.Join(names, ",")
	p["stream_mode"] = strconv.FormatBool(stream)
	telemetry.Current().Configure(p)
}

func disableTelemetryOnConfigChange() {
	o, err := telemetryOptions()
	if err != nil || o.Disabled {
		telemetry.DisableCurrent()
	}
}

func init() {
	mountCmd.PersistentFlags().BoolVar(&disableMicrosoftTelemetry, "disable-telemetry", false,
		"Disable Microsoft lifecycle and error telemetry and delete retained telemetry state.")
}
