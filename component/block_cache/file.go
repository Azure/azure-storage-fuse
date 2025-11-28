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
	mu        sync.RWMutex
	Name      string                         // File Name
	size      int64                          // File Size
	Etag      string                         // Etag of the file
	handles   map[*handlemap.Handle]struct{} // Open file handles for this file
	blockList *blockList                     //  These blocks inside blocklist is used for files which can both read and write.
	synced    bool                           // Is file synced with Azure storage?

	// Number of pending read operations
	numPendingReads atomic.Int32

	// Store any error occurred during file operations
	// If we encounter any write error, we set this error and return it for subsequent operations.
	err atomic.Value
	//
	// To wait for pending writes to complete during flushing the file to the storage.
	pendingWriters sync.WaitGroup
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

		f.mu.RLock()
		if blockIdx < len(f.blockList.list) {
			blk = f.blockList.list[blockIdx]
		}
		f.mu.RUnlock()

		if blk == nil {
			log.Err("File::read: Block not found for file %s blockIdx %d", f.Name, blockIdx)
			return 0, io.EOF
		}

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			true, /*download*/
			true, /*sync*/
		)
		if err != nil {
			log.Err("File::read: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
			return 0, err
		}

		log.Debug("File::read: Got buffer descriptor for file: %s, blockIdx: %d, status: %v, numParallelReaders: %d, took: %v",
			f.Name, blockIdx, status, f.numPendingReads.Load(), time.Since(stime))

		// Copy data from block buffer to user buffer
		bufDesc.contentLock.RLock()
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset)
		blockLen := getBlockSize(fileSize, blockIdx)
		n := copy(options.Data[bufOffset:], bufDesc.buf[offsetInsideBlock:blockLen])
		bufDesc.contentLock.RUnlock()

		if bufDesc.usageCount.Add(int32(n)) == int32(bc.blockSize) {
			// Remove this buffer from cache as it is fully read
			if ok, _ := btm.removeBufferDescriptor(bufDesc, false /*strict*/); ok {
				log.Debug("File::read: Removed bufferIdx: %d for blockIdx: %d from buffer table manager after full read at file: %s, offset: %d",
					bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
			}
		}

		log.Debug("File::read: Read %d bytes from file: %s, blockIdx: %d, refCnt: %d, usageCnt: %d, numParallelReaders: %d, took: %v",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load(), f.numPendingReads.Load(), time.Since(stime))

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

		bufDesc, status, _ := GetOrCreateBufferDescriptor(blk,
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
		// var blkAlreadyExists bool = true

		f.mu.Lock()
		// Increment write wait group to track pending writes, This must be done under lock as flush would block the
		// upcoming writers when it acquires the lock. The call to f.writeWG.Done() is done in the caller after the
		// write is completed even if there is an error.
		f.pendingWriters.Add(1)

		if blockIdx < len(f.blockList.list) {
			blk = f.blockList.list[blockIdx]
		} else {
			// Need to create new block
			blockListLen := len(f.blockList.list)
			for i := blockListLen; i <= blockIdx; i++ {
				blk = createBlock(i, common.GetBlockID(common.BlockIDLength), localBlock, f)
				f.blockList.list = append(f.blockList.list, blk)
				// blkAlreadyExists = false
			}

		}
		f.mu.Unlock()

		bufDesc, status, err := GetOrCreateBufferDescriptor(blk,
			false, /*doesRead*/
			true,  /*sync*/
		)
		if err != nil {
			f.pendingWriters.Done() // Decrement the write wait group on error
			log.Err("File::write: Failed to get buffer descriptor for file: %s, blockIdx: %d, [%v]", f.Name, blockIdx, err)
			return err
		}

		log.Debug("File::write: Got buffer descriptor for file: %s, blockIdx: %d, status: %v", f.Name, blockIdx, status)

		// Copy data from user buffer to block buffer
		bufDesc.contentLock.Lock()
		// Change the block state to localBlock as it is being modified
		atomic.StoreInt32((*int32)(&blk.state), int32(localBlock))
		bufDesc.dirty.Store(true)
		offsetInsideBlock := convertOffsetIntoBlockOffset(offset)
		n := copy(bufDesc.buf[offsetInsideBlock:bc.blockSize], options.Data[bufOffset:])
		bufDesc.contentLock.Unlock()

		bufDesc.usageCount.Add(int32(n))

		log.Debug("File::write: Wrote %d bytes to file: %s, blockIdx: %d, refCnt: %d, usageCnt: %d",
			n, f.Name, blockIdx, bufDesc.refCnt.Load(), bufDesc.usageCount.Load())

		if bufDesc.usageCount.Load() == int32(bc.blockSize) && bufDesc.uploadInProgress.CompareAndSwap(false, true) {
			blk.scheduleUpload(bufDesc, false /*sync*/)
		}

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::write: Released bufferIdx: %d for blockIdx: %d back to free list after write at file: %s, offset: %d",
				bufDesc.bufIdx, blk.idx, f.Name, options.Offset)
		}

		offset += int64(n)
		bufOffset += n

		// Update file size if needed
		f.updateFileSize(offset)

		// Decrement the write wait group after write is completed
		f.pendingWriters.Done()
	}

	return nil
}

func (f *File) flush(options *internal.FlushFileOptions, takeFileLock bool) error {
	log.Debug("File::flush: Flushing file: %s", f.Name)

	if takeFileLock {
		// Take an exclusive lock on file to prevent further writes during flush.
		f.mu.Lock()
		defer f.mu.Unlock()

		log.Debug("File::flush: Acquired exclusive lock for flush on file: %s", f.Name)
	}

	if f.blockList.state != blockListValid {
		return nil
	}

	// Wait for all pending writes to complete inorder to have the clean state of the file.
	// We dont allow the new writers to proceed as we have the exclusive lock on file.
	f.pendingWriters.Wait()

	// Schedule upload for all dirty blocks
	for _, blk := range f.blockList.list {
		if blk.state == committedBlock || blk.state == uncommitedBlock {
			// No need to upload committed or uncommitted blocks
			continue
		}

		bufDesc, _ := btm.LookUpBufferDescriptor(blk)
		if bufDesc == nil {
			// No buffer descriptor found for this block, so nothing to upload
			// Check if block is in local state, which is unexpected
			if blk.state == localBlock {
				panic(fmt.Sprintf("File::flush: No buffer descriptor found for local blockIdx: %d during flush at file: %s",
					blk.idx, f.Name))
			}
			continue
		}

		// If there is any upload scheduled for this buffer, wait for it to complete, this content lock is taken
		// exclusively during upload.
		bufDesc.contentLock.Lock()
		bufDesc.contentLock.Unlock()

		if bufDesc.dirty.Load() && bufDesc.uploadErr == nil {
			log.Debug("File::flush: Scheduling upload for bufferIdx: %d, blockIdx: %d during flush, usageCnt: %d",
				bufDesc.bufIdx, blk.idx, bufDesc.usageCount.Load())

			if !bufDesc.uploadInProgress.CompareAndSwap(false, true) {
				// There is already an upload in progress for this buffer, this should not happen, as there are no
				// writers present to schedule async upload.
				panic(fmt.Sprintf("File::flush: Inconsistent state for bufferIdx: %d, blockIdx: %d during flush at file: %s, uploadInProgress: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadInProgress.Load()))
			}

			blk.scheduleUpload(bufDesc, true /*sync*/)

			if bufDesc.uploadErr != nil {
				log.Err("File::flush: Upload error for bufferIdx: %d, blockIdx: %d during flush at file: %s: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadErr)
				bufDesc.contentLock.Unlock()

				return bufDesc.uploadErr
			}

			log.Debug("File::flush: Successfully uploaded bufferIdx: %d, blockIdx: %d during flush at file: %s",
				bufDesc.bufIdx, blk.idx, f.Name)

		} else {
			if bufDesc.uploadErr != nil {
				log.Err("File::flush: Previous upload error for bufferIdx: %d, blockIdx: %d during flush at file: %s: %v",
					bufDesc.bufIdx, blk.idx, f.Name, bufDesc.uploadErr)

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

		// Release the buffer descriptor
		if ok := bufDesc.release(); ok {
			log.Debug("File::flush: Released bufferIdx: %d for blockIdx: %d back to free list after flush at file: %s",
				bufDesc.bufIdx, blk.idx, f.Name)
		}
	}

	// Do PutBlockList to commit all the blocks.
	blockList := make([]string, 0, len(f.blockList.list))
	for _, blk := range f.blockList.list {
		blockList = append(blockList, blk.id)
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
		log.Debug("File::flush: Successfully committed block list for file: %s", f.Name)
		f.synced = true
	}

	// update the block states.
	for _, blk := range f.blockList.list {
		blk.state = committedBlock
	}

	return nil
}

func (f *File) truncate(options *internal.TruncateFileOptions) error {
	log.Debug("File::truncate: Truncating file: %s to size: %d", f.Name, options.NewSize)
	f.mu.Lock()
	defer f.mu.Unlock()

	log.Debug("File::truncate: Acquired exclusive lock for truncate on file: %s", f.Name)

	// check error state
	if f.err.Load() != nil {
		return fmt.Errorf("previous write error: %v", f.err.Load())
	}

	if options.NewSize == atomic.LoadInt64(&f.size) {
		// No need to truncate
		log.Debug("File::truncate: No truncation needed for file: %s, size is already: %d", f.Name, options.NewSize)
		return nil
	}

	// Flush the file before truncating
	log.Debug("File::truncate: Flushing file: %s before truncation", f.Name)
	if err := f.flush(&internal.FlushFileOptions{}, false /*takeFileLock*/); err != nil {
		return err
	}

	// Update the file size
	atomic.StoreInt64(&f.size, options.NewSize)

	noOfBlocks := getNoOfBlocksInFile(options.NewSize)

	if noOfBlocks <= len(f.blockList.list) {
		// Shrink the block list
		f.blockList.list = f.blockList.list[:noOfBlocks]
	}

	// change the state of the last block to localBlock
	if len(f.blockList.list) > 0 {
		// make the last block as local block.

		lastBlock := f.blockList.list[len(f.blockList.list)-1]

		atomic.StoreInt32((*int32)(&lastBlock.state), int32(localBlock))
		bufDesc, status, err := GetOrCreateBufferDescriptor(lastBlock,
			true, /*download*/
			true, /*sync*/
		)
		if err != nil {
			log.Err("File::truncate: Failed to get buffer descriptor for last blockIdx: %d during truncate at file: %s, [%v]",
				lastBlock.idx, f.Name, err)
			return err
		}

		bufDesc.dirty.Store(true)

		log.Debug("File::truncate: Got buffer descriptor for last blockIdx: %d during truncate at file: %s, status: %v",
			lastBlock.idx, f.Name, status)

		defer func() {
			// Release the buffer descriptor
			if ok := bufDesc.release(); ok {
				log.Debug("File::truncate: Released bufferIdx: %d for last blockIdx: %d back to free list after truncate at file: %s",
					bufDesc.bufIdx, lastBlock.idx, f.Name)
			}
		}()

		log.Debug("File::truncate: Shrink block list for file: %s to noOfBlocks: %d", f.Name, noOfBlocks)
	}

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
	if err := f.flush(&internal.FlushFileOptions{}, false /*takeFileLock*/); err != nil {
		return err
	}

	return nil
}
