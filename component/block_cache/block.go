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
	"fmt"
	"syscall"
)

// Block is a memory mapped buffer with its state
type Block struct {
	id     uint64
	closed bool
	state  chan int
	data   []byte
}

// newblock creates a new memory mapped buffer with the specified size
func AllocateBlock(size uint64) (*Block, error) {
	if size == 0 {
		return nil, fmt.Errorf("invalid size")
	}

	prot, flags := syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE
	addr, err := syscall.Mmap(-1, 0, int(size), prot, flags)

	if err != nil {
		return nil, fmt.Errorf("mmap error: %v", err)
	}

	return &Block{
		data:   addr,
		state:  nil,
		id:     0,
		closed: true,
	}, nil

	// we do not create channel here, as that will be created when buffer is retreived
	// reinit will always be called before use and that will create the channel as well.
}

// Delete cleans up the memory mapped buffer
func (b *Block) Delete() error {
	if b.data == nil {
		return fmt.Errorf("invalid buffer")
	}

	err := syscall.Munmap(b.data)
	b.data = nil
	if err != nil {
		// if we get here, there is likely memory corruption.
		return fmt.Errorf("munmap error: %v", err)
	}

	return nil
}

// reinit the Block by recreating its channel
func (b *Block) ReUse() {
	b.state = make(chan int, 1)
	b.id = 0
	b.closed = false
}

// mark this Block is now ready for ops
func (b *Block) ReadyForReading() error {
	if b.state == nil {
		return fmt.Errorf("block was never used")
	}

	if len(b.state) != 0 {
		return fmt.Errorf("invalid state to mark it ready")
	}

	b.state <- 1
	return nil
}

// mark this Block is ready to be read freely now without blocking
func (b *Block) Unblock() error {
	if b.closed {
		return fmt.Errorf("invalid state of block to unblock")
	}

	close(b.state)
	b.closed = true

	return nil
}
