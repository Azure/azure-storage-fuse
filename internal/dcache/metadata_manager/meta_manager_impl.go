package metadata_manager

import "github.com/Azure/azure-storage-fuse/v2/internal/dcache"

// FileMetaManager is the implementation of MetaManager interface
type FileMetaManager struct {
	cacheDir string
}

// NewMetaManager creates a new implementation of the MetaManager interface
func NewMetaManager(cacheID string) (MetadataManager, error) {
	return &FileMetaManager{
		cacheDir: cacheID,
	}, nil
}

// Implement all interface methods
func (m *FileMetaManager) CreateFile(filename string, filelayout *dcache.FileLayout) (*dcache.FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaManager) CreateCacheInternalFile(filename string, data []byte) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) DeleteFile(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) IncrementFileOpenCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) DecrementFileOpenCount(filename string) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) GetFileOpenCount(filename string) (int64, error) {
	// Implementation here
	return 0, nil
}

func (m *FileMetaManager) GetFile(filename string) (*dcache.FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaManager) SetFileSize(filename string, size int64) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) GetCacheInternalFile(filename string) ([]byte, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaManager) SetCacheInternalFile(filename string, data []byte) error {
	// Implementation here
	return nil
}

// func (m *FileMetaManager) SetBlobMetadata(filename string, metadata map[string]string) error {
// 	// Implementation here
// 	return nil
// }
//
// func (m *FileMetaManager) GetBlobMetadata(filename string) (map[string]string, error) {
// 	// Implementation here
// 	return nil, nil
// }
