// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

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
	"IngestionEndpoint=https://eastus-8.in.applicationinsights.azure.com/"

type microsoftTelemetryOptions struct {
	Disabled bool `config:"disabled"`
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
		Version:    common.Blobfuse2Version,
		Diagnostic: func(message string) { log.Warn("%s", message) },
	}
	if daemon.WasReborn() {
		if err := applyTelemetryHandoff(&input, os.Getenv(telemetryHandoff)); err != nil {
			input.Disabled = true
			log.Warn("Microsoft telemetry inactive: invalid daemon policy handoff")
		}
		if err := os.Unsetenv(telemetryHandoff); err != nil {
			log.Warn("Microsoft telemetry: cannot clear daemon handoff")
		}
	}
	session, err := telemetry.Start(input)
	if err != nil {
		log.Warn("Microsoft telemetry inactive: configuration or initialization failed")
		return nil
	}
	return session
}

func applyTelemetryHandoff(input *telemetry.Options, value string) error {
	fields := strings.Split(value, ",")
	if len(fields) != 3 {
		return fmt.Errorf("invalid telemetry handoff fields")
	}
	disabled, err := strconv.ParseBool(fields[2])
	if err != nil {
		return fmt.Errorf("invalid telemetry handoff policy")
	}
	input.Disabled = input.Disabled || disabled
	if input.Disabled {
		return nil
	}
	id, err := hex.DecodeString(fields[0])
	if err != nil || len(id) != 16 || fields[0] != strings.ToLower(fields[0]) {
		return fmt.Errorf("invalid telemetry handoff identity")
	}
	start, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || start <= 0 {
		return fmt.Errorf("invalid telemetry handoff clock")
	}
	input.SessionID, input.StartMonotonic = fields[0], start
	return nil
}

func telemetryDaemonEnvironment(session *telemetry.Session, environment []string) []string {
	id, start := "", int64(0)
	if session != nil {
		id, start = session.Identity(), session.StartMonotonic()
	}
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != telemetryHandoff {
			result = append(result, entry)
		}
	}
	return append(result, telemetryHandoff+"="+strings.Join([]string{
		id, strconv.FormatInt(start, 10), strconv.FormatBool(session == nil),
	}, ","))
}

func telemetryPlatform() map[string]string {
	properties := map[string]string{}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		properties["deployment_environment"] = "kubernetes"
	}
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
	session := telemetry.Current()
	if session == nil {
		return
	}
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
	session.Configure(p)
}

func init() {
	mountCmd.PersistentFlags().BoolVar(&disableMicrosoftTelemetry, "disable-telemetry", false,
		"Disable Microsoft mount startup telemetry.")
}
