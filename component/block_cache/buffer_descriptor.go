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
	refCnt atomic.Int32
	// # of bytes used in this buffer. This is useful in the eviction logic.
	usageCount atomic.Int32
	// # of eviction cycles passed since this buffer was assigned.
	numEvictionCyclesPassed atomic.Int32

	// TODO: use atomic int for these bools.
	evicted atomic.Bool
	dirty   atomic.Bool

	// This lock must be held while reading/writing the content of the buffer.
	// While downloading the buffer content, this lock must be held in exclusively, while reading the data from the buffer,
	// this lock must be held in shared mode.
	contentLock sync.RWMutex
	buf         []byte
	valid       atomic.Bool
	downloadErr error
}

// ensureBufferValidForRead: ensures that the buffer is valid, i.e., no download error, if there is download error,
func (bd *bufferDescriptor) ensureBufferValidForRead() error {
	// Wait for the Download to happen. if there was an error during download, it will be set in downloadErr.
	bd.contentLock.RLock()
	bd.contentLock.RUnlock()

	if bd.valid.Load() && bd.downloadErr == nil {
		// Safe to read data from this buffer.
		return nil
	}

	if !bd.valid.Load() && bd.downloadErr != nil {
		// There was an error during download.
		return bd.downloadErr
	}

	// This should not happen.
	err := fmt.Sprintf("bufferDescriptor::ensureBufferValidForRead: Inconsistent state for bufferIdx: %d, blockIdx: %d, valid: %v, downloadErr: %v",
		bd.bufIdx, bd.block.idx, bd.valid.Load(), bd.downloadErr)
	panic(err)
}

// release: releases the buffer descriptor, decrements the ref count.
// If the ref count reaches -1, it returns true, indicating that the buffer descriptor is returned to free list.
func (bd *bufferDescriptor) release() bool {
	newRefCnt := bd.refCnt.Add(-1)

	if newRefCnt == -1 {
		// This means the buffer descriptor has removed from the buffer table manager, safe to return it back to free list.
		log.Debug("bufferDescriptor::release: Releasing bufferIdx: %d for blockIdx: %d back to free list, usageCnt: %d",
			bd.bufIdx, bd.block.idx, bd.usageCount.Load())
		freeList.releaseBuffer(bd)
		return true
	} else if newRefCnt < -1 {
		err := fmt.Sprintf("bufferDescriptor::release: bufferIdx: %d for blockIdx: %d has negative refCount: %d, usageCnt: %d",
			bd.bufIdx, bd.block.idx, bd.refCnt, bd.usageCount)
		log.Err(err)
		panic(err)
	}

	return false
}

func (bd *bufferDescriptor) reset() {
	bd.block = nil
	bd.nxtFreeBuffer = -1
	bd.refCnt.Store(0)
	bd.usageCount.Store(0)
	bd.numEvictionCyclesPassed.Store(0)
	bd.valid.Store(false)
	bd.evicted.Store(false)
	bd.dirty.Store(false)
	bd.downloadErr = nil
}
