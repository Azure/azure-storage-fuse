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
package distributed_cache

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	. "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/api"
)

// StorageCallbackImpl is a struct that implements the Storage interface
type StorageCallbackImpl struct {
	nextComp internal.Component
	storage  internal.Component
}

// GetBlob implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) GetBlob(options internal.ReadFileWithNameOptions) ([]byte, error) {
	return sci.nextComp.ReadFileWithName(options)
}

// GetBlobFromStroage implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) GetBlobFromStroage(options internal.ReadFileWithNameOptions) ([]byte, error) {
	return sci.storage.ReadFileWithName(options)
}

// GetProperties implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) GetProperties(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	return sci.nextComp.GetAttr(options)
}

func (sci *StorageCallbackImpl) GetPropertiesFromStorage(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	return sci.storage.GetAttr(options)
}

// ReadDir implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	return sci.nextComp.ReadDir(options)
}

// ReadDirFromStroage implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) ReadDirFromStroage(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	return sci.storage.ReadDir(options)
}

// SetProperties implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) SetProperties(path string, properties map[string]string) error {
	panic("unimplemented")
}

// SetPropertiesInStorage implements dcachelib.StorageCallbacks.
func (sci *StorageCallbackImpl) SetPropertiesInStorage(path string, properties map[string]string) error {
	panic("unimplemented")
}

func (sci *StorageCallbackImpl) PutBlobInStorage(options internal.WriteFromBufferOptions) error {
	return sci.storage.WriteFromBuffer(options)
}

func (sci *StorageCallbackImpl) PutBlob(options internal.WriteFromBufferOptions) error {
	return sci.nextComp.WriteFromBuffer(options)
}

// Factory function to create a new instance of StorageCallbacks
func initStorageCallback(nextComp internal.Component, azstorage internal.Component) StorageCallbacks {

	return &StorageCallbackImpl{
		nextComp: nextComp,
		storage:  azstorage,
	}
}
