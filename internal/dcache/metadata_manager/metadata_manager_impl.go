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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common"
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
				if err = storageCallback.CreateDir(internal.CreateDirOptions{Name: dir, ForceDirCreationDisabled: true}); err != nil {

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
	common.Assert(err == nil, "Failed to create directories", err)
	return nil
}

// Package-level functions that delegate to the singleton instance

func GetMdRoot() string {
	return metadataManagerInstance.mdRoot
}

func CreateFileInit(filePath string, fileMetadata []byte) error {
	return metadataManagerInstance.createFileInit(filePath, fileMetadata)
}

func CreateFileFinalize(filePath string, fileMetadata []byte, fileSize int64) error {
	return metadataManagerInstance.createFileFinalize(filePath, fileMetadata, fileSize)
}

func GetFile(filePath string) ([]byte, int64, error) {
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

func DeleteHeartbeat(nodeId string) error {
	return metadataManagerInstance.deleteHeartbeat(nodeId)
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

func UpdateClusterMapStart(clustermap []byte, etag *string) error {
	return metadataManagerInstance.updateClusterMapStart(clustermap, etag)
}

func UpdateClusterMapEnd(clustermap []byte) error {
	return metadataManagerInstance.updateClusterMapEnd(clustermap)
}

func GetClusterMap() ([]byte, *string, error) {
	return metadataManagerInstance.getClusterMap()
}

// CreateFileInit creates the initial metadata for a file
// TODO :: Return etag value to use for CreateFileFinalize
// Can help ensure cases where the initial node went down before finalizing and tried to finalize later
func (m *BlobMetadataManager) createFileInit(filePath string, fileMetadata []byte) error {
	path := filepath.Join(m.mdRoot, "Objects", filePath)

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
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
	log.Debug("CreateFileInit :: Created file %s in storage", path)
	return nil
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) createFileFinalize(filePath string, fileMetadata []byte, fileSize int64) error {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// Store the open-count and file size in the metadata blob property
	openCount := "0"
	sizeStr := strconv.FormatInt(fileSize, 10)
	metadata := map[string]*string{
		"opencount":           &openCount,
		"cache-object-length": &sizeStr,
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Data:                   fileMetadata,
		Metadata:               metadata,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("CreateFileFinalize :: Failed to put blob %s in storage: %v", path, err)
		return err
	}
	log.Debug("CreateFileFinalize :: Finalized file %s in storage %v", path, err)
	return err
}

// TODO :: Replace the two REST API calls with a single call to DownloadStream
// GetFile reads and returns the content of metadata for a file
func (m *BlobMetadataManager) getFile(filePath string) ([]byte, int64, error) {
	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// Get the file content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: path,
	})
	if err != nil {
		log.Debug("GetFile :: Failed to get metadata file content for file %s : %v", path, err)
		return nil, -1, err
	}
	// Get the file size from the metadata properties
	prop, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetFile :: Failed to get properties for path %s : %v", path, err)
		return nil, -1, err
	}
	// Extract the size from the metadata properties
	size, ok := prop.Metadata["cache-object-length"]
	common.Assert(ok, fmt.Sprintf("size not found in metadata for path %s", path))
	if !ok {
		log.Err("GetFile :: size not found in metadata for path %s", path)
		return nil, -1, err
	}

	sizeInt, err := strconv.ParseInt(*size, 10, 64)
	if err != nil {
		log.Err("GetFile :: Failed to parse size for path %s with value %s : %v", path, *size, err)
		return nil, -1, err
	}

	common.Assert(sizeInt >= 0, "size cannot be negative", sizeInt)
	if sizeInt < 0 {
		log.Warn("GetFile :: Size is negative for path %s : %d", path, sizeInt)
		return nil, -1, fmt.Errorf("size is negative for path %s : %d", path, sizeInt)
	}

	log.Debug("GetFile :: Size for path %s : %d", path, sizeInt)
	return data, sizeInt, nil
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
		return err
	}
	log.Debug("DeleteFile :: Deleted blob %s in storage", path)
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
	common.Assert(count > 0, "Open file cannot have count <= 0", count)
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
	log.Debug("CloseFile :: Updated file open count for path %s : %d", path, count)
	common.Assert(count >= 0, "File cannot have -ve opencount", count)
	return count, nil
}

func (m *BlobMetadataManager) updateHandleCount(path string, increment bool) (int64, error) {
	const maxRetryTime = 1 * time.Minute // Maximum Retry time in minutes
	const maxBackoff = 1 * time.Second   // Maximum Retry time in seconds
	backoff := 1 * time.Millisecond      // Initial backoff time in milliseconds
	var openCount int
	retryTime := time.Now()
	for {
		// Get the current open count
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
			return -1, fmt.Errorf("opencount property not found in metadata for path %s. Issue needs to be debugged", path)
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
			return -1, fmt.Errorf("handle count is negative for path %s : %d", path, openCount)
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
					return -1, fmt.Errorf("retrying exceeded one minute for path %s", path)
				}
				continue
			} else {
				log.Err("updateHandleCount :: Failed to update metadata property for path %s : %v", path, err)
				return -1, err
			}
		} else {
			log.Debug("updateHandleCount :: Updated metadata property for path %s : %d", path, openCount)
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
		common.Assert(false, fmt.Sprintf("openCount not found in metadata for path %s", path))
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

	log.Debug("GetFileOpenCount :: Open count for path %s : %d", path, count)
	return int64(count), nil
}

// UpdateHeartbeat creates or updates the heartbeat file
func (m *BlobMetadataManager) updateHeartbeat(nodeId string, data []byte) error {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   heartbeatFilePath,
		Data:                   data,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("UpdateHeartbeat :: Failed to put heartbeat blob path %s in storage: %v", heartbeatFilePath, err)
		common.Assert(false, fmt.Sprintf("Failed to put heartbeat blob path %s in storage: %v", heartbeatFilePath, err))
		return err
	}
	log.Debug("UpdateHeartbeat :: Updated heartbeat blob path %s in storage", heartbeatFilePath)
	return err
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) deleteHeartbeat(nodeId string) error {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: heartbeatFilePath,
	})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			log.Err("DeleteHeartbeat :: DeleteBlobInStorage failed since blob %s is already deleted", heartbeatFilePath)
		} else {
			log.Err("DeleteHeartbeat :: Failed to delete heartbeat blob %s in storage: %v", heartbeatFilePath, err)
		}
		return err
	}
	log.Debug("DeleteHeartbeat :: Deleted heartbeat blob %s in storage", heartbeatFilePath)
	return err
}

// GetHeartbeat reads and returns the content of the heartbeat file
func (m *BlobMetadataManager) getHeartbeat(nodeId string) ([]byte, error) {
	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	// Get the heartbeat content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: heartbeatFilePath,
	})
	if err != nil {
		log.Err("GetHeartbeat :: Failed to get heartbeat file content for %s: %v", heartbeatFilePath, err)
		common.Assert(false, fmt.Sprintf("Failed to get heartbeat file content for %s: %v", heartbeatFilePath, err))
		return nil, err
	}
	log.Debug("GetHeartbeat :: Successfully got heartbeat file content for %s", heartbeatFilePath)
	return data, err
}

// GetAllNodes enumerates and returns the list of all nodes with a heartbeat
func (m *BlobMetadataManager) getAllNodes() ([]string, error) {
	path := filepath.Join(m.mdRoot, "Nodes")
	list, err := m.storageCallback.ReadDirFromStorage(internal.ReadDirOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetAllNodes :: Failed to enumerate nodes list from %s : %v", path, err)
		common.Assert(false, fmt.Sprintf("Failed to enumerate nodes list from %s : %v", path, err))
		return nil, err
	}
	// Extract the node IDs from the list of blobs
	var nodes []string
	for _, blob := range list {
		log.Debug("GetAllNodes :: Found blob: %s", blob.Name)
		if strings.HasSuffix(blob.Name, ".hb") {
			nodeId := blob.Name[:len(blob.Name)-3] // Remove the ".hb" extension
			if common.IsValidUUID(nodeId) {
				nodes = append(nodes, nodeId)
			} else {
				log.Err("Invalid heartbeat blob: %s", blob.Name)
				common.Assert(false, "Invalid heartbeat blob", blob.Name)
			}
		} else {
			log.Warn("GetAllNodes :: Unexpected blob found in Nodes folder: %s", blob.Name)
			common.Assert(false, "Unexpected blob found in Nodes folder", blob.Name)
		}
	}

	log.Debug("GetAllNodes :: Found %d nodes", len(nodes))
	return nodes, nil
}

// CreateInitialClusterMap creates the initial cluster map
func (m *BlobMetadataManager) createInitialClusterMap(clustermap []byte) error {
	// Create the clustermap file path
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})
	// Caller has to check if the error is ConditionNotMet or something else
	// and take appropriate action.
	// If the error is ConditionNotMet, it means the clustermap already exists
	// and the caller should not overwrite it.
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			// Log the reason for failure and return the error for caller to handle
			log.Info("CreateInitialClusterMap :: PutBlobInStorage failed for %s due to ETag mismatch", clustermapPath)
			return err
		}
		log.Err("CreateInitialClusterMap :: Failed to put blob %s in storage: %v", clustermapPath, err)
		return err
	}
	log.Info("CreateInitialClusterMap :: Created initial clustermap with path %s", clustermapPath)
	return nil
}

// UpdateClusterMapStart claims update ownership of the cluster map
func (m *BlobMetadataManager) updateClusterMapStart(clustermap []byte, etag *string) error {
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    *etag,
	})
	// Caller should add a check to identify the error is ConditionNotMet or something else
	// and take appropriate action.
	// If the error is ConditionNotMet, it means the clustermap is already being updated
	// and the caller should not overwrite it.
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Warn("UpdateClusterMapStart :: ETag mismatch some other node has taken ownership for updating clustermap with path %s", clustermapPath)
		} else {
			log.Err("UpdateClusterMapStart :: Failed to update clustermap %s : %v", clustermapPath, err)
		}
	}
	log.Debug("UpdateClusterMapStart :: Updated clustermap with path %s", clustermapPath)
	return err
}

// TODO :: for safe update  updateClusterMapStart should return a Etag
// value to be used for updateClusterMapEnd
// UpdateClusterMapEnd finalizes the cluster map update
func (m *BlobMetadataManager) updateClusterMapEnd(clustermap []byte) error {
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("UpdateClusterMapEnd :: Failed to finalize clustermap update for %s: %v", clustermapPath, err)
	}
	log.Debug("UpdateClusterMapEnd :: Finalized clustermap update for %s", clustermapPath)
	return err
}

// GetClusterMap reads and returns the content of the cluster map as a byte array, the current Etag value and error if any
func (m *BlobMetadataManager) getClusterMap() ([]byte, *string, error) {
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: clustermapPath,
	})
	if err != nil {
		log.Err("GetClusterMap :: Failed to get cluster map properties %s : %v", clustermapPath, err)
		return nil, nil, err
	}
	// TODO :: If some node updates the clustermap between GetProperties and GetBlobFromStorage
	// then updateClusterMapStart will fail with ETag mismatch.
	// In that case we can create a new function to call doawnloadStream directly for content and etag.
	// Get the cluster map content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: clustermapPath,
	})
	if err != nil {
		log.Err("GetClusterMap :: Failed to get cluster map content with path %s : %v", clustermapPath, err)
		return nil, nil, err
	}
	log.Debug("GetClusterMap :: Successfully got cluster map content for %s", clustermapPath)
	// Return the cluster map content and ETag
	return data, &attr.ETag, nil
}
