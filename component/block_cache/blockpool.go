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
	maxBlocks uint32
}

func NewBlockPool(blockSize uint64, memSize uint64) *BlockPool {
	if blockSize == 0 || memSize < blockSize {
		log.Err("blockpool::NewBlockPool : blockSize : %v, memsize: %v", blockSize, memSize)
		return nil
	}

	blockCount := uint32(memSize / blockSize)

	pool := &BlockPool{
		blocksCh:  make(chan *Block, blockCount),
		maxBlocks: uint32(blockCount),
	}

	for i := (uint32)(0); i < blockCount; i++ {
		b, err := AllocateBlock(blockSize)
		if err != nil {
			return nil
		}

		pool.blocksCh <- b
	}

	return pool
}

func (pool *BlockPool) Usage() uint32 {
	return ((pool.maxBlocks - (uint32)(len(pool.blocksCh))) * 100) / pool.maxBlocks
}

// Available returns back how many of requested blocks can be made available approx
func (pool *BlockPool) Available(cnt uint32) uint32 {
	// Calculate how much is possible at max to allocate and provide
	possible := (uint32)(len(pool.blocksCh))

	percentAvailable := (possible * 100) / pool.maxBlocks
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

	for {
		b := <-pool.blocksCh
		_ = b.Delete()
	}
}

// Must Get a Block from the pool, wait untill something is free
func (pool *BlockPool) MustGet() *Block {
	b := <-pool.blocksCh
	b.ReUse()
	return b
}

// Must Get a Block from the pool, wait untill something is free
func (pool *BlockPool) TryGet() *Block {
	var b *Block

	select {
	case b = <-pool.blocksCh:
		break
	default:
		return nil
	}

	b.ReUse()
	return b
}

// Release back the Block to the pool
func (pool *BlockPool) Release(b *Block) {
	// This goes to the first Block channel
	pool.blocksCh <- b
}
