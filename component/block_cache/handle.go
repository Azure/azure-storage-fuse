package block_cache

import (
	"fmt"
	"sync"
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
	file            *File            // Shared file object (same for all handles to this path)
	patternDetector *patternDetector // Per-handle access pattern tracking for read-ahead
}

// createFreshHandleForFile creates a new handle for a file with initial metadata.
//
// Parameters:
//   - name: File path
//   - size: Initial file size
//   - mtime: Last modification time
//   - flags: Open flags (O_RDONLY, O_RDWR, etc.)
//
// Returns a new handle ready for BlockCache operations.
//
// This handle will later have its IFObj field populated with a blockCacheHandle.
func createFreshHandleForFile(name string, size int64, mtime time.Time, flags int) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	return handle
}

// fileMap is a global map tracking all files with open handles.
//
// Map: filepath (string) -> *File
//
// Thread Safety: sync.Map provides built-in concurrency safety.
//
// Lifecycle:
//   - File added on first open (via getFileFromPath)
//   - File shared across multiple opens of the same path
//   - File removed when last handle closes (via deleteOpenHandleForFile)
var fileMap sync.Map

// getFileFromPath retrieves or creates a File object for the given handle.
//
// This function implements a thread-safe "get or create" pattern:
//
//  1. Try to load existing File from fileMap
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
func getFileFromPath(handle *handlemap.Handle) (*File, bool) {
	f := createFile(handle.Path)
	var first_open bool = false

retry:
	file, loaded := fileMap.LoadOrStore(handle.Path, f)
	if !loaded {
		first_open = true
		log.Debug("getFileFromPath: File %s not found in fileMap, stored new file: %v", handle.Path, f)

		f.mu.Lock()
		f.handles[handle] = struct{}{}
		f.mu.Unlock()
	} else {
		f2 := file.(*File)

		f2.mu.Lock()
		if len(f2.handles) == 0 {
			// previous handle released the file, after we loaded it from fileMap
			log.Err("getFileFromPath: File %s found in fileMap but has zero open handles, race with deleting the handles, file: %v",
				handle.Path, f2)
			f2.mu.Unlock()
			goto retry
		} else {
			log.Debug("getFileFromPath: File %s found in fileMap with open handles, proceeding, file: %v", handle.Path, f2)
			f2.handles[handle] = struct{}{}
		}
		f2.mu.Unlock()
	}

	return file.(*File), first_open
}

// deleteOpenHandleForFile removes a handle from the file's handle set.
//
// This function is called when a handle is released (closed). It performs:
//
//  1. Removes handle from file's handle set
//  2. If last handle: removes file from fileMap and releases all buffers
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
func deleteOpenHandleForFile(handle *handlemap.Handle, takeFileLock bool) {
	file := handle.IFObj.(*blockCacheHandle).file
	log.Debug("deleteOpenHandleForFile: Deleting handle: %d for file %s", handle.ID, file.Name)

	if takeFileLock {
		file.mu.Lock()
	}

	delete(file.handles, handle)

	if len(file.handles) == 0 {
		fileMap.Delete(file.Name)
		if takeFileLock {
			file.mu.Unlock()
		}

		releaseAllBuffersForFile(file)
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
// file is removed from fileMap, ensuring no new operations can start.
//
// Panics:
//
// This function panics if:
//   - Buffer is released to free list during lookup (indicates double-free bug)
//   - Buffer cannot be removed from table (indicates refcount bug)
//
// These panics indicate serious bugs in reference counting or buffer lifecycle
// management and help catch correctness issues during development.
func releaseAllBuffersForFile(file *File) {
	log.Debug("releaseAllBuffersForFile: Releasing all buffers for file %s", file.Name)
	// Release all buffers held by this file
	for _, blk := range file.blockList.list {
		bufDesc, _ := btm.LookUpBufferDescriptor(blk)
		if bufDesc == nil {
			continue
		}

		// Release the reference held just now for lookup
		if ok := bufDesc.release(); ok {
			// It is present in buffer table manager, so should not be released here
			panic(fmt.Sprintf("releaseAllBuffersForFile: Released bufferIdx: %d for blockIdx: %d of file %s back to free list during lookup release",
				bufDesc.bufIdx, blk.idx, file.Name))
		}

		// Ensure the buffer is valid for read before releasing, if read-ahead is in progress, wait for it to complete.
		bufDesc.ensureBufferValidForRead()

		log.Debug("releaseAllBuffersForFile: Releasing bufferIdx: %d for blockIdx: %d of file %s from buffer table manager",
			bufDesc.bufIdx, blk.idx, file.Name)

		if ok1, ok2 := btm.removeBufferDescriptor(bufDesc, true /*strict*/); !ok1 || !ok2 {
			// There should be no more readers for this buffer descriptor, that mean it should always release the buffer
			// descriptor successfully here.
			panic(fmt.Sprintf("releaseAllBuffersForFile: Failed to remove buffer: [%v], refCnt: %d, for blockIdx: %d of file %s from buffer table manager, isRemovedFromBufMgr: %v, isReleasedToFreeList: %v",
				bufDesc, bufDesc.refCnt.Load(), blk.idx, file.Name, ok1, ok2))
		}

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
func deleteFileIfNoOpenHandles(key string) {
	file, ok := checkFileExistsInOpen(key)
	if !ok {
		return
	}

	if len(file.handles) == 0 {
		fileMap.Delete(file.Name)
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
func checkFileExistsInOpen(key string) (*File, bool) {
	f, ok := fileMap.Load(key)
	if ok {
		return f.(*File), ok
	}
	return nil, ok
}
