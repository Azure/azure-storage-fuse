package block_cache_new

import (
	"context"
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
var defaultBlockTimeout = 100 * time.Microsecond

type Buffer struct {
	data     []byte      // Data holding in the buffer
	dataSize int64       // Size of the data
	expiry   *time.Timer // Timer for buffer expiry
	checksum []byte      // Checksum for the data
}

func (buf *Buffer) resetTimer() {
	buf.expiry.Reset(defaultBlockTimeout)
}

type BufferPool struct {
	pool       sync.Pool   // Pool used to get and put the buffers.
	localBlks  chan *block // Local Blocks which are yet to upload. This list can contain only Local Blocks
	syncedBlks chan *block // Current Synced Blocks. This list can contain both commited and Uncommited blocks.
	bufferSize int         // Size of the each buffer in the bytes for this pool
}

func createBufferPool(bufSize int) *BufferPool {
	bPool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(Buffer)
			},
		},
		// Sizes of following channels need to be decided.
		localBlks:  make(chan *block, 5000), // These blks are modified blocks and in local.
		syncedBlks: make(chan *block, 5000), // These blks are synced with the azure storage.
		bufferSize: bufSize,
	}
	zeroBuffer = bPool.getBuffer()
	go bPool.asyncUploadScheduler()
	go bPool.blockCleaner()
	return bPool
}

// It will Schedule a task for uploading the block to Azure Storage
// It will schedule a local block to upload when
// 1. timer since last operation on the block was over.
// Schedules the task and push the block into uploadingBlks.
func (bp *BufferPool) asyncUploadScheduler() {
	for blk := range bp.localBlks {
		<-blk.buf.expiry.C
		blk.Lock()
		if blk.state == localBlock {
			select {
			case _, ok := <-blk.uploadInProgress: // Check if sync upload is in progress
				if !ok {
					logy.Write([]byte(fmt.Sprintf("Async Uploader: Scheduling blk idx: %d, filePath: %s\n", blk.idx, blk.file.Name)))
					scheduleUpload(blk, false)
				}
			case <-time.NewTimer(5 * time.Millisecond).C:
			}
		}
		blk.Unlock()
	}
}

func scheduleUpload(blk *block, sync bool) {
	blk.uploadDone = make(chan error, 1)
	blk.uploadInProgress = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	blk.uploadCtx = ctx
	taskDone := make(chan struct{}, 1)
	blk.cancelOngoingAsyncUpload = func() {
		cancel()
		<-taskDone
	}
	wp.createTask(ctx, taskDone, true, sync, blk)
}

func scheduleDownload(blk *block, sync bool) {

}

// Responsible for giving blocks back to the pool,
// It can happen in 2 scenarios:
// 1. Block is not referenced by any open handle.
// 2. timer since last operation on the block was over.
func (bp *BufferPool) blockCleaner() {
	for blk := range bp.localBlks {
		blk.Lock()
		releaseBufferForBlock(blk)
		blk.Unlock()
	}
}

func (bp *BufferPool) getBuffer() *Buffer {
	b := bp.pool.Get().(*Buffer)
	if b.data == nil {
		b.data = make([]byte, BlockSize)
	} else {
		copy(b.data, zeroBuffer.data)
	}
	//b.data = b.data[:0]
	b.expiry = time.NewTimer(defaultBlockTimeout)
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
			blk = createBlock(idx, "", committedBlock, file)
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
	_, err := getBlock(idx, blk, sync)
	return blk, err
}

func getBlock(idx int, blk *block, sync bool) (state blockState, err error) {
	blk.Lock()
	defer blk.Unlock()
	if blk.buf == nil {
		blk.downloadDone = make(chan error, 1)
		wp.createTask(context.Background(), nil, false, sync, blk)
		if sync {
			err = <-blk.downloadDone
		}
		return
	}
	err = <-blk.downloadDone
	state = blk.state
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
		// Update the state of the last block to local as null data may get's appended to it.
		if lenOfBlkLst > 0 {
			changeStateOfBlockToLocal(lenOfBlkLst-1, file.blockList[lenOfBlkLst-1])
		}

		// Create at least 1 block. i.e, create blocks in the range (len(blocklist), idx]
		for i := lenOfBlkLst; i <= idx; i++ {
			id := base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16))
			blk := createBlock(i, id, localBlock, file)
			file.blockList = append(file.blockList, blk)
			if i == idx {
				//Allocate a buffer for last block.
				//No need to lock the block as we already acquired lock on file
				blk.buf = bPool.getBuffer()
			} else {
				blk.hole = true
			}
		}
		blk := file.blockList[idx]
		bPool.localBlks <- blk
		file.Unlock()
		return blk, nil
	}
	blk := file.blockList[idx]
	file.Unlock()
	blkState, err := getBlock(idx, blk, true)
	if blkState == committedBlock || blkState == uncommitedBlock {
		bPool.localBlks <- blk
	}
	return blk, err
}

// Schedules the upload and return true if the block is local/uncommited.
func syncUploader(blk *block) bool {
	blk.Lock()
	defer blk.Unlock()
	if blk.state == localBlock {
		if blk.hole {
			// This is a sparse block.
			err := punchHole(blk.file)
			if err == nil {
				blk.state = uncommitedBlock
			}
		} else {
			if blk.buf == nil {
				panic("Local Block must always have some buffer")
			}
			// Check if async upload is in progress.
			select {
			case err, ok := <-blk.uploadDone:
				if ok && err == nil && !errors.Is(blk.uploadCtx.Err(), context.Canceled) {
					// Upload was already completed by async scheduler and no more write came after it.
					blk.state = uncommitedBlock
					close(blk.uploadDone)
					close(blk.uploadInProgress)
				} else {
					scheduleUpload(blk, true)
				}
			case <-time.NewTimer(20 * time.Millisecond).C:
				// Taking toomuch time for async upload to complete, cancel the upload and schedule a new one.
				blk.cancelOngoingAsyncUpload()
				scheduleUpload(blk, true)
			}
			//err = syncBuffer(file.Name, file.size, blk)
		}
		return true
	} else if blk.state == uncommitedBlock {
		return true
	}
	return false
}

// Write all the Modified buffers to Azure Storage and return whether while is modified or not.
func syncBuffersForFile(h *handlemap.Handle, file *File) (bool, error) {
	var err error = nil
	var fileChanged bool = false

	file.Lock()
	for _, blk := range file.blockList {
		if syncUploader(blk) {
			fileChanged = true
		}
	}
	for _, blk := range file.blockList {
		err, ok := <-blk.uploadDone
		if ok && err == nil {
			blk.Lock()
			blk.state = uncommitedBlock
			close(blk.uploadDone)
			close(blk.uploadInProgress)
			blk.Unlock()
		}
		if err != nil {
			panic(fmt.Sprintf("Upload doesn't happen for the block idx : %d, file : %s\n", blk.idx, blk.file.Name))
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
			Ctx:  context.Background(),
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
	logy.Write([]byte(fmt.Sprintf("BlockCache::commitFile : %s\n", file.Name)))
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
	if blk.state == committedBlock || blk.state == uncommitedBlock {
		bPool.putBuffer(blk)
	}
}
