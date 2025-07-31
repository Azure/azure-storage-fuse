/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package file_cache

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type fileCacheTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	fileCache         *FileCache
	loopback          internal.Component
	cache_path        string
	fake_storage_path string
}

func newLoopbackFS() internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	loopback.Configure(true)

	return loopback
}

func newTestFileCache(next internal.Component) *FileCache {

	fileCache := NewFileCacheComponent()
	fileCache.SetNextComponent(next)
	err := fileCache.Configure(true)
	if err != nil {
		panic("Unable to configure file cache.")
	}

	return fileCache.(*FileCache)
}

func randomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *fileCacheTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	rand := randomString(8)
	suite.cache_path = filepath.Join(home_dir, "file_cache"+rand)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	log.Debug(defaultConfig)

	// Delete the temp directories created
	os.RemoveAll(suite.cache_path)
	os.RemoveAll(suite.fake_storage_path)
	suite.setupTestHelper(defaultConfig)
}

func (suite *fileCacheTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.loopback = newLoopbackFS()
	suite.fileCache = newTestFileCache(suite.loopback)
	suite.loopback.Start(context.Background())
	err := suite.fileCache.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start file cache [%s]", err.Error()))
	}

}

func (suite *fileCacheTestSuite) cleanupTest() {
	suite.loopback.Stop()
	err := suite.fileCache.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop file cache [%s]", err.Error()))
	}

	// Delete the temp directories created
	os.RemoveAll(suite.cache_path)
	os.RemoveAll(suite.fake_storage_path)
}

// Tests the default configuration of file cache
func (suite *fileCacheTestSuite) TestEmpty() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	emptyConfig := fmt.Sprintf("file_cache:\n  path: %s\n\n  offload-io: true\n\nloopbackfs:\n  path: %s", suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(emptyConfig) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.fileCache.Name(), "file_cache")
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), "lru")

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, defaultMaxEviction)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, defaultMaxThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, defaultMinThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, false)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, false)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, 120)
}

// Tests configuration of file cache
func (suite *fileCacheTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 60
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.fileCache.Name(), "file_cache")
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
}

func (suite *fileCacheTestSuite) TestDefaultCacheSize() {
	defer suite.cleanupTest()
	// Setup
	config := fmt.Sprintf("file_cache:\n  path: %s\n", suite.cache_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	cmd := exec.Command("bash", "-c", fmt.Sprintf("df -B1 %s | awk 'NR==2{print $4}'", suite.cache_path))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	suite.assert.Nil(err)
	freeDisk, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.Nil(err)
	expected := uint64(0.8 * float64(freeDisk))
	actual := suite.fileCache.maxCacheSize * MB
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance, "mssg:", actual, expected)
}

func (suite *fileCacheTestSuite) TestConfigPolicyTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 60
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.fileCache.Name(), "file_cache")
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).cacheTimeout, cacheTimeout)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
}

func (suite *fileCacheTestSuite) TestConfigPolicyDefaultTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := defaultFileCacheTimeout
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.fileCache.Name(), "file_cache")
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).cacheTimeout, cacheTimeout)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
}

func (suite *fileCacheTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	policy := "lru"
	maxSizeMb := 1024
	cacheTimeout := 0
	maxDeletion := 10
	highThreshold := 90
	lowThreshold := 10
	createEmptyFile := true
	allowNonEmptyTemp := true
	cleanupOnStart := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  policy: %s\n  max-size-mb: %d\n  timeout-sec: %d\n  max-eviction: %d\n  high-threshold: %d\n  low-threshold: %d\n  create-empty-file: %t\n  allow-non-empty-temp: %t\n  cleanup-on-start: %t",
		suite.cache_path, policy, maxSizeMb, cacheTimeout, maxDeletion, highThreshold, lowThreshold, createEmptyFile, allowNonEmptyTemp, cleanupOnStart)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	suite.assert.Equal(suite.fileCache.Name(), "file_cache")
	suite.assert.Equal(suite.fileCache.tmpPath, suite.cache_path)
	suite.assert.Equal(suite.fileCache.policy.Name(), policy)

	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxSizeMB, maxSizeMb)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).maxEviction, maxDeletion)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).highThreshold, highThreshold)
	suite.assert.EqualValues(suite.fileCache.policy.(*lruPolicy).lowThreshold, lowThreshold)

	suite.assert.Equal(suite.fileCache.createEmptyFile, createEmptyFile)
	suite.assert.Equal(suite.fileCache.allowNonEmpty, allowNonEmptyTemp)
	suite.assert.EqualValues(suite.fileCache.cacheTimeout, cacheTimeout)
}

// Tests CreateDir
func (suite *fileCacheTestSuite) TestCreateDir() {
	defer suite.cleanupTest()
	path := "a"
	options := internal.CreateDirOptions{Name: path}
	err := suite.fileCache.CreateDir(options)
	suite.assert.Nil(err)

	// Path should not be added to the file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestDeleteDir() {
	defer suite.cleanupTest()
	// Setup
	// Configure to create empty files so we create the file in storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	dir := "dir"
	path := fmt.Sprintf("%s/file", dir)
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)
	// Delete the file since we can only delete empty directories
	suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})

	// Delete the directory
	err := suite.fileCache.DeleteDir(internal.DeleteDirOptions{Name: dir})
	suite.assert.Nil(err)
	suite.assert.False(suite.fileCache.policy.IsCached(dir)) // Directory should not be cached
}

// TODO: Test Deleting a directory that has a file in the file cache

func (suite *fileCacheTestSuite) TestReadDirCase1() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1")
	file2 := filepath.Join(name, "file2")
	file3 := filepath.Join(name, "file3")
	// Create files directly in "fake_storage"
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})

	// Read the Directory
	dir, err := suite.fileCache.ReadDir(internal.ReadDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)
	suite.assert.EqualValues(4, len(dir))
	suite.assert.EqualValues(file1, dir[0].Path)
	suite.assert.EqualValues(file2, dir[1].Path)
	suite.assert.EqualValues(file3, dir[2].Path)
	suite.assert.EqualValues(subdir, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestReadDirCase2() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1")
	file2 := filepath.Join(name, "file2")
	file3 := filepath.Join(name, "file3")
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	// Read the Directory
	dir, err := suite.fileCache.ReadDir(internal.ReadDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)
	suite.assert.EqualValues(4, len(dir))
	suite.assert.EqualValues(subdir, dir[0].Path)
	suite.assert.EqualValues(file1, dir[1].Path)
	suite.assert.EqualValues(file2, dir[2].Path)
	suite.assert.EqualValues(file3, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestReadDirCase3() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1")
	file2 := filepath.Join(name, "file2")
	file3 := filepath.Join(name, "file3")
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file1, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})
	// Create the files in fake_storage and simulate different sizes
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	// Read the Directory
	dir, err := suite.fileCache.ReadDir(internal.ReadDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)
	suite.assert.EqualValues(4, len(dir))
	suite.assert.EqualValues(file1, dir[0].Path)
	suite.assert.EqualValues(1024, dir[0].Size)
	suite.assert.EqualValues(file2, dir[1].Path)
	suite.assert.EqualValues(1024, dir[1].Size)
	suite.assert.EqualValues(file3, dir[2].Path)
	suite.assert.EqualValues(1024, dir[2].Size)
	suite.assert.EqualValues(subdir, dir[3].Path)
}

func pos(s []*internal.ObjAttr, e string) int {
	for i, v := range s {
		if v.Path == e {
			return i
		}
	}
	return -1
}

func (suite *fileCacheTestSuite) TestReadDirMixed() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1") // case 1
	file2 := filepath.Join(name, "file2") // case 2
	file3 := filepath.Join(name, "file3") // case 3
	file4 := filepath.Join(name, "file4") // case 4

	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})

	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file2, Size: 1024})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file3, Size: 1024})

	// Create the files in fake_storage and simulate different sizes
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777}) // Length is default 0
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file4, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file4, Size: 1024})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file4, Size: 0})

	// Read the Directory
	dir, err := suite.fileCache.ReadDir(internal.ReadDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)

	var i int
	i = pos(dir, file1)
	suite.assert.EqualValues(0, dir[i].Size)

	i = pos(dir, file3)
	suite.assert.EqualValues(1024, dir[i].Size)

	i = pos(dir, file2)
	suite.assert.EqualValues(1024, dir[i].Size)

	i = pos(dir, file4)
	suite.assert.EqualValues(0, dir[i].Size)
}

func (suite *fileCacheTestSuite) TestReadDirError() {
	defer suite.cleanupTest()
	// Setup
	name := "dir" // Does not exist in cache or storage

	dir, err := suite.fileCache.ReadDir(internal.ReadDirOptions{Name: name})
	suite.assert.Nil(err) // This seems wrong, I feel like we should return ENOENT? But then again, see the comment in BlockBlob List.
	suite.assert.Empty(dir)
}

func (suite *fileCacheTestSuite) TestStreamDirCase1() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1")
	file2 := filepath.Join(name, "file2")
	file3 := filepath.Join(name, "file3")
	// Create files directly in "fake_storage"
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.loopback.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file1})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file2})
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file3})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)
	suite.assert.EqualValues(4, len(dir))
	suite.assert.EqualValues(file1, dir[0].Path)
	suite.assert.EqualValues(file2, dir[1].Path)
	suite.assert.EqualValues(file3, dir[2].Path)
	suite.assert.EqualValues(subdir, dir[3].Path)
}

// TODO: case3 requires more thought due to the way loopback fs is designed, specifically getAttr and streamDir
func (suite *fileCacheTestSuite) TestStreamDirCase2() {
	defer suite.cleanupTest()
	// Setup
	name := "dir"
	subdir := filepath.Join(name, "subdir")
	file1 := filepath.Join(name, "file1")
	file2 := filepath.Join(name, "file2")
	file3 := filepath.Join(name, "file3")
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file1, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file2, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file3, Mode: 0777})

	// Read the Directory
	dir, _, err := suite.fileCache.StreamDir(internal.StreamDirOptions{Name: name})
	suite.assert.Nil(err)
	suite.assert.NotEmpty(dir)
	suite.assert.EqualValues(4, len(dir))
	suite.assert.EqualValues(subdir, dir[0].Path)
	suite.assert.EqualValues(file1, dir[1].Path)
	suite.assert.EqualValues(file2, dir[2].Path)
	suite.assert.EqualValues(file3, dir[3].Path)
}

func (suite *fileCacheTestSuite) TestFileUsed() {
	defer suite.cleanupTest()
	suite.fileCache.FileUsed("temp")
	suite.fileCache.policy.IsCached("temp")
}

// File cache does not have CreateDir Method implemented hence results are undefined here
func (suite *fileCacheTestSuite) TestIsDirEmpty() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.True(empty)
}

func (suite *fileCacheTestSuite) TestIsDirEmptyFalse() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	subdir := filepath.Join(path, "subdir")
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: subdir, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.False(empty)
}

func (suite *fileCacheTestSuite) TestIsDirEmptyFalseInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "dir"
	file := filepath.Join(path, "file")
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: path, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: path})
	suite.assert.False(empty)
}

func (suite *fileCacheTestSuite) TestRenameDir() {
	defer suite.cleanupTest()
	// Setup
	// Configure to create empty files so we create the file in storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "src"
	dst := "dst"
	path := fmt.Sprintf("%s/file", src)
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: src, Mode: 0777})
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file (and directory) is in the cache and storage (see TestCreateFileInDirCreateEmptyFile)
	// Delete the file since we can only delete empty directories
	suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})

	// Delete the directory
	err := suite.fileCache.RenameDir(internal.RenameDirOptions{Src: src, Dst: dst})
	suite.assert.Nil(err)
	suite.assert.False(suite.fileCache.policy.IsCached(src)) // Directory should not be cached
}

func (suite *fileCacheTestSuite) TestCreateFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file1"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

	// Path should be added to the file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestCreateFileWithNoPerm() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file1"
	options := internal.CreateFileOptions{Name: path, Mode: 0000}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

	// Path should be added to the file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
	info, err := os.Stat(suite.cache_path + "/" + path)
	// Since the default config has timeout-sec as 0 there is a chance that the file gets evicted before we stat the file.
	if err == nil && info != nil {
		suite.assert.Equal(info.Mode(), os.FileMode(0000))
	}
}

func (suite *fileCacheTestSuite) TestCreateFileWithWritePerm() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file1"
	options := internal.CreateFileOptions{Name: path, Mode: 0222}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

	os.Chmod(suite.cache_path+"/"+path, 0331)

	// Path should be added to the file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
	info, _ := os.Stat(suite.cache_path + "/" + path)
	if info != nil {
		suite.assert.Equal(info.Mode(), fs.FileMode(0331))
	}
}

func (suite *fileCacheTestSuite) TestCreateFileInDir() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	dir := "dir"
	path := fmt.Sprintf("%s/file", dir)
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.True(f.Dirty()) // Handle should be dirty since it was not created in storage

	// Path should be added to the file cache, including directory
	_, err = os.Stat(suite.cache_path + "/" + dir)
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestCreateFileCreateEmptyFile() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file2"
	options := internal.CreateFileOptions{Name: path}
	f, err := suite.fileCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty()) // Handle should not be dirty since it was written to storage

	// Path should be added to the file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestCreateFileInDirCreateEmptyFile() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	dir := "dir"
	path := fmt.Sprintf("%s/file", dir)
	suite.fileCache.CreateDir(internal.CreateDirOptions{Name: dir, Mode: 0777})
	f, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty()) // Handle should be dirty since it was not created in storage

	// Path should be added to the file cache, including directory
	_, err = os.Stat(suite.cache_path + "/" + dir)
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage, including directory
	_, err = os.Stat(suite.fake_storage_path + "/" + dir)
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestSyncFile() {
	defer suite.cleanupTest()
	path := "file3"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// On a sync we open, sync, flush and close
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	err = suite.fileCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.Nil(err)
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// Path should not be in file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))

	path = "file.fsync"
	suite.fileCache.syncToFlush = true
	handle, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	_, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(handle.Dirty(), true)
	err = suite.fileCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.Equal(handle.Dirty(), false)
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.fileCache.syncToFlush = false
}

func (suite *fileCacheTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	path := "file4"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.Nil(err)

	// Path should not be in file cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}

// Case 2 Test cover when the file does not exist in storage but it exists in the local cache.
// This can happen if createEmptyFile is false and the file hasn't been flushed yet.
func (suite *fileCacheTestSuite) TestDeleteFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file5"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})

	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NotNil(err)
	suite.assert.Equal(err, syscall.EIO)

	// Path should not be in local cache (since we failed the operation)
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestDeleteFileError() {
	defer suite.cleanupTest()
	path := "file6"
	err := suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: path})
	suite.assert.NotNil(err)
	suite.assert.EqualValues(syscall.ENOENT, err)
}

func (suite *fileCacheTestSuite) TestOpenFileNotInCache() {
	defer suite.cleanupTest()
	path := "file7"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	// loop until file does not exist - done due to async nature of eviction
	_, err := os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Download is required
	handle, err = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.EqualValues(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestOpenFileInCache() {
	defer suite.cleanupTest()
	path := "file8"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// Download is required
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.EqualValues(path, handle.Path)
	suite.assert.False(handle.Dirty())

	// File should exist in cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

// Tests for GetProperties in OpenFile should be done in E2E tests
// - there is no good way to test it here with a loopback FS without a mock component.

func (suite *fileCacheTestSuite) TestCloseFile() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file9"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file is in the cache but not in storage (see TestCreateFileInDirCreateEmptyFile)

	// CloseFile
	err := suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.Nil(err)

	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	suite.assert.False(suite.fileCache.policy.IsCached(path)) // File should be invalidated
	// File should not be in cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
	// File should be in storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestCloseFileTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest() // teardown the default file cache generated
	cacheTimeout := 5
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, cacheTimeout, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file10"

	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	// The file is in the cache but not in storage (see TestCreateFileInDirCreateEmptyFile)

	// CloseFile
	err := suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.False(suite.fileCache.policy.IsCached(path)) // File should be invalidated

	// File should be in cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// File should be in storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < (cacheTimeout*3) && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}

	// File should not be in cache
	_, err = os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))

	// File should be in storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestReadFileEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file11"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	d, err := suite.fileCache.ReadFile(internal.ReadFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.Empty(d)
}

func (suite *fileCacheTestSuite) TestReadFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file12"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	d, err := suite.fileCache.ReadFile(internal.ReadFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, d)
}

func (suite *fileCacheTestSuite) TestReadFileNoFlush() {
	defer suite.cleanupTest()
	// Setup
	file := "file13"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	d, err := suite.fileCache.ReadFile(internal.ReadFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, d)
}

func (suite *fileCacheTestSuite) TestReadFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file14"
	handle := handlemap.NewHandle(file)
	data, err := suite.fileCache.ReadFile(internal.ReadFileOptions{Handle: handle})
	suite.assert.NotNil(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.Nil(data)
}

func (suite *fileCacheTestSuite) TestReadInBufferEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file15"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	data := make([]byte, 0)
	length, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.EqualValues(0, length)
	suite.assert.Empty(data)
}

func (suite *fileCacheTestSuite) TestReadInBufferNoFlush() {
	defer suite.cleanupTest()
	// Setup
	file := "file16"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, output)
	suite.assert.EqualValues(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBuffer() {
	defer suite.cleanupTest()
	// Setup
	file := "file17"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	output := make([]byte, 9)
	length, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle, Offset: 0, Data: output})
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, output)
	suite.assert.EqualValues(len(data), length)
}

func (suite *fileCacheTestSuite) TestReadInBufferErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file18"
	handle := handlemap.NewHandle(file)
	length, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: handle})
	suite.assert.NotNil(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, length)
}

func (suite *fileCacheTestSuite) TestWriteFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file19"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	handle.Flags.Clear(handlemap.HandleFlagDirty) // Technically create file will mark it as dirty, we just want to check write file updates the dirty flag, so temporarily set this to false
	testData := "test data"
	data := []byte(testData)
	length, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	suite.assert.Nil(err)
	suite.assert.EqualValues(len(data), length)
	// Check that the local cache updated with data
	d, _ := os.ReadFile(suite.cache_path + "/" + file)
	suite.assert.EqualValues(data, d)
	suite.assert.True(handle.Dirty())
}

func (suite *fileCacheTestSuite) TestWriteFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file20"
	handle := handlemap.NewHandle(file)
	len, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle})
	suite.assert.NotNil(err)
	suite.assert.EqualValues(syscall.EBADF, err)
	suite.assert.EqualValues(0, len)
}

func (suite *fileCacheTestSuite) TestFlushFileEmpty() {
	defer suite.cleanupTest()
	// Setup
	file := "file21"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Path should not be in fake storage
	_, err := os.Stat(suite.fake_storage_path + "/" + file)
	suite.assert.True(os.IsNotExist(err))

	// Flush the Empty File
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.False(handle.Dirty())

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + file)
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestFlushFile() {
	defer suite.cleanupTest()
	// Setup
	file := "file22"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})

	// Path should not be in fake storage
	_, err := os.Stat(suite.fake_storage_path + "/" + file)
	suite.assert.True(os.IsNotExist(err))

	// Flush the Empty File
	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.False(handle.Dirty())

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + file)
	suite.assert.True(err == nil || os.IsExist(err))
	// Check that fake_storage updated with data
	d, _ := os.ReadFile(suite.fake_storage_path + "/" + file)
	suite.assert.EqualValues(data, d)
}

func (suite *fileCacheTestSuite) TestFlushFileErrorBadFd() {
	defer suite.cleanupTest()
	// Setup
	file := "file23"
	handle := handlemap.NewHandle(file)
	handle.Flags.Set(handlemap.HandleFlagDirty)
	err := suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	suite.assert.NotNil(err)
	suite.assert.EqualValues(syscall.EBADF, err)
}

func (suite *fileCacheTestSuite) TestGetAttrCase1() {
	defer suite.cleanupTest()
	// Setup
	file := "file24"
	// Create files directly in "fake_storage"
	suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.Nil(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
}

func (suite *fileCacheTestSuite) TestGetAttrCase2() {
	defer suite.cleanupTest()
	// Setup
	file := "file25"
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.Nil(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
}

func (suite *fileCacheTestSuite) TestGetAttrCase3() {
	defer suite.cleanupTest()
	// Setup
	file := "file26"
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: file, Size: 1024})
	// Create the files in fake_storage and simulate different sizes
	//suite.loopback.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777}) // Length is default 0

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.Nil(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
	suite.assert.EqualValues(1024, attr.Size)
}

func (suite *fileCacheTestSuite) TestGetAttrCase4() {
	defer suite.cleanupTest()
	// Setup
	file := "file27"
	// By default createEmptyFile is false, so we will not create these files in storage until they are closed.
	createHandle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(createHandle)

	size := (100 * 1024 * 1024)
	data := make([]byte, size)

	written, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: createHandle, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.EqualValues(size, written)

	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
	suite.assert.Nil(err)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	suite.assert.Nil(err)

	// Wait  file is evicted
	_, err = os.Stat(suite.cache_path + "/" + file)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + file)
	}
	suite.assert.True(os.IsNotExist(err))

	// open the file in parallel and try getting the size of file while open is on going
	go suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0666})

	// Read the Directory
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: file})
	suite.assert.Nil(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(file, attr.Path)
	suite.assert.EqualValues(size, attr.Size)
}

// func (suite *fileCacheTestSuite) TestGetAttrError() {
// defer suite.cleanupTest()
// 	// Setup
// 	name := "file"
// 	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: name})
// 	suite.assert.NotNil(err)
// 	suite.assert.EqualValues(syscall.ENOENT, err)
// 	suite.assert.EqualValues("", attr.Name)
// }

func (suite *fileCacheTestSuite) TestRenameFileNotInCache() {
	defer suite.cleanupTest()
	// Setup
	src := "source1"
	dst := "destination1"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(suite.cache_path + "/" + src)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + src)
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.Nil(err)

	// Path in fake storage should be updated
	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
	suite.assert.True(err == nil || os.IsExist(err))
}

func (suite *fileCacheTestSuite) TestRenameFileInCache() {
	defer suite.cleanupTest()
	// Setup
	src := "source2"
	dst := "destination2"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	_, err = os.Stat(suite.cache_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall exists in cache
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
	suite.assert.True(err == nil || os.IsExist(err))

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestRenameFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	src := "source3"
	dst := "destination3"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0777})

	err := suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.NotNil(err)
	suite.assert.Equal(err, syscall.EIO)

	// Src should be in local cache (since we failed the operation)
	_, err = os.Stat(suite.cache_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))
	// Src should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + src)
	suite.assert.True(os.IsNotExist(err))
	// Dst should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + dst)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestRenameFileAndCacheCleanup() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 10\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "source4"
	dst := "destination4"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	_, err = os.Stat(suite.cache_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall exists in cache
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
	suite.assert.True(err == nil || os.IsExist(err))

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})

	time.Sleep(5 * time.Second)                    // Check once before the cache cleanup that file exists
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall exists in cache
	suite.assert.True(err == nil || os.IsExist(err))

	time.Sleep(8 * time.Second)                    // Wait for the cache cleanup to occur
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall not exists in cache
	suite.assert.True(err == nil || os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestRenameFileAndCacheCleanupWithNoTimeout() {
	defer suite.cleanupTest()
	suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	src := "source5"
	dst := "destination5"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: src, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: src, Mode: 0666})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + src)
	suite.assert.True(err == nil || os.IsExist(err))

	// RenameFile
	err = suite.fileCache.RenameFile(internal.RenameFileOptions{Src: src, Dst: dst})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	_, err = os.Stat(suite.cache_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall exists in cache
	suite.assert.True(err == nil || os.IsExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + src) // Src does not exist
	suite.assert.True(os.IsNotExist(err))
	_, err = os.Stat(suite.fake_storage_path + "/" + dst) // Dst does exist
	suite.assert.True(err == nil || os.IsExist(err))

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})

	time.Sleep(1 * time.Second)                    // Wait for the cache cleanup to occur
	_, err = os.Stat(suite.cache_path + "/" + dst) // Dst shall not exists in cache
	suite.assert.True(err == nil || os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestTruncateFileNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file30"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chmod
	size := 1024
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.Nil(err)

	// Path in fake storage should be updated
	info, _ := os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.EqualValues(info.Size(), size)
}

func (suite *fileCacheTestSuite) TestTruncateFileInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file31"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chmod
	size := 1024
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	info, _ := os.Stat(suite.cache_path + "/" + path)
	suite.assert.EqualValues(info.Size(), size)
	info, _ = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.EqualValues(info.Size(), size)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestTruncateFileCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file32"
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})

	size := 1024
	err := suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: path, Size: int64(size)})
	suite.assert.Nil(err)

	// Path should be in the file cache and size should be updated
	info, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(info.Size(), size)

	// Path should not be in fake storage
	// With new changes we always download and then truncate so file will exists in local path
	// _, err = os.Stat(suite.fake_storage_path + "/" + path)
	// suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestChmodNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file33"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chmod
	err = suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0666)})
	suite.assert.Nil(err)

	// Path in fake storage should be updated
	info, _ := os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.EqualValues(info.Mode(), 0666)
}

func (suite *fileCacheTestSuite) TestChmodInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file34"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0666})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0666})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chmod
	err = suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: os.FileMode(0755)})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	info, _ := os.Stat(suite.cache_path + "/" + path)
	suite.assert.EqualValues(info.Mode(), 0755)
	info, _ = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.EqualValues(info.Mode(), 0755)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestChmodCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file35"
	oldMode := os.FileMode(0511)

	createHandle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	suite.assert.Nil(err)

	newMode := os.FileMode(0666)
	err = suite.fileCache.Chmod(internal.ChmodOptions{Name: path, Mode: newMode})
	suite.assert.Nil(err)

	err = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: createHandle})
	suite.assert.Nil(err)

	// Path should be in the file cache with old mode (since we failed the operation)
	info, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(info.Mode(), newMode)

	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	suite.assert.Nil(err)

	// loop until file does not exist - done due to async nature of eviction
	_, err = os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Get the attributes and now and check file mode is set correctly or not
	attr, err := suite.fileCache.GetAttr(internal.GetAttrOptions{Name: path})
	suite.assert.Nil(err)
	suite.assert.NotNil(attr)
	suite.assert.EqualValues(path, attr.Path)
	suite.assert.EqualValues(attr.Mode, newMode)
}

func (suite *fileCacheTestSuite) TestChownNotInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file36"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})

	_, err := os.Stat(suite.cache_path + "/" + path)
	for i := 0; i < 10 && !os.IsNotExist(err); i++ {
		time.Sleep(time.Second)
		_, err = os.Stat(suite.cache_path + "/" + path)
	}
	suite.assert.True(os.IsNotExist(err))

	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err = suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.Nil(err)

	// Path in fake storage should be updated
	info, err := os.Stat(suite.fake_storage_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
}

func (suite *fileCacheTestSuite) TestChownInCache() {
	defer suite.cleanupTest()
	// Setup
	path := "file37"
	createHandle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: createHandle})
	openHandle, _ := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: path, Mode: 0777})

	// Path should be in the file cache
	_, err := os.Stat(suite.cache_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))
	// Path should be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(err == nil || os.IsExist(err))

	// Chown
	owner := os.Getuid()
	group := os.Getgid()
	err = suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.Nil(err)
	// Path in fake storage and file cache should be updated
	info, err := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)
	info, err = os.Stat(suite.fake_storage_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(owner, stat.Uid)
	suite.assert.EqualValues(group, stat.Gid)

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: openHandle})
}

func (suite *fileCacheTestSuite) TestChownCase2() {
	defer suite.cleanupTest()
	// Default is to not create empty files on create file to support immutable storage.
	path := "file38"
	oldMode := os.FileMode(0511)
	suite.fileCache.CreateFile(internal.CreateFileOptions{Name: path, Mode: oldMode})
	info, _ := os.Stat(suite.cache_path + "/" + path)
	stat := info.Sys().(*syscall.Stat_t)
	oldOwner := stat.Uid
	oldGroup := stat.Gid

	owner := os.Getuid()
	group := os.Getgid()
	err := suite.fileCache.Chown(internal.ChownOptions{Name: path, Owner: owner, Group: group})
	suite.assert.NotNil(err)
	suite.assert.Equal(err, syscall.EIO)

	// Path should be in the file cache with old group and owner (since we failed the operation)
	info, err = os.Stat(suite.cache_path + "/" + path)
	stat = info.Sys().(*syscall.Stat_t)
	suite.assert.True(err == nil || os.IsExist(err))
	suite.assert.EqualValues(oldOwner, stat.Uid)
	suite.assert.EqualValues(oldGroup, stat.Gid)
	// Path should not be in fake storage
	_, err = os.Stat(suite.fake_storage_path + "/" + path)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *fileCacheTestSuite) TestZZMountPathConflict() {
	defer suite.cleanupTest()
	cacheTimeout := 1
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, cacheTimeout, suite.fake_storage_path)

	fileCache := NewFileCacheComponent()
	config.ReadConfigFromReader(strings.NewReader(configuration))
	config.Set("mount-path", suite.cache_path)
	err := fileCache.Configure(true)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "[tmp-path is same as mount path]")
}

func (suite *fileCacheTestSuite) TestCachePathSymlink() {
	defer suite.cleanupTest()
	// Setup
	suite.cleanupTest()
	err := os.Mkdir(suite.cache_path, 0777)
	defer os.RemoveAll(suite.cache_path)
	suite.assert.Nil(err)
	symlinkPath := suite.cache_path + ".lnk"
	err = os.Symlink(suite.cache_path, symlinkPath)
	defer os.Remove(symlinkPath)
	suite.assert.Nil(err)
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		symlinkPath, suite.fake_storage_path)
	suite.setupTestHelper(configuration)

	file := "file39"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	handle, _ = suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})

	d, err := suite.fileCache.ReadFile(internal.ReadFileOptions{Handle: handle})
	suite.assert.Nil(err)
	suite.assert.EqualValues(data, d)
}

func (suite *fileCacheTestSuite) TestZZOffloadIO() {
	defer suite.cleanupTest()
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)

	suite.setupTestHelper(configuration)

	file := "file40"
	handle, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(handle)
	suite.assert.True(handle.Cached())

	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
}

func (suite *fileCacheTestSuite) TestZZZZLazyWrite() {
	defer suite.cleanupTest()
	configuration := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 0\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)

	suite.setupTestHelper(configuration)
	suite.fileCache.lazyWrite = true

	file := "file101"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	_ = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(5 * time.Second)
	suite.fileCache.lazyWrite = false

	// As lazy write is enabled flush shall not upload the file
	suite.assert.False(handle.Dirty())
}

func (suite *fileCacheTestSuite) TestStatFS() {
	defer suite.cleanupTest()
	cacheTimeout := 5
	maxSizeMb := 2
	config := fmt.Sprintf("file_cache:\n  path: %s\n  max-size-mb: %d\n  offload-io: true\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, maxSizeMb, cacheTimeout, suite.fake_storage_path)
	os.Mkdir(suite.cache_path, 0777)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	file := "file41"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	suite.fileCache.FlushFile(internal.FlushFileOptions{Handle: handle})
	stat, ret, err := suite.fileCache.StatFs()
	suite.assert.Equal(ret, true)
	suite.assert.Equal(err, nil)
	suite.assert.NotEqual(stat, &syscall.Statfs_t{})
}

func (suite *fileCacheTestSuite) TestReadFileWithRefresh() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in storage
	createEmptyFile := true
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  create-empty-file: %t\n  timeout-sec: 1000\n  refresh-sec: 10\n\nloopbackfs:\n  path: %s",
		suite.cache_path, createEmptyFile, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	path := "file42"
	err := os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data"), 0777)
	suite.assert.Nil(err)

	data := make([]byte, 20)
	options := internal.OpenFileOptions{Name: path, Mode: 0777}

	// Read file once and we shall get the same data
	f, err := suite.fileCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty())
	n, err := suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	// Modify the fil ein background but we shall still get the old data
	err = os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data1"), 0777)
	suite.assert.Nil(err)
	f, err = suite.fileCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty())
	n, err = suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(9, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	// Now wait for 5 seconds and we shall get the updated content on next read
	err = os.WriteFile(suite.fake_storage_path+"/"+path, []byte("test data123456"), 0777)
	suite.assert.Nil(err)
	time.Sleep(12 * time.Second)
	f, err = suite.fileCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty())
	n, err = suite.fileCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(15, n)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)
}

func (suite *fileCacheTestSuite) TestHardLimitOnSize() {
	defer suite.cleanupTest()
	// Configure to create empty files so we create the file in storage
	config := fmt.Sprintf("file_cache:\n  path: %s\n  offload-io: true\n  hard-limit: true\n  max-size-mb: 2\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	data := make([]byte, 3*MB)
	pathbig := "filebig"
	err := os.WriteFile(suite.fake_storage_path+"/"+pathbig, data, 0777)
	suite.assert.Nil(err)

	data = make([]byte, 1*MB)
	pathsmall := "filesmall"
	err = os.WriteFile(suite.fake_storage_path+"/"+pathsmall, data, 0777)
	suite.assert.Nil(err)

	// try opening small file
	options := internal.OpenFileOptions{Name: pathsmall, Mode: 0777}
	f, err := suite.fileCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.False(f.Dirty())
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	// try opening bigger file which shall fail due to hardlimit
	options = internal.OpenFileOptions{Name: pathbig, Mode: 0777}
	f, err = suite.fileCache.OpenFile(options)
	suite.assert.NotNil(err)
	suite.assert.Nil(f)
	suite.assert.Equal(err, syscall.ENOSPC)

	// try writing a small file
	options1 := internal.CreateFileOptions{Name: pathsmall + "_new", Mode: 0777}
	f, err = suite.fileCache.CreateFile(options1)
	suite.assert.Nil(err)
	data = make([]byte, 1*MB)
	n, err := suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(n, 1*MB)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	// try writing a bigger file
	options1 = internal.CreateFileOptions{Name: pathbig + "_new", Mode: 0777}
	f, err = suite.fileCache.CreateFile(options1)
	suite.assert.Nil(err)
	data = make([]byte, 3*MB)
	n, err = suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: f, Offset: 0, Data: data})
	suite.assert.NotNil(err)
	suite.assert.Equal(n, 0)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: f})
	suite.assert.Nil(err)

	// try opening small file
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: pathsmall, Size: 1 * MB})
	suite.assert.Nil(err)

	// try opening small file
	err = suite.fileCache.TruncateFile(internal.TruncateFileOptions{Name: pathsmall, Size: 3 * MB})
	suite.assert.NotNil(err)
}

// create a list of empty directories in local and storage and then try to delete those to validate empty directories
// are allowed be to deleted but non empty are not
func (suite *fileCacheTestSuite) TestDeleteDirectory() {
	defer suite.cleanupTest()

	config := fmt.Sprintf("file_cache:\n  path: %s\n  timeout-sec: 1000\n\nloopbackfs:\n  path: %s",
		suite.cache_path, suite.fake_storage_path)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	// Create local and remote dir structures
	suite.createLocalDirectoryStructure()
	suite.createRemoteDirectoryStructure()

	// Create a file in the some random directories
	file := "file43"
	h, err := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: filepath.Join("a", "b", "c", "d", file), Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	h, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: filepath.Join("a", "b", file), Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	h, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", file), Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	// Check directories are counted as non empty right now
	empty := suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a")})
	suite.assert.False(empty)

	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c", "d")})
	suite.assert.False(empty)

	// Validate one empty directory as well
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "e", "f")})
	suite.assert.True(empty)

	// Delete file from one of the directory and validate its empty now, but its parent is not empty
	err = suite.fileCache.DeleteFile(internal.DeleteFileOptions{Name: filepath.Join("a", "b", "c", "d", file)})
	suite.assert.Nil(err)

	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c", "d")})
	suite.assert.True(empty)
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("a", "b", "c")})
	suite.assert.False(empty)

	// Delete file only locally and not on remote and validate the directory is still not empty
	h, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", "n", file), Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.False(empty)

	os.Remove(filepath.Join(suite.cache_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.False(empty)
	os.Remove(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.True(empty)

	// Delete file only on remote and not on local and validate the directory is still not empty
	h, err = suite.fileCache.CreateFile(internal.CreateFileOptions{Name: filepath.Join("h", "l", "m", "n", file), Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	err = suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.False(empty)

	os.Remove(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.False(empty)
	os.Remove(filepath.Join(suite.cache_path, "h", "l", "m", "n", file))
	empty = suite.fileCache.IsDirEmpty(internal.IsDirEmptyOptions{Name: filepath.Join("h", "l", "m", "n")})
	suite.assert.True(empty)
}

func (suite *fileCacheTestSuite) createLocalDirectoryStructure() {
	err := os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "c", "d"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "f"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "a", "b", "e", "g"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.cache_path, "h", "l", "m", "n"), 0777)
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) createRemoteDirectoryStructure() {
	err := os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "c", "d"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "e", "f"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "a", "b", "e", "g"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "h", "i", "j", "k"), 0777)
	suite.assert.NoError(err)

	err = os.MkdirAll(filepath.Join(suite.fake_storage_path, "h", "l", "m", "n"), 0777)
	suite.assert.NoError(err)
}

func (suite *fileCacheTestSuite) TestHardLimit() {
	defer suite.cleanupTest()
	cacheTimeout := 0
	maxSizeMb := 2
	config := fmt.Sprintf("file_cache:\n  path: %s\n  max-size-mb: %d\n  timeout-sec: %d\n\nloopbackfs:\n  path: %s",
		suite.cache_path, maxSizeMb, cacheTimeout, suite.fake_storage_path)
	os.Mkdir(suite.cache_path, 0777)
	suite.setupTestHelper(config) // setup a new file cache with a custom config (teardown will occur after the test as usual)

	file := "file96"
	handle, _ := suite.fileCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 1024*1024)
	for i := int64(0); i < 5; i++ {
		suite.fileCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: i * 1024 * 1024, Data: data})
	}
	suite.fileCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(1)

	// Now try to open the file and validate we get an error due to hard limit
	handle, err := suite.fileCache.OpenFile(internal.OpenFileOptions{Name: file, Mode: 0777})
	suite.assert.NotNil(err)
	suite.assert.Nil(handle)
	suite.assert.Equal(err, syscall.ENOSPC)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFileCacheTestSuite(t *testing.T) {
	suite.Run(t, new(fileCacheTestSuite))
}
