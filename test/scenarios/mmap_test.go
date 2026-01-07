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
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

// Test open, mmap, read, write, munmap, close
func TestMmapReadWrite(t *testing.T) {
	if directIOEnabledOnMountpoint {
		t.Skip("Skipping mmap tests as Direct I/O is enabled on mountpoint")
	}

	t.Parallel()
	filename := "testfile_mmap_read_write.txt"
	content := []byte("Hello, Memory Mapped File!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		_, err = file.Write(content)
		assert.NoError(t, err)

		// Memory map the file
		data, err := syscall.Mmap(int(file.Fd()), 0, len(content), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		assert.NoError(t, err)

		// Read the mapped data
		assert.Equal(t, content, data)

		// Modify the mapped data
		copy(data, []byte("Hello, MMap!"))

		// Unmap the file
		err = syscall.Munmap(data)
		assert.NoError(t, err)

		err = file.Close()
		assert.NoError(t, err)

		// Read back the modified content
		readContent, err := os.ReadFile(filePath)
		expectedContent := make([]byte, len(content))
		copy(expectedContent, content)
		copy(expectedContent, []byte("Hello, MMap!"))
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, readContent)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test mmap a large file and read from different offsets
func TestMmapLargeFileRead(t *testing.T) {
	if directIOEnabledOnMountpoint {
		t.Skip("Skipping mmap tests as Direct I/O is enabled on mountpoint")
	}

	t.Parallel()
	filename := "testfile_mmap_large_read.txt"
	content := []byte("Memory Mapped Large File Read Test Data")
	offsets := []int64{0, 8 * 1024 * 1024, 16 * 1024 * 1024} // 0MB, 8MB, 16MB
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		// Write to the file at different offsets
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.NoError(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.NoError(t, err)

		// Memory map the file
		file, err = os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		stat, err := file.Stat()
		assert.NoError(t, err)

		data, err := syscall.Mmap(int(file.Fd()), 0, int(stat.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
		assert.NoError(t, err)

		// Read from different offsets
		for _, off := range offsets {
			readData := data[off : off+int64(len(content))]
			assert.Equal(t, content, readData)
		}

		// Unmap the file
		err = syscall.Munmap(data)
		assert.NoError(t, err)

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test open, mmap, close, read/write, msync, munmap
func TestMmapWithMsync(t *testing.T) {
	if directIOEnabledOnMountpoint {
		t.Skip("Skipping mmap tests as Direct I/O is enabled on mountpoint")
	}

	t.Parallel()
	filename := "testfile_mmap_with_msync.txt"
	content := []byte("MMap With Msync Test Data")

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		_, err = file.Write(content)
		assert.NoError(t, err)

		// Memory map the file
		data, err := syscall.Mmap(int(file.Fd()), 0, len(content), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		assert.NoError(t, err)

		// Close the file
		err = file.Close()
		assert.NoError(t, err)

		// Modify the mapped data
		copy(data, []byte("MMap With Msync!"))

		// Sync the changes to the file
		err = unix.Msync(data, syscall.MS_SYNC)
		assert.NoError(t, err)

		// Unmap the file
		err = syscall.Munmap(data)
		assert.NoError(t, err)

		// Read back the modified content
		readContent, err := os.ReadFile(filePath)
		expectedContent := make([]byte, len(content))
		copy(expectedContent, content)
		copy(expectedContent, []byte("MMap With Msync!"))
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, readContent)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test open, memory map, close, read/write, munmap
// In this test, we don't get flush after writing the data as we are not calling msync before munmap, we should ensure
// the data is written when release is called.
func TestMmapAfterFileClose(t *testing.T) {
	if directIOEnabledOnMountpoint {
		t.Skip("Skipping mmap tests as Direct I/O is enabled on mountpoint")
	}

	t.Parallel()
	filename := "testfile_mmap_after_close.txt"
	content := []byte("MMap After File Close Test Data")

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		_, err = file.Write(content)
		assert.NoError(t, err)

		// Memory map the file
		data, err := syscall.Mmap(int(file.Fd()), 0, len(content), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		assert.NoError(t, err)

		// Close the file
		err = file.Close()
		assert.NoError(t, err)

		// Modify the mapped data
		copy(data, []byte("MMap After Close!"))

		// Unmap the file
		err = syscall.Munmap(data)
		assert.NoError(t, err)

		// Read back the modified content
		readContent, err := os.ReadFile(filePath)
		expectedContent := make([]byte, len(content))
		copy(expectedContent, content)
		copy(expectedContent, []byte("MMap After Close!"))
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, readContent)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
