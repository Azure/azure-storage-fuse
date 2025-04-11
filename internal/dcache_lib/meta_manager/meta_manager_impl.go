package meta_manager

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"
	"github.com/google/uuid"
)

// FileMetaManager is the implementation of MetaManager interface
type FileMetaManager struct {
	cacheID         string
	storageCallback dcachelib.StorageCallbacks
}

// NewMetaManager creates a new implementation of the MetaManager interface
func NewMetaManager(cacheID string, storageCallback dcachelib.StorageCallbacks) (MetaManager, error) {
	return &FileMetaManager{
		cacheID:         cacheID,
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
	cacheDir := "__CACHE__" + m.cacheID

	err = m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(cacheDir, filename),
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
	// Implementation here
	return nil
}

func (m *FileMetaManager) IncrementHandleCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) DecrementHandleCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) GetHandleCount(filename string) (int64, error) {
	// Implementation here
	return 0, nil
}

func (m *FileMetaManager) GetFileContent(filename string) ([]byte, error) {
	// Implementation here
	return nil, nil
}
