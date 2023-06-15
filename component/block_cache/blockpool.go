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

const _1MB uint64 = (1024 * 1024)

type BlockPool struct {
	blocksCh  chan *Block
	blockSize uint64
	blockMax  uint32
	blocks    uint32
}

func NewBlockPool(blockSize uint64, memSize uint64) *BlockPool {
	if blockSize == 0 || memSize < blockSize {
		log.Err("blockpool::NewBlockPool : blockSize : %v, memsize: %v", blockSize, memSize)
		return nil
	}

	blockCount := memSize / blockSize

	return &BlockPool{
		blocksCh:  make(chan *Block, blockCount),
		blockSize: blockSize,
		blockMax:  uint32(blockCount),
		blocks:    0,
	}
}

// Available returns back how many of requested blocks can be made available approx
func (pool *BlockPool) Available(cnt uint32) uint32 {
	// Calculate how much is possible at max to allocate and provide
	possible := pool.blockMax - pool.blocks
	possible += uint32(len(pool.blocksCh))

	percentAvailable := (possible * 100) / pool.blockMax
	if percentAvailable > 70 && possible > cnt {
		return cnt
	}

	avail := (cnt * percentAvailable) / 100

	if avail >= possible {
		return 0
	}

	return avail
}

// Recalculate the block size and pool size
func (pool *BlockPool) Terminate() {
	close(pool.blocksCh)

	if pool.blocks > 0 {
		for {
			b := <-pool.blocksCh
			if b == nil {
				break
			}
			_ = b.Delete()
		}
	}
}

// Recalculate the block size and pool size
func (pool *BlockPool) ReSize(blockSize uint64, memSize uint64) {
	blockCount := memSize / blockSize
	pool.blockMax = uint32(blockCount)

	for pool.blocks < pool.blockMax/2 {
		pool.expand()
	}
}

// Get a Block from the pool
func (pool *BlockPool) expand() {
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

// Get a Block from the pool
func (pool *BlockPool) Get(wait bool) *Block {
	var b *Block

	select {
	// Check if there is a buffer already available in the pool
	case b = <-pool.blocksCh:
		break
	default:
		// If not available try to allocate a new buffer and add to pool if possible
		pool.expand()
		if !wait {
			// Caller asked for immediate answer so even after expanding if its not possible return nil
			select {
			case b = <-pool.blocksCh:
				break
			default:
				return nil
			}
		} else {
			// Caller is ready to wait so block untill buffer is available
			b = <-pool.blocksCh
		}
	}

	b.ReUse()
	return b
}

// Release back the Block to the pool
func (pool *BlockPool) Release(b *Block) {
	// This goes to the first Block channel
	if pool.blocks > pool.blockMax {
		pool.blocks--
		_ = b.Delete()
		return
	}

	pool.blocksCh <- b
}
