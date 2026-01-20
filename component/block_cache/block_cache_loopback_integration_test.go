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

package block_cache

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	integrationTestMountPath  = "/tmp/blobfuse_integration_mount"
	integrationTestCachePath  = "/tmp/blobfuse_integration_cache"
	integrationTestLoopbackPath = "/tmp/blobfuse_integration_loopback"
)

type BlockCacheLoopbackIntegrationTestSuite struct {
	suite.Suite
	assert      *assert.Assertions
	blockCache  *BlockCache
	loopbackFS  *loopback.LoopbackFS
	testPath    string
	cachePath   string
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	
	// Generate unique test paths for this test
	testID := fmt.Sprintf("%d", time.Now().UnixNano())
	suite.testPath = filepath.Join(integrationTestLoopbackPath, testID)
	suite.cachePath = filepath.Join(integrationTestCachePath, testID)

	// Setup test directories
	err := os.MkdirAll(suite.testPath, 0777)
	suite.assert.NoError(err)
	err = os.MkdirAll(suite.cachePath, 0777)
	suite.assert.NoError(err)

	// Configure logging
	cfg := common.LogConfig{
		Level: common.ELogLevel.LOG_DEBUG(),
	}
	log.SetDefaultLogger("base", cfg)

	// Setup configuration for block_cache and loopbackfs
	configString := fmt.Sprintf(`
loopbackfs:
  path: %s

block_cache:
  block-size-mb: 1
  mem-size-mb: 20
  prefetch: 12
  parallelism: 10
  path: %s
  disk-size-mb: 50
  disk-timeout-sec: 20
`, suite.testPath, suite.cachePath)

	err = config.ReadConfigFromReader(strings.NewReader(configString))
	suite.assert.NoError(err)
	config.Set("mount-path", integrationTestMountPath)

	// Initialize loopbackfs component
	suite.loopbackFS = loopback.NewLoopbackFSComponent().(*loopback.LoopbackFS)
	err = suite.loopbackFS.Configure(true)
	suite.assert.NoError(err)

	// Initialize block_cache component
	suite.blockCache = NewBlockCacheComponent().(*BlockCache)
	suite.blockCache.SetNextComponent(suite.loopbackFS)
	err = suite.blockCache.Configure(true)
	suite.assert.NoError(err)

	// Start both components
	err = suite.loopbackFS.Start(context.Background())
	suite.assert.NoError(err)
	err = suite.blockCache.Start(context.Background())
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TearDownTest() {
	if suite.blockCache != nil {
		suite.blockCache.Stop()
	}
	if suite.loopbackFS != nil {
		suite.loopbackFS.Stop()
	}

	// Cleanup test directories
	os.RemoveAll(suite.testPath)
	os.RemoveAll(suite.cachePath)
}

// ============================================================================
// Basic File Operations Tests
// ============================================================================

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestBasicFileCreateAndOpen() {
	defer suite.TearDownTest()
	
	fileName := "test_create_open.txt"

	// Create file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)

	// Close file
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	// Verify file exists
	_, err = os.Stat(filepath.Join(suite.testPath, fileName))
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestBasicFileRead() {
	defer suite.TearDownTest()
	
	fileName := "test_read.txt"
	content := "Hello, World! This is a test file for reading."

	// Create file directly in loopback storage
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, []byte(content), 0777)
	suite.assert.NoError(err)

	// Open file through block_cache
	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)

	// Read file content
	buffer := make([]byte, len(content))
	bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buffer,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), bytesRead)
	suite.assert.Equal(content, string(buffer))

	// Close file
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestBasicFileWrite() {
	defer suite.TearDownTest()
	
	fileName := "test_write.txt"
	content := "This is test content for writing."

	// Create and open file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(handle)

	// Write content
	bytesWritten, err := suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(content),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), bytesWritten)

	// Sync and close file
	err = suite.blockCache.SyncFile(internal.SyncFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{
		Handle: handle,
	})
	suite.assert.NoError(err)

	// Verify content by reading directly from loopback storage
	filePath := filepath.Join(suite.testPath, fileName)
	data, err := os.ReadFile(filePath)
	suite.assert.NoError(err)
	suite.assert.Equal(content, string(data))
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestWriteThenRead() {
	defer suite.TearDownTest()
	
	fileName := "test_write_read.txt"
	content := "Data to write and then read back for verification."

	// Create and write
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	bytesWritten, err := suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(content),
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), bytesWritten)

	// Sync and close
	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Re-open and read
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	buffer := make([]byte, len(content))
	bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buffer,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), bytesRead)
	suite.assert.Equal(content, string(buffer))

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

// ============================================================================
// Advanced File Operations Tests
// ============================================================================

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestFileTruncate() {
	defer suite.TearDownTest()
	
	fileName := "test_truncate.txt"
	initialContent := "This is a longer content that will be truncated."

	// Create and write initial content
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(initialContent),
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Truncate to smaller size
	truncateSize := int64(10)
	err = suite.blockCache.TruncateFile(internal.TruncateFileOptions{
		Name:    fileName,
		NewSize: truncateSize,
	})
	suite.assert.NoError(err)

	// Verify truncated size
	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: fileName})
	suite.assert.NoError(err)
	suite.assert.Equal(truncateSize, attr.Size)

	// Read and verify truncated content
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	buffer := make([]byte, truncateSize)
	bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buffer,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(int(truncateSize), bytesRead)
	suite.assert.Equal(initialContent[:truncateSize], string(buffer))

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestFileTruncateToZero() {
	defer suite.TearDownTest()
	
	fileName := "test_truncate_zero.txt"
	initialContent := "This content will be truncated to zero."

	// Create and write
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(initialContent),
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Truncate to zero
	err = suite.blockCache.TruncateFile(internal.TruncateFileOptions{
		Name:    fileName,
		NewSize: 0,
	})
	suite.assert.NoError(err)

	// Verify size is zero
	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: fileName})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), attr.Size)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestFileRename() {
	defer suite.TearDownTest()
	
	oldName := "test_old_name.txt"
	newName := "test_new_name.txt"
	content := "Content for rename test."

	// Create and write file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: oldName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(content),
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Rename file
	err = suite.blockCache.RenameFile(internal.RenameFileOptions{
		Src: oldName,
		Dst: newName,
	})
	suite.assert.NoError(err)

	// Verify old file doesn't exist
	_, err = os.Stat(filepath.Join(suite.testPath, oldName))
	suite.assert.Error(err)
	suite.assert.True(os.IsNotExist(err))

	// Verify new file exists with correct content
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  newName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	buffer := make([]byte, len(content))
	bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: handle,
		Offset: 0,
		Data:   buffer,
	})
	suite.assert.NoError(err)
	suite.assert.Equal(len(content), bytesRead)
	suite.assert.Equal(content, string(buffer))

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestFileDelete() {
	defer suite.TearDownTest()
	
	fileName := "test_delete.txt"
	content := "This file will be deleted."

	// Create file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(content),
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Delete file
	err = suite.blockCache.DeleteFile(internal.DeleteFileOptions{
		Name: fileName,
	})
	suite.assert.NoError(err)

	// Verify file doesn't exist
	_, err = os.Stat(filepath.Join(suite.testPath, fileName))
	suite.assert.Error(err)
	suite.assert.True(os.IsNotExist(err))
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestReadNonExistentFile() {
	defer suite.TearDownTest()
	
	fileName := "nonexistent.txt"

	// Try to open non-existent file
	_, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.Error(err)
}

// ============================================================================
// Concurrent Operations Tests
// ============================================================================

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestConcurrentReads() {
	defer suite.TearDownTest()
	
	fileName := "test_concurrent_reads.txt"
	content := strings.Repeat("Concurrent read test data. ", 100)

	// Create and write file
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, []byte(content), 0777)
	suite.assert.NoError(err)

	// Perform concurrent reads
	numReaders := 10
	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerId int) {
			defer wg.Done()

			handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
				Name:  fileName,
				Flags: os.O_RDONLY,
				Mode:  0777,
			})
			if err != nil {
				errors <- fmt.Errorf("reader %d: failed to open file: %v", readerId, err)
				return
			}
			defer suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})

			buffer := make([]byte, len(content))
			bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
				Handle: handle,
				Offset: 0,
				Data:   buffer,
			})
			if err != nil {
				errors <- fmt.Errorf("reader %d: failed to read: %v", readerId, err)
				return
			}
			if bytesRead != len(content) {
				errors <- fmt.Errorf("reader %d: expected %d bytes, got %d", readerId, len(content), bytesRead)
				return
			}
			if string(buffer) != content {
				errors <- fmt.Errorf("reader %d: content mismatch", readerId)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		suite.assert.NoError(err)
	}
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestConcurrentWrites() {
	defer suite.TearDownTest()
	
	fileName := "test_concurrent_writes.txt"
	numWriters := 5
	contentPerWriter := "Writer data chunk. "
	
	// Create file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Perform concurrent writes to different offsets
	var wg sync.WaitGroup
	errors := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerId int) {
			defer wg.Done()

			handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
				Name:  fileName,
				Flags: os.O_RDWR,
				Mode:  0777,
			})
			if err != nil {
				errors <- fmt.Errorf("writer %d: failed to open: %v", writerId, err)
				return
			}
			defer suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})

			content := fmt.Sprintf("Writer %d: %s", writerId, contentPerWriter)
			offset := int64(writerId * len(content))

			_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
				Handle: handle,
				Offset: offset,
				Data:   []byte(content),
			})
			if err != nil {
				errors <- fmt.Errorf("writer %d: failed to write: %v", writerId, err)
				return
			}

			err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
			if err != nil {
				errors <- fmt.Errorf("writer %d: failed to sync: %v", writerId, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		suite.assert.NoError(err)
	}

	// Verify all writes succeeded by reading back
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	for i := 0; i < numWriters; i++ {
		expectedContent := fmt.Sprintf("Writer %d: %s", i, contentPerWriter)
		offset := int64(i * len(expectedContent))
		buffer := make([]byte, len(expectedContent))

		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: offset,
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(len(expectedContent), bytesRead)
		suite.assert.Equal(expectedContent, string(buffer))
	}

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestConcurrentReadWrite() {
	defer suite.TearDownTest()
	
	fileName := "test_concurrent_read_write.txt"
	initialContent := strings.Repeat("Initial data. ", 50)

	// Create file with initial content
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, []byte(initialContent), 0777)
	suite.assert.NoError(err)

	// Launch concurrent readers and writers
	numReaders := 5
	numWriters := 3
	var wg sync.WaitGroup
	errors := make(chan error, numReaders+numWriters)

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerId int) {
			defer wg.Done()

			for j := 0; j < 3; j++ {
				handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
					Name:  fileName,
					Flags: os.O_RDONLY,
					Mode:  0777,
				})
				if err != nil {
					errors <- fmt.Errorf("reader %d iteration %d: failed to open: %v", readerId, j, err)
					return
				}

				buffer := make([]byte, 100)
				_, err = suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
					Handle: handle,
					Offset: int64(readerId * 10),
					Data:   buffer,
				})
				if err != nil && err != io.EOF {
					errors <- fmt.Errorf("reader %d iteration %d: failed to read: %v", readerId, j, err)
				}

				suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerId int) {
			defer wg.Done()

			for j := 0; j < 2; j++ {
				handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
					Name:  fileName,
					Flags: os.O_RDWR,
					Mode:  0777,
				})
				if err != nil {
					errors <- fmt.Errorf("writer %d iteration %d: failed to open: %v", writerId, j, err)
					return
				}

				content := fmt.Sprintf("W%d-%d ", writerId, j)
				offset := int64(writerId * 100)

				_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
					Handle: handle,
					Offset: offset,
					Data:   []byte(content),
				})
				if err != nil {
					errors <- fmt.Errorf("writer %d iteration %d: failed to write: %v", writerId, j, err)
				}

				suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
				suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
				time.Sleep(time.Millisecond * 20)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		suite.T().Logf("Error during concurrent read/write: %v", err)
		errorCount++
	}
	
	// Allow some errors due to concurrent access but ensure no deadlocks
	suite.assert.Less(errorCount, numReaders+numWriters, "Too many errors during concurrent operations")
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestMultipleHandlesToSameFile() {
	defer suite.TearDownTest()
	
	fileName := "test_multiple_handles.txt"
	content := "Content for multiple handles test."

	// Create and write file
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, []byte(content), 0777)
	suite.assert.NoError(err)

	// Open multiple handles to the same file
	numHandles := 5
	handles := make([]*handlemap.Handle, numHandles)

	for i := 0; i < numHandles; i++ {
		handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
			Name:  fileName,
			Flags: os.O_RDONLY,
			Mode:  0777,
		})
		suite.assert.NoError(err)
		handles[i] = handle
	}

	// Read from each handle
	for i, handle := range handles {
		buffer := make([]byte, len(content))
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: 0,
			Data:   buffer,
		})
		suite.assert.NoError(err, "Handle %d failed to read", i)
		suite.assert.Equal(len(content), bytesRead)
		suite.assert.Equal(content, string(buffer))
	}

	// Close all handles
	for i, handle := range handles {
		err := suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{
			Handle: handle,
		})
		suite.assert.NoError(err, "Failed to close handle %d", i)
	}
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestInterleavedReadWrite() {
	defer suite.TearDownTest()
	
	fileName := "test_interleaved_ops.txt"
	
	// Create file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Perform interleaved operations
	operations := []string{"write", "read", "write", "read", "write", "read"}
	offset := int64(0)
	
	for i, op := range operations {
		handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
			Name:  fileName,
			Flags: os.O_RDWR,
			Mode:  0777,
		})
		suite.assert.NoError(err)

		if op == "write" {
			content := fmt.Sprintf("Data-%d ", i)
			_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
				Handle: handle,
				Offset: offset,
				Data:   []byte(content),
			})
			suite.assert.NoError(err)
			err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
			suite.assert.NoError(err)
			offset += int64(len(content))
		} else {
			buffer := make([]byte, 20)
			_, err = suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
				Handle: handle,
				Offset: 0,
				Data:   buffer,
			})
			// Read may fail if nothing written yet, that's okay
			if err != nil && err != io.EOF {
				suite.T().Logf("Read operation %d returned error (may be expected): %v", i, err)
			}
		}

		err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
}

// ============================================================================
// Data Integrity Tests
// ============================================================================

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestPartialReadWrite() {
	defer suite.TearDownTest()
	
	fileName := "test_partial_ops.txt"
	fullContent := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Write full content
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   []byte(fullContent),
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Read partial content from different offsets
	testCases := []struct {
		offset int64
		length int
		expected string
	}{
		{0, 10, fullContent[0:10]},
		{10, 10, fullContent[10:20]},
		{20, 5, fullContent[20:25]},
		{30, 6, fullContent[30:36]},
	}

	for _, tc := range testCases {
		handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
			Name:  fileName,
			Flags: os.O_RDONLY,
			Mode:  0777,
		})
		suite.assert.NoError(err)

		buffer := make([]byte, tc.length)
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: tc.offset,
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(tc.length, bytesRead)
		suite.assert.Equal(tc.expected, string(buffer))

		err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestBlockBoundaryOperations() {
	defer suite.TearDownTest()
	
	fileName := "test_block_boundary.txt"
	blockSize := 1024 * 1024 // 1MB (matching config)
	
	// Create content that spans multiple blocks
	content := make([]byte, blockSize*2+500)
	for i := range content {
		content[i] = byte(i % 256)
	}

	// Write content
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
		Handle: handle,
		Offset: 0,
		Data:   content,
	})
	suite.assert.NoError(err)

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Read at block boundaries
	testOffsets := []int64{
		0,
		int64(blockSize - 100),
		int64(blockSize),
		int64(blockSize + 100),
		int64(blockSize * 2),
	}

	for _, offset := range testOffsets {
		handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
			Name:  fileName,
			Flags: os.O_RDONLY,
			Mode:  0777,
		})
		suite.assert.NoError(err)

		readSize := 200
		if offset+int64(readSize) > int64(len(content)) {
			readSize = len(content) - int(offset)
		}

		buffer := make([]byte, readSize)
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: offset,
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(readSize, bytesRead)
		suite.assert.Equal(content[offset:offset+int64(readSize)], buffer)

		err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		suite.assert.NoError(err)
	}
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestLargeFileOperations() {
	defer suite.TearDownTest()
	
	fileName := "test_large_file.txt"
	size := 5 * 1024 * 1024 // 5MB
	
	// Create large content with pattern
	content := make([]byte, size)
	pattern := []byte("LARGE_FILE_TEST_PATTERN_")
	for i := 0; i < size; i++ {
		content[i] = pattern[i%len(pattern)]
	}

	// Write large file
	handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
		Name: fileName,
		Mode: 0777,
	})
	suite.assert.NoError(err)

	// Write in chunks to avoid memory issues
	chunkSize := 1024 * 1024 // 1MB chunks
	for offset := 0; offset < size; offset += chunkSize {
		endOffset := offset + chunkSize
		if endOffset > size {
			endOffset = size
		}

		_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
			Handle: handle,
			Offset: int64(offset),
			Data:   content[offset:endOffset],
		})
		suite.assert.NoError(err)
	}

	err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
	suite.assert.NoError(err)
	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Verify file size
	attr, err := suite.blockCache.GetAttr(internal.GetAttrOptions{Name: fileName})
	suite.assert.NoError(err)
	suite.assert.Equal(int64(size), attr.Size)

	// Read and verify random chunks
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 5; i++ {
		offset := rand.Int63n(int64(size - 1000))
		buffer := make([]byte, 1000)
		
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: offset,
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(1000, bytesRead)
		suite.assert.Equal(content[offset:offset+1000], buffer)
	}

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestSequentialVsRandomAccess() {
	defer suite.TearDownTest()
	
	fileName := "test_access_patterns.txt"
	size := 2 * 1024 * 1024 // 2MB
	
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	// Write file
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, content, 0777)
	suite.assert.NoError(err)

	// Test sequential access
	handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	chunkSize := 64 * 1024 // 64KB
	for offset := 0; offset < size; offset += chunkSize {
		readSize := chunkSize
		if offset+readSize > size {
			readSize = size - offset
		}

		buffer := make([]byte, readSize)
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: int64(offset),
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(readSize, bytesRead)
		suite.assert.Equal(content[offset:offset+readSize], buffer)
	}

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	// Test random access
	handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
		Name:  fileName,
		Flags: os.O_RDONLY,
		Mode:  0777,
	})
	suite.assert.NoError(err)

	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 20; i++ {
		offset := rand.Intn(size - chunkSize)
		buffer := make([]byte, chunkSize)
		
		bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
			Handle: handle,
			Offset: int64(offset),
			Data:   buffer,
		})
		suite.assert.NoError(err)
		suite.assert.Equal(chunkSize, bytesRead)
		suite.assert.Equal(content[offset:offset+chunkSize], buffer)
	}

	err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)
}

// ============================================================================
// Stress Tests
// ============================================================================

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestRapidOpenCloseCycles() {
	defer suite.TearDownTest()
	
	fileName := "test_rapid_cycles.txt"
	content := "Data for rapid open/close test."

	// Create file
	filePath := filepath.Join(suite.testPath, fileName)
	err := os.WriteFile(filePath, []byte(content), 0777)
	suite.assert.NoError(err)

	// Rapidly open and close file
	cycles := 100
	for i := 0; i < cycles; i++ {
		handle, err := suite.blockCache.OpenFile(internal.OpenFileOptions{
			Name:  fileName,
			Flags: os.O_RDONLY,
			Mode:  0777,
		})
		suite.assert.NoError(err, "Cycle %d: failed to open", i)

		// Optionally read
		if i%10 == 0 {
			buffer := make([]byte, len(content))
			_, err = suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
				Handle: handle,
				Offset: 0,
				Data:   buffer,
			})
			suite.assert.NoError(err, "Cycle %d: failed to read", i)
		}

		err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{
			Handle: handle,
		})
		suite.assert.NoError(err, "Cycle %d: failed to close", i)
	}
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestManyConcurrentFileOperations() {
	defer suite.TearDownTest()
	
	numFiles := 20
	numOpsPerFile := 5

	var wg sync.WaitGroup
	errors := make(chan error, numFiles*numOpsPerFile)

	for fileIdx := 0; fileIdx < numFiles; fileIdx++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			fileName := fmt.Sprintf("test_concurrent_file_%d.txt", idx)
			content := fmt.Sprintf("Content for file %d. ", idx)

			for op := 0; op < numOpsPerFile; op++ {
				// Create/Open
				handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
					Name: fileName,
					Mode: 0777,
				})
				if err != nil && !strings.Contains(err.Error(), "exist") {
					errors <- fmt.Errorf("file %d op %d: create failed: %v", idx, op, err)
					continue
				}
				if err != nil {
					// File exists, open it
					handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
						Name:  fileName,
						Flags: os.O_RDWR,
						Mode:  0777,
					})
					if err != nil {
						errors <- fmt.Errorf("file %d op %d: open failed: %v", idx, op, err)
						continue
					}
				}

				// Write
				_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
					Handle: handle,
					Offset: int64(op * len(content)),
					Data:   []byte(content),
				})
				if err != nil {
					errors <- fmt.Errorf("file %d op %d: write failed: %v", idx, op, err)
				}

				// Sync
				err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
				if err != nil {
					errors <- fmt.Errorf("file %d op %d: sync failed: %v", idx, op, err)
				}

				// Close
				err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
				if err != nil {
					errors <- fmt.Errorf("file %d op %d: close failed: %v", idx, op, err)
				}

				time.Sleep(time.Millisecond * 5)
			}
		}(fileIdx)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		suite.T().Logf("Error: %v", err)
		errorCount++
	}

	// Allow some errors but ensure most operations succeed
	suite.assert.Less(errorCount, numFiles*numOpsPerFile/2, "Too many operations failed")
}

func (suite *BlockCacheLoopbackIntegrationTestSuite) TestFileOperationsUnderMemoryPressure() {
	defer suite.TearDownTest()
	
	// Create multiple large files to stress memory
	numFiles := 10
	fileSize := 3 * 1024 * 1024 // 3MB each

	var wg sync.WaitGroup
	errors := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(fileIdx int) {
			defer wg.Done()

			fileName := fmt.Sprintf("test_memory_pressure_%d.txt", fileIdx)
			content := make([]byte, fileSize)
			for j := range content {
				content[j] = byte(fileIdx + (j % 256))
			}

			// Create and write
			handle, err := suite.blockCache.CreateFile(internal.CreateFileOptions{
				Name: fileName,
				Mode: 0777,
			})
			if err != nil {
				errors <- fmt.Errorf("file %d: create failed: %v", fileIdx, err)
				return
			}

			chunkSize := 512 * 1024 // 512KB chunks
			for offset := 0; offset < fileSize; offset += chunkSize {
				endOffset := offset + chunkSize
				if endOffset > fileSize {
					endOffset = fileSize
				}

				_, err = suite.blockCache.WriteFile(&internal.WriteFileOptions{
					Handle: handle,
					Offset: int64(offset),
					Data:   content[offset:endOffset],
				})
				if err != nil {
					errors <- fmt.Errorf("file %d: write at %d failed: %v", fileIdx, offset, err)
					break
				}
			}

			err = suite.blockCache.SyncFile(internal.SyncFileOptions{Handle: handle})
			if err != nil {
				errors <- fmt.Errorf("file %d: sync failed: %v", fileIdx, err)
			}

			err = suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
			if err != nil {
				errors <- fmt.Errorf("file %d: close failed: %v", fileIdx, err)
			}

			// Verify by reading back random chunk
			handle, err = suite.blockCache.OpenFile(internal.OpenFileOptions{
				Name:  fileName,
				Flags: os.O_RDONLY,
				Mode:  0777,
			})
			if err != nil {
				errors <- fmt.Errorf("file %d: reopen failed: %v", fileIdx, err)
				return
			}

			offset := rand.Intn(fileSize - 1000)
			buffer := make([]byte, 1000)
			bytesRead, err := suite.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{
				Handle: handle,
				Offset: int64(offset),
				Data:   buffer,
			})
			if err != nil || bytesRead != 1000 {
				errors <- fmt.Errorf("file %d: verification read failed: %v", fileIdx, err)
			} else if string(buffer) != string(content[offset:offset+1000]) {
				errors <- fmt.Errorf("file %d: data verification failed", fileIdx)
			}

			suite.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		suite.T().Logf("Error: %v", err)
		errorCount++
	}

	// Under memory pressure some operations may fail, but most should succeed
	suite.assert.Less(errorCount, numFiles/2, "Too many failures under memory pressure")
}

// ============================================================================
// Test Suite Runner
// ============================================================================

func TestBlockCacheLoopbackIntegrationSuite(t *testing.T) {
	suite.Run(t, new(BlockCacheLoopbackIntegrationTestSuite))
}
