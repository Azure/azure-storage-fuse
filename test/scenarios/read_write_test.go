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
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test Read Write From Same handle
func TestOpenWriteRead(t *testing.T) {
	t.Parallel()
	filename := "testfile_open_write_read.txt"
	tempbuffer := make([]byte, 4*1024)
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		written, err := file.WriteAt(databuffer, 200)
		assert.NoError(t, err)
		assert.Equal(t, 4096, written)
		read, err := file.Read(tempbuffer)
		assert.NoError(t, err)
		assert.Equal(t, 4096, read)
		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)

}

// Test reading the data written by the other file handle.
func TestReadWrittenData(t *testing.T) {
	t.Parallel()
	filename := "testfile_read_written_data.txt"
	content := []byte("Read Written Data Test data")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		fileWrite, err := os.Create(filePath)
		assert.NoError(t, err)

		byteswritten, err := fileWrite.Write(content)
		assert.Equal(t, len(content), byteswritten)
		assert.NoError(t, err)

		// Open another file handle to read the data.
		fileRead, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		readContent := make([]byte, len(content))
		_, err = fileRead.Read(readContent)
		assert.True(t, err == nil || err == io.EOF)

		assert.Equal(t, string(content), string(readContent))

		err = fileWrite.Close()
		assert.NoError(t, err)
		err = fileRead.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test Writing the data that was read from other file handle.
func TestWriteReadData(t *testing.T) {
	t.Parallel()
	filename := "testfile_write_read_data.txt"
	dataBuffer := make([]byte, 4*1024*1024)
	_, err := io.ReadFull(rand.Reader, dataBuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, dataBuffer, 0644)
		assert.NoError(t, err)

		// Open 2 handles to read and write the data.
		fileRead, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		fileWrite, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		readBuffer := make([]byte, 128*1024)
		totalRead := 0
		for totalRead < len(dataBuffer) {
			bytesRead, err := fileRead.Read(readBuffer)
			assert.NoError(t, err)
			// Write the read data to fileWrite handle
			bytesWritten, err := fileWrite.Write(readBuffer[:bytesRead])
			assert.NoError(t, err)
			assert.Equal(t, bytesRead, bytesWritten)
			totalRead += bytesRead
		}
	}
	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test for writing from 1 fd and reading from another fd.
func TestOpenWriteReadMultipleHandles(t *testing.T) {
	t.Parallel()
	filename := "testfile_open_write_read_multiple_handles.txt"
	tempbuffer := make([]byte, 4*1024)
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)
		file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		file3, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)
		file4, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		for i := range 10 {
			// Write the buffer 10 times from file
			written, err := file.Write(databuffer)
			assert.NoError(t, err)
			assert.Equal(t, 4*1024, written)

			// write the buffer 10 times from file2 from offset 40KB
			written, err = file2.WriteAt(databuffer, int64(40*1024)+int64(i*(4*1024)))
			assert.NoError(t, err)
			assert.Equal(t, 4*1024, written)

			// write the buffer 10 times from file3 from offset 80KB
			written, err = file3.WriteAt(databuffer, int64(80*1024)+int64(i*(4*1024)))
			assert.NoError(t, err)
			assert.Equal(t, 4*1024, written)
		}

		for range 30 {
			// Read the entire file before closing the write handles.
			copy(tempbuffer, make([]byte, 4*1024)) // Clear the buffer
			read, err := file4.Read(tempbuffer)
			assert.NoError(t, err)
			assert.Equal(t, 4*1024, read)
			assert.Equal(t, databuffer, tempbuffer)
		}
		err = file.Close()
		assert.NoError(t, err)
		err = file2.Close()
		assert.NoError(t, err)
		err = file3.Close()
		assert.NoError(t, err)
		err = file4.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
