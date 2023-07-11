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

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestResize() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestGetRelease() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 5)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)

	b := bp.Get(true)
	suite.assert.NotNil(b)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 1)

	suite.assert.Equal(len(bp.blocksCh), 5)

	b = bp.Get(false)
	suite.assert.NotNil(b)
	suite.assert.Equal(len(bp.blocksCh), 4)

	bp.Release(b)
	suite.assert.Equal(len(bp.blocksCh), 5)

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func (suite *blockpoolTestSuite) TestAvailable() {
	suite.assert = assert.New(suite.T())

	bp := NewBlockPool(1, 10)
	suite.assert.NotNil(bp)
	suite.assert.NotNil(bp.blocksCh)

	avail := bp.Available(5)
	suite.assert.Equal(avail, uint32(5))

	b := make([]*Block, 10)
	suite.assert.NotNil(b)

	for i := 0; i < 10; i++ {
		b[i] = bp.Get(false)
		suite.assert.NotNil(b[i])
	}

	for i := 0; i < 10; i++ {
		bp.Release(b[i])
	}

	avail = bp.Available(5)
	suite.assert.Equal(avail, uint32(5))

	for i := 0; i < 8; i++ {
		b[i] = bp.Get(false)
		suite.assert.NotNil(b[i])
	}
	avail = bp.Available(5)
	suite.assert.Equal(avail, uint32(1))

	for i := 8; i < 10; i++ {
		b[i] = bp.Get(false)
		suite.assert.NotNil(b[i])
	}
	avail = bp.Available(5)
	suite.assert.Equal(avail, uint32(0))

	bp.Terminate()
	suite.assert.Equal(len(bp.blocksCh), 0)
}

func TestBlockPoolSuite(t *testing.T) {
	suite.Run(t, new(blockpoolTestSuite))
}
