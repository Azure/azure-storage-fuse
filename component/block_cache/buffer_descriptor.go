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
	"math/bits"
	"os"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

const (
	// Reference counting
	refCountTableOnly       = 1
	refCountTableAndOneUser = 2
)

var (
	writeCoverageGranularity, writeCoverageShift = writeCoverageGeometry()
	writeCoveragePageMask                        = writeCoverageGranularity - 1
)

func writeCoverageGeometry() (pageSize, pageShift int) {
	pageSize = os.Getpagesize()
	if pageSize <= 0 || pageSize&(pageSize-1) != 0 {
		panic(fmt.Sprintf("block cache requires a power-of-two page size, got %d", pageSize))
	}
	return pageSize, bits.TrailingZeros(uint(pageSize))
}

// bufferDescriptor tracks metadata and reference count for a memory buffer that caches block data.
// Each buffer is associated with a specific block of a file.
type bufferDescriptor struct {
	block *block // Pointer to the block this buffer caches
	buf   []byte // Actual memory buffer holding block data

	bufIdx        int // Index of this buffer in the buffer pool
	nxtFreeBuffer int // Index of next free buffer (used when buffer is in free list)

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
	numEvictionCyclesPassed atomic.Int32 // # of eviction cycles this buffer has survived, heuristic used in victim selection

	// Content synchronization lock for buffer data access
	// - Held exclusively during download/upload operations (modifying buffer content)
	// - Held in shared mode during read operations (multiple readers can proceed concurrently)
	contentLock sync.RWMutex

	valid            atomic.Bool // True if buffer contains valid data (download completed successfully)
	dirty            atomic.Bool // True if buffer has been modified and needs to be uploaded
	downloadErr      error       // Captures any error that occurred during download
	uploadErr        error       // Captures any error that occurred during upload
	writeCoverage    []uint64    // One bit per fully written OS page; protected by contentLock
	writeRegionCount int         // Fixed number of pages represented by this buffer
	coveredRegions   int         // Number of bits set in writeCoverage; protected by contentLock
}

// bufferContentLease represents exclusive ownership of a descriptor's content.
// A lease may be transferred to a worker task, but it must be released exactly once.
type bufferContentLease struct {
	bufDesc  *bufferDescriptor
	released atomic.Bool
}

func (bd *bufferDescriptor) lockContent() *bufferContentLease {
	bd.contentLock.Lock()
	return &bufferContentLease{bufDesc: bd}
}

func (lease *bufferContentLease) release() {
	if lease == nil || !lease.released.CompareAndSwap(false, true) {
		panic("block cache content lease released more than once")
	}
	lease.bufDesc.contentLock.Unlock()
}

func (lease *bufferContentLease) belongsTo(bd *bufferDescriptor) bool {
	return lease != nil && lease.bufDesc == bd && !lease.released.Load()
}

// markWriteCoverage records OS pages fully covered by [start,end) and reports
// whether every page in the buffer has been fully written. FUSE writeback sends
// page-granular requests even when their total sizes vary. Partial page writes
// are intentionally not combined; they are uploaded during flush or pressure
// eviction. The caller must hold contentLock exclusively.
func (bd *bufferDescriptor) markWriteCoverage(start, end int) bool {
	if start < 0 || end <= start || end > len(bd.buf) {
		return false
	}

	if bd.writeRegionCount == 0 {
		bd.writeRegionCount = (len(bd.buf) + writeCoveragePageMask) >> writeCoverageShift
		wordCount := (bd.writeRegionCount + 63) >> 6
		bd.writeCoverage = make([]uint64, wordCount)
		bd.coveredRegions = 0
	}

	firstRegion := start >> writeCoverageShift
	lastRegion := (end - 1) >> writeCoverageShift
	for region := firstRegion; region <= lastRegion; region++ {
		regionStart := region << writeCoverageShift
		regionEnd := min(regionStart+writeCoverageGranularity, len(bd.buf))
		if start > regionStart || end < regionEnd {
			continue
		}

		word := region >> 6
		mask := uint64(1) << (region & 63)
		if bd.writeCoverage[word]&mask == 0 {
			bd.writeCoverage[word] |= mask
			bd.coveredRegions++
		}
	}

	return bd.coveredRegions == bd.writeRegionCount
}

// resetWriteCoverage starts coverage tracking for a new dirty block version.
// The caller must hold contentLock exclusively.
func (bd *bufferDescriptor) resetWriteCoverage() {
	clear(bd.writeCoverage)
	bd.coveredRegions = 0
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
	bd.contentLock.RUnlock() //nolint:staticcheck

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
	err := fmt.Errorf("bufferDescriptor::ensureBufferValidForRead: Inconsistent state for bufferIdx: %d, valid: %v, downloadErr: %v",
		bd.bufIdx, bd.valid.Load(), bd.downloadErr)
	log.Err("%v", err)

	return err
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
func (bd *bufferDescriptor) release(fl *freeListType) bool {
	newRefCnt := bd.refCnt.Add(-1)

	if newRefCnt == 0 {
		// No more references exist (table removed it and all users released).
		// Safe to return buffer to free list for reuse.
		log.Debug("bufferDescriptor::release: Releasing bufferIdx: %d for blockIdx: %d back to free list, bytesRead: %d, bytesWritten: %d, file: %s",
			bd.bufIdx, bd.block.idx, bd.bytesRead.Load(), bd.bytesWritten.Load(), bd.block.file.Name)
		fl.releaseBuffer(bd)
		return true
	} else if newRefCnt < 0 {
		// Negative refCnt indicates a bug: release() called more times than acquire.
		// This should never happen and represents a serious reference counting error.
		err := fmt.Sprintf("bufferDescriptor::release: bufferIdx: %d has negative refCount: %d, bytesRead: %d, bytesWritten: %d",
			bd.bufIdx, bd.refCnt.Load(), bd.bytesRead.Load(), bd.bytesWritten.Load())
		log.Err("%s", err)
		panic(err)
	}

	// Buffer still has active references, not ready for release yet.
	return false
}

// resetMetadata clears descriptor state before the buffer is returned to the free list.
func (bd *bufferDescriptor) resetMetadata() {
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
	bd.resetWriteCoverage()
}

// reset prepares a victim descriptor for immediate reassignment.
func (bd *bufferDescriptor) reset() {
	bd.resetMetadata()
	clear(bd.buf)
}
