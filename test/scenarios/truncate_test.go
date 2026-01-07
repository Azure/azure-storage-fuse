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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Add Tests for reading and writing to the newly created blocks and modified blocks while truncate.
const (
	truncate int = iota
	ftruncate
)

func TestFileTruncateSameSize(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_same_size.txt"
	FileTruncate(t, filename, 10, 10, truncate)
	FileTruncate(t, filename, 9*1024*1024, 9*1024*1024, truncate)
	FileTruncate(t, filename, 8*1024*1024, 8*1024*1024, truncate)
}

func TestFileTruncateShrink(t *testing.T) {
	t.Parallel()

	filename := "testfile_truncate_shrink.txt"
	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name        string
		initialSize int
		finalSize   int
		truncation  int
	}{
		{fmt.Sprintf("%s_20_5_truncate", filename), 20, 5, truncate},
		{fmt.Sprintf("%s_10M_5K_truncate", filename), 10 * 1024 * 1024, 5 * 1024, truncate},
		{fmt.Sprintf("%s_20M_5K_truncate", filename), 20 * 1024 * 1024, 5 * 1024, truncate},
		{fmt.Sprintf("%s_30M_20M_truncate", filename), 30 * 1024 * 1024, 20 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_20_5_ftruncate", filename), 20, 5, ftruncate},
		{fmt.Sprintf("%s_10M_5K_ftruncate", filename), 10 * 1024 * 1024, 5 * 1024, ftruncate},
		{fmt.Sprintf("%s_20M_5K_ftruncate", filename), 20 * 1024 * 1024, 5 * 1024, ftruncate},
		{fmt.Sprintf("%s_30M_20M_ftruncate", filename), 30 * 1024 * 1024, 20 * 1024 * 1024, ftruncate},
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name        string
			initialSize int
			finalSize   int
			truncation  int
		}) {
			defer wg.Done()
			FileTruncate(t, tt.name, tt.initialSize, tt.finalSize, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestFileTruncateExpand(t *testing.T) {
	t.Parallel()

	filename := "testfile_truncate_expand.txt"
	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name        string
		initialSize int
		finalSize   int
		truncation  int
	}{
		{fmt.Sprintf("%s_5_20_truncate", filename), 5, 20, truncate},
		{fmt.Sprintf("%s_5K_10M_truncate", filename), 5 * 1024, 10 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_5K_20M_truncate", filename), 5 * 1024, 20 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_20M_30M_truncate", filename), 20 * 1024 * 1024, 30 * 1024 * 1024, truncate},
		{fmt.Sprintf("%s_5_20_ftruncate", filename), 5, 20, ftruncate},
		{fmt.Sprintf("%s_5K_10M_ftruncate", filename), 5 * 1024, 10 * 1024 * 1024, ftruncate},
		{fmt.Sprintf("%s_5K_20M_ftruncate", filename), 5 * 1024, 20 * 1024 * 1024, ftruncate},
		{fmt.Sprintf("%s_20M_30M_ftruncate", filename), 20 * 1024 * 1024, 30 * 1024 * 1024, ftruncate},
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name        string
			initialSize int
			finalSize   int
			truncation  int
		}) {
			defer wg.Done()
			FileTruncate(t, tt.name, tt.initialSize, tt.finalSize, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestTruncateNoFile(t *testing.T) {
	t.Parallel()
	filename := "testfile_truncate_no_file.txt"

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.Truncate(filePath, 5)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "no such file or directory")
	}
}

// Test for writing data, truncate and close the file.
// Truncate can be done using os.Truncate or file.Truncate.
func TestWriteTruncateClose(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name       string
		writeSize  int
		truncSize  int
		truncation int
	}{
		{"testWriteTruncateClose1M7M_truncate", 1 * 1024 * 1024, 7 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M13M_truncate", 1 * 1024 * 1024, 13 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M20M_truncate", 1 * 1024 * 1024, 20 * 1024 * 1024, truncate},
		{"testWriteTruncateClose7M1M_truncate", 7 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose13M1M_truncate", 13 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose20M1M_truncate", 20 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateClose1M7M_ftruncate", 1 * 1024 * 1024, 7 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose1M13M_ftruncate", 1 * 1024 * 1024, 13 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose1M20M_ftruncate", 1 * 1024 * 1024, 20 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose7M1M_ftruncate", 7 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose13M1M_ftruncate", 13 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateClose20M1M_ftruncate", 20 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
	}

	WriteTruncateClose := func(t *testing.T, filename string, writeSize int, truncSize int, call int) {
		content := make([]byte, writeSize)
		_, err := io.ReadFull(rand.Reader, content)
		assert.NoError(t, err)

		for _, mnt := range mountpoints {
			filePath := filepath.Join(mnt, filename)
			file, err := os.Create(filePath)
			assert.NoError(t, err)

			written, err := file.Write(content)
			assert.NoError(t, err)
			assert.Equal(t, writeSize, written)

			if call == truncate {
				err := os.Truncate(filePath, int64(truncSize))
				assert.NoError(t, err)
			} else {
				err := file.Truncate(int64(truncSize))
				assert.NoError(t, err)
			}

			err = file.Close()
			assert.NoError(t, err)
		}

		checkFileIntegrity(t, filename)
		removeFiles(t, filename)
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name       string
			writeSize  int
			truncSize  int
			truncation int
		}) {
			defer wg.Done()
			WriteTruncateClose(t, tt.name, tt.writeSize, tt.truncSize, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

// Test Write, truncate, write again and close the file.
func TestWriteTruncateWriteClose(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	// Define table tests
	tests := []struct {
		name       string
		writeSize  int
		truncSize  int
		truncation int
	}{
		{"testWriteTruncateWriteClose1M7M_truncate", 1 * 1024 * 1024, 7 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose1M13M_truncate", 1 * 1024 * 1024, 13 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose1M20M_truncate", 1 * 1024 * 1024, 20 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose7M1M_truncate", 7 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose13M1M_truncate", 13 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose20M1M_truncate", 20 * 1024 * 1024, 1 * 1024 * 1024, truncate},
		{"testWriteTruncateWriteClose1M7M_ftruncate", 1 * 1024 * 1024, 7 * 1024 * 1024, ftruncate},
		{"testWriteTruncateWriteClose1M13M_ftruncate", 1 * 1024 * 1024, 13 * 1024 * 1024, ftruncate},
		{"testWriteTruncateWriteClose1M20M_ftruncate", 1 * 1024 * 1024, 20 * 1024 * 1024, ftruncate},
		{"testWriteTruncateWriteClose7M1M_ftruncate", 7 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateWriteClose13M1M_ftruncate", 13 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
		{"testWriteTruncateWriteClose20M1M_ftruncate", 20 * 1024 * 1024, 1 * 1024 * 1024, ftruncate},
	}

	WriteTruncateWriteClose := func(t *testing.T, filename string, writeSize int, truncSize int, call int) {
		content := make([]byte, writeSize)
		_, err := io.ReadFull(rand.Reader, content)
		assert.NoError(t, err)

		for _, mnt := range mountpoints {
			filePath := filepath.Join(mnt, filename)
			file, err := os.Create(filePath)
			assert.NoError(t, err)

			written, err := file.Write(content)
			assert.NoError(t, err)
			assert.Equal(t, writeSize, written)

			if call == truncate {
				err := os.Truncate(filePath, int64(truncSize))
				assert.NoError(t, err)
			} else {
				err := file.Truncate(int64(truncSize))
				assert.NoError(t, err)
			}

			written, err = file.Write(content)
			assert.NoError(t, err)
			assert.Equal(t, writeSize, written)

			err = file.Close()
			assert.NoError(t, err)
		}

		checkFileIntegrity(t, filename)
		removeFiles(t, filename)
	}

	// Add the number of test cases to the WaitGroup
	wg.Add(len(tests))

	// Iterate over the test cases
	for _, tt := range tests {
		go func(tt struct {
			name       string
			writeSize  int
			truncSize  int
			truncation int
		}) {
			defer wg.Done()
			WriteTruncateWriteClose(t, tt.name, tt.writeSize, tt.truncSize, tt.truncation)
		}(tt)
	}

	// Wait for all goroutines to complete
	wg.Wait()

}

// tests for truncate function which works on path
func FileTruncate(t *testing.T, filename string, initialSize int, finalSize int, call int) {
	content := make([]byte, initialSize)
	_, err := io.ReadFull(rand.Reader, content)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		err := os.WriteFile(filePath, content, 0644)
		assert.NoError(t, err)

		switch call {
		case truncate:
			err = os.Truncate(filePath, int64(finalSize))
			assert.NoError(t, err)
		case ftruncate:
			file, _ := os.OpenFile(filePath, os.O_RDWR, 0644)
			assert.NoError(t, err)
			err = file.Truncate(int64(finalSize))
			assert.NoError(t, err)
			err = file.Close()
			assert.NoError(t, err)
		}

		readContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		expectedContent := make([]byte, initialSize)
		copy(expectedContent, content)
		if finalSize > initialSize {
			expectedContent = append(expectedContent, make([]byte, finalSize-initialSize)...)
		} else {
			expectedContent = expectedContent[:finalSize]
		}
		assert.Equal(t, string(expectedContent), string(readContent))
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
