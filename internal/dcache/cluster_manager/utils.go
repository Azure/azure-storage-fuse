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
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

var (
	minEpoch = 1735689600 // Jan 1 2025, safe lowerlimit for epoch validity check
	maxEpoch = 2524608000 // Jan 1 2050, safe upperlimit for epoch validity check
)

// TODO{Akku} : Return error instead of string
func IsValidClusterMap(cm dcache.ClusterMap) (bool, string) {
	// top‐level fields
	if cm.CreatedAt < int64(minEpoch) || cm.CreatedAt > int64(maxEpoch) {
		return false, fmt.Sprintf("ClusterMap:: %+v Invalid CreatedAt: %d", cm, cm.CreatedAt)
	}
	if cm.LastUpdatedAt < int64(minEpoch) || cm.LastUpdatedAt > int64(maxEpoch) {
		return false, fmt.Sprintf("ClusterMap:: %+v Invalid LastUpdatedAt: %d", cm, cm.LastUpdatedAt)
	}
	if cm.LastUpdatedAt < cm.CreatedAt {
		return false, fmt.Sprintf("ClusterMap:: %+v LastUpdatedAt (%d) < CreatedAt (%d)", cm, cm.LastUpdatedAt, cm.CreatedAt)
	}
	if !common.IsValidUUID(cm.LastUpdatedBy) {
		return false, fmt.Sprintf("ClusterMap:: %+v Invalid LastUpdatedBy UUID: %q", cm, cm.LastUpdatedBy)
	}

	// config
	isValidDcacheConfig, errString := IsValidDcacheConfig(cm)
	if !isValidDcacheConfig {
		return false, errString
	}

	// RVMap
	isValidRVMap, errString := IsValidRVMap(cm.RVMap)
	if !isValidRVMap {
		return false, errString
	}

	// MVMap
	return IsValidMvMap(cm)
}

func IsValidDcacheConfig(cm dcache.ClusterMap) (bool, string) {
	cfg := cm.Config
	if cfg.HeartbeatSeconds <= 0 {
		return false, fmt.Sprintf("ClusterMap:: %+v Invalid Config.HeartbeatSeconds: %d", cm, cfg.HeartbeatSeconds)
	}
	if cfg.ClustermapEpoch <= 0 {
		return false, fmt.Sprintf("ClusterMap:: %+v Invalid Config.ClustermapEpoch: %d", cm, cfg.ClustermapEpoch)
	}
	return true, ""
}

func IsValidMvMap(cm dcache.ClusterMap) (bool, string) {
	expectedReplicasCount := int(cm.Config.NumReplicas)
	for name, mv := range cm.MVMap {
		switch mv.State {
		case dcache.StateOnline, dcache.StateOffline:
		default:
			return false, fmt.Sprintf("ClusterMap:: %+v MVMap[%q]: Invalid mv State: %q", cm, name, mv.State)
		}
		if mv.RVs == nil {
			return false, fmt.Sprintf("ClusterMap:: %+v MVMap[%q]: Rvs is nil", cm, name)
		}
		if len(mv.RVs) != expectedReplicasCount {
			return false, fmt.Sprintf(
				"ClusterMap:: %+v MVMap[%q]: expected %d replicas, found %d",
				cm, name, expectedReplicasCount, len(mv.RVs),
			)
		}
		for rvName, state := range mv.RVs {
			if rvName == "" {
				return false, fmt.Sprintf("ClusterMap:: %+v MVMap[%q].Rvs: empty key", cm, name)
			}
			switch state {
			case dcache.StateOnline, dcache.StateOffline, dcache.StateSyncing:
			default:
				return false, fmt.Sprintf("ClusterMap:: %+v MVMap[%q].Rvs[%q]: Invalid RV state %q in MV ", cm, name, rvName, state)
			}
		}
	}
	return true, ""
}

func IsValidRVMap(rVMap map[string]dcache.RawVolume) (bool, string) {
	seen := make(map[string]string, len(rVMap))
	for rvName, rv := range rVMap {
		if prev, ok := seen[rv.RvId]; ok {
			return false, fmt.Sprintf(
				"ClusterMap::RVMap %+v duplicate RvId %q found in RVMap entries %q and %q", rVMap, rv.RvId, prev, rvName,
			)
		}
		seen[rv.RvId] = rvName
		if !common.IsValidUUID(rv.RvId) {
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: invalid RvId UUID: %q", rVMap, rvName, rv.RvId)
		}
		if !common.IsValidUUID(rv.NodeId) {
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: invalid NodeId UUID: %q", rVMap, rvName, rv.NodeId)
		}
		if !common.IsValidIP(rv.IPAddress) {
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: invalid IPAddress: %q", rVMap, rvName, rv.IPAddress)
		}
		if rv.TotalSpace <= 0 {
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: bad space metrics avail=%d total=%d", rVMap, rvName, rv.AvailableSpace, rv.TotalSpace)
		}
		if rv.AvailableSpace > rv.TotalSpace {
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: AvailableSpace %d > TotalSpace %d", rVMap, rvName, rv.AvailableSpace, rv.TotalSpace)
		}
		switch rv.State {
		case dcache.StateOnline, dcache.StateOffline:
		default:
			return false, fmt.Sprintf("ClusterMap::RVMap %+v RVMap[%q]: unknown State: %q", rVMap, rvName, rv.State)
		}
	}
	return true, ""
}
