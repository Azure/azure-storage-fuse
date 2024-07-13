/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	if metadata {
		flags.Set(internal.PropFlagMetadataRetrieved)
	}
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

func addPathToCache(assert *assert.Assertions, attrCache *AttrCache, path string, metadata bool) {
	path = internal.TruncateDirName(path)
	attrCache.cacheMap[path] = newAttrCacheItem(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), metadata), true, time.Now())
	assert.Contains(attrCache.cacheMap, path)
}

func assertDeleted(suite *attrCacheTestSuite, path string) {
	suite.assert.Contains(suite.attrCache.cacheMap, path)
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr, &internal.ObjAttr{})
	suite.assert.True(suite.attrCache.cacheMap[path].valid())
	suite.assert.False(suite.attrCache.cacheMap[path].exists())
}

func assertInvalid(suite *attrCacheTestSuite, path string) {
	suite.assert.Contains(suite.attrCache.cacheMap, path)
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr, &internal.ObjAttr{})
	suite.assert.False(suite.attrCache.cacheMap[path].valid())
}

func assertUntouched(suite *attrCacheTestSuite, path string) {
	suite.assert.Contains(suite.attrCache.cacheMap, path)
	suite.assert.NotEqualValues(suite.attrCache.cacheMap[path].attr, &internal.ObjAttr{})
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr.Size, defaultSize)
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr.Mode, defaultMode)
	suite.assert.True(suite.attrCache.cacheMap[path].valid())
	suite.assert.True(suite.attrCache.cacheMap[path].exists())
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
	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, 120)
	suite.assert.Equal(suite.attrCache.noSymlinks, false)
}

// Tests configuration
func (suite *attrCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 60\n  no-cache-on-list: true\n  no-symlinks: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, 60)
	suite.assert.Equal(suite.attrCache.noSymlinks, true)
}

// Tests max files config
func (suite *attrCacheTestSuite) TestConfigMaxFiles() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	cacheTimeout := 1
	maxFiles := 10
	config := fmt.Sprintf("attr_cache:\n  timeout-sec: %d\n  max-files: %d", cacheTimeout, maxFiles)
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)
	suite.assert.EqualValues(suite.attrCache.maxFiles, maxFiles)
}

func (suite *attrCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default attr cache generated
	config := "attr_cache:\n  timeout-sec: 0\n  no-cache-on-list: true\n  no-symlinks: true"
	suite.setupTestHelper(config) // setup a new attr cache with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal(suite.attrCache.Name(), "attr_cache")
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, 0)
	suite.assert.Equal(suite.attrCache.noSymlinks, true)
}

// Tests Create Directory
func (suite *attrCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	var paths = []string{"a", "a/"}

	for _, path := range paths {
		log.Debug(path)
		// This is a little janky but required since testify suite does not support running setup or clean up for subtests.
		suite.cleanupTest()
		suite.SetupTest()
		suite.Run(path, func() {
			truncatedPath := internal.TruncateDirName(path)
			options := internal.CreateDirOptions{Name: path}

			// Error
			suite.mock.EXPECT().CreateDir(options).Return(errors.New("Failed"))

			err := suite.attrCache.CreateDir(options)
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().CreateDir(options).Return(nil)

			err = suite.attrCache.CreateDir(options)
			suite.assert.Nil(err)
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().DeleteDir(options).Return(nil)

			err = suite.attrCache.DeleteDir(options)
			suite.assert.Nil(err)
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

			suite.assert.Empty(suite.attrCache.cacheMap) // cacheMap should be empty before call
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.Nil(err)
			suite.assert.Equal(aAttr, returnedAttr)
			suite.assert.Equal(len(suite.attrCache.cacheMap), len(aAttr))

			// Entries should now be in the cache
			for _, p := range aAttr {
				suite.assert.Contains(suite.attrCache.cacheMap, p.Path)
				suite.assert.NotEqualValues(suite.attrCache.cacheMap[p.Path].attr, &internal.ObjAttr{})
				suite.assert.EqualValues(suite.attrCache.cacheMap[p.Path].attr.Size, size) // new size should be set
				suite.assert.EqualValues(suite.attrCache.cacheMap[p.Path].attr.Mode, mode) // new mode should be set
				suite.assert.True(suite.attrCache.cacheMap[p.Path].valid())
				suite.assert.True(suite.attrCache.cacheMap[p.Path].exists())
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

			suite.assert.NotEmpty(suite.attrCache.cacheMap) // cacheMap should NOT be empty before read dir call and values should be untouched
			for _, p := range aAttr {
				assertUntouched(suite, p.Path)
			}
			suite.mock.EXPECT().ReadDir(options).Return(aAttr, nil)
			returnedAttr, err := suite.attrCache.ReadDir(options)
			suite.assert.Nil(err)
			suite.assert.Equal(aAttr, returnedAttr)

			// a paths should now be updated in the cache
			for p := a.Front(); p != nil; p = p.Next() {
				pString := p.Value.(string)
				suite.assert.Contains(suite.attrCache.cacheMap, pString)
				suite.assert.NotEqualValues(suite.attrCache.cacheMap[pString].attr, &internal.ObjAttr{})
				suite.assert.EqualValues(suite.attrCache.cacheMap[pString].attr.Size, size) // new size should be set
				suite.assert.EqualValues(suite.attrCache.cacheMap[pString].attr.Mode, mode) // new mode should be set
				suite.assert.True(suite.attrCache.cacheMap[pString].valid())
				suite.assert.True(suite.attrCache.cacheMap[pString].exists())
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedSrc)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedDst)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedSrc)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedDst)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, input.src, false)

			suite.mock.EXPECT().RenameDir(options).Return(nil)

			err = suite.attrCache.RenameDir(options)
			suite.assert.Nil(err)
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
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateFile(options).Return(&handlemap.Handle{}, nil)

	_, err = suite.attrCache.CreateFile(options)
	suite.assert.Nil(err)
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
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().DeleteFile(options).Return(nil)

	err = suite.attrCache.DeleteFile(options)
	suite.assert.Nil(err)
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
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().SyncFile(options).Return(nil)

	err = suite.attrCache.SyncFile(options)
	suite.assert.Nil(err)
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Entry Already Exists
			a, ab, ac := addDirectoryToCache(suite.assert, suite.attrCache, path, false)

			suite.mock.EXPECT().SyncDir(options).Return(nil)

			err = suite.attrCache.SyncDir(options)
			suite.assert.Nil(err)
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
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, src)
	suite.assert.NotContains(suite.attrCache.cacheMap, dst)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, src)
	suite.assert.NotContains(suite.attrCache.cacheMap, dst)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, src, false)
	addPathToCache(suite.assert, suite.attrCache, dst, false)
	suite.mock.EXPECT().RenameFile(options).Return(nil)

	err = suite.attrCache.RenameFile(options)
	suite.assert.Nil(err)
	assertDeleted(suite, src)
	assertInvalid(suite, dst)
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
	suite.mock.EXPECT().WriteFile(options).Return(0, errors.New("Failed to write a file"))

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.NotNil(err)
	suite.assert.Contains(suite.attrCache.cacheMap, path) // GetAttr call will add this to the cache
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
	suite.mock.EXPECT().WriteFile(options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.Nil(err)
	suite.assert.Contains(suite.attrCache.cacheMap, path) // GetAttr call will add this to the cache
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
	suite.mock.EXPECT().WriteFile(options).Return(0, nil)

	_, err := suite.attrCache.WriteFile(options)
	suite.assert.Nil(err)
	assertInvalid(suite, path)
}

// Tests Truncate File
func (suite *attrCacheTestSuite) TestTruncateFile() {
	defer suite.cleanupTest()
	path := "a"
	size := 1024

	options := internal.TruncateFileOptions{Name: path, Size: int64(size)}

	// Error
	suite.mock.EXPECT().TruncateFile(options).Return(errors.New("Failed to truncate a file"))

	err := suite.attrCache.TruncateFile(options)
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().TruncateFile(options).Return(nil)

	err = suite.attrCache.TruncateFile(options)
	suite.assert.Nil(err)
	suite.assert.Contains(suite.attrCache.cacheMap, path)
	suite.assert.NotEqualValues(suite.attrCache.cacheMap[path].attr, &internal.ObjAttr{})
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr.Size, size) // new size should be set
	suite.assert.EqualValues(suite.attrCache.cacheMap[path].attr.Mode, defaultMode)
	suite.assert.True(suite.attrCache.cacheMap[path].valid())
	suite.assert.True(suite.attrCache.cacheMap[path].exists())
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
	suite.assert.NotNil(err)
	suite.assert.Contains(suite.attrCache.cacheMap, path) // GetAttr call will add this to the cache
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
	suite.assert.Nil(err)
	suite.assert.Contains(suite.attrCache.cacheMap, path) // GetAttr call will add this to the cache
}

func (suite *attrCacheTestSuite) TestCopyFromFileExists() {
	defer suite.cleanupTest()
	path := "a"

	options := internal.CopyFromFileOptions{Name: path, File: nil, Metadata: nil}
	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, path, true)
	suite.mock.EXPECT().CopyFromFile(options).Return(nil)

	err := suite.attrCache.CopyFromFile(options)
	suite.assert.Nil(err)
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
			// no call to mock component since attributes are accessible

			result, err := suite.attrCache.GetAttr(options)
			suite.assert.Equal(err, syscall.ENOENT)
			suite.assert.EqualValues(result, &internal.ObjAttr{})
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
			suite.assert.Nil(err)
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
			suite.assert.Nil(err)
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
			// attributes should not be accessible so call the mock
			suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), false), nil)

			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
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

			suite.assert.Empty(suite.attrCache.cacheMap) // cacheMap should be empty before call
			_, err := suite.attrCache.GetAttr(options)
			suite.assert.Nil(err)
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
			suite.assert.EqualValues(result, &internal.ObjAttr{})
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)
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
			suite.assert.Equal(err, syscall.ENOENT)
			suite.assert.EqualValues(result, &internal.ObjAttr{})
			suite.assert.Contains(suite.attrCache.cacheMap, truncatedPath)
			suite.assert.EqualValues(suite.attrCache.cacheMap[truncatedPath].attr, &internal.ObjAttr{})
			suite.assert.True(suite.attrCache.cacheMap[truncatedPath].valid())
			suite.assert.False(suite.attrCache.cacheMap[truncatedPath].exists())
			suite.assert.NotNil(suite.attrCache.cacheMap[truncatedPath].cachedAt)
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
	suite.assert.EqualValues(suite.attrCache.cacheTimeout, cacheTimeout)

	path := "a"
	options := internal.GetAttrOptions{Name: path}
	// attributes should not be accessible so call the mock
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)

	suite.assert.Empty(suite.attrCache.cacheMap) // cacheMap should be empty before call
	_, err := suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)
	assertUntouched(suite, path) // item added to cache after

	// Before cache timeout elapses, subsequent get attr should work without calling next component
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)

	// Wait for cache timeout
	time.Sleep(time.Second * time.Duration(cacheTimeout))

	// After cache timeout elapses, subsequent get attr should need to call next component
	suite.mock.EXPECT().GetAttr(options).Return(getPathAttr(path, defaultSize, fs.FileMode(defaultMode), true), nil)
	_, err = suite.attrCache.GetAttr(options)
	suite.assert.Nil(err)
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
	suite.assert.NotNil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, link)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Success
	// Entry Does Not Already Exist
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.Nil(err)
	suite.assert.NotContains(suite.attrCache.cacheMap, link)
	suite.assert.NotContains(suite.attrCache.cacheMap, path)

	// Entry Already Exists
	addPathToCache(suite.assert, suite.attrCache, link, false)
	addPathToCache(suite.assert, suite.attrCache, path, false)
	suite.mock.EXPECT().CreateLink(options).Return(nil)

	err = suite.attrCache.CreateLink(options)
	suite.assert.Nil(err)
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chmod(options).Return(nil)

			err = suite.attrCache.Chmod(options)
			suite.assert.Nil(err)
			suite.assert.Contains(suite.attrCache.cacheMap, truncatedPath)
			suite.assert.NotEqualValues(suite.attrCache.cacheMap[truncatedPath].attr, &internal.ObjAttr{})
			suite.assert.EqualValues(suite.attrCache.cacheMap[truncatedPath].attr.Size, defaultSize)
			suite.assert.EqualValues(suite.attrCache.cacheMap[truncatedPath].attr.Mode, mode) // new mode should be set
			suite.assert.True(suite.attrCache.cacheMap[truncatedPath].valid())
			suite.assert.True(suite.attrCache.cacheMap[truncatedPath].exists())
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
			suite.assert.NotNil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Success
			// Entry Does Not Already Exist
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.Nil(err)
			suite.assert.NotContains(suite.attrCache.cacheMap, truncatedPath)

			// Entry Already Exists
			addPathToCache(suite.assert, suite.attrCache, path, false)
			suite.mock.EXPECT().Chown(options).Return(nil)

			err = suite.attrCache.Chown(options)
			suite.assert.Nil(err)
			assertUntouched(suite, truncatedPath)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAttrCacheTestSuite(t *testing.T) {
	suite.Run(t, new(attrCacheTestSuite))
}
