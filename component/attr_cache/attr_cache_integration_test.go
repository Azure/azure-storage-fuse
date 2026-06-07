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

package attr_cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	lbfs "github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// attrCacheIntegrationTestSuite wires AttrCache -> LoopbackFS so every
// operation actually touches the filesystem.  No mocks are used.
type attrCacheIntegrationTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	attrCache   *AttrCache
	loopbackDir string
}

func (suite *attrCacheIntegrationTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	suite.Require().NoError(err)
	suite.assert = assert.New(suite.T())

	dir, err := os.MkdirTemp("", "attr_cache_integration_*")
	suite.Require().NoError(err)
	suite.loopbackDir = dir

	cfg := fmt.Sprintf(
		"attr_cache:\n  timeout-sec: 30\n  max-size-mb: 32\n\nloopbackfs:\n  path: %s\n",
		dir,
	)
	suite.Require().NoError(config.ReadConfigFromReader(strings.NewReader(cfg)))

	loopback := lbfs.NewLoopbackFSComponent()
	suite.Require().NoError(loopback.Configure(true))
	suite.Require().NoError(loopback.Start(context.Background()))

	ac := NewAttrCacheComponent()
	ac.SetNextComponent(loopback)
	suite.Require().NoError(ac.Configure(true))
	suite.Require().NoError(ac.Start(context.Background()))
	suite.attrCache = ac.(*AttrCache)
}

func (suite *attrCacheIntegrationTestSuite) TearDownTest() {
	_ = suite.attrCache.Stop()
	os.RemoveAll(suite.loopbackDir)
}

// ---- helpers ----------------------------------------------------------------

// touchFile creates a file (and any parent dirs) directly in the loopback dir.
func (suite *attrCacheIntegrationTestSuite) touchFile(relPath string) {
	full := filepath.Join(suite.loopbackDir, relPath)
	suite.Require().NoError(os.MkdirAll(filepath.Dir(full), 0755))
	f, err := os.Create(full)
	suite.Require().NoError(err)
	f.Close()
}

// mkDir creates a directory in the loopback dir.
func (suite *attrCacheIntegrationTestSuite) mkDir(relPath string) {
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.loopbackDir, relPath), 0755))
}

// cacheViaGetAttr calls GetAttr through the cache, populating the cache.
// It asserts that the path is absent or invalid before the call, then confirms
// it is cached and valid afterwards.
func (suite *attrCacheIntegrationTestSuite) cacheViaGetAttr(path string) *attrCacheItem {
	// Pre-condition: entry must not already be present in the LRU.
	_, ok := suite.attrCache.lru.Peek(path)
	suite.assert.False(ok, "cacheViaGetAttr: path %q already has an entry in the LRU before GetAttr", path)

	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.NoError(err)

	// Post-condition: entry must now be present.
	item, ok := suite.attrCache.lru.Peek(path)
	suite.assert.True(ok, "cacheViaGetAttr: path %q not found in LRU after GetAttr", path)
	return item
}

// forceExpire moves the cached item's cachedAt into the past so it is stale.
func (suite *attrCacheIntegrationTestSuite) forceExpire(path string) {
	if item, ok := suite.attrCache.lru.Peek(path); ok {
		item.cachedAt = time.Now().Add(-suite.attrCache.cacheTimeout - time.Second)
	}
}

// createFileViaCache creates a file through attr_cache and returns the handle.
func (suite *attrCacheIntegrationTestSuite) createFileViaCache(name string) *handlemap.Handle {
	h, err := suite.attrCache.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0644})
	suite.assert.NoError(err)
	return h
}

// ---- GetAttr tests ----------------------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestGetAttrCacheMiss() {
	suite.touchFile("miss.txt")
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "miss.txt"})
	suite.assert.NoError(err)
	suite.assert.Equal("miss.txt", attr.Path)

	item, ok := suite.attrCache.lru.Peek("miss.txt")
	suite.assert.True(ok)
	suite.assert.True(item.exists)
	suite.assert.NotNil(item.attr)
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrCacheHit() {
	suite.touchFile("hit.txt")

	// First call: populates cache.
	suite.cacheViaGetAttr("hit.txt")

	item, _ := suite.attrCache.lru.Peek("hit.txt")
	cachedAt := item.cachedAt

	// Second call: must be served from cache; cachedAt must not change.
	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "hit.txt"})
	suite.assert.NoError(err)

	item, _ = suite.attrCache.lru.Peek("hit.txt")
	suite.assert.Equal(cachedAt, item.cachedAt, "cache hit should not refresh cachedAt")
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrTTLExpiry() {
	suite.touchFile("ttl.txt")
	suite.cacheViaGetAttr("ttl.txt")

	suite.forceExpire("ttl.txt")

	item, _ := suite.attrCache.lru.Peek("ttl.txt")
	staleTime := item.cachedAt

	// After TTL expiry the cache should refetch, producing a fresh cachedAt.
	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ttl.txt"})
	suite.assert.NoError(err)

	item, _ = suite.attrCache.lru.Peek("ttl.txt")
	suite.assert.True(item.cachedAt.After(staleTime), "TTL expiry should refresh the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrNegativeCache() {
	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ghost.txt"})
	suite.assert.True(os.IsNotExist(err))

	item, ok := suite.attrCache.lru.Peek("ghost.txt")
	suite.assert.True(ok, "negative entry must be cached")
	suite.assert.False(item.exists)
	suite.assert.Nil(item.attr)
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrNegativeTTLExpiry() {
	// Populate a negative entry.
	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "late.txt"})
	suite.assert.True(os.IsNotExist(err))

	suite.forceExpire("late.txt")

	// Create the file in loopback (behind the cache).
	suite.touchFile("late.txt")

	// After TTL expires, the cache should re-check and find the file.
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "late.txt"})
	suite.assert.NoError(err)
	suite.assert.Equal("late.txt", attr.Path)

	item, _ := suite.attrCache.lru.Peek("late.txt")
	suite.assert.True(item.exists, "negative entry should be replaced by positive one")
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrValidEntryNotExpired() {
	suite.touchFile("fresh.txt")
	item := suite.cacheViaGetAttr("fresh.txt")
	originalCachedAt := item.cachedAt

	// Repeated calls within TTL must not re-fetch.
	for i := 0; i < 5; i++ {
		attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "fresh.txt"})
		suite.assert.NoError(err)
		suite.assert.Equal("fresh.txt", attr.Path)
	}
	item, _ = suite.attrCache.lru.Peek("fresh.txt")
	suite.assert.Equal(originalCachedAt, item.cachedAt)
}

// ---- Directory tests --------------------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestCreateDirInvalidatesCache() {
	// Pre-populate a stale entry for the dir.
	suite.mkDir("newdir")
	suite.cacheViaGetAttr("newdir")

	// Remove from disk then re-create via the cache.
	os.Remove(filepath.Join(suite.loopbackDir, "newdir"))

	err := suite.attrCache.CreateDir(internal.CreateDirOptions{Name: "newdir2", Mode: 0755})
	suite.assert.NoError(err)

	// The created path should not be in the cache (invalidated by CreateDir).
	_, ok := suite.attrCache.lru.Peek("newdir2")
	suite.assert.False(ok)
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteDirMarksDirectoryNegative() {
	suite.mkDir("todelete")
	suite.cacheViaGetAttr("todelete")

	err := suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "todelete"})
	suite.assert.NoError(err)

	item, ok := suite.attrCache.lru.Peek("todelete")
	suite.assert.True(ok)
	suite.assert.False(item.exists, "deleted dir must be a negative entry")
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteDirPrefixIsolation() {
	// "a" and "ab" must be independent cache entries.
	suite.mkDir("a")
	suite.mkDir("ab")
	suite.cacheViaGetAttr("a")
	suite.cacheViaGetAttr("ab")

	err := suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "a"})
	suite.assert.NoError(err)

	aItem, _ := suite.attrCache.lru.Peek("a")
	suite.assert.False(aItem.exists, `"a" must be negative`)

	abItem, ok := suite.attrCache.lru.Peek("ab")
	suite.assert.True(ok)
	suite.assert.True(abItem.exists, `"ab" must remain a positive entry`)
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteDirMarksChildrenNegative() {
	suite.mkDir("parent")
	suite.mkDir("parent/child")
	suite.touchFile("parent/child/file.txt")

	// Cache all three entries.
	suite.cacheViaGetAttr("parent")
	suite.cacheViaGetAttr("parent/child")
	suite.cacheViaGetAttr("parent/child/file.txt")

	// Remove children first, then the empty parent dir.
	os.Remove(filepath.Join(suite.loopbackDir, "parent/child/file.txt"))
	os.Remove(filepath.Join(suite.loopbackDir, "parent/child"))
	err := suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "parent"})
	suite.assert.NoError(err)

	for _, p := range []string{"parent", "parent/child", "parent/child/file.txt"} {
		item, ok := suite.attrCache.lru.Peek(p)
		suite.assert.True(ok, "entry %s should still be in cache", p)
		suite.assert.False(item.exists, "entry %s should be negative after parent dir deletion", p)
	}
}

func (suite *attrCacheIntegrationTestSuite) TestRenameDirMarksSourceNegativeInvalidatesDest() {
	suite.mkDir("srcdir")
	suite.cacheViaGetAttr("srcdir")

	err := suite.attrCache.RenameDir(internal.RenameDirOptions{Src: "srcdir", Dst: "dstdir"})
	suite.assert.NoError(err)

	// Source must be negative.
	srcItem, ok := suite.attrCache.lru.Peek("srcdir")
	suite.assert.True(ok)
	suite.assert.False(srcItem.exists, "renamed-away dir must be a negative entry")

	// Destination, if cached, must be absent (invalidated by rename).
	_, ok = suite.attrCache.lru.Peek("dstdir")
	suite.assert.False(ok, "dst should be deleted from cache after rename")
}

func (suite *attrCacheIntegrationTestSuite) TestSyncDirInvalidatesSubtree() {
	suite.mkDir("syncdir")
	suite.mkDir("syncdir/sub")
	suite.touchFile("syncdir/sub/f.txt")

	suite.cacheViaGetAttr("syncdir")
	suite.cacheViaGetAttr("syncdir/sub")
	suite.cacheViaGetAttr("syncdir/sub/f.txt")

	err := suite.attrCache.SyncDir(internal.SyncDirOptions{Name: "syncdir"})
	suite.assert.NoError(err)

	for _, p := range []string{"syncdir", "syncdir/sub", "syncdir/sub/f.txt"} {
		_, ok := suite.attrCache.lru.Peek(p)
		suite.assert.False(ok, "SyncDir must remove %s from cache", p)
	}
}

// ---- ReadDir / StreamDir tests ----------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestReadDirPopulatesCache() {
	suite.mkDir("listdir")
	suite.touchFile("listdir/a.txt")
	suite.touchFile("listdir/b.txt")

	pathList, err := suite.attrCache.ReadDir(internal.ReadDirOptions{Name: "listdir"})
	suite.assert.NoError(err)
	suite.assert.Len(pathList, 2)

	for _, attr := range pathList {
		item, ok := suite.attrCache.lru.Peek(attr.Path)
		suite.assert.True(ok, "ReadDir must cache %s", attr.Path)
		suite.assert.True(item.exists)
	}
}

func (suite *attrCacheIntegrationTestSuite) TestReadDirCacheHitOnSubsequentGetAttr() {
	suite.mkDir("listdir2")
	suite.touchFile("listdir2/c.txt")

	_, err := suite.attrCache.ReadDir(internal.ReadDirOptions{Name: "listdir2"})
	suite.assert.NoError(err)

	item, _ := suite.attrCache.lru.Peek("listdir2/c.txt")
	cachedAt := item.cachedAt

	// GetAttr must be served from cache; cachedAt unchanged.
	_, err = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "listdir2/c.txt"})
	suite.assert.NoError(err)

	item, _ = suite.attrCache.lru.Peek("listdir2/c.txt")
	suite.assert.Equal(cachedAt, item.cachedAt, "GetAttr after ReadDir should be a cache hit")
}

func (suite *attrCacheIntegrationTestSuite) TestStreamDirPopulatesCache() {
	suite.mkDir("streamdir")
	suite.touchFile("streamdir/x.txt")
	suite.touchFile("streamdir/y.txt")

	pathList, _, err := suite.attrCache.StreamDir(internal.StreamDirOptions{Name: "streamdir"})
	suite.assert.NoError(err)
	suite.assert.Len(pathList, 2)

	for _, attr := range pathList {
		item, ok := suite.attrCache.lru.Peek(attr.Path)
		suite.assert.True(ok, "StreamDir must cache %s", attr.Path)
		suite.assert.True(item.exists)
	}
}

func (suite *attrCacheIntegrationTestSuite) TestStreamDirCacheHitOnSubsequentGetAttr() {
	suite.mkDir("streamdir2")
	suite.touchFile("streamdir2/z.txt")

	_, _, err := suite.attrCache.StreamDir(internal.StreamDirOptions{Name: "streamdir2"})
	suite.assert.NoError(err)

	item, _ := suite.attrCache.lru.Peek("streamdir2/z.txt")
	cachedAt := item.cachedAt

	_, err = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "streamdir2/z.txt"})
	suite.assert.NoError(err)

	item, _ = suite.attrCache.lru.Peek("streamdir2/z.txt")
	suite.assert.Equal(cachedAt, item.cachedAt, "GetAttr after StreamDir should be a cache hit")
}

// ---- File operation tests ---------------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestCreateFileInvalidatesEntry() {
	// Pre-populate a stale entry for a non-existent path.
	suite.attrCache.lru.Put("newfile.txt",
		&attrCacheItem{cachedAt: time.Now()})

	h, err := suite.attrCache.CreateFile(internal.CreateFileOptions{Name: "newfile.txt", Mode: 0644})
	suite.assert.NoError(err)
	h.GetFileObject().Close()

	_, ok := suite.attrCache.lru.Peek("newfile.txt")
	suite.assert.False(ok, "CreateFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteFileMarksEntryNegative() {
	suite.touchFile("del.txt")
	suite.cacheViaGetAttr("del.txt")

	err := suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "del.txt"})
	suite.assert.NoError(err)

	item, ok := suite.attrCache.lru.Peek("del.txt")
	suite.assert.True(ok)
	suite.assert.False(item.exists, "deleted file must be a negative entry")

	// GetAttr must return ENOENT from cache.
	_, err = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "del.txt"})
	suite.assert.Equal(syscall.ENOENT, err)
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteFileGetAttrAfterTTLReturnsENOENT() {
	// Negative entry remains negative even after TTL if file is still gone.
	suite.touchFile("gone.txt")
	suite.cacheViaGetAttr("gone.txt")

	err := suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "gone.txt"})
	suite.assert.NoError(err)

	suite.forceExpire("gone.txt")

	_, err = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "gone.txt"})
	suite.assert.True(os.IsNotExist(err))
}

func (suite *attrCacheIntegrationTestSuite) TestRenameFileMarksSourceNegativeUpdatesDest() {
	suite.touchFile("src.txt")
	srcItem := suite.cacheViaGetAttr("src.txt")
	srcAttr := srcItem.attr

	err := suite.attrCache.RenameFile(internal.RenameFileOptions{
		Src:     "src.txt",
		Dst:     "dst.txt",
		SrcAttr: srcAttr,
	})
	suite.assert.NoError(err)

	srcEntry, ok := suite.attrCache.lru.Peek("src.txt")
	suite.assert.True(ok)
	suite.assert.False(srcEntry.exists, "renamed-away file must be a negative entry")

	// Destination must be accessible.
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "dst.txt"})
	suite.assert.NoError(err)
	suite.assert.Equal("dst.txt", attr.Path)
}

func (suite *attrCacheIntegrationTestSuite) TestRenameFilePreCachedDestGetsUpdatedAttrs() {
	suite.touchFile("rsrc.txt")
	suite.touchFile("rdst.txt")

	srcItem := suite.cacheViaGetAttr("rsrc.txt")
	srcAttr := srcItem.attr

	// Pre-cache the destination.
	suite.cacheViaGetAttr("rdst.txt")

	err := suite.attrCache.RenameFile(internal.RenameFileOptions{
		Src:     "rsrc.txt",
		Dst:     "rdst.txt",
		SrcAttr: srcAttr,
	})
	suite.assert.NoError(err)

	dstItem, ok := suite.attrCache.lru.Peek("rdst.txt")
	suite.assert.True(ok)
	suite.assert.True(dstItem.exists)
}

func (suite *attrCacheIntegrationTestSuite) TestWriteFileInvalidatesEntry() {
	h := suite.createFileViaCache("write.txt")
	defer h.GetFileObject().Close()

	suite.cacheViaGetAttr("write.txt")

	_, err := suite.attrCache.WriteFile(&internal.WriteFileOptions{
		Handle: h,
		Offset: 0,
		Data:   []byte("hello world"),
	})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("write.txt")
	suite.assert.False(ok, "WriteFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestTruncateFileInvalidatesEntry() {
	suite.touchFile("trunc.txt")
	suite.cacheViaGetAttr("trunc.txt")

	err := suite.attrCache.TruncateFile(internal.TruncateFileOptions{
		Name:    "trunc.txt",
		NewSize: 0,
	})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("trunc.txt")
	suite.assert.False(ok, "TruncateFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestFlushFileInvalidatesEntry() {
	h := suite.createFileViaCache("flush.txt")
	defer h.GetFileObject().Close()

	suite.cacheViaGetAttr("flush.txt")

	err := suite.attrCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("flush.txt")
	suite.assert.False(ok, "FlushFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestSyncFileInvalidatesEntry() {
	suite.touchFile("sync.txt")
	suite.cacheViaGetAttr("sync.txt")

	h := handlemap.NewHandle("sync.txt")
	err := suite.attrCache.SyncFile(internal.SyncFileOptions{Handle: h})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("sync.txt")
	suite.assert.False(ok, "SyncFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestChmodUpdatesInPlace() {
	suite.touchFile("chmod.txt")
	suite.cacheViaGetAttr("chmod.txt")

	newMode := os.FileMode(0600)
	err := suite.attrCache.Chmod(internal.ChmodOptions{Name: "chmod.txt", Mode: newMode})
	suite.assert.NoError(err)

	// Chmod must update the mode in-place, not invalidate.
	item, ok := suite.attrCache.lru.Peek("chmod.txt")
	suite.assert.True(ok)
	suite.assert.True(item.exists)
	suite.assert.Equal(newMode, item.attr.Mode)
}

func (suite *attrCacheIntegrationTestSuite) TestChmodOnNonCachedEntryNoOp() {
	suite.touchFile("chmod_nc.txt")
	// Don't cache it first.

	err := suite.attrCache.Chmod(internal.ChmodOptions{Name: "chmod_nc.txt", Mode: 0600})
	suite.assert.NoError(err)

	// Entry should not be in cache.
	_, ok := suite.attrCache.lru.Peek("chmod_nc.txt")
	suite.assert.False(ok)
}

func (suite *attrCacheIntegrationTestSuite) TestCopyFromFileInvalidatesEntry() {
	suite.touchFile("copydst.txt")
	suite.cacheViaGetAttr("copydst.txt")

	tmpSrc, err := os.CreateTemp("", "copy_src_*")
	suite.Require().NoError(err)
	defer func() {
		tmpSrc.Close()
		os.Remove(tmpSrc.Name())
	}()
	_, _ = tmpSrc.WriteString("content")
	_, _ = tmpSrc.Seek(0, 0)

	err = suite.attrCache.CopyFromFile(internal.CopyFromFileOptions{
		Name: "copydst.txt",
		File: tmpSrc,
	})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("copydst.txt")
	suite.assert.False(ok, "CopyFromFile must remove the cache entry")
}

func (suite *attrCacheIntegrationTestSuite) TestCreateLinkInvalidatesBothEnds() {
	suite.touchFile("linktarget.txt")
	suite.cacheViaGetAttr("linktarget.txt")

	// Pre-populate the link name in cache (as negative so it is present).
	suite.attrCache.lru.cacheNegativeEntry("linkname")

	err := suite.attrCache.CreateLink(internal.CreateLinkOptions{
		Name:   "linkname",
		Target: "linktarget.txt",
	})
	suite.assert.NoError(err)

	for _, p := range []string{"linkname", "linktarget.txt"} {
		_, ok := suite.attrCache.lru.Peek(p)
		suite.assert.False(ok, "CreateLink must remove %s from cache", p)
	}
}

func (suite *attrCacheIntegrationTestSuite) TestCommitDataInvalidatesEntry() {
	suite.touchFile("commit.txt")
	suite.cacheViaGetAttr("commit.txt")

	err := suite.attrCache.CommitData(internal.CommitDataOptions{
		Name: "commit.txt",
		List: []string{},
	})
	suite.assert.NoError(err)

	_, ok := suite.attrCache.lru.Peek("commit.txt")
	suite.assert.False(ok, "CommitData must remove the cache entry")
}

// ---- LRU eviction tests -----------------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestLRUEvictionForcesRefetch() {
	// Create three files to use as cache entries.
	for _, name := range []string{"lru_a.txt", "lru_b.txt", "lru_c.txt"} {
		suite.touchFile(name)
	}

	// Measure the cost of a single entry.
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "lru_a.txt"})
	singleSize := suite.attrCache.lru.Size()
	suite.assert.Positive(singleSize)

	// Allow exactly two entries.
	suite.attrCache.lru.SetMaxSize(singleSize * 2)

	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "lru_b.txt"})
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "lru_c.txt"})

	// LRU must have evicted the oldest entry (lru_a).
	suite.assert.LessOrEqual(suite.attrCache.lru.Size(), singleSize*2)
	suite.assert.LessOrEqual(suite.attrCache.lru.Len(), 2)

	// GetAttr on an evicted entry must still succeed (re-fetched from loopback).
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "lru_a.txt"})
	suite.assert.NoError(err)
	suite.assert.Equal("lru_a.txt", attr.Path)
}

func (suite *attrCacheIntegrationTestSuite) TestLRUEvictionPreservesRecentlyAccessed() {
	for _, name := range []string{"ev_a.txt", "ev_b.txt", "ev_c.txt"} {
		suite.touchFile(name)
	}

	// Populate A and B.
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ev_a.txt"})
	singleSize := suite.attrCache.lru.Size()
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ev_b.txt"})

	// Promote A to MRU by re-accessing it.
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ev_a.txt"})

	// Restrict to two entries.  Adding C should evict B (LRU).
	suite.attrCache.lru.SetMaxSize(singleSize * 2)
	_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "ev_c.txt"})

	suite.assert.True(suite.attrCache.lru.Has("ev_a.txt"), "recently accessed A should be retained")
	suite.assert.True(suite.attrCache.lru.Has("ev_c.txt"), "newest C should be retained")
	suite.assert.False(suite.attrCache.lru.Has("ev_b.txt"), "LRU entry B should have been evicted")
}

// ---- Concurrency tests ------------------------------------------------------

func (suite *attrCacheIntegrationTestSuite) TestConcurrentGetAttrNoRace() {
	const n = 10
	for i := 0; i < n; i++ {
		suite.touchFile(fmt.Sprintf("conc_%d.txt", i))
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("conc_%d.txt", idx)
			for j := 0; j < 20; j++ {
				_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: name})
			}
		}(i)
	}
	wg.Wait()

	// Verify all entries are cached correctly after concurrent access.
	for i := 0; i < n; i++ {
		_, ok := suite.attrCache.lru.Peek(fmt.Sprintf("conc_%d.txt", i))
		suite.assert.True(ok)
	}
}

func (suite *attrCacheIntegrationTestSuite) TestConcurrentMutationsAndReads() {
	const n = 5
	for i := 0; i < n; i++ {
		suite.touchFile(fmt.Sprintf("mut_%d.txt", i))
	}

	var wg sync.WaitGroup

	// Readers: repeatedly call GetAttr.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{
					Name: fmt.Sprintf("mut_%d.txt", idx),
				})
			}
		}(i)
	}

	// Writers: invalidate entries via Chmod.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = suite.attrCache.Chmod(internal.ChmodOptions{
					Name: fmt.Sprintf("mut_%d.txt", idx),
					Mode: os.FileMode(0644),
				})
			}
		}(i)
	}

	wg.Wait()
}

func (suite *attrCacheIntegrationTestSuite) TestConcurrentGetAttrAndNilAttrRace() {
	// This test specifically exercises the crash that occurred before items became immutable:
	//
	//   Thread A: lru.Get("f") → returns *item with valid=true, exists=true, attr=<ptr>
	//   Thread B: invalidatePath("f") → used to mutate item.attr=nil in-place
	//   Thread A: item.getAttr() → returned nil → caller dereferences → crash
	//
	// With immutable items, Thread B's Put replaces the LRU entry with a NEW item; Thread A
	// still holds the OLD pointer whose attr field is never mutated, so no nil dereference.
	const n = 8
	for i := 0; i < n; i++ {
		suite.touchFile(fmt.Sprintf("race_%d.txt", i))
	}

	// Pre-populate cache so readers find valid entries immediately.
	for i := 0; i < n; i++ {
		_, _ = suite.attrCache.GetAttr(internal.GetAttrOptions{Name: fmt.Sprintf("race_%d.txt", i)})
	}

	var wg sync.WaitGroup

	// Readers: call GetAttr and assert the returned attr is never nil.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("race_%d.txt", idx)
			for j := 0; j < 50; j++ {
				attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: name})
				if err == nil {
					// A nil attr on a successful GetAttr is the crash scenario.
					suite.assert.NotNil(attr, "GetAttr must never return nil attr on success")
				}
			}
		}(i)
	}

	// Invalidators: repeatedly invalidate the same entries via DeleteFile + re-create.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("race_%d.txt", idx)
			for j := 0; j < 20; j++ {
				_ = suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: name})
				full := filepath.Join(suite.loopbackDir, name)
				_ = os.MkdirAll(filepath.Dir(full), 0755)
				if f, err := os.Create(full); err == nil {
					f.Close()
				}
				h, err := suite.attrCache.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0644})
				if err == nil {
					h.GetFileObject().Close()
				}
			}
		}(i)
	}

	// Directory invalidators: exercise the two-phase directory invalidation path.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			_ = suite.attrCache.SyncDir(internal.SyncDirOptions{Name: ""})
		}
	}()

	wg.Wait()
}

func (suite *attrCacheIntegrationTestSuite) TestGetAttrOnRootDirectory() {
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: ""})
	// Root directory should resolve to the loopback path — either a valid attr or ENOENT.
	if err == nil {
		suite.assert.NotNil(attr)
	} else {
		suite.assert.True(os.IsNotExist(err))
	}
}

func (suite *attrCacheIntegrationTestSuite) TestCacheEntriesDoNotCrossContainerBoundary() {
	// "dir" and "dir2" must have independent entries even though one is a prefix of the other.
	suite.mkDir("dir")
	suite.mkDir("dir2")
	suite.touchFile("dir/f.txt")
	suite.touchFile("dir2/f.txt")

	suite.cacheViaGetAttr("dir")
	suite.cacheViaGetAttr("dir2")
	suite.cacheViaGetAttr("dir/f.txt")
	suite.cacheViaGetAttr("dir2/f.txt")

	// Remove the child file and child dir from disk so the parent dir is empty
	// for DeleteDir (loopback uses os.Remove which requires an empty directory).
	os.Remove(filepath.Join(suite.loopbackDir, "dir/f.txt"))

	err := suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "dir"})
	suite.assert.NoError(err)

	dirItem, _ := suite.attrCache.lru.Peek("dir")
	suite.assert.False(dirItem.exists, `"dir" must be negative`)

	dir2Item, _ := suite.attrCache.lru.Peek("dir2")
	suite.assert.True(dir2Item.exists, `"dir2" must remain positive`)

	dirFItem, _ := suite.attrCache.lru.Peek("dir/f.txt")
	suite.assert.False(dirFItem.exists, `"dir/f.txt" must be negative`)

	dir2FItem, _ := suite.attrCache.lru.Peek("dir2/f.txt")
	suite.assert.True(dir2FItem.exists, `"dir2/f.txt" must remain positive`)
}

func (suite *attrCacheIntegrationTestSuite) TestStaleEntryReplacedOnRefetch() {
	suite.touchFile("stale.txt")
	suite.cacheViaGetAttr("stale.txt")

	// Modify the file directly in the backing store.
	err := os.WriteFile(filepath.Join(suite.loopbackDir, "stale.txt"), []byte("new content"), 0644)
	suite.assert.NoError(err)

	// Entry is still valid (not expired), so GetAttr returns cached attrs.
	suite.forceExpire("stale.txt")

	// After expiry, the new attributes should be fetched.
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "stale.txt"})
	suite.assert.NoError(err)
	suite.assert.Positive(attr.Size, "refreshed entry must reflect new file size")
}

func (suite *attrCacheIntegrationTestSuite) TestDeleteThenRecreateFile() {
	suite.touchFile("lifecycle.txt")
	suite.cacheViaGetAttr("lifecycle.txt")

	_ = suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "lifecycle.txt"})

	// Verify negative entry.
	item, _ := suite.attrCache.lru.Peek("lifecycle.txt")
	suite.assert.False(item.exists)

	// Expire the negative entry.
	suite.forceExpire("lifecycle.txt")

	// Recreate via attr_cache.
	h, err := suite.attrCache.CreateFile(internal.CreateFileOptions{Name: "lifecycle.txt", Mode: 0644})
	suite.assert.NoError(err)
	h.GetFileObject().Close()

	// GetAttr must now succeed.
	attr, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: "lifecycle.txt"})
	suite.assert.NoError(err)
	suite.assert.Equal("lifecycle.txt", attr.Path)
}

// ---- Test runner ------------------------------------------------------------

func TestAttrCacheIntegration(t *testing.T) {
	suite.Run(t, new(attrCacheIntegrationTestSuite))
}
