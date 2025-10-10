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

	// TODO: This is released under MPL 2.0 license, need to include the license text.
	lru "github.com/hashicorp/golang-lru/v2"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	//
	// In case of multiple readers reading the same file, we may saturate the disk b/w, so we use an LRU cache
	// to cache recently served chunks. When multiple readers are reading the same file, they would do so usually
	// simultaneously, so the cache would help reduce the disk IO.
	// Note that this is the max number of chunks, not the size in bytes. Also, this may be reduced later depending
	// on the available memory.
	//
	ChunkCacheSize = 1024

	// TODO: These are for debug purposes only, remove them later.
	NumChunkWrites          atomic.Int64
	CumChunkWrites          atomic.Int64 // cumulative number of chunks written
	CumBytesWritten         atomic.Int64 // cumulative number of bytes written
	OpenDepth               atomic.Int64 // number of go routines inside syscall.Open() for to-be-written chunk files
	WriteDepth              atomic.Int64 // number of go routines inside syscall.Write()
	RenameDepth             atomic.Int64 // number of go routines inside common.RenameNoReplace()
	AggrChunkWritesDuration atomic.Int64 // time in nanoseconds for NumChunkWrites.

	NumChunkReads          atomic.Int64
	AggrChunkReadsDuration atomic.Int64 // time in nanoseconds for NumChunkReads.

	SlowReadWriteThreshold = 1 * time.Second // anything more than this is considered a slow chunk read/write
)

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

	//
	// LRU cache for chunks.
	// Helps scenarios when many nodes read the same file simultaneously.
	// Indexed by local chunk path.
	//
	chunkCache       *lru.Cache[string, []byte]
	chunkCacheLookup atomic.Int64
	chunkCacheHit    atomic.Int64
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
	// to the RV as a result of sync for all MVs hosted by this RV. This is used to calculate
	// the available space in the RV after subtracting the reserved space from the actual disk
	// space available.
	// JoinMV() will increment this space indicating that new MV is being added to this RV.
	// On the other hand, PutChunk() sync RPC call will decrement this space indicating
	// that the chunk has been written to the RV.
	reservedSpace atomic.Int64

	// Cumulative bytes read from this RV by GetChunk() requests.
	// Mostly used for debugging distribution of reads across RVs.
	totalBytesRead atomic.Int64

	// Number of chunks being currently being written to this RV.
	wqsize atomic.Int64
}

// This holds information about one MV hosted by our local RV. This is known as "MV Replica".
// rvInfo.mvMap contains one such struct for each MV Replica that the RV hosts.
// Note that this is not information about the entire MV. One MV is replicated across multiple RVs and this holds
// only the information about the "MV Replica" that our RV hosts.
type mvInfo struct {
	rwMutex      sync.RWMutex
	mdChunkMutex sync.Mutex // to serialize writes to the MD chunk
	mvName       string     // mv0, mv1, etc.

	// RV this MV is part of.
	// Note that mvInfo is referenced via rvInfo.mvMap so when we have rvInfo we already know the
	// RV name. This is for making the hosting RV available to functions that operate on mvInfo
	// and do not have the rvInfo.
	rv *rvInfo

	// sorted list of component RVs for this MV
	componentRVs []*models.RVNameAndState

	// mvInfo is updated in response to an RPC call made by a client.
	// This is the ClustermapEpoch carried by that RPC call, but note that the mvInfo may contain changes
	// on top of the clustermap corresponding to clustermapEpoch, e.g., a JoinMV RPC will carry the epoch
	// of the clustermap which has one of the component RVs as offline, but mvInfo will not contain the
	// offline RV but the new replacement RV with state as outofsync, which will be committed to clustermap
	// only after the JoinMV RPC completes successfully (along with other UpdateMV) and it'll get committed
	// as the next (even) epoch.
	// The only case when the aboce is true if when the mvInfo.clustermapEpoch is odd, and one or more
	// component RV state is outofsync. In all other cases, the componentRVs list in mvInfo will correspond
	// exactly to the clustermap at epoch mvInfo.clustermapEpoch. This means for the above case, a clustermap
	// with epoch X MUST NOT overwrite the componentRVs list of an mvInfo with clustermapEpoch X.
	// See refreshFromClustermap() for more details.
	clustermapEpoch int64

	// When was this MV replica components/state last updated and by which node.
	// An MV replica components/state is updated by the following RPCs:
	// JoinMV    - this creates a new MV replica. It is called by the new-mv and the fix-mv workflows.
	//             A new-mv workflow causes an MV replica to start as "online" while a fix-mv workflow
	//             causes an MV replica to start as "outofsync".
	// UpdateMV  - this updates the composition of an MV replica.
	//
	// Other than the above two RPCs, MV replica state/components can be updated by refreshFromClustermap().
	//
	// lmt - Last Modified Time
	// lmb - Last Modified By
	//
	// Note: These are only used for logging and assertions.
	//
	lmt time.Time
	lmb string

	// Total amount of space used up inside the MV directory, by all the chunks stored in it.
	// Any RV that has to replace one of the existing component RVs needs to have at least this much space.
	// JoinMV() requests this much space to be reserved in the new-to-be-inducted RV.
	// This is incremented whenever a new chunk is written to this MV replica by PutChunk (client or sync)
	// requests, and decremented whenever a chunk is deleted by RemoveChunk requests.
	totalChunkBytes atomic.Int64

	// Amount of space reserved for this MV replica, on the hosting RV.
	// When a new mvInfo is created by JoinMV() this is set to the ReserveSpace parameter to JoinMV.
	// This is also added to rvInfo.reservedSpace to reserve space in the RV, to prevent oversubscription
	// by accepting JoinMV requests for more MVs than we can host.
	// This is non-zero only for MV replicas which are added by the fix-mv workflow and not for MV replicas
	// added by new-mv workflow. Put another way, this will be non-zero only for MV replicas which are in
	// outofsync or syncing state. When refreshFromClustermap() changes the state of an MV replica from
	// syncing to online, this is set to zero and also reduced from rvInfo.reservedSpace. Now we don't need
	// this we the chunks are actually written to the MV replica and hence the space is actually used and
	// accounted.
	// If an MV replica cannot complete resync, and thus cannot move from syncing->online, this must be
	// reduced from rvInfo.reservedSpace.
	// This means while an MV replica is being sync'ed the space used on the RV may be overcompensated, this
	// is corrected once sync completes.
	reservedSpace atomic.Int64
}

var handler *ChunkServiceHandler

// NewChunkServiceHandler creates a new ChunkServiceHandler instance.
// This MUST be called only once by the RPC server, on startup.
func NewChunkServiceHandler(rvMap map[string]dcache.RawVolume) error {
	log.Debug("NewChunkServiceHandler: called with rvMap: %+v, sepoch: %d", rvMap, cm.GetEpoch())
	common.Assert(handler == nil, "NewChunkServiceHandler called more than once")

	// Must be called only once.
	common.Assert(dcache.MDChunkOffsetInMiB == 0, dcache.MDChunkOffsetInMiB, dcache.MDChunkIdx)
	dcache.MDChunkOffsetInMiB = int64(cm.GetCacheConfig().ChunkSizeMB) * dcache.MDChunkIdx
	// It'll be larger than 1ZiB but this's enough for sanity check.
	common.Assert(dcache.MDChunkOffsetInMiB > (1024*1024*1024*1024),
		dcache.MDChunkOffsetInMiB, cm.GetCacheConfig().ChunkSizeMB, dcache.MDChunkIdx)

	handler = &ChunkServiceHandler{
		rvIDMap: getRvIDMap(rvMap),
	}

	//
	// Initialize chunk cache, 1024 chunks or 10% of available memory, whichever is lower.
	//
	const usablePercentSystemRAM = 10

	ramMB, err := common.GetAvailableMemoryInMB()
	if err != nil {
		return fmt.Errorf("NewChunkServiceHandler: %v", err)
	}

	usableMemoryMB := (ramMB * uint64(usablePercentSystemRAM)) / 100
	ChunkCacheSize = min(ChunkCacheSize, int(usableMemoryMB/cm.GetCacheConfig().ChunkSizeMB))
	ChunkCacheSize = max(ChunkCacheSize, 64) // at least 64 chunks.

	chunkCache, err := lru.New[string, []byte](ChunkCacheSize)
	if err != nil {
		err = fmt.Errorf("NewChunkServiceHandler: Failed to create chunk cache of size: %d chunks: %v",
			ChunkCacheSize, err)
		common.Assert(false, err)
		return err
	}

	handler.chunkCache = chunkCache

	log.Info("NewChunkServiceHandler: Created chunk cache, size: %d chunks (ramMB: %d)",
		ChunkCacheSize, ramMB)

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
			mvState, componentRVMap, clustermapEpoch := cm.GetRVsEx(mvName)
			_ = mvState
			// Offline MVs must not be present in the cache dir (see safeCleanupMyRVs()).
			common.Assert(mvState != dcache.StateOffline, rvName, mvName, rv.LocalCachePath)
			_, ok := componentRVMap[rvName]
			_ = ok

			// We should only have MV dirs for active MVs for the RV.
			common.Assert(ok, rvName, mvName, componentRVMap)

			componentRVs := cm.RVMapToList(mvName, componentRVMap, false /* randomize */)
			sortComponentRVs(componentRVs)

			log.Debug("NewChunkServiceHandler: %s/%s has componentRVs: %v, at epoch: %d",
				rvName, mvName, rpc.ComponentRVsToString(componentRVs), clustermapEpoch)

			//
			// If the component RVs list has any RV with inband-offline state, update it to offline.
			// This is done because we don't allow inband-offline state in the mvInfo.
			//
			updateInbandOfflineToOffline(&componentRVs)

			mvDirSize, err := getMVDirSize(filepath.Join(rv.LocalCachePath, mvName))
			if err != nil {
				log.Err("NewChunkServiceHandler: %v", err)
				common.Assert(false, rv.LocalCachePath, mvName, err)
			}

			mvInfo := newMVInfo(rvInfo, mvName, componentRVs, clustermapEpoch, mvDirSize, rpc.GetMyNodeUUID())

			//
			// Acquire lock on rvInfo.rwMutex.
			// This is running from the single startup thread, so we don't really need the lock, but
			// addToMVMap() asserts for that.
			//
			rvInfo.acquireRvInfoLock()
			rvInfo.addToMVMap(mvName, mvInfo, 0 /* reservedSpace */)
			rvInfo.releaseRvInfoLock()
		}
	}

	return nil
}

// Create new mvInfo instance. This is used by the JoinMV() RPC call to create a new mvInfo.
func newMVInfo(rv *rvInfo, mvName string, componentRVs []*models.RVNameAndState,
	clustermapEpoch int64, totalChunkBytes int64, joinedBy string) *mvInfo {
	common.Assert(common.IsValidUUID(joinedBy), rv.rvName, mvName, joinedBy)
	common.Assert(!containsInbandOfflineState(&componentRVs), componentRVs)
	common.Assert(clustermapEpoch > 0, rv.rvName, mvName, clustermapEpoch)
	common.Assert(totalChunkBytes >= 0, rv.rvName, mvName, totalChunkBytes)

	mv := &mvInfo{
		rv:              rv,
		mvName:          mvName,
		componentRVs:    componentRVs,
		lmt:             time.Now(),
		lmb:             joinedBy,
		clustermapEpoch: clustermapEpoch,
	}

	mv.totalChunkBytes.Store(totalChunkBytes)

	log.Debug("newMVInfo: %s/%s, %+v", rv.rvName, mvName, *mv)

	return mv
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
		// Must correspond to a valid clustermap epoch (client clustermap epoch when they sent the JoinMV, or
		// clustermap epoch when NewChunkServiceHandler initialized the mvInfo from cache dir).
		common.Assert(mvInfo.clustermapEpoch > 0, rv.rvName, mvInfo.mvName, mvInfo.clustermapEpoch,
			mvInfo.lmb, mvInfo.lmt)

		return mvInfo
	}

	// Value not of type mvInfo.
	common.Assert(false, mvName, rv.rvName)

	return nil
}

// return the list of MVs for this RV
func (rv *rvInfo) getMVs() []string {
	mvs := make([]string, 0)
	rv.mvMap.Range(func(key, val interface{}) bool {
		mvName := key.(string)
		_ = mvName
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

			mvs = append(mvs, mvInfo.mvName)
		} else {
			err := fmt.Errorf("mvMap[%s] has value which is not of type *mvInfo: %T", mvName, val)
			common.Assert(false, err)
			log.Err("rvInfo::getMVs: %v", err)

			/* Skip invalid entry */
		}

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

	val, ok := rv.mvMap.Load(mvName)
	if !ok {
		common.Assert(false, fmt.Sprintf("mvMap[%s] not found", mvName))
		return
	}

	mv := val.(*mvInfo)

	common.Assert(mv != nil, rv.rvName, mvName)
	common.Assert(mvName == mv.mvName, mvName, mv.mvName, rv.rvName)
	common.Assert(rv.rvName == mv.rv.rvName, rv.rvName, mv.rv.rvName, mvName)

	//
	// Undo the reserved space for this MV if any, from the hosting RV.
	//
	common.Assert(mv.reservedSpace.Load() >= 0, mv.reservedSpace.Load(), rv.rvName, mvName)
	common.Assert(rv.reservedSpace.Load() >= mv.reservedSpace.Load(), rv.rvName, mvName, rv.reservedSpace.Load(),
		mv.reservedSpace.Load())

	rv.decReservedSpace(mv.reservedSpace.Load())

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

	//
	// Subtract the reserved space for this RV.
	// Note that the following will result in a more conservative available space estimate for the RV if one
	// or more MV replicas hosted by this RV are syncing. This is because we increment rv.reservedSpace inside
	// the JoinMV handler when this RV was picked as the replacement RV by the fixMV workflow, and we decrement
	// it inside the EndSync handler, only after the sync completes. During the sync process there will be chunks
	// written to the MV replica being synced, but we don't decrement rv.reservedSpace for each chunk written.
	// This conservative estimate is deliberate, as we don't want to overcommit space on the RV.
	//
	// TODO: there can be a case when the available space is negative. We decrement the reserved space only in EndSync,
	//       and not in PutChunk(sync) calls. So, after writing the chunks via sync and regular client PutChunk
	//       calls to the cache directory, the disk available space can become less than the reserved space. Thus,
	//       availableSpace can be negative. This can be fixed by decrementing the reserved space after each
	//       successful PutChunk(sync) call.
	//       However this is also not straight forward. When we send JoinMV call as part of fix-mv workflow,
	//       we send the reserved space required for the MV replica to be added to this RV. When we called
	//       GetMVSize() from JoinMV, there could have been more chunks being written to the source MV replica after
	//       we read the mvInfo.totalChunkBytes. So we reserved less but actually sync'ed more. So, directly
	//       decrementing the reserved space in PutChunk(sync) can lead to negative reserved space for both MV and RV.
	//
	availableSpace := int64(diskSpaceAvailable) - rv.reservedSpace.Load()
	common.Assert(availableSpace >= 0, rv.rvName, availableSpace, diskSpaceAvailable, rv.reservedSpace.Load())

	log.Debug("rvInfo::getAvailableSpace: RV: %s, availableSpace: %d, diskSpaceAvailable: %d, reservedSpace: %d",
		rv.rvName, availableSpace, diskSpaceAvailable, rv.reservedSpace.Load())

	return availableSpace, err
}

// Return available space for our local RV.
// It queries the file system to get the available space in the cache directory for the RV and subtracts
// any space reserved for the RV by the JoinMV RPC call.
func GetAvailableSpaceForRV(rvId, rvName string) (int64, error) {
	//
	// Initial call(s) before RPC server is started must simply return the available space as reported
	// by the file system, else we must subtract the reserved space for the RV.
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

// Get component RVs for this MV.
func (mv *mvInfo) getComponentRVs() []*models.RVNameAndState {
	mv.rwMutex.RLock()
	defer mv.rwMutex.RUnlock()

	common.Assert(len(mv.componentRVs) == int(cm.GetCacheConfig().NumReplicas),
		len(mv.componentRVs), cm.GetCacheConfig().NumReplicas)

	return mv.componentRVs
}

// Update the component RVs for the MV. Called by UpdateMV() handler.
// clustermapEpoch is the clustermap epoch that triggered this mvInfo update. If the update is done in response
// to a client request, this is the client clustermap epoch, while if the update is done as part of
// refreshFromClustermap(), this is the current clustermap epoch.
//
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
//
// When called from UpdateMV(), forceUpdate must be false.
// When called from refreshFromClustermap(), forceUpdate must be true.

func (mv *mvInfo) updateComponentRVs(componentRVs []*models.RVNameAndState, clustermapEpoch int64,
	forceUpdate bool, senderNodeId string) error {
	common.Assert(len(componentRVs) == int(cm.GetCacheConfig().NumReplicas),
		len(componentRVs), cm.GetCacheConfig().NumReplicas)
	common.Assert(clustermapEpoch > 0, mv.mvName, clustermapEpoch)

	//
	// A refreshFromClustermap() call can start before another one (and gets an older epoch) but the first one
	// may reach here after the second call (which may have updated mvInfo) so we may come here with a
	// clustermapEpoch which is less than mv.clustermapEpoch. In that case we simply skip the update.
	//
	//common.Assert(clustermapEpoch >= mv.clustermapEpoch, clustermapEpoch, mv.clustermapEpoch, mv.rv.rvName, mv.mvName)

	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	// We should only update mvInfo with newer content.
	if clustermapEpoch <= mv.clustermapEpoch {
		log.Debug("mvInfo::updateComponentRVs: %s/%s from %s -> %s [forceUpdate: %v, epoch: %d, cepoch: %d], skipping",
			mv.rv.rvName, mv.mvName,
			rpc.ComponentRVsToString(mv.componentRVs),
			rpc.ComponentRVsToString(componentRVs),
			forceUpdate, mv.clustermapEpoch, clustermapEpoch)
		return nil
	}

	// Update must be called only to update not to add.
	common.Assert(mv.componentRVs != nil)
	common.Assert(len(mv.componentRVs) == len(componentRVs), len(mv.componentRVs), len(componentRVs))

	// TODO: check if this is safe
	// componentRVs point to a thrift req member. Does thrift say anything about safety of that,
	// or should we do a deep copy of the list.

	// mvInfo.componentRVs is a sorted list.
	sortComponentRVs(componentRVs)

	log.Debug("mvInfo::updateComponentRVs: %s/%s from %s -> %s [forceUpdate: %v, epoch: %d, cepoch: %d]",
		mv.rv.rvName, mv.mvName,
		rpc.ComponentRVsToString(mv.componentRVs),
		rpc.ComponentRVsToString(componentRVs),
		forceUpdate, mv.clustermapEpoch, clustermapEpoch)

	//
	// Catch invalid membership changes.
	//
	// Note: Cluster manager doesn't commit clustermap after the degrade-mv workflow that marks component
	//       RVs as offline, so we won't get updated offline state even after a refresh.
	//       We either let JoinMV fail in this iteration and the next time around when clustermap would have
	//       the offline state, it succeeds or we change updateMVList() to commit clustermap after marking
	//       component RVs offline.
	//
	if !forceUpdate {
		//
		// We come here only from UpdateMV(), so we only consider those state transitions as valid which
		// can practically be caused by UpdateMV. Note that UpdateMV() is called from the fix-mv workflow,
		// so its goal is to replace one or more offline RVs with new RVs in outofsync state. Though the
		// RV is offline but it may not be offline in our mvInfo, also note that mvInfo could be (very)
		// stale depending on when was the last time this mvInfo had an RPC targeted to it. In that case
		// the RVs could be in any state. We need to stick to only the following state transitions as valid:
		// - offline->outofsync (new RV replacing an offline RV)
		// To get the existing mvInfo to a state matching the clustermap, we may need to refresh it once.
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

				// Same RV same state.
				if oldState == newState {
					continue
				}

				if oldState == string(dcache.StateOffline) && newState == string(dcache.StateOutOfSync) {
					// Same RV (now online) being reused by fix-mv.
					continue
				}

				//
				// This is not really an error, but just an indication that our mvInfo is stale and must be
				// refreshed from the clustermap before again doing the check.
				//
				errStr := fmt.Sprintf("Invalid change by UpdateMV to %s/%s (%s=%s -> %s=%s), clustermapEpoch: %d",
					mv.rv.rvName, mv.mvName, oldName, oldState, oldName, newState, clustermapEpoch)
				log.Debug("mvInfo::updateComponentRVs: %s", errStr)

				//
				// We don't refresh the clustermap here, so the caller must do that.
				//
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

				if oldState == string(dcache.StateOutOfSync) && newState == string(dcache.StateOutOfSync) {
					//
					// New RV replaced by fix-mv, after the prev replacement RV went offline and had to be
					// replaced by another RV.
					//
					continue
				}

				errStr := fmt.Sprintf("Invalid change by UpdateMV to %s/%s (%s=%s -> %s=%s), clustermapEpoch: %d",
					mv.rv.rvName, mv.mvName, oldName, oldState, newName, newState, clustermapEpoch)
				log.Info("mvInfo::updateComponentRVs: %s", errStr)

				//
				// We don't refresh the clustermap here, so the caller must do that.
				//
				return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
			}
		}
	}

	// We cannot have inband offline state in the componentRVs.
	common.Assert(!containsInbandOfflineState(&componentRVs), componentRVs)

	// Valid membership changes, update the saved componentRVs.
	mv.componentRVs = componentRVs
	// We shouldn't go backwards in clustermap epoch.
	common.Assert(clustermapEpoch >= mv.clustermapEpoch, clustermapEpoch, mv.clustermapEpoch, mv.rv.rvName, mv.mvName)
	mv.clustermapEpoch = clustermapEpoch
	mv.lmt = time.Now()
	mv.lmb = senderNodeId

	common.Assert(common.IsValidUUID(mv.lmb), mv.lmb)

	return nil
}

// Update the state of the given component RV in this MV.
func (mv *mvInfo) updateComponentRVState(rvName string, oldState, newState dcache.StateEnum,
	clustermapEpoch int64, senderNodeId string) {
	common.Assert(oldState != newState &&
		cm.IsValidComponentRVState(oldState) &&
		cm.IsValidComponentRVState(newState) &&
		oldState != dcache.StateInbandOffline &&
		newState != dcache.StateInbandOffline, rvName, oldState, newState)
	common.Assert(clustermapEpoch > 0, rvName, mv.mvName, oldState, newState, clustermapEpoch)
	// We shouldn't go backwards in clustermap epoch.
	common.Assert(clustermapEpoch >= mv.clustermapEpoch, clustermapEpoch, mv.clustermapEpoch,
		rvName, mv.mvName, oldState, newState)

	mv.rwMutex.Lock()
	defer mv.rwMutex.Unlock()

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)
		if rv.Name == rvName {
			common.Assert(rv.State == string(oldState), rvName, rv.State, oldState)
			log.Debug("mvInfo::updateComponentRVState: [%s/%s] %s (%s -> %s) %s, changed by sender %s, epoch (%d -> %d)",
				mv.rv.rvName, mv.mvName, rvName, rv.State, newState, rpc.ComponentRVsToString(mv.componentRVs),
				senderNodeId, mv.clustermapEpoch, clustermapEpoch)

			rv.State = string(newState)
			mv.lmt = time.Now()
			mv.lmb = senderNodeId

			// We shouldn't go backwards in clustermap epoch.
			common.Assert(clustermapEpoch >= mv.clustermapEpoch, clustermapEpoch, mv.clustermapEpoch,
				rvName, mv.mvName, oldState, newState)

			mv.clustermapEpoch = clustermapEpoch
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

	//
	// mvInfo is always updated from our local clustermap, so its epoch must be <= cm's epoch.
	// XXX We cannot assert the following, as in JoinMV() we update mvInfo.clustermapEpoch to the epoch
	//     passed by the client, which can be greater than our local clustermap epoch.
	//
	//common.Assert(mv.clustermapEpoch <= cm.GetEpoch(), mv.clustermapEpoch, cm.GetEpoch(), mv.rv.rvName, mv.mvName)

	for _, rv := range mv.componentRVs {
		common.Assert(rv != nil)
		common.Assert(cm.IsValidComponentRVState(dcache.StateEnum(rv.State)), rv.Name, mv.mvName, rv.State)
		// We don't save inband-offline state in mvInfo, see updateInbandOfflineToOffline().
		common.Assert(rv.State != string(dcache.StateInbandOffline), rv.Name, mv.mvName, rv.State)
		common.Assert(mv.totalChunkBytes.Load() >= 0, mv.rv.rvName, mv.mvName, mv.totalChunkBytes.Load())

		// Max filesystem size for XFS is 8 EiB, ext4 is lower.
		const _8EiB = int64(8 * 1024 * 1024 * 1024 * 1024 * 1024)
		common.Assert(mv.totalChunkBytes.Load() < _8EiB, mv.rv.rvName, mv.mvName, mv.totalChunkBytes.Load())

		if rv.Name == rvName {
			return rv
		}
	}

	return nil
}

// Refresh componentRVs (name and state) for the MV, from the clustermap.
//
// Description:
// Fix-mv workflow that updates an MV's membership information (either component RVs and/or their states)
// first updates the membership in the replica's mvInfo data, by a JoinMV/UpdateMV RPC message.
// Once all involved component RVs respond with a success the sender commits the change in the clustermap.
// If one or more component RVs fail the request while some other succeed, the membership details might
// become inconsistent. Since the sender will only update the clustermap after *all* the component RVs
// respond with a success, in this case those component RVs which did make the change have information
// that is different from the clustermap. These mvInfo will have the offline RV replaced with the new
// OutOfSync replacement RV whereas the ones that failed will still have the old offline RV, which also
// matches the clustermap.
//
// Thus, an incoming request's component RVs may not match the mvInfo's component RVs for one of two reasons:
// 1. The sender has a stale clustermap.
// 2. mvInfo has inconsistent info due to the previous partially applied change.
//
// mvInfo.clustermapEpoch and clientClustermapEpoch (passed by caller) can be compared to find which of this
// is more current and which one needs a clustermap refresh.
// If server needs a refresh, refreshFromClustermap() internally calls cm.RefreshClusterMap() which will update
// the mvInfo according to the latest info from the clustermap. So, once this function returns mvInfo has updated
// membership info not older than than the clientClustermapEpoch value passed.
//
// Return values:
// - nil on success, in which case the mvInfo componentRVs are updated to match the clustermap.
//   In this case the caller can retry the operation that needed this refresh.
// - ErrorCode_NeedToRefreshClusterMap error if it didn't refresh the clustermap, for some reason.
//   This implies that refreshFromClustermap() didn't change mvInfo, so the caller must not retry the operation,
//   instead it must return this error to the client, so that the client can refresh its clustermap and retry.
//
// Caller passes clientClustermapEpoch which is the epoch of the clustermap that the client used while making
// the RPC call and is the epoch to which we must refresh our clustermap.
//
// Note: This can change mvInfo.

func (mv *mvInfo) refreshFromClustermap(cepoch int64) *models.ResponseError {
	log.Debug("mvInfo::refreshFromClustermap: %s/%s mv.componentRVs: %s, epoch: %d, cepoch: %d, sepoch: %d",
		mv.rv.rvName, mv.mvName, rpc.ComponentRVsToString(mv.componentRVs),
		mv.clustermapEpoch, cepoch, cm.GetEpoch())

	common.Assert(cepoch > 0, cepoch, mv.rv.rvName, mv.mvName)
	common.Assert(mv.clustermapEpoch > 0, mv.clustermapEpoch, mv.rv.rvName, mv.mvName)
	//
	// mvInfo is always updated from our local clustermap, so its epoch must be <= cm's epoch.
	// XXX We cannot assert the following, as in JoinMV() we update mvInfo.clustermapEpoch to the epoch
	//     passed by the client, which can be greater than our local clustermap epoch.
	//
	//common.Assert(mv.clustermapEpoch <= cm.GetEpoch(), mv.clustermapEpoch, cm.GetEpoch(), mv.rv.rvName, mv.mvName)

	//
	// The equal to in >= below is important!!
	// mv.clustermapEpoch X already contains the changes from cepoch X, and may contain some
	// ongoing changes on top (e.g., JoinMV replacing offline RV with a new outofsync RV), so we don't refresh,
	// else we risk overwriting the ongoing change.
	// Client should refresh and retry and if it's already at mv.clustermapEpoch, then it will need to wait
	// till the ongoing change completes and the clustermap epoch becomes even again.
	//
	if mv.clustermapEpoch >= cepoch {
		errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: %s/%s, epoch (%d) >= cepoch (%d), client must refresh",
			mv.rv.rvName, mv.mvName, mv.clustermapEpoch, cepoch)
		log.Debug("%s", errStr)
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	//
	// Refresh the clustermap synchronously. Once this returns, clustermap package will have the updated
	// clustermap.
	//
	err := cm.RefreshClusterMap(cepoch)
	if err != nil {
		errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: %s/%s, failed, epoch: %d, cepoch: %d: %v",
			mv.rv.rvName, mv.mvName, mv.clustermapEpoch, cepoch, err)
		log.Err("%s", errStr)
		common.Assert(false, errStr)
		//
		// ErrorCode_NeedToRefreshClusterMap can also be used to convey to the client that we ran into a
		// transient error and possibly refreshing the clustermap and retrying will help.
		//
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	// Get component RV details from the just refreshed clustermap.
	_, cmRVs, cmEpoch := cm.GetRVsEx(mv.mvName)
	if cmRVs == nil {
		errStr := fmt.Sprintf("mvInfo::refreshFromClustermap: GetRVsEx(%s/%s) failed, no such MV! epoch: %d, cepoch: %d, sepoch: %d",
			mv.rv.rvName, mv.mvName, mv.clustermapEpoch, cepoch, cm.GetEpoch())
		log.Err("%s", errStr)
		common.Assert(false, errStr)
		//
		// ErrorCode_NeedToRefreshClusterMap can also be used to convey to the client that we ran into a
		// transient error and possibly refreshing the clustermap and retrying will help.
		//
		// Note: This shouldn't happen in practice.
		//
		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	//
	// TODO: Deep copy is a temporary workaround for the bug mentioned below.
	// [BUG] If the cmRVs map is updated, it also changes the state of the component RV in the
	//       local clustermap copy, obviously it doesn't change the MV state, so this results
	//       in assert failure that claims mv state must be offline if any component RV is offline.
	//
	newRVs := deepCopyRVMap(cmRVs)

	//
	// Must have the hosting RV in the componentRVs list.
	//
	myRvInfo := mv.getComponentRVNameAndState(mv.rv.rvName)
	common.Assert(myRvInfo != nil, mv.rv.rvName, mv.mvName, rpc.ComponentRVsToString(mv.componentRVs))
	common.Assert(myRvInfo.Name == mv.rv.rvName, myRvInfo.Name, mv.rv.rvName, mv.mvName)

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
			log.Warn("mvInfo::refreshFromClustermap: %s/%s state is %s while RV state is %s, marking component RV state as offline, epoch: %d, cepoch: %d, sepoch: %d",
				rvName, mv.mvName, rvState, cm.GetRVState(rvName),
				mv.clustermapEpoch, cepoch, cmEpoch)
			rvState = dcache.StateOffline
			//
			// [BUG] This changes the state of the component RV in the local clustermap copy, obviously
			//       it doesn't change the MV state, so this results in assert failure that claims mv
			//       state must be offline if any component RV is offline.
			//
			newRVs[rvName] = rvState
		}

		newComponentRVs = append(newComponentRVs, &models.RVNameAndState{
			Name:  rvName,
			State: string(rvState),
		})
	}

	//
	// We have already waited for an even epoch on the clustermap, this means that there is no ongoing
	// fix-mv workflow that might be sending out JoinMV RPCs to add OutOfSync component RVs. IOW, the
	// clustermap state is stable (not in the middle of a change) and can be used to refresh mvInfo.
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

		log.Debug("mvInfo::refreshFromClustermap: %s/%s clustermap state (%s) != mvInfo state (%s), epoch: %d, cepoch: %d, sepoch: %d",
			mv.rv.rvName, mv.mvName, stateAsPerClustermap, myRvInfo.State,
			mv.clustermapEpoch, cepoch, cmEpoch)
	}

	if clusterMapWantsToChangeMyRV {
		common.Assert(string(stateAsPerClustermap) != myRvInfo.State,
			mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap)

		//
		// If membership info as per clustermap is different from our incore mvInfo, it might mean two
		// things:
		// 1. Our incore info is "different" due to a previous incomplete state change transaction which
		//    was not committed to the clustermap.
		// 2. mvInfo has a stale state, different from the clustermap. This is possible since clustermap
		//    is the single synchronization point that we use to coordinate state changes and when server
		//    discovers that its state is different from the clustermap, it refreshes its state from the
		//    clustermap. e.g., sync worker will change the RV state from outofsync->syncing->online, w/o
		//    explicitly letting the server know, server will find out about this change only when there
		//    is a request from a client that carries the updated component RVs list, then it'll refresh
		//    its state.
		//
		// Regardless of the case, when we transition mvInfo from one state to another, we might need to
		// perform some rollback/update actions, depending on the state transition.
		//
		if myRvInfo.State == string(dcache.StateOutOfSync) {
			//
			// mvInfo is marked OutOfSync during JoinMV call made from the fix-mv workflow.
			// During that, space is reserved in both mvInfo and rvInfo, which we have to undo once we
			// move out of OutOfSync state, except if we are moving to Syncing state, in which case the
			// actual undo will happen when we move from Syncing->Online state.
			//
			// Following are the valid transitions out of OutOfSync state:
			// - OutOfSync -> Syncing is a forward state transition triggered by the sync worker.
			// - OutOfSync -> Online is a forward state transition, happens when there are no chunks
			//                to sync, hence server doesn't notice transition to syncing state.
			// - OutOfSync -> Invalid, implies rollback for a previously incomplete fix-mv workflow.
			//
			if stateAsPerClustermap != dcache.StateSyncing {
				// OutOfSync->{Online, Invalid}, undo needed.
				log.Debug("mvInfo::refreshFromClustermap: Undoing space reserved by JoinMV, %s/%s (%s -> %s), %d bytes, epoch: %d, cepoch: %d, sepoch: %d",
					mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap,
					mv.reservedSpace.Load(),
					mv.clustermapEpoch, cepoch, cmEpoch)

				mv.rv.decReservedSpace(mv.reservedSpace.Load())
				mv.reservedSpace.Store(0)
			} else {
				// OutOfSync->Syncing, undo not needed.
				log.Debug("mvInfo::refreshFromClustermap: %s/%s (%s -> %s), reserved space is %d bytes, epoch: %d, cepoch: %d, sepoch: %d",
					mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap,
					mv.reservedSpace.Load(),
					mv.clustermapEpoch, cepoch, cmEpoch)
			}
		}

		//
		// The target component RV is set as Syncing by the sync worker, when it starts syncing chunks.
		// Note that we don't explicitly let the server know about this state change, so server doesn't
		// get a chance to change mvInfo state to Syncing. Server finds out about this state change only
		// when there is a request from a client that carries the updated component RVs list, mostly the
		// PutChunk(sync) requests, but it could be any request.
		// When there are no chunks to sync, the server may not come to know about the OutOfSync->Syncing
		// state transition, instead it may find out about the OutOfSync->Online transition if there are
		// some requests sent to the server, o/w it may not find out and will remain in OutOfSync state
		// for a long time.
		// Anyways, here we are concerned with transitions out of Syncing state, which can be:
		// - Syncing -> Online is a forward state transition, when sync completes successfully.
		// - Syncing -> Offline, if the RV goes offline/unreachable while syncing.
		// - Syncing -> OutOfSync, is not possible as OutOfSync is only set by JoinMV call when it adds
		//              a new mvInfo, here we have the mvInfo already present.
		// - Syncing -> Invalid, can happen if sync doesn't complete and then RV goes offline and it's
		//              replaced by some other RV by the fix-mv workflow.
		//
		// One thing is certain, whenever we move out of Syncing state, we must undo the reserved space.
		//
		if myRvInfo.State == string(dcache.StateSyncing) {
			// We must have reserved mv.reservedSpace in RV as well.
			common.Assert(mv.rv.reservedSpace.Load() >= mv.reservedSpace.Load(),
				mv.rv.reservedSpace.Load(), mv.reservedSpace.Load(), mv.rv.rvName,
				mv.mvName, myRvInfo.State, stateAsPerClustermap)

			if stateAsPerClustermap == dcache.StateOnline {
				//
				// As sync has completed successfully, the sync process must have written all chunks to the MV
				// replica. These must be not less than mv.reservedSpace. This is because space is reserved
				// in JoinMV call which looks at the MV size at that time. If no new writes are happening on the
				// MV, the space at the time of JoinMV is what will be used to hold all the chunks, but in case of
				// writes happening after JoinMV, we will need more space depending on how many extra client writes
				// are done.
				//
				common.Assert(mv.totalChunkBytes.Load() >= mv.reservedSpace.Load(),
					mv.rv.rvName, mv.mvName, mv.totalChunkBytes.Load(), mv.reservedSpace.Load())
			}

			log.Debug("mvInfo::refreshFromClustermap: Undoing space reserved by JoinMV, %s/%s (%s -> %s), %d bytes, epoch: %d, cepoch: %d, sepoch: %d",
				mv.rv.rvName, mv.mvName, myRvInfo.State, stateAsPerClustermap,
				mv.reservedSpace.Load(),
				mv.clustermapEpoch, cepoch, cmEpoch)

			mv.rv.decReservedSpace(mv.reservedSpace.Load())
			mv.reservedSpace.Store(0)
		}

		//
		// TODO: If an RV is being added in "outofsync" or "syncing" state (and it was in a different
		//       state earlier) we must also update rvInfo.reservedSpace.
		//
	}

	//
	// Update unconditionally, even if it may not have changed, doesn't matter.
	// We force the update as this is the membership info that we got from clustermap.
	// Note that with forceUpdate=true, updateComponentRVs() must never fail, hence the assert.
	//
	err = mv.updateComponentRVs(newComponentRVs, cmEpoch, true /* forceUpdate */, rpc.GetMyNodeUUID())
	_ = err
	common.Assert(err == nil, err)

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
	err := cm.RefreshClusterMap(0 /* targetEpoch */)
	if err != nil {
		err := fmt.Errorf("mvInfo::pruneStaleEntriesFromMvMap: %s (%d MVs) failed: %v",
			rv.rvName, rv.mvCount.Load(), err)
		log.Err("%v", err)
		common.Assert(false, err)
		return err
	}

	//
	// pruneStaleEntriesFromMvMap() is called from JoinMV() which must have the clustermap lock held,
	// which means that the clustermap epoch is odd.
	//
	common.Assert(cm.GetEpoch()%2 == 1, cm.GetEpoch())

	//
	// Go over all the MVs hosted on this RV as per our rvInfo, and for each of these MVs check clustermap
	// to see if this RV is indeed a valid component RV for the MV. If not, this is a stale mvMap entry and
	// we must remove it.
	//
	mvs := rv.getMVs()

	// Caller will call us only when it wants to prune mvMap, which means it must have entries.
	common.Assert(len(mvs) > 0, rv.rvName)

	var cmEpoch int64
	for _, mvName := range mvs {
		mv := rv.getMVInfo(mvName)

		// mvInfo is always updated from our local clustermap, so its epoch must be <= cm's epoch.
		common.Assert(mv.clustermapEpoch <= cm.GetEpoch(), mv.clustermapEpoch, cm.GetEpoch())

		//
		// Don't delete mvInfo entries added by this fix-mv run.
		// Note that a single fix-mv run can add multiple MVs to the same RV, those added before
		// this, in this same fix-mv run, will have the clustermapEpoch same as the current clustermap
		// epoch, skip them.
		//
		if mv.clustermapEpoch == cm.GetEpoch() {
			log.Debug("mvInfo::pruneStaleEntriesFromMvMap: %s/%s (%d MVs), time since lmt (%s), added in this epoch , skipping...",
				rv.rvName, mvName, rv.mvCount.Load(), time.Since(mv.lmt), mv.clustermapEpoch)
			continue
		}

		// Get component RV details for this MV from the just refreshed clustermap.
		var rvs map[string]dcache.StateEnum
		_, rvs, cmEpoch = cm.GetRVsEx(mvName)
		_ = cmEpoch
		if rvs == nil {
			err := fmt.Errorf("mvInfo::pruneStaleEntriesFromMvMap: GetRVs(%s) failed", mvName)
			log.Err("%v", err)
			//
			// This may be a JoinMV call made by the new-mv workflow.
			// The MV is still not in clustermap but rvInfo has it, ignore it.
			//
			continue
		}

		// We must be called with clustermap lock held, so epoch must not change.
		common.Assert(cmEpoch == cm.GetEpoch(), cmEpoch, cm.GetEpoch(), mvName, rv.rvName)

		//
		// Is this RV a valid component RV for this MV as per the clustermap?
		//
		rvState, ok := rvs[rv.rvName]
		if !ok {
			_ = rvState
			log.Debug("mvInfo::pruneStaleEntriesFromMvMap: deleting stale mv replica %s/%s (state: %s), epoch: %d, sepoch: %d",
				rv.rvName, mvName, rvState, mv.clustermapEpoch, cmEpoch)
			// Remove the stale MV replica.
			rv.deleteFromMVMap(mvName)
		}
	}

	// Clustermap cannot be updated while JoinMV has the lock.
	common.Assert(cm.GetEpoch()%2 == 1, cm.GetEpoch())

	log.Debug("mvInfo::pruneStaleEntriesFromMvMap: after pruning %s now hosts %d MVs, sepoch: %d",
		rv.rvName, rv.mvCount.Load(), cmEpoch)
	return nil
}

// increment the total chunk bytes for this MV
func (mv *mvInfo) incTotalChunkBytes(bytes int64) {
	mv.totalChunkBytes.Add(bytes)
	log.Debug("mvInfo::incTotalChunkBytes: totalChunkBytes for %s/%s is %d",
		mv.rv.rvName, mv.mvName, mv.totalChunkBytes.Load())
}

// decrement the total chunk bytes for this MV
func (mv *mvInfo) decTotalChunkBytes(bytes int64) {
	mv.totalChunkBytes.Add(-bytes)
	log.Debug("mvInfo::decTotalChunkBytes: totalChunkBytes for %s/%s is %d",
		mv.rv.rvName, mv.mvName, mv.totalChunkBytes.Load())
	common.Assert(mv.totalChunkBytes.Load() >= 0, mv.mvName, mv.totalChunkBytes.Load(), bytes)
}

// Check if the component RVs list in the request is valid for the given MV replica.
// componentRVsInReq corresponds to clientClustermapEpoch.
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

func (mv *mvInfo) isComponentRVsValid(componentRVsInReq []*models.RVNameAndState,
	clientClustermapEpoch int64, checkState bool) error {
	common.Assert(!containsInbandOfflineState(&componentRVsInReq), componentRVsInReq)
	common.Assert(clientClustermapEpoch > 0, clientClustermapEpoch)

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
				rpcErr := mv.refreshFromClustermap(clientClustermapEpoch)
				if rpcErr != nil {
					errStr := fmt.Sprintf("Request component RVs are invalid for MV %s,  cepoch: %d, sepoch: %d [%v]",
						mv.mvName, clientClustermapEpoch, cm.GetEpoch(), rpcErr.String())
					log.Err("ChunkServiceHandler::isComponentRVsValid: %s", errStr)
					// refreshFromClustermap() only returns ErrorCode_NeedToRefreshClusterMap.
					common.Assert(rpcErr.Code == models.ErrorCode_NeedToRefreshClusterMap, errStr)
					return rpc.NewResponseError(rpcErr.Code, errStr)
				}
				clustermapRefreshed = true
				continue
			}

			errStr := fmt.Sprintf("Request component RVs are invalid for MV %s, cepoch: %d, sepoch: %d [%v]",
				mv.mvName, clientClustermapEpoch, cm.GetEpoch(), err)
			log.Err("ChunkServiceHandler::isComponentRVsValid: %s", errStr)
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
	//    through a JoinMV call. Only after a successful JoinMV response would the caller update the MV's
	//    component RV list. If we do not have this MV added to our RV, that means we would not have
	//    responded to the JoinMV RPC, which would mean the clustermap cannot have it.
	//    For quick-restart case, NewChunkServiceHandler() will duly add all hosted MVs to the rvInfo.mvMap.
	//    For rebalancing, a component RV would be removed from an MV only after the rebalancing has
	//    completed and there's no undoing it.
	//    Other way to look at it is, if we don't have the MV directory then we do not host the MV and there's
	//    nothing we can do to read/write the chunk. The only plausible action is for the client to refresh
	//    the clustermap and retry the operation.
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
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
func readChunkAndHash(chunkPath, hashPath *string,
	readOffset int64, data *[]byte) (int /* read bytes */, string /* hash */, error) {
	var hash string

	//
	// Unless o/w specified, we do direct IO for chunk reads falling to buffered IO in case of some issue,
	// alignment issue being the most common one.
	//
	n, err := SafeRead(chunkPath, readOffset, data, rpc.ReadIOMode == rpc.BufferedIO)

	//
	// Hash file is small, perform buffered read.
	//
	if err == nil && hashPath != nil {
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

	return n, hash, err
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, req *models.GetChunkRequest) (*models.GetChunkResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Address != nil)

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::GetChunk: Received GetChunk request (%v): %v",
		rpc.ReadIOMode, rpc.GetChunkRequestToString(req))

	// Sender node id must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
	clustermapRefreshed := false

	for {
		rvNameAndState := mvInfo.getComponentRVNameAndState(rvInfo.rvName)

		// checkValidChunkAddress() had succeeded above, so RV must exist.
		common.Assert(rvNameAndState != nil)

		//
		// We allow reading only from "online" component RVs.
		// Note: Though we may be able to serve the chunk from a component RV in "syncing" or even "offline"
		//       state, it usually indicates client using an older clustermap so we rather ask the client to refresh.
		// TODO: See if going ahead and checking the chunk anyways is better.
		//
		// One example where refreshFromClustermap() will help is if the mvInfo is in "syncing" state but the
		// resync has completed since then and the sync worker has indeed moved the component RV to "online" state.
		// Since server is not explicitly notified about the state change by the sync worker, it won't know.
		// A reader can send a GetChunk request to the server, and the server can refreshFromClustermap() to find
		// that the component RV is now "online" and then serve the chunk.
		//
		if rvNameAndState.State != string(dcache.StateOnline) {
			errStr := fmt.Sprintf("GetChunk request for %s/%s cannot be satisfied in state %s [NeedToRefreshClusterMap], epoch: %d, cepoch:%d, sepoch: %d",
				rvInfo.rvName, req.Address.MvName, rvNameAndState.State,
				mvInfo.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch())
			log.Err("ChunkServiceHandler::GetChunk: %s", errStr)

			if !clustermapRefreshed {
				rpcErr := mvInfo.refreshFromClustermap(req.ClustermapEpoch)
				if rpcErr != nil {
					err1 := fmt.Errorf("ChunkServiceHandler::GetChunk: Failed to refresh clustermap, to epoch %d [%s]",
						req.ClustermapEpoch, rpcErr.String())
					log.Err("%v", err1)
					// refreshFromClustermap() only returns ErrorCode_NeedToRefreshClusterMap.
					common.Assert(rpcErr.Code == models.ErrorCode_NeedToRefreshClusterMap, err1)
					return nil, rpcErr
				}
				clustermapRefreshed = true
				continue
			}

			return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

		break
	}

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
	var data []byte
	var lmt string
	var n int
	_ = n
	var hashPathPtr *string
	var thisDuration time.Duration
	var readStartTime time.Time

	if req.IsLocalRV {
		//
		// As this call has not come through the RPC request this can be allocated from the pool, and also the buffer
		// that is allocated here would be released by the file manager after its use.
		//
		data, err = dcache.GetBuffer()
		if err != nil {
			errStr := fmt.Sprintf("failed to Allocate Buffer for chunk file %s [%v]", chunkPath, err)
			log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
		}
		// Reslice the data buffer accordingly, length of the buffer that we get from the BufferPool is of
		// maximum size(i.e., Chunk Size)
		data = data[:req.Length]

		defer func() {
			// For any error that was caused from here, We must release the buffer that was taken from the buffer pool.
			if err != nil {
				if req.IsLocalRV {
					dcache.PutBuffer(data)
				}
			}
		}()
	} else {
		//
		// local reader expects the buffer to be allocated from the pool, and it'll release the buffer
		// back to the pool. We cannot safely return cached chunk buffer in that case. Moreover, we don't
		// cache local RV reads, as they are less indicative of the chunk being hot (read by multiple nodes).
		//
		// TODO: If we use pooled allocation for non-local reads too, we will need a way to indicate that
		//       this buffer is from the cache and must not be released.
		//
		var ok bool
		if common.IsDebugBuild() {
			h.chunkCacheLookup.Add(1)
		}
		data, ok = h.chunkCache.Get(chunkPath)
		if ok {
			// We don't add metadata chunk to the cache, so we must not get it from the cache.
			common.Assert(req.Address.OffsetInMiB != dcache.MDChunkOffsetInMiB, chunkPath, req.Address.OffsetInMiB)

			if common.IsDebugBuild() {
				h.chunkCacheHit.Add(1)

				log.Debug("ChunkServiceHandler::GetChunk: Cache hit for chunk %s on %s [ %d/%d (hit rate: %.2f%%)]",
					chunkPath, rvInfo.rvName, h.chunkCacheHit.Load(), h.chunkCacheLookup.Load(),
					(float64(h.chunkCacheHit.Load())/float64(h.chunkCacheLookup.Load()))*100)
			}
			n = len(data)
			// Must be the entire exact chunk.
			common.Assert(n == int(req.Length), n, req.Length, chunkPath)
			goto cached_chunk_read
		}

		//
		// We cannot make pool allocation here, as this call has come as part of handling the RPC request.
		// TODO: Convert this to pooled allocation.
		//
		data = make([]byte, req.Length)
	}

	// Metadata chunk IOs are serialized as it's mutable unlike other data chunks.
	if req.Address.OffsetInMiB == dcache.MDChunkOffsetInMiB {
		mvInfo.mdChunkMutex.Lock()
		defer mvInfo.mdChunkMutex.Unlock()
	} else {
		// No dummy read for metadata chunk.
		if performDummyReadWrite() {
			goto dummy_read
		}
	}

	// Avoid the stats() call for release builds.
	if common.IsDebugBuild() {
		var stat syscall.Stat_t

		err = syscall.Stat(chunkPath, &stat)
		if err != nil {
			errStr := fmt.Sprintf("Failed to stat chunk file %s [%v]", chunkPath, err)
			log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
			//
			// Metadata chunk is special, caller may ask for it even before it's created.
			// Note that we create the metadata chunk on file create, so this should not happen, but
			// there's a small window between file create and metadata chunk create which can cause this.
			// See NewDcacheFile().
			//
			common.Assert(req.Address.OffsetInMiB == dcache.MDChunkOffsetInMiB, errStr)
			common.Assert(req.Length == dcache.MDChunkSize, errStr)

			return nil, rpc.NewResponseError(models.ErrorCode_ChunkNotFound, errStr)
		}

		chunkSize := stat.Size
		lmt = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec).UTC().String()

		// Again, relax the assert for metadata chunk.
		common.Assert((req.OffsetInChunk+req.Length <= chunkSize) ||
			(req.Address.OffsetInMiB == dcache.MDChunkOffsetInMiB),
			"Read beyond eof", chunkPath, req.OffsetInChunk, req.Length, chunkSize)
		log.Debug("ChunkServiceHandler::GetChunk: %s, chunkSize: %d", chunkPath, chunkSize)
	}

	//
	// TODO: hash validation will be done later
	// Only read hash if read is requested for entire chunk.
	//
	//if req.OffsetInChunk == 0 && req.Length == chunkSize {
	//	hashPathPtr := &hashPath
	//}

	readStartTime = time.Now()
	n, _, err = readChunkAndHash(&chunkPath, hashPathPtr, req.OffsetInChunk, &data)
	thisDuration = time.Since(readStartTime)

	// Consider only recent reads for calculating avg read duration.
	if NumChunkReads.Add(1) == 1000 {
		NumChunkReads.Store(1)
		AggrChunkReadsDuration.Store(thisDuration.Nanoseconds())
	} else {
		AggrChunkReadsDuration.Add(thisDuration.Nanoseconds())
	}

	if thisDuration > SlowReadWriteThreshold {
		log.Warn("[SLOW] readChunkAndHash: Slow read for %s, chunkIdx: %d, took %s (>%s), avg: %s",
			chunkPath, rpc.ChunkAddressToChunkIdx(req.Address),
			thisDuration, SlowReadWriteThreshold,
			time.Duration(AggrChunkReadsDuration.Load()/NumChunkReads.Load()))
	}

	if err != nil {
		errStr := fmt.Sprintf("failed to read chunk file %s [%v]", chunkPath, err)
		log.Err("ChunkServiceHandler::GetChunk: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
	}

	common.Assert((n == len(data)) || (n > 0 && n < len(data) && len(data) == dcache.MDChunkSize),
		fmt.Sprintf("bytes read %d is less than expected buffer size %d", n, len(data)))

	rvInfo.totalBytesRead.Add(int64(n))
	log.Info("ChunkServiceHandler::GetChunk: [STATS] chunk path %s, %s, totalBytesRead: %d ",
		chunkPath, rvInfo.rvName, rvInfo.totalBytesRead.Load())

	//
	// Don't cache local RV reads, as they are less indicative of the chunk being hot (read by multiple nodes).
	// Also don't cache metadata chunk, as it's mutable.
	//
	if !req.IsLocalRV && (req.Address.OffsetInMiB != dcache.MDChunkOffsetInMiB) {
		common.Assert(len(data) == int(req.Length), len(data), req.Length, chunkPath)
		h.chunkCache.Add(chunkPath, data)
		// Make sure LRU cache honors the size limit.
		common.Assert(h.chunkCache.Len() <= ChunkCacheSize, h.chunkCache.Len(), ChunkCacheSize, chunkPath)
	}

dummy_read:
cached_chunk_read:
	resp := &models.GetChunkResponse{
		Chunk: &models.Chunk{
			Address: req.Address,
			Data:    data[:n], // reslice to actual read length (only really needed for metadata chunk)
			Hash:    "",       // TODO: hash validation will be done later
		},
		ChunkWriteTime: lmt,
		TimeTaken:      time.Since(startTime).Microseconds(),
		ComponentRV:    mvInfo.getComponentRVs(),
	}

	return resp, nil
}

// Write chunk, safe from existing chunk file, partial writes, interrupted writes.
// If flag is set to syscall.O_DIRECT, it will perform direct write, else buffered write.
// If direct write fails with EINVAL, it will retry with buffered write.
func safeWrite(chunkPath *string, data *[]byte, flag int) error {
	common.Assert(chunkPath != nil && len(*chunkPath) > 0)
	common.Assert(data != nil && len(*data) > 0)

	tmpChunkPath := *chunkPath + ".tmp"

	// Caller wants to perform direct write, with fallback to buffered write if direct write fails with EINVAL.
	odirect := (flag & syscall.O_DIRECT) != 0

	//
	// Use O_EXCL flag just in case two writers are trying to write the same chunk simultaneously.
	// Note that for actually protecting overwriting an existing chunk we rely on the atomic rename below.
	// Rename also helps in avoiding serving a partially written chunk file.
	//
	OpenDepth.Add(1)
	fd, err := syscall.Open(tmpChunkPath, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|flag, 0400)
	OpenDepth.Add(-1)
	if err != nil {
		//
		// This is most likely open failure due to the file already existing.
		// We need to fail the call, caller will fail the client request appropriately.
		//
		err1 := fmt.Errorf("safeWrite: failed to open chunk file %s, flag: 0x%x: %w", tmpChunkPath, flag, err)
		log.Warn("%v", err1)
		return err1
	}

	deleteTmpFile := true

	defer func() {
		if deleteTmpFile {
			err := os.Remove(tmpChunkPath)
			if err != nil {
				log.Err("safeWrite: failed to remove chunk file %s: %v", tmpChunkPath, err)
			}
		}

		if fd != -1 {
			closeErr := syscall.Close(fd)
			if closeErr != nil {
				log.Err("safeWrite: failed to close chunk file %s, flag: 0x%x: %v",
					tmpChunkPath, flag, closeErr)
			}
		}
	}()

	for {
		WriteDepth.Add(1)
		n, err := syscall.Write(fd, *data)
		WriteDepth.Add(-1)
		if err == nil {
			// write should never succeed with 0 bytes written.
			common.Assert(n > 0, n, len(*data), tmpChunkPath)

			if n == len(*data) {
				//
				// Common case, written everything requested.
				// Rename the tmp chunk file to the final chunk file name.
				//
				RenameDepth.Add(1)
				renameErr := common.RenameNoReplace(tmpChunkPath, *chunkPath)
				RenameDepth.Add(-1)
				if renameErr != nil {
					err := fmt.Errorf("safeWrite: failed to rename chunk file %s to %s: %w",
						tmpChunkPath, *chunkPath, renameErr)
					log.Err("%v", err)
					return err
				}
				deleteTmpFile = false
				return nil
			} else if odirect {
				//
				// Direct write must not perform partial write, but for resilience we fallback to
				// buffered write, with a warning log to know if this happens frequently.
				//
				log.Warn("safeWrite: partial (direct) write to chunk file %s (%d of %d), retrying as buffered write",
					tmpChunkPath, n, len(*data))
				break
			}

			//
			// Partial buffered write.
			// Even this is not expected for local files, but retry the remaining write.
			// Emit a warning log to know if this happens frequently.
			//
			log.Warn("safeWrite: partial write to chunk file %s (%d of %d), retrying remaining write",
				tmpChunkPath, n, len(*data))
			*data = (*data)[n:]
			continue
		} else if errors.Is(err, syscall.EINTR) {
			log.Warn("safeWrite: write to chunk file %s (len: %d, odirect: %v) interrupted, retrying",
				tmpChunkPath, len(*data), odirect)
			continue
		} else if !odirect {
			//
			// If Write() failed for buffered write, we have no choice but to fail the call, else
			// if it fails for direct write we can retry once with buffered write.
			//
			return fmt.Errorf("safeWrite: buffered write of %d bytes to chunk file %s failed: %w",
				len(*data), tmpChunkPath, err)
		} else if !errors.Is(err, syscall.EINVAL) {
			return fmt.Errorf("safeWrite: direct write of %d bytes to chunk file %s failed: %w",
				len(*data), tmpChunkPath, err)
		}

		// For direct write failing with EINVAL, fall through to buffered write.
		log.Warn("safeWrite: direct write to chunk file %s (len: %d) failed with EINVAL, retrying with buffered write",
			tmpChunkPath, len(*data))
		break
	}

	//
	// Before retrying buffered write, we need to remove the tmp chunk file as it was created readonly.
	//
	deleteTmpFile = false
	err1 := os.Remove(tmpChunkPath)
	if err1 != nil {
		return fmt.Errorf("safeWrite: failed to remove chunk file %s: %v", tmpChunkPath, err1)
	}

	closeErr := syscall.Close(fd)
	if closeErr != nil {
		log.Err("safeWrite: failed to close chunk file %s, flag: 0x%x: %v",
			tmpChunkPath, flag, closeErr)
	}
	// defer should skip closing.
	fd = -1

	// Buffered write.
	return safeWrite(chunkPath, data, 0)
}

// Helper function to write given chunk and (optionally) the hash file.
// It performs direct or buffered write as per the configured setting or may fallback to buffered write for
// cases where direct write cannot be performed due to alignment restrictions.
func writeChunkAndHash(chunkPath, hashPath *string, data *[]byte, hash *string) error {
	var err error

	common.Assert(chunkPath != nil && len(*chunkPath) > 0)
	common.Assert(data != nil)
	common.Assert(len(*data) > 0)
	common.Assert(hashPath == nil || (len(*hashPath) > 0 && hash != nil && len(*hash) > 0), hashPath, hash)

	writeLength := len(*data)

	//
	// Caller must pass data buffer aligned on FS_BLOCK_SIZE, else we have to unnecessarily perform buffered write.
	// For writes we always allocate chunk sized buffers so buffer must be aligned to FS_BLOCK_SIZE.
	//
	dataAddr := unsafe.Pointer(&(*data)[0])
	isDataBufferAligned := ((uintptr(dataAddr) % common.FS_BLOCK_SIZE) == 0)
	common.Assert(isDataBufferAligned, uintptr(dataAddr), writeLength, common.FS_BLOCK_SIZE)

	//
	// Write the chunk using buffered IO mode if,
	//   - Write IO type is configured as BufferedIO, or
	//   - The write length (or chunk size) is not aligned to file system block size.
	//   - The buffer is not aligned to file system block size.
	//
	bufferedWrite := false

	if rpc.WriteIOMode == rpc.BufferedIO ||
		writeLength%common.FS_BLOCK_SIZE != 0 ||
		!isDataBufferAligned {
		bufferedWrite = true

		// Warn if we are doing buffered write for a large chunk due to unaligned buffer.
		if rpc.WriteIOMode != rpc.BufferedIO && (writeLength >= (1024 * 1024)) && !isDataBufferAligned {
			log.Warn("writeChunkAndHash: Performing buffered write for chunk %s, length: %d",
				*chunkPath, writeLength)
		}
	}

	if !bufferedWrite {
		// Direct IO write.
		err = safeWrite(chunkPath, data, syscall.O_DIRECT)
	} else {
		// Buffered wriyte.
		err = safeWrite(chunkPath, data, 0)
	}

	if err != nil {
		//
		// If chunk file already exists, we don't delete the file.
		// The caller checks that if it is a sync write or "MaybeOverwrite" flag is set in request,
		// then it ignores the write. Else it returns error back to the client.
		//
		// TODO: Make sure the hash file is also present and valid.
		//
		if errors.Is(err, syscall.EEXIST) {
			log.Debug("ChunkServiceHandler::writeChunkAndHash: Chunk file %s already exists [%v]",
				*chunkPath, err)
			return err
		}

		goto cleanup_chunk_file_and_fail
	}

	//
	// Write hash file after successful chunk file write.
	// Hash file is small, perform buffered write.
	//
	if hashPath != nil {
		err = os.WriteFile(*hashPath, []byte(*hash), 0400)
		if err != nil {
			err = fmt.Errorf("failed to write hash file %s [%v]", *hashPath, err)
			goto cleanup_chunk_file_and_fail
		}
	}

	return nil

cleanup_chunk_file_and_fail:
	// Remove chunk file, to avoid confusion later.
	log.Debug("ChunkServiceHandler::writeChunkAndHash: Removing chunk file %s", *chunkPath)

	err1 := os.Remove(*chunkPath)
	if err1 != nil {
		log.Warn("ChunkServiceHandler::writeChunkAndHash: Failed to remove chunk file %s [%v]",
			*chunkPath, err1)
	}

	if hashPath != nil {
		log.Debug("ChunkServiceHandler::writeChunkAndHash: Removing hash file %s", *hashPath)

		err1 := os.Remove(*hashPath)
		if err1 != nil {
			log.Warn("ChunkServiceHandler::writeChunkAndHash: Failed to remove hash file %s [%v]",
				*hashPath, err1)
		}
	}

	return err
}

// Check if the given PutChunkRequest from client is compatible with this mvInfo.
// A request is compatible if client's notion of which MV replicas it must write and which it must skip
// matches with this mvInfo's component RV list and their states, i.e., either the client writes to an MV
// replica or it is guaranteed by the current state of the component RVs that the chunk will be resynced
// later. We MUST NEVER have a situation where client skips writing to an MV replica hoping that it'll
// be sync'ed later but it is not sync'ed because the sync process already started and skipped that chunk.
//
// A simple (and rugged) check is to make sure that client's component RV list and this mvInfo's component RV
// list exactly match, i.e., they have the same RVs with same states. In case of any difference, server and/or
// client must refresh their clustermap, depending on who has the stale clustermap.
// Note that this should work since any global change is synchronized through the clustermap.
//
// For PutChunk(sync) requests, we can have more relaxed checks as the sync process is only concerned about a
// single MV replica (the target of the sync) and the source RV. We just need to ensure that both are still
// part of the MV and the source RV is online while target RV is syncing. We don't check the other component RV(s)
// as they are not involved in the sync process and we want to allow sync process to continue even while other
// RVs may be replaced by other outofsync RVs and later changed to syncing when their sync process starts and
// eventually to online. This allows multiple sync processes to proceed in parallel without stepping on each
// other.

func (mv *mvInfo) isClientPutChunkRequestCompatible(req *models.PutChunkRequest) error {
	log.Debug("ChunkServiceHandler::isClientPutChunkRequestCompatible: Request: %v, mvInfo: {%s/%s, componentRVs: %s}, epoch: %d, cepoch: %d, sepoch: %d",
		rpc.PutChunkRequestToString(req), mv.rv.rvName, mv.mvName, rpc.ComponentRVsToString(mv.componentRVs),
		mv.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch())

	clustermapRefreshed := false

refreshFromClustermapAndRetry:
	if common.IsDebugBuild() {
		componentRVsInMV := mv.getComponentRVs()
		common.Assert(len(req.ComponentRV) == len(componentRVsInMV),
			len(req.ComponentRV), len(componentRVsInMV))
	}

	if len(req.SyncID) == 0 {
		//
		// PutChunk(client) - Make sure client's component RV list and states exactly match with ours.
		//
		// TODO: Need to support cases where client has an offline RV while server has another RV which is
		//       in outofsync state. Without this we will work fine but for the case where a node dies
		//       while it was running the fix-mv workflow and the clustermap epoch is stuck at an odd
		//       value.
		//
		for _, rv := range req.ComponentRV {
			common.Assert(rv != nil)

			// Component RV details from mv.
			rvNameAndState := mv.getComponentRVNameAndState(rv.Name)

			// Common case, all RVs and their states match.
			if rvNameAndState != nil && rv.State == rvNameAndState.State {
				continue
			}

			var errStr string
			if rvNameAndState == nil {
				errStr = fmt.Sprintf("PutChunk(client) -> %s/%s, sender (%s) has a non-existent component RV %s/%s, epoch: %d, cepoch: %d, sepoch: %d [NeedToRefreshClusterMap]: %s",
					mv.rv.rvName, req.Chunk.Address.MvName, req.SenderNodeID,
					rv.Name, req.Chunk.Address.MvName,
					mv.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch(),
					rpc.PutChunkRequestToString(req))
				log.Err("ChunkServiceHandler::isClientPutChunkRequestCompatible: %s", errStr)
			} else {
				errStr = fmt.Sprintf("PutChunk(client) -> %s/%s, sender (%s) RV %s/%s state (%s) != mvInfo state (%s), epoch: %d, cepoch: %d, sepoch: %d [NeedToRefreshClusterMap]: %s",
					mv.rv.rvName, req.Chunk.Address.MvName, req.SenderNodeID,
					rv.Name, req.Chunk.Address.MvName,
					rv.State, rvNameAndState.State,
					mv.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch(),
					rpc.PutChunkRequestToString(req))
				log.Err("ChunkServiceHandler::isClientPutChunkRequestCompatible: %s", errStr)
			}

			if !clustermapRefreshed {
				rpcErr := mv.refreshFromClustermap(req.ClustermapEpoch)
				if rpcErr != nil {
					err1 := fmt.Errorf("ChunkServiceHandler::isClientPutChunkRequestCompatible: Failed to refresh clustermap, to epoch %d [%s]",
						req.ClustermapEpoch, rpcErr.String())
					log.Err("%v", err1)
					// refreshFromClustermap() only returns ErrorCode_NeedToRefreshClusterMap.
					common.Assert(rpcErr.Code == models.ErrorCode_NeedToRefreshClusterMap, err1)
					return rpcErr
				}
				clustermapRefreshed = true
				goto refreshFromClustermapAndRetry
			}

			return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}
	} else {
		//
		// PutChunk(sync) - Make sure the source and target MV replica match (both name and state).
		//
		sourceOK := true
		targetOK := true
		var errStr string
		for _, rv := range req.ComponentRV {
			common.Assert(rv != nil)

			// Source RV must be present in both and must be online.
			if rv.Name == req.SourceRVName {
				rvNameAndState := mv.getComponentRVNameAndState(rv.Name)
				if rvNameAndState == nil ||
					rv.State != string(dcache.StateOnline) ||
					rvNameAndState.State != string(dcache.StateOnline) {
					sourceOK = false
					errStr = fmt.Sprintf("PutChunk(sync) -> %s/%s, sender (%s) has a bad source RV %s/%s, epoch: %d, cepoch: %d, sepoch: %d [NeedToRefreshClusterMap]: %s vs %s",
						mv.rv.rvName, req.Chunk.Address.MvName, req.SenderNodeID,
						rv.Name, req.Chunk.Address.MvName,
						mv.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch(),
						rpc.PutChunkRequestToString(req),
						rpc.ComponentRVsToString(mv.getComponentRVs()))
					log.Err("ChunkServiceHandler::isClientPutChunkRequestCompatible: %s", errStr)
					break
				}
			} else if rv.Name == mv.rv.rvName {
				// Target RV must be present in both and must be syncing.
				rvNameAndState := mv.getComponentRVNameAndState(rv.Name)
				if rvNameAndState == nil ||
					rv.State != string(dcache.StateSyncing) ||
					rvNameAndState.State != string(dcache.StateSyncing) {
					targetOK = false
					errStr = fmt.Sprintf("PutChunk(sync) -> %s/%s, sender (%s) has a bad target RV %s/%s, epoch: %d, cepoch: %d, sepoch: %d [NeedToRefreshClusterMap]: %s vs %s",
						mv.rv.rvName, req.Chunk.Address.MvName, req.SenderNodeID,
						rv.Name, req.Chunk.Address.MvName,
						mv.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch(),
						rpc.PutChunkRequestToString(req),
						rpc.ComponentRVsToString(mv.getComponentRVs()))
					log.Err("ChunkServiceHandler::isClientPutChunkRequestCompatible: %s", errStr)
					break
				}
			}
		}

		if sourceOK && targetOK {
			return nil
		}

		if !clustermapRefreshed {
			rpcErr := mv.refreshFromClustermap(req.ClustermapEpoch)
			if rpcErr != nil {
				err1 := fmt.Errorf("ChunkServiceHandler::isClientPutChunkRequestCompatible: Failed to refresh clustermap, to epoch %d [%s]",
					req.ClustermapEpoch, rpcErr.String())
				log.Err("%v", err1)
				// refreshFromClustermap() only returns ErrorCode_NeedToRefreshClusterMap.
				common.Assert(rpcErr.Code == models.ErrorCode_NeedToRefreshClusterMap, err1)
				return rpcErr
			}
			clustermapRefreshed = true
			goto refreshFromClustermapAndRetry
		}

		return rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	return nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, req *models.PutChunkRequest) (*models.PutChunkResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Thrift should not be calling us with nil Address.
	common.Assert(req.Chunk != nil)
	common.Assert(req.Chunk.Address != nil)
	common.Assert(req.Length == int64(len(req.Chunk.Data)), req.Length, len(req.Chunk.Data))

	startTime := time.Now()

	log.Debug("ChunkServiceHandler::PutChunk: Received PutChunk request (%v): %v",
		rpc.WriteIOMode, rpc.PutChunkRequestToString(req))

	// Sender node id must be valid.
	common.Assert(common.IsValidUUID(req.SenderNodeID), req.SenderNodeID)
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
	// This is done because we don't allow inband-offline state in the mvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	//
	// Do not allow incompatible PutChunk requests from client.
	//
	err = mvInfo.isClientPutChunkRequestCompatible(req)
	if err != nil {
		return nil, err
	}

	// TODO: check later if lock is needed
	// acquire lock for the chunk address to prevent concurrent writes
	// chunkAddress := getChunkAddress(req.Chunk.Address.FileID, req.Chunk.Address.RvID, req.Chunk.Address.MvName, req.Chunk.Address.OffsetInMiB)
	// flock := h.locks.Get(chunkAddress)
	// flock.Lock()
	// defer flock.Unlock()

	cacheDir := rvInfo.cacheDir

	var chunkPath, hashPath string
	_ = hashPath

	//
	// In both client as well as sync write PutChunk calls,
	// the chunks must be written to the mv directory, i.e. rv0/mv0.
	//
	chunkPath, hashPath = getChunkAndHashPath(cacheDir, req.Chunk.Address.MvName,
		req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB)

	log.Debug("ChunkServiceHandler::PutChunk: chunk path %s, hash path %s", chunkPath, hashPath)

	var availableSpace int64
	var thisDuration time.Duration
	var writeStartTime time.Time

	//
	// If the PutChunk is for the special metadata chunk, remove existing metadata chunk file if any, to
	// be able to write the new metadata chunk.
	//
	// TODO: See if we should write the metadata chunk with writeable permissions so that we can overwrite it
	//       without needing to delete it first. Metadata chunk write should be very infrequent so this is not
	//       a big deal.
	//
	if req.Chunk.Address.OffsetInMiB == dcache.MDChunkOffsetInMiB {
		//
		// Metadata chunk can be written multiple times and even simultaneously from different threads, so
		// we need to serialize in order to update the totalChunkBytes correctly.
		//
		mvInfo.mdChunkMutex.Lock()
		defer mvInfo.mdChunkMutex.Unlock()

		common.Assert(req.Length < dcache.MDChunkSize, req.Length, dcache.MDChunkSize, chunkPath)

		info, err1 := os.Stat(chunkPath)
		if err1 != nil && !os.IsNotExist(err1) {
			errStr := fmt.Sprintf("failed to stat metadata chunk file %s before deleting [%v]", chunkPath, err1)
			log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
			common.Assert(false, errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
		}

		if err == nil && info != nil {
			err1 = os.Remove(chunkPath)
			if err1 != nil {
				errStr := fmt.Sprintf("failed to remove metadata chunk file %s before writing [%v]", chunkPath, err1)
				log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
				// Stat() just returned success, so the file must be present.
				common.Assert(!os.IsNotExist(err1), errStr)
				return nil, rpc.NewResponseError(models.ErrorCode_InternalServerError, errStr)
			}

			// Successfully deleted the metadata chunk, update the mvInfo accounting.
			common.Assert(info.Size() > 0 && info.Size() < dcache.MDChunkSize,
				info.Size(), dcache.MDChunkSize, chunkPath)
			mvInfo.decTotalChunkBytes(info.Size())
		}
	} else {
		// No dummy write for metadata chunk.
		if performDummyReadWrite() {
			goto dummy_write
		}
	}

	// TODO: hash validation will be done later
	writeStartTime = time.Now()
	rvInfo.wqsize.Add(1)
	err = writeChunkAndHash(&chunkPath, nil /* &hashPath */, &req.Chunk.Data, &req.Chunk.Hash)
	common.Assert(rvInfo.wqsize.Load() >= 0, rvInfo.wqsize.Load())
	rvInfo.wqsize.Add(-1)
	thisDuration = time.Since(writeStartTime)

	// Consider only recent writes for calculating avg write duration.
	if NumChunkWrites.Add(1) == 1000 {
		NumChunkWrites.Store(1)
		AggrChunkWritesDuration.Store(thisDuration.Nanoseconds())
	} else {
		AggrChunkWritesDuration.Add(thisDuration.Nanoseconds())
	}

	CumChunkWrites.Add(1)
	CumBytesWritten.Add(int64(len(req.Chunk.Data)))

	// Too many outstanding writes to a disk can make the writes very slow, alert to know that.
	if thisDuration > SlowReadWriteThreshold {
		log.Warn("[SLOW] writeChunkAndHash: Slow write for %s, chunkIdx: %d, took %s (>%s), avg: %s, cum: {%d, %d}, iodepth: %d, openDepth: %d, writeDepth: %d, renameDepth: %d",
			chunkPath, rpc.ChunkAddressToChunkIdx(req.Chunk.Address),
			thisDuration, SlowReadWriteThreshold,
			time.Duration(AggrChunkWritesDuration.Load()/NumChunkWrites.Load()),
			CumChunkWrites.Load(), CumBytesWritten.Load(), rvInfo.wqsize.Load(),
			OpenDepth.Load(), WriteDepth.Load(), RenameDepth.Load())
	}

	if err != nil {
		//
		// Chunk file must not be present, unless it is either a sync write or client has retried the write
		// after some failure in an earlier attempt.
		//
		if errors.Is(err, syscall.EEXIST) {
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
					log.Err("ChunkServiceHandler::PutChunk: syncID = %s, Failed to get available disk space, using availableSpace as 0: [%v]",
						req.SyncID, err)
					availableSpace = 0
				}

				return &models.PutChunkResponse{
					TimeTaken:      time.Since(startTime).Microseconds(),
					Qsize:          rvInfo.wqsize.Load(),
					AvailableSpace: availableSpace,
					ComponentRV:    mvInfo.getComponentRVs(),
				}, nil
			} else {
				errStr := fmt.Sprintf("Chunk file %s already exists", chunkPath)
				log.Err("ChunkServiceHandler::PutChunk: %s", errStr)
				return nil, rpc.NewResponseError(models.ErrorCode_ChunkAlreadyExists, errStr)
			}
		}

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
	// As a new chunk is written, update the MV replica's total chunk bytes.
	// We do it for both PutChunk(client) as well as PutChunk(sync) writes so that this reflects the true MV size
	// at all times. Note that we don't decrement mvInfo.reservedSpace up until EndSync(), so mvInfo.reservedSpace
	// + mvInfo.totalChunkBytes will account for more space than what will be taken up after the sync completes.
	//
	if len(req.SyncID) == 0 {
		mvInfo.incTotalChunkBytes(req.Length)
	} else {
		// JoinMV would have reserved this space before starting sync.
		// TODO: [Tomar] I've seen this assert fail and also some other places where we assert for reservedSpace
		//       panic: Assertion failed: [13091 4194304]
		//       The reservedSpace update possibly has some race.
		//       One likely possibility is that when we called GetMVSize() from JoinMV, there were more chunks
		//       written to the source MV replica after we read the mvInfo.totalChunkBytes, so we reserved less
		//       but actually sync'ed more. It's not a big deal as we will differ only slightly.
		common.Assert(rvInfo.reservedSpace.Load() >= req.Length,
			rvInfo.reservedSpace.Load(), req.Length, rvInfo.rvName, mvInfo.mvName, req.SyncID)
		common.Assert(rvInfo.reservedSpace.Load() >= mvInfo.reservedSpace.Load(),
			rvInfo.reservedSpace.Load(), mvInfo.reservedSpace.Load(), rvInfo.rvName, mvInfo.mvName, req.SyncID)

		mvInfo.incTotalChunkBytes(req.Length)
	}

dummy_write:
	availableSpace, err = rvInfo.getAvailableSpace()
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to get available disk space [%v]", err)
	}

	resp := &models.PutChunkResponse{
		TimeTaken:      time.Since(startTime).Microseconds(),
		Qsize:          rvInfo.wqsize.Load(),
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

	// Client must send a valid clustermap epoch.
	common.Assert(req.Request.ClustermapEpoch > 0, req.Request.ClustermapEpoch)

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
	// PutChunkRequest must have a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
		Length:          req.Length,
		SyncID:          req.SyncID,
		ComponentRV:     req.ComponentRV,
		MaybeOverwrite:  req.MaybeOverwrite,
		ClustermapEpoch: req.ClustermapEpoch,
	}

	//
	// This is the last RV in the list, so we will call PutChunk directly on it.
	// Else, we will call PutChunkDC on it with the next RVs in the list.
	//
	if len(nextRVs) == 0 {
		log.Debug("ChunkServiceHandler::forwardPutChunk: Forwarding PutChunk request to last RV %s/%s on node %s: %s",
			nexthopRV, req.Chunk.Address.MvName, nexthopNodeId, rpc.PutChunkRequestToString(putChunkReq))

		var rpcErr *models.ResponseError

		putChunkResp, err := rpc_client.PutChunk(ctx, nexthopNodeId, putChunkReq, true /* fromFwder */)
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

		dcResp, err := rpc_client.PutChunkDC(ctx, nexthopNodeId, putChunkDCReq, true /*fromFwder */)

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
			log.Debug("ChunkServiceHandler::forwardPutChunk: Received response from nexthop %s/%s (file id %s, offset in MiB %d, chunkIdx: %d): %s",
				nexthopRV, req.Chunk.Address.MvName, req.Chunk.Address.FileID, req.Chunk.Address.OffsetInMiB,
				rpc.ChunkAddressToChunkIdx(req.Chunk.Address),
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
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
	// This is done because we don't allow inband-offline state in the mvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	// Validate the component RVs list.
	err = mvInfo.isComponentRVsValid(req.ComponentRV, req.ClustermapEpoch, true /* checkState */)
	if err != nil {
		errStr := fmt.Sprintf("Component RVs are invalid for MV %s [%v]", req.Address.MvName, err)
		log.Err("ChunkServiceHandler::RemoveChunk: %s", errStr)
		return nil, err
	}

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

	log.Debug("ChunkServiceHandler::RemoveChunk: Deleted %d chunks from %s", numChunksDeleted, mvDir)
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
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	// JoinMV is called by updateMVList() which holds the clustermap lock, so epoch must be odd.
	common.Assert(req.ClustermapEpoch%2 == 1, req.ClustermapEpoch)
	// JoinMV is called after fetching the latest clustermap and bumping the epoch, so it must be the max seen.
	common.Assert(req.ClustermapEpoch >= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

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
	var totalChunkBytes int64
	if mvInfo != nil {
		//
		// Fail any attempt by client to push an older clustermap epoch with NeedToRefreshClusterMap error.
		// This should never happen unless the client is misbehaving.
		// If it happens, joinMV() will fail in the client, since client doesn't retry joinMV() on failure,
		// it will try with another RV.
		//
		// Note: req.ClustermapEpoch will mostly be greater than mvInfo.clustermapEpoch, the only case when
		//       req.ClustermapEpoch will be equal to mvInfo.clustermapEpoch is when GetMVSize() (called right
		//       before JoinMV in fixMV() refreshes mvInfo from clustermap.
		//
		if req.ClustermapEpoch < mvInfo.clustermapEpoch {
			errStr := fmt.Sprintf("[CLUSTERMAP EPOCH RENEGE] ChunkServiceHandler::JoinMV: for %s/%s, last updated at %s (%s ago), by %s, from (%d -> %d)",
				mvInfo.rv.rvName, mvInfo.mvName, mvInfo.lmt, time.Since(mvInfo.lmt), mvInfo.lmb,
				mvInfo.clustermapEpoch, req.ClustermapEpoch)
			log.Err("%s", errStr)
			common.Assert(false, errStr)
			// TODO: Return mvInfo.clustermapEpoch in the RPC response asking client to refresh to at least that.
			return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

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
		errStr := fmt.Sprintf("JoinMV for existing MV replica %s/%s, mvInfo last updated at %s (%s ago), by %s, totalChunkBytes: %d, epoch (%d -> %d)",
			req.RVName, req.MV, mvInfo.lmt, time.Since(mvInfo.lmt), mvInfo.lmb,
			mvInfo.totalChunkBytes.Load(), mvInfo.clustermapEpoch, req.ClustermapEpoch)

		log.Warn("ChunkServiceHandler::JoinMV: %s", errStr)

		//
		// Refresh our mvInfo state as per the latest clustermap.
		// This will undo the changes made by the prev incomplete JoinMV, updating mvInfo as if the
		// previous JoinMV never happened. If refreshFromClustermap() fails, we cannot safely proceed.
		// For newMV, we won't have the MV in clustermap yet, so no need to refresh.
		//
		if !newMV {
			rpcErr := mvInfo.refreshFromClustermap(req.ClustermapEpoch)
			if rpcErr != nil {
				errStr = fmt.Sprintf("%s, refreshFromClustermap() failed, aborting JoinMV: %s",
					errStr, rpcErr.String())
				log.Err("ChunkServiceHandler::JoinMV: %s", errStr)
				return nil, rpc.NewResponseError(rpcErr.Code, errStr)
			}
		}

		//
		// Remove the MV replica, we will add a fresh one later down.
		// We need to initialize totalChunkBytes for the new mv replica to the value from the old mv replica,
		// since that's the actual space used by the MV on this RV.
		//
		totalChunkBytes = mvInfo.totalChunkBytes.Load()
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
	// This is done because we don't allow inband-offline state in the mvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	rvInfo.addToMVMap(req.MV, newMVInfo(rvInfo, req.MV, req.ComponentRV, req.ClustermapEpoch, totalChunkBytes,
		req.SenderNodeID), req.ReserveSpace)

	return &models.JoinMVResponse{}, nil
}

func (h *ChunkServiceHandler) UpdateMV(ctx context.Context, req *models.UpdateMVRequest) (*models.UpdateMVResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)
	// UpdateMV is called by updateMVList() which holds the clustermap lock, so epoch must be odd.
	common.Assert(req.ClustermapEpoch%2 == 1, req.ClustermapEpoch)
	// UpdateMV is called after fetching the latest clustermap and bumping the epoch, so it must be the max seen.
	common.Assert(req.ClustermapEpoch >= cm.GetEpoch(), req.ClustermapEpoch, cm.GetEpoch())

	log.Debug("ChunkServiceHandler::UpdateMV: Received UpdateMV request: %v", rpc.UpdateMVRequestToString(req))

	if !common.IsValidUUID(req.SenderNodeID) ||
		!cm.IsValidMVName(req.MV) ||
		!cm.IsValidRVName(req.RVName) ||
		len(req.ComponentRV) == 0 {
		errStr := fmt.Sprintf("Invalid SenderNodeID, MV, RV or ComponentRV: %v", rpc.UpdateMVRequestToString(req))
		log.Err("ChunkServiceHandler::UpdateMV: %s", errStr)
		return nil, rpc.NewResponseError(models.ErrorCode_InvalidRequest, errStr)
	}

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

	//
	// Fail any attempt by client to push an older clustermap epoch with NeedToRefreshClusterMap error.
	// UpdateMV must carry a strictly newer clustermap epoch since client takes the clustermap lock which
	// updates the epoch before calling UpdateMV.
	// This should never happen unless the client is misbehaving.
	// If it happens, joinMV() will fail in the client, since client doesn't retry joinMV() on failure,
	// it will try with another RV.
	//
	// Note: req.ClustermapEpoch will mostly be greater than mvInfo.clustermapEpoch, the only case when
	//       req.ClustermapEpoch will be equal to mvInfo.clustermapEpoch is when GetMVSize() (called right
	//       before JoinMV in fixMV() refreshes mvInfo from clustermap.
	//
	if req.ClustermapEpoch < mvInfo.clustermapEpoch {
		errStr := fmt.Sprintf("[CLUSTERMAP EPOCH RENEGE] ChunkServiceHandler::UpdateMV: for %s/%s, last updated at %s (%s ago), by %s, from (%d -> %d)",
			mvInfo.rv.rvName, mvInfo.mvName, mvInfo.lmt, time.Since(mvInfo.lmt), mvInfo.lmb,
			mvInfo.clustermapEpoch, req.ClustermapEpoch)
		log.Err("%s", errStr)
		common.Assert(false, errStr)
		// TODO: Return mvInfo.clustermapEpoch in the RPC response asking client to refresh to at least that.
		return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
	}

	//
	// If the component RVs list has any RV with inband-offline state, update it to offline.
	// This is done because we don't allow inband-offline state in the mvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	clustermapRefreshed := false

	for {
		componentRVsInMV := mvInfo.getComponentRVs()
		_ = componentRVsInMV

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
		//       already marked syncing, but now it has rv as outofsync and it forces it as that
		// Update: Now we are checking clustermap epoch and rejecting any attempt to push an older epoch, so
		//       this should be safe.
		//
		err := mvInfo.updateComponentRVs(req.ComponentRV, req.ClustermapEpoch, false /* forceUpdate */, req.SenderNodeID)
		if err != nil {
			if !clustermapRefreshed {
				rpcErr := mvInfo.refreshFromClustermap(req.ClustermapEpoch)
				if rpcErr != nil {
					log.Err("ChunkServiceHandler::UpdateMV: Failed to refresh clustermap, to epoch %d [%s]",
						req.ClustermapEpoch, rpcErr.String())
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
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
	// This is done because we don't allow inband-offline state in the mvInfo.
	//
	updateInbandOfflineToOffline(&req.ComponentRV)

	// validate the component RVs list
	err := mvInfo.isComponentRVsValid(req.ComponentRV, req.ClustermapEpoch, true /* checkState */)
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

func (h *ChunkServiceHandler) GetMVSize(ctx context.Context, req *models.GetMVSizeRequest) (*models.GetMVSizeResponse, error) {
	// Thrift should not be calling us with nil req.
	common.Assert(req != nil)
	// Client must send a valid clustermap epoch.
	common.Assert(req.ClustermapEpoch > 0, req.ClustermapEpoch)

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
	// Caller will only call GetMVSize on online MV replicas, but our mvInfo may not be refreshed yet,
	// so refresh from clustermap and even then if the MV replica is not online, ask client to refresh
	// and retry.
	//
	clustermapRefreshed := false
	for {
		myRvInfo := mvInfo.getComponentRVNameAndState(req.RVName)
		common.Assert(myRvInfo != nil, mvInfo.rv.rvName, mvInfo.mvName, rpc.ComponentRVsToString(mvInfo.componentRVs))
		common.Assert(myRvInfo.Name == mvInfo.rv.rvName, myRvInfo.Name, mvInfo.rv.rvName, mvInfo.mvName)

		// Happy path, MV replica is online.
		if myRvInfo.State == string(dcache.StateOnline) {
			break
		}

		if clustermapRefreshed {
			errStr := fmt.Sprintf("GetMVSize() called on component RV %s/%s which is not online (is %s), epoch: %d, cepoch: %d, sepoch: %d",
				mvInfo.rv.rvName, mvInfo.mvName, myRvInfo.State,
				mvInfo.clustermapEpoch, req.ClustermapEpoch, cm.GetEpoch())
			log.Info("ChunkServiceHandler::GetMVSize: %s", errStr)
			return nil, rpc.NewResponseError(models.ErrorCode_NeedToRefreshClusterMap, errStr)
		}

		rpcErr := mvInfo.refreshFromClustermap(req.ClustermapEpoch)
		if rpcErr != nil {
			log.Err("ChunkServiceHandler::GetMVSize: Failed to refresh clustermap, to epoch %d [%s]",
				req.ClustermapEpoch, rpcErr.String())
			return nil, rpcErr
		}

		clustermapRefreshed = true
		continue
	}

	//
	// GetMVSize is only called for online MV replicas, for which reservedSpace should be 0.
	//
	common.Assert(mvInfo.reservedSpace.Load() == 0, rvInfo.rvName, req.MV, mvInfo.reservedSpace.Load())

	resp := &models.GetMVSizeResponse{
		MvSize: mvInfo.totalChunkBytes.Load(),
	}

	log.Debug("ChunkServiceHandler::GetMVSize: Returning size %d for %s/%s",
		resp.MvSize, rvInfo.rvName, req.MV)

	return resp, nil
}

// Silence unused import errors for release builds.
func init() {
	slices.Contains([]int{0}, 0)
}
