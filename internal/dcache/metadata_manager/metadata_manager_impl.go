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
	// MetadataManagerInstance is the singleton instance of BlobMetadataManager.
	metadataManagerInstance *BlobMetadataManager
)

// BlobMetadataManager is the implementation of MetadataManager interface.
// It stores metadata as Blobs in a top level folder with prefix __CACHE__ inside the same container where
// Blobfuse stores the data.
type BlobMetadataManager struct {
	// BlobMetadataManager stores the metadata in special Blobs.
	// This is the root folder under which all the metadata Blobs are stored.
	mdRoot          string
	storageCallback dcache.StorageCallbacks
}

// This must be called from DistributedCache component's Start() method.
// If it fails this node will fail to join the cluster.
func Init(storageCallback dcache.StorageCallbacks, cacheId string) error {
	common.Assert(metadataManagerInstance == nil, "MetadataManager Init must be called only once")
	common.Assert(len(cacheId) > 0)

	metadataManagerInstance = &BlobMetadataManager{
		mdRoot:          "__CACHE__" + cacheId, // Set a default cache directory
		storageCallback: storageCallback,       // Initialize storage callback
	}

	_, err := storageCallback.GetPropertiesFromStorage(
		internal.GetAttrOptions{Name: metadataManagerInstance.mdRoot + "/Objects"})
	if err == nil {
		//
		// Node that would have created the Objects folder must have created the Node folder too,
		// so nothing to do.
		//
		log.Info("BlobMetadataManager::Init %s already present, nothing to do!",
			metadataManagerInstance.mdRoot+"/Objects")

		// In debug env, make sure Nodes folder is also present.
		if common.IsDebugBuild() {
			_, err = storageCallback.GetPropertiesFromStorage(
				internal.GetAttrOptions{Name: metadataManagerInstance.mdRoot + "/Nodes"})
			common.Assert(err == nil, err)
		}

		return nil
	}

	//
	// Not-exist failure is fine as this may be the first node of the cluster coming up,
	// any other error is fatal.
	//
	if !os.IsNotExist(err) && err != syscall.ENOENT {
		log.Err("BlobMetadataManager::Init Failed to get properties for %s: %v",
			metadataManagerInstance.mdRoot+"/Objects", err)
		common.Assert(false, err)
		return err
	}

	//
	// Create all the required directories.
	// Create /Objects in the end as other nodes treat presence of /Objects folder to mean all the
	// required folders are there.
	//
	directories := []string{metadataManagerInstance.mdRoot,
		metadataManagerInstance.mdRoot + "/Nodes",
		metadataManagerInstance.mdRoot + "/Objects"}
	for _, dir := range directories {
		if err = storageCallback.CreateDir(
			internal.CreateDirOptions{Name: dir, ForceDirCreationDisabled: true}); err != nil {

			// Already-exists is fine as we may race with some other node, any other error is fatal.
			if !bloberror.HasCode(err, bloberror.BlobAlreadyExists) {
				log.Err("BlobMetadataManager::Init Failed to create directory %s: %v", dir, err)
				common.Assert(false, err)
				return err
			}
			log.Info("BlobMetadataManager::Init Directory %s already exists!", dir)
		} else {
			log.Info("BlobMetadataManager::Init Created directory %s", dir)
		}
	}

	common.Assert(err == nil, "Failed to create directories", err)
	return nil
}

// Package-level functions that delegate to the singleton instance.

func GetMdRoot() string {
	common.Assert(len(metadataManagerInstance.mdRoot) > len("__CACHE__"))
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

// CreateFileInit() creates the initial metadata for a file.
// It ensures that in case two nodes race to create the same file only one succeeds.
// The node that wins the race, then goes ahead writing the data chunks for the file and once done calls
// CreateFileFinalize() to make the file accessible to readers.
//
// TODO: Return etag value to use for CreateFileFinalize() so that we can be assured that the same node
//
//	that started file creation, does the finalize. This can help prevent cases where the initial node
//	went down before finalizing and tries to finalize later
func (m *BlobMetadataManager) createFileInit(filePath string, fileMetadata []byte) error {
	common.Assert(len(filePath) > 0)
	common.Assert(len(fileMetadata) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)

	// The size of the file is set to -1 to represent the file is not finalized.
	sizeStr := "-1"
	metadata := map[string]*string{
		"cache_object_length": &sizeStr,
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Data:                   fileMetadata,
		Metadata:               metadata,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})

	//
	// PutBlobInStorage() can complete with following possible results:
	// 1. Success, blob created
	// 2. Blob already exists
	// 3. Some other failure
	//
	// Both (2) and (3) must be considered failure as CreateFileInit() semantics demand exclusive
	// creation of file metadata blob.
	//
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Err("CreateFileInit:: PutBlobInStorage for %s failed as blob was already present: %v",
				path, err)
			return err
		}

		log.Err("CreateFileInit:: Failed to put blob %s in storage: %v", path, err)
		return err
	}

	log.Debug("CreateFileInit:: Created file %s in storage", path)
	return nil
}

// CreateFileFinalize() finalizes the metadata for a file.
// Must be called only after prior call to CreateFileInit() suceeded.
func (m *BlobMetadataManager) createFileFinalize(filePath string, fileMetadata []byte, fileSize int64) error {
	common.Assert(len(filePath) > 0)
	common.Assert(len(fileMetadata) > 0)
	common.Assert(fileSize >= 0)
	common.Assert(fileSize < (1000 * 1000 * 1000 * common.GbToBytes)) // Sanity check.

	path := filepath.Join(m.mdRoot, "Objects", filePath)

	//
	// In debug env, make sure the metadata file is present, it must have been created by a prior call
	// to CreateFileInit().
	//
	// TODO: See if can do the PutBlobInStorage condtionally with an If-Match condition to fail for cases
	//       where CreateFileFinalize() is incorrectly called without a prior successful CreateFileInit() call.
	//
	if common.IsDebugBuild() {
		prop, err := m.storageCallback.GetPropertiesFromStorage(
			internal.GetAttrOptions{Name: path})
		common.Assert(err == nil, err)

		// Extract the size from the metadata properties.
		size, ok := prop.Metadata["cache_object_length"]
		common.Assert(ok && *size == "-1", ok, *size)
	}

	// Store the open-count and file size in the metadata blob property.
	openCount := "0"
	sizeStr := strconv.FormatInt(fileSize, 10)
	metadata := map[string]*string{
		"opencount":           &openCount,
		"cache_object_length": &sizeStr,
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Data:                   fileMetadata,
		Metadata:               metadata,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})

	if err != nil {
		log.Err("CreateFileFinalize:: Failed to put blob %s in storage: %v", path, err)
		common.Assert(false, err)
		return err
	}

	log.Debug("CreateFileFinalize:: Finalized file %s in storage with size %d bytes", path, fileSize)
	return nil
}

// GetFile reads and returns the content of metadata for a file.
// TODO: Replace the two REST API calls with a single call to DownloadStream.
func (m *BlobMetadataManager) getFile(filePath string) ([]byte, int64, error) {
	common.Assert(len(filePath) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)
	// Get the metadata content from storage.
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: path,
	})
	if err != nil {
		log.Debug("GetFile:: Failed to get metadata file content for file %s: %v", path, err)
		return nil, -1, err
	}

	// Get the file size from the metadata properties.
	prop, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetFile:: Failed to get properties for path %s: %v", path, err)
		common.Assert(false, err)
		return nil, -1, err
	}

	// Extract the size from the metadata properties.
	size, ok := prop.Metadata["cache_object_length"]
	if !ok {
		log.Err("GetFile:: size not found in metadata for path %s", path)
		common.Assert(false, fmt.Sprintf("size not found in metadata for path %s", path))
		return nil, -1, err
	}

	fileSize, err := strconv.ParseInt(*size, 10, 64)
	if err != nil {
		log.Err("GetFile:: Failed to parse size for path %s with value %s: %v", path, *size, err)
		common.Assert(false, err)
		return nil, -1, err
	}

	//
	// Size can be -1 for files which are not in Ready state.
	//

	//if fileSize < 0 {
	if fileSize < -1 {
		log.Warn("GetFile:: Size is negative for path %s: %d", path, fileSize)
		common.Assert(false, "size cannot be negative", fileSize)
		return nil, -1, fmt.Errorf("size is negative for path %s: %d", path, fileSize)
	}

	log.Debug("GetFile:: Size for path %s: %d", path, fileSize)
	return data, fileSize, nil
}

// DeleteFile removes metadata for a file.
func (m *BlobMetadataManager) deleteFile(filePath string) error {
	common.Assert(len(filePath) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: path,
	})
	if err != nil {
		// Treat BlobNotFound as success.
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteFile:: DeleteBlobInStorage failed since blob %s is already deleted: %v",
				path, err)
			return nil
		}

		log.Err("DeleteFile:: Failed to delete blob %s in storage: %v", path, err)
		common.Assert(false, err)
		return err
	}

	log.Debug("DeleteFile:: Deleted blob %s in storage", path)
	return err
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) openFile(filePath string) (int64, error) {
	common.Assert(len(filePath) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)
	count, err := m.updateHandleCount(path, true /* increment */)
	if err != nil {
		log.Err("OpenFile:: Failed to update file open count for path %s: %v", path, err)
		common.Assert(false, err)
		return -1, err
	}

	log.Debug("OpenFile:: Updated file open count for path %s: %d", path, count)
	common.Assert(count > 0, "Open file cannot have count <= 0", count)
	common.Assert(count < 1000000) // Sanity check.

	return count, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) closeFile(filePath string) (int64, error) {
	common.Assert(len(filePath) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)
	count, err := m.updateHandleCount(path, false /* increment */)
	if err != nil {
		log.Err("CloseFile:: Failed to update file open count for path %s: %v", path, err)
		return -1, err
	}

	log.Debug("CloseFile:: Updated file open count for path %s: %d", path, count)
	common.Assert(count >= 0, "File cannot have -ve opencount", count)

	return count, nil
}

// Helper function used by openFile() and closeFile() to atomically update the value of the "opencount"
// metadata variable.
func (m *BlobMetadataManager) updateHandleCount(path string, increment bool) (int64, error) {
	common.Assert(len(path) > 0)

	const maxRetryTime = 1 * time.Minute // Maximum Retry time in minutes
	const maxBackoff = 1 * time.Second   // Maximum backoff time in seconds
	backoff := 1 * time.Millisecond      // Initial backoff time in milliseconds
	var openCount int
	startTime := time.Now()

	for {
		// Get the current open count.
		attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
			Name: path,
		})
		if err != nil {
			log.Err("updateHandleCount:: Failed to get open count for %s: %v", path, err)
			return -1, err
		}

		// We never create file metadata blob w/o opencount property set.
		if attr.Metadata["opencount"] == nil {
			log.Err("updateHandleCount:: File metadata blob found w/o opencount property: %s", path)
			common.Assert(false)
			return -1, fmt.Errorf("[BUG] opencount property not found in metadata for path %s", path)
		}

		openCount, err = strconv.Atoi(*attr.Metadata["opencount"])
		if err != nil {
			// This is unexpected as we always set opencount to an integer value.
			log.Err("GetFileOpenCount:: Failed to parse open count for path %s with value %s: %v",
				path, *attr.Metadata["opencount"], err)
			common.Assert(false, err)
			return -1, err
		}

		if increment {
			openCount++
		} else {
			openCount--
		}

		if openCount < 0 {
			log.Err("updateHandleCount:: open count is negative for path %s: %d", path, openCount)
			common.Assert(false)
			return -1, fmt.Errorf("open count is negative for path %s: %d", path, openCount)
		}

		openCountStr := strconv.Itoa(openCount)
		attr.Metadata["opencount"] = &openCountStr

		// Set the new metadata in storage with etag conditional.
		err = m.storageCallback.SetMetaPropertiesInStorage(internal.SetMetadataOptions{
			Path:      path,
			Metadata:  attr.Metadata,
			Etag:      to.Ptr(azcore.ETag(attr.ETag)),
			Overwrite: true,
		})
		if err != nil {
			if bloberror.HasCode(err, bloberror.ConditionNotMet) {
				log.Warn("updateHandleCount:: SetPropertiesInStorage failed for path %s due to ETag mismatch, retrying...", path)

				// Apply exponential backoff.
				log.Debug("updateHandleCount:: Retrying in %d milliseconds...", backoff)
				time.Sleep(backoff)

				// Double the backoff time, but cap it at maxBackoff.
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				//
				// Multiple nodes trying to read the same file simultaneously may result in
				// etag mismatch failures, few of them is fine but if it fails beyond a reasonable
				// time it indicates some issue, bail out.
				//
				if time.Since(startTime) >= maxRetryTime {
					log.Warn("updateHandleCount:: Retrying exceeded %s for path %s, exiting...",
						maxRetryTime, path)
					common.Assert(false, err)
					return -1, fmt.Errorf("retrying exceeded %s for path %s", maxRetryTime, path)
				}
				continue
			} else {
				log.Err("updateHandleCount:: Failed to update metadata property for path %s: %v",
					path, err)
				common.Assert(false, err)
				return -1, err
			}
		} else {
			log.Debug("updateHandleCount:: Updated metadata property for path %s: %d", path, openCount)
			break
		}
	}

	return int64(openCount), nil
}

// GetFileOpenCount returns the current open count for a file.
func (m *BlobMetadataManager) getFileOpenCount(filePath string) (int64, error) {
	common.Assert(len(filePath) > 0)

	path := filepath.Join(m.mdRoot, "Objects", filePath)
	prop, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetFileOpenCount:: Failed to get properties for path %s: %v", path, err)
		return -1, err
	}

	openCount, ok := prop.Metadata["opencount"]
	if !ok {
		log.Err("GetFileOpenCount:: openCount not found in metadata for path %s", path)
		common.Assert(false, fmt.Sprintf("openCount not found in metadata for path %s", path))
		return -1, err
	}

	count, err := strconv.Atoi(*openCount)
	if err != nil {
		log.Err("GetFileOpenCount:: Failed to parse open count for path %s with value %s: %v",
			path, *openCount, err)
		common.Assert(false, err)
		return -1, err
	}

	common.Assert(count >= 0, fmt.Sprintf("GetHandleCount:: Open count is negative for path %s: %d", path, count))

	log.Debug("GetFileOpenCount:: Open count for path %s: %d", path, count)
	return int64(count), nil
}

// UpdateHeartbeat creates or updates the heartbeat file.
func (m *BlobMetadataManager) updateHeartbeat(nodeId string, data []byte) error {
	common.Assert(common.IsValidUUID(nodeId))
	common.Assert(len(data) > 0)

	// Create the heartbeat file path.
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   heartbeatFilePath,
		Data:                   data,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("UpdateHeartbeat:: Failed to put heartbeat blob path %s in storage: %v", heartbeatFilePath, err)
		common.Assert(false, fmt.Sprintf("Failed to put heartbeat blob path %s in storage: %v",
			heartbeatFilePath, err))
		return err
	}

	log.Debug("UpdateHeartbeat:: Updated heartbeat blob path %s in storage", heartbeatFilePath)
	return nil
}

// DeleteHeartbeat deletes the heartbeat file
func (m *BlobMetadataManager) deleteHeartbeat(nodeId string) error {
	common.Assert(common.IsValidUUID(nodeId))

	// Create the heartbeat file path.
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	err := m.storageCallback.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: heartbeatFilePath,
	})
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			log.Err("DeleteHeartbeat:: DeleteBlobInStorage failed since blob %s is already deleted: %v",
				heartbeatFilePath, err)
		} else {
			log.Err("DeleteHeartbeat:: Failed to delete heartbeat blob %s in storage: %v",
				heartbeatFilePath, err)
			common.Assert(false, err)
		}
		return err
	}

	log.Debug("DeleteHeartbeat:: Deleted heartbeat blob %s in storage", heartbeatFilePath)
	return nil
}

// GetHeartbeat reads and returns the content of the heartbeat file.
func (m *BlobMetadataManager) getHeartbeat(nodeId string) ([]byte, error) {
	common.Assert(common.IsValidUUID(nodeId))

	// Create the heartbeat file path
	heartbeatFilePath := filepath.Join(m.mdRoot, "Nodes", nodeId+".hb")
	// Get the heartbeat content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: heartbeatFilePath,
	})
	if err != nil {
		log.Err("GetHeartbeat:: Failed to get heartbeat file content for %s: %v", heartbeatFilePath, err)
		common.Assert(false, fmt.Sprintf("Failed to get heartbeat file content for %s: %v",
			heartbeatFilePath, err))
		return nil, err
	}

	common.Assert(len(data) > 0)

	log.Debug("GetHeartbeat:: Successfully got heartbeat file content for %s, %d bytes",
		heartbeatFilePath, len(data))
	return data, nil
}

// GetAllNodes enumerates and returns the list of all nodes that have ever punched a heartbeat.
func (m *BlobMetadataManager) getAllNodes() ([]string, error) {
	path := filepath.Join(m.mdRoot, "Nodes")
	list, err := m.storageCallback.ReadDirFromStorage(internal.ReadDirOptions{
		Name: path,
	})
	if err != nil {
		log.Err("GetAllNodes:: Failed to enumerate nodes list from %s: %v", path, err)
		common.Assert(false, fmt.Sprintf("Failed to enumerate nodes list from %s: %v", path, err))
		return nil, err
	}

	// Extract the node IDs from the list of blobs.
	var nodes []string
	for _, blob := range list {
		log.Debug("GetAllNodes:: Found blob: %s", blob.Name)

		if strings.HasSuffix(blob.Name, ".hb") {
			nodeId := blob.Name[:len(blob.Name)-3] // Remove the ".hb" extension
			if common.IsValidUUID(nodeId) {
				nodes = append(nodes, nodeId)
			} else {
				log.Err("Invalid heartbeat blob: %s", blob.Name)
				common.Assert(false, "Invalid heartbeat blob", blob.Name)
			}
		} else {
			log.Warn("GetAllNodes:: Unexpected blob found in Nodes folder: %s", blob.Name)
			common.Assert(false, "Unexpected blob found in Nodes folder", blob.Name)
		}
	}

	log.Debug("GetAllNodes:: Found %d nodes", len(nodes))
	return nodes, nil
}

// CreateInitialClusterMap creates the initial cluster map.
func (m *BlobMetadataManager) createInitialClusterMap(clustermap []byte) error {
	common.Assert(len(clustermap) > 0)

	// Create the clustermap file path.
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: true,
		EtagMatchConditions:    "",
	})

	//
	// TODO:
	// Caller has to check if the error is ConditionNotMet or something else
	// and take appropriate action.
	// If the error is ConditionNotMet, it means the clustermap already exists
	// and the caller should not overwrite it.
	// For now we treat "already exists" as success.
	//
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Info("CreateInitialClusterMap:: PutBlobInStorage failed for %s due to ETag mismatch, treating as success: %v",
				clustermapPath, err)
			return nil
		}

		log.Err("CreateInitialClusterMap:: Failed to put blob %s in storage: %v", clustermapPath, err)
		common.Assert(false, err)
		return err
	}

	log.Info("CreateInitialClusterMap:: Created initial clustermap with path %s", clustermapPath)
	return nil
}

// UpdateClusterMapStart claims update ownership of the cluster map.
func (m *BlobMetadataManager) updateClusterMapStart(clustermap []byte, etag *string) error {
	common.Assert(len(clustermap) > 0)
	common.Assert(etag != nil)

	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")

	// In debug env make sure clustermap.json is already present.
	// Caller must call us only to update an existing clustermap.json.
	if common.IsDebugBuild() {
		_, err := m.storageCallback.GetPropertiesFromStorage(
			internal.GetAttrOptions{Name: clustermapPath})
		common.Assert(err == nil, err)
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    *etag,
	})

	//
	// Caller should add a check to identify the error is ConditionNotMet or something else
	// and take appropriate action.
	// If the error is ConditionNotMet, it means the clustermap is already being updated
	// and the caller should not overwrite it.
	//
	if err != nil {
		if bloberror.HasCode(err, bloberror.ConditionNotMet) {
			log.Warn("UpdateClusterMapStart:: ETag mismatch some other node has taken ownership for updating clustermap with path %s, etag %s: %v", clustermapPath, *etag, err)
		} else {
			log.Err("UpdateClusterMapStart:: Failed to update clustermap %s: %v", clustermapPath, err)
			common.Assert(false, err)
		}
	}

	log.Debug("UpdateClusterMapStart:: Updated clustermap with path %s", clustermapPath)
	return err
}

// UpdateClusterMapEnd finalizes the cluster map update.
// TODO: For safe update updateClusterMapStart() should return a Etag which must be passed to updateClusterMapEnd()
func (m *BlobMetadataManager) updateClusterMapEnd(clustermap []byte) error {
	common.Assert(len(clustermap) > 0)

	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")

	// In debug env make sure clustermap.json is already present.
	// Caller must call us only to update an existing clustermap.json.
	if common.IsDebugBuild() {
		_, err := m.storageCallback.GetPropertiesFromStorage(
			internal.GetAttrOptions{Name: clustermapPath})
		common.Assert(err == nil, err)
	}

	err := m.storageCallback.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   clustermapPath,
		Data:                   clustermap,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Err("UpdateClusterMapEnd:: Failed to finalize clustermap update for %s: %v", clustermapPath, err)
		common.Assert(false, err)
		return err
	}

	log.Debug("UpdateClusterMapEnd:: Finalized clustermap update for %s", clustermapPath)
	return nil
}

// GetClusterMap reads and returns the content of the cluster map as a byte array, the current Etag value and error if any
func (m *BlobMetadataManager) getClusterMap() ([]byte, *string, error) {
	clustermapPath := filepath.Join(m.mdRoot, "clustermap.json")
	attr, err := m.storageCallback.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: clustermapPath,
	})
	if err != nil {
		log.Err("GetClusterMap:: Failed to get cluster map properties %s: %v", clustermapPath, err)
		return nil, nil, err
	}

	// Must have a valid etag.
	common.Assert(len(attr.ETag) > 0)

	// TODO:: If some node updates the clustermap between GetProperties and GetBlobFromStorage
	// then updateClusterMapStart will fail with ETag mismatch.
	// In that case we can create a new function to call doawnloadStream directly for content and etag.
	// Get the cluster map content from storage
	data, err := m.storageCallback.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: clustermapPath,
	})
	if err != nil {
		log.Err("GetClusterMap:: Failed to get cluster map content with path %s: %v", clustermapPath, err)
		common.Assert(false, err)
		return nil, nil, err
	}

	log.Debug("GetClusterMap:: Successfully got cluster map content for %s, %d bytes, Etag: %s",
		clustermapPath, len(data), attr.ETag)

	// Return the cluster map content and ETag
	return data, &attr.ETag, nil
}
