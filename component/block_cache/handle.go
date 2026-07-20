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
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// blockCacheHandle is the interface-specific handle data for BlockCache.
//
// Each file handle opened through BlockCache has an associated blockCacheHandle
// that stores BlockCache-specific state. This is stored in Handle.IFObj.
//
// Fields:
//   - file: Reference to the shared File object for this path
//   - patternDetector: Per-handle access pattern detector for read-ahead optimization
//
// Why per-handle pattern detection:
// Different handles to the same file may have different access patterns.
// For example, one thread might read sequentially while another reads randomly.
// Per-handle detection allows independent read-ahead behavior for each handle.
type blockCacheHandle struct {
	file            *file            // Shared file object (same for all handles to this path)
	patternDetector *patternDetector // Per-handle access pattern tracking for read-ahead
	openFlags       int              // Access mode and other flags supplied to open(2)
}

// createFreshHandleForFile creates a new handle for a file with initial metadata.
//
// Parameters:
//   - name: File path
//   - size: Initial file size
//   - mtime: Last modification time
//
// Returns a new handle ready for BlockCache operations.
//
// This handle will later have its IFObj field populated with a blockCacheHandle.
func createFreshHandleForFile(name string, size int64, mtime time.Time) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	return handle
}

// getFileFromPath retrieves or creates a File object for the given handle.
//
// This function implements a thread-safe "get or create" pattern:
//
//  1. Try to load existing File from the cache instance's openFiles map
//  2. If not found, create new File and store in map
//  3. Add handle to File's handle set
//  4. Handle race condition where File was closed between load and store
//
// Parameters:
//   - handle: File handle to associate with the File
//
// Returns:
//   - *File: The File object for this path (new or existing)
//   - bool: true if this is the first open (File was created), false otherwise
//
// Race Condition Handling:
//
// There's a subtle race: another goroutine might close the last handle and
// remove the File from the map between our Load and Store operations.
// We detect this by checking if the File has zero handles after we load it.
// If so, we retry the entire operation.
//
// Thread Safety:
//
// Multiple goroutines can call this concurrently for the same path. The
// sync.Map and File mutex ensure correct behavior:
//   - Only one goroutine creates a new File for a given path
//   - All goroutines correctly add their handles to the File
//   - No handles are lost due to race conditions
func getFileFromPath(bc *BlockCache, handle *handlemap.Handle) (*file, bool, error) {
	const maxRetries = 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		f := createFile(handle.Path)
		existing, loaded := bc.openFiles.LoadOrStore(handle.Path, f)
		fileObj, ok := existing.(*file)
		if !ok {
			return nil, false, fmt.Errorf("invalid file type in map")
		}

		fileObj.mu.Lock()

		if len(fileObj.handles) == 0 && loaded {
			// File is being deleted/ some other handle is in race for creation,
			// retry with backoff
			fileObj.mu.Unlock()
			time.Sleep(time.Millisecond * time.Duration(attempt+1))
			continue
		}

		fileObj.handles[handle] = struct{}{}
		firstOpen := !loaded
		fileObj.mu.Unlock()

		return fileObj, firstOpen, nil
	}

	return nil, false, fmt.Errorf("failed to get file after %d retries", maxRetries)
}

// deleteOpenHandleForFile removes a handle from the file's handle set.
//
// This function is called when a handle is released (closed). It performs:
//
//  1. Removes handle from file's handle set
//  2. If last handle: removes file from openFiles and releases all buffers
//  3. If not last handle: file remains for other open handles
//
// Parameters:
//   - handle: Handle to remove
//   - takeFileLock: If true, acquires file lock; if false, assumes lock held
//
// Buffer Release:
//
// When the last handle is closed, all cached blocks for the file are released
// back to the free list. This ensures cache space is reclaimed for other files.
//
// Thread Safety:
//
// Multiple handles can be closed concurrently. The file mutex (if taken) and
// atomic map operations ensure correct behavior.
//
// Important: This function must be called for every handle, exactly once,
// to prevent buffer leaks and maintain correct reference counts.
func deleteOpenHandleForFile(bc *BlockCache, handle *handlemap.Handle, file *file, takeFileLock bool) {
	log.Debug("deleteOpenHandleForFile: Deleting handle: %d for file %s", handle.ID, file.Name)

	if takeFileLock {
		file.mu.Lock()
	}

	delete(file.handles, handle)

	if len(file.handles) == 0 {
		bc.openFiles.CompareAndDelete(file.Name, file)
		if takeFileLock {
			file.mu.Unlock()
		}

		releaseAllBuffersForFile(bc, file)
		return
	}

	if takeFileLock {
		file.mu.Unlock()
	}
}

// releaseAllBuffersForFile releases all cached blocks for a file.
//
// This function is called when the last handle to a file is closed. It:
//
//  1. Iterates through all blocks in the file's block list
//  2. Looks up each block's buffer (if cached)
//  3. Waits for any async operations to complete
//  4. Removes buffer from buffer table manager
//  5. Releases buffer back to free list
//
// Parameters:
//   - file: File whose buffers should be released
//
// Thread Safety:
//
// This function should be called only after all handles are closed and the
// file is removed from openFiles, ensuring no new operations can start.
//
// Panics:
//
// This function panics if:
//   - Buffer is released to free list during lookup (indicates double-free bug)
//   - Buffer cannot be removed from table (indicates refcount bug)
//
// These panics indicate serious bugs in reference counting or buffer lifecycle
// management and help catch correctness issues during development.
func releaseAllBuffersForFile(bc *BlockCache, file *file) {
	log.Debug("releaseAllBuffersForFile: Releasing all buffers for file %s", file.Name)
	// Release all buffers held by this file
	for _, blk := range file.blockList.list {
		bufDesc, _ := bc.btm.lookupBufferDescriptor(blk, bc.freeList)
		if bufDesc == nil {
			continue
		}

		// Ensure the buffer is valid for read before releasing, if read-ahead is in progress, wait for it to complete.
		if err := bufDesc.ensureBufferValidForRead(); err != nil {
			log.Warn("releaseAllBuffersForFile: BufferIdx: %d for blockIdx: %d of file %s is not valid for read",
				bufDesc.bufIdx, blk.idx, file.Name)
			// Continue with releasing the buffer, this buffer would be collected by the victim selection algo later.
		}

		log.Debug("releaseAllBuffersForFile: Releasing bufferIdx: %d for blockIdx: %d of file %s from buffer table manager",
			bufDesc.bufIdx, blk.idx, file.Name)

		if bc.btm.detachBufferDescriptor(bufDesc, bc.freeList) {
			log.Debug("releaseAllBuffersForFile: Detached bufferIdx: %d for blockIdx: %d of file %s",
				bufDesc.bufIdx, blk.idx, file.Name)
		}
		bufDesc.release(bc.freeList)

		log.Debug("releaseAllBuffersForFile: Released bufferIdx: %d for blockIdx: %d of file %s",
			bufDesc.bufIdx, blk.idx, file.Name)
	}

	// Clear the block list
	file.blockList = nil
}

// deleteFileIfNoOpenHandles removes a file from the map if it has no open handles.
//
// This is a utility function for cleanup. Currently not actively used but
// provided for completeness.
//
// Parameters:
//   - key: File path to check
func deleteFileIfNoOpenHandles(bc *BlockCache, key string) {
	file, ok := checkFileExistsInOpen(bc, key)
	if !ok {
		return
	}

	if len(file.handles) == 0 {
		bc.openFiles.CompareAndDelete(file.Name, file)
		// TODO: Release the buffers held by this file
	}
}

// checkFileExistsInOpen checks if a file has any open handles.
//
// Parameters:
//   - key: File path to check
//
// Returns:
//   - *File: The file object if it exists
//   - bool: true if file exists in map, false otherwise
//
// This is useful for checking file state without modifying the map.
func checkFileExistsInOpen(bc *BlockCache, key string) (*file, bool) {
	f, ok := bc.openFiles.Load(key)
	if ok {
		return f.(*file), true
	}
	return nil, false
}

func renameFileInFileMap(bc *BlockCache, oldPath, newPath string) error {
	value, ok := bc.openFiles.Load(oldPath)
	if !ok {
		return fmt.Errorf("file not found for path: %s", oldPath)
	}

	fileObj, ok := value.(*file)
	if !ok {
		return fmt.Errorf("invalid file type in map for path: %s", oldPath)
	}

	fileObj.mu.Lock()
	fileObj.Name = newPath
	fileObj.mu.Unlock()

	bc.openFiles.CompareAndDelete(oldPath, fileObj)
	bc.openFiles.Store(newPath, fileObj)
	return nil
}

func renameOpenFilesInDirectory(bc *BlockCache, oldDir, newDir string) {
	oldPrefix := strings.TrimSuffix(oldDir, "/") + "/"
	newPrefix := strings.TrimSuffix(newDir, "/") + "/"

	bc.openFiles.Range(func(key, value any) bool {
		oldPath, ok := key.(string)
		if !ok || !strings.HasPrefix(oldPath, oldPrefix) {
			return true
		}
		fileObj, ok := value.(*file)
		if !ok {
			return true
		}

		newPath := newPrefix + strings.TrimPrefix(oldPath, oldPrefix)
		fileObj.mu.Lock()
		fileObj.Name = newPath
		fileObj.mu.Unlock()
		bc.openFiles.CompareAndDelete(oldPath, fileObj)
		bc.openFiles.Store(newPath, fileObj)
		return true
	})
}
