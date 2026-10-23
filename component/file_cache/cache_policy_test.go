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

package file_cache

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cachePolicyTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *cachePolicyTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
	suite.assert = assert.New(suite.T())
	err = os.Mkdir(cache_path, fs.FileMode(0777))
	suite.assert.NoError(err)
}

func (suite *cachePolicyTestSuite) cleanupTest() {
	os.RemoveAll(cache_path)
}

func (suite *cachePolicyTestSuite) TestGetUsage() {
	defer suite.cleanupTest()
	f, _ := os.Create(cache_path + "/test")
	data := make([]byte, 1024*1024)
	_, err := f.Write(data)
	suite.assert.NoError(err)
	result, _ := common.GetUsage(cache_path)
	suite.assert.InEpsilon(float64(1), math.Floor(result), 0.1)
	f.Close()
}

func (suite *cachePolicyTestSuite) TestGetUsagePercentage() {
	defer suite.cleanupTest()
	data := make([]byte, 1024*1024)

	f, _ := os.Create(cache_path + "/test")
	_, err := f.Write(data)
	suite.assert.NoError(err)
	result := getUsagePercentage(cache_path, 4)
	// since the value might defer a little distro to distro
	suite.assert.GreaterOrEqual(result, float64(25))
	suite.assert.LessOrEqual(result, float64(30))
	f.Close()

	result = getUsagePercentage("/", 0)
	// since the value might defer a little distro to distro
	suite.assert.GreaterOrEqual(result, float64(0))
	suite.assert.LessOrEqual(result, float64(90))
}

func (suite *cachePolicyTestSuite) TestDeleteFile() {
	defer suite.cleanupTest()
	f, _ := os.Create(cache_path + "/test")
	result := deleteFile(f.Name() + "not_exist")
	suite.assert.NoError(result)
	f.Close()
}

// mkdirs creates the given directories under cache_path.
func (suite *cachePolicyTestSuite) mkdirs(dirs ...string) {
	for _, dir := range dirs {
		suite.Require().NoError(os.MkdirAll(filepath.Join(cache_path, dir), 0777))
	}
}

func (suite *cachePolicyTestSuite) requireExists(rel string) {
	_, err := os.Stat(filepath.Join(cache_path, rel))
	suite.Require().NoError(err, "%s should exist", rel)
}

func (suite *cachePolicyTestSuite) requireRemoved(rel string) {
	_, err := os.Stat(filepath.Join(cache_path, rel))
	suite.Require().True(os.IsNotExist(err), "%s should be removed", rel)
}

// All the empty directories above a deleted file are removed, but never the cache directory itself.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParents() {
	defer suite.cleanupTest()
	suite.mkdirs("a/b/c")

	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/b/c/file"))

	suite.requireRemoved("a")
	suite.requireExists("")
}

// Removal stops at the first directory that still holds a file.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsStopsAtFile() {
	defer suite.cleanupTest()
	suite.mkdirs("a/b/c")
	suite.Require().NoError(os.WriteFile(filepath.Join(cache_path, "a/keep"), []byte("data"), 0666))

	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/b/c/file"))

	suite.requireRemoved("a/b")
	suite.requireExists("a/keep")
}

// Removal stops at the first directory that still holds another directory, which is left alone.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsStopsAtDirectory() {
	defer suite.cleanupTest()
	suite.mkdirs("a/b/c", "a/b/other")

	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/b/c/file"))

	suite.requireRemoved("a/b/c")
	suite.requireExists("a/b/other")
}

func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsKeepsCacheDirectory() {
	defer suite.cleanupTest()

	removeEmptyParents(cache_path, filepath.Join(cache_path, "file"))

	suite.requireExists("")
}

// A directory that is already gone does not stop the removal of its empty parents.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsSkipsMissingDirectory() {
	defer suite.cleanupTest()
	suite.mkdirs("a")

	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/missing/file"))

	suite.requireRemoved("a")
	suite.requireExists("")

	// Directories below the missing one are gone too.
	suite.mkdirs("a")
	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/missing/deeper/file"))

	suite.requireRemoved("a")
	suite.requireExists("")
}

// Nothing outside the cache directory is removed, including a sibling whose name starts with the cache path.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsOutsideCacheDirectory() {
	defer suite.cleanupTest()
	sibling := cache_path + "_sibling"
	suite.Require().NoError(os.MkdirAll(filepath.Join(sibling, "a"), 0777))
	defer os.RemoveAll(sibling)

	removeEmptyParents(cache_path, filepath.Join(sibling, "a", "file"))

	_, err := os.Stat(filepath.Join(sibling, "a"))
	suite.NoError(err)
}

// A name that is not in canonical form, with ".." or repeated separators, is cleaned first. Directories it resolves
// to inside the cache are removed, and nothing outside the cache is, even if the name starts with the cache path.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsNonCanonicalName() {
	defer suite.cleanupTest()
	outside := filepath.Join(filepath.Dir(cache_path), "outside_"+filepath.Base(cache_path))
	suite.Require().NoError(os.MkdirAll(filepath.Join(outside, "x"), 0777))
	defer os.RemoveAll(outside)
	suite.mkdirs("a/c", "a/b")

	// Resolves to <cache>/a/c/file.
	removeEmptyParents(cache_path, cache_path+"//a/b/../c//file")
	suite.requireRemoved("a/c")
	suite.requireExists("a/b")

	// Resolves to a directory next to the cache.
	removeEmptyParents(cache_path, cache_path+"/../"+filepath.Base(outside)+"/x/file")
	_, err := os.Stat(filepath.Join(outside, "x"))
	suite.NoError(err, "a directory outside the cache was removed")

	// Resolves to the cache directory itself.
	removeEmptyParents(cache_path, cache_path+"/a/b/../../file")
	suite.requireExists("")
}

// The cache path may end with a separator or be in non-canonical form. The cache directory is never removed.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsNonCanonicalCachePath() {
	defer suite.cleanupTest()
	suite.mkdirs("a/b")

	removeEmptyParents(cache_path+"/", filepath.Join(cache_path, "a/b/file"))
	suite.requireRemoved("a")
	suite.requireExists("")

	suite.mkdirs("a")
	removeEmptyParents(cache_path+"//.", filepath.Join(cache_path, "a/file"))
	suite.requireRemoved("a")
	suite.requireExists("")
}

// rmdir follows symlinks in the middle of a path. A symlink in the cache, for example left in a reused cache directory,
// must not let the removal reach the directories it points to.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsDoesNotFollowSymlinks() {
	defer suite.cleanupTest()
	outside := filepath.Join(filepath.Dir(cache_path), "outside_"+filepath.Base(cache_path))
	suite.Require().NoError(os.MkdirAll(filepath.Join(outside, "child", "grandchild"), 0777))
	defer os.RemoveAll(outside)
	suite.mkdirs("a")
	suite.Require().NoError(os.Symlink(outside, filepath.Join(cache_path, "link")))
	suite.Require().NoError(os.Symlink(outside, filepath.Join(cache_path, "a", "link")))
	suite.Require().NoError(os.Symlink(filepath.Join(outside, "child", "grandchild"), filepath.Join(cache_path, "last")))

	removeEmptyParents(cache_path, filepath.Join(cache_path, "link/child/grandchild/file"))
	removeEmptyParents(cache_path, filepath.Join(cache_path, "a/link/child/grandchild/file"))
	removeEmptyParents(cache_path, filepath.Join(cache_path, "last/file"))

	_, err := os.Stat(filepath.Join(outside, "child", "grandchild"))
	suite.NoError(err, "a directory outside the cache was removed through a symlink")
	for _, link := range []string{"link", "a/link", "last"} {
		_, err := os.Lstat(filepath.Join(cache_path, link))
		suite.NoError(err, "%s should exist", link)
	}
}

// The cache path itself may be a symlink to the cache directory.
func (suite *cachePolicyTestSuite) TestRemoveEmptyParentsSymlinkedCachePath() {
	defer suite.cleanupTest()
	link := cache_path + "_link"
	suite.Require().NoError(os.Symlink(cache_path, link))
	defer os.Remove(link)
	suite.mkdirs("a/b")

	removeEmptyParents(link, filepath.Join(link, "a/b/file"))

	suite.requireRemoved("a")
	suite.requireExists("")
	_, err := os.Lstat(link)
	suite.NoError(err)
}

func TestCachePolicyTestSuite(t *testing.T) {
	suite.Run(t, new(cachePolicyTestSuite))
}
