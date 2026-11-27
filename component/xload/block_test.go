// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockTestSuite) TestBlockAllocate() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(0)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid size")

	b, err = AllocateBlock(10)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.Data)

	err = b.Delete()
	suite.assert.NoError(err)
}

func (suite *blockTestSuite) TestBlockAllocateBig() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(100 * 1024 * 1024)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.Data)
	suite.assert.Equal(100*1024*1024, cap(b.Data))

	err = b.Delete()
	suite.assert.NoError(err)
}

func (suite *blockTestSuite) TestBlockAllocateHuge() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(50 * 1024 * 1024 * 1024)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "mmap error")
}

func (suite *blockTestSuite) TestBlockFreeNilData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.Data = nil

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid buffer")
}

func (suite *blockTestSuite) TestBlockFreeInvalidData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.Data = make([]byte, 1)

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid argument")
}

func (suite *blockTestSuite) TestBlockResuse() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.Index = 1

	b.ReUse()
	suite.assert.Equal(0, b.Index)

	err = b.Delete()
	suite.assert.NoError(err)
}

func TestBlockSuite(t *testing.T) {
	suite.Run(t, new(blockTestSuite))
}
