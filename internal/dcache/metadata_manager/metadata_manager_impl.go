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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
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
func Init(storageCallback dcache.StorageCallbacks, cacheId string) error {
	once.Do(func() {
		metadataManagerInstance = &BlobMetadataManager{
			mdRoot:          "__CACHE__" + cacheId, // Set a default cache directory
			storageCallback: storageCallback,       // Initialize storage callback
		}
	})

	_, err := storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{Name: metadataManagerInstance.mdRoot + "/Objects"})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			directories := []string{metadataManagerInstance.mdRoot, metadataManagerInstance.mdRoot + "/Nodes", metadataManagerInstance.mdRoot + "/Objects"}
			for _, dir := range directories {
				if err := storageCallback.CreateDir(internal.CreateDirOptions{Name: dir, ForceDirCreationDisabled: true}); err != nil {

					if !bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
						log.Err("BlobMetadataManager :: Init error [failed to create directory %s: %v]", dir, err)
						return err
					} else {
						log.Info("BlobMetadataManager :: Init [directory %s already exists]", dir)
					}
				} else {
					log.Info("BlobMetadataManager :: Init [created directory %s]", dir)
				}
			}

		} else {
			log.Err("BlobMetadataManager :: Init error [failed to get properties for %s: %v]", metadataManagerInstance.mdRoot, err)
			return err
		}
	}
	// TODO :: Verify/Asseet that the directories are created successfully
	return nil
}

// Package-level functions that delegate to the singleton instance

func GetMdRoot() string {
	return metadataManagerInstance.mdRoot
}

func CreateFileInit(filePath string, fileMetadata []byte) error {
	return metadataManagerInstance.createFileInit(filePath, fileMetadata)
}

func CreateFileFinalize(filePath string, fileMetadata []byte) error {
	return metadataManagerInstance.createFileFinalize(filePath, fileMetadata)
}

func GetFile(filePath string) (*dcache.FileMetadata, error) {
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
// TODO :: Return etag value to use for CreateFileFinalize
// Can help ensure cases where the initial node went down before finalizing and tried to finalize later
func (m *BlobMetadataManager) createFileInit(filePath string, fileMetadata []byte) error {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// Store the open-count in the metadata blob property
	openCount := "0"
	metadata := map[string]*string{
		"opencount": &openCount,
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Metadata:               metadata,
		Data:                   fileMetadata,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	// If the node is able to create a file it succeeds
	// If it fails with ETag mismatch, it means the file was already created by another node
	// and the current node should check the error code to ascertain the reason
	// If the error is not ETag mismatch, it means the file creation failed due to some other reason
	// and createFileInit should not proceed.
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Err("CreateFileInit :: PutBlobInStorage for %s failed due to ETag mismatch", path)
			return err
		}
		log.Err("CreateFileInit :: Failed to put blob %s in storage: %v", path, err)
		return err
	}
	return nil
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) createFileFinalize(filePath string, fileMetadata []byte) error {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// TODO :: check metadata property is not overwritten by this
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Data:                   fileMetadata,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("CreateFileFinalize :: Failed to put blob %s in storage: %v", path, err)
	}
	return err
}

// GetFile reads and returns the content of metadata for a file
// TODO :: Check if we can return []byte to make this function symmetric with others
func (m *BlobMetadataManager) getFile(filePath string) (*dcache.FileMetadata, error) {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// Get the metadata content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: path,
	})
	if err != nil {
		log.Debug("GetFile :: Failed to get metadata file content for file %s : %v", path, err)
		return nil, err
	}
	// Unmarshal the JSON data into the Metadata struct
	var metadata dcache.FileMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		log.Debug("GetFile :: Failed to unmarshal JSON data: %v", err)
		return nil, err
	}
	// Return the Metadata struct
	return &metadata, nil
}

// DeleteFile removes metadata for a file
func (m *BlobMetadataManager) deleteFile(filePath string) error {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: path,
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteFile :: DeleteBlobInStorage failed since blob %s is already deleted", path)
			return nil
		}
		log.Err("DeleteFile :: Failed to delete blob %s in storage: %v", path, err)
	}
	return err
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) openFile(filePath string) (int64, error) {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	count, err := m.updateHandleCount(path, true)
	if err != nil {
		log.Err("OpenFile :: Failed to update file open count for path %s : %v", path, err)
		return -1, err
	}
	log.Debug("OpenFile :: Updated file open count for path %s : %d", path, count)
	return count, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) closeFile(filePath string) (int64, error) {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	count, err := m.updateHandleCount(path, false)
	if err != nil {
		log.Err("CloseFile :: Failed to update file open count for path %s : %v", path, err)
		return -1, err
	}
	// TODO :: Add assert file open count is not negative "file cannot have count < 0"
	log.Debug("CloseFile :: Updated file open count for path %s : %d", path, count)
	return count, nil
}

func (m *BlobMetadataManager) updateHandleCount(path string, increment bool) (int64, error) {
	const maxRetryTime = 1 * time.Minute // Maximum Retry time in minutes
	const maxBackoff = 1 * time.Second   // Maximum Retry time in seconds
	backoff := 1 * time.Millisecond      // Initial backoff time in milliseconds
	var openCount int
	retryTime := time.Now()
	for {
		// Get the current handle count
		attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
			Name: path,
		})
		if err != nil {
			log.Err("updateHandleCount :: Failed to get handle count for %s : %v", path, err)
			return -1, err
		}

		// We never create file metadata blob w/o opencount property set.
		if attr.Metadata["opencount"] == nil {
			log.Err("updateHandleCount :: File metadata blob found w/o opencount property: %s", path)
			return -1, fmt.Errorf("Opencount property not found in metadata for path %s. Issue needs to be debugged.", path)
		}
		openCount, err = strconv.Atoi(*attr.Metadata["opencount"])
		if err != nil {
			log.Err("GetFileOpenCount :: Failed to parse handle count for path %s with value %s : %v", path, *attr.Metadata["opencount"], err)
			return -1, err
		}
		if increment {
			openCount++
		} else {
			openCount--
		}
		if openCount < 0 {
			log.Err("updateHandleCount :: Handle count is negative for path %s : %d", path, openCount)
			return -1, fmt.Errorf("Handle count is negative for path %s : %d", path, openCount)
		}
		openCountStr := strconv.Itoa(openCount)
		attr.Metadata["opencount"] = &openCountStr

		// Set the new metadata in storage
		err = m.storageCallback.SetMetaPropertiesInStorage(internal.SetMetadataOptions{
			Path:      path,
			Metadata:  attr.Metadata,
			Etag:      to.Ptr(azcore.ETag(attr.ETag)),
			Overwrite: true,
		})
		if err != nil {
			if bloberror.HasCode(err, bloberror.ConditionNotMet) {
				log.Warn("updateHandleCount :: SetPropertiesInStorage failed for path %s due to ETag mismatch, retrying...", path)

				// Apply exponential backoff
				log.Debug("updateHandleCount :: Retrying in %d milliseconds...", backoff)
				time.Sleep(backoff)

				// Double the backoff time, but cap it at maxBackoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				// Check if retrying has exceeded a minute
				if time.Since(retryTime) >= maxRetryTime {
					log.Warn("updateHandleCount :: Retrying exceeded one minute for path %s, exiting...", path)
					return -1, fmt.Errorf("Retrying exceeded one minute for path %s", path)
				}
				continue
			} else {
				log.Err("updateHandleCount :: Failed to update metadata property for path %s : %v", path, err)
				return -1, err
			}
		} else {
			break
		}
	}
	return int64(openCount), nil
}

// GetFileOpenCount returns the current open count for a file
func (m *BlobMetadataManager) getFileOpenCount(filePath string) (int64, error) {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	prop, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetFileOpenCount :: Failed to get open count for path %s : %v", path, err)
		return -1, err
	}
	openCount, ok := prop.Metadata["opencount"]
	if !ok {
		log.Err("GetFileOpenCount :: openCount not found in metadata for path %s", path)
		// TODO :: Add asserts
		return -1, err
	}
	count, err := strconv.Atoi(*openCount)

	if err != nil {
		log.Err("GetFileOpenCount :: Failed to parse open count for path %s with value %s : %v", path, *openCount, err)
		return -1, err
	}
	if count < 0 {
		log.Warn("GetHandleCount :: Open count is negative for path %s : %d", path, count)
	}

	return count, nil
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
