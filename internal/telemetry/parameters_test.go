// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigSourceUsesOnlyReportedParameters(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)
	flags := pflag.NewFlagSet("telemetry-mount", pflag.ContinueOnError)
	flags.Bool("foreground", false, "")
	flags.String("log-level", "", "")
	flags.Bool("disable-kernel-cache", false, "")
	flags.StringSliceP("o", "o", nil, "")
	config.BindPFlag("foreground", flags.Lookup("foreground"))
	config.BindPFlag("logging.level", flags.Lookup("log-level"))
	config.BindPFlag("disable-kernel-cache", flags.Lookup("disable-kernel-cache"))
	config.BindPFlag("libfuse-options", flags.Lookup("o"))
	config.BindEnv("azstorage.sas", "TEST_TELEMETRY_SAS")
	config.BindEnv("libfuse.attribute-expiration-sec", "TEST_TELEMETRY_ATTR_TIMEOUT")

	for _, tc := range []struct {
		name, yaml, env, want string
		flags                 []string
	}{
		{"unreported-inputs", "logging:\n  level: LOG_INFO\nfile_cache:\n  refresh-sec: 10\n", "", "defaults",
			[]string{"--foreground", "--log-level=LOG_WARNING", "-o", "allow_other"}},
		{"approved-flag-overrides-yaml", "disable-kernel-cache: true\n", "", "cli",
			[]string{"--disable-kernel-cache=false"}},
		{"approved-env-overrides-yaml", "libfuse:\n  attribute-expiration-sec: 30\n", "45", "env", nil},
		{"approved-fuse-flag", "", "", "cli", []string{"-o", "attr_timeout=30"}},
		{"yaml-fuse-options-override-cli-slice", "libfuse-options: ['attr_timeout=45']\n", "", "yaml",
			[]string{"-o", "attr_timeout=30"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			flags.VisitAll(func(flag *pflag.Flag) {
				flag.Changed = false
				if value, ok := flag.Value.(pflag.SliceValue); ok {
					require.NoError(t, value.Replace(nil))
				}
			})
			t.Setenv("TEST_TELEMETRY_SAS", "private-fixture")
			t.Setenv("TEST_TELEMETRY_ATTR_TIMEOUT", tc.env)
			require.NoError(t, config.ReadConfigFromReader(strings.NewReader(tc.yaml)))
			require.NoError(t, flags.Parse(tc.flags))

			s, events := recordingSession(t, Options{})
			inputs := s.ReadCustomerParameters()
			if tc.env == "" && !strings.Contains(tc.yaml, "attribute-expiration-sec") {
				require.NotContains(t, inputs.explicit, "libfuse.attribute-expiration-sec", "empty env counted as input")
			}
			// Start with the Cobra slice just as mount does, then apply YAML.
			var options struct {
				Fuse []string `config:"libfuse-options"`
			}
			options.Fuse, _ = flags.GetStringSlice("o")
			require.NoError(t, config.Unmarshal(&options))
			for _, option := range options.Fuse {
				if value, ok := strings.CutPrefix(option, "attr_timeout="); ok {
					inputs.RecordFuseOption("attribute-expiration-sec", value)
				}
			}
			s.SetCustomerParameters([]string{"libfuse", "azstorage"}, inputs)
			s.Close()

			require.Len(t, events, 1)
			result := (<-events).Data.Data
			require.Equal(t, tc.want, result.Properties["config_source"])
			if tc.want == "yaml" {
				require.Equal(t, "45", result.Properties["libfuse"+customerPrefix+"attribute-expiration-sec"])
			}
		})
	}
}

func TestStartupParameterProvenance(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(`
block_cache:
  prefetch: 0
  cleanup-on-start: false
  path: /private/cache
distributed_cache:
  parallelism: 24
file_cache:
  refresh-sec: 10
azstorage:
  tier: hot
  sas: private-fixture
`)))
	flags := pflag.NewFlagSet("telemetry", pflag.ContinueOnError)
	flags.Uint("block-size-mb", 16, "")
	flag := flags.Lookup("block-size-mb")
	config.BindPFlag("block_cache.block-size-mb", flag)
	t.Cleanup(func() { flag.Changed = false })
	require.NoError(t, flags.Set("block-size-mb", "16"))
	inputs := readCustomerParameters()

	s, events := recordingSession(t, Options{})
	s.SetCustomerParameters([]string{"block_cache", "distributed_cache", "azstorage"}, inputs)
	s.DynamicParameter("block_cache", "prefetch", 12)
	s.DynamicParameter("block_cache", "parallelism", 24)
	s.DynamicParameter("block_cache", "mem_size_bytes", 4294967296)
	s.Configure(map[string]string{"azstorage.account_name": "fixtureaccount"})
	Ready(nil)
	s.Close()

	require.Len(t, events, 1)
	e := (<-events).Data.Data
	require.Equal(t, mountResultEvent, e.Name)
	p := e.Properties
	require.Equal(t, "both", p["config_source"])
	require.Equal(t, "0", p["block_cache"+customerPrefix+"prefetch"])
	require.Equal(t, "false", p["block_cache"+customerPrefix+"cleanup-on-start"])
	require.Equal(t, "16", p["block_cache"+customerPrefix+"block-size-mb"])
	require.Equal(t, "24", p["distributed_cache"+customerPrefix+"parallelism"])
	require.Equal(t, "hot", p["azstorage"+customerPrefix+"tier"])
	require.Equal(t, "4294967296", p[dynamicPrefix+"block_cache.mem_size_bytes"])
	require.Equal(t, "fixtureaccount", p["azstorage.account_name"])
	for _, key := range []string{dynamicPrefix + "block_cache.parallelism", dynamicPrefix + "block_cache.prefetch",
		"block_cache" + customerPrefix + "disk-timeout-sec", "block_cache" + customerPrefix + "path",
		"azstorage" + customerPrefix + "sas", "file_cache" + customerPrefix + "refresh-sec"} {
		require.NotContains(t, p, key)
	}
}

func TestOversizedStartupParameterDoesNotDropResult(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	atLimit := strings.Repeat("0", maxPropertyValueBytes-1) + "1"
	oversized := "0" + atLimit
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(`
block_cache:
  prefetch: "`+oversized+`"
  parallelism: "`+atLimit+`"
libfuse:
  attribute-expiration-sec: 30
libfuse-options: ["attr_timeout=`+oversized+`"]
`)))
	inputs := readCustomerParameters()
	overrideKey := "libfuse" + customerPrefix + "attribute-expiration-sec"
	require.Equal(t, "30", inputs.values[overrideKey].value)
	inputs.RecordFuseOption("attribute-expiration-sec", oversized)
	require.True(t, inputs.explicit["libfuse.attribute-expiration-sec"])

	s, events := recordingSession(t, Options{})
	s.SetCustomerParameters([]string{"block_cache", "libfuse"}, inputs)
	s.DynamicParameter("block_cache", "prefetch", 12)
	Ready(nil)
	s.Close()

	require.Len(t, events, 1)
	result := (<-events).Data.Data.Properties
	require.Equal(t, atLimit, result["block_cache"+customerPrefix+"parallelism"])
	require.NotContains(t, result, "block_cache"+customerPrefix+"prefetch")
	require.NotContains(t, result, dynamicPrefix+"block_cache.prefetch")
	require.NotContains(t, result, overrideKey)
}

func TestLibfuseStartupParameterAliases(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(`
ignore-open-flags: true
disable-writeback-cache: true
direct-io: true
disable-kernel-cache: false
libfuse:
  ignore-open-flags: false
  disable-kernel-cache: true
  entry-expiration-sec: 30
lfuse:
  disable-kernel-cache: true
  entry-expiration-sec: 10
`)))
	flags := pflag.NewFlagSet("telemetry", pflag.ContinueOnError)
	flags.Bool("ignore-open-flags", true, "")
	flag := flags.Lookup("ignore-open-flags")
	config.BindPFlag("libfuse.ignore-open-flags", flag)
	t.Cleanup(func() { flag.Changed = false })

	prefix := "libfuse" + customerPrefix
	inputs := readCustomerParameters()
	require.Equal(t, "false", inputs.values[prefix+"ignore-open-flags"].value)
	require.Equal(t, "false", inputs.values[prefix+"disable-kernel-cache"].value)
	require.Equal(t, "10", inputs.values[prefix+"entry-expiration-sec"].value)
	require.NotContains(t, inputs.values, prefix+"disable-writeback-cache")
	require.NotContains(t, inputs.values, prefix+"direct-io")

	require.NoError(t, flags.Set("ignore-open-flags", "true"))
	inputs = readCustomerParameters()
	require.Equal(t, customerParameter{"true", sourceCLI}, inputs.values[prefix+"ignore-open-flags"])
}

func TestLegacyStreamStartupParameters(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(`
stream:
  block-size-mb: 16
  max-blocks-per-file: 16
  stream-cache-mb: 1024
block_cache:
  parallelism: 24
`)))
	s, events := recordingSession(t, Options{})
	s.SetCustomerParameters([]string{"stream"}, readCustomerParameters())
	Ready(nil)
	s.Close()

	require.Len(t, events, 1)
	result := (<-events).Data.Data.Properties
	require.Equal(t, "16", result["stream"+customerPrefix+"block-size-mb"])
	require.Equal(t, "16", result["stream"+customerPrefix+"max-blocks-per-file"])
	require.Equal(t, "1024", result["stream"+customerPrefix+"stream-cache-mb"])
	require.Equal(t, "24", result["block_cache"+customerPrefix+"parallelism"])
}
