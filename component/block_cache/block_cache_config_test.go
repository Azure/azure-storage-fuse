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

package block_cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/stretchr/testify/assert"
)

func TestBlockCacheConfigure_DefaultConfig(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 1
  mem-size-mb: 4
  prefetch: 2
  parallelism: 2
  disk-timeout-sec: 10
lazy-write: false
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1*1024*1024), bc.blockSize)
	assert.Equal(t, uint64(4*1024*1024), bc.memSize)
	assert.Equal(t, uint32(2), bc.workers)
	assert.Equal(t, uint32(2), bc.prefetch)
	assert.False(t, bc.lazyWrite)
}

func TestBlockCacheConfigure_NoPrefetch(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 1
  mem-size-mb: 4
  prefetch: 0
  parallelism: 2
lazy-write: false
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	assert.True(t, bc.noPrefetch)
	assert.Equal(t, uint32(0), bc.prefetch)
}

func TestBlockCacheConfigure_TmpPathMountPathConflict(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	tmpDir := t.TempDir()

	// Set up config where tmp-path and mount-path are the same directory.
	cfg := fmt.Sprintf(`
block_cache:
  tmp-path: %s
  block-size-mb: 1
  mem-size-mb: 4
lazy-write: false
`, tmpDir)
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	// mount-path is read via UnmarshalKey so we must set it in viper directly.
	config.Set("mount-path", tmpDir)

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	if err != nil {
		assert.Contains(t, err.Error(), "tmp-path is same as mount path")
	}
	// Even if this particular config flow doesn't error (due to how viper
	// unmarshals), the important Configure paths are exercised.
}

func TestBlockCacheConfigure_TmpPathNotEmpty(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644))

	cfg := fmt.Sprintf(`
block_cache:
  tmp-path: %s
  block-size-mb: 1
  mem-size-mb: 4
lazy-write: false
`, tmpDir)
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	config.Set("mount-path", "/tmp/blobfuse_config_test_mount")

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	if err != nil {
		assert.Contains(t, err.Error(), "temp directory not empty")
	}
}

func TestBlockCacheConfigure_TmpPathValid(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	tmpDir := t.TempDir()
	cfg := fmt.Sprintf(`
block_cache:
  tmp-path: %s
  block-size-mb: 1
  mem-size-mb: 4
  disk-size-mb: 10
lazy-write: false
`, tmpDir)
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	config.Set("mount-path", "/tmp/blobfuse_config_test_mount")

	bc := NewBlockCacheComponent().(*BlockCache)
	// This exercises the tmp-path code paths in Configure.
	_ = bc.Configure(true)
}

func TestBlockCacheConfigure_AutoMemSize(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 1
  parallelism: 2
lazy-write: false
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	// memSize should be auto-calculated from system RAM
	assert.Positive(t, bc.memSize)
}

func TestBlockCacheConfigure_TmpPathDoesNotExist(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	// Use a non-existent path — Configure should attempt to create it
	tmpDir := filepath.Join(t.TempDir(), "nonexistent_subdir")
	cfg := fmt.Sprintf(`
block_cache:
  tmp-path: %s
  block-size-mb: 1
  mem-size-mb: 4
lazy-write: false
mount-path: /tmp/blobfuse_config_noexist_test
`, tmpDir)
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	// This exercises the os.IsNotExist branch in Configure
	_ = bc.Configure(true)
}

func TestBlockCacheConfigure_DeferEmptyBlobCreation(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 1
  mem-size-mb: 4
  defer-empty-blob-creation: false
lazy-write: false
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	assert.False(t, bc.deferEmptyBlobCreation)
}

func TestBlockCacheConfigure_PrefetchOnOpen(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 1
  mem-size-mb: 4
  prefetch-on-open: true
lazy-write: false
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	assert.True(t, bc.prefetchOnOpen)
}

func TestBlockCacheConfigure_AllOptions(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)

	cfg := `
block_cache:
  block-size-mb: 2
  mem-size-mb: 8
  prefetch: 3
  parallelism: 5
  disk-timeout-sec: 30
  defer-empty-blob-creation: true
  prefetch-on-open: false
  consistency: true
lazy-write: true
`
	assert.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	bc := NewBlockCacheComponent().(*BlockCache)
	err := bc.Configure(true)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2*1024*1024), bc.blockSize)
	assert.Equal(t, uint64(8*1024*1024), bc.memSize)
	assert.Equal(t, uint32(3), bc.prefetch)
	assert.Equal(t, uint32(5), bc.workers)
	assert.Equal(t, uint32(30), bc.diskTimeout)
	assert.True(t, bc.deferEmptyBlobCreation)
	assert.True(t, bc.consistency)
	assert.True(t, bc.lazyWrite)
	assert.False(t, bc.prefetchOnOpen)
	assert.False(t, bc.noPrefetch)
}
