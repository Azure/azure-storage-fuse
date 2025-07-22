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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/clustermap"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc"
	rpc_client "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/client"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/service"
	gouuid "github.com/google/uuid"
)

//go:generate $ASSERT_REMOVER $GOFILE

// type check to ensure that ChunkServiceHandler implements dcache.ChunkService interface
var _ service.ChunkService = &ChunkServiceHandler{}

// ChunkServiceHandler struct implements the ChunkService interface
type ChunkServiceHandler struct {
	//
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
	//
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
	// JoinMV() causes entries to be added to mvMap and LeaveMV() causes entries to be removed.
	// Also pruneStaleEntriesFromMvMap() can remove stale entries from mvMap. These are those MVs which are
	// present in rvInfo.mvMap, i.e., they are hosted on this RV as per rvInfo, but as per clustermap this RV
	// is not a component RV for this MV (these would be MVs for which the JoinMV was never completed).
	// For RVs that go offline and are thus removed from an MV, we don't need to remove them explicitly as an
	// offline RV when it joins the cluster again must start afresh, with brand new rvInfo.
	//
	mvMap sync.Map
	//
	// count of MV replicas hosted by this RV, this should be updated whenever an MV is added or removed from
	// the mvMap.
	//
	mvCount atomic.Int64

	//
	// This mutex has limited usage it's just to ensure mvCount and mvMap are update synchronously.
	// TODO: See if we need to extend the scope of locking.
	//
	rwMutex sync.RWMutex

	// Companion boolean flag to rwMutex to check if the lock is held or not.
	// [DEBUG ONLY]
	rwMutexDbgFlag atomic.Bool

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
	// RV name. This is for making the hosting RV available to functions that operate on mvInfo
	// and do not have the rvInfo.
	rv *rvInfo

	// sorted list of component RVs for this MV
	componentRVs []*models.RVNameAndState

	// When was this MV replica composition/state last updated and by which node.
	// An MV replica composition/state is updated by the following RPCs:
	// JoinMV    - this creates a new MV replica. It is called by the new-mv and the fix-mv workflows.
	//             A new-mv workflow causes an MV replica to start as "online" while a fix-mv workflow
	//             causes an MV replica to start as "outofsync".
	// UpdateMV  - this updates the composition of an MV replica.
	// StartSync - this changes the state of an MV replica added by fix-mv workflow, to "syncing", from
	//             "outofsync".
	// EndSync   - this changes the state of an MV replica added by fix-mv workflow, and synchronized by the
	//             sync-mv workflow, to "online", from "syncing".
	//
	// This can be used for logging and for timing out "incomplete transactions".
	//
	// Incomplete transactions could be due to the following:
	// Incomplete JoinMV is one for which the JoinMV was sent as a result of fix-mv workflow, but it
	// was not followed by StartSync. This can happen if the fix-mv couldn't complete due to one or more
	// RVs failing their JoinMV/UpdateMV calls.
	// Incomplete StartSync is one for which the StartSync was sent as a result of sync-mv workflow, but
	// the "syncing" state was not committed for some reason (either not all nodes responded to StartSync
	// with success or the sender died before it could commit the updated clustermap).
	//
	// Since there's a finite time before a node responds positively to an RPC and before the update can be
	// committed in the clustermap, we have to wait for some timeout period before we can consider the
	// transaction as incomplete and can revert it. See mvInfoTimeout.
	//
	// Thus, after a workflow like JoinMV/StartSync/EndSync updates the rvInfo state and till mvInfoTimeout,
	// we do not allow the rvInfo state to be changed, during that time RPCs that want to change the componentRVs
	// will fail with ErrorCode_InvalidRV and the caller will need to handle this appropriately. For a JoinMV, it
	// will skip this RV, for a StartSync it'll defer the sync for later.
	//
	// lmt - Last Modified Time
	// lmb - Last Modified By
	//
	lmt time.Time
	lmb string

	// Total amount of space used up inside the MV directory,
	// by all the chunks stored in it. Any RV that has to replace one of the existing component
	// RVs needs to have at least this much space.
	// JoinMV() requests this much space to be reserved in the new-to-be-inducted RV.
	// This is updated only by PutChunk(client) requests. For PutChunk(sync) requests it's not
	// updated for each request but once the sync completes, in the EndSync handler, we add
	// mvInfo.reservedSpace to totalChunkBytes thus accounting all the chunks that were copied
	// to this MV as a result of the sync operations. This means that totalChunkBytes will be
	// 0 for outofsync MV replicas while it can be non-zero for online and even syncing replicas.
	// syncing MV replicas will have non-zero totalChunkBytes only if there are client writes
	// during the sync. This will happen if an MV replica went offline during a file write and
	// a new one had to be picked.
	totalChunkBytes atomic.Int64

	// Amount of space reserved for this MV replica, on the hosting RV.
	// When a new mvInfo is created by JoinMV() this is set to the ReserveSpace parameter to JoinMV.
	// This is also added to rvInfo.reservedSpace to reserve space in the RV.
	// This is non-zero only for MV replicas which are added by the fix-mv workflow and not for MV replicas
	// added by new-mv workflow. Put another way, this will be non-zero only for MV replicas which are in
	// outofsync or syncing state. On an EndSync request, that converts syncing state to online state
	// mvInfo.reservedSpace is cleared and its value is added to mvInfo.totalChunkBytes, also this is
	// reduced from rvInfo.reservedSpace as this space is no longer reserved but rather actual space is now
	// used by chunks stored in the RV.
	// If an MV replica cannot complete resync, this must be reduced from rvInfo.reservedSpace.
	// This means while an MV replica is being sync'ed the space used on the RV may be overcompensated, this
	// is corrected once sync completes.
	reservedSpace atomic.Int64

	// Two MV states are interesting from an IO standpoint.
	// An online MV is the happy case where all RVs are online and sync'ed. In this state there won't be any
	// resync Writes, and client Writes if any will be replicated to all the RVs, each of them storing the chunks
	// in their respective mv folders. This is the normal case.
	// A syncing MV is interesting. In this case there are resync writes going and possibly client writes too.
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
	// If this is empty it means the MV Replica is currently not participating in any sync job.
	// If non empty, these are all the sync jobs that this MV Replica is currently participating in, either
	// as source or target of a sync job.
	// The information on each sync job is held inside the syncJob struct. Since an MV Replica can be the
	// source of multiple sync jobs but can be a target for only one sync job, if this contains more than
	// one sync jobs, all of them MUST be source sync jobs.
	//
	// Indexed by syncID and stores value of type *syncJob.
	// syncJobsCount is the count of sync jobs stored in syncJobs.
	syncJobs      sync.Map
	syncJobsCount atomic.Int64
}

// Time we wait after mvInfo.lmt before we can delete it from rvInfo.mvMap.
// This should be the time during which the caller is gathering RPC responses from other component RV and/or
// it's waiting for the change to be committed to clustermap. It should be a few msecs in most cases, but we
// play safe in case the communication from RVs is hampered due to n/w connectivity. Note that it's ok if we
// don't prune an mvMap entry for some time, that RV may not be able to host a new MV for some time, so the
// corresponding sync will get delayed, but if we remove some valid MV that's part of a legit state change
// transaction, it can cause real confusion.
var mvInfoTimeout time.Duration = 60 * time.Second

// Users performing transactional changes like JoinMV/UpdateMV/StartSync/EndSync, which send RPCs to one or
// more component RVs and on getting success responses from all, commit state change in clustermap, can assume
// state change caused by RPC to be valid only for this much time. After this, if the clustermap state change
// is not committed, mvInfo state change can be reverted by the server.
func GetMvInfoTimeout() time.Duration {
	//
	// Caller usually performs this check before clustermap commit, and clustermap commit may take some more
	// time. This is the margin to protect that.
	// So, we do not purge an mvInfo state change for mvInfoTimeout seconds while we want caller to not assume
	// it to be more than mvInfoTimeout-margin.
	//
	margin := 15 * time.Second
	return mvInfoTimeout - margin
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
func NewChunkServiceHandler(rvMap map[string]dcache.RawVolume) error {
	common.Assert(handler == nil, "NewChunkServiceHandler called more than once")

	handler = &ChunkServiceHandler{
		rvIDMap: getRvIDMap(rvMap),
	}

	// If no RVs are hosted by this node, we should not create the chunk service handler.
	common.Assert(len(handler.rvIDMap) > 0)

	//
	// For active MVs that are hosted by this node, we must correctly update rvInfo.mvMap.
	// See safeCleanupMyRVs()->cm.GetActiveMVsForRV() to see how we can have active MVs for an RV
	// when a node starts up.
	//
	for rvName, rv := range rvMap {
		rvInfo := handler.getRVInfoFromRVName(rvName)
		common.Assert(rvInfo != nil, rvName, handler.rvIDMap)

		entries, err := os.ReadDir(rv.LocalCachePath)
		if err != nil {
			common.Assert(false, err)
			return fmt.Errorf("NewChunkServiceHandler: os.ReadDir(%s) failed: %v", rv.LocalCachePath, err)
		}

		// Must not have more than getMVsPerRV() MVs in the cache dir.
		common.Assert(len(entries) <= int(getMVsPerRV()), rvName, len(entries), getMVsPerRV())

		//
		// Cache dir must contain only those MVs for which this RV is actively being used.
		// We need to add such MVs to rvInfo.mvMap as if the RV was joined to those MVs using
		// a JoinMV RPC call.
		//
		for _, entry := range entries {
			log.Debug("NewChunkServiceHandler: Got %s/%s", rv.LocalCachePath, entry.Name())

			if !entry.IsDir() {
				common.Assert(false, rv.LocalCachePath, entry.Name())
				return fmt.Errorf("NewChunkServiceHandler: %s/%s is not a directory %+v",
					rv.LocalCachePath, entry.Name(), entry)
			}

			mvName := entry.Name()
			if !cm.IsValidMVName(mvName) {
				common.Assert(false, rv.LocalCachePath, entry.Name())
				return fmt.Errorf("NewChunkServiceHandler: %s/%s is not a valid MV directory %+v",
					rv.LocalCachePath, entry.Name(), entry)
			}

			//
			// Component RVs for this MV, as per clustermap.
			//
			componentRVMap := cm.GetRVs(mvName)
			_, ok := componentRVMap[rvName]
			_ = ok

			// We should only have MV dirs for active MVs for the RV.
			common.Assert(ok, rvName, mvName, componentRVMap)

			componentRVs := cm.RVMapToList(mvName, componentRVMap)
			sortComponentRVs(componentRVs)

			//
			// If the component RVs list has any RV with inband-offline state, update it to offline.
			// This is done because we don't allow inband-offline state in the rvInfo.
			//
			updateInbandOfflineToOffline(&componentRVs)

			//
			// Acquire lock on rvInfo.rwMutex.
			// This is running from the single startup thread, so we don't really need the lock, but
			// addToMVMap() asserts for that.
			//
			mvDirSize, err := getMVDirSize(filepath.Join(rv.LocalCachePath, mvName))
			if err != nil {
				log.Err("NewChunkServiceHandler: %v", err)
			}

			mvInfo := newMVInfo(rvInfo, mvName, componentRVs, rpc.GetMyNodeUUID())
			mvInfo.incTotalChunkBytes(mvDirSize)

			rvInfo.acquireRvInfoLock()
			rvInfo.addToMVMap(mvName, mvInfo, 0 /* reservedSpace */)
			rvInfo.releaseRvInfoLock()
		}
	}

	return nil
}

// Create new mvInfo instance. This is used by the JoinMV() RPC call to create a new mvInfo.
func newMVInfo(rv *rvInfo, mvName string, componentRVs []*models.RVNameAndState, joinedBy string) *mvInfo {
	common.Assert(common.IsValidUUID(joinedBy), rv.rvName, mvName, joinedBy)
	common.Assert(!containsInbandOfflineState(&componentRVs), componentRVs)

	return &mvInfo{
		rv:           rv,
		mvName:       mvName,
		componentRVs: componentRVs,
		lmt:          time.Now(),
		lmb:          joinedBy,
	}
}

// Get the total chunk bytes for the MV path by summing up the size of all the chunks in the MV directory.
func getMVDirSize(mvPath string) (int64, error) {
	if !common.DirectoryExists(mvPath) {
		return 0, fmt.Errorf("getMVDirSize: %s does not exist", mvPath)
	}

	totalBytes := int64(0)
	chunksCount := int64(0)
	err := filepath.Walk(mvPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			err = fmt.Errorf("getMVDirSize: filepath.Walk(%s) failed: %v", path, err)
			return err
		}

		//
		// There won't be directories inside an MV directory, but filepath.Walk() will return
		// directories corresponding to "." and "..".
		//
		if info.IsDir() {
			log.Debug("getMVDirSize: skipping directory %s", path)
			return nil
		}

		// Only count chunks (not hashes).
		if !strings.HasSuffix(info.Name(), ".data") {
			return nil
		}

		chunksCount++
		totalBytes += info.Size()

		return nil
	})

	if err != nil {
		log.Err("getMVDirSize: failed for %s: %v", mvPath, err)
	} else {
		log.Debug("getMVDirSize: %s has %d chunks with total size %d bytes",
			mvPath, chunksCount, totalBytes)
	}

	return totalBytes, err
}

// This is a test trick to dummy out the reads/writes in order to test files larger than the RV available space.
// Dummy writes simply skip and no chunks are created, while dummy reads return 0s.
func performDummyReadWrite() bool {
	if !common.IsDebugBuild() {
		return false
	}

	// Test for the presence of the special marker file.
	dummyFile := "/tmp/DCACHE_DUMMY_RW"

	_, err := os.Stat(dummyFile)
	return err == nil
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
// There's no guarantee that after the function returns MV is still hosted on RV, unless caller ensures through
// some other means, but the returned mvInfo can be safely used. See deleteFromMVMap() how it can be deleted.
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
		common.Assert(rv.rvName == mvInfo.rv.rvName, rv.rvName, mvInfo.rv.rvName, mvName)
		// Technically mv can be deleted after the Load() above, so this assert may fail, but extremely unlikely.
		common.Assert(rv.mvCount.Load() > 0, rv.rvName, mvInfo.mvName, rv.mvCount.Load())
		common.Assert(common.IsValidUUID(mvInfo.lmb), rv.rvName, mvInfo.mvName, mvInfo.lmb,
			mvInfo.lmt)
		common.Assert(!mvInfo.lmt.IsZero(), rv.rvName, mvInfo.mvName, mvInfo.lmb,
			mvInfo.lmt)

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
			common.Assert(rv.rvName == mvInfo.rv.rvName, rv.rvName, mvInfo.rv.rvName, mvInfo.mvName)
			// Technically mv can be deleted after Range() returns it, so this assert may fail, but extremely unlikely.
			common.Assert(rv.mvCount.Load() > 0, rv.rvName, mvInfo.mvName, rv.mvCount.Load())
			common.Assert(common.IsValidUUID(mvInfo.lmb), rv.rvName, mvInfo.mvName, mvInfo.lmb,
				mvInfo.lmt)
			common.Assert(!mvInfo.lmt.IsZero(), rv.rvName, mvInfo.mvName, mvInfo.lmb,
				mvInfo.lmt)
		} else {
			common.Assert(false, fmt.Sprintf("mvMap[%s] has value which is not of type *mvInfo", mvName))
		}

		mvs = append(mvs, mvInfo.mvName)
		return true
	})

	return mvs
}

// Add a new MV replica to the given RV.
// Caller must ensure that the RV is not already hosting the MV replica and
// that the rvInfo.rwMutex lock is acquired.
func (rv *rvInfo) addToMVMap(mvName string, mv *mvInfo, reservedSpace int64) {
	common.Assert(rv.isRvInfoLocked(), rv.rvName, mvName, reservedSpace)

	mvPath := filepath.Join(rv.cacheDir, mvName)
	_ = mvPath
	common.Assert(common.DirectoryExists(mvPath), mvPath)
	common.Assert(mv.rv == rv, mv.rv.rvName, rv.rvName)

	//
	// Set reservedSpace for this MV and increment the reserved space for the hosting RV.
	// Note that the actual space reservation is done in rvInfo.reservedSpace, while mvInfo.reservedSpace
	// is used for undoing rvInfo.reservedSpace in case the sync does not complete.
	//
	common.Assert(mv.reservedSpace.Load() == 0, mv.reservedSpace.Load(), rv.rvName, mvName)
	mv.reservedSpace.Store(reservedSpace)
	rv.incReservedSpace(reservedSpace)

	rv.mvMap.Store(mvName, mv)
	rv.mvCount.Add(1)

	common.Assert(rv.mvCount.Load() <= getMVsPerRV(), rv.rvName, rv.mvCount.Load(), getMVsPerRV())
}

// Delete the MV replica from the given RV.
// Caller must ensure that the rvInfo.rwMutex lock is acquired.
func (rv *rvInfo) deleteFromMVMap(mvName string) {
	common.Assert(rv.isRvInfoLocked(), rv.rvName)

	_, ok := rv.mvMap.Load(mvName)
	if !ok {
		common.Assert(false, fmt.Sprintf("mvMap[%s] not found", mvName))
		return
	}

	rv.mvMap.Delete(mvName)
	rv.mvCount.Add(-1)

	common.Assert(rv.mvCount.Load() >= 0, fmt.Sprintf("mvCount for RV %s is negative", rv.rvName))
}

// Increment the reserved space for this RV.
func (rv *rvInfo) incReservedSpace(bytes int64) {
	common.Assert(bytes >= 0)
	rv.reservedSpace.Add(bytes)
	log.Debug("rvInfo::incReservedSpace: reserved space for RV %s is %d, mvCount: %d",
		rv.rvName, rv.reservedSpace.Load(), rv.mvCount.Load())
}

// Decrement the reserved space for this RV.
func (rv *rvInfo) decReservedSpace(bytes int64) {
	common.Assert(bytes >= 0, bytes)
	common.Assert(rv.reservedSpace.Load() >= bytes, rv.rvName, rv.reservedSpace.Load(), bytes, rv.mvCount.Load())
	rv.reservedSpace.Add(-bytes)
	log.Debug("rvInfo::decReservedSpace: reserved space for RV %s is %d, mvCount: %d",
		rv.rvName, rv.reservedSpace.Load(), rv.mvCount.Load())
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

// Return available space for our local RV.
// It queries the file system to get the available space in the cache directory for the RV and subtracts
// any space reserved for the RV by the JoinMV RPC call.
func GetAvailableSpaceForRV(rvId, rvName string) (int64, error) {
	//
	// Initial call(s) before RPC server is started must simply return the available space as reported
	// by the file system, else we must subtract the reserved space for the RV
	//
	if handler == nil {
		_, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(rvName)
		return int64(availableSpace), err
	}

	// rvId passed must refer to one of of our local RVs.
	rvInfo, ok := handler.rvIDMap[rvId]
	_ = ok
	common.Assert(ok && rvInfo != nil, rvId, handler.rvIDMap)

	return rvInfo.getAvailableSpace()
}

// Acquire lock on rvInfo.
// This is used to ensure that only one operation among JoinMVs or LeaveMVs for an RV is in progress at a time.
func (rv *rvInfo) acquireRvInfoLock() {
	rv.rwMutex.Lock()

	common.Assert(!rv.rwMutexDbgFlag.Load(), rv.rvName)
	rv.rwMutexDbgFlag.Store(true)
}

// Release lock on rvInfo.
func (rv *rvInfo) releaseRvInfoLock() {
	common.Assert(rv.rwMutexDbgFlag.Load(), rv.rvName)
	rv.rwMutexDbgFlag.Store(false)

	rv.rwMutex.Unlock()
}

// Check if lock is held on rvInfo.
// [DEBUG ONLY]
func (rv *rvInfo) isRvInfoLocked() bool {
	return rv.rwMutexDbgFlag.Load()
}

// Check if this MV replica is the source or target of any sync job.
func (mv *mvInfo) isSyncing() bool {
	return mv.syncJobsCount.Load() > 0
}

// Add a new sync job to the syncJobs map for this MV replica.
func (mv *mvInfo) addSyncJob(sourceRVName string, targetRVName string) string {
	// One and only one of sourceRVName and targetRVName can be valid.
	common.Assert(sourceRVName == "" || targetRVName == "", sourceRVName, targetRVName)
	common.Assert(cm.IsValidRVName(sourceRVName) || cm.IsValidRVName(targetRVName), sourceRVName, targetRVName)

	// Create a unique syncID for this sync job.
	syncID := gouuid.New().String()

	// Unlikely, but still do the check for correctness.
	_, ok := mv.syncJobs.Load(syncID)
	common.Assert(!ok, fmt.Sprintf("[BUG] %s already has syncJob with syncID %s: %+v",
		mv.mvName, syncID, mv.getSyncJobs()))
	_ = ok

	newSyncJob := syncJob{
		syncID:       syncID,
		sourceRVName: sourceRVName,
		targetRVName: targetRVName,
	}
	mv.syncJobs.Store(syncID, &newSyncJob)
	mv.syncJobsCount.Add(1)

	log.Debug("Added syncJob #%d, %s: %+v",
		mv.syncJobsCount.Load(), mv.syncJobToString(&newSyncJob), mv.getSyncJobs())

	return syncID
}

// Return the syncJob corresponding to syncID.
// Returns nil if no such syncJob exists.
func (mv *mvInfo) getSyncJob(syncID string) *syncJob {
	common.Assert(common.IsValidUUID(syncID))

	val, ok := mv.syncJobs.Load(syncID)
	if ok {
		return val.(*syncJob)
	}
	return nil
}

// Check if the syncID is valid for this MV replica, i.e., there is currently a syncJob running with this syncID.
func (mv *mvInfo) isSyncIDValid(syncID string) bool {
	common.Assert(common.IsValidUUID(syncID))

	return mv.getSyncJob(syncID) != nil
}

// Given a syncJob return a pretty print string for logging the syncJob.
func (mv *mvInfo) syncJobToString(syncJob *syncJob) string {
	//
	// If sourceRVName is set then our local RV is the target of this sync job, else it's the source
	// of this sync job.
	//
	if len(syncJob.sourceRVName) > 0 {
		common.Assert(len(syncJob.targetRVName) == 0, syncJob)

		return fmt.Sprintf("[%s/%s -> %s/%s {Local Replica: %s/%s, syncID: %s}]",
			syncJob.sourceRVName, mv.mvName, mv.rv.rvName, mv.mvName, mv.rv.rvName,
			mv.mvName, syncJob.syncID)
	} else {
		common.Assert(len(syncJob.targetRVName) > 0, syncJob)

		return fmt.Sprintf("[%s/%s -> %s/%s {Local Replica: %s/%s, syncID: %s}]",
			mv.rv.rvName, mv.mvName, syncJob.targetRVName, mv.mvName, mv.rv.rvName,
			mv.mvName, syncJob.syncID)
	}
}

// Return a list of string representation of all the sync jobs currently running where this MV replica is
// either the source or target of sync.
func (mv *mvInfo) getSyncJobs() []string {
	syncJobs := make([]string, 0)
	mv.syncJobs.Range(func(key, val interface{}) bool {
		syncJob := val.(*syncJob)
		syncID := key.(string)

		common.Assert(syncJob != nil, syncID)
		common.Assert(syncID == syncJob.syncID, syncID, syncJob.syncID)
		_ = syncID

		syncJobs = append(syncJobs, mv.syncJobToString(syncJob))
		return true
	})

	return syncJobs
}

// Delete sync job entry from the syncJobs map for this MV replica.
func (mv *mvInfo) deleteSyncJob(syncID string) {
	val, ok := mv.syncJobs.Load(syncID)
	common.Assert(ok, fmt.Sprintf("%s does not have syncJob with syncID %s: %+v",
		mv.mvName, syncID, mv.getSyncJobs()))
	_ = ok

	syncJob := val.(*syncJob)
	common.Assert(syncJob.syncID == syncID, syncJob.syncID, syncID, mv.mvName)
	_ = syncJob

	mv.syncJobs.Delete(syncID)

	log.Debug("Deleted syncJob #%d %s: %+v",
		mv.syncJobsCount.Load(), mv.syncJobToString(syncJob), mv.getSyncJobs())

	common.Assert(mv.syncJobsCount.Load() > 0, mv.syncJobsCount.Load())
	mv.syncJobsCount.Add(-1)
}

// Delete all sync jobs from the syncJobs map for this MV replica.
func (mv *mvInfo) deleteAllSyncJobs() {
	mv.syncJobs.Range(func(key, val interface{}) bool {
		mv.deleteSyncJob(key.(string))
		return true
	})

	common.Assert(mv.syncJobsCount.Load() == 0, mv.syncJobsCount.Load())
}

// Return if this MV replica is the source or target of a sync job.
// An MV replica can act as source for multiple simultaneous sync jobs (each of which would be resyncing one distinct
// MV replica for the MV) but can act as target for one and only one sync job.
// For MV replicas acting as source, the target MV replica will be outside this node and targetRVName contains the
// name of the RV on which the target MV replica resides, similarly for MV replicas acting as target, the source
// MV replica will be outside this node and sourceRVName contains the name of the RV on which the source MV replica
// resides.
// In both source and target MV replicas, both client and resync PutChunk chunks are written to the rv/mv folder.
//
// Caller must hold opMutex read lock.
func (mv *mvInfo) isSourceOrTargetOfSync() (isSource bool, isTarget bool) {
	common.Assert(mv.isSyncOpReadLocked(), mv.opMutexDbgCntr.Load())

	// No entry in syncJobs map means that the MV is not in syncing state.
	// This is the common case.
	if mv.syncJobsCount.Load() == 0 {
		return false, false /* MV replica is not syncing */
	}

	// If there are more than one entries in the syncJobs map, it means that this MV replica is the source of
	// all those sync jobs. Note that an MV replica can be target to one and only one sync job.
	if mv.syncJobsCount.Load() > 1 {
		return true, false /* MV replica is source for more than one sync jobs */
	}

	mv.syncJobs.Range(func(key, val interface{}) bool {
		syncJob := val.(*syncJob)

		common.Assert(syncJob.sourceRVName == "" || syncJob.targetRVName == "",
			fmt.Sprintf("Both source and target RV names cannot be set in a syncJob %+v", syncJob))
		common.Assert(cm.IsValidRVName(syncJob.sourceRVName) || cm.IsValidRVName(syncJob.targetRVName),
			fmt.Sprintf("One of source or target RV name must be set in a syncJob %+v", syncJob))

		// If sourceRVName is set that means this MV Replica is the target of this syncJob,
		// while if targetRVName is set it means this MV Replica is the source of this syncJob.
		if syncJob.sourceRVName != "" {
			/* MV replica is target for one syncJob */
			isSource = false
			isTarget = true
			return false
		} else {
			/* MV replica is source for one syncJob */
			isSource = true
			isTarget = false
			return false
		}
	})

	return isSource, isTarget
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
// Also note that since UpdateMV (like all their RPCs) is not transactional, sender will send multiple of these
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
func (mv *mvInfo) updateComponentRVs(componentRVs []*models.RVNameAndState, forceUpdate bool, senderNodeId string) error {
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

	// We cannot have inband offline state in the componentRVs.
	common.Assert(!containsInbandOfflineState(&componentRVs), componentRVs)

	// Valid membership changes, update the saved componentRVs.
	mv.componentRVs = componentRVs
	mv.lmt = time.Now()
	mv.lmb = senderNodeId

	common.Assert(common.IsValidUUID(mv.lmb), mv.lmb)

	return nil
}

// Update the state of the given component RV in this MV.
func (mv *mvInfo) updateComponentRVState(rvName string, oldState, newState dcache.StateEnum, senderNodeId string) {
	common.Assert(oldState != newState &&
		cm.IsValidComponentRVState(oldState) &&
		cm.IsValidComponentRVState(newState) &&
		oldState != dcache.StateInbandOffline &&
		newState != dcache.StateInbandOffline, rvName, oldState, newState)

	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			common.Assert(rv.State == string(oldState), rvName, rv.State, oldState)
			log.Debug("mvInfo::updateComponentRVState: %s/%s (%s -> %s) %s, changed by sender %s",
				rvName, mv.mvName, rv.State, newState, rpc.ComponentRVsToString(mv.componentRVs),
				senderNodeId)

			rv.State = string(newState)
			mv.lmt = time.Now()
			mv.lmb = senderNodeId
			common.Assert(common.IsValidUUID(mv.lmb), mv.lmb)
			return
		}
	}

	common.Assert(false, rpc.ComponentRVsToString(mv.componentRVs), rvName, newState)
}

// From the list of component RVs for this MV return RVNameAndState for the requested RV, if not found returns nil.
func (mv *mvInfo) getComponentRVNameAndState(rvName string) *models.RVNameAndState {
	common.Assert(cm.IsValidRVName(rvName), rvName, mv.mvName, mv.rv.rvName)
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)
		common.Assert(cm.IsValidComponentRVState(dcache.StateEnum(rv.State)), rv.Name, mv.mvName, rv.State)

		//
		// Only online and syncing local MV replicas can have non-zero totalChunkBytes.
		//
		common.Assert((mv.rv.rvName != rv.Name) || (mv.totalChunkBytes.Load() == 0) ||
			(rv.State == string(dcache.StateOnline) || rv.State == string(dcache.StateSyncing)),
			rv.Name, mv.mvName, rv.State, mv.totalChunkBytes.Load())

		if rv.Name == rvName {
			return rv
		}
	}

	return nil
}

// Refresh componentRVs for the MV, from the clustermap.
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
// 2. rvInfo has inconsistent info due to the previous partially applied change.
//
// So, whenever a request and mvInfo's component RV details don't match, the server needs to refresh its
// membership details from the clustermap and if there still is a mismatch indicating client using stale
// clustermap, fail the call with NeedToRefreshClusterMap asking the sender to refresh too. This function
// helps to refresh the rvInfo component RV details from the clustermap.
//
// Note that it returns a failure if the rvInfo state change corresponds to a valid ongoing transaction
// for which the clustermap is not yet updated and hence we cannot revert the rvInfo change to match the
// clustermap.
//
// Return values:
// - nil on success, in which case the mvInfo componentRVs are updated to match the clustermap.
// - models.ErrorCode_InvalidRV if rvInfo cannot be refreshed from the clustermap due to an ongoing transaction.
// - models.ErrorCode_NeedToRefreshClusterMap on any failure, in which case the mvInfo componentRVs are not
//   updated. We return this error anyways so that the client can refresh its clustermap and retry the RPC.
//   This provides resilience against any temporary error in reading clustermap as we will again get to perform
//   this check when client retries.

func (mv *mvInfo) refreshFromClustermap() *models.ResponseError {
	log.Debug("mvInfo::refreshFromClustermap: %s/%s", mv.rv.rvName, mv.mvName)

	//
	// Refresh the clustermap synchronously. Once this returns, clustermap package has the updated
	// clustermap.
	//
	err := cm.RefreshClusterMap(0 /* higherThanEpoch */)
	if err != nil {
		errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: %s/%s, failed: %v", mv.rv.rvName, mv.mvName, err)
		log.Err("%s", errStr)
		common.Assert(false, errStr)
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	// Get component RV details from the just refreshed clustermap.
	newRVs := cm.GetRVs(mv.mvName)
	if newRVs == nil {
		errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: GetRVs(%s) failed", mv.mvName)
		log.Err("%s", errStr)
		common.Assert(false, errStr)
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	//
	// Must have the hosting RV in the componentRVs list.
	//
	myRvInfo := mv.getComponentRVNameAndState(mv.rv.rvName)
	common.Assert(myRvInfo != nil, mv.rv.rvName, mv.mvName, rpc.ComponentRVsToString(mv.componentRVs))

	//
	// Convert newRVs from RV Name->State map, to RVNameAndState slice.
	// Later we will use this to update the mvInfo componentRVs.
	//
	// Note: We do it before checking the RV states, so that we can correctly update the component RV
	//       state to offline if the RV is offline in the clustermap.
	//
	var newComponentRVs []*models.RVNameAndState
	for rvName, rvState := range newRVs {
		common.Assert(cm.IsValidComponentRVState(rvState), rvName, mv.mvName, rvState, mv.rv.rvName)

		//
		// degrade-mv workflow marks component RVs as offline, for the RVs which are marked offline,
		// but it doesn't commit the changes to clustermap before it runs the fix-mv workflow, the
		// following achieves that w/o a clustermap update being forced after degrade-mv.
		// Note that this is one of those "safe deductions" that we can do while taking the risk of
		// deviating away from the actual clustermap, but note that clustermap will soon be updated
		// to reflect it.
		// If the state of the RV is inband-offline, we treat it as offline.
		//
		if (cm.GetRVState(rvName) == dcache.StateOffline && rvState != dcache.StateOffline) ||
			rvState == dcache.StateInbandOffline {
			log.Warn("mvInfo::refreshFromClustermap: %s/%s state is %s while RV state is offline, marking component RV state as offline",
				rvName, mv.mvName, rvState)
			rvState = dcache.StateOffline
			newRVs[rvName] = rvState
		}

		newComponentRVs = append(newComponentRVs, &models.RVNameAndState{
			Name:  rvName,
			State: string(rvState),
		})
	}

	//
	// We should refresh our incore rvInfo/mvInfo details from the clustermap but only if it doesn't amount
	// to reverting a legitimate change that was done by a very recent RPC and for which the corresponding
	// clustermap update might be in progress. Note that we are responsible for our local RV (mv.rv) and we
	// MUST NOT allow any illegal change to that component RV state. For other component RVs we simply accept
	// the change suggested by the clustermap.
	//
	// What we want to do?
	// - Clear/revert stale/stuck mvInfo/rvInfo due to incomplete state change transaction where the incore
	//   mvInfo/rvInfo was changed but the change couldn't be persisted in the clustermap.
	//
	// What we want to avoid?
	// - Clear/revert a legitimate ongoing mvInfo/rvInfo change in case some other node makes an invalid RPC
	//   call as the change was still not persisted in the clustermap.
	//
	// Why can't we unconditionally refresh the mvInfo/rvInfo state from the latest clustermap?
	// - Note that our state changes are not strictly transactional, we provide a semblance of transaction
	//   by sender sending the state change RPC (JoinMV/UpdateMV/StartSync/EndSync) to all involved RVs and
	//   only when all of them respond successfully, it commits the change in the clustermap. If any of the
	//   RV fails the RPC the sender doesn't commit the state change but it doesn't send undo RPCs, so the
	//   RVs which responded positively have invalid state not matching the clustermap in rvInfo/mvInfo.
	//   Any future RPC will find that the sender's state (which it got from clustermap) doesn't match the
	//   rvInfo state, this will trigger a clustermap refresh at the server as well as sender, causing update
	//   of the RV state from the clustermap (our rollback mechanism). This has one issue though, we cannot
	//   let the state be reverted till some reasonable timeout period since the sender will take some time
	//   to commit the state change in the clustermap. This is to not revert a legitimate ongoing state change.
	//   Timeout must be large enough to safely consider the state difference between rvInfo and clustermap
	//   as being due to incomplete state change workflow (JoinMV/StartSync etc) and not a transient state
	//   of an ongoing transaction.
	//
	clusterMapWantsToChangeMyRV := false

	stateAsPerClustermap, isPresentInClusterMap := newRVs[mv.rv.rvName]
	if !isPresentInClusterMap || string(stateAsPerClustermap) != myRvInfo.State {
		//
		// My RV is being removed from the component RV list for this MV, or
		// my RV state is being changed.
		//
		clusterMapWantsToChangeMyRV = true

		if !isPresentInClusterMap {
			//
			// clustermap doesn't have my RV, indicate that by StateInvalid.
			// A likely case is if an offline RV is replaced by a new RV by the fix-mv workflow,
			// the first JoinMV RPC will cause rvInfo for the new RV to be set to StateOutOfSync.
			// Before the clustermap is updated with this new RV, if some node also runs the fix-mv
			// workflow with the same new RV, it'll be a case of double join and to us (the hosting RV)
			// it'll appear as if the new RV is being removed from the clustermap's component RVs list.
			//
			stateAsPerClustermap = dcache.StateInvalid
		}
	}

	if clusterMapWantsToChangeMyRV {
		//
		// Don't allow rvInfo/mvInfo changes corresponding to ongoing updates to be reverted.
		//
		if time.Since(mv.lmt) < mvInfoTimeout {
			errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: %s/%s ongoing state change (clustermap:%s -> rvInfo:%s), not timed out yet (%s < %s)",
				mv.rv.rvName, mv.mvName, stateAsPerClustermap, myRvInfo.State, time.Since(mv.lmt), mvInfoTimeout)
			log.Err("%s", errStr)
			return rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
		}

		//
		// OK, it's not the case of an ongoing state change, so we can safely revert the rvInfo/mvInfo.
		// Look for various valid rollback scenarios and revert the rvInfo/mvInfo along with anything
		// else needed.
		//
		// Rollback from:
		// StateOutOfSync -> StateOffline, or
		// StateOutOfSync -> not present in clustermap.
		//
		// An outofsync component RV is marked by a JoinMV RPC call sent as a result of the fix-mv workflow.
		// It marks reservedSpace in the mvInfo and rvInfo to reserve the space needed for sync'ing the MV
		// replica. Normally this reserved space would be deducted from rvInfo.reservedSpace as part of
		// the EndSync processing, after the sync has copied data to the new MV replica. At this point
		// mvInfo.totalChunkBytes will be increased by mvInfo.reservedSpace, and rvInfo.reservedSpace will
		// be reduced by mvInfo.reservedSpace and mvInfo.reservedSpace will be set to 0.
		//
		// Hence if our in-core mvInfo has the state of a component RV as StateOutOfSync while clustermap either
		// - has the same RV with state StateOffline, or,
		// - doesn't have that component RV present,
		// it means the fix-mv workflow didn't complete successfully so we need to rollback the reserved space
		// changes.
		// Note that the first one represents the case where same RV was used as the replacement RV as it came
		// back online, while the second one is the more common case of a different RV picked as the replacement
		// RV.
		//
		if myRvInfo.State == string(dcache.StateOutOfSync) {
			//
			// Since all state transitions of an RV must be approved by the RV before they are committed
			// to clustermap, there can only be the following valid transitions for an RV.
			//
			common.Assert(!isPresentInClusterMap || stateAsPerClustermap == dcache.StateOffline,
				mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap, isPresentInClusterMap)

			log.Warn("mvInfo::refreshFromClustermap: Rolling back %s/%s (%s -> %s (present: %v)), clearing reservedSpace (%d bytes) left from previous incomplete join attempt",
				mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap,
				isPresentInClusterMap, mv.reservedSpace.Load())

			mv.rv.decReservedSpace(mv.reservedSpace.Load())
			mv.reservedSpace.Store(0)
		}

		//
		// Rollback from:
		// StateSyncing -> StateOutOfSync
		//
		// Consider the following case:
		// client is running the sync-mv workflow and decides to sync rv0/mv0 -> rv2/mv0
		// it'll send a StartSync request and the mvInfo.syncJobs will have a new sync job added.
		// If all went well, the client would send EndSync on completion of the sync job, which will
		// remove the sync job from mvInfo.syncJobs, but let's say sync didn't proceed normally and
		// was aborted. Next time when the mv was again picked for sync'ing, this time rv0 went offline
		// and hence rv1 was picked as the source replica, so now the client sends a fresh StartSync
		// request for rv1/mv0 -> rv2/mv0. This will find the rvInfo in StateSyncing which it doesn't
		// expect so refreshFromClustermap() is called. If we do not remove the older syncJob from
		// mvInfo.syncJobs, we will have multiple syncJobs queued for a target RV. Note that we consider
		// an mvInfo with more than one syncJobs as being a source replica (ref mvInfo.isSourceOrTargetOfSync).
		// So we wrongly treat it as a being a source replica.
		// Hence whenever we have refreshFromClustermap() see a state transition from StateSyncing to
		// StateOutOfSync, it means that it's this case and we must clear the old syncJob.
		//
		// Note: We also must consider "offline" RVs, as an "outofsync" RV in clustermap can also go offline.
		//
		if myRvInfo.State == string(dcache.StateSyncing) {
			//
			// Since all state transitions of an RV must be approved by the RV before they are committed
			// to clustermap, there can only be the following valid transitions for an RV.
			//
			common.Assert(isPresentInClusterMap &&
				(stateAsPerClustermap == dcache.StateOutOfSync ||
					stateAsPerClustermap == dcache.StateOffline),
				mv.rv.rvName, mv.mvName, stateAsPerClustermap, isPresentInClusterMap)
			//
			// Only a target replica can be in StateSyncing and a target replica MUST have one and
			// only one syncJob, clear that.
			//
			common.Assert(mv.syncJobsCount.Load() == 1, mv.rv.rvName, mv.mvName, mv.syncJobsCount.Load(),
				mv.getSyncJobs(), rpc.ComponentRVsToString(mv.componentRVs))

			log.Warn("mvInfo::refreshFromClustermap: Rolling back %s/%s (%s -> %s), clearing old syncJob left from previous incomplete sync attempt, syncJobs: %+v",
				mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap, mv.getSyncJobs())

			mv.deleteAllSyncJobs()
		}

		//
		// TODO: If an RV is being added in "outofsync" or "syncing" state (and it was in a different
		//       state earlier) we must also update rvInfo.reservedSpace.
		//
	}

	//
	// Update unconditionally, even if it may not have changed, doesn't matter.
	// We force the update as this is the membership info that we got from clustermap.
	//
	mv.updateComponentRVs(newComponentRVs, true /* forceUpdate */, rpc.GetMyNodeUUID())

	return nil
}

// Refresh clustermap and remove any stale MV entries from rvInfo.mvMap.
// This is used for deleting any MVs by a prior JoinMV call which were never committed by the sender in the
// clustermap. Note that these could be MVs added by new-mv or fix-mv workflows.
// Caller must ensure that the rvInfo.rwMutex lock is acquired.
func (rv *rvInfo) pruneStaleEntriesFromMvMap() error {
	common.Assert(rv.isRvInfoLocked(), rv.rvName)

	log.Debug("mvInfo::pruneStaleEntriesFromMvMap: %s hosts %d MVs", rv.rvName, rv.mvCount.Load())

	//
	// Refresh the clustermap synchronously.
	//
	err := cm.RefreshClusterMap(0 /* higherThanEpoch */)
	if err != nil {
		err := fmt.Errorf("mvInfo::pruneStaleEntriesFromMvMap: %s failed: %v", rv.rvName, err)
		log.Err("%v", err)
		common.Assert(false, err)
		return err
	}

	//
	// Go over all the MVs hosted on this RV as per our rvInfo, and for each of these MVs check clustermap
	// to see if this RV is indeed a valid component RV for the MV. If not, this is a stale mvMap entry and
	// we must remove it.
	//
	mvs := rv.getMVs()

	// Caller will call us only when it wants to prune mvMap, which means it must have entries.
	common.Assert(len(mvs) > 0, rv.rvName)

	for _, mvName := range mvs {
		mv := rv.getMVInfo(mvName)

		//
		// Skip MVs which were added not earlier than mvInfoTimeout.
		// These might be just added by a JoinMV RPC and sender might still be waiting JoinMV responses
		// from all RVs and/or might be in the process of committing the changes to clustermap.
		//
		if time.Since(mv.lmt) < mvInfoTimeout {
			log.Debug("mvInfo::pruneStaleEntriesFromMvMap: %s/%s (%d MVs), time since lmt (%s) < %s, skipping...",
				rv.rvName, mvName, rv.mvCount.Load(), time.Since(mv.lmt), mvInfoTimeout)
			continue
		}

		// Get component RV details for this MV from the just refreshed clustermap.
		rvs := cm.GetRVs(mvName)
		if rvs == nil {
			err := fmt.Errorf("mvInfo::pruneStaleEntriesFromMvMap: GetRVs(%s) failed", mvName)
			log.Err("%v", err)
			//
			// This may be a JoinMV call made by the new-mv workflow.
			// The MV is still not in clustermap but rvInfo has it, ignore it.
			//
			continue
		}

		//
		// Is this RV a valid component RV for this MV as per the clustermap?
		//
		rvState, ok := rvs[rv.rvName]
		if !ok {
			_ = rvState
			log.Debug("mvInfo::pruneStaleEntriesFromMvMap: deleting stale replica %s/%s (state: %s)",
				rv.rvName, mvName, rvState)
			// Remove the stale MV replica.
			rv.deleteFromMVMap(mvName)
		}
	}

	log.Debug("mvInfo::pruneStaleEntriesFromMvMap: after pruning %s now hosts %d MVs", rv.rvName, rv.mvCount.Load())
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
	common.Assert(!containsInbandOfflineState(&componentRVsInReq), componentRVsInReq)

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
				rpcErr := mv.refreshFromClustermap()
				if rpcErr != nil {
					errStr := fmt.Sprintf("Request component RVs are invalid for MV %s [%v]",
						mv.mvName, rpcErr.String())
					log.Err("ChunkServiceHandler::isComponentRVsValid: %s", errStr)
					return rpc.NewResponseError(rpcErr.Code, errStr)
				}
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
//       it's important for server (which always has the latest cluster membership info) to let client know if
//       its clustermap copy is stale and it needs to refresh it.

func (mv *mvInfo) validateComponentRVsInSync(componentRVsInReq []*models.RVNameAndState,
	sourceRVName string, targetRVName string, isStartSync bool) error {
	common.Assert(cm.IsValidRVName(sourceRVName) &&
		cm.IsValidRVName(targetRVName) &&
		sourceRVName != targetRVName, sourceRVName, targetRVName)
	common.Assert(!containsInbandOfflineState(&componentRVsInReq), componentRVsInReq)

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
		// A: If we are hosting the source or target RV, then this validState change must have been approved by
		//    us (prior JoinMV or StartSync) and only after that the sender could have committed the state
		//    change in clustermap. If we do not have the validState in our rvInfo then it cannot be in the
		//    clustermap and if it's not in the clustermap sender won't have sent the StartSync/EndSync RPC.
		//    Note that even if we are not hosting the target RV, we would have been informed through a
		//    StartSync request and we must have acknowledged it.
		//
		//    There is one possibility though. A prior StartSync succeeded and the mvInfo state was changed to
		//    syncing, but the sender couldn' persist that change in the clustermap (some node that was updating
		//    the clustermap took really long, due to some other node being down and JoinMV taking long time).
		//    Meanwhile the lowest online RV on the node attempting the sync is marked offline in clustermap,
		//    so some other node now has the lowest online RV, and that node now attempts the sync. It sends a
		//    StartSync RPC to this RV which is already marked syncing by the previous StartSync. In clustermap
		//    it's outofsync so a refresh will get the desired state.
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
				rpcErr := mv.refreshFromClustermap()
				if rpcErr != nil {
					log.Err("ChunkServiceHandler::validateComponentRVsInSync: Failed to refresh clustermap [%s]",
						rpcErr.String())
					return rpcErr
				}
				clustermapRefreshed = true
				continue
			}

			//
			// Offline is one state which is outside our control so a component RV can go to offline state
			// at any point, w/o we knowing about it. This will happen when the node hosting the component
			// RV goes offline.
			//
			common.Assert(targetRVNameAndState.State == string(dcache.StateOffline),
				targetRVNameAndState.State, validState, sourceRVName, targetRVName, mv.mvName, errStr)

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
	//
	// OffsetInMiB will be -1 when address identifies not one chunk but all chunks belonging to a file.
	// This is used by RemoveChunk to remove all chunks for a file from the given mv.
	//
	common.Assert(address.OffsetInMiB >= -1, address.OffsetInMiB)

	// rvID must refer to one of of our local RVs.
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
		_ = rvID
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

	// Sender and receiver node IDs must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)
	common.Assert(common.IsValidUUID(req.ReceiverNodeID), req.ReceiverNodeID)

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

// Helper function to read given chunk and (optionally) the hash file.
// It performs direct or buffered read as per the configured setting or may fallback to buffered read for
// cases where direct read cannot be performed due to alignment restrictions.
func readChunkAndHash(chunkPath, hashPath *string, readOffset int64, data *[]byte) (int /* read bytes */, string /* hash */, error) {
	var fh *os.File
	var n, fd int
	var err error
	var hash string

	common.Assert(chunkPath != nil && len(*chunkPath) > 0)
	common.Assert(data != nil && len(*data) > 0)
	common.Assert(readOffset >= 0)

	readLength := len(*data)

	//
	// Caller must pass data buffer aligned on FS_BLOCK_SIZE, else we have to unnecessarily perform buffered read.
	//
	dataAddr := unsafe.Pointer(&(*data)[0])
	isDataBufferAligned := ((uintptr(dataAddr) % common.FS_BLOCK_SIZE) == 0)
	common.Assert(isDataBufferAligned, uintptr(dataAddr), common.FS_BLOCK_SIZE)

	//
	// Hash file is small, perform buffered read.
	//
	if hashPath != nil {
		// Caller must ask hash only for full chunk reads.
		common.Assert(readOffset == 0)
		common.Assert(len(*hashPath) > 0)
		hashData, err := os.ReadFile(*hashPath)
		if err != nil {
			return -1, "", fmt.Errorf("failed to read hash file %s [%v]", *hashPath, err)
		}
		//
		// Just a sanity check.
		// TODO: Make it accurate once we decide on the hash algo
		//
		common.Assert(len(hashData) >= 16)
		hash = string(hashData)
	}

	//
	// Read the chunk using buffered IO mode if,
	//   - Read IO type is configured as BufferedIO, or
	//   - The requested offset and length is not aligned to file system block size.
	//   - The buffer is not aligned to file system block size.
	//
	if rpc.ReadIOMode == rpc.BufferedIO ||
		readLength%common.FS_BLOCK_SIZE != 0 ||
		readOffset%common.FS_BLOCK_SIZE != 0 ||
		!isDataBufferAligned {
		goto bufferedRead
	}

	//
	// Direct IO read.
	//
	fd, err = syscall.Open(*chunkPath, syscall.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return -1, "", fmt.Errorf("failed to open chunk file %s [%v]", *chunkPath, err)
	}
	defer syscall.Close(fd)

	if readOffset != 0 {
		_, err = syscall.Seek(fd, readOffset, 0)
		if err != nil {
			return -1, "", fmt.Errorf("failed to seek in chunk file %s at offset %d [%v]",
				*chunkPath, readOffset, err)
		}
	}

	n, err = syscall.Read(fd, *data)
	if err == nil {
		//
		// Partial reads should be rare, if it happens fallback to the buffered ReadAt() call which will
		// try to read all the requested byted.
		// TODO: Make sure this is not common path.
		//
		if n != readLength {
			common.Assert(false, n, readLength, *chunkPath)
			goto bufferedRead
		}
		return n, hash, nil
	}

	// For EINVAL, fall through to buffered read.
	if !errors.Is(err, syscall.EINVAL) {
		return -1, "", fmt.Errorf("failed to read chunk file %s offset %d [%v]", *chunkPath, readOffset, err)
	}

	// TODO: Remove this once this is tested sufficiently.
	log.Warn("Direct read failed with EINVAL, performing buffered read, file: %s, offset: %d, err: %v",
		*chunkPath, readOffset, err)

bufferedRead:
	fh, err = os.Open(*chunkPath)
	if err != nil {
		return -1, "", fmt.Errorf("failed to open chunk file %s [%v]", *chunkPath, err)
	}
	defer fh.Close()

	n, err = fh.ReadAt(*data, readOffset)
	if err != nil {
		return -1, "", fmt.Errorf("failed to read chunk file %s at offset %d [%v]", *chunkPath, readOffset, err)
	}

	common.Assert(n == readLength, n, readLength, *chunkPath)

	return n, hash, nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Address != nil)

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request (%v): %v", rpc.ReadIOMode, rpc.GetChunkRequestToString(req))

	// Sender node id must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)

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
	_ = hashPath
	log.Debug("ChunkServiceHandler::GetChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	//
	// Allocate byte slice, data from the chunk file will be read into this.
	//
	// TODO: Need to ensure this is FS_BLOCK_SIZE aligned.
	//
	data := make([]byte, req.Length)

	var lmt string
	var n int
	_ = n
	var chunkSize int64
	_ = chunkSize
	var stat syscall.Stat_t
	var hashPathPtr *string

	if performDummyReadWrite() {
		goto dummy_read
	}

	err = syscall.Stat(chunkPath, &stat)
	if err != nil {
		errStr := fmt.Sprintf("Failed to stat chunk file %s [%v]", chunkPath, err)
		log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, errStr)
	}

	chunkSize = stat.Size
	lmt = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec).UTC().String()

	common.Assert(req.OffsetInChunk+req.Length <= chunkSize,
		"Read beyond eof", req.OffsetInChunk, req.Length, chunkSize)

	//
	// TODO: hash validation will be done later
	// Only read hash if read is requested for entire chunk.
	//
	//if req.OffsetInChunk == 0 && req.Length == chunkSize {
	//	hashPathPtr := &hashPath
	//}
	n, _, err = readChunkAndHash(&chunkPath, hashPathPtr, req.OffsetInChunk, &data)
	if err != nil {
		errStr := fmt.Sprintf("failed to read chunk file %s [%v]", chunkPath, err)
		log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	common.Assert(n == len(data),
		fmt.Sprintf("bytes read %d is less than expected buffer size %d", n, len(data)))

dummy_read:
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

// Helper function to write given chunk and (optionally) the hash file.
// It performs direct or buffered write as per the configured setting or may fallback to buffered write for
// cases where direct write cannot be performed due to alignment restrictions.
func writeChunkAndHash(chunkPath, hashPath *string, data *[]byte, hash *string) error {
	var n, fd int
	var err error

	common.Assert(chunkPath != nil && len(*chunkPath) > 0)
	common.Assert(data != nil)
	common.Assert(len(*data) > 0)
	common.Assert(hashPath == nil || (len(*hashPath) > 0 && hash != nil && len(*hash) > 0), hashPath, hash)

	writeLength := len(*data)

	//
	// Caller must pass data buffer aligned on FS_BLOCK_SIZE, else we have to unnecessarily perform buffered write.
	//
	dataAddr := unsafe.Pointer(&(*data)[0])
	isDataBufferAligned := ((uintptr(dataAddr) % common.FS_BLOCK_SIZE) == 0)
	common.Assert(isDataBufferAligned, uintptr(dataAddr), common.FS_BLOCK_SIZE)

	//
	// Write to .tmp file first and rename it to the final file after successful write.
	// TODO: Get rid of an extra rename() call for every chunk write.
	//
	tmpChunkPath := fmt.Sprintf("%s.tmp", *chunkPath)

	//
	// Write the chunk using buffered IO mode if,
	//   - Write IO type is configured as BufferedIO, or
	//   - The write length (or chunk size) is not aligned to file system block size.
	//   - The buffer is not aligned to file system block size.
	//
	if rpc.WriteIOMode == rpc.BufferedIO ||
		writeLength%common.FS_BLOCK_SIZE != 0 ||
		!isDataBufferAligned {
		goto bufferedWrite
	}

	//
	// Direct IO write.
	//
	fd, err = syscall.Open(tmpChunkPath,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC|syscall.O_DIRECT, 0400)
	if err != nil {
		return fmt.Errorf("failed to open chunk file %s [%v]", tmpChunkPath, err)
	}
	defer syscall.Close(fd)

	n, err = syscall.Write(fd, *data)
	if err == nil {
		if n != len(*data) {
			return fmt.Errorf("partial write to chunk file %s (%d of %d) [%v]",
				tmpChunkPath, n, len(*data), err)
		}
		goto renameChunkFile
	}

	// For EINVAL, fall through to buffered write.
	if !errors.Is(err, syscall.EINVAL) {
		return fmt.Errorf("failed to write chunk file %s [%v]", tmpChunkPath, err)
	}

bufferedWrite:
	err = os.WriteFile(tmpChunkPath, *data, 0400)
	if err != nil {
		return fmt.Errorf("failed to write chunk file %s [%v]", tmpChunkPath, err)
	}

renameChunkFile:
	// Rename the .tmp file to the final file.
	err = os.Rename(tmpChunkPath, *chunkPath)
	if err != nil {
		return fmt.Errorf("failed to rename chunk file %s -> %s [%v]",
			tmpChunkPath, *chunkPath, err)
	}

	//
	// Write hash file after successful chunk file write.
	// Hash file is small, perform buffered write.
	//
	if hashPath != nil {
		err = os.WriteFile(*hashPath, []byte(*hash), 0400)
		if err != nil {
			return fmt.Errorf("failed to write hash file %s [%v]", *hashPath, err)
		}
	}

	return nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)
	common.Assert(req.Length == int64(len(req.Chunk.Data)),
		req.Length, len(req.Chunk.Data))

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request (%v): %v",
		rpc.WriteIOMode, rpc.PutChunkRequestToString(req))

	// Sender node id must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)

	// Check if the chunk address is valid.
	err := h.checkValidChunkAddress(req.Chunk.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Invalid chunk address %v, request = %v [%v]",
			req.Chunk.Address.String(), rpc.PutChunkRequestToString(req), err)
		return nil, err
	}

	rvInfo := h.rvIDMap[req.Chunk.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Chunk.Address.MvName)

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

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
	// we are accessing it.
	//
	mvInfo.acquireSyncOpReadLock()
	defer mvInfo.releaseSyncOpReadLock()

	clustermapRefreshed := false

refreshFromClustermapAndRetry:
	componentRVsInMV := mvInfo.getComponentRVs()
	_ = componentRVsInMV

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

			// If RV info has the RV as offline or outofsync, it'll be properly sync'ed later.
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
					rpcErr := mvInfo.refreshFromClustermap()
					if rpcErr != nil {
						log.Err("ChunkServiceHandler::PutChunk: Failed to refresh clustermap [%s]",
							rpcErr.String())
						return nil, rpcErr
					}
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
		syncJob := mvInfo.getSyncJob(req.SyncID)
		if syncJob == nil {
			errStr := fmt.Sprintf("PutChunk(sync) syncID %s not valid for %s/%s [NeedToRefreshClusterMap]",
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
	_, isTgtOfSync := mvInfo.isSourceOrTargetOfSync()

	var chunkPath, hashPath string
	_ = hashPath
	if len(req.SyncID) > 0 {
		//
		// Sync PutChunk call (as opposed to a client write PutChunk call).
		// This is called after the StartSync RPC to synchronize an OutOfSyc MV replica from a healthy MV
		// replica.
		//
		// Sync PutChunk call will be made in the ResyncMV() workflow, and should only be sent to RVs which
		// are target of a sync job.
		//
		if !isTgtOfSync {
			errStr := fmt.Sprintf("PutChunk(sync) syncID = %s, call received for %s/%s, which is currently not the target of any sync job",
				req.SyncID, rvInfo.rvName, req.Chunk.Address.MvName)

			log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
		}
	}

	//
	// In both client as well as sync write PutChunk calls,
	// the chunks must be written to the mv directory, i.e. rv0/mv0.
	//
	chunkPath, hashPath = getChunkAndHashPath(cacheDir, req.Chunk.Address.MvName,
		req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)

	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	var availableSpace int64

	// Chunk file must not be present.
	_, err = os.Stat(chunkPath)
	if err == nil {
		if req.SyncID != "" || req.MaybeOverwrite {
			if req.SyncID != "" {
				//
				// In case of sync PutChunk calls, we can get sync write for chunks already present in
				// the target RV because of the NTPClockSkewMargin added to the sync write time. These
				// chunks were written by the client write PutChunk calls to the target RV.
				// So, ignore this and return success.
				//
				log.Debug("ChunkServiceHandler::PutChunk: syncID = %s, chunk file %s already exists, ignoring sync write",
					req.SyncID, chunkPath)
				common.Assert(!req.MaybeOverwrite,
					"Only PutChunk(client) can have MaybeOverwrite set", rpc.PutChunkRequestToString(req))
			} else {
				//
				// Client can set the "MaybeOverwrite" flag to true in PutChunkRequest to let the server
				// know that this could potentially be an overwrite of a chunk that we previously wrote,
				// due to client retrying the WriteMV workflow after refreshing the clustermap.
				//
				log.Debug("ChunkServiceHandler::PutChunk: MaybeOverwrite = true, chunk file %s already exists, ignoring write",
					chunkPath)
			}

			availableSpace, err = rvInfo.getAvailableSpace()
			if err != nil {
				log.Err("ChunkServiceHandler::PutChunk: syncID = %s, Failed to get available disk space [%v]",
					req.SyncID, err)
			}

			return &models.PutChunkResponse{
				TimeTaken:      time.Since(startTime).Microseconds(),
				AvailableSpace: availableSpace,
				ComponentRV:    mvInfo.getComponentRVs(),
			}, nil

		} else {
			errStr := fmt.Sprintf("Chunk file %s already exists", chunkPath)
			log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_ChunkAlreadyExists, errStr)
		}
	}

	if performDummyReadWrite() {
		goto dummy_write
	}

	// TODO: hash validation will be done later
	err = writeChunkAndHash(&chunkPath, nil /* &hashPath */, &req.Chunk.Data, &req.Chunk.Hash)
	if err != nil {
		errStr := fmt.Sprintf("failed to write chunk file %s [%v]", chunkPath, err)
		log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	// TODO: see if this is needed -> store max mtime of chunk written by client write.
	// Needed for start sync time optimization in the sync component RV workflow. ON basis of this time,
	// we can decide chunks which need to be synced to the target RV. The chunks having mtime less than this,
	// should be synced to the target RV.

	// TODO: should we also consider the hash file size in the total chunk bytes
	//       For accurate accounting we can, but we should not do an extra stat() call for the hash file
	//       but instead use a hardcoded value which will be true for a given hash algo.
	//       Also we need to be sure that hash is calculated uniformly (either always or never)

	//
	// Increment the total chunk bytes for this MV for PutChunk(client) calls.
	// For PutChunk(sync) calls, the MV's totalChunkBytes will be updated in the EndSync call,
	// once the sync completes.
	//
	if len(req.SyncID) == 0 {
		mvInfo.incTotalChunkBytes(req.Length)
	} else {
		// JoinMV would have reserved this space before starting sync.
		common.Assert(rvInfo.reservedSpace.Load() >= req.Length, rvInfo.reservedSpace.Load(), req.Length)
		common.Assert(rvInfo.reservedSpace.Load() >= mvInfo.reservedSpace.Load(),
			rvInfo.reservedSpace.Load(), mvInfo.reservedSpace.Load())
	}

dummy_write:
	availableSpace, err = rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err)
	}

	resp := &models.PutChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		AvailableSpace: availableSpace,
		ComponentRV:    mvInfo.getComponentRVs(),
	}

	return resp, nil
}

// PutChunkDC RPC processes a PutChunkDCRequest that has a PutChunkRequest to process and a list of one or more next
// RVs to forward the request to. The PutChunkRequest must be for one of our local RVs. If the RV mentioned in the
// PutChunkRequest is not the local RV, it will return an InvalidRVID error.
// Parallelly, it also forwards the PutChunkRequest to the next RV in the list, making a daisy chain.
//
// For the local RV, it calls the PutChunk RPC via the handler directly.
// The response/error returned by the PutChunk calls to the local RV and next RVs are returned in the
// PutChunkDCResponse, with the RV name as the key and its PutChunkResponse or a ResponseError as the value.
// The PutChunkDCResponse will have the responses for all the RVs in the list, including the local RV.
func (h *ChunkServiceHandler) PutChunkDC(ctx context.Context, req *models.PutChunkDCRequest) (*models.PutChunkDCResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	// Thrift should not be calling us with nil Request, Chunk or Address.
	common.Assert(req.Request != nil)
	common.Assert(req.Request.Chunk != nil)
	common.Assert(req.Request.Chunk.Address != nil)

	//
	// Caller should not be calling us with empty NextRVs.
	// It should call PutChunkDC only if it needs to be forwarded to at least one RV, else it should simply
	// call PutChunk.
	//
	common.Assert(len(req.NextRVs) > 0)

	log.Debug("ChunkServiceHandler::PutChunkDC: Received PutChunkDC request: %v",
		rpc.PutChunkDCRequestToString(req))

	// Nexthop RV must be one of our local RVs.
	rvInfo, ok := h.rvIDMap[req.Request.Chunk.Address.RvID]
	if !ok {
		errStr := fmt.Sprintf("Nexthop RV is not local: %s", req.Request.Chunk.Address.String())
		log.Err("ChunkServiceHandler::PutChunkDC: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRVID, errStr)
	}

	common.Assert(rvInfo != nil, req.Request.Chunk.Address.String())

	// Nexthop RV must not be repeated in NextRVs.
	common.Assert(!slices.Contains(req.NextRVs, rvInfo.rvName), rvInfo.rvName, req.NextRVs)

	var rpcResp *models.PutChunkDCResponse
	var err error
	var wg sync.WaitGroup

	//
	// Parallelly forward the PutChunkDC request to the next RV in the list.
	// The first RV in req.NextRVs will become the nexthop RV to which forwardPutChunk() will forward the
	// PutChunkDCRequest, it'll be removed from the NextRVs list, and the remaining NextRVs list will be
	// sent to the nexthop RV to further forward the request.
	//
	wg.Add(1)
	go func() {
		defer wg.Done()

		rpcResp = h.forwardPutChunk(ctx, req.Request, req.NextRVs)
		common.Assert(rpcResp != nil)
		// Must return status for every RV.
		common.Assert(len(rpcResp.Responses) == len(req.NextRVs))
	}()

	//
	// The PutChunkRequest in the PutChunkDCRequest corresponds to the PutChunk call to the local RV.
	// So, call PutChunk from the handler directly.
	//
	resp, err := h.PutChunk(ctx, req.Request)
	var rpcErr *models.ResponseError

	//
	// The PutChunk call made here is for the local RV. So, the error returned here will be
	// an RPC error, if the PutChunk call failed, or nil if it succeeded.
	// So, we can assert that the error if non-nil, is of type *models.ResponseError.
	//
	if err != nil {
		common.Assert(resp == nil)
		log.Err("ChunkServiceHandler::PutChunkDC: PutChunk failed for local RV %s/%s, request: %s [%v]",
			rvInfo.rvName, req.Request.Chunk.Address.MvName,
			rpc.PutChunkRequestToString(req.Request), err)
		rpcErr = rpc.GetRPCResponseError(err)
		common.Assert(rpcErr != nil, err)
	} else {
		common.Assert(resp != nil)
		log.Debug("ChunkServiceHandler::PutChunkDC: PutChunk succeeded for local RV %s/%s, request: %s, response: %s",
			rvInfo.rvName, req.Request.Chunk.Address.MvName,
			rpc.PutChunkRequestToString(req.Request), rpc.PutChunkResponseToString(resp))
	}

	// Wait for the forwarded request to complete.
	wg.Wait()

	common.Assert(rpcResp != nil)
	// forwardPutChunk() must return status for every RV.
	common.Assert(len(rpcResp.Responses) == len(req.NextRVs))

	rpcResp.Responses[rvInfo.rvName] = &models.PutChunkResponseOrError{
		Response: resp,
		Error:    rpcErr,
	}

	log.Debug("ChunkServiceHandler::PutChunkDC: Completing for nexthop %s/%s (file id: %s, offset in MiB: %d): %s",
		rvInfo.rvName, req.Request.Chunk.Address.MvName, req.Request.Chunk.Address.FileID,
		req.Request.Chunk.Address.OffsetInMiB, rpc.PutChunkDCResponseToString(rpcResp))

	// We must return status for every RV we were asked to write to.
	common.Assert(len(rpcResp.Responses) == len(req.NextRVs)+1, len(rpcResp.Responses), len(req.NextRVs))

	return rpcResp, nil
}

// This method sends the PutChunkRequest 'req' to all the RVs in 'rvs' list in a daisy chain fashion.
// The first RV in rvs[] becomes the nexthop RV to which the PutChunkDCRequest is sent and the remaining RVs in rvs[]
// will be set as the NextRVs for the PutChunkDCRequest. The nexthop will run the PutChunkRequest and send the
// request to its nexthop and set NextRVs to the remaining, and so on, till the request reaches all the RVs in rvs[].
func (h *ChunkServiceHandler) forwardPutChunk(ctx context.Context, req *models.PutChunkRequest, rvs []string) *models.PutChunkDCResponse {
	common.Assert(req != nil)
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)
	common.Assert(len(rvs) > 0)

	nexthopRV := rvs[0]
	common.Assert(cm.IsValidRVName(nexthopRV), nexthopRV, rvs)

	var nextRVs []string
	if len(rvs) > 1 {
		nextRVs = rvs[1:]
	}

	log.Debug("ChunkServiceHandler::forwardPutChunk: Forwarding PutChunk to nexthop RV %s, daisy chaining to %d more RV(s): %v, request: %s",
		nexthopRV, len(nextRVs), nextRVs, rpc.PutChunkRequestToString(req))

	nexthopRVId := cm.RvNameToId(nexthopRV)
	common.Assert(common.IsValidUUID(nexthopRVId))

	nexthopNodeId := cm.RVNameToNodeId(nexthopRV)
	common.Assert(common.IsValidUUID(nexthopNodeId))

	log.Debug("ChunkServiceHandler::forwardPutChunk: Writing to nexthop RV %s/%s (RVId: %s) on node %s",
		nexthopRV, req.Chunk.Address.MvName, nexthopRVId, nexthopNodeId)

	//
	// Create PutChunkRequest for the nexthop RV.
	// The only updated fields in the request is RvID.
	//
	putChunkReq := &models.PutChunkRequest{
		Chunk: &models.Chunk{
			Address: &models.Address{
				FileID:      req.Chunk.Address.FileID,
				RvID:        nexthopRVId,
				MvName:      req.Chunk.Address.MvName,
				OffsetInMiB: req.Chunk.Address.OffsetInMiB,
			},
			Data: req.Chunk.Data,
			Hash: req.Chunk.Hash,
		},
		Length:         req.Length,
		SyncID:         req.SyncID,
		ComponentRV:    req.ComponentRV,
		MaybeOverwrite: req.MaybeOverwrite,
	}

	//
	// This is the last RV in the list, so we will call PutChunk directly on it.
	// Else, we will call PutChunkDC on it with the next RVs in the list.
	//
	if len(nextRVs) == 0 {
		log.Debug("ChunkServiceHandler::forwardPutChunk: Forwarding PutChunk request to last RV %s/%s on node %s: %s",
			nexthopRV, req.Chunk.Address.MvName, nexthopNodeId, rpc.PutChunkRequestToString(putChunkReq))

		var rpcErr *models.ResponseError

		putChunkResp, err := rpc_client.PutChunk(ctx, nexthopNodeId, putChunkReq)
		if err != nil {
			log.Err("ChunkServiceHandler::forwardPutChunk: Failed to forward PutChunk request to last RV %s/%s on node %s: %v",
				nexthopRV, req.Chunk.Address.MvName, nexthopNodeId, err)
			common.Assert(putChunkResp == nil)

			rpcErr = rpc.GetRPCResponseError(err)
			if rpcErr == nil {
				//
				// This error indicates some Thrift error like connection error, timeout, etc. or,
				// it could be an RPC client side error like failed to get RPC client for target node.
				// We wrap this error in *models.ResponseError with code ThriftError.
				// This is to ensure that the client can take appropriate action based on this error
				// code.
				//
				rpcErr = rpc.NewResponseError(models.ErrorCode_ThriftError, err.Error())
			}
		} else {
			common.Assert(putChunkResp != nil)
		}

		common.Assert(len(rvs) == 1, rvs)
		return &models.PutChunkDCResponse{
			Responses: map[string]*models.PutChunkResponseOrError{
				nexthopRV: {
					Response: putChunkResp,
					Error:    rpcErr,
				},
			},
		}
	} else {
		putChunkDCReq := &models.PutChunkDCRequest{
			Request: putChunkReq,
			NextRVs: nextRVs,
		}

		log.Debug("ChunkServiceHandler::forwardPutChunk: Forwarding PutChunkDC request to nexthop %s/%s on node %s: %s",
			nexthopRV, req.Chunk.Address.MvName, nexthopNodeId, rpc.PutChunkDCRequestToString(putChunkDCReq))

		dcResp, err := rpc_client.PutChunkDC(ctx, nexthopNodeId, putChunkDCReq)

		//
		// If the PutChunkDC RPC call fails, the error returned can be,
		// - Thrift error like connection error, timeout, etc.
		// - RPC client side error like failed to get RPC client for node.
		// - RPC error of type *models.ResponseError returned by the server like InvalidRVID.
		// We classify the first two as ThriftError. This indicates the caller that the PutChunk calls
		// were not forwarded after this RV and the caller can take appropriate action like marking
		// this RV as offline and retrying the PutChunkDC call.
		// For the next RVs in this call, the PutChunk calls were not forwarded. So, it returns
		// BrokenChain error for these RVs indicating that the PutChunkDC call was not forwarded
		// to them and the caller can retry.
		//
		if err != nil {
			log.Err("ChunkServiceHandler::forwardPutChunk: Failed to forward PutChunkDC request to nexthop %s/%s on node %s: %s",
				nexthopRV, req.Chunk.Address.MvName, nexthopNodeId, err)
			common.Assert(dcResp == nil)

			dcResp = rpc.HandlePutChunkDCError(nexthopRV, nextRVs, req.Chunk.Address.MvName, err)
		} else {
			log.Debug("ChunkServiceHandler::forwardPutChunk: Received response from nexthop %s/%s (file id %s, offset in MiB %d): %s",
				nexthopRV, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB,
				rpc.PutChunkDCResponseToString(dcResp))
		}

		common.Assert(dcResp != nil)
		common.Assert(len(dcResp.Responses) == len(rvs), len(dcResp.Responses), rvs)
		return dcResp
	}
}

func (h *ChunkServiceHandler) RemoveChunk(ctx context.Context, req *models.RemoveChunkRequest) (*models.RemoveChunkResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Address != nil)

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::RemoveChunk: Received RemoveChunk request %v", rpc.RemoveChunkRequestToString(req))

	// Sender node id must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)

	// Check if the chunk address is valid.
	err := h.checkValidChunkAddress(req.Address)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Invalid chunk address %v [%s]", req.Address.String(), err.Error())
		return nil, err
	}

	// RemoveChunk must not address a specific chunk but all chunks of a file.
	common.Assert(req.Address.OffsetInMiB == -1, req.Address.OffsetInMiB)

	rvInfo := h.rvIDMap[req.Address.RvID]
	mvInfo := rvInfo.getMVInfo(req.Address.MvName)

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	// Validate the component RVs list.
	err = mvInfo.isComponentRVsValid(req.ComponentRV, true /* checkState */)
	if err != nil {
		errStr := fmt.Sprintf("Component RVs are invalid for MV %s [%v]", req.Address.MvName, err)
		log.Err("ChunkServiceHandler::RemoveChunk: %s", errStr)
		return nil, err
	}

	//
	// Acquire read lock on the opMutex for this MV to prevent sync from starting for this MV while
	// we are deleting file chunks to avoid situations where a chunk is read by the sync thread but before
	// it can read and copy, it's deleted.
	//
	mvInfo.acquireSyncOpReadLock()
	defer mvInfo.releaseSyncOpReadLock()

	cacheDir := rvInfo.cacheDir
	numChunksDeleted := int64(0)

	// MV directory containing the requested chunks.
	mvDir := filepath.Join(cacheDir, req.Address.MvName)

	//
	// Enumerate all chunks and hashes in the MV directory, filter out the ones belonging to the
	// requested file and delete them.
	//
	// TODO: Replace this with chunked readdir to support huge number of chunks.
	//
	log.Debug("ChunkServiceHandler::RemoveChunk: Starting listing MV directory: %s", mvDir)

	dirEntries, err := os.ReadDir(mvDir)
	if err != nil {
		err = fmt.Errorf("failed to read mv directory: %s [%v]", mvDir, err)
		log.Err("ChunkServiceHandler::RemoveChunk: %v", err)
		common.Assert(false, err)
		return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, err.Error())
	}

	// Iterate and remove chunks and hashes belonging to the requested file.
	for _, dirent := range dirEntries {
		if !strings.HasPrefix(dirent.Name(), req.Address.FileID) {
			continue
		}

		fileInfo, err := dirent.Info()
		if err != nil {
			err = fmt.Errorf("failed to stat chunk file: %s [%v]", dirent.Name(), err)
			log.Err("ChunkServiceHandler::RemoveChunk: %v", err)
			common.Assert(false, err)
			//
			// If we are able to delete at least one chunk, respond with success.
			// Caller should assume all chunks of file deleted only when a RemoveChunk call succeeds with
			// NumChunksDeleted == 0.
			//
			if numChunksDeleted > 0 {
				break
			}
			return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, err.Error())
		}

		chunkPath := filepath.Join(mvDir, dirent.Name())

		err = os.Remove(chunkPath)
		if err != nil {
			err = fmt.Errorf("failed to remove chunk file: %s [%v]", dirent.Name(), err)
			log.Err("ChunkServiceHandler::RemoveChunk: %v", err)
			common.Assert(false, err)

			if numChunksDeleted > 0 {
				break
			}
			return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, err.Error())
		}

		// Decrement the total chunk bytes for this MV.
		mvInfo.decTotalChunkBytes(fileInfo.Size())

		numChunksDeleted++
	}

	common.Assert(numChunksDeleted >= 0)

	availableSpace, err := rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to get available disk space [%v]", err.Error())
		availableSpace = 0
	}

	resp := &models.RemoveChunkResponse{
		TimeTaken:        time.Since(startTime).Microseconds(),
		AvailableSpace:   availableSpace,
		ComponentRV:      mvInfo.getComponentRVs(),
		NumChunksDeleted: numChunksDeleted,
	}

	return resp, nil
}

func (h *ChunkServiceHandler) JoinMV(ctx context.Context, req *models.JoinMVRequest) (*models.JoinMVResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	//
	// See if it's a new-mv request (and not a fix-mv request)
	//
	newMV := false
	found := false
	_ = found
	for _, rv := range req.ComponentRV {
		if rv.Name == req.RVName {
			// Must be either online for new-mv and outofsync for fix-mv.
			common.Assert(rv.State == string(dcache.StateOnline) ||
				rv.State == string(dcache.StateOutOfSync), rv.Name, req.MV, rv.State)

			newMV = (rv.State == string(dcache.StateOnline))
			found = true
		}
	}
	common.Assert(found, req.RVName, req.MV, rpc.JoinMVRequestToString(req))

	// TODO:: discuss: changing type of component RV from string to RVNameAndState
	// requires to call componentRVsToString method as it is of type []*models.RVNameAndState
	log.Debug("ChunkServiceHandler::JoinMV: Received JoinMV request (newMV: %v): %v",
		newMV, rpc.JoinMVRequestToString(req))

	if !common.IsValidUUID(req.SenderNodeID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.RVName) ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid SenderNodeID, MV, RV or ComponentRV: %v", rpc.JoinMVRequestToString(req))
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

	// Acquire lock on rvInfo.rwMutex to prevent concurrent JoinMV calls for different MVs.
	rvInfo.acquireRvInfoLock()

	// Release lock on rvInfo.rwMutex for this RV when the function returns.
	defer rvInfo.releaseRvInfoLock()

	// Check if RV is already part of the given MV.
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo != nil {
		//
		// JoinMV and UpdateMV need to be idempotent to not treat "double join" as failure.
		// Double join can happen when let's say we have two or more outofsync component RVs
		// for an MV and fixMV() sends JoinMV request to each of the outofsync RVs. If one or
		// more of these fail, the joinMV() will treat it as a failure and not update clustermap.
		// Next time when fixMV() is called it'll again attempt fixing and again send JoinMV.
		// Note that for proper handling we need to ensure that the reservedSpace remains
		// same across both calls. Also if an RV is joined but never used later (maybe joinMV()
		// picked a new RV in the next iteration), we should time out and undo the reservedSpace.
		// This one is a TODO.
		//
		errStr := fmt.Sprintf("Double join for %s/%s, prev join at: %s, by: %s",
			req.RVName, req.MV, mvInfo.lmt, mvInfo.lmb)

		log.Warn("ChunkServiceHandler::JoinMV: %s", errStr)

		//
		// Refresh our mvInfo state as per the latest clustermap.
		// This will undo the changes made by the prev incomplete JoinMV, updating mvInfo as if the
		// previous JoinMV never happened. If refreshFromClustermap() fails, we cannot safely proceed.
		// For newMV, we won't have the MV in clustermap yet, so no need to refresh.
		//
		if !newMV {
			rpcErr := mvInfo.refreshFromClustermap()
			if rpcErr != nil {
				errStr = fmt.Sprintf("%s, refreshFromClustermap() failed, aborting JoinMV: %s",
					errStr, rpcErr.String())
				log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
				return nil, rpc.NewResponseError(rpcErr.Code, errStr)
			}
		}

		// Remove the MV replica, we will add a fresh one later down.
		rvInfo.deleteFromMVMap(req.MV)
		mvInfo = nil
	}

	mvLimit := getMVsPerRV()
	pruned := false

	for {
		if rvInfo.mvCount.Load() >= mvLimit {
			//
			// This might happen due to incomplete JoinMV requests taking up space, so it will help
			// to refresh rvInfo details from the clustermap and remove any unused MVs, and try again.
			//
			errStr := fmt.Sprintf("%s cannot host any more MVs (MVsPerRV: %d)", req.RVName, mvLimit)
			log.Err("ChunkServiceHandler::JoinMV: %s", errStr)

			if !pruned {
				rvInfo.pruneStaleEntriesFromMvMap()
				pruned = true
				continue
			}

			return nil, rpc.NewResponseError(models.ErrorCode_MaxMVsExceeded, errStr)
		}

		break
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
	// JoinMV calls [TODO].
	//
	sortComponentRVs(req.ComponentRV)

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	rvInfo.addToMVMap(req.MV, newMVInfo(rvInfo, req.MV, req.ComponentRV, req.SenderNodeID), req.ReserveSpace)

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) UpdateMV(ctx context.Context, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::UpdateMV: Received UpdateMV request: %v", rpc.UpdateMVRequestToString(req))

	if !common.IsValidUUID(req.SenderNodeID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.RVName) ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid SenderNodeID, MV, RV or ComponentRV: %v", rpc.UpdateMVRequestToString(req))
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
			errStr := fmt.Sprintf("%s/%s not hosted by this node", req.RVName, req.MV)
			log.Err("ChunkServiceHandler::UpdateMV: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
		}

		componentRVsInMV := mvInfo.getComponentRVs()
		_ = componentRVsInMV

		log.Debug("ChunkServiceHandler::UpdateMV: Updating %s from (%s -> %s)",
			req.MV, rpc.ComponentRVsToString(componentRVsInMV), rpc.ComponentRVsToString(req.ComponentRV))

		//
		// If the component RVs list has any RV with inband-offline state, update it to offline.
		// This is done because we don't allow inband-offline state in the rvInfo.
		//
		updateInbandOfflineToOffline(&req.ComponentRV)

		//
		// update the component RVs list for this MV
		// mvInfo.updateComponentRVs() only allows valid changes to cluster membership.
		//
		// Note: Updating this unconditionally could be risky.
		//       A node with an outdated clustermap can reverse a later change.
		//       f.e. some node is syncing and has changed state of an rv to syncing
		//       meanwhile some other node with an older clustermap wants to join an MV to this rv.
		//       it fetched clustermap but then due to n/w down, by the time it reached fixMV, rv was
		//       already marked syncing, but now it has rv as outofsync and it forces it as that
		//
		err := mvInfo.updateComponentRVs(req.ComponentRV, false /* forceUpdate */, req.SenderNodeID)
		if err != nil {
			if !clustermapRefreshed {
				rpcErr := mvInfo.refreshFromClustermap()
				if rpcErr != nil {
					log.Err("ChunkServiceHandler::UpdateMV: Failed to refresh clustermap [%s]",
						rpcErr.String())
					return nil, rpcErr
				}
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

	if !common.IsValidUUID(req.SenderNodeID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.RVName) ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid SenderNodeID, MV, RV or ComponentRV: %v", rpc.LeaveMVRequestToString(req))
		log.Err("ChunkServiceHandler::LeaveMV: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		log.Err("ChunkServiceHandler::LeaveMV: Invalid RV %s", req.RVName)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, fmt.Sprintf("invalid RV %s", req.RVName))
	}

	cacheDir := rvInfo.cacheDir

	// Acquire lock on rvInfo.rwMutex to prevent concurrent JoinMV or LeaveMV calls for different MVs.
	rvInfo.acquireRvInfoLock()

	// Release lock on rvInfo.rwMutex for this RV when the function returns.
	defer rvInfo.releaseRvInfoLock()

	//
	// LeaveMV() RPC is only sent to RVs which are already members of the MV, and it is sent
	// when the membership changes (due to rebalancing workflow).
	// Since the sender is referring to the global clustermap and this RV is part of the given MV as
	// per the global clustermap, since an RV is added to an MV only after a successful JoinMV response
	// from all component RVs, we *must* have the MV replica in our rvInfo.
	//
	// TODO: There is one scenario in which this is possible, if a node responds to LeaveMV() successfully
	//       but the sender cannot commit it to clustermap for some reason, then when LeaveMV() is retried
	//       it'll not find the MV.
	//
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		errStr := fmt.Sprintf("%s/%s not hosted by this node", req.RVName, req.MV)
		log.Err("ChunkServiceHandler::LeaveMV: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	// validate the component RVs list
	err := mvInfo.isComponentRVsValid(req.ComponentRV, true /* checkState */)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: %v", err)
		return nil, err
	}

	// delete the MV directory
	mvPath := filepath.Join(cacheDir, req.MV)

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

	if !common.IsValidUUID(req.SenderNodeID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.SourceRVName) ||
		!cm.IsValidRVName(req.TargetRVName) ||
		req.SourceRVName == req.TargetRVName ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid StartSync request: %s", rpc.StartSyncRequestToString(req))
		log.Err("ChunkServiceHandler::StartSync: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	//
	// Source RV is the lowest index online RV. The node hosting this RV will send the start sync call
	// to the outofsync component RVs which become the target of the sync.
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

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

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
	// If sourceRVName is set that means this MV Replica is the target of this sync job, while if
	// targetRVName is set it means this MV Replica is the source of this sync job.
	//
	var sourceRVName, targetRVName string

	if isSrcOfSync {
		targetRVName = req.TargetRVName
	} else {
		sourceRVName = req.SourceRVName
	}

	//
	// Add this sync job to the syncJobs map.
	// This will be removed by EndSync RPC after the sync job completes.
	// If the sender cannot complete the sync job for some reason (caller crashed or maybe it could not
	// get successful RPC responses from all parties or maybe it could not commit the global clustermap
	// changes) it won't run the EndSync RPC. In such cases the syncJob will be sitting in mvInfo.
	// Later when some client sends some RPC to this RV which expects it to not be in syncing state,
	// refreshFromClustermap() would run and if it finds that the global clustermap has the RV in outofsync
	// state that would be used as an indicator to undo the StartSync, most importantly reset the rvInfo
	// state to OutOfSyc and purging the sync job.
	//
	syncID := mvInfo.addSyncJob(sourceRVName, targetRVName)

	// Update the state of target RV in this MV replica from outofsync to syncing.
	mvInfo.updateComponentRVState(req.TargetRVName, dcache.StateOutOfSync, dcache.StateSyncing, req.SenderNodeID)

	log.Debug("ChunkServiceHandler::StartSync: %s/%s responding to StartSync request: %s, with syncID: %s",
		rvInfo.rvName, req.MV, rpc.StartSyncRequestToString(req), syncID)

	return &models.StartSyncResponse{
		SyncID: syncID,
	}, nil
}

func (h *ChunkServiceHandler) EndSync(ctx context.Context, req *models.EndSyncRequest) (*models.EndSyncResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::EndSync: Received EndSync request: %v", rpc.EndSyncRequestToString(req))

	if !common.IsValidUUID(req.SenderNodeID) ||
		!common.IsValidUUID(req.SyncID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.SourceRVName) ||
		!cm.IsValidRVName(req.TargetRVName) ||
		req.SourceRVName == req.TargetRVName ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid EndSync request: %s", rpc.EndSyncRequestToString(req))
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

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the rvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

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
	mvInfo.updateComponentRVState(req.TargetRVName, dcache.StateSyncing, dcache.StateOnline, req.SenderNodeID)

	// As sync has completed, clear reservedSpace and commit it in totalChunkBytes.
	mvInfo.totalChunkBytes.Add(mvInfo.reservedSpace.Load())
	common.Assert(rvInfo.reservedSpace.Load() >= mvInfo.reservedSpace.Load(),
		rvInfo.reservedSpace.Load(), mvInfo.reservedSpace.Load(), rvInfo.rvName,
		req.MV, rpc.EndSyncRequestToString(req))
	rvInfo.decReservedSpace(mvInfo.reservedSpace.Load())
	mvInfo.reservedSpace.Store(0)

	log.Debug("ChunkServiceHandler::EndSync: %s/%s responding to EndSync request: %s",
		rvInfo.rvName, req.MV, rpc.EndSyncRequestToString(req))

	//
	// If we were the target of this sync job, then nothing else to do.
	// Assert if that the MV replica can be the target of only one sync job at a time.
	//
	if !isSrcOfSync {
		// An MV replica can be the target of only one sync job at a time.
		common.Assert(!mvInfo.isSyncing())
		return &models.EndSyncResponse{}, nil
	}

	//
	// After deleting this sync job, check if there are any other sync jobs in progress for this MV replica.
	// If yes, then return success for this EndSync call.
	// Else, this EndSync call is for the last running syncJob for this MV replica.
	//
	if mvInfo.isSyncing() {
		log.Debug("ChunkServiceHandler::EndSync: %s/%s is source replica for %d running sync job(s): %+v",
			rvInfo.rvName, req.MV, mvInfo.syncJobsCount.Load(), mvInfo.getSyncJobs())
	}

	return &models.EndSyncResponse{}, nil
}

func (h *ChunkServiceHandler) GetMVSize(ctx context.Context, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)

	log.Debug("ChunkServiceHandler::GetMVSize: Received GetMVSize request: %v", rpc.GetMVSizeRequestToString(req))

	if !common.IsValidUUID(req.SenderNodeID) || !cm.IsValidMVName(req.MV) || !cm.IsValidRVName(req.RVName) {
		errStr := fmt.Sprintf("Invalid GetMVSize request: %v", rpc.GetMVSizeRequestToString(req))
		log.Err("ChunkServiceHandler::GetMVSize: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	rvInfo := h.getRVInfoFromRVName(req.RVName)
	if rvInfo == nil {
		errStr := fmt.Sprintf("node %s does not host %s", rpc.GetMyNodeUUID(), req.RVName)
		log.Err("ChunkServiceHandler::GetMVSize: %s", errStr)
		common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRV, errStr)
	}

	//
	// An MV replica can be added to rvInfo only via a JoinMV RPC, and only when we respond successfully
	// to the JoinMV call will the sender persist it in the clustermap, so if the clustermap has it we
	// must have sent it and if we don't have it, refreshing from clustermap cannot add it.
	// This cannot happen unless sender is doing something wrong, hence assert.
	//
	// Update: This can happen if a component RV which is still part of the MV is no longer published
	//         by the owning node after it restarted. Client who fetches the component RV info from
	//         clustermap will find the RV as part of the MV and hence it may send the GetMVSize request
	//         to the node but the node that has now restarted doesn't have the component RV, hence it
	//         fails with "InvalidRequest" error.
	//
	mvInfo := rvInfo.getMVInfo(req.MV)
	if mvInfo == nil {
		errStr := fmt.Sprintf("%s/%s not hosted by this node", rvInfo.rvName, req.MV)
		log.Err("ChunkServiceHandler::GetMVSize: %s", errStr)
		//common.Assert(false, errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

	//
	// GetMVSize is only called for online MV replicas, for which reservedSpace should be 0.
	//
	common.Assert(mvInfo.reservedSpace.Load() == 0, rvInfo.rvName, req.MV, mvInfo.reservedSpace.Load())

	return &models.GetMVSizeResponse{
		MvSize: mvInfo.totalChunkBytes.Load(),
	}, nil
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
}
