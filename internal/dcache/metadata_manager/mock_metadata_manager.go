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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/stretchr/testify/mock"
)

// Make sure MockMetaDataManager implements the MetadataManager interface.
// The real interface might look like:
// type MetadataManager interface {
//     GetClusterMap() ([]byte, *azcore.ETag, error)
//     CreateInitialClusterMap(clusterMap []byte) error
//     // Other methods...
// }

type MockMetaDataManager struct {
	mock.Mock
}

// NewMockMetaDataManager creates a new instance of the mock object.
func NewMockMetaDataManager() *MockMetaDataManager {
	return &MockMetaDataManager{}
}

// GetClusterMap mocks the real GetClusterMap method in MetadataManager.
func (m *MockMetaDataManager) GetClusterMap() ([]byte, *azcore.ETag, error) {
	args := m.Called()
	return args.Get(0).([]byte), nil, args.Error(2)
}

// CreateInitialClusterMap is another mocked method, if needed.
func (m *MockMetaDataManager) CreateInitialClusterMap(clusterMap []byte) error {
	args := m.Called(clusterMap)
	return args.Error(0)
}

// Below are example stubs for common MetadataManager methods.
// Update them if your tests need to mock their behaviors.

func (m *MockMetaDataManager) UpdateClusterMapStart(clustermap []byte, etag *azcore.ETag) error {
	args := m.Called(clustermap, etag)
	return args.Error(0)
}

func (m *MockMetaDataManager) UpdateClusterMapEnd(clustermap []byte) error {
	args := m.Called(clustermap)
	return args.Error(0)
}

// If your interface has other methods, add them here similarly.
func (m *MockMetaDataManager) CreateFileInit(filePath string, fileMetadata *dcache.FileMetadata) error {
	panic("not implemented")
}

func (m *MockMetaDataManager) CreateFileFinalize(filePath string, fileMetadata *dcache.FileMetadata) error {
	panic("not implemented")
}

func (m *MockMetaDataManager) GetFile(filePath string) (*dcache.FileMetadata, error) {
	panic("not implemented")
}

func (m *MockMetaDataManager) DeleteFile(filePath string) error {
	panic("not implemented")
}

func (m *MockMetaDataManager) OpenFile(filePath string) (int64, error) {
	panic("not implemented")
}

func (m *MockMetaDataManager) CloseFile(filePath string) (int64, error) {
	panic("not implemented")
}

func (m *MockMetaDataManager) GetFileOpenCount(filePath string) (int64, error) {
	panic("not implemented")
}

func (m *MockMetaDataManager) UpdateHeartbeat(nodeId string, data []byte) error {
	panic("not implemented")
}

func (m *MockMetaDataManager) DeleteHeartbeat(nodeId string, data []byte) error {
	panic("not implemented")
}

func (m *MockMetaDataManager) GetHeartbeat(nodeId string) ([]byte, error) {
	panic("not implemented")
}

func (m *MockMetaDataManager) GetAllNodes() ([]string, error) {
	panic("not implemented")
}
