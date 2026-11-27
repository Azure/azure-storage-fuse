// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package scenarios

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test O_TRUNC flag
func TestOTruncFlag(t *testing.T) {
	t.Parallel()
	filename := "testfile_trunc.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.NoError(t, err)

		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Empty(t, readContent)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestOTruncWhileWriting(t *testing.T) {
	t.Parallel()
	OTruncWhileWritingHelper(t, 64*1024)
	OTruncWhileWritingHelper(t, 10*1024*1024)
	OTruncWhileWritingHelper(t, 24*1024*1024)
}

func OTruncWhileWritingHelper(t *testing.T, size int) {
	filename := "testfile_O_trunc_while_writing.txt"
	databuf := make([]byte, 4096)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.NoError(t, err)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)

		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		assert.NoError(t, err)

		for i := 0; i < size; i += 4096 {
			bytesWritten, err := file.Write(databuf)
			assert.Equal(t, 4096, bytesWritten)
			assert.NoError(t, err)
		}
		// lets open file with O_TRUNC
		file2, err := os.OpenFile(filePath, os.O_TRUNC, 0644)
		assert.NoError(t, err)

		// Continue the write on first fd.
		bytesWritten, err := file.Write(databuf)
		assert.Equal(t, 4096, bytesWritten)
		assert.NoError(t, err)
		// Now a big hole is formed at the starting of the file
		err = file2.Close()
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestOTruncWhileReading(t *testing.T) {
	t.Parallel()
	OTruncWhileReadingHelper(t, 64*1024)
	OTruncWhileReadingHelper(t, 10*1024*1024)
	OTruncWhileReadingHelper(t, 24*1024*1024)
}

func OTruncWhileReadingHelper(t *testing.T, size int) {
	filename := "testfile_O_trunc_while_reading.txt"
	databuf := make([]byte, 4096)
	_, err := io.ReadFull(rand.Reader, databuf)
	assert.NoError(t, err)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		// Create the file with desired size before starting the test
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		assert.NoError(t, err)

		for i := 0; i < size; i += 4096 {
			bytesWritten, err := file.Write(databuf)
			assert.Equal(t, 4096, bytesWritten)
			assert.NoError(t, err)
		}
		err = file.Close()
		assert.NoError(t, err)
		//------------------------------------------------------
		// Start reading the file
		file, err = os.OpenFile(filePath, os.O_RDONLY, 0644)
		assert.NoError(t, err)
		bytesread, err := file.Read(databuf)
		assert.Equal(t, 4096, bytesread)
		assert.NoError(t, err)

		// lets open file with O_TRUNC
		file2, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC, 0644)
		assert.NoError(t, err)

		// Continue the reading on first fd.
		bytesRead, err := file.Read(databuf)
		assert.Equal(t, 0, bytesRead)
		assert.Equal(t, io.EOF, err)

		err = file2.Close()
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
