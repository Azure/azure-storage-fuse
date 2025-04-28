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

import "github.com/Azure/azure-storage-fuse/v2/internal/dcache"

// ClusterManager defines the interface for managing distributed cache, cluster configuration and hearbeat related APIs.
type ClusterManager interface {

	// Start cluster manager which expects cluster config and list of raw volumes.
	//1. Create cluster map if not present
	//2. Schedule heartbeat punching
	//3. Schedule clusterMap update for storage
	//4. Schedule clusterMap update for local cache
	start(*dcache.DCacheConfig, []dcache.RawVolume) error

	// Stop shuts down the cluster manager and releases any resources.
	//1. Cancel schedule of cluster update over storage and local cache
	//2. Cancel schedule of heartbeat punching
	stop() error

	//It will return online MVs as per local cache copy of cluster map
	getActiveMVs() map[string]dcache.MirroredVolume

	//It will return the cache config as per local cache copy of cluster map
	getCacheConfig() *dcache.DCacheConfig

	//It will return offline/down MVs as per local cache copy of cluster map
	getDegradedMVs() map[string]dcache.MirroredVolume

	//It will return all the RVs for this particular node as per local cache copy of cluster map
	getMyRVs() map[string]dcache.RawVolume

	//It will return all the RVs for particular mv name as per local cache copy of cluster map
	getRVs(mvName string) map[string]string

	//It will check if the given nodeId is online as per local cache copy of cluster map
	isOnline(nodeId string) bool

	//It will evaluate the lowest number of RV for given rv Names
	lowestNumberRV(rvNames []string) string

	//It will return the IP address of the given nodeId as per local cache copy of cluster map
	nodeIdToIP(nodeId string) string

	//It will return the name of RV for the given RV FSID/Blkid as per local cache copy of cluster map
	rvIdToName(rvId string) string

	//It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster map
	rvNameToId(rvName string) string

	//It will return the nodeId/node uuid of the given RV name as per local cache copy of cluster map
	rVNameToNodeId(rvName string) string

	//It will return the IP address of the given RV name as per local cache copy of cluster map
	rVNameToIp(rvName string) string

	//Update RV state to down and update MVs
	reportRVDown(rvName string) error

	//Update RV state to offline and update MVs
	reportRVFull(rvName string) error

	//Notify consumer about cluster manager Event
	notifyUpdates() <-chan dcache.ClusterManagerEvent
}
