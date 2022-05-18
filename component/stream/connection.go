package stream

import (
	"blobfuse2/internal"
	"blobfuse2/internal/handlemap"
)

type StreamConfig struct {
	blockSize           int64
	bufferSizePerHandle uint64 // maximum number of blocks allowed to be stored for a file
	handleLimit         int32
	openHandles         int32
}

type StreamConnection interface {
	RenameDirectory(options internal.RenameDirOptions) error
	DeleteDirectory(options internal.DeleteDirOptions) error
	RenameFile(options internal.RenameFileOptions) error
	DeleteFile(options internal.DeleteFileOptions) error
	CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) //TODO TEST THIS
	Configure(cfg StreamOptions) error
	ReadInBuffer(internal.ReadInBufferOptions) (int, error)
	OpenFile(internal.OpenFileOptions) (*handlemap.Handle, error)
	WriteFile(options internal.WriteFileOptions) (int, error)
	CloseFile(internal.CloseFileOptions) error
	TruncateFile(internal.TruncateFileOptions) error
	Stop() error
}

// NewAzStorageConnection : Based on account type create respective AzConnection Object
func NewStreamConnection(cfg StreamOptions, stream *Stream) StreamConnection {
	if cfg.readOnly {
		r := ReadCache{}
		r.Stream = stream
		r.Configure(cfg)
		return &r
	}
	rw := ReadWriteCache{}
	rw.Stream = stream
	rw.Configure(cfg)
	return &rw
}
