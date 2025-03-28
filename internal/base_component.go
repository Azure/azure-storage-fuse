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

// BaseComponent : Base implementation of the component interface
type BaseComponent struct {
	compName string
	next     Component
}

var _ Component = &BaseComponent{}

////////////////////////////////////////
//	Default Component Implementation

// Pipeline participation related methods
func (base *BaseComponent) Name() string {
	return base.compName
}

func (base *BaseComponent) SetName(name string) {
	base.compName = name
}

func (base *BaseComponent) Configure(isParent bool) error {
	return nil
}

func (base *BaseComponent) GenConfig() string {
	return ""
}

func (base *BaseComponent) Priority() ComponentPriority {
	return EComponentPriority.LevelMid()
}

func (base *BaseComponent) SetNextComponent(c Component) {
	if base.next == nil {
		base.next = c
	} else {
		panic("base.next not implemented")
	}
}

func (base *BaseComponent) NextComponent() Component {
	return base.next
}

func (base *BaseComponent) Start(ctx context.Context) error {
	return nil
}

func (base *BaseComponent) Stop() error {
	return nil
}

// Directory operations
func (base *BaseComponent) CreateDir(options CreateDirOptions) error {
	if base.next != nil {
		return base.next.CreateDir(options)
	}
	return nil
}

func (base *BaseComponent) DeleteDir(options DeleteDirOptions) error {
	if base.next != nil {
		return base.next.DeleteDir(options)
	}
	return nil
}

func (base *BaseComponent) IsDirEmpty(options IsDirEmptyOptions) bool {
	if base.next != nil {
		return base.next.IsDirEmpty(options)
	}
	return false
}

func (base *BaseComponent) DeleteEmptyDirs(options DeleteDirOptions) (bool, error) {
	if base.next != nil {
		return base.next.DeleteEmptyDirs(options)
	}
	return false, nil
}

func (base *BaseComponent) OpenDir(options OpenDirOptions) error {
	if base.next != nil {
		return base.next.OpenDir(options)
	}
	return nil
}

func (base *BaseComponent) ReadDir(options ReadDirOptions) (attr []*ObjAttr, err error) {
	if base.next != nil {
		return base.next.ReadDir(options)
	}
	return attr, err
}

func (base *BaseComponent) StreamDir(options StreamDirOptions) ([]*ObjAttr, string, error) {
	if base.next != nil {
		return base.next.StreamDir(options)
	}
	return nil, "", nil
}

func (base *BaseComponent) CloseDir(options CloseDirOptions) error {
	if base.next != nil {
		return base.next.CloseDir(options)
	}
	return nil
}

func (base *BaseComponent) RenameDir(options RenameDirOptions) error {
	if base.next != nil {
		return base.next.RenameDir(options)
	}
	return nil
}

// File operations
func (base *BaseComponent) CreateFile(options CreateFileOptions) (*handlemap.Handle, error) {
	if base.next != nil {
		return base.next.CreateFile(options)
	}
	return nil, nil
}

func (base *BaseComponent) DeleteFile(options DeleteFileOptions) error {
	if base.next != nil {
		return base.next.DeleteFile(options)
	}
	return nil
}

func (base *BaseComponent) OpenFile(options OpenFileOptions) (*handlemap.Handle, error) {
	if base.next != nil {
		return base.next.OpenFile(options)
	}
	return nil, nil
}

func (base *BaseComponent) CloseFile(options CloseFileOptions) error {
	if base.next != nil {
		return base.next.CloseFile(options)
	}
	return nil
}

func (base *BaseComponent) RenameFile(options RenameFileOptions) error {
	if base.next != nil {
		return base.next.RenameFile(options)
	}
	return nil
}

func (base *BaseComponent) ReadFile(options ReadFileOptions) (b []byte, err error) {
	if base.next != nil {
		return base.next.ReadFile(options)
	}
	return b, err
}

func (base *BaseComponent) ReadFileWithName(options ReadFileWithNameOptions) (b []byte, err error) {
	if base.next != nil {
		return base.next.ReadFileWithName(options)
	}
	return b, err
}

func (base *BaseComponent) ReadInBuffer(options ReadInBufferOptions) (int, error) {
	if base.next != nil {
		return base.next.ReadInBuffer(options)
	}
	return 0, nil
}

func (base *BaseComponent) WriteFile(options WriteFileOptions) (int, error) {
	if base.next != nil {
		return base.next.WriteFile(options)
	}
	return 0, nil
}

func (base *BaseComponent) TruncateFile(options TruncateFileOptions) error {
	if base.next != nil {
		return base.next.TruncateFile(options)
	}
	return nil
}

func (base *BaseComponent) CopyToFile(options CopyToFileOptions) error {
	if base.next != nil {
		return base.next.CopyToFile(options)
	}
	return nil
}

func (base *BaseComponent) CopyFromFile(options CopyFromFileOptions) error {
	if base.next != nil {
		return base.next.CopyFromFile(options)
	}
	return nil
}

func (base *BaseComponent) SyncFile(options SyncFileOptions) error {
	if base.next != nil {
		return base.next.SyncFile(options)
	}
	return nil
}

func (base *BaseComponent) SyncDir(options SyncDirOptions) error {
	if base.next != nil {
		return base.next.SyncDir(options)
	}
	return nil
}

func (base *BaseComponent) FlushFile(options FlushFileOptions) error {
	if base.next != nil {
		return base.next.FlushFile(options)
	}
	return nil
}

func (base *BaseComponent) ReleaseFile(options ReleaseFileOptions) error {
	if base.next != nil {
		return base.next.ReleaseFile(options)
	}
	return nil
}

func (base *BaseComponent) UnlinkFile(options UnlinkFileOptions) error {
	if base.next != nil {
		return base.next.UnlinkFile(options)
	}
	return nil
}

// Symlink operations
func (base *BaseComponent) CreateLink(options CreateLinkOptions) error {
	if base.next != nil {
		return base.next.CreateLink(options)
	}
	return nil
}

func (base *BaseComponent) ReadLink(options ReadLinkOptions) (string, error) {
	if base.next != nil {
		return base.next.ReadLink(options)
	}
	return "", nil
}

// Filesystem level operations
func (base *BaseComponent) GetAttr(options GetAttrOptions) (*ObjAttr, error) {
	if base.next != nil {
		return base.next.GetAttr(options)
	}
	return &ObjAttr{}, nil
}

func (base *BaseComponent) GetFileBlockOffsets(options GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	if base.next != nil {
		return base.next.GetFileBlockOffsets(options)
	}
	return &common.BlockOffsetList{}, nil
}

func (base *BaseComponent) SetAttr(options SetAttrOptions) error {
	if base.next != nil {
		return base.next.SetAttr(options)
	}
	return nil
}

func (base *BaseComponent) Chmod(options ChmodOptions) error {
	if base.next != nil {
		return base.next.Chmod(options)
	}
	return nil
}

func (base *BaseComponent) Chown(options ChownOptions) error {
	if base.next != nil {
		return base.next.Chown(options)
	}
	return nil
}

func (base *BaseComponent) FileUsed(name string) error {
	if base.next != nil {
		return base.next.FileUsed(name)
	}
	return nil
}

func (base *BaseComponent) StatFs() (*syscall.Statfs_t, bool, error) {
	if base.next != nil {
		return base.next.StatFs()
	}
	return nil, false, nil
}

func (base *BaseComponent) GetCommittedBlockList(name string) (*CommittedBlockList, error) {
	if base.next != nil {
		return base.next.GetCommittedBlockList(name)
	}

	return nil, nil
}

func (base *BaseComponent) StageData(opt StageDataOptions) error {
	if base.next != nil {
		return base.next.StageData(opt)
	}
	return nil
}

func (base *BaseComponent) CommitData(opt CommitDataOptions) error {
	if base.next != nil {
		return base.next.CommitData(opt)
	}
	return nil
}

func (base *BaseComponent) WriteFromBuffer(opt WriteFromBufferOptions) error {
	if base.next != nil {
		return base.next.WriteFromBuffer(opt)
	}
	return nil
}
