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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
	gouuid "github.com/google/uuid"
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
	// [readonly] -
	// the map itself will not have any new entries added after startup, but
	// some of the fields of those entries may change.
	rvIDMap map[string]*rvInfo
}

type rvInfo struct {
	rvID     string       // id for this RV [readonly]
	rvName   string       // rv0, rv1, etc. [readonly]
	cacheDir string       // cache dir path for this RV [readonly]
	mvMap    sync.Map     // all MVs this RV is part of, indexed by MV name (e.g., "mv0"), updated by JoinMV, UpdateMV and LeaveMV
	mvCount  atomic.Int64 // count of MVs for this RV, this should be updated whenever a MV is added or removed from the sync map

	// reserved space for the RV is the space reserved for chunks which will be synced
	// to the RV after the StartSync() call. This is used to calculate the available space
	// in the RV after subtracting the reserved space from the actual disk space available.
	// JoinMV() will increment this space indicating that new MV is being added to this RV.
	// On the other hand, PutChunk() sync RPC call will decrement this space indicating
	// that the chunk has been written to the RV.
	reservedSpace atomic.Int64
}

type mvInfo struct {
	rwMutex      sync.RWMutex
	mvName       string                   // mv0, mv1, etc.
	componentRVs []*models.RVNameAndState // sorted list of component RVs for this MV

	// total amount of space used up inside an MV by all the chunks stored in it.
	// Any RV that has to replace one of the existing component RVs needs to have
	// at least this much space. JoinMV() requests this much space to be reserved
	// in the new-to-be-inducted RV.
	totalChunkBytes atomic.Int64

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
	// opMutex is used to ensure that only one operation, chunk IO (get, put or remove chunk) or
	// sync (start sync or end sync) is in progress at a time.
	// IO operations like get, put or remove chunk takes read lock on opMutex, and sync operations
	// like StartSync or EndSync takes write lock on it.
	// This ensures that the sync operation waits for the ongoing IO operations to complete.
	// It also makes sure that no new IO operations till the sync is complete.
	opMutex sync.RWMutex

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

var handler *ChunkServiceHandler

// NewChunkServiceHandler creates a new ChunkServiceHandler instance.
// This MUST be called only once by the RPC server, on startup.
func NewChunkServiceHandler(rvs map[string]dcache.RawVolume) *ChunkServiceHandler {
	common.Assert(handler == nil, "NewChunkServiceHandler called more than once")

	handler = &ChunkServiceHandler{
		locks:   common.NewLockMap(),
		rvIDMap: getRvIDMap(rvs),
	}

	// Every node MUST contribute at least one RV.
	// Note: We can probably relax this later if we want to support nodes which do not
	//       contribute any storage.
	common.Assert(len(handler.rvIDMap) > 0)

	return handler
}

// check if the given mv is valid
func (rv *rvInfo) isMvPathValid(mvPath string) bool {
	mvName := filepath.Base(mvPath)
	mvInfo := rv.getMVInfo(mvName)
	common.Assert(mvInfo == nil || common.DirectoryExists(mvPath), fmt.Sprintf("mvPath %s MUST be present", mvPath))
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

// return the list of MVs for this RV
func (rv *rvInfo) getMVs() []string {
	mvs := make([]string, 0)
	rv.mvMap.Range(func(mvName, val interface{}) bool {
		mvInfo, ok := val.(*mvInfo)
		if ok {
			common.Assert(mvInfo != nil, fmt.Sprintf("mvMap[%s] has nil value", mvName))
			common.Assert(mvName == mvInfo.mvName, "MV name mismatch in mv", mvName, mvInfo.mvName)
		} else {
			common.Assert(false, fmt.Sprintf("mvMap[%s] has value which is not of type *mvInfo", mvName))
		}

		mvs = append(mvs, mvInfo.mvName)
		return true
	})

	return mvs
}

// caller of this method must ensure that the RV is not part of the given MV
func (rv *rvInfo) addToMVMap(mvName string, val *mvInfo) {
	mvPath := filepath.Join(rv.cacheDir, mvName)
	common.Assert(common.DirectoryExists(mvPath), fmt.Sprintf("mvPath %s MUST be present", mvPath))

	rv.mvMap.Store(mvName, val)
	rv.mvCount.Add(1)

	common.Assert(rv.mvCount.Load() <= getMVsPerRV(), fmt.Sprintf("mvCount for RV %s is greater than max MVs %d", rv.rvName, getMVsPerRV()))
}

func (rv *rvInfo) deleteFromMVMap(mvName string) {
	_, ok := rv.mvMap.Load(mvName)
	common.Assert(ok, fmt.Sprintf("mvMap[%s] not found", mvName))

	rv.mvMap.Delete(mvName)
	rv.mvCount.Add(-1)

	common.Assert(rv.mvCount.Load() >= 0, fmt.Sprintf("mvCount for RV %s is negative", rv.rvName))
}

// increment the reserved space for this RV
func (rv *rvInfo) incReservedSpace(bytes int64) {
	rv.reservedSpace.Add(bytes)
	log.Debug("rvInfo::incReservedSpace: reserved space for RV %s is %d", rv.rvName, rv.reservedSpace.Load())
}

// decrement the reserved space for this RV
func (rv *rvInfo) decReservedSpace(bytes int64) {
	rv.reservedSpace.Add(-bytes)
	common.Assert(rv.reservedSpace.Load() >= 0, fmt.Sprintf("reserved space for RV %s is %d", rv.rvName, rv.reservedSpace.Load()))
	log.Debug("rvInfo::decReservedSpace: reserved space for RV %s is %d", rv.rvName, rv.reservedSpace.Load())
}

// return available space for the given RV.
// This is calculated after subtracting the reserved space for this RV
// from the actual disk space available in the cache directory.
func (rv *rvInfo) getAvailableSpace() (int64, error) {
	cacheDir := rv.cacheDir
	_, diskSpaceAvailable, err := common.GetDiskSpaceMetricsFromStatfs(cacheDir)
	common.Assert(err == nil, fmt.Sprintf("failed to get available disk space for path %s [%v]", cacheDir, err))

	// decrement this by the reserved space for this RV
	availableSpace := int64(diskSpaceAvailable) - rv.reservedSpace.Load()

	log.Debug("rvInfo::getAvailableSpace: available space for RV %s is %d, total disk space available is %d and reserved space is %d",
		rv.rvName, availableSpace, diskSpaceAvailable, rv.reservedSpace.Load())
	common.Assert(availableSpace >= 0, fmt.Sprintf("available space for RV %s is %d", rv.rvName, availableSpace))

	return availableSpace, err
}

// return the current sync state of the MV
func (mv *mvInfo) getIsSyncing() bool {
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
		// TODO: add assert for IsValidRVName
		common.Assert(mv.sourceRVName != "")

		mv.isSyncing.Store(false)
		mv.syncID = ""
		mv.sourceRVName = ""

		return nil
	}

	// StartSync
	common.Assert(common.IsValidUUID(syncID))
	common.Assert(sourceRVName != "")

	if mv.isSyncing.Load() {
		msg := fmt.Sprintf("Got StartSync(%s) while already in syncing state with syncID %s", syncID, mv.syncID)
		common.Assert(false, msg)
		return fmt.Errorf("%s", msg)
	}

	// Must not already be in syncing state.
	common.Assert(mv.syncID == "")
	common.Assert(mv.sourceRVName == "")

	mv.isSyncing.Store(isSyncing)
	mv.syncID = syncID
	mv.sourceRVName = sourceRVName

	return nil
}

// get component RVs for this MV
func (mv *mvInfo) getComponentRVs() []*models.RVNameAndState {
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	return mv.componentRVs
}

// update the component RVs for the MV
func (mv *mvInfo) updateComponentRVs(componentRVs []*models.RVNameAndState) {
	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	// TODO: check if this is safe
	// componentRVs point to a thrift req member. Does thrift say anything about safety of that,
	// or should we do a deep copy of the list.
	mv.componentRVs = componentRVs
}

// increment the total chunk bytes for this MV
func (mv *mvInfo) incTotalChunkBytes(bytes int64) {
	mv.totalChunkBytes.Add(bytes)
	log.Debug("mvInfo::incTotalChunkBytes: totalChunkBytes for MV %s is %d", mv.mvName, mv.totalChunkBytes.Load())
}

// decrement the total chunk bytes for this MV
func (mv *mvInfo) decTotalChunkBytes(bytes int64) {
	mv.totalChunkBytes.Add(-bytes)
	log.Debug("mvInfo::decTotalChunkBytes: totalChunkBytes for MV %s is %d", mv.mvName, mv.totalChunkBytes.Load())
	common.Assert(mv.totalChunkBytes.Load() >= 0, fmt.Sprintf("totalChunkBytes for MV %s is %d", mv.mvName, mv.totalChunkBytes.Load()))
}

// acquire read lock on the opMutex.
// This will allow other ongoing chunk IO operations to proceed in parallel
// but will block sync operations like StartSync or EndSync,
// until the read lock is released.
func (mv *mvInfo) acquireSyncOpReadLock() {
	mv.opMutex.RLock()
}

// release the read lock on the opMutex
func (mv *mvInfo) releaseSyncOpReadLock() {
	mv.opMutex.RUnlock()
}

// acquire write lock on the opMutex.
// This will wait till all the ongoing chunk IO operations are completed
// and will block any new chunk IO operations.
// This is used in StartSync and EndSync RPC calls.
func (mv *mvInfo) acquireSyncOpWriteLock() {
	mv.opMutex.Lock()
}

// release the write lock on the opMutex
func (mv *mvInfo) releaseSyncOpWriteLock() {
	mv.opMutex.Unlock()
}

// check the if the chunk address is valid
// - check if the rvID is valid
// - check if the cache dir exists
// - check if the MV is valid
func (h *ChunkServiceHandler) checkValidChunkAddress(address *models.Address) error {
	// TODO: add assert for IsValidUUID(), IsValidMVName()
	if address == nil || address.FileID == "" || address.RvID == "" || address.MvName == "" {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid chunk address %v", address.String())
		return rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("invalid chunk address %v", address.String()))
	}

	// check if the rvID is valid
	rvInfo, ok := h.rvIDMap[address.RvID]
	common.Assert(ok && rvInfo != nil, fmt.Sprintf("rvInfo nil for rvID %s", address.RvID))
	if !ok || rvInfo == nil {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: Invalid rvID %s", address.RvID)
		return rpc.NewResponseError(rpc.InvalidRVID, fmt.Sprintf("invalid rvID %s", address.RvID))
	}

	cacheDir := rvInfo.cacheDir
	common.Assert(cacheDir != "", fmt.Sprintf("cacheDir is empty for RV %s", rvInfo.rvName))
	common.Assert(common.DirectoryExists(cacheDir), fmt.Sprintf("cacheDir %s does not exist for RV %s", cacheDir, rvInfo.rvName))

	// check if the MV is valid
	mvPath := filepath.Join(cacheDir, address.MvName)
	if !rvInfo.isMvPathValid(mvPath) {
		log.Err("ChunkServiceHandler::checkValidChunkAddress: MV %s is not hosted by RV %s", address.MvName, rvInfo.rvName)
		return rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("MV %s is not hosted by RV %s", address.MvName, rvInfo.rvName))
	}

	return nil
}

// get the RVInfo from the RV name
func (h *ChunkServiceHandler) getRVInfoFromRVName(rvName string) *rvInfo {
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

	log.Debug("ChunkServiceHandler::Hello: Received Hello request: %v", req.String())

	// TODO: send more information in response on Hello RPC

	myNodeID := rpc.GetMyNodeUUID()
	common.Assert(req.ReceiverNodeID == myNodeID, "Received Hello RPC destined for another node", req.ReceiverNodeID, myNodeID)

	// get all the RVs exported by this node
	myRvList := make([]string, 0)
	myMvList := make([]string, 0)
	for _, rvInfo := range h.rvIDMap {
		myRvList = append(myRvList, rvInfo.rvName)
		myMvList = append(myMvList, rvInfo.getMVs()...)
	}

	return &models.HelloResponse{
		ReceiverNodeID: myNodeID,
		Time:           time.Now().UnixMicro(),
		RVName:         myRvList,
		MV:             myMvList,
	}, nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	if req == nil || req.Address == nil {
		log.Err("ChunkServiceHandler::GetChunk: Received nil GetChunk request")
		common.Assert(false, "received nil GetChunk request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil GetChunk request")
	}

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request: %v", rpc.GetChunkRequestToString(req))

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Invalid chunk address %v [%s]", req.Address.String(), err.Error())
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	// validate the component RVs list
	if err := isComponentRVsValid(mvInfo.getComponentRVs(), req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Request component RVs are invalid for MV %s [%v]", req.Address.MvName, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.Address.MvName, err.Error()))
	}

	// acquire read lock on the opMutex for this MV
	mvInfo.acquireSyncOpReadLock()

	// release the read lock on the opMutex for this MV when the function returns
	defer mvInfo.releaseSyncOpReadLock()

	// TODO: check if lock is needed for GetChunk
	// check if the chunk file is being updated in parallel by some other thread
	// chunkAddress := getChunkAddress(req.Address.FileID, req.Address.RvID, req.Address.MvName, req.Address.OffsetInMiB)
	// flock := h.locks.Get(chunkAddress)
	// flock.Lock()
	// defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	chunkPath, hashPath := getChunkAndHashPath(cacheDir, req.Address.MvName, req.Address.FileID, req.Address.OffsetInMiB)
	log.Debug("ChunkServiceHandler::GetChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	fh, err := os.Open(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to open chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.ChunkNotFound, fmt.Sprintf("failed to open chunk file %s [%v]", chunkPath, err.Error()))
	}
	defer fh.Close()

	fInfo, err := fh.Stat()
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to stat chunk file %s [%v]", chunkPath, err.Error())
		common.Assert(false, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
		return nil, rpc.NewResponseError(rpc.ChunkNotFound, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
	}

	chunkSize := fInfo.Size()
	lmt := fInfo.ModTime().UTC().String()

	common.Assert(req.OffsetInChunk+req.Length <= chunkSize, fmt.Sprintf("chunkSize %d is less than OffsetInChunk %d + Length %d", chunkSize, req.OffsetInChunk, req.Length))

	// TODO: data buffer should come in the request
	data := make([]byte, req.Length)
	n, err := fh.ReadAt(data, req.OffsetInChunk)
	common.Assert(n == len(data), fmt.Sprintf("bytes read %v is less than expected buffer size %v", n, len(data)))
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to read chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to read chunk file %s [%v]", chunkPath, err.Error()))
	}

	// TODO: hash validation will be done later
	// get hash if requested for entire chunk
	// hash := ""
	// if req.OffsetInChunk == 0 && req.Length == chunkSize {
	// 	hashData, err := os.ReadFile(hashPath)
	// 	if err != nil {
	// 		log.Err("ChunkServiceHandler::GetChunk: Failed to read hash file %s [%v]", hashPath, err.Error())
	// 		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to read hash file %s [%v]", hashPath, err.Error()))
	// 	}
	// 	hash = string(hashData)
	// }

	resp := &models.GetChunkResponse{
		Chunk: &models.Chunk{
			Address: req.Address,
			Data:    data,
			Hash:    "", // TODO: hash validation will be done later
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

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request: %v", rpc.PutChunkRequestToString(req))

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Chunk.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Invalid chunk address %v [%s]", req.Chunk.Address.String(), err.Error())
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Chunk.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Chunk.Address.MvName)

	// validate the component RVs list
	if err := isComponentRVsValid(mvInfo.getComponentRVs(), req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Request component RVs are invalid for MV %s [%v]", req.Chunk.Address.MvName, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.Chunk.Address.MvName, err.Error()))
	}

	// acquire read lock on the opMutex for this MV
	mvInfo.acquireSyncOpReadLock()

	// release the read lock on the opMutex for this MV when the function returns
	defer mvInfo.releaseSyncOpReadLock()

	// TODO: check later if lock is needed
	// acquire lock for the chunk address to prevent concurrent writes
	// chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.RvID, req.Chunk.Address.MvName, req.Chunk.Address.OffsetInMiB)
	// flock := h.locks.Get(chunkAddress)
	// flock.Lock()
	// defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	mvIsSyncing := mvInfo.getIsSyncing()

	var chunkPath, hashPath string
	if req.IsSync {
		// sync write RPC call. This is called after the StartSync RPC to copy the contents
		// from the online RV (lowest index RV) to the new out of sync RV.
		// In this case the chunks must be written to the regular mv directory, i.e. rv0/mv0
		if mvIsSyncing {
			chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
		} else {
			log.Err("ChunkServiceHandler::PutChunk: MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName)
			common.Assert(false, fmt.Sprintf("MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName))
			return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("MV %s is not in sync state, whereas the client request is sync call", req.Chunk.Address.MvName))
		}
	} else {
		// client write RPC call. If the MV is in sync state, the chunks must be written to the sync directory, i.e. rv0/mv0.sync
		// If the MV is not in sync state, the chunks must be written to the regular mv directory, i.e. rv0/mv0
		if mvIsSyncing {
			chunkPath, hashPath = getSyncMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
		} else {
			chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
		}
	}

	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	// check if the chunk file is already present
	_, err = os.Stat(chunkPath)
	if err == nil {
		log.Err("ChunkServiceHandler::PutChunk: chunk file %s already exists", chunkPath)
		return nil, rpc.NewResponseError(rpc.ChunkAlreadyExists, fmt.Sprintf("chunk file %s already exists", chunkPath))
	}

	// write to .tmp file first and rename it to the final file
	tmpChunkPath := fmt.Sprintf("%s.tmp", chunkPath)
	err = os.WriteFile(tmpChunkPath, req.Chunk.Data, 0400)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to write chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to write chunk file %s [%v]", chunkPath, err.Error()))
	}

	// TODO: hash validation will be done later
	// err = os.WriteFile(hashPath, []byte(req.Chunk.Hash), 0400)
	// if err != nil {
	// 	log.Err("ChunkServiceHandler::PutChunk: Failed to write hash file %s [%v]", hashPath, err.Error())
	// 	return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to write hash file %s [%v]", hashPath, err.Error()))
	// }

	availableSpace, err := rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err.Error())
	}

	// TODO: should we verify the hash after writing the chunk

	// rename the .tmp file to the final file
	err = os.Rename(tmpChunkPath, chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to rename chunk file %s to %s [%v]", tmpChunkPath, chunkPath, err.Error())
		common.Assert(false, fmt.Sprintf("failed to rename chunk file %s to %s [%v]", tmpChunkPath, chunkPath, err.Error()))
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to rename chunk file %s to %s [%v]", tmpChunkPath, chunkPath, err.Error()))
	}

	// TODO: should we also consider the hash file size in the total chunk bytes
	//       For accurate accounting we can, but we should not do an extra stat() call for the hash file
	//       but instead use a hardcoded value which will be true for a given hash algo.
	//       Also we need to be sure that hash is calculated uniformly (either always or never)

	// increment the total chunk bytes for this MV
	mvInfo.incTotalChunkBytes(req.Length)

	// for successful sync PutChunk calls, decrement the reserved space for this RV
	if req.IsSync {
		rvInfo.decReservedSpace(req.Length)
	}

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

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::RemoveChunk: Received RemoveChunk request %v", rpc.RemoveChunkRequestToString(req))

	// check if the chunk address is valid
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Invalid chunk address %v [%s]", req.Address.String(), err.Error())
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	// validate the component RVs list
	if err := isComponentRVsValid(mvInfo.getComponentRVs(), req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Request component RVs are invalid for MV %s [%v]", req.Address.MvName, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.Address.MvName, err.Error()))
	}

	// acquire read lock on the opMutex for this MV
	mvInfo.acquireSyncOpReadLock()

	// release the read lock on the opMutex for this MV when the function returns
	defer mvInfo.releaseSyncOpReadLock()

	// TODO: check if lock is needed for RemoveChunk
	// acquire lock for the chunk address to prevent concurrent delete operations
	// chunkAddress := getChunkAddress(req.Address.FileID, req.Address.RvID, req.Address.MvName, req.Address.OffsetInMiB)
	// flock := h.locks.Get(chunkAddress)
	// flock.Lock()
	// defer flock.Unlock()

	cacheDir := rvInfo.cacheDir

	chunkPath, hashPath := getChunkAndHashPath(cacheDir, req.Address.MvName, req.Address.FileID, req.Address.OffsetInMiB)
	log.Debug("ChunkServiceHandler::RemoveChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	// check if the chunk is present
	fInfo, err := os.Stat(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to stat chunk file %s [%v]", chunkPath, err.Error())
		common.Assert(false, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
		return nil, rpc.NewResponseError(rpc.ChunkNotFound, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
	}

	err = os.Remove(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to remove chunk file %s [%v]", chunkPath, err.Error()))
	}

	// TODO: hash validation will be done later
	// err = os.Remove(hashPath)
	// if err != nil {
	// 	log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove hash file %s [%v]", hashPath, err.Error())
	// 	return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to remove hash file %s [%v]", hashPath, err.Error()))
	// }

	availableSpace, err := rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to get available disk space [%v]", err.Error())
	}

	// TODO: should we also consider the hash file size in the total chunk bytes
	//       For accurate accounting we can, but we should not do an extra stat() call for the hash file
	//       but instead use a hardcoded value which will be true for a given hash algo.
	//       Also we need to be sure that hash is calculated uniformly (either always or never)

	// decrement the total chunk bytes for this MV
	mvInfo.decTotalChunkBytes(fInfo.Size())

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

	// TODO:: discuss: changing type of component RV from string to RVNameAndState
	// requires to call componentRVsToString method as it is of type []*models.RVNameAndState
	log.Debug("ChunkServiceHandler::JoinMV: Received JoinMV request: %v", rpc.JoinMVRequestToString(req))

	if req.MV == "" || req.RVName == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::JoinMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::JoinMV: Invalid RV %s", req.RVName)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RVName))
	}

	cacheDir := rvInfo.cacheDir

	// acquire lock for the RV to prevent concurrent JoinMV calls for different MVs
	flock := h.locks.Get(rvInfo.rvID)
	flock.Lock()
	defer flock.Unlock()

	// check if RV is already part of the given MV
	mvi := rvInfo.getMVInfo(req.MV)
	if mvi != nil {
		log.Err("ChunkServiceHandler::JoinMV: RV %s is already part of the given MV %s", req.RVName, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is already part of the given MV %s", req.RVName, req.MV))
	}

	mvLimit := getMVsPerRV()
	if rvInfo.mvCount.Load() >= mvLimit {
		log.Err("ChunkServiceHandler::JoinMV: RV %s has reached the maximum number of MVs %d", req.RVName, mvLimit)
		return nil, rpc.NewResponseError(rpc.MaxMVsExceeded, fmt.Sprintf("RV %s has reached the maximum number of MVs %d", req.RVName, mvLimit))
	}

	// RV is being added to an already existing MV
	// check if the RV has enough space to store the new MV data
	if req.ReserveSpace != 0 {
		availableSpace, err := rvInfo.getAvailableSpace()
		if err != nil {
			log.Err("ChunkServiceHandler::JoinMV: Failed to get available disk space for RV %v [%v]", req.RVName, err.Error())
			return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to get available disk space for RV %v [%v]", req.RVName, err.Error()))
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
	sortComponentRVs(req.ComponentRV)
	rvInfo.addToMVMap(req.MV, &mvInfo{mvName: req.MV, componentRVs: req.ComponentRV})

	// increment the reserved space for this RV
	rvInfo.incReservedSpace(req.ReserveSpace)

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) UpdateMV(ctx context.Context, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	if req == nil {
		log.Err("ChunkServiceHandler::UpdateMV: Received nil UpdateMV request")
		common.Assert(false, "received nil UpdateMV request")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "received nil UpdateMV request")
	}

	log.Debug("ChunkServiceHandler::UpdateMV: Received UpdateMV request: %v", rpc.UpdateMVRequestToString(req))

	if req.MV == "" || req.RVName == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::UpdateMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::UpdateMV: Invalid RV %s", req.RVName)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RVName))
	}

	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::UpdateMV: RV %s is not part of the given MV %s", req.RVName, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RVName, req.MV))
	}

	componentRVsInMV := mvInfo.getComponentRVs()
	log.Debug("ChunkServiceHandler::UpdateMV: Current component RVs %v, updated component RVs %v", rpc.ComponentRVsToString(componentRVsInMV), rpc.ComponentRVsToString(req.ComponentRV))

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

	log.Debug("ChunkServiceHandler::LeaveMV: Received LeaveMV request: %v", rpc.LeaveMVRequestToString(req))

	if req.MV == "" || req.RVName == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::LeaveMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RVName)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.RVName))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is part of the given MV
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: RV %s is not part of the given MV %s", req.RVName, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RVName, req.MV))
	}

	// validate the component RVs list
	if err := isComponentRVsValid(mvInfo.getComponentRVs(), req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::LeaveMV: Request component RVs are invalid for MV %s [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.MV, err.Error()))
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

	log.Debug("ChunkServiceHandler::StartSync: Received StartSync request: %v", rpc.StartSyncRequestToString(req))

	if req.MV == "" || req.SourceRVName == "" || req.TargetRVName == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::StartSync: MV, SourceRV, TargetRV or ComponentRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, TargetRV or ComponentRVs is empty")
	}

	// source RV is the lowest index online RV. The node hosting this RV will send the start sync call to the component RVs
	// target RV is the RV which has to mark that the MV will be in sync state
	rvInfo := h.getRVInfoFromRVName(req.TargetRVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: Invalid RV %s", req.TargetRVName)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRVName))
	}

	// check if MV is valid
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::StartSync: MV %s is invalid for RV %s", req.MV, req.TargetRVName)
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRVName))
	}

	componentRVsInMV := mvInfo.getComponentRVs()

	// check if the source RV is present in the component RVs list
	if !isRVPresentInMV(componentRVsInMV, req.SourceRVName) {
		rvsInMvStr := rpc.ComponentRVsToString(componentRVsInMV)
		log.Err("ChunkServiceHandler::StartSync: Source RV %s is not present in the component RVs list %v", req.SourceRVName, rvsInMvStr)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRVName, rvsInMvStr))
	}

	// validate the component RVs list
	if err := isComponentRVsValid(componentRVsInMV, req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::StartSync: Request component RVs are invalid for MV %s [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.MV, err.Error()))
	}

	// acquire write lock on the opMutex for this MV. Now GetChunk, PutChunk and RemoveChunk will not allow any new IO.
	// Also wait for any ongoing IOs to complete.
	mvInfo.acquireSyncOpWriteLock()

	// release the write lock on the opMutex for this MV when the function returns
	defer mvInfo.releaseSyncOpWriteLock()

	// create the MV sync directory
	syncDir := filepath.Join(rvInfo.cacheDir, req.MV+".sync")
	err := h.createMVDirectory(syncDir)
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: Failed to create sync directory %s [%v]", syncDir, err.Error())
		return nil, rpc.NewResponseError(rpc.InternalServerError, fmt.Sprintf("failed to create sync directory %s [%v]", syncDir, err.Error()))
	}

	// update the sync state and sync id of the MV
	err = mvInfo.updateSyncState(true, gouuid.New().String(), req.SourceRVName)
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

	log.Debug("ChunkServiceHandler::EndSync: Received EndSync request: %v", rpc.EndSyncRequestToString(req))

	if req.SyncID == "" || req.MV == "" || req.SourceRVName == "" || req.TargetRVName == "" || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::EndSync: MV, SourceRV, TargetRV or ComponentRVs is empty")
		return nil, rpc.NewResponseError(rpc.InvalidRequest, "MV, SourceRV, TargetRV or ComponentRVs is empty")
	}

	// source RV is the lowest index online RV. The node hosting this RV will send the end sync call to the component RVs
	// target RV is the RV which has to mark the completion of sync in MV
	rvInfo := h.getRVInfoFromRVName(req.TargetRVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: Invalid RV %s", req.TargetRVName)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("invalid RV %s", req.TargetRVName))
	}

	// check if MV is valid
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::EndSync: MV %s is invalid for RV %s", req.MV, req.TargetRVName)
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("MV %s is invalid for RV %s", req.MV, req.TargetRVName))
	}

	if mvInfo.syncID != req.SyncID {
		log.Err("ChunkServiceHandler::EndSync: SyncID %s is invalid for MV %s", req.SyncID, req.MV)
		return nil, rpc.NewResponseError(rpc.InvalidRequest, fmt.Sprintf("syncID %s is invalid for MV %s", req.SyncID, req.MV))
	}

	componentRVsInMV := mvInfo.getComponentRVs()

	// check if the source RV is present in the component RVs list
	if !isRVPresentInMV(componentRVsInMV, req.SourceRVName) {
		rvsInMvStr := rpc.ComponentRVsToString(componentRVsInMV)
		log.Err("ChunkServiceHandler::EndSync: Source RV %s is not present in the component RVs list %v", req.SourceRVName, rvsInMvStr)
		return nil, rpc.NewResponseError(rpc.InvalidRV, fmt.Sprintf("source RV %s is not present in the component RVs list %v", req.SourceRVName, rvsInMvStr))
	}

	// validate the component RVs list
	if err := isComponentRVsValid(componentRVsInMV, req.ComponentRV); err != nil {
		log.Err("ChunkServiceHandler::StartSync: Request component RVs are invalid for MV %s [%v]", req.MV, err.Error())
		return nil, rpc.NewResponseError(rpc.NeedToRefreshClusterMap, fmt.Sprintf("request component RVs are invalid for MV %s [%v]", req.MV, err.Error()))
	}

	// acquire write lock on the opMutex for this MV. Now GetChunk, PutChunk and RemoveChunk will not allow any new IO.
	// Also wait for any ongoing IOs to complete.
	mvInfo.acquireSyncOpWriteLock()

	// release the write lock on the opMutex for this MV when the function returns
	defer mvInfo.releaseSyncOpWriteLock()

	// update the sync state and sync id of the MV
	err := mvInfo.updateSyncState(false, req.SyncID, req.SourceRVName)
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
	common.Assert(err == nil, fmt.Sprintf("failed to remove sync directory %s [%v]", syncMvPath, err))

	return &models.EndSyncResponse{}, nil
}
