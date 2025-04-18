package metadata_manager

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

// BlobMetadataManager is the implementation of MetadataManager interface
type BlobMetadataManager struct {
	cacheDir string
}

// NewMetadataManager creates a new implementation of the MetadataManager interface
func NewMetadataManager(cacheDir string) (MetadataManager, error) {
	return &BlobMetadataManager{
		cacheDir: cacheDir,
	}, nil
}

// CreateFileInit creates the initial metadata for a file
func (m *BlobMetadataManager) CreateFileInit(filePath string, fileMetadata *dcache.FileMetadata) error {
	// Dummy implementation
	return nil
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) CreateFileFinalize(filePath string, fileMetadata *dcache.FileMetadata) error {
	// Dummy implementation
	return nil
}

// GetFile reads and returns the content of metadata for a file
func (m *BlobMetadataManager) GetFile(filePath string) (*dcache.FileMetadata, error) {
	// Dummy implementation
	return nil, nil
}

// DeleteFile removes metadata for a file
func (m *BlobMetadataManager) DeleteFile(filePath string) error {
	// Dummy implementation
	return nil
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) OpenFile(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) CloseFile(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// GetFileOpenCount returns the current open count for a file
func (m *BlobMetadataManager) GetFileOpenCount(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// UpdateHeartbeat creates or updates the heartbeat file
func (m *BlobMetadataManager) UpdateHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) DeleteHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// GetHeartbeat reads and returns the content of the heartbeat file
func (m *BlobMetadataManager) GetHeartbeat(nodeId string) ([]byte, error) {
	// Dummy implementation
	return nil, nil
}

// GetAllNodes enumerates and returns the list of all nodes with a heartbeat
func (m *BlobMetadataManager) GetAllNodes() ([]string, error) {
	// Dummy implementation
	return nil, nil
}

// CreateInitialClusterMap creates the initial cluster map
func (m *BlobMetadataManager) CreateInitialClusterMap(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapStart claims update ownership of the cluster map
func (m *BlobMetadataManager) UpdateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapEnd finalizes the cluster map update
func (m *BlobMetadataManager) UpdateClusterMapEnd(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// GetClusterMap reads and returns the content of the cluster map
func (m *BlobMetadataManager) GetClusterMap() ([]byte, *azcore.ETag, error) {
	// Dummy implementation
	return nil, nil, nil
}
