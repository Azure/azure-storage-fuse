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
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

type ClusterManagerImpl struct {
	storageCallback dcache.StorageCallbacks
}

// Start implements ClusterManager.
func (c *ClusterManagerImpl) Start(*dcache.DCacheConfig, []dcache.RawVolume) error {
	return nil
}

// Stop implements ClusterManager.
func (c *ClusterManagerImpl) Stop() error {
	return nil
}

// GetActiveMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetActiveMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// GetDegradedMVs implements ClusterManager.
func (c *ClusterManagerImpl) GetDegradedMVs() []dcache.MirroredVolume {
	return make([]dcache.MirroredVolume, 0)
}

// GetRVs implements ClusterManager.
func (c *ClusterManagerImpl) GetRVs(mvName string) []dcache.RawVolume {
	return make([]dcache.RawVolume, 0)
}

// IsAlive implements ClusterManager.
func (c *ClusterManagerImpl) IsAlive(nodeId string) bool {
	return false
}

// LowestNumberRV implements ClusterManager.
func (c *ClusterManagerImpl) LowestNumberRV(rvNames []string) []string {
	return make([]string, 0)
}

// NodeIdToIP implements ClusterManager.
func (c *ClusterManagerImpl) NodeIdToIP(nodeId string) string {
	return ""
}

// RVFsidToName implements ClusterManager.
func (c *ClusterManagerImpl) RVFsidToName(rvFsid string) string {
	return ""
}

// RVNameToFsid implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToFsid(rvName string) string {
	return ""
}

// RVNameToIp implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToIp(rvName string) string {
	return ""
}

// RVNameToNodeId implements ClusterManager.
func (c *ClusterManagerImpl) RVNameToNodeId(rvName string) string {
	return ""
}

// ReportRVDown implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVDown(rvName string) error {
	return nil
}

// ReportRVFull implements ClusterManager.
func (c *ClusterManagerImpl) ReportRVFull(rvName string) error {
	return nil
}

func NewClusterManager(callback dcache.StorageCallbacks) ClusterManager {
	return &ClusterManagerImpl{
		storageCallback: callback,
	}
}
