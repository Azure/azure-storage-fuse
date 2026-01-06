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
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFsync(t *testing.T) {
	t.Parallel()
	filename := "testfile_fsync.txt"
	content := []byte("Hello, World!")
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		_, err = file.Write(content)
		assert.NoError(t, err)

		err = file.Sync()
		assert.NoError(t, err)

		readContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		assert.Equal(t, string(content), string(readContent))

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

func TestFsyncWhileWriting(t *testing.T) {
	t.Parallel()
	var err error
	filename := "testfile_fsync_while_writing.txt"
	readBufSize := 4 * 1024
	content := make([]byte, readBufSize)
	_, err = io.ReadFull(rand.Reader, content)
	assert.NoError(t, err)
	expectedContent := make([]byte, 4*1024, 10*1024*1024)
	copy(expectedContent, content)
	actualContent := make([]byte, 10*1024*1024)
	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		// Write 9MB data, do an fsync for each 4K buffer. do read the data after fsync with other handle.
		for i := 0; i*readBufSize < 9*1024*1024; i += 4 * 1024 {
			bytesWritten, err := file.Write(content)
			assert.NoError(t, err)
			assert.Equal(t, len(content), bytesWritten)

			// We cannot do fsync for every 4K write, as the test takes long time to finish
			// do it for every 512K
			if i%(512*1024) == 0 {
				err = file.Sync()
				assert.NoError(t, err)
			}

			file1, err := os.Open(filePath)
			assert.NoError(t, err)
			bytesRead, err := file1.Read(actualContent)
			assert.Equal(t, (i+1)*readBufSize, bytesRead)
			assert.NoError(t, err)
			err = file1.Close()
			assert.NoError(t, err)

			assert.Equal(t, expectedContent[:(i+1)*readBufSize], actualContent[:(i+1)*readBufSize])
			expectedContent = append(expectedContent, content...)
		}

		err = file.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Test for multiple handles, parallel fsync calls while writing.
func TestParallelFsyncCalls(t *testing.T) {
	t.Parallel()
	filename := "testfile_parallel_fsync_calls.txt"
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file0, err := os.Create(filePath)
		assert.NoError(t, err)
		file1, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		assert.NoError(t, err)

		// for each 1MB writes trigger a flush call from another go routine.
		trigger_flush := make(chan struct{}, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := <-trigger_flush
				if !ok {
					break
				}
				err := file1.Sync()
				assert.NoError(t, err)
				if err != nil {
					fmt.Printf("%s", err.Error())
				}
			}
		}()
		// Write 40M data
		for i := 0; i < 40*1024*1024; i += 4 * 1024 {
			if i%(1*1024*1024) == 0 {
				trigger_flush <- struct{}{}
			}
			byteswritten, err := file0.Write(databuffer)
			assert.Equal(t, 4*1024, byteswritten)
			assert.NoError(t, err)
		}
		close(trigger_flush)
		wg.Wait()
		err = file0.Close()
		assert.NoError(t, err)
		err = file1.Close()
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}

// Dup the FD and do parallel flush calls while writing.
func TestParallelFsyncCallsByDuping(t *testing.T) {
	t.Parallel()
	filename := "testfile_parallel_fsync_calls_using_dup.txt"
	databuffer := make([]byte, 4*1024) // 4KB buffer
	_, err := io.ReadFull(rand.Reader, databuffer)
	assert.NoError(t, err)

	for _, mnt := range mountpoints {
		filePath := filepath.Join(mnt, filename)
		file, err := os.Create(filePath)
		assert.NoError(t, err)

		fd1, err := syscall.Dup(int(file.Fd()))
		assert.NotEqual(t, int(file.Fd()), fd1)
		assert.NoError(t, err)

		// for each 1MB writes trigger a flush call from another go routine.
		triggerFlush := make(chan struct{}, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := <-triggerFlush
				if !ok {
					break
				}
				err := syscall.Fdatasync(fd1)
				assert.NoError(t, err)
			}
		}()
		// Write 40M data
		for i := 0; i < 40*1024*1024; i += 4 * 1024 {
			if i%(1*1024*1024) == 0 {
				triggerFlush <- struct{}{}
			}
			byteswritten, err := file.Write(databuffer)
			assert.Equal(t, 4*1024, byteswritten)
			assert.NoError(t, err)
		}
		close(triggerFlush)
		wg.Wait()
		err = file.Close()
		assert.NoError(t, err)
		err = syscall.Close(fd1)
		assert.NoError(t, err)
	}

	checkFileIntegrity(t, filename)
	removeFiles(t, filename)
}
