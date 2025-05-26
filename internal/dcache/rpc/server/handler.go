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
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
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
	// things like, what all RVs are hosted by this node, state of each of those RVs,
	// what all MVs are hosted by these RVs, state of those MVs, etc.
	// It is initially created from the clustermap which is the source of truth regarding cluster information,
	// and once the cluster is working it's updated using various RPCs received from various nodes.
	// Note that any time Cluster Manager needs to update clustermap, before publishing the updated clustermap,
	// it'll send out one or more RPCs to update the rvIDMap info in all the affected nodes,
	// thus rvIDMap always contains the latest info and hence is used by RVs to fail requests
	// which might be sent by nodes having a stale clustermap.
	//
	// [readonly] -
	// the map itself will not have any new entries added after startup, but
	// some of the fields of those entries may change.
	rvIDMap map[string]*rvInfo
}

// This holds information on one of our local RV.
// ChunkServiceHandler.rvIDMap contains one such struct for each RV that this node contributes to the cluster.
type rvInfo struct {
	rvID     string // id for this RV [readonly]
	rvName   string // rv0, rv1, etc. [readonly]
	cacheDir string // cache dir path for this RV [readonly]
	//
	// all MV replicas hosted by this RV, indexed by MV name (e.g., "mv0"), updated by JoinMV/UpdateMV/LeaveMV.
	//
	// TODO: Currently we don't call LeaveMV, so once added to mvMap, mvInfo won't be removed.
	//       If an RV is removed from an MV only when it goes offline, then it's not such a big problem, as
	//       an offline RV when it joins the cluster again must start afresh, with brand new rvInfo, but if
	//       an RV is removed due to rebalancing we must use LeaveMV to update the membership.
	//
	mvMap sync.Map
	//
	// count of MV replicas hosted by this RV, this should be updated whenever an MV is added or removed from
	// the mvMap.
	//
	mvCount atomic.Int64

	// reserved space for the RV is the space reserved for chunks which will be synced
	// to the RV after the StartSync() call. This is used to calculate the available space
	// in the RV after subtracting the reserved space from the actual disk space available.
	// JoinMV() will increment this space indicating that new MV is being added to this RV.
	// On the other hand, PutChunk() sync RPC call will decrement this space indicating
	// that the chunk has been written to the RV.
	reservedSpace atomic.Int64
}

// This holds information about one MV hosted by our local RV. This is known as "MV Replica".
// rvInfo.mvMap contains one such struct for each MV Replica that the RV hosts.
// Note that this is not information about the entire MV. One MV is replicated across multiple RVs and this holds
// only the information about the "MV Replica" that our RV hosts.
type mvInfo struct {
	rwMutex sync.RWMutex
	mvName  string // mv0, mv1, etc.

	// RV this MV is part of.
	// Note that mvInfo is referenced via rvInfo.mvMap so when we have rvInfo we already know the
	// RV name. This is for making the RV name available to functions that operate on mvInfo and do
	// not have the rvInfo.
	rvName string

	componentRVs []*models.RVNameAndState // sorted list of component RVs for this MV

	// Total amount of space used up inside the MV directory (both MV and .sync directory),
	// by all the chunks stored in it. Any RV that has to replace one of the existing component
	// RVs needs to have at least this much space.
	// JoinMV() requests this much space to be reserved in the new-to-be-inducted RV.
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
	// It also makes sure that if start/end sync is waiting for the write lock,
	// no new IO operations are started till the start/end sync gets the write lock and completes.
	// This ensures that a continuous flow of IOs will not delay the start/end sync indefinitely.
	opMutex sync.RWMutex

	// Companion counter to opMutex for performing various locking related assertions.
	// [DEBUG ONLY]
	opMutexDbgCntr atomic.Int64

	// Zero or more sync jobs this MV Replica is participating in.
	// If this is an empty slice it means the MV Replica is currently not participating in any sync job.
	// If non empty, these are all the sync jobs that this MV Replica is currently participating in.
	// The information on each sync job is held inside the syncJob struct. Since an MV Replica can be the
	// source of multiple sync jobs but can be a target for only one sync job, if this contains more than
	// one sync jobs, all of them MUST be source sync jobs.
	syncJobs map[string]syncJob // syncJobs is map of syncID to syncJob.
}

// A sync job syncs data between an online component RV to an outofsync component RV of the same MV.
// Note that in an MV the Lowest Index Online RV ("rv0" < "rv1") is the one that is responsible for performing the
// data copy, hence Replication Manager on the node hosting the Lowest Index Online RV (LIO RV) sets up a sync job
// and orchestrates the copy. It sends the StartSync RPC request to the source and the target RV, performs the chunk
// transfer and ends with an EndSync request.
//
// This syncJob structure holds information on each sync job that a particular "MV Replica" is participating in.
// Note that an MV Replica can be taking part in multiple simultaneous sync jobs, with the following rules:
//   - An MV Replica can either be the source or target of a sync job.
//   - Online MV Replicas will act as sources while OutOfSyc MV Replicas will act as targets.
//   - An MV Replica can be source to multiple sync jobs while it can be target to one and only one sync job.
//   - Every sync job has an id, called the SyncId. This is returned by a successful StartSync call and must be provided
//     in the EndSync call to end that sync job.
//   - When an MV Replica is acting as the source of a sync job any client writes targeted to that MV Replica will be
//     stored in the ".sync" folder.
type syncJob struct {
	// sync ID for this sync job.
	// This is returned in the StartSync response and EndSync should carry this.
	syncID string

	// Source and target RVs for this sync job.
	// An MV Replica can either act as source or target in a sync job, so one and only one of these will be set.
	// If sourceRVName is set that means this MV Replica is the target of this sync job, while if
	// targetRVName is set it means this MV Replica is the source of this sync job.
	sourceRVName string
	targetRVName string
}

var handler *ChunkServiceHandler

// NewChunkServiceHandler creates a new ChunkServiceHandler instance.
// This MUST be called only once by the RPC server, on startup.
func NewChunkServiceHandler(rvs map[string]dcache.RawVolume) {
	common.Assert(handler == nil, "NewChunkServiceHandler called more than once")

	handler = &ChunkServiceHandler{
		locks:   common.NewLockMap(),
		rvIDMap: getRvIDMap(rvs),
	}

	// Every node MUST contribute at least one RV.
	// Note: We can probably relax this later if we want to support nodes which do not
	//       contribute any storage.
	common.Assert(len(handler.rvIDMap) > 0)
}

// Create new mvInfo instance. This is used by the JoinMV() RPC call to create a new mvInfo.
func newMVInfo(rvName, mvName string, componentRVs []*models.RVNameAndState) *mvInfo {
	return &mvInfo{
		rvName:       rvName,
		mvName:       mvName,
		componentRVs: componentRVs,
		syncJobs:     make(map[string]syncJob),
	}
}

// Check if the given mvPath is valid on this node.
func (rv *rvInfo) isMvPathValid(mvPath string) bool {
	mvName := filepath.Base(mvPath)
	mvInfo := rv.getMVInfo(mvName)

	// If we are hosting MV replica mvName, directory mvPath must exist.
	common.Assert(mvInfo == nil || common.DirectoryExists(mvPath), mvPath, mvName)

	return mvInfo != nil
}

// Get MV replica info for the given MV on rv.
func (rv *rvInfo) getMVInfo(mvName string) *mvInfo {
	val, ok := rv.mvMap.Load(mvName)

	// Not found.
	if !ok {
		return nil
	}

	// Found, value must be of type *mvInfo.
	mvInfo, ok := val.(*mvInfo)
	if ok {
		common.Assert(mvInfo != nil, mvName, rv.rvName)
		common.Assert(mvName == mvInfo.mvName, mvName, mvInfo.mvName, rv.rvName)
		common.Assert(rv.rvName == mvInfo.rvName, rv.rvName, mvInfo.rvName, mvName)

		return mvInfo
	}

	// Value not of type mvInfo.
	common.Assert(false, mvName, rv.rvName)

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
			common.Assert(rv.rvName == mvInfo.rvName, rv.rvName, mvInfo.rvName, mvInfo.mvName)
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
	common.Assert(common.DirectoryExists(mvPath), mvPath)

	rv.mvMap.Store(mvName, val)
	rv.mvCount.Add(1)

	common.Assert(val.rvName == rv.rvName, val.rvName, rv.rvName)
	common.Assert(rv.mvCount.Load() <= getMVsPerRV(), rv.rvName, rv.mvCount.Load(), getMVsPerRV())
}

func (rv *rvInfo) deleteFromMVMap(mvName string) {
	_, ok := rv.mvMap.Load(mvName)
	common.Assert(ok, fmt.Sprintf("mvMap[%s] not found", mvName))

	rv.mvMap.Delete(mvName)
	rv.mvCount.Add(-1)

	common.Assert(rv.mvCount.Load() >= 0, fmt.Sprintf("mvCount for RV %s is negative", rv.rvName))
}

// Increment the reserved space for this RV.
func (rv *rvInfo) incReservedSpace(bytes int64) {
	common.Assert(bytes >= 0)
	rv.reservedSpace.Add(bytes)
	log.Debug("rvInfo::incReservedSpace: reserved space for RV %s is %d", rv.rvName, rv.reservedSpace.Load())
}

// Decrement the reserved space for this RV.
func (rv *rvInfo) decReservedSpace(bytes int64) {
	common.Assert(bytes > 0)
	//
	// TODO: Uncomment this only after clustermanager joinMV() correctly reserves space for MV replica.
	//
	// rv.reservedSpace.Add(-bytes)
	// common.Assert(rv.reservedSpace.Load() >= 0, rv.rvName, rv.reservedSpace.Load())
	log.Debug("rvInfo::decReservedSpace: reserved space for RV %s is %d", rv.rvName, rv.reservedSpace.Load())
}

// Return available space for the given RV.
// This is calculated after subtracting the reserved space for this RV
// from the actual disk space available in the cache directory.
func (rv *rvInfo) getAvailableSpace() (int64, error) {
	cacheDir := rv.cacheDir
	_, diskSpaceAvailable, err := common.GetDiskSpaceMetricsFromStatfs(cacheDir)
	common.Assert(err == nil, cacheDir, err)

	// Subtract the reserved space for this RV.
	availableSpace := int64(diskSpaceAvailable) - rv.reservedSpace.Load()
	common.Assert(availableSpace >= 0, rv.rvName, availableSpace, diskSpaceAvailable, rv.reservedSpace.Load())

	log.Debug("rvInfo::getAvailableSpace: available space for RV %s is %d, total disk space available is %d and reserved space is %d",
		rv.rvName, availableSpace, diskSpaceAvailable, rv.reservedSpace.Load())

	return availableSpace, err
}

// Return if the MV is in syncing state.
// If there are more than one entries in the syncJobs map, it means that the MV is in syncing state.
//
// Caller must hold opMutex write lock.
func (mv *mvInfo) isSyncing() bool {
	common.Assert(mv.isSyncOpWriteLocked(), mv.opMutexDbgCntr.Load())
	return len(mv.syncJobs) > 0
}

// Add a new sync job entry to the syncJobs map for this MV replica.
//
// Caller must hold opMutex write lock.
func (mv *mvInfo) addSyncJob(sourceRVName string, targetRVName string) string {
	common.Assert(mv.isSyncOpWriteLocked(), mv.opMutexDbgCntr.Load())
	// One and only one of sourceRVName and targetRVName can be valid.
	common.Assert(sourceRVName == "" || targetRVName == "", sourceRVName, targetRVName)
	common.Assert(cm.IsValidRVName(sourceRVName) || cm.IsValidRVName(targetRVName), sourceRVName, targetRVName)

	syncID := gouuid.New().String()
	_, ok := mv.syncJobs[syncID]
	common.Assert(!ok, fmt.Sprintf("%s already has syncJob with syncID %s: %+v", mv.mvName, syncID, mv.syncJobs))

	mv.syncJobs[syncID] = syncJob{
		syncID:       syncID,
		sourceRVName: sourceRVName,
		targetRVName: targetRVName,
	}

	return syncID
}

// Check if the syncID is valid for this MV replica, i.e., there is currently a syncJob running with this syncID.
//
// Caller must hold opMutex write lock.
//
// TODO: Later when we add syncIds to PutChunk request then we will need to call this with opMutex read lock too.
func (mv *mvInfo) isSyncIDValid(syncID string) bool {
	common.Assert(mv.isSyncOpWriteLocked(), mv.opMutexDbgCntr.Load())
	common.Assert(common.IsValidUUID(syncID))

	_, ok := mv.syncJobs[syncID]
	return ok
}

// Delete sync job entry from the syncJobs map for this MV replica.
//
// Caller must hold opMutex write lock.
func (mv *mvInfo) deleteSyncJob(syncID string) {
	common.Assert(mv.isSyncOpWriteLocked(), mv.opMutexDbgCntr.Load())

	_, ok := mv.syncJobs[syncID]
	common.Assert(ok, fmt.Sprintf("%s does not have syncJob with syncID %s: %+v", mv.mvName, syncID, mv.syncJobs))

	delete(mv.syncJobs, syncID)
}

// Return if this MV replica is the source or target of a sync job.
// An MV replica can act as source for multiple simultaneous sync jobs (each of which would be resyncing one distinct
// MV replica for the MV) but can act as target for one and only one sync job.
// For MV replicas acting as source, the target MV replica will be outside this node and targetRVName contains the
// name of the RV on which the target MV replica resides, similarly for MV replicas acting as target, the source
// MV replica will be outside this node and sourceRVName contains the name of the RV on which the source MV replica
// resides.
// Source MV replicas MUST have a <mv>.sync folder and all client PutChunk requests must write chunks to this folder
// while resync PutChunk requests must be written to the regular mv folder.
// Target MV replicas MUST write both client and resync PutChunk chunks to the regular mv folder.
//
// Caller must hold opMutex read lock.
func (mv *mvInfo) isSourceOrTargetOfSync() (isSource bool, isTarget bool) {
	common.Assert(mv.isSyncOpReadLocked(), mv.opMutexDbgCntr.Load())

	// No entry in syncJobs map means that the MV is not in syncing state.
	// This is the common case.
	if len(mv.syncJobs) == 0 {
		return false, false /* MV replica is not syncing */
	}

	// If there are more than one entries in the syncJobs map, it means that this MV replica is the source of
	// all those sync jobs. Note that an MV replica can be target to one and only one sync job.
	if len(mv.syncJobs) > 1 {
		return true, false /* MV replica is source for more than one sync jobs */
	}

	for _, job := range mv.syncJobs {
		common.Assert(job.sourceRVName == "" || job.targetRVName == "",
			fmt.Sprintf("Both source and target RV names cannot be set in a syncJob %+v", job))
		common.Assert(cm.IsValidRVName(job.sourceRVName) || cm.IsValidRVName(job.targetRVName),
			fmt.Sprintf("One of source or target RV name must be set in a syncJob %+v", job))

		// If sourceRVName is set that means this MV Replica is the target of this sync job,
		// while if targetRVName is set it means this MV Replica is the source of this sync job.
		if job.sourceRVName != "" {
			return false, true /* MV replica is target for one sync job */
		} else {
			return true, false /* MV replica is source for one sync job */
		}
	}

	// Unreachable code.
	common.Assert(false)

	return false, false
}

// Get component RVs for this MV.
func (mv *mvInfo) getComponentRVs() []*models.RVNameAndState {
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	common.Assert(len(mv.componentRVs) == int(cm.GetCacheConfig().NumReplicas),
		len(mv.componentRVs), cm.GetCacheConfig().NumReplicas)

	return mv.componentRVs
}

// Update the component RVs for the MV. Called by UpdateMV() handler.
// UpdateMV RPC can only replace one or more component RVs and must not change the state of the unchanged
// RVs, also for the RVs which are changed the state should change from offline (for the old RV) to outofsync
// (for the replacement RV).
// Also note that since UpdateMV (like all ther RPCs) is not transactional, sender will send multiple of these
// RPCs in order to run one high level workflow (like fix-mv, new-mv, start-sync, end-sync, etc) and each of them
// can fail independently. The workflow will complete, causing a change to be committed to clustermap, only
// if all these RPCs complete successfully. When a workflow fails due to one or more RPCs failing, the sender
// simply omits committing the change in clustermap, and doesn't bother undoing the mvInfo changes that the
// successful RPCs would have caused (this is the non-transactional nature). This means that when an UpdateMV
// RPC is received at an RV, it doesn't necessarily see offline->outofsync as the only state change (as some
// RVs might have a stale state, different from the clustermap). In that case we need to refresh our mvInfo
// from the clustermap (by calling mvInfo.refreshFromClustermap()) and then try again.
//
// This means that following must be true for UpdateMV RPC:
//   - It can only replace one or more RVs and never change the state of existing/unchanged RVs.
//   - The new RVs added by UpdateMV, must be in outofsync state.
//   - Since we can replace a component RV with itself (if it comes back up online, after going offline
//     for some time) such an RV will appear to undergo a state change, but this must be offline->outofsync.
//
// These checks must be performed to ensure consistent updates to mvInfo.
// When called from refreshFromClustermap() we don't need to do these checks and forceUpdate must be true.
func (mv *mvInfo) updateComponentRVs(componentRVs []*models.RVNameAndState, forceUpdate bool) error {
	common.Assert(len(componentRVs) == int(cm.GetCacheConfig().NumReplicas),
		len(componentRVs), cm.GetCacheConfig().NumReplicas)

	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	// Update must be called only to update not to add.
	common.Assert(mv.componentRVs != nil)
	common.Assert(len(mv.componentRVs) == len(componentRVs), len(mv.componentRVs), len(componentRVs))

	// TODO: check if this is safe
	// componentRVs point to a thrift req member. Does thrift say anything about safety of that,
	// or should we do a deep copy of the list.

	// mvInfo.componentRVs is a sorted list.
	sortComponentRVs(componentRVs)

	log.Debug("mvInfo::updateComponentRVs: %s from %s -> %s [forceUpdate: %v]",
		mv.mvName,
		rpc.ComponentRVsToString(mv.componentRVs),
		rpc.ComponentRVsToString(componentRVs),
		forceUpdate)

	//
	// Catch invalid membership changes.
	//
	// Note: Cluster manager doesn't commit clustermap after the degrade-mv workflow that marks component
	//       RVs as offline, so we won't get updated offline state even after a refresh.
	//       We either let JoinMV fail in this iteration and the next time around when clustermap would have
	//       the offline state, it succeeds or we change updateMvReq() to commit clustermap after marking
	//       component RVs offline.
	//
	if !forceUpdate {
		//
		// To compare the old and new RVs we use the following approach:
		// - First find common RVs.
		//   These are the RVs which are not changed by this update. The old and new states must match.
		//   Additionally we need to handle the case where the same RV is used as a replacement RV, in which
		//   case the only valid state transition is offline->outofsync.
		// - RVs which are not common, add them to oldList and newList.
		//   These represent RVs which are being replaced.
		//   They should all move from offline->outofsync. Note that it doesn't matter if we get the correct
		//   list of replacements, since all of them have to move from offline->outofsync.
		//
		oldMap := rpc.ComponentRVsListToMap(mv.componentRVs)
		newMap := rpc.ComponentRVsListToMap(componentRVs)

		//
		// Find common RVs, remove them from the map, so what's left in each map are the distinct RVs,
		// those which are changed.
		//
		for oldName, oldState := range oldMap {
			newState, exists := newMap[oldName]
			if exists {
				delete(oldMap, oldName)
				delete(newMap, oldName)

				if oldState == newState {
					// No change in RV.
					continue
				}

				if oldState == string(dcache.StateOffline) && newState == string(dcache.StateOutOfSync) {
					// Same RV (now online) being reused by fix-mv.
					continue
				}

				errStr := fmt.Sprintf("Invalid change attempted to %s (%s=%s -> %s=%s)",
					mv.mvName, oldName, oldState, oldName, newState)
				log.Info("mvInfo::updateComponentRVs: %s", errStr)
				return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
			}
		}

		//
		// What is left in oldMap (and newMap) are the RVs which have undergone replacement.
		// They can only transition from offline->outofsync.
		//
		if len(oldMap) > 0 {
			common.Assert(len(oldMap) == len(newMap), len(oldMap), len(newMap))

			oldList := rpc.ComponentRVsMapToList(oldMap)
			newList := rpc.ComponentRVsMapToList(newMap)

			for i := 0; i < len(oldList); i++ {
				oldName := oldList[i].Name
				oldState := oldList[i].State
				newName := newList[i].Name
				newState := newList[i].State

				common.Assert(oldName != newName, oldName, newName)

				if oldState == string(dcache.StateOffline) && newState == string(dcache.StateOutOfSync) {
					// New RV replaced by fix-mv.
					continue
				}

				errStr := fmt.Sprintf("Invalid change attempted to %s (%s=%s -> %s=%s)",
					mv.mvName, oldName, oldState, newName, newState)
				log.Info("mvInfo::updateComponentRVs: %s", errStr)
				return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
			}
		}
	}

	// Valid membership changes, update the saved componentRVs.
	mv.componentRVs = componentRVs
	return nil
}

// Update the state of the given component RV in this MV.
func (mv *mvInfo) updateComponentRVState(rvName string, oldState, newState dcache.StateEnum) {
	common.Assert(oldState != newState &&
		cm.IsValidComponentRVState(oldState) &&
		cm.IsValidComponentRVState(newState), rvName, oldState, newState)

	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			common.Assert(rv.State == string(oldState), rvName, rv.State, oldState)
			log.Debug("mvInfo::updateComponentRVState: %s/%s (%s -> %s) %s",
				rvName, mv.mvName, rv.State, newState, rpc.ComponentRVsToString(mv.componentRVs))

			rv.State = string(newState)
			return
		}
	}

	common.Assert(false, rpc.ComponentRVsToString(mv.componentRVs), rvName, newState)
}

// From the list of component RVs for this MV return RVNameAndState for the requested RV, if not found returns nil.
func (mv *mvInfo) getComponentRVNameAndState(rvName string) *models.RVNameAndState {
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)

		if rv.Name == rvName {
			return rv
		}
	}

	return nil
}

// Refresh componentRVs for the MV.
//
// Description:
// Any workflow that updates an MV's membership information (either component RVs and/or their states)
// first updates the membership in the node's rvInfo data, by an UpdateMV/StartSync/EndSync RPC message.
// Once all involved component RVs respond with a success the sender commits the change in the clustermap.
// If one or more component RVs fail the request while some other succeed, the membership details might
// become inconsistent. Since the sender will only update the clustermap after *all* the component RVs
// respond with a success, in this case those component RVs which did make the change have information
// that is different from the clustermap.
//
// Thus, an incoming request's component RVs may not match the rvInfo's component RVs for one of two reasons:
// 1. The sender has a stale clustermap.
// 2. rvInfo has inconsistent info due to the partially applied change.
//
// So, whenever a request and mvInfo's component RV details don't match, the server needs to refresh its
// membership details from the clustermap and if there still is a mismatch indicating client using stale
// clustermap, fail the call with NeedToRefreshClusterMap asking the sender to refresh too. This function
// helps to refresh the rvInfo component RV details.
func (mv *mvInfo) refreshFromClustermap() error {
	log.Debug("mvInfo::refreshFromClustermap: %s/%s", mv.rvName, mv.mvName)

	//
	// Refresh the clustermap synchronously. Once this returns, clustermap package has the updated
	// clustermap.
	//
	err := cm.RefreshClusterMapSync()
	if err != nil {
		err := fmt.Errorf("mvInfo::refreshFromClustermap: %s/%s, failed: %v", mv.rvName, mv.mvName, err)
		log.Err("%v", err)
		common.Assert(false, err)
		return err
	}

	// Get component RV details from the just refreshed clustermap.
	newRVs := cm.GetRVs(mv.mvName)
	if newRVs == nil {
		err := fmt.Errorf("mvInfo::refreshFromClustermap: GetRVs(%s) failed", mv.mvName)
		log.Err("%v", err)
		common.Assert(false, err)
		return err
	}

	// Convert newRVs from RV Name->State map, to RVNameAndState slice.
	var newComponentRVs []*models.RVNameAndState
	for rvName, rvState := range newRVs {
		common.Assert(cm.IsValidComponentRVState(rvState), rvName, mv.mvName, rvState)

		newComponentRVs = append(newComponentRVs, &models.RVNameAndState{
			Name:  rvName,
			State: string(rvState),
		})

		//
		// TODO: If an RV is being added in "outofsync" or "syncing" state (and it was in a different
		// 	     state earlier) we must also update rvInfo.reservedSpace.
		//
	}

	//
	// Update unconditionally, even if it may not have changed, doesn't matter.
	// We force the update as this is the membership info that we got from clustermap.
	//
	mv.updateComponentRVs(newComponentRVs, true /* forceUpdate */)

	//
	// TODO: Remove any syncJobs which are no longer running.
	//

	return nil
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

	common.Assert(mv.opMutexDbgCntr.Load() >= 0, mv.opMutexDbgCntr.Load())
	mv.opMutexDbgCntr.Add(1)
}

// release the read lock on the opMutex
func (mv *mvInfo) releaseSyncOpReadLock() {
	common.Assert(mv.opMutexDbgCntr.Load() > 0, mv.opMutexDbgCntr.Load())
	mv.opMutexDbgCntr.Add(-1)

	mv.opMutex.RUnlock()
}

// acquire write lock on the opMutex.
// This will wait till all the ongoing chunk IO operations are completed
// and will block any new chunk IO operations.
// This is used in StartSync and EndSync RPC calls.
func (mv *mvInfo) acquireSyncOpWriteLock() {
	mv.opMutex.Lock()
	log.Debug("mvInfo::acquireSyncOpWriteLock: acquired write lock by sync operation in MV %s", mv.mvName)

	common.Assert(mv.opMutexDbgCntr.Load() == 0, mv.opMutexDbgCntr.Load())
	mv.opMutexDbgCntr.Store(-12345) // Special value to signify write lock.

}

// release the write lock on the opMutex
func (mv *mvInfo) releaseSyncOpWriteLock() {
	common.Assert(mv.opMutexDbgCntr.Load() == -12345, mv.opMutexDbgCntr.Load())
	mv.opMutexDbgCntr.Store(0)

	mv.opMutex.Unlock()
	log.Debug("mvInfo::releaseSyncOpWriteLock: released write lock by sync operation in MV %s", mv.mvName)
}

// Check if read/shared lock is held on opMutex.
// [DEBUG ONLY]
func (mv *mvInfo) isSyncOpReadLocked() bool {
	return mv.opMutexDbgCntr.Load() > 0
}

// Check if write/exclusive lock is held on opMutex.
// [DEBUG ONLY]
func (mv *mvInfo) isSyncOpWriteLocked() bool {
	return mv.opMutexDbgCntr.Load() == -12345
}

// Check if the component RVs in the request is valid for the given MV replica.
// checkState boolean flag indicates if the state of the component RVs in the request should also
// be matched against the state of the component RVs in the mvInfo data, o/w only the component RV
// names are matched.
//
// If the request's component RVs match with the node's mvInfo data, then it means that the request
// is valid and the mvInfo data is up to date.
//
// If the request's component RVs do not match with the mvInfo data, it means that either,
//   - The sender has a stale clustermap.
//   - mvInfo has inconsistent info due to the partially applied change.
//
// In this case, mvInfo membership details need to be refreshed from the clustermap and if there still
// is a mismatch indicating client using stale clustermap, then we need to fail the client call with
// NeedToRefreshClusterMap asking the sender to refresh and retry.
// This function helps to refresh the mvInfo component RV details and returns the NeedToRefreshClusterMap
// if the component RV details don't match. Caller should then pass on the error eventually failing the
// RPC server method with NeedToRefreshClusterMap.
func (mv *mvInfo) isComponentRVsValid(componentRVsInReq []*models.RVNameAndState, checkState bool) error {
	var componentRVsInMV []*models.RVNameAndState
	clustermapRefreshed := false

	for {
		componentRVsInMV = mv.getComponentRVs()

		//
		// Component RVs received in req must be exactly same as component RVs list for this MV replica.
		// We may fail once due to out-of-date cluster membership info, refresh clustermap and try once
		// more.
		//
		err := isComponentRVsValid(componentRVsInMV, componentRVsInReq, checkState)
		if err != nil {
			if !clustermapRefreshed {
				mv.refreshFromClustermap()
				clustermapRefreshed = true
				continue
			}

			errStr := fmt.Sprintf("Request component RVs are invalid for MV %s [%v]", mv.mvName, err)
			log.Err("ChunkServiceHandler::isComponentRVsValid: %s", errStr)
			return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

		break
	}

	return nil
}

// Given component RVs and source and target RV names received in a StartSync/EndSync request, check their validity.
// It checks the following:
//   - Component RVs received in req are the same as component RVs list for this MV replica.
//     Only the component RV names are matched and not their state.
//     This is because multiple syncjobs could be simultaneously syncing different replicas of the same MV,
//     so the state of each will be changing from outofsync->syncing->online and we don't want to consider
//     that as a mismatch, else multiple sync jobs for the same MV cannot seamlessely proceed.
//   - Source and target RVs are indeed present in the component RVs list for this MV replica.
//   - Target RV is in the correct state based on the StartSync/EndSync request.
//     For StartSync() call, target RV must be in outofsync state. Whereas for EndSync() call,
//     target RV must be in syncing state.
//
// Note: This is a very critical correctness check used by dcache. Since client may be using a stale clustermap,
//
//	it's important for server (which always has the latest cluster membership info) to let client know if
//	its clustermap copy is stale and it needs to refresh it.
func (mv *mvInfo) validateComponentRVsInSync(componentRVsInReq []*models.RVNameAndState,
	sourceRVName string, targetRVName string, isStartSync bool) error {

	common.Assert(cm.IsValidRVName(sourceRVName) &&
		cm.IsValidRVName(targetRVName) &&
		sourceRVName != targetRVName, sourceRVName, targetRVName)

	//
	// validate the component RVs in request against the component RVs in mvInfo.
	// The state of the component RVs in the request is not checked for StartSync/EndSync requests.
	//
	err := mv.isComponentRVsValid(componentRVsInReq, false /* checkState */)
	if err != nil {
		log.Err("ChunkServiceHandler::validateComponentRVsInSync: %v", err)
		return err
	}

	componentRVsInMV := mv.getComponentRVs()

	// Source RV must be present in the component RVs list for this MV replica.
	if !isRVPresentInMV(componentRVsInMV, sourceRVName) {
		rvsInMvStr := rpc.ComponentRVsToString(componentRVsInMV)
		errStr := fmt.Sprintf("Source RV %s is not part of MV %s %s",
			sourceRVName, mv.mvName, rvsInMvStr)
		log.Err("ChunkServiceHandler::validateComponentRVsInSync: %s", errStr)
		return rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	// Target RV must be present in the component RVs list for this MV replica.
	if !isRVPresentInMV(componentRVsInMV, targetRVName) {
		rvsInMvStr := rpc.ComponentRVsToString(componentRVsInMV)
		errStr := fmt.Sprintf("Target RV %s is not part of MV %s %s",
			targetRVName, mv.mvName, rvsInMvStr)
		log.Err("ChunkServiceHandler::validateComponentRVsInSync: %s", errStr)
		return rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	//
	// Now that the target RV is present in the component RVs list for this MV replica,
	// validate its state based on the StartSync/EndSync request.
	//
	// StartSync() call is made after the fix-mv workflow has replaced the offline
	// RVs and marked the new/target RVs state as outofsync.
	//
	// EndSync() RPC call is made only after the StartSync() call, which marks the
	// target RV state to syncing.
	//
	// If the isStartSync flag is true, it means that the target RV should be in outofsync state for
	// StartSync() call.
	// Else, check if the target RV is in syncing state for EndSync() call.
	//
	var validState string

	if isStartSync {
		validState = string(dcache.StateOutOfSync)
	} else {
		validState = string(dcache.StateSyncing)
	}

	clustermapRefreshed := false

	for {
		targetRVNameAndState := mv.getComponentRVNameAndState(targetRVName)

		//
		// Q: Why refreshFromClustermap() is needed?
		// A: If we are hosting the target RV, then this validState change must have been approved by us
		//    (prior JoinMV or StartSync) and only after that the sender could have committed the state
		//    change in clustermap. If we do not have the validState in our rvInfo then it cannot be in the
		//    clustermap and if it's not in the clustermap sender won't have sent the StartSync/EndSync RPC.
		//    Note that even if we are not hosting the target RV, we would have been informed through a
		//	  StartSync request and we must have acknowledged it.
		//
		//    There is one possibility though. A prior StartSync succeeded and the mvInfo state was changed to
		//    syncing, but the sender couldn' persist that change in the clustermap (some node that was updating
		//    the clustermap took really long, due to some other node being down and JoinMV taking long time).
		//    Meanwhile the lowest online RV on the node attempting the sync is marked offline in clustermap,
		//	  so some other node now has the lowest online RV, and that node now attempts the sync. It sends a
		//	  StartSync RPC to this RV which is already marked syncing by the previous StartSync.
		//
		if targetRVNameAndState.State != validState {
			errStr := fmt.Sprintf("Target RV %s is not in %s state (%s/%s -> %s/%s): %s [NeedToRefreshClusterMap]",
				targetRVName, validState,
				sourceRVName, mv.mvName,
				targetRVName, mv.mvName,
				rpc.ComponentRVsToString(mv.getComponentRVs()))

			log.Err("ChunkServiceHandler::validateComponentRVsInSync: %s, clustermapRefreshed: %v",
				errStr, clustermapRefreshed)

			if !clustermapRefreshed {
				mv.refreshFromClustermap()
				clustermapRefreshed = true
				continue
			}

			common.Assert(false, errStr)
			return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

		break
	}

	return nil
}

// check the if the chunk address is valid
// - check if the rvID is valid
// - check if the cache dir exists
// - check if the MV is valid
func (h *ChunkServiceHandler) checkValidChunkAddress(address *models.Address) error {
	common.Assert(address != nil)
	common.Assert(common.IsValidUUID(address.FileID), address.FileID)
	common.Assert(common.IsValidUUID(address.RvID), address.RvID)
	common.Assert(cm.IsValidMVName(address.MvName), address.MvName)

	// rvID must refer to one of of out local RVs.
	rvInfo, ok := h.rvIDMap[address.RvID]
	common.Assert(!ok || rvInfo != nil, address.RvID)
	if !ok {
		errStr := fmt.Sprintf("Invalid rvID %s", address.RvID)
		log.Err("ChunkServiceHandler::checkValidChunkAddress: %s", errStr)
		return rpc.NewResponseError(models.ErrorCode_InvalidRVID, errStr)
	}

	cacheDir := rvInfo.cacheDir
	common.Assert(cacheDir != "", rvInfo.rvName)
	common.Assert(common.DirectoryExists(cacheDir), cacheDir, rvInfo.rvName)

	//
	// MV replica must exist.
	//
	// Q: Why refreshFromClustermap() cannot help?
	// A: An RV can be added as a component RV to an MV only after approval from the node hosting the RV,
	//    through a JoinMV call. Only after a successful JoinMV response would the caller update the MV
	//    component RV list. If we do not have this MV added to our RV, that means we would not have
	//    responded to the JoinMV RPC, which would mean the clustermap cannot have it.
	//    For rebalancing, a component RV would be removed from an MV only after the rebalancing has
	//    completed and there's no undoing it.
	//
	mvPath := filepath.Join(cacheDir, address.MvName)
	if !rvInfo.isMvPathValid(mvPath) {
		errStr := fmt.Sprintf("MV %s is not hosted by RV %s [NeedToRefreshClusterMap]",
			address.MvName, rvInfo.rvName)
		log.Err("ChunkServiceHandler::checkValidChunkAddress: %s", errStr)
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	return nil
}

// Get rvInfo for a given RV name that corresponds to one of our local RVs.
func (h *ChunkServiceHandler) getRVInfoFromRVName(rvName string) *rvInfo {
	var rvInfo *rvInfo
	for rvID, info := range h.rvIDMap {
		common.Assert(info != nil, rvID)

		if info.rvName == rvName {
			rvInfo = info
			break
		}
	}

	return rvInfo
}

func (h *ChunkServiceHandler) createMVDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("MkdirAll(%s) failed: %v", path, err)
	}

	log.Debug("ChunkServiceHandler::createMVDirectory: Created MV directory %s", path)

	return nil
}

// Return source or target RV info for the sync operation. Only one of the source or target RV can be hosted by this
// node, so one and only one of source or target rvInfo will be non-nil.
// - If neither source nor target RVs is hosted by this node, return error.
// - If both source and target RVs are hosted by this node, return error.
func (h *ChunkServiceHandler) getSrcAndDestRVInfoForSync(sourceRVName string, targetRVName string) (*rvInfo, *rvInfo, error) {
	srcRVInfo := h.getRVInfoFromRVName(sourceRVName)
	targetRVInfo := h.getRVInfoFromRVName(targetRVName)

	if srcRVInfo == nil && targetRVInfo == nil {
		errStr := fmt.Sprintf("Neither source RV %s nor target RV %s is hosted by this node",
			sourceRVName, targetRVName)
		log.Err("ChunkServiceHandler::getSrcAndDestRVInfoForSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	if srcRVInfo != nil && targetRVInfo != nil {
		errStr := fmt.Sprintf("Both source RV %s and target RV %s are hosted by this node",
			sourceRVName, targetRVName)
		log.Err("ChunkServiceHandler::getSrcAndDestRVInfoForSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	return srcRVInfo, targetRVInfo, nil
}

func (h *ChunkServiceHandler) Hello(ctx context.Context, req *models.HelloRequest) (*models.HelloResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::Hello: Received Hello request: %v", req.String())

	// TODO: send more information in response on Hello RPC

	myNodeID := rpc.GetMyNodeUUID()
	common.Assert(req.ReceiverNodeID == myNodeID,
		"Received Hello RPC destined for another node", req.ReceiverNodeID, myNodeID)

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
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Address != nil)

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request: %v", rpc.GetChunkRequestToString(req))

	//
	// Check if the chunk address is valid. We basically check for the following:
	// - RV id in the chunk address is one of our local RVs.
	// - MV name in the chunk address is indeed hosted by that RV.
	//
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Invalid chunk address %v [%s]",
			req.Address.String(), err.Error())
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	//
	// RVInfo validation.
	// The only RVInfo validation needed for GetChunk request is that the target RV is indeed a valid
	// component RV for this MV and it's in a valid state for serving chunks. "online" is the only valid state
	// when a component RV can serve chunks. offline/outofsync RVs cannot serve the chunks so sender should
	// not have requested the GetChunk to those, if we get a GetChunk for those RVs it means client has a
	// stale clustermap and hence we must help the client by failing with NeedToRefreshClusterMap.
	// Similarly "syncing" RV may or may not have the chunk yet, and client should not be asking a chunk from
	// a syncing component RV, so again we play safe and let the client know about it.
	//
	// checkValidChunkAddress() has already done the membership check, so we just need to do the state
	// check.
	//
	rvNameAndState := mvInfo.getComponentRVNameAndState(rvInfo.rvName)

	// checkValidChunkAddress() had succeeded above, so RV must exist.
	common.Assert(rvNameAndState != nil)

	//
	// We allow reading only from "online" component RVs.
	// Note: Though we may be able to serve the chunk from a component RV in "syncing" or even "offline"
	//       state, it usually indicates client using an older clustermap so we rather ask the client to refresh.
	// TODO: See if going ahead and checking the chunk anyways is better.
	//
	// Q: Why refreshFromClustermap() cannot help this?
	// A: Let's consder all possible RV states other than online:
	//    - offline
	//      There's no workflow to set rvInfo state as offline, but due to mvInfo.refreshFromClustermap()
	//		we can have a component RV state as offline. If the state were to change from offline, it must
	//      be through JoinMV/UpdateMV RPC, so we must be in the loop.
	//    - outofsync
	//      outofsync state can be set through the fix-mv workflow when it replaces an offline component RV
	//		with a new RV. The new RVs state will be set to outofsync through the JoinMV RPC call, but before
	//		this component RV is considered for reading it must have been updated to syncing->online, both
	//      of which need to be approved by us. So if we are in outofsync, sender cannot legitimately be
	//      reading from us.
	//    - syncing
	//      Same as above. Data can be read from an mv replica only after it goes from syncing->online
	//      through an EndSync call, which must be approved by us.
	//
	if rvNameAndState.State != string(dcache.StateOnline) {
		errStr := fmt.Sprintf("GetChunk request for %s/%s cannot be satisfied in state %s [NeedToRefreshClusterMap]",
			rvInfo.rvName, req.Address.MvName, rvNameAndState.State)
		log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
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
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, fmt.Sprintf("failed to open chunk file %s [%v]", chunkPath, err.Error()))
	}
	defer fh.Close()

	fInfo, err := fh.Stat()
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to stat chunk file %s [%v]", chunkPath, err.Error())
		common.Assert(false, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
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
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to read chunk file %s [%v]", chunkPath, err.Error()))
	}

	// TODO: hash validation will be done later
	// get hash if requested for entire chunk
	// hash := ""
	// if req.OffsetInChunk == 0 && req.Length == chunkSize {
	//      hashData, err := os.ReadFile(hashPath)
	//      if err != nil {
	//              log.Err("ChunkServiceHandler::GetChunk: Failed to read hash file %s [%v]", hashPath, err.Error())
	//              return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to read hash file %s [%v]", hashPath, err.Error()))
	//      }
	//      hash = string(hashData)
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
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request: %v", rpc.PutChunkRequestToString(req))

	// Check if the chunk address is valid.
	err := h.checkValidChunkAddress(req.Chunk.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Invalid chunk address %v [%v]",
			req.Chunk.Address.String(), err)
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Chunk.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Chunk.Address.MvName)

	//
	// RVInfo validation. PutChunk(client) and PutChunk(sync) need different validations.
	//
	// For a PutChunk(client) we need to do the following validation.
	// For all component RVs specified in the PutChunk request, ensure:
	// - If the component RV is offline/outofsync it's offline/outofsync in the RV Info's component RV list too.
	//   This is required to ensure that client/sender didn't skip PutChunk to a component RV which won't be
	//   sync'ed later.
	// - If the component RV is either online/syncing it's present in the RV Info's component RV list and has
	//   state either online or syncing.
	//   This is required to ensure that client/sender is not writing to different set of component RVs which
	//   may be futile and may result in missing writing chunks to some valid component RVs.
	// - There should not be any component RV different between the two lists. This is a corollary to the
	//   above two.
	//
	// For a PutChunk(sync) we need to do the following validation.
	// PutChunk(sync) is only concerned about a specific sync job, from one (online) source RV to one
	// (outofsync) target RV. We just need to ensure sanity of that specific PutChunk.
	// We need to check if the SyncId carried in the PutChunk(sync) request indeed refers to an active
	// sync job and this MV replica is indeed the target of that sync job.
	//

	//
	// Acquire read lock on the opMutex for this MV to block any StartSync request from updating rvInfo while
	// we are accessing it. Note that depending on the sync state of an MV replica, the client PutChunk requests
	// may need to be saved in regular or the sync mv folder. This read lock prevents any races in that.
	//
	mvInfo.acquireSyncOpReadLock()
	defer mvInfo.releaseSyncOpReadLock()

	clustermapRefreshed := false

refreshFromClustermapAndRetry:
	componentRVsInMV := mvInfo.getComponentRVs()

	if len(req.SyncID) == 0 {
		//
		// PutChunk(client) - Make sure caller only skipped offline or outofsync component RVs.
		//
		common.Assert(len(req.ComponentRV) == len(componentRVsInMV),
			len(req.ComponentRV), len(componentRVsInMV))

		for _, rv := range req.ComponentRV {
			common.Assert(rv != nil)

			// Component RV details from mvInfo.
			rvNameAndState := mvInfo.getComponentRVNameAndState(rv.Name)

			//
			// Sender's clustermap has a component RV which is not part of this MV.
			//
			// Q: Why refreshFromClustermap() cannot help this?
			// A: An RV can be added to an MV in the clustermap only after successful JoinMV+UpdateMV calls
			//    to all the component RVs. If we don't have the MV added to the rvInfo we must not have
			//    responded positively to JoinMV/UpdateMV, so sender must not have updated the clustermap.
			//    Hence we also assert for this.
			//
			if rvNameAndState == nil {
				errStr := fmt.Sprintf("PutChunk(client) sender has a non-existent RV %s/%s",
					rv.Name, req.Chunk.Address.MvName)
				log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
				common.Assert(false, errStr)
				return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
			}

			// Sender would skip component RVs which are either offline or outofsync.
			senderSkippedRV := (rv.State == string(dcache.StateOffline) ||
				rv.State == string(dcache.StateOutOfSync))
			// If RV info has the RV as offline or outofsync it'll be properly sync'ed later.
			isRVSafeToSkip := (rvNameAndState.State == string(dcache.StateOffline) ||
				rvNameAndState.State == string(dcache.StateOutOfSync))

			if senderSkippedRV && !isRVSafeToSkip {
				//
				// This can happen when sender comes to know about an RV being offline, through clustermap,
				// obviously since RV state has not changed as a result of some workflow, hence rvInfo is
				// not updated and it doesn't know about the RV going offline.
				// We must refresh our rvInfo from the clustermap and retry the check.
				//
				errStr := fmt.Sprintf("PutChunk(client) sender skipped RV %s/%s in invalid state %s [NeedToRefreshClusterMap]",
					rv.Name, req.Chunk.Address.MvName, rvNameAndState.State)
				log.Err("ChunkServiceHandler::PutChunk: %s", errStr)

				if !clustermapRefreshed {
					mvInfo.refreshFromClustermap()
					clustermapRefreshed = true
					goto refreshFromClustermapAndRetry
				}

				return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
			}
		}
	} else {
		//
		// PutChunk(sync) - Make sure the target MV replica is indeed target of this sync job.
		//
		// Q: Why refreshFromClustermap() cannot help this?
		// A: PutChunk(sync) requests can only be sent after a successful StartSync response from
		//    us and when we would have responded we would have added the syncJob.
		//
		syncJob, ok := mvInfo.syncJobs[req.SyncID]
		if !ok {
			errStr := fmt.Sprintf("PutChunk(sync) syncId %s not valid for %s/%s [NeedToRefreshClusterMap]",
				req.SyncID, rvInfo.rvName, req.Chunk.Address.MvName)
			log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

		common.Assert(syncJob.targetRVName == "")
		common.Assert(syncJob.sourceRVName != "")
	}

	// TODO: check later if lock is needed
	// acquire lock for the chunk address to prevent concurrent writes
	// chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.RvID, req.Chunk.Address.MvName, req.Chunk.Address.OffsetInMiB)
	// flock := h.locks.Get(chunkAddress)
	// flock.Lock()
	// defer flock.Unlock()

	cacheDir := rvInfo.cacheDir
	isSrcOfSync, isTgtOfSync := mvInfo.isSourceOrTargetOfSync()

	var chunkPath, hashPath string
	if len(req.SyncID) > 0 {
		//
		// Sync PutChunk call (as opposed to a client write PutChunk call).
		// This is called after the StartSync RPC to synchronize an OutOfSyc MV replica from a healthy MV
		// replica.
		// In this case the chunks must be written to the regular mv directory, i.e. rv0/mv0
		//
		// Sync PutChunk call will be made in the ResyncMV() workflow, and should only be sent to RVs which
		// are target of a sync job.
		//
		if !isTgtOfSync {
			errStr := fmt.Sprintf("Sync PutChunk call received for %s/%s, which is currently not the target of any sync job",
				rvInfo.rvName, req.Chunk.Address.MvName)

			log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
		}

		chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName,
			req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
	} else {
		//
		// Client write PutChunk call. If this MV replica is currently acting as the source for any sync job,
		// the chunks must be written to the sync directory, i.e. rv0/mv0.sync, else they must be written
		// to the regular mv directory, i.e. rv0/mv0.
		//
		if isSrcOfSync {
			chunkPath, hashPath = getSyncMVPath(cacheDir, req.Chunk.Address.MvName,
				req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
		} else {
			chunkPath, hashPath = getRegularMVPath(cacheDir, req.Chunk.Address.MvName,
				req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)
		}
	}

	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	// Chunk file must not be present.
	_, err = os.Stat(chunkPath)
	if err == nil {
		errStr := fmt.Sprintf("Chunk file %s already exists", chunkPath)
		log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkAlreadyExists, errStr)
	}

	// Write to .tmp file first and rename it to the final file.
	tmpChunkPath := fmt.Sprintf("%s.tmp", chunkPath)
	err = os.WriteFile(tmpChunkPath, req.Chunk.Data, 0400)
	if err != nil {
		errStr := fmt.Sprintf("Failed to write chunk file %s [%v]", chunkPath, err)
		log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	// TODO: hash validation will be done later
	// err = os.WriteFile(hashPath, []byte(req.Chunk.Hash), 0400)
	// if err != nil {
	//      log.Err("ChunkServiceHandler::PutChunk: Failed to write hash file %s [%v]", hashPath, err.Error())
	//      return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to write hash file %s [%v]", hashPath, err.Error()))
	// }

	availableSpace, err := rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err)
	}

	// TODO: should we verify the hash after writing the chunk

	// rename the .tmp file to the final file
	err = os.Rename(tmpChunkPath, chunkPath)
	if err != nil {
		errStr := fmt.Sprintf("Failed to rename chunk file %s -> %s [%v]",
			tmpChunkPath, chunkPath, err)
		log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	// TODO: should we also consider the hash file size in the total chunk bytes
	//       For accurate accounting we can, but we should not do an extra stat() call for the hash file
	//       but instead use a hardcoded value which will be true for a given hash algo.
	//       Also we need to be sure that hash is calculated uniformly (either always or never)

	// Increment the total chunk bytes for this MV.
	mvInfo.incTotalChunkBytes(req.Length)

	//
	// For successful sync PutChunk calls, decrement the reserved space for this RV.
	// JoinMV would have reserved this space before starting sync.
	//
	if len(req.SyncID) > 0 {
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
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Address != nil)

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
	err = mvInfo.isComponentRVsValid(req.ComponentRV, true /* checkState */)
	if err != nil {
		errStr := fmt.Sprintf("Component RVs are invalid for MV %s [%v]", req.Address.MvName, err)
		log.Err("ChunkServiceHandler::RemoveChunk: %s", errStr)
		return nil, err
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
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, fmt.Sprintf("failed to stat chunk file %s [%v]", chunkPath, err.Error()))
	}

	err = os.Remove(chunkPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove chunk file %s [%v]", chunkPath, err.Error())
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to remove chunk file %s [%v]", chunkPath, err.Error()))
	}

	// TODO: hash validation will be done later
	// err = os.Remove(hashPath)
	// if err != nil {
	//      log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove hash file %s [%v]", hashPath, err.Error())
	//      return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to remove hash file %s [%v]", hashPath, err.Error()))
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
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	// TODO:: discuss: changing type of component RV from string to RVNameAndState
	// requires to call componentRVsToString method as it is of type []*models.RVNameAndState
	log.Debug("ChunkServiceHandler::JoinMV: Received JoinMV request: %v", rpc.JoinMVRequestToString(req))

	if !cm.IsValidMVName(req.MV) || !cm.IsValidRVName(req.RVName) || len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid MV, RV or ComponentRV: %v", rpc.JoinMVRequestToString(req))
		log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		errStr := fmt.Sprintf("node %s does not host %s", rpc.GetMyNodeUUID(), req.RVName)
		log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	cacheDir := rvInfo.cacheDir

	// Acquire lock for the RV to prevent concurrent JoinMV calls for different MVs.
	flock := h.locks.Get(rvInfo.rvID)
	flock.Lock()
	defer flock.Unlock()

	// Check if RV is already part of the given MV.
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo != nil {
		//
		// TODO: Till Sourav formally implements idempotent handling of JoinMV and UpdateMV RPCs,
		//	 we have the following to not treat "double join" as failure.
		//	 Double join can happen when let's say we have two or more outofsync component RVs
		//	 for an MV and fixMV() sends JoinMV request to each of the outofsync RVs. If one or
		//	 more of these fail, the joinMV() will treat it as a failure and not update clustermap.
		//	 Next time when fixMV() is called it'll again attempt fixing and again send JoinMV.
		//	 Note that for proper handling we need to ensure that the reservedSpace remains
		//	 same across both calls. Also if an RV is joined but never used later (maybe joinMV()
		//	 picked a new RV in the next iteration), we should time out and undo the reservedSpace.
		//
		log.Warn("ChunkServiceHandler::JoinMV: %s is already part of %s, ignoring", req.RVName, req.MV)
		return &models.JoinMVResponse{}, nil
	}

	mvLimit := getMVsPerRV()
	if rvInfo.mvCount.Load() >= mvLimit {
		//
		// TODO: This might happen due to incomplete JoinMV requests taking up space, so it will help
		//		 to refresh rvInfo details from the clustermap and remove any unused MVs, and try again.
		//
		errStr := fmt.Sprintf("%s cannot host any more MVs (MVsPerRv: %d)", req.RVName, mvLimit)
		log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_MaxMVsExceeded, errStr)
	}

	//
	// JoinMV is used both for new-mv and fix-mv workflows.
	// For new-mv, req.ReserveSpace will be 0 as there's no specific space requirement, but in the fix-mv
	// case this RV will have to store one copy of MVs data, so it must have that much free space.
	//
	if req.ReserveSpace != 0 {
		availableSpace, err := rvInfo.getAvailableSpace()
		if err != nil {
			errStr := fmt.Sprintf("failed to get available disk space for %v [%v]", req.RVName, err)
			log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
		}

		// TODO: should we keep some buffer space for the MV,
		// like reserve space should be 20% less than available space
		if availableSpace < req.ReserveSpace {
			errStr := fmt.Sprintf("not enough space in %s to reserve %d bytes for %s, has only %d bytes",
				req.RVName, req.ReserveSpace, req.MV, availableSpace)
			log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
		}
	}

	// Create the MV directory.
	mvPath := filepath.Join(cacheDir, req.MV)
	err := h.createMVDirectory(mvPath)
	if err != nil {
		errStr := fmt.Sprintf("failed to create MV directory %s [%v]", mvPath, err)
		log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	//
	// Add the newly created MV replica to the MV map for the RV.
	// JoinMV is not transactional, so if one or more JoinMVs fail, the caller won't rollback but simply
	// leave the debris. Note that in case of failure the clustermap won't be updated so we can find out
	// from the clustermap, and we use that to resolve conflicts when they arise, not proactively.
	// But, the space reservation needs to be undone, else we may run out of space due to these incomplete
	// JoinMV calls.
	//
	sortComponentRVs(req.ComponentRV)
	rvInfo.addToMVMap(req.MV, newMVInfo(rvInfo.rvName, req.MV, req.ComponentRV))

	// Increment the reserved space for this RV.
	rvInfo.incReservedSpace(req.ReserveSpace)

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) UpdateMV(ctx context.Context, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::UpdateMV: Received UpdateMV request: %v", rpc.UpdateMVRequestToString(req))

	if !cm.IsValidMVName(req.MV) || !cm.IsValidRVName(req.RVName) || len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid MV, RV or ComponentRV: %v", rpc.UpdateMVRequestToString(req))
		log.Err("ChunkServiceHandler::UpdateMV: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	clustermapRefreshed := false
	for {
		rvInfo := h.getRVInfoFromRVName(req.RVName)
		if rvInfo == nil {
			errStr := fmt.Sprintf("node %s does not host %s", rpc.GetMyNodeUUID(), req.RVName)
			log.Err("ChunkServiceHandler::UpdateMV: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
		}

		//
		// A membership update RPC is only sent to RVs which are already members of the MV, and it is sent
		// when the membership changes (an existing RV is replaced by another RV by the fix-mv workflow).
		// Since the sender is referring to the global clustermap and this RV is part of the given MV as
		// per the global clustermap, since an RV is added to an MV only after a successful JoinMV response
		// from all component RVs, we *must* have the MV replica in our rvInfo.
		//
		mvInfo := rvInfo.getMVInfo(req.MV)
		if mvInfo == nil {
			errStr := fmt.Sprintf("%s is not part of %s", req.RVName, req.MV)
			log.Err("ChunkServiceHandler::UpdateMV: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
		}

		componentRVsInMV := mvInfo.getComponentRVs()

		log.Debug("ChunkServiceHandler::UpdateMV: Updating %s from (%s -> %s)",
			req.MV, rpc.ComponentRVsToString(componentRVsInMV), rpc.ComponentRVsToString(req.ComponentRV))

		//
		// update the component RVs list for this MV
		// mvInfo.updateComponentRVs() only allows valid changes to cluster membership.
		//
		// Note: Updating this unconditionally could be risky.
		//       A node with an outdated clustermap can reverse a later change.
		//       f.e. some node is syncing and has changed state of an rv to syncing
		//       meanwhile some other node with an older clustermap wants to join an MV to this rv.
		//       it fetched clustermap but then due to n/w down, by the time it reached fixMV, rv was
		//		 already marked syncing, but now it has rv as outofsync and it forces it as that
		//
		err := mvInfo.updateComponentRVs(req.ComponentRV, false /* forceUpdate */)
		if err != nil {
			if !clustermapRefreshed {
				mvInfo.refreshFromClustermap()
				clustermapRefreshed = true
				continue
			}

			return nil, err
		}

		break
	}

	return &models.UpdateMVResponse{}, nil
}

func (h *ChunkServiceHandler) LeaveMV(ctx context.Context, req *models.LeaveMVRequest) (*models.LeaveMVResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::LeaveMV: Received LeaveMV request: %v", rpc.LeaveMVRequestToString(req))

	if !cm.IsValidMVName(req.MV) || !cm.IsValidRVName(req.RVName) || len(req.ComponentRV) == 0 {
		log.Err("ChunkServiceHandler::LeaveMV: MV, RV or ComponentRV is empty")
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, "MV, RV or ComponentRV is empty")
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RVName)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, fmt.Sprintf("invalid RV %s", req.RVName))
	}

	cacheDir := rvInfo.cacheDir

	// check if RV is part of the given MV
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: RV %s is not part of the given MV %s", req.RVName, req.MV)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, fmt.Sprintf("RV %s is not part of the given MV %s", req.RVName, req.MV))
	}

	// validate the component RVs list
	err := mvInfo.isComponentRVsValid(req.ComponentRV, true /* checkState */)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: %v", err)
		return nil, err
	}

	// delete the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)
	flock := h.locks.Get(mvPath) // TODO: check if lock is needed in directory deletion
	flock.Lock()
	defer flock.Unlock()

	err = os.RemoveAll(mvPath)
	if err != nil {
		log.Err("ChunkServiceHandler::LeaveMV: Failed to remove MV directory %s [%v]", mvPath, err.Error())
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, fmt.Sprintf("failed to remove MV directory %s [%v]", mvPath, err.Error()))
	}

	// add in sync map
	rvInfo.deleteFromMVMap(req.MV)

	return &models.LeaveMVResponse{}, nil
}

func (h *ChunkServiceHandler) StartSync(ctx context.Context, req *models.StartSyncRequest) (*models.StartSyncResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::StartSync: Received StartSync request: %s",
		rpc.StartSyncRequestToString(req))

	if !cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.SourceRVName) ||
		!cm.IsValidRVName(req.TargetRVName) ||
		req.SourceRVName == req.TargetRVName ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("MV (%s), SourceRV (%s), TargetRV (%s) or ComponentRVs (%d) invalid",
			req.MV, req.SourceRVName, req.TargetRVName, len(req.ComponentRV))
		log.Err("ChunkServiceHandler::StartSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	//
	// Source RV is the lowest index online RV. The node hosting this RV will send the start sync call
	// to the outofsync component RVs.
	//
	srcRVInfo, targetRVInfo, err := h.getSrcAndDestRVInfoForSync(req.SourceRVName, req.TargetRVName)
	if err != nil {
		log.Err("ChunkServiceHandler::StartSync: Failed to get source and target RV info [%v]",
			err)
		common.Assert(false, err)
		return nil, err
	}

	var rvInfo *rvInfo
	var isSrcOfSync bool

	if srcRVInfo != nil {
		common.Assert(targetRVInfo == nil)
		rvInfo = srcRVInfo
		isSrcOfSync = true
	} else {
		common.Assert(targetRVInfo != nil)
		rvInfo = targetRVInfo
	}

	//
	// Check if we are hosting the requested MV replica.
	//
	// Q: Why refreshFromClustermap() cannot help this?
	// A: An MV replica can be added to rvInfo only via a JoinMV RPC, and only when we respond successfully
	//    to the JoinMV call will the sender persist it in the clustermap, so if the clustermap has it we
	//    must have sent it and if we don't have it, refreshing from clustermap cannot add it.
	//    This cannot happen unless sender is doing something wrong, hence assert.
	//
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		errStr := fmt.Sprintf("%s/%s not hosted by this node", rvInfo.rvName, req.MV)
		log.Err("ChunkServiceHandler::StartSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	err = mvInfo.validateComponentRVsInSync(req.ComponentRV, req.SourceRVName, req.TargetRVName, true /* isStartSync */)
	if err != nil {
		errStr := fmt.Sprintf("Failed to validate component RVs in sync [%v]", err)
		log.Err("ChunkServiceHandler::StartSync: %s", errStr)
		return nil, err
	}

	//
	// Ok, it's a valid StartSync request for one of our MV replicas.
	// We synchronize chunk IO requests (GetChunk/PutChunk/RemoveChunk) with StartSync requests.
	// Acquire write lock on the opMutex for this MV. Now GetChunk, PutChunk and RemoveChunk will not allow
	// any new IO. It will also wait for any ongoing IOs to complete.
	//
	mvInfo.acquireSyncOpWriteLock()
	defer mvInfo.releaseSyncOpWriteLock()

	//
	// If this MV replica is the source of this sync job, we will need the .sync directory,
	// create if it doesn't exist.
	//
	if isSrcOfSync {
		// Create MV sync directory if it doesn't exist.
		syncDir := filepath.Join(rvInfo.cacheDir, req.MV+".sync")
		err := h.createMVDirectory(syncDir)
		if err != nil {
			errStr := fmt.Sprintf("Failed to create sync directory %s [%v]", syncDir, err)
			log.Err("ChunkServiceHandler::StartSync: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
		}
	}

	//
	// If sourceRVName is set that means this MV Replica is the target of this sync job, while if
	// targetRVName is set it means this MV Replica is the source of this sync job.
	//
	var sourceRVName, targetRVName string

	if isSrcOfSync {
		targetRVName = req.TargetRVName
	} else {
		sourceRVName = req.SourceRVName
	}

	// Add this sync job to the syncJobs map.
	syncID := mvInfo.addSyncJob(sourceRVName, targetRVName)

	// Update the state of target RV in this MV replica from outofsync to syncing.
	mvInfo.updateComponentRVState(req.TargetRVName, dcache.StateOutOfSync, dcache.StateSyncing)

	log.Debug("ChunkServiceHandler::StartSync: Responding to StartSync request: %s, with syncID: %s",
		rpc.StartSyncRequestToString(req), syncID)

	return &models.StartSyncResponse{
		SyncID: syncID,
	}, nil
}

func (h *ChunkServiceHandler) EndSync(ctx context.Context, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::EndSync: Received EndSync request: %v", rpc.EndSyncRequestToString(req))

	if !common.IsValidUUID(req.SyncID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.SourceRVName) ||
		!cm.IsValidRVName(req.TargetRVName) ||
		req.SourceRVName == req.TargetRVName ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("SyncID (%s) MV (%s), SourceRV (%s), TargetRV (%s) or ComponentRVs (%d) invalid",
			req.SyncID, req.MV, req.SourceRVName, req.TargetRVName, len(req.ComponentRV))
		log.Err("ChunkServiceHandler::EndSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	//
	// Source RV is the lowest index online RV. The node hosting this RV will send the start sync call
	// to the outofsync component RVs.
	//
	srcRVInfo, targetRVInfo, err := h.getSrcAndDestRVInfoForSync(req.SourceRVName, req.TargetRVName)
	if err != nil {
		log.Err("ChunkServiceHandler::EndSync: Failed to get source and target RV info [%v]", err)
		common.Assert(false, err)
		return nil, err
	}

	var rvInfo *rvInfo
	var isSrcOfSync bool

	if srcRVInfo != nil {
		common.Assert(targetRVInfo == nil)
		rvInfo = srcRVInfo
		isSrcOfSync = true
	} else {
		common.Assert(targetRVInfo != nil)
		rvInfo = targetRVInfo
	}

	//
	// Check if we are hosting the requested MV replica.
	//
	// Q: Why refreshFromClustermap() cannot help this?
	// A: An MV replica can be added to rvInfo only via a JoinMV RPC, and only when we respond successfully
	//    to the JoinMV call will the sender persist it in the clustermap, so if the clustermap has it we
	//    must have sent it and if we don't have it, refreshing from clustermap cannot add it.
	//    This cannot happen unless sender is doing something wrong, hence assert.
	//
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		errStr := fmt.Sprintf("%s/%s not hosted by this node", rvInfo.rvName, req.MV)
		log.Err("ChunkServiceHandler::EndSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	err = mvInfo.validateComponentRVsInSync(req.ComponentRV, req.SourceRVName, req.TargetRVName, false /* isStartSync */)
	if err != nil {
		errStr := fmt.Sprintf("Failed to validate component RVs in sync [%v]", err)
		log.Err("ChunkServiceHandler::EndSync: %s", errStr)
		return nil, err
	}

	//
	// Ok, it's a valid EndSync request for one of our MV replicas.
	// We synchronize chunk IO requests (GetChunk/PutChunk/RemoveChunk) with EndSync requests.
	// Acquire write lock on the opMutex for this MV. Now GetChunk, PutChunk and RemoveChunk will not allow
	// any new IO. It will also wait for any ongoing IOs to complete.
	//
	mvInfo.acquireSyncOpWriteLock()
	defer mvInfo.releaseSyncOpWriteLock()

	//
	// EndSync must carry a valid syncID returned by a prior StartSync call.
	//
	if !mvInfo.isSyncIDValid(req.SyncID) {
		errStr := fmt.Sprintf("SyncID %s is invalid for %s/%s", req.SyncID, rvInfo.rvName, req.MV)
		log.Err("ChunkServiceHandler::EndSync: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	// Delete the sync job from the syncJobs map.
	mvInfo.deleteSyncJob(req.SyncID)

	// Update the state of target RV in this MV replica from syncing to online.
	mvInfo.updateComponentRVState(req.TargetRVName, dcache.StateSyncing, dcache.StateOnline)

	//
	// If we were the target of this sync job, then nothing else to do, else if it's the last sync
	// job, then we will need to move chunks from .sync folder and delete it.
	//
	if !isSrcOfSync {
		// An MV replica can be the target of only one sync job at a time.
		common.Assert(!mvInfo.isSyncing())
		return &models.EndSyncResponse{}, nil
	}

	//
	// After deleting this sync job, check if there are any other sync jobs in progress for this MV replica.
	// If yes, then return success for this EndSync call.
	// Else, this EndSync call is for the last running syncJob for this MV replica. So, move the chunks from
	// the sync folder to the regular MV folder and delete the sync folder.
	// This is done to avoid moving chunks from the sync folder to the regular MV folder if there are other
	// sync jobs in progress for this MV replica.
	//
	if mvInfo.isSyncing() {
		log.Debug("ChunkServiceHandler::EndSync: %s/%s is source replica for %d running sync job(s): %+v",
			rvInfo.rvName, req.MV, len(mvInfo.syncJobs), mvInfo.syncJobs)
		return &models.EndSyncResponse{}, nil
	}

	// Move all chunks from sync folder to the regular MV folder and then resume processing.
	regMVPath := filepath.Join(rvInfo.cacheDir, req.MV)
	syncMvPath := filepath.Join(rvInfo.cacheDir, req.MV+".sync")

	log.Debug("ChunkServiceHandler::EndSync: Moving chunks from %s -> %s", syncMvPath, regMVPath)

	err = moveChunksToRegularMVPath(syncMvPath, regMVPath)
	if err != nil {
		errStr := fmt.Sprintf("Failed to move chunks from %s -> %s [%v]",
			syncMvPath, regMVPath, err)
		log.Err("ChunkServiceHandler::EndSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	// Delete the sync directory. It must be empty now.
	err = os.Remove(syncMvPath)
	common.Assert(err == nil, fmt.Sprintf("failed to remove sync directory %s [%v]", syncMvPath, err))

	return &models.EndSyncResponse{}, nil
}
