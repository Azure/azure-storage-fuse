// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockpoolTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockpoolTestSuite) TestBlockPoolAllocate() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(0, 0, context.TODO())
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 0, context.TODO())
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 1, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Len(bp.blocksCh, 1)
	suite.assert.Empty(bp.priorityCh)
	suite.assert.EqualValues(1, bp.maxBlocks)
	suite.assert.EqualValues(1, bp.blockSize)

	bp.Terminate()
	suite.assert.Empty(bp.blocksCh)
	suite.assert.Empty(bp.priorityCh)
}

func (suite *blockpoolTestSuite) TestBlockPoolGetRelease() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Len(bp.blocksCh, 5)
	suite.assert.Empty(bp.priorityCh)
	suite.assert.EqualValues(5, bp.maxBlocks)
	suite.assert.EqualValues(1, bp.blockSize)

	b := bp.GetBlock(true)
	suite.assert.NotNil(b)
	suite.assert.Len(bp.blocksCh, 4)

	bp.Release(b)
	suite.assert.Len(bp.blocksCh, 5)

	b = bp.GetBlock(false)
	suite.assert.NotNil(b)
	suite.assert.Len(bp.blocksCh, 4)

	bp.Release(b)
	suite.assert.Len(bp.blocksCh, 5)

	bp.Terminate()
	suite.assert.Empty(bp.blocksCh)
}

func (suite *blockpoolTestSuite) TestBlockPoolUsage() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 10, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Len(bp.blocksCh, 9)
	suite.assert.Len(bp.priorityCh, 1)
	suite.assert.EqualValues(10, bp.maxBlocks)
	suite.assert.EqualValues(1, bp.blockSize)

	var blocks []*Block
	b := bp.GetBlock(true)
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage := bp.Usage()
	suite.assert.Equal(uint32(10), usage)

	b = bp.GetBlock(false)
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage = bp.Usage()
	suite.assert.Equal(uint32(20), usage)

	for _, blk := range blocks {
		bp.Release(blk)
	}

	usage = bp.Usage()
	suite.assert.Equal(uint32(0), usage)

	bp.Terminate()
	suite.assert.Empty(bp.blocksCh)
	suite.assert.Empty(bp.priorityCh)
}

func (suite *blockpoolTestSuite) TestBlockPoolBufferExhaution() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 10, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Len(bp.blocksCh, 9)
	suite.assert.Len(bp.priorityCh, 1)
	suite.assert.EqualValues(10, bp.maxBlocks)
	suite.assert.EqualValues(1, bp.blockSize)

	var blocks []*Block
	for range 10 {
		b := bp.GetBlock(true)
		suite.assert.NotNil(b)
		blocks = append(blocks, b)
	}

	usage := bp.Usage()
	suite.assert.Equal(uint32(100), usage)

	for _, blk := range blocks {
		bp.Release(blk)
	}

	bp.Terminate()
	suite.assert.Empty(bp.blocksCh)
	suite.assert.Empty(bp.priorityCh)
}

func TestBlockPoolSuite(t *testing.T) {
	suite.Run(t, new(blockpoolTestSuite))
}
