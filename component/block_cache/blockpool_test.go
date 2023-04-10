// +build !authtest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"testing"

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

func (suite *blockpoolTestSuite) TestAllocate() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(0, 0)
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 0)
	suite.assert.Nil(bp)

	bp = NewBlockPool(1, 1)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.Equal(bp.blockMax, uint32(1))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(0))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestResize() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(0))

	bp.ReSize(1, 10)
	suite.assert.Equal(bp.blockMax, uint32(10))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(5))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestExpand() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(0))

	for i := 0; i < 10; i++ {
		bp.expand()
	}

	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(5))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestGetRelease() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)
	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(0))

	b := bp.Get(true)
	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(1))
	suite.assert.NotNil(b)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 1)

	for i := 0; i < 10; i++ {
		bp.expand()
	}

	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(5))
	suite.assert.Equal(len(bp.blocksCh), 5)

	b = bp.Get(false)
	suite.assert.Equal(bp.blockMax, uint32(5))
	suite.assert.Equal(bp.blockSize, uint64(1))
	suite.assert.Equal(bp.blocks, uint32(5))
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 4)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 5)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func TestBlockPoolSuite(t *testing.T) {
	suite.Run(t, new(blockpoolTestSuite))
}
