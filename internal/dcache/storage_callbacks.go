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

package dcache

import "github.com/Azure/azure-storage-fuse/v2/internal"

// Storage callback defines the interface for managing storage/nextComponent related APIs.
type StorageCallbacks interface {

	//It will Delete the blob in storage
	DeleteBlobInStorage(opt internal.DeleteFileOptions) error

	//It will Get the blob from storage
	GetBlobFromStorage(opt internal.ReadFileWithNameOptions) ([]byte, error)

	//It will Get the properties of the blob from storage
	GetPropertiesFromStorage(opt internal.GetAttrOptions) (*internal.ObjAttr, error)

	//It will Put the blob in storage
	PutBlobInStorage(opt internal.WriteFromBufferOptions) error

	//It will Read the directory from storage
	ReadDirFromStorage(options internal.ReadDirOptions) ([]*internal.ObjAttr, error)

	//It will Set the properties of the blob in storage
	SetMetaPropertiesInStorage(options internal.SetMetadataOptions) error

	//It will Delete the blob through next Component whichever is in pipeline
	DeleteBlob(opt internal.DeleteFileOptions) error

	//It will Get the blob through next Component whichever is in pipeline
	GetBlob(opt internal.ReadFileWithNameOptions) ([]byte, error)

	//It will Get the properties of the blob through next Component whichever is in pipeline
	GetProperties(opt internal.GetAttrOptions) (*internal.ObjAttr, error)

	//It will Put the blob through next Component whichever is in pipeline
	PutBlob(opt internal.WriteFromBufferOptions) error

	//It will Read the directory through next Component whichever is in pipeline
	ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error)

	//It will Set the properties of the blob through next Component whichever is in pipeline
	SetMetaProperties(options internal.SetMetadataOptions) error

	//It will Create the directory through next Component whichever is in pipeline
	CreateDir(options internal.CreateDirOptions) error

	//It will Create the directory in storage
	CreateDirInStorage(options internal.CreateDirOptions) error
}
