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
	"os"
	"syscall"
)

// block is a memory mapped buffer
type block struct {
	state chan int
	data  []byte
}

// newblock creates a new memory mapped buffer with the specified size
func newblock(size uint64) (*block, error) {
	prot, flags := syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE
	addr, err := syscall.Mmap(-1, 0, int(size), prot, flags)

	if err != nil {
		return nil, os.NewSyscallError("Mmap", err)
	}

	return &block{
		data: addr,
	}, nil
}

// delete cleans up the memory mapped buffer
func (b *block) delete() error {
	err := syscall.Munmap(b.data)
	b.data = nil
	if err != nil {
		// if we get here, there is likely memory corruption.
		return fmt.Errorf("Munmap error: %v", err)
	}

	return nil
}

// mark this block is now ready for ops
func (b *block) ready() {
	b.state <- 1
	b.state <- 2
}

// mark this block is ready to be reused now
func (b *block) done() {
	close(b.state)
}

// reinit the block by recreating its channel
func (b *block) reinit() {
	b.state = make(chan int, 2)
}

func (b *block) size() uint64 {
	s := cap(b.data)
	return uint64(s)
}
