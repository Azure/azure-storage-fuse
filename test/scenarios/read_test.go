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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileRead(t *testing.T) {
	t.Parallel()
	filename := "testfile_read.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.NoError(t, err)

		file, err := os.Open(filePath)
		assert.NoError(t, err)

		readContent := make([]byte, len(content))
		_, err = file.Read(readContent)
		assert.True(t, err == nil || err == io.EOF)

		assert.Equal(t, string(content), string(readContent))

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test stripe reading. Create a large file say 32M, then open the files at different offsets and whether data is getting matched.
func TestStripeReading(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_reading.txt"
	content := []byte("Stripe Reading Test data")
	tempbuf := make([]byte, len(content))
	offsets := []int64{69, 8*1024*1024 + 69, 16*1024*1024 + 69}
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		// Write to the file.
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.NoError(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.NoError(t, err)
		// Read from the different offsets using different file descriptions
		file0, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //read at 0MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file1.ReadAt(tempbuf, offsets[1]) //read at 8MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = file2.ReadAt(tempbuf, offsets[2]) //read at 16MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)

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

// Test stripe reading with dup.
func TestStripeReadingWithDup(t *testing.T) {
	t.Parallel()
	filename := "testfile_stripe_reading_dup.txt"
	content := []byte("Stripe Reading With Dup Test data")
	tempbuf := make([]byte, len(content))
	offsets := []int64{69, 8*1024*1024 + 69, 16*1024*1024 + 69}
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		// Write to the file.
		for _, off := range offsets {
			written, err := file.WriteAt(content, int64(off))
			assert.NoError(t, err)
			assert.Equal(t, len(content), written)
		}
		err = file.Close()
		assert.NoError(t, err)

		// Read from the different offsets using same file description, by duplicating the fd
		file0, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		fd1, err := syscall.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)
		fd2, err := syscall.Dup(int(file0.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		bytesread, err := file0.ReadAt(tempbuf, offsets[0]) //read at 0MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = syscall.Pread(fd1, tempbuf, offsets[1]) //write at 8MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)
		bytesread, err = syscall.Pread(fd2, tempbuf, offsets[2]) //write at 16MB
		assert.NoError(t, err)
		assert.Equal(t, len(tempbuf), bytesread)
		assert.Equal(t, content, tempbuf)

		err = file0.Close()
		assert.NoError(t, err)
		err = syscall.Close(fd1)
		assert.NoError(t, err)
		err = syscall.Close(fd2)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestReadingUncommittedData(t *testing.T) {
	t.Parallel()
	filename := "testfile_reading_uncommitted_data.txt"
	// Write 16MB data and read the data before and after flush
	databuffer := make([]byte, 16*1024*1024) // 16MB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		byteswritten, err := file.Write(databuffer)
		assert.Equal(t, 16*1024*1024, byteswritten)
		assert.NoError(t, err)

		// Wait for a while to ensure data is uploaded and flushed from cache.
		time.Sleep(5 * time.Second)

		// Read the data before flush
		readbuffer := make([]byte, 16*1024*1024)
		_, err = file.ReadAt(readbuffer, 0)
		assert.NoError(t, err)
		assert.Equal(t, databuffer, readbuffer)

		// Flush the data
		err = file.Sync()
		assert.NoError(t, err)

		// Read the data after flush
		readbuffer2 := make([]byte, 16*1024*1024)
		_, err = file.ReadAt(readbuffer2, 0)
		assert.NoError(t, err)
		assert.Equal(t, databuffer, readbuffer2)

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
