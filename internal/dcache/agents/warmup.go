package agents

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

var inProgressWarmUpFilesMap sync.Map // map[string]*fm.DcacheFile(this is warmUp Write Dcache file)

var (
	ErrWarmupNotInManualMode   error = errors.New("Warmup not in manual mode")
	ErrWarmupChunkNotAvailable error = errors.New("Warmup chunk not available")
)

func GetSizeIfIScheduledWarmup(filePath string) int64 {

	IwarmupHandle, exists := inProgressWarmUpFilesMap.Load(filePath)
	if exists {
		// Warmup is in progress for this file.
		log.Info("DistributedCache::updateSizeIfWarmupScheduled : Warmup in progress for file : %s", filePath)
		return IwarmupHandle.(*fm.DcacheFile).FileMetadata.WarmupSize
	}

	return -1
}

func TryWarmup(handle *handlemap.Handle,
	readFileFromAzure func(*handlemap.Handle, int64 /* offset */, int64 /* size */, []byte /* data */) (int, error),
	releaseFileAzure func(*handlemap.Handle) error) (*fm.DcacheFile, error) {

	chunkSize := int64(clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes)
	chanClosed := false

	warmDcFile, err := fm.NewDcacheFile(handle.Path, true, handle.Size)
	if err != nil {
		log.Err("DistributedCache::TryWarmup : Dcache File Creation failed with err : %v, path : %s", err, handle.Path)
		return nil, err
	}

	readDcFile, err := fm.OpenDcacheFile(handle.Path, false /* fromFuse */)
	if err != nil && err != fm.ErrFileNotReady {
		log.Err("DistributedCache::TryWarmup : Dcache File Open failed with err : %v, path : %s", err, handle.Path)
		warmDcFile.CloseFile()
		return nil, err
	}

	log.Info("DistributedCache::TryWarmup : Starting warmup for file : %s, size : %d", handle.Path, handle.Size)

	data := make([]byte, chunkSize)

	readDcFile.WarmupFileInfo = &fm.WarmupFileInfo{
		WarmupFile:               warmDcFile,
		CurWarmChunkReadRequests: make(chan *fm.CurWarmChunkReadReq, 100),
	}

	// Start a go routine to warmup the file from Azure to Dcache.
	go func() {
		var err, dcacheErr error
		var bytesRead int

		defer func() {
			// Check if the Azure File handle is already closed before warmup completed for the file. otherwise, it is
			// our responsibility to close it.
			if ok := readDcFile.WarmupFileInfo.CloseOnWarmupComplete.CompareAndSwap(false, true); !ok {
				if err = releaseFileAzure(handle); err != nil {
					log.Err("DistributedCache::TryWarmup : ReleaseFileAzure Failed with err : %v, path : %s", err, handle.Path)
				} else {
					log.Info("DistributedCache::TryWarmup : Released Azure handle for file : %s", handle.Path)
				}
			}

			if !chanClosed {
				close(readDcFile.WarmupFileInfo.CurWarmChunkReadRequests)
				// Empty the channel for the ones who are waiting for their responses.
				for req := range readDcFile.WarmupFileInfo.CurWarmChunkReadRequests {
					req.ErrorResp <- ErrWarmupNotInManualMode
				}
			}

			inProgressWarmUpFilesMap.Delete(handle.Path)
		}()

		atomaticWarmup := false

		for i := int64(0); i < handle.Size; i += chunkSize {
			readDcFile.WarmupFileInfo.CurWarmChunkIdx.Store(i / chunkSize)
			// Read the chunk from Azure.
			bytesRead, err = readFileFromAzure(handle, i, handle.Size-i, data)

			common.Assert(bytesRead > 0 || err == io.EOF, handle.Path, bytesRead, err)

			if err != nil && err != io.EOF {
				log.Err("DistributedCache::TryWarmup : Failed with err : %v, path : %s", err, handle.Path)
				break
			} else {
				log.Info("DistributedCache::TryWarmup : Warmup read %d bytes for file : %s, offset : %d", bytesRead, handle.Path, i)
			}

			// Write the chunk to Dcache.
			dcacheErr = warmDcFile.WriteFile(i, data[:bytesRead], false /* fromFuse */)
			if dcacheErr != nil {
				// If write on one media fails, then return err instantly
				log.Err("DistributedCache::TryWarmup : Dcache File write Failed, offset : %d, file : %s",
					i, handle.Path)
				break
			} else {
				log.Info("DistributedCache::TryWarmup : Warmup wrote %d bytes for file : %s, offset : %d", bytesRead, handle.Path, i)
			}

			// We start the with manual warmup mode. So, we wait for the chunk to be consumed before reading next chunk.
			// else we just continue reading next chunk. and fall back to slow path.
			if atomaticWarmup {
				continue
			}

			bytesReadFromChunk := int64(0)
			// we give a timeout of 5 seconds to user application to read from the warmed upchunk. Otherwise, we switch
			// atomatic warmup mode. where user application reads will go to slow path.
			clientChunkReadTimeout := time.After(5 * time.Second)
		loop:
			for {
				select {
				case req := <-readDcFile.WarmupFileInfo.CurWarmChunkReadRequests:
					// Read request received for the current warmed up chunk.
					if req.ChunkIdx != readDcFile.WarmupFileInfo.CurWarmChunkIdx.Load() {
						log.Err("DistributedCache::TryWarmup : Warmup chunk read request chunkIdx %d does not match current warmup chunkIdx %d, file : %s",
							req.ChunkIdx, readDcFile.WarmupFileInfo.CurWarmChunkIdx.Load(), handle.Path)
						req.ErrorResp <- ErrWarmupChunkNotAvailable
						continue
					}

					n := copy(req.Buf, data[req.OffsetInChunk:min(int64(bytesRead), req.OffsetInChunk+req.LenInterested)])
					req.BytesReadResp = int64(n)
					bytesReadFromChunk += int64(n)

					log.Info("DistributedCache::TryWarmup : Served warmup chunk read request for chunkIdx %d, offset: %d, bytesRead %d, file : %s",
						req.ChunkIdx, req.ChunkIdx*int64(chunkSize)+req.OffsetInChunk, n, handle.Path)
					req.ErrorResp <- nil
					if bytesReadFromChunk >= int64(bytesRead) {
						// All data from this chunk has been read.
						break loop
					}

				case <-clientChunkReadTimeout:
					log.Info("DistributedCache::TryWarmup : Changing mode from MANUAL->ATOMATIC, file : %s", handle.Path)
					close(readDcFile.WarmupFileInfo.CurWarmChunkReadRequests)
					// Empty the channel for the ones who are waiting for their responses.
					for req := range readDcFile.WarmupFileInfo.CurWarmChunkReadRequests {
						req.ErrorResp <- ErrWarmupNotInManualMode
					}
					atomaticWarmup = true
					chanClosed = true
					break loop
				}

			}

		}

		if (err != nil && err != io.EOF) || (dcacheErr != nil) {
			// Delete the file from Dcache.
			err := fm.DeleteDcacheFile(handle.Path)
			if err != nil {
				log.Err("DistributedCache::TryWarmup: Delete failed for Dcache file %s: %v", handle.Path, err)
				return
			}
		} else {
			log.Info("DistributedCache::TryWarmup : Warmup completed for file : %s, finalizing the file", handle.Path)
			dcacheErr := warmDcFile.CloseFile()
			if dcacheErr != nil {
				log.Err("DistributedCache::TryWarmup : Dcache File close Failed, file : %s", handle.Path)
				// Delete the file from Dcache.
				err := fm.DeleteDcacheFile(handle.Path)
				if err != nil {
					log.Err("DistributedCache::TryWarmup: Delete failed for Dcache file %s: %v", handle.Path, err)
				} else {
					log.Info("DistributedCache::TryWarmup: Deleted dcache file %s after close failed", handle.Path)
				}
			} else {
				log.Info("DistributedCache::TryWarmup : Warmup finalized for file : %s", handle.Path)
			}
		}
	}()

	inProgressWarmUpFilesMap.Store(handle.Path, warmDcFile)

	return readDcFile, nil
}

func TryReadFromCurWarmedUpChunk(dcFile *fm.DcacheFile, offset int64, buf []byte) (ok bool, bytesRead int64) {
	len := int64(len(buf))
	if dcFile.WarmupFileInfo == nil {
		// warmUp is not scheduled for this file.
		return false, 0
	}

	// check if both offset and offset+len-1 fall within the same chunk
	chunkSize := int64(clustermap.GetCacheConfig().ChunkSizeMB * common.MbToBytes)

	chunkIdx := offset / chunkSize
	chunkStart := chunkIdx * chunkSize
	chunkEnd := chunkStart + chunkSize - 1
	if offset < chunkStart || (offset+len-1) > chunkEnd {
		return false, 0
	}

	if dcFile.WarmupFileInfo.CurWarmChunkIdx.Load() != chunkIdx {
		// warmUp has not started for this file.
		return false, 0
	}

	// Read data from the current warmed up chunk.
	req := &fm.CurWarmChunkReadReq{
		ChunkIdx:      chunkIdx,
		OffsetInChunk: offset - chunkStart,
		LenInterested: len,
		Buf:           buf,
		ErrorResp:     make(chan error, 1),
	}

	select {
	case dcFile.WarmupFileInfo.CurWarmChunkReadRequests <- req:
		log.Info("DistributedCache::TryReadFromCurWarmedUpChunk : Sent request to read from warmup chunkIdx : %d, offset : %d, lenInterested : %d",
			req.ChunkIdx, offset, req.LenInterested)
		// Request sent successfully.
		err := <-req.ErrorResp
		if err != nil {
			log.Err("DistributedCache::TryReadFromCurWarmedUpChunk : Failed to read from warmup chunk, err : %v", err)
			return false, 0
		}
		return true, req.BytesReadResp
	default:
		// Warmup is in atomatic mode or we are unable to send request, that is fine move it to slow path.
		return false, 0
	}

}
