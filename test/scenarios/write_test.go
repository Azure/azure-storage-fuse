/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package scenarios

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileWrite(t *testing.T) {
	t.Parallel()
	filename := "testfile_write.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		_, err = file.Write(content)
		assert.NoError(t, err)

		err = file.Close()
		assert.NoError(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		assert.Equal(t, string(content), string(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestWrite10MB(t *testing.T) {
	t.Parallel()
	filename := "testfile_write_10mb.txt"
	content := make([]byte, 10*1024*1024) // 10MB of data
	_, err := io.ReadFull(rand.Reader, content)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.NoError(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, content, readContent)
		assert.Len(t, readContent, len(content))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe writing.
// Write to the same file at different offsets using different file descriptions.
func TestStripeWriting(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_writing.txt"
	content := []byte("Stripe writing test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file0, err := os.Create(filePath)
		assert.NoError(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		written, err := file0.WriteAt(content, int64(0)) //write at 0MB
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = file1.WriteAt(content, int64(8*1024*1024)) //write at 8MB
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = file2.WriteAt(content, int64(16*1024*1024)) //write at 16MB
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)

		err = file0.Close()
		assert.NoError(t, err)
		err = file1.Close()
		assert.NoError(t, err)
		err = file2.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe writing with dup. same as the stripe writing but rather than opening so many files duplicate the file descriptor.
func TestStripeWritingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_writing_dup.txt"
	content := []byte("Stripe writing with dup test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		fd1, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		fd2, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		written, err := file.WriteAt(content, int64(0))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = syscall.Pwrite(fd1, content, int64(8*1024*1024))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)
		written, err = syscall.Pwrite(fd1, content, int64(16*1024*1024))
		assert.NoError(t, err)
		assert.Equal(t, len(content), written)

		err = file.Close()
		assert.NoError(t, err)
		err = syscall.Close(fd1)
		assert.NoError(t, err)
		err = syscall.Close(fd2)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test rand sparse writing on a file.
func TestRandSparseWriting(t *testing.T) {
	t.Parallel()
	filename := "testfile_sparse_write.txt"
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		written, err := file.WriteAt([]byte("Hello"), 1024*1024) // Write at 1MB offset, 1st block
		assert.NoError(t, err)
		assert.Equal(t, 5, written)

		written, err = file.WriteAt([]byte("World"), 12*1024*1024) // Write at 12MB offset, 2nd block
		assert.NoError(t, err)
		assert.Equal(t, 5, written)

		written, err = file.WriteAt([]byte("Cosmos"), 30*1024*1024) // Write at 30MB offset, 4th block
		assert.NoError(t, err)
		assert.Equal(t, 6, written)

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test sparse writing on blockoverlap assume block size as 8MB,
// write 4K buffers on overlapping zones of blocks.
func TestSparseWritingBlockOverlap(t *testing.T) {
	t.Parallel()
	filename := "testfile_block_overlap.txt"
	blockSize := 8 * 1024 * 1024 // 8MB
	bufferSize := 4 * 1024       // 4KB
	databuf := make([]byte, bufferSize)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		for i := 1; i <= 2; i++ {
			offset := i * blockSize
			offset -= 2 * 1024
			_, err = file.WriteAt(databuf, int64(offset))
			assert.NoError(t, err)
		}

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
