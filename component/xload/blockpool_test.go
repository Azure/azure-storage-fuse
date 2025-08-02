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
	suite.assert.Equal(len(bp.blocksCh), 1)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.EqualValues(bp.maxBlocks, 1)
	suite.assert.EqualValues(bp.blockSize, 1)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
}

func (suite *blockpoolTestSuite) TestBlockPoolGetRelease() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Equal(len(bp.blocksCh), 5)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.EqualValues(bp.maxBlocks, 5)
	suite.assert.EqualValues(bp.blockSize, 1)

	b := bp.GetBlock(true)
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 4)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 5)

	b = bp.GetBlock(false)
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 4)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 5)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestBlockPoolUsage() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 10, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Equal(len(bp.blocksCh), 9)
	suite.assert.Equal(len(bp.priorityCh), 1)
	suite.assert.EqualValues(bp.maxBlocks, 10)
	suite.assert.EqualValues(bp.blockSize, 1)

	var blocks []*Block
	b := bp.GetBlock(true)
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage := bp.Usage()
	suite.assert.Equal(usage, uint32(10))

	b = bp.GetBlock(false)
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage = bp.Usage()
	suite.assert.Equal(usage, uint32(20))

	for _, blk := range blocks {
		bp.Release(blk)
	}

	usage = bp.Usage()
	suite.assert.Equal(usage, uint32(0))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
}

func (suite *blockpoolTestSuite) TestBlockPoolBufferExhaution() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 10, context.TODO())
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.Equal(len(bp.blocksCh), 9)
	suite.assert.Equal(len(bp.priorityCh), 1)
	suite.assert.EqualValues(bp.maxBlocks, 10)
	suite.assert.EqualValues(bp.blockSize, 1)

	var blocks []*Block
	for range 10 {
		b := bp.GetBlock(true)
		suite.assert.NotNil(b)
		blocks = append(blocks, b)
	}

	usage := bp.Usage()
	suite.assert.Equal(usage, uint32(100))

	for _, blk := range blocks {
		bp.Release(blk)
	}

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
}

func TestBlockPoolSuite(t *testing.T) {
	suite.Run(t, new(blockpoolTestSuite))
}
