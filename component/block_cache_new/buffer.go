package block_cache_new

import (
	"container/list"
	"encoding/base64"
	"errors"
	"fmt"
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
	b.checksum = nil
	return b
}

func (bp *BufferPool) putBuffer(blk *block) {
	if blk.buf != nil {
		bp.pool.Put(blk.buf)
		blk.buf = nil
	}
}

func getBlockWithReadAhead(idx int, start int, h *handlemap.Handle, file *File) (*block, error) {
	defer func() {
		if r := recover(); r != nil {
			// Print the panic info
			logy.Write([]byte(fmt.Sprintf("Panic: Name: %s, blkidx: %d\n", h.Path, idx)))
			logy.Write([]byte(fmt.Sprintf("Panic recovered: %v\n", r)))
			panic("Read ahead panic")
		}
	}()
	blk, err := getBlockForRead(idx, h, file)
	for i := 1; i <= 3; i++ {
		if i+start < (int(h.Size)+BlockSize-1)/BlockSize {
			logy2.WriteString(fmt.Sprintf("%v, idx %d, read ahead %d\n", h.Path, idx, i+start))
			go getBlockForRead(i+start, h, file)
		}
	}
	return blk, err
}

// Returns the buffer containing block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, h *handlemap.Handle, file *File) (*block, error) {
	var blk *block

	file.Lock()
	if file.readOnly {
		var ok bool
		blk, ok = file.readOnlyBlocks[idx]
		if !ok {
			blk = createBlock(idx, "", committedBlock)
			file.readOnlyBlocks[idx] = blk
		}
		// TODO: blocks are not getting cached for readonly files
	} else {
		if idx >= len(file.blockList) {
			file.Unlock()
			return nil, errors.New("block is out of the blocklist scope")
		}
		h.Size = file.size // This is necessary as next component uses this value to check bounds
		blk = file.blockList[idx]
	}
	file.Unlock()

	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		blk.buf = bPool.getBuffer()
		switch blk.state {
		case localBlock:
			// This case occurs when we get read call on local Blocks which are not even put on the wire.
			return blk, nil
		case uncommitedBlock:
			// This case occurs when we clear the uncommited block from the cache.
			// generally the block should be committed otherwise old data will be served.
			// Todo: Handle this case.
			// We don't hit here yet as we dont invalidate cache entries for local and uncommited blocks
			return blk, errors.New("todo : read for uncommited block which was removed from the cache")
		}
		dataRead, err := bc.NextComponent().ReadInBuffer(internal.ReadInBufferOptions{
			Handle: h,
			Offset: int64(idx * BlockSize),
			Data:   blk.buf.data,
		})
		if err == nil {
			blk.buf.dataSize = int64(dataRead)
			blk.buf.timer = time.Now()
		} else {
			blk.buf = nil
			return blk, err
		}
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
	lenOfBlkLst := len(file.blockList)
	if idx >= lenOfBlkLst {
		// Create at least 1 block. i.e, create blocks in the range (len(blocklist), idx]
		for i := lenOfBlkLst; i <= idx; i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, localBlock)
			file.blockList = append(file.blockList, blk)
			if i == idx {
				//Allocate a buffer for last block.
				//No need to lock the block as we already acquired lock on file
				blk.buf = bPool.getBuffer()
			} else {
				blk.hole = true
			}
		}
		file.Unlock()
		return file.blockList[idx], nil
	}
	h.Size = file.size // This is necessary as next component uses this value to check bounds
	file.Unlock()

	return getBlockForRead(idx, h, file)
}

// Write all the Modified buffers to Azure Storage.
func syncBuffersForFile(h *handlemap.Handle, file *File) (bool, error) {
	var err error = nil
	var fileChanged bool = false

	file.Lock()
	for _, blk := range file.blockList {
		blk.Lock()
		if blk.state == localBlock {
			fileChanged = true
			if blk.hole {
				// This is a sparse block.
				err = punchHole(file)
			} else {
				if blk.buf == nil {
					panic("Local Block must always have some buffer")
				}
				err = syncBuffer(file.Name, file.size, blk)
			}
			if err == nil {
				blk.state = uncommitedBlock
			}
		}
		blk.Unlock()
		if err != nil {
			// One of the buffer has failed to commit, its better to fail early.
			break
		}
	}
	file.Unlock()
	return fileChanged, err
}

func syncBuffer(name string, size int64, blk *block) error {
	blk.id = base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
	if blk.buf == nil {
		panic("Something has seriously messed up")
	}
	err := bc.NextComponent().StageData(
		internal.StageDataOptions{
			Name: name,
			Id:   blk.id,
			Data: blk.buf.data[:getBlockSize(size, blk.idx)],
		},
	)
	return err
}

func syncZeroBuffer(name string) error {
	return bc.NextComponent().StageData(
		internal.StageDataOptions{
			Name: name,
			Id:   zeroBlockId,
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
	if file.readOnly {
		return nil
	}

	for _, blk := range file.blockList {
		if blk.hole {
			blklist = append(blklist, zeroBlockId)
		} else {
			blklist = append(blklist, blk.id)
		}
	}
	err := bc.NextComponent().CommitData(internal.CommitDataOptions{Name: file.Name, List: blklist, BlockSize: uint64(BlockSize)})
	if err == nil {
		file.synced = true
	}

	return err
}

// Release all the buffers to the file if this handle is the last one opened on the file.
func releaseBuffers(f *File) {
	//Lock was already acquired on file
	if f.readOnly {
		for _, blk := range f.readOnlyBlocks {
			blk.Lock()
			bPool.putBuffer(blk)
			blk.Unlock()
		}
		f.readOnlyBlocks = make(map[int]*block)
	}
	for _, blk := range f.blockList {
		blk.Lock()
		bPool.putBuffer(blk)
		blk.Unlock()
	}
}

func releaseBufferForBlock(blk *block) {
	if blk.state == committedBlock {
		bPool.putBuffer(blk)
	}
}
