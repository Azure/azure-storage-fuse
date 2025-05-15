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

	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

type ReadMvRequest struct {
	FileID string // unique guid of the file, as stored in metadata blob
	MvName string // name of the MV to be read, e.g., "mv0", "mv1", etc.

	// An MV can hold multiple chunks of the file (one from each stripe).
	// ChunkIndex identifies the exact chunk to be read. It is the index of the chunk
	// within the file and is 0 based.
	// e.g., for stripe size of 16MiB and ChunkSizeInMiB == 4,
	// file offset of 1MiB would translate to the following:
	//  MvName: "mv0"   (assuming first MV in the file's mv list is "mv0")
	//  ChunkIndex: 0
	//  OffsetInChunk: 1048576 (1MiB)
	// while a file offset of 17MiB would translate to the following:
	//  MvName: "mv0"
	//  ChunkIndex: 4
	//  OffsetInChunk: 1048576 (1MiB)
	//
	// The chunks are stored in RV as,
	//  <MvName>/<FileID>.<ChunkIndex * ChunkSizeInMiB>.data and
	//  <MvName>/<FileID>.<ChunkIndex * ChunkSizeInMiB>.hash
	//
	ChunkIndex     int64
	OffsetInChunk  int64 // read offset within the chunk. This should not be greater than ChunkSizeInMiB
	Length         int64 // Length in bytes of data to be read
	ChunkSizeInMiB int64 // Chunk size in MiB

	Data []byte // buffer to store the data read from the chunk. Must be at least Length bytes
}

// helper method which can be used for logging the request contents except the data buffer
// Use this instead of %+v to avoid printing the data buffer
func (req *ReadMvRequest) toString() string {
	return fmt.Sprintf("{FileID: %s, MvName: %s, ChunkIndex: %d, OffsetInChunk: %d, Length: %d, ChunkSizeInMiB: %d, Data buffer size: %d}",
		req.FileID, req.MvName, req.ChunkIndex, req.OffsetInChunk, req.Length,
		req.ChunkSizeInMiB, len(req.Data))
}

// check if the request is valid
func (req *ReadMvRequest) isValid() error {
	common.Assert(common.IsValidUUID(req.FileID))
	common.Assert(cm.IsValidMVName(req.MvName))

	if (req.ChunkSizeInMiB*common.MbToBytes) < cm.MinChunkSize ||
		(req.ChunkSizeInMiB*common.MbToBytes) > cm.MaxChunkSize {
		reqStr := req.toString()
		err := fmt.Errorf("ChunkSizeInMiB is invalid in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	if req.ChunkIndex < 0 || req.ChunkIndex > ChunkIndexUpperBound {
		reqStr := req.toString()
		err := fmt.Errorf("ChunkIndex is invalid in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	if req.OffsetInChunk < 0 || req.OffsetInChunk >= req.ChunkSizeInMiB*common.MbToBytes {
		reqStr := req.toString()
		err := fmt.Errorf("OffsetInChunk is invalid in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	if req.Length <= 0 || req.Length > req.ChunkSizeInMiB*common.MbToBytes {
		reqStr := req.toString()
		err := fmt.Errorf("length is invalid in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	// check if the requested data is not overlapping between two chunks
	// For example, if chunk size is 4MB and offset is 3MB, then length should be less than 1MB
	if req.OffsetInChunk+req.Length > req.ChunkSizeInMiB*common.MbToBytes {
		reqStr := req.toString()
		err := fmt.Errorf("length and OffsetInChunk are invalid in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	// check if the requested data size is greater than the buffer provided
	if len(req.Data) < int(req.Length) {
		reqStr := req.toString()
		err := fmt.Errorf("data buffer size is less than requested data size in request: %s", reqStr)
		log.Err("ReadMvRequest::isValid: %v", err)
		return err
	}

	return nil
}

type ReadMvResponse struct {
	BytesRead int64 // Number of bytes read
}

type WriteMvRequest struct {
	FileID string // unique guid of the file, as stored in metadata blob
	MvName string // name of the MV where this chunk will be written, e.g., "mv0", "mv1", etc.

	// An MV can hold multiple chunks of the file (one from each stripe).
	// ChunkIndex identifies the exact chunk to be read. It is the index of the chunk
	// within the file and is 0 based.
	// e.g., for stripe size of 16MiB and ChunkSizeInMiB == 4,
	// file offset of 1MiB would translate to the following:
	//  MvName: "mv0"   (assuming first MV in the file's mv list is "mv0")
	//  ChunkIndex: 0
	//  OffsetInChunk: 1048576 (1MiB)
	// while a file offset of 17MiB would translate to the following:
	//  MvName: "mv0"
	//  ChunkIndex: 4
	//  OffsetInChunk: 1048576 (1MiB)
	//
	// The chunks are stored in RV as,
	//  <MvName>/<FileID>.<ChunkIndex * ChunkSizeInMiB>.data and
	//  <MvName>/<FileID>.<ChunkIndex * ChunkSizeInMiB>.hash
	//
	ChunkIndex     int64
	Data           []byte // Data to be written
	ChunkSizeInMiB int64  // Chunk size in MiB
	IsLastChunk    bool   // boolean flag to indicate if this is the last chunk
}

// helper method which can be used for logging the request contents except the data buffer
// Use this instead of %+v to avoid printing the data buffer
func (req *WriteMvRequest) toString() string {
	return fmt.Sprintf("{FileID: %s, MvName: %s, ChunkIndex: %d, ChunkSizeInMiB: %d, IsLastChunk: %v, Data buffer size: %d}",
		req.FileID, req.MvName, req.ChunkIndex, req.ChunkSizeInMiB, req.IsLastChunk, len(req.Data))
}

// check if the request is valid
func (req *WriteMvRequest) isValid() error {
	common.Assert(common.IsValidUUID(req.FileID))
	common.Assert(cm.IsValidMVName(req.MvName))

	if (req.ChunkSizeInMiB*common.MbToBytes) < cm.MinChunkSize ||
		(req.ChunkSizeInMiB*common.MbToBytes) > cm.MaxChunkSize {
		reqStr := req.toString()
		err := fmt.Errorf("ChunkSizeInMiB is invalid in request: %s", reqStr)
		log.Err("WriteMvRequest::isValid: %v", err)
		return err
	}

	if req.ChunkIndex < 0 || req.ChunkIndex > ChunkIndexUpperBound {
		reqStr := req.toString()
		err := fmt.Errorf("ChunkIndex is invalid in request: %s", reqStr)
		log.Err("WriteMvRequest::isValid: %v", err)
		return err
	}

	if len(req.Data) == 0 || len(req.Data) > int(req.ChunkSizeInMiB*common.MbToBytes) {
		reqStr := req.toString()
		err := fmt.Errorf("data buffer is invalid in request: %s", reqStr)
		log.Err("WriteMvRequest::isValid: %v", err)
		return err
	}

	if !req.IsLastChunk && len(req.Data) != int(req.ChunkSizeInMiB*common.MbToBytes) {
		reqStr := req.toString()
		err := fmt.Errorf("data buffer length is not equal to chunk size in request: %s", reqStr)
		log.Err("WriteMvRequest::isValid: %v", err)
		return err
	}

	return nil
}

type WriteMvResponse struct {
}

// syncJob tracks resync of one MV replica, srcRVName/mvName -> destRVName/mvName.
type syncJob struct {
	mvName       string                   // name of the MV to be synced
	srcRVName    string                   // name of the source RV
	srcSyncID    string                   // sync ID of the StartSync() RPC call made to the node hosting the source RV
	destRVName   string                   // name of the destination RV
	destSyncID   string                   // sync ID of the StartSync() RPC call made to the node hosting the destination RV
	syncSize     int64                    // total number of bytes to be synced
	componentRVs []*models.RVNameAndState // list of component RVs for the MV
}

// Helper method which can be used for logging the syncJob.
func (job *syncJob) toString() string {
	return fmt.Sprintf("{%s/%s -> %s/%s, srcSyncID: %s, destSyncID: %s, syncSize: %d bytes, componentRVs: %v}",
		job.srcRVName, job.mvName, job.destRVName, job.mvName, job.srcSyncID, job.destSyncID,
		job.syncSize, rpc.ComponentRVsToString(job.componentRVs))
}
