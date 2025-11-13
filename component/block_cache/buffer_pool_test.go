package block_cache

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/stretchr/testify/assert"
)

var bufPool *BufferPool

func TestInitBufferPool(t *testing.T) {
	bufPool := initBufferPool(8*common.MbToBytes, 64)
	assert.NotNil(t, bufPool)
	assert.Equal(t, int64(0), bufPool.curBuffers.Load())
	assert.Equal(t, int64(0), bufPool.maxUsed.Load())
	assert.Equal(t, int64(64), bufPool.maxBuffers)
}

func TestGetBuffer(t *testing.T) {
	buf, err := bufPool.GetBuffer()
	assert.NoError(t, err)
	assert.Equal(t, len(buf), bufPool.bufSize)
	assert.Equal(t, bufPool.bufSize, len(buf))
	bufPool.PutBuffer(buf)
}

func TestGetBufferExceedLimit(t *testing.T) {
	// Exhaust the buffer pool
	var buffers [][]byte
	for i := int64(0); i < bufPool.maxBuffers; i++ {
		buf, err := bufPool.GetBuffer()
		assert.Equal(t, len(buf), bufPool.bufSize)
		assert.NoError(t, err)
		buffers = append(buffers, buf)
	}

	// Now try to get one more buffer which should fail
	_, err := bufPool.GetBuffer()
	assert.Error(t, err)

	// Release all buffers
	for _, buf := range buffers {
		bufPool.PutBuffer(buf)
	}
}

func TestBufSizeConsistency(t *testing.T) {
	// get all the buffers and check their sizes and reslice them and put them back
	// to the pool, now get them back from the pool and check their sizes again
	var buffers [][]byte
	for i := int64(0); i < bufPool.maxBuffers; i++ {
		buf, err := bufPool.GetBuffer()
		assert.Equal(t, len(buf), bufPool.bufSize)
		assert.NoError(t, err)

		// Reslice the buffer
		buf = buf[:bufPool.bufSize/2]
		buffers = append(buffers, buf)
	}
	// Release all buffers
	for _, buf := range buffers {
		bufPool.PutBuffer(buf)
	}
	// Get all buffers again and check their sizes
	for i := int64(0); i < bufPool.maxBuffers; i++ {
		buf, err := bufPool.GetBuffer()
		assert.Equal(t, len(buf), bufPool.bufSize)
		assert.NoError(t, err)
	}
}

func TestPanicInPutBuffer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic when putting nil buffer")
		}
	}()
	bufPool.PutBuffer(nil)
}

func TestMain(m *testing.M) {
	// Setup code if needed
	bufPool = initBufferPool(8*common.MbToBytes, 64)

	// Run tests
	m.Run()

	// Teardown code if needed
}
