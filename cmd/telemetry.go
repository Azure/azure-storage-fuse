// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
	"github.com/sevlyar/go-daemon"
)

var disableMicrosoftTelemetry bool

const telemetryHandoff = "_BLOBFUSE_TELEMETRY_HANDOFF"

type microsoftTelemetryOptions struct {
	Disabled         bool   `config:"disabled"`
	ConnectionString string `config:"connection-string"`
	StateDirectory   string `config:"state-directory"`
	IntervalSeconds  int    `config:"snapshot-interval-sec"`
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
		Disabled: o.Disabled, ConnectionString: o.ConnectionString,
		StateDirectory: o.StateDirectory, Interval: time.Duration(o.IntervalSeconds) * time.Second,
		Version: common.Blobfuse2Version, Child: daemon.WasReborn(),
		Diagnostic: func(message string) { log.Warn("%s", message) },
	}
	if daemon.WasReborn() {
		fields := strings.Split(os.Getenv(telemetryHandoff), ",")
		if len(fields) == 4 {
			input.SessionID, input.AttemptID = fields[0], fields[1]
			input.StartedAt, err = time.Parse(time.RFC3339Nano, fields[2])
			if err != nil {
				log.Warn("Microsoft telemetry: invalid daemon handoff timestamp")
				input.StartedAt = time.Time{}
			}
			input.StartMonotonic, err = strconv.ParseInt(fields[3], 10, 64)
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
	p["attr_cache_enabled"] = strconv.FormatBool(common.ComponentInPipeline(components, "attr_cache"))
	telemetry.Current().Configure(p)
}

func disableTelemetryOnConfigChange() {
	o, err := telemetryOptions()
	if err != nil || o.Disabled {
		if s := telemetry.Current(); s != nil {
			s.Disable()
		}
	}
}

func init() {
	mountCmd.PersistentFlags().BoolVar(&disableMicrosoftTelemetry, "disable-telemetry", false,
		"Disable Microsoft lifecycle and error telemetry and delete retained telemetry state.")
	mountCmd.PersistentFlags().String("telemetry-connection-string", "",
		"Application Insights connection string for Microsoft telemetry (no default destination).")
	config.BindPFlag("microsoft-telemetry.connection-string", mountCmd.PersistentFlags().Lookup("telemetry-connection-string"))
	config.BindEnv("microsoft-telemetry.connection-string", "BLOBFUSE_TELEMETRY_CONNECTION_STRING")
}
