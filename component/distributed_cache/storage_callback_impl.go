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
package distributedcache

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"
)

// StorageCallbackImpl is a struct that implements the Storage interface
type StorageCallbackImpl struct {
	comp      internal.Component
	azstorage internal.Component
}

// Implement the GetBlob method
func (s *StorageCallbackImpl) GetBlob(blobName string) (string, error) {
	return "", nil
}

// Implement the PutBlob method
func (s *StorageCallbackImpl) PutBlob(blobName string, data string) error {
	return nil
}

// Implement the GetProperties method
func (s *StorageCallbackImpl) GetProperties(blobName string) (map[string]string, error) {
	return map[string]string{}, nil
}

// Implement the SetProperties method
func (s *StorageCallbackImpl) SetProperties(blobName string, properties map[string]string) error {
	return nil
}

// Implement the ListBlobs method
func (s *StorageCallbackImpl) ListAllBlobs(path string) ([]string, error) {
	return []string{}, nil
}

// Factory function to create a new instance of StorageImpl
func newStorageImpl(nextComp internal.Component, azstorage internal.Component) dcachelib.StorageCallbacks {

	return &StorageCallbackImpl{
		comp:      nextComp,
		azstorage: azstorage,
	}
}
