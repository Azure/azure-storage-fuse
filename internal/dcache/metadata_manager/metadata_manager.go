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
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

// MetaDataManager defines the interface for managing metadata for the distributed cache.
// There are primarily two kinds of metadata managed by this interface:
// 1. Metadata for files stored in the distributed cache.
// 2. Metadata for internal files used by distributed cache, e.g., cluster map and heartbeat.
type MetadataManager interface {

	//
	// Following APIs are used to manage metadata for files stored in the distributed cache.
	//

	// CreateFileInit creates the initial metadata for a file.
	// Succeeds only when the file metadata is not already present.
	// Returns an etag value that must be passed to the corresponding createFileFinalize().
	// This will be called by the File Manager to create a non-existing file in response to a create call from fuse.
	// TODO :: Handle the case where the node fails before CreateFileFinalize is called.
	createFileInit(filePath string, fileMetadata []byte) (string, error)

	// CreateFileFinalize finalizes the metadata for a file updating size, sha256, and other properties.
	// For properties which were not available at the time of CreateFileInit.
	// Called by the File Manager in response to a close call from fuse.
	// The eTag parameter must be passed the etag returned by the corresponding createFileInit() call.
	createFileFinalize(filePath string, fileMetadata []byte, fileSize int64, eTag string) error

	// GetFile reads and returns the content of metadata for a file.
	// Also returns, file size, file state, opencount and attributes.
	getFile(filePath string) ([]byte, int64, dcache.FileState, int, *internal.ObjAttr, error)

	// Renames the metadata file to <fileName>.<fileId>.dcache.deleting
	// This would fail if the dest file already exists, which is unlikely due to the fileid in the name.
	renameFileToDeleting(filePath string, fileId string) error

	// DeleteFile removes metadata for a file.
	deleteFile(filePath string) error

	// OpenFile must be called when a file is opened by the application.
	// This will increment the open count for the file and return the updated open count.
	// Also updates the Etag in attr to new Etag on success.
	openFile(filePath string, attr *internal.ObjAttr) (int64, error)

	// CloseFile must be called when a file is closed by the application.
	// This will decrement the open count for the file and return the updated open count.
	closeFile(filePath string, attr *internal.ObjAttr) (int64, error)

	// GetFileOpenCount returns the current open count for a file.
	getFileOpenCount(filePath string) (int64, error)

	//
	// Following APIs are used to manage internal files in the distributed cache.
	//

	// CreateHeartbeat creates the initial heartbeat file if not present else updates the heartbeat.
	updateHeartbeat(nodeId string, data []byte) error

	// DeleteHeartbeat deletes the heartbeat file.
	deleteHeartbeat(nodeId string) error

	// GetHeartbeat reads and returns the content of the heartbeat file.
	getHeartbeat(nodeId string) ([]byte, error)

	// GetAllNodes enumerates and returns the list of all the nodes who have a heartbeat.
	getAllNodes() ([]string, error)

	// CreateInitialClusterMap creates the initial clustermap.
	createInitialClusterMap(clustermap []byte) error

	// Any node wanting to update the clustermap should do the following:
	// 1. Call GetClusterMap() to get the current clustermap and etag.
	// 2. Call UpdateClusterMapStart() passing the etag returned by GetClusterMap()
	//    to claim update ownership of the clustermap.
	// 3. Call UpdateClusterMapEnd() with the finalized clustermap.
	updateClusterMapStart(clustermap []byte, etag *string) error
	updateClusterMapEnd(clustermap []byte) error

	// GetClusterMap reads and returns the content of the clustermap.
	// The clustermap returned could be in ready or syncing state and the caller should appropriately handle it.
	getClusterMap() ([]byte, *string, error)
}
