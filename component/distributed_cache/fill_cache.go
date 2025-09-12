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
	maxBackgroundCacheWarmupChunks = 16
)

func startCacheWarmup(dc *DistributedCache, handle *handlemap.Handle) {
	dcFile := handle.IFObj.(*fm.DcacheFile)

	if dcFile == nil {
		// no cache warmup setup for this file.
		return
	}

	var err error

	// wait for status of last write to complete.
	waitForLastWrite := func() error {
		chunkStatus := <-dcFile.CacheWarmup.SuccessCh
		if chunkStatus.Err != nil {
			log.Err("DistributedCache::startCacheWarmup: Error during cache warmup for file: %s, handle: %d, error: %v",
				handle.Path, handle.ID, err)
			// stop further cache warmup.
			dcFile.CacheWarmup.Error.Store(err)
			return err
		} else {
			dcFile.CacheWarmup.SuccessfulChunks.Add(1)
			ok := common.AtomicTestAndSetBitUint64(&dcFile.CacheWarmup.Bitmap[chunkStatus.ChunkIdx/64], uint(chunkStatus.ChunkIdx%64))
			common.Assert(ok, chunkStatus.ChunkIdx)
			_ = ok
		}
		return nil
	}

	// work on 16 chunks at a time.
	for i := int64(0); i < dcFile.CacheWarmup.MaxChunks; i++ {
		if i > maxBackgroundCacheWarmupChunks {
			// wait for the first write to complete before proceeding.
			if err = waitForLastWrite(); err != nil {
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

	}

	// wait for all pending writes to complete.
	for dcFile.CacheWarmup.SuccessfulChunks.Load() < dcFile.CacheWarmup.MaxChunks && dcFile.CacheWarmup.Error.Load() == nil {
		waitForLastWrite()
	}

	// flush and finalize the dcache file if the cache warmup completed successfully.
	checkStatusForCacheWarmup(handle, dcFile)
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
		return err
	}

	common.Assert(currentChunkSize == int64(bytesRead), currentChunkSize, bytesRead, err)

	dcacheWrite := func() error {
		log.Debug("DistributedCache::fillCache: writing chunk idx: %d, offset: %d to cache for file: %s, handle: %d",
			chunkIdx, chunkStartoffset, handle.Path, handle.ID)

		err = dcFile.WriteFile(chunkStartoffset, chunkData)
		if err != nil {
			log.Err("DistributedCache::fillCache: failed to write chunk idx: %d, offset: %d to cache for file: %s, handle: %d, error: %v",
				chunkIdx, chunkStartoffset, handle.Path, handle.ID, err)
			dcFile.CacheWarmup.Error.Store(err)
			dcFile.CacheWarmup.SuccessCh <- fm.ChunkWarmupStatus{ChunkIdx: chunkIdx, Err: err}
			return err
		}
		dcFile.CacheWarmup.SuccessCh <- fm.ChunkWarmupStatus{ChunkIdx: chunkIdx, Err: nil}
		return nil
	}

	dc.pw.EnqueuAzureWrite(dcacheWrite)

	return nil
}

// It blocks the caller until the chunk enclosing this offset is uploaded to the cache.
func waitForChunkWarmupCompletion(handle *handlemap.Handle, dcFile *fm.DcacheFile, offset int64) error {

	chunkSize := clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes
	chunkIdx := offset / int64(chunkSize)

	var err error

	for {
		if Ierr := dcFile.CacheWarmup.Error.Load(); err != nil {
			// cache warmup failed.
			err = Ierr.(error)
			break
		}

		// check if corresponding bit is set in the bitmap.
		if ok := common.AtomicTestBitUint64(&dcFile.CacheWarmup.Bitmap[chunkIdx/64], uint(chunkIdx%64)); ok {
			break
		}

		log.Debug("DistributedCache::waitForChunkWarmupCompletion: Waiting for cache warmup completion for file: %s, handle: %d, chunk Idx: %d, SuccessfulChunks: %d, MaxChunks: %d",
			handle.Path, handle.ID, chunkIdx, dcFile.CacheWarmup.SuccessfulChunks.Load(), dcFile.CacheWarmup.MaxChunks)

		time.Sleep(100 * time.Millisecond)
	}

	return err
}

func logStatusForCacheWarmup(handle *handlemap.Handle, dcFile *fm.DcacheFile) {
	log.Info("DistributedCache::logStatusForCacheWarmup : Cache warmup status for Dcache file : %s, handle: %d, Successful chunks: %d, MaxChunks: %d",
		handle.Path, handle.ID, dcFile.CacheWarmup.SuccessfulChunks.Load(), dcFile.CacheWarmup.MaxChunks)

	if dcFile.CacheWarmup.Error.Load() != nil {
		log.Err("DistributedCache::logStatusForCacheWarmup : Cache warmup failed for Dcache file : %s, handle: %d, error: %v",
			handle.Path, handle.ID, dcFile.CacheWarmup.Error.Load())
	}
}

func checkStatusForCacheWarmup(handle *handlemap.Handle, dcFile *fm.DcacheFile) {
	log.Debug("DistributedCache::checkStatusForCacheWarmup : Checking cache warmup status for Dcache file : %s, handle: %d",
		handle.Path, handle.ID)

	// check if the cache warmup completed successfully.
	if dcFile.CacheWarmup.Error.Load() != nil {
		// no cache warmup setup for this file.
		log.Err("DistributedCache::checkStatusForCacheWarmup : Cache warmup failed for Dcache file : %s, handle: %d, error: %v",
			handle.Path, handle.ID, dcFile.CacheWarmup.Error.Load())
		return
	}

	common.Assert(dcFile.CacheWarmup.SuccessfulChunks.Load() == dcFile.CacheWarmup.MaxChunks,
		dcFile.CacheWarmup.SuccessfulChunks.Load(), dcFile.CacheWarmup.MaxChunks)

	removeFile := false

	if dcFile.CacheWarmup.SuccessfulChunks.Load() == dcFile.CacheWarmup.MaxChunks {

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
		}
	} else {
		// delete the dcache file if the cache warmup did not complete successfully or there was an error during
		// cache warmup.
		log.Err("DistributedCache::checkStatusForCacheWarmup : Cache warmup status for Dcache file : %s, Successful chunks: %d, MaxChunks: %d",
			handle.Path, dcFile.CacheWarmup.SuccessfulChunks.Load(), dcFile.CacheWarmup.MaxChunks)
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
