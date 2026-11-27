// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

import (
	"os"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type CreateDirOptions struct {
	Name string
	Mode os.FileMode
}

type DeleteDirOptions struct {
	Name string
}

type IsDirEmptyOptions struct {
	Name string
}

type OpenDirOptions struct {
	Name string
}

type ReadDirOptions struct {
	Name string
}

type StreamDirOptions struct {
	Name   string
	Offset uint64
	Token  string
	Count  int32
}

type CloseDirOptions struct {
	Name string
}

type RenameDirOptions struct {
	Src string
	Dst string
}

type CreateFileOptions struct {
	Name string
	Mode os.FileMode
}

type DeleteFileOptions struct {
	Name string
}

type OpenFileOptions struct {
	Name  string
	Flags int
	Mode  os.FileMode
}

type ReleaseFileOptions struct {
	Handle *handlemap.Handle
}

type RenameFileOptions struct {
	Src     string
	Dst     string
	SrcAttr *ObjAttr
	DstAttr *ObjAttr
}

type ReadFileOptions struct {
	Handle *handlemap.Handle
}

type ReadInBufferOptions struct {
	Handle *handlemap.Handle
	Offset int64
	Etag   *string
	Data   []byte
	Path   string
	Size   int64
}

type WriteFileOptions struct {
	Handle   *handlemap.Handle
	Offset   int64
	Data     []byte
	Metadata map[string]*string
}

type GetFileBlockOffsetsOptions struct {
	Name string
}

type TruncateFileOptions struct {
	Handle  *handlemap.Handle
	Name    string
	OldSize int64
	NewSize int64
	// This is equivalent to the storage block Size.
	BlockSize int64
}

type CopyToFileOptions struct {
	Name   string
	Offset int64
	Count  int64
	File   *os.File
}

type CopyFromFileOptions struct {
	Name     string
	File     *os.File
	Metadata map[string]*string
}

type FlushFileOptions struct {
	Handle          *handlemap.Handle
	CloseInProgress bool
}

type SyncFileOptions struct {
	Handle *handlemap.Handle
}

type SyncDirOptions struct {
	Name string
}

type UnlinkFileOptions struct {
	Name string
}

type CreateLinkOptions struct {
	Name   string
	Target string
}

type ReadLinkOptions struct {
	Name string
	// Size is the length of the link target as reported by GetAttr and must be set by the caller.
	// Implementations may use it to size the read instead of looking the target up again, so a
	// Size of 0 can yield an empty target.
	Size int64
}

type GetAttrOptions struct {
	Name             string
	RetrieveMetadata bool
}

type ChmodOptions struct {
	Name string
	Mode os.FileMode
}

type ChownOptions struct {
	Name  string
	Owner int
	Group int
}

type StageDataOptions struct {
	Name   string
	Id     string
	Data   []byte
	Offset uint64
}

type CommitDataOptions struct {
	Name      string
	List      []string
	BlockSize uint64
	NewETag   *string
}

type CommittedBlock struct {
	Id     string
	Offset int64
	Size   uint64
}
type CommittedBlockList []CommittedBlock

func TruncateDirName(name string) string {
	if len(name) == 0 {
		return ""
	}
	if name[len(name)-1:] == "/" {
		name = name[:len(name)-1]
	}
	return name
}

func ExtendDirName(name string) string {
	if len(name) == 0 {
		return "/"
	}
	if name[len(name)-1:] != "/" {
		name = name + "/"
	}
	return name
}
