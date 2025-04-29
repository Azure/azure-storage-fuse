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

package clustermanager

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
)

type ClusterManagerImpl struct {
	hbTicker            *time.Ticker
	clusterMapticker    *time.Ticker
	nodeId              string
	hostname            string
	ipAddress           string
	localClusterMapPath string

	localMap     *dcache.ClusterMap
	localMapETag *string
	updatesChan  chan dcache.ClusterManagerEvent
}

// It will return online MVs as per local cache copy of cluster map
func GetActiveMVs() map[string]dcache.MirroredVolume {
	return clusterManagerInstance.getActiveMVs()
}

// It will return the cache config as per local cache copy of cluster map
func GetCacheConfig() *dcache.DCacheConfig {
	return clusterManagerInstance.getCacheConfig()
}

// It will return offline/down MVs as per local cache copy of cluster map
func GetDegradedMVs() map[string]dcache.MirroredVolume {
	return clusterManagerInstance.getDegradedMVs()
}

// It will return all the RVs for this particular node as per local cache copy of cluster map
func GetMyRVs() map[string]dcache.RawVolume {
	return clusterManagerInstance.getMyRVs()
}

// It will return all the RVs for particular mv name as per local cache copy of cluster map
func GetRVs(mvName string) map[string]dcache.StateEnum {
	return clusterManagerInstance.getRVs(mvName)
}

// It will check if the given nodeId is online as per local cache copy of cluster map
func IsOnline(nodeId string) bool {
	return clusterManagerInstance.isOnline(nodeId)
}

// It will evaluate the lowest number of RV for given rv Names
func LowestNumberRV(rvNames []string) string {
	return clusterManagerInstance.lowestNumberRV(rvNames)
}

// It will return the IP address of the given nodeId as per local cache copy of cluster map
func NodeIdToIP(nodeId string) string {
	return clusterManagerInstance.nodeIdToIP(nodeId)
}

// It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map
func RvIdToName(rvId string) string {
	return clusterManagerInstance.rvIdToName(rvId)
}

// It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map
func RvNameToId(rvName string) string {
	return clusterManagerInstance.rvNameToId(rvName)
}

// It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map
func RVNameToNodeId(rvName string) string {
	return clusterManagerInstance.rVNameToNodeId(rvName)
}

// It will return the IP address of the given RV name as per local cache copy of cluster map
func RVNameToIp(rvName string) string {
	return clusterManagerInstance.rVNameToIp(rvName)
}

// Update RV state to down and update MVs
func ReportRVDown(rvName string) error {
	return clusterManagerInstance.reportRVDown(rvName)
}

// Update RV state to offline and update MVs
func ReportRVFull(rvName string) error {
	return clusterManagerInstance.reportRVFull(rvName)
}

func Stop() error {
	return clusterManagerInstance.stop()
}

func GetNotificationChannel() <-chan dcache.ClusterManagerEvent {
	return clusterManagerInstance.getNotificationChannel()
}

// start implements ClusterManager.
func (cmi *ClusterManagerImpl) start(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	cmi.nodeId = rvs[0].NodeId

	common.Assert(common.IsValidUUID(cmi.nodeId), fmt.Sprintf("Invalid nodeId[%s]", cmi.nodeId))
	err := cmi.checkAndCreateInitialClusterMap(dCacheConfig)
	if err != nil {
		return err
	}
	cmi.hostname, err = os.Hostname()
	if err != nil {
		return err
	}
	cmi.ipAddress = rvs[0].IPAddress
	common.Assert(common.IsValidIP(cmi.ipAddress), fmt.Sprintf("Invalid Ip[%s] for nodeId[%s]", cmi.ipAddress, cmi.nodeId))

	// allocate notifyUpdates channel with small buffer
	cmi.updatesChan = make(chan dcache.ClusterManagerEvent, 5)

	cmi.hbTicker = time.NewTicker(time.Duration(dCacheConfig.HeartbeatSeconds) * time.Second)

	//Initial punch heartbeat triggering in a sync way to make this node available to detect as soon as possible
	log.Debug("Task \"Heartbeat Punch\" triggered (initial)")
	cmi.punchHeartBeat(rvs)

	go func() {
		for range cmi.hbTicker.C {
			log.Debug("Scheduled task \"Heartbeat Punch\" triggered")
			cmi.punchHeartBeat(rvs)
		}
		log.Info("Scheduled task \"Heartbeat Punch\" stopped")
	}()

	cmi.localClusterMapPath = filepath.Join(common.DefaultWorkDir, "clustermap.json")
	cmi.clusterMapticker = time.NewTicker(time.Duration(dCacheConfig.ClustermapEpoch) * time.Second)

	//Initial local copy update triggered in a sync way to make this node at least available with existing clusterMap configuration
	log.Debug("Task \"Cluster Map update\" (initial) task triggered")
	cmi.updateClusterMapLocalCopyIfRequired()
	go func() {
		for range cmi.clusterMapticker.C {
			log.Debug("Scheduled \"Cluster Map update\" task triggered")
			cmi.updateStorageClusterMapIfRequired()
			cmi.updateClusterMapLocalCopyIfRequired()
		}
		log.Info("Scheduled task \"ClusterMap update\" stopped")
	}()

	return nil
}

func (cmi *ClusterManagerImpl) updateClusterMapLocalCopyIfRequired() {
	// 1. Fetch the latest from storage
	storageBytes, etag, err := getClusterMap()
	if err != nil {
		log.Err("ClusterManagerImpl::updateClusterMapLocalCopyIfRequired: failed to fetch cluster map for nodeId %s: %v", cmi.nodeId, err)
		common.Assert(false)
		return
	}

	common.Assert(etag != nil, fmt.Sprintf("expected non‑nil ETag for node %s", cmi.nodeId))
	common.Assert(len(storageBytes) > 0,
		fmt.Sprintf("received empty cluster map for node %s",
			cmi.nodeId))

	//2. if we've already loaded this exact version, skip the update
	if cmi.localMap != nil && etag != nil && cmi.localMapETag != nil && *etag == *cmi.localMapETag {
		log.Debug("ClusterManagerImpl::updateClusterMapLocalCopyIfRequired: earlier and new etag matching, skipping update")
		return
	}

	//3. unmarshal storage copy
	var storageClusterMap dcache.ClusterMap
	if err := json.Unmarshal(storageBytes, &storageClusterMap); err != nil {
		log.Err("ClusterManagerImpl::updateClusterMapLocalCopyIfRequired: invalid storage clustermap JSON for nodeId %s: %v", cmi.nodeId, err)
		common.Assert(false)
		return
	}

	common.Assert(IsValidClusterMap(storageClusterMap))

	//4. atomically write new local file
	tmp := cmi.localClusterMapPath + ".tmp"
	if err := os.WriteFile(tmp, storageBytes, 0644); err != nil {
		log.Err("ClusterManagerImpl::updateClusterMapLocalCopyIfRequired: write temp file for localclustermap %+v failed: %v", storageClusterMap, err)
		common.Assert(false)
	} else if err := os.Rename(tmp, cmi.localClusterMapPath); err != nil {
		log.Err("ClusterManagerImpl::updateClusterMapLocalCopyIfRequired: Tmp file rename (%s) ->(%s) for localclustermap %+v failed: %v", tmp, cmi.localClusterMapPath, storageClusterMap, err)
		common.Assert(false)
	}

	//5. update in‑memory cache
	cmi.localMap = &storageClusterMap
	cmi.localMapETag = etag

	//TODO{Akku}: Notify only if there is a change in the MVs/RVs
	//6. fire an notification event
	select {
	case cmi.updatesChan <- dcache.ClusterManagerEvent{}:
	default:
		// drop if nobody is listening or buffer is full
	}
}

func (cmi *ClusterManagerImpl) getNotificationChannel() <-chan dcache.ClusterManagerEvent {
	return cmi.updatesChan
}

// Stop implements ClusterManager.
func (cmi *ClusterManagerImpl) stop() error {
	if cmi.hbTicker != nil {
		cmi.hbTicker.Stop()
	}
	// TODO{Akku}: Delete the heartbeat file
	// mm.DeleteHeartbeat(cmi.nodeId)
	if cmi.clusterMapticker != nil {
		cmi.clusterMapticker.Stop()
	}
	if cmi.updatesChan != nil {
		close(cmi.updatesChan)
	}
	return nil
}

// getActiveMVs implements ClusterManager.
func (cmi *ClusterManagerImpl) getActiveMVs() map[string]dcache.MirroredVolume {
	common.Assert(cmi.localMap != nil)

	activeMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range cmi.localMap.MVMap {
		if mv.State == dcache.StateOnline {
			activeMVs[mvName] = mv
		}
	}
	return activeMVs
}

// getCacheConfig implements ClusterManager.
func (cmi *ClusterManagerImpl) getCacheConfig() *dcache.DCacheConfig {
	common.Assert(cmi.localMap != nil)

	return &cmi.localMap.Config
}

// getDegradedMVs implements ClusterManager.
func (cmi *ClusterManagerImpl) getDegradedMVs() map[string]dcache.MirroredVolume {
	common.Assert(cmi.localMap != nil)

	degradedMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range cmi.localMap.MVMap {
		if mv.State == dcache.StateDegraded {
			degradedMVs[mvName] = mv
		}
	}
	return degradedMVs
}

// getMyRVs implements ClusterManager
func (cmi *ClusterManagerImpl) getMyRVs() map[string]dcache.RawVolume {
	common.Assert(cmi.localMap != nil)

	myRvs := make(map[string]dcache.RawVolume)
	for name, rv := range cmi.localMap.RVMap {
		if rv.NodeId == cmi.nodeId {
			myRvs[name] = rv
		}
	}
	return myRvs
}

// getRVs implements ClusterManager.
func (cmi *ClusterManagerImpl) getRVs(mvName string) map[string]dcache.StateEnum {

	mv, ok := cmi.localMap.MVMap[mvName]
	if !ok {
		log.Debug("ClusterManagerImpl::getRVs: no mirrored volume named %s", mvName)
		return nil
	}
	return mv.RVs
}

func (cmi *ClusterManagerImpl) isOnline(nodeId string) bool {
	common.Assert(cmi.localMap != nil)

	for _, rv := range cmi.localMap.RVMap {
		if rv.NodeId == nodeId {
			return rv.State == dcache.StateOnline
		}
	}
	log.Debug("ClusterManagerImpl::isOnline: node %s not found", nodeId)
	return false
}

// lowestNumberRV implements ClusterManager.
func (c *ClusterManagerImpl) lowestNumberRV(rvNames []string) string {
	lowestNumberRv := ""
	min := math.MaxInt32
	for _, rvName := range rvNames {
		num, err := strconv.Atoi(strings.TrimPrefix(rvName, "rv"))
		common.Assert(err == nil, fmt.Sprintf("Error converting rvName Suffix %s to int: %v", rvName, err))
		if num < min {
			min = num
			lowestNumberRv = rvName
		}
	}
	log.Debug("ClusterManagerImpl::lowestNumberRV: lowest number rvName in %v is %s", rvNames, lowestNumberRv)
	return lowestNumberRv
}

// nodeIdToIP implements ClusterManager.
func (cmi *ClusterManagerImpl) nodeIdToIP(nodeId string) string {
	common.Assert(cmi.localMap != nil)

	for _, rv := range cmi.localMap.RVMap {
		if rv.NodeId == nodeId {
			return rv.IPAddress
		}
	}
	log.Debug("ClusterManagerImpl::nodeIdToIP: node %s not found", nodeId)
	return ""
}

// rvIdToName implements ClusterManager.
func (cmi *ClusterManagerImpl) rvIdToName(rvId string) string {
	common.Assert(cmi.localMap != nil)

	for rvName, rv := range cmi.localMap.RVMap {
		if rv.RvId == rvId {
			return rvName
		}
	}
	log.Debug("ClusterManagerImpl::rvIdToName: rvID %s not found", rvId)
	return ""
}

// rvNameToId implements ClusterManager.
func (cmi *ClusterManagerImpl) rvNameToId(rvName string) string {
	common.Assert(cmi.localMap != nil)

	rv, ok := cmi.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterManagerImpl::rvNameToId: rvName %s not found", rvName)
		return ""
	}
	return rv.RvId
}

// rVNameToIp implements ClusterManager.
func (cmi *ClusterManagerImpl) rVNameToIp(rvName string) string {
	common.Assert(cmi.localMap != nil)

	rv, ok := cmi.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterManagerImpl::rVNameToIp: rvName %s not found", rvName)
		return ""
	}
	return rv.IPAddress
}

// rVNameToNodeId implements ClusterManager.
func (cmi *ClusterManagerImpl) rVNameToNodeId(rvName string) string {
	common.Assert(cmi.localMap != nil)

	rv, ok := cmi.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterManagerImpl::rVNameToNodeId: rvName %s not found", rvName)
		return ""
	}
	return rv.NodeId
}

// reportRVDown implements ClusterManager.
func (c *ClusterManagerImpl) reportRVDown(rvName string) error {
	return nil
}

// reportRVFull implements ClusterManager.
func (c *ClusterManagerImpl) reportRVFull(rvName string) error {
	return nil
}

func (cmi *ClusterManagerImpl) checkAndCreateInitialClusterMap(dCacheConfig *dcache.DCacheConfig) error {
	isClusterMapExists, err := cmi.checkIfClusterMapExists()
	if err != nil {
		log.Err("ClusterManagerImpl::checkAndCreateInitialClusterMap: Failed to check clusterMap file presence in Storage. error -: %v", err)
		return err
	}
	if isClusterMapExists {
		log.Info("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap already exists")
		return nil
	}
	currentTime := time.Now().Unix()
	clusterMap := dcache.ClusterMap{
		Readonly:      true,
		State:         dcache.StateReady,
		Epoch:         1,
		CreatedAt:     currentTime,
		LastUpdatedAt: currentTime,
		LastUpdatedBy: cmi.nodeId,
		Config:        *dCacheConfig,
		RVMap:         map[string]dcache.RawVolume{},
		MVMap:         map[string]dcache.MirroredVolume{},
	}
	clusterMapBytes, err := json.Marshal(clusterMap)
	if err != nil {
		log.Err("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap Marshalling fail : %+v, err %v", clusterMap, err)
		return err
	}

	err = mm.CreateInitialClusterMap(clusterMapBytes)
	if err != nil {
		log.Err("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap creation fail : %+v, err %v", clusterMap, err)
		return err
	} else {
		log.Info("ClusterManager::checkAndCreateInitialClusterMap : ClusterMap created successfully : %+v", clusterMap)
	}
	return err
}

func (cmi *ClusterManagerImpl) checkIfClusterMapExists() (bool, error) {
	_, _, err := getClusterMap()
	if err != nil {
		if os.IsNotExist(err) || err == syscall.ENOENT {
			return false, nil
		} else {
			return false, err
		}
	}
	//TODO: Save the cluster map in local copy
	return true, nil
}

var getClusterMap = func() ([]byte, *string, error) {
	return mm.GetClusterMap()
}

var getHeartbeat = func(nodeId string) ([]byte, error) {
	return mm.GetHeartbeat(nodeId)
}

var getAllNodes = func() ([]string, error) {
	return mm.GetAllNodes()
}

func evaluateReadOnlyState() bool {
	return false
}

func (cmi *ClusterManagerImpl) punchHeartBeat(rvList []dcache.RawVolume) {

	listMyRVs(rvList)
	hbData := dcache.HeartbeatData{
		IPAddr:        cmi.ipAddress,
		NodeID:        cmi.nodeId,
		Hostname:      cmi.hostname,
		LastHeartbeat: uint64(time.Now().Unix()),
		RVList:        rvList,
	}

	// Marshal the data into JSON
	data, err := json.MarshalIndent(hbData, "", "  ")
	//Adding Assert because error capturing can just log the error and continue because it's a schedule method
	common.Assert(err == nil, fmt.Sprintf("Error marshalling heartbeat data %+v : error - %v", hbData, err))
	if err == nil {
		// Create and update heartbeat file in storage with <nodeId>.hb
		err = mm.UpdateHeartbeat(cmi.nodeId, data)
		common.Assert(err == nil, fmt.Sprintf("Error updating heartbeat file with nodeId %s in storage: %v", cmi.nodeId, err))
		log.Debug("AddHeartBeat: Heartbeat file updated successfully %+v", hbData)
	} else {
		log.Warn("Error Updating heartbeat for nodeId %s with data %+v : error - %v", cmi.nodeId, hbData, err)
	}
}

func (cmi *ClusterManagerImpl) updateStorageClusterMapIfRequired() {
	clusterMapBytes, etag, err := getClusterMap()
	if err != nil {
		log.Err("updateStorageClusterMapIfRequired: GetClusterMap failed. err %v", err)
		return
	}
	var clusterMap dcache.ClusterMap
	if err := json.Unmarshal(clusterMapBytes, &clusterMap); err != nil {
		log.Err("updateStorageClusterMapIfRequired: failed to unmarshal clusterMapBytes(%d), error: %v",
			len(clusterMapBytes), err)
		return
	}
	// LastUpdatedBy must be a valid nodeid.
	common.Assert(IsValidClusterMap(clusterMap))

	//
	// The node that updated the clusterMap last is preferred over others, for updating the clusterMap.
	// This helps to avoid multiple nodes unnecessarily trying to update the clusterMap (only one of them will
	// succeed but we don't want to waste the effort put by all nodes). But, we have to be wary of the fact that
	// the leader node may go offline, in which case we would want some other node to step up and take the role of
	// the leader. We use the following simple strategy:
	// - Every ClustermapEpoch when the ticker fires, the leader node is automatically eligible for updating the
	//   clusterMap, it need not perform the staleness check.
	// - Every non-leader node has to perform a staleness check which defines a stale clusterMap as one that was
	//   updated more than ClustermapEpoch+thresholdEpochTime seconds in the past. thresholdEpochTime is chosen to
	//   be 60 secs to prevent minor clock skews from causing a non-leader to wrongly consider the clusterMap stale
	//   and race with the leader for updating the clusterMap. Only when the leader is down, on the next tick, one
	//   of the nodes that runs this code first will correctly find the clusterMap stale and it'd then take up the
	//   job of updating the clusterMap and becoming the new leader if it's able to successfully update the
	//   clusterMap.
	//
	// With these rules, the leader is the one that updates the clusterMap in every tick (ClustermapEpoch), while in
	// case of leader node going down, some other node will update the clusterMap in the next tick. In such case
	// the clusterMap will be updated after two consecutive ClustermapEpoch.
	//

	now := time.Now().Unix()
	if clusterMap.LastUpdatedAt > now {
		log.Warn("updateStorageClusterMapIfRequired: LastUpdatedAt(%d) in future, now(%d), skipping update",
			clusterMap.LastUpdatedAt, now)

		// Assert, taking into account potential clock skew.
		common.Assert((clusterMap.LastUpdatedAt-now) < 300, "cluster.LastUpdatedAt is too much in future")
		return
	}

	clusterMapAge := now - clusterMap.LastUpdatedAt
	// Assert if clusterMap is not updated for 3 consecutive epochs.
	// That might indicate some bug.
	common.Assert(clusterMapAge < int64(clusterMap.Config.ClustermapEpoch*3),
		fmt.Sprintf("clusterMapAge (%d) >= %d", clusterMapAge, clusterMap.Config.ClustermapEpoch*3))

	const thresholdEpochTime = 60
	stale := clusterMapAge > int64(clusterMap.Config.ClustermapEpoch+thresholdEpochTime)
	leader := clusterMap.LastUpdatedBy == cmi.nodeId

	//stale for checking state can be different than the stale for ready state
	// TODO{Akku}: update stale calculation for checking state
	// Skip if clustermap already in checking state
	if clusterMap.State == dcache.StateChecking && !stale {
		log.Debug("updateStorageClusterMapIfRequired: skipping,  Cluster map is under update by (leader %s). current node (%s)", clusterMap.LastUpdatedBy, cmi.nodeId)

		//Leader node should have updated the state to checking and it should not find the state to checking.
		common.Assert(!leader, "We don't expect leader to see the clustermap in checking state")
		return
	}

	// Skip if we're neither leader nor the clustermap is stale
	if !leader && !stale {
		log.Info("updateStorageClusterMapIfRequired: skipping, node (%s) is not leader (leader is %s) and clusterMap is fresh (last updated at epoch %d, now %d).",
			cmi.nodeId, clusterMap.LastUpdatedBy, clusterMap.LastUpdatedAt, now)
		return
	}

	clusterMap.LastUpdatedBy = cmi.nodeId
	clusterMap.State = dcache.StateChecking
	updatedClusterMapBytes, err := json.Marshal(clusterMap)

	if err != nil {
		log.Err("updateStorageClusterMapIfRequired: Marshal failed for clustermap %+v: %v", clusterMap, err)
		return
	}

	if err = mm.UpdateClusterMapStart(updatedClusterMapBytes, etag); err != nil {
		log.Err("updateStorageClusterMapIfRequired: start Clustermap update failed for nodeId %s: %v", cmi.nodeId, err)
		common.Assert(false)
		return
	}

	changed, err := cmi.updateRVList(clusterMap.RVMap)
	if err != nil {
		log.Err("updateStorageClusterMapIfRequired: failed to reconcile RV mapping: %v", err)
		common.Assert(false)
		return
	}
	if changed {
		cmi.updateMVList(clusterMap.RVMap, clusterMap.MVMap, int(GetCacheConfig().NumReplicas), int(GetCacheConfig().MvsPerRv))
	} else {
		log.Debug("updateStorageClusterMapIfRequired: No changes in RV mapping")
	}

	clusterMap.LastUpdatedAt = time.Now().Unix()
	clusterMap.State = dcache.StateReady
	updatedClusterMapBytes, err = json.Marshal(clusterMap)
	if err != nil {
		log.Err("updateStorageClusterMapIfRequired: Marshal failed for clustermap %+v: %v", clusterMap, err)
		return
	}

	//TODO{Akku}: Make sure end update is happing with the same node as of start update
	if err = mm.UpdateClusterMapEnd(updatedClusterMapBytes); err != nil {
		log.Err("updateStorageClusterMapIfRequired: end failed to update cluster map %+v, error: %v", clusterMap, err)
		common.Assert(false)
	} else {
		log.Info("updateStorageClusterMapIfRequired: cluster map %+v updated by %s at %d", clusterMap, cmi.nodeId, now)
	}
}

func (cmi *ClusterManagerImpl) updateMVList(rvMap map[string]dcache.RawVolume, existingMVMap map[string]dcache.MirroredVolume, NumReplicas int, MvsPerRv int) map[string]dcache.MirroredVolume {

	//
	// Approach:
	//
	// We make a list of nodes each having a list of RVs hosted by that node. This is
	// typically one RV per node, but it can be higher.
	// Each RV starts with a slot count equal to MvsPerRv. This is done so that we can
	// assign one RV to MvsPerRv MVs.
	// In Phase#1 we go over existing MVs and deduct slot count for all the RVs used
	// by the existing MVs. After that's done, we are left with RVs with updated slot
	// count signifying how many more MVs they can host.
	// In this phase we also check if any of the RVs used in existing MVs are offline
	// and mark the MVs as degraded. If all the RVs in a MV are offline, we mark the
	// MV as offline.
	// Now in Phase#2 we create as many new MVs as we can, continuing with the next
	// available MV name, each MV is assigned one RV from a different node, upto
	// NumReplicas for each MV.
	// This continues till we do not have enough RVs (from distinct nodes) for creating
	// a new MV.
	//

	// Local types
	type rv struct {
		rvName string
		slots  int //MvsPerRv
	}

	type node struct {
		nodeId string
		rvs    []rv
	}

	nodeToRvs := make(map[string]node)

	// TODO :: Handle scenarios for fix scenarios
	// Degraded - When any of the rv's in a mv is offline make mv as degraded [Done]
	// Fix - Replace any rv which is offline with an available rv and mark rv as out-of-sync while mv's state will be degraded
	// Sync - This will be handled by replica manager and it will change mv state to syncing
	// Offline - Mark a mv as offline when all the rv's within it are offline [Done]

	// Populate the RV struct and node struct
	for rvName, rvInfo := range rvMap {
		common.Assert(rvInfo.State == dcache.StateOnline || rvInfo.State == dcache.StateOffline, fmt.Sprintf("Invalid state %s for RV %s", rvInfo.State, rvName))
		common.IsValidUUID(rvInfo.NodeId)
		if rvInfo.State == dcache.StateOffline {
			// Skip RVs that are offline as they cannot contribute to any MV
			continue
		}
		if nodeInfo, exists := nodeToRvs[rvInfo.NodeId]; exists {
			// If the node already exists, append the RV to its list
			nodeInfo.rvs = append(nodeInfo.rvs, rv{
				rvName: rvName,
				slots:  MvsPerRv,
			})
			nodeToRvs[rvInfo.NodeId] = nodeInfo
		} else {
			// Create a new node and add the RV to it
			nodeToRvs[rvInfo.NodeId] = node{
				nodeId: rvInfo.NodeId,
				rvs:    []rv{{rvName: rvName, slots: MvsPerRv}},
			}
		}
	}

	// Phase#1
	for mvName, mv := range existingMVMap {
		offlineRv := 0
		for rvName := range mv.RVs {
			if rvMap[rvName].State == dcache.StateOffline {
				offlineRv++
				mv.RVs[rvName] = dcache.StateOffline
				mv.State = dcache.StateDegraded
				if offlineRv == len(mv.RVs) {
					mv.State = dcache.StateOffline
					// Update the MV state to offline
				}
				existingMVMap[mvName] = mv
				continue
			}
			nodeId := rvMap[rvName].NodeId
			found := false
			for i := range len(nodeToRvs[nodeId].rvs) {
				if nodeToRvs[nodeId].rvs[i].rvName == rvName {
					nodeToRvs[nodeId].rvs[i].slots--
					found = true
					break
				}
			}
			common.Assert(found, fmt.Sprintf("MV Map lists this as a online RV but the current RV %s was not found in node %s populated from RVMap", rvName, nodeId))
		}
	}

	// Phase#2
	for {
		// Stored nodes in a slice as its wasy to shuffle
		var availableNodes []node
		for _, n := range nodeToRvs {
			availableNodes = append(availableNodes, n)

		}

		maxMVsAllowed := (len(rvMap) * MvsPerRv) / NumReplicas

		common.Assert(maxMVsAllowed > 0, fmt.Sprintf("Max number of MVs %d is less than 0", maxMVsAllowed))
		// Check if we have reached the maximum number of MVs possible
		common.Assert(len(availableNodes) < maxMVsAllowed, fmt.Sprintf("Number of available nodes %d is greater than max MVs Allowed %d", len(availableNodes), maxMVsAllowed))
		// End of MV generation if we have enough MVs or if we have exhausted all the nodes
		if len(availableNodes) < NumReplicas {
			break
		}

		// Shuffle the available nodes to randomize selection
		rand.Shuffle(len(availableNodes), func(i, j int) {
			availableNodes[i], availableNodes[j] = availableNodes[j], availableNodes[i]
		})

		// Take the first NumReplicas nodes
		selectedNodes := availableNodes[:NumReplicas]

		mvName := fmt.Sprintf("mv%d", len(existingMVMap)) // starting from index 0

		// Select the first available RV from each selected node
		for _, n := range selectedNodes {
			for _, r := range n.rvs {
				// We should not have a rv in the list with slots <= 0
				common.Assert(r.slots > 0, fmt.Sprintf("RV %s has no slots left", r.rvName))
				// Safe check for slots
				if r.slots > 0 {
					if _, exists := existingMVMap[mvName]; !exists {
						rvwithstate := make(map[string]dcache.StateEnum)
						rvwithstate[r.rvName] = dcache.StateOnline
						// Create a new MV
						existingMVMap[mvName] = dcache.MirroredVolume{
							RVs:   rvwithstate,
							State: dcache.StateOnline,
						}
					} else {
						// Update the existing MV
						existingMVMap[mvName].RVs[r.rvName] = dcache.StateOnline
					}

					found := false
					// Decrease the slot count for the RV in nodeToRvs
					for i := range nodeToRvs[n.nodeId].rvs {
						if nodeToRvs[n.nodeId].rvs[i].rvName == r.rvName {
							nodeToRvs[n.nodeId].rvs[i].slots--
							found = true
							break
						}
					}
					common.Assert(found, fmt.Sprintf("MV Map lists this as a online RV but the current RV %s was not found in node %s populated from RVMap", r.rvName, n.nodeId))
					break
				}
			}
		}

		// Check if any node has exhausted all its rv's
		// Remove the node from the map if it has no RVs left
		for nodeId, node := range nodeToRvs {
			for j := 0; j < len(node.rvs); {
				// Remove a RV if its slots value is 0
				if node.rvs[j].slots == 0 {
					node.rvs = append(node.rvs[:j], node.rvs[j+1:]...)
				} else {
					j++
				}
			}

			// If the node has no RVs left, remove it from the map
			if len(node.rvs) == 0 {
				delete(nodeToRvs, nodeId)
			} else {
				nodeToRvs[nodeId] = node
			}
		}
		// The nodeToRvs map is updated with reamining nodes and their RVs
		// Only those RVs are left which have slots > 0
	}
	return existingMVMap
}

func (cmi *ClusterManagerImpl) updateRVList(clusterMapRVMap map[string]dcache.RawVolume) (bool, error) {
	nodeIds, err := getAllNodes()
	if err != nil {
		return false, fmt.Errorf("ClusterManagerImpl::updateRVList: Failed to get all nodes: error: %v", err)
	}
	log.Debug("ClusterManagerImpl::updateRVList: All nodes in the cluster: %v", nodeIds)
	rVsByRvId := make(map[string]dcache.RawVolume)
	changed := false
	for _, nodeId := range nodeIds {
		bytes, err := getHeartbeat(nodeId)
		if err != nil {
			return false, fmt.Errorf("ClusterManagerImpl::updateRVList: Failed to read heartbeat file for node %s: %v", nodeId, err)
		}
		var hbData dcache.HeartbeatData
		if err := json.Unmarshal(bytes, &hbData); err != nil {
			return false, fmt.Errorf("ClusterManagerImpl::updateRVList: Failed to parse heartbeat bytes for node %s: %v", nodeId, err)
		}
		log.Debug("ClusterManagerImpl::updateRVList: Iterating node : %s", nodeId)
		for _, rv := range hbData.RVList {
			if _, exists := rVsByRvId[rv.RvId]; exists {
				common.Assert(false, fmt.Sprintf("Duplicate RVId[%s] in heartbeats", rv.RvId))
			}
			common.Assert(rv.AvailableSpace <= rv.TotalSpace, fmt.Sprintf("Available space %d is greater than total space %d for RVId %s", rv.AvailableSpace, rv.TotalSpace, rv.RvId))
			common.Assert(common.IsValidUUID(rv.RvId), fmt.Sprintf("Invalid RvId[%s]", rv.RvId))
			rVsByRvId[rv.RvId] = rv
		}
	}
	//There can be 3 scenarios
	//1. There is nothing in clusterMap and RVs are present in heartbeat
	//2. There is something in clusterMap which needs to be updated
	//3. There is something in heartbeat which needs to be added to clusterMap

	for rvName, rvInClusterMap := range clusterMapRVMap {
		if rvHb, found := rVsByRvId[rvInClusterMap.RvId]; found {
			if (rvInClusterMap.State != rvHb.State) || (rvInClusterMap.AvailableSpace != rvHb.AvailableSpace) {
				changed = true
				rvInClusterMap.State = rvHb.State
				rvInClusterMap.AvailableSpace = rvHb.AvailableSpace
				//TODO{Akku}: IF available space is less than 10% of total space, we might need to update the state
				clusterMapRVMap[rvName] = rvInClusterMap
			}
			delete(rVsByRvId, rvHb.RvId)
		} else {
			log.Debug("ClusterManagerImpl::updateRVList: RvName=%s missing in new heartbeats", rvName)
			rvInClusterMap.State = dcache.StateOffline
			clusterMapRVMap[rvName] = rvInClusterMap
			changed = true

		}
	}

	// add any new RVs
	if len(rVsByRvId) != 0 {

		// find max index RV
		maxIdx := -1
		for name := range clusterMapRVMap {
			if i, err := strconv.Atoi(strings.TrimPrefix(name, "rv")); err == nil && i > maxIdx {
				maxIdx = i
			}
		}

		// Add new RV into clusterMap
		idx := maxIdx + 1
		for _, rv := range rVsByRvId {
			rvName := fmt.Sprintf("rv%d", idx)
			clusterMapRVMap[rvName] = rv
			idx++
			changed = true
			log.Info("updateRVList: Adding new RV %+v by rvName %s to cluster map.", rv, rvName)
		}
	}
	return changed, nil
}

func listMyRVs(rvList []dcache.RawVolume) {
	for index, rv := range rvList {
		_, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(rv.LocalCachePath)
		common.Assert(err == nil, fmt.Sprintf("Error getting disk space metrics for path %s for punching heartbeat: %v", rv.LocalCachePath, err))
		if err != nil {
			availableSpace = 0

			log.Warn("Error getting disk space metrics for path %s for punching heartbeat that's why forcing available Space to set to zero : %v", rv.LocalCachePath, err)
		}
		rvList[index].AvailableSpace = availableSpace
		rvList[index].State = dcache.StateOnline
	}
}

var (
	// clusterManagerInstance is the singleton instance of the ClusterManagerImpl
	clusterManagerInstance ClusterManager = &ClusterManagerImpl{}
	initCalled                            = false
)

func Init(dCacheConfig *dcache.DCacheConfig, rvs []dcache.RawVolume) error {
	common.Assert(!initCalled, "Cluster Manager Init should only be called once")
	initCalled = true
	err := clusterManagerInstance.start(dCacheConfig, rvs)
	return err
}
