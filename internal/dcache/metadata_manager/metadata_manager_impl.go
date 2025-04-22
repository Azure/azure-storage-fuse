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
						log.Err("DistributedCache::Start error [failed to create directory %s: %v]", dir, err)
						return err
					}
				}
			}

		} else {
			log.Err("DistributedCache::Start error [failed to get properties for %s: %v]", metadataManagerInstance.mdRoot, err)
			return err
		}
	}
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
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Warn("CreateFileInit :: PutBlobInStorage for %s failed due to ETag mismatch", path)
			return nil
		}
		log.Err("CreateFileInit :: Failed to put blob %s in storage: %v", path, err)
	}
	return err
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
	return count, nil
}

func (m *BlobMetadataManager) updateHandleCount(path string, increment bool) (int64, error) {
	const maxBackoff = 1 * time.Second // Maximum backoff time in seconds
	backoff := 1 * time.Millisecond    // Initial backoff time in milliseconds
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
			log.Err("GetFileOpenCount :: Failed to parse handle count for path %s : %v", path, err)
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
			Path:      filepath.Join(m.mdRoot, path),
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
				if time.Since(retryTime) >= time.Minute {
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
		log.Err("GetFileOpenCount :: Failed to get handle count for path %s : %v", path, err)
		return -1, err
	}
	openCount, ok := prop.Metadata["opencount"]
	if !ok {
		log.Err("GetFileOpenCount :: openCount not found in metadata for path %s : Error %v", path, err)
		// TODO :: Add asserts
		return -1, err
	}
	count, err := strconv.ParseInt(*openCount, 10, 64)
	if err != nil {
		log.Err("GetFileOpenCount :: Failed to parse handle count for path %s : %v", path, err)
		return -1, err
	}
	if count < 0 {
		log.Warn("GetHandleCount :: Handle count is negative for path %s : %d", path, count)
	}

	return count, nil
}

// UpdateHeartbeat creates or updates the heartbeat file
func (m *BlobMetadataManager) updateHeartbeat(nodeId string, data []byte) error {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.cacheDir, "Nodes", nodeId+".hb")
	err := m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   heartbeatFilePath,
		Data:                   data,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("UpdateHeartbeat :: Failed to put heartbeat blob in storage: %v", err)
	}
	return err
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) deleteHeartbeat(nodeId string) error {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.cacheDir, "Nodes", nodeId+".hb")
	err := m.storageCallbacks.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: heartbeatFilePath,
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteHeartbeat :: DeleteBlobInStorage failed since blob is already deleted")
		}
	}
	return nil
}

// GetHeartbeat reads and returns the content of the heartbeat file
func (m *BlobMetadataManager) getHeartbeat(nodeId string) ([]byte, error) {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.cacheDir, "Nodes", nodeId+".hb")
	// Get the heartbeat content from storage
	data, err := m.storageCallbacks.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: heartbeatFilePath,
	})
	if err != nil {
		log.Debug("GetHeartbeat :: Failed to get heartbeat file content: %v", err)
	}

	return data, err
}

// GetAllNodes enumerates and returns the list of all nodes with a heartbeat
func (m *BlobMetadataManager) getAllNodes() ([]string, error) {
	list, err := m.storageCallbacks.ListBlobs(internal.ReadDirOptions{
		Name: filepath.Join(m.cacheDir, "Nodes"),
	})
	if err != nil {
		log.Debug("GetAllNodes :: Failed to get nodes list: %v", err)
		return nil, err
	}
	// Extract the node IDs from the list of blobs
	var nodes []string
	for _, blob := range list {
		if blob.Name != "" {
			nodeId := blob.Name[:len(blob.Name)-3] // Remove the ".hb" extension
			if nodeId != "" {
				nodes = append(nodes, nodeId)
			}
		}
	}

	return nil, nil
}

// CreateInitialClusterMap creates the initial cluster map
func (m *BlobMetadataManager) createInitialClusterMap(clustermap []byte) error {
	// Create the clustermap file path
	clustermapPath := filepath.Join(m.cacheDir, "clustermap.json")
	err := m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("CreateInitialClusterMap :: Failed to create clustermap: %v", err)
	}
	return err
}

// UpdateClusterMapStart claims update ownership of the cluster map
func (m *BlobMetadataManager) updateClusterMapStart(clustermap []byte, etag *string) error {
	err := m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(m.cacheDir, "clustermap.json"),
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    *etag,
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Warn("UpdateClusterMapStart :: ETag mismatch some other node has taken ownership for updating clustermap")
		} else {
			log.Debug("UpdateClusterMapStart :: Failed to update clustermap: %v", err)
		}
	}
	return err
}

// UpdateClusterMapEnd finalizes the cluster map update
func (m *BlobMetadataManager) updateClusterMapEnd(clustermap []byte) error {
	err := m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   filepath.Join(m.cacheDir, "clustermap.json"),
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("UpdateClusterMapEnd :: Failed to finalize clustermap update: %v", err)
	}

	return err
}

// GetClusterMap reads and returns the content of the cluster map
func (m *BlobMetadataManager) getClusterMap() ([]byte, *string, error) {
	attr, err := m.storageCallbacks.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: filepath.Join(m.cacheDir, "clustermap.json"),
	})
	if err != nil {
		log.Debug("GetClusterMap :: Failed to get cluster map: %v", err)
		return nil, nil, err
	}
	// Get the cluster map content from storage
	data, err := m.storageCallbacks.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: filepath.Join(m.cacheDir, "clustermap.json"),
	})
	if err != nil {
		log.Debug("GetClusterMap :: Failed to get cluster map content: %v", err)
		return nil, nil, err
	}
	// Return the cluster map content and ETag
	return data, &attr.ETag, nil
}
