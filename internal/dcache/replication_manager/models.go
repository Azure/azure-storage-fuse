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

package replication_manager

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type ReadMvRequest struct {
	FileID string // unique guid of the file stored in metadata blob
	MvName string // name of the MV which has this chunk. For example, mv0, mv1, etc.

	// ChunkIndex is the name of the chunk to be read. It should be multiple of chunk size.
	// For example, if chunk size is 4MB, then chunk index should be 0, 4, 8, etc.
	// The chunks are stored in RV as,
	// <mvName>/<fileID>.<chunkIndex>.data and
	// <mvName>/<fileID>.<chunkIndex>.hash
	ChunkIndex    int64
	OffsetInChunk int64 // read offset within the chunk. This should not be greater than chunk size
	Length        int64 // Length in bytes of data to be read. If length is -1, then read till the end of the chunk from offsetInChunk
	ChunkSizeInMB int64 // Chunk size in MB; 4MB, 8MB, etc.

	Data []byte // buffer to store the data read from the chunk
}

// helper method which can be used for logging the request contents except the data buffer
// Use this instead of %+v to avoid printing the data buffer
func (req *ReadMvRequest) toString() string {
	return fmt.Sprintf("FileID: %s, MvName: %s, ChunkIndex: %d, OffsetInChunk: %d, Length: %d, ChunkSizeInMB: %d, Data buffer size: %d",
		req.FileID, req.MvName, req.ChunkIndex, req.OffsetInChunk, req.Length, req.ChunkSizeInMB, len(req.Data))
}

// check if the request is valid
func (req *ReadMvRequest) isValid() error {
	reqStr := req.toString()

	if req.FileID == "" || req.MvName == "" {
		log.Err("ReadMvRequest::isValid: FileID or MvName is empty in request: %s", reqStr)
		return fmt.Errorf("FileID or MvName is empty in request: %s", reqStr)
	}

	if req.ChunkSizeInMB <= 0 {
		log.Err("ReadMvRequest::isValid: ChunkSizeInMB is invalid in request: %s", reqStr)
		return fmt.Errorf("ChunkSizeInMB is invalid in request: %s", reqStr)
	}

	if req.ChunkIndex < 0 || req.ChunkIndex%req.ChunkSizeInMB != 0 {
		log.Err("ReadMvRequest::isValid: ChunkIndex is invalid in request: %s", reqStr)
		return fmt.Errorf("ChunkIndex is invalid in request: %s", reqStr)
	}

	if req.OffsetInChunk < 0 || req.OffsetInChunk >= req.ChunkSizeInMB*common.MbToBytes {
		log.Err("ReadMvRequest::isValid: OffsetInChunk is invalid in request: %s", reqStr)
		return fmt.Errorf("OffsetInChunk is invalid in request: %s", reqStr)
	}

	if req.Length < -1 || req.Length == 0 || req.Length > req.ChunkSizeInMB*common.MbToBytes {
		log.Err("ReadMvRequest::isValid: Length is invalid in request: %s", reqStr)
		return fmt.Errorf("length is invalid in request: %s", reqStr)
	}

	// check if the requested data is not overlapping between two chunks
	// For example, if chunk size is 4MB and offset is 3MB, then length should be less than 1MB
	if req.Length > 0 && req.OffsetInChunk+req.Length > req.ChunkSizeInMB*common.MbToBytes {
		log.Err("ReadMvRequest::isValid: Length and OffsetInChunk are invalid in request: %s", reqStr)
		return fmt.Errorf("length and OffsetInChunk are invalid in request: %s", reqStr)
	}

	if len(req.Data) == 0 || len(req.Data) > int(req.ChunkSizeInMB*common.MbToBytes) {
		log.Err("ReadMvRequest::isValid: Data buffer is invalid in request: %s", reqStr)
		return fmt.Errorf("data buffer is invalid in request: %s", reqStr)
	}

	requestedDataSize := req.Length
	if requestedDataSize == -1 {
		requestedDataSize = req.ChunkSizeInMB*common.MbToBytes - req.OffsetInChunk
	}

	// check if the requested data size is less than the buffer size
	if len(req.Data) < int(requestedDataSize) {
		log.Err("ReadMvRequest::isValid: Data buffer size is less than requested data size in request: %s", reqStr)
		return fmt.Errorf("data buffer size is less than requested data size in request: %s", reqStr)
	}

	return nil
}

type ReadMvResponse struct {
	BytesRead int64 // Number of bytes read
}

type WriteMvRequest struct {
	FileID string // unique guid of the file stored in metadata blob
	MvName string // name of the MV where this chunk will be written. For example, mv0, mv1, etc.

	// ChunkIndex is the name of the chunk to be written. It should be multiple of chunk size.
	// For example, if chunk size is 4MB, then chunk index should be 0, 4, 8, etc.
	// The chunks are stored in RV as,
	// <mvName>/<fileID>.<chunkIndex>.data and
	// <mvName>/<fileID>.<chunkIndex>.hash
	ChunkIndex    int64
	Data          []byte // Data to be written
	ChunkSizeInMB int64  // Chunk size in MB; 4MB, 8MB, etc.
	IsLastChunk   bool   // true if this is the last chunk
}

// helper method which can be used for logging the request contents except the data buffer
// Use this instead of %+v to avoid printing the data buffer
func (req *WriteMvRequest) toString() string {
	return fmt.Sprintf("FileID: %s, MvName: %s, ChunkIndex: %d, ChunkSizeInMB: %d, IsLastChunk: %v, Data buffer size: %d",
		req.FileID, req.MvName, req.ChunkIndex, req.ChunkSizeInMB, req.IsLastChunk, len(req.Data))
}

// check if the request is valid
func (req *WriteMvRequest) isValid() error {
	reqStr := req.toString()

	if req.FileID == "" || req.MvName == "" {
		log.Err("WriteMvRequest::isValid: FileID or MvName is empty in request: %s", reqStr)
		return fmt.Errorf("FileID or MvName is empty in request: %s", reqStr)
	}

	if req.ChunkSizeInMB <= 0 {
		log.Err("WriteMvRequest::isValid: ChunkSizeInMB is invalid in request: %s", reqStr)
		return fmt.Errorf("ChunkSizeInMB is invalid in request: %s", reqStr)
	}

	if req.ChunkIndex < 0 || req.ChunkIndex%req.ChunkSizeInMB != 0 {
		log.Err("WriteMvRequest::isValid: ChunkIndex is invalid in request: %s", reqStr)
		return fmt.Errorf("ChunkIndex is invalid in request: %s", reqStr)
	}

	if len(req.Data) == 0 || len(req.Data) > int(req.ChunkSizeInMB*common.MbToBytes) {
		log.Err("WriteMvRequest::isValid: Data buffer is invalid in request: %s", reqStr)
		return fmt.Errorf("data buffer is invalid in request: %s", reqStr)
	}

	if !req.IsLastChunk && len(req.Data) != int(req.ChunkSizeInMB*common.MbToBytes) {
		log.Err("WriteMvRequest::isValid: Data buffer length is not equal to chunk size in request: %s", reqStr)
		return fmt.Errorf("data buffer length is not equal to chunk size in request: %s", reqStr)
	}

	return nil
}

type WriteMvResponse struct {
}
