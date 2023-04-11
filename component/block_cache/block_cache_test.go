/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type blockCacheTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	blockCache        *BlockCache
	loopback          internal.Component
	fake_storage_path string
}

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	loopback.Configure(true)

	return loopback
}

func newTestBlockCache(next internal.Component) *BlockCache {

	blockCache := NewBlockCacheComponent()
	blockCache.SetNextComponent(next)
	err := blockCache.Configure(true)
	if err != nil {
		panic("Unable to configure block cache.")
	}

	return blockCache.(*BlockCache)
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *blockCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	rand := randomString(8)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	config := fmt.Sprintf("block_cache:\n  block-size-mb: 8\n  mem-size-mb: 100\n  prefetch: 5\n  parallelism: 10\n\nloopbackfs:\n  path: %s\n", suite.fake_storage_path)
	log.Debug(config)

	// Delete the temp directories created
	os.RemoveAll(suite.fake_storage_path)
	suite.setupTestHelper(config)
}

func (suite *blockCacheTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	config.ResetConfig()
	config.ReadConfigFromReader(strings.NewReader(configuration))
	config.SetBool("read-only", true)

	suite.loopback = newLoopbackFS()
	suite.blockCache = newTestBlockCache(suite.loopback)
	suite.loopback.Start(context.Background())
	err := suite.blockCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start block cache [%s]", err.Error()))
	}

}

func (suite *blockCacheTestSuite) cleanupTest() {
	suite.loopback.Stop()
	err := suite.blockCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop block cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	os.RemoveAll(suite.fake_storage_path)
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default block cache generated
	emptyConfig := fmt.Sprintf("loopbackfs:\n  path: %s", suite.fake_storage_path)
	suite.setupTestHelper(emptyConfig) // setup a new block cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.blockCache.Name(), "block_cache")

	suite.assert.Equal(suite.blockCache.blockSizeMB, uint32(8))
	suite.assert.Equal(suite.blockCache.memSizeMB, uint32(1024))
	suite.assert.EqualValues(suite.blockCache.prefetch, uint32(8))
	suite.assert.Equal(suite.blockCache.workers, uint32(32))
}

// Tests configuration of block cache
func (suite *blockCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default block cache generated

	config := fmt.Sprintf("block_cache:\n  block-size-mb:16 \n  mem-size-mb:100 \n  prefetch: 5 \n  parallelism: 10\n\nloopbackfs:\n  path: %s\n\n", suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.blockCache.Name(), "block_cache")

	suite.assert.Equal(suite.blockCache.blockSizeMB, uint32(16))
	suite.assert.Equal(suite.blockCache.memSizeMB, uint32(100))
	suite.assert.EqualValues(suite.blockCache.prefetch, uint32(5))
	suite.assert.Equal(suite.blockCache.workers, uint32(10))
}

// // Tests CreateDir
// func (suite *blockCacheTestSuite) TestCreateDir() {
// 	defer suite.cleanupTest()
// 	path := "a"
// 	options := internal.CreateDirOptions{Name: path}
// 	err := suite.blockCache.CreateDir(options)
// 	suite.assert.Nil(err)

// 	// Path should not be added to the block cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestDeleteDir() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	// Configure to create empty files so we create the block in storage
// 	createEmptyFile := true
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, createEmptyFile, suite.fake_storage_path)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	dir := "dir"
// 	path := fmt.Sprintf("%s/file", dir)
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	// The block (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)
// 	// Delete the block since we can only delete empty directories
// 	suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: path})

// 	// Delete the directory
// 	err := suite.blockCache.DeleteDir(internal.DeleteDirOptions{Name: dir})
// 	suite.assert.Nil(err)
// 	suite.assert.False(suite.blockCache.policy.IsCached(dir)) // Directory should not be cached
// }

// // TODO: Test Deleting a directory that has a block in the block cache

// func (suite *blockCacheTestSuite) TestReadDirCase1() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1")
// 	file2 := filepath.Join(name, "file2")
// 	file3 := filepath.Join(name, "file3")
// 	// Create files directly in "fake_storage"
// 	suite.loopback.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.loopback.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})

// 	// Read the Directory
// 	dir, err := suite.blockCache.ReadDir(internal.ReadDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(file1, dir[0].Path)
// 	suite.assert.EqualValues(file2, dir[1].Path)
// 	suite.assert.EqualValues(file3, dir[2].Path)
// 	suite.assert.EqualValues(subdir, dir[3].Path)
// }

// func (suite *blockCacheTestSuite) TestReadDirCase2() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1")
// 	file2 := filepath.Join(name, "file2")
// 	file3 := filepath.Join(name, "file3")
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

// 	// Read the Directory
// 	dir, err := suite.blockCache.ReadDir(internal.ReadDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(subdir, dir[0].Path)
// 	suite.assert.EqualValues(file1, dir[1].Path)
// 	suite.assert.EqualValues(file2, dir[2].Path)
// 	suite.assert.EqualValues(file3, dir[3].Path)
// }

// func (suite *blockCacheTestSuite) TestReadDirCase3() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1")
// 	file2 := filepath.Join(name, "file2")
// 	file3 := filepath.Join(name, "file3")
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file1, Size: 1024})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})
// 	// Create the files in fake_storage and simulate different sizes
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

// 	// Read the Directory
// 	dir, err := suite.blockCache.ReadDir(internal.ReadDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(file1, dir[0].Path)
// 	suite.assert.EqualValues(1024, dir[0].Size)
// 	suite.assert.EqualValues(file2, dir[1].Path)
// 	suite.assert.EqualValues(1024, dir[1].Size)
// 	suite.assert.EqualValues(file3, dir[2].Path)
// 	suite.assert.EqualValues(1024, dir[2].Size)
// 	suite.assert.EqualValues(subdir, dir[3].Path)
// }

// func (suite *blockCacheTestSuite) TestReadDirMixed() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1") // case 1
// 	file2 := filepath.Join(name, "file2") // case 2
// 	file3 := filepath.Join(name, "file3") // case 3
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})
// 	// Create the files in fake_storage and simulate different sizes
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

// 	// Read the Directory
// 	dir, err := suite.blockCache.ReadDir(internal.ReadDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(file1, dir[0].Path)
// 	suite.assert.EqualValues(0, dir[0].Size)
// 	suite.assert.EqualValues(file3, dir[1].Path)
// 	suite.assert.EqualValues(1024, dir[1].Size)
// 	suite.assert.EqualValues(subdir, dir[2].Path)
// 	suite.assert.EqualValues(file2, dir[3].Path)
// 	suite.assert.EqualValues(1024, dir[3].Size)
// }

// func (suite *blockCacheTestSuite) TestReadDirError() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir" // Does not exist in cache or storage

// 	dir, err := suite.blockCache.ReadDir(internal.ReadDirOptions{Name: name})
// 	suite.assert.Nil(err) // This seems wrong, I feel like we should return ENOENT? But then again, see the comment in BlockBlob List.
// 	suite.assert.Empty(dir)
// }

// func (suite *blockCacheTestSuite) TestStreamDirCase1() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1")
// 	file2 := filepath.Join(name, "file2")
// 	file3 := filepath.Join(name, "file3")
// 	// Create files directly in "fake_storage"
// 	suite.loopback.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.loopback.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})

// 	// Read the Directory
// 	dir, _, err := suite.blockCache.StreamDir(internal.StreamDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(file1, dir[0].Path)
// 	suite.assert.EqualValues(file2, dir[1].Path)
// 	suite.assert.EqualValues(file3, dir[2].Path)
// 	suite.assert.EqualValues(subdir, dir[3].Path)
// }

// //TODO: case3 requires more thought due to the way loopback fs is designed, specifically getAttr and streamDir
// func (suite *blockCacheTestSuite) TestStreamDirCase2() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	name := "dir"
// 	subdir := filepath.Join(name, "subdir")
// 	file1 := filepath.Join(name, "file1")
// 	file2 := filepath.Join(name, "file2")
// 	file3 := filepath.Join(name, "file3")
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

// 	// Read the Directory
// 	dir, _, err := suite.blockCache.StreamDir(internal.StreamDirOptions{Name: name})
// 	suite.assert.Nil(err)
// 	suite.assert.NotEmpty(dir)
// 	suite.assert.EqualValues(4, len(dir))
// 	suite.assert.EqualValues(subdir, dir[0].Path)
// 	suite.assert.EqualValues(file1, dir[1].Path)
// 	suite.assert.EqualValues(file2, dir[2].Path)
// 	suite.assert.EqualValues(file3, dir[3].Path)
// }

// func (suite *blockCacheTestSuite) TestFileUsed() {
// 	defer suite.cleanupTest()
// 	suite.blockCache.FileUsed("temp")
// 	suite.blockCache.policy.IsCached("temp")
// }

// // File cache does not have CreateDir Method implemented hence results are undefined here
// func (suite *blockCacheTestSuite) TestIsDirEmpty() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "dir"
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})

// 	empty := suite.blockCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
// 	suite.assert.True(empty)
// }

// func (suite *blockCacheTestSuite) TestIsDirEmptyFalse() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "dir"
// 	subdir := filepath.Join(path, "subdir")
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})

// 	empty := suite.blockCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
// 	suite.assert.False(empty)
// }

// func (suite *blockCacheTestSuite) TestIsDirEmptyFalseInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "dir"
// 	file := filepath.Join(path, "file")
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	empty := suite.blockCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
// 	suite.assert.False(empty)
// }

// func (suite *blockCacheTestSuite) TestRenameDir() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	// Configure to create empty files so we create the block in storage
// 	createEmptyFile := true
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, createEmptyFile, suite.fake_storage_path)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	src := "src"
// 	dst := "dst"
// 	path := fmt.Sprintf("%s/file", src)
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: src, Mode: 0777})
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	// The block (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)
// 	// Delete the block since we can only delete empty directories
// 	suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: path})

// 	// Delete the directory
// 	err := suite.blockCache.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
// 	suite.assert.Nil(err)
// 	suite.assert.False(suite.blockCache.policy.IsCached(src)) // Directory should not be cached
// }

// func (suite *blockCacheTestSuite) TestCreateFile() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"
// 	options := internal.CreateFileOptions{Name: path}
// 	f, err := suite.blockCache.CreateFile(options)
// 	suite.assert.Nil(err)
// 	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

// 	// Path should be added to the block cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestCreateFileInDir() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	dir := "dir"
// 	path := fmt.Sprintf("%s/file", dir)
// 	options := internal.CreateFileOptions{Name: path}
// 	f, err := suite.blockCache.CreateFile(options)
// 	suite.assert.Nil(err)
// 	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

// 	// Path should be added to the block cache, including directory
// 	_, err = os.Stat(suite.cache_path + "/" + dir)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestCreateFileCreateEmptyFile() {
// 	defer suite.cleanupTest()
// 	// Configure to create empty files so we create the block in storage
// 	createEmptyFile := true
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, createEmptyFile, suite.fake_storage_path)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	path := "file"
// 	options := internal.CreateFileOptions{Name: path}
// 	f, err := suite.blockCache.CreateFile(options)
// 	suite.assert.Nil(err)
// 	suite.assert.False(f.Dirty()) // Handle should not be dirty since it was written to storage

// 	// Path should be added to the block cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestCreateFileInDirCreateEmptyFile() {
// 	defer suite.cleanupTest()
// 	// Configure to create empty files so we create the block in storage
// 	createEmptyFile := true
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, createEmptyFile, suite.fake_storage_path)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	dir := "dir"
// 	path := fmt.Sprintf("%s/file", dir)
// 	suite.blockCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
// 	f, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.assert.Nil(err)
// 	suite.assert.False(f.Dirty()) // Handle should be dirty since it was not created in storage

// 	// Path should be added to the block cache, including directory
// 	_, err = os.Stat(suite.cache_path + "/" + dir)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage, including directory
// 	_, err = os.Stat(suite.fake_storage_path + "/" + dir)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestSyncFile() {
// 	defer suite.cleanupTest()
// 	path := "file"

// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	// On a sync we open, sync, flush and close
// 	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
// 	suite.assert.Nil(err)
// 	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	// Path should not be in block cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestDeleteFile() {
// 	defer suite.cleanupTest()
// 	path := "file"

// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	err := suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: path})
// 	suite.assert.Nil(err)

// 	// Path should not be in block cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// // Case 2 Test cover when the block does not exist in storage but it exists in the local cache.
// // This can happen if createEmptyFile is false and the block hasn't been flushed yet.
// func (suite *blockCacheTestSuite) TestDeleteFileCase2() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

// 	err := suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: path})
// 	suite.assert.NotNil(err)
// 	suite.assert.Equal(err, syscall.EIO)

// 	// Path should not be in local cache (since we failed the operation)
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestDeleteFileError() {
// 	defer suite.cleanupTest()
// 	path := "file"
// 	err := suite.blockCache.DeleteFile(internal.DeleteFileOptions{Name: path})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.ENOENT, err)
// }

// func (suite *blockCacheTestSuite) TestOpenFileNotInCache() {
// 	defer suite.cleanupTest()
// 	path := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	// loop until block does not exist - done due to async nature of eviction
// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Download is required
// 	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(path, handle.Path)
// 	suite.assert.False(handle.Dirty())

// 	// File should exist in cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestOpenFileInCache() {
// 	defer suite.cleanupTest()
// 	path := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

// 	// Download is required
// 	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(path, handle.Path)
// 	suite.assert.False(handle.Dirty())

// 	// File should exist in cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// // Tests for GetProperties in OpenFile should be done in E2E tests
// // - there is no good way to test it here with a loopback FS without a mock component.

// func (suite *blockCacheTestSuite) TestCloseFile() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"

// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	// The block is in the cache but not in storage (see TestCreateFileInDirCreateEmptyFile)

// 	// CloseFile
// 	err := suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})
// 	suite.assert.Nil(err)

// 	// loop until block does not exist - done due to async nature of eviction
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	suite.assert.False(suite.blockCache.policy.IsCached(path)) // File should be invalidated
// 	// File should not be in cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// 	// File should be in storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestCloseFileTimeout() {
// 	defer suite.cleanupTest()
// 	suite.cleanupTest() // teardown the default block cache generated
// 	cacheTimeout := 5
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, cacheTimeout, suite.fake_storage_path)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	path := "file"

// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	// The block is in the cache but not in storage (see TestCreateFileInDirCreateEmptyFile)

// 	// CloseFile
// 	err := suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.False(suite.blockCache.policy.IsCached(path)) // File should be invalidated

// 	// File should be in cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// File should be in storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// loop until block does not exist - done due to async nature of eviction
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < (cacheTimeout*3) && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}

// 	// File should not be in cache
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))

// 	// File should be in storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestReadFileEmpty() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	d, err := suite.blockCache.ReadFile(internal.ReadFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.Empty(d)
// }

// func (suite *blockCacheTestSuite) TestReadFile() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

// 	handle, _ = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

// 	d, err := suite.blockCache.ReadFile(internal.ReadFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *blockCacheTestSuite) TestReadFileNoFlush() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

// 	handle, _ = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

// 	d, err := suite.blockCache.ReadFile(internal.ReadFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *blockCacheTestSuite) TestReadFileErrorBadFd() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle := handlemap.NewHandle(file)
// 	data, err := suite.blockCache.ReadFile(internal.ReadFileOptions{Handle: handle})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.EBADF, err)
// 	suite.assert.Nil(data)
// }

// func (suite *blockCacheTestSuite) TestReadInBufferEmpty() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	data := make([]byte, 0)
// 	length, err := suite.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(0, length)
// 	suite.assert.Empty(data)
// }

// func (suite *blockCacheTestSuite) TestReadInBufferNoFlush() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

// 	handle, _ = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

// 	output := make([]byte, 9)
// 	length, err := suite.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(data, output)
// 	suite.assert.EqualValues(len(data), length)
// }

// func (suite *blockCacheTestSuite) TestReadInBuffer() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

// 	handle, _ = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

// 	output := make([]byte, 9)
// 	length, err := suite.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(data, output)
// 	suite.assert.EqualValues(len(data), length)
// }

// func (suite *blockCacheTestSuite) TestReadInBufferErrorBadFd() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle := handlemap.NewHandle(file)
// 	length, err := suite.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: handle})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.EBADF, err)
// 	suite.assert.EqualValues(0, length)
// }

// func (suite *blockCacheTestSuite) TestWriteFile() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	handle.Flags.Clear(handlemap.HandleFlagDirty) // Technically create block will mark it as dirty, we just want to check write block updates the dirty flag, so temporarily set this to false
// 	testData := "test data"
// 	data := []byte(testData)
// 	length, err := suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(len(data), length)
// 	// Check that the local cache updated with data
// 	d, _ := os.ReadFile(suite.cache_path + "/" + file)
// 	suite.assert.EqualValues(data, d)
// 	suite.assert.True(handle.Dirty())
// }

// func (suite *blockCacheTestSuite) TestWriteFileErrorBadFd() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle := handlemap.NewHandle(file)
// 	len, err := suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.EBADF, err)
// 	suite.assert.EqualValues(0, len)
// }

// func (suite *blockCacheTestSuite) TestFlushFileEmpty() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	// Path should not be in fake storage
// 	_, err := os.Stat(suite.fake_storage_path + "/" + file)
// 	suite.assert.True(os.IsNotExist(err))

// 	// Flush the Empty File
// 	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.False(handle.Dirty())

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + file)
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestFlushFile() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

// 	// Path should not be in fake storage
// 	_, err := os.Stat(suite.fake_storage_path + "/" + file)
// 	suite.assert.True(os.IsNotExist(err))

// 	// Flush the Empty File
// 	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.False(handle.Dirty())

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + file)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Check that fake_storage updated with data
// 	d, _ := os.ReadFile(suite.fake_storage_path + "/" + file)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *blockCacheTestSuite) TestFlushFileErrorBadFd() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	handle := handlemap.NewHandle(file)
// 	handle.Flags.Set(handlemap.HandleFlagDirty)
// 	err := suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.EBADF, err)
// }

// func (suite *blockCacheTestSuite) TestGetAttrCase1() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	// Create files directly in "fake_storage"
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	// Read the Directory
// 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: file})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(attr)
// 	suite.assert.EqualValues(file, attr.Path)
// }

// func (suite *blockCacheTestSuite) TestGetAttrCase2() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

// 	// Read the Directory
// 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: file})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(attr)
// 	suite.assert.EqualValues(file, attr.Path)
// }

// func (suite *blockCacheTestSuite) TestGetAttrCase3() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: file, Size: 1024})
// 	// Create the files in fake_storage and simulate different sizes
// 	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777}) // Length is default 0

// 	// Read the Directory
// 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: file})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(attr)
// 	suite.assert.EqualValues(file, attr.Path)
// 	suite.assert.EqualValues(1024, attr.Size)
// }

// func (suite *blockCacheTestSuite) TestGetAttrCase4() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	file := "file"
// 	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
// 	createHandle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(createHandle)

// 	size := (100 * 1024 * 1024)
// 	data := make([]byte, size)

// 	written, err := suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: createHandle, Offset: 0, Data: data})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(size, written)

// 	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
// 	suite.assert.Nil(err)

// 	err = suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	suite.assert.Nil(err)

// 	// Wait  block is evicted
// 	_, err = os.Stat(suite.cache_path + "/" + file)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + file)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// open the block in parallel and try getting the size of block while open is on going
// 	go suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0666})

// 	// Read the Directory
// 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: file})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(attr)
// 	suite.assert.EqualValues(file, attr.Path)
// 	suite.assert.EqualValues(size, attr.Size)
// }

// // func (suite *blockCacheTestSuite) TestGetAttrError() {
// // defer suite.cleanupTest()
// // 	// Setup
// // 	name := "file"
// // 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: name})
// // 	suite.assert.NotNil(err)
// // 	suite.assert.EqualValues(syscall.ENOENT, err)
// // 	suite.assert.EqualValues("", attr.Name)
// // }

// func (suite *blockCacheTestSuite) TestRenameFileNotInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	src := "source"
// 	dst := "destination"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	_, err := os.Stat(suite.cache_path + "/" + src)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + src)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + src)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// RenameFile
// 	err = suite.blockCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
// 	suite.assert.Nil(err)

// 	// Path in fake storage should be updated
// 	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
// 	suite.assert.True(os.IsNotExist(err))
// 	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
// 	suite.assert.True(err == nil || os.IsExist(err))
// }

// func (suite *blockCacheTestSuite) TestRenameFileInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	src := "source"
// 	dst := "destination"
// 	createHandle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	openHandle, _ := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

// 	// Path should be in the block cache
// 	_, err := os.Stat(suite.cache_path + "/" + src)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + src)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// RenameFile
// 	err = suite.blockCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
// 	suite.assert.Nil(err)
// 	// Path in fake storage and block cache should be updated
// 	_, err = os.Stat(suite.cache_path + "/" + src) // Src does not exist
// 	suite.assert.True(os.IsNotExist(err))
// 	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall exists in cache
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
// 	suite.assert.True(os.IsNotExist(err))
// 	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
// }

// func (suite *blockCacheTestSuite) TestRenameFileCase2() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	src := "source"
// 	dst := "destination"
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})

// 	err := suite.blockCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
// 	suite.assert.NotNil(err)
// 	suite.assert.Equal(err, syscall.EIO)

// 	// Src should be in local cache (since we failed the operation)
// 	_, err = os.Stat(suite.cache_path + "/" + src)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Src should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + src)
// 	suite.assert.True(os.IsNotExist(err))
// 	// Dst should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + dst)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestTruncateFileNotInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chmod
// 	size := 1024
// 	err = suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
// 	suite.assert.Nil(err)

// 	// Path in fake storage should be updated
// 	info, _ := os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.EqualValues(info.Size(), size)
// }

// func (suite *blockCacheTestSuite) TestTruncateFileInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	createHandle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	openHandle, _ := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})

// 	// Path should be in the block cache
// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chmod
// 	size := 1024
// 	err = suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
// 	suite.assert.Nil(err)
// 	// Path in fake storage and block cache should be updated
// 	info, _ := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.EqualValues(info.Size(), size)
// 	info, _ = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.EqualValues(info.Size(), size)

// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
// }

// func (suite *blockCacheTestSuite) TestTruncateFileCase2() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})

// 	size := 1024
// 	err := suite.blockCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
// 	suite.assert.Nil(err)

// 	// Path should be in the block cache and size should be updated
// 	info, err := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(info.Size(), size)

// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestChmodNotInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chmod
// 	err = suite.blockCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0666)})
// 	suite.assert.Nil(err)

// 	// Path in fake storage should be updated
// 	info, _ := os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.EqualValues(info.Mode(), 0666)
// }

// func (suite *blockCacheTestSuite) TestChmodInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	createHandle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	openHandle, _ := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})

// 	// Path should be in the block cache
// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chmod
// 	err = suite.blockCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0755)})
// 	suite.assert.Nil(err)
// 	// Path in fake storage and block cache should be updated
// 	info, _ := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.EqualValues(info.Mode(), 0755)
// 	info, _ = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.EqualValues(info.Mode(), 0755)

// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
// }

// func (suite *blockCacheTestSuite) TestChmodCase2() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"
// 	oldMode := os.FileMode(0511)

// 	createHandle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
// 	suite.assert.Nil(err)

// 	newMode := os.FileMode(0666)
// 	err = suite.blockCache.Chmod(internal.ChmodOptions{Name: path, Mode: newMode})
// 	suite.assert.Nil(err)

// 	err = suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
// 	suite.assert.Nil(err)

// 	// Path should be in the block cache with old mode (since we failed the operation)
// 	info, err := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(info.Mode(), newMode)

// 	err = suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	suite.assert.Nil(err)

// 	// loop until block does not exist - done due to async nature of eviction
// 	_, err = os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Get the attributes and now and check block mode is set correctly or not
// 	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: path})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(attr)
// 	suite.assert.EqualValues(path, attr.Path)
// 	suite.assert.EqualValues(attr.Mode, newMode)
// }

// func (suite *blockCacheTestSuite) TestChownNotInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})

// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
// 		time.Sleep(time.Second)
// 		_, err = os.Stat(suite.cache_path + "/" + path)
// 	}
// 	suite.assert.True(os.IsNotExist(err))

// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chown
// 	owner := os.Getuid()
// 	group := os.Getgid()
// 	err = suite.blockCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
// 	suite.assert.Nil(err)

// 	// Path in fake storage should be updated
// 	info, err := os.Stat(suite.fake_storage_path + "/" + path)
// 	stat := info.Sys().(*syscall.Stat_t)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(owner, stat.Uid)
// 	suite.assert.EqualValues(group, stat.Gid)
// }

// func (suite *blockCacheTestSuite) TestChownInCache() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	path := "file"
// 	createHandle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
// 	openHandle, _ := suite.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})

// 	// Path should be in the block cache
// 	_, err := os.Stat(suite.cache_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	// Path should be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(err == nil || os.IsExist(err))

// 	// Chown
// 	owner := os.Getuid()
// 	group := os.Getgid()
// 	err = suite.blockCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
// 	suite.assert.Nil(err)
// 	// Path in fake storage and block cache should be updated
// 	info, err := os.Stat(suite.cache_path + "/" + path)
// 	stat := info.Sys().(*syscall.Stat_t)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(owner, stat.Uid)
// 	suite.assert.EqualValues(group, stat.Gid)
// 	info, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	stat = info.Sys().(*syscall.Stat_t)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(owner, stat.Uid)
// 	suite.assert.EqualValues(group, stat.Gid)

// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
// }

// func (suite *blockCacheTestSuite) TestChownCase2() {
// 	defer suite.cleanupTest()
// 	// Default is to not create empty files on create block to support immutable storage.
// 	path := "file"
// 	oldMode := os.FileMode(0511)
// 	suite.blockCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
// 	info, _ := os.Stat(suite.cache_path + "/" + path)
// 	stat := info.Sys().(*syscall.Stat_t)
// 	oldOwner := stat.Uid
// 	oldGroup := stat.Gid

// 	owner := os.Getuid()
// 	group := os.Getgid()
// 	err := suite.blockCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
// 	suite.assert.NotNil(err)
// 	suite.assert.Equal(err, syscall.EIO)

// 	// Path should be in the block cache with old group and owner (since we failed the operation)
// 	info, err = os.Stat(suite.cache_path + "/" + path)
// 	stat = info.Sys().(*syscall.Stat_t)
// 	suite.assert.True(err == nil || os.IsExist(err))
// 	suite.assert.EqualValues(oldOwner, stat.Uid)
// 	suite.assert.EqualValues(oldGroup, stat.Gid)
// 	// Path should not be in fake storage
// 	_, err = os.Stat(suite.fake_storage_path + "/" + path)
// 	suite.assert.True(os.IsNotExist(err))
// }

// func (suite *blockCacheTestSuite) TestZZMountPathConflict() {
// 	defer suite.cleanupTest()
// 	cacheTimeout := 1
// 	configuration := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, cacheTimeout, suite.fake_storage_path)

// 	blockCache := NewFileCacheComponent()
// 	config.ReadConfigFromReader(strings.NewReader(configuration))
// 	config.Set("mount-path", suite.cache_path)
// 	err := blockCache.Configure(true)
// 	suite.assert.NotNil(err)
// 	suite.assert.Contains(err.Error(), "[tmp-path is same as mount path]")
// }

// func (suite *blockCacheTestSuite) TestCachePathSymlink() {
// 	defer suite.cleanupTest()
// 	// Setup
// 	suite.cleanupTest()
// 	err := os.Mkdir(suite.cache_path, 0777)
// 	defer os.RemoveAll(suite.cache_path)
// 	suite.assert.Nil(err)
// 	symlinkPath := suite.cache_path + ".lnk"
// 	err = os.Symlink(suite.cache_path, symlinkPath)
// 	defer os.Remove(symlinkPath)
// 	suite.assert.Nil(err)
// 	configuration := fmt.Sprintf("block_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
// 		symlinkPath, suite.fake_storage_path)
// 	suite.setupTestHelper(configuration)

// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	testData := "test data"
// 	data := []byte(testData)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

// 	handle, _ = suite.blockCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

// 	d, err := suite.blockCache.ReadFile(internal.ReadFileOptions{Handle: handle})
// 	suite.assert.Nil(err)
// 	suite.assert.EqualValues(data, d)
// }

// func (suite *blockCacheTestSuite) TestZZOffloadIO() {
// 	defer suite.cleanupTest()
// 	configuration := fmt.Sprintf("block_cache:\n  path: %s\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, suite.fake_storage_path)

// 	suite.setupTestHelper(configuration)

// 	file := "file"
// 	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	suite.assert.Nil(err)
// 	suite.assert.NotNil(handle)
// 	suite.assert.True(handle.Cached())

// 	suite.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})
// }

// func (suite *blockCacheTestSuite) TestStatFS() {
// 	defer suite.cleanupTest()
// 	cacheTimeout := 5
// 	maxSizeMb := 2
// 	config := fmt.Sprintf("block_cache:\n  path: %s\n  max-size-mb: %d\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
// 		suite.cache_path, maxSizeMb, cacheTimeout, suite.fake_storage_path)
// 	os.Mkdir(suite.cache_path, 0777)
// 	suite.setupTestHelper(config) // setup a new block cache with a custom config (teardown will occur after the test as usual)

// 	file := "file"
// 	handle, _ := suite.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
// 	data := make([]byte, 1024*1024)
// 	suite.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
// 	suite.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})
// 	stat, ret, err := suite.blockCache.StatFs()
// 	suite.assert.Equal(ret, true)
// 	suite.assert.Equal(err, nil)
// 	suite.assert.NotEqual(stat, &syscall.Statfs_t{})
// }

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheTestSuite(t *testing.T) {
	suite.Run(t, new(blockCacheTestSuite))
}
