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

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type genConfig struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *genConfig) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestGenConfig(t *testing.T) {
	suite.Run(t, new(genConfig))
}

func (suite *genConfig) cleanupTest() {
	os.Remove("../generatedConfig.yaml")
}

func (suite *genConfig) TestFileCacheConfigGen() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "file_cache"), fmt.Sprintf("--tmp-path=%s", tempDir))
	_, err := cmd.Output()
	suite.assert.Nil(err)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists("../generatedConfig.yaml")

	//check if the generated file is not empty
	file, err := os.ReadFile("../generatedConfig.yaml")
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.Contains(string(file), tempDir)
}

func (suite *genConfig) TestBlockCacheConfigGen() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "block_cache"), fmt.Sprintf("--tmp-path=%s", tempDir))
	_, err := cmd.Output()
	suite.assert.Nil(err)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists("../generatedConfig.yaml")

	//check if the generated file is not empty
	file, err := os.ReadFile("../generatedConfig.yaml")
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "block_cache")

	//check if the generated file has the correct temp path
	suite.assert.Contains(string(file), tempDir)

	cmd = exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "block_cache"))
	_, err = cmd.Output()
	suite.assert.Nil(err)

	file, err = os.ReadFile("../generatedConfig.yaml")
	suite.assert.Nil(err)
	//check if the generated file has no tmp path
	suite.assert.NotContains(string(file), tempDir)
}

// test direct io flag
func (suite *genConfig) TestDirectIOConfigGen() {
	defer suite.cleanupTest()

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "block_cache"), "--direct-io")

	_, err := cmd.Output()
	suite.assert.Nil(err)

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists("../generatedConfig.yaml")

	//check if the generated file is not empty
	file, err := os.ReadFile("../generatedConfig.yaml")
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct direct io flag
	suite.assert.Contains(string(file), "direct-io: true")
}

func (suite *genConfig) TestInvalidComponent() {
	defer suite.cleanupTest()

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "invalid_component"))
	_, err := cmd.Output()
	suite.assert.NotNil(err)
}

func (suite *genConfig) TestInvalidTempPath() {
	defer suite.cleanupTest()

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "file_cache"))
	_, err := cmd.Output()
	suite.assert.NotNil(err)
}

func (suite *genConfig) TestInvalidComponentAndTempPath() {
	defer suite.cleanupTest()

	cmd := exec.Command("../blobfuse2", "gen-config")
	_, err := cmd.Output()
	suite.assert.NotNil(err)
}

func (suite *genConfig) TestInvalidComponentAndValidTempPath() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--tmp-path=%s", tempDir))
	_, err := cmd.Output()
	suite.assert.NotNil(err)
}

func (suite *genConfig) TestValidComponentAndInvalidTempPath() {
	defer suite.cleanupTest()

	cmd := exec.Command("../blobfuse2", "gen-config", fmt.Sprintf("--component=%s", "file_cache"))
	_, err := cmd.Output()
	suite.assert.NotNil(err)
}
