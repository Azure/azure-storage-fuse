/*
    _____              _____   _____    _____             _____    _____
   |       |  |       |       |        |       |     |  |         |
   |       |  |       |       |        |       |     |  |         |
   |----   |  |       |----   |-----|  | ----  |     |  |------|  |----
   |       |  |       |             |  |       |     |         |  |
   |       |  |_____  |_____   _____|  |       |_____|   ______|  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package azstorage

import (
	"blobfuse2/common"
	"blobfuse2/internal"
	"os"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
)

type FileShare struct { // unsure of struct properties
	AzStorageConnection
	Auth       azAuth
	Service    azbfs.ServiceURL
	Filesystem azbfs.FileSystemURL
	BlockBlob  BlockBlob
}

func (fs *FileShare) Configure(cfg AzStorageConfig) error
func (fs *FileShare) UpdateConfig(cfg AzStorageConfig) error

func (fs *FileShare) SetupPipeline() error
func (fs *FileShare) TestPipeline() error

func (fs *FileShare) ListContainers() ([]string, error)

// This is just for test, shall not be used otherwise
func (fs *FileShare) SetPrefixPath(string) error

func (fs *FileShare) Exists(name string) bool
func (fs *FileShare) CreateFile(name string, mode os.FileMode) error
func (fs *FileShare) CreateDirectory(name string) error
func (fs *FileShare) CreateLink(source string, target string) error

func (fs *FileShare) DeleteFile(name string) error
func (fs *FileShare) DeleteDirectory(name string) error

func (fs *FileShare) RenameFile(string, string) error
func (fs *FileShare) RenameDirectory(string, string) error

func (fs *FileShare) GetAttr(name string) (attr *internal.ObjAttr, err error)

// Standard operations to be supported by any account type
func (fs *FileShare) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error)

func (fs *FileShare) ReadToFile(name string, offset int64, count int64, fi *os.File) error
func (fs *FileShare) ReadBuffer(name string, offset int64, len int64) ([]byte, error)
func (fs *FileShare) ReadInBuffer(name string, offset int64, len int64, data []byte) error

func (fs *FileShare) WriteFromFile(name string, metadata map[string]string, fi *os.File) error
func (fs *FileShare) WriteFromBuffer(name string, metadata map[string]string, data []byte) error
func (fs *FileShare) Write(options internal.WriteFileOptions) error
func (fs *FileShare) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error)

func (fs *FileShare) ChangeMod(string, os.FileMode) error
func (fs *FileShare) ChangeOwner(string, int, int) error
func (fs *FileShare) TruncateFile(string, int64) error
func (fs *FileShare) StageAndCommit(name string, bol *common.BlockOffsetList) error

func (fs *FileShare) NewCredentialKey(_, _ string) error
