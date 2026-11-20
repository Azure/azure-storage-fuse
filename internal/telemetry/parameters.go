// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
)

const customerPrefix = ".customer_provided_parameters."
const dynamicPrefix = "dynamic_parameters."

// Only scalar tuning values are admitted. Free-form strings are never copied.
var parameterGroups = []struct{ component, numbers, booleans string }{
	{"libfuse", "default-permission attribute-expiration-sec entry-expiration-sec negative-entry-expiration-sec kernel-list-cache-expiration-sec max-fuse-threads", "direct-io disable-kernel-cache disable-writeback-cache ignore-open-flags fuse-trace"},
	{"entry_cache", "timeout-sec", ""},
	{"xload", "block-size-mb workers pool-size", "export-progress validate-md5 cleanup-on-start"},
	{"block_cache", "block-size-mb mem-size-mb disk-size-mb disk-timeout-sec prefetch parallelism", "prefetch-on-open consistency cleanup-on-start"},
	{"stream", "block-size-mb max-blocks-per-file stream-cache-mb", ""},
	{"file_cache", "timeout-sec max-size-mb refresh-sec high-threshold low-threshold", "allow-non-empty-temp cleanup-on-start sync-to-flush ignore-sync hard-limit"},
	{"attr_cache", "timeout-sec max-size-mb max-files", "no-symlinks no-cache-on-list"},
	{"distributed_cache", "port ttl-seconds block-size-mb mem-size-mb prefetch parallelism", "verify-checksum"},
	{"azstorage", "block-size-mb max-concurrency block-list-on-mount-sec max-retries max-retry-timeout-sec retry-backoff-sec max-retry-delay-sec cap-mbps-read cap-iops", "use-http virtual-directory fail-unsupported-op update-md5 validate-md5 disable-compression honour-acl cpk-enabled preserve-acl skip-mount-validation"},
}

var customerParameters = func() map[string]string {
	fields := map[string]string{"azstorage" + customerPrefix + "tier": "tier"}
	for _, group := range parameterGroups {
		for _, name := range strings.Fields(group.numbers) {
			fields[group.component+customerPrefix+name] = "number"
		}
		for _, name := range strings.Fields(group.booleans) {
			fields[group.component+customerPrefix+name] = "bool"
		}
	}
	return fields
}()

// Map resolved quantities to any explicit inputs that govern the same setting.
var dynamicParameters = map[string][]string{
	"file_cache.max_size_bytes":            {"file_cache.max-size-mb"},
	"block_cache.mem_size_bytes":           {"block_cache.mem-size-mb", "distributed_cache.mem-size-mb"},
	"block_cache.disk_size_bytes":          {"block_cache.disk-size-mb"},
	"block_cache.prefetch":                 {"block_cache.prefetch", "distributed_cache.prefetch"},
	"block_cache.parallelism":              {"block_cache.parallelism", "distributed_cache.parallelism"},
	"attr_cache.max_size_bytes":            {"attr_cache.max-size-mb", "attr_cache.max-files"},
	"distributed_cache.async_buffer_bytes": nil,
}

func startupOnlyProperty(key string) bool {
	return key == "azstorage.account_name" || key == "config_source" || key == "error_code" ||
		strings.HasPrefix(key, dynamicPrefix) || strings.Contains(key, customerPrefix)
}

func validStartupParameter(key, value string) bool {
	if len(value) > maxPropertyValueBytes {
		return false
	}
	if name, ok := strings.CutPrefix(key, dynamicPrefix); ok {
		if _, known := dynamicParameters[name]; known {
			_, err := strconv.ParseUint(value, 10, 64)
			return err == nil
		}
	}
	switch customerParameters[key] {
	case "bool":
		return value == "true" || value == "false"
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		return err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
	case "tier":
		switch value {
		case "hot", "cool", "cold", "premium", "archive", "none":
			return true
		}
	}
	return false
}

type CustomerParameters struct {
	values           map[string]customerParameter
	explicit         map[string]bool
	fuseOptionSource parameterSource
}

type customerParameter struct {
	value  string
	source parameterSource
}

type parameterSource uint8

const (
	sourceYAML parameterSource = 1 << iota
	sourceCLI
	sourceEnv
)

func (s parameterSource) String() string {
	switch s {
	case 0:
		return "defaults"
	case sourceYAML | sourceCLI:
		return "both"
	}
	var sources []string
	if s&sourceYAML != 0 {
		sources = append(sources, "yaml")
	}
	if s&sourceCLI != 0 {
		sources = append(sources, "cli")
	}
	if s&sourceEnv != 0 {
		sources = append(sources, "env")
	}
	return strings.Join(sources, "+")
}

func explicitParameterSource(key string) parameterSource {
	yaml, env, cli := config.ExplicitInput(key)
	switch {
	case cli:
		return sourceCLI
	case env:
		return sourceEnv
	case yaml:
		return sourceYAML
	default:
		return 0
	}
}

// RecordFuseOption captures an approved libfuse-options entry after translation.
func (p CustomerParameters) RecordFuseOption(setting, value string) {
	property := "libfuse" + customerPrefix + setting
	if _, allowed := customerParameters[property]; p.values == nil || !allowed {
		return
	}
	p.explicit["libfuse."+setting] = true
	if p.fuseOptionSource != 0 && validStartupParameter(property, value) {
		p.values[property] = customerParameter{value, p.fuseOptionSource}
	} else {
		delete(p.values, property)
	}
}

// ReadCustomerParameters runs before the mount command rewrites component config.
func (s *Session) ReadCustomerParameters() CustomerParameters {
	if s == nil {
		return CustomerParameters{}
	}
	return readCustomerParameters()
}

func readCustomerParameters() CustomerParameters {
	p := CustomerParameters{values: make(map[string]customerParameter), explicit: make(map[string]bool)}
	// Cobra populates the -o slice before config.Unmarshal. YAML replaces it
	// when present; the config flag tree does not overlay slice values.
	yaml, _, cli := config.ExplicitInput("libfuse-options")
	if yaml {
		p.fuseOptionSource = sourceYAML
	} else if cli {
		p.fuseOptionSource = sourceCLI
	}
	for property, kind := range customerParameters {
		component, setting, _ := strings.Cut(property, customerPrefix)
		key := component + "." + setting
		keys := []string{key}
		if component == "libfuse" {
			// Match Libfuse.Configure: the legacy section is read after
			// libfuse, but disable-kernel-cache is read only at the root.
			if setting == "disable-kernel-cache" {
				keys = []string{setting}
			} else {
				keys = append(keys, "lfuse."+setting)
			}
		}
		for _, input := range keys {
			source := explicitParameterSource(input)
			if source == 0 {
				continue
			}
			p.explicit[key] = true
			delete(p.values, property)
			var value string
			switch kind {
			case "bool":
				var raw string
				if config.UnmarshalKey(input, &raw) == nil {
					if b, err := strconv.ParseBool(raw); err == nil {
						value = strconv.FormatBool(b)
					}
				}
			case "number":
				_ = config.UnmarshalKey(input, &value)
			case "tier":
				var tier string
				if config.UnmarshalKey(input, &tier) == nil {
					value = strings.ToLower(tier)
				}
			}
			if validStartupParameter(property, value) {
				p.values[property] = customerParameter{value, source}
			}
		}
	}
	return p
}

func (s *Session) SetCustomerParameters(components []string, inputs CustomerParameters) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return
	}
	s.explicitParameters = maps.Clone(inputs.explicit)
	selected := make(map[string]bool, len(components))
	for _, name := range components {
		selected[name] = true
		if name == "stream" {
			selected["block_cache"] = true
		}
	}
	var sources parameterSource
	for property, parameter := range inputs.values {
		component, _, _ := strings.Cut(property, customerPrefix)
		if selected[component] {
			s.config[property] = parameter.value
			sources |= parameter.source
		}
	}
	s.config["config_source"] = sources.String()
}

func (s *Session) DynamicParameter(component, name string, value uint64) {
	if s == nil {
		return
	}
	key := component + "." + name
	inputs, ok := dynamicParameters[key]
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return
	}
	for _, input := range inputs {
		if s.explicitParameters[input] {
			return
		}
	}
	s.config[dynamicPrefix+key] = strconv.FormatUint(value, 10)
}
