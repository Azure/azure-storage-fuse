package block_cache_new

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	checksum []byte    // Checksum for the data
}

type BufferPool struct {
	pool            sync.Pool   // Pool used to get and put the buffers.
	localBlks       chan *block // Local Blocks which are yet to upload. This list can contain only Local Blocks
	uploadingBlks   chan *block // Blocks which are uploading. This list contains local blocks.
	downloadingBlks chan *block // Blocks which are downloading. This list contains Commited blocks.
	syncedBlks      chan *block // Current Synced Blocks. This list can contain both commited and Uncommited blocks.
	bufferSize      int         // Size of the each buffer in the bytes for this pool
}

func createBufferPool(bufSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		localBlks:       make(chan *block, 100), // Sizes of these channels need to be decided.
		uploadingBlks:   make(chan *block, 100),
		downloadingBlks: make(chan *block, 100),
		syncedBlks:      make(chan *block, 100),
		bufferSize:      bufSize,
	}
	zeroBuffer = bPool.getBuffer()
	go bPool.asyncUploadScheduler()
	go bPool.uploadWatcher()
	go bPool.downloadWatcher()
	go bPool.blockCleaner()
	return bPool
}

type blkNode struct {
	file *File
	blk  *block
}

// It will Schedule a task for uploading the block to Azure Storage
// It will schedule a local block to upload when
// 1. timer since last operation on the block was over.
// Schedules the task and push the block into uploadingBlks.
func (bp *BufferPool) asyncUploadScheduler() {
	for blk := range bp.localBlks {
		blk.Lock()

		blk.Unlock()
	}
}

// Checks the upload status of a block and responsible for moving it into the synced blocks
func (bp *BufferPool) uploadWatcher() {

}

// Checks the download status of a block and responsible for moving it into the synced blocks
func (bp *BufferPool) downloadWatcher() {

}

// Responsible for giving blocks back to the pool,
// It can happen in 2 scenarios:
// 1. Block is not referenced by any open handle.
// 2. timer since last operation on the block was over.
func (bp *BufferPool) blockCleaner() {
	// totMemory :=
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
	blk, err := getBlockForRead(idx, h, file, true)
	for i := 1; i <= 3; i++ {
		if i+start < (int(atomic.LoadInt64(&file.size))+BlockSize-1)/BlockSize {
			logy2.WriteString(fmt.Sprintf("%v, idx %d, read ahead %d\n", h.Path, idx, i+start))
			getBlockForRead(i+start, h, file, false)
		}
	}
	return blk, err
}

// Returns the buffer containing block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, h *handlemap.Handle, file *File, sync bool) (*block, error) {
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
		//h.Size = file.size // This is necessary as next component uses this value to check bounds
		blk = file.blockList[idx]
	}
	file.Unlock()
	return blk, getBlock(idx, h, file, blk, sync)
}

func getBlock(idx int, h *handlemap.Handle, f *File, blk *block, sync bool) (err error) {
	blk.Lock()
	if blk.buf == nil {
		blk.Unlock()
		wp.createTask(false, sync, f, blk)
		if sync {
			err = <-blk.downloadDone
		}
		return
	}
	blk.Unlock()
	err = <-blk.downloadDone
	return
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
				close(blk.downloadDone)
			} else {
				blk.hole = true
			}
		}
		file.Unlock()
		return file.blockList[idx], nil
	}
	blk := file.blockList[idx]
	//h.Size = file.size // This is necessary as next component uses this value to check bounds
	file.Unlock()

	return blk, getBlock(idx, h, file, blk, true)
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
