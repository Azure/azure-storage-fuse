package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type bufferDescriptor struct {
	bufIdx        int
	block         *block
	nxtFreeBuffer int

	// Ref count indicates how many users are using this buffer.
	// When ref count is zero, it means no one is using this buffer and it can be evicted.
	// while incrementing refCnt, bufferTableMgr Lock must be held, either in shared or exclusive mode.
	refCnt     atomic.Int32
	usageCount atomic.Int32

	// TODO: use atomic int for these bools.
	valid atomic.Bool
	dirty atomic.Bool

	// This lock must be held while reading/writing the content of the buffer.
	// While downloading the buffer content, this lock must be held in exclusively, while reading the data from the buffer,
	// this lock must be held in shared mode.
	contentLock sync.RWMutex
	buf         []byte
	downloadErr error
}

// ensureBufferValid: ensures that the buffer is valid, i.e., no download error, if there is download error,
// do housekeeping, like releasing the buffer descriptor and buffer.
func (bd *bufferDescriptor) ensureBufferValid() error {
	// Wait for the Download to happen. if there was an error during download, it will be set in downloadErr.
	bd.contentLock.RLock()
	bd.contentLock.RUnlock()

	return bd.canIRead()
}

// canIRead: checks if the buffer is readable, i.e., no download error.
func (bd *bufferDescriptor) canIRead() error {

	if bd.valid.Load() && bd.downloadErr == nil {
		// Safe to read data from this buffer.
		return nil
	}

	blk := bd.block
	err := bd.downloadErr
	// Release the refCnt and usageCnt that you already acquired.
	bd.release()
	bd.usageCount.Add(-1)

	if bd.refCnt.Load() == 0 {
		log.Debug("BufferTableMgr::canIRead : BufferIdx: %d, blockIdx: %d has download error: %v, refCnt is zero, removing buffer",
			bd.bufIdx, blk.idx, err)
		// Remove the buffer descriptor from buffer table.
		btm.removeBufferDescriptor(blk)

		// If in the mean time, no one else has acquired the buffer, release it back to free list.
		// No new readers can come in, as buffer descriptor is already removed from buffer table, hence last reader who
		// finds refCnt as zero, can safely release the buffer.
		if bd.refCnt.Load() == 0 {
			// Release the buffer back to free list.
			log.Debug("BufferTableMgr::canIRead : Released bufferIdx: %d, blockIdx: %d due to download failure",
				bd.bufIdx, blk.idx)
			freeList.releaseBuffer(bd)
		} else {
			log.Debug("BufferTableMgr::canIRead : BufferIdx: %d, blockIdx: %d has download error: %v, but refCnt is now %d, not releasing buffer",
				bd.bufIdx, blk.idx, err, bd.refCnt.Load())
		}
	}

	return err
}

func (bd *bufferDescriptor) release() {
	if bd.refCnt.Add(-1) < 0 {
		err := fmt.Sprintf("bufferDescriptor::release: bufferIdx: %d for blockIdx: %d has negative refCount: %d, usageCnt: %d",
			bd.bufIdx, bd.block.idx, bd.refCnt, bd.usageCount)
		log.Err(err)
		panic(err)
	}
}
