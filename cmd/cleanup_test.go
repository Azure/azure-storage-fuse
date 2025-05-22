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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CleanupTestSuite struct {
	suite.Suite
	testDir string
}

func (suite *CleanupTestSuite) SetupTest() {
	// Create a test directory
	suite.testDir = filepath.Join(os.TempDir(), "cleanup_test")
	os.RemoveAll(suite.testDir)
	os.MkdirAll(suite.testDir, 0755)
}

func (suite *CleanupTestSuite) TearDownTest() {
	os.RemoveAll(suite.testDir)
	config.ResetConfig()
}

func (suite *CleanupTestSuite) TestCleanupCachePath() {
	testPath := filepath.Join(suite.testDir, "cache")
	os.MkdirAll(testPath, 0755)

	// Create some test files
	testFile := filepath.Join(testPath, "testfile")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	assert.NoError(suite.T(), err)

	// Set up test component
	testComponent := "test_component"
	config.Set(testComponent+".path", testPath)
	
	// Test case 1: Global flag true, component flag false
	err = cleanupCachePath(testComponent, true)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), common.IsDirectoryEmpty(testPath))

	// Reset and create test files again
	err = os.WriteFile(testFile, []byte("test"), 0644)
	assert.NoError(suite.T(), err)

	// Test case 2: Global flag false, component flag true
	config.Set(testComponent+".cleanup-on-start", "true")
	err = cleanupCachePath(testComponent, false)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), common.IsDirectoryEmpty(testPath))
	
	// Reset and create test files again
	err = os.WriteFile(testFile, []byte("test"), 0644)
	assert.NoError(suite.T(), err)
	
	// Test case 3: Both flags false
	config.Set(testComponent+".cleanup-on-start", "false")
	err = cleanupCachePath(testComponent, false)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), common.IsDirectoryEmpty(testPath))
}

func TestCleanupTestSuite(t *testing.T) {
	suite.Run(t, new(CleanupTestSuite))
}