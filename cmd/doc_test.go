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
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type docTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *docTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	docCmdInput = struct{ outputLocation string }{}
	options = mountOptions{}
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *docTestSuite) cleanupTest() {
	resetCLIFlags(*docCmd)
}

func (suite *docTestSuite) TestDocsGeneration() {
	defer suite.cleanupTest()

	opDir := "/tmp/docs_" + randomString(6)
	defer os.RemoveAll(opDir)

	_, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.Nil(err)
	files, err := os.ReadDir(opDir)
	suite.assert.Nil(err)
	suite.assert.NotZero(len(files))
}

func (suite *docTestSuite) TestOutputDirCreationError() {
	defer suite.cleanupTest()

	opDir := "/var/docs_" + randomString(6)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "failed to create output location")
}

func (suite *docTestSuite) TestDocsGenerationError() {
	defer suite.cleanupTest()

	opDir := "/var"

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "cannot generate command tree")
}

func (suite *docTestSuite) TestOutputDirIsFileError() {
	defer suite.cleanupTest()

	opFile, err := os.CreateTemp("", "docfile*")
	suite.assert.Nil(err)
	opFileName := opFile.Name()
	opFile.Close()
	defer os.Remove(opFileName)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opFileName))
	suite.assert.NotNil(err)
	suite.assert.Contains(op, "output location is invalid as it is pointing to a file")
}

func TestDocCommand(t *testing.T) {
	suite.Run(t, new(docTestSuite))
}
