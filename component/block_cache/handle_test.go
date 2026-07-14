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
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/stretchr/testify/assert"
)

func TestCreateFreshHandleForFile(t *testing.T) {
	now := time.Now()
	handle := createFreshHandleForFile("test.txt", 1024, now)

	assert.NotNil(t, handle)
	assert.Equal(t, "test.txt", handle.Path)
	assert.Equal(t, int64(1024), handle.Size)
	assert.Equal(t, now, handle.Mtime)
}

func TestGetFileFromPath_FirstOpen(t *testing.T) {
	cache := &BlockCache{}
	handle := handlemap.NewHandle("newfile.txt")

	f, firstOpen, err := getFileFromPath(cache, handle)
	assert.NoError(t, err)

	assert.NotNil(t, f)
	assert.True(t, firstOpen, "Should be first open")
	assert.Equal(t, "newfile.txt", f.Name)
	assert.Len(t, f.handles, 1, "Should have one handle")
	_, exists := f.handles[handle]
	assert.True(t, exists, "Handle should be in file's handle map")
}

func TestGetFileFromPath_SecondOpen(t *testing.T) {
	cache := &BlockCache{}
	// First open
	handle1 := handlemap.NewHandle("existingfile.txt")
	f1, firstOpen1, err := getFileFromPath(cache, handle1)
	assert.NoError(t, err)
	assert.True(t, firstOpen1)

	// Second open
	handle2 := handlemap.NewHandle("existingfile.txt")
	f2, firstOpen2, err := getFileFromPath(cache, handle2)
	assert.NoError(t, err)

	assert.False(t, firstOpen2, "Should not be first open")
	assert.Equal(t, f1, f2, "Should be same file object")
	assert.Len(t, f2.handles, 2, "Should have two handles")
	_, exists1 := f2.handles[handle1]
	_, exists2 := f2.handles[handle2]
	assert.True(t, exists1)
	assert.True(t, exists2)
}

func TestDeleteOpenHandleForFile_LastHandle(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	// Create file with one handle
	handle := handlemap.NewHandle("deleteme.txt")
	f, _, err := getFileFromPath(bc, handle)
	assert.NoError(t, err)

	handle.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Verify file is in map
	_, exists := bc.openFiles.Load("deleteme.txt")
	assert.True(t, exists)

	// Delete the handle
	deleteOpenHandleForFile(bc, handle, f, true)

	// File should be removed from map
	_, exists = bc.openFiles.Load("deleteme.txt")
	assert.False(t, exists, "File should be removed when last handle is closed")
}

func TestDeleteOpenHandleForFile_NotLastHandle(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	// Create file with two handles
	handle1 := handlemap.NewHandle("multi-handle.txt")
	f, _, err := getFileFromPath(bc, handle1)
	assert.NoError(t, err)

	handle2 := handlemap.NewHandle("multi-handle.txt")
	_, _, err = getFileFromPath(bc, handle2)
	assert.NoError(t, err)

	handle1.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Delete first handle
	deleteOpenHandleForFile(bc, handle1, f, true)

	// File should still be in map
	_, exists := bc.openFiles.Load("multi-handle.txt")
	assert.True(t, exists, "File should remain when other handles are open")

	// Should have one handle left
	assert.Len(t, f.handles, 1)
}

func TestCheckFileExistsInOpen_Exists(t *testing.T) {
	cache := &BlockCache{}
	// Create a file
	handle := handlemap.NewHandle("checkexists.txt")
	f, _, err := getFileFromPath(cache, handle)
	assert.NoError(t, err)

	// Check it exists
	foundFile, exists := checkFileExistsInOpen(cache, "checkexists.txt")

	assert.True(t, exists)
	assert.Equal(t, f, foundFile)
}

func TestCheckFileExistsInOpen_NotExists(t *testing.T) {
	cache := &BlockCache{}
	// Check non-existent file
	foundFile, exists := checkFileExistsInOpen(cache, "doesnotexist.txt")

	assert.False(t, exists)
	assert.Nil(t, foundFile)
}

func TestDeleteFileIfNoOpenHandles(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	// Create a file
	handle := handlemap.NewHandle("deleteifno.txt")
	f, _, err := getFileFromPath(bc, handle)
	assert.NoError(t, err)

	handle.IFObj = &blockCacheHandle{
		file:            f,
		patternDetector: newPatternDetector(),
	}

	// Close the handle
	deleteOpenHandleForFile(bc, handle, f, true)

	// File should already be deleted by deleteOpenHandleForFile
	_, exists := bc.openFiles.Load("deleteifno.txt")
	assert.False(t, exists)

	// Call deleteFileIfNoOpenHandles - should be no-op (not in map)
	deleteFileIfNoOpenHandles(bc, "deleteifno.txt")

	// Still should not exist
	_, exists = bc.openFiles.Load("deleteifno.txt")
	assert.False(t, exists)
}

func TestDeleteFileIfNoOpenHandles_WithEmptyHandles(t *testing.T) {
	cache := &BlockCache{}
	// Insert a file into the map with zero handles — simulates an orphaned entry
	f := createFile("orphan.txt")
	cache.openFiles.Store("orphan.txt", f)

	_, exists := cache.openFiles.Load("orphan.txt")
	assert.True(t, exists)

	// This should detect zero handles and remove the file from the map
	deleteFileIfNoOpenHandles(cache, "orphan.txt")

	_, exists = cache.openFiles.Load("orphan.txt")
	assert.False(t, exists, "File with no handles should be removed from map")
}

func TestDeleteFileIfNoOpenHandles_WithHandles(t *testing.T) {
	cache := &BlockCache{}
	// Insert a file into the map with one handle — should NOT delete
	f := createFile("has_handle.txt")
	handle := handlemap.NewHandle("has_handle.txt")
	f.handles[handle] = struct{}{}
	cache.openFiles.Store("has_handle.txt", f)

	deleteFileIfNoOpenHandles(cache, "has_handle.txt")

	_, exists := cache.openFiles.Load("has_handle.txt")
	assert.True(t, exists, "File with handles should not be removed")

	// Clean up
	cache.openFiles.Delete("has_handle.txt")
}

func TestGetFileFromPath_IsolatedByCacheInstance(t *testing.T) {
	firstCache := &BlockCache{}
	secondCache := &BlockCache{}
	first, _, err := getFileFromPath(firstCache, handlemap.NewHandle("same.txt"))
	assert.NoError(t, err)
	second, _, err := getFileFromPath(secondCache, handlemap.NewHandle("same.txt"))
	assert.NoError(t, err)
	assert.NotSame(t, first, second)
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
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	// Test basic concurrent access doesn't crash
	done := make(chan bool)
	for i := 0; i < 1000; i++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic in goroutine: %v", r)
				}
				done <- true
			}()

			handle := handlemap.NewHandle("racefile.txt")
			f, _, err := getFileFromPath(bc, handle)
			assert.NoError(t, err)
			handle.IFObj = &blockCacheHandle{
				file:            f,
				patternDetector: newPatternDetector(),
			}
			// Brief delay to allow overlapping access
			time.Sleep(time.Millisecond)
			deleteOpenHandleForFile(bc, handle, f, true)
		}(i)
	}

	for i := 0; i < 1000; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
}

func TestReleaseAllBuffersForFile_EmptyBlockList(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("empty.txt")

	// Release with no blocks - should not panic
	releaseAllBuffersForFile(bc, f)

	// Should work without error
}

func TestReleaseAllBuffersForFile_WithBlocks(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("withblocks.txt")

	// Add some blocks with buffers
	for i := 0; i < 3; i++ {
		blk := createBlock(i, "testId", localBlock, f)
		f.blockList.list = append(f.blockList.list, blk)

		bufDesc, err := freeList.allocateBuffer(blk)
		assert.NoError(t, err)
		bufDesc.refCnt.Store(1) // Table holds 1 reference
		bufDesc.valid.Store(true)

		btm.mu.Lock()
		btm.table[blk] = bufDesc
		btm.mu.Unlock()

		// Don't release here - releaseAllBuffersForFile will handle it
	}

	// Release all buffers
	releaseAllBuffersForFile(bc, f)

	// Block list should be nil
	assert.Nil(t, f.blockList)
}
