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

package metadata_manager

import (
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

// MetaManager defines the interface for managing file metadata
type MetadataManager interface {

	// Following APIs are used to manage metadata for files in the distributed cache
	// CreateFile creates or updates metadata for a file with its associated materialized views
	CreateFile(filePath string, filelayout *dcache.FileLayout) (*dcache.FileMetadata, error)

	// DeleteFile removes metadata for a file
	DeleteFile(filePath string) error

	// IncrementHandleCount increases the handle count for a file
	IncrementFileOpenCount(filePath string) error

	// DecrementHandleCount decreases the handle count for a file
	DecrementFileOpenCount(filePath string) error

	// GetHandleCount returns the current handle count for a file
	GetFileOpenCount(filePath string) (int64, error)

	// GetFileContent reads and returns the content of a file
	GetFile(filePath string) (*dcache.FileMetadata, error)

	SetFileSize(filePath string, size int64) error

	// Following APIs are used to manage internal files in the distributed cache
	// CreateCacheInternalFile creates file for cluster map and heartbeat
	CreateInternalFile(filePath string, data []byte) error

	GetInternalFile(filePath string) ([]byte, error)

	SetInternalFile(filePath string, data []byte) error

	//Optional
	// SetBlobMetadata(filename string, metadata map[string]string) error
	// GetBlobMetadata(filename string) (map[string]string, error)
}
