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

package meta_manager

// MetaFile represents the metadata structure for a file
type MetaFile struct {
	Filename        string     `json:"filename"`
	FileID          string     `json:"file_id"`
	Size            int64      `json:"size"`
	ClusterMapEpoch int64      `json:"cluster_map_epoch"`
	FileLayout      FileLayout `json:"file_layout"`
}

type FileLayout struct {
	ChunkSize  int64    `json:"chunk_size"`
	StripeSize int64    `json:"stripe_size"`
	MVList     []string `json:"mv_list"`
}

// MetaManager defines the interface for managing file metadata
type MetaManager interface {
	// CreateMetaFile creates or updates metadata for a file with its associated materialized views
	CreateMetaFile(filename string, filelayout FileLayout) error

	// DeleteMetaFile removes metadata for a file
	DeleteMetaFile(filename string) error

	// IncrementHandleCount increases the handle count for a file
	IncrementHandleCount(filename string) error

	// DecrementHandleCount decreases the handle count for a file
	DecrementHandleCount(filename string) error

	// GetHandleCount returns the current handle count for a file
	GetHandleCount(filename string) (int64, error)

	// GetFileContent reads and returns the content of a file
	GetContent(filename string) ([]byte, error)

	// SetContent sets the content of a file with the specified chunk and stripe sizes
	SetContent(filename string, data []byte) error

	//Optional
	// SetBlobMetadata(filename string, metadata map[string]string) error
	// GetBlobMetadata(filename string) (map[string]string, error)
}
