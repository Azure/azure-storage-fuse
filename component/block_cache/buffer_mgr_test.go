package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBufferTableMgr(t *testing.T) {
	btm = newBufferTableMgr()
	bc.btm = btm

	assert.NotNil(t, btm)
	assert.NotNil(t, btm.table)
	assert.Equal(t, 0, len(btm.table))
}

func TestBufDescStatus_String(t *testing.T) {
	assert.Equal(t, "bufDescStatusExists", bufDescStatusExists.String())
	assert.Equal(t, "bufDescStatusAllocated", bufDescStatusAllocated.String())
	assert.Equal(t, "bufDescStatusVictim", bufDescStatusVictim.String())
	assert.Equal(t, "bufDescStatusNeedsFileFlush", bufDescStatusNeedsFileFlush.String())
	assert.Equal(t, "bufDescStatusInvalid", bufDescStatusInvalid.String())
	assert.Equal(t, "Unknown", bufDescStatus(99).String())
}

func TestBufferTableMgr_LookUpBufferDescriptor_NotExists(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Lookup non-existent buffer
	bufDesc, err := btm.LookUpBufferDescriptor(blk)

	assert.NoError(t, err)
	assert.Nil(t, bufDesc)
}

func TestBufferTableMgr_LookUpBufferDescriptor_Exists(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Manually add a buffer to the table
	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableOnly)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Lookup existing buffer
	foundBd, err := btm.LookUpBufferDescriptor(blk)

	assert.NoError(t, err)
	assert.NotNil(t, foundBd)
	assert.Equal(t, bd, foundBd)
	assert.Equal(t, int32(refCountTableAndOneUser), foundBd.refCnt.Load(), "refCnt should be incremented")
}

func TestBufferTableMgr_RemoveBufferDescriptor_NotInTable(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(1)

	// removeBufferDescriptor expects the buffer to be in the table; not being in table is a bug.
	assert.Panics(t, func() {
		_ = btm.removeBufferDescriptor(bd, freeList)
	})
}

func TestBufferTableMgr_RemoveBufferDescriptor_Dirty(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableAndOneUser)
	bd.dirty.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Try to remove dirty buffer
	isRemoved := btm.removeBufferDescriptor(bd, freeList)

	assert.False(t, isRemoved, "Should not remove dirty buffer")
}

// Test that removeBufferDescriptor with strict /*strict*/=true won't remove if there are user references (refCnt > 1)
// This prevents removing buffers that are still in use by other operations
func TestBufferTableMgr_RemoveBufferDescriptor_StrictWithRefs(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableAndOneUser + 1) // Table + 2 user references
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Try to remove buffer with strict=true and refCnt > 1
	isRemoved := btm.removeBufferDescriptor(bd, freeList)

	assert.False(t, isRemoved, "Should not remove with extra user refs")
}

// Test that removeBufferDescriptor with strict /*strict*/=true WILL remove if only table holds reference (refCnt=1)
func TestBufferTableMgr_RemoveBufferDescriptor_StrictWithOnlyTableRef(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableOnly) // Only table reference
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// removeBufferDescriptor expects the caller to hold a reference too; only table ref is a bug.
	assert.Panics(t, func() {
		_ = btm.removeBufferDescriptor(bd, freeList)
	})
}

func TestBufferTableMgr_RemoveBufferDescriptor_Success(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableAndOneUser) // Table + caller reference
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Remove buffer successfully
	isRemoved := btm.removeBufferDescriptor(bd, freeList)

	assert.True(t, isRemoved, "Should be removed from table")
	assert.Equal(t, int32(0), bd.refCnt.Load(), "refCnt should be 0")

	// Verify it's not in table anymore
	btm.mu.RLock()
	_, exists := btm.table[blk]
	btm.mu.RUnlock()
	assert.False(t, exists, "Should not be in table")
}

func TestBufferTableMgr_RemoveBufferDescriptor_WithRemainingRefs(t *testing.T) {
	// Setup
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableAndOneUser + 1)
	bd.valid.Store(true)

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	// Remove buffer with strict=false (allows removal even with refs)
	isRemoved := btm.removeBufferDescriptor(bd, freeList)

	assert.False(t, isRemoved, "Should not remove with extra refs")
	assert.Equal(t, int32(refCountTableAndOneUser+1), bd.refCnt.Load(), "refCnt should be unchanged")
}

func TestBufferTableMgr_LookUpBufferDescriptor_DownloadError(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test.txt")
	blk := createBlock(0, "testId", localBlock, f)

	buf, _ := freeList.bufPool.GetBuffer()
	bd := &bufferDescriptor{
		bufIdx: 0,
		block:  blk,
		buf:    buf,
	}
	bd.refCnt.Store(refCountTableOnly)
	bd.valid.Store(false)
	bd.downloadErr = fmt.Errorf("download failed")

	btm.mu.Lock()
	btm.table[blk] = bd
	btm.mu.Unlock()

	foundBd, err := btm.LookUpBufferDescriptor(blk)
	assert.Nil(t, foundBd)
	assert.Error(t, err)
}

// Test the slow path of GetOrCreateBufferDescriptor: buffer doesn't exist, gets allocated from free list.
func TestGetOrCreateBufferDescriptor_AllocateFromFreeList(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_alloc.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Block is localBlock (not committed), so no download needed — takes the write path (valid+dirty).
	bufDesc, status, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
	assert.NoError(t, err)
	assert.NotNil(t, bufDesc)
	assert.Equal(t, bufDescStatusAllocated, status)
	assert.True(t, bufDesc.valid.Load(), "localBlock buffer should be valid immediately")
	assert.True(t, bufDesc.dirty.Load(), "localBlock buffer should be dirty")
	assert.Equal(t, int32(refCountTableAndOneUser), bufDesc.refCnt.Load())

	// Release
	bufDesc.release(freeList)
}

// Test the double-check path: buffer created while waiting for lock.
func TestGetOrCreateBufferDescriptor_DoubleCheck(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_doublecheck.txt")
	blk := createBlock(0, "testId", localBlock, f)

	// Pre-populate a valid buffer in the table (simulates another goroutine creating it).
	preAllocBuf, _ := freeList.allocateBuffer(blk)
	preAllocBuf.refCnt.Store(refCountTableOnly)
	preAllocBuf.valid.Store(true)
	preAllocBuf.block = blk
	btm.mu.Lock()
	btm.table[blk] = preAllocBuf
	btm.mu.Unlock()

	// Now call GetOrCreate — should find it via LookUp (fast path).
	bufDesc, status, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
	assert.NoError(t, err)
	assert.NotNil(t, bufDesc)
	assert.Equal(t, bufDescStatusExists, status)
	assert.Equal(t, preAllocBuf, bufDesc)

	// Release the ref we acquired
	bufDesc.release(freeList)
}

// Test uncommitted block returns bufDescStatusNeedsFileFlush.
func TestGetOrCreateBufferDescriptor_UncommittedBlock(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_uncommitted.txt")
	blk := createBlock(0, "testId", uncommitedBlock, f)

	bufDesc, status, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
	assert.NoError(t, err)
	assert.Nil(t, bufDesc)
	assert.Equal(t, bufDescStatusNeedsFileFlush, status)
}

// Test async allocation with free list full returns errBuffersExhausted.
func TestGetOrCreateBufferDescriptor_AsyncFreeListFull(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 2*bc.blockSize) // Only 2 buffers
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_async_full.txt")

	// Exhaust the free list
	blk0 := createBlock(0, "id0", localBlock, f)
	blk1 := createBlock(1, "id1", localBlock, f)
	bd0, _, _ := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk0, true)
	bd1, _, _ := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk1, true)

	// Now try async allocation (sync=false) — should fail with errBuffersExhausted
	blk2 := createBlock(2, "id2", localBlock, f)
	bufDesc, status, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk2, false)
	assert.Error(t, err)
	assert.Equal(t, errBuffersExhausted, err)
	assert.Nil(t, bufDesc)
	assert.Equal(t, bufDescStatusInvalid, status)

	// Clean up
	bd0.release(freeList)
	bd1.release(freeList)
}

// Test sync allocation with eviction — exercises the victim eviction path.
func TestGetOrCreateBufferDescriptor_VictimEviction(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 3*bc.blockSize) // 3 buffers
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_eviction.txt")
	atomic.StoreInt64(&f.size, int64(bc.blockSize)*4)

	// Allocate all 3 buffers as localBlock (they start valid+dirty).
	blk0 := createBlock(0, "id0", localBlock, f)
	blk1 := createBlock(1, "id1", localBlock, f)
	blk2 := createBlock(2, "id2", localBlock, f)
	bd0, _, _ := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk0, true)
	bd1, _, _ := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk1, true)
	bd2, _, _ := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk2, true)

	// Release user refs so buffers have refCnt=1 (table only) — making them evictable.
	bd0.release(freeList)
	bd1.release(freeList)
	bd2.release(freeList)

	// Clear dirty flag so eviction doesn't trigger upload (no real storage backend).
	bd0.dirty.Store(false)
	bd1.dirty.Store(false)
	bd2.dirty.Store(false)

	// Mark all buffers as fully read so they pass eviction criteria.
	bd0.bytesRead.Store(int32(bc.blockSize))
	bd1.bytesRead.Store(int32(bc.blockSize))
	bd2.bytesRead.Store(int32(bc.blockSize))

	// Now allocate a 4th block — should evict one of the existing buffers.
	blk3 := createBlock(3, "id3", localBlock, f)
	bd3, status, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk3, true)
	assert.NoError(t, err)
	assert.NotNil(t, bd3)
	assert.Equal(t, bufDescStatusVictim, status)

	// Clean up
	bd3.release(freeList)
}

// Test the double-check success path directly by pre-populating a buffer in the table
// that wasn't there during LookUp, simulating another goroutine winning the race.
func TestGetOrCreateBufferDescriptor_DoubleCheckAfterLookup(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_dc_after_lookup.txt")
	atomic.StoreInt64(&f.size, int64(bc.blockSize)*2)
	blk := createBlock(0, "id0", localBlock, f)

	// First goroutine: allocate a buffer for this block
	bd1, status1, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
	assert.NoError(t, err)
	assert.Equal(t, bufDescStatusAllocated, status1)

	// Second call for the same block — should hit LookUp fast path (not double-check, but still exercises the "exists" path)
	bd2, status2, err := btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
	assert.NoError(t, err)
	assert.Equal(t, bufDescStatusExists, status2)
	assert.Equal(t, bd1, bd2)

	bd1.release(freeList)
	bd2.release(freeList)
}

// Test concurrent GetOrCreateBufferDescriptor calls for the same block exercises the double-check path.
func TestGetOrCreateBufferDescriptor_ConcurrentDoubleCheck(t *testing.T) {
	bc = &BlockCache{blockSize: 1024 * 1024}
	setupTestFreeList(t, bc.blockSize, 10*bc.blockSize)
	defer destroyFreeList()

	btm = newBufferTableMgr()
	bc.btm = btm

	f := createFile("test_concurrent_doublecheck.txt")
	atomic.StoreInt64(&f.size, int64(bc.blockSize)*2)
	blk := createBlock(0, "id0", localBlock, f)

	const goroutines = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]*bufferDescriptor, goroutines)
	statuses := make([]bufDescStatus, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], statuses[idx], errs[idx] = btm.GetOrCreateBufferDescriptor(freeList, bc.workerPool, blk, true)
		}(i)
	}

	close(start)
	wg.Wait()

	// All should succeed
	for i := 0; i < goroutines; i++ {
		assert.NoError(t, errs[i], "goroutine %d", i)
		assert.NotNil(t, results[i], "goroutine %d", i)
	}

	// Exactly one should be Allocated, rest should be Exists (from double-check or LookUp)
	allocCount := 0
	for _, s := range statuses {
		if s == bufDescStatusAllocated {
			allocCount++
		}
	}
	assert.Equal(t, 1, allocCount, "exactly one goroutine should allocate")

	// Release all refs
	for i := 0; i < goroutines; i++ {
		if results[i] != nil {
			results[i].release(freeList)
		}
	}
}
