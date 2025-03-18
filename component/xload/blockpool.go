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
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// BlockPool is a pool of Blocks
type BlockPool struct {
	blocks *sync.Pool

	// Size of each block this pool holds
	blockSize uint64
}

// NewBlockPool allocates a new pool of blocks
func NewBlockPool(blockSize uint64, blockCount uint32) *BlockPool {
	// Ignore if config is invalid
	if blockSize == 0 || blockCount == 0 {
		log.Err("BlockPool::NewBlockPool : blockSize : %v, block count : %v", blockSize, blockCount)
		return nil
	}

	return &BlockPool{
		blockSize: blockSize,
		blocks: &sync.Pool{
			New: func() interface{} {
				return &Block{
					Data: make([]byte, blockSize),
				}
			},
		},
	}
}

// Terminate ends the block pool life
func (pool *BlockPool) Terminate() {
}

// Usage provides % usage of this block pool
func (pool *BlockPool) Usage() uint32 {
	return 0
}

func (pool *BlockPool) GetBlockSize() uint64 {
	return pool.blockSize
}

func (pool *BlockPool) GetBlock(priority bool) *Block {
	block := pool.blocks.Get().(*Block)
	block.ReUse()
	return block
}

// Release back the Block to the pool
func (pool *BlockPool) Release(block *Block) {
	pool.blocks.Put(block)
}
