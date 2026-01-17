package block_cache

import (
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/stretchr/testify/assert"
)

func TestCreateFreshHandleForFile(t *testing.T) {
	now := time.Now()
	handle := createFreshHandleForFile("test.txt", 1024, now, 0)

	assert.NotNil(t, handle)
	assert.Equal(t, "test.txt", handle.Path)
	assert.Equal(t, int64(1024), handle.Size)
	assert.Equal(t, now, handle.Mtime)
}

func TestGetFileFromPath_FirstOpen(t *testing.T) {
	// Clear the file map
	fileMap = fileMap // Reset doesn't work, but we can use a clean test

	handle := handlemap.NewHandle("newfile.txt")

	f, firstOpen := getFileFromPath(handle)

	assert.NotNil(t, f)
	assert.True(t, firstOpen, "Should be first open")
	assert.Equal(t, "newfile.txt", f.Name)
	assert.Equal(t, 1, len(f.handles), "Should have one handle")
	_, exists := f.handles[handle]
	assert.True(t, exists, "Handle should be in file's handle map")
}

func TestGetFileFromPath_SecondOpen(t *testing.T) {
	// First open
	handle1 := handlemap.NewHandle("existingfile.txt")
	f1, firstOpen1 := getFileFromPath(handle1)
	assert.True(t, firstOpen1)

	// Second open
	handle2 := handlemap.NewHandle("existingfile.txt")
	f2, firstOpen2 := getFileFromPath(handle2)

	assert.False(t, firstOpen2, "Should not be first open")
	assert.Equal(t, f1, f2, "Should be same file object")
	assert.Equal(t, 2, len(f2.handles), "Should have two handles")
	_, exists1 := f2.handles[handle1]
	_, exists2 := f2.handles[handle2]
	assert.True(t, exists1)
	assert.True(t, exists2)
}

func TestDeleteOpenHandleForFile_LastHandle(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	// Create file with one handle
	handle := handlemap.NewHandle("deleteme.txt")
	f, _ := getFileFromPath(handle)

	handle.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Verify file is in map
	_, exists := fileMap.Load("deleteme.txt")
	assert.True(t, exists)

	// Delete the handle
	deleteOpenHandleForFile(handle, true)

	// File should be removed from map
	_, exists = fileMap.Load("deleteme.txt")
	assert.False(t, exists, "File should be removed when last handle is closed")
}

func TestDeleteOpenHandleForFile_NotLastHandle(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	// Create file with two handles
	handle1 := handlemap.NewHandle("multi-handle.txt")
	f, _ := getFileFromPath(handle1)

	handle2 := handlemap.NewHandle("multi-handle.txt")
	getFileFromPath(handle2)

	handle1.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Delete first handle
	deleteOpenHandleForFile(handle1, true)

	// File should still be in map
	_, exists := fileMap.Load("multi-handle.txt")
	assert.True(t, exists, "File should remain when other handles are open")

	// Should have one handle left
	assert.Equal(t, 1, len(f.handles))
}

func TestCheckFileExistsInOpen_Exists(t *testing.T) {
	// Create a file
	handle := handlemap.NewHandle("checkexists.txt")
	f, _ := getFileFromPath(handle)

	// Check it exists
	foundFile, exists := checkFileExistsInOpen("checkexists.txt")

	assert.True(t, exists)
	assert.Equal(t, f, foundFile)
}

func TestCheckFileExistsInOpen_NotExists(t *testing.T) {
	// Check non-existent file
	foundFile, exists := checkFileExistsInOpen("doesnotexist.txt")

	assert.False(t, exists)
	assert.Nil(t, foundFile)
}

func TestDeleteFileIfNoOpenHandles(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	// Create a file
	handle := handlemap.NewHandle("deleteifno.txt")
	f, _ := getFileFromPath(handle)

	handle.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Close the handle
	deleteOpenHandleForFile(handle, true)

	// File should already be deleted by deleteOpenHandleForFile
	_, exists := fileMap.Load("deleteifno.txt")
	assert.False(t, exists)

	// Call deleteFileIfNoOpenHandles - should be no-op
	deleteFileIfNoOpenHandles("deleteifno.txt")

	// Still should not exist
	_, exists = fileMap.Load("deleteifno.txt")
	assert.False(t, exists)
}

func TestBlockCacheHandle_Structure(t *testing.T) {
	f := createFile("test.txt")
	pd := newPatternDetector()

	bch := &blockCacheHandle{
		file:            f,
		patternDetector: pd,
	}

	assert.Equal(t, f, bch.file)
	assert.Equal(t, pd, bch.patternDetector)
}

// SUSPICIOUS FINDING: getFileFromPath has a retry mechanism for race conditions
// This handles the case where a file is being deleted while another thread tries to open it
func TestGetFileFromPath_RaceCondition(t *testing.T) {
	// This test documents the retry logic in getFileFromPath
	// The actual race is hard to reproduce reliably, but we can verify basic concurrent safety

	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	// Test basic concurrent access doesn't crash
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in goroutine: %v", r)
				}
				done <- true
			}()

			handle := handlemap.NewHandle("racefile.txt")
			f, _ := getFileFromPath(handle)
			handle.IFObj = &blockCacheHandle{
				file:            f,
				patternDetector: newPatternDetector(),
			}
			// Brief delay to allow overlapping access
			time.Sleep(time.Millisecond)
			deleteOpenHandleForFile(handle, true)
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
}

func TestReleaseAllBuffersForFile_EmptyBlockList(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("empty.txt")

	// Release with no blocks - should not panic
	releaseAllBuffersForFile(f)

	// Should work without error
}

func TestReleaseAllBuffersForFile_WithBlocks(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	err := createFreeList(bc.blockSize, 10*bc.blockSize)
	assert.NoError(t, err)
	defer destroyFreeList()

	NewBufferTableMgr()

	f := createFile("withblocks.txt")

	// Add some blocks with buffers
	for i := 0; i < 3; i++ {
		blk := createBlock(i, "testId", localBlock, f)
		f.blockList.list = append(f.blockList.list, blk)

		bufDesc, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
		bufDesc.refCnt.Store(1)
		bufDesc.valid.Store(true)

		btm.mu.Lock()
		btm.table[blk] = bufDesc
		btm.mu.Unlock()

		// Release the buffer so refCnt is 0, making it eligible for removal
		bufDesc.release()
	}

	// Release all buffers
	releaseAllBuffersForFile(f)

	// Block list should be nil
	assert.Nil(t, f.blockList)
}
