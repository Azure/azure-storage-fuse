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

package rpc_server

import (
	"context"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
)

// type check to ensure that ChunkServiceHandler implements dcache.ChunkService interface
var _ service.ChunkService = &ChunkServiceHandler{}

// ChunkServiceHandler struct implements the ChunkService interface
type ChunkServiceHandler struct {
	locks *common.LockMap
	// more fields can be added here as needed
}

func NewChunkServiceHandler() *ChunkServiceHandler {
	return &ChunkServiceHandler{
		locks: common.NewLockMap(),
	}
}

func (h *ChunkServiceHandler) Hello(ctx context.Context, req *models.HelloRequest) (*models.HelloResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) RemoveChunk(ctx context.Context, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) JoinMV(ctx context.Context, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) LeaveMV(ctx context.Context, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) StartSync(ctx context.Context, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	return nil, nil
}

func (h *ChunkServiceHandler) EndSync(ctx context.Context, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	return nil, nil
}
