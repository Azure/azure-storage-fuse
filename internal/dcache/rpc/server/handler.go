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
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
)

// type check to ensure that ChunkServiceHandler implements dcache.ChunkService interface
var _ service.ChunkService = &ChunkServiceHandler{}

// ChunkServiceHandler struct implements the ChunkService interface
type ChunkServiceHandler struct {
	locks   *common.LockMap
	fsIDMap map[string]*rvInfo // map to store the fsID to rvInfo mapping
}

type rvInfo struct {
	rvID     string       // fsID for this RV
	rvName   string       // rv0, rv1, etc.
	cacheDir string       // cache dir path for this RV
	mvMap    sync.Map     // sync map of MV id against mvInfo
	mvCount  atomic.Int64 // count of MVs for this RV, this should be updated whenever a MV is added or removed from the sync map
}

type mvInfo struct {
	mvName       string       // mv0, mv1, etc.
	componentRVs []string     // sorted list of component RVs for this MV
	chunkOpsCnt  atomic.Int64 // count of chunk operations (get, put or remove) for this MV
	blockOpsFlag atomic.Bool  // block chunk operations (get, put or remove) for this MV. This is done when the .sync dir contents are being moved to the regular MV dir
	syncInfo                  // sync info for this MV
}

type syncInfo struct {
	mu        sync.Mutex
	isSyncing bool   // is the MV in syncing state
	syncID    string // sync ID for this MV if it in syncing state

	// sourceRV is the RV that is syncing the target RV in the MV.
	// This is used when the source RV goes offline and is replaced by the cluster manager with a new RV.
	// In this case, the sync will need to be restarted from the new RV.
	sourceRV string // source RV for syncing this MV
}

func NewChunkServiceHandler() *ChunkServiceHandler {
	// TODO:: integration: get fsID, rvName and cache dir path for different RVs for the node from cluster manager
	// below will be call to cluster manager to get the information
	fsIDMap := make(map[string]*rvInfo)

	return &ChunkServiceHandler{
		locks:   common.NewLockMap(),
		fsIDMap: fsIDMap,
	}
}

// check if the given mv is valid
func (rv *rvInfo) isMvValid(mvPath string) error {
	if !common.DirectoryExists(mvPath) {
		return fmt.Errorf("MV path %s does not exist", mvPath)
	}

	mvName := filepath.Base(mvPath)
	_, err := rv.getMVInfo(mvName)
	return err
}

func (rv *rvInfo) getComponentRVsForMV(mvName string) []string {
	mvInfo, err := rv.getMVInfo(mvName)
	if err != nil {
		log.Warn("ChunkServiceHandler::getComponentRVsForMV: Failed to get MV %s info for %s [%v]", mvName, rv.rvName, err.Error())
		return nil
	}

	return mvInfo.componentRVs
}

// getMVInfo returns the mvInfo for the given mvName
func (rv *rvInfo) getMVInfo(mvName string) (*mvInfo, error) {
	val, ok := rv.mvMap.Load(mvName)
	mvInfo := val.(*mvInfo)
	if !ok || mvInfo == nil {
		return nil, fmt.Errorf("MV %s is invalid for RV %s", mvName, rv.rvName)
	}

	common.Assert(mvName == mvInfo.mvName, "MV name mismatch in mv", mvName, mvInfo.mvName)

	return mvInfo, nil
}

func (rv *rvInfo) addToMVMap(mvName string, val *mvInfo) {
	rv.mvMap.Store(mvName, val)
	rv.mvCount.Add(1)
}

func (rv *rvInfo) deleteFromMVMap(mvName string) {
	rv.mvMap.Delete(mvName)
	rv.mvCount.Add(-1)
}

func (mv *mvInfo) updateSyncState(isSyncing bool, syncID string, sourceRV string) error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	if isSyncing && mv.isSyncing {
		// if the source RV is different from the current source RV, it means that the current source RV's node is offline
		// and is replaced by a new RV. So, the sync process needs to be restarted with the new RV.
		if mv.sourceRV != sourceRV {
			mv.syncID = syncID
			mv.sourceRV = sourceRV
		} else {
			return fmt.Errorf("MV is already in syncing state with sync id %s and source RV %s", mv.syncID, mv.sourceRV)
		}
	}

	mv.isSyncing = isSyncing
	mv.syncID = syncID
	mv.sourceRV = sourceRV

	return nil
}

// block new chunk operations for this MV till the sync is in progress
func (mv *mvInfo) blockChunkOps() {
	for {
		if mv.blockOpsFlag.Load() {
			time.Sleep(100 * time.Microsecond) // TODO: check if this is optimal
		} else {
			break
		}
	}
}

// block the sync operation for this MV till the ongoing chunk operations are completed
func (mv *mvInfo) blockSyncOps() {
	for {
		if mv.chunkOpsCnt.Load() > 0 {
			time.Sleep(100 * time.Microsecond) // TODO: check if this is optimal
		} else {
			break
		}
	}
}

// increment the chunk operation (get, put or remove) count for this MV
func (mv *mvInfo) incrementChunkOps() {
	mv.chunkOpsCnt.Add(1)
}

// ddecrement the chunk operation (get, put or remove) count for this MV
func (mv *mvInfo) decrementChunkOps() {
	mv.chunkOpsCnt.Add(-1)
}

func (mv *mvInfo) enableBlockChunkOps() {
	mv.blockOpsFlag.Store(true)
}

func (mv *mvInfo) disableBlockChunkOps() {
	mv.blockOpsFlag.Store(false)
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager to get chunk size from config
func getChunkSize() int64 {
	return 4 * 1024 * 1024 // 4MB
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager to get mvs-per-rv from config
func getMVsPerRV() int64 {
	return 10
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get available disk space for the given path
func getAvailableDiskSpace(path string) (int64, error) {
	return 0, nil
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager helper method to get the node ID of this node
func getNodeUUID() string {
	return "node-uuid" // TODO: get the node uuid from the config
}

// check the if the chunk address is valid
// - check if the fsID is valid
// - check if the cache dir exists
// - check if the MV is valid
func (h *ChunkServiceHandler) checkValidChunkAddress(address *models.Address) error {
	if address == nil || address.FileID == "" || address.FsID == "" || address.MvID == "" {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid chunk address %+v", *address)
		return rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("invalid chunk address %+v", *address))
	}

	// check if the fsID is valid
	rvInfo, ok := h.fsIDMap[address.FsID]
	if !ok || rvInfo == nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid fsID %s", address.FsID)
		return rpc.NewResponseError(rpc.InvalidFSID, fmt.Sprintf("invalid fsID %s", address.FsID))
	}

	cacheDir := rvInfo.cacheDir
	if cacheDir == "" || !common.DirectoryExists(cacheDir) {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Cache dir not found for RV %s", rvInfo.rvName)
		return rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("cache dir not found for RV %s", rvInfo.rvName))
	}

	// check if the MV is valid
	mvPath := filepath.Join(cacheDir, address.MvID)
	if err := rvInfo.isMvValid(mvPath); err != nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: MV %s is not hosted by RV %s [%s]", address.MvID, rvInfo.rvName, err.Error())
		return rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is not hosted by RV %s [%s]", address.MvID, rvInfo.rvName, err.Error()))
	}

	return nil
}

// get the RVInfo from the RV name
func (h *ChunkServiceHandler) getRVInfoFromRvName(rvName string) *rvInfo {
	var rvInfo *rvInfo
	for _, info := range h.fsIDMap {
		if info == nil {
			continue
		}
		if info.rvName == rvName {
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
		common.Assert(false, "received nil Hello request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil Hello request")
	}

	log.Debug("ChunkServiceHandler::Hello: Received Hello request: %+v", *req)

	// // TODO:: integration: call cluster manager to get the node ID of this node
	myNodeID := getNodeUUID()
	common.Assert(req.ReceiverNodeID == myNodeID, "Received Hello RPC destined for another node", req.ReceiverNodeID, myNodeID)

	// get all the RVs exported by this node
	myRvList := make([]string, 0)
	for _, info := range h.fsIDMap {
		myRvList = append(myRvList, info.rvName)
	}

	return &models.HelloResponse{
		ReceiverNodeID: myNodeID,
		Time:           time.Now().UnixMicro(),
		RV:             myRvList,
		MV:             req.MV, // TODO: discuss, how to fetch the MV list receiver shares with the sender
	}, nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	if req == nil || req.Address == nil {
		log.Err("ChunkServiceHandler::GetChunk: Received nil GetChunk request")
		common.Assert(false, "received nil GetChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil GetChunk request")
	}

	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request: %+v", *req)

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	rvInfo := h.fsIDMap[req.Address.FsID]
	mvInfo, _ := rvInfo.getMVInfo(req.Address.MvID)

	// check if the sync has started. If yes, block the chunk operations till the sync is completed
	mvInfo.blockChunkOps()

	// increment the chunk operation count for this MV
	mvInfo.incrementChunkOps()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decrementChunkOps()

	startTime := time.Now()

	// check if the chunk file is being updated in parallel by some other thread
	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.FsID, req.Address.MvID, req.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	chunkPath, hashPath := getChunkAndHashPath(cacheDir, req.Address.MvID, req.Address.FileID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::GetChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	fh, err := os.Open(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to open chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to open chunk file %s [%v]", chunkPath, err.Error()))
	}
	defer fh.Close()

	// TODO:: integration: call cluster manager to get chunk size
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
		PeerRV:         rvInfo.getComponentRVsForMV(req.Address.MvID),
	}

	return resp, nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	if req == nil || req.Chunk == nil || req.Chunk.Address == nil {
		log.Err("ChunkServiceHandler::PutChunk: Received nil PutChunk request")
		common.Assert(false, "received nil PutChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil PutChunk request")
	}

	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request: chunk address %+v, data length %v, isSync %v", *req.Chunk.Address, req.Length, req.IsSync)

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Chunk.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	rvInfo := h.fsIDMap[req.Chunk.Address.FsID]
	mvInfo, _ := rvInfo.getMVInfo(req.Chunk.Address.MvID)

	// check if the sync has started. If yes, block the chunk operations till the sync is completed
	mvInfo.blockChunkOps()

	// increment the chunk operation count for this MV
	mvInfo.incrementChunkOps()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decrementChunkOps()

	startTime := time.Now()

	// acquire lock for the chunk address to prevent concurrent writes
	chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.FsID, req.Chunk.Address.MvID, req.Chunk.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

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

	// TODO:: integration: call cluster manager to get the available disk space
	availableSpace, err := getAvailableDiskSpace(cacheDir)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err.Error())
	}

	// TODO: should we verify the hash after writing the chunk

	resp := &models.PutChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		AvailableSpace: availableSpace,
		PeerRV:         rvInfo.getComponentRVsForMV(req.Chunk.Address.MvID),
	}

	return resp, nil
}

func (h *ChunkServiceHandler) RemoveChunk(ctx context.Context, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	if req == nil || req.Address == nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Received nil RemoveChunk request")
		common.Assert(false, "received nil RemoveChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil RemoveChunk request")
	}

	log.Debug("ChunkServiceHandler::RemoveChunk: Received RemoveChunk request %+v", *req)

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Invalid chunk address [%s]", err.Error())
		return nil, err
	}

	rvInfo := h.fsIDMap[req.Address.FsID]
	mvInfo, _ := rvInfo.getMVInfo(req.Address.MvID)

	// check if the sync has started. If yes, block the chunk operations till the sync is completed
	mvInfo.blockChunkOps()

	// increment the chunk operation count for this MV
	mvInfo.incrementChunkOps()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decrementChunkOps()

	startTime := time.Now()

	// acquire lock for the chunk address to prevent concurrent delete operations
	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.FsID, req.Address.MvID, req.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

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

	// TODO:: integration: call cluster manager to get the available disk space
	availableSpace, err := getAvailableDiskSpace(cacheDir)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to get available disk space [%v]", err.Error())
	}

	resp := &models.RemoveChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		AvailableSpace: availableSpace,
		PeerRV:         rvInfo.getComponentRVsForMV(req.Address.MvID),
	}

	return resp, nil
}

func (h *ChunkServiceHandler) JoinMV(ctx context.Context, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::JoinMV: Received nil JoinMV request")
		common.Assert(false, "received nil JoinMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil JoinMV request")
	}

	log.Debug("ChunkServiceHandler::JoinMV: Received JoinMV request: %+v", *req)

	if req.MV == "" || req.RV == "" {
		log.Err("ChunkServiceHandler::JoinMV: MV or RV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV or RV is empty")
	}

	rvInfo := h.getRVInfoFromRvName(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::JoinMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is already part of the given MV
	_, err := rvInfo.getMVInfo(req.MV)
	if err == nil {
		log.Err("ChunkServiceHandler::JoinMV: RV %s is already part of the given MV %s", req.RV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is already part of the given MV %s", req.RV, req.MV))
	}

	// TODO:: integration: call cluster manager to get mvs-per-rv from config
	mvLimit := getMVsPerRV()
	if rvInfo.mvCount.Load() >= mvLimit {
		log.Err("ChunkServiceHandler::JoinMV: RV %s has reached the maximum number of MVs %d", req.RV, mvLimit)
		return nil, rpc.NewResponseError(rpc.MaxMVsExceeded, fmt.Sprintf("RV %s has reached the maximum number of MVs %d", req.RV, mvLimit))
	}

	// RV is being added to an already existing MV
	// check if the RV has enough space to store the new MV data
	if req.ReserveSpace != 0 {
		// TODO:: integration: call cluster manager to get the available disk space
		availableSpace, err := getAvailableDiskSpace(cacheDir)
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
	err = h.createMVDirectory(mvPath)
	if err != nil {
		log.Err("ChunkServiceHandler::JoinMV: Failed to create MV directory %s [%v]", mvPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create MV directory %s [%v]", mvPath, err.Error()))
	}

	// add in sync map
	slices.Sort(req.PeerRV)
	rvInfo.addToMVMap(req.MV, &mvInfo{mvName: req.MV, componentRVs: req.PeerRV})

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) LeaveMV(ctx context.Context, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Received nil LeaveMV request")
		common.Assert(false, "received nil LeaveMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil LeaveMV request")
	}

	log.Debug("ChunkServiceHandler::LeaveMV: Received LeaveMV request: %+v", *req)

	if req.MV == "" || req.RV == "" {
		log.Err("ChunkServiceHandler::LeaveMV: MV or RV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV or RV is empty")
	}

	rvInfo := h.getRVInfoFromRvName(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is part of the given MV
	mvInfo, err := rvInfo.getMVInfo(req.MV)
	if err != nil {
		log.Err("ChunkServiceHandler::LeaveMV: RV %s is not part of the given MV %s [%v]", req.RV, req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RV, req.MV))
	}

	// validate the component RVs list
	slices.Sort(req.PeerRV)
	if !isComponentRVsValid(mvInfo.componentRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::LeaveMV: Component RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("component RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// delete the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)
	flock := h.locks.Get(mvPath) // TODO: check if lock is needed in directory deletion
	flock.Lock()
	defer flock.Unlock()

	err = os.RemoveAll(mvPath)
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
		common.Assert(false, "received nil StartSync request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil StartSync request")
	}

	log.Debug("ChunkServiceHandler::StartSync: Received StartSync request: %+v", *req)

	if req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.PeerRV) == 0 {
		log.Err("ChunkServiceHandler::StartSync: MV, SourceRV, TargetRV or ComponentRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, TargetRV or ComponentRVs is empty")
	}

	// source RV is the lowest index online RV. The node hosting this RV will send the start sync call to the component RVs
	// target RV is the RV which has to mark that the MV will be in sync state
	rvInfo := h.getRVInfoFromRvName(req.TargetRV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: Invalid RV %s", req.TargetRV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRV))
	}

	// check if MV is valid
	mvInfo, err := rvInfo.getMVInfo(req.MV)
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: MV %s is invalid for RV %s [%v]", req.MV, req.TargetRV, err.Error())
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	// check if the source RV is present in the component RVs list
	if !slices.Contains(mvInfo.componentRVs, req.SourceRV) {
		log.Err("ChunkServiceHandler::StartSync: Source RV %s is not present in the component RVs list %v", req.SourceRV, mvInfo.componentRVs)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRV, mvInfo.componentRVs))
	}

	// validate the component RVs list
	slices.Sort(req.PeerRV)
	if !isComponentRVsValid(mvInfo.componentRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::StartSync: Component RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("component RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// update the sync state and sync id of the MV
	err = mvInfo.updateSyncState(true, base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(16)), req.SourceRV)
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
		common.Assert(false, "received nil EndSync request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil EndSync request")
	}

	log.Debug("ChunkServiceHandler::EndSync: Received EndSync request: %+v", *req)

	if req.SyncID == "" || req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.PeerRV) == 0 {
		log.Err("ChunkServiceHandler::EndSync: MV, SourceRV, TargetRV or ComponentRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, TargetRV or ComponentRVs is empty")
	}

	// source RV is the lowest index online RV. The node hosting this RV will send the end sync call to the component RVs
	// target RV is the RV which has to mark the completion of sync in MV
	rvInfo := h.getRVInfoFromRvName(req.TargetRV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: Invalid RV %s", req.TargetRV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRV))
	}

	// check if MV is valid
	mvInfo, err := rvInfo.getMVInfo(req.MV)
	if err != nil {
		log.Err("ChunkServiceHandler::EndSync: MV %s is invalid for RV %s [%v]", req.MV, req.TargetRV, err.Error())
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	if mvInfo.syncID != req.SyncID {
		log.Err("ChunkServiceHandler::EndSync: SyncID %s is invalid for MV %s", req.SyncID, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("syncID %s is invalid for MV %s", req.SyncID, req.MV))
	}

	// check if the source RV is present in the component RVs list
	if !slices.Contains(mvInfo.componentRVs, req.SourceRV) {
		log.Err("ChunkServiceHandler::EndSync: Source RV %s is not present in the component RVs list %v", req.SourceRV, mvInfo.componentRVs)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRV, mvInfo.componentRVs))
	}

	// validate the component RVs list
	slices.Sort(req.PeerRV)
	if !isComponentRVsValid(mvInfo.componentRVs, req.PeerRV) {
		log.Err("ChunkServiceHandler::StartSync: Component RVs %v are invalid for MV %s", req.PeerRV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("component RVs %v are invalid for MV %s", req.PeerRV, req.MV))
	}

	// enable block chunk operations flag for this MV to pause further chunk operations (get, put or remove) on this MV
	mvInfo.enableBlockChunkOps()

	// disable block chunk operations flag for this MV when the function returns
	defer mvInfo.disableBlockChunkOps()

	// wait till the ongoing chunk operations  on this MV are completed
	mvInfo.blockSyncOps()

	// update the sync state and sync id of the MV
	err = mvInfo.updateSyncState(false, "", "")
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: Failed to mark sync completion state in MV %s [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to mark sync completion state in MV %s [%v]", req.MV, err.Error()))
	}

	// move all chunks from sync folder to the regular MV folder and then resume processing.
	regMVPath := filepath.Join(rvInfo.cacheDir, req.MV)
	syncMvPath := filepath.Join(rvInfo.cacheDir, req.MV+".sync")

	log.Debug("ChunkServiceHandler::EndSync: Moving chunks from sync folder %s to regular MV folder %s", syncMvPath, regMVPath)
	err = moveChunksToRegularMVPath(syncMvPath, regMVPath)
	if err != nil {
		log.Err("ChunkServiceHandler::EndSync: Failed to move chunks from sync folder %s to regular MV folder %s [%v]", syncMvPath, regMVPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to move chunks from sync folder %s to regular MV folder %s [%v]", syncMvPath, regMVPath, err.Error()))
	}

	// TODO: should we also remove the sync folder

	return &models.EndSyncResponse{}, nil
}
