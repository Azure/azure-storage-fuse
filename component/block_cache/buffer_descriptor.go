package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// bufferDescriptor tracks metadata and reference count for a memory buffer that caches block data.
// Each buffer is associated with a specific block of a file.
type bufferDescriptor struct {
	bufIdx        int    // Index of this buffer in the buffer pool
	block         *block // Pointer to the block this buffer caches
	nxtFreeBuffer int    // Index of next free buffer (used when buffer is in free list)

	// Reference count tracking how many concurrent users hold this buffer.
	// refCnt semantics:
	//   - When buffer is added to bufferTableMgr, refCnt is initialized to 1 (table holds a reference)
	//   - Each LookUp or GetOrCreate operation increments refCnt (user acquires a reference)
	//   - Each release() call decrements refCnt (user releases their reference)
	//   - When refCnt reaches 0, buffer has no users and can be returned to free list
	// Thread-safety: refCnt must be incremented while holding bufferTableMgr lock (shared or exclusive)
	refCnt atomic.Int32

	// Track buffer usage for eviction decisions
	bytesRead               atomic.Int32 // # of bytes read from this buffer (helps determine if buffer was used)
	bytesWritten            atomic.Int32 // # of bytes written to this buffer (helps determine if upload is needed)
	numEvictionCyclesPassed atomic.Int32 // # of eviction cycles this buffer has survived

	// Content synchronization lock for buffer data access
	// - Held exclusively during download/upload operations (modifying buffer content)
	// - Held in shared mode during read operations (multiple readers can proceed concurrently)
	contentLock sync.RWMutex

	// Buffer state and data
	buf         []byte      // Actual memory buffer holding block data
	valid       atomic.Bool // True if buffer contains valid data (download completed successfully)
	dirty       atomic.Bool // True if buffer has been modified and needs to be uploaded
	downloadErr error       // Captures any error that occurred during download
	uploadErr   error       // Captures any error that occurred during upload
}

func (bd *bufferDescriptor) String() string {
	return fmt.Sprintf("BufferDescriptor{bufIdx: %d, blockIdx: %d, refCnt: %d, bytesRead: %d, bytesWritten: %d, numEvictionCyclesPassed: %d, valid: %v, dirty: %v, downloadErr: %v, uploadErr: %v, file: %s}",
		bd.bufIdx,
		bd.block.idx,
		bd.refCnt.Load(),
		bd.bytesRead.Load(),
		bd.bytesWritten.Load(),
		bd.numEvictionCyclesPassed.Load(),
		bd.valid.Load(),
		bd.dirty.Load(),
		bd.downloadErr,
		bd.uploadErr,
		bd.block.file.Name)
}

// ensureBufferValidForRead verifies that the buffer contains valid data and is safe to read.
// This method handles synchronization with ongoing download operations.
//
// Return values:
//   - nil: buffer is valid and ready for reading
//   - downloadErr: buffer download failed, error details provided
//   - panic: buffer is in an inconsistent state (neither valid nor errored)
//
// Implementation details:
//   - If buffer is already valid, returns immediately
//   - If download is in progress, waits by acquiring and releasing contentLock (download holds it exclusively)
//   - After download completes, checks if data is valid or if an error occurred
func (bd *bufferDescriptor) ensureBufferValidForRead() error {
	if bd.valid.Load() {
		// Buffer is valid, safe to read.
		return nil
	}

	// Wait for the download to complete by acquiring the read lock.
	// If download is in progress, it holds the lock exclusively, so we wait here.
	// Once download completes and releases the lock, we can proceed.
	bd.contentLock.RLock()
	bd.contentLock.RUnlock()

	if bd.valid.Load() && bd.downloadErr == nil {
		// Download completed successfully, buffer is now valid and safe to read.
		return nil
	}

	if !bd.valid.Load() && bd.downloadErr != nil {
		// Download failed, return the error to the caller.
		return bd.downloadErr
	}

	// Inconsistent state: buffer is not valid but also has no error.
	// This should never happen and indicates a bug in the download logic.
	err := fmt.Sprintf("bufferDescriptor::ensureBufferValidForRead: Inconsistent state for bufferIdx: %d, blockIdx: %d, valid: %v, downloadErr: %v, file: %s",
		bd.bufIdx, bd.block.idx, bd.valid.Load(), bd.downloadErr, bd.block.file.Name)
	panic(err)
}

// release decrements the reference count for this buffer descriptor.
// When refCnt reaches 0, the buffer is returned to the free list for reuse.
//
// Return value:
//   - true: buffer was released to free list (refCnt reached 0)
//   - false: buffer still has active references (refCnt > 0)
//
// Reference counting semantics:
//   - Each user (including the buffer table) holds a counted reference
//   - Buffer table holds refCnt=1 when buffer is only in the table
//   - Additional users increment refCnt (e.g., reads, writes, lookups)
//   - When buffer is removed from table AND all users release, refCnt reaches 0
//   - refCnt=0 means buffer can be safely recycled
//
// Thread-safety: Uses atomic operations for refCnt manipulation.
// Panics if refCnt goes negative, indicating a reference counting bug.
func (bd *bufferDescriptor) release() bool {
	newRefCnt := bd.refCnt.Add(-1)

	if newRefCnt == 0 {
		// No more references exist (table removed it and all users released).
		// Safe to return buffer to free list for reuse.
		log.Debug("bufferDescriptor::release: Releasing bufferIdx: %d for blockIdx: %d back to free list, bytesRead: %d, bytesWritten: %d, file: %s",
			bd.bufIdx, bd.block.idx, bd.bytesRead.Load(), bd.bytesWritten.Load(), bd.block.file.Name)
		freeList.releaseBuffer(bd)
		return true
	} else if newRefCnt < 0 {
		// Negative refCnt indicates a bug: release() called more times than acquire.
		// This should never happen and represents a serious reference counting error.
		err := fmt.Sprintf("bufferDescriptor::release: bufferIdx: %d for blockIdx: %d has negative refCount: %d, bytesRead: %d, bytesWritten: %d, file: %s",
			bd.bufIdx, bd.block.idx, bd.refCnt.Load(), bd.bytesRead.Load(), bd.bytesWritten.Load(), bd.block.file.Name)
		log.Err(err)
		panic(err)
	}

	// Buffer still has active references, not ready for release yet.
	return false
}

// reset clears all fields of the buffer descriptor and zeros the buffer content.
// This prepares the buffer for reuse by a different block.
// Called when buffer is returned to free list or when a victim buffer is being reassigned.
func (bd *bufferDescriptor) reset() {
	bd.block = nil
	bd.nxtFreeBuffer = -1
	bd.refCnt.Store(0)
	bd.bytesRead.Store(0)
	bd.bytesWritten.Store(0)
	bd.numEvictionCyclesPassed.Store(0)
	bd.valid.Store(false)
	bd.dirty.Store(false)
	bd.downloadErr = nil
	bd.uploadErr = nil
	// Zero out the buffer content for security and consistency
	copy(bd.buf, freeList.bufPool.GetZeroBuffer())
}
