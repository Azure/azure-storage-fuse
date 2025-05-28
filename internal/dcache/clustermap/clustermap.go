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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

func Stop() {
	clusterMap.stop()
}

// Update will load the local clustermap.
func Update() {
	clusterMap.loadLocalMap()
}

// It will return online MVs Map <mvName, MV> as per local cache copy of cluster map.
func GetActiveMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getActiveMVs()
}

// It will return degraded MVs Map <mvName, MV> as per local cache copy of cluster map.
func GetDegradedMVs() map[string]dcache.MirroredVolume {
	return clusterMap.getDegradedMVs()
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

// Is rvName hosted on this node.
func IsMyRV(rvName string) bool {
	return clusterMap.isMyRV(rvName)
}

// It will return all the RVs Map <rvName, rvState> for the particular mvName as per local cache copy of cluster map.
func GetRVs(mvName string) map[string]dcache.StateEnum {
	return clusterMap.getRVs(mvName)
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
//
//	 This API must be used by callers which cannot safely proceed
//		w/o knowing the latest clustermap. This should not be a common requirement and codepaths calling it should
//		be very infrequently executed.
func RefreshClusterMap() error {
	// Clustermanager must call RegisterClusterMapSyncRefresher() in startup, so we don't expect this to be nil.
	common.Assert(clusterMapRefresher != nil)
	log.Debug("RefreshClusterMap: Fetching latest clustermap from metadata store")

	return clusterMapRefresher()
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

// Tell clustermanager to update the state of a component RV for an MV.
func UpdateComponentRVState(mvName string, rvName string, rvNewState dcache.StateEnum) error {
	// Clustermanager must call RegisterComponentRVStateUpdater() in startup, so we don't expect this to be nil.
	common.Assert(componentRVStateUpdater != nil)
	return componentRVStateUpdater(mvName, rvName, rvNewState)
}

// RegisterComponentRVStateUpdater is how the cluster_manager registers its real implementation.
func RegisterComponentRVStateUpdater(fn func(mvName string, rvName string, rvNewState dcache.StateEnum) error) {
	componentRVStateUpdater = fn
}

var (
	componentRVStateUpdater func(mvName string, rvName string, rvNewState dcache.StateEnum) error
	clusterMapRefresher     func() error
	clusterMap              = &ClusterMap{
		// This MUST match localClusterMapPath in clustermanager.
		localClusterMapPath: filepath.Join(common.DefaultWorkDir, "clustermap.json"),
	}
)

// clustermap package provides client methods to interact with the clusterManager, most importantly it provides
// methods for querying clustermap.
type ClusterMap struct {
	localMap            *dcache.ClusterMap
	localClusterMapPath string
}

func (c *ClusterMap) stop() {
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

	c.localMap = &newClusterMap
}

func (c *ClusterMap) getActiveMVs() map[string]dcache.MirroredVolume {
	common.Assert(c.localMap != nil)

	activeMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateOnline {
			activeMVs[mvName] = mv
		}
	}
	return activeMVs
}

func (c *ClusterMap) getActiveMVNames() []string {
	common.Assert(c.localMap != nil)

	i := 0
	activeMVNames := make([]string, len(c.localMap.MVMap))
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateOnline {
			activeMVNames[i] = mvName
			i++
		}
	}
	return activeMVNames[:i]
}

func (c *ClusterMap) getDegradedMVs() map[string]dcache.MirroredVolume {
	common.Assert(c.localMap != nil)

	degradedMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateDegraded {
			degradedMVs[mvName] = mv
		}
	}
	return degradedMVs
}

func (c *ClusterMap) getOfflineMVs() map[string]dcache.MirroredVolume {
	common.Assert(c.localMap != nil)

	offlineMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateOffline {
			offlineMVs[mvName] = mv
		}
	}
	return offlineMVs
}

// Scan through the RV list and return the set of all nodes which have contributed at least one RV.
func (c *ClusterMap) getAllNodes() map[string]struct{} {
	common.Assert(c.localMap != nil)

	nodesMap := make(map[string]struct{})

	for _, rv := range c.localMap.RVMap {
		nodesMap[rv.NodeId] = struct{}{}
	}

	return nodesMap
}

func (c *ClusterMap) isClusterReadonly() bool {
	common.Assert(c.localMap != nil)

	return c.localMap.Readonly
}

func (c *ClusterMap) getCacheConfig() *dcache.DCacheConfig {
	common.Assert(c.localMap != nil)

	return &c.localMap.Config
}

func (c *ClusterMap) getClusterMap() dcache.ClusterMap {
	common.Assert(c.localMap != nil)
	return *c.localMap
}

// Get RVs belonging to this node.
func (c *ClusterMap) getMyRVs() map[string]dcache.RawVolume {
	common.Assert(c.localMap != nil)

	nodeId, err := common.GetNodeUUID()
	common.Assert(err == nil, fmt.Sprintf("Error getting nodeId: %v", err))

	myRvs := make(map[string]dcache.RawVolume)
	for name, rv := range c.localMap.RVMap {
		if rv.NodeId == nodeId {
			myRvs[name] = rv
		}
	}
	return myRvs
}

func (c *ClusterMap) isMyRV(rvName string) bool {
	myNodeID, err := common.GetNodeUUID()
	common.Assert(err == nil, err)

	return c.rVNameToNodeId(rvName) == myNodeID
}

// Get component RVs for the given MV.
func (c *ClusterMap) getRVs(mvName string) map[string]dcache.StateEnum {
	mv, ok := c.localMap.MVMap[mvName]
	if !ok {
		log.Err("ClusterMap::getRVs: no mirrored volume named %s", mvName)
		return nil
	}
	return mv.RVs
}

func (c *ClusterMap) getRVState(rvName string) dcache.StateEnum {
	rv, ok := c.localMap.RVMap[rvName]
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
	common.Assert(c.localMap != nil)

	for _, rv := range c.localMap.RVMap {
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
	common.Assert(c.localMap != nil)

	for _, rv := range c.localMap.RVMap {
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
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rvNameToId: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}

	return rv.NodeId
}

func (c *ClusterMap) rvIdToName(rvId string) string {
	common.Assert(c.localMap != nil)

	for rvName, rv := range c.localMap.RVMap {
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
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rvNameToId: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}
	return rv.RvId
}

func (c *ClusterMap) rVNameToIp(rvName string) string {
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClusterMap::rVNameToIp: rvName %s not found", rvName)
		// Callers should not call for non-existent RV.
		common.Assert(false, rvName)
		return ""
	}
	return rv.IPAddress
}
