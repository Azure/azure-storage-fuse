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

package exported

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Package exported is a wrapper around internal package to expose the internal attributes for writing custom components.
const (
	PropFlagUnknown uint16 = iota
	PropFlagNotExists
	PropFlagIsDir
	PropFlagEmptyDir
	PropFlagSymlink
	PropFlagMetadataRetrieved
	PropFlagModeDefault
	PropFlagOwnerInfoFound
	PropFlagGroupInfoFound
)

// Type aliases for base component
type BaseComponent = internal.BaseComponent

// Type aliases for component
type Component = internal.Component

type ComponentPriority = internal.ComponentPriority

// Type aliases for attributes
type ObjAttr = internal.ObjAttr

// Type aliases for component options
type CreateDirOptions = internal.CreateDirOptions
type DeleteDirOptions = internal.DeleteDirOptions
type IsDirEmptyOptions = internal.IsDirEmptyOptions
type OpenDirOptions = internal.OpenDirOptions
type ReadDirOptions = internal.ReadDirOptions
type StreamDirOptions = internal.StreamDirOptions
type CloseDirOptions = internal.CloseDirOptions
type RenameDirOptions = internal.RenameDirOptions
type CreateFileOptions = internal.CreateFileOptions
type DeleteFileOptions = internal.DeleteFileOptions
type OpenFileOptions = internal.OpenFileOptions
type CloseFileOptions = internal.CloseFileOptions
type RenameFileOptions = internal.RenameFileOptions
type ReadFileOptions = internal.ReadFileOptions
type ReadInBufferOptions = internal.ReadInBufferOptions
type WriteFileOptions = internal.WriteFileOptions
type GetFileBlockOffsetsOptions = internal.GetFileBlockOffsetsOptions
type TruncateFileOptions = internal.TruncateFileOptions
type CopyToFileOptions = internal.CopyToFileOptions
type CopyFromFileOptions = internal.CopyFromFileOptions
type FlushFileOptions = internal.FlushFileOptions
type SyncFileOptions = internal.SyncFileOptions
type SyncDirOptions = internal.SyncDirOptions
type ReleaseFileOptions = internal.ReleaseFileOptions
type UnlinkFileOptions = internal.UnlinkFileOptions
type CreateLinkOptions = internal.CreateLinkOptions
type ReadLinkOptions = internal.ReadLinkOptions
type GetAttrOptions = internal.GetAttrOptions
type SetAttrOptions = internal.SetAttrOptions
type ChmodOptions = internal.ChmodOptions
type ChownOptions = internal.ChownOptions
type StageDataOptions = internal.StageDataOptions
type CommitDataOptions = internal.CommitDataOptions
type CommittedBlock = internal.CommittedBlock
type CommittedBlockList = internal.CommittedBlockList

// Type aliases for pipeline
type Handle = handlemap.Handle

// Wrapper function
func NewHandle(path string) *Handle {
	return handlemap.NewHandle(path)
}

type ComponentPriorityWrapper struct {
	internal.ComponentPriority
}

// Wrapper functions to expose ComponentPriority methods
func (ComponentPriorityWrapper) LevelMid() ComponentPriority {
	return internal.ComponentPriority(0).LevelMid()
}

func (ComponentPriorityWrapper) Producer() ComponentPriority {
	return internal.ComponentPriority(0).Producer()
}

func (ComponentPriorityWrapper) Consumer() ComponentPriority {
	return internal.ComponentPriority(0).Consumer()
}

func (ComponentPriorityWrapper) LevelOne() ComponentPriority {
	return internal.ComponentPriority(0).LevelOne()
}

func (ComponentPriorityWrapper) LevelTwo() ComponentPriority {
	return internal.ComponentPriority(0).LevelTwo()
}

// wrapper utility functions to expose internal functions
func TruncateDirName(name string) string {
	return internal.TruncateDirName(name)
}

func ExtendDirName(name string) string {
	return internal.ExtendDirName(name)
}
