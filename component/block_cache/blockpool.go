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
	"sync/atomic"
)

const _1MB uint32 = (1024 * 1024)

type BlockPool struct {
	blocksCh  chan *Block
	blockSize uint64
	blockMax  uint32
	blocks    int32
}

func NewBlockPool(blockSize uint64, memSize uint64) *BlockPool {
	if blockSize == 0 || memSize < blockSize {
		return nil
	}

	blockCount := memSize / blockSize

	pool := &BlockPool{
		blocksCh:  make(chan *Block, blockCount),
		blockSize: blockSize,
		blockMax:  uint32(blockCount),
	}
	
	for len(pool.blocksCh) < int(pool.blockMax) {
		pool.expand()
	}
	
	return pool
}

// Recalculate the block size and pool size
func (pool *BlockPool) Terminate() {
	close(pool.blocksCh)

	if atomic.LoadInt32(&pool.blocks) > 0 {
		for {
			b := <-pool.blocksCh
			if b == nil {
				break
			}
			_ = b.Delete()
		}
	}
}

// Get a Block from the pool
func (pool *BlockPool) expand() {
	if atomic.LoadInt32(&pool.blocks) < int32(pool.blockMax) {
		// Time to allocate a new Block
		b, err := AllocateBlock(pool.blockSize)
		if err != nil {
			return
		}

		atomic.AddInt32(&pool.blocks, 1)
		pool.blocksCh <- b
	}
}

// Get a Block from the pool
func (pool *BlockPool) Get() *Block {
	var b *Block

	select {
	// Check if there is a buffer already available in the pool
	case b = <-pool.blocksCh:
		break
	default:
		// If not available try to allocate a new buffer and add to pool if possible
		pool.expand()
	
		// Caller is ready to wait so block untill buffer is available
		b = <-pool.blocksCh
	}

	b.ReUse()
	return b
}

// Release back the Block to the pool
func (pool *BlockPool) Release(b *Block) {
	atomic.AddInt32(&pool.blocks, -1)
	pool.blocksCh <- b
}
