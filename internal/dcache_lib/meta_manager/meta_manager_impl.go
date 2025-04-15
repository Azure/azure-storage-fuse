package meta_manager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"
	"github.com/google/uuid"
)

// FileMetaManager is the implementation of MetaManager interface
type FileMetaManager struct {
	cacheDir        string
	storageCallback dcachelib.StorageCallbacks
}

// NewMetaManager creates a new implementation of the MetaManager interface
func NewMetaManager(cacheID string, storageCallback dcachelib.StorageCallbacks) (MetaManager, error) {
	cacheDir := filepath.Join("__CACHE__"+cacheID, "Objects")
	return &FileMetaManager{
		cacheDir:        cacheDir,
		storageCallback: storageCallback,
	}, nil
}

// Implement all interface methods
func (m *FileMetaManager) CreateMetaFile(filename string, mvList []string) error {
	guid := uuid.New()
	// Create the metadata structure
	metaData := MetaFile{
		Filename:        filename,
		FileID:          guid,
		Size:            23,
		ClusterMapEpoch: 23,
		MVList:          mvList,
	}

	// Convert the metadata to JSON
	jsonData, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return fmt.Errorf("CreateMetaFile :: Failed to marshal metadata to JSON: %w", err)
	}
	// Store the open-count in the metadata blob property
	openCount := "0"
	metadata := map[string]*string{
		"openCount": &openCount,
	}
	filename = filename + ".md"

	err = m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(m.cacheDir, filename),
		Metadata:               metadata,
		Data:                   jsonData,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	if err != nil {
		return fmt.Errorf("CreateMetaFile :: Failed to put blob in storage: %w", err)
	}
	return nil
}

func (m *FileMetaManager) DeleteMetaFile(filename string) error {
	filePath := filepath.Join(m.cacheDir, filename+".md")
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: filePath,
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteMetaFile :: DeleteBlobInStorage failed since blob is already deleted")
			return nil
		}
	}
	return err
}

func (m *FileMetaManager) IncrementHandleCount(filename string) error {
	err := m.updateHandleCount(filename, true)
	if err != nil {
		return fmt.Errorf("IncrementHandleCount :: Failed to update handle count: %w", err)
	}
	return nil
}

func (m *FileMetaManager) DecrementHandleCount(filename string) error {
	err := m.updateHandleCount(filename, false)
	if err != nil {
		return fmt.Errorf("DecrementHandleCount :: Failed to update handle count: %w", err)
	}
	return nil
}

func (m *FileMetaManager) updateHandleCount(filename string, increment bool) error {
	filePath := filepath.Join(m.cacheDir, filename+".md")
	for {
		// Get the current handle count
		attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
			Name: filename,
		})
		if err != nil {
			return fmt.Errorf("GetHandleCount :: Failed to get handle count: %w", err)
		}

		newAttr := &internal.ObjAttr{
			ETag: attr.ETag,
			Metadata: func() map[string]*string {
				openCountStr := "0"
				if attr.Metadata["opencount"] != nil {
					openCount, err := strconv.Atoi(*attr.Metadata["opencount"])
					if err == nil {
						if increment {
							openCountStr = strconv.Itoa(openCount + 1)
						} else {
							openCountStr = strconv.Itoa(openCount - 1)
						}
					}
				}
				return map[string]*string{
					"opencount": &openCountStr,
				}
			}(),
		}

		// Set the new metadata in storage
		err = m.storageCallback.SetPropertiesInStorage(internal.SetAttrOptions{
			Name: filePath,
			Attr: newAttr,
		})
		if err != nil {
			if bloberror.HasCode(err, bloberror.ConditionNotMet) {
				log.Warn("updateHandleCount :: SetPropertiesInStorage failed due to ETag mismatch, retrying...")
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

func (m *FileMetaManager) GetHandleCount(filename string) (int64, error) {
	filePath := filepath.Join(m.cacheDir, filename+".md")
	count, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: filePath,
	})
	if err != nil {
		return 0, fmt.Errorf("GetHandleCount :: Failed to get handle count: %w", err)
	}
	openCount, ok := count.Metadata["openCount"]
	if !ok {
		return 0, fmt.Errorf("GetHandleCount :: openCount not found in metadata")
	}
	handleCount, err := strconv.ParseInt(*openCount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("GetHandleCount :: Failed to parse handle count: %w", err)
	}
	if handleCount < 0 {
		return 0, fmt.Errorf("GetHandleCount :: Handle count is negative")
	}

	return handleCount, nil
}

func (m *FileMetaManager) GetFileContent(filename string) (*MetaFile, error) {
	filePath := filepath.Join(m.cacheDir, filename+".md")
	// Get the file content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("GetFileContent :: Failed to get file content: %w", err)
	}
	// Unmarshal the JSON data into the MetaFile struct
	var metaFile MetaFile
	err = json.Unmarshal(data, &metaFile)
	if err != nil {
		return nil, fmt.Errorf("GetFileContent :: Failed to unmarshal JSON data: %w", err)
	}
	// Return the MetaFile struct
	return &metaFile, nil
}
