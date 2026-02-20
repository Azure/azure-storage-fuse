package block_cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var bc *BlockCache
var freeList *freeListType
var btm *BufferTableMgr

func setupTestFreeList(t *testing.T, bufSize uint64, memSize uint64) {
	t.Helper()
	var err error
	freeList, err = createFreeList(bufSize, memSize)
	assert.NoError(t, err)
	if bc != nil {
		bc.freeList = freeList
	}
}

func destroyFreeList() {
	if freeList != nil {
		freeList.destroy()
		freeList = nil
	}
	if bc != nil {
		bc.freeList = nil
	}
}
