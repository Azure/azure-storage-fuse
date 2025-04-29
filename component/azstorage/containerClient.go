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

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

type ContainerClient interface {
	getContainerClient(path string) *container.Client
	updateContainerClient()
}

type baseContainerClient struct {
	containerName string
	serviceClient *service.Client
}

// ----------------------------------------------------------------------------------------------
type SingleContainerClient struct {
	baseContainerClient
	containerClient *container.Client
}

func (scc *SingleContainerClient) getContainerClient(path string) *container.Client {
	return scc.containerClient
}

func (scc *SingleContainerClient) updateContainerClient() {
	scc.containerClient = scc.serviceClient.NewContainerClient(scc.containerName)
}

// ----------------------------------------------------------------------------------------------

type MultiContainerClient struct {
	baseContainerClient
	containerClients map[string]*container.Client
	rwmtx            sync.Mutex
}

func (mcc *MultiContainerClient) getContainerClient(path string) *container.Client {
	mcc.rwmtx.Lock()
	defer mcc.rwmtx.Unlock()

	client, ok := mcc.containerClients[path]
	if ok && client != nil {
		return client
	}

	// Extract the first part of the path seperated by '/' to get the container name
	paths := strings.Split(path, "/")
	if len(paths) == 0 {
		return nil
	}

	client = mcc.serviceClient.NewContainerClient(paths[0])
	mcc.containerClients[path] = client
	return client
}

func (mcc *MultiContainerClient) updateContainerClient() {
	mcc.rwmtx.Lock()
	defer mcc.rwmtx.Unlock()

	for name := range mcc.containerClients {
		mcc.containerClients[name] = mcc.serviceClient.NewContainerClient(name)
	}
}

// ----------------------------------------------------------------------------------------------

func newContainerClient(svc *service.Client, containerName string) ContainerClient {
	if containerName == "" {
		return &MultiContainerClient{
			baseContainerClient: baseContainerClient{
				serviceClient: svc,
			},
			containerClients: make(map[string]*container.Client),
		}
	}

	return &SingleContainerClient{
		baseContainerClient: baseContainerClient{
			containerName: containerName,
			serviceClient: svc,
		},
		containerClient: svc.NewContainerClient(containerName),
	}
}
