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

package azstorage

import (
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
)

type FilesystemClient interface {
	getFilesystemClient(path string) *filesystem.Client
	updateFilesystemClient()
}

type baseFilesystemClient struct {
	containerName string
	serviceClient *service.Client
}

// ----------------------------------------------------------------------------------------------
type SingleFilesystemClient struct {
	baseFilesystemClient
	FilesystemClient *filesystem.Client
}

func (scc *SingleFilesystemClient) getFilesystemClient(_ string) *filesystem.Client {
	return scc.FilesystemClient
}

func (scc *SingleFilesystemClient) updateFilesystemClient() {
	scc.FilesystemClient = scc.serviceClient.NewFileSystemClient(scc.containerName)
}

// ----------------------------------------------------------------------------------------------

type MultiFilesystemClient struct {
	baseFilesystemClient
	FilesystemClients map[string]*filesystem.Client
	rwmtx             sync.Mutex
}

func (mcc *MultiFilesystemClient) getFilesystemClient(path string) *filesystem.Client {
	mcc.rwmtx.Lock()
	defer mcc.rwmtx.Unlock()

	client, ok := mcc.FilesystemClients[path]
	if ok && client != nil {
		return client
	}

	// Extract the first part of the path seperated by '/' to get the container name
	paths := strings.Split(path, "/")
	if len(paths) == 0 {
		return nil
	}

	client = mcc.serviceClient.NewFileSystemClient(paths[0])
	mcc.FilesystemClients[path] = client
	return client
}

func (mcc *MultiFilesystemClient) updateFilesystemClient() {
	mcc.rwmtx.Lock()
	defer mcc.rwmtx.Unlock()

	for name := range mcc.FilesystemClients {
		mcc.FilesystemClients[name] = mcc.serviceClient.NewFileSystemClient(name)
	}
}

// ----------------------------------------------------------------------------------------------

func newFileSystemClient(svc *service.Client, containerName string) FilesystemClient {
	if containerName == "" {
		return &MultiFilesystemClient{
			baseFilesystemClient: baseFilesystemClient{
				serviceClient: svc,
			},
			FilesystemClients: make(map[string]*filesystem.Client),
		}
	}

	return &SingleFilesystemClient{
		baseFilesystemClient: baseFilesystemClient{
			containerName: containerName,
			serviceClient: svc,
		},
		FilesystemClient: svc.NewFileSystemClient(containerName),
	}
}
