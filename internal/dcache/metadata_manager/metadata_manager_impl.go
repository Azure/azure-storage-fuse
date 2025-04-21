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
	"path/filepath"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

// BlobMetadataManager is the implementation of MetadataManager interface
type BlobMetadataManager struct {
	cacheDir         string
	storageCallbacks dcache.StorageCallbacks
}

// NewMetadataManager creates a new implementation of the MetadataManager interface
func NewMetadataManager(cacheDir string) (*BlobMetadataManager, error) {
	return &BlobMetadataManager{
		cacheDir: cacheDir,
	}, nil
}

// CreateFileInit creates the initial metadata for a file
// TODO :: Return etag value to use for CreateFileFinalize
// Can help ensure cases where the initial node went down before finalizing and tried to finalize later
func (m *BlobMetadataManager) CreateFileInit(filePath string, fileMetadata []byte) error {
	path := filepath.Join(m.cacheDir, "Objects", filePath)
	// Store the open-count in the metadata blob property
	openCount := "0"
	metadata := map[string]*string{
		"opencount": &openCount,
	}

	err := m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
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
		log.Debug("CreateFileInit :: Failed to put blob %s in storage: %v", path, err)
	}
	return err
}

// CreateFileFinalize finalizes the metadata for a file
func (m *BlobMetadataManager) CreateFileFinalize(filePath string, fileMetadata *dcache.FileMetadata) error {
	path := filepath.Join(m.cacheDir, "Objects", filePath)
	// Convert the metadata to JSON
	jsonData, err := json.MarshalIndent(fileMetadata, "", "  ")
	if err != nil {
		log.Debug("CreateFileFinalize :: Failed to marshal metadata to JSON: %v", err)
		return err
	}
	// TODO :: check metadata is not overwritten byt this
	err = m.storageCallbacks.PutBlobInStorage(internal.WriteFromBufferOptions{
		Name:                   path,
		Data:                   jsonData,
		IsNoneMatchEtagEnabled: false,
		EtagMatchConditions:    "",
	})
	if err != nil {
		log.Debug("CreateFileFinalize :: Failed to put blob %s in storage: %v", path, err)
	}
	return err
}

// GetFile reads and returns the content of metadata for a file
func (m *BlobMetadataManager) GetFile(filePath string) (*dcache.FileMetadata, error) {
	path := filepath.Join(m.cacheDir, "Objects", filePath)
	// Get the metadata content from storage
	data, err := m.storageCallbacks.GetBlobFromStorage(internal.ReadFileWithNameOptions{
		Path: path,
	})
	if err != nil {
		log.Debug("GetFile :: Failed to get metadata file content: %v", err)
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
func (m *BlobMetadataManager) DeleteFile(filePath string) error {
	err := m.storageCallbacks.DeleteBlobInStorage(internal.DeleteFileOptions{
		Name: filepath.Join(m.cacheDir, "Objects", filePath),
	})
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			log.Warn("DeleteFile :: DeleteBlobInStorage failed since blob is already deleted")
			return nil
		}
	}
	return err
}

// OpenFile increments the open count for a file and returns the updated count
func (m *BlobMetadataManager) OpenFile(filePath string) (int64, error) {
	count, err := m.updateHandleCount(filePath, true)
	if err != nil {
		log.Debug("OpenFile :: Failed to update file open count: %v", err)
		return -1, err
	}
	return count, nil
}

// CloseFile decrements the open count for a file and returns the updated count
func (m *BlobMetadataManager) CloseFile(filePath string) (int64, error) {
	count, err := m.updateHandleCount(filePath, false)
	if err != nil {
		log.Debug("CloseFile :: Failed to update file close count: %v", err)
		return -1, err
	}
	return count, nil
}

func (m *BlobMetadataManager) updateHandleCount(filePath string, increment bool) (int64, error) {
	const maxBackoff = 30           // Maximum backoff time in seconds
	backoff := 1 * time.Millisecond // Initial backoff time in milliseconds
	var openCount int
	for {
		// Get the current handle count
		attr, err := m.storageCallbacks.GetPropertiesFromStorage(internal.GetAttrOptions{
			Name: filepath.Join(m.cacheDir, "Objects", filePath),
		})
		if err != nil {
			log.Debug("updateHandleCount :: Failed to get handle count: %v", err)
			return -1, err
		}

		if attr.Metadata["opencount"] != nil {
			openCount, err = strconv.Atoi(*attr.Metadata["opencount"])
			if err != nil {
				log.Debug("GetFileOpenCount :: Failed to parse handle count: %v", err)
				return -1, err
			}
			if increment {
				openCount++
			} else {
				openCount--
			}
			if openCount < 0 {
				openCount = 0
			}
			openCountStr := strconv.Itoa(openCount)
			attr.Metadata["opencount"] = &openCountStr
		}

		// Set the new metadata in storage
		err = m.storageCallbacks.SetMetaPropertiesInStorage(internal.SetMetadataOptions{
			Path:      filepath.Join(m.cacheDir, filePath),
			Metadata:  attr.Metadata,
			Etag:      to.Ptr(azcore.ETag(attr.ETag)),
			Overwrite: true,
		})
		if err != nil {
			if bloberror.HasCode(err, bloberror.ConditionNotMet) {
				log.Warn("updateHandleCount :: SetPropertiesInStorage failed due to ETag mismatch, retrying...")

				// Apply exponential backoff
				log.Debug("updateHandleCount :: Retrying in %d milliseconds...", backoff)
				time.Sleep(time.Duration(backoff) * time.Millisecond)

				// Double the backoff time, but cap it at maxBackoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			} else {
				log.Debug("updateHandleCount :: Failed to update metadata property: %v", err)
				return -1, err
			}
		} else {
			break
		}
	}
	return int64(openCount), nil
}

// GetFileOpenCount returns the current open count for a file
func (m *BlobMetadataManager) GetFileOpenCount(filePath string) (int64, error) {
	prop, err := m.storageCallbacks.GetPropertiesFromStorage(internal.GetAttrOptions{
		Name: filepath.Join(m.cacheDir, "Objects", filePath),
	})
	if err != nil {
		log.Debug("GetFileOpenCount :: Failed to get handle count: %v", err)
		return -1, err
	}
	openCount, ok := prop.Metadata["opencount"]
	if !ok {
		log.Debug("GetFileOpenCount :: openCount not found in metadata")
		return -1, err
	}
	count, err := strconv.ParseInt(*openCount, 10, 64)
	if err != nil {
		log.Debug("GetFileOpenCount :: Failed to parse handle count: %v", err)
		return -1, err
	}
	if count < 0 {
		log.Warn("GetHandleCount :: Handle count is negative")
	}

	return count, nil
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
