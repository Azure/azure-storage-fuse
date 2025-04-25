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
	locks *common.LockMap

	// map to store the rvID to rvInfo mapping
	// rvIDMap contains any and all cluster awareness information that a node needs to have,
	// things like what all RVs are hosted by this node, state of each of those RVs,
	// what all MVs are hosted by these RVs, state of those MVs, etc.
	// It is initially created from the clustermap which is the source of truth regarding cluster information,
	// and once the cluster is working it's updated using various RPCs.
	// Note that any time Cluster Manager needs to update clustermap, before publishing the updated clustermap,
	// it'll send our one or more RPCs to update the rvIDMap info in all the affected nodes,
	// thus rvIDMap always contains the latest info and hence is used by RVs to fail requests
	// which might be sent by nodes having a stale clustermap.
	//
	// [readonly] - the map itself will not have any new entries added after startup, but
	// some of the fields of those entries may change.
	rvIDMap map[string]*rvInfo
}

type rvInfo struct {
	rvID     string       // id for this RV [readonly]
	rvName   string       // rv0, rv1, etc. [readonly]
	cacheDir string       // cache dir path for this RV [readonly]
	mvMap    sync.Map     // all MVs this RV is part of, indexed by MV name (e.g., "mv0"), updated by JoinMV and LeaveMV
	mvCount  atomic.Int64 // count of MVs for this RV, this should be updated whenever a MV is added or removed from the sync map
}

type mvInfo struct {
	rwMutex      sync.RWMutex
	mvName       string   // mv0, mv1, etc.
	componentRVs []string // sorted list of component RVs for this MV

	// count of in-progress chunk operations (get, put or remove) for this MV.
	// This is used to block the end sync call till all the ongoing chunk operations are completed.
	chunkIOInProgress atomic.Int64

	// flag to block chunk operations (get, put or remove) for this MV.
	// This flag is enabled in the end sync call to pause new chunk operations in the MV.
	// When the end sync call is completed, this flag is disabled.
	blockOpsFlag atomic.Bool

	// Two MV states are interesting from an IO standpoint.
	// An online MV is the happy case where all RVs are online and sync'ed. In this state there won't be any
	// resync Writes, and client Writes if any will be replicated to all the RVs, each of them storing the chunks
	// in their respective mv folders. This is the normal case.
	// A syncing MV is interesting. In this case there are resync writes going and possibly client writes too.
	// Client chunks are saved in a special mv.sync folder while sync writes are saved in the regular mv folder.
	// During EndSync, the client writes are moved from mv.sync to the regular mv folder and then we have the MV
	// back in online state.
	// The short period when an MV moves in and out of syncing state is important. We need to quiesce any IOs
	// to make sure we don't miss resyncing any chunk.
	// Both StartSync and EndSync will quiesce IOs just before they move the mv into and out of syncing state, and
	// resume IOs once the MV is safely moved into the new state.
	//
	// This boolean flag will be set by StartSync and EndSync to put the mv in quiesce state and it'll be read
	// and honored by the various chunk related APIs - GetChunk, PutChunk, etc.
	quiesceIOs atomic.Bool

	syncInfo // sync info for this MV
}

type syncInfo struct {
	// Is the MV in syncing state?
	// An MV enters syncing state after StartSync command is successfully executed.
	// In syncing state, PutChunk requests corresponding to client writes will be
	// saved in the mv#.sync folder. Similarly a successful EndSync will take an
	// MV out of syncing state.
	isSyncing atomic.Bool

	// sync ID for this MV if it is in syncing state.
	// This is returned in the StartSync response and EndSync should carry this.
	syncID string

	// sourceRV is the RV that is syncing a (local) target RV in this MV.
	// Since not more than one component RVs of an MV will be from the same node, there will be one and only
	// one local RV as the target RV.
	// Communicated using the StartSync message sent by the Replication Manager as part of the resync-mv workflow.
	// We will only accept PutChunk(isSync=true) requests only with source RV value matching this.
	// This is used when the source RV goes offline and is replaced by the cluster manager with a new RV.
	// In this case, the sync will need to be restarted from the new RV.
	sourceRVName string // source RV name for syncing this MV
}

func NewChunkServiceHandler() *ChunkServiceHandler {
	// TODO:: integration: get rvID, rvName and cache dir path for different RVs for the node from cluster manager
	// below will be call to cluster manager to get the information
	rvIDMap := getRvIDMap()

	return &ChunkServiceHandler{
		locks:   common.NewLockMap(),
		rvIDMap: rvIDMap,
	}
}

// check if the given mv is valid
func (rv *rvInfo) isMvPathValid(mvPath string) bool {
	mvName := filepath.Base(mvPath)
	mvInfo := rv.getMVInfo(mvName)
	common.Assert(mvInfo != nil || common.DirectoryExists(mvPath), fmt.Sprintf("mvPath %s MUST be present", mvPath))
	return mvInfo != nil
}

// getMVInfo returns the mvInfo for the given mvName
func (rv *rvInfo) getMVInfo(mvName string) *mvInfo {
	val, ok := rv.mvMap.Load(mvName)

	// Not found.
	if !ok {
		return nil
	}

	// Found, value must be of type *mvInfo.
	mvInfo, ok := val.(*mvInfo)
	if ok {
		common.Assert(mvInfo != nil, fmt.Sprintf("mvMap[%s] has nil value", mvName))
		common.Assert(mvName == mvInfo.mvName, "MV name mismatch in mv", mvName, mvInfo.mvName)

		return mvInfo
	}

	common.Assert(false, fmt.Sprintf("mvMap[%s] has value which is not of type *mvInfo", mvName))

	return nil
}

// caller of this method must ensure that the RV is not part of the given MV
func (rv *rvInfo) addToMVMap(mvName string, val *mvInfo) {
	rv.mvMap.Store(mvName, val)
	rv.mvCount.Add(1)
}

func (rv *rvInfo) deleteFromMVMap(mvName string) {
	_, ok := rv.mvMap.Load(mvName)
	common.Assert(ok, fmt.Sprintf("mvMap[%s] not found", mvName))

	rv.mvMap.Delete(mvName)
	rv.mvCount.Add(-1)

	common.Assert(rv.mvCount.Load() >= 0, fmt.Sprintf("mvCount for RV %s is negative", rv.rvName))
}

// return the current sync state of the MV
func (mv *mvInfo) getSyncStatus() bool {
	return mv.isSyncing.Load()
}

// update the sync state of the MV
func (mv *mvInfo) updateSyncState(isSyncing bool, syncID string, sourceRVName string) error {
	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	//
	// EndSync.
	// Must be received after a matching StartSync, i.e., with the same syncID.
	// Due to connectivity issues we may miss some of the requests, but we still assert in debug builds
	// as those are not common and we would like to understand, to handle those better.
	//
	if !isSyncing {
		if mv.syncID == "" {
			msg := fmt.Sprintf("EndSync(%s) received w/o StartSync", syncID)
			common.Assert(false, msg)
			return fmt.Errorf("%s", msg)
		}

		if mv.syncID != syncID {
			msg := fmt.Sprintf("Unexpected EndSync(%s) received, expected syncID %s ", syncID, mv.syncID)
			common.Assert(false, msg)
			return fmt.Errorf("%s", msg)
		}

		// sourceRV must be valid.
		common.Assert(mv.sourceRVName != "")

		mv.isSyncing.Store(false)
		mv.syncID = ""
		mv.sourceRVName = ""

		return nil
	}

	// StartSync.
	// - Must not already be in syncing state.
	common.Assert(syncID != "")
	common.Assert(sourceRVName != "")

	if mv.isSyncing.Load() {
		msg := fmt.Sprintf("Got StartSync(%s) while already in syncing state with syncID %s", syncID, mv.syncID)
		common.Assert(false, msg)
		return fmt.Errorf("%s", msg)
	}

	common.Assert(mv.syncID == "")
	common.Assert(mv.sourceRVName == "")

	mv.isSyncing.Store(isSyncing)
	mv.syncID = syncID
	mv.sourceRVName = sourceRVName

	return nil
}

// get component RVs for this MV
func (mv *mvInfo) getComponentRVs() []string {
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	return mv.componentRVs
}

// update the component RVs for the MV
func (mv *mvInfo) updateComponentRVs(componentRVs []string) {
	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	// TODO: check if this is safe
	// componentRVs point to a thrift req member. Does thrift say anything about safety of that,
	// or should we do a deep copy of the list.
	mv.componentRVs = componentRVs
}

// increment the in-progress chunk operation (get, put or remove) count for this MV
func (mv *mvInfo) incOngoingIOs() {
	mv.chunkIOInProgress.Add(1)
}

// ddecrement the in-progress chunk operation (get, put or remove) count for this MV after it has completed
func (mv *mvInfo) decOngoingIOs() {
	common.Assert(mv.chunkIOInProgress.Load() > 0, fmt.Sprintf("chunkOpsInProgress for MV %s is <= 0", mv.mvName))
	mv.chunkIOInProgress.Add(-1)
}

// Block the calling thread if this MV is currently quiesced, by StartSync or EndSync.
func (mv *mvInfo) blockIOIfMVQuiesced() error {
	if !mv.quiesceIOs.Load() {
		return nil
	}

	// Wait till MV is quiesced.
	now := time.Now()
	maxWait := 30 * time.Second

	for {
		if mv.quiesceIOs.Load() {
			time.Sleep(1 * time.Millisecond)

			elapsed := time.Since(now)
			if elapsed > maxWait {
				msg := fmt.Sprintf("%s still quiesced after %s", mv.mvName, maxWait)
				common.Assert(false, msg)
				return fmt.Errorf("%s", msg)
			}
		} else {
			break
		}
	}

	return nil
}

// Set IO quiescing in the mv. Now GetChunk, PutChunk, will not allow any new IO.
// Also, wait for any ongoing IOs to complete.
func (mv *mvInfo) quiesceIOsStart() error {
	mv.quiesceIOs.Store(true)

	// Wait for any ongoing IOs to complete.
	now := time.Now()
	maxWait := 30 * time.Second

	for {
		if mv.chunkIOInProgress.Load() > 0 {
			time.Sleep(1 * time.Millisecond)

			elapsed := time.Since(now)
			if elapsed > maxWait {
				msg := fmt.Sprintf("%d ongoing IOs still pending after waiting for %s", mv.chunkIOInProgress.Load(), maxWait)
				common.Assert(false, msg)
				return fmt.Errorf("%s", msg)
			}
		} else {
			log.Info("%s quiesced successfully!", mv.mvName)
			break
		}
	}

	// Quiesced successfully, no ongoing IOs and no new IOs will be allowed.
	return nil
}

// quiesceIOsEnd() must be called only after quiesceIOsStart().
func (mv *mvInfo) quiesceIOsEnd() {
	common.Assert(mv.quiesceIOs.Load(), fmt.Sprintf("quiesceIOsEnd() called without quiesceIOsStart() for MV %s", mv.mvName))
	mv.quiesceIOs.Store(false)
}

// TODO:: integration: sample method, will be later removed after integrating with cluster manager
// call cluster manager to create the rvID map from config
func getRvIDMap() map[string]*rvInfo {
	return make(map[string]*rvInfo)
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
// - check if the rvID is valid
// - check if the cache dir exists
// - check if the MV is valid
func (h *ChunkServiceHandler) checkValidChunkAddress(address *models.Address) error {
	if address == nil || address.FileID == "" || address.RvID == "" || address.MvName == "" {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid chunk address %+v", *address)
		return rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("invalid chunk address %+v", *address))
	}

	// check if the rvID is valid
	rvInfo, ok := h.rvIDMap[address.RvID]
	if !ok || rvInfo == nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid rvID %s", address.RvID)
		return rpc.NewResponseError(rpc.InvalidRVID, fmt.Sprintf("invalid rvID %s", address.RvID))
	}

	common.Assert(ok && rvInfo != nil, fmt.Sprintf("rvInfo nil for rvID %s", address.RvID))

	cacheDir := rvInfo.cacheDir
	common.Assert(cacheDir != "", fmt.Sprintf("cacheDir is empty for RV %s", rvInfo.rvName))
	common.Assert(common.DirectoryExists(cacheDir), fmt.Sprintf("cacheDir %s does not exist for RV %s", cacheDir, rvInfo.rvName))

	// check if the MV is valid
	mvPath := filepath.Join(cacheDir, address.MvName)
	if rvInfo.isMvPathValid(mvPath) {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: MV %s is not hosted by RV %s", address.MvName, rvInfo.rvName)
		return rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is not hosted by RV %s", address.MvName, rvInfo.rvName))
	}

	return nil
}

// get the RVInfo from the RV name
func (h *ChunkServiceHandler) getRVInfoFromRvName(rvName string) *rvInfo {
	var rvInfo *rvInfo
	for rvID, info := range h.rvIDMap {
		common.Assert(info != nil, fmt.Sprintf("rvInfo nil for rvID %s", rvID))

		if info.rvName == rvName {
			rvInfo = info
			break
		}
	}

	return rvInfo
}

func (h *ChunkServiceHandler) createMVDirectory(path string) error {
	log.Debug("ChunkServiceHandler::createMVDirectory: Creating MV directory %s", path)

	if err := os.MkdirAll(path, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("MkdirAll(%s) failed: %v", path, err)
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
	for _, info := range h.rvIDMap {
		myRvList = append(myRvList, info.rvName)
	}

	return &models.HelloResponse{
		ReceiverNodeID: myNodeID,
		Time:           time.Now().UnixMicro(),
		RV:             myRvList,
		MV:             req.MV, // TODO:: discuss: how to fetch the MV list receiver shares with the sender
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

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	// Block the calling thread if this MV is currently quiesced
	err = mvInfo.blockIOIfMVQuiesced()
	common.Assert(err == nil, fmt.Sprintf("failed to block IO for MV %s", mvInfo.mvName))

	// increment the chunk operation count for this MV
	mvInfo.incOngoingIOs()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decOngoingIOs()

	startTime := time.Now()

	// check if the chunk file is being updated in parallel by some other thread
	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.RvID, req.Address.MvName, req.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	chunkPath, hashPath := getChunkAndHashPath(cacheDir, req.Address.MvName, req.Address.FileID, req.Address.OffsetInMB)
	log.Debug("ChunkServiceHandler::GetChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	fh, err := os.Open(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to open chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.ChunkNotFound, fmt.Sprintf("failed to open chunk file %s [%v]", chunkPath, err.Error()))
	}
	defer fh.Close()

	// TODO:: integration: call cluster manager to get chunk size
	fInfo, err := fh.Stat()
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to stat chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
	}

	chunkSize := fInfo.Size()
	lmt := fInfo.ModTime().UTC().String()

	if req.Length == -1 {
		common.Assert(chunkSize > req.Offset, fmt.Sprintf("chunkSize %d is less than req.Offset %d", chunkSize, req.Offset))
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

	resp := &models.GetChunkResponse{
		Chunk: &models.Chunk{
			Address: req.Address,
			Data:    data,
			Hash:    hash,
		},
		ChunkWriteTime: lmt,
		TimeTaken:      time.Since(startTime).Microseconds(),
		ComponentRV:    mvInfo.getComponentRVs(),
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

	rvInfo := h.rvIDMap[req.Chunk.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Chunk.Address.MvName)

	// validate the component RVs list
	slices.Sort(req.ComponentRV)
	componentRVsInMV := mvInfo.getComponentRVs()
	if !isComponentRVsValid(componentRVsInMV, req.ComponentRV) {
		log.Err("ChunkServiceHandler::PutChunk: Request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.Chunk.Address.MvName, componentRVsInMV)
		common.Assert(false, fmt.Sprintf("invalid component RVs for MV %s", req.Chunk.Address.MvName), req.ComponentRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.ComponentRVsInvalid, fmt.Sprintf("request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.Chunk.Address.MvName, componentRVsInMV))
	}

	// Block the calling thread if this MV is currently quiesced
	err = mvInfo.blockIOIfMVQuiesced()
	common.Assert(err == nil, fmt.Sprintf("failed to block IO for MV %s", mvInfo.mvName))

	// increment the chunk operation count for this MV
	mvInfo.incOngoingIOs()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decOngoingIOs()

	startTime := time.Now()

	// acquire lock for the chunk address to prevent concurrent writes
	chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.RvID, req.Chunk.Address.MvName, req.Chunk.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	mvSyncState := mvInfo.getSyncStatus()

	var chunkPath, hashPath string
	if req.IsSync {
		// sync write RPC call. This is called after the StartSync RPC to copy the contents
		// from the online RV (lowest index RV) to the new out of sync RV.
		// In this case the chunks must be written to the regular mv directory, i.e. rv0/mv0
		if mvSyncState {
			chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
		} else {
			log.Err("ChunkServiceHandler::PutChunk: MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName)
			common.Assert(false, fmt.Sprintf("MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName))
			return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName))
		}
	} else {
		// client write RPC call. If the MV is in sync state, the chunks must be written to the sync directory, i.e. rv0/mv0.sync
		// If the MV is not in sync state, the chunks must be written to the regular mv directory, i.e. rv0/mv0
		if mvSyncState {
			chunkPath, hashPath = getSyncMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
		} else {
			chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMB)
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
		ComponentRV:    mvInfo.getComponentRVs(),
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

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	// Block the calling thread if this MV is currently quiesced
	err = mvInfo.blockIOIfMVQuiesced()
	common.Assert(err == nil, fmt.Sprintf("failed to block IO for MV %s", mvInfo.mvName))

	// increment the chunk operation count for this MV
	mvInfo.incOngoingIOs()

	// decrement the chunk operation count for this MV when the function returns
	defer mvInfo.decOngoingIOs()

	startTime := time.Now()

	// acquire lock for the chunk address to prevent concurrent delete operations
	chunkAddress := getChunkAddress(req.Address.FileID, req.Address.RvID, req.Address.MvName, req.Address.OffsetInMB)
	flock := h.locks.Get(chunkAddress)
	flock.Lock()
	defer flock.Unlock()

	cacheDir := rvInfo.cacheDir

	// TODO: check if we need to add isSync flag to remove from the sync directory explicitly
	chunkPath, hashPath := getRegularMVPath(cacheDir, req.Address.MvName, req.Address.FileID, req.Address.OffsetInMB)
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
		ComponentRV:    mvInfo.getComponentRVs(),
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

	if req.MV == "" || req.RV == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::JoinMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRvName(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::JoinMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is already part of the given MV
	mvInf := rvInfo.getMVInfo(req.MV)
	if mvInf != nil {
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
	err := h.createMVDirectory(mvPath)
	if err != nil {
		log.Err("ChunkServiceHandler::JoinMV: Failed to create MV directory %s [%v]", mvPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create MV directory %s [%v]", mvPath, err.Error()))
	}

	// add in sync map
	slices.Sort(req.ComponentRV)
	rvInfo.addToMVMap(req.MV, &mvInfo{mvName: req.MV, componentRVs: req.ComponentRV})

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) UpdateMV(ctx context.Context, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::UpdateMV: Received nil UpdateMV request")
		common.Assert(false, "received nil UpdateMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil UpdateMV request")
	}

	log.Debug("ChunkServiceHandler::UpdateMV: Received UpdateMV request: %+v", *req)

	if req.MV == "" || req.RV == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::UpdateMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRvName(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::UpdateMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::UpdateMV: RV %s is not part of the given MV %s", req.RV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RV, req.MV))
	}

	componentRVsInMV := mvInfo.getComponentRVs()
	log.Debug("ChunkServiceHandler::UpdateMV: Current component RVs %v, updated component RVs", componentRVsInMV, req.ComponentRV)

	// update the component RVs list for this MV
	mvInfo.updateComponentRVs(req.ComponentRV)

	// TODO: check if this is needed as mvInfo is a pointer
	// rvInfo.addToMVMap(req.MV, mvInfo)

	return &models.UpdateMVResponse{}, nil
}

func (h *ChunkServiceHandler) LeaveMV(ctx context.Context, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Received nil LeaveMV request")
		common.Assert(false, "received nil LeaveMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil LeaveMV request")
	}

	log.Debug("ChunkServiceHandler::LeaveMV: Received LeaveMV request: %+v", *req)

	if req.MV == "" || req.RV == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::LeaveMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRvName(req.RV)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RV))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is part of the given MV
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: RV %s is not part of the given MV %s", req.RV, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RV, req.MV))
	}

	// validate the component RVs list
	slices.Sort(req.ComponentRV)
	componentRVsInMV := mvInfo.getComponentRVs()
	if !isComponentRVsValid(componentRVsInMV, req.ComponentRV) {
		log.Err("ChunkServiceHandler::LeaveMV: Request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV)
		common.Assert(false, fmt.Sprintf("invalid component RVs for MV %s", req.MV), req.ComponentRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.ComponentRVsInvalid, fmt.Sprintf("request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV))
	}

	// delete the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)
	flock := h.locks.Get(mvPath) // TODO: check if lock is needed in directory deletion
	flock.Lock()
	defer flock.Unlock()

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
		common.Assert(false, "received nil StartSync request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil StartSync request")
	}

	log.Debug("ChunkServiceHandler::StartSync: Received StartSync request: %+v", *req)

	if req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.ComponentRV) == 0 {
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
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: MV %s is invalid for RV %s", req.MV, req.TargetRV)
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	componentRVsInMV := mvInfo.getComponentRVs()

	// check if the source RV is present in the component RVs list
	if !slices.Contains(componentRVsInMV, req.SourceRV) {
		log.Err("ChunkServiceHandler::StartSync: Source RV %s is not present in the component RVs list %v", req.SourceRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRV, componentRVsInMV))
	}

	// validate the component RVs list
	slices.Sort(req.ComponentRV)
	if !isComponentRVsValid(componentRVsInMV, req.ComponentRV) {
		log.Err("ChunkServiceHandler::StartSync: Request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV)
		common.Assert(false, fmt.Sprintf("invalid component RVs for MV %s", req.MV), req.ComponentRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.ComponentRVsInvalid, fmt.Sprintf("request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV))
	}

	// create the MV sync directory
	syncDir := filepath.Join(rvInfo.cacheDir, req.MV+".sync")
	err := h.createMVDirectory(syncDir)
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: Failed to create sync directory %s [%v]", syncDir, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create sync directory %s [%v]", syncDir, err.Error()))
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

	if req.SyncID == "" || req.MV == "" || req.SourceRV == "" || req.TargetRV == "" || len(req.ComponentRV) == 0 {
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
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: MV %s is invalid for RV %s", req.MV, req.TargetRV)
		return nil, rpc.NewResponseError(rpc.MVNotHostedByRV, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRV))
	}

	if mvInfo.syncID != req.SyncID {
		log.Err("ChunkServiceHandler::EndSync: SyncID %s is invalid for MV %s", req.SyncID, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("syncID %s is invalid for MV %s", req.SyncID, req.MV))
	}

	componentRVsInMV := mvInfo.getComponentRVs()

	// check if the source RV is present in the component RVs list
	if !slices.Contains(componentRVsInMV, req.SourceRV) {
		log.Err("ChunkServiceHandler::EndSync: Source RV %s is not present in the component RVs list %v", req.SourceRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRV, componentRVsInMV))
	}

	// validate the component RVs list
	slices.Sort(req.ComponentRV)
	if !isComponentRVsValid(componentRVsInMV, req.ComponentRV) {
		log.Err("ChunkServiceHandler::EndSync: Request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV)
		common.Assert(false, fmt.Sprintf("invalid component RVs for MV %s", req.MV), req.ComponentRV, componentRVsInMV)
		return nil, rpc.NewResponseError(rpc.ComponentRVsInvalid, fmt.Sprintf("request component RVs %v are invalid for MV %s component RVs %v", req.ComponentRV, req.MV, componentRVsInMV))
	}

	// Set IO quiescing in the mv. Now GetChunk, PutChunk, will not allow any new IO.
	// Also wait for any ongoing IOs to complete.
	err := mvInfo.quiesceIOsStart()
	common.Assert(err == nil, fmt.Sprintf("failed to quiesce IOs for MV %s [%v]", req.MV, err.Error()))

	// disable block chunk operations flag for this MV when the function returns
	defer mvInfo.quiesceIOsEnd()

	// update the sync state and sync id of the MV
	err = mvInfo.updateSyncState(false, req.SyncID, req.SourceRV)
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

	// delete the sync directory
	err = os.RemoveAll(syncMvPath)
	common.Assert(err == nil, fmt.Sprintf("failed to remove sync directory %s [%v]", syncMvPath, err.Error()))

	return &models.EndSyncResponse{}, nil
}
