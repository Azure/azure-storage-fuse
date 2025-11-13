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

func getFileFromPath(key string) (*File, bool) {
	f := createFile(key)
	var first_open bool = false
	retryCnt := 0
retry:
	file, loaded := fileMap.LoadOrStore(key, f)
	if !loaded {
		first_open = true
	} else {
		f2 := file.(*File)
		retryNeeded := false

		f2.mu.Lock()
		if len(f2.handles) == 0 {
			retryCnt++
			retryNeeded = true
			log.Err("getFileFromPath: File %s found in fileMap but has zero open handles, race with deleting the handles, retryCnt: %d, file: %v",
				key, retryCnt, f2)
		} else {
			log.Debug("getFileFromPath: File %s found in fileMap with open handles, proceeding, file: %v", key, f2)
		}
		f2.mu.Unlock()

		if !retryNeeded {
			return f2, first_open
		}

		if retryCnt > 100 {
			// Race when release and open happens together, should not happen normally
			panic(fmt.Sprintf("getFileFromPath: Too many retries(%d) for file %s getting zero open handles, something is wrong",
				retryCnt, key))
			return nil, false
		}
		goto retry
	}

	return file.(*File), first_open
}

// Remove the handle from the file
// Release the buffers if the openFDcount is zero for the file
func deleteOpenHandleForFile(handle *handlemap.Handle) {
	file := handle.IFObj.(*blockCacheHandle).file
	log.Debug("deleteOpenHandleForFile: Deleting handle for file %s: %v", file.Name, file)

	file.mu.Lock()
	delete(file.handles, handle)

	if len(file.handles) == 0 {
		fileMap.Delete(file.Name)
		file.mu.Unlock()
		releaseAllBuffersForFile(file)
		return
	}

	file.mu.Unlock()
}

func releaseAllBuffersForFile(file *File) {
	log.Debug("releaseAllBuffersForFile: Releasing all buffers for file %s", file.Name)
	// Release all buffers held by this file
	for _, blk := range file.blockList.list {
		bufDesc, _ := btm.LookUpBufferDescriptor(blk, 0 /*bytesInterested*/)
		if bufDesc == nil {
			continue
		}

		// Release the reference held just now for lookup
		if ok := bufDesc.release(); ok {
			// It is present in buffer table manager, so should not be released here
			panic(fmt.Sprintf("releaseAllBuffersForFile: Released bufferIdx: %d for blockIdx: %d of file %s back to free list during lookup release",
				bufDesc.bufIdx, blk.idx, file.Name))
		}

		if ok := btm.removeBufferDescriptor(bufDesc); !ok {
			// There should be no more readers for this buffer descriptor, that mean it should always release the buffer
			// descriptor successfully here.
			panic(fmt.Sprintf("releaseAllBuffersForFile: Failed to remove bufferIdx: %d for blockIdx: %d of file %s from buffer table manager",
				bufDesc.bufIdx, blk.idx, file.Name))
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

	file.mu.Lock()
	defer file.mu.Unlock()

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
