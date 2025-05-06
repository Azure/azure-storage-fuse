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
	"os"
	"regexp"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

var (
	minUnixEpoch int64 = 1735689600 // Jan 1 2025, safe lowerlimit for Unix epoch validity check
	maxUnixEpoch int64 = 2524608000 // Jan 1 2050, safe upperlimit for Unix epoch validity check

	minClusterMapEpoch int64 = 30
	maxClusterMapEpoch int64 = 300

	minHeartbeatFrequency int64 = 5
	maxHeartbeatFrequency int64 = 60

	// TODO: chunk and stripe sizes must be expressed in units of MiB, in the config.
	minChunkSize int64 = 4 * common.MbToBytes
	maxChunkSize int64 = 64 * common.MbToBytes

	// StripeSize = ChunkSize * StripeWidth
	minStripeWidth int64 = 4
	maxStripeWidth int64 = 256

	minNumReplicas int64 = 1
	maxNumReplicas int64 = 256

	minMvsPerRv int64 = 10
	maxMvsPerRv int64 = 100

	minRvFullThreshold int64 = 80
	maxRvFullThreshold int64 = 100

	minRvNearFullThreshold int64 = 80
	maxRvNearFullThreshold int64 = 100

	rvNameRegex = regexp.MustCompile("^rv[0-9]+$")
	mvNameRegex = regexp.MustCompile("^mv[0-9]+$")
)

// Valid RV name is of the form "rv0", "rv99", etc.
func IsValidRVName(rvName string) bool {
	return rvNameRegex.MatchString(rvName)
}

// Valid MV name is of the form "mv0", "mv99", etc.
func IsValidMVName(mvName string) bool {
	return mvNameRegex.MatchString(mvName)
}

// Check all clustermap components for validity.
func IsValidClusterMap(cm *dcache.ClusterMap) (bool, error) {
	//
	// Top level fields
	//

	// Only valid clustermap states are "ready" and "checking".
	if cm.State != dcache.StateReady && cm.State != dcache.StateChecking {
		return false, fmt.Errorf("ClusterMap: Invalid State: %s %+v", cm.State, *cm)
	}

	if cm.CreatedAt < minUnixEpoch || cm.CreatedAt > maxUnixEpoch {
		return false, fmt.Errorf("ClusterMap: Invalid CreatedAt: %d %+v", cm.CreatedAt, *cm)
	}

	if cm.LastUpdatedAt < minUnixEpoch || cm.LastUpdatedAt > maxUnixEpoch {
		return false, fmt.Errorf("ClusterMap: Invalid LastUpdatedAt: %d %+v", cm.LastUpdatedAt, *cm)
	}

	if cm.LastUpdatedAt < cm.CreatedAt {
		return false, fmt.Errorf("ClusterMap: LastUpdatedAt (%d) < CreatedAt (%d) %+v",
			cm.LastUpdatedAt, cm.CreatedAt, *cm)
	}

	if !common.IsValidUUID(cm.LastUpdatedBy) {
		return false, fmt.Errorf("ClusterMap: Invalid LastUpdatedBy: %s %+v", cm.LastUpdatedBy, *cm)
	}

	//
	// Config
	//

	isValidDcacheConfig, err := IsValidDcacheConfig(&cm.Config)
	if !isValidDcacheConfig {
		return false, fmt.Errorf("ClusterMap: Invalid Config: %v %+v", err, *cm)
	}

	//
	// RVMap
	//

	isValidRVMap, err := IsValidRVMap(cm.RVMap)
	if !isValidRVMap {
		return false, fmt.Errorf("ClusterMap: Invalid RVMap: %v %+v", err, *cm)
	}

	//
	// MVMap
	//

	isValidMVMap, err := IsValidMvMap(cm.MVMap, int(cm.Config.NumReplicas))
	if !isValidMVMap {
		return false, fmt.Errorf("ClusterMap: Invalid MVMap: %v %+v", err, *cm)
	}

	return true, nil
}

// Config sanity validation.
func IsValidDcacheConfig(cfg *dcache.DCacheConfig) (bool, error) {

	// CacheId must be a valid non-empty string.
	if len(cfg.CacheId) == 0 {
		return false, fmt.Errorf("DCacheConfig: Empty CacheId %+v", *cfg)
	}

	// MinNodes must be at least 1.
	// We add an upper limit of 1000 as a sanity check.
	if cfg.MinNodes < 1 || cfg.MinNodes > 1000 {
		return false, fmt.Errorf("DCacheConfig: Invalid MinNodes: %d +%v", cfg.MinNodes, *cfg)
	}

	if int64(cfg.HeartbeatSeconds) < minHeartbeatFrequency ||
		int64(cfg.HeartbeatSeconds) > maxHeartbeatFrequency {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatSeconds: %d %+v", cfg.HeartbeatSeconds, *cfg)
	}

	// At least two heartbeats till node down.
	if cfg.HeartbeatsTillNodeDown < 2 {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatsTillNodeDown: %d %+v",
			cfg.HeartbeatsTillNodeDown, *cfg)
	}

	if int64(cfg.ClustermapEpoch) < minClusterMapEpoch || int64(cfg.ClustermapEpoch) > maxClusterMapEpoch {
		return false, fmt.Errorf("DCacheConfig: Invalid ClustermapEpoch: %d %+v", cfg.ClustermapEpoch, *cfg)
	}

	// Updating clustermap sooner than one heartbeat is usually pointless.
	if uint64(cfg.HeartbeatSeconds) > cfg.ClustermapEpoch {
		return false, fmt.Errorf("DCacheConfig: HeartbeatSeconds (%d) > ClustermapEpoch (%d) %+v", cfg.HeartbeatSeconds, cfg.ClustermapEpoch, *cfg)
	}

	if int64(cfg.ChunkSize) < minChunkSize || int64(cfg.ChunkSize) > maxChunkSize {
		return false, fmt.Errorf("DCacheConfig: Invalid ChunkSize: %d %+v", cfg.ChunkSize, *cfg)
	}

	if int64(cfg.StripeSize) < (int64(cfg.ChunkSize)*minStripeWidth) ||
		int64(cfg.StripeSize) > (int64(cfg.ChunkSize)*maxStripeWidth) {
		return false, fmt.Errorf("DCacheConfig: Invalid StripeSize: %d %+v", cfg.StripeSize, *cfg)
	}

	if int64(cfg.NumReplicas) < minNumReplicas || int64(cfg.NumReplicas) > maxNumReplicas {
		return false, fmt.Errorf("DCacheConfig: Invalid NumReplicas: %d %+v", cfg.NumReplicas, *cfg)
	}

	if int64(cfg.MvsPerRv) < minMvsPerRv || int64(cfg.MvsPerRv) > maxMvsPerRv {
		return false, fmt.Errorf("DCacheConfig: Invalid MvsPerRv: %d %+v", cfg.MvsPerRv, *cfg)
	}

	// MvsPerRv less than NumReplicas is usually pointless.
	if int64(cfg.MvsPerRv) < int64(cfg.NumReplicas) {
		return false, fmt.Errorf("DCacheConfig: MvsPerRv(%d) < NumReplicas(%d) %+v",
			cfg.MvsPerRv, cfg.NumReplicas, *cfg)
	}

	if int64(cfg.RvFullThreshold) < minRvFullThreshold || int64(cfg.RvFullThreshold) > maxRvFullThreshold {
		return false, fmt.Errorf("DCacheConfig: Invalid RvFullThreshold: %d %+v", cfg.RvFullThreshold, *cfg)
	}

	if int64(cfg.RvNearfullThreshold) < minRvNearFullThreshold ||
		int64(cfg.RvNearfullThreshold) > maxRvNearFullThreshold {
		return false, fmt.Errorf("DCacheConfig: Invalid RvNearfullThreshold: %d %+v", cfg.RvFullThreshold, *cfg)
	}

	if cfg.RvFullThreshold < cfg.RvNearfullThreshold {
		return false, fmt.Errorf("DCacheConfig: RvFullThreshold (%d) < RvNearfullThreshold (%d) %+v", cfg.RvFullThreshold, cfg.RvNearfullThreshold, *cfg)
	}

	return true, nil
}

func IsValidMvMap(mvMap map[string]dcache.MirroredVolume, expectedReplicasCount int) (bool, error) {

	common.Assert(expectedReplicasCount > 0)

	for mvName, mv := range mvMap {
		if mv.State != dcache.StateOnline &&
			mv.State != dcache.StateOffline &&
			mv.State != dcache.StateDegraded {
			return false, fmt.Errorf("MVMap: MV %s has invalid State: %s %+v", mvName, mv.State, mvMap)
		}

		if mv.RVs == nil {
			return false, fmt.Errorf("MVMap: MV %s has nil RVs %+v", mvName, mvMap)
		}

		if len(mv.RVs) != expectedReplicasCount {
			return false, fmt.Errorf("MVMap: MV %s has unexpected replica count, expected (%d), found (%d) %+v",
				mvName, expectedReplicasCount, len(mv.RVs), mvMap)
		}

		for rvName, state := range mv.RVs {
			if !IsValidRVName(rvName) {
				return false, fmt.Errorf("MVMap: MV %s has RV with invalid name %s %+v",
					mvName, rvName, mvMap)
			}

			if state != dcache.StateOnline &&
				state != dcache.StateOffline &&
				state != dcache.StateOutOfSync &&
				state != dcache.StateSyncing {
				return false, fmt.Errorf("MVMap: MV %s has RV with invalid state %s %+v",
					mvName, state, mvMap)
			}
		}
	}

	return true, nil
}

func IsValidRV(rv *dcache.RawVolume) (bool, error) {
	if !common.IsValidUUID(rv.NodeId) {
		return false, fmt.Errorf("RawVolume: Invalid NodeId: %s: %+v", rv.NodeId, *rv)
	}

	if !common.IsValidIP(rv.IPAddress) {
		return false, fmt.Errorf("RawVolume: Invalid IPAddress: %s: %+v", rv.IPAddress, *rv)
	}

	if !common.IsValidUUID(rv.RvId) {
		return false, fmt.Errorf("RawVolume: Invalid RvId: %s: %+v", rv.RvId, *rv)
	}

	if len(rv.FDID) == 0 {
		return false, fmt.Errorf("RawVolume: Invalid empty FDID: %+v", *rv)
	}

	if rv.State != dcache.StateOnline && rv.State != dcache.StateOffline {
		return false, fmt.Errorf("RawVolume: Invalid state: %s: %+v", rv.State, *rv)
	}

	if rv.TotalSpace <= 0 {
		return false, fmt.Errorf("RawVolume: Invalid TotalSpace: %d: %+v", rv.TotalSpace, *rv)
	}

	if rv.AvailableSpace > rv.TotalSpace {
		return false, fmt.Errorf("RawVolume: rv.AvailableSpace(%d) > rv.TotalSpace(%d): %+v",
			rv.AvailableSpace, rv.TotalSpace, *rv)
	}

	_, err := os.Stat(rv.LocalCachePath)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("RawVolume: Non-existent LocalCachePath: %s: %+v", rv.LocalCachePath, *rv)
	}

	return true, nil
}

// If myRVs is true it means that rvs is a list of my RVs.
func IsValidRVList(rvs []dcache.RawVolume, myRVs bool) (bool, error) {
	myNodeID, err := common.GetNodeUUID()
	common.Assert(err == nil, fmt.Sprintf("Failed to get our NodeId [%v]", err))
	common.Assert(len(rvs) > 0)

	myIP := rvs[0].IPAddress

	for _, rv := range rvs {
		if myRVs {
			if rv.NodeId != myNodeID {
				return false, fmt.Errorf("RVList: NodeId (%s) doesn't match my NodeId (%s) +%v",
					rv.NodeId, myNodeID, rv)
			}

			if rv.IPAddress != myIP {
				return false, fmt.Errorf("RVList: IPAddress (%s) doesn't match my IPAddress (%s) +%v",
					rv.IPAddress, myIP, rv)
			}
		}

		valid, err := IsValidRV(&rv)

		if !valid {
			return false, fmt.Errorf("RVList: Invalid RV: %v +%v", err, rv)
		}
	}

	return true, nil
}

func IsValidRVMap(rVMap map[string]dcache.RawVolume) (bool, error) {
	seen := make(map[string]string, len(rVMap))

	for rvName, rv := range rVMap {
		if prev, ok := seen[rv.RvId]; ok {
			return false, fmt.Errorf("ClusterMap::RVMap Duplicate RvId %s found in RVMap (%s and %s) %+v",
				rv.RvId, prev, rvName, rVMap)
		}
		seen[rv.RvId] = rvName

		valid, err := IsValidRV(&rv)

		if !valid {
			return false, fmt.Errorf("ClusterMap::RVMap Invalid RV %s :%v %+v", rvName, err, rVMap)
		}
	}

	return true, nil
}
