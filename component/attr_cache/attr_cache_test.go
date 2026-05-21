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
	"container/list"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type attrCacheTestSuite struct {
	suite.Suite
	assert    *assert.Assertions
	attrCache *AttrCache
	mockCtrl  *gomock.Controller
	mock      *internal.MockComponent
}

var emptyConfig = ""
var defaultSize = int64(0)
var defaultMode = 0777

func newTestAttrCache(next internal.Component, configuration string) *AttrCache {
	_ = config.ReadConfigFromReader(strings.NewReader(configuration))
	attrCache := NewAttrCacheComponent()
	attrCache.SetNextComponent(next)
	_ = attrCache.Configure(true)

	return attrCache.(*AttrCache)
}

func getPathAttr(path string, size int64, mode os.FileMode, metadata bool) *internal.ObjAttr {
	flags := internal.NewFileBitMap()
	return &internal.ObjAttr{
		Path:     path,
		Name:     filepath.Base(path),
		Size:     size,
		Mode:     mode,
		Mtime:    time.Now(),
		Atime:    time.Now(),
		Ctime:    time.Now(),
		Crtime:   time.Now(),
		Flags:    flags,
		Metadata: nil,
	}
}

// getCacheItem returns the cached item for path without altering LRU order.
func getCacheItem(ac *AttrCache, path string) *attrCacheItem {
	item, ok := ac.lru.Peek(path)
	if !ok {
		return nil
	}
	return item
}

func addPathToCache(assert *assert.Assertions, attrCache *AttrCache, path string, metadata bool) {
	path = internal.TruncateDirName(path)
	item := newAttrCacheItem(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), metadata), true, time.Now())
	attrCache.lru.Put(path, item)
	assert.True(attrCache.lru.Has(path))
}

func assertDeleted(suite *attrCacheTestSuite, path string) {
	item := getCacheItem(suite.attrCache, path)
	suite.assert.NotNil(item)
	suite.assert.Nil(item.attr)
	suite.assert.True(item.valid)
	suite.assert.False(item.exists)
}

func assertInvalid(suite *attrCacheTestSuite, path string) {
	item := getCacheItem(suite.attrCache, path)
	suite.assert.NotNil(item)
	suite.assert.Nil(item.attr)
	suite.assert.False(item.valid)
}

func assertUntouched(suite *attrCacheTestSuite, path string) {
	item := getCacheItem(suite.attrCache, path)
	suite.assert.NotNil(item)
	suite.assert.NotEqual(&internal.ObjAttr{}, item.attr)
	suite.assert.Equal(item.attr.Size, defaultSize)
	suite.assert.EqualValues(item.attr.Mode, defaultMode)
	suite.assert.True(item.valid)
	suite.assert.True(item.exists)
}

// assertAttributesTransferred checks that dst has the same attrs as src and is valid/exists.
func assertAttributesTransferred(suite *attrCacheTestSuite, srcAttr *internal.ObjAttr, dstAttr *internal.ObjAttr) {
	suite.assert.Equal(srcAttr.Size, dstAttr.Size)
	suite.assert.Equal(srcAttr.Path, dstAttr.Path)
	suite.assert.Equal(srcAttr.Mode, dstAttr.Mode)
	suite.assert.Equal(srcAttr.Atime, dstAttr.Atime)
	suite.assert.Equal(srcAttr.Mtime, dstAttr.Mtime)
	suite.assert.Equal(srcAttr.Ctime, dstAttr.Ctime)
	dstItem := getCacheItem(suite.attrCache, dstAttr.Path)
	suite.assert.NotNil(dstItem)
	suite.assert.True(dstItem.exists)
	suite.assert.True(dstItem.valid)
}

// If next component changes the times of the attribute.
func assertSrcAttributeTimeChanged(suite *attrCacheTestSuite, srcAttr *internal.ObjAttr, srcAttrCopy internal.ObjAttr) {
	suite.assert.NotEqualValues(suite, srcAttr.Atime, srcAttrCopy.Atime)
	suite.assert.NotEqualValues(suite, srcAttr.Mtime, srcAttrCopy.Mtime)
	suite.assert.NotEqualValues(suite, srcAttr.Ctime, srcAttrCopy.Ctime)
}

// Directory structure
// a/
//
//	 a/c1/
//	  a/c1/gc1
//		a/c2
//
// ab/
//
//	ab/c1
//
// ac
func generateNestedDirectory(path string) (*list.List, *list.List, *list.List) {
	aPaths := list.New()
	aPaths.PushBack(internal.TruncateDirName(path))

	aPaths.PushBack(filepath.Join(path, "c1"))
	aPaths.PushBack(filepath.Join(path, "c2"))
	aPaths.PushBack(filepath.Join(filepath.Join(path, "c1"), "gc1"))

	abPaths := list.New()
	path = internal.TruncateDirName(path)
	abPaths.PushBack(path + "b")
	abPaths.PushBack(filepath.Join(path+"b", "c1"))

	acPaths := list.New()
	acPaths.PushBack(path + "c")

	return aPaths, abPaths, acPaths
}

func generateNestedPathAttr(path string, size int64, mode os.FileMode) []*internal.ObjAttr {
	a, _, _ := generateNestedDirectory(path)
	pathAttrs := make([]*internal.ObjAttr, 0)
	i := 0
	for p := a.Front(); p != nil; p = p.Next() {
		pathAttrs = append(pathAttrs, getPathAttr(p.Value.(string), size, mode, false))
		i++
	}
	return pathAttrs
}

func addDirectoryToCache(assert *assert.Assertions, attrCache *AttrCache, path string, metadata bool) (*list.List, *list.List, *list.List) {
	aPaths, abPaths, acPaths := generateNestedDirectory(path)

	for p := aPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}
	for p := abPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}
	for p := acPaths.Front(); p != nil; p = p.Next() {
		addPathToCache(assert, attrCache, p.Value.(string), metadata)
	}

	return aPaths, abPaths, acPaths
}

func (suite *attrCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	suite.setupTestHelper(emptyConfig)
}

func (suite *attrCacheTestSuite) setupTestHelper(config string) {
	suite.assert = assert.New(suite.T())

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.attrCache = newTestAttrCache(suite.mock, config)
	_ = suite.attrCache.Start(context.Background())
}

func (suite *attrCacheTestSuite) cleanupTest() {
	_ = suite.attrCache.Stop()
	suite.mockCtrl.Finish()
}

// Tests the default configuration of attribute cache
func (suite *attrCacheTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.Equal(120*time.Second, suite.attrCache.cacheTimeout)
	// suite.assert.Equal(suite.attrCache.noSymlinks, false)
}

// Tests configuration
func (suite *attrCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 60\n  no-cache-on-list: true\n  no-symlinks: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.Equal(60*time.Second, suite.attrCache.cacheTimeout)
	suite.assert.True(suite.attrCache.noSymlinks)
}

// Tests max-size-mb config
func (suite *attrCacheTestSuite) TestConfigMaxSizeMB() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	maxSizeMB := 256
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d\n  max-size-mb: %d", cacheTimeout, maxSizeMB)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.Equal(suite.attrCache.maxSizeBytes, int64(maxSizeMB)*1024*1024)
}

func (suite *attrCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 0\n  no-cache-on-list: true\n  no-symlinks: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("attr_cache", suite.attrCache.Name())
	suite.assert.EqualValues(0, suite.attrCache.cacheTimeout)
	suite.assert.True(suite.attrCache.noSymlinks)
}

// Tests Create Directory
func (suite *attrCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		log.Debug("%s", path)
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.CreateDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().CreateDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.CreateDir(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.NoError(err)
			assertInvalid(suite, truncatedPath)
		})
	}
}

// Tests Delete Directory
func (suite *attrCacheTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.DeleteDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().DeleteDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.DeleteDir(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
			}
		})
	}
}

// Tests Read Directory
func (suite *attrCacheTestSuite) TestReadDirDoesNotExist() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}
	size := int64(1024)
	mode := os.FileMode(0)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			aAttr := generateNestedPathAttr(path, size, mode)

			options := internal.ReadDirOptions{Name: path}

			// Success
			// Entries Do Not Already Exist
			suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)

			suite.assert.Zero(suite.attrCache.lru.Len()) // cache should be empty before call
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.NoError(err)
			suite.assert.Equal(aAttr, returnedAttr)
			suite.assert.Len(aAttr, suite.attrCache.lru.Len())

			// Entries should now be in the cache
			for _, p := range aAttr {
				item := getCacheItem(suite.attrCache, p.Path)
				suite.assert.NotNil(item)
				suite.assert.NotEqual(&internal.ObjAttr{}, item.attr)
				suite.assert.Equal(item.attr.Size, size) // new size should be set
				suite.assert.Equal(item.attr.Mode, mode) // new mode should be set
				suite.assert.True(item.valid)
				suite.assert.True(item.exists)
			}
		})
	}
}

func (suite *attrCacheTestSuite) TestReadDirExists() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}
	size := int64(1024)
	mode := os.FileMode(0)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			aAttr := generateNestedPathAttr(path, size, mode)

			options := internal.ReadDirOptions{Name: path}

			// Success
			// Entries Already Exist
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.assert.NotZero(suite.attrCache.lru.Len()) // cache should NOT be empty before read dir call and values should be untouched
			for _, p := range aAttr {
				assertUntouched(suite, p.Path)
			}
			suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.NoError(err)
			suite.assert.Equal(aAttr, returnedAttr)

			// a paths should now be updated in the cache
			for p := a.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				item := getCacheItem(suite.attrCache, pString)
				suite.assert.NotNil(item)
				suite.assert.NotEqual(&internal.ObjAttr{}, item.attr)
				suite.assert.Equal(item.attr.Size, size) // new size should be set
				suite.assert.Equal(item.attr.Mode, mode) // new mode should be set
				suite.assert.True(item.valid)
				suite.assert.True(item.exists)
			}

			// ab and ac paths should be untouched
			ab.PushBackList(ac)
			for p := ab.Front(); p != nil; p = p.Next() {
				assertUntouched(suite, p.Value.(string))
			}
		})
	}
}

func (suite *attrCacheTestSuite) TestReadDirError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "ab", "ab/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ReadDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().ReadDir(options).Return(make([]*internal.ObjAttr, 0), errors.New("Failed to read a directory"))

			_, err := suite.attrCache.ReadDir(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))
		})
	}
}

// Tests Rename Directory
func (suite *attrCacheTestSuite) TestRenameDir() {
	defer suite.cleanupTest()
	var inputs = []struct {
		src string
		dst string
	}{
		{src: "a", dst: "ab"},
		{src: "a/", dst: "ab"},
		{src: "a", dst: "ab/"},
		{src: "a/", dst: "ab/"},
	}

	for _, input := range inputs {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(input.src+"->"+input.dst, func() {
			truncatedSrc := internal.TruncateDirName(input.src)
			truncatedDst := internal.TruncateDirName(input.dst)
			options := internal.RenameDirOptions{Src: input.src, Dst: input.dst}

			// Error
			suite.mock.EXPECT().RenameDir(options).Return(errors.New("Failed to rename a directory"))

			err := suite.attrCache.RenameDir(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedSrc))
			suite.assert.False(suite.attrCache.lru.Has(truncatedDst))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedSrc))
			suite.assert.False(suite.attrCache.lru.Has(truncatedDst))

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertDeleted(suite, truncatedPath)
			}
			// ab paths should be invalidated
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertInvalid(suite, truncatedPath)
			}
			// ac paths should be untouched
			for p := ac.Front(); p != nil; p = p.Next() {
				truncatedPath := internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
			}
		})
	}
}

// Tests Create File
func (suite *attrCacheTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CreateFileOptions{Name: path}

	// Error
	suite.mock.EXPECT().CreateFile(options).Return(nil, errors.New("Failed to create a file"))

	_, err := suite.attrCache.CreateFile(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.NoError(err)
	assertInvalid(suite, path)
}

// Tests Delete File
func (suite *attrCacheTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.DeleteFileOptions{Name: path}

	// Error
	suite.mock.EXPECT().DeleteFile(options).Return(errors.New("Failed to delete a file"))

	err := suite.attrCache.DeleteFile(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.NoError(err)
	assertDeleted(suite, path)
}

// Tests Sync File
func (suite *attrCacheTestSuite) TestSyncFile() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.SyncFileOptions{Handle: &handle}

	// Error
	suite.mock.EXPECT().SyncFile(options).Return(errors.New("Failed to sync a file"))

	err := suite.attrCache.SyncFile(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.NoError(err)
	assertInvalid(suite, path)
}

// Tests Sync Directory
func (suite *attrCacheTestSuite) TestSyncDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.SyncDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().SyncDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.SyncDir(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.NoError(err)
			// a paths should be deleted
			for p := a.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertInvalid(suite, truncatedPath)
			}
			ab.PushBackList(ac) // ab and ac paths should be untouched
			for p := ab.Front(); p != nil; p = p.Next() {
				truncatedPath = internal.TruncateDirName(p.Value.(string))
				assertUntouched(suite, truncatedPath)
			}
		})
	}
}

// Tests Rename File
func (suite *attrCacheTestSuite) TestRenameFile() {
	defer suite.cleanupTest()
	src := "a"
	dst := "b"

	options := internal.RenameFileOptions{Src: src, Dst: dst}

	// Error
	suite.mock.EXPECT().RenameFile(options).Return(errors.New("Failed to rename a file"))

	err := suite.attrCache.RenameFile(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(src))
	suite.assert.False(suite.attrCache.lru.Has(dst))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(src))
	suite.assert.False(suite.attrCache.lru.Has(dst))

	// Src, Dst Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, src, false)
	addPathToCache(suite.assert, suite.attrCache, dst, false)
	options.SrcAttr = getCacheItem(suite.attrCache, src).attr
	options.SrcAttr.Size = 1
	options.SrcAttr.Mode = 2
	options.DstAttr = getCacheItem(suite.attrCache, dst).attr
	options.DstAttr.Size = 3
	options.DstAttr.Mode = 4
	srcAttrCopy := *options.SrcAttr

	suite.mock.EXPECT().RenameFile(options).Return(nil)
	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	assertDeleted(suite, src)
	modifiedDstAttr := getCacheItem(suite.attrCache, dst).attr
	assertSrcAttributeTimeChanged(suite, options.SrcAttr, srcAttrCopy)
	// Check the attributes of the dst are same as the src.
	assertAttributesTransferred(suite, options.SrcAttr, modifiedDstAttr)

	// Src Entry Exist and Dst Entry Don't Exist
	addPathToCache(suite.assert, suite.attrCache, src, false)
	// Add negative entry to cache for Dst
	suite.attrCache.lru.Put(dst, newAttrCacheItem(&internal.ObjAttr{}, false, time.Now()))
	options.SrcAttr = getCacheItem(suite.attrCache, src).attr
	options.DstAttr = getCacheItem(suite.attrCache, dst).attr
	options.SrcAttr.Size = 1
	options.SrcAttr.Mode = 2
	suite.mock.EXPECT().RenameFile(options).Return(nil)
	err = suite.attrCache.RenameFile(options)
	suite.assert.NoError(err)
	assertDeleted(suite, src)
	modifiedDstAttr = getCacheItem(suite.attrCache, dst).attr
	assertSrcAttributeTimeChanged(suite, options.SrcAttr, srcAttrCopy)
	assertAttributesTransferred(suite, options.SrcAttr, modifiedDstAttr)
}

// Tests Write File
func (suite *attrCacheTestSuite) TestWriteFileError() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}

	// Error
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(nil, nil)
	suite.mock.EXPECT().WriteFile(&options).Return(0, errors.New("Failed to write a file"))

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.Error(err)
	suite.assert.True(suite.attrCache.lru.Has(path)) // GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestWriteFileDoesNotExist() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}
	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(nil, nil)
	suite.mock.EXPECT().WriteFile(&options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.NoError(err)
	suite.assert.True(suite.attrCache.lru.Has(path)) // GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestWriteFileExists() {
	defer suite.cleanupTest()
	path := "a"
	handle := handlemap.Handle{
		Path: path,
	}

	options := internal.WriteFileOptions{Handle: &handle, Metadata: nil}
	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, true)
	suite.mock.EXPECT().WriteFile(&options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(&options)
	suite.assert.NoError(err)
	assertInvalid(suite, path)
}

// Tests Truncate File
func (suite *attrCacheTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	path := "a"
	size := 1024

	options := internal.TruncateFileOptions{Name: path, NewSize: int64(size)}

	// Error
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("Failed to truncate a file"))

	err := suite.attrCache.TruncateFile(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.NoError(err)
	item := getCacheItem(suite.attrCache, path)
	suite.assert.NotNil(item)
	suite.assert.False(item.valid)
}

// Tests CopyFromFile
func (suite *attrCacheTestSuite) TestCopyFromFileError() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(nil, nil)
	// Error
	suite.mock.EXPECT().CopyFromFile(options).Return(errors.New("Failed to copy from file"))

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.Error(err)
	suite.assert.True(suite.attrCache.lru.Has(path)) // GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileDoesNotExist() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().GetAttr(internal.GetAttrOptions{Name: path, RetrieveMetadata: true}).Return(nil, nil)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.NoError(err)
	suite.assert.True(suite.attrCache.lru.Has(path)) // GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileExists() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, true)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.NoError(err)
	assertInvalid(suite, path)
}

// GetAttr
func (suite *attrCacheTestSuite) TestGetAttrExistsDeleted() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {

			addDirectoryToCache(suite.assert, suite.attrCache, "a", false)
			// delete directory a and file ac
			suite.mock.EXPECT().DeleteDir(gomock.Any()).Return(nil)
			suite.mock.EXPECT().DeleteFile(gomock.Any()).Return(nil)
			_ = suite.attrCache.DeleteDir(internal.DeleteDirOptions{Name: "a"})
			_ = suite.attrCache.DeleteFile(internal.DeleteFileOptions{Name: "ac"})

			options := internal.GetAttrOptions{Name: path}

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(syscall.ENOENT, err)
			suite.assert.Equal(&internal.ObjAttr{}, result)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithMetadata() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			addDirectoryToCache(suite.assert, suite.attrCache, "a", true) // add the paths to the cache with IsMetadataRetrived=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since attributes are accessible

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadataNoSymlinks() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	noSymlinks := true
	config := fmt.Sprintf("attr_cache:\n  no-symlinks: %t", noSymlinks)

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			addDirectoryToCache(suite.assert, suite.attrCache, "a", true) // add the paths to the cache with IsMetadataRetrived=true

			options := internal.GetAttrOptions{Name: path}
			// no call to mock component since metadata is not needed in noSymlinks mode

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrExistsWithoutMetadata() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			addDirectoryToCache(suite.assert, suite.attrCache, "a", false) // add the paths to the cache with IsMetadataRetrived=false

			options := internal.GetAttrOptions{Name: path}

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrDoesNotExist() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/", "a/c1", "a/c1/", "a/c2", "a/c1/gc1", "ab", "ab/", "ab/c1", "ac"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			// attributes should not be accessible so call the mock
			suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil)

			suite.assert.Zero(suite.attrCache.lru.Len()) // cache should be empty before call
			_, err := suite.attrCache.GetAttr(options)
			suite.assert.NoError(err)
			assertUntouched(suite, truncatedPath) // item added to cache after
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrOtherError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			suite.mock.EXPECT().GetAttr(options).Return(&internal.ObjAttr{}, os.ErrNotExist)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, os.ErrNotExist)
			suite.assert.Equal(&internal.ObjAttr{}, result)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))
		})
	}
}

func (suite *attrCacheTestSuite) TestGetAttrEnonetError() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)

			options := internal.GetAttrOptions{Name: path}
			suite.mock.EXPECT().GetAttr(options).Return(&internal.ObjAttr{}, syscall.ENOENT)

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(syscall.ENOENT, err)
			suite.assert.Equal(&internal.ObjAttr{}, result)
			item := getCacheItem(suite.attrCache, truncatedPath)
			suite.assert.NotNil(item)
			suite.assert.Nil(item.attr)
			suite.assert.True(item.valid)
			suite.assert.False(item.exists)
			suite.assert.NotNil(item.cachedAt)
		})
	}
}

// Tests Cache Timeout
func (suite *attrCacheTestSuite) TestCacheTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d", cacheTimeout)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.Equal(time.Duration(cacheTimeout)*time.Second, suite.attrCache.cacheTimeout)

	path := "a"
	options := internal.GetAttrOptions{Name: path}
	// attributes should not be accessible so call the mock
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)

	suite.assert.Zero(suite.attrCache.lru.Len()) // cache should be empty before call
	_, err := suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)
	assertUntouched(suite, path) // item added to cache after

	// Before cache timeout elapses, subsequent get attr should work without calling next component
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)

	// Wait for cache timeout
	time.Sleep(time.Second * time.Duration(cacheTimeout))

	// After cache timeout elapses, subsequent get attr should need to call next component
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.NoError(err)
}

// Tests CreateLink
func (suite *attrCacheTestSuite) TestCreateLink() {
	defer suite.cleanupTest()
	link := "a.lnk"
	path := "a"

	options := internal.CreateLinkOptions{Name: link, Target: path}

	// Error
	suite.mock.EXPECT().CreateLink(options).Return(errors.New("Failed to create a link to a file"))

	err := suite.attrCache.CreateLink(options)
	suite.assert.Error(err)
	suite.assert.False(suite.attrCache.lru.Has(link))
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.NoError(err)
	suite.assert.False(suite.attrCache.lru.Has(link))
	suite.assert.False(suite.attrCache.lru.Has(path))

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, link, false)
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.NoError(err)
	assertInvalid(suite, link)
	assertInvalid(suite, path)
}

// Tests Chmod
func (suite *attrCacheTestSuite) TestChmod() {
	defer suite.cleanupTest()
	mode := fs.FileMode(0)
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ChmodOptions{Name: path, Mode: mode}

			// Error
			suite.mock.EXPECT().Chmod(options).Return(errors.New("Failed to chmod"))

			err := suite.attrCache.Chmod(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.NoError(err)
			item := getCacheItem(suite.attrCache, truncatedPath)
			suite.assert.NotNil(item)
			suite.assert.NotEqual(&internal.ObjAttr{}, item.attr)
			suite.assert.Equal(item.attr.Size, defaultSize)
			suite.assert.Equal(item.attr.Mode, mode) // new mode should be set
			suite.assert.True(item.valid)
			suite.assert.True(item.exists)
		})
	}
}

// Tests Chown
func (suite *attrCacheTestSuite) TestChown() {
	defer suite.cleanupTest()
	// TODO: Implement when datalake chown is supported.
	owner := 0
	group := 0
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.ChownOptions{Name: path, Owner: owner, Group: group}

			// Error
			suite.mock.EXPECT().Chown(options).Return(errors.New("Failed to chown"))

			err := suite.attrCache.Chown(options)
			suite.assert.Error(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.NoError(err)
			suite.assert.False(suite.attrCache.lru.Has(truncatedPath))

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.NoError(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

// TestLRUEvictionOnMemoryLimit verifies that the LRU evicts the least-recently-used entry
// when the cache exceeds its configured memory limit.
func (suite *attrCacheTestSuite) TestLRUEvictionOnMemoryLimit() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	// setupTestHelper creates and starts the cache.
	suite.setupTestHelper(emptyConfig)

	// Insert one entry to measure its cost.
	path0 := "measure"
	item0 := newAttrCacheItem(getPathAttr(path0, defaultSize, fs.FileMode(defaultMode), false), true, time.Now())
	suite.attrCache.lru.Put(path0, item0)
	singleEntrySize := suite.attrCache.lru.Size()
	suite.attrCache.lru.Purge()
	// Set max to exactly 2 entries.
	suite.attrCache.lru.SetMaxSize(singleEntrySize * 2)

	// Add 3 entries (A, B, C in insertion order: A is LRU).
	pathA, pathB, pathC := "lru_a", "lru_b", "lru_c"
	for _, p := range []string{pathA, pathB, pathC} {
		item := newAttrCacheItem(getPathAttr(p, defaultSize, fs.FileMode(defaultMode), false), true, time.Now())
		suite.attrCache.lru.Put(p, item)
	}

	// Only 2 entries should remain, and A (the LRU) should have been evicted.
	suite.assert.Equal(2, suite.attrCache.lru.Len())
	suite.assert.False(suite.attrCache.lru.Has(pathA), "LRU entry should have been evicted")
	suite.assert.True(suite.attrCache.lru.Has(pathB))
	suite.assert.True(suite.attrCache.lru.Has(pathC))
	suite.assert.LessOrEqual(suite.attrCache.lru.Size(), singleEntrySize*2)
}

// TestLRUOrderPreservesRecentlyAccessed verifies that a recently-accessed entry
// survives eviction over an older, less-recently-used entry.
func (suite *attrCacheTestSuite) TestLRUOrderPreservesRecentlyAccessed() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	suite.setupTestHelper(emptyConfig)

	// Measure single entry size and set limit to exactly 2 entries.
	path0 := "measure"
	item0 := newAttrCacheItem(getPathAttr(path0, defaultSize, fs.FileMode(defaultMode), false), true, time.Now())
	suite.attrCache.lru.Put(path0, item0)
	singleEntrySize := suite.attrCache.lru.Size()
	suite.attrCache.lru.Purge()
	suite.attrCache.lru.SetMaxSize(singleEntrySize * 2)

	// Insert A, then B.  Order: B (MRU) → A (LRU).
	pathA, pathB := "ord_a", "ord_b"
	for _, p := range []string{pathA, pathB} {
		item := newAttrCacheItem(getPathAttr(p, defaultSize, fs.FileMode(defaultMode), false), true, time.Now())
		suite.attrCache.lru.Put(p, item)
	}

	// Access A via GetAttr — this promotes A to MRU.  Order: A (MRU) → B (LRU).
	// pathA is valid in cache; GetAttr serves it without calling the mock.
	_, err := suite.attrCache.GetAttr(internal.GetAttrOptions{Name: pathA})
	suite.assert.NoError(err)

	// Add C — this should evict B (now LRU), not A.
	pathC := "ord_c"
	itemC := newAttrCacheItem(getPathAttr(pathC, defaultSize, fs.FileMode(defaultMode), false), true, time.Now())
	suite.attrCache.lru.Put(pathC, itemC)

	suite.assert.Equal(2, suite.attrCache.lru.Len())
	suite.assert.True(suite.attrCache.lru.Has(pathA), "A was recently accessed and should survive")
	suite.assert.False(suite.attrCache.lru.Has(pathB), "B should be evicted (LRU)")
	suite.assert.True(suite.attrCache.lru.Has(pathC))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAttrCacheTestSuite(t *testing.T) {
	suite.Run(t, new(attrCacheTestSuite))
}
