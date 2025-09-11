package distributed_cache

import (
	"fmt"
	"io"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Reads the file in chunks and populates dcache with the data read from the Azure.
// Always reads and writes the chunks in whole, except for the last chunk which may be partial.
// If there is any error during reading from Azure or writing to dcache, the error is recorded and stops the further
// cache warmup. Such stale dcache file will be deleted when the file is closed. Currenlty the file is only cached to
// the dcache, when it is fully read sequentially. Any random read or read beyond 16 chunks from the last warmed up chunk
// will stop the cache warmup and such stale dcache file will be deleted when the file is closed.
func (dc *DistributedCache) fillCache(handle *handlemap.Handle, offset int64) {
	dcFile := handle.IFObj.(*fm.DcacheFile)

	if dcFile == nil {
		// no cache warmup setup for this file.
		return
	}
	chunkSize := clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes

	warmedUpBytes := dcFile.CacheWarmup.NxtChunkIdxToRead.Load() * int64(chunkSize)

	if offset <= warmedUpBytes || dcFile.CacheWarmup.Error.Load() != nil {
		return
	}

	// Start cache warmup only if the offset being read is beyond the warmed up bytes.
	dcFile.CacheWarmup.Lock()
	defer dcFile.CacheWarmup.Unlock()

	nxtChunkIdxToRead := dcFile.CacheWarmup.NxtChunkIdxToRead.Load()

	warmedUpBytes = nxtChunkIdxToRead * int64(chunkSize)
	if offset <= warmedUpBytes || dcFile.CacheWarmup.Error.Load() != nil {
		return
	}

	// downlaod the chunk from Azure and write it to the cache.
	log.Info("DistributedCache::fillCache: Starting cache warmup for file: %s, handle: %d, offset: %d",
		handle.Path, handle.ID, offset)

	chunkIdx := offset / int64(chunkSize)
	chunkIdx = min(chunkIdx, dcFile.CacheWarmup.MaxChunks-1)

	// we allow async reads of 16 chunks, all the other io patterns will cause the cache warmup to stop &
	// we delete the the chunks downloaded so far on close.
	if chunkIdx-nxtChunkIdxToRead > 16 {
		err := fmt.Errorf("random read detected; stopping warming up the cache for file: %s, handle: %d, offset: %d, lastWarmedUpChnkIdx: %d, chunkIdx: %d",
			handle.Path, handle.ID, offset, nxtChunkIdxToRead, chunkIdx)
		dcFile.CacheWarmup.Error.Store(err)
		log.Err("DistributedCache::fillCache: %v", err)
		return
	}

	chunkData, err := dcache.GetBuffer()
	if err != nil {
		log.Err("DistributedCache::fillCache: failed to get buffer for file: %s, handle: %d, error: %v",
			handle.Path, handle.ID, err)
		dcFile.CacheWarmup.Error.Store(err)
		return
	}

	for i := nxtChunkIdxToRead; i <= chunkIdx; i++ {
		chunkStartoffset := i * int64(chunkSize)
		log.Info("DistributedCache::fillCache: downloading chunk idx: %d, offset: %d for file: %s, handle: %d",
			i, chunkStartoffset, handle.Path, handle.ID)

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
				i, chunkStartoffset, handle.Path, handle.ID, err)
			dcFile.CacheWarmup.Error.Store(err)
			return
		}

		common.Assert(currentChunkSize == int64(bytesRead), currentChunkSize, bytesRead, err)

		// Write the chunk data to the cache.
		// This can be done in parallel for some chunks, but for now doing it serially.
		err = dcFile.WriteFile(chunkStartoffset, chunkData)
		if err != nil {
			log.Err("DistributedCache::fillCache: failed to write chunk idx: %d, offset: %d to cache for file: %s, handle: %d, error: %v",
				i, chunkStartoffset, handle.Path, handle.ID, err)
			dcFile.CacheWarmup.Error.Store(err)
			return
		}

		dcFile.CacheWarmup.NxtChunkIdxToRead.Add(1)
	}

	dcache.PutBuffer(chunkData)

}

func checkStatusForCacheWarmup(handle *handlemap.Handle) {

	dcFile := handle.IFObj.(*fm.DcacheFile)
	if dcFile == nil {
		return
	}

	removeFile := false

	if dcFile.CacheWarmup.NxtChunkIdxToRead.Load() == dcFile.CacheWarmup.MaxChunks &&
		dcFile.CacheWarmup.Error.Load() == nil {

		// finalize the dcache file only if the cache warmup completed successfully and all chunks are read.
		// Flush and release the dcache file. Any error during the flush will result in deleteing
		// such dcache file.
		err := dcFile.CloseFile()
		if err != nil {
			log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to CloseFile for Dcache file : %s, error: %v",
				handle.Path, err)
			// Remove this file from the dcache.
			removeFile = true
		}

		err = dcFile.ReleaseFile(false)
		if err != nil {
			log.Err("DistributedCache::checkStatusForCacheWarmup : Failed to ReleaseFile for Dcache file : %s, error: %v",
				handle.Path, err)
		}
	} else {
		// delete the dcache file if the cache warmup did not complete successfully or there was an error during
		// cache warmup.
		log.Err("DistributedCache::checkStatusForCacheWarmup : Cache warmup status for Dcache file : %s, NxtChunkIdxToRead: %d, MaxChunks: %d, Error: %v",
			handle.Path, dcFile.CacheWarmup.NxtChunkIdxToRead.Load(), dcFile.CacheWarmup.MaxChunks, dcFile.CacheWarmup.Error)
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
