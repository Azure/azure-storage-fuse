package block_cache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// bufferPool is a fixed-size pool of memory buffers for caching block data.
//
// Overview:
//
// bufferPool manages a pool of fixed-size byte slices that are used to cache
// file blocks. It provides a simple interface: getBuffer() to allocate a buffer
// and putBuffer() to return it when done.
//
// Key Features:
//
//   - Fixed size: All buffers are exactly bufSize bytes (typically equals block size)
//   - Bounded capacity: Maximum maxBuffers can be allocated simultaneously
//   - Zero-copy reuse: Uses sync.Pool to minimize GC pressure
//   - Zero buffer: Provides a shared read-only zero buffer for sparse block handling
//
// Thread Safety:
//
// All methods are thread-safe and designed for concurrent access. The sync.Pool
// provides lock-free allocation in the common case, falling back to creation
// when the pool is empty.
//
// Memory Management:
//
// Buffers are allocated on-demand up to maxBuffers. Once maxBuffers is reached,
// getBuffer() returns an error. Callers must call putBuffer() to return buffers
// to the pool, enabling reuse by other operations.
//
// Usage Pattern:
//
//	buf, err := bufPool.getBuffer()
//	if err != nil {
//	    return err // All buffers in use
//	}
//	defer bufPool.putBuffer(buf)
//	// Use buf...
type bufferPool struct {
	pool       sync.Pool    // sync.Pool for efficient buffer reuse and reduced GC pressure
	zeroBuf    []byte       // Read-only zero-filled buffer of size bufSize (shared, never modified)
	bufSize    int          // Size of each buffer in bytes (must match block size)
	maxBuffers int64        // Maximum number of buffers that can be allocated
	curBuffers atomic.Int64 // Current number of allocated buffers (atomic for thread-safe counting)
	maxUsed    atomic.Int64 // High-water mark of buffer usage (for monitoring pool pressure)
}

// initBufferPool creates and initializes a new bufferPool.
//
// Parameters:
//   - bufSize: Size of each buffer in bytes (should match block size)
//   - maxBuffers: Maximum number of buffers that can be allocated
//
// Returns a new bufferPool ready for use.
//
// The pool is configured with a constructor that creates new byte slices of
// size bufSize when the pool is empty. The zero buffer is allocated once
// and shared for all zero-fill operations.
func initBufferPool(bufSize uint64, maxBuffers uint64) *bufferPool {

	log.Info("bufferPool::initBufferPool: Initialized with buffer size: %d bytes, max buffers: %d, total size: %.2f MB",
		bufSize, maxBuffers, float64(maxBuffers*bufSize)/(1024.0*1024.0))

	return &bufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, bufSize)
			},
		},
		zeroBuf:    make([]byte, bufSize),
		bufSize:    int(bufSize),
		maxBuffers: int64(maxBuffers),
	}
}

// getBuffer allocates a buffer from the pool.
//
// This method attempts to get a buffer from the pool. If no buffers are
// available in the pool, a new one is created (up to maxBuffers limit).
//
// Returns:
//   - []byte: A buffer of size bufSize, or nil if all buffers are in use
//   - error: Non-nil if buffer pool is exhausted (curBuffers >= maxBuffers)
//
// Behavior:
//   - Increments curBuffers atomically
//   - Tracks maximum buffer usage for monitoring
//   - Logs warnings when buffer pressure is high
//
// The caller MUST call putBuffer() when done to return the buffer to the pool.
// Failure to do so will leak buffers and eventually exhaust the pool.
//
// Example:
//
//	buf, err := bufPool.getBuffer()
//	if err != nil {
//	    return err // All buffers in use
//	}
//	defer bufPool.putBuffer(buf)
//	// Use buf for I/O operations...
func (bufPool *bufferPool) getBuffer() ([]byte, error) {
	if bufPool.curBuffers.Load() >= bufPool.maxBuffers {
		return nil, fmt.Errorf("buffers exhausted (%d)", bufPool.curBuffers.Load())
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
		log.Warn("bufferPool::getBuffer: Max buffers used: %d out of %d", bufPool.maxUsed.Load(), bufPool.maxBuffers)
	}
	return buf, nil
}

// putBuffer returns a buffer to the pool for reuse.
//
// This method decrements the active buffer count and returns the buffer to
// sync.Pool for reuse by future getBuffer() calls.
//
// Parameters:
//   - buf: The buffer to return (must be non-nil and originally from getBuffer())
//
// Behavior:
//   - Decrements curBuffers atomically
//   - Reslices buffer to full capacity (in case it was sliced smaller)
//   - Returns buffer to sync.Pool for reuse
//   - Panics if curBuffers goes negative (indicates double-free bug)
//   - Panics if buf is nil (indicates caller error)
//
// Important:
// - Each buffer obtained from getBuffer() must be returned exactly once
// - Double-free (calling putBuffer twice) will cause panic
// - Never modify buffer after calling putBuffer (it may be reused immediately)
//
// Example:
//
//	buf, _ := bufPool.getBuffer()
//	defer bufPool.putBuffer(buf)
//	// Use buf...
func (bufPool *bufferPool) putBuffer(buf []byte) {
	if buf == nil {
		panic("Buffer Pool: putBuffer: nil buffer passed!")
	}

	if bufPool.curBuffers.Add(-1) < 0 {
		panic("Buffer Pool: putBuffer: curBuffers went below zero!")
	}

	// Reslice the length of the buffer to its original capacity if it got compacted.
	buf = buf[:bufPool.bufSize]

	bufPool.pool.Put(buf) //nolint:staticcheck
}
