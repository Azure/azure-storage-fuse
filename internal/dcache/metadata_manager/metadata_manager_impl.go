package metadata_manager

import "github.com/Azure/azure-storage-fuse/v2/internal/dcache"

// FileMetaDataManager is the implementation of MetaDataManager interface
type FileMetaDataManager struct {
	cacheDir string
}

// NewMetaDataManager creates a new implementation of the MetaDataManager interface
func NewMetaDataManager(cacheID string) (MetadataManager, error) {
	return &FileMetaDataManager{
		cacheDir: cacheID,
	}, nil
}

// Implement all interface methods
func (m *FileMetaDataManager) CreateFile(filename string, filelayout *dcache.FileLayout) (*dcache.FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaDataManager) CreateCacheInternalFile(filename string, data []byte) error {
	// Implementation here
	return nil
}

func (m *FileMetaDataManager) DeleteFile(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaDataManager) IncrementFileOpenCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaDataManager) DecrementFileOpenCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaDataManager) GetFileOpenCount(filename string) (int64, error) {
	// Implementation here
	return 0, nil
}

func (m *FileMetaDataManager) GetFile(filename string) (*dcache.FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaDataManager) SetFileSize(filename string, size int64) error {
	// Implementation here
	return nil
}

func (m *FileMetaDataManager) GetCacheInternalFile(filename string) ([]byte, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaDataManager) SetCacheInternalFile(filename string, data []byte) error {
	// Implementation here
	return nil
}

// func (m *FileMetaDataManager) SetBlobMetadata(filename string, metadata map[string]string) error {
// 	// Implementation here
// 	return nil
// }
//
// func (m *FileMetaDataManager) GetBlobMetadata(filename string) (map[string]string, error) {
// 	// Implementation here
// 	return nil, nil
// }
