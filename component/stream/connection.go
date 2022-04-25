package stream

type StreamConfig struct {
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
}

type StreamConnection interface {
	Configure(cfg StreamConfig) error
	// CreateFile(name string, mode os.FileMode) error
	// CreateDirectory(name string) error
	// CreateLink(source string, target string) error

	// DeleteFile(name string) error
	// DeleteDirectory(name string) error

	// RenameFile(string, string) error
	// RenameDirectory(string, string) error

	// GetAttr(name string) (attr *internal.ObjAttr, err error)

	// // Standard operations to be supported by any account type
	// List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

	// ReadToFile(name string, offset int64, count int64, fi *os.File) error
	// ReadBuffer(name string, offset int64, len int64) ([]byte, error)
	// ReadInBuffer(name string, offset int64, len int64, data []byte) error

	// WriteFromFile(name string, metadata map[string]string, fi *os.File) error
	// WriteFromBuffer(name string, metadata map[string]string, data []byte) error
	// Write(options internal.WriteFileOptions) error
	// GetFileBlockOffsets(name string) (*common.BlockOffsetList, error)

	// ChangeMod(string, os.FileMode) error
	// ChangeOwner(string, int, int) error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewStreamConnection(cfg StreamOptions) StreamConnection {
	if cfg.readOnly {
		return &ReadCache{}
	}
	return &ReadWriteCache{}
}
