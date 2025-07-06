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
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	MinUnixEpoch int64 = 1735689600 // Jan 1 2025, safe lowerlimit for Unix epoch validity check
	MaxUnixEpoch int64 = 2524608000 // Jan 1 2050, safe upperlimit for Unix epoch validity check

	MinClusterMapEpoch int64 = 30
	MaxClusterMapEpoch int64 = 300

	MinHeartbeatFrequency int64 = 5
	MaxHeartbeatFrequency int64 = 60

	// TODO: chunk and stripe sizes must be expressed in units of MiB, in the config.
	MinChunkSize int64 = 4 * common.MbToBytes
	MaxChunkSize int64 = 64 * common.MbToBytes

	// StripeSize = ChunkSize * StripeWidth
	MinStripeWidth int64 = 4
	MaxStripeWidth int64 = 256

	MinNumReplicas int64 = 1
	MaxNumReplicas int64 = 256

	MinMvsPerRv int64 = 10
	MaxMvsPerRv int64 = 100

	MinRvFullThreshold int64 = 80
	MaxRvFullThreshold int64 = 100

	MinRvNearFullThreshold int64 = 80
	MaxRvNearFullThreshold int64 = 100

	rvNameRegex = regexp.MustCompile("^rv[0-9]+$")
	mvNameRegex = regexp.MustCompile("^mv[0-9]+$")
)

// Valid RV name is of the form "rv0", "rv99", etc.
func IsValidRVName(rvName string) bool {
	return rvNameRegex.MatchString(rvName)
}

// Valid component RV states are online, offline, outofsync, syncing.
func IsValidComponentRVState(rvState dcache.StateEnum) bool {
	switch rvState {
	case dcache.StateOnline,
		dcache.StateOffline,
		dcache.StateOutOfSync,
		dcache.StateSyncing:
		return true
	}
	return false
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

	if cm.CreatedAt < MinUnixEpoch || cm.CreatedAt > MaxUnixEpoch {
		return false, fmt.Errorf("ClusterMap: Invalid CreatedAt: %d %+v", cm.CreatedAt, *cm)
	}

	if cm.LastUpdatedAt < MinUnixEpoch || cm.LastUpdatedAt > MaxUnixEpoch {
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

	if int64(cfg.HeartbeatSeconds) < MinHeartbeatFrequency ||
		int64(cfg.HeartbeatSeconds) > MaxHeartbeatFrequency {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatSeconds: %d %+v", cfg.HeartbeatSeconds, *cfg)
	}

	// At least two heartbeats till node down.
	if cfg.HeartbeatsTillNodeDown < 2 {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatsTillNodeDown: %d %+v",
			cfg.HeartbeatsTillNodeDown, *cfg)
	}

	if int64(cfg.ClustermapEpoch) < MinClusterMapEpoch || int64(cfg.ClustermapEpoch) > MaxClusterMapEpoch {
		return false, fmt.Errorf("DCacheConfig: Invalid ClustermapEpoch: %d %+v", cfg.ClustermapEpoch, *cfg)
	}

	// Updating clustermap sooner than one heartbeat is usually pointless.
	if uint64(cfg.HeartbeatSeconds) > cfg.ClustermapEpoch {
		return false, fmt.Errorf("DCacheConfig: HeartbeatSeconds (%d) > ClustermapEpoch (%d) %+v", cfg.HeartbeatSeconds, cfg.ClustermapEpoch, *cfg)
	}

	if int64(cfg.ChunkSize) < MinChunkSize || int64(cfg.ChunkSize) > MaxChunkSize {
		return false, fmt.Errorf("DCacheConfig: Invalid ChunkSize: %d %+v", cfg.ChunkSize, *cfg)
	}

	if int64(cfg.StripeSize) < (int64(cfg.ChunkSize)*MinStripeWidth) ||
		int64(cfg.StripeSize) > (int64(cfg.ChunkSize)*MaxStripeWidth) {
		return false, fmt.Errorf("DCacheConfig: Invalid StripeSize: %d %+v", cfg.StripeSize, *cfg)
	}

	if int64(cfg.NumReplicas) < MinNumReplicas || int64(cfg.NumReplicas) > MaxNumReplicas {
		return false, fmt.Errorf("DCacheConfig: Invalid NumReplicas: %d %+v", cfg.NumReplicas, *cfg)
	}

	if int64(cfg.MvsPerRv) < MinMvsPerRv || int64(cfg.MvsPerRv) > MaxMvsPerRv {
		return false, fmt.Errorf("DCacheConfig: Invalid MvsPerRv: %d %+v", cfg.MvsPerRv, *cfg)
	}

	// MvsPerRv less than NumReplicas is usually pointless.
	if int64(cfg.MvsPerRv) < int64(cfg.NumReplicas) {
		return false, fmt.Errorf("DCacheConfig: MvsPerRv(%d) < NumReplicas(%d) %+v",
			cfg.MvsPerRv, cfg.NumReplicas, *cfg)
	}

	if int64(cfg.RvFullThreshold) < MinRvFullThreshold || int64(cfg.RvFullThreshold) > MaxRvFullThreshold {
		return false, fmt.Errorf("DCacheConfig: Invalid RvFullThreshold: %d %+v", cfg.RvFullThreshold, *cfg)
	}

	if int64(cfg.RvNearfullThreshold) < MinRvNearFullThreshold ||
		int64(cfg.RvNearfullThreshold) > MaxRvNearFullThreshold {
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
		isValid, err := IsValidMV(&mv, expectedReplicasCount)
		if !isValid {
			return false, fmt.Errorf("MVList: Invalid MV: %v +%v", mvName, err)
		}

	}

	return true, nil
}

func IsValidMV(mv *dcache.MirroredVolume, expectedReplicasCount int) (bool, error) {
	if mv.State != dcache.StateOnline &&
		mv.State != dcache.StateOffline &&
		mv.State != dcache.StateDegraded &&
		mv.State != dcache.StateSyncing {
		return false, fmt.Errorf("MirroredVolume: Invalid State: %s %+v", mv.State, mv)
	}

	if mv.RVs == nil {
		return false, fmt.Errorf("MirroredVolume: Nil RVs %+v", mv)
	}

	if len(mv.RVs) != expectedReplicasCount {
		return false, fmt.Errorf("MirroredVolume: Unexpected replica count, expected (%d), found (%d) %+v",
			expectedReplicasCount, len(mv.RVs), mv)
	}

	for rvName, state := range mv.RVs {
		if !IsValidRVName(rvName) {
			return false, fmt.Errorf("MirroredVolume: RV with invalid name %s %+v",
				rvName, mv)
		}
		if !IsValidComponentRVState(state) {
			return false, fmt.Errorf("MirroredVolume: RV with invalid state %s %+v", state, mv)
		}

	}
	return true, nil
}

// The existence and permissions of the cachedir are validated during configuration before the blobfuse2 daemon starts.
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

	// TODO: Check for some minimum amount of total/free space.

	return true, nil
}

// If myRVs is true it means that rvs is a list of my RVs.
func IsValidRVList(rvs []dcache.RawVolume, myRVs bool) (bool, error) {
	myNodeID, err := common.GetNodeUUID()
	_ = err
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

func IsValidHeartbeat(hb *dcache.HeartbeatData) (bool, error) {

	// NodeID
	if !common.IsValidUUID(hb.NodeID) {
		return false, fmt.Errorf("HeartbeatData: Invalid NodeId %s %+v", hb.NodeID, *hb)
	}

	// HostName
	if len(hb.Hostname) == 0 {
		return false, fmt.Errorf("HeartbeatData: Empty HostName %+v", *hb)
	}

	// IPAddress
	if !common.IsValidIP(hb.IPAddr) {
		return false, fmt.Errorf("HeartbeatData: Invalid IPAddress: %s: %+v", hb.IPAddr, *hb)
	}

	// LastHeartbeat
	if hb.LastHeartbeat < uint64(MinUnixEpoch) || hb.LastHeartbeat > uint64(MaxUnixEpoch) {
		return false, fmt.Errorf("HeartbeatData: Invalid LastHeartbeat: %d: %+v", hb.LastHeartbeat, *hb)
	}

	// At least one RV
	//TODO: Have to support nodes with no RVs
	if len(hb.RVList) == 0 {
		return false, fmt.Errorf("HeartbeatData: no RawVolumes %+v", *hb)
	}

	// Validate each RV
	for _, rv := range hb.RVList {
		if valid, err := IsValidRV(&rv); !valid {
			return false, fmt.Errorf("HeartbeatData: Invalid RV: %v", err)
		}

		if hb.NodeID != rv.NodeId {
			return false, fmt.Errorf("HeartbeatData: HB & RV NodeId mismatch: %s: %s: %+v", hb.NodeID, rv.NodeId, *hb)
		}

		if hb.IPAddr != rv.IPAddress {
			return false, fmt.Errorf("HeartbeatData: HB & RV IPAddr mismatch: %s: %s: %+v", hb.IPAddr, rv.IPAddress, *hb)
		}
	}
	return true, nil
}

// This function is used to export the clustermap for better viewing.
func ExportClusterMap(cm *dcache.ClusterMap) *dcache.ClusterMapExport {
	common.Assert(IsValidClusterMap(cm))
	// Sort keys
	rvKeys := make([]string, 0, len(cm.RVMap))
	for k := range cm.RVMap {
		rvKeys = append(rvKeys, k)
	}
	// Sort rvKeys by their names.
	sort.Slice(rvKeys, func(i, j int) bool {
		// Strip the "rv" prefix and convert the numeric part to integers for human sort order.
		numI, _ := strconv.Atoi(strings.TrimPrefix(rvKeys[i], "rv"))
		numJ, _ := strconv.Atoi(strings.TrimPrefix(rvKeys[j], "rv"))
		return numI < numJ
	})

	mvKeys := make([]string, 0, len(cm.MVMap))
	for k := range cm.MVMap {
		mvKeys = append(mvKeys, k)
	}
	// Sort mvKeys by their names.
	sort.Slice(mvKeys, func(i, j int) bool {
		// Strip the "mv" prefix and convert the numeric part to integers for human sort order.
		numI, _ := strconv.Atoi(strings.TrimPrefix(mvKeys[i], "mv"))
		numJ, _ := strconv.Atoi(strings.TrimPrefix(mvKeys[j], "mv"))
		return numI < numJ
	})

	// Create sorted slices
	rvList := make([]map[string]dcache.RawVolume, 0, len(rvKeys))
	for _, k := range rvKeys {
		rvList = append(rvList, map[string]dcache.RawVolume{k: cm.RVMap[k]})
	}

	mvList := make([]map[string]dcache.MirroredVolume, 0, len(mvKeys))
	for _, k := range mvKeys {
		mvList = append(mvList, map[string]dcache.MirroredVolume{k: cm.MVMap[k]})
	}

	return &dcache.ClusterMapExport{
		Readonly:      cm.Readonly,
		State:         cm.State,
		Epoch:         cm.Epoch,
		CreatedAt:     cm.CreatedAt,
		LastUpdatedAt: cm.LastUpdatedAt,
		LastUpdatedBy: cm.LastUpdatedBy,
		Config:        cm.Config,
		RVList:        rvList,
		MVList:        mvList,
	}
}
