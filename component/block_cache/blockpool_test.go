//go:build !authtest
// +build !authtest

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

package block_cache

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type blockpoolTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockpoolTestSuite) SetupTest() {
}

func (suite *blockpoolTestSuite) cleanupTest() {
}

func validateNullData(b *Block) bool {
	for i := 0; i < len(b.data); i++ {
		if b.data[i] != 0 {
			return false
		}
	}

	return true
}

func (suite *blockpoolTestSuite) TestAllocate() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(0, 0)
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 0)
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 1)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.NotNil(bp.resetBlockCh)
	suite.assert.NotNil(bp.zeroBlock)
	suite.assert.True(validateNullData(bp.zeroBlock))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.EqualValues(bp.maxBlocks, 1)
	suite.assert.EqualValues(bp.blockSize, 1)
	suite.assert.Equal(len(bp.zeroBlock.data), 0)
}

func (suite *blockpoolTestSuite) TestGetRelease() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.NotNil(bp.resetBlockCh)
	suite.assert.NotNil(bp.zeroBlock)
	suite.assert.Equal(len(bp.blocksCh), 4)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.True(validateNullData(bp.zeroBlock))

	b, err := bp.MustGet()
	suite.assert.Nil(err)
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 3)

	bp.Release(b)
	time.Sleep(1 * time.Second)
	suite.assert.Equal(len(bp.blocksCh), 4)

	b = bp.TryGet()
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 3)

	bp.Release(b)
	time.Sleep(1 * time.Second)
	suite.assert.Equal(len(bp.blocksCh), 4)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.Equal(len(bp.zeroBlock.data), 0)
}

func (suite *blockpoolTestSuite) TestUsage() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.NotNil(bp.resetBlockCh)
	suite.assert.NotNil(bp.zeroBlock)
	suite.assert.Equal(len(bp.blocksCh), 4)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.True(validateNullData(bp.zeroBlock))

	var blocks []*Block
	b, err := bp.MustGet()
	suite.assert.Nil(err)
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage := bp.Usage()
	suite.assert.Equal(usage, uint32(40))

	b = bp.TryGet()
	suite.assert.NotNil(b)
	blocks = append(blocks, b)

	usage = bp.Usage()
	suite.assert.Equal(usage, uint32(60))

	for _, blk := range blocks {
		bp.Release(blk)
	}

	// adding wait for the blocks to be reset and pushed back to the blocks channel
	time.Sleep(2 * time.Second)

	usage = bp.Usage()
	suite.assert.Equal(usage, uint32(20)) // because of zeroBlock

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.Equal(len(bp.zeroBlock.data), 0)
}

func (suite *blockpoolTestSuite) TestBufferExhaustion() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.NotNil(bp.resetBlockCh)
	suite.assert.NotNil(bp.zeroBlock)
	suite.assert.Equal(len(bp.blocksCh), 4)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.True(validateNullData(bp.zeroBlock))

	var blocks []*Block
	for i := 0; i < 4; i++ {
		b, err := bp.MustGet()
		suite.assert.Nil(err)
		suite.assert.NotNil(b)
		blocks = append(blocks, b)
	}

	usage := bp.Usage()
	suite.assert.Equal(usage, uint32(100))

	b := bp.TryGet()
	suite.assert.Nil(b)

	// MustGet should return nil as no blocks are available
	b, err := bp.MustGet()
	suite.assert.NotNil(err)
	suite.assert.Nil(b)

	for _, blk := range blocks {
		bp.Release(blk)
	}

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.Equal(len(bp.zeroBlock.data), 0)
}

// get n blocks
func getBlocks(suite *blockpoolTestSuite, bp *BlockPool, n int) []*Block {
	var blocks []*Block
	for i := 0; i < n; i++ {
		b := bp.TryGet()
		suite.assert.NotNil(b)

		// validate that the block has null data
		suite.assert.True(validateNullData(b))
		blocks = append(blocks, b)
	}
	return blocks
}

func releaseBlocks(suite *blockpoolTestSuite, bp *BlockPool, blocks []*Block) {
	for _, b := range blocks {
		b.data[0] = byte(rand.Int()%100 + 1)
		b.data[1] = byte(rand.Int()%100 + 1)

		// validate that the block being released does not have null data
		suite.assert.False(validateNullData(b))
		bp.Release(b)
	}
}

func (suite *blockpoolTestSuite) TestBlockReset() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(2, 10)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.NotNil(bp.priorityCh)
	suite.assert.NotNil(bp.resetBlockCh)
	suite.assert.NotNil(bp.zeroBlock)
	suite.assert.Equal(len(bp.blocksCh), 4)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.True(validateNullData(bp.zeroBlock))

	blocks := getBlocks(suite, bp, 4)

	releaseBlocks(suite, bp, blocks)

	// adding wait for the blocks to be reset and pushed back to the blocks channel
	time.Sleep(2 * time.Second)

	blocks = getBlocks(suite, bp, 4)

	releaseBlocks(suite, bp, blocks)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
	suite.assert.Equal(len(bp.priorityCh), 0)
	suite.assert.Equal(len(bp.resetBlockCh), 0)
	suite.assert.Equal(len(bp.zeroBlock.data), 0)
}

func TestBlockPoolSuite(t *testing.T) {
	suite.Run(t, new(blockpoolTestSuite))
}
