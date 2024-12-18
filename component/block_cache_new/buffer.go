package block_cache_new

import (
	"container/list"
	"errors"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

//TODO: Implement GC after 80% of memory given for blobfuse

type Buffer struct {
	data       []byte     // Data holding in the buffer
	dataSize   int64      // Size of the data
	synced     int        // Flag representing the data is synced with Azure Storage or not
	timer      time.Timer // Timer for buffer expiry
	blockIndex int        // Block Index inside blob, -1 if the blob doesn't contain blocks
	checksum   []byte     // Check sum for the data
	sync.RWMutex
}

type BufferPool struct {
	pool       sync.Pool  // Pool used to get and put the buffers.
	bufferList *list.List // List of Outstanding buffers.
	bufferSize int        // Size of the each buffer in the bytes for this pool
}

func createBufferPool(bufSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		bufferList: list.New(),
		bufferSize: bufSize,
	}
}

func (bp *BufferPool) getBuffer() *Buffer {
	b := bp.pool.Get().(*Buffer)
	if b.data == nil {
		b.data = make([]byte, BlockSize)
	}
	//b.data = b.data[:0]
	b.synced = -1
	b.blockIndex = -1
	b.checksum = nil
	return b
}

func (bp *BufferPool) putBuffer(b *Buffer) {
	bp.pool.Put(b)
}

func getBlockForRead(idx int, h *handlemap.Handle, file *File) ([]byte, error) {
	var download bool = false
	file.Lock()
	if idx >= len(file.blockList) {
		file.Unlock()
		return nil, errors.New("block is out of the blocklist scope")
	}
	if file.blockList[idx].buf == nil {
		// I will start the download
		download = true
		file.blockList[idx].buf = bPool.getBuffer()
	}
	file.Unlock()

	if download {
		dataRead, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Handle: h,
			Offset: int64(idx * BlockSize),
			Data:   file.blockList[idx].buf.data,
		})
		file.blockList[idx].buf.dataSize = int64(dataRead)
		if err != nil {
			file.blockList[idx].downloadStatus <- 1
		} else {
			close(file.blockList[idx].downloadStatus)
		}
	}
	_, ok := <-file.blockList[idx].downloadStatus
	if ok {
		return nil, errors.New("failed to get the block")
	}
	b := file.blockList[idx].buf
	return b.data[0:b.dataSize], nil
}
