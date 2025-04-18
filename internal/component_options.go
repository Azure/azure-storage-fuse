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

type CloseFileOptions struct {
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
	Name   string
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
	Handle *handlemap.Handle
	Name   string
	Size   int64
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

type ReleaseFileOptions struct {
	Handle *handlemap.Handle
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
	Size int64
}

type GetAttrOptions struct {
	Name             string
	RetrieveMetadata bool
}

type SetAttrOptions struct {
	Name string
	Attr *ObjAttr
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
	Ctx    context.Context
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
