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
			name: "no dist_cache: unchanged",
			in:   []string{"libfuse", "file_cache", "attr_cache", "azstorage"},
			want: []string{"libfuse", "file_cache", "attr_cache", "azstorage"},
		},
		{
			name: "dist_cache without block_cache: block_cache spliced immediately before",
			in:   []string{"libfuse", "file_cache", "dist_cache", "attr_cache", "azstorage"},
			want: []string{"libfuse", "file_cache", "block_cache", "dist_cache", "attr_cache", "azstorage"},
		},
		{
			name: "dist_cache at index 0: block_cache prepended",
			in:   []string{"dist_cache", "azstorage"},
			want: []string{"block_cache", "dist_cache", "azstorage"},
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
	// ReadFromConfigBuffer has no filename to derive the format from.
	viper.SetConfigType("yaml")
	if yaml == "" {
		return
	}
	err := config.ReadFromConfigBuffer([]byte(yaml))
	assert.NoError(t, err)
}

func TestNormalizeDistCacheConfig_NoDistCacheSignal(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
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
	// dist_cache in components: but no distributed_cache: section. Normalize
	// should succeed and touch no block_cache keys.
	setDistCacheYAML(t, `
read-only: true
components:
  - libfuse
  - dist_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "dist_cache", "azstorage"})
	assert.NoError(t, err)
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
}

func TestNormalizeDistCacheConfig_DistCacheSectionWithoutComponents(t *testing.T) {
	// distributed_cache: set but components: omitted — the synthesis-path case.
	// The CLI flag "distributed-cache" is the activation signal checked here.
	setDistCacheYAML(t, `
read-only: true
distributed-cache: true
distributed_cache:
  discovery-url: http://d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.NoError(t, err)
}

// A distributed_cache: section alongside an explicit components: that omits
// dist_cache is silently ignored, matching how the codebase treats stray
// block_cache:/file_cache: sections. Normalize must not raise a
// misleading "incompatible" error against the L1 the user actually chose.
func TestNormalizeDistCacheConfig_StaleSectionWithOtherL1IsNoop(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-url: http://d
file_cache:
  path: /tmp/fc
components:
  - libfuse
  - file_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "file_cache", "azstorage"})
	assert.NoError(t, err)
	// Fanout must not run either.
	assert.False(t, viper.IsSet("block_cache.block-size-mb"))
}

func TestNormalizeDistCacheConfig_StaleSectionNoL1IsNoop(t *testing.T) {
	// Explicit components: without dist_cache; stray section ignored.
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-url: http://d
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
  discovery-url: http://d
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "block_cache", "dist_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

func TestNormalizeDistCacheConfig_RejectsBlockCacheSection(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-url: http://d
block_cache:
  block-size-mb: 8
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "dist_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

func TestNormalizeDistCacheConfig_RejectsBothSurfaces(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-url: http://d
block_cache:
  block-size-mb: 8
components:
  - libfuse
  - block_cache
  - dist_cache
  - azstorage
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "block_cache", "dist_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block_cache")
}

// Other L1 caches (file_cache, xload, stream) are incompatible because
// block_cache — dist_cache's required L1 — cannot coexist with them per
// common.ValidatePipeline. Reject each via components: entry and section.
func TestNormalizeDistCacheConfig_RejectsOtherL1InComponents(t *testing.T) {
	for _, name := range []string{"file_cache", "xload", "stream"} {
		t.Run(name, func(t *testing.T) {
			setDistCacheYAML(t, `
read-only: true
distributed_cache:
  discovery-url: http://d
`)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", name, "dist_cache", "azstorage"})
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
  discovery-url: http://d
file_cache:
  path: /tmp/fc
`,
		},
		{
			name: "xload",
			yaml: `
read-only: true
distributed_cache:
  discovery-url: http://d
xload:
  path: /tmp/xl
`,
		},
		{
			name: "stream",
			yaml: `
read-only: true
distributed_cache:
  discovery-url: http://d
stream:
  block-size-mb: 16
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setDistCacheYAML(t, tc.yaml)
			defer viper.Reset()

			err := normalizeDistCacheConfig([]string{"libfuse", "dist_cache", "azstorage"})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.name)
		})
	}
}

func TestNormalizeDistCacheConfig_FansOutTuningKnobs(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
distributed-cache: true
distributed_cache:
  discovery-url: http://d
  block-size-mb: 32
  mem-size-mb: 4096
  prefetch: 24
  parallelism: 128
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
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
distributed-cache: true
distributed_cache:
  discovery-url: http://d
  block-size-mb: 32
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig(nil)
	assert.NoError(t, err)

	assert.True(t, viper.IsSet("block_cache.block-size-mb"))
	assert.False(t, viper.IsSet("block_cache.mem-size-mb"))
	assert.False(t, viper.IsSet("block_cache.prefetch"))
	assert.False(t, viper.IsSet("block_cache.parallelism"))
}

func TestNormalizeDistCache_RequiresReadOnly(t *testing.T) {
	setDistCacheYAML(t, `
components:
  - libfuse
  - dist_cache
  - azstorage
distributed_cache:
  discovery-url: http://foo:9065
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "dist_cache", "azstorage"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

func TestNormalizeDistCache_ReadOnlyPasses(t *testing.T) {
	setDistCacheYAML(t, `
read-only: true
components:
  - libfuse
  - dist_cache
  - azstorage
distributed_cache:
  discovery-url: http://foo:9065
`)
	defer viper.Reset()

	err := normalizeDistCacheConfig([]string{"libfuse", "dist_cache", "azstorage"})
	assert.NoError(t, err)
}
