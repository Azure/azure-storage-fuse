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
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	models "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager/models"
)

var (
	// MetadataManagerInstance is the singleton instance of BlobMetadataManager
	metadataManagerInstance *BlobMetadataManager
	once                    sync.Once
)

// BlobMetadataManager is the implementation of MetadataManager interface
type BlobMetadataManager struct {
	mdRoot          string
	storageCallback dcache.StorageCallbacks
}

// init initializes the singleton instance of BlobMetadataManager
func Init(storageCallback dcache.StorageCallbacks, cacheId string) {
	once.Do(func() {
		metadataManagerInstance = &BlobMetadataManager{
			mdRoot:          "__CACHE__" + cacheId, // Set a default cache directory
			storageCallback: storageCallback,       // Initialize storage callback
		}
	})
}

// Package-level functions that delegate to the singleton instance

func CreateFileInit(filePath string, fileMetadata *models.FileMetadata) error {
	return metadataManagerInstance.createFileInit(filePath, fileMetadata)
}

func CreateFileFinalize(filePath string, fileMetadata *models.FileMetadata) error {
	return metadataManagerInstance.createFileFinalize(filePath, fileMetadata)
}

func GetFile(filePath string) (*models.FileMetadata, error) {
	return metadataManagerInstance.getFile(filePath)
}

func DeleteFile(filePath string) error {
	return metadataManagerInstance.deleteFile(filePath)
}

func OpenFile(filePath string) (int64, error) {
	return metadataManagerInstance.openFile(filePath)
}

func CloseFile(filePath string) (int64, error) {
	return metadataManagerInstance.closeFile(filePath)
}

func GetFileOpenCount(filePath string) (int64, error) {
	return metadataManagerInstance.getFileOpenCount(filePath)
}

func UpdateHeartbeat(nodeId string, data []byte) error {
	return metadataManagerInstance.updateHeartbeat(nodeId, data)
}

func DeleteHeartbeat(nodeId string, data []byte) error {
	return metadataManagerInstance.deleteHeartbeat(nodeId, data)
}

func GetHeartbeat(nodeId string) ([]byte, error) {
	return metadataManagerInstance.getHeartbeat(nodeId)
}

func GetAllNodes() ([]string, error) {
	return metadataManagerInstance.getAllNodes()
}

func CreateInitialClusterMap(clustermap []byte) error {
	return metadataManagerInstance.createInitialClusterMap(clustermap)
}

func UpdateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	return metadataManagerInstance.updateClusterMapStart(clustermap, etag)
}

func UpdateClusterMapEnd(clustermap []byte) error {
	return metadataManagerInstance.updateClusterMapEnd(clustermap)
}

func GetClusterMap() ([]byte, *azcore.ETag, error) {
	return metadataManagerInstance.getClusterMap()
}

// CreateFileInit creates the initial metadata for a file
func (m *BlobMetadataManager) createFileInit(filePath string, fileMetadata *models.FileMetadata) error {
	// Dummy implementation
	return nil
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) createFileFinalize(filePath string, fileMetadata *models.FileMetadata) error {
	// Dummy implementation
	return nil
}

// GetFile reads and returns the content of metadata for a file
func (m *BlobMetadataManager) getFile(filePath string) (*models.FileMetadata, error) {
	// Dummy implementation
	return nil, nil
}

// DeleteFile removes metadata for a file
func (m *BlobMetadataManager) deleteFile(filePath string) error {
	// Dummy implementation
	return nil
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) openFile(filePath string) (int64, error) {
	// Dummy implementation

	return 0, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) closeFile(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// GetFileOpenCount returns the current open count for a file
func (m *BlobMetadataManager) getFileOpenCount(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// UpdateHeartbeat creates or updates the heartbeat file
func (m *BlobMetadataManager) updateHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) deleteHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// GetHeartbeat reads and returns the content of the heartbeat file
func (m *BlobMetadataManager) getHeartbeat(nodeId string) ([]byte, error) {
	// Dummy implementation
	return nil, nil
}

// GetAllNodes enumerates and returns the list of all nodes with a heartbeat
func (m *BlobMetadataManager) getAllNodes() ([]string, error) {
	// Dummy implementation
	return nil, nil
}

// CreateInitialClusterMap creates the initial cluster map
func (m *BlobMetadataManager) createInitialClusterMap(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapStart claims update ownership of the cluster map
func (m *BlobMetadataManager) updateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapEnd finalizes the cluster map update
func (m *BlobMetadataManager) updateClusterMapEnd(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// GetClusterMap reads and returns the content of the cluster map
func (m *BlobMetadataManager) getClusterMap() ([]byte, *azcore.ETag, error) {
	// Dummy implementation
	return nil, nil, nil
}
