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
)

var (
	// MetadataManagerInstance is the singleton instance of BlobMetadataManager
	MetadataManagerInstance *BlobMetadataManager
	once                    sync.Once
)

// BlobMetadataManager is the implementation of MetadataManager interface
type BlobMetadataManager struct {
	cacheDir string
}

// init initializes the singleton instance of BlobMetadataManager
func init() {
	once.Do(func() {
		MetadataManagerInstance = &BlobMetadataManager{
			cacheDir: "/default/cache/dir", // Set a default cache directory
		}
	})
}

// Package-level functions that delegate to the singleton instance

func CreateFileInit(filePath string, fileMetadata *dcache.FileMetadata) error {
	return MetadataManagerInstance.CreateFileInit(filePath, fileMetadata)
}

func CreateFileFinalize(filePath string, fileMetadata *dcache.FileMetadata) error {
	return MetadataManagerInstance.CreateFileFinalize(filePath, fileMetadata)
}

func GetFile(filePath string) (*dcache.FileMetadata, error) {
	return MetadataManagerInstance.GetFile(filePath)
}

func DeleteFile(filePath string) error {
	return MetadataManagerInstance.DeleteFile(filePath)
}

func OpenFile(filePath string) (int64, error) {
	return MetadataManagerInstance.OpenFile(filePath)
}

func CloseFile(filePath string) (int64, error) {
	return MetadataManagerInstance.CloseFile(filePath)
}

func GetFileOpenCount(filePath string) (int64, error) {
	return MetadataManagerInstance.GetFileOpenCount(filePath)
}

func UpdateHeartbeat(nodeId string, data []byte) error {
	return MetadataManagerInstance.UpdateHeartbeat(nodeId, data)
}

func DeleteHeartbeat(nodeId string, data []byte) error {
	return MetadataManagerInstance.DeleteHeartbeat(nodeId, data)
}

func GetHeartbeat(nodeId string) ([]byte, error) {
	return MetadataManagerInstance.GetHeartbeat(nodeId)
}

func GetAllNodes() ([]string, error) {
	return MetadataManagerInstance.GetAllNodes()
}

func CreateInitialClusterMap(clustermap []byte) error {
	return MetadataManagerInstance.CreateInitialClusterMap(clustermap)
}

func UpdateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	return MetadataManagerInstance.UpdateClusterMapStart(clustermap, etag)
}

func UpdateClusterMapEnd(clustermap []byte) error {
	return MetadataManagerInstance.UpdateClusterMapEnd(clustermap)
}

func GetClusterMap() ([]byte, *azcore.ETag, error) {
	return MetadataManagerInstance.GetClusterMap()
}

// CreateFileInit creates the initial metadata for a file
func (m *BlobMetadataManager) CreateFileInit(filePath string, fileMetadata *dcache.FileMetadata) error {
	// Dummy implementation
	return nil
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) CreateFileFinalize(filePath string, fileMetadata *dcache.FileMetadata) error {
	// Dummy implementation
	return nil
}

// GetFile reads and returns the content of metadata for a file
func (m *BlobMetadataManager) GetFile(filePath string) (*dcache.FileMetadata, error) {
	// Dummy implementation
	return nil, nil
}

// DeleteFile removes metadata for a file
func (m *BlobMetadataManager) DeleteFile(filePath string) error {
	// Dummy implementation
	return nil
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) OpenFile(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) CloseFile(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// GetFileOpenCount returns the current open count for a file
func (m *BlobMetadataManager) GetFileOpenCount(filePath string) (int64, error) {
	// Dummy implementation
	return 0, nil
}

// UpdateHeartbeat creates or updates the heartbeat file
func (m *BlobMetadataManager) UpdateHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) DeleteHeartbeat(nodeId string, data []byte) error {
	// Dummy implementation
	return nil
}

// GetHeartbeat reads and returns the content of the heartbeat file
func (m *BlobMetadataManager) GetHeartbeat(nodeId string) ([]byte, error) {
	// Dummy implementation
	return nil, nil
}

// GetAllNodes enumerates and returns the list of all nodes with a heartbeat
func (m *BlobMetadataManager) GetAllNodes() ([]string, error) {
	// Dummy implementation
	return nil, nil
}

// CreateInitialClusterMap creates the initial cluster map
func (m *BlobMetadataManager) CreateInitialClusterMap(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapStart claims update ownership of the cluster map
func (m *BlobMetadataManager) UpdateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	// Dummy implementation
	return nil
}

// UpdateClusterMapEnd finalizes the cluster map update
func (m *BlobMetadataManager) UpdateClusterMapEnd(clustermap []byte) error {
	// Dummy implementation
	return nil
}

// GetClusterMap reads and returns the content of the cluster map
func (m *BlobMetadataManager) GetClusterMap() ([]byte, *azcore.ETag, error) {
	// Dummy implementation
	return nil, nil, nil
}
