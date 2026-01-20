package block_cache

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Note: There is a reason why we are storing the references to open handles inside a file rather
// maintaing a counter, because to support deferring the removal of files when some open handles are present.
// At that time we dont want to iterate over entire open handle map to change some fields
type File struct {
	mu            sync.RWMutex
	Name          string                         // File Name
	sizeOnStorage int64                          // File Size on the Azure storage
	size          int64                          // File Size
	Etag          string                         // Etag of the file
	handles       map[*handlemap.Handle]struct{} // Open file handles for this file
	blockList     *blockList                     //  These blocks inside blocklist is used for files which can both read and write.
	synced        bool                           // Is file synced with Azure storage?

	// Number of pending read operations
	numPendingReads atomic.Int32

	// Store any error occurred during file operations
	// If we encounter any write error, we set this error and return it for subsequent operations.
	err atomic.Value
	//
	// To wait for pending writes to complete during flushing the file to the storage.
	pendingWriters sync.WaitGroup
	//
	// If file is small enough to fit in a single block, then we can optimize the flush to do putBlob instead of
	// putBlock & putBlockList. This boolean indicates whether the file is flushed using putBlob previously.
	singleBlockFilePersisted bool
}

func createFile(fileName string) *File {
	f := &File{
		Name:          fileName,
		handles:       make(map[*handlemap.Handle]struct{}),
		blockList:     newBlockList(),
		size:          -1,
		sizeOnStorage: -1,
		synced:        true,
	}

	return f
}

func (f *File) updateFileSize(size int64) {
	for {
		currentSize := atomic.LoadInt64(&f.size)

		if size <= currentSize {
			break
		}
		if atomic.CompareAndSwapInt64(&f.size, currentSize, size) {
			break
		}
	}
}

func (f *File) read(options *internal.ReadInBufferOptions) (int, error) {
	f.numPendingReads.Add(1)
	defer f.numPendingReads.Add(-1)

	stime := time.Now()

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
	retry:

		f.mu.RLock()
		if blockIdx < len(f.blockList.list) {
			blk = f.blockList.list[blockIdx]
		}
		f.mu.RUnlock()

		if blk == nil {
			log.Err("File::read: Block not found for file %s blockIdx %d", f.Name, blockIdx)
			// TODO: is this the right error to return? or EIO is better?
			return 0, io.EOF
		}

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			true, /*sync*/
		)
		if err != nil {
			log.Err("File::read: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
			return 0, err
		}

		if status == bufDescStatusNeedsFileFlush {
			// The block is in uncommited state, need to flush the file first before reading.
			log.Debug("File::read: Block in uncommited state, flushing file: %s before read, blockIdx: %d", f.Name, blockIdx)

			if err := f.flush(true /*takeFileLock*/); err != nil {
				log.Err("File::read: Failed to flush file: %s before read, blockIdx: %d: %v", f.Name, blockIdx, err)
				return 0, err
			}

			// Retry getting the block descriptor after flush
			goto retry
		}

		log.Debug("File::read: Got buffer descriptor for file: %s, blockIdx: %d, status: %v, numParallelReaders: %d, took: %v",
			f.Name, blockIdx, status, f.numPendingReads.Load(), time.Since(stime))

		// Copy data from block buffer to user buffer
		bufDesc.contentLock.RLock()
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset)
		blockLen := getBlockSize(fileSize, blockIdx)
		n := copy(options.Data[bufOffset:], bufDesc.buf[offsetInsideBlock:blockLen])
		bufDesc.contentLock.RUnlock()

		if bufDesc.bytesRead.Add(int32(n)) >= int32(bc.blockSize) {
			// Remove this buffer from table as it is fully read.
			if ok, _ := btm.removeBufferDescriptor(bufDesc, true /*strict*/); ok {
				log.Debug("File::read: Removed bufferIdx: %d for blockIdx: %d from buffer table manager after full read at file: %s, offset: %d",
					bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
			}
		}

		log.Debug("File::read: Read %d bytes from file: %s, blockIdx: %d, refCnt: %d, bytesRead: %d, numParallelReaders: %d, took: %v",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.bytesRead.Load(), f.numPendingReads.Load(), time.Since(stime))

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::read: Released bufferIdx: %d for blockIdx: %d back to free list after read at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		bytesRead += n
		bufOffset += n
		offset += int64(n)
	}

	log.Debug("File::read: Completed read of %d bytes from file: %s, offset: %d, took: %v",
		bytesRead, f.Name, options.Offset, time.Since(stime))

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

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			false, /* sync */
		)
		if err != nil {
			log.Err("File::scheduleReadAhead: Failed to get buffer descriptor for file: %s, blockIdx: %d during read-ahead, [%v]",
				f.Name, blk.idx, err)
			return
		}

		if bufDesc != nil {
			// Release the buffer descriptor as we dont need it
			if ok := bufDesc.release(); ok {
				log.Debug("File::scheduleReadAhead: Released bufferIdx: %d for blockIdx: %d back to free list after read-ahead at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
		}

		if status == bufDescStatusExists {
			log.Debug("File::scheduleReadAhead: Block already in cache, wrong read-ahead scheduled for file: %s, blockIdx: %d, patter: %v, status: %v",
				f.Name, blk.idx, patterntype, status)

		} else {
			// We have scheduled read-ahead for this block
			log.Debug("File::scheduleReadAhead: Scheduled read-ahead for file: %s, blockIdx: %d, patter: %v, status: %v",
				f.Name, blk.idx, patterntype, status)
		}
	}
}

// write: writes data to the file at the given offset.
// This should always write len(options.Data) bytes, otherwise it must return an error.
func (f *File) write(options *internal.WriteFileOptions) error {

	offset := options.Offset
	endOffset := options.Offset + int64(len(options.Data))
	bufOffset := 0

	if endOffset > int64(bc.maxFileSize) {
		log.Err("File::write: Write exceeds maximum file size for file %s, offset %d, data length %d",
			f.Name, options.Offset, len(options.Data))
		return fmt.Errorf("write exceeds maximum file size")
	}

	// If there was any previous write error, return that error, this will safely prevent further writes to the file.
	if f.err.Load() != nil {
		return fmt.Errorf("previous write error: %v", f.err.Load())
	}

	for offset < endOffset {
		blockIdx := getBlockIndex(offset)
		var blk *block
	retry:

		f.mu.Lock()
		// Increment write wait group to track pending writes, This must be done under lock as flushing the file would
		// block the upcoming writers when it acquires the lock. The call to f.pendingWriters.Done() must be called
		// after the write is completed even if there is an error, otherwise flush will wait indefinitely.
		f.pendingWriters.Add(1)

		blockListLen := len(f.blockList.list)

		if blockIdx < blockListLen {
			blk = f.blockList.list[blockIdx]
		} else {
			// Need to create new block
			for i := blockListLen; i <= blockIdx; i++ {
				blk = createBlock(i, common.GetBlockID(common.BlockIDLength), localBlock, f)
				f.blockList.list = append(f.blockList.list, blk)
				log.Debug("File::write: Created new blockIdx: %d for file: %s during write at offset: %d",
					blk.idx, f.Name, options.Offset)
			}
		}
		f.synced = false
		f.mu.Unlock()

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			true, /*sync*/
		)
		if err != nil {
			// Decrement the write wait group on error
			f.pendingWriters.Done()
			log.Err("File::write: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
			return err
		}

		if status == bufDescStatusNeedsFileFlush {
			// The block is in uncommited state, need to flush the file first before writing.
			log.Debug("File::write: Block in uncommited state, flushing file: %s before write, blockIdx: %d", f.Name, blockIdx)
			// Decrement the write wait group before flushing, as flush will wait for all pending writers to complete.
			f.pendingWriters.Done()

			if err := f.flush(true /*takeFileLock*/); err != nil {
				log.Err("File::write: Failed to flush file: %s before write, blockIdx: %d: %v", f.Name, blockIdx, err)
				return err
			}
			// Retry gettting the block descriptor after flush
			goto retry
		}

		log.Debug("File::write: Got buffer descriptor for file: %s, blockIdx: %d, status: %v", f.Name, blockIdx, status)
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset)

		// Take the exclusive lock on buffer content to write data
		bufDesc.contentLock.Lock()

		// Change the block state to localBlock as it is being modified
		atomic.StoreInt32((*int32)(&blk.state), int32(localBlock))
		blk.numWrites.Add(1)
		bufDesc.dirty.Store(true)

		// Copy data from user buffer to block buffer
		n := copy(bufDesc.buf[offsetInsideBlock:bc.blockSize], options.Data[bufOffset:])

		bufDesc.bytesWritten.Add(int32(n))

		offset += int64(n)
		bufOffset += n

		// Update file size if needed
		f.updateFileSize(offset /* newFileSize */)

		//
		// Schedule upload if buffer is fully written and no other references
		uploadScheduled := false
		if bufDesc.bytesWritten.Load() >= int32(bc.blockSize) && bufDesc.refCnt.Load() == 1 {
			blk.scheduleUpload(bufDesc, false /*sync*/)
			uploadScheduled = true
		}
		//
		// Unlock the buffer content lock after write, if upload is scheduled, the lock will be
		// released after upload is complete in the different goroutine, else we can release it now.
		if !uploadScheduled {
			bufDesc.contentLock.Unlock()
		}

		log.Debug("File::write: Wrote %d bytes to file: %s, size: %d, blockIdx: %d, refCnt: %d, usageCnt: %d, uploadScheduled: %v",
			n, f.Name, f.size, blockIdx, bufDesc.refCnt.Load(), bufDesc.bytesWritten.Load(), uploadScheduled)

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::write: Released bufferIdx: %d for blockIdx: %d back to free list after write at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		// Decrement the write wait group after write is completed
		f.pendingWriters.Done()
	}

	return nil
}

func (f *File) flush(takeFileLock bool) error {
	log.Debug("File::flush: Flushing file: %s, takeFileLock: %v", f.Name, takeFileLock)

	if takeFileLock {
		// Take an exclusive lock on file to prevent further writes during flush.
		f.mu.Lock()
		defer f.mu.Unlock()

		log.Debug("File::flush: Acquired exclusive lock for flush on file: %s", f.Name)
	}

	log.Debug("File::flush: Flushing file: %s, size: %d, takeFileLock: %v", f.Name, f.size, takeFileLock)

	if f.blockList.state != blockListValid {
		return nil
	}

	if f.synced == true {
		log.Debug("File::flush: File: %s is already synced, no flush needed", f.Name)
		return nil
	}

	if f.err.Load() != nil {
		log.Err("File::flush: Previous write error found for file: %s, error: %v", f.Name, f.err.Load())
		return fmt.Errorf("previous write error: %v", f.err.Load())
	}

	// Wait for all pending writes to complete inorder to have the clean state of the file.
	// We dont allow the new writers to proceed as we have the exclusive lock on file.
	f.pendingWriters.Wait()

	zeroBlockId := common.GetBlockID(common.BlockIDLength)
	isZeroBlockUploaded := false
	uploadZeroBlock := func(blk *block, isLastBlock bool) error {
		blk.id = zeroBlockId
		if isZeroBlockUploaded && !isLastBlock {
			// Zero block is already uploaded, reuse the block ID
			return nil
		}
		offsetInsideBlock := int64(bc.blockSize)

		if isLastBlock {
			offsetInsideBlock = convertOffsetIntoBlockOffset(f.size - 1)
			offsetInsideBlock++
			blk.id = common.GetBlockID(common.BlockIDLength)
		}
		log.Debug("File::flush: Uploading zero block for blockIdx: %d during flush at file: %s, zeroBlockId: %s, bytesUploading: %d",
			blk.idx, f.Name, blk.id, offsetInsideBlock)

		err := bc.NextComponent().StageData(internal.StageDataOptions{
			Name: f.Name,
			Data: freeList.bufPool.zeroBuf[:offsetInsideBlock],
			Id:   blk.id,
		})
		if err == nil && offsetInsideBlock == int64(bc.blockSize) {
			isZeroBlockUploaded = true
		}

		return err
	}

	blockListLen := len(f.blockList.list)

	// If the file is expanded by write, the last block may got sparse, may need to extend it.
	if blockListLen > 0 && f.size > f.sizeOnStorage && f.sizeOnStorage%int64(bc.blockSize) != 0 {
		// reupload the block that was partially filled earlier to extend it with zeros.
		lastBlockIdx := getBlockIndex(f.sizeOnStorage - 1)
		lastBlock := f.blockList.list[lastBlockIdx]
		if lastBlock.state == committedBlock && lastBlock.numWrites.Load() == 0 {
			// Last block is committed and no writes on it, need to extend it with zeros by making it dirty.
			log.Debug("File::flush: Extending last blockIdx: %d for file: %s during flush to accommodate file size expansion",
				lastBlock.idx, f.Name)
			bufDesc, _, err := GetOrCreateBufferDescriptor(lastBlock,
				true, /*sync*/
			)
			if err != nil {
				log.Err("File::flush: Failed to get buffer descriptor for last blockIdx: %d during flush at file: %s, [%v]",
					lastBlock.idx, f.Name, err)
				return err
			}

			atomic.StoreInt32((*int32)(&lastBlock.state), int32(localBlock))
			bufDesc.dirty.Store(true)
			// Release the buffer descriptor
			if ok := bufDesc.release(); ok {
				panic(fmt.Sprintf("File::flush: Released bufferIdx: %d for last blockIdx: %d back to free list after flush at file: %s",
					bufDesc.bufIdx, lastBlock.idx, f.Name))
			}
		}

	}

	// Schedule upload for all dirty blocks
	for i, blk := range f.blockList.list {
		if blk.state == committedBlock || blk.state == uncommitedBlock {
			// No need to upload committed or uncommitted blocks
			continue
		}

		bufDesc, _ := btm.LookUpBufferDescriptor(blk)
		if bufDesc == nil {
			// No buffer descriptor found for this block, sparse blocks must have no writes on it.
			if blk.state == localBlock && blk.numWrites.Load() > 0 {
				panic(fmt.Sprintf("File::flush: No buffer descriptor found for local blockIdx: %d during flush at file: %s",
					blk.idx, f.Name))
			}

			// This is a sparse block which is not modified. Hence no buffer descriptor is present. Upload zero block if
			// needed.
			err := uploadZeroBlock(blk, i == blockListLen-1 /*isLastBlock*/)
			if err != nil {
				log.Err("File::flush: Failed to upload zero block for sparse blockIdx: %d during flush at file: %s: %v",
					blk.idx, f.Name, err)
				f.err.Store(err.Error())
				return err
			}
			continue
		}

		// Release the buffer descriptor
		releaseBufDesc := func() {
			if ok := bufDesc.release(); ok {
				log.Debug("File::flush: Released bufferIdx: %d for blockIdx: %d back to free list after flush at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
		}

		// If there is any upload scheduled for this buffer, wait for it to complete, this content lock is taken
		// exclusively during upload.
		bufDesc.contentLock.Lock()
		bufDesc.contentLock.Unlock()

		if bufDesc.dirty.Load() && bufDesc.uploadErr == nil {
			log.Debug("File::flush: Scheduling upload for bufferIdx: %d, blockIdx: %d during flush, bytesRead: %d, bytesWritten: %d at file: %s",
				bufDesc.bufIdx, blk.idx, bufDesc.bytesRead.Load(), bufDesc.bytesWritten.Load(), f.Name)

			blk.scheduleUpload(bufDesc, true /*sync*/)

			if bufDesc.uploadErr != nil {
				log.Err("File::flush: Upload error for bufferIdx: %d, blockIdx: %d during flush at file: %s: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadErr)
				releaseBufDesc()

				return bufDesc.uploadErr
			}

			log.Debug("File::flush: Successfully uploaded bufferIdx: %d, blockIdx: %d during flush at file: %s",
				bufDesc.bufIdx, blk.idx, f.Name)

		} else {
			if bufDesc.uploadErr != nil {
				log.Err("File::flush: Previous upload error for bufferIdx: %d, blockIdx: %d during flush at file: %s: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadErr)
				releaseBufDesc()

				return bufDesc.uploadErr
			} else if blk.state == localBlock {
				// not expected as error must be set when dirty is false
				panic(fmt.Sprintf("File::flush: Inconsistent state for bufferIdx: %d, blockIdx: %d during flush at file: %s, dirty: %v, uploadErr: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.dirty.Load(), bufDesc.uploadErr))
			} else {
				log.Debug("File::flush: No upload needed for bufferIdx: %d, blockIdx: %d during flush at file: %s",
					bufDesc.bufIdx, blk.idx, f.Name)
			}
		}
		releaseBufDesc()
	}

	// Do PutBlockList to commit all the blocks.
	blockList := make([]string, 0, len(f.blockList.list))
	for _, blk := range f.blockList.list {
		blockList = append(blockList, blk.id)
	}
	log.Debug("File::flush: Committing block list for file: %s, number of blocks: %d, blockList: %v", f.Name, len(blockList), blockList)

	if len(blockList) == 0 {
		// Need to create an empty file in the storage
		log.Debug("File::flush: Creating empty file in storage for file: %s", f.Name)
		_, err := bc.NextComponent().CreateFile(internal.CreateFileOptions{
			Name: f.Name,
		})
		if err != nil {
			log.Err("File::flush: Failed to create empty file in storage for file: %s: %v", f.Name, err)
			f.err.Store(err.Error())
			return err
		}
		log.Debug("File::flush: Successfully created empty file in storage for file: %s", f.Name)
		f.synced = true
		return nil
	}

	err := bc.NextComponent().CommitData(internal.CommitDataOptions{
		Name:      f.Name,
		List:      blockList,
		BlockSize: bc.blockSize,
	})
	if err != nil {
		log.Err("File::flush: Failed to commit block list for file: %s: %v", f.Name, err)
		f.err.Store(err.Error())
		return err
	} else {
		log.Debug("File::flush: Successfully committed block list for file: %s, size: %d", f.Name, f.size)
		f.synced = true
	}

	// update the block states.
	for _, blk := range f.blockList.list {
		blk.state = committedBlock
	}

	f.sizeOnStorage = f.size

	return nil
}

func (f *File) truncate(options *internal.TruncateFileOptions) error {
	log.Debug("File::truncate: Truncating file: %s to size: %d", f.Name, options.NewSize)
	f.mu.Lock()
	defer f.mu.Unlock()

	log.Debug("File::truncate: Acquired exclusive lock for truncate on file: %s", f.Name)

	// check error state
	if f.err.Load() != nil {
		log.Err("File::truncate: Previous write error found for file: %s, error: %v", f.Name, f.err.Load())
		return fmt.Errorf("previous write error: %v", f.err.Load())
	}

	if options.NewSize == atomic.LoadInt64(&f.size) {
		// No need to truncate
		log.Debug("File::truncate: No truncation needed for file: %s, size is already: %d", f.Name, options.NewSize)
		return nil
	}

	// Flush the file before truncating
	log.Debug("File::truncate: Flushing file: %s before truncation", f.Name)
	if err := f.flush(false /*takeFileLock*/); err != nil {
		return err
	}

	// Update the file size
	isFileShrinking := f.size > options.NewSize
	atomic.StoreInt64(&f.size, options.NewSize)
	f.synced = false

	noOfBlocks := getNoOfBlocksInFile(options.NewSize)

	if noOfBlocks < len(f.blockList.list) {
		// Shrink the block list, give back the buffers shrinked to free list.
		for i := noOfBlocks; i < len(f.blockList.list); i++ {
			blk := f.blockList.list[i]
			bufDesc, _ := btm.LookUpBufferDescriptor(blk)
			if bufDesc != nil {
				// Release the buffer descriptor
				if ok := bufDesc.release(); ok {
					panic(fmt.Sprintf("File::truncate: Released bufferIdx: %d for blockIdx: %d back to free list during truncate at file: %s",
						bufDesc.bufIdx, blk.idx, f.Name))
				}
				// Remove this buffer from buffer table manager
				if ok1, ok2 := btm.removeBufferDescriptor(bufDesc, false /*strict*/); !ok1 {
					panic(fmt.Sprintf("File::truncate: Removed buffer: %v for blockIdx: %d from buffer table manager during truncate at file: %s, isRemovedFromBufMgr: %v, isReleasedToFreeList: %v, refCnt: %d",
						bufDesc, blk.idx, f.Name, ok1, ok2, bufDesc.refCnt.Load()))
				}
			}
		}
		f.blockList.list = f.blockList.list[:noOfBlocks]
	}

	// change the state of the last block to localBlock
	if len(f.blockList.list) > 0 {

		// make the last block as local block.
		lastBlock := f.blockList.list[len(f.blockList.list)-1]

		bufDesc, status, err := GetOrCreateBufferDescriptor(lastBlock,
			true, /*sync*/
		)
		if err != nil {
			log.Err("File::truncate: Failed to get buffer descriptor for last blockIdx: %d during truncate at file: %s, [%v]",
				lastBlock.idx, f.Name, err)
			return err
		}

		atomic.StoreInt32((*int32)(&lastBlock.state), int32(localBlock))
		bufDesc.dirty.Store(true)

		// Clean the rest of the buffer if file is getting shrinked as it may contain old/dirty data.
		if isFileShrinking {
			bufDesc.contentLock.Lock()
			offsetInsideBlock := convertOffsetIntoBlockOffset(f.size - 1)
			copy(bufDesc.buf[offsetInsideBlock+1:], freeList.bufPool.zeroBuf[:])
			bufDesc.contentLock.Unlock()
		}

		log.Debug("File::truncate: Got buffer descriptor for last blockIdx: %d during truncate at file: %s, status: %v",
			lastBlock.idx, f.Name, status)

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::truncate: Released bufferIdx: %d for last blockIdx: %d back to free list after truncate at file: %s",
				bufDesc.bufIdx, lastBlock.idx, f.Name)
		}
	}

	log.Debug("File::truncate: New block list for file: %s to noOfBlocks: %d", f.Name, noOfBlocks)

	if noOfBlocks > len(f.blockList.list) {
		// Expand the block blockList, create one localBlock for new blocks and duplicate it.
		blkId := common.GetBlockID(common.BlockIDLength)

		for i := len(f.blockList.list); i < noOfBlocks; i++ {
			blk := createBlock(i, blkId, localBlock, f)
			f.blockList.list = append(f.blockList.list, blk)
			log.Debug("File::truncate: Expanded block list for file: %s, added blockIdx: %d", f.Name, i)
		}
	}

	// Flush the file again to commit the truncation
	log.Debug("File::truncate: Flushing file: %s after truncation", f.Name)
	if err := f.flush(false /*takeFileLock*/); err != nil {
		return err
	}

	return nil
}
