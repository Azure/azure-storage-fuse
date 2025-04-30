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
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

// It will be used to close the notification channel
func CloseNotificationChannel() {
	clustermapImpl.closeNotificationChannel()
}

// GetNotificationChannel returns a read‐only channel of events.
func GetNotificationChannel() <-chan dcache.ClustermapEvent {
	return clustermapImpl.getNotificationChannel()
}

// PublishEvent is used by publishers to push ClusterManagerEvent events.
func PublishEvent(evt dcache.ClustermapEvent) {
	clustermapImpl.publishEvent(evt)
}

// It will return online MVs Map <mvName, MV> as per local cache copy of cluster map
func GetActiveMVs() map[string]dcache.MirroredVolume {
	return clustermapImpl.getActiveMVs()
}

// It will return the cache config as per local cache copy of cluster map
func GetCacheConfig() *dcache.DCacheConfig {
	return clustermapImpl.getCacheConfig()
}

// It will return degraded MVs Map <mvName, MV> as per local cache copy of cluster map
func GetDegradedMVs() map[string]dcache.MirroredVolume {
	return clustermapImpl.getDegradedMVs()
}

// It will return all the RVs Map <rvName, RV> for this particular node as per local cache copy of cluster map
func GetMyRVs() map[string]dcache.RawVolume {
	return clustermapImpl.getMyRVs()
}

// It will return all the RVs Map <rvName, rvState> for the particular mvName as per local cache copy of cluster map
func GetRVs(mvName string) map[string]dcache.StateEnum {
	return clustermapImpl.getRVs(mvName)
}

// It will check if the given nodeId is online as per local cache copy of cluster map
func IsOnline(nodeId string) bool {
	return clustermapImpl.isOnline(nodeId)
}

// It will evaluate the lowest number of RV for given rv Names
func LowestNumberRV(rvNames []string) string {
	return clustermapImpl.lowestNumberRV(rvNames)
}

// It will return the IP address of the given nodeId as per local cache copy of cluster map
func NodeIdToIP(nodeId string) string {
	return clustermapImpl.nodeIdToIP(nodeId)
}

// It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map
func RvIdToName(rvId string) string {
	return clustermapImpl.rvIdToName(rvId)
}

// It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map
func RvNameToId(rvName string) string {
	return clustermapImpl.rvNameToId(rvName)
}

// It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map
func RVNameToNodeId(rvName string) string {
	return clustermapImpl.rVNameToNodeId(rvName)
}

// It will return the IP address of the given RV name as per local cache copy of cluster map
func RVNameToIp(rvName string) string {
	return clustermapImpl.rVNameToIp(rvName)
}

var (
	clustermapImpl Clustermap = &ClustermapImpl{
		updatesChan:         make(chan dcache.ClustermapEvent, 8),
		localClusterMapPath: filepath.Join(common.DefaultWorkDir, "clustermap.json"),
	}
)

type ClustermapImpl struct {
	updatesChan         chan dcache.ClustermapEvent
	localMap            *dcache.ClusterMap
	localClusterMapPath string
}

// closeNotificationChannel implements Clustermap.
func (c *ClustermapImpl) closeNotificationChannel() {
	if c.updatesChan != nil {
		close(c.updatesChan)
	}
}

func (c *ClustermapImpl) consume() {
	for evt := range GetNotificationChannel() {
		log.Debug("ClustermapImpl::consume: received dcache.ClusterManagerEvent")

		// On every cluster‐map update event, reload localMap from the JSON file
		data, err := os.ReadFile(c.localClusterMapPath)
		if err != nil {
			log.Err("ClustermapImpl::consume: failed to read %s: %v", c.localClusterMapPath, err)
			continue
		}
		var newClusterMap dcache.ClusterMap
		if err := json.Unmarshal(data, &newClusterMap); err != nil {
			log.Err("ClustermapImpl::consume: invalid JSON in %s: %v", c.localClusterMapPath, err)
			continue
		}
		c.localMap = &newClusterMap
		// evt can carry metadata if needed
		_ = evt
	}
	// once CloseNotificationChannel() is called, the loop exits cleanly
}

// getNotificationChannel implements Clustermap.
func (c *ClustermapImpl) getNotificationChannel() <-chan dcache.ClustermapEvent {
	return c.updatesChan
}

// publishEvent implements Clustermap.
func (c *ClustermapImpl) publishEvent(evt dcache.ClustermapEvent) {
	select {
	case c.updatesChan <- evt:
	default:
	}
}

// getActiveMVs implements Clustermap.
func (c *ClustermapImpl) getActiveMVs() map[string]dcache.MirroredVolume {
	common.Assert(c.localMap != nil)

	activeMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateOnline {
			activeMVs[mvName] = mv
		}
	}
	return activeMVs
}

// getCacheConfig implements Clustermap.
func (c *ClustermapImpl) getCacheConfig() *dcache.DCacheConfig {
	common.Assert(c.localMap != nil)

	return &c.localMap.Config
}

// getDegradedMVs implements Clustermap.
func (c *ClustermapImpl) getDegradedMVs() map[string]dcache.MirroredVolume {
	common.Assert(c.localMap != nil)

	degradedMVs := make(map[string]dcache.MirroredVolume)
	for mvName, mv := range c.localMap.MVMap {
		if mv.State == dcache.StateDegraded {
			degradedMVs[mvName] = mv
		}
	}
	return degradedMVs
}

// getMyRVs implements Clustermap.
func (c *ClustermapImpl) getMyRVs() map[string]dcache.RawVolume {
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

// getRVs implements Clustermap.
func (c *ClustermapImpl) getRVs(mvName string) map[string]dcache.StateEnum {
	mv, ok := c.localMap.MVMap[mvName]
	if !ok {
		log.Debug("ClustermapImpl::getRVs: no mirrored volume named %s", mvName)
		return nil
	}
	return mv.RVs
}

// isOnline implements Clustermap.
func (c *ClustermapImpl) isOnline(nodeId string) bool {
	common.Assert(c.localMap != nil)

	for _, rv := range c.localMap.RVMap {
		if rv.NodeId == nodeId {
			return rv.State == dcache.StateOnline
		}
	}
	log.Debug("ClustermapImpl::isOnline: node %s not found", nodeId)
	return false
}

// lowestNumberRV implements Clustermap.
func (c *ClustermapImpl) lowestNumberRV(rvNames []string) string {
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
	log.Debug("ClustermapImpl::lowestNumberRV: lowest number rvName in %v is %s", rvNames, lowestNumberRv)
	return lowestNumberRv
}

// nodeIdToIP implements Clustermap.
func (c *ClustermapImpl) nodeIdToIP(nodeId string) string {
	common.Assert(c.localMap != nil)

	for _, rv := range c.localMap.RVMap {
		if rv.NodeId == nodeId {
			return rv.IPAddress
		}
	}
	log.Debug("ClustermapImpl::nodeIdToIP: node %s not found", nodeId)
	return ""
}

// rVNameToNodeId implements Clustermap.
func (c *ClustermapImpl) rVNameToNodeId(rvName string) string {
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClustermapImpl::rvNameToId: rvName %s not found", rvName)
		return ""
	}
	return rv.NodeId
}

// rvIdToName implements Clustermap.
func (c *ClustermapImpl) rvIdToName(rvId string) string {
	common.Assert(c.localMap != nil)

	for rvName, rv := range c.localMap.RVMap {
		if rv.RvId == rvId {
			return rvName
		}
	}
	log.Debug("ClustermapImpl::rvIdToName: rvID %s not found", rvId)
	return ""
}

// rvNameToId implements Clustermap.
func (c *ClustermapImpl) rvNameToId(rvName string) string {
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClustermapImpl::rvNameToId: rvName %s not found", rvName)
		return ""
	}
	return rv.RvId
}

// rVNameToIp implements Clustermap.
func (c *ClustermapImpl) rVNameToIp(rvName string) string {
	common.Assert(c.localMap != nil)

	rv, ok := c.localMap.RVMap[rvName]
	if !ok {
		log.Debug("ClustermapImpl::rVNameToIp: rvName %s not found", rvName)
		return ""
	}
	return rv.IPAddress
}

// start a consumer that does something with every event
func init() {
	go clustermapImpl.consume()
}
