package agents

import (
	"io"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

var inProgressWarmUpFilesMap sync.Map // map[string]*fm.DcacheFile(i.e., warm one)

func GetSizeIfIScheduledWarmup(filePath string) int64 {

	IwarmupHandle, exists := inProgressWarmUpFilesMap.Load(filePath)
	if exists {
		// Warmup is in progress for this file.
		log.Info("DistributedCache::updateSizeIfWarmupScheduled : Warmup in progress for file : %s", filePath)
		return IwarmupHandle.(*fm.DcacheFile).FileMetadata.WarmupSize
	}

	return -1
}

func TryWarmup(handle *handlemap.Handle, chunkSize int64,
	readFileFromAzure func(*handlemap.Handle, int64 /* offset */, int64 /* size */, []byte /* data */) (int, error),
	releaseFileAzure func(*handlemap.Handle) error) (*fm.DcacheFile, error) {

	warmDcFile, err := fm.NewDcacheFile(handle.Path, true, handle.Size)
	if err != nil {
		log.Err("DistributedCache::CreateFile : Dcache File Creation failed with err : %v, path : %s", err, handle.Path)
		return nil, err
	}

	readDcFile, err := fm.OpenDcacheFile(handle.Path, false /* fromFuse */)
	if err != nil && err != fm.ErrFileNotReady {
		log.Err("DistributedCache::CreateFile : Dcache File Open failed with err : %v, path : %s", err, handle.Path)
		warmDcFile.CloseFile()
		return nil, err
	}

	log.Info("DistributedCache::TryWarmup : Starting warmup for file : %s, size : %d", handle.Path, handle.Size)

	data := make([]byte, chunkSize)
	// Start a go routine to warmup the file from Azure to Dcache.
	go func() {
		var err, dcacheErr error
		var bytesRead int

		defer func() {
			// Check if the Azure File handle is already closed before warmup completed for the file. otherwise, it is
			// our responsibility to close it.
			if ok := readDcFile.CloseOnWarmupComplete.CompareAndSwap(false, true); !ok {
				if err = releaseFileAzure(handle); err != nil {
					log.Err("DistributedCache::TryWarmup : ReleaseFileAzure Failed with err : %v, path : %s", err, handle.Path)
				} else {
					log.Info("DistributedCache::TryWarmup : Released Azure handle for file : %s", handle.Path)
				}
			}

			inProgressWarmUpFilesMap.Delete(handle.Path)
		}()

		for i := int64(0); i < handle.Size; i += chunkSize {
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

	readDcFile.WarmupFile = warmDcFile
	inProgressWarmUpFilesMap.Store(handle.Path, warmDcFile)

	return readDcFile, nil
}
