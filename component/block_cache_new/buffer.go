package block_cache_new

import (
	"container/list"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// TODO: Implement GC after 80% of memory given for blobfuse
var zeroBuffer *Buffer

type Buffer struct {
	data     []byte    // Data holding in the buffer
	dataSize int64     // Size of the data
	synced   int       // Flag representing the data is synced with Azure Storage or not
	timer    time.Time // Timer for buffer expiry
	checksum []byte    // Check sum for the data
}

type BufferPool struct {
	pool       sync.Pool  // Pool used to get and put the buffers.
	bufferList *list.List // List of Outstanding buffers.
	bufferSize int        // Size of the each buffer in the bytes for this pool
}

func createBufferPool(bufSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		bufferList: list.New(),
		bufferSize: bufSize,
	}
	zeroBuffer = bPool.getBuffer()
	go doGC()
	return bPool
}

type gcNode struct {
	file *File
	idx  int // block index inside the file
}

func doGC() {

}

func (bp *BufferPool) getBuffer() *Buffer {
	b := bp.pool.Get().(*Buffer)
	if b.data == nil {
		b.data = make([]byte, BlockSize)
	} else {
		copy(b.data, zeroBuffer.data)
	}
	//b.data = b.data[:0]
	b.synced = -1
	b.checksum = nil
	return b
}

func (bp *BufferPool) putBuffer(b *Buffer) {
	bp.pool.Put(b)
}

func getBlockWithReadAhead(idx int, h *handlemap.Handle, file *File) (*block, error) {
	for i := 1; i <= 3; i++ {
		go getBlockForRead(idx+i, h, file)
	}
	return getBlockForRead(idx, h, file)
}

// Returns the buffer containing block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, h *handlemap.Handle, file *File) (*block, error) {
	var download bool = false
	var blk *block

	file.Lock()
	if idx >= len(file.blockList) {
		file.Unlock()
		return nil, errors.New("block is out of the blocklist scope")
	}
	h.Size = file.size // This is necessary as next component uses this value to check bounds
	blk = file.blockList[idx]
	if blk.buf == nil {
		// I will start the download
		download = blk.block_type
		blk.buf = bPool.getBuffer()
	}
	file.Unlock()

	if download {
		blk.Lock()
		buf := blk.buf
		dataRead, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Handle: h,
			Offset: int64(idx * BlockSize),
			Data:   file.blockList[idx].buf.data,
		})
		if err == nil {
			buf.dataSize = int64(dataRead)
			buf.synced = 1
			buf.timer = time.Now()
			close(blk.downloadStatus) // This is causing panic sometimes while reading sequential files of large size find out why?
		} else {
			buf = nil
			file.blockList[idx].downloadStatus <- 1 //something is wrong here can i update it without acquring lock??
		}
		blk.Unlock()
	}
	_, ok := <-blk.downloadStatus
	if ok {
		return nil, errors.New("failed to get the block")
	}
	return blk, nil
}

// This call will return buffer for writing for the block
// This call should always return some buffer if len(blocklist) <= 50000
func getBlockForWrite(idx int, h *handlemap.Handle, file *File) (*block, error) {
	if idx >= MAX_BLOCKS {
		return nil, errors.New("write not supported space completed") // can we return ENOSPC error here?
	}

	file.Lock()
	file.synced = false
	len_of_blocklist := len(file.blockList)
	if idx >= len_of_blocklist {
		// Create at least 1 block. i.e, create blocks in the range (len(blocklist), idx]
		// Close the download channel as it is not necessary
		for i := len_of_blocklist; i <= idx; i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, local_block)
			close(blk.downloadStatus)
			file.blockList = append(file.blockList, blk)
		}
	}
	h.Size = file.size // This is necessary as next component uses this value to check bounds
	file.Unlock()

	return getBlockForRead(idx, h, file)
}

// Write all the Modified buffers to Azure Storage.
func syncBuffersForFile(h *handlemap.Handle, file *File) error {
	var err error = nil

	file.Lock()
	len_of_blocklist := len(file.blockList)
	for i := 0; i < len_of_blocklist; i++ {
		if file.blockList[i].block_type == local_block {
			if file.blockList[i].buf == nil && i != len_of_blocklist-1 {
				err = punchHole(file)
				continue
			}
			err = syncBuffer(file.Name, file.size, file.blockList[i])
			if err != nil {
				file.blockList[i].block_type = remote_block
			}
		} else {
			if file.blockList[i].buf != nil && file.blockList[i].buf.synced == 0 {
				syncBuffer(file.Name, file.size, file.blockList[i])
			}
		}
	}
	file.Unlock()
	return err
}

func syncBuffer(name string, size int64, blk *block) error {
	blk.Lock()
	blk.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	err := bc.NextComponent().StageData(
		internal.StageDataOptions{
			Name: name,
			Id:   blk.id,
			Data: blk.buf.data[:getBlockSize(size, blk.idx)],
		},
	)
	if err == nil {
		blk.buf.synced = 1
	}
	blk.Unlock()
	return err
}

func syncZeroBuffer(name string) error {
	return bc.NextComponent().StageData(
		internal.StageDataOptions{
			Name: name,
			Id:   zero_block_id,
			Data: zeroBuffer.data,
		},
	)

}

// stages empty block for the hole
func punchHole(f *File) error {
	if f.holePunched {
		return nil
	}
	err := syncZeroBuffer(f.Name)
	if err == nil {
		f.holePunched = true
	}

	return err
}

func commitBuffersForFile(h *handlemap.Handle, file *File) error {
	var blklist []string
	file.Lock()
	defer file.Unlock()
	len_of_blocklist := len(file.blockList)
	for i := 0; i < len_of_blocklist; i++ {
		if file.blockList[i].block_type == local_block && file.blockList[i].buf == nil { //dirty way to do stuff
			blklist = append(blklist, zero_block_id)
		} else {
			blklist = append(blklist, file.blockList[i].id)
		}
	}
	// if file.synced {
	// 	return nil
	// }
	err := bc.NextComponent().CommitData(internal.CommitDataOptions{Name: file.Name, List: blklist, BlockSize: uint64(BlockSize)})
	if err == nil {
		file.synced = true
	}

	return err
}

// Release all the buffers to the file if this handle is the last one opened on the file.
func releaseBuffers(f *File) {
	//Lock was already acquired on file
	len_of_blocklist := len(f.blockList)
	for i := 0; i < len_of_blocklist; i++ {
		if f.blockList[i].buf != nil {
			bPool.putBuffer(f.blockList[i].buf)
		}
		f.blockList[i].buf = nil
	}
}

func releaseBufferForBlock(b *block) {
	if b.buf != nil {
		bPool.putBuffer(b.buf)
		b.buf = nil
	}
}
