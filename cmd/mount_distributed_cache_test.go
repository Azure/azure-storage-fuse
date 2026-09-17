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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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
	// distributed_cache: set but components: omitted — the synthesis-path case.
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
  block-size-mb: 32
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.NoError(t, err)

	var blockSize uint32
	assert.NoError(t, config.UnmarshalKey("block_cache.block-size-mb", &blockSize))
	assert.Equal(t, uint32(32), blockSize)
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
		})
	}
}

func TestNormalizeDistCacheConfig_DiscoversEndpointFromEnvironment(t *testing.T) {
	t.Setenv("DISTRIBUTED_CACHE_DISCOVERY_ENDPOINT", "discovery.example.com:9065")
	setDistCacheYAML(t, "")
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
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

// A distributed_cache: section alongside an explicit components: that omits
// distributed_cache is silently ignored, matching how the codebase treats stray
// block_cache:/file_cache: sections. Normalize must not raise a

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

func TestNormalizeDistCacheConfig_RejectsWhenReadOnlyFalseViaCLIFlag(t *testing.T) {
	// CLI-flag synthesis path with read-only explicitly false; the
	// discovery-endpoint being set (via CLI or YAML) is the enable signal.
	setDistCacheYAML(t, `
read-only: false
distributed_cache:
  discovery-endpoint: d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
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

// A distributed_cache: section alongside an explicit components: that
// omits distributed_cache is no longer treated as a stray/no-op: the
// presence of discovery-endpoint is the enable signal. These paths are
// covered by the read-only gating and sibling-L1 rejection tests below.
func TestNormalizeDistCacheConfig_StaleSectionNoL1ErrorsWithoutReadOnly(t *testing.T) {
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
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

// The CLI-flag entry path (discovery-endpoint set via CLI/YAML) combined
// with a sibling L1 already present in components: must be rejected for
// every incompatible L1 — block_cache, file_cache, xload, stream. Mirrors
// the components-list entry path (RejectsBlockCacheInComponents /
// RejectsOtherL1InComponents), but exercises the flag-driven activation
// so we catch regressions where the gate only fires when
// distributed_cache is spelled out in components:.
func TestNormalizeDistCacheConfig_RejectsSiblingL1InComponentsViaCLIFlag(t *testing.T) {
	for _, name := range []string{"block_cache", "file_cache", "xload", "stream"} {
		t.Run(name, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", name, "azstorage"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), name)
		})
	}
}

// Same as above but the sibling L1 is signalled via its top-level YAML
// section (e.g. `block_cache:` / `file_cache:`) rather than a components:
// entry, again with distributed_cache activated through the discovery-endpoint.
func TestNormalizeDistCacheConfig_RejectsSiblingL1SectionViaCLIFlag(t *testing.T) {
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
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.name)
		})
	}
}

// A sibling L1 tuning knob passed on the CLI (e.g.
// `--block-cache-block-size=80`, which binds to `block_cache.block-size-mb`)
// alongside a distributed-cache discovery-endpoint must be rejected — the
// user likely meant `--distributed-cache-block-size`. The gate fires because
// viper's IsSet on the parent key ("block_cache") returns true once any
// nested key is set.
func TestNormalizeDistCacheConfig_RejectsSiblingL1TuningKnobsViaCLI(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		wantSub string
	}{
		{
			name:    "block_cache block-size-mb",
			key:     "block_cache.block-size-mb",
			value:   "80",
			wantSub: "block_cache",
		},
		{
			name:    "block_cache mem-size-mb",
			key:     "block_cache.mem-size-mb",
			value:   "4096",
			wantSub: "block_cache",
		},
		{
			name:    "file_cache path",
			key:     "file_cache.path",
			value:   "/tmp/fc",
			wantSub: "file_cache",
		},
		{
			name:    "file_cache timeout-sec",
			key:     "file_cache.timeout-sec",
			value:   "120",
			wantSub: "file_cache",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()
			config.Set(tc.key, tc.value)

			err := normalizeDistCacheConfig([]string{"libfuse", "azstorage"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
		})
	}
}

// Sibling L1 CLI enable-flags must be rejected when combined with
// a distributed-cache discovery-endpoint.
func TestNormalizeDistCacheConfig_RejectsSiblingL1CLIFlags(t *testing.T) {
	for _, key := range []string{"streaming", "block-cache", "preload"} {
		t.Run(key, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()
			config.Set(key, "true")

			err := normalizeDistCacheConfig(nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

// An explicit `--<flag>=false` alongside a distributed-cache discovery-endpoint must NOT reject.
func TestNormalizeDistCacheConfig_DisabledSiblingL1CLIFlagsAreOK(t *testing.T) {
	for _, key := range []string{"streaming", "block-cache", "preload"} {
		t.Run(key, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-endpoint: d
`)
			defer viper.Reset()
			config.Set(key, "false")

			err := normalizeDistCacheConfig(nil)
			assert.NoError(t, err)
		})
	}
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
