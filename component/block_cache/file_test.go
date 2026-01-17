package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	f := createFile("test.txt")
	
	assert.NotNil(t, f)
	assert.Equal(t, "test.txt", f.Name)
	assert.Equal(t, int64(-1), f.size)
	assert.Equal(t, int64(-1), f.sizeOnStorage)
	assert.True(t, f.synced)
	assert.NotNil(t, f.handles)
	assert.Equal(t, 0, len(f.handles))
	assert.NotNil(t, f.blockList)
	assert.Equal(t, int32(0), f.numPendingReads.Load())
}

func TestFileUpdateFileSize(t *testing.T) {
	f := createFile("test.txt")
	f.size = 100
	
	// Update to larger size
	f.updateFileSize(200)
	assert.Equal(t, int64(200), f.size)
	
	// Try to update to smaller size - should not change
	f.updateFileSize(150)
	assert.Equal(t, int64(200), f.size, "Size should not decrease")
	
	// Update to same size
	f.updateFileSize(200)
	assert.Equal(t, int64(200), f.size)
	
	// Update to even larger size
	f.updateFileSize(300)
	assert.Equal(t, int64(300), f.size)
}

func TestFileUpdateFileSize_Concurrent(t *testing.T) {
	f := createFile("test.txt")
	f.size = 0
	
	// Simulate concurrent updates
	done := make(chan bool)
	for i := 1; i <= 10; i++ {
		go func(size int64) {
			f.updateFileSize(size)
			done <- true
		}(int64(i * 100))
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Final size should be the maximum
	assert.Equal(t, int64(1000), f.size)
}

// SUSPICIOUS FINDING: File size is initialized to -1, which could cause issues
// if not properly checked before arithmetic operations
func TestFileInitialSizeIsNegative(t *testing.T) {
	f := createFile("test.txt")
	assert.Equal(t, int64(-1), f.size, "Initial size is -1, operations must handle this")
	assert.Equal(t, int64(-1), f.sizeOnStorage, "Initial sizeOnStorage is -1")
}

// SUSPICIOUS FINDING: The synced flag starts as true for new files
// This might be intentional but seems counterintuitive
func TestFileInitialSyncedState(t *testing.T) {
	f := createFile("test.txt")
	assert.True(t, f.synced, "New file starts as synced=true, verify this is intended")
}

func TestFile_ErrorState(t *testing.T) {
	f := createFile("test.txt")
	
	// Initially no error
	assert.Nil(t, f.err.Load())
	
	// Store an error
	f.err.Store("test error")
	assert.NotNil(t, f.err.Load())
	assert.Equal(t, "test error", f.err.Load())
}

func TestFile_PendingReads(t *testing.T) {
	f := createFile("test.txt")
	
	assert.Equal(t, int32(0), f.numPendingReads.Load())
	
	// Simulate pending reads
	f.numPendingReads.Add(1)
	assert.Equal(t, int32(1), f.numPendingReads.Load())
	
	f.numPendingReads.Add(5)
	assert.Equal(t, int32(6), f.numPendingReads.Load())
	
	f.numPendingReads.Add(-6)
	assert.Equal(t, int32(0), f.numPendingReads.Load())
}

func TestFile_SingleBlockFilePersisted(t *testing.T) {
	f := createFile("test.txt")
	
	// Test initial state
	assert.False(t, f.singleBlockFilePersisted)
	
	// Simulate persisting as single block
	f.singleBlockFilePersisted = true
	assert.True(t, f.singleBlockFilePersisted)
}

func TestFile_BlockListInitialization(t *testing.T) {
	f := createFile("test.txt")
	
	assert.NotNil(t, f.blockList)
	assert.Equal(t, blockListNotRetrieved, f.blockList.state)
	assert.Equal(t, 0, len(f.blockList.list))
}

// SUSPICIOUS FINDING: The pendingWriters WaitGroup is used without initialization
// Go initializes it to zero, but explicit initialization would be clearer
func TestFile_PendingWritersInitialization(t *testing.T) {
	f := createFile("test.txt")
	
	// Add and wait should work without explicit initialization
	f.pendingWriters.Add(1)
	go func() {
		f.pendingWriters.Done()
	}()
	f.pendingWriters.Wait()
	// If we reach here, it means WaitGroup works correctly
}

func TestFile_EtagField(t *testing.T) {
	f := createFile("test.txt")
	
	// Initially empty
	assert.Equal(t, "", f.Etag)
	
	// Can be set
	f.Etag = "some-etag-value"
	assert.Equal(t, "some-etag-value", f.Etag)
}

func TestFile_NameField(t *testing.T) {
	f := createFile("test-file-name.txt")
	assert.Equal(t, "test-file-name.txt", f.Name)
	
	// Test with path
	f2 := createFile("path/to/file.txt")
	assert.Equal(t, "path/to/file.txt", f2.Name)
}

func TestFile_SizeFields(t *testing.T) {
	f := createFile("test.txt")
	
	// Test independent size fields
	f.size = 1000
	f.sizeOnStorage = 500
	
	assert.Equal(t, int64(1000), f.size)
	assert.Equal(t, int64(500), f.sizeOnStorage)
	
	// These should be independent
	f.size = 2000
	assert.Equal(t, int64(2000), f.size)
	assert.Equal(t, int64(500), f.sizeOnStorage, "sizeOnStorage should not change")
}
