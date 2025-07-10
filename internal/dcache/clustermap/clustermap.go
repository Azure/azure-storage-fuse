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

package clustermap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

//go:generate $ASSERT_REMOVER $GOFILE

func Start() {
	// This MUST match localClusterMapPath in clustermanager.
	clusterMap.localClusterMapPath = filepath.Join(common.DefaultWorkDir, "clustermap.json")
}

func Stop() {
	clusterMap.stop()
}

// Update will load the local clustermap.
func Update() {
	clusterMap.loadLocalMap()
}

// Return Epoch value of the cached clustermap.
func GetEpoch() int64 {
	return clusterMap.getEpoch()
}

// It will return online MVs Map <mvName, MV> as per local cache copy of cluster map.
func GetActiveMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getActiveMVs()
}

// It will return degraded MVs Map <mvName, MV> as per local cache copy of cluster map.
func GetDegradedMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getDegradedMVs()
}

// It will return syncable MVs Map <mvName, MV> as per local cache copy of cluster map.
// syncable MVs are those degraded MVs which have at least one component RV in outofsync state.
// Degraded MVs with no outofsync (only offline) RVs need to be first fixed by fix-mv before they
// can be sync'ed.
func GetSyncableMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getSyncableMVs()
}

// It will return offline MVs Map <mvName, MV> as per local cache copy of cluster map.
func GetOfflineMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getOfflineMVs()
}

// It will return the cache config as per local cache copy of cluster map.
func GetCacheConfig() *dcache.DCacheConfig {
	return clusterMap.getCacheConfig()
}

// It will return the clustermap per local cache copy of it.
func GetClusterMap() dcache.ClusterMap {
	return clusterMap.getClusterMap()
}

// It will return all the RVs Map <rvName, RV> for this particular node as per local cache copy of cluster map.
func GetMyRVs() map[string]dcache.RawVolume {
	return clusterMap.getMyRVs()
}

// It will return the RVs Map <rvName, RV> as per local cache copy of cluster map.
func GetAllRVs() map[string]dcache.RawVolume {
	return clusterMap.getAllRVs()
}

// Is rvName hosted on this node.
func IsMyRV(rvName string) bool {
	return clusterMap.isMyRV(rvName)
}

// It will return all the RVs Map <rvName, rvState> for the particular mvName as per local cache copy of cluster map.
func GetRVs(mvName string) map[string]dcache.StateEnum {
	return clusterMap.getRVs(mvName)
}

// Same as GetRVs() but also returns the MV state and clusterMap epoch that corresponds to the component RVs returned.
// Useful for callers who might want to refresh the clusterMap on receiving the NeedToRefreshClusterMap error
// from the server. They can refresh the clusterMap till they get a higher epoch value than the one corresponding
// to the component RVs which were dismissed by the server.
func GetRVsEx(mvName string) (dcache.StateEnum, map[string]dcache.StateEnum, int64) {
	return clusterMap.getRVsEx(mvName)
}

// Return the state of the given RV from the local cache copy of cluster map.
func GetRVState(rvName string) dcache.StateEnum {
	return clusterMap.getRVState(rvName)
}

// It will check if the given nodeId is online as per local cache copy of cluster map.
func IsOnline(nodeId string) bool {
	return clusterMap.isOnline(nodeId)
}

// For a given MirroredVolume return the component RV that's online and has the lowest index.
func LowestIndexOnlineRV(mv dcache.MirroredVolume) string {
	return clusterMap.lowestIndexOnlineRV(mv)
}

// It will return the IP address of the given nodeId as per local cache copy of cluster map.
func NodeIdToIP(nodeId string) string {
	return clusterMap.nodeIdToIP(nodeId)
}

// It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map.
func RvIdToName(rvId string) string {
	return clusterMap.rvIdToName(rvId)
}

// It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map.
func RvNameToId(rvName string) string {
	return clusterMap.rvNameToId(rvName)
}

// It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map.
func RVNameToNodeId(rvName string) string {
	return clusterMap.rVNameToNodeId(rvName)
}

// It will return the IP address of the given RV name as per local cache copy of cluster map.
func RVNameToIp(rvName string) string {
	return clusterMap.rVNameToIp(rvName)
}

func GetActiveMVNames() []string {
	return clusterMap.getActiveMVNames()
}

func GetAllNodes() map[string]struct{} {
	return clusterMap.getAllNodes()
}

func IsClusterReadonly() bool {
	return clusterMap.isClusterReadonly()
}

// Refresh clustermap local copy from the metadata store.
// Once RefreshClusterMap() completes successfully, any clustermap call made would return results from the
// updated clustermap.
// higherThanEpoch is typically the current clustermap epoch value that solicited a NeedToRefreshClusterMap
// error from the server, so the caller is interested in a clustermap having epoch value higher than this.
// Note that it's not guaranteed that the next higher epoch would have the changes the caller expects, it's
// upto the caller to retry till it gets the required clusterMap.
// If you do not care about any specific clusterMap epoch but just want it to be refreshed once, pass 0 for
// higherThanEpoch.
//
// Note: Usually you will not need to work on the most up-to-date clustermap, the last periodically refreshed copy
//       of clustermap should be fine for most users. This API must be used by callers which cannot safely proceed
//       w/o knowing the latest clustermap. This should not be a common requirement and codepaths calling it should
//       be very infrequently executed.

func RefreshClusterMap(higherThanEpoch int64) error {
	// Clustermanager must call RegisterClusterMapSyncRefresher() in startup, so we don't expect this to be nil.
	common.Assert(clusterMapRefresher != nil)

	//
	// NeedToRefreshClusterMap return from the server typically means that the global clusterMap is always
	// updated and client can simply refresh and get that, but sometimes server may update the global
	// clusterMap after returning the NeedToRefreshClusterMap, so we try for a small time.
	//
	startTime := time.Now()
	maxWait := 5 * time.Second

	for {
		// Time check.
		elapsed := time.Since(startTime)
		if elapsed > maxWait {
			//
			// This can happen since callers sometimes do a best-effort clusterMap refresh,
			// as they are not sure that clusterMap has indeed changed.
			// Let the caller know and handle it as they see fit.
			//
			return fmt.Errorf("RefreshClusterMap: timed out waiting for epoch %d, got %d",
				higherThanEpoch+1, GetEpoch())
		}

		log.Debug("RefreshClusterMap: Fetching latest clustermap from metadata store")

		err := clusterMapRefresher()
		if err != nil {
			common.Assert(false)
			return fmt.Errorf("RefreshClusterMap: failed to fetch clusterMap: %v", err)
		}

		//
		// Break if we got the desired epoch, else try after a small wait.
		//
		if GetEpoch() > higherThanEpoch {
			break
		}

		log.Warn("RefreshClusterMap: Got epoch %d, while waiting for %d, retrying...",
			GetEpoch(), higherThanEpoch+1)
		time.Sleep(1 * time.Second)
	}

	return nil
}

// RegisterClusterMapRefresher is how the cluster_manager registers its real implementation.
func RegisterClusterMapRefresher(fn func() error) {
	clusterMapRefresher = fn
}

// This function must be called by any code that, through some other means (other than heartbeats) detects
// that an RV has gone offline.
// The RV state will be set to offline in the RV list in clustermap, along with all other side effects on
// MVs that have that RV as a component RV.
// The call blocks till the RV (and all affected MVs) is updated in the global (and local) clustermap.
func ReportRVOffline(rvName string) error {
	// TODO: Implement it.
	common.Assert(false, "Not implemented")
	return nil
}

// This function must be called by any caller that wants to change and persist the state of a component RV
// belonging to an MV. It blocks till the change is committed to the global clustermap.
// To avoid too many updates to the global clustermap, each of which will have to wait for the optimistic
// concurrent update, it batches updates received till the next update window and then makes a single update
// to the clustermap. The success or failure of this batched update will decide the success/failure of each
// of the individual updates.
func UpdateComponentRVState(mvName string, rvName string, rvNewState dcache.StateEnum) error {
	updateRVMessage := dcache.ComponentRVUpdateMessage{
		MvName:     mvName,
		RvName:     rvName,
		RvNewState: rvNewState,
		Err:        make(chan error),
	}
	common.Assert(updateComponentRVStateChannel != nil)

	//
	// Queue the update request to the channel.
	// It'll be picked by the periodic updater in the next update window along with all the other update requests
	// queued. Those updates are then applied to the clustermap and the updated clustermap committed at once.
	// The batch updater will push the return status on the error channel and close the channel.
	//
	updateComponentRVStateChannel <- updateRVMessage

	common.Assert(updateRVMessage.Err != nil)

	return <-updateRVMessage.Err
}

func GetComponentRVStateChannel() chan dcache.ComponentRVUpdateMessage {
	return updateComponentRVStateChannel
}

const (
	//
	// This is the size of the channel where RV updates are queued.
	// These many max updates can be batched. This must be greater than rm.MAX_SIMUL_SYNC_JOBS as each
	// sync job can generate one outstanding update.
	//
	MAX_SIMUL_RV_STATE_UPDATES = 10000
)

var (
	clusterMapRefresher func() error
	clusterMap          = &ClusterMap{}

	//
	// All go routines calling UpdateComponentRVState() around the same time will end up adding a corresponding
	// update message to this channel. Typically various sync jobs will call this to update the state of component
	// RVs from outofsync->syncing or syncing->online, so the size of this channel should be of the order of
	// simultaneous sync jobs, ref MAX_SIMUL_SYNC_JOBS.
	//
	updateComponentRVStateChannel = make(chan dcache.ComponentRVUpdateMessage, MAX_SIMUL_RV_STATE_UPDATES)
)

// clustermap package provides client methods to interact with the clusterManager, most importantly it provides
// methods for querying clustermap.
type ClusterMap struct {
	localMap            *dcache.ClusterMap
	mu                  sync.RWMutex // Synchronizes access to localMap.
	localClusterMapPath string
}

func (c *ClusterMap) stop() {
	close(updateComponentRVStateChannel)
	updateComponentRVStateChannel = nil
}

// Use this to get the local clustermap pointer safe from update by loadLocalMap().
// Note: Do not use c.localMap directly.
func (c *ClusterMap) getLocalMap() *dcache.ClusterMap {
	//
	// TODO: Evaluate if atomic.Pointer is faster than RWMutex.
	//       Since we can have heavy read access while very infrequent write access, RWMutex seems to
	//       be better, but need to evaluate under extreme load.
	//
	c.mu.RLock()
	defer c.mu.RUnlock()

	common.Assert(c.localMap != nil)
	return c.localMap
}

func (c *ClusterMap) loadLocalMap() {
	data, err := os.ReadFile(c.localClusterMapPath)
	if err != nil {
		log.Err("ClusterMap::loadLocalMap: Failed to read %s: %v", c.localClusterMapPath, err)
		common.Assert(false, err)
		return
	}

	var newClusterMap dcache.ClusterMap
	if err := json.Unmarshal(data, &newClusterMap); err != nil {
		log.Err("ClusterMap::loadLocalMap: Invalid JSON in %s: %v", c.localClusterMapPath, err)
		common.Assert(false, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.localMap = &newClusterMap
}

func (c *ClusterMap) getEpoch() int64 {
	return c.getLocalMap().Epoch
}

func (c *ClusterMap) getActiveMVs() map[string]dcache.MirroredVolume {
	activeMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.getLocalMap().MVMap {
		if mv.State == dcache.StateOnline {
			activeMVs[mvName] = mv
		}
	}
	return activeMVs
}

func (c *ClusterMap) getActiveMVNames() []string {
	localMap := c.getLocalMap()
	i := 0
	activeMVNames := make([]string, len(localMap.MVMap))
	for mvName, mv := range localMap.MVMap {
		if mv.State == dcache.StateOnline {
			activeMVNames[i] = mvName
			i++
		}
	}
	return activeMVNames[:i]
}

func (c *ClusterMap) getDegradedMVs() map[string]dcache.MirroredVolume {
	degradedMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.getLocalMap().MVMap {
		if mv.State == dcache.StateDegraded {
			degradedMVs[mvName] = mv
		}
	}
	return degradedMVs
}

func (c *ClusterMap) getSyncableMVs() map[string]dcache.MirroredVolume {
	syncableMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.getLocalMap().MVMap {
		if mv.State == dcache.StateDegraded {
			rvs := c.getRVs(mvName)
			// We got mvName from MVMap, so getRVs() should succeed.
			common.Assert(rvs != nil, mvName)

			for _, rvState := range rvs {
				if rvState == dcache.StateOutOfSync {
					syncableMVs[mvName] = mv
					break
				}
			}
		}
	}
	return syncableMVs
}

func (c *ClusterMap) getOfflineMVs() map[string]dcache.MirroredVolume {
	offlineMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.getLocalMap().MVMap {
		if mv.State == dcache.StateOffline {
			offlineMVs[mvName] = mv
		}
	}
	return offlineMVs
}

// Scan through the RV list and return the set of all nodes which have contributed at least one RV.
func (c *ClusterMap) getAllNodes() map[string]struct{} {
	nodesMap := make(map[string]struct{})

	for _, rv := range c.getLocalMap().RVMap {
		nodesMap[rv.NodeId] = struct{}{}
	}

	return nodesMap
}

func (c *ClusterMap) isClusterReadonly() bool {
	return c.getLocalMap().Readonly
}

func (c *ClusterMap) getCacheConfig() *dcache.DCacheConfig {
	return &c.getLocalMap().Config
}

func (c *ClusterMap) getClusterMap() dcache.ClusterMap {
	return *c.getLocalMap()
}

// Get RVs belonging to this node.
func (c *ClusterMap) getMyRVs() map[string]dcache.RawVolume {
	nodeId, err := common.GetNodeUUID()
	_ = err
	common.Assert(err == nil, fmt.Sprintf("Error getting nodeId: %v", err))

	myRvs := make(map[string]dcache.RawVolume)
	for name, rv := range c.getLocalMap().RVMap {
		if rv.NodeId == nodeId {
			myRvs[name] = rv
		}
	}
	return myRvs
}

func (c *ClusterMap) getAllRVs() map[string]dcache.RawVolume {
	return c.getLocalMap().RVMap
}

func (c *ClusterMap) isMyRV(rvName string) bool {
	myNodeID, err := common.GetNodeUUID()
	_ = err
	common.Assert(err == nil, err)

	return c.rVNameToNodeId(rvName) == myNodeID
}

// Get component RVs for the given MV.
func (c *ClusterMap) getRVs(mvName string) map[string]dcache.StateEnum {
	mv, ok := c.getLocalMap().MVMap[mvName]
	if !ok {
		log.Err("ClusterMap::getRVs: no mirrored volume named %s", mvName)
		return nil
	}
	return mv.RVs
}

// Get component RVs for the given MV, along with MV state and the clustermap epoch.
func (c *ClusterMap) getRVsEx(mvName string) (dcache.StateEnum, map[string]dcache.StateEnum, int64) {
	//
	// Save a copy of the clusterMap pointer to use for accessing MVMap and Epoch, so that both
	// correspond to the same instance of clusterMap.
	//
	localMap := c.getLocalMap()
	mv, ok := localMap.MVMap[mvName]
	if !ok {
		log.Err("ClusterMap::getRVs: no mirrored volume named %s", mvName)
		return dcache.StateInvalid, nil, -1
	}
	return mv.State, mv.RVs, localMap.Epoch
}

func (c *ClusterMap) getRVState(rvName string) dcache.StateEnum {
	rv, ok := c.getLocalMap().RVMap[rvName]
	if !ok {
		log.Err("ClusterMap::getRVState: no raw volume named %s", rvName)
		common.Assert(false, rvName)
		return dcache.StateInvalid
	}

	// online and offline are the only valid states for an RV.
	common.Assert(rv.State == dcache.StateOnline || rv.State == dcache.StateOffline, rvName, rv.State)
	return rv.State
}

func (c *ClusterMap) isOnline(nodeId string) bool {
	for _, rv := range c.getLocalMap().RVMap {
		if rv.NodeId == nodeId {
			return rv.State == dcache.StateOnline
		}
	}

	log.Debug("ClusterMap::isOnline: node %s not found", nodeId)

	// No caller should ask for a non-existent node.
	common.Assert(false, nodeId)
	return false
}

func (c *ClusterMap) lowestIndexOnlineRV(mv dcache.MirroredVolume) string {
	// We should be called only for a degraded MV>
	common.Assert(mv.State == dcache.StateDegraded)

	lowestIdxRVName := ""

	for rvName, state := range mv.RVs {
		if state != dcache.StateOnline {
			continue
		}

		if lowestIdxRVName == "" || strings.Compare(rvName, lowestIdxRVName) < 0 {
			lowestIdxRVName = rvName
		}
	}

	// For a degraded MV we must find the lowest index online RV,
	common.Assert(lowestIdxRVName != "", mv)
	common.Assert(IsValidRVName(lowestIdxRVName), lowestIdxRVName, mv)

	return lowestIdxRVName
}

func (c *ClusterMap) nodeIdToIP(nodeId string) string {
	for _, rv := range c.getLocalMap().RVMap {
		if rv.NodeId == nodeId {
			return rv.IPAddress
		}
	}

	log.Debug("ClusterMap::nodeIdToIP: node %s not found", nodeId)

	// Callers should not call for non-existent nodes.
	common.Assert(false, nodeId)
	return ""
}

func (c *ClusterMap) rVNameToNodeId(rvName string) string {
	rv, ok := c.getLocalMap().RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rvNameToId: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}

	return rv.NodeId
}

func (c *ClusterMap) rvIdToName(rvId string) string {
	for rvName, rv := range c.getLocalMap().RVMap {
		if rv.RvId == rvId {
			// TODO: Uncomment once we move IsValidRVName() and other utility functions to clustermap package.
			//common.Assert(IsValidRVName(rvName))
			return rvName
		}
	}

	log.Debug("ClusterMap::rvIdToName: rvID %s not found", rvId)

	// Callers should not call for non-existent RV.
	common.Assert(false, rvId)
	return ""
}

func (c *ClusterMap) rvNameToId(rvName string) string {
	rv, ok := c.getLocalMap().RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rvNameToId: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}
	return rv.RvId
}

func (c *ClusterMap) rVNameToIp(rvName string) string {
	rv, ok := c.getLocalMap().RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rVNameToIp: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}
	return rv.IPAddress
}
