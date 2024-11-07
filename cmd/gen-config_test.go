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

func (suite *genConfig) cleanupTest() {
	os.Remove(suite.getDefaultLogLocation())
	optsGenCfg = genConfigParams{}
}

func (suite *genConfig) getDefaultLogLocation() string {
	var homeDir, err = os.UserHomeDir()
	suite.assert.Nil(err)
	var logFilePath = homeDir + "/.blobfuse2/generatedConfig.yaml"
	return logFilePath
}

func (suite *genConfig) TestNoTempPath() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config")
	suite.assert.NotNil(err)
}

func (suite *genConfig) TestFileCacheConfigGen() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	_, err := executeCommandC(rootCmd, "gen-config", fmt.Sprintf("--tmp-path=%s", tempDir))
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
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

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache", fmt.Sprintf("--tmp-path=%s", tempDir))
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "block_cache")
	suite.assert.NotContains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.Contains(string(file), tempDir)
}

func (suite *genConfig) TestBlockCacheConfigGen1() {
	defer suite.cleanupTest()

	tempDir, _ := os.MkdirTemp("", "TestTempDir")
	os.MkdirAll(tempDir, 0777)
	defer os.RemoveAll(tempDir)

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache")
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()

	//Check if a file is generated named generatedConfig.yaml
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct component
	suite.assert.Contains(string(file), "block_cache")
	suite.assert.NotContains(string(file), "file_cache")

	//check if the generated file has the correct temp path
	suite.assert.NotContains(string(file), tempDir)
}

// test direct io flag
func (suite *genConfig) TestDirectIOConfigGen() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache", "--direct-io")
	suite.assert.Nil(err)

	logFilePath := suite.getDefaultLogLocation()
	suite.assert.FileExists(logFilePath)

	//check if the generated file is not empty
	file, err := os.ReadFile(logFilePath)
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct direct io flag
	suite.assert.Contains(string(file), "direct-io: true")
	suite.assert.NotContains(string(file), " path: ")
}

func (suite *genConfig) TestOutputFile() {
	defer suite.cleanupTest()

	_, err := executeCommandC(rootCmd, "gen-config", "--block-cache", "--direct-io", "--o", "1.yml")
	suite.assert.Nil(err)

	//check if the generated file is not empty
	file, err := os.ReadFile("1.yml")
	suite.assert.Nil(err)
	suite.assert.NotEmpty(file)

	//check if the generated file has the correct direct io flag
	suite.assert.Contains(string(file), "direct-io: true")
	suite.assert.NotContains(string(file), " path: ")
}

func (suite *genConfig) TestConsoleOutput() {
	defer suite.cleanupTest()

	op, err := executeCommandC(rootCmd, "gen-config", "--block-cache", "--direct-io", "--o", "console")
	suite.assert.Nil(err)

	//check if the generated file has the correct direct io flag
	suite.assert.Empty(op)
}

func TestGenConfig(t *testing.T) {
	suite.Run(t, new(genConfig))
}
