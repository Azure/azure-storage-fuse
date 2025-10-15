package agents

import (
	"io"
	"sync"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type WarmupHandle struct {
	WriteFile *fm.DcacheFile
	ReadFile  *fm.DcacheFile
}

var inProgressFiles sync.Map // map[string]WarmupHandle

func GetSizeIfWarmupScheduled(filePath string) int64 {

	IwarmupHandle, exists := inProgressFiles.Load(filePath)
	if exists {
		// Warmup is in progress for this file.
		log.Info("DistributedCache::updateSizeIfWarmupScheduled : Warmup in progress for file : %s", filePath)
		return IwarmupHandle.(WarmupHandle).WriteFile.FileMetadata.WarmupSize
	}

	return -1
}

func TryWarmup(handle *handlemap.Handle, chunkSize int64,
	readFileFromAzure func(*handlemap.Handle, int64 /* offset */, int64 /* size */, []byte /* data */) (int, error)) (*WarmupHandle, error) {

	dcFile, err := fm.NewDcacheFile(handle.Path, true, handle.Size)
	if err != nil {
		log.Err("DistributedCache::CreateFile : Dcache File Creation failed with err : %v, path : %s", err, handle.Path)
		return nil, err
	}

	readDcFile, err := fm.OpenDcacheFile(handle.Path, false /* fromFuse */)
	if err != nil {
		log.Err("DistributedCache::CreateFile : Dcache File Open failed with err : %v, path : %s", err, handle.Path)
		dcFile.CloseFile()
		return nil, err
	}

	log.Info("DistributedCache::TryWarmup : Starting warmup for file : %s, size : %d", handle.Path, handle.Size)

	handle.IFObj = dcFile

	data := make([]byte, chunkSize)
	// Start a go routine to warmup the file from Azure to Dcache.
	go func() {
		for i := int64(0); i < handle.Size; i += chunkSize {
			// Read the chunk from Azure.
			bytesRead, err := readFileFromAzure(handle, i, handle.Size-i, data)

			common.Assert(bytesRead > 0 || err == io.EOF, handle.Path, bytesRead, err)

			if err != nil && err != io.EOF {
				log.Err("DistributedCache::TryWarmup : Failed with err : %v, path : %s", err, handle.Path)
				break
			} else {
				log.Info("DistributedCache::TryWarmup : Warmup read %d bytes for file : %s, offset : %d", bytesRead, handle.Path, i)
			}

			// Write the chunk to Dcache.
			dcacheErr := dcFile.WriteFile(i, data[:bytesRead])
			if dcacheErr != nil {
				// If write on one media fails, then return err instantly
				log.Err("DistributedCache::TryWarmup : Dcache File write Failed, offset : %d, file : %s",
					i, handle.Path)
				break
			} else {
				log.Info("DistributedCache::TryWarmup : Warmup wrote %d bytes for file : %s, offset : %d", bytesRead, handle.Path, i)
			}

		}

		if err != nil && err != io.EOF {
			// Delete the file from Dcache.
			err := fm.DeleteDcacheFile(handle.Path)
			if err != nil {
				log.Err("DistributedCache::TryWarmup: Delete failed for Dcache file %s: %v", handle.Path, err)
				return
			}
		} else {
			log.Info("DistributedCache::TryWarmup : Warmup completed for file : %s, finalizing the file", handle.Path)
			dcacheErr := dcFile.CloseFile()
			if dcacheErr != nil {
				log.Err("DistributedCache::TryWarmup : Dcache File close Failed, file : %s", handle.Path)
				// Delete the file from Dcache.
				err := fm.DeleteDcacheFile(handle.Path)
				if err != nil {
					log.Err("DistributedCache::TryWarmup: Delete failed for Dcache file %s: %v", handle.Path, err)
				}
			} else {
				// Clear this flag to signal no more writes on this handle.
				// Fail any writes that come after this.
				handle.SetDcacheStopWrites()
			}
		}
	}()

	wh := WarmupHandle{
		WriteFile: dcFile,
		ReadFile:  readDcFile,
	}

	inProgressFiles.Store(handle.Path, wh)

	return &wh, nil

}
