//go:build integration && !fuse2

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

package libfuse

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// CountingBackend is a minimal pipeline component that counts StreamDir calls.
// It lets tests assert whether the kernel served a directory listing from its
// readdir cache (count unchanged) or issued a fresh READDIRPLUS (count incremented).
type CountingBackend struct {
	internal.BaseComponent
	count atomic.Int64
	dirs  map[string][]*internal.ObjAttr
}

func (c *CountingBackend) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	c.count.Add(1)
	return c.dirs[options.Name], "", nil
}

func (c *CountingBackend) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	var flags common.BitMap64
	flags.Set(internal.PropFlagIsDir)
	return &internal.ObjAttr{
		Path:  options.Name,
		Name:  options.Name,
		Flags: flags,
		Mode:  os.ModeDir | 0755,
	}, nil
}

func (c *CountingBackend) StreamDirCount() int64 { return c.count.Load() }
func (c *CountingBackend) ResetCount()           { c.count.Store(0) }

func dirFlags() common.BitMap64 {
	var bm common.BitMap64
	bm.Set(internal.PropFlagIsDir)
	return bm
}

func newCountingBackend() *CountingBackend {
	root := []*internal.ObjAttr{
		{Path: "file1.txt", Name: "file1.txt", Mode: 0644},
		{Path: "file2.txt", Name: "file2.txt", Mode: 0644},
		{Path: "subdir", Name: "subdir", Flags: dirFlags(), Mode: os.ModeDir | 0755},
	}
	sub := []*internal.ObjAttr{
		{Path: "subdir/child.txt", Name: "child.txt", Mode: 0644},
	}
	return &CountingBackend{
		dirs: map[string][]*internal.ObjAttr{
			"":        root,
			"subdir/": sub,
		},
	}
}

func klcCfg(ttlSec uint32) string {
	return fmt.Sprintf("libfuse:\n  kernel-list-cache-expiration-sec: %d\n  allow-other: true\n", ttlSec)
}

// kernelListCacheIntegrationSuite tests the end-to-end kernel directory-listing
// cache using a live FUSE mount.
//
// Run with:
//
//	go test -tags "integration fuse3" -v ./component/libfuse/... -timeout 120s
type kernelListCacheIntegrationSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (s *kernelListCacheIntegrationSuite) SetupTest() {
	s.assert = assert.New(s.T())
}

// TestColdCache verifies that the very first ls for a directory reaches the backend.
func (s *kernelListCacheIntegrationSuite) TestColdCache() {
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(30))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "")

	s.assert.Equal(int64(1), backend.StreamDirCount(),
		"cold cache: backend must be called exactly once on first ls")
}

// TestHitWithinTTL verifies that a second ls within the TTL is served from the
// kernel's readdir cache and does NOT reach the backend.
func (s *kernelListCacheIntegrationSuite) TestHitWithinTTL() {
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(30))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "") // cold — count → 1
	h.listDir(s.T(), "") // within TTL — kernel cache hit, count stays 1

	s.assert.Equal(int64(1), backend.StreamDirCount(),
		"within TTL: second ls must be served from kernel cache, not backend")
}

// TestMissAfterTTL verifies that an ls after TTL expiry causes the kernel to
// discard the cached listing and issue a fresh READDIRPLUS to the backend.
func (s *kernelListCacheIntegrationSuite) TestMissAfterTTL() {
	const ttlSec = 3
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(ttlSec))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "") // cold — count → 1
	s.assert.Equal(int64(1), backend.StreamDirCount())

	// Wait just past the TTL.  No need for 2×: opendir itself checks trackDir
	// and returns keep_cache=0 for expired entries, which makes the kernel
	// discard its cached listing and issue a fresh READDIRPLUS immediately.
	time.Sleep(time.Duration(ttlSec)*time.Second + 100*time.Millisecond)

	h.listDir(s.T(), "") // TTL expired — count → 2

	s.assert.Equal(int64(2), backend.StreamDirCount(),
		"after TTL: backend must be called again for the stale directory")
}

// TestDisabledWhenTTLZero verifies that with TTL=0 every ls reaches the backend
// (kernel caching is disabled).
func (s *kernelListCacheIntegrationSuite) TestDisabledWhenTTLZero() {
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(0))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "") // count → 1
	h.listDir(s.T(), "") // count → 2
	h.listDir(s.T(), "") // count → 3

	s.assert.Equal(int64(3), backend.StreamDirCount(),
		"TTL=0: every ls must reach the backend (no kernel caching)")
}

// TestRootAndSubdirAreCachedIndependently verifies that root and subdirectory
// listings are tracked as separate cache entries with independent TTLs.
func (s *kernelListCacheIntegrationSuite) TestRootAndSubdirAreCachedIndependently() {
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(30))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "")       // root cold — count → 1
	h.listDir(s.T(), "subdir") // subdir cold — count → 2

	// Both within TTL: neither should reach the backend again.
	h.listDir(s.T(), "")       // root cache hit — count stays 2
	h.listDir(s.T(), "subdir") // subdir cache hit — count stays 2

	s.assert.Equal(int64(2), backend.StreamDirCount(),
		"root and subdir are cached independently; repeat ls must not hit backend")
}

// TestSubdirExpiredAfterTTL verifies that a subdirectory's cache entry expires
// independently of root.
func (s *kernelListCacheIntegrationSuite) TestSubdirExpiredAfterTTL() {
	const ttlSec = 3
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(ttlSec))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "subdir") // cold — count → 1
	time.Sleep(time.Duration(ttlSec)*time.Second + 100*time.Millisecond)
	h.listDir(s.T(), "subdir") // TTL expired — count → 2

	s.assert.Equal(int64(2), backend.StreamDirCount(),
		"subdir cache must expire after TTL and trigger a fresh backend call")
}

// TestCacheHitAfterReset verifies that after the cache is reset (simulating a
// new mount or invalidation) the first ls goes to the backend again.
func (s *kernelListCacheIntegrationSuite) TestCacheHitAfterReset() {
	backend := newCountingBackend()
	h := newIntegrationHarness(s.T(), backend, klcCfg(30))
	h.start(s.T())
	defer h.stop(s.T())

	h.listDir(s.T(), "") // cold — count → 1
	h.listDir(s.T(), "") // hit — count stays 1
	s.assert.Equal(int64(1), backend.StreamDirCount())

	// Forcibly evict the kernel's cached listing via fuse_invalidate_path,
	// as the TTL sweeper would do for a stale directory.
	_ = h.lf.InvalidateKernelListCache("/")
	backend.ResetCount()

	h.listDir(s.T(), "") // cache was evicted — count → 1 again

	s.assert.Equal(int64(1), backend.StreamDirCount(),
		"after manual invalidation backend must be called again")
}

func TestKernelListCacheIntegrationSuite(t *testing.T) {
	suite.Run(t, new(kernelListCacheIntegrationSuite))
}
