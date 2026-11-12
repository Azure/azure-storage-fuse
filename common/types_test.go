// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type typesTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *typesTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestGenerateConfig(t *testing.T) {
	suite.Run(t, new(typesTestSuite))
}

func (suite *typesTestSuite) TestBinarySearch() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	found, startingIndex := bol.BinarySearch(5)
	suite.assert.True(found)
	suite.assert.Equal(1, startingIndex)

	found, startingIndex = bol.BinarySearch(20)
	suite.assert.False(found)
	suite.assert.Equal(3, startingIndex)
}

func (suite *typesTestSuite) TestFindBlocksToModify() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	index, size, largerThanFile, _ := bol.FindBlocksToModify(3, 7)
	suite.assert.Equal(0, index)
	suite.assert.Equal(int64(12), size)
	suite.assert.False(largerThanFile)

	index, size, largerThanFile, _ = bol.FindBlocksToModify(8, 10)
	suite.assert.Equal(2, index)
	suite.assert.Equal(int64(5), size)
	suite.assert.True(largerThanFile)

	_, size, largerThanFile, appendOnly := bol.FindBlocksToModify(20, 20)
	suite.assert.Equal(int64(0), size)
	suite.assert.True(largerThanFile)
	suite.assert.True(appendOnly)
}

func (suite *typesTestSuite) TestDefaultWorkDir() {
	val, err := os.UserHomeDir()
	suite.assert.NoError(err)
	suite.assert.Equal(DefaultWorkDir, filepath.Join(val, ".blobfuse2"))
	suite.assert.Equal(DefaultLogFilePath, filepath.Join(val, ".blobfuse2/blobfuse2.log"))
	suite.assert.Equal(StatsConfigFilePath, filepath.Join(val, ".blobfuse2/stats_monitor.cfg"))
}
