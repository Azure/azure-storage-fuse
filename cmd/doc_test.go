// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
	suite.assert.NoError(err)
	files, err := os.ReadDir(opDir)
	suite.assert.NoError(err)
	suite.assert.NotEmpty(files)
}

func (suite *docTestSuite) TestOutputDirCreationError() {
	defer suite.cleanupTest()

	opDir := "/var/docs_" + randomString(6)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.Error(err)
	suite.assert.Contains(op, "failed to create output location")
}

func (suite *docTestSuite) TestDocsGenerationError() {
	defer suite.cleanupTest()

	opDir := "/var"

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opDir))
	suite.assert.Error(err)
	suite.assert.Contains(op, "cannot generate command tree")
}

func (suite *docTestSuite) TestOutputDirIsFileError() {
	defer suite.cleanupTest()

	opFile, err := os.CreateTemp("", "docfile*")
	suite.assert.NoError(err)
	opFileName := opFile.Name()
	opFile.Close()
	defer os.Remove(opFileName)

	op, err := executeCommandC(rootCmd, "doc", fmt.Sprintf("--output-location=%s", opFileName))
	suite.assert.Error(err)
	suite.assert.Contains(op, "output location is invalid as it is pointing to a file")
}

func TestDocCommand(t *testing.T) {
	suite.Run(t, new(docTestSuite))
}
