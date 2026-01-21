package block_cache

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// ErrInvalidBlockList indicates that the block list retrieved from storage is not
// compatible with BlockCache's requirements (e.g., blocks are not aligned to the
// configured block size).
var ErrInvalidBlockList = errors.New("Invalid Block List, not compatible with Block Cache for write operations")

// blockState represents the current state of a block in its lifecycle.
//
// State transitions:
//
//	localBlock -> uncommitedBlock -> committedBlock
//	committedBlock -> localBlock (when modified)
//
// These states track whether a block's data is only in memory, uploaded but
// not committed, or fully committed to storage.
type blockState int32

const (
	// localBlock indicates the block exists only in local memory and has not been
	// uploaded to Azure Storage. The data is out of sync with storage.
	localBlock blockState = iota

	// uncommitedBlock indicates the block has been uploaded to Azure Storage via
	// StageData but has not yet been committed via CommitData (PutBlockList).
	// The block is visible to the uploader but not to other clients.
	uncommitedBlock

	// committedBlock indicates the block has been uploaded and committed to Azure Storage.
	// The data is synchronized with storage and visible to all clients.
	committedBlock
)

// block represents a fixed-size chunk of a file.
//
// Files are divided into sequential blocks for caching. Each block tracks its
// position in the file (idx), its state (local/uncommitted/committed), and
// metadata for upload operations.
//
// Thread Safety:
//   - The block's state is protected by atomic operations
//   - The mutex protects block metadata during state transitions
//   - Reference counting via buffer descriptors prevents premature eviction
type block struct {
	mu    sync.RWMutex // Protects block metadata during state transitions
	file  *File        // Pointer to the parent file (back reference)
	idx   int          // Block index in the file (0-based)
	id    string       // Azure Storage block ID (base64-encoded, generated during upload)
	state blockState   // Current state: localBlock, uncommitedBlock, or committedBlock

	// numWrites tracks the number of write operations performed on this block.
	// Used to detect if a committed block has been modified and needs re-upload.
	// Reset to 0 after successful upload.
	numWrites atomic.Int32
}

// createBlock creates a new block with the specified parameters.
//
// Parameters:
//   - idx: Block index in the file (0 for first block)
//   - id: Azure Storage block ID (empty string for new local blocks)
//   - state: Initial block state
//   - f: Parent file object
//
// Returns a new block instance ready for use.
func createBlock(idx int, id string, state blockState, f *File) *block {
	blk := &block{
		idx:   idx,
		id:    id,
		state: state,
		file:  f,
	}

	return blk
}

// blocklistState represents the state of a file's block list.
//
// The block list state determines whether BlockCache can perform write operations
// on a file. Invalid block lists (e.g., blocks not aligned to configured block size)
// prevent write operations to maintain data integrity.
type blocklistState int

const (
	// blockListInvalid means the block list is not compatible with BlockCache.
	// This happens when:
	//   - Block sizes don't match the configured block size
	//   - Block IDs have incorrect length
	//   - Blocks are not properly aligned
	// Files with invalid block lists can only be read, not written.
	blockListInvalid blocklistState = iota

	// blockListValid means the block list has been validated and is compatible
	// with BlockCache. Write operations are allowed.
	blockListValid

	// blockListNotRetrieved means the block list has not yet been fetched from
	// storage. This is the initial state for newly opened files.
	blockListNotRetrieved
)

// blockList maintains the list of blocks for a file.
//
// The block list tracks all blocks in a file in sequential order and
// maintains validation state to ensure compatibility with BlockCache operations.
type blockList struct {
	list  []*block       // Ordered list of blocks (index 0 is first block)
	state blocklistState // Validation state of the block list
}

// newBlockList creates a new empty block list with state blockListNotRetrieved.
func newBlockList() *blockList {
	return &blockList{
		list:  make([]*block, 0),
		state: blockListNotRetrieved,
	}
}

// validateBlockList validates that a committed block list is compatible with BlockCache.
//
// This function checks that:
//  1. All blocks (except possibly the last) have size equal to bc.blockSize
//  2. The last block has size <= bc.blockSize
//  3. All block IDs have the correct length (common.BlockIDLenghtBase64)
//
// If validation succeeds, the file's block list is populated with block objects
// and the state is set to blockListValid.
//
// Parameters:
//   - blkList: Committed block list retrieved from Azure Storage
//   - f: File object to populate with validated blocks
//
// Returns ErrInvalidBlockList if validation fails, nil on success.
//
// Why validation is needed:
// BlockCache assumes all blocks (except the last) are exactly blockSize bytes.
// Files created by other tools or older versions may have differently sized blocks,
// which would break BlockCache's offset calculations and read/write operations.
func validateBlockList(blkList *internal.CommittedBlockList, f *File) error {
	if blkList == nil || len(*blkList) == 0 {
		return ErrInvalidBlockList
	}
	listLen := len(*blkList)
	var newblkList []*block = make([]*block, 0, listLen)

	for idx, blk := range *blkList {
		if idx < (listLen-1) && blk.Size != bc.blockSize {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format blk idx : %d is having size %d bytes, while block size set is %d bytes", idx, blk.Size, bc.blockSize)
			return ErrInvalidBlockList
		} else if idx == (listLen-1) && blk.Size > bc.blockSize {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format, Last block(i.e., blk idx : %d) is having greater size(i.e., %d bytes) than block size configured is %d bytes", idx, blk.Size, bc.blockSize)
			return ErrInvalidBlockList
		} else if len(blk.Id) != common.BlockIDLenghtBase64 {
			log.Err("BlockCache::validateBlockList : Unsupported blocklist Format, block Id length for blk idx : %d is %d bytes is not matching to what blobfuse uses(i.e., %d bytes)", idx, len(blk.Id), common.BlockIDLenghtBase64)
			return ErrInvalidBlockList
		}
		newblkList = append(newblkList, createBlock(idx, blk.Id, committedBlock, f))
	}

	f.blockList.list = newblkList

	return nil
}

// updateBlockListForReadOnlyFile creates a synthetic block list for read-only files.
//
// When a file is opened read-only, we don't need to validate the actual block list
// from storage. Instead, we create a synthetic block list based on the file size,
// with each block marked as committed.
//
// This optimization:
//   - Avoids the cost of fetching and validating the block list from storage
//   - Allows reading files that have non-aligned blocks (since we're not writing)
//   - Simplifies read-only access patterns
//
// Parameters:
//   - f: File object to populate with synthetic block list
//
// The synthetic block list is created only if it doesn't already exist.
func updateBlockListForReadOnlyFile(f *File) {
	if len(f.blockList.list) != 0 {
		// no need to update blocklist again, if already present
		return
	}

	noOfBlocks := (f.size + int64(bc.blockSize) - 1) / int64(bc.blockSize)
	var newblkList []*block = make([]*block, 0, noOfBlocks)

	for i := range int(noOfBlocks) {
		newblkList = append(newblkList, createBlock(i, "", committedBlock, f))
	}

	f.blockList.list = newblkList
}

// Helper functions for block calculations

// getBlockIndex calculates which block contains the given offset.
//
// Parameters:
//   - offset: Byte offset in the file
//
// Returns the 0-based block index.
//
// Example: With 16MB blocks, offset 17MB returns 1 (second block).
func getBlockIndex(offset int64) int {
	return int(offset / int64(bc.blockSize))
}

// convertOffsetIntoBlockOffset converts a file offset to an offset within a block.
//
// Parameters:
//   - offset: Byte offset in the file
//
// Returns the offset within the block (0 to blockSize-1).
//
// Example: With 16MB blocks, offset 17MB returns 1MB (offset within second block).
func convertOffsetIntoBlockOffset(offset int64) int64 {
	return offset - int64(getBlockIndex(offset))*int64(bc.blockSize)
}

// getBlockSize calculates the actual size of a block in a file.
//
// Most blocks are exactly blockSize bytes, but the last block may be smaller.
//
// Parameters:
//   - size: Total file size in bytes
//   - idx: Block index (0-based)
//
// Returns the size of the block in bytes.
//
// Example: With 16MB blocks and 17MB file, block 0 is 16MB, block 1 is 1MB.
func getBlockSize(size int64, idx int) int {
	return min(int(bc.blockSize), int(size)-(idx*int(bc.blockSize)))
}

// getNoOfBlocksInFile calculates how many blocks are needed for a file of given size.
//
// Parameters:
//   - size: Total file size in bytes
//
// Returns the number of blocks needed (rounds up for partial blocks).
//
// Example: With 16MB blocks, a 17MB file needs 2 blocks.
func getNoOfBlocksInFile(size int64) int {
	return int((size + int64(bc.blockSize) - 1) / int64(bc.blockSize))
}

// scheduleUpload queues a block upload operation to the worker pool.
//
// This method schedules the block data in bufDesc to be uploaded to Azure Storage.
// Upload can be synchronous (blocks until complete) or asynchronous (returns immediately).
//
// Parameters:
//   - bufDesc: Buffer descriptor containing the block data to upload
//   - sync: If true, waits for upload to complete; if false, returns immediately
//
// Behavior:
//   - Increments buffer refCnt to prevent eviction during upload
//   - Locks buffer content (exclusively) during upload to prevent concurrent access
//   - For sync uploads: blocks until upload completes, then releases buffer
//   - For async uploads: releases buffer after upload completes in worker goroutine
//
// After upload:
//   - Block state changes to uncommitedBlock
//   - Block ID is generated and assigned
//   - Buffer is marked as not dirty
//   - Any upload errors are captured in bufDesc.uploadErr
func (blk *block) scheduleUpload(bufDesc *bufferDescriptor, sync bool) {
	// This buffer descriptor has reached its maximum usage count, schedule upload.
	log.Debug("block::scheduleUpload: Scheduling upload for blockIdx: %d, bufferIdx: %d, sync: %v, usageCnt: %d, refCnt: %d",
		blk.idx, bufDesc.bufIdx, sync, bufDesc.bytesWritten.Load(), bufDesc.refCnt.Load())

	wait := make(chan struct{}, 1)
	//
	// Take the exclusive lock on buffer content to prevent further writes while upload is in progress.
	// This will be released after upload is complete.
	if sync {
		bufDesc.contentLock.Lock()
	}
	// Increment refCnt for upload
	bufDesc.refCnt.Add(1)

	// Schedule upload
	wp.queueWork(blk, bufDesc, false /*download*/, wait, sync /*sync*/)

	if sync {
		// Wait for upload to complete.
		<-wait
		if ok := bufDesc.release(); ok {
			log.Debug("BlockCache::scheduleUpload: Released bufferIdx: %d for blockIdx: %d back to free list after sync upload",
				bufDesc.bufIdx, blk.idx)
		}
	}
}

// scheduleDownload queues a block download operation to the worker pool.
//
// This method schedules downloading block data from Azure Storage into bufDesc.
// Download can be synchronous (blocks until complete) or asynchronous (returns immediately).
//
// Parameters:
//   - bufDesc: Buffer descriptor to receive the downloaded data
//   - sync: If true, waits for download to complete; if false, returns immediately
//
// Behavior:
//   - Increments buffer refCnt to prevent eviction during download
//   - For sync downloads: blocks until download completes, then releases buffer
//   - For async downloads: releases buffer after download completes in worker goroutine
//
// After download:
//   - Buffer is marked as valid (or invalid if download failed)
//   - Any download errors are captured in bufDesc.downloadErr
//   - Content lock is released allowing reads to proceed
func (blk *block) scheduleDownload(bufDesc *bufferDescriptor, sync bool) {
	wait := make(chan struct{}, 1)
	// Increment refCnt for download
	bufDesc.refCnt.Add(1)

	// Schedule download
	wp.queueWork(blk, bufDesc, true, wait, sync)

	if sync {
		// Wait for download to complete.
		<-wait
		if ok := bufDesc.release(); ok {
			log.Debug("BlockCache::scheduleDownload: Released bufferIdx: %d for blockIdx: %d back to free list after sync download",
				bufDesc.bufIdx, blk.idx)
		}
	}
}
