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
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
)

//go:generate $ASSERT_REMOVER $GOFILE

var (
	MinUnixEpoch int64 = 1735689600 // Jan 1 2025, safe lowerlimit for Unix epoch validity check
	MaxUnixEpoch int64 = 2524608000 // Jan 1 2050, safe upperlimit for Unix epoch validity check

	MinClusterMapEpoch int64 = 30
	MaxClusterMapEpoch int64 = 300

	MinHeartbeatFrequency int64 = 5
	MaxHeartbeatFrequency int64 = 60

	MinHeartbeatsTillNodeDown int64 = 2
	MaxHeartbeatsTillNodeDown int64 = 5

	//
	// The range of chunk size and stripe width is deliberately kept large so that we can experiment with
	// various values and find the optimal ones.
	// TODO: Later we can reduce the range to a more reasonable one.
	//
	MinChunkSizeMB int64 = 1
	MaxChunkSizeMB int64 = 1024

	// StripeWidthMB = ChunkSizeMB * StripeWidth
	MinStripeWidth int64 = 1
	MaxStripeWidth int64 = 1024

	MinNumReplicas int64 = 1
	MaxNumReplicas int64 = 256

	MinMaxRVs int64 = 100
	MaxMaxRVs int64 = 100000

	//
	// Unless explicitly set, the system sets cfg.MVsPerRV in a way so as to get close to PreferredMVs
	// number of MVs. Obviously it'll honour MaxMVsPerRV.
	//
	// TODO: See if we need to make this a config option.
	// TODO: Test if 1000 is a good number.
	//
	PreferredMVs int64 = 1000

	MinMVsPerRV int64 = 1

	// This will be bumped up if RingBasedMVPlacement is enabled.
	MaxMVsPerRV int64 = 100

	MVsPerRVLocked bool = false

	//
	// When using an RV for placing a new MV, how many MV replicas are we allowed to place on an RV.
	// See MVsPerRVForFixMV which is a higher value used when using an RV to place MV replicas for
	// fixing degraded MVs.
	// This is not a fixed value, instead it's updated dynamically based on the number of RVs and MVs.
	// It starts with the value from config.MVsPerRV, so as to quickly get PreferredMVs and as the number
	// of RVs grows, it's exponentially reduced to limit the total number of MVs in the cluster while
	// still adding new MVs as we add more RVs. See ClusterManager.computeMVsPerRV().
	// Our goal is to distribute MVs as evenly as possible across all RVs, while also making sure that we
	// utilize new RVs for placing additional new MVs.
	//
	MVsPerRVForNewMV int

	//
	// When using an RV for placing MV replicas for fixing degraded MVs, how many MV replicas are we allowed
	// to place on an RV. This is a higher value than MVsPerRVForNewMV as we technically we have fewer RVs
	// to place the same number of MV replicas, so we need to allow more MV replicas per RV.
	// See ClusterManager.computeMVsPerRV().
	//
	MVsPerRVForFixMV atomic.Int32

	//
	// MVsPerRVScaleFactor decides how many times more MVs can we allow in the FixMV workflow, than the NewMV
	// workflow.
	//
	MVsPerRVScaleFactor int64 = 4

	MinRvFullThreshold int64 = 80
	MaxRvFullThreshold int64 = 100

	MinRvNearFullThreshold int64 = 80
	MaxRvNearFullThreshold int64 = 100

	rvNameRegex = regexp.MustCompile("^rv[0-9]+$")
	mvNameRegex = regexp.MustCompile("^mv[0-9]+$")

	RingBasedMVPlacement bool
	ThriftServerType     string
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
		dcache.StateInbandOffline,
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
		return false, fmt.Errorf("DCacheConfig: Invalid MinNodes: %d (valid range [%d, %d]) %+v",
			cfg.MinNodes, 1, 1000, *cfg)
	}

	if int64(cfg.MaxRVs) < MinMaxRVs || int64(cfg.MaxRVs) > MaxMaxRVs {
		return false, fmt.Errorf("DCacheConfig: Invalid MaxRVs: %d (valid range [%d, %d]) %+v",
			cfg.MaxRVs, MinMaxRVs, MaxMaxRVs, *cfg)
	}

	// Every node must contribute at least one RV.
	if cfg.MaxRVs < cfg.MinNodes {
		return false, fmt.Errorf("DCacheConfig: Invalid MaxRVs: %d (cannot be less than min-nodes (%d)) %+v",
			cfg.MaxRVs, cfg.MinNodes, *cfg)
	}

	// We must have RVs no less than NumReplicas.
	if cfg.MaxRVs < cfg.NumReplicas {
		return false, fmt.Errorf("DCacheConfig: Invalid MaxRVs: %d (cannot be less than replicas (%d)) %+v",
			cfg.MaxRVs, cfg.NumReplicas, *cfg)
	}

	if int64(cfg.HeartbeatSeconds) < MinHeartbeatFrequency ||
		int64(cfg.HeartbeatSeconds) > MaxHeartbeatFrequency {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatSeconds: %d (valid range [%d, %d]) %+v",
			cfg.HeartbeatSeconds, MinHeartbeatFrequency, MaxHeartbeatFrequency, *cfg)
	}

	// At least two heartbeats till node down.
	if int64(cfg.HeartbeatsTillNodeDown) < MinHeartbeatsTillNodeDown ||
		int64(cfg.HeartbeatsTillNodeDown) > MaxHeartbeatsTillNodeDown {
		return false, fmt.Errorf("DCacheConfig: Invalid HeartbeatsTillNodeDown: %d (valid range [%d, %d]) %+v",
			cfg.HeartbeatsTillNodeDown, MinHeartbeatsTillNodeDown, MaxHeartbeatsTillNodeDown, *cfg)
	}

	if int64(cfg.ClustermapEpoch) < MinClusterMapEpoch ||
		int64(cfg.ClustermapEpoch) > MaxClusterMapEpoch {
		return false, fmt.Errorf("DCacheConfig: Invalid ClustermapEpoch: %d (valid range [%d, %d]) %+v",
			cfg.ClustermapEpoch, MinClusterMapEpoch, MaxClusterMapEpoch, *cfg)
	}

	// Updating clustermap sooner than one heartbeat is usually pointless.
	if uint64(cfg.HeartbeatSeconds) > cfg.ClustermapEpoch {
		return false, fmt.Errorf("DCacheConfig: HeartbeatSeconds (%d) > ClustermapEpoch (%d) %+v",
			cfg.HeartbeatSeconds, cfg.ClustermapEpoch, *cfg)
	}

	if int64(cfg.ChunkSizeMB) < MinChunkSizeMB || int64(cfg.ChunkSizeMB) > MaxChunkSizeMB {
		return false, fmt.Errorf("DCacheConfig: Invalid ChunkSizeMB: %d (valid range [%d, %d]) %+v",
			cfg.ChunkSizeMB, MinChunkSizeMB, MaxChunkSizeMB, *cfg)
	}

	if int64(cfg.StripeWidth) < MinStripeWidth || int64(cfg.StripeWidth) > MaxStripeWidth {
		return false, fmt.Errorf("DCacheConfig: Invalid StripeWidth: %d (valid range [%d, %d]) %+v",
			cfg.StripeWidth, MinStripeWidth, MaxStripeWidth, *cfg)
	}

	if int64(cfg.NumReplicas) < MinNumReplicas || int64(cfg.NumReplicas) > MaxNumReplicas {
		return false, fmt.Errorf("DCacheConfig: Invalid NumReplicas: %d (valid range [%d, %d]) %+v",
			cfg.NumReplicas, MinNumReplicas, MaxNumReplicas, *cfg)
	}

	if int64(cfg.MVsPerRV) < MinMVsPerRV || int64(cfg.MVsPerRV) > MaxMVsPerRV {
		return false, fmt.Errorf("DCacheConfig: Invalid MVsPerRV: %d (valid range [%d, %d]) %+v",
			cfg.MVsPerRV, MinMVsPerRV, MaxMVsPerRV, *cfg)
	}

	if int64(cfg.RvFullThreshold) < MinRvFullThreshold || int64(cfg.RvFullThreshold) > MaxRvFullThreshold {
		return false, fmt.Errorf("DCacheConfig: Invalid RvFullThreshold: %d (valid range [%d, %d]) %+v",
			cfg.RvFullThreshold, MinRvFullThreshold, MaxRvFullThreshold, *cfg)
	}

	if int64(cfg.RvNearfullThreshold) < MinRvNearFullThreshold ||
		int64(cfg.RvNearfullThreshold) > MaxRvNearFullThreshold {
		return false, fmt.Errorf("DCacheConfig: Invalid RvNearfullThreshold: %d (valid range [%d, %d]) %+v",
			cfg.RvNearfullThreshold, MinRvNearFullThreshold, MaxRvNearFullThreshold, *cfg)
	}

	if cfg.RvFullThreshold < cfg.RvNearfullThreshold {
		return false, fmt.Errorf("DCacheConfig: RvFullThreshold (%d) < RvNearfullThreshold (%d) %+v",
			cfg.RvFullThreshold, cfg.RvNearfullThreshold, *cfg)
	}

	return true, nil
}

func IsValidMvMap(mvMap map[string]dcache.MirroredVolume, expectedReplicasCount int) (bool, error) {

	common.Assert(expectedReplicasCount > 0)

	for mvName, mv := range mvMap {
		isValid, err := IsValidMV(&mv, expectedReplicasCount)
		if !isValid {
			return false, fmt.Errorf("MVList: Invalid MV: %v %+v", mvName, err)
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

	// TODO: Is there a validity check for FDId and UDId?

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

	//
	// LocalCachePath must exist and must be a directory.
	// Avoid these for release builds to avoid unnecessary system calls.
	//
	if common.IsDebugBuild() {
		info, err := os.Stat(rv.LocalCachePath)
		if err != nil && os.IsNotExist(err) {
			return false, fmt.Errorf("RawVolume: LocalCachePath %s does not exist: %+v",
				rv.LocalCachePath, *rv)
		} else if err != nil {
			return false, fmt.Errorf("RawVolume: Cannot access LocalCachePath %s: %v: %+v",
				rv.LocalCachePath, err, *rv)
		}

		if !info.IsDir() {
			return false, fmt.Errorf("RawVolume: LocalCachePath %s is not a directory: %+v",
				rv.LocalCachePath, *rv)
		}
	}

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
				return false, fmt.Errorf("RVList: NodeId (%s) doesn't match my NodeId (%s) %+v",
					rv.NodeId, myNodeID, rv)
			}

			if rv.IPAddress != myIP {
				return false, fmt.Errorf("RVList: IPAddress (%s) doesn't match my IPAddress (%s) %+v",
					rv.IPAddress, myIP, rv)
			}
		}

		valid, err := IsValidRV(&rv)

		if !valid {
			return false, fmt.Errorf("RVList: Invalid RV: %v %+v", err, rv)
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
		Epoch:         cm.Epoch,
		CreatedAt:     cm.CreatedAt,
		LastUpdatedAt: cm.LastUpdatedAt,
		LastUpdatedBy: cm.LastUpdatedBy,
		Config:        cm.Config,
		RVList:        rvList,
		MVList:        mvList,
	}
}

// Convert an RVMap returned by clustermap APIs like GetRVs()/GetRVsEx() to a list of RVNameAndState
// needed by rvInfo and others.
func RVMapToList(mvName string, rvMap map[string]dcache.StateEnum, randomize bool) []*models.RVNameAndState {
	componentRVs := make([]*models.RVNameAndState, 0, int(GetCacheConfig().NumReplicas))

	for rvName, rvState := range rvMap {
		common.Assert(IsValidRVName(rvName), rvName)
		common.Assert(IsValidComponentRVState(rvState), rvName, rvState)

		componentRVs = append(componentRVs,
			&models.RVNameAndState{Name: rvName, State: string(rvState)})
	}

	common.Assert(len(componentRVs) == int(GetCacheConfig().NumReplicas),
		mvName, len(componentRVs), GetCacheConfig().NumReplicas)

	//
	// Take this opportunity to randomize the order of RVs in the list, this is usually desirable
	// to distribute load evenly across RVs.
	//
	if randomize {
		rand.Shuffle(len(componentRVs), func(i, j int) {
			componentRVs[i], componentRVs[j] = componentRVs[j], componentRVs[i]
		})
	}

	return componentRVs
}

var uuidToUniqueInt = map[string]int{}
var uuidToUniqueIntMapMutex sync.RWMutex
var uniqueInt int

// Return a unique integer for the given UUID. The returned integer is guaranteed to be unique
// for each unique UUID passed to this function and will be in the range [1, 2^31-1].
// The uniqueness is in the scope of this blobfuse2 instance, so don't use it outside that.
// Useful to convert UUIDs to integers once and then use for faster comparison in the fastpath.
func UUIDToUniqueInt(uuid string) int {
	common.Assert(common.IsValidUUID(uuid), uuid)

	// Fastpath, UUID already exists in the map.
	uuidToUniqueIntMapMutex.RLock()
	uuidInt, exists := uuidToUniqueInt[uuid]
	uuidToUniqueIntMapMutex.RUnlock()

	if exists {
		return uuidInt
	}

	uuidToUniqueIntMapMutex.Lock()
	defer uuidToUniqueIntMapMutex.Unlock()

	uniqueInt++
	uuidToUniqueInt[uuid] = uniqueInt

	if uniqueInt <= 0 {
		log.GetLoggerObj().Panicf("UUIDToUniqueInt: uniqueInt (%d) overflowed while adding UUID %s",
			uniqueInt, uuid)
	}

	return uniqueInt
}

// Use this when you are sure that the UUID has already been converted to an integer, and you
// just want to retrieve it.
<<<<<<< HEAD
// Unlike UUIDToUniqueInt() which adds a UUID if it doesn't exist, this helps to catch unexpected
// bugs where a UUID is not already added to the map where it should have been.
=======
>>>>>>> 3600ca86 (change locking in client pool)
func UUIDToInt(uuid string) int {
	common.Assert(common.IsValidUUID(uuid), uuid)

	uuidToUniqueIntMapMutex.RLock()
	uuidInt, exists := uuidToUniqueInt[uuid]
	_ = exists
	uuidToUniqueIntMapMutex.RUnlock()

	common.Assert(exists, uuid)
<<<<<<< HEAD
	common.Assert(uuidInt > 0, uuid)
=======
>>>>>>> 3600ca86 (change locking in client pool)

	return uuidInt
}

// Silence unused import errors for release builds.
func init() {
	os.Stat("/tmp")
}
