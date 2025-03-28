/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

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
	DeleteEmptyDirs(DeleteDirOptions) (bool, error)

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
	CloseFile(CloseFileOptions) error

	RenameFile(RenameFileOptions) error

	ReadFile(ReadFileOptions) ([]byte, error)
	ReadFileWithName(ReadFileWithNameOptions) ([]byte, error)
	ReadInBuffer(ReadInBufferOptions) (int, error)

	WriteFile(WriteFileOptions) (int, error)
	TruncateFile(TruncateFileOptions) error

	CopyToFile(CopyToFileOptions) error
	CopyFromFile(CopyFromFileOptions) error

	SyncDir(SyncDirOptions) error
	SyncFile(SyncFileOptions) error
	FlushFile(FlushFileOptions) error
	ReleaseFile(ReleaseFileOptions) error
	UnlinkFile(UnlinkFileOptions) error // TODO: What does this do? Not used anywhere

	// Symlink operations
	CreateLink(CreateLinkOptions) error
	ReadLink(ReadLinkOptions) (string, error)

	// Filesystem level operations
	//GetAttr: Implementation expectations:
	//1. must return ErrNotExist for absence of a file/directory/symlink
	//2. must return valid nodeID that was passed with any create/update operations for eg: SetAttr, CreateFile, CreateDir etc
	GetAttr(GetAttrOptions) (*ObjAttr, error)
	SetAttr(SetAttrOptions) error

	Chmod(ChmodOptions) error
	Chown(ChownOptions) error
	GetFileBlockOffsets(options GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error)

	FileUsed(name string) error
	StatFs() (*syscall.Statfs_t, bool, error)

	GetCommittedBlockList(string) (*CommittedBlockList, error)
	StageData(StageDataOptions) error
	CommitData(CommitDataOptions) error
	WriteFromBuffer(WriteFromBufferOptions) error
}
