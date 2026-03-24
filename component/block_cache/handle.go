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
func getFileFromPath(handle *handlemap.Handle) (*File, bool, error) {
	const maxRetries = 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		f := createFile(handle.Path)
		file, loaded := fileMap.LoadOrStore(handle.Path, f)
		fileObj, ok := file.(*File)
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
func deleteOpenHandleForFile(bc *BlockCache, handle *handlemap.Handle, file *File, takeFileLock bool) {
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
func releaseAllBuffersForFile(bc *BlockCache, file *File) {
	log.Debug("releaseAllBuffersForFile: Releasing all buffers for file %s", file.Name)
	// Release all buffers held by this file
	for _, blk := range file.blockList.list {
		bufDesc, _ := bc.btm.LookUpBufferDescriptor(blk)
		if bufDesc == nil {
			continue
		}

		// Ensure the buffer is valid for read before releasing, if read-ahead is in progress, wait for it to complete.
		bufDesc.ensureBufferValidForRead()

		log.Debug("releaseAllBuffersForFile: Releasing bufferIdx: %d for blockIdx: %d of file %s from buffer table manager",
			bufDesc.bufIdx, blk.idx, file.Name)

		if ok := bc.btm.removeBufferDescriptor(bufDesc, bc.freeList); !ok {
			// This should always succeed because where we are at the release, there shouldn't be any active references
			// to the buffer (all handles are closed), so it must be present in the buffer table manager and must have
			// refCnt exactly equal to refCntTableAndOneUser.
			//
			// This buffer may get chosen as victim, so max refCnt can be refCntTableAndOneUser+1 (the +1 is for the victim selection algo).
			// If it's more than that, it indicates a bug in reference counting or buffer lifecycle management.
			if bufDesc.refCnt.Load() > refCountTableAndOneUser+1 {
				panic(fmt.Sprintf("releaseAllBuffersForFile: Failed to remove buffer: [%v], for blockIdx: %d of file %s from buffer table manager",
					bufDesc, blk.idx, file.Name))
			}

			// TODO: force remove such buffers, currently force removal is not present which could cause leak.
			if bufDesc.dirty.Load() {
				// This means upload has failed for the buffer, release the buffer for now, this buffer would be collected
				// by the victim selection algo later.
				log.Warn("releaseAllBuffersForFile: BufferIdx: %d [%v] for blockIdx: %d of file %s is dirty, which indicates upload failure",
					bufDesc.bufIdx, bufDesc, blk.idx, file.Name)
			}

			if ok := bufDesc.release(bc.freeList); ok {
				log.Debug("releaseAllBuffersForFile: BufferIdx: %d for blockIdx: %d of file %s released to free list",
					bufDesc.bufIdx, blk.idx, file.Name)
			}
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
		return f.(*File), true
	}
	return nil, false
}

func renameFileInFileMap(oldPath, newPath string) error {
	value, ok := fileMap.Load(oldPath)
	if !ok {
		return fmt.Errorf("file not found for path: %s", oldPath)
	}

	fileObj, ok := value.(*File)
	if !ok {
		return fmt.Errorf("invalid file type in map for path: %s", oldPath)
	}

	fileObj.mu.Lock()
	fileObj.Name = newPath
	fileObj.mu.Unlock()

	// Attempt to store the file with the new path
	_, loaded := fileMap.LoadOrStore(newPath, fileObj)
	if loaded {
		return fmt.Errorf("a file already exists for the new path: %s", newPath)
	}

	// Remove the old path from the map
	fileMap.Delete(oldPath)
	return nil
}
