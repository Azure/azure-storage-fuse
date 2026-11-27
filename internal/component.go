// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

import (
	"context"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type ComponentPriority int

var EComponentPriority = ComponentPriority(0).LevelMid()

func (ComponentPriority) LevelMid() ComponentPriority {
	return ComponentPriority(500)
}

func (ComponentPriority) Producer() ComponentPriority {
	return ComponentPriority(1000)
}

func (ComponentPriority) Consumer() ComponentPriority {
	return ComponentPriority(100)
}

func (ComponentPriority) LevelOne() ComponentPriority {
	return ComponentPriority(700)
}

func (ComponentPriority) LevelTwo() ComponentPriority {
	return ComponentPriority(300)
}

// Component : Base internal for every component to participate in pipeline
type Component interface {
	// Pipeline participation related methods
	Name() string
	SetName(string)
	Configure(bool) error
	GenConfig() string
	Priority() ComponentPriority

	SetNextComponent(c Component)
	NextComponent() Component

	Start(context.Context) error
	Stop() error

	// Directory operations
	CreateDir(CreateDirOptions) error
	DeleteDir(DeleteDirOptions) error
	IsDirEmpty(IsDirEmptyOptions) bool

	OpenDir(OpenDirOptions) error
	//ReadDir: implementation expectations
	//must return ErrNotExist for absence of the requested directory
	ReadDir(ReadDirOptions) ([]*ObjAttr, error)
	StreamDir(StreamDirOptions) ([]*ObjAttr, string, error)

	CloseDir(CloseDirOptions) error

	RenameDir(RenameDirOptions) error

	// File operations
	//CreateFile Implementation expectations
	//1. must return ErrExist if file already exists
	CreateFile(CreateFileOptions) (*handlemap.Handle, error)
	DeleteFile(DeleteFileOptions) error

	OpenFile(OpenFileOptions) (*handlemap.Handle, error)
	ReadFile(ReadFileOptions) ([]byte, error)
	ReadInBuffer(*ReadInBufferOptions) (int, error)
	WriteFile(*WriteFileOptions) (int, error)

	SyncFile(SyncFileOptions) error
	// Flush is called on each close() of a file descriptor, as opposed to release which is called on the close of the
	// last file descriptor for a file.
	//
	// NOTE: The flush() method may be called more than once for each open().  This happens if more than one file
	//       descriptor refers to an open file handle, e.g. due to dup(), dup2() or fork() calls.  It is not possible to
	//       determine if a flush is final, so each flush should be treated equally. Multiple write-flush sequences are
	//       relatively rare, so this shouldn't be a problem.
	FlushFile(FlushFileOptions) error
	// Release is called when there are no more references to an open file: all file descriptors are closed for this
	// handle.
	ReleaseFile(ReleaseFileOptions) error

	RenameFile(RenameFileOptions) error

	CopyToFile(CopyToFileOptions) error
	CopyFromFile(CopyFromFileOptions) error

	SyncDir(SyncDirOptions) error
	UnlinkFile(UnlinkFileOptions) error // TODO: What does this do? Not used anywhere

	// Symlink operations
	CreateLink(CreateLinkOptions) error
	ReadLink(ReadLinkOptions) (string, error)

	// Filesystem level operations
	//GetAttr: Implementation expectations:
	//1. must return ErrNotExist for absence of a file/directory/symlink
	//2. must return valid nodeID that was passed with any create/update operations for eg: SetAttr, CreateFile, CreateDir etc
	GetAttr(GetAttrOptions) (*ObjAttr, error)

	// SetAttr is implemented by the following functions in libfuse High level API.
	Chmod(ChmodOptions) error
	Chown(ChownOptions) error
	TruncateFile(TruncateFileOptions) error

	GetFileBlockOffsets(options GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error)

	FileUsed(name string) error
	StatFs() (*syscall.Statfs_t, bool, error)

	GetCommittedBlockList(string) (*CommittedBlockList, error)
	StageData(StageDataOptions) error
	CommitData(CommitDataOptions) error
}
