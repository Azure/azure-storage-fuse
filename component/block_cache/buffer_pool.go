package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// This pool is shared across the block-cache for allocation of the buffers, Currently this a fixed size singleton buffer
// pool implementation, where caller needs to request the buffer using GetBuffer() and it's responsibility of the caller
// to release the buffer using PutBuffer() after its use. The size of the buffers requested from the buffer will have
// length bufSize.
type BufferPool struct {
	pool       sync.Pool    // sync.Pool to relieve GC
	bufSize    int          // size of buffers in this pool
	maxBuffers int64        // max allocated buffers allowed
	curBuffers atomic.Int64 // buffers currently allocated
	maxUsed    atomic.Int64 // max buffers used at any point of time
}

func initBufferPool(bufSize uint64, maxBuffers uint64) *BufferPool {

	log.Info("Buffer Pool: Initialized with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufSize, maxBuffers, float64(maxBuffers*bufSize)/(1024.0*1024.0))

	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		bufSize:    int(bufSize),
		maxBuffers: int64(maxBuffers),
	}
}

func (bufPool *BufferPool) GetBuffer() ([]byte, error) {
	if bufPool.curBuffers.Load() >= bufPool.maxBuffers {
		return nil, fmt.Errorf("Buffers Exhausted (%d)", bufPool.curBuffers.Load())
	}

	buf := bufPool.pool.Get().([]byte)

	bufPool.curBuffers.Add(1)

	//
	// Track max buffers used at any point of time.
	// Due to race between multiple threads, this may not be exact value, but that's okay, we just need
	// rough estimate of whether buffers are being held for long.
	//
	if bufPool.curBuffers.Load() > bufPool.maxUsed.Load() {
		bufPool.maxUsed.Store(bufPool.curBuffers.Load())
		log.Warn("Buffer Pool: Max buffers used: %d out of %d", bufPool.maxUsed.Load(), bufPool.maxBuffers)
	}
	return buf, nil
}

func (bufPool *BufferPool) PutBuffer(buf []byte) {
	if bufPool.curBuffers.Add(-1) < 0 {
		panic("Buffer Pool: PutBuffer: curBuffers went below zero!")
	}

	if buf == nil {
		panic("Buffer Pool: PutBuffer: nil buffer passed!")
	}

	// Reslice the length of the buffer to its original capacity if it got compacted.
	buf = buf[:bufPool.bufSize]

	bufPool.pool.Put(buf)
}
