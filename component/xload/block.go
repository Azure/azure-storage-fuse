// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"fmt"
	"syscall"
)

// Block is a memory mapped buffer with its state to hold data
type Block struct {
	Index  int    // Index of the block in the pool
	Offset int64  // Start offset of the data this block holds
	Length int64  // Length of data that this block holds
	Id     string // ID to represent this block in the blob
	Data   []byte // Data this block holds
}

// AllocateBlock creates a new memory mapped buffer for the given size
func AllocateBlock(size uint64) (*Block, error) {
	if size == 0 {
		return nil, fmt.Errorf("invalid size")
	}

	prot, flags := syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE
	addr, err := syscall.Mmap(-1, 0, int(size), prot, flags)

	if err != nil {
		return nil, fmt.Errorf("mmap error: %v", err)
	}

	block := &Block{
		Data: addr,
	}

	return block, nil
}

// Delete cleans up the memory mapped buffer
func (b *Block) Delete() error {
	if b.Data == nil {
		return fmt.Errorf("invalid buffer")
	}

	err := syscall.Munmap(b.Data)
	b.Data = nil
	if err != nil {
		// if we get here, there is likely memory corruption.
		return fmt.Errorf("munmap error: %v", err)
	}

	return nil
}

// Clear the old data of this block
func (b *Block) ReUse() {
	b.Id = ""
	b.Index = 0
	b.Offset = 0
	b.Length = 0
}
