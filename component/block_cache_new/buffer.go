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
)

// TODO: Implement GC after 80% of memory given for blobfuse
var zeroBuffer *Buffer
var defaultBlockTimeout = 1000 * time.Millisecond

type Buffer struct {
	data     []byte // Data hoding in the buffer
	valid    bool   // is date present in the buffer valid?
	checksum []byte // Checksum for the data
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
	zeroBuffer = bPool.getBuffer(true)
	go bPool.asyncUploadScheduler()
	go bPool.asyncBufferReclaimation()
	return bPool
}

// It will Schedule a task for uploading the block to Azure Storage asynchronously
// when timer since last operation on the block was over.
func (bp *BufferPool) asyncUploadScheduler() {
	for blk := range bp.localBlks {
		<-blk.asyncUploadTimer.C
		uploader(blk, asyncRequest)
	}
}

// Responsible for giving blocks back to the pool,
// This can only take the buffers from the commited/uncommited blocks
// and not from the local block
// This will kick when the memory of the process is getting exhausted:
// So we will take back the memory from the already open handles
func (bp *BufferPool) asyncBufferReclaimation() {
	// for blk := range bp.localBlks {
	// 	blk.Lock()
	// 	releaseBufferForBlock(blk)
	// 	blk.Unlock()
	// }
}

func (bp *BufferPool) getBufferForBlock(blk *block) {
	switch blk.state {
	case localBlock:
		// This block is a hole
		blk.buf = bPool.getBuffer(true)
	case committedBlock:
		blk.buf = bPool.getBuffer(false)
	case uncommitedBlock:
		//Todo: Block is evicted from the cache, to retrieve it, first we should do putBlockList
		panic("Todo: Read of evicted Uncommited block")
	}
}

// Returns the buffer.
// parameters: valid (Represents the data present in buffer is valid/not)
func (bp *BufferPool) getBuffer(valid bool) *Buffer {
	b := bp.pool.Get().(*Buffer)
	if b.data == nil {
		b.data = make([]byte, BlockSize)
	} else {
		copy(b.data, zeroBuffer.data)
	}
	//b.data = b.data[:0]
	b.valid = valid
	b.checksum = nil
	return b
}

func (bp *BufferPool) putBuffer(blk *block) {
	if blk.buf != nil {
		bp.pool.Put(blk.buf)
		blk.buf = nil
	}
}

// ************************************************************************
// The following are the functions describes on the buffer
// **************************************************************

func getBlockWithReadAhead(idx int, start int, file *File) (*block, error) {
	for i := 1; i <= 3; i++ {
		if i+start < (int(atomic.LoadInt64(&file.size))+BlockSize-1)/BlockSize {
			getBlockForRead(i+start, file, asyncRequest)
		}
	}
	blk, err := getBlockForRead(idx, file, syncRequest)
	return blk, err
}

// Returns the buffer containing block.
// This call only successed if the block idx < len(blocklist)
func getBlockForRead(idx int, file *File, r requestType) (blk *block, err error) {

	file.Lock()
	if idx >= len(file.blockList) {
		file.Unlock()
		return nil, errors.New("block is out of the blocklist scope")
	}
	blk = file.blockList[idx]
	file.Unlock()
	_, err = downloader(blk, r)
	return blk, err
}

// This call will return buffer for writing for the block
// This call should always return some buffer if len(blocklist) <= 50000
func getBlockForWrite(idx int, file *File) (*block, error) {
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
				//No need to lock the block as these are newly created blocks
				bPool.getBufferForBlock(blk)
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
	blkState, err := downloader(blk, syncRequest)
	if blkState == committedBlock || blkState == uncommitedBlock {
		bPool.localBlks <- blk
	}
	return blk, err
}

// Write all the Modified buffers to Azure Storage and return whether while is modified or not.
func syncBuffersForFile(file *File) (bool, error) {
	var err error = nil
	var fileChanged bool = false

	file.Lock()
	for _, blk := range file.blockList {
		// To make the sync upload faster, first we schedule all the requests as async
		// Then status would be checked using sync requests.
		blkstate, _ := uploader(blk, asyncRequest)
		if blkstate != committedBlock {
			fileChanged = true
		}
	}
	for _, blk := range file.blockList {
		_, err := uploader(blk, syncRequest)
		if err != nil {
			panic(fmt.Sprintf("Upload doesn't happen for the block idx : %d, file : %s\n", blk.idx, blk.file.Name))
		}
	}
	file.Unlock()
	return fileChanged, err
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

func commitBuffersForFile(file *File) error {
	logy.Write([]byte(fmt.Sprintf("BlockCache::commitFile : %s\n", file.Name)))
	var blklist []string
	file.Lock()
	defer file.Unlock()
	if file.blkListState == blockListInvalid || file.blkListState == blockListNotRetrieved {
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
		for _, blk := range file.blockList {
			blk.Lock()
			blk.state = committedBlock
			blk.Unlock()
		}
	}
	return err
}

// Release all the buffers to the file if this handle is the last one opened on the file.
func releaseBuffers(f *File) {
	//Lock was already acquired on file
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
