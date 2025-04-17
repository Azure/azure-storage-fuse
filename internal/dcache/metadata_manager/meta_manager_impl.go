package metadata_manager

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
func (m *FileMetaManager) CreateFile(filename string, filelayout *FileLayout) (*FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaManager) DeleteFile(filename string) error {
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

func (m *FileMetaManager) GetFileOpenCount(filename string) (int64, error) {
	// Implementation here
	return 0, nil
}

func (m *FileMetaManager) GetFile(filename string) (*FileMetadata, error) {
	// Implementation here
	return nil, nil
}

func (m *FileMetaManager) SetFile(filename string, data []byte) error {
	// Implementation here
	return nil
}

func (m *FileMetaManager) CreateCacheInternalFile(filename string, data []byte) error {
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
