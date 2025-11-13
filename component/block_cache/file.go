package block_cache

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Note: There is a reason why we are storing the references to open handles inside a file rather
// maintaing a counter, because to support deferring the removal of files when some open handles are present.
// At that time we dont want to iterate over entire open handle map to change some fields
type File struct {
	mu              sync.RWMutex
	Name            string                         // File Name
	size            int64                          // File Size
	Etag            string                         // Etag of the file
	handles         map[*handlemap.Handle]struct{} // Open file handles for this file
	blockList       *blockList                     //  These blocks inside blocklist is used for files which can both read and write.
	synced          bool                           // Is file synced with Azure storage?
	nxtReadAheadIdx atomic.Int32                   // Next block index to read ahead
}

func CreateFile(fileName string) *File {
	f := &File{
		Name:      fileName,
		handles:   make(map[*handlemap.Handle]struct{}),
		blockList: newBlockList(),
		size:      -1,
		synced:    true,
	}

	return f
}

func (f *File) read(options *internal.ReadInBufferOptions) (int, error) {
	fileSize := atomic.LoadInt64(&f.size)
	if options.Offset >= fileSize {
		return 0, io.EOF
	}

	offset := options.Offset
	endOffset := min(fileSize, options.Offset+int64(len(options.Data)))
	bufOffset := 0
	bytesRead := 0

	for offset < endOffset {
		blockIdx := getBlockIndex(offset)
		var blk *block

		f.mu.RLock()
		if blockIdx < len(f.blockList.list) {
			blk = f.blockList.list[blockIdx]
		}
		f.mu.RUnlock()

		if blk == nil {
			log.Err("File::read: Block not found for file %s blockIdx %d", f.Name, blockIdx)
			return 0, io.EOF
		}

		// Schedule read-ahead if applicable
		f.scheduleReadAhead(int32(blockIdx))

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk, true, true)
		if err != nil {
			log.Err("File::read: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
			return 0, err
		}

		log.Debug("File::read: Got buffer descriptor for file: %s, blockIdx: %d, status: %v", f.Name, blockIdx, status)

		// Copy data from block buffer to user buffer
		bufDesc.contentLock.RLock()
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset)
		blockLen := getBlockSize(fileSize, blockIdx)
		n := copy(options.Data[bufOffset:], bufDesc.buf[offsetInsideBlock:blockLen])
		bufDesc.contentLock.RUnlock()

		// Release the buffer descriptor
		bufDesc.release()

		log.Debug("File::read: Read %d bytes from file %s blockIdx %d, refCnt: %d, usageCnt: %d",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load())

		bytesRead += n
		bufOffset += n
		offset += int64(n)
	}

	return bytesRead, nil
}

func (f *File) scheduleReadAhead(currentBlockIdx int32) {
	for {
		f.mu.Lock()
		nxtReadAheadIdx := f.nxtReadAheadIdx.Load()
		if currentBlockIdx+int32(bc.prefetch) < nxtReadAheadIdx {
			f.mu.Unlock()
			return
		}

		if nxtReadAheadIdx >= int32(len(f.blockList.list)) {
			f.mu.Unlock()
			break
		}
		blk := f.blockList.list[nxtReadAheadIdx]
		f.nxtReadAheadIdx.Store(nxtReadAheadIdx + 1)
		f.mu.Unlock()

		bufDesc, status, _ := GetOrCreateBufferDescriptor(blk, true, false)
		if status == bufDescStatusExists {
			// Drop the ref-cnt as we don't intend to use it now
			bufDesc.release()
		}
	}
}

func (f *File) write(options *internal.WriteFileOptions) (int, error) {
	return 0, nil
}

func createFreshHandleForFile(name string, size int64, mtime time.Time, flags int) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	return handle
}

// Sync Map filepath->*File
var fileMap sync.Map

func getFileFromPath(key string) (*File, bool) {
	f := CreateFile(key)
	var first_open bool = false
	retryCnt := 0
retry:
	file, loaded := fileMap.LoadOrStore(key, f)
	if !loaded {
		first_open = true
	} else {
		f = file.(*File)
		// There should be atleast one open handle for this file if loaded is true.
		// Hypothetically it is possible when open of a file come before releasing all the previous handles.
		panic(fmt.Sprintf("getFileFromPath: File %s found in fileMap but has zero open handles", key))
		f.mu.Lock()
		if len(f.handles) == 0 {
			retryCnt++
			log.Err("getFileFromPath: File %s found in fileMap but has zero open handles, race with deleting the handles, retryCnt: %d",
				key, retryCnt)
		}
		f.mu.Unlock()

		if retryCnt > 100 {
			panic(fmt.Sprintf("getFileFromPath: Too many retries(%d) for file %s getting zero open handles, something is wrong",
				retryCnt, key))
			return nil, false
		}
		goto retry
	}

	return file.(*File), first_open
}

// Remove the handle from the file
// Release the buffers if the openFDcount is zero for the file
func deleteOpenHandleForFile(handle *handlemap.Handle) {
	file := handle.IFObj.(*File)
	file.mu.Lock()
	defer file.mu.Unlock()
	delete(file.handles, handle)
	if len(file.handles) == 0 {
		fileMap.Delete(file.Name)
		releaseAllBuffersForFile(file)
	}
}

func releaseAllBuffersForFile(file *File) {
	log.Debug("releaseAllBuffersForFile: Releasing all buffers for file %s", file.Name)
	// Release all buffers held by this file
	for _, blk := range file.blockList.list {
		bufDesc, _ := btm.LookUpBufferDescriptor(blk)
		if bufDesc == nil {
			continue
		}

		btm.removeBufferDescriptor(blk)

		// Now all the refereces to this buffer descriptor must be removed from buffer table, apart from this one.
		if bufDesc.refCnt.Load() != 1 {
			// print debug info
			panic(fmt.Sprintf("releaseAllBuffersForFile: bufferIdx: %d for blockIdx: %d of file %s still has ref count %d, usage count %d",
				bufDesc.bufIdx, blk.idx, file.Name, bufDesc.refCnt.Load(), bufDesc.usageCount.Load()))
		}

		log.Debug("releaseAllBuffersForFile: Released bufferIdx: %d for blockIdx: %d of file %s",
			bufDesc.bufIdx, blk.idx, file.Name)

		bufDesc.release()

		// Release the bufferDescriptor
		freeList.releaseBuffer(bufDesc)
	}

	// Clear the block list
	file.blockList = nil
}

func deleteFileIfNoOpenHandles(key string) {
	file, ok := checkFileExistsInOpen(key)
	if !ok {
		return
	}

	file.mu.Lock()
	defer file.mu.Unlock()

	if len(file.handles) == 0 {
		fileMap.Delete(file.Name)
		// TODO: Release the buffers held by this file
	}
}

func checkFileExistsInOpen(key string) (*File, bool) {
	f, ok := fileMap.Load(key)
	if ok {
		return f.(*File), ok
	}
	return nil, ok
}
