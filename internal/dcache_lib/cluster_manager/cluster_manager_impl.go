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
	dcachelib "github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib"
)

type ClusterManagerImpl struct {
	StorageCallback dcachelib.StorageCallbacks
}

// GetPeer implements ClusterManager.
func (cmi *ClusterManagerImpl) GetPeer(nodeId string) dcachelib.Peer {
	return dcachelib.Peer{}
}

// GetPeerRVs implements ClusterManager.
func (cmi *ClusterManagerImpl) GetPeerRVs(mvName string) []dcachelib.RawVolume {
	return nil
}

func (cmi *ClusterManagerImpl) Start() error {
	return nil
}

func (cmi *ClusterManagerImpl) Stop() error {
	return nil
}

func NewClusterManager(callback dcachelib.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{}
}

func (cmi *ClusterManagerImpl) CreateClusterConfig(dcacheConfig dcachelib.DCacheConfig, storageCachepath string) error {
	return nil
}

func (cmi *ClusterManagerImpl) GetActiveMVs() []dcachelib.MirroredVolume {
	return nil
}

func (cmi *ClusterManagerImpl) IsAlive(peerId string) bool {
	return false
}

func (cmi *ClusterManagerImpl) UpdateMVs(mvs []dcachelib.MirroredVolume) {
}

func (cmi *ClusterManagerImpl) UpdateStorageConfigIfRequired() error {
	checkForRVs()
	evaluateMVsRVMapping()
	//Mark the Mv degraded
	return nil
}

func (cmi *ClusterManagerImpl) WatchForConfigChanges() error {

	// Update your local cluster config in the Path
	return nil
}

func checkForRVs() {
}

func evaluateMVsRVMapping() {}

func IsAlive(peerId string) bool {
	return false
}

func GetActiveMVs() []dcachelib.MirroredVolume {
	return nil
}
