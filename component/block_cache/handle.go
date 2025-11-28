package block_cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type blockCacheHandle struct {
	file            *File
	patternDetector *patternDetector
}

func createFreshHandleForFile(name string, size int64, mtime time.Time, flags int) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	return handle
}

// Sync Map filepath->*File
var fileMap sync.Map

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

// Remove the handle from the file
// Release the buffers if the openFDcount is zero for the file
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

func checkFileExistsInOpen(key string) (*File, bool) {
	f, ok := fileMap.Load(key)
	if ok {
		return f.(*File), ok
	}
	return nil, ok
}
