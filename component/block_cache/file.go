package block_cache

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Note: There is a reason why we are storing the references to open handles inside a file rather
// maintaing a counter, because to support deferring the removal of files when some open handles are present.
// At that time we dont want to iterate over entire open handle map to change some fields
type File struct {
	mu        sync.RWMutex
	Name      string                         // File Name
	size      int64                          // File Size
	Etag      string                         // Etag of the file
	handles   map[*handlemap.Handle]struct{} // Open file handles for this file
	blockList *blockList                     //  These blocks inside blocklist is used for files which can both read and write.
	synced    bool                           // Is file synced with Azure storage?
}

func createFile(fileName string) *File {
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

		bytesInterested := min(int32(len(options.Data)-bufOffset), int32(int64(bc.blockSize)-convertOffsetIntoBlockOffset(offset)))

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			bytesInterested, /*bytesInterested*/
			true,            /*download*/
			true,            /*sync*/
		)
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

		log.Debug("File::read: Read %d bytes from file: %s, blockIdx: %d, refCnt: %d, usageCnt: %d",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load())

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::read: Released bufferIdx: %d for blockIdx: %d back to free list after read at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		bytesRead += n
		bufOffset += n
		offset += int64(n)
	}

	return bytesRead, nil
}

func (f *File) scheduleReadAhead(pd *patternDetector, offset int64) {
	patterntype := pd.updateAccessPattern(offset)
	if patterntype != patternSequential {
		return
	}

	// Only schedule read-ahead for sequential access patterns
	numReadAheadBlocks := int(bc.prefetch)
	currentBlockIdx := getBlockIndex(offset)

	for range numReadAheadBlocks {
		nextBlockIdx := pd.nxtReadAheadBlockIdx.Add(1)
		nextBlockIdx--
		if currentBlockIdx+numReadAheadBlocks < int(nextBlockIdx) {
			// Exceeded the read-ahead limit
			pd.nxtReadAheadBlockIdx.Add(-1)
			return
		}

		var blk *block

		f.mu.RLock()
		if int(nextBlockIdx) < len(f.blockList.list) {
			blk = f.blockList.list[nextBlockIdx]
		}
		f.mu.RUnlock()

		if blk == nil {
			// No more blocks to read-ahead
			return
		}

		bufDesc, status, _ := GetOrCreateBufferDescriptor(blk,
			0,     /* bytesInterested */
			true,  /* download */
			false, /* sync */
		)
		if status == bufDescStatusExists {
			log.Debug("File::scheduleReadAhead: Block already in cache, wrong read-ahead scheduled for file: %s, blockIdx: %d, patter: %v, status: %v",
				f.Name, blk.idx, patterntype, status)

			// Release the buffer descriptor as we dont need it
			if ok := bufDesc.release(); ok {
				log.Debug("File::scheduleReadAhead: Released bufferIdx: %d for blockIdx: %d back to free list after read-ahead at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
		} else {
			// We have scheduled read-ahead for this block
			log.Debug("File::scheduleReadAhead: Scheduled read-ahead for file: %s, blockIdx: %d, patter: %v, status: %v",
				f.Name, blk.idx, patterntype, status)
		}
	}
}

func (f *File) write(options *internal.WriteFileOptions) (int, error) {
	return 0, nil
}
