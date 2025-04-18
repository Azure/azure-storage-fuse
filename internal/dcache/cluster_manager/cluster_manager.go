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

// ClusterManager defines the interface for managing cluster configuration and hearbeat related APIs.
type ClusterManager interface {

	// Start initializes the cluster manager with the given configuration.
	Start(*ClusterManagerConfig) error

	// Stop shuts down the cluster manager and releases any resources. Like stop heartbeat punching stop cluster config update
	Stop() error

	//It will return online MVs as per local cache copy of cluster config map
	GetActiveMVs() []dcache.MirroredVolume

	//It will return offline/ MVs as per local cache copy of cluster config map
	GetDegradedMVs() []dcache.MirroredVolume

	//It will return peer/node information by peerId/nodeId as per local cache copy of cluster config map
	GetPeer(nodeId string) dcache.Peer

	//It will return all the RVs for particular mv name as per local cache copy of cluster config map
	GetPeerRVs(mvName string) []dcache.RawVolume

	//It will check if the given nodeId is online as per local cache copy of cluster config map
	IsAlive(peerId string) bool

	//It will evaluate the lowest number of RVs for given rvs
	LowestNumberRV(rvs []string) []string

	//It will return the IP address of the given nodeId as per local cache copy of cluster config map
	NodeIdToIP(nodeId string) string

	//It will return the name of RV of the given RV FSID/Blkid as per local cache copy of cluster config map
	RVFsidToName(rvFsid string) string

	//It will return the RV FSID/Blkid of the given RV name as per local cache copy of cluster config map
	RVNameToFsid(rvName string) string

	//It will return the nodeId/uuid/peerid of the given RV name as per local cache copy of cluster config map
	RVNameToNodeId(rvName string) string

	//It will return the IP address of the given RV name as per local cache copy of cluster config map
	RVNameToIp(rvName string) string

	//It will Update the MV mapping in local as well as in Storage
	UpdateMVs(mvs []dcache.MirroredVolume)

	//It will Update the clusterMap config in Storage
	UpdateStorageConfigIfRequired()

	//It will Update the clusterMap config in local as per storage update
	WatchForConfigChanges() error
}

type ClusterManagerConfig struct {
	MinNodes               int
	ChunkSize              uint64
	StripeSize             uint64
	NumReplicas            uint8
	MvsPerRv               uint64
	HeartbeatSeconds       uint16
	HeartbeatsTillNodeDown uint8
	ClustermapEpoch        uint64
	RebalancePercentage    uint64
	SafeDeletes            bool
	CacheAccess            string
	StorageCachePath       string
	RVList                 []dcache.RawVolume
}
