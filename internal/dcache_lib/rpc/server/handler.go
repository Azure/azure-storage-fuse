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

package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache/models"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache/service"
)

// type check to ensure that ChunkServiceHandler implements dcache.ChunkService interface
var _ service.ChunkService = &ChunkServiceHandler{}

// ChunkServiceHandler struct implements the ChunkService interface
type ChunkServiceHandler struct {
	locks   *common.LockMap
	fsIDMap map[string]*RVInfo // map to store the fsID to RVInfo mapping
	// more fields can be added here as needed
}

type RVInfo struct {
	rvID     string             // rv0, rv1, etc.
	cacheDir string             // cache dir path for this RV
	mvMap    map[string]*MVInfo // map of MV id against MV info
}

type MVInfo struct {
	peerRVs []string // peer RVs for this MV
}

func NewChunkServiceHandler() *ChunkServiceHandler {
	// TODO: get fsID, rvID and cache dir path for different RVs for the node from cluster manager
	// below will be call to cluster manager to get the information
	fsIDMap := make(map[string]*RVInfo)

	return &ChunkServiceHandler{
		locks:   common.NewLockMap(),
		fsIDMap: fsIDMap,
	}
}

// check if the given mv is valid
func (rv *RVInfo) isMvValid(mvPath string) error {
	if !common.DirectoryExists(mvPath) {
		return fmt.Errorf("MV path %s does not exist", mvPath)
	}

	mvID := filepath.Base(mvPath)
	mvInfo, ok := rv.mvMap[mvID]
	if !ok || mvInfo == nil {
		return fmt.Errorf("MV %s is not valid", mvID)
	}

	return nil
}

func (rv *RVInfo) getPeerRVs(mvID string) []string {
	mvInfo, ok := rv.mvMap[mvID]
	if !ok || mvInfo == nil {
		return nil
	}
	return mvInfo.peerRVs
}

// sample method; will be later removed after integrating with cluster manager
func getChunkSize() int64 {
	return 4 * 1024 * 1024 // 4MB
}

// check the if the chunk address is valid
// - check if the fsID is valid
// - check if the cache dir exists
// - check if the MV is valid
func (h *ChunkServiceHandler) checkValidChunkAddress(address *models.Address) error {
	if address == nil || address.FileID == "" || address.FsID == "" || address.MvID == "" {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid chunk address")
		return rpc.NewResponseError(rpc.InvalidRequest, "invalid chunk address")
	}

	// check if the fsID is valid
	rvInfo, ok := h.fsIDMap[address.FsID]
	if !ok || rvInfo == nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid fsID %s", address.FsID)
		return rpc.NewResponseError(rpc.InvalidFSID, fmt.Sprintf("invalid fsID %s", address.FsID))
	}

	cacheDir := rvInfo.cacheDir
	if cacheDir == "" || !common.DirectoryExists(cacheDir) {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Cache dir not found for RV %s", rvInfo.rvID)
		return rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("cache dir not found for RV %s", rvInfo.rvID))
	}

	// check if the MV is valid
	mvPath := filepath.Join(cacheDir, address.MvID)
	if err := rvInfo.isMvValid(mvPath); err != nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: MV %s is not valid [%s]", address.MvID, err.Error())
		return rpc.NewResponseError(rpc.InvalidMV, fmt.Sprintf("MV %s is not valid [%s]", address.MvID, err.Error()))
	}

	return nil
}

func (h *ChunkServiceHandler) Hello(ctx context.Context, req *models.HelloRequest) (*models.HelloResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::Hello: Received nil Hello request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil Hello request")
	}

	log.Debug("ChunkServiceHandler::Hello: Received Hello request from %s to %s at %v, sender's RV = %v, shared MV = %v", req.SenderNodeID, req.ReceiverNodeID, req.Time, req.RV, req.MV)

	// get the RV list from the fsIDMap
	rvList := make([]string, 0)
	for _, info := range h.fsIDMap {
		rvList = append(rvList, info.rvID)
	}

	return &models.HelloResponse{
		ReceiverNodeID: req.ReceiverNodeID,
		Time:           time.Now().UnixMicro(),
		RV:             rvList,
		MV:             req.MV,
	}, nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	if req == nil || req.Address == nil {
		log.Err("ChunkServiceHandler::GetChunk: Received nil GetChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil GetChunk request")
	}

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	startTime := time.Now()

	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.FsID, req.Address.MvID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request for chunk address %v, offset within chunk %v, length %v", chunkAddress, req.Offset, req.Length)

	// check if the chunk file is being written to in parallel by some other thread
	isLocked := h.locks.Locked(chunkAddress)
	if isLocked {
		log.Err("ChunkServiceHandler::GetChunk: chunk %v is being written", chunkAddress)
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("chunk %v is being written", chunkAddress))
	}

	rvInfo := h.fsIDMap[req.Address.FsID]
	cacheDir := rvInfo.cacheDir
	chunkPath, hashPath := getChunkAndHashPath(cacheDir, req.Address.MvID, req.Address.FileID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::GetChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	fh, err := os.Open(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to open chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to open chunk file %s [%v]", chunkPath, err.Error()))
	}
	defer fh.Close()

	// TODO: call cluster manager to get chunk size
	chunkSize := getChunkSize()

	if req.Length == -1 {
		req.Length = chunkSize - req.Offset
	}

	data := make([]byte, req.Length)
	_, err = fh.ReadAt(data, req.Offset)
	// TODO: should we handle EOF error
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to read chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to read chunk file %s [%v]", chunkPath, err.Error()))
	}

	// get hash if requested for entire chunk
	hash := ""
	if req.Offset == 0 && req.Length == chunkSize {
		hashData, err := os.ReadFile(hashPath)
		if err != nil {
			log.Err("ChunkServiceHandler::GetChunk: Failed to read hash file %s [%v]", hashPath, err.Error())
			return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to read hash file %s [%v]", hashPath, err.Error()))
		}
		hash = string(hashData)
	}

	lmt, err := getLMT(fh)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to get LMT for chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to get LMT for chunk file %s [%v]", chunkPath, err.Error()))
	}

	resp := &models.GetChunkResponse{
		Chunk: &models.Chunk{
			Address: req.Address,
			Data:    data,
			Hash:    hash,
		},
		ChunkWriteTime: lmt,
		TimeTaken:      time.Since(startTime).Microseconds(),
		PeerRV:         rvInfo.getPeerRVs(req.Address.MvID),
	}

	return resp, nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	if req == nil || req.Chunk == nil || req.Chunk.Address == nil {
		log.Err("ChunkServiceHandler::PutChunk: Received nil PutChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil PutChunk request")
	}

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Chunk.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	startTime := time.Now()

	chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.FsID, req.Chunk.Address.MvID, req.Chunk.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request for chunk address %v, length %v, isSync %v", chunkAddress, req.Length, req.IsSync)

	// acquire lock for the chunk address
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	rvInfo := h.fsIDMap[req.Chunk.Address.FsID]
	cacheDir := rvInfo.cacheDir

	chunkPath, hashPath := getRegularMVPath(cacheDir, req.Chunk.Address.MvID, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
	if req.IsSync {
		chunkPath, hashPath = getSyncMVPath(cacheDir, req.Chunk.Address.MvID, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
	}
	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	err = os.WriteFile(chunkPath, req.Chunk.Data, 0400)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to write chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to write chunk file %s [%v]", chunkPath, err.Error()))
	}

	err = os.WriteFile(hashPath, []byte(req.Chunk.Hash), 0400)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to write hash file %s [%v]", hashPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to write hash file %s [%v]", hashPath, err.Error()))
	}

	availableSpace, err := common.GetAvailableDiskSpaceFromStatfs(cacheDir)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err.Error())
	}

	// TODO: should we verify the hash after writing the chunk

	resp := &models.PutChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		AvailableSpace: availableSpace,
		PeerRV:         rvInfo.getPeerRVs(req.Chunk.Address.MvID),
	}

	return resp, nil
}

func (h *ChunkServiceHandler) RemoveChunk(ctx context.Context, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	if req == nil || req.Address == nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Received nil RemoveChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil RemoveChunk request")
	}

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	startTime := time.Now()

	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.FsID, req.Address.MvID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::RemoveChunk: Received RemoveChunk request for chunk address %v", chunkAddress)

	// acquire lock for the chunk address
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	rvInfo := h.fsIDMap[req.Address.FsID]
	cacheDir := rvInfo.cacheDir

	// TODO: check if we need to add isSync flag to remove from the sync directory explicitly
	chunkPath, hashPath := getRegularMVPath(cacheDir, req.Address.MvID, req.Address.FileID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::RemoveChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	err = os.Remove(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to remove chunk file %s [%v]", chunkPath, err.Error()))
	}

	err = os.Remove(hashPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove hash file %s [%v]", hashPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to remove hash file %s [%v]", hashPath, err.Error()))
	}

	availableSpace, err := common.GetAvailableDiskSpaceFromStatfs(cacheDir)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to get available disk space [%v]", err.Error())
	}

	// TODO: should we verify the hash after writing the chunk

	resp := &models.RemoveChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		AvailableSpace: availableSpace,
		PeerRV:         rvInfo.getPeerRVs(req.Address.MvID),
	}

	return resp, nil
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
