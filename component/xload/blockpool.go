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
	"fmt"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// BlockPool is a pool of Blocks
type BlockPool struct {
	// Channel holding free blocks
	blocksCh chan *Block

	// Channel holding free blocks for priority threads
	priorityCh chan *Block

	// Size of each block this pool holds
	blockSize uint64

	// Number of block that this pool can handle at max
	maxBlocks uint32

	// Number of threads waiting for block
	waitLength atomic.Int32

	// Context to cancel new block allocation
	ctx context.Context
}

// NewBlockPool allocates a new pool of blocks
func NewBlockPool(blockSize uint64, blockCount uint32, ctx context.Context) *BlockPool {
	// Ignore if config is invalid
	if blockSize == 0 || blockCount == 0 {
		log.Err("BlockPool::NewBlockPool : blockSize : %v, block count : %v", blockSize, blockCount)
		return nil
	}

	highPriority := (blockCount * 10) / 100

	pool := &BlockPool{
		blocksCh:   make(chan *Block, blockCount-highPriority),
		priorityCh: make(chan *Block, highPriority),
		maxBlocks:  uint32(blockCount),
		blockSize:  blockSize,
		ctx:        ctx,
	}

	pool.waitLength.Store(0)

	// Preallocate all blocks so that during runtime we do not spend CPU cycles on this
	for i := range blockCount {
		block, err := AllocateBlock(blockSize)
		if err != nil {
			log.Err("BlockPool::NewBlockPool : unable to allocate block [%s]", err.Error())
			return nil
		}

		if i < highPriority {
			pool.priorityCh <- block
		} else {
			pool.blocksCh <- block
		}
	}

	return pool
}

// Terminate ends the block pool life
func (pool *BlockPool) Terminate() {
	close(pool.blocksCh)
	close(pool.priorityCh)

	releaseBlocks(pool.blocksCh)
	releaseBlocks(pool.priorityCh)
}

// release back the memory allocated to each block
func releaseBlocks(ch chan *Block) {
	for {
		block := <-ch
		if block == nil {
			break
		}
		_ = block.Delete()
	}
}

// Usage provides % usage of this block pool
func (pool *BlockPool) Usage() uint32 {
	return ((pool.maxBlocks - (uint32)(len(pool.blocksCh)+len(pool.priorityCh))) * 100) / pool.maxBlocks
}

func (pool *BlockPool) GetUsageDetails() (uint32, uint32, uint32, int32) {
	return pool.maxBlocks, uint32(len(pool.priorityCh)), uint32(len(pool.blocksCh)), pool.waitLength.Load()
}

func (pool *BlockPool) GetBlockSize() uint64 {
	return pool.blockSize
}

func (pool *BlockPool) GetBlock(priority bool) *Block {
	// increment an atomic variable to track the number of blocks in use
	pool.waitLength.Add(1)
	defer pool.waitLength.Add(-1)

	if priority {
		return pool.mustGet()
	} else {
		return pool.tryGet()
	}

}

// TryGet a block from the pool. If the pool is empty, wait till a block is released back to the pool
func (pool *BlockPool) tryGet() *Block {
	var block *Block = nil

	select {
	case <-pool.ctx.Done():
		err := fmt.Errorf("Failed to Allocate Buffer as the process was cancelled, Len (blocksCh: %d, priorityCh: %d), MaxBlocks: %d",
			len(pool.blocksCh), len(pool.priorityCh), pool.maxBlocks)
		log.Debug("BlockPool::GetBlock : %v", err)
		return nil
	// getting a block from pool will be a blocking operation if the pool is empty
	case block = <-pool.blocksCh:
		break
	}

	// Mark the buffer ready for reuse now
	block.ReUse()
	return block
}

func (pool *BlockPool) mustGet() *Block {
	var block *Block = nil

	select {
	case <-pool.ctx.Done():
		err := fmt.Errorf("Failed to Allocate Buffer as the process was cancelled, Len (priorityCh: %d, blockCh: %d), MaxBlocks: %d",
			len(pool.priorityCh), len(pool.blocksCh), pool.maxBlocks)
		log.Debug("BlockPool::MustGet : %v", err)
		return nil
	case block = <-pool.priorityCh:
		break
	case block = <-pool.blocksCh:
		break
	}

	// Mark the buffer ready for reuse now
	block.ReUse()
	return block
}

// Release back the Block to the pool
func (pool *BlockPool) Release(block *Block) {
	select {
	case pool.priorityCh <- block:
		break
	default:
		select {
		case pool.blocksCh <- block:
			break
		default:
			_ = block.Delete()
		}
	}
}
