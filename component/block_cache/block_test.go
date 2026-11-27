//go:build !authtest
// +build !authtest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockTestSuite) SetupTest() {
}

func (suite *blockTestSuite) CleanupTest() {
}

func (suite *blockTestSuite) TestAllocate() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(0)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid size")

	b, err = AllocateBlock(10)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.data)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestAllocateBig() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(100 * 1024 * 1024)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.NotNil(b.data)
	suite.assert.Equal(100*1024*1024, cap(b.data))

	err = b.Delete()
	suite.assert.NoError(err)
}

func (suite *blockTestSuite) TestAllocateHuge() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(50 * 1024 * 1024 * 1024)
	suite.assert.Nil(b)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "mmap error")
}

func (suite *blockTestSuite) TestFreeNilData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.data = nil

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid buffer")
}

func (suite *blockTestSuite) TestFreeInvalidData() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	b.data = make([]byte, 1)

	err = b.Delete()
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid argument")
}

func (suite *blockTestSuite) TestResuse() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestReady() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestUnBlock() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	<-b.state
	suite.assert.Empty(b.state)

	_ = b.Delete()
}

func (suite *blockTestSuite) TestWriter() {
	suite.assert = assert.New(suite.T())

	b, err := AllocateBlock(1)
	suite.assert.NotNil(b)
	suite.assert.NoError(err)
	suite.assert.Nil(b.state)
	suite.assert.Nil(b.node)
	suite.assert.False(b.IsDirty())

	b.ReUse()
	suite.assert.NotNil(b.state)
	suite.assert.Nil(b.node)
	suite.assert.Zero(b.offset)
	suite.assert.Equal(int64(-1), b.id)
	suite.assert.False(b.IsDirty())

	b.Ready(BlockStatusDownloaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	b.Uploading()
	suite.assert.NotNil(b.state)

	b.Dirty()
	suite.assert.True(b.IsDirty())

	b.Failed()
	suite.assert.True(b.IsDirty())

	b.NoMoreDirty()
	suite.assert.False(b.IsDirty())

	b.Ready(BlockStatusUploaded)
	suite.assert.Len(b.state, 1)

	<-b.state
	suite.assert.Empty(b.state)

	b.Unblock()
	suite.assert.NotNil(b.state)
	suite.assert.Empty(b.state)

	<-b.state
	suite.assert.Empty(b.state)

	_ = b.Delete()
}

func TestBlockSuite(t *testing.T) {
	suite.Run(t, new(blockTestSuite))
}
