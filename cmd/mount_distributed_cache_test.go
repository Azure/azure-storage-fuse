/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package cmd

import (
	"slices"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- injectBlockCacheForDistCache ---------------------------------------------

func TestInjectBlockCacheForDistCache(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "no distributed_cache: unchanged",
			in:   []string{"libfuse", "file_cache", "attr_cache", "azstorage"},
			want: []string{"libfuse", "file_cache", "attr_cache", "azstorage"},
		},
		{
			name: "distributed_cache without block_cache: block_cache spliced immediately before",
			in:   []string{"libfuse", "file_cache", "distributed_cache", "attr_cache", "azstorage"},
			want: []string{"libfuse", "file_cache", "block_cache", "distributed_cache", "attr_cache", "azstorage"},
		},
		{
			name: "distributed_cache at index 0: block_cache prepended",
			in:   []string{"distributed_cache", "azstorage"},
			want: []string{"block_cache", "distributed_cache", "azstorage"},
		},
		{
			name: "empty input: unchanged",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Detect accidental mutation. slices.Clone preserves the
			// nil-vs-empty distinction for the empty-input case.
			orig := slices.Clone(tc.in)
			got := injectBlockCacheForDistCache(tc.in)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, orig, tc.in, "input slice should not be mutated")
		})
	}
}

// --- normalizeDistCacheConfig -------------------------------------------------

// setDistCacheYAML loads a YAML fragment through the real config parser
// so tests exercise the same code path mount uses.
func setDistCacheYAML(t *testing.T, yaml string) {
	t.Helper()
	viper.Reset()
	options = mountOptions{}
	// ReadFromConfigBuffer has no filename to derive the format from.
	viper.SetConfigType("yaml")
	if yaml != "" {
		err := config.ReadFromConfigBuffer([]byte(yaml))
		assert.NoError(t, err)
	}
	err := config.Unmarshal(&options)
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_NoDistCacheSignal(t *testing.T) {
	setDistCacheYAML(t, `
azstorage:
  account-name: acct
  container: c
components:
  - libfuse
  - file_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "file_cache", "azstorage"})
	assert.NoError(t, err)

	// No block_cache.* keys should have been written.
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
	assert.False(t, viper.IsSet("block_cache.mem-size-mb"))
}

func TestNormalizeDistCacheConfig_DistCacheOnlyInComponents(t *testing.T) {
	// distributed_cache in components: but no distributed_cache: section.
	// Normalize should succeed and touch no block_cache keys.
	setDistCacheYAML(t, `
read-only: true
components:
  - libfuse
  - distributed_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.NoError(t, err)
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
}

func TestNormalizeDistCacheConfig_DistCacheSectionWithoutComponents(t *testing.T) {
	// distributed_cache: set but components: omitted. Only components: enables
	// distributed_cache, so the section is ignored and nothing is fanned out.
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.NoError(t, err)
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
}

func TestDistributedCacheMountOptions_DiscoveryMethods(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "discovery endpoint",
			yaml: `
distributed_cache:
  discovery-endpoint: discovery.example.com:9065
`,
		},
		{
			name: "server list",
			yaml: `
distributed_cache:
  server-list: cache-0:9065,cache-1:9065
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setDistCacheYAML(t, tc.yaml)
			defer viper.Reset()

			assert.True(t, options.DistributedCache.isConfigured())
			assert.True(t, distCacheDiscoverySet(), "config file discovery settings are detected")
		})
	}
}

func TestNormalizeDistCacheConfig_EnvironmentAloneDoesNotEnableDistCache(t *testing.T) {
	// Launchers such as the Blob CSI driver pass their environment to every mount they start, so
	// these variables must not turn distributed_cache on for, or reject, an unrelated mount.
	t.Setenv("DISTRIBUTED_CACHE_DISCOVERY_ENDPOINT", "discovery.example.com:9065")
	t.Setenv("DISTRIBUTED_CACHE_SERVER_LIST", "cache-0:9065")
	defer viper.Reset()

	for _, components := range [][]string{
		nil,
		{"libfuse", "file_cache", "attr_cache", "azstorage"},
		{"libfuse", "block_cache", "attr_cache", "azstorage"},
		{"libfuse", "attr_cache", "azstorage"},
	} {
		setDistCacheYAML(t, "")
		assert.True(t, options.DistributedCache.isConfigured(), "environment values are still read")
		assert.False(t, distCacheDiscoverySet())
		assert.NoError(t, normalizeDistCacheConfig(components), "read-write mount, components=%v", components)
	}
}

func TestNormalizeDistCacheConfig_EnvironmentSuppliesEndpointOnceListed(t *testing.T) {
	t.Setenv("DISTRIBUTED_CACHE_DISCOVERY_ENDPOINT", "discovery.example.com:9065")
	setDistCacheYAML(t, `
read-only: true
components:
  - libfuse
  - distributed_cache
  - azstorage
`)
	defer viper.Reset()

	assert.NoError(t, normalizeDistCacheConfig(options.Components))

	var endpoint string
	assert.NoError(t, config.UnmarshalKey("distributed_cache.discovery-endpoint", &endpoint))
	assert.Equal(t, "discovery.example.com:9065", endpoint)
}

func TestIsLibfuseReadOnlyOption(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "ro", want: true},
		{value: "ro=true", want: true},
		{value: " ro ", want: true},
		{value: " ro=true ", want: true},
		{value: "rw", want: false},
		{value: "ro=false", want: false},
		{value: "ro=1", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			assert.Equal(t, tc.want, isLibfuseReadOnlyOption(tc.value))
		})
	}
}

// --- read-only gating --------------------------------------------------------

func TestNormalizeDistCacheConfig_RejectsWhenReadOnlyUnset(t *testing.T) {
	setDistCacheYAML(t, `
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

func TestNormalizeDistCacheConfig_RejectsWhenReadOnlyFalse(t *testing.T) {
	setDistCacheYAML(t, `
read-only: false
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

func TestNormalizeDistCacheConfig_DiscoveryWithoutComponentsAllowsReadWrite(t *testing.T) {
	// A discovery endpoint without distributed_cache in components: does not
	// enable it, so a read-write mount is not rejected.
	setDistCacheYAML(t, `
read-only: false
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_ReadOnlyNotCheckedWhenDistCacheAbsent(t *testing.T) {
	// No distributed_cache signal at all: read-only being unset must not error
	// on a plain file_cache mount.
	setDistCacheYAML(t, `
components:
  - libfuse
  - file_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "file_cache", "azstorage"})
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_ReadOnlyGateRunsBeforeIncompatibility(t *testing.T) {
	// Both a sibling L1 AND read-only unset: the read-only gate runs
	// first, so the error should mention read-only, not block_cache.
	setDistCacheYAML(t, `
distributed_cache:
  discovery-endpoint: d
block_cache:
  block-size-mb: 8
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

// A distributed_cache: section alongside an explicit components: that omits
// distributed_cache is ignored like any other unused section, even on a
// read-write mount, and nothing is fanned out to block_cache.
func TestNormalizeDistCacheConfig_StaleSectionIsIgnored(t *testing.T) {
	setDistCacheYAML(t, `
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
components:
  - libfuse
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "azstorage"})
	assert.NoError(t, err)
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
}

func TestNormalizeDistCacheConfig_RejectsBlockCacheInComponents(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "block_cache", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

func TestNormalizeDistCacheConfig_RejectsBlockCacheSection(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
block_cache:
  block-size-mb: 8
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

func TestNormalizeDistCacheConfig_RejectsBothSurfaces(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
block_cache:
  block-size-mb: 8
components:
  - libfuse
  - block_cache
  - distributed_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "block_cache", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

// Other L1 caches (file_cache, xload, stream) are incompatible because
// block_cache — distributed_cache's required L1 — cannot coexist with them per
// common.ValidatePipeline. Reject each via components: entry and section.
func TestNormalizeDistCacheConfig_RejectsOtherL1InComponents(t *testing.T) {
	for _, name := range []string{"file_cache", "xload", "stream"} {
		t.Run(name, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", name, "distributed_cache", "azstorage"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), name)
		})
	}
}

func TestNormalizeDistCacheConfig_RejectsOtherL1Sections(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "file_cache",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
file_cache:
  path: /tmp/fc
`,
		},
		{
			name: "xload",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
xload:
  path: /tmp/xl
`,
		},
		{
			name: "stream",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
stream:
  block-size-mb: 16
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setDistCacheYAML(t, tc.yaml)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.name)
		})
	}
}

// A sibling L1 in components: (block_cache, file_cache, xload, stream) next to a
// distributed_cache: section is an ordinary mount when distributed_cache itself
// is not listed: the section is ignored.
func TestNormalizeDistCacheConfig_SiblingL1InComponentsIgnoresSection(t *testing.T) {
	for _, name := range []string{"block_cache", "file_cache", "xload", "stream"} {
		t.Run(name, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", name, "azstorage"})
			assert.NoError(t, err)
		})
	}
}

// Same as above but the sibling L1 is signalled via its top-level YAML
// section (e.g. `block_cache:` / `file_cache:`) rather than a components:
// entry. Without distributed_cache in components:, this is an ordinary mount.
func TestNormalizeDistCacheConfig_SiblingL1SectionIgnoresDistCacheSection(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "block_cache",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
block_cache:
  block-size-mb: 8
`,
		},
		{
			name: "file_cache",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
file_cache:
  path: /tmp/fc
`,
		},
		{
			name: "xload",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
xload:
  path: /tmp/xl
`,
		},
		{
			name: "stream",
			yaml: `
read-only: true
distributed_cache:
  discovery-endpoint: d
stream:
  block-size-mb: 16
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setDistCacheYAML(t, tc.yaml)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", "azstorage"})
			assert.NoError(t, err)
		})
	}
}

// A sibling L1 tuning knob passed on the CLI (e.g.
// `--block-cache-block-size=80`, which binds to `block_cache.block-size-mb`)
// on a distributed_cache mount must be rejected — the
// user likely meant `--distributed-cache-block-size`. Parse real flags so the
// parent-key check must inspect changed descendants in the config flag tree.
func TestNormalizeDistCacheConfig_RejectsSiblingL1TuningKnobsViaCLI(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		arg     string
		wantSub string
	}{
		{
			name:    "block_cache block-size-mb",
			key:     "block_cache.block-size-mb",
			arg:     "--block-cache-block-size=80",
			wantSub: "block_cache",
		},
		{
			name:    "block_cache mem-size-mb",
			key:     "block_cache.mem-size-mb",
			arg:     "--block-cache-pool-size=4096",
			wantSub: "block_cache",
		},
		{
			name:    "block_cache disk path",
			key:     "block_cache.path",
			arg:     "--block-cache-path=/tmp/cache",
			wantSub: "block_cache",
		},
		{
			name:    "block_cache zero prefetch",
			key:     "block_cache.prefetch",
			arg:     "--block-cache-prefetch=0",
			wantSub: "block_cache",
		},
		{
			name:    "file_cache path",
			key:     "file_cache.path",
			arg:     "--tmp-path=/tmp/fc",
			wantSub: "file_cache",
		},
		{
			name:    "file_cache timeout-sec",
			key:     "file_cache.timeout-sec",
			arg:     "--file-cache-timeout=120",
			wantSub: "file_cache",
		},
		{
			name:    "xload workers",
			key:     "xload.workers",
			arg:     "--workers=100",
			wantSub: "xload",
		},
		{
			name:    "stream block size",
			key:     "stream.block-size-mb",
			arg:     "--block-size-mb=16",
			wantSub: "stream",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetMountCLIFlags()
			t.Cleanup(resetMountCLIFlags)
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
`)
			require.NoError(t, mountCmd.PersistentFlags().Parse([]string{tc.arg}))
			assert.False(t, viper.IsSet(tc.wantSub), "CLI settings must not be simulated through Viper")
			assert.True(t, config.IsSet(tc.key), "the bound child flag must be set")

			err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
			assert.False(t, viper.IsSet("block_cache.block-size-mb"), "reject before fan-out")

			// Without distributed_cache in components:, the same setting is an ordinary mount.
			assert.NoError(t, normalizeDistCacheConfig(nil))
		})
	}
}

// Sibling L1 CLI enable-flags must be rejected on a distributed_cache mount.
func TestNormalizeDistCacheConfig_RejectsSiblingL1CLIFlags(t *testing.T) {
	for _, key := range []string{"streaming", "block-cache", "preload"} {
		t.Run(key, func(t *testing.T) {
			resetMountCLIFlags()
			t.Cleanup(resetMountCLIFlags)
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			require.NoError(t, mountCmd.Flags().Parse([]string{"--" + key + "=true"}))

			err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

// An explicit `--<flag>=false` on a distributed_cache mount must NOT reject.
func TestNormalizeDistCacheConfig_DisabledSiblingL1CLIFlagsAreOK(t *testing.T) {
	for _, key := range []string{"streaming", "block-cache", "preload"} {
		t.Run(key, func(t *testing.T) {
			resetMountCLIFlags()
			t.Cleanup(resetMountCLIFlags)
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			require.NoError(t, mountCmd.Flags().Parse([]string{"--" + key + "=false"}))

			err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
			assert.NoError(t, err)
		})
	}
}

func TestNormalizeDistCacheConfig_IgnoresSectionWhenComponentsOmitDistCache(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
components:
  - libfuse
  - attr_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "attr_cache", "azstorage"})
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_IgnoresServerListWhenComponentsOmitDistCache(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  server-list: cache-0:9065
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "azstorage"})
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_AcceptsExplicitComponentsWithDistCache(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_FansOutTuningKnobs(t *testing.T) {
	// User writes tuning under distributed_cache:; normalize fans out to
	// block_cache.* (the auto-injected L1).
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
  mem-size-mb: 4096
  prefetch: 24
  parallelism: 128
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.NoError(t, err)

	// All four fan-out targets should be populated and typed correctly.
	var blockSize float64
	assert.NoError(t, config.UnmarshalKey("block_cache.block-size-mb", &blockSize))
	assert.InDelta(t, float64(32), blockSize, 0)

	var memSize uint64
	assert.NoError(t, config.UnmarshalKey("block_cache.mem-size-mb", &memSize))
	assert.Equal(t, uint64(4096), memSize)

	var prefetch uint32
	assert.NoError(t, config.UnmarshalKey("block_cache.prefetch", &prefetch))
	assert.Equal(t, uint32(24), prefetch)

	var parallelism uint32
	assert.NoError(t, config.UnmarshalKey("block_cache.parallelism", &parallelism))
	assert.Equal(t, uint32(128), parallelism)
}

func TestNormalizeDistCacheConfig_UnsetKnobsAreNotForwarded(t *testing.T) {
	// Only block-size-mb is set; the other three targets must remain unset
	// so block_cache falls back to its own defaults.
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.NoError(t, err)

	assert.True(t, viper.IsSet("block_cache.block-size-mb"))
	assert.False(t, viper.IsSet("block_cache.mem-size-mb"))
	assert.False(t, viper.IsSet("block_cache.prefetch"))
	assert.False(t, viper.IsSet("block_cache.parallelism"))
}

// resetMountCLIFlags clears flag state on the real mount flag set, which is
// what the config flag tree points at.
func resetMountCLIFlags() {
	resetFlag := func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	}
	mountCmd.Flags().VisitAll(resetFlag)
	mountCmd.PersistentFlags().VisitAll(resetFlag)
	viper.Reset()
}

// The distributed-cache tuning flags must land under distributed_cache.* and
// reach block_cache.* only through normalizeDistCacheConfig's fan-out.
func TestDistCacheTuningCLIFlagsFanOutToBlockCache(t *testing.T) {
	resetMountCLIFlags()
	defer resetMountCLIFlags()

	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)

	require.NoError(t, mountCmd.PersistentFlags().Parse([]string{
		"--distributed-cache-block-size=32",
		"--distributed-cache-node-memory=4096",
		"--distributed-cache-prefetch=24",
		"--distributed-cache-parallelism=128",
	}))

	// Flags are visible under distributed_cache.* and do not leak into block_cache.*.
	for _, key := range []string{"block-size-mb", "mem-size-mb", "prefetch", "parallelism"} {
		assert.True(t, config.IsSet("distributed_cache."+key), key)
		assert.False(t, config.IsSet("block_cache."+key), key)
	}

	assert.NoError(t, normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"}))

	var blockSize float64
	assert.NoError(t, config.UnmarshalKey("block_cache.block-size-mb", &blockSize))
	assert.InDelta(t, float64(32), blockSize, 0)

	var memSize uint64
	assert.NoError(t, config.UnmarshalKey("block_cache.mem-size-mb", &memSize))
	assert.Equal(t, uint64(4096), memSize)

	var prefetch uint32
	assert.NoError(t, config.UnmarshalKey("block_cache.prefetch", &prefetch))
	assert.Equal(t, uint32(24), prefetch)

	var parallelism uint32
	assert.NoError(t, config.UnmarshalKey("block_cache.parallelism", &parallelism))
	assert.Equal(t, uint32(128), parallelism)
}

// --- applyLibfuseReadOnlyOption ----------------------------------------------

func TestApplyLibfuseReadOnlyOption_RoSetsReadOnly(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"ro"})

	var ro bool
	assert.NoError(t, config.UnmarshalKey("read-only", &ro))
	assert.True(t, ro)
}

func TestApplyLibfuseReadOnlyOption_RoTrueSetsReadOnly(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"ro=true"})

	var ro bool
	assert.NoError(t, config.UnmarshalKey("read-only", &ro))
	assert.True(t, ro)
}

func TestApplyLibfuseReadOnlyOption_RoWithWhitespaceSetsReadOnly(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"  ro  "})

	var ro bool
	assert.NoError(t, config.UnmarshalKey("read-only", &ro))
	assert.True(t, ro)
}

func TestApplyLibfuseReadOnlyOption_RoAmongOtherOptionsSetsReadOnly(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"allow_other", "attr_timeout=120", "ro"})

	var ro bool
	assert.NoError(t, config.UnmarshalKey("read-only", &ro))
	assert.True(t, ro)
}

func TestApplyLibfuseReadOnlyOption_NoRoLeavesReadOnlyUnset(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"allow_other", "attr_timeout=120"})

	assert.False(t, viper.IsSet("read-only"))
}

func TestApplyLibfuseReadOnlyOption_EmptyOptionsIsNoop(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption(nil)

	assert.False(t, viper.IsSet("read-only"))
}

func TestApplyLibfuseReadOnlyOption_UnrelatedRoPrefixDoesNotMatch(t *testing.T) {
	// e.g. `root=...` or `rootmode=...` should not be treated as `ro`.
	viper.Reset()
	defer viper.Reset()

	applyLibfuseReadOnlyOption([]string{"rootmode=755", "root_squash"})

	assert.False(t, viper.IsSet("read-only"))
}

// End-to-end: `-o ro` satisfies the distributed_cache read-only gate.
func TestApplyLibfuseReadOnlyOption_SatisfiesDistCacheReadOnlyGate(t *testing.T) {
	setDistCacheYAML(t, `
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	// Without -o ro, the gate must reject.
	err := normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")

	// Applying -o ro flips read-only=true, so the gate must now pass.
	applyLibfuseReadOnlyOption([]string{"ro"})
	err = normalizeDistCacheConfig([]string{"libfuse", "distributed_cache", "azstorage"})
	assert.NoError(t, err)
}
