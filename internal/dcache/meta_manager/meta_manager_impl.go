package metadata_manager

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcache "github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/google/uuid"
)

// FileMetaManager is the implementation of MetaManager interface
type FileMetaManager struct {
	cacheDir        string
	storageCallback dcache.StorageCallbacks
}

// NewMetaManager creates a new implementation of the MetaManager interface
func NewMetaManager(cacheID string, storageCallback dcache.StorageCallbacks) (*FileMetaManager, error) {
	return &FileMetaManager{
		cacheDir:        "__CACHE__" + cacheID,
		storageCallback: storageCallback,
	}, nil
}

// Implement all interface methods
func (m *FileMetaManager) CreateFile(filePath string, fileLayout *dcache.FileLayout) error {
	// TODO :: Use existing function
	guid := uuid.New()
	// Create the metadata structure
	fileMetaData := dcache.FileMetadata{
		FilePath:        filePath,
		FileID:          guid.String(),
		Size:            0,
		ClusterMapEpoch: 23, //TODO :: Get this value :: pass or from config if possible?
		FileLayout:      *fileLayout,
	}

	// Convert the metadata to JSON
	jsonData, err := json.MarshalIndent(fileMetaData, "", "  ")
	if err != nil {
		log.Debug("CreateMetaFile :: Failed to marshal metadata to JSON: %v", err)
		return err
	}
	// Store the open-count in the metadata blob property
	openCount := "0"
	metadata := map[string]*string{
		"openCount": &openCount,
	}

	err = m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(m.cacheDir, filePath),
		Metadata:               metadata,
		Data:                   jsonData,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("CreateMetaFile :: Failed to put blob in storage: %v", err)
		return err
	}
	return nil
}

func (m *FileMetaManager) CreateCacheInternalFile(filePath string, data []byte) error {
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(m.cacheDir, filePath),
		Data:                   data,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("CreateCacheInternalFile :: Failed to put blob in storage: %v", err)
		return err
	}
	return nil
}

func (m *FileMetaManager) DeleteFile(filePath string) error {
	// TODO :: Need to check need to send filepath or filename
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: filepath.Join(m.cacheDir, filePath),
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteMetaFile :: DeleteBlobInStorage failed since blob is already deleted")
			return nil
		}
	}
	return err
}

func (m *FileMetaManager) IncrementFileOpenCount(filePath string) error {
	err := m.updateHandleCount(filePath, true)
	if err != nil {
		log.Debug("IncrementFileOpenCount :: Failed to update file open count: %v", err)
		return err
	}
	return nil
}

func (m *FileMetaManager) DecrementFileOpenCount(filePath string) error {
	err := m.updateHandleCount(filePath, false)
	if err != nil {
		log.Debug("DecrementFileOpenCount :: Failed to update file open count: %v", err)
		return err
	}
	return nil
}

func (m *FileMetaManager) updateHandleCount(filename string, increment bool) error {
	const maxBackoff = 30 // Maximum backoff time in seconds
	backoff := 1          // Initial backoff time in seconds

	for {
		// Get the current handle count
		attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
			Name: filename,
		})
		if err != nil {
			log.Debug("GetFileOpenCount :: Failed to get handle count: %v", err)
			return err
		}

		if attr.Metadata["opencount"] != nil {
			openCount, err := strconv.Atoi(*attr.Metadata["opencount"])
			if err != nil {
				log.Debug("GetFileOpenCount :: Failed to parse handle count: %v", err)
				return err
			}
			if increment {
				openCount++
			} else {
				openCount--
			}
			if openCount < 0 {
				openCount = 0
			}
			openCountStr := strconv.Itoa(openCount)
			attr.Metadata["opencount"] = &openCountStr
		}

		// Set the new metadata in storage
		err = m.storageCallback.SetPropertiesInStorage(internal.SetAttrOptions{
			Path:      filename,
			Metadata:  attr.Metadata,
			ETag:      to.Ptr(azcore.ETag(attr.ETag)),
			Overwrite: true,
		})
		if err != nil {
			if bloberror.HasCode(err, bloberror.ConditionNotMet) {
				log.Warn("updateHandleCount :: SetPropertiesInStorage failed due to ETag mismatch, retrying...")

				// Apply exponential backoff
				log.Debug("updateHandleCount :: Retrying in %d seconds...", backoff)
				time.Sleep(time.Duration(backoff) * time.Second)

				// Double the backoff time, but cap it at maxBackoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			} else {
				log.Debug("updateHandleCount :: Failed to update metadata property: %v", err)
				return err
			}
		} else {
			break
		}
	}
	return nil
}

func (m *FileMetaManager) GetFileOpenCount(filename string) (int64, error) {
	prop, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: filename,
	})
	if err != nil {
		log.Debug("GetFileOpenCount :: Failed to get handle count: %v", err)
		return 0, err
	}
	openCount, ok := prop.Metadata["opencount"]
	if !ok {
		log.Debug("GetFileOpenCount :: openCount not found in metadata")
		return 0, err
	}
	count, err := strconv.ParseInt(*openCount, 10, 64)
	if err != nil {
		log.Debug("GetFileOpenCount :: Failed to parse handle count: %v", err)
		return 0, err
	}
	if count < 0 {
		log.Debug("GetHandleCount :: Handle count is negative")
		return 0, err
	}

	return count, nil
}

func (m *FileMetaManager) GetFile(filename string) (*dcache.FileMetadata, error) {
	filePath := filepath.Join(m.cacheDir, filename)
	// Get the file content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: filePath,
	})
	if err != nil {
		log.Debug("GetFile :: Failed to get file content: %v", err)
		return nil, err
	}
	// Unmarshal the JSON data into the MetaFile struct
	var metaFile dcache.FileMetadata
	err = json.Unmarshal(data, &metaFile)
	if err != nil {
		log.Debug("GetFileContent :: Failed to unmarshal JSON data: %v", err)
		return nil, err
	}
	// Return the MetaFile struct
	return &metaFile, nil
}

func (m *FileMetaManager) SetFileSize(filePath string, size int64) error {
	// Get the file content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: filepath.Join(m.cacheDir, filePath),
	})
	if err != nil {
		log.Debug("SetFileSize :: Failed to get file content: %v", err)
		return err
	}
	// Unmarshal the JSON data into the MetaFile struct
	var metaFile dcache.FileMetadata
	err = json.Unmarshal(data, &metaFile)
	if err != nil {
		log.Debug("SetFileSize :: Failed to unmarshal JSON data: %v", err)
		return err
	}
	// Update the size in the MetaFile struct
	metaFile.Size = size
	// Marshal the updated MetaFile struct back to JSON
	jsonData, err := json.MarshalIndent(metaFile, "", "  ")
	if err != nil {
		log.Debug("SetFileSize :: Failed to marshal updated metadata to JSON: %v", err)
		return err
	}
	// Store the updated metadata in storage
	err = m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name: filepath.Join(m.cacheDir, filePath),
		Data: jsonData,
	})
	if err != nil {
		log.Debug("SetFileSize :: Failed to put updated blob in storage: %v", err)
		return err
	}
	return nil
}

func (m *FileMetaManager) GetCacheInternalFile(filePath string) ([]byte, error) {
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: filepath.Join(m.cacheDir, filePath),
	})
	if err != nil {
		log.Debug("GetCacheInternalFile :: Failed to get file content: %v", err)
		return nil, err
	}
	return data, nil
}

func (m *FileMetaManager) SetCacheInternalFile(filePath string, data []byte) error {
	// TODO :: Etag will be required here and possibly the loop mechanism with backoff time
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name: filepath.Join(m.cacheDir, filePath),
		Data: data,
	})
	if err != nil {
		log.Debug("SetCacheInternalFile :: Failed to put updated blob in storage: %v", err)
		return err
	}
	return nil
}
