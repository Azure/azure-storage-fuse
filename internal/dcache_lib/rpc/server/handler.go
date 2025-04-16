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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
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
	rvID     string       // rv0, rv1, etc.
	cacheDir string       // cache dir path for this RV
	mvMap    sync.Map     // sync map of MV id against MVInfo
	mvCount  atomic.Int64 // count of MVs for this RV, this should be updated whenever a MV is added or removed from the sync map
}

type MVInfo struct {
	SyncInfo              // sync info for this MV
	peerRVs  []string     // sorted list of peer RVs for this MV
	chunkOps atomic.Int64 // count of chunk operations (get, put or remove) for this MV in the given RV
	blockOps atomic.Bool  // block chunk operations for this MV in the given RV. This is done when the .sync dir contents are being moved to the regular MV dir
}

type SyncInfo struct {
	mu        sync.Mutex
	isSyncing bool   // is the MV in syncing state
	syncID    string // sync ID for this MV if it in syncing state

	// sourceRV is the RV that is syncing the target RV in the MV.
	// This is used when the source RV goes offline and is replaced by the cluster manager with a new RV.
	// In this case, the sync will need to be restarted from the new RV.
	sourceRV string // source RV for syncing this MV
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
	val, ok := rv.mvMap.Load(mvID)
	mvInfo := val.(*MVInfo)
	if !ok || mvInfo == nil {
		return fmt.Errorf("MV %s is invalid", mvID)
	}

	return nil
}

func (rv *RVInfo) getPeerRVs(mvID string) []string {
	val, ok := rv.mvMap.Load(mvID)
	mvInfo := val.(*MVInfo)
	if !ok || mvInfo == nil {
		return nil
	}
	return mvInfo.peerRVs
}

func (rv *RVInfo) addToMVMap(mvID string, val *MVInfo) {
	rv.mvMap.Store(mvID, val)
	rv.mvCount.Add(1)
}

func (rv *RVInfo) deleteFromMVMap(mvID string) {
	rv.mvMap.Delete(mvID)
	rv.mvCount.Add(-1)
}

func (mv *MVInfo) updateSyncState(isSyncing bool, syncID string, sourceRV string) error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	if isSyncing && mv.isSyncing {
		// if the source RV is different from the current source RV, it means that the current source RV's node is offline
		// and is replaced by a new RV. So, the sync process needs to be restarted with the new RV.
		if mv.sourceRV != sourceRV {
			mv.syncID = syncID
			mv.sourceRV = sourceRV
		} else {
			return fmt.Errorf("MV is already in syncing state with sync id %s", mv.syncID)
		}
	}

	mv.isSyncing = isSyncing
	mv.syncID = syncID
	mv.sourceRV = sourceRV

	return nil
}

// block new chunk operations for this MV till the sync is in progress
func (mv *MVInfo) blockChunkOps() {
	for {
		if mv.blockOps.Load() {
			time.Sleep(100 * time.Microsecond) // TODO: check if this is optimal
		} else {
			break
		}
	}
}

// block the sync operation for this MV till the ongoing chunk operations are completed
func (mv *MVInfo) blockSyncOp() {
	for {
		if mv.chunkOps.Load() > 0 {
			time.Sleep(100 * time.Microsecond) // TODO: check if this is optimal
		} else {
			break
		}
	}
}

// TODO: sample method, will be later removed after integrating with cluster manager
// call cluster manager to get chunk size from config
func getChunkSize() int64 {
	return 4 * 1024 * 1024 // 4MB
}

// TODO: sampel method, will be later removed after integrating with cluster manager
// call cluster manager to get mvs-per-rv from config
func getMVsPerRV() int64 {
	return 10
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
		log.Err("ChunkServiceHandler::checkValidChunkAddress: MV %s is not hosted by RV %s [%s]", address.MvID, rvInfo.rvID, err.Error())
		return rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is not hosted by RV %s [%s]", address.MvID, rvInfo.rvID, err.Error()))
	}

	return nil
}

// get the RVInfo from the rv id
func (h *ChunkServiceHandler) getRVInfoFromRvID(rvID string) *RVInfo {
	var rvInfo *RVInfo
	for _, info := range h.fsIDMap {
		if info == nil {
			continue
		}
		if info.rvID == rvID {
			rvInfo = info
			break
		}
	}

	return rvInfo
}

func (h *ChunkServiceHandler) createMVDirectory(path string) error {
	log.Debug("ChunkServiceHandler::createMVDirectory: Creating MV directory %s", path)
	flock := h.locks.Get(path) // TODO: check if lock is needed in directory creation
	flock.Lock()
	defer flock.Unlock()

	// TODO: dir check is not needed as os.MkdirAll does nothing if the dir already exists
	if !common.DirectoryExists(path) {
		err := os.MkdirAll(path, 0700)
		if err != nil {
			log.Err("ChunkServiceHandler::createMVDirectory: Failed to create MV directory %s [%v]", path, err.Error())
			return err
		}
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

// check if sync has started. If yes, block new chunk operations for this MV till the sync is completed
func (h *ChunkServiceHandler) checkSyncStatus(fsID string, mvID string) {
	rvInfo := h.fsIDMap[fsID]
	val, _ := rvInfo.mvMap.Load(mvID)
	mvInfo := val.(*MVInfo)
	mvInfo.blockChunkOps()
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

	// check if the sync has started. If yes, block the chunk operations till the sync is completed
	h.checkSyncStatus(req.Address.FsID, req.Address.MvID)

	// increment the chunk operation count for this MV

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
		// return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to get LMT for chunk file %s [%v]", chunkPath, err.Error()))
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

	// acquire lock for the chunk address to prevent concurrent writes
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	rvInfo := h.fsIDMap[req.Chunk.Address.FsID]
	cacheDir := rvInfo.cacheDir

	chunkPath, hashPath := getRegularMVPath(cacheDir, req.Chunk.Address.MvID, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
	if req.IsSync {
		chunkPath, hashPath = getSyncMVPath(cacheDir, req.Chunk.Address.MvID, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)

		//create sync directory if not present
		syncDir := filepath.Join(cacheDir, req.Chunk.Address.MvID+".sync")
		err = h.createMVDirectory(syncDir)
		if err != nil {
			log.Err("ChunkServiceHandler::PutChunk: Failed to create sync directory %s [%v]", syncDir, err.Error())
			return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create sync directory %s [%v]", syncDir, err.Error()))
		}
	}

	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	// check if the chunk file is already present
	_, err = os.Stat(chunkPath)
	if err == nil {
		log.Err("ChunkServiceHandler::PutChunk: chunk file %s already exists", chunkPath)
		return nil, rpc.NewResponseError(rpc.ChunkAlreadyExists, fmt.Sprintf("chunk file %s already exists", chunkPath))
	}

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

	// acquire lock for the chunk address to prevent concurrent delete operations
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
	if req == nil {
		log.Err("ChunkServiceHandler::JoinMV: Received nil JoinMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil JoinMV request")
	}

	if req.MV == "" || req.RV == "" {
		log.Err("ChunkServiceHandler::JoinMV: MV or RV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV or RV is empty")
	}

	log.Debug("ChunkServiceHandler::JoinMV: Received JoinMV request for MV %s, RV %s, reserve space %v, peer RVs %v", req.MV, req.RV, req.ReserveSpace, req.PeerRV)

	rvInfo := h.getRVInfoFromRvID(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::JoinMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is already part of the given MV
	_, ok := rvInfo.mvMap.Load(req.MV)
	if ok {
		log.Err("ChunkServiceHandler::JoinMV: RV %s is already part of the given MV %s", req.RV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is already part of the given MV %s", req.RV, req.MV))
	}

	// TODO: call cluster manager to get mvs-per-rv from config
	mvLimit := getMVsPerRV()
	if rvInfo.mvCount.Load() >= mvLimit {
		log.Err("ChunkServiceHandler::JoinMV: RV %s has reached the maximum number of MVs %d", req.RV, mvLimit)
		return nil, rpc.NewResponseError(rpc.MaxMVsExceeded, fmt.Sprintf("RV %s has reached the maximum number of MVs %d", req.RV, mvLimit))
	}

	// RV is being added to an already existing MV
	// check if the RV has enough space to store the new MV data
	if req.ReserveSpace != 0 {
		availableSpace, err := common.GetAvailableDiskSpaceFromStatfs(cacheDir)
		if err != nil {
			log.Err("ChunkServiceHandler::JoinMV: Failed to get available disk space for RV %v [%v]", req.RV, err.Error())
			return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to get available disk space for RV %v [%v]", req.RV, err.Error()))
		}

		if availableSpace < req.ReserveSpace {
			log.Err("ChunkServiceHandler::JoinMV: Not enough space to reserve %v bytes for joining MV %v", req.ReserveSpace, req.MV)
			return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("not enough space to reserve %v bytes for joining MV %v", req.ReserveSpace, req.MV))
		}
	}

	// create the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)
	err := h.createMVDirectory(mvPath)
	if err != nil {
		log.Err("ChunkServiceHandler::JoinMV: Failed to create MV directory %s [%v]", mvPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create MV directory %s [%v]", mvPath, err.Error()))
	}

	// add in sync map
	rvInfo.addToMVMap(req.MV, &MVInfo{peerRVs: req.PeerRV})

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) LeaveMV(ctx context.Context, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Received nil LeaveMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil LeaveMV request")
	}

	if req.MV == "" || req.RV == "" {
		log.Err("ChunkServiceHandler::LeaveMV: MV or RV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV or RV is empty")
	}

	log.Debug("ChunkServiceHandler::LeaveMV: Received LeaveMV request for MV %s, RV %s, peer RVs %v", req.MV, req.RV, req.PeerRV)

	rvInfo := h.getRVInfoFromRvID(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is part of the given MV
	val, ok := rvInfo.mvMap.Load(req.MV)
	mvInfo := val.(*MVInfo)
	if !ok || mvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: RV %s is not part of the given MV %s", req.RV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RV, req.MV))
	}

	// validate the peer RVs list
	slices.Sort(req.PeerRV)
	if !isPeerRVsValid(mvInfo.peerRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::LeaveMV: Peer RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("peer RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// create the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)
	err := os.RemoveAll(mvPath)
	if err != nil {
		log.Err("ChunkServiceHandler::LeaveMV: Failed to remove MV directory %s [%v]", mvPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to remove MV directory %s [%v]", mvPath, err.Error()))
	}

	// add in sync map
	rvInfo.deleteFromMVMap(req.MV)

	return &models.LeaveMVResponse{}, nil
}

func (h *ChunkServiceHandler) StartSync(ctx context.Context, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::StartSync: Received nil StartSync request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil StartSync request")
	}

	if req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.PeerRV) == 0 {
		log.Err("ChunkServiceHandler::StartSync: MV, SourceRV, PeerRVs or TargetRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, PeerRVs or TargetRVs is empty")
	}

	log.Debug("ChunkServiceHandler::StartSync: Received StartSync request for MV %s, SourceRV %s, TargetRV %s, PeerRVs %v, Data length %v", req.MV, req.SourceRV, req.TargetRV, req.PeerRV, req.DataLength)

	// source RV is the lowest index online RV. The node hosting this RV will send the start sync call to the peer RVs
	// target RV is the RV which has to mark that the MV will be in sync state
	rvInfo := h.getRVInfoFromRvID(req.TargetRV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: Invalid RV %s", req.TargetRV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRV))
	}

	// check if MV is valid
	val, ok := rvInfo.mvMap.Load(req.MV)
	mvInfo := val.(*MVInfo)
	if !ok || mvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: MV %s is invalid for RV %s", req.MV, req.TargetRV)
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	// check if the source RV is present in the peer RVs list
	if !slices.Contains(mvInfo.peerRVs, req.SourceRV) {
		log.Err("ChunkServiceHandler::StartSync: Source RV %s is not present in the peer RVs list %v", req.SourceRV, mvInfo.peerRVs)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the peer RVs list %v", req.SourceRV, mvInfo.peerRVs))
	}

	// validate the peer RVs list
	slices.Sort(req.PeerRV)
	if !isPeerRVsValid(mvInfo.peerRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::StartSync: Peer RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("peer RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// update the sync state and sync id of the MV
	err := mvInfo.updateSyncState(true, base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16)), req.SourceRV)
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: MV %s is already in sync state [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("MV %s is already in sync state [%v]", req.MV, err.Error()))
	}

	return &models.StartSyncResponse{
		SyncID: mvInfo.syncID,
	}, nil
}

func (h *ChunkServiceHandler) EndSync(ctx context.Context, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::EndSync: Received nil EndSync request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil EndSync request")
	}

	if req.SyncID == "" || req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.PeerRV) == 0 {
		log.Err("ChunkServiceHandler::EndSync: MV, SourceRV, PeerRVs or TargetRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, PeerRVs or TargetRVs is empty")
	}

	log.Debug("ChunkServiceHandler::EndSync: Received EndSync request for MV %s, sync id %s, SourceRV %s, TargetRV %s, PeerRVs %v, Data length %v", req.MV, req.SyncID, req.SourceRV, req.TargetRV, req.PeerRV, req.DataLength)

	// source RV is the lowest index online RV. The node hosting this RV will send the end sync call to the peer RVs
	// target RV is the RV which has to mark the completion of sync in MV
	rvInfo := h.getRVInfoFromRvID(req.TargetRV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: Invalid RV %s", req.TargetRV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRV))
	}

	// check if MV is valid
	val, ok := rvInfo.mvMap.Load(req.MV)
	mvInfo := val.(*MVInfo)
	if !ok || mvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: MV %s is invalid for RV %s", req.MV, req.TargetRV)
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	if mvInfo.syncID != req.SyncID {
		log.Err("ChunkServiceHandler::EndSync: SyncID %s is invalid for MV %s", req.SyncID, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("syncID %s is invalid for MV %s", req.SyncID, req.MV))
	}

	// check if the source RV is present in the peer RVs list
	if !slices.Contains(mvInfo.peerRVs, req.SourceRV) {
		log.Err("ChunkServiceHandler::EndSync: Source RV %s is not present in the peer RVs list %v", req.SourceRV, mvInfo.peerRVs)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the peer RVs list %v", req.SourceRV, mvInfo.peerRVs))
	}

	// validate the peer RVs list
	slices.Sort(req.PeerRV)
	if !isPeerRVsValid(mvInfo.peerRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::StartSync: Peer RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("peer RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// update the sync state and sync id of the MV
	err := mvInfo.updateSyncState(false, "", "")
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: Failed to mark sync completion state in MV %s [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to mark sync completion state in MV %s [%v]", req.MV, err.Error()))
	}

	// TODO: Node will wait for any ongoing put-chunk/get-chunk requests, pause further put-chunk/get-chunk processing,
	// move all chunks from “MV.sync” folder to the regular MV folder and then resume processing.

	return &models.EndSyncResponse{}, nil
}
