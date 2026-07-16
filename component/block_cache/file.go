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
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// file represents a cached file with associated metadata and open handles.
//
// Overview:
//
// The file struct is the central structure for managing file state in BlockCache.
// Multiple file handles can reference the same file object, allowing concurrent
// access while maintaining consistent state.
//
// Key Responsibilities:
//
//   - Track all open handles for a file
//   - Maintain file size (both in memory and on storage)
//   - Manage the list of blocks that make up the file
//   - Coordinate read and write operations
//   - Handle flush operations to sync data to storage
//
// Concurrency:
//
//   - file-level RWMutex protects metadata and block list
//   - Atomic operations protect size and error fields
//   - Pending operation tracking prevents race conditions during flush
//
// Lifecycle:
//
//  1. Created when first handle is opened (via getFileFromPath)
//  2. Shared across multiple handles to the same path
//  3. Removed from file map when last handle is closed
//  4. All buffers released when file is removed
//
// Note: We store references to open handles (rather than just counting them)
// to support deferred file removal. When a file is deleted while handles are
// still open, we can iterate through handles to mark them appropriately.
//
// TODO: Add global context to the file that can propagate to all the operations
// on the file, this would simplify cancellations and timeouts easier.
type generationTracker struct {
	current atomic.Uint64
	mu      sync.Mutex
	cond    *sync.Cond
	active  map[uint64]int
}

func newGenerationTracker() *generationTracker {
	tracker := &generationTracker{active: make(map[uint64]int)}
	tracker.current.Store(1)
	tracker.cond = sync.NewCond(&tracker.mu)
	return tracker
}

func (tracker *generationTracker) currentID() uint64 {
	return tracker.current.Load()
}

// begin admits work only into the current generation. Holding mu across the
// generation check and count increment prevents advance from missing a task.
func (tracker *generationTracker) begin(generation uint64) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if generation != tracker.current.Load() {
		return false
	}
	tracker.active[generation]++
	return true
}

func (tracker *generationTracker) finish(generation uint64) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.active[generation] <= 1 {
		delete(tracker.active, generation)
	} else {
		tracker.active[generation]--
	}
	tracker.cond.Broadcast()
}

func (tracker *generationTracker) advance() (oldGeneration, newGeneration uint64) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	oldGeneration = tracker.current.Load()
	newGeneration = oldGeneration + 1
	tracker.current.Store(newGeneration)
	return oldGeneration, newGeneration
}

func (tracker *generationTracker) wait(generation uint64) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for tracker.active[generation] > 0 {
		tracker.cond.Wait()
	}
}

type file struct {
	mu            sync.RWMutex                   // Protects file metadata and block list
	Name          string                         // File path (absolute)
	sizeOnStorage atomic.Int64                   // File size as last known in Azure Storage
	size          atomic.Int64                   // Current file size (may differ from storage if modified)
	Etag          string                         // ETag from Azure Storage (for consistency checks)
	handles       map[*handlemap.Handle]struct{} // Set of open handles for this file
	blockList     *blockList                     // Ordered list of blocks composing this file
	synced        bool                           // True if file is synchronized with Azure Storage

	// Concurrency tracking for read operations
	numPendingReads atomic.Int32

	// Error handling: stores any error encountered during file operations.
	// Once set, subsequent operations fail fast with this error.
	// This provides "sticky error" semantics to prevent cascading failures.
	err atomic.Pointer[error]

	// Synchronization for write operations during flush.
	// Writers increment this before modifying the file, allowing flush to wait
	// for all pending writes to complete before uploading data.
	pendingWriters sync.WaitGroup

	// generations identifies the current logical contents and drains work from
	// invalidated contents before their buffers are released.
	generations *generationTracker

	// TODO: Optimization flag: if true, the file was uploaded using PutBlob (for small files)
	// rather than PutBlock + PutBlockList. This tracks the upload method for
	// consistency during subsequent flushes.
	singleBlockFilePersisted bool

	// lmtNano stores the last modification time as Unix nanoseconds.
	// Updated atomically on write and truncate, read by GetAttr to return
	// correct mtime while the file is open and modified.
	lmtNano atomic.Int64
}

// createFile creates a new File instance with default values.
//
// Parameters:
//   - fileName: Full path to the file
//
// Returns a new File object with:
//   - Empty handle map
//   - Empty block list (state: blockListNotRetrieved)
//   - Size set to -1 (indicates uninitialized)
//   - Synced set to true (no pending changes)
func createFile(fileName string) *file {
	f := &file{
		Name:        fileName,
		handles:     make(map[*handlemap.Handle]struct{}),
		blockList:   newBlockList(),
		synced:      true,
		generations: newGenerationTracker(),
	}
	f.size.Store(-1)
	f.sizeOnStorage.Store(-1)

	return f
}

// updateFileSize atomically updates the file size if the new size is larger.
//
// This method ensures file size only increases, preventing corruption from
// out-of-order updates. Uses compare-and-swap for thread-safe updates.
//
// Parameters:
//   - size: New file size to set (if larger than current size)
//
// This is called after write operations to extend the file size.
// Multiple concurrent writes may call this, so CAS ensures correct ordering.
func (f *file) updateFileSize(size int64) {
	for {
		currentSize := f.size.Load()

		if size <= currentSize {
			return
		}
		if f.size.CompareAndSwap(currentSize, size) {
			return
		}
	}
}

// read reads data from the file into the provided buffer.
//
// This method implements the core read logic for BlockCache, handling:
//
//  1. Block-level I/O: Maps file offset to blocks and reads from each block
//  2. Cache management: Gets or creates buffer descriptors for blocks
//  3. Download coordination: Triggers downloads for uncached blocks
//  4. Flush handling: Flushes uncommitted blocks before reading
//  5. Buffer cleanup: Removes fully-read buffers to free cache space
//
// Parameters:
//   - options: Read options including offset, data buffer, and handle
//
// Returns:
//   - int: Number of bytes read
//   - error: Any error encountered (EOF if reading past end of file)
//
// Concurrency:
//   - Tracks pending reads via numPendingReads for monitoring
//   - Multiple reads can proceed concurrently (shared block locks)
//   - Reads may block waiting for downloads to complete
//
// Performance optimization:
//   - Removes buffers from cache after they are fully read
//   - This frees space for more useful blocks (prefetch, write buffers)
//
// Thread Safety:
// Safe to call concurrently from multiple goroutines, even for the same file.
// Block-level locking ensures consistent reads during concurrent operations.
func (f *file) read(bc *BlockCache, options *internal.ReadInBufferOptions) (int, error) {
	f.numPendingReads.Add(1)
	defer f.numPendingReads.Add(-1)

	if options.Offset < 0 {
		return 0, syscall.EINVAL
	}
	if len(options.Data) == 0 {
		return 0, nil
	}

	stime := time.Now()

	fileSize := f.size.Load()
	if options.Offset >= fileSize {
		return 0, io.EOF
	}

	offset := options.Offset
	endOffset := min(fileSize, options.Offset+int64(len(options.Data)))
	bufOffset := 0
	bytesRead := 0

	for offset < endOffset {
		blockIdx := getBlockIndex(offset, int64(bc.blockSize))
		if err := validateBlockIndex(blockIdx); err != nil {
			return 0, err
		}

		var (
			blk     *block
			bufDesc *bufferDescriptor
			status  bufDescStatus
		)

		// Acquire the block and a valid buffer descriptor for it. If the block
		// is in uncommitted state, flush the file once and retry the lookup.
		// Only a single flush is attempted; if the status is still
		// bufDescStatusNeedsFileFlush after that, we treat it as an error.
		flushed := false
		for {
			f.mu.RLock()
			if blockIdx < len(f.blockList.list) {
				blk = f.blockList.list[blockIdx]
			}
			f.mu.RUnlock()

			if blk == nil {
				log.Err("File::read: Block not found for file %s blockIdx %d", f.Name, blockIdx)
				// TODO: is this the right error to return? or EIO is better?
				return 0, io.EOF
			}

			var err error
			bufDesc, status, err = bc.btm.getOrCreateBufferDescriptor(
				bc.freeList,
				bc.workerPool,
				blk,
				true, /*sync*/
			)
			if err != nil {
				log.Err("File::read: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
				return 0, err
			}

			if status != bufDescStatusNeedsFileFlush {
				break
			}

			if flushed {
				// We already flushed once, but the block is still in uncommitted state. This should not happen.
				log.Err("File::read: Block still in uncommitted state after flush for file: %s, blockIdx: %d", f.Name, blockIdx)
				return 0, fmt.Errorf("block %d for file %s still in uncommitted state after flush", blockIdx, f.Name)
			}

			// The block is in uncommitted state, need to flush the file first before reading.
			log.Debug("File::read: Block in uncommitted state, flushing file: %s before read, blockIdx: %d", f.Name, blockIdx)

			if err := f.flush(bc, true /*takeFileLock*/); err != nil {
				log.Err("File::read: Failed to flush file: %s before read, blockIdx: %d: %v", f.Name, blockIdx, err)
				return 0, err
			}
			flushed = true
		}

		log.Debug("File::read: Got buffer descriptor bufIdx: %d for file: %s, blockIdx: %d, status: %v, numParallelReaders: %d, took: %v",
			bufDesc.bufIdx, f.Name, blockIdx, status, f.numPendingReads.Load(), time.Since(stime))

		// Copy data from block buffer to user buffer
		bufDesc.contentLock.RLock()
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset, int64(bc.blockSize))
		blockLen := getBlockSize(fileSize, blockIdx, int64(bc.blockSize))
		n := copy(options.Data[bufOffset:], bufDesc.buf[offsetInsideBlock:blockLen])
		bufDesc.contentLock.RUnlock()

		log.Debug("File::read: Read %d bytes from file: %s, blockIdx: %d, refCnt: %d, bytesRead: %d, numParallelReaders: %d, took: %v",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), f.numPendingReads.Load(), time.Since(stime))

		bufDesc.bytesRead.Add(int32(n))

		if bufDesc.release(bc.freeList) {
			log.Debug("File::read: Released bufferIdx: %d for blockIdx: %d back to free list after read at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		bytesRead += n
		bufOffset += n
		offset += int64(n)
	}

	log.Debug("File::read: Completed read of %d bytes from file: %s, offset: %d, took: %v",
		bytesRead, f.Name, options.Offset, time.Since(stime))

	return bytesRead, nil
}

// scheduleReadAhead triggers prefetching of blocks for sequential access patterns.
//
// This method analyzes the access pattern using the pattern detector and schedules
// asynchronous downloads for future blocks if sequential access is detected.
//
// Parameters:
//   - pd: Pattern detector tracking this handle's access pattern
//   - offset: Current read offset
//   - length: Number of bytes returned by the demand read
//
// Behavior:
//   - Only schedules read-ahead for sequential patterns
//   - Prefetches up to bc.prefetch blocks ahead
//   - Tracks next read-ahead block index to avoid duplicate prefetches
//   - Skips blocks that are already in cache
//   - Stops when reaching end of file
//
// Why per-handle detection:
// Different handles may read the same file with different patterns
// (e.g., one sequential, one random). Per-handle detection optimizes
// for each access pattern independently.
//
// Async operation:
// Read-ahead downloads run asynchronously. The calling read operation doesn't
// wait for prefetches to complete. Future reads benefit from prefetched data.
func (f *file) scheduleReadAhead(bc *BlockCache, pd *patternDetector, offset int64, length int) {
	if length <= 0 || bc.prefetch == 0 {
		return
	}

	patterntype := pd.updateAccessPattern(offset, int64(bc.blockSize))
	if patterntype != patternSequential {
		return
	}

	numReadAheadBlocks := int(bc.prefetch)
	lastDemandBlockIdx := getBlockIndex(offset+int64(length)-1, int64(bc.blockSize))
	firstReadAheadBlockIdx := int64(lastDemandBlockIdx + 1)
	for {
		next := pd.nxtReadAheadBlockIdx.Load()
		if next >= firstReadAheadBlockIdx || pd.nxtReadAheadBlockIdx.CompareAndSwap(next, firstReadAheadBlockIdx) {
			break
		}
	}

	for range numReadAheadBlocks {
		nextBlockIdx := pd.nxtReadAheadBlockIdx.Load()
		if int(nextBlockIdx) > lastDemandBlockIdx+numReadAheadBlocks {
			return
		}
		if !pd.nxtReadAheadBlockIdx.CompareAndSwap(nextBlockIdx, nextBlockIdx+1) {
			continue
		}

		var blk *block

		f.mu.RLock()
		if int(nextBlockIdx) < len(f.blockList.list) {
			blk = f.blockList.list[nextBlockIdx]
		}
		f.mu.RUnlock()

		if blk == nil {
			// No more blocks to read-ahead
			return
		}

		bufDesc, status, err := bc.btm.getOrCreateBufferDescriptor(
			bc.freeList,
			bc.workerPool,
			blk,
			false, /* sync */
		)
		if err != nil {
			pd.nxtReadAheadBlockIdx.CompareAndSwap(nextBlockIdx+1, nextBlockIdx)
			log.Err("File::scheduleReadAhead: Failed to get buffer descriptor for file: %s, blockIdx: %d during read-ahead, [%v]",
				f.Name, blk.idx, err)
			return
		}

		if bufDesc != nil {
			// Release the buffer descriptor as we dont need it
			if ok := bufDesc.release(bc.freeList); ok {
				log.Debug("File::scheduleReadAhead: Released bufferIdx: %d for blockIdx: %d back to free list after read-ahead at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
		}

		if status == bufDescStatusExists {
			log.Debug("File::scheduleReadAhead: Block already in cache, wrong read-ahead scheduled for file: %s, blockIdx: %d, pattern: %v, status: %v",
				f.Name, blk.idx, patterntype, status)

		} else {
			// We have scheduled read-ahead for this block
			log.Debug("File::scheduleReadAhead: Scheduled read-ahead for file: %s, blockIdx: %d, pattern: %v, status: %v",
				f.Name, blk.idx, patterntype, status)
		}
	}
}

// write writes data to the file at the specified offset.
//
// This method implements the core write logic for BlockCache, handling:
//
//  1. Block allocation: Creates new blocks as needed to cover write range
//  2. Buffer management: Gets or creates buffers for modified blocks
//  3. Data copying: Copies user data into cached blocks
//  4. Dirty tracking: Marks modified blocks as dirty
//  5. Size updates: Extends file size if the write reaches beyond EOF
//  6. Error handling: Implements sticky error semantics
//
// Parameters:
//   - options: Write options including offset, data buffer, and handle
//
// Returns an error if:
//   - Write would exceed maximum file size (blockSize * MAX_BLOCKS)
//   - Previous write operation failed (sticky error)
//   - Block allocation or buffer acquisition fails
//
// Write Behavior:
//
//   - Partial block writes are supported (read-modify-write)
//   - Multiple writes to same block accumulate in memory
//   - Blocks are uploaded when full or during flush
//   - Writes are serialized per file via file mutex
//   - Write wait group tracks pending writes for flush coordination
//
// Thread Safety:
// While multiple goroutines can call write concurrently, the file mutex
// serializes writes to maintain consistency. Each write operation completes
// atomically from the file's perspective.
//
// Important: This method MUST write all len(options.Data) bytes successfully
// or return an error. Partial writes are not allowed.
func (f *file) write(bc *BlockCache, options *internal.WriteFileOptions) error {
	if options.Offset < 0 {
		return syscall.EINVAL
	}
	if len(options.Data) == 0 {
		return nil
	}

	if uint64(options.Offset) > bc.maxFileSize || uint64(len(options.Data)) > bc.maxFileSize-uint64(options.Offset) {
		log.Err("File::write: Write exceeds maximum file size for file %s, offset %d, data length %d",
			f.Name, options.Offset, len(options.Data))
		return fmt.Errorf("write exceeds maximum file size")
	}

	offset := options.Offset
	endOffset := options.Offset + int64(len(options.Data))
	bufOffset := 0

	// If there was any previous write error, return that error, this will safely prevent further writes to the file.
	if e := f.err.Load(); e != nil {
		return fmt.Errorf("previous write error: %w", *e)
	}

	for offset < endOffset {
		blockIdx := getBlockIndex(offset, int64(bc.blockSize))
		var (
			blk     *block
			bufDesc *bufferDescriptor
			status  bufDescStatus
		)

		// Acquire (or create) the block and a valid buffer descriptor for it.
		// If the block is in uncommitted state, flush the file once and retry
		// the lookup. Only a single flush is attempted; if the status is still
		// bufDescStatusNeedsFileFlush after that, we treat it as an error.
		// pendingWriters is incremented per attempt under f.mu so that flush
		// (which takes f.mu and then waits on pendingWriters) sees a consistent
		// view; it is decremented before recursing into flush.
		flushed := false
		for {
			f.mu.Lock()
			// Increment write wait group to track pending writes, This must be done under lock as flushing the file would
			// block the upcoming writers when it acquires the lock. The call to f.pendingWriters.Done() must be called
			// after the write is completed even if there is an error, otherwise flush will wait indefinitely.
			f.pendingWriters.Add(1)

			blockListLen := len(f.blockList.list)

			if blockIdx < blockListLen {
				blk = f.blockList.list[blockIdx]
			} else {
				// Need to create new block
				for i := blockListLen; i <= blockIdx; i++ {
					blk = createBlock(i, common.GetBlockID(common.BlockIDLength), localBlock, f)
					f.blockList.list = append(f.blockList.list, blk)
					log.Debug("File::write: Created new blockIdx: %d for file: %s during write at offset: %d",
						blk.idx, f.Name, options.Offset)
				}
			}
			f.synced = false
			f.mu.Unlock()

			var err error
			bufDesc, status, err = bc.btm.getOrCreateBufferDescriptor(
				bc.freeList,
				bc.workerPool,
				blk,
				true, /*sync*/
			)
			if err != nil {
				// Decrement the write wait group on error
				f.pendingWriters.Done()
				log.Err("File::write: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
				return err
			}

			if status != bufDescStatusNeedsFileFlush {
				break
			}

			// Decrement the write wait group before flushing, as flush will wait for all pending writers to complete.
			f.pendingWriters.Done()

			if flushed {
				// We already flushed once, but the block is still in uncommitted state. This should not happen.
				log.Err("File::write: Block still in uncommitted state after flush for file: %s, blockIdx: %d", f.Name, blockIdx)
				return fmt.Errorf("block %d for file %s still in uncommitted state after flush", blockIdx, f.Name)
			}

			// The block is in uncommitted state, need to flush the file first before writing.
			log.Debug("File::write: Block in uncommitted state, flushing file: %s before write, blockIdx: %d", f.Name, blockIdx)

			if err := f.flush(bc, true /*takeFileLock*/); err != nil {
				log.Err("File::write: Failed to flush file: %s before write, blockIdx: %d: %v", f.Name, blockIdx, err)
				return err
			}
			flushed = true
		}

		log.Debug("File::write: Got buffer descriptor bufIdx: %d for file: %s, blockIdx: %d, status: %v",
			bufDesc.bufIdx, f.Name, blockIdx, status)

		offsetInsideBlock := convertOffsetIntoBlockOffset(offset, int64(bc.blockSize))

		// Take the exclusive lock on buffer content to write data
		// Change the block state to localBlock as it is being modified
		contentLease := bufDesc.lockContent()
		if !bufDesc.dirty.Load() {
			bufDesc.resetWriteCoverage()
			bufDesc.uploadErr = nil
		}
		blk.setState(localBlock)
		blk.numWrites.Add(1)
		bufDesc.dirty.Store(true)
		n := copy(bufDesc.buf[offsetInsideBlock:bc.blockSize], options.Data[bufOffset:])
		bufDesc.bytesWritten.Add(int32(n))
		fullyCovered := bufDesc.markWriteCoverage(int(offsetInsideBlock), int(offsetInsideBlock)+n)

		offset += int64(n)
		bufOffset += n

		// Update file size if needed
		f.updateFileSize(offset /* newFileSize */)

		uploadQueued := false
		if fullyCovered {
			if _, err := blk.queueUploadLocked(bc.workerPool, bufDesc, contentLease, false /* callerWaits */); err != nil {
				contentLease.release()
				bufDesc.release(bc.freeList)
				f.pendingWriters.Done()
				return err
			}
			uploadQueued = true
		}
		if !uploadQueued {
			contentLease.release()
		}

		log.Debug("File::write: Wrote %d bytes to file: %s, size: %d, blockIdx: %d, refCnt: %d, usageCnt: %d, pageOffset: %d, pageRemainder: %d, writeback: %t",
			n, f.Name, f.size.Load(), blockIdx, bufDesc.refCnt.Load(), bufDesc.bytesWritten.Load(),
			int(offsetInsideBlock)&writeCoveragePageMask, n&writeCoveragePageMask, uploadQueued)

		// Release the buffer descriptor
		if ok := bufDesc.release(bc.freeList); ok {
			log.Debug("File::write: Released bufferIdx: %d for blockIdx: %d back to free list after write at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		// Decrement the write wait group after write is completed
		f.pendingWriters.Done()
	}

	// Record last modification time after all bytes are written.
	f.lmtNano.Store(time.Now().UnixNano())

	return nil
}

// flush synchronizes all file data with Azure Storage.
//
// This is the most complex operation in BlockCache, handling:
//
//  1. Wait for pending writes: Ensures no writes are in progress
//  2. Block state analysis: Identifies which blocks need uploading
//  3. Sparse block handling: Uploads zero blocks for unwritten regions
//  4. Dirty block upload: Uploads all modified blocks
//  5. Block list commit: Calls PutBlockList to finalize the file
//
// Parameters:
//   - takeFileLock: If true, acquires exclusive file lock; if false, assumes lock is held
//
// Returns an error if any upload or commit operation fails.
//
// Block Upload Logic:
//
//   - committedBlock: Already in storage, no upload needed
//   - uncommitedBlock: Already uploaded via StageData, no re-upload needed
//   - localBlock (no buffer): Sparse block, upload zero-filled data
//   - localBlock (with buffer, dirty): Modified block, upload actual data
//   - localBlock (with buffer, not dirty): Bug (should not happen)
//
// Sparse Block Optimization:
//
// When a file is extended (e.g., via truncate), new blocks may exist in the
// block list but have never been written. These are "sparse" blocks. Rather
// than allocating buffers for them, we upload a single zero block and reuse
// its block ID for all sparse blocks (except the last block).
//
// File Extension Handling:
//
// If a file is extended (write beyond previous EOF), the last block of the
// previous size may need to be extended with zeros. This is detected by
// comparing size with sizeOnStorage.
//
// Empty Files:
//
// Files with no blocks (zero length) are created using CreateFile rather
// than PutBlockList, as Azure Storage requires at least one block for
// PutBlockList.
//
// Error Handling:
//
// Any upload or commit error is stored in f.err (sticky error semantics).
// Subsequent operations will fail fast with this error.
//
// Thread Safety:
//
// This method must be called with the file lock held (or takeFileLock=true).
// It waits for all pending writers to complete before proceeding.
//
// Important: After flush succeeds, f.synced is set to true and subsequent
// flush calls become no-ops until the file is modified again.
func (f *file) flush(bc *BlockCache, takeFileLock bool) error {
	log.Debug("File::flush: Flushing file: %s, takeFileLock: %v", f.Name, takeFileLock)

	if takeFileLock {
		// Take an exclusive lock on file to prevent further writes during flush.
		f.mu.Lock()
		defer f.mu.Unlock()

		log.Debug("File::flush: Acquired exclusive lock for flush on file: %s", f.Name)
	}

	log.Debug("File::flush: Flushing file: %s, size: %d, takeFileLock: %v", f.Name, f.size.Load(), takeFileLock)

	if f.blockList.state != blockListValid {
		return nil
	}

	if e := f.err.Load(); e != nil {
		log.Err("File::flush: Previous write error found for file: %s, error: %v", f.Name, *e)
		return fmt.Errorf("previous write error: %w", *e)
	}

	if f.synced {
		log.Debug("File::flush: File: %s is already synced, no flush needed", f.Name)
		return nil
	}

	// Wait for all pending writes to complete inorder to have the clean state of the file.
	// We dont allow the new writers to proceed as we have the exclusive lock on file.
	f.pendingWriters.Wait()

	size := f.size.Load()
	sizeOnStorage := f.sizeOnStorage.Load()

	zeroBlockId := common.GetBlockID(common.BlockIDLength)
	isZeroBlockUploaded := false
	uploadZeroBlock := func(blk *block, isLastBlock bool) error {
		blk.id = zeroBlockId
		if isZeroBlockUploaded && !isLastBlock {
			// Zero block is already uploaded, reuse the block ID
			return nil
		}
		offsetInsideBlock := int64(bc.blockSize)

		if isLastBlock {
			offsetInsideBlock = convertOffsetIntoBlockOffset(f.size.Load()-1, int64(bc.blockSize))
			offsetInsideBlock++
			blk.id = common.GetBlockID(common.BlockIDLength)
		}
		log.Debug("File::flush: Uploading zero block for blockIdx: %d during flush at file: %s, zeroBlockId: %s, bytesUploading: %d",
			blk.idx, f.Name, blk.id, offsetInsideBlock)

		err := bc.NextComponent().StageData(internal.StageDataOptions{
			Name: f.Name,
			Data: bc.freeList.zeroBuf[:offsetInsideBlock],
			Id:   blk.id,
		})
		if err == nil && offsetInsideBlock == int64(bc.blockSize) {
			isZeroBlockUploaded = true
		}

		return err
	}

	blockListLen := len(f.blockList.list)

	// If the file is expanded by write, the last block may got sparse, may need to extend it.
	if blockListLen > 0 && size > sizeOnStorage && sizeOnStorage%int64(bc.blockSize) != 0 {
		// reupload the block that was partially filled earlier to extend it with zeros.
		lastBlockIdx := getBlockIndex(sizeOnStorage-1, int64(bc.blockSize))
		lastBlock := f.blockList.list[lastBlockIdx]
		if lastBlock.getState() == committedBlock && lastBlock.numWrites.Load() == 0 {
			// Last block is committed and no writes on it, need to extend it with zeros by making it dirty.
			log.Debug("File::flush: Extending last blockIdx: %d for file: %s during flush to accommodate file size expansion",
				lastBlock.idx, f.Name)
			bufDesc, _, err := bc.btm.getOrCreateBufferDescriptor(
				bc.freeList,
				bc.workerPool,
				lastBlock,
				true, /*sync*/
			)
			if err != nil {
				log.Err("File::flush: Failed to get buffer descriptor for last blockIdx: %d during flush at file: %s, [%v]",
					lastBlock.idx, f.Name, err)
				return err
			}

			lastBlock.setState(localBlock)
			bufDesc.dirty.Store(true)
			// Release the buffer descriptor, that is just acquired, this should not free the buffer as buffer is dirty.
			if ok := bufDesc.release(bc.freeList); ok {
				err = fmt.Errorf("File::flush: Released bufferIdx: %d for last blockIdx: %d back to free list after flush at file: %s",
					bufDesc.bufIdx, lastBlock.idx, f.Name)
				log.Crit("%s", err.Error())
				return err
			}
		}

	}

	type dirtyBuffer struct {
		block   *block
		bufDesc *bufferDescriptor
	}
	dirtyBuffers := make([]dirtyBuffer, 0)
	var scanErr error

	// Pin and collect all dirty buffers before queueing uploads. This lets the
	// worker pool stage blocks in parallel while keeping failure cleanup simple.
	for i, blk := range f.blockList.list {
		if blk.getState() == committedBlock || blk.getState() == uncommitedBlock {
			// No need to upload committed or uncommitted blocks
			continue
		}

		bufDesc, _ := bc.btm.lookupBufferDescriptor(blk, bc.freeList)
		if bufDesc == nil {
			// It might happen this buffer has chosen as victim and removed from table after uploading.
			if blk.getState() == committedBlock || blk.getState() == uncommitedBlock {
				// No need to upload committed or uncommitted blocks
				continue
			}

			// No buffer descriptor found for this block, sparse blocks must have no writes on it.
			if blk.getState() == localBlock && blk.numWrites.Load() > 0 {
				scanErr = fmt.Errorf("File::flush: No buffer descriptor found for local blockIdx: %d during flush at file: %s",
					blk.idx, f.Name)
				log.Crit("%s", scanErr.Error())
				break
			}

			// This is a sparse block which is not modified. Hence no buffer descriptor is present. Upload zero block if
			// needed.
			scanErr = uploadZeroBlock(blk, i == blockListLen-1 /*isLastBlock*/)
			if scanErr != nil {
				log.Err("File::flush: Failed to upload zero block for sparse blockIdx: %d during flush at file: %s: %v",
					blk.idx, f.Name, scanErr)
				f.err.Store(&scanErr)
				break
			}
			continue
		}

		// If there is any upload scheduled for this buffer, wait for it to complete, this content lock is taken
		// exclusively during upload.
		bufDesc.contentLock.Lock()
		bufDesc.contentLock.Unlock() //nolint:staticcheck

		if bufDesc.dirty.Load() && bufDesc.uploadErr == nil {
			log.Debug("File::flush: Queuing upload for bufferIdx: %d, blockIdx: %d during flush, bytesRead: %d, bytesWritten: %d at file: %s",
				bufDesc.bufIdx, blk.idx, bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), f.Name)
			dirtyBuffers = append(dirtyBuffers, dirtyBuffer{block: blk, bufDesc: bufDesc})
		} else {
			if bufDesc.uploadErr != nil {
				log.Err("File::flush: Previous upload error for bufferIdx: %d, blockIdx: %d during flush at file: %s: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadErr)
				scanErr = bufDesc.uploadErr
			} else if blk.getState() == localBlock {
				scanErr = fmt.Errorf("File::flush: Inconsistent state for bufferIdx: %d, blockIdx: %d during flush at file: %s, dirty: %v, uploadErr: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.dirty.Load(), bufDesc.uploadErr)
				log.Crit("%s", scanErr.Error())
			} else {
				log.Debug("File::flush: No upload needed for bufferIdx: %d, blockIdx: %d during flush at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
			bufDesc.release(bc.freeList)
			if scanErr != nil {
				break
			}
		}
	}

	if scanErr != nil {
		for _, dirty := range dirtyBuffers {
			dirty.bufDesc.release(bc.freeList)
		}
		return scanErr
	}

	type queuedUpload struct {
		dirty dirtyBuffer
		task  *task
	}
	uploads := make([]queuedUpload, 0, len(dirtyBuffers))
	var queueErr error
	for i, dirty := range dirtyBuffers {
		task, err := dirty.block.queueUpload(bc.workerPool, dirty.bufDesc)
		if err != nil {
			queueErr = err
			for _, unqueued := range dirtyBuffers[i:] {
				unqueued.bufDesc.release(bc.freeList)
			}
			break
		}
		uploads = append(uploads, queuedUpload{dirty: dirty, task: task})
	}

	var uploadErr error
	for _, upload := range uploads {
		<-upload.task.signalOnCompletion
		dirty := upload.dirty
		dirty.bufDesc.release(bc.freeList) // task reference
		if upload.task.err != nil && uploadErr == nil {
			uploadErr = upload.task.err
		}
		dirty.bufDesc.release(bc.freeList) // lookup reference
	}
	if queueErr != nil {
		return queueErr
	}
	if uploadErr != nil {
		return uploadErr
	}

	// Do PutBlockList to commit all the blocks.
	blockList := make([]string, 0, len(f.blockList.list))
	for _, blk := range f.blockList.list {
		blockList = append(blockList, blk.id)
	}
	log.Debug("File::flush: Committing block list for file: %s, number of blocks: %d, blockList: %v", f.Name, len(blockList), blockList)

	if len(blockList) == 0 {
		// Need to create an empty file in the storage
		log.Debug("File::flush: Creating empty file in storage for file: %s", f.Name)
		err := bc.createFileOnStorage(internal.CreateFileOptions{
			Name: f.Name,
		})
		if err != nil {
			log.Err("File::flush: Failed to create empty file in storage for file: %s: %v", f.Name, err)
			f.err.Store(&err)
			return err
		}
		log.Debug("File::flush: Successfully created empty file in storage for file: %s", f.Name)
		f.synced = true
		f.sizeOnStorage.Store(0)
		return nil
	}

	err := bc.NextComponent().CommitData(internal.CommitDataOptions{
		Name:      f.Name,
		List:      blockList,
		BlockSize: bc.blockSize,
	})
	if err != nil {
		log.Err("File::flush: Failed to commit block list for file: %s: %v", f.Name, err)
		f.err.Store(&err)
		return err
	} else {
		log.Debug("File::flush: Successfully committed block list for file: %s, size: %d", f.Name, size)
		f.synced = true
	}

	// update the block states.
	for _, blk := range f.blockList.list {
		blk.setState(committedBlock)
	}

	f.sizeOnStorage.Store(size)

	return nil
}

// truncate changes the file size to the specified value.
//
// This method handles both shrinking and extending files, with different
// operations required for each case:
//
// Shrinking (newSize < currentSize):
//  1. Flush file to ensure all data is in storage
//  2. Reduce number of blocks to fit new size
//  3. Release buffers for removed blocks
//  4. Clear partial data in last block
//  5. Mark last block as dirty (needs re-upload with correct size)
//  6. Flush again to commit the truncation
//
// Extending (newSize > currentSize):
//  1. Flush file to ensure current state is saved
//  2. Add new zero-filled blocks as needed
//  3. All new blocks share the same block ID (zero block optimization)
//  4. Flush again to commit the extension
//
// Parameters:
//   - options: Truncate options including new size and handle
//
// Returns an error if:
//   - Previous write operation failed (sticky error)
//   - Flush operations fail
//   - Buffer operations fail
//
// Block Management:
//
//   - Shrinking: Blocks beyond newSize are removed from block list
//   - Extending: New blocks are added with localBlock state
//   - Last block: Always marked as localBlock and dirty after truncate
//
// Zero-filling:
//
// When extending, new blocks are zero-filled implicitly (during flush,
// sparse blocks are uploaded as zeros). When shrinking, the remainder
// of the last block is explicitly zero-filled for security and consistency.
//
// Flush Behavior:
//
// Truncate performs TWO flushes:
//  1. Before: Ensures current data is saved (prevents data loss)
//  2. After: Commits the size change to storage
//
// Thread Safety:
//
// This method acquires exclusive file lock to prevent concurrent modifications.
// It's safe to call from multiple goroutines.
//
// Important: newSize must be within [0, maxFileSize]. Truncating beyond
// maxFileSize is not supported.
func (f *file) truncate(bc *BlockCache, options *internal.TruncateFileOptions) error {
	log.Debug("File::truncate: Truncating file: %s to size: %d", f.Name, options.NewSize)
	f.mu.Lock()
	defer f.mu.Unlock()

	log.Debug("File::truncate: Acquired exclusive lock for truncate on file: %s", f.Name)

	// check error state
	if e := f.err.Load(); e != nil {
		log.Err("File::truncate: Previous write error found for file: %s, error: %v", f.Name, *e)
		return fmt.Errorf("previous write error: %w", *e)
	}

	if options.NewSize == f.size.Load() {
		// No need to truncate
		log.Debug("File::truncate: No truncation needed for file: %s, size is already: %d", f.Name, options.NewSize)
		return nil
	}

	// Flush the file before truncating
	log.Debug("File::truncate: Flushing file: %s before truncation", f.Name)
	if err := f.flush(bc, false /*takeFileLock*/); err != nil {
		return err
	}
	oldGeneration, newGeneration := f.generations.advance()
	f.generations.wait(oldGeneration)
	for _, blk := range f.blockList.list {
		blk.fileGeneration.Store(newGeneration)
	}

	// Update the file size
	currentSize := f.size.Load()
	isFileShrinking := currentSize > options.NewSize
	f.size.Store(options.NewSize)
	f.synced = false

	noOfBlocks := getNoOfBlocksInFile(options.NewSize, int64(bc.blockSize))

	if noOfBlocks < len(f.blockList.list) {
		// Shrink the block list, give back the buffers shrank to free list.
		for i := noOfBlocks; i < len(f.blockList.list); i++ {
			blk := f.blockList.list[i]
			bufDesc, _ := bc.btm.lookupBufferDescriptor(blk, bc.freeList)
			if bufDesc != nil {
				if bc.btm.detachBufferDescriptor(bufDesc, bc.freeList) {
					log.Debug("File::truncate: Removed bufferIdx: %d for blockIdx: %d from buffer table manager during truncate at file: %s",
						bufDesc.bufIdx, blk.idx, f.Name)
				}
				bufDesc.release(bc.freeList)
			}
		}
		f.blockList.list = f.blockList.list[:noOfBlocks]
	}

	// change the state of the last block to localBlock
	if len(f.blockList.list) > 0 {

		// make the last block as local block.
		lastBlock := f.blockList.list[len(f.blockList.list)-1]

		bufDesc, status, err := bc.btm.getOrCreateBufferDescriptor(
			bc.freeList,
			bc.workerPool,
			lastBlock,
			true, /*sync*/
		)
		if err != nil {
			log.Err("File::truncate: Failed to get buffer descriptor for last blockIdx: %d during truncate at file: %s, [%v]",
				lastBlock.idx, f.Name, err)
			return err
		}

		lastBlock.setState(localBlock)
		bufDesc.dirty.Store(true)

		// Clean the rest of the buffer if file is getting shrank as it may contain old/dirty data.
		if isFileShrinking {
			bufDesc.contentLock.Lock()
			offsetInsideBlock := convertOffsetIntoBlockOffset(f.size.Load()-1, int64(bc.blockSize))
			copy(bufDesc.buf[offsetInsideBlock+1:], bc.freeList.zeroBuf)
			bufDesc.contentLock.Unlock()
		}

		log.Debug("File::truncate: Got buffer descriptor for last blockIdx: %d during truncate at file: %s, status: %v",
			lastBlock.idx, f.Name, status)

		// Release the buffer descriptor
		if ok := bufDesc.release(bc.freeList); ok {
			log.Debug("File::truncate: Released bufferIdx: %d for last blockIdx: %d back to free list after truncate at file: %s",
				bufDesc.bufIdx, lastBlock.idx, f.Name)
		}
	}

	log.Debug("File::truncate: New block list for file: %s to noOfBlocks: %d", f.Name, noOfBlocks)

	if noOfBlocks > len(f.blockList.list) {
		// Expand the block blockList, create one localBlock for new blocks and duplicate it.
		blkId := common.GetBlockID(common.BlockIDLength)

		for i := len(f.blockList.list); i < noOfBlocks; i++ {
			blk := createBlock(i, blkId, localBlock, f)
			f.blockList.list = append(f.blockList.list, blk)
			log.Debug("File::truncate: Expanded block list for file: %s, added blockIdx: %d", f.Name, i)
		}
	}

	// Flush the file again to commit the truncation
	log.Debug("File::truncate: Flushing file: %s after truncation", f.Name)
	if err := f.flush(bc, false /*takeFileLock*/); err != nil {
		return err
	}

	// Record last modification time after truncation.
	f.lmtNano.Store(time.Now().UnixNano())

	return nil
}
