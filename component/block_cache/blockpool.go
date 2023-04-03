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

import "github.com/Azure/azure-storage-fuse/v2/common/log"

const _1MB uint32 = (1024 * 1024)

type BlockPool struct {
	firstBlockCh chan *Block
	blocksCh     chan *Block

	firstBlockSize uint64
	blockSize      uint64

	firstBlockMax uint32
	blockMax      uint32

	firstBlocks uint32
	blocks      uint32
}

func NewBlockPool(blockSize uint64, memSize uint64) *BlockPool {
	firstBlockCount := (memSize * 20 / 100) / (blockSize * 8)
	blockCount := (memSize * 80 / 100) / (blockSize)

	log.Info("BlockPool : %v blocks of %v size, %v blocks of %v size", firstBlockCount, (blockSize * 8), blockCount, blockSize)

	return &BlockPool{
		firstBlockCh:   make(chan *Block, firstBlockCount),
		firstBlockSize: blockSize * 8,
		firstBlockMax:  uint32(firstBlockCount),
		firstBlocks:    0,

		blocksCh:  make(chan *Block, blockCount),
		blockSize: blockSize,
		blockMax:  uint32(blockCount),
		blocks:    0,
	}
}

// Get a Block from the pool
func (pool *BlockPool) expand(first bool) {
	if first {
		if pool.firstBlocks < pool.firstBlockMax {
			// Time to allocate a new Block
			b, err := AllocateBlock(pool.firstBlockSize)
			if err != nil {
				return
			}

			pool.firstBlocks++
			pool.firstBlockCh <- b
			return
		}
	} else {
		if pool.blocks < pool.blockMax {
			// Time to allocate a new Block
			b, err := AllocateBlock(pool.blockSize)
			if err != nil {
				return
			}

			pool.blocks++
			pool.blocksCh <- b
			return
		}
	}
}

// Get a Block from the pool
func (pool *BlockPool) Get(first bool) *Block {
	var b *Block

	if first && pool.firstBlockMax > 0 {
		select {
		case b = <-pool.firstBlockCh:
		default:
			pool.expand(first)
			b = <-pool.firstBlockCh
		}
	} else {
		select {
		case b = <-pool.blocksCh:
		default:
			pool.expand(first)
			b = <-pool.blocksCh
		}
	}

	b.ReUse()
	return b
}

// Release back the Block to the pool
func (pool *BlockPool) Release(b *Block) {
	if b.Size() > pool.blockSize {
		// This goes to the first Block channel
		if pool.firstBlocks > pool.firstBlockMax {
			pool.firstBlocks--
			b.Delete()
			return
		}

		pool.firstBlockCh <- b
	} else {
		// This goes to the first Block channel
		if pool.blocks > pool.blockMax {
			pool.blocks--
			b.Delete()
			return
		}

		pool.blocksCh <- b
	}
}
