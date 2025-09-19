package distributed_cache

import (
	"io"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

const (
	maxBackgroundCacheWarmupChunksPerFile = 32
	// TODO: Remove the following const. It is same as the staging area in dcache file.
	numStagingChunks = 256 // this is same as the staging area in dcache file
)

// Cache warmup logic: When a file is opened for read, we check if the file is already present in the dcache.
// If not, we create a dcache file and start a background go-routine to download the file from Azure in chunks
// and write it to the dcache file. The chunks are written in parallel. The read path checks if the chunk
// enclosing the read offset is successfully written to dcache(i.e, writeMV is success). If not, it waits for the
// chunk to be successfully written to dcache before reading the data from dcache file. The read path also
// maintains a bitmap to track which chunks have been successfully written to dcache. The reads for this handle
// can proceed in parallel with the background cache warmup. user application reads were server from the different
// dcache file while the cache warmup was in progress (i.e., from dcFile.CacheWarmup.warmDcFile).
func startCacheWarmup(dc *DistributedCache, handle *handlemap.Handle) {
	dcFile := handle.IFObj.(*fm.DcacheFile)

	if dcFile == nil {
		// no cache warmup setup for this file.
		return
	}

	log.Info("DistributedCache::startCacheWarmup: Starting cache warmup for file: %s, handle: %d, size: %d bytes",
		handle.Path, handle.ID, dcFile.CacheWarmup.Size)

	var err error

	// wait for status of last write to complete.
	waitForChunkWriteToComplete := func() (int64, error) {
		chunkStatus := <-dcFile.CacheWarmup.SuccessCh
		dcFile.CacheWarmup.ProcessedChunkWrites.Add(1)
		if chunkStatus.Err != nil {
			log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, error: %v",
				handle.Path, handle.ID, err)
			return chunkStatus.ChunkIdx, err
		} else {
			dcFile.CacheWarmup.SuccessfulChunkWrites.Add(1)
		}

		return chunkStatus.ChunkIdx, nil
	}

	// wait for first chunk in dcache write staging area to be successfully flushed out to dcache before proceeding.
	waitForChunkUploadToComplete := func(chunkIdx int64) error {
		for {
			// This error would be set by the file manager when the chunk failed to write to any of the MV.
			if Ierr := dcFile.CacheWarmup.Error.Load(); Ierr != nil {
				// cache warmup failed.
				err = Ierr.(error)
				return err
			}

			// check if corresponding bit is set in the bitmap.
			if ok := common.AtomicTestBitUint64(&dcFile.CacheWarmup.Bitmap[chunkIdx/64], uint(chunkIdx%64)); ok {
				break
			}

			log.Debug("DistributedCache::startCacheWarmup: Waiting for chunk upload completion for file: %s, handle: %d, chunk Idx: %d, SuccessfulChunks: %d, MaxChunks: %d",
				handle.Path, handle.ID, chunkIdx, dcFile.CacheWarmup.SuccessfulChunkWrites.Load(), dcFile.CacheWarmup.MaxChunks)

			time.Sleep(100 * time.Millisecond)
		}

		return nil
	}

	for i := int64(0); i < dcFile.CacheWarmup.MaxChunks; i++ {
		if i > maxBackgroundCacheWarmupChunksPerFile {
			// wait for any one of the write to complete before proceeding.
			if chunkIdx, err := waitForChunkWriteToComplete(); err != nil {
				log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, chunk Idx: %d, error: %v",
					handle.Path, handle.ID, chunkIdx, err)
				break
			}
		}

		// There is a write-back cache/staging aread maintained in the dcache file while file is being written. which is
		// set to 256 chunks by default. Although we are writing the 32 chunks in parallel, but the status of writes are
		// captured in the channel. So it is possible that the writes are completed out of order and slowly exhausting
		// staging aread and detecting our further writes as random write and fail. So better wait all the chunks that
		// fall outside the staging area to be written before proceeding to issue more writes.
		//
		if i >= numStagingChunks {
			// wait for the first chunk in the staging area to be uploaded before proceeding.
			// This condition is rarely hit as we are writing 32 chunks in parallel and the staging area is 256 chunks.
			if err := waitForChunkUploadToComplete(i - numStagingChunks); err != nil {
				log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, chunk Idx: %d, error: %v",
					handle.Path, handle.ID, i-numStagingChunks, err)
				break
			}
		}

		err := fillCache(dc, handle, dcFile, i)
		if err != nil {
			// stop further cache warmup.
			log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, error: %v",
				handle.Path, handle.ID, err)
			dcFile.CacheWarmup.Error.Store(err)
			break
		}

		dcFile.CacheWarmup.ScheduledChunkWrites.Add(1)
	}

	// wait for all pending writes to complete.
	for dcFile.CacheWarmup.ProcessedChunkWrites.Load() < dcFile.CacheWarmup.ScheduledChunkWrites.Load() {
		if chunkIdx, err := waitForChunkWriteToComplete(); err != nil {
			log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, chunk Idx: %d, error: %v",
				handle.Path, handle.ID, chunkIdx, err)
		}
	}

	finishCacheWarmupForFile(dc, handle, dcFile)
}

func fillCache(dc *DistributedCache, handle *handlemap.Handle, dcFile *fm.DcacheFile, chunkIdx int64) error {

	chunkSize := clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes

	// downlaod the chunk from Azure and write it to the cache.
	log.Debug("DistributedCache::fillCache: Starting cache warmup for file: %s, handle: %d, chunk Idx: %d",
		handle.Path, handle.ID, chunkIdx)

	chunkData, err := dcache.GetBuffer()
	if err != nil {
		log.Err("DistributedCache::fillCache: failed to get buffer for file: %s, handle: %d, error: %v",
			handle.Path, handle.ID, err)
		dcFile.CacheWarmup.Error.Store(err)
		return err
	}

	chunkStartoffset := chunkIdx * int64(chunkSize)
	log.Info("DistributedCache::fillCache: downloading chunk idx: %d, offset: %d for file: %s, handle: %d",
		chunkIdx, chunkStartoffset, handle.Path, handle.ID)

	currentChunkSize := min(int64(chunkSize), dcFile.CacheWarmup.Size-chunkStartoffset)

	// Read the chunk from Azure into the buffer.
	bytesRead, err := dc.NextComponent().ReadInBuffer(&internal.ReadInBufferOptions{
		Handle: handle,
		Offset: chunkStartoffset,
		Path:   handle.Path,
		Size:   int64(currentChunkSize),
		Data:   chunkData,
	})
	if err != io.EOF && err != nil {
		log.Err("DistributedCache::fillCache: failed to read chunk idx: %d, offset: %d for file: %s, handle: %d, error: %v",
			chunkIdx, chunkStartoffset, handle.Path, handle.ID, err)

		// free the buffer.
		dcache.PutBuffer(chunkData)
		return err
	}

	common.Assert(currentChunkSize == int64(bytesRead), currentChunkSize, bytesRead, err)
	common.Assert(int64(bytesRead) == currentChunkSize && bytesRead > 0, bytesRead, currentChunkSize)

	if bytesRead == 0 || currentChunkSize != int64(bytesRead) {
		// This should happen only when the file is being update in Azure, else this is a bug.
		err = io.ErrUnexpectedEOF
		log.Err("DistributedCache::fillCache: [BUG] failed to read chunk idx: %d, offset: %d for file: %s, handle: %d, error: %v",
			chunkIdx, chunkStartoffset, handle.Path, handle.ID, err)

		// free the buffer.
		dcache.PutBuffer(chunkData)
		return err
	}

	dcacheWrite := func() error {
		log.Debug("DistributedCache::fillCache: writing chunk idx: %d, offset: %d to cache for file: %s, handle: %d",
			chunkIdx, chunkStartoffset, handle.Path, handle.ID)

		err = dcFile.WriteFile(chunkStartoffset, chunkData[:bytesRead])
		if err != nil {
			log.Err("DistributedCache::fillCache: failed to write chunk idx: %d, offset: %d to cache for file: %s, handle: %d, error: %v",
				chunkIdx, chunkStartoffset, handle.Path, handle.ID, err)
			dcFile.CacheWarmup.Error.Store(err)
			dcFile.CacheWarmup.SuccessCh <- fm.ChunkWarmupStatus{ChunkIdx: chunkIdx, Err: err}
			return err
		}
		dcFile.CacheWarmup.SuccessCh <- fm.ChunkWarmupStatus{ChunkIdx: chunkIdx, Err: nil}

		// free the buffer.
		// TODO: we can avoid the buf copy in the write flow.. as chunkData was allocated from the buffer pool.
		dcache.PutBuffer(chunkData)
		return nil
	}

	dc.pw.EnqueuDcacheWrite(dcacheWrite)

	return nil
}

// It blocks the caller until the chunks enclosing this offset is uploaded to the cache.
func waitForChunksToCompleteWarmup(handle *handlemap.Handle, dcFile *fm.DcacheFile, offset int64, bufSize int64) error {

	chunkSize := clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes
	startChunkIdx := offset / int64(chunkSize)
	endOffset := offset + bufSize - 1
	endChunkIdx := min(endOffset/int64(chunkSize), dcFile.CacheWarmup.MaxChunks-1)

	// max Read Buf Size can be 1MB from FUSE. Hence that can fall in 2 chunks at max.
	common.Assert(endChunkIdx-startChunkIdx <= 1, startChunkIdx, endChunkIdx, offset, bufSize, chunkSize)

	var err error

	for chunkIdx := startChunkIdx; chunkIdx <= endChunkIdx; chunkIdx++ {
		if Ierr := dcFile.CacheWarmup.Error.Load(); err != nil {
			// cache warmup failed.
			err = Ierr.(error)
			break
		}

		// check if corresponding bit is set in the bitmap.
		if ok := common.AtomicTestBitUint64(&dcFile.CacheWarmup.Bitmap[chunkIdx/64], uint(chunkIdx%64)); !ok {
			log.Debug("DistributedCache::waitForChunksToComplete: Waiting for cache warmup completion for file: %s, handle: %d, chunk Idx: %d, SuccessfulChunks: %d, MaxChunks: %d",
				handle.Path, handle.ID, chunkIdx, dcFile.CacheWarmup.SuccessfulChunkWrites.Load(), dcFile.CacheWarmup.MaxChunks)

			time.Sleep(100 * time.Millisecond)
			chunkIdx--
		}
	}

	return err
}

func logStatusForCacheWarmup(handle *handlemap.Handle, dcFile *fm.DcacheFile) {
	log.Info("DistributedCache::logStatusForCacheWarmup : Cache warmup status for Dcache file : %s, handle: %d, Successful chunks: %d, MaxChunks: %d",
		handle.Path, handle.ID, dcFile.CacheWarmup.SuccessfulChunkWrites.Load(), dcFile.CacheWarmup.MaxChunks)

	if dcFile.CacheWarmup.Error.Load() != nil {
		log.Err("DistributedCache::logStatusForCacheWarmup : Cache warmup failed for Dcache file : %s, handle: %d, error: %v",
			handle.Path, handle.ID, dcFile.CacheWarmup.Error.Load())
	}
}

func finishCacheWarmupForFile(dc *DistributedCache, handle *handlemap.Handle, dcFile *fm.DcacheFile) {
	log.Debug("DistributedCache::checkStatusForCacheWarmup : Checking cache warmup status for Dcache file : %s, handle: %d",
		handle.Path, handle.ID)

	defer func() {
		// check if the read handle is closed before warmup and mark the cache warmup as completed. in that case we
		// should also release azure handle.
		if ok := dcFile.CacheWarmup.Completed.CompareAndSwap(false, true); !ok {
			azureErr := dc.NextComponent().CloseFile(internal.CloseFileOptions{Handle: handle})
			if azureErr != nil {
				log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to Close Azure handle for file : %s, handle: %d, error: %v",
					handle.Path, handle.ID, azureErr)
			} else {
				log.Info("DistributedCache::checkStatusForCacheWarmup : Successfully Closed Azure handle for file : %s, handle: %d",
					handle.Path, handle.ID)
			}
		}
	}()

	removeFile := false

	// check if the cache warmup completed successfully.
	if dcFile.CacheWarmup.Error.Load() != nil {
		// no cache warmup setup for this file.
		log.Err("DistributedCache::checkStatusForCacheWarmup : Cache warmup failed for Dcache file : %s, handle: %d, error: %v",
			handle.Path, handle.ID, dcFile.CacheWarmup.Error.Load())

		if err := dcFile.ReleaseFile(true); err != nil {
			log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to ReleaseFile for Dcache file : %s, error: %v",
				handle.Path, err)
		}
		removeFile = true
	}

	common.Assert(dcFile.CacheWarmup.Error.Load() != nil ||
		(dcFile.CacheWarmup.SuccessfulChunkWrites.Load() == dcFile.CacheWarmup.MaxChunks &&
			dcFile.CacheWarmup.ScheduledChunkWrites.Load() == dcFile.CacheWarmup.MaxChunks &&
			dcFile.CacheWarmup.ProcessedChunkWrites.Load() == dcFile.CacheWarmup.MaxChunks),
		dcFile.CacheWarmup.SuccessfulChunkWrites.Load(), dcFile.CacheWarmup.ScheduledChunkWrites.Load(),
		dcFile.CacheWarmup.ProcessedChunkWrites.Load(), dcFile.CacheWarmup.MaxChunks)

	if dcFile.CacheWarmup.SuccessfulChunkWrites.Load() == dcFile.CacheWarmup.MaxChunks {

		// finalize the dcache file only if the cache warmup completed successfully and all chunks are read.
		// Flush and release the dcache file. Any error during the flush will result in deleteing
		// such dcache file.
		err := dcFile.CloseFile()
		if err != nil {
			log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to CloseFile for Dcache file : %s, error: %v",
				handle.Path, err)
			// Remove this file from the dcache.
			removeFile = true
		} else {
			log.Info("DistributedCache::checkStatusForCacheWarmup : Successfully finalized Dcache file : %s",
				handle.Path)
			// remove the chunks from the chunk map.
			if err = dcFile.ReleaseFile(false); err != nil {
				log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to ReleaseFile for Dcache file : %s, error: %v",
					handle.Path, err)
				// No need to delete the dcache file as its finalized successfully.
			} else {
				log.Info("DistributedCache::checkStatusForCacheWarmup : Successfully Released Dcache file : %s",
					handle.Path)
			}
		}
	} else {
		// delete the dcache file if the cache warmup did not complete successfully or there was an error during
		// cache warmup.
		log.Err("DistributedCache::checkStatusForCacheWarmup : Cache warmup status for Dcache file : %s, Successful chunks: %d, MaxChunks: %d",
			handle.Path, dcFile.CacheWarmup.SuccessfulChunkWrites.Load(), dcFile.CacheWarmup.MaxChunks)
		removeFile = true
	}

	if removeFile {
		log.Info("DistributedCache::checkStatusForCacheWarmup : Deleting Dcache file : %s", dcFile.FileMetadata.Filename)

		err := fm.DeleteDcacheFile(dcFile.FileMetadata.Filename, true)
		if err != nil {
			log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to Delete wDcache file : %s, error: %v",
				dcFile.FileMetadata.Filename, err)
		} else {
			log.Info("DistributedCache::checkStatusForCacheWarmup : Successfully Deleted Dcache file : %s",
				dcFile.FileMetadata.Filename)
		}
	}
}
