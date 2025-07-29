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
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const _1MB uint64 = (1024 * 1024)

// BlockPool is a pool of Blocks
type BlockPool struct {
	// Channel holding free blocks
	blocksCh chan *Block

	// Channel holding free blocks for priority threads
	priorityCh chan *Block

	// block having null data
	zeroBlock *Block

	// channel to reset the data in a block
	resetBlockCh chan *Block

	// Wait group to wait for resetBlock() thread to finish
	wg sync.WaitGroup

	// Size of each block this pool holds
	blockSize uint64

	// Number of block that this pool can handle at max
	maxBlocks uint32
}

// NewBlockPool allocates a new pool of blocks
func NewBlockPool(blockSize uint64, memSize uint64) *BlockPool {
	// Ignore if config is invalid
	if blockSize == 0 || memSize < blockSize {
		log.Err("blockpool::NewBlockPool : blockSize : %v, memsize: %v", blockSize, memSize)
		return nil
	}

	// Calculate how many blocks can be allocated
	blockCount := uint32(memSize / blockSize)
	highPriority := (blockCount * 10) / 100

	pool := &BlockPool{
		blocksCh:     make(chan *Block, blockCount-highPriority-1),
		priorityCh:   make(chan *Block, highPriority),
		resetBlockCh: make(chan *Block, blockCount-1), // -1 because one block is used for zero data
		maxBlocks:    uint32(blockCount),
		blockSize:    blockSize,
	}

	// Preallocate all blocks so that during runtime we do not spend CPU cycles on this
	for i := (uint32)(0); i < blockCount; i++ {
		block, err := AllocateBlock(blockSize)
		if err != nil {
			log.Err("BlockPool::NewBlockPool : Failed to allocate block [%v]", err.Error())
			return nil
		}

		if i == blockCount-1 {
			pool.zeroBlock = block
		} else if i < highPriority {
			pool.priorityCh <- block
		} else {
			pool.blocksCh <- block
		}
	}

	// run a thread to reset the data in a block
	pool.wg.Add(1)
	go pool.resetBlock()

	return pool
}

// Terminate ends the block pool life
func (pool *BlockPool) Terminate() {
	// TODO: call terminate after all the threads have completed
	close(pool.resetBlockCh)
	pool.wg.Wait()

	close(pool.blocksCh)
	close(pool.priorityCh)

	_ = pool.zeroBlock.Delete()

	releaseBlock(pool.blocksCh)
	releaseBlock(pool.priorityCh)
}

// release back the memory allocated to each block
func releaseBlock(ch chan *Block) {
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
	return ((pool.maxBlocks - (uint32)(len(pool.blocksCh)+len(pool.priorityCh)+len(pool.resetBlockCh))) * 100) / pool.maxBlocks
}

// MustGet a Block from the pool, waits until defaultTimeout period before giving up the allocation of the buffer.
func (pool *BlockPool) MustGet() (*Block, error) {
	var block *Block = nil
	defaultTimeout := time.After(5 * time.Second)

	select {
	case block = <-pool.priorityCh:
		break
	case block = <-pool.blocksCh:
		break
	// Return error in case no blocks are available after default timeout
	case <-defaultTimeout:
		err := fmt.Errorf("Failed to Allocate Buffer, Len (priorityCh: %d, blockCh: %d), MaxBlocks: %d",
			len(pool.priorityCh), len(pool.blocksCh), pool.maxBlocks)
		log.Err("BlockPool::MustGet : %v", err)
		return nil, err
	}

	// Mark the buffer ready for reuse now
	block.ReUse()
	return block, nil
}

// TryGet a Block from the pool, return back if nothing is available
func (pool *BlockPool) TryGet() *Block {
	var block *Block = nil

	select {
	case block = <-pool.blocksCh:
		break

	default:
		return nil
	}

	// Mark the buffer ready for reuse now
	block.ReUse()
	return block
}

// Release back the Block to the pool
func (pool *BlockPool) Release(b *Block) {
	select {
	case pool.resetBlockCh <- b:
		break
	default:
		_ = b.Delete()
	}
}

// reset the data in a block before its next use
func (pool *BlockPool) resetBlock() {
	defer pool.wg.Done()

	for block := range pool.resetBlockCh {
		// reset the data with null entries
		copy(block.data, pool.zeroBlock.data)

		select {
		case pool.priorityCh <- block:
			continue
		default:
			select {
			case pool.blocksCh <- block:
				break
			default:
				_ = block.Delete()
			}
		}
	}
}
